// Package cctest runs the library against a locally fetched Common Crawl
// corpus of real-world OOXML files (see testdata/cc/README.md). The corpus is
// gitignored; when it is absent the test skips, mirroring the external-fixture
// philosophy. Failures on wild files are expected: known ones are cataloged in
// testdata/cc/known_failures.tsv and skip-counted, new ones fail the test.
package cctest

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/mgilbir/spine/common/validate"
	"github.com/mgilbir/spine/docx"
	"github.com/mgilbir/spine/internal/testutil"
	"github.com/mgilbir/spine/pptx"
	"github.com/mgilbir/spine/xlsx"
)

// maxParallel bounds concurrently processed corpus files to keep peak memory
// reasonable (some spreadsheets decompress large). SPINE_CC_PARALLEL
// overrides it: race-detector runs need a lower bound, since shadow memory
// scales with the heap touched concurrently (`make test-race` sets 2).
var maxParallel = func() int {
	if n, err := strconv.Atoi(os.Getenv("SPINE_CC_PARALLEL")); err == nil && n > 0 {
		return n
	}
	return 4
}()

// subsetPerType is how many files per document type the default (fast) mode
// checks. The full corpus needs ~15-20 minutes — past Go's default 10m
// package timeout — so a plain `go test ./...` runs this deterministic
// subset instead (the first N per type in sha16 order, still catching gross
// regressions); set SPINE_CC_FULL=1 (or use `make test-corpus`) for the
// complete corpus. SPINE_CC_SUBSET overrides the size: race-detector runs
// are several-fold slower per file, so `make test-race` uses a smaller
// subset — data races surface from path coverage, not corpus volume.
var subsetPerType = func() int {
	if n, err := strconv.Atoi(os.Getenv("SPINE_CC_SUBSET")); err == nil && n > 0 {
		return n
	}
	return 60
}()

var docTypes = []string{"pptx", "xlsx", "docx"}

// saver is the save surface shared by the three document types.
type saver interface {
	SaveBytes() ([]byte, error)
}

// validator is the pre-save validation surface shared by the three document
// types (Wave F).
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

// corpusDir returns the corpus root, honoring the SPINE_CC_CORPUS override.
func corpusDir() string {
	if d := os.Getenv("SPINE_CC_CORPUS"); d != "" {
		return d
	}
	return filepath.Join("..", "testdata", "corpus", "cc")
}

// quarantineEntry is one row of testdata/cc/known_failures.tsv.
type quarantineEntry struct {
	typ, note string
}

// stageWontfix marks a quarantine row as permanent-by-design: the file cannot
// round-trip byte-identically for a reason outside the library's control
// (corrupt source zip, XML the decoder normalizes before the model sees it).
// It matches a failure at any stage and is skip-counted like a quarantined
// row, but reported separately in the stats block.
const stageWontfix = "wontfix"

// loadQuarantine reads known_failures.tsv into sha16 -> stage -> entry.
// A missing file is an empty quarantine.
func loadQuarantine(t *testing.T) map[string]map[string]quarantineEntry {
	t.Helper()
	q := make(map[string]map[string]quarantineEntry)
	data, err := os.ReadFile(filepath.Join("..", "testdata", "cc", "known_failures.tsv"))
	if err != nil {
		if os.IsNotExist(err) {
			return q
		}
		t.Fatalf("reading quarantine: %v", err)
	}
	for i, line := range strings.Split(string(data), "\n") {
		if i == 0 || line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.SplitN(line, "\t", 4)
		if len(f) != 4 {
			t.Fatalf("known_failures.tsv line %d: want 4 tab-separated fields", i+1)
		}
		sha16, typ, stage, note := f[0], f[1], f[2], f[3]
		if q[sha16] == nil {
			q[sha16] = make(map[string]quarantineEntry)
		}
		q[sha16][stage] = quarantineEntry{typ: typ, note: note}
	}
	return q
}

// loadURLs maps sha16 -> source URL from the fetcher journal, so failure
// reports can cite where a file came from.
func loadURLs(dir string) map[string]string {
	urls := make(map[string]string)
	data, err := os.ReadFile(filepath.Join(dir, "fetched.tsv"))
	if err != nil {
		return urls
	}
	for i, line := range strings.Split(string(data), "\n") {
		if i == 0 || line == "" {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) >= 3 && len(f[0]) >= 16 {
			urls[f[0][:16]] = f[2]
		}
	}
	return urls
}

// typeStats aggregates outcomes for one document type. pass means the file
// survived all four stages, i.e. its zero-modification save is part-for-part
// byte-identical to the original.
type typeStats struct {
	total, pass, quarantined, wontfix, newFail int
	// validateWarn counts files that emitted at least one warning-severity
	// validation finding (never a failure); validateWarnFindings is the total
	// warning count across all files. Error-severity findings are not counted
	// here — they surface as a save-stage failure.
	validateWarn, validateWarnFindings int
}

type aggregate struct {
	mu    sync.Mutex
	stats map[string]*typeStats
	// rows collects quarantine lines when SPINE_CC_UPDATE_QUARANTINE is set.
	rows []string
}

func (a *aggregate) add(typ string, f func(*typeStats)) {
	a.mu.Lock()
	defer a.mu.Unlock()
	f(a.stats[typ])
}

func (a *aggregate) record(row string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.rows = append(a.rows, row)
}

// TestCCCorpus opens, saves, reopens, and part-compares corpus files. By
// default it checks a fast deterministic subset (subsetPerType files per
// type); SPINE_CC_FULL=1 runs the complete corpus, and
// SPINE_CC_UPDATE_QUARANTINE=1 additionally ignores the committed quarantine
// and rewrites testdata/cc/known_failures.tsv from the observed failures.
func TestCCCorpus(t *testing.T) {
	dir := corpusDir()
	if _, err := os.Stat(dir); err != nil {
		t.Skipf("Common Crawl corpus not present at %s (run 'make fetch-cc'); skipping", dir)
	}

	update := os.Getenv("SPINE_CC_UPDATE_QUARANTINE") != ""
	full := update || os.Getenv("SPINE_CC_FULL") != ""

	quarantine := loadQuarantine(t)
	// origQuarantine keeps the full committed set so update-mode regeneration
	// can carry forward rows for files absent from THIS machine's corpus (see
	// the cleanup below). The corpus is machine-dependent, so a row we could not
	// have re-derived must not be silently dropped.
	origQuarantine := quarantine
	// presentSHAs records every sha16 present in this machine's local corpus, so
	// carry-forward can tell an absent-file row (keep verbatim) from one this run
	// re-judged (which may now pass, fail elsewhere, or stay).
	presentSHAs := make(map[string]bool)
	if update {
		// Regeneration judges every file afresh; the old quarantine must not
		// mask entries that would now pass (or fail at a different stage).
		// Curated wontfix rows survive: their reason is hand-written and the
		// regeneration would otherwise demote them to plain failure rows. A
		// wontfix file that now passes emits no row and drops out anyway.
		kept := make(map[string]map[string]quarantineEntry)
		for sha16, stages := range quarantine {
			if e, ok := stages[stageWontfix]; ok {
				kept[sha16] = map[string]quarantineEntry{stageWontfix: e}
			}
		}
		quarantine = kept
	}
	urls := loadURLs(dir)
	agg := &aggregate{stats: make(map[string]*typeStats)}
	for _, typ := range docTypes {
		agg.stats[typ] = &typeStats{}
	}
	t.Cleanup(func() {
		t.Log("CC corpus stats (total / pass / quarantined / wontfix / new-fail):")
		for _, typ := range docTypes {
			s := agg.stats[typ]
			t.Logf("  %-5s %5d / %5d / %5d / %5d / %5d",
				typ, s.total, s.pass, s.quarantined, s.wontfix, s.newFail)
		}
		t.Log("Validate() warnings (files-with-warnings / total-warning-findings; zero error-severity findings):")
		for _, typ := range docTypes {
			s := agg.stats[typ]
			t.Logf("  %-5s %5d / %5d", typ, s.validateWarn, s.validateWarnFindings)
		}
		if update {
			rows := mergeCarryForward(agg.rows, origQuarantine, presentSHAs)
			carried := len(rows) - len(agg.rows)
			if err := writeQuarantine(rows); err != nil {
				t.Errorf("writing refreshed quarantine: %v", err)
			} else {
				t.Logf("wrote %d rows to testdata/cc/known_failures.tsv (%d re-derived, %d carried forward for files absent locally)",
					len(rows), len(agg.rows), carried)
			}
		}
	})

	sem := make(chan struct{}, maxParallel)
	for _, typ := range docTypes {
		files, err := filepath.Glob(filepath.Join(dir, typ, "*."+typ))
		if err != nil {
			t.Fatal(err)
		}
		sort.Strings(files)
		// Record every locally present sha16 before any subset truncation, so
		// update-mode carry-forward sees the full local corpus.
		for _, path := range files {
			presentSHAs[strings.TrimSuffix(filepath.Base(path), "."+typ)] = true
		}
		if !full && len(files) > subsetPerType {
			files = files[:subsetPerType]
		}
		t.Run(typ, func(t *testing.T) {
			for _, path := range files {
				sha16 := strings.TrimSuffix(filepath.Base(path), "."+typ)
				t.Run(sha16, func(t *testing.T) {
					t.Parallel()
					sem <- struct{}{}
					defer func() { <-sem }()
					agg.add(typ, func(s *typeStats) { s.total++ })
					checkFile(t, agg, typ, sha16, path, update, quarantine[sha16], urls[sha16])
				})
			}
		})
	}
}

// mergeCarryForward appends to the freshly observed rows every row of the
// previously committed quarantine whose sha16 is absent from the local corpus
// (present). Update-mode regeneration only re-derives rows for files that exist
// on THIS machine, but the corpus is machine-dependent: a row for a file this
// machine never fetched could not have been re-derived, so dropping it would
// silently delete a failure (or hand-written wontfix) recorded elsewhere. Rows
// for present files are left to the fresh run, which may legitimately drop a
// now-passing file. The fresh and carried sets are disjoint by construction
// (present vs. absent sha16), so no row is duplicated.
func mergeCarryForward(fresh []string, orig map[string]map[string]quarantineEntry, present map[string]bool) []string {
	out := fresh
	for sha16, stages := range orig {
		if present[sha16] {
			continue
		}
		for stage, e := range stages {
			out = append(out, fmt.Sprintf("%s\t%s\t%s\t%s", sha16, e.typ, stage, e.note))
		}
	}
	return out
}

// writeQuarantine rewrites testdata/cc/known_failures.tsv from the collected
// rows, sorted for a stable diff.
func writeQuarantine(rows []string) error {
	sort.Strings(rows)
	var b strings.Builder
	b.WriteString("sha16\ttype\tstage\tnote\n")
	for _, row := range rows {
		b.WriteString(row)
		b.WriteByte('\n')
	}
	return os.WriteFile(filepath.Join("..", "testdata", "cc", "known_failures.tsv"), []byte(b.String()), 0o644)
}

// checkFile runs the four-stage discipline over one corpus file.
func checkFile(t *testing.T, agg *aggregate, typ, sha16, path string, update bool, quarantined map[string]quarantineEntry, url string) {
	fail := func(stage string, err error) {
		if e, ok := quarantined[stageWontfix]; ok {
			// Permanent-by-design: matches a failure at any stage.
			agg.add(typ, func(s *typeStats) { s.wontfix++ })
			if update {
				agg.record(fmt.Sprintf("%s\t%s\t%s\t%s", sha16, typ, stageWontfix, e.note))
			}
			t.Skipf("wontfix (%s): %v", e.note, err)
		}
		if _, ok := quarantined[stage]; ok {
			agg.add(typ, func(s *typeStats) { s.quarantined++ })
			t.Skipf("quarantined at stage %s: %v", stage, err)
		}
		if update {
			// Regeneration run: the failure becomes a fresh quarantine row
			// instead of failing the test.
			agg.add(typ, func(s *typeStats) { s.quarantined++ })
			agg.record(fmt.Sprintf("%s\t%s\t%s\t%s", sha16, typ, stage, signature(err)))
			t.Skipf("quarantining at stage %s: %v", stage, err)
		}
		agg.add(typ, func(s *typeStats) { s.newFail++ })
		if os.Getenv("SPINE_CC_EMIT_QUARANTINE") != "" {
			fmt.Printf("CCQUARANTINE\t%s\t%s\t%s\t%s\n", sha16, typ, stage, signature(err))
		}
		t.Errorf("new failure: sha16=%s type=%s stage=%s url=%s err=%v", sha16, typ, stage, url, err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("reading corpus file: %v", err)
	}

	// (a) Open succeeds.
	doc, err := openDoc(typ, data)
	if err != nil {
		fail("open", err)
		return
	}

	// (a2) Validate the freshly opened model. No error-severity finding may
	// appear on a file the corpus accepts (that would mean a check is wrong);
	// warnings are tallied for the stats block. SaveBytes re-runs Validate and
	// would reject an error-severity finding anyway, but checking here pins the
	// failure to the validate stage with the finding detail.
	if v, ok := doc.(validator); ok {
		report := v.Validate()
		if report.HasErrors() {
			fail("validate", report)
			return
		}
		if w := report.Warnings(); len(w) > 0 {
			agg.add(typ, func(s *typeStats) {
				s.validateWarn++
				s.validateWarnFindings += len(w)
			})
		}
	}

	// (b) Zero-modification save succeeds.
	saved, err := doc.SaveBytes()
	if err != nil {
		fail("save", err)
		return
	}

	// (c) The saved output reopens cleanly.
	if _, err := openDoc(typ, saved); err != nil {
		fail("reopen", err)
		return
	}

	// (d) Part-level fidelity: compare original and saved package part by
	// part. Wild originals can be damaged in ways the library tolerates but
	// a strict zip read does not (e.g. CRC mismatches), so a re-read
	// failure is a fidelity finding, not a test bug.
	origParts, err := testutil.ReadZipPartsBytes(data)
	if err != nil {
		fail("fidelity", fmt.Errorf("re-reading original parts: %w", err))
		return
	}
	savedParts, err := testutil.ReadZipPartsBytes(saved)
	if err != nil {
		fail("fidelity", fmt.Errorf("reading saved parts: %w", err))
		return
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
		fail("fidelity", fmt.Errorf("%d identical, %d changed, %d missing, %d extra parts (first: %s)",
			identical, len(changed), len(missing), len(extra), first))
		return
	}
	agg.add(typ, func(s *typeStats) { s.pass++ })
}

// signature normalizes an error into a stable grouping key: digits collapsed,
// whitespace flattened, truncated.
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
