package cctest

import (
	"sort"
	"strings"
	"testing"
)

// TestMergeCarryForwardPreservesAbsentRows guards C337: update-mode quarantine
// regeneration must not drop rows whose file is absent from the local corpus.
// The corpus is machine-dependent, so a row this machine could not re-derive
// (no local file) is carried forward verbatim; a row for a present file is left
// to the fresh run.
func TestMergeCarryForwardPreservesAbsentRows(t *testing.T) {
	orig := map[string]map[string]quarantineEntry{
		// Absent from the local corpus: must be carried forward.
		"absentsha0000001": {"fidelity": {typ: "docx", note: "N changed part"}},
		// Absent, hand-written wontfix: must survive too.
		"absentsha0000002": {stageWontfix: {typ: "xlsx", note: "corrupt source zip"}},
		// Present locally: the fresh run owns this sha16; do NOT carry it.
		"presentsha000001": {"save": {typ: "pptx", note: "old signature"}},
	}
	present := map[string]bool{"presentsha000001": true}

	fresh := []string{"presentsha000001\tpptx\tsave\tnew signature"}
	got := mergeCarryForward(fresh, orig, present)
	sort.Strings(got)

	want := []string{
		"absentsha0000001\tdocx\tfidelity\tN changed part",
		"absentsha0000002\txlsx\twontfix\tcorrupt source zip",
		"presentsha000001\tpptx\tsave\tnew signature",
	}
	sort.Strings(want)

	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("mergeCarryForward mismatch:\n got:\n  %s\nwant:\n  %s",
			strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}

	// The present-file row must appear exactly once (the fresh version), never
	// duplicated by a carried-forward copy of the stale row.
	for _, row := range got {
		if strings.HasPrefix(row, "presentsha000001\t") && strings.Contains(row, "old signature") {
			t.Errorf("stale present-file row was carried forward: %q", row)
		}
	}
}
