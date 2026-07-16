package ccharvest

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"syscall"
	"time"
)

// ErrTooLarge marks a live payload that exceeded the caller's size cap.
var ErrTooLarge = errors.New("live: payload exceeds the size cap")

// HTTPStatusError reports a non-success HTTP status from a fetch attempt. It
// carries the numeric code so callers can classify the outcome (e.g. 404/410
// permanent, 429/5xx transient) via errors.As without parsing the message.
type HTTPStatusError struct {
	Code   int    // HTTP status code
	Status string // full status line, e.g. "404 Not Found"
	Detail string // request context, e.g. "GET https://host/a.docx"
}

func (e *HTTPStatusError) Error() string {
	return fmt.Sprintf("%s: status %s", e.Detail, e.Status)
}

// FetchDisposition is the durable disposition of one failed fetch attempt.
type FetchDisposition int

const (
	// DispTransient marks a failure worth retrying: a timeout, HTTP 429, an
	// HTTP 5xx, a connection reset, or a temporary DNS failure (SERVFAIL). The
	// caller defers the reference, subject to a retry cap.
	DispTransient FetchDisposition = iota
	// DispPermanent marks a failure that will never succeed on retry: a DNS
	// NXDOMAIN / no-such-host, a connection refused, a TLS/certificate failure,
	// an HTTP 4xx (404/410/403/401 and most others), a blocked or dead gate
	// verdict, an over-cap body, or a payload that is not a valid OOXML
	// package. The caller writes a terminal ledger entry and never retries.
	DispPermanent
)

func (d FetchDisposition) String() string {
	if d == DispTransient {
		return "transient"
	}
	return "permanent"
}

// ClassifyFetchError maps a non-nil fetch error to a durable disposition and a
// stable "fetch:*" signature. transientHint is the low-level retryable flag the
// fetch helper returned; it encodes path-specific knowledge the error text
// cannot (the WARC CDN throttles with 403, which is transient there but a
// refusal at a live origin; a WARC-record decode failure is permanent). A
// recognizable error — a gate verdict, an HTTP status, a DNS/connection/TLS
// failure, or a timeout — overrides the hint with a specific verdict; only an
// otherwise-unrecognized error falls back to it.
func ClassifyFetchError(err error, transientHint bool) (FetchDisposition, string) {
	switch {
	case err == nil:
		return DispTransient, ""
	case errors.Is(err, ErrNotOOXML):
		return DispPermanent, "fetch:not-ooxml"
	case errors.Is(err, ErrGateDead):
		return DispPermanent, "fetch:dns"
	case errors.Is(err, ErrGateBlocked):
		return DispPermanent, "fetch:blocked"
	case errors.Is(err, ErrTooLarge):
		return DispPermanent, "fetch:too-large"
	}

	var se *HTTPStatusError
	if errors.As(err, &se) {
		return classifyStatus(se.Code, transientHint)
	}

	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		if dnsErr.IsNotFound {
			return DispPermanent, "fetch:dns" // NXDOMAIN / no such host
		}
		return DispTransient, "fetch:dns-temp" // SERVFAIL / temporary resolver failure
	}

	switch {
	case errors.Is(err, syscall.ECONNREFUSED):
		return DispPermanent, "fetch:conn-refused"
	case errors.Is(err, syscall.ECONNRESET):
		return DispTransient, "fetch:conn-reset"
	}

	if isTLSError(err) {
		return DispPermanent, "fetch:tls"
	}

	if errors.Is(err, context.DeadlineExceeded) {
		return DispTransient, "fetch:timeout"
	}
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return DispTransient, "fetch:timeout"
	}
	if errors.Is(err, context.Canceled) {
		return DispTransient, "fetch:canceled"
	}

	// Unrecognized: honor the low-level layer's retryable hint (e.g. a WARC
	// decode failure arrives here with the hint already set to permanent).
	if transientHint {
		return DispTransient, "fetch:transient"
	}
	return DispPermanent, "fetch:terminal"
}

// classifyStatus classifies an HTTP status code. 429/408/5xx are transient;
// other 4xx are a permanent origin refusal unless the low-level layer flagged
// the response transient (the WARC CDN throttles with 403).
func classifyStatus(code int, transientHint bool) (FetchDisposition, string) {
	sig := fmt.Sprintf("fetch:http-%d", code)
	switch {
	case code == http.StatusTooManyRequests || code == http.StatusRequestTimeout || code >= 500:
		return DispTransient, sig
	case transientHint:
		return DispTransient, sig
	default:
		return DispPermanent, sig
	}
}

// isTLSError reports whether err is a TLS handshake or certificate-validation
// failure, which is terminal for a fetch.
func isTLSError(err error) bool {
	var rhe *tls.RecordHeaderError
	if errors.As(err, &rhe) {
		return true
	}
	var cve *tls.CertificateVerificationError
	if errors.As(err, &cve) {
		return true
	}
	var uae x509.UnknownAuthorityError
	if errors.As(err, &uae) {
		return true
	}
	var hne x509.HostnameError
	if errors.As(err, &hne) {
		return true
	}
	var cie x509.CertificateInvalidError
	return errors.As(err, &cie)
}

// NewWARCTransport builds an HTTP/1.1-only transport for the WARC archive.
//
// With HTTP/2 every worker multiplexes over one TCP connection, and the CDN
// throttling that single connection stalls the whole pool. Separate HTTP/1.1
// connections degrade independently, so HTTP/2 is disabled and the idle-conn
// pool is sized to the worker count.
func NewWARCTransport(maxConnsPerHost int) *http.Transport {
	if maxConnsPerHost < 1 {
		maxConnsPerHost = 1
	}
	return &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		ForceAttemptHTTP2:   false,
		TLSNextProto:        map[string]func(string, *tls.Conn) http.RoundTripper{},
		MaxIdleConnsPerHost: maxConnsPerHost,
		IdleConnTimeout:     90 * time.Second,
	}
}

// TryFetchRange issues one ranged GET for a single WARC record and returns the
// raw (still gzipped) record bytes. The bool reports whether a failure looks
// transient (throttling/5xx/network) and is worth retrying.
func TryFetchRange(ctx context.Context, client *http.Client, warcFile string, offset, length int64) (data []byte, retryable bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, CCBaseURL+warcFile, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", UserAgent)
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", offset, offset+length-1))

	resp, err := client.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusPartialContent {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		// data.commoncrawl.org throttles with 403 as well as 429.
		retry := resp.StatusCode == http.StatusTooManyRequests ||
			resp.StatusCode == http.StatusForbidden ||
			resp.StatusCode >= 500
		return nil, retry, &HTTPStatusError{
			Code:   resp.StatusCode,
			Status: resp.Status,
			Detail: fmt.Sprintf("GET %s range %d+%d", warcFile, offset, length),
		}
	}
	data, err = io.ReadAll(io.LimitReader(resp.Body, length+1))
	if err != nil {
		return nil, true, err
	}
	if int64(len(data)) != length {
		return nil, true, fmt.Errorf("GET %s: got %d bytes, want %d", warcFile, len(data), length)
	}
	return data, false, nil
}

// TryFetchLive issues one GET against a live origin URL and returns the body,
// enforcing the size cap. The caller is responsible for gating the host first
// (see HostGate) and for retry/politeness. The bool reports whether a failure
// looks transient.
func TryFetchLive(ctx context.Context, client *http.Client, rawURL string, maxSize int64) (data []byte, retryable bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", UserAgent)

	resp, err := client.Do(req)
	if err != nil {
		// Gate sentinels from a CheckRedirect hook arrive wrapped in *url.Error.
		if errors.Is(err, ErrGateBlocked) || errors.Is(err, ErrGateDead) {
			return nil, false, err
		}
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		retry := resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500
		return nil, retry, &HTTPStatusError{
			Code:   resp.StatusCode,
			Status: resp.Status,
			Detail: "GET " + rawURL,
		}
	}
	data, err = io.ReadAll(io.LimitReader(resp.Body, maxSize+1))
	if err != nil {
		return nil, true, err
	}
	if int64(len(data)) > maxSize {
		return nil, false, ErrTooLarge
	}
	return data, false, nil
}
