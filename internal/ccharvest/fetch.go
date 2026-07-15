package ccharvest

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ErrTooLarge marks a live payload that exceeded the caller's size cap.
var ErrTooLarge = errors.New("live: payload exceeds the size cap")

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
		return nil, retry, fmt.Errorf("GET %s range %d+%d: status %s", warcFile, offset, length, resp.Status)
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
		return nil, retry, fmt.Errorf("GET %s: status %s", rawURL, resp.Status)
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
