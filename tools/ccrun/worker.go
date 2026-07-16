package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/mgilbir/spine/common/validate"
	"github.com/mgilbir/spine/docx"
	"github.com/mgilbir/spine/internal/ccharvest"
	"github.com/mgilbir/spine/internal/testutil"
	"github.com/mgilbir/spine/pptx"
	"github.com/mgilbir/spine/xlsx"
)

// saver is the save surface shared by the three document types.
type saver interface {
	SaveBytes() ([]byte, error)
}

// validator is the pre-save validation surface shared by the three types.
type validator interface {
	Validate() validate.Report
}

func openDoc(typ string, data []byte) (saver, error) {
	r := bytes.NewReader(data)
	switch typ {
	case "pptx":
		return pptx.OpenReader(r, int64(len(data)))
	case "xlsx":
		return xlsx.OpenReader(r, int64(len(data)))
	case "docx":
		return docx.OpenReader(r, int64(len(data)))
	}
	return nil, fmt.Errorf("unknown type %q", typ)
}

// result is a single-file test outcome. ok means the file survived every stage.
type result struct {
	ok        bool
	stage     string
	signature string
}

// testBytes runs the four-stage round-trip discipline over one file's bytes,
// mirroring cctest: Open, Validate (error-severity findings fail the validate
// stage), zero-modification SaveBytes, reopen, and part-by-part byte fidelity.
func testBytes(typ string, data []byte) result {
	doc, err := openDoc(typ, data)
	if err != nil {
		return result{stage: "open", signature: signature(err)}
	}
	if v, ok := doc.(validator); ok {
		report := v.Validate()
		if report.HasErrors() {
			return result{stage: "validate", signature: signature(report.Errors())}
		}
	}
	saved, err := doc.SaveBytes()
	if err != nil {
		return result{stage: "save", signature: signature(err)}
	}
	if _, err := openDoc(typ, saved); err != nil {
		return result{stage: "reopen", signature: signature(err)}
	}

	origParts, err := testutil.ReadZipPartsBytes(data)
	if err != nil {
		return result{stage: "fidelity", signature: signature(fmt.Errorf("re-reading original parts: %w", err))}
	}
	savedParts, err := testutil.ReadZipPartsBytes(saved)
	if err != nil {
		return result{stage: "fidelity", signature: signature(fmt.Errorf("reading saved parts: %w", err))}
	}
	identical := 0
	var changed, missing, extra []string
	for _, name := range testutil.SortedKeys(origParts) {
		sv, ok := savedParts[name]
		switch {
		case !ok:
			missing = append(missing, name)
		case !bytes.Equal(origParts[name], sv):
			changed = append(changed, name)
		default:
			identical++
		}
	}
	for _, name := range testutil.SortedKeys(savedParts) {
		if _, ok := origParts[name]; !ok {
			extra = append(extra, name)
		}
	}
	if len(changed)+len(missing)+len(extra) > 0 {
		first := ""
		switch {
		case len(changed) > 0:
			first = "changed " + changed[0]
		case len(missing) > 0:
			first = "missing " + missing[0]
		default:
			first = "extra " + extra[0]
		}
		return result{stage: "fidelity", signature: signature(fmt.Errorf(
			"%d identical, %d changed, %d missing, %d extra parts (first: %s)",
			identical, len(changed), len(missing), len(extra), first))}
	}
	return result{ok: true}
}

// fetchRef downloads the file a Ref points at and returns its bytes. WARC refs
// are read by range from the archive and decoded; live (truncated) refs are
// refetched from the origin URL through the DoH blocklist gate. dohURL may be
// empty only for WARC refs. The retryable bool distinguishes a transient
// failure (CDN throttling, timeout, network) — which the orchestrator defers
// so a later run retries it — from a terminal one (dead record, gate block).
func fetchRef(ctx context.Context, ref Ref, dohURL string, timeout time.Duration) (data []byte, retryable bool, err error) {
	if ref.Kind == kindLive {
		return fetchLive(ctx, ref, dohURL, timeout)
	}
	return fetchWARC(ctx, ref, timeout)
}

func fetchWARC(ctx context.Context, ref Ref, timeout time.Duration) ([]byte, bool, error) {
	client := &http.Client{Timeout: timeout, Transport: ccharvest.NewWARCTransport(1)}
	var lastErr error
	lastRetryable := true
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(time.Duration(attempt) * time.Second):
			case <-ctx.Done():
				return nil, true, ctx.Err()
			}
		}
		raw, retryable, err := ccharvest.TryFetchRange(ctx, client, ref.WARCFile, ref.Offset, ref.Length)
		if err == nil {
			payload, derr := ccharvest.DecodeRecord(raw)
			if derr != nil {
				// A decode failure (e.g. a truncated/rotten record) is terminal.
				return nil, false, fmt.Errorf("warc decode: %w", derr)
			}
			return payload, false, nil
		}
		lastErr, lastRetryable = err, retryable
		if !retryable || ctx.Err() != nil {
			break
		}
	}
	return nil, lastRetryable, fmt.Errorf("warc fetch: %w", lastErr)
}

const liveMaxSize = 50 << 20 // 50 MiB cap for live refetches

func fetchLive(ctx context.Context, ref Ref, dohURL string, timeout time.Duration) ([]byte, bool, error) {
	if dohURL == "" {
		return nil, false, errors.New("live fetch requires a DoH resolver (-doh-url / SPINE_DOH_URL)")
	}
	u, err := url.Parse(ref.URL)
	if err != nil {
		return nil, false, fmt.Errorf("live: bad url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, false, fmt.Errorf("live: unsupported scheme %q", u.Scheme)
	}
	gate := ccharvest.NewHostGate(&http.Client{Timeout: timeout}, dohURL)
	v, err := gate.Check(ctx, u.Hostname())
	if err != nil {
		return nil, true, fmt.Errorf("%w (gate: %v)", ccharvest.ErrGateDead, err)
	}
	switch v {
	case ccharvest.VerdictBlocked:
		return nil, false, ccharvest.ErrGateBlocked
	case ccharvest.VerdictDead:
		return nil, false, ccharvest.ErrGateDead
	}
	client := &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("stopped after 5 redirects")
			}
			prev := via[len(via)-1].URL.Hostname()
			next := req.URL.Hostname()
			if strings.EqualFold(prev, next) {
				return nil
			}
			gv, err := gate.Check(req.Context(), next)
			if err != nil {
				return err
			}
			switch gv {
			case ccharvest.VerdictBlocked:
				return fmt.Errorf("redirect to %s: %w", next, ccharvest.ErrGateBlocked)
			case ccharvest.VerdictDead:
				return fmt.Errorf("redirect to %s: %w", next, ccharvest.ErrGateDead)
			}
			return nil
		},
	}
	data, retryable, err := ccharvest.TryFetchLive(ctx, client, ref.URL, liveMaxSize)
	if err != nil {
		return nil, retryable, fmt.Errorf("live fetch: %w", err)
	}
	return data, false, nil
}

// runWorker fetches one Ref to the scratch file, tests it, prints exactly one
// result line, deletes the scratch file, and returns. It exits 0 for any TEST
// outcome (pass or fail); only a genuine crash/OOM/timeout leaves the process
// with a nonzero status or no output, which the orchestrator reads as a
// resource kill.
func runWorker(ref Ref, scratch, dohURL string, timeout time.Duration) {
	ctx := context.Background()
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	// Fetch to scratch, and always remove it before returning.
	data, retryable, ferr := fetchRef(ctx, ref, dohURL, timeout)
	if scratch != "" && len(data) > 0 {
		_ = os.WriteFile(scratch, data, 0o644)
	}
	defer func() {
		if scratch != "" {
			_ = os.Remove(scratch)
		}
	}()

	if ferr != nil {
		// Classify the failure durably: a transient outcome (throttle, timeout,
		// 429/5xx, connection reset, temporary DNS) is deferred for a later
		// batch; a permanent one (dead host, connection refused, TLS failure,
		// 4xx, gate block) is burned now with a specific fetch:* signature so a
		// dead origin never loops.
		disp, sig := ccharvest.ClassifyFetchError(ferr, retryable)
		if disp == ccharvest.DispTransient {
			fmt.Printf("%s\t%s\t%s\t%s\n", ref.Digest, outcomeRetry, "fetch", sig)
			return
		}
		emitResult(ref.Digest, result{stage: "fetch", signature: sig})
		return
	}
	// Re-read from scratch when present, exercising the real on-disk path.
	if scratch != "" {
		if b, err := os.ReadFile(scratch); err == nil {
			data = b
		}
	}
	// A dead origin can answer a live refetch with an HTML error page or a
	// login redirect (HTTP 200, non-file body). Reject any non-OOXML payload at
	// the fetch stage — as a permanent failure — before handing it to Open, so
	// it is categorized as a fetch failure, not a spurious open failure.
	if err := ccharvest.ValidateOOXMLPackage(data); err != nil {
		emitResult(ref.Digest, result{stage: "fetch", signature: "fetch:not-ooxml"})
		return
	}
	emitResult(ref.Digest, testBytes(ref.Type, data))
}

// emitResult prints the single worker result line to stdout.
func emitResult(digest string, r result) {
	if r.ok {
		fmt.Printf("%s\t%s\n", digest, outcomePass)
		return
	}
	fmt.Printf("%s\t%s\t%s\t%s\n", digest, outcomeFail, r.stage, r.signature)
}

// signature normalizes an error into a stable grouping key: digit runs
// collapsed to N, whitespace flattened, truncated. It matches cctest's
// signature so quarantine rows cluster consistently across the two tools.
func signature(err error) string {
	msg := err.Error()
	var b strings.Builder
	lastDigit := false
	for _, r := range msg {
		switch {
		case r >= '0' && r <= '9':
			if !lastDigit {
				b.WriteByte('N')
			}
			lastDigit = true
		case r == '\t' || r == '\n' || r == '\r':
			b.WriteByte(' ')
			lastDigit = false
		default:
			b.WriteRune(r)
			lastDigit = false
		}
	}
	s := b.String()
	if len(s) > 160 {
		s = s[:160]
	}
	return s
}
