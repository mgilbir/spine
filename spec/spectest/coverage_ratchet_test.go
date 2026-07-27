package spectest

import (
	"reflect"
	"strings"
	"testing"
)

// TestCoverageRatchetBites is the C445 guard on the guard: coverage moving the
// wrong way must produce a failure, and coverage moving the right way must not.
// Without this the "No Go type mapped" skip silently absorbs a deleted typemap
// entry or a renamed root element.
func TestCoverageRatchetBites(t *testing.T) {
	base := baselineRow{mapped: 67, unmapped: 137, outOfScope: 2}

	tests := []struct {
		name         string
		got          coverage
		haveBaseline bool
		wantProblem  string // substring; empty means "must not fail"
		wantImproved bool
	}{
		{
			name:         "unchanged",
			got:          coverage{mapped: 67, unmapped: 137, outOfScope: 2},
			haveBaseline: true,
		},
		{
			// The exact shape of a deleted typemap entry: one example moves
			// from asserted to skipped.
			name:         "typemap entry deleted",
			got:          coverage{mapped: 66, unmapped: 138, outOfScope: 2},
			haveBaseline: true,
			wantProblem:  "now asserts 66 examples, baseline is 67",
		},
		{
			// An examples-JSON regeneration that renames root elements: the
			// totals hold but assertions become skips.
			name:         "regeneration turns assertions into skips",
			got:          coverage{mapped: 40, unmapped: 164, outOfScope: 2},
			haveBaseline: true,
			wantProblem:  "now skips 164 examples as unmapped",
		},
		{
			name:         "failure parked in the out-of-scope map",
			got:          coverage{mapped: 66, unmapped: 137, outOfScope: 3},
			haveBaseline: true,
			wantProblem:  "now skips 3 examples as out-of-scope",
		},
		{
			// The PR #225 shape: more modeled types, fewer unmapped skips.
			// Improvement must pass without the baseline being touched first.
			name:         "more types modeled",
			got:          coverage{mapped: 90, unmapped: 114, outOfScope: 2},
			haveBaseline: true,
			wantImproved: true,
		},
		{
			name:        "new suite with no baseline row",
			got:         coverage{mapped: 5, unmapped: 5},
			wantProblem: "no baseline row for suite",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			problems, improved := coverageProblems("TestSuite", tc.got, base, tc.haveBaseline)
			joined := strings.Join(problems, "\n")
			switch {
			case tc.wantProblem == "" && len(problems) > 0:
				t.Fatalf("coverage flagged a non-regression:\n%s", joined)
			case tc.wantProblem != "" && len(problems) == 0:
				t.Fatalf("coverage regression went unreported (got %+v, baseline %+v)", tc.got, base)
			case tc.wantProblem != "" && !strings.Contains(joined, tc.wantProblem):
				t.Errorf("problem %q does not mention %q", joined, tc.wantProblem)
			}
			if tc.wantImproved && improved == "" {
				t.Error("improved coverage produced no ratchet-tightening note")
			}
			if !tc.wantImproved && improved != "" {
				t.Errorf("unexpected improvement note: %s", improved)
			}
		})
	}
}

// TestCoverageBaselineParses keeps the committed file honest: a malformed row
// would otherwise only surface as a t.Fatal inside an unrelated suite.
func TestCoverageBaselineParses(t *testing.T) {
	rows, err := parseBaseline(coverageBaselineTSV)
	if err != nil {
		t.Fatalf("committed baseline does not parse: %v", err)
	}
	if len(rows) == 0 {
		t.Fatal("committed baseline has no rows")
	}
	for name, r := range rows {
		if !strings.HasSuffix(name, "_Unmarshal") && !strings.HasSuffix(name, "_RoundTrip") {
			t.Errorf("baseline row %q is not an Unmarshal/RoundTrip suite", name)
		}
		if r.mapped < 0 || r.unmapped < 0 || r.outOfScope < 0 {
			t.Errorf("baseline row %q has a negative count: %+v", name, r)
		}
	}
}

// TestCoverageClassifyMatchesHarness pins that the counting uses the same
// precedence as the subtests (out-of-scope wins over a mapped type), so the
// baseline describes what actually ran.
func TestCoverageClassifyMatchesHarness(t *testing.T) {
	typeMap := map[string]reflect.Type{"p": reflect.TypeOf(Example{})}
	outOfScope := map[string]string{"p": "", "q": "reason"}

	var c coverage
	c.classify("p", typeMap, outOfScope) // out of scope wins, even with an empty reason
	c.classify("q", typeMap, outOfScope)
	c.classify("r", typeMap, nil) // no mapping
	c.classify("p", typeMap, nil) // mapped

	want := coverage{mapped: 1, unmapped: 1, outOfScope: 2}
	if c != want {
		t.Errorf("classify = %+v, want %+v", c, want)
	}
}
