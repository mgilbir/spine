package xlsx

import (
	"bytes"
	"testing"
	"time"

	"github.com/mgilbir/spine/internal/fuzzseed"
)

// Every fuzz fixture must be byte-identical however much wall-clock time
// separates two builds of it.
//
// A fixture that moves cannot be reproduced from a crasher: the corpus entries
// accumulated against it describe a package that no longer exists, and a
// reproducer replays a mutation of different bytes. Only one fixture ever
// checked itself — assertCommentsFixture — and it compares two back-to-back
// builds, which land in the same second and so agree for the wrong reason. On
// 2026-08-06 one pair straddled a second boundary and failed the nightly's race
// job and xlsx fuzz job at once, while every local run passed. Five more
// fixtures across the three format packages had the same defect and no check at
// all.
//
// Crossing the boundary explicitly turns that coin flip into a decision. The
// sleep is what does the work: without it this passes whether or not the
// fixtures pin their timestamp. One sleep covers every fixture in the package.
//
// It is real time rather than a synctest bubble because synctest.Test takes a
// *testing.T and fixtures are built in fuzz-target setup, which holds a
// *testing.F. The fixtures are pinned at the source instead (see
// fuzzseed.FixtureModified); this is the check on that.
func TestXlsxFuzzFixturesAreByteStableAcrossASecond(t *testing.T) {
	if testing.Short() {
		t.Skip("sleeps for a second to cross a wall-clock boundary")
	}
	build := func() map[string][]byte {
		return map[string][]byte{
			"buildXlsxCommentsFixture": buildXlsxCommentsFixture(t),
			"buildXlsxPivotFixture":    buildXlsxPivotFixture(t),
			"buildXlsxPartsFixture":    buildXlsxPartsFixture(),
		}
	}
	before := build()
	time.Sleep(1100 * time.Millisecond)
	assertFixturesStable(t, before, build())
}

// assertFixturesStable reports which fixtures changed across the boundary, and
// which part of each moved. Naming the part is what turns "the fixture moved"
// into a diagnosis: docProps/core.xml means an unpinned modified stamp, and
// anything else means a second source of nondeterminism worth its own look.
func assertFixturesStable(t *testing.T, before, after map[string][]byte) {
	t.Helper()
	for name, first := range before {
		second := after[name]
		if bytes.Equal(first, second) {
			continue
		}
		t.Errorf("%s changed across a second boundary (%d bytes then %d): "+
			"a crasher found against it could not be reproduced",
			name, len(first), len(second))
		for _, part := range fixtureParts(t, first, second) {
			t.Errorf("  %s: %s differs", name, part)
		}
	}
}

// fixtureParts returns the entry names whose bytes differ between two packages.
func fixtureParts(t *testing.T, a, b []byte) []string {
	t.Helper()
	var out []string
	for _, name := range []string{
		"docProps/core.xml", "docProps/app.xml", partWorkbook, partSheet1,
		partComments, partThreadedComments, partPersons, partSharedStrings,
		partStyles, partTheme, partTable, partPivotCache, partPivotTable,
	} {
		ea, eb := fuzzseed.ZipEntry(a, name), fuzzseed.ZipEntry(b, name)
		if !bytes.Equal(ea, eb) {
			out = append(out, name)
		}
	}
	return out
}
