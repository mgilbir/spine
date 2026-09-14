package pptx

import (
	"fmt"
	"testing"
)

// TestRelIDNumMatchesSscanf pins the replacement against the thing it replaced.
// relIDNum stands in for fmt.Sscanf(id, "rId%d", &n) inside the relationship-id
// allocators, and a disagreement there hands out an id another relationship
// already holds — the shape of C363.
func TestRelIDNumMatchesSscanf(t *testing.T) {
	inputs := []string{
		"rId1", "rId12", "rId0", "rId00", "rId007",
		"rId", "", "rid1", "RID1", "Rid1",
		"rId-5", "rId+5", "rIdx", "rId 1", "rId1 ",
		"rId12abc", "rId1.5", "xrId1", "rrId1",
		"rId2147483647",
	}
	for _, in := range inputs {
		var want int
		_, err := fmt.Sscanf(in, "rId%d", &want)
		wantOK := err == nil

		got, gotOK := relIDNum(in)

		// The guard against absurd ids is a deliberate divergence; skip inputs
		// that reach it, and assert separately that no realistic id does.
		if wantOK && want > 1<<30 {
			continue
		}
		if gotOK != wantOK {
			t.Errorf("relIDNum(%q) ok = %v, Sscanf ok = %v", in, gotOK, wantOK)
			continue
		}
		if wantOK && got != want {
			t.Errorf("relIDNum(%q) = %d, Sscanf = %d", in, got, want)
		}
	}

	// Every id the allocators themselves produce must round-trip.
	for _, n := range []int{1, 2, 9, 10, 99, 1000, 65535, 1 << 20} {
		id := fmt.Sprintf("rId%d", n)
		got, ok := relIDNum(id)
		if !ok || got != n {
			t.Errorf("relIDNum(%q) = %d, %v; want %d, true", id, got, ok, n)
		}
	}
}
