package spectest

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

// Spec-coverage ratchet (C445).
//
// An element with no entry in a suite's type map does not fail — it skips ("No
// Go type mapped"). That is the right behavior (a model is allowed to be
// narrower than the spec), but it makes two very different states look
// identical in a green run: "this part of the schema is deliberately out of
// model" and "somebody deleted a typemap entry, or an examples-JSON
// regeneration renamed a root element". Deleting a mapping converts real
// assertions into skips, silently.
//
// So every suite carries a committed baseline of its (mapped, unmapped,
// out-of-scope) example counts, and each run compares against it:
//
//   - mapped may only go UP, unmapped and out-of-scope may only go DOWN.
//     Improvement — modeling more types, as PR #225 does for xlsx — always
//     passes and never needs the baseline touched first.
//   - Coverage going the other way fails with the numbers, so shrinking the
//     model is a deliberate, reviewable baseline edit rather than a silent
//     green run.
//
// Regenerate with SPINE_SPEC_UPDATE_BASELINE=1 (see updateBaseline).

//go:embed coverage_baseline.tsv
var coverageBaselineTSV string

// coverage counts how each considered example was classified by a suite.
// mapped examples are really asserted; the other two skip.
type coverage struct {
	mapped     int // a Go type is registered for the example's root element
	unmapped   int // no Go type mapped: the coverage this ratchet watches
	outOfScope int // explicitly declared out of scope, with a written reason
}

func (c coverage) total() int { return c.mapped + c.unmapped + c.outOfScope }

// classify records one example against the suite's type map and out-of-scope
// map, using exactly the precedence the subtests apply.
func (c *coverage) classify(rootElem string, typeMap map[string]reflect.Type, outOfScope map[string]string) {
	if _, skip := outOfScope[rootElem]; skip {
		c.outOfScope++
		return
	}
	if _, mapped := typeMap[rootElem]; mapped {
		c.mapped++
		return
	}
	c.unmapped++
}

// baselineRow is one committed suite baseline.
type baselineRow struct {
	mapped, unmapped, outOfScope int
}

// parseBaseline reads the committed TSV into suite -> row.
func parseBaseline(data string) (map[string]baselineRow, error) {
	rows := make(map[string]baselineRow)
	for i, line := range strings.Split(data, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		f := strings.Split(line, "\t")
		if len(f) != 4 {
			return nil, fmt.Errorf("coverage_baseline.tsv line %d: want 4 tab-separated fields, got %d", i+1, len(f))
		}
		var nums [3]int
		for j := 0; j < 3; j++ {
			n, err := strconv.Atoi(strings.TrimSpace(f[j+1]))
			if err != nil {
				return nil, fmt.Errorf("coverage_baseline.tsv line %d: field %d: %w", i+1, j+2, err)
			}
			nums[j] = n
		}
		rows[f[0]] = baselineRow{mapped: nums[0], unmapped: nums[1], outOfScope: nums[2]}
	}
	return rows, nil
}

// checkCoverage compares a suite's observed coverage with its committed
// baseline. The suite key is the test's own name, so adding a suite requires
// adding a baseline row — a new suite cannot start life unmonitored.
func checkCoverage(t *testing.T, got coverage) {
	t.Helper()

	if os.Getenv("SPINE_SPEC_UPDATE_BASELINE") != "" {
		updateBaseline(t, got)
		return
	}

	rows, err := parseBaseline(coverageBaselineTSV)
	if err != nil {
		t.Fatalf("spec coverage baseline: %v", err)
	}
	name := t.Name()
	want, ok := rows[name]
	problems, improved := coverageProblems(name, got, want, ok)
	for _, p := range problems {
		t.Error(p)
	}
	if improved != "" {
		// Improvement is never a failure, but say so, so the ratchet gets
		// tightened on the PR that earns it rather than drifting slack.
		t.Log(improved)
	}
}

// coverageProblems compares one suite's observed coverage with its baseline and
// returns the regressions (each a complete message) plus, when coverage went the
// good way, a note suggesting the tightened row. It is pure so the ratchet's own
// behavior is testable without a spec suite.
func coverageProblems(name string, got coverage, want baselineRow, haveBaseline bool) (problems []string, improved string) {
	if !haveBaseline {
		return []string{fmt.Sprintf("spec coverage: no baseline row for suite %q. Every example suite must be "+
			"ratcheted, otherwise unmapped elements skip unnoticed. Add this row to "+
			"spec/spectest/coverage_baseline.tsv (or run with SPINE_SPEC_UPDATE_BASELINE=1):\n\t%s",
			name, baselineLine(name, got))}, ""
	}

	if got.mapped < want.mapped {
		problems = append(problems, fmt.Sprintf("spec coverage regressed: %s now asserts %d examples, "+
			"baseline is %d. A type mapping was removed or an element was renamed, converting real "+
			"assertions into \"No Go type mapped\" skips. Restore the mapping, or lower the baseline "+
			"deliberately in spec/spectest/coverage_baseline.tsv.", name, got.mapped, want.mapped))
	}
	if got.unmapped > want.unmapped {
		problems = append(problems, fmt.Sprintf("spec coverage regressed: %s now skips %d examples as "+
			"unmapped, baseline is %d (mapped %d, out-of-scope %d, total %d).",
			name, got.unmapped, want.unmapped, got.mapped, got.outOfScope, got.total()))
	}
	if got.outOfScope > want.outOfScope {
		problems = append(problems, fmt.Sprintf("spec coverage regressed: %s now skips %d examples as "+
			"out-of-scope, baseline is %d. Widening the out-of-scope map is a coverage decision; "+
			"record it in the baseline.", name, got.outOfScope, want.outOfScope))
	}

	if got.mapped > want.mapped || got.unmapped < want.unmapped || got.outOfScope < want.outOfScope {
		improved = fmt.Sprintf("spec coverage improved for %s: mapped %d (baseline %d), unmapped %d "+
			"(baseline %d), out-of-scope %d (baseline %d). Tighten "+
			"spec/spectest/coverage_baseline.tsv to lock it in:\n\t%s",
			name, got.mapped, want.mapped, got.unmapped, want.unmapped,
			got.outOfScope, want.outOfScope, baselineLine(name, got))
	}
	return problems, improved
}

func baselineLine(name string, c coverage) string {
	return fmt.Sprintf("%s\t%d\t%d\t%d", name, c.mapped, c.unmapped, c.outOfScope)
}

// updateBaseline appends the observed row to coverage_baseline.tsv.observed
// next to the committed baseline. Suites live in several packages, which
// `go test` runs as concurrent processes, so this appends (atomic for short
// writes) rather than rewriting the file; sort the result into
// coverage_baseline.tsv yourself:
//
//	SPINE_SPEC_UPDATE_BASELINE=1 go test ./... -run SpecExamples -count=1
//	sort spec/spectest/coverage_baseline.tsv.observed > /tmp/rows && ...
func updateBaseline(t *testing.T, got coverage) {
	t.Helper()
	_, self, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("spec coverage: cannot locate the spectest package directory")
	}
	path := filepath.Join(filepath.Dir(self), "coverage_baseline.tsv.observed")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatalf("spec coverage: %v", err)
	}
	defer func() { _ = f.Close() }()
	if _, err := fmt.Fprintln(f, baselineLine(t.Name(), got)); err != nil {
		t.Fatalf("spec coverage: %v", err)
	}
	t.Logf("spec coverage: recorded %s in %s", baselineLine(t.Name(), got), path)
}
