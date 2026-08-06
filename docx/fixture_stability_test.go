package docx

import (
	"bytes"
	"testing"
	"time"

	"github.com/mgilbir/spine/internal/fuzzseed"
)

// Every fuzz fixture must be byte-identical however much wall-clock time
// separates two builds of it. See the xlsx test of the same name for why this
// crosses a real second rather than using a synctest bubble, and
// fuzzseed.FixtureModified for what the fixtures pin.
func TestDocxFuzzFixturesAreByteStableAcrossASecond(t *testing.T) {
	if testing.Short() {
		t.Skip("sleeps for a second to cross a wall-clock boundary")
	}
	build := func() map[string][]byte {
		return map[string][]byte{
			"buildValidDocxFuzzSeed": buildValidDocxFuzzSeed(t),
			"buildRichDocxFuzzSeed":  buildRichDocxFuzzSeed(t),
		}
	}
	before := build()
	time.Sleep(1100 * time.Millisecond)
	after := build()

	for name, first := range before {
		second := after[name]
		if bytes.Equal(first, second) {
			continue
		}
		t.Errorf("%s changed across a second boundary (%d bytes then %d): "+
			"a crasher found against it could not be reproduced",
			name, len(first), len(second))
		for _, part := range []string{
			"docProps/core.xml", "docProps/app.xml", "word/document.xml",
			"word/styles.xml", "word/numbering.xml", "word/settings.xml",
		} {
			if !bytes.Equal(fuzzseed.ZipEntry(first, part), fuzzseed.ZipEntry(second, part)) {
				t.Errorf("  %s: %s differs", name, part)
			}
		}
	}
}
