package xlsx

import (
	"bytes"
	"testing"
	"time"

	"github.com/mgilbir/spine/internal/fuzzseed"
)

// The comments fuzz fixture must be byte-identical however much wall-clock time
// separates two builds of it.
//
// assertCommentsFixture already compares two builds, but back to back: they
// almost always land in the same second, so it agrees for the wrong reason and
// only reports the drift when a build happens to straddle a second boundary.
// That is what it did on 2026-08-06, failing the nightly's race job and xlsx
// fuzz job while every local run passed — a real defect surfacing as a flake.
//
// Crossing the boundary explicitly turns that coin flip into a decision. The
// sleep is what does the work: without it this test passes whether or not the
// fixture pins its timestamp.
//
// It is a second of real time, not a synctest bubble. synctest is how the
// modified-stamping tests and TestSaveBytesIsIdempotent pin the clock, and it
// would be the better tool — but synctest.Test takes a *testing.T, and the
// build this guards runs in fuzz-target setup holding a *testing.F. The
// fixture is pinned at the source instead, and this test is the check on that.
func TestCommentsFixtureIsByteStableAcrossASecond(t *testing.T) {
	if testing.Short() {
		t.Skip("sleeps for a second to cross a wall-clock boundary")
	}
	first := buildXlsxCommentsFixture(t)
	time.Sleep(1100 * time.Millisecond)
	second := buildXlsxCommentsFixture(t)

	if !bytes.Equal(first, second) {
		t.Errorf("the comments fixture changed across a second boundary (%d bytes then %d): "+
			"a crasher found against it could not be reproduced", len(first), len(second))
		for name, a := range fixtureParts(t, first) {
			if b := fixtureParts(t, second)[name]; !bytes.Equal(a, b) {
				t.Errorf("  %s differs:\n    first:  %.200s\n    second: %.200s", name, a, b)
			}
		}
	}
}

// fixtureParts returns the fixture entries this test compares, by name.
func fixtureParts(t *testing.T, pkg []byte) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	for _, name := range []string{
		"docProps/core.xml", partWorkbook, partSheet1, partComments,
		partThreadedComments, partPersons, partSharedStrings, partStyles,
	} {
		if data := fuzzseed.ZipEntry(pkg, name); data != nil {
			out[name] = data
		}
	}
	return out
}
