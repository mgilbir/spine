package pptx

import (
	"sort"
	"strings"
	"testing"

	coxml "github.com/mgilbir/spine/common/oxml"
)

// TestKnownPartNamesListsSessionPartsInOrder pins the order of the part list
// the validation report is built from.
//
// otherParts and themeData hold the parts a session adds — embedded media,
// merged masters, notes — which are not in reader.Files and so are not already
// deduplicated by the reader pass. Ranging those maps put them into the list,
// and therefore into the Report, in Go's randomized order: a deck with two
// embedded images produced a differently ordered Report on each call (C497).
func TestKnownPartNamesListsSessionPartsInOrder(t *testing.T) {
	added := []string{
		"/ppt/media/image3.png",
		"/ppt/media/image1.png",
		"/ppt/media/image2.png",
		"/ppt/media/audio1.m4a",
		"/ppt/notesSlides/notesSlide1.xml",
	}
	themes := []string{
		"/ppt/theme/theme3.xml",
		"/ppt/theme/theme1.xml",
		"/ppt/theme/theme2.xml",
	}

	// Repeated because a single run of the old code had a fair chance of
	// yielding a sorted list by luck.
	for run := 0; run < 16; run++ {
		p := &Presentation{
			otherParts: map[string]*coxml.RawPart{},
			themeData:  map[string][]byte{},
		}
		for _, name := range added {
			p.otherParts[name] = &coxml.RawPart{Data: []byte("x")}
		}
		for _, name := range themes {
			p.themeData[name] = []byte("x")
		}

		got := p.knownPartNames()
		media := filterPrefix(got, "/ppt/media/")
		media = append(media, filterPrefix(got, "/ppt/notesSlides/")...)
		assertSorted(t, run, "otherParts", filterPrefix(got, "/ppt/media/"))
		assertSorted(t, run, "themeData", filterPrefix(got, "/ppt/theme/"))
		if len(media) != 5 {
			t.Fatalf("run %d: expected all five added parts in the list, got %v", run, got)
		}
	}
}

func filterPrefix(names []string, prefix string) []string {
	var out []string
	for _, n := range names {
		if strings.HasPrefix(n, prefix) {
			out = append(out, n)
		}
	}
	return out
}

func assertSorted(t *testing.T, run int, what string, got []string) {
	t.Helper()
	want := append([]string(nil), got...)
	sort.Strings(want)
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("run %d: %s names are not in part-name order:\n got %v\nwant %v",
				run, what, got, want)
		}
	}
}
