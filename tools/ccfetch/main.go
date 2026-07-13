// Command ccfetch downloads the OOXML test corpus described by the committed
// Common Crawl manifests (testdata/cc/manifest-*.tsv, produced by
// testdata/cc/sweep.sh).
//
// Non-truncated rows (manifest-<type>.tsv) are read straight out of the
// Common Crawl WARC archive with HTTP range requests. Truncated rows
// (manifest-<type>-truncated.tsv; Common Crawl caps stored payloads at
// 1 MiB) are refetched from their original URL on the live web — gated by a
// filtering DNS-over-HTTPS resolver supplied via -doh-url / SPINE_DOH_URL —
// so the corpus also contains files larger than the crawl's truncation
// limit. Every payload is validated as an OPC/OOXML package, classified by
// its actual content, deduplicated by SHA-256, and stored under the output
// directory. Progress is journaled to <out>/fetched.tsv so an interrupted
// run resumes where it left off.
//
// The tool is standalone (stdlib only) and is not imported by the library.
//
// Usage:
//
//	go run ./tools/ccfetch -manifest testdata/cc -out testdata/corpus/cc -n 1000
package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	ccBaseURL = "https://data.commoncrawl.org/"
	userAgent = "spine-corpus-fetch/1.0 (+github.com/mgilbir/spine)"

	maxAttempts     = 3
	politenessDelay = 150 * time.Millisecond

	// Live fetches hit origin servers, not Common Crawl's CDN: keep
	// concurrency low and pause longer between requests.
	liveConcurrency     = 2
	livePolitenessDelay = 500 * time.Millisecond
	maxRedirects        = 5

	sourceWARC = "warc"
	sourceLive = "live"
)

var docTypes = []string{"pptx", "xlsx", "docx"}

// row is one manifest line: a pointer at a single WARC response record and
// the URL it was crawled from.
type row struct {
	source       string // sourceWARC or sourceLive
	manifestType string // type the manifest filed this row under
	url          string
	warcFile     string
	offset       int64
	length       int64
	digest       string // content_digest from the CC index
}

type multiFlag []string

func (m *multiFlag) String() string { return strings.Join(*m, ",") }

func (m *multiFlag) Set(v string) error {
	*m = append(*m, v)
	return nil
}

// stats accumulates per source/manifest-type counters. blocked, dead, rot
// and tooLarge only occur on the live path.
type stats struct {
	attempted  int
	fetched    int
	truncated  int
	invalid    int
	dedup      int
	reclassed  int // valid file whose real type differed from the manifest's
	errored    int
	typeIsFull int
	blocked    int
	dead       int
	rot        int
	tooLarge   int
}

type fetcher struct {
	client      *http.Client
	liveClient  *http.Client
	gate        *hostGate
	outDir      string
	limit       int // per-type cap for WARC-sourced files
	liveLimit   int // per-type cap for live-sourced files
	liveMaxSize int64

	mu        sync.Mutex
	stateFile *os.File
	haveSHA   map[string]bool   // full sha256 hex -> already stored
	doneRow   map[string]bool   // manifest content_digest -> already processed
	counts    map[string]int    // source/type -> stored file count
	stats     map[string]*stats // source/type -> counters
	journaled int               // journal lines written this run
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "ccfetch:", err)
		os.Exit(1)
	}
}

func run() error {
	var manifests multiFlag
	flag.Var(&manifests, "manifest", "manifest TSV file or directory containing manifest-*.tsv (repeatable)")
	outDir := flag.String("out", "testdata/corpus/cc", "output corpus directory")
	limit := flag.Int("n", 1000, "stop after this many WARC-sourced valid files per type")
	liveLimit := flag.Int("live-n", 200, "stop after this many live-fetched valid files per type")
	liveMaxSize := flag.Int64("live-max-size", 50<<20, "size cap in bytes for live-fetched files")
	dohURL := flag.String("doh-url", os.Getenv("SPINE_DOH_URL"),
		"DNS-over-HTTPS resolver URL gating live fetches (required for truncated manifests; e.g. a NextDNS profile endpoint)")
	concurrency := flag.Int("concurrency", 4, "parallel fetch workers for the WARC phase")
	timeout := flag.Duration("timeout", 60*time.Second, "per-request timeout")
	flag.Parse()

	if len(manifests) == 0 {
		manifests = multiFlag{"testdata/cc"}
	}
	files, err := resolveManifests(manifests)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return errors.New("no manifest files found")
	}

	warcRows, liveRows, err := loadManifests(files)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Force HTTP/1.1 towards the WARC archive: with HTTP/2 every worker
	// multiplexes over one TCP connection, and the CDN throttling that
	// single connection stalls the whole pool. Separate HTTP/1.1
	// connections degrade independently.
	warcTransport := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		ForceAttemptHTTP2:   false,
		TLSNextProto:        map[string]func(string, *tls.Conn) http.RoundTripper{},
		MaxIdleConnsPerHost: *concurrency,
		IdleConnTimeout:     90 * time.Second,
	}
	f := &fetcher{
		client:      &http.Client{Timeout: *timeout, Transport: warcTransport},
		outDir:      *outDir,
		limit:       *limit,
		liveLimit:   *liveLimit,
		liveMaxSize: *liveMaxSize,
		haveSHA:     map[string]bool{},
		doneRow:     map[string]bool{},
		counts:      map[string]int{},
		stats:       map[string]*stats{},
	}
	f.gate = newHostGate(&http.Client{Timeout: *timeout}, *dohURL)
	f.liveClient = &http.Client{
		Timeout: *timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return fmt.Errorf("stopped after %d redirects", maxRedirects)
			}
			// Re-gate any redirect that changes hosts.
			prev := via[len(via)-1].URL.Hostname()
			next := req.URL.Hostname()
			if strings.EqualFold(prev, next) {
				return nil
			}
			v, err := f.gate.check(req.Context(), next)
			if err != nil {
				return err
			}
			switch v {
			case verdictBlocked:
				return fmt.Errorf("redirect to %s: %w", next, errGateBlocked)
			case verdictDead:
				return fmt.Errorf("redirect to %s: %w", next, errGateDead)
			}
			return nil
		},
	}
	for _, t := range docTypes {
		for _, src := range []string{sourceWARC, sourceLive} {
			f.stats[src+"/"+t] = &stats{}
		}
		if err := os.MkdirAll(filepath.Join(*outDir, t), 0o755); err != nil {
			return err
		}
	}
	if err := f.loadState(); err != nil {
		return err
	}
	defer func() { _ = f.stateFile.Close() }()

	fmt.Printf("ccfetch: %d warc rows, %d live rows, resuming with %d already stored (%s)\n",
		len(warcRows), len(liveRows), f.storedTotal(), f.countsString())

	f.runPhase(ctx, warcRows, *concurrency, sourceWARC)

	if len(liveRows) > 0 {
		if *dohURL == "" {
			fmt.Fprintln(os.Stderr, "ccfetch: skipping live phase: truncated manifests present but no DoH resolver configured;")
			fmt.Fprintln(os.Stderr, "ccfetch: live fetches require the blocklist gate — set -doh-url or SPINE_DOH_URL to your DNS-over-HTTPS resolver")
		} else if ctx.Err() == nil {
			f.runPhase(ctx, liveRows, liveConcurrency, sourceLive)
		}
	}

	f.printSummary()
	if ctx.Err() != nil {
		return errors.New("interrupted (state saved; rerun to resume)")
	}
	return nil
}

// runPhase drains rows through a worker pool.
func (f *fetcher) runPhase(ctx context.Context, rows []row, workers int, source string) {
	jobs := make(chan row)
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for r := range jobs {
				f.process(ctx, r)
			}
		}()
	}
feed:
	for _, r := range rows {
		if f.allFull(source) {
			break
		}
		select {
		case jobs <- r:
		case <-ctx.Done():
			break feed
		}
	}
	close(jobs)
	wg.Wait()
}

// resolveManifests expands directory arguments into manifest-*.tsv globs.
func resolveManifests(args []string) ([]string, error) {
	var files []string
	for _, a := range args {
		info, err := os.Stat(a)
		if err != nil {
			return nil, err
		}
		if info.IsDir() {
			matches, err := filepath.Glob(filepath.Join(a, "manifest-*.tsv"))
			if err != nil {
				return nil, err
			}
			sort.Strings(matches)
			files = append(files, matches...)
		} else {
			files = append(files, a)
		}
	}
	return files, nil
}

// parseManifestName derives (type, source) from a manifest file name:
// manifest-<type>.tsv fetches from the WARC archive, and
// manifest-<type>-truncated.tsv refetches from the live web.
func parseManifestName(base string) (typ, source string, err error) {
	name := strings.TrimSuffix(strings.TrimPrefix(base, "manifest-"), ".tsv")
	source = sourceWARC
	if strings.HasSuffix(name, "-truncated") {
		source = sourceLive
		name = strings.TrimSuffix(name, "-truncated")
	}
	if !isDocType(name) {
		return "", "", fmt.Errorf("%s: cannot derive document type (want manifest-{pptx,xlsx,docx}[-truncated].tsv)", base)
	}
	return name, source, nil
}

// loadManifests reads every manifest, splitting rows by source and
// interleaving them across types so all corpora make progress from the
// start of the run.
func loadManifests(files []string) (warcRows, liveRows []row, err error) {
	var warcSets, liveSets [][]row
	for _, path := range files {
		typ, source, err := parseManifestName(filepath.Base(path))
		if err != nil {
			return nil, nil, err
		}
		rows, err := loadManifest(path, typ, source)
		if err != nil {
			return nil, nil, err
		}
		if source == sourceLive {
			liveSets = append(liveSets, rows)
		} else {
			warcSets = append(warcSets, rows)
		}
	}
	return interleave(warcSets), interleave(liveSets), nil
}

func interleave(sets [][]row) []row {
	var out []row
	for i := 0; ; i++ {
		added := false
		for _, rows := range sets {
			if i < len(rows) {
				out = append(out, rows[i])
				added = true
			}
		}
		if !added {
			return out
		}
	}
}

func loadManifest(path, typ, source string) ([]row, error) {
	fh, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = fh.Close() }()

	var rows []row
	sc := bufio.NewScanner(fh)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := sc.Text()
		if lineNo == 1 || line == "" { // header
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) != 5 {
			return nil, fmt.Errorf("%s:%d: want 5 tab-separated fields, got %d", path, lineNo, len(fields))
		}
		off, err := strconv.ParseInt(fields[2], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%s:%d: bad offset: %w", path, lineNo, err)
		}
		length, err := strconv.ParseInt(fields[3], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%s:%d: bad length: %w", path, lineNo, err)
		}
		rows = append(rows, row{
			source:       source,
			manifestType: typ,
			url:          fields[0],
			warcFile:     fields[1],
			offset:       off,
			length:       length,
			digest:       fields[4],
		})
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return rows, nil
}

func isDocType(t string) bool {
	for _, d := range docTypes {
		if t == d {
			return true
		}
	}
	return false
}

func (f *fetcher) limitFor(source string) int {
	if source == sourceLive {
		return f.liveLimit
	}
	return f.limit
}

// loadState opens (creating if needed) the fetched.tsv journal and replays
// it. Journal columns: sha256, outcome, url, manifest_digest, size, source.
// outcome is a document type for stored files, or a terminal skip reason
// (truncated, invalid, dedup, type-full, blocked, dead, rot, too-large,
// error-terminal) so restarts do not refetch known-bad rows.
func (f *fetcher) loadState() error {
	path := filepath.Join(f.outDir, "fetched.tsv")
	if data, err := os.ReadFile(path); err == nil {
		for i, line := range strings.Split(string(data), "\n") {
			if i == 0 || line == "" {
				continue
			}
			fields := strings.Split(line, "\t")
			if len(fields) != 6 {
				return fmt.Errorf("%s: malformed journal line %d", path, i+1)
			}
			sha, outcome, digest, source := fields[0], fields[1], fields[3], fields[5]
			f.doneRow[digest] = true
			if isDocType(outcome) {
				f.haveSHA[sha] = true
				f.counts[source+"/"+outcome]++
			}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	fh, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	if st, err := fh.Stat(); err == nil && st.Size() == 0 {
		if _, err := fh.WriteString("sha256\toutcome\turl\tmanifest_digest\tsize\tsource\n"); err != nil {
			return err
		}
	}
	f.stateFile = fh
	return nil
}

// journal must be called with f.mu held.
func (f *fetcher) journal(sha, outcome string, r row, size int) {
	line := fmt.Sprintf("%s\t%s\t%s\t%s\t%d\t%s\n", sha, outcome, r.url, r.digest, size, r.source)
	if _, err := f.stateFile.WriteString(line); err != nil {
		fmt.Fprintf(os.Stderr, "ccfetch: journal write: %v\n", err)
	}
	f.doneRow[r.digest] = true
	f.journaled++
	if f.journaled%250 == 0 {
		var parts []string
		for _, src := range []string{sourceWARC, sourceLive} {
			for _, t := range docTypes {
				if c := f.counts[src+"/"+t]; c > 0 {
					parts = append(parts, fmt.Sprintf("%s/%s=%d", src, t, c))
				}
			}
		}
		fmt.Printf("ccfetch: progress: %d rows journaled (%s)\n", f.journaled, strings.Join(parts, " "))
	}
}

func (f *fetcher) allFull(source string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, t := range docTypes {
		if f.counts[source+"/"+t] < f.limitFor(source) {
			return false
		}
	}
	return true
}

func (f *fetcher) storedTotal() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, c := range f.counts {
		n += c
	}
	return n
}

func (f *fetcher) countsString() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	var parts []string
	for _, src := range []string{sourceWARC, sourceLive} {
		for _, t := range docTypes {
			if c := f.counts[src+"/"+t]; c > 0 {
				parts = append(parts, fmt.Sprintf("%s/%s=%d", src, t, c))
			}
		}
	}
	if len(parts) == 0 {
		return "empty"
	}
	return strings.Join(parts, " ")
}

// process handles one manifest row end to end.
func (f *fetcher) process(ctx context.Context, r row) {
	if ctx.Err() != nil {
		return
	}
	key := r.source + "/" + r.manifestType
	f.mu.Lock()
	if f.doneRow[r.digest] || f.counts[key] >= f.limitFor(r.source) {
		f.mu.Unlock()
		return
	}
	st := f.stats[key]
	st.attempted++
	f.mu.Unlock()

	var payload []byte
	if r.source == sourceLive {
		data, err := f.fetchLive(ctx, r)
		if err != nil {
			f.recordLiveFailure(ctx, st, r, err)
			return
		}
		payload = data
	} else {
		raw, retryable, err := f.fetchRange(ctx, r)
		if err != nil {
			f.mu.Lock()
			st.errored++
			if ctx.Err() == nil && !retryable {
				// Terminal fetch failure (bad range, 404): journal it so
				// the row is not endlessly retried on resume. Transient
				// failures (throttling, timeouts) stay unjournaled and are
				// picked up again by the next run.
				f.journal("-", "error-terminal", r, 0)
			}
			f.mu.Unlock()
			return
		}
		data, err := decodeRecord(raw)
		if err != nil {
			f.mu.Lock()
			defer f.mu.Unlock()
			if errors.Is(err, errTruncated) {
				st.truncated++
				f.journal("-", "truncated", r, 0)
			} else {
				st.invalid++
				f.journal("-", "invalid", r, 0)
			}
			return
		}
		payload = data
	}
	f.store(st, r, payload)
}

// recordLiveFailure buckets a live-path error into blocked/dead/rot.
func (f *fetcher) recordLiveFailure(ctx context.Context, st *stats, r row, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if ctx.Err() != nil {
		st.errored++
		return // interrupted: leave the row retryable
	}
	switch {
	case errors.Is(err, errGateBlocked):
		st.blocked++
		f.journal("-", "blocked", r, 0)
	case errors.Is(err, errGateDead):
		st.dead++
		f.journal("-", "dead", r, 0)
	case errors.Is(err, errTooLarge):
		st.tooLarge++
		f.journal("-", "too-large", r, 0)
	default:
		st.rot++
		f.journal("-", "rot", r, 0)
	}
}

// store validates, classifies, dedups, and writes one payload.
func (f *fetcher) store(st *stats, r row, payload []byte) {
	actual, err := classifyOOXML(payload)
	if err != nil {
		f.mu.Lock()
		st.invalid++
		f.journal("-", "invalid", r, 0)
		f.mu.Unlock()
		return
	}

	sum := sha256.Sum256(payload)
	sha := hex.EncodeToString(sum[:])

	f.mu.Lock()
	defer f.mu.Unlock()
	if f.haveSHA[sha] {
		st.dedup++
		f.journal(sha, "dedup", r, 0)
		return
	}
	key := r.source + "/" + actual
	if f.counts[key] >= f.limitFor(r.source) {
		st.typeIsFull++
		f.journal(sha, "type-full", r, 0)
		return
	}
	name := filepath.Join(f.outDir, actual, sha[:16]+"."+actual)
	if err := os.WriteFile(name, payload, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "ccfetch: write %s: %v\n", name, err)
		st.errored++
		return
	}
	f.haveSHA[sha] = true
	f.counts[key]++
	st.fetched++
	if actual != r.manifestType {
		st.reclassed++
	}
	f.journal(sha, actual, r, len(payload))
}

// errTooLarge marks live payloads exceeding -live-max-size.
var errTooLarge = errors.New("live: payload exceeds the size cap")

// fetchLive refetches a truncated candidate from its original URL, gated by
// the DoH blocklist check on the initial host and on every cross-host
// redirect. One retry on transient failures.
func (f *fetcher) fetchLive(ctx context.Context, r row) ([]byte, error) {
	u, err := url.Parse(r.url)
	if err != nil {
		return nil, fmt.Errorf("live: bad url: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("live: unsupported scheme %q", u.Scheme)
	}
	v, err := f.gate.check(ctx, u.Hostname())
	if err != nil {
		return nil, fmt.Errorf("%w (gate: %v)", errGateDead, err)
	}
	switch v {
	case verdictBlocked:
		return nil, errGateBlocked
	case verdictDead:
		return nil, errGateDead
	}

	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(2 * time.Second):
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}
		data, retryable, err := f.tryFetchLive(ctx, r.url)
		if err == nil {
			select {
			case <-time.After(livePolitenessDelay):
			case <-ctx.Done():
			}
			return data, nil
		}
		lastErr = err
		if !retryable || ctx.Err() != nil {
			break
		}
	}
	return nil, lastErr
}

func (f *fetcher) tryFetchLive(ctx context.Context, rawURL string) (data []byte, retryable bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := f.liveClient.Do(req)
	if err != nil {
		// Gate sentinels from CheckRedirect arrive wrapped in *url.Error.
		if errors.Is(err, errGateBlocked) || errors.Is(err, errGateDead) {
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
	data, err = io.ReadAll(io.LimitReader(resp.Body, f.liveMaxSize+1))
	if err != nil {
		return nil, true, err
	}
	if int64(len(data)) > f.liveMaxSize {
		return nil, false, errTooLarge
	}
	return data, false, nil
}

// decodeRecord gunzips the raw WARC record bytes and extracts the payload.
func decodeRecord(raw []byte) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("warc: record gzip: %w", err)
	}
	gz.Multistream(false)
	return extractPayload(gz)
}

// fetchRange issues the ranged request for one WARC record with retry on
// transient failures (403/429 throttling, 5xx, network/timeout errors). The
// returned bool reports whether the final failure was transient.
func (f *fetcher) fetchRange(ctx context.Context, r row) ([]byte, bool, error) {
	var lastErr error
	lastRetryable := true
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			backoff += time.Duration(rand.Int63n(int64(backoff))) // jitter
			select {
			case <-time.After(backoff):
			case <-ctx.Done():
				return nil, true, ctx.Err()
			}
		}
		data, retryable, err := f.tryFetch(ctx, r)
		if err == nil {
			// Small politeness delay so each worker paces its requests.
			select {
			case <-time.After(politenessDelay):
			case <-ctx.Done():
			}
			return data, false, nil
		}
		lastErr, lastRetryable = err, retryable
		if !retryable || ctx.Err() != nil {
			break
		}
	}
	return nil, lastRetryable, lastErr
}

func (f *fetcher) tryFetch(ctx context.Context, r row) (data []byte, retryable bool, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ccBaseURL+r.warcFile, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", r.offset, r.offset+r.length-1))

	resp, err := f.client.Do(req)
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
		return nil, retry, fmt.Errorf("GET %s range %d+%d: status %s", r.warcFile, r.offset, r.length, resp.Status)
	}
	data, err = io.ReadAll(io.LimitReader(resp.Body, r.length+1))
	if err != nil {
		return nil, true, err
	}
	if int64(len(data)) != r.length {
		return nil, true, fmt.Errorf("GET %s: got %d bytes, want %d", r.warcFile, len(data), r.length)
	}
	return data, false, nil
}

func (f *fetcher) printSummary() {
	f.mu.Lock()
	defer f.mu.Unlock()
	fmt.Println("\nccfetch summary — WARC phase (rows grouped by manifest type):")
	fmt.Printf("%-6s %10s %8s %10s %8s %6s %10s %7s %10s\n",
		"type", "attempted", "fetched", "truncated", "invalid", "dedup", "reclassed", "errors", "type-full")
	for _, t := range docTypes {
		s := f.stats[sourceWARC+"/"+t]
		fmt.Printf("%-6s %10d %8d %10d %8d %6d %10d %7d %10d\n",
			t, s.attempted, s.fetched, s.truncated, s.invalid, s.dedup, s.reclassed, s.errored, s.typeIsFull)
	}
	fmt.Println("\nccfetch summary — live phase (truncated candidates):")
	fmt.Printf("%-6s %10s %8s %8s %6s %6s %10s %8s %6s %10s %7s\n",
		"type", "attempted", "fetched", "blocked", "dead", "rot", "too-large", "invalid", "dedup", "reclassed", "errors")
	for _, t := range docTypes {
		s := f.stats[sourceLive+"/"+t]
		fmt.Printf("%-6s %10d %8d %8d %6d %6d %10d %8d %6d %10d %7d\n",
			t, s.attempted, s.fetched, s.blocked, s.dead, s.rot, s.tooLarge, s.invalid, s.dedup, s.reclassed, s.errored)
	}
	fmt.Println("\nstored files per source/type (including previous runs):")
	for _, src := range []string{sourceWARC, sourceLive} {
		for _, t := range docTypes {
			fmt.Printf("  %s/%-5s %d\n", src, t, f.counts[src+"/"+t])
		}
	}
}
