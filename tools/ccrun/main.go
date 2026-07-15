// Command ccrun processes a batched, reference-only Common Crawl OOXML harvest.
//
// It consumes the committed reference manifests (testdata/cc/manifest-*.tsv;
// see testdata/cc/sweep-multi.sh) and, one batch at a time, fetches each
// referenced file to a scratch area, runs the library round-trip discipline
// against it, records the outcome to a durable ledger, catalogs failures in an
// aggregate quarantine, and deletes the binary. Binaries are transient: only
// references are ever committed.
//
// Each file is tested in a separate worker subprocess with a per-file timeout,
// so a single pathological file (OOM, hang, or panic) is recorded as one
// quarantine row instead of killing the batch. Under a systemd-run cgroup with
// MemoryMax set, a memory-blowing file makes the kernel OOM-kill the worker
// (the largest process); the lightweight orchestrator survives and records it.
//
// Modes:
//
//	ccrun -manifest testdata/cc -ledger L -quarantine Q -scratch S   (orchestrator: one batch)
//	ccrun -worker -scratch S ... < ref.json                          (worker: one reference)
//	ccrun -refetch <digest> -manifest testdata/cc -out FILE          (refetch one reference for debugging)
//
// stdlib + internal/ccharvest only; not imported by the library.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/mgilbir/spine/internal/ccharvest"
)

type multiFlag []string

func (m *multiFlag) String() string { return fmt.Sprint([]string(*m)) }
func (m *multiFlag) Set(v string) error {
	*m = append(*m, v)
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "ccrun:", err)
		os.Exit(1)
	}
}

func run() error {
	var manifests multiFlag
	flag.Var(&manifests, "manifest", "manifest TSV file or directory of manifest-*.tsv (repeatable)")
	worker := flag.Bool("worker", false, "run as a single-reference worker subprocess (reads a Ref as JSON on stdin)")
	refetch := flag.String("refetch", "", "refetch the reference with this content_digest to -out and exit (debug)")
	out := flag.String("out", "", "refetch destination path")
	ledgerPath := flag.String("ledger", "testdata/corpus/cc-batch/ledger.tsv", "durable per-reference progress ledger")
	quarantinePath := flag.String("quarantine", "testdata/cc/batch-quarantine.tsv", "aggregate reference-keyed failure catalog")
	scratch := flag.String("scratch", "testdata/corpus/cc-batch/scratch", "scratch dir (orchestrator) or scratch file (worker)")
	batch := flag.Int("batch", 2000, "references to process in this invocation")
	workers := flag.Int("workers", 2, "concurrent worker subprocesses")
	timeout := flag.Duration("timeout", 90*time.Second, "per-file timeout")
	dohURL := flag.String("doh-url", os.Getenv("SPINE_DOH_URL"),
		"DoH resolver gating live (truncated) refetches; empty skips truncated rows")
	flag.Parse()

	switch {
	case *worker:
		return runWorkerMode(*scratch, *dohURL, *timeout)
	case *refetch != "":
		return runRefetch(manifests, *refetch, *out, *dohURL, *timeout)
	default:
		return runOrchestrator(orchConfig{
			manifests:  manifests,
			ledger:     *ledgerPath,
			quarantine: *quarantinePath,
			scratchDir: *scratch,
			batch:      *batch,
			workers:    *workers,
			timeout:    *timeout,
			dohURL:     *dohURL,
		})
	}
}

// runWorkerMode reads a single Ref as JSON from stdin and runs the worker.
func runWorkerMode(scratch, dohURL string, timeout time.Duration) error {
	data, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("worker: reading stdin: %w", err)
	}
	var ref Ref
	if err := json.Unmarshal(data, &ref); err != nil {
		return fmt.Errorf("worker: bad ref json: %w", err)
	}
	runWorker(ref, scratch, dohURL, timeout)
	return nil
}

// runRefetch downloads one reference (found by digest) to a named path and
// leaves it there for debugging.
func runRefetch(manifestArgs []string, digest, out, dohURL string, timeout time.Duration) error {
	if out == "" {
		return errors.New("refetch: -out is required")
	}
	if len(manifestArgs) == 0 {
		manifestArgs = []string{"testdata/cc"}
	}
	files, err := resolveManifests(manifestArgs)
	if err != nil {
		return err
	}
	refs, _, err := loadAllRefs(files)
	if err != nil {
		return err
	}
	var found *Ref
	for i := range refs {
		if refs[i].Digest == digest {
			found = &refs[i]
			break
		}
	}
	if found == nil {
		return fmt.Errorf("refetch: digest %s not found in the given manifests", digest)
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout+15*time.Second)
	defer cancel()
	data, _, err := fetchRef(ctx, *found, dohURL, timeout)
	if err != nil {
		return fmt.Errorf("refetch: %w", err)
	}
	if err := os.WriteFile(out, data, 0o644); err != nil {
		return err
	}
	actual, cerr := ccharvest.ClassifyOOXML(data)
	if cerr != nil {
		actual = "unclassifiable: " + cerr.Error()
	}
	fmt.Printf("refetched %s (%d bytes, %s) from %s\n  -> %s\n",
		digest, len(data), actual, found.URL, out)
	return nil
}

type orchConfig struct {
	manifests  []string
	ledger     string
	quarantine string
	scratchDir string
	batch      int
	workers    int
	timeout    time.Duration
	dohURL     string
}

// batchStats accumulates the outcome counters for one batch invocation.
type batchStats struct {
	processed     int
	pass          int
	failByStage   map[string]int
	resource      map[string]int
	deferred      int // transient fetch failures, left for a later run
	noDohSkips    int
	manifestDupes int
}

func runOrchestrator(cfg orchConfig) error {
	if len(cfg.manifests) == 0 {
		cfg.manifests = []string{"testdata/cc"}
	}
	files, err := resolveManifests(cfg.manifests)
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return errors.New("no manifest files found")
	}
	refs, dupes, err := loadAllRefs(files)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(cfg.scratchDir, 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(cfg.ledger), 0o755); err != nil {
		return err
	}

	ledger, err := openLedger(cfg.ledger)
	if err != nil {
		return err
	}
	defer func() { _ = ledger.Close() }()
	quarantine, err := openQuarantine(cfg.quarantine)
	if err != nil {
		return err
	}
	defer func() { _ = quarantine.Close() }()

	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locating worker binary: %w", err)
	}

	stats := &batchStats{failByStage: map[string]int{}, resource: map[string]int{}, manifestDupes: dupes}

	// Select up to cfg.batch not-yet-processed references. Live rows with no
	// DoH resolver are recorded as skips (they still consume batch budget so a
	// batch stays bounded and resumable).
	var work []Ref
	var mu sync.Mutex
	now := time.Now
	selected := 0
	for _, r := range refs {
		if selected >= cfg.batch {
			break
		}
		if ledger.Has(r.Digest) {
			continue
		}
		selected++
		if r.Kind == kindLive && cfg.dohURL == "" {
			_ = ledger.Append(r.Digest, outcomeSkip, "no-doh", "", now())
			stats.noDohSkips++
			continue
		}
		work = append(work, r)
	}

	total := len(refs)
	remaining := 0
	for _, r := range refs {
		if !ledger.Has(r.Digest) {
			remaining++
		}
	}
	fmt.Printf("ccrun: %d references (%d manifest dupes dropped); %d already in ledger, %d remaining\n",
		total, dupes, total-remaining, remaining)
	fmt.Printf("ccrun: processing a batch of %d (%d live rows skipped: no DoH), %d workers, %s timeout\n",
		selected, stats.noDohSkips, cfg.workers, cfg.timeout)

	jobs := make(chan Ref)
	var wg sync.WaitGroup
	for i := 0; i < cfg.workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for r := range jobs {
				outcome, stage, sig := processRef(self, r, cfg)
				mu.Lock()
				switch outcome {
				case outcomeRetry:
					// Transient fetch failure: leave it out of the ledger so a
					// later run retries it. Not a processed reference.
					stats.deferred++
					mu.Unlock()
					continue
				case outcomePass:
					stats.pass++
				case outcomeFail:
					stats.failByStage[stage]++
					_ = quarantine.Append(r, stage, sig)
				case outcomeResource:
					stats.resource[stage]++
					_ = quarantine.Append(r, "resource:"+stage, sig)
				}
				_ = ledger.Append(r.Digest, outcome, stage, sig, now())
				stats.processed++
				mu.Unlock()
			}
		}()
	}
	for _, r := range work {
		jobs <- r
	}
	close(jobs)
	wg.Wait()

	printStats(stats, selected)
	return nil
}

// processRef spawns a worker subprocess for one reference and interprets its
// result. The scratch file is deleted regardless of how the worker exited.
func processRef(self string, r Ref, cfg orchConfig) (outcome, stage, sig string) {
	scratch := filepath.Join(cfg.scratchDir, r.Digest+"."+r.Type)
	defer func() { _ = os.Remove(scratch) }()

	args := []string{
		"-worker",
		"-scratch", scratch,
		"-timeout", cfg.timeout.String(),
		"-doh-url", cfg.dohURL,
	}
	// Hard cap allows the worker to bail cleanly on a slow fetch before the
	// orchestrator kills it; a genuine processing hang is killed here.
	stdout, runErr, timedOut := spawnWorker(context.Background(), self, args, r, cfg.timeout+10*time.Second)
	return interpretWorker(timedOut, runErr, stdout, r.Digest)
}

// spawnWorker runs one worker command with a hard timeout, feeding the Ref as
// JSON on stdin, and returns its stdout, the run error, and whether the hard
// timeout fired. exe/args are separated so tests can inject a fake worker.
func spawnWorker(parent context.Context, exe string, args []string, ref Ref, hard time.Duration) (stdout string, runErr error, timedOut bool) {
	ctx, cancel := context.WithTimeout(parent, hard)
	defer cancel()

	payload, _ := json.Marshal(ref)
	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.Stdin = bytes.NewReader(payload)
	var outBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = io.Discard
	err := cmd.Run()
	return outBuf.String(), err, ctx.Err() == context.DeadlineExceeded
}

// interpretWorker maps a worker's (timedOut, runErr, stdout) into a ledger
// outcome. A clean exit with a valid result line is a genuine pass/fail; a
// timeout, a signal-kill (OOM), a nonzero exit, or a silent exit are all
// resource kills attributed to the file, not the batch.
func interpretWorker(timedOut bool, runErr error, stdout, digest string) (outcome, stage, sig string) {
	if timedOut {
		return outcomeResource, "timeout", "worker exceeded per-file timeout"
	}
	line, ok := lastResultLine(stdout, digest)
	if runErr == nil {
		if !ok {
			return outcomeResource, "panic", "worker exited 0 without a result line"
		}
		return parseResultLine(line)
	}
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		if exitErr.ExitCode() == -1 { // terminated by a signal (e.g. OOM SIGKILL)
			return outcomeResource, "killed", "worker terminated by signal"
		}
		return outcomeResource, "panic", fmt.Sprintf("worker exited with status %d", exitErr.ExitCode())
	}
	return outcomeResource, "panic", "worker did not run: " + runErr.Error()
}

// lastResultLine returns the last stdout line that begins with the digest and a
// tab — the worker's single result line.
func lastResultLine(stdout, digest string) (string, bool) {
	lines := bytes.Split([]byte(stdout), []byte("\n"))
	prefix := digest + "\t"
	for i := len(lines) - 1; i >= 0; i-- {
		s := string(lines[i])
		if len(s) >= len(prefix) && s[:len(prefix)] == prefix {
			return s, true
		}
	}
	return "", false
}

// parseResultLine parses a worker result line into an outcome.
func parseResultLine(line string) (outcome, stage, sig string) {
	fields := splitTab(line)
	if len(fields) < 2 {
		return outcomeResource, "panic", "malformed result line"
	}
	switch fields[1] {
	case outcomePass:
		return outcomePass, "", ""
	case outcomeRetry:
		st := ""
		if len(fields) >= 3 {
			st = fields[2]
		}
		return outcomeRetry, st, ""
	case outcomeFail:
		st, s := "", ""
		if len(fields) >= 3 {
			st = fields[2]
		}
		if len(fields) >= 4 {
			s = fields[3]
		}
		return outcomeFail, st, s
	default:
		return outcomeResource, "panic", "unknown outcome " + fields[1]
	}
}

func splitTab(s string) []string {
	var out []string
	start := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\t' {
			out = append(out, s[start:i])
			start = i + 1
		}
	}
	out = append(out, s[start:])
	return out
}

func printStats(s *batchStats, selected int) {
	fmt.Println("\nccrun batch complete:")
	fmt.Printf("  selected        %d\n", selected)
	fmt.Printf("  processed       %d\n", s.processed)
	fmt.Printf("  pass            %d\n", s.pass)
	fmt.Printf("  deferred        %d (transient fetch; retried next run)\n", s.deferred)
	fmt.Printf("  no-DoH skips    %d\n", s.noDohSkips)
	fmt.Printf("  manifest dupes  %d\n", s.manifestDupes)

	failTotal := 0
	for _, n := range s.failByStage {
		failTotal += n
	}
	fmt.Printf("  fail            %d\n", failTotal)
	for _, stage := range sortedKeys(s.failByStage) {
		fmt.Printf("      %-10s %d\n", stage, s.failByStage[stage])
	}
	resTotal := 0
	for _, n := range s.resource {
		resTotal += n
	}
	fmt.Printf("  resource-kills  %d\n", resTotal)
	for _, stage := range sortedKeys(s.resource) {
		fmt.Printf("      %-10s %d\n", stage, s.resource[stage])
	}
}

func sortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
