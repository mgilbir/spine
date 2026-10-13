package pptx

import (
	"bytes"
	"maps"
	"slices"
	"testing"

	"github.com/mgilbir/spine/internal/testutil"
)

// TestWindows1252PackageRoundTrip covers a deck whose parts declare a
// single-byte code page. The text reads correctly; an untouched save keeps
// every part's bytes (presentation.xml, layouts and masters are rebuilt on
// every save and must still prefer the source when nothing changed); and an
// edited save, a duplicated slide and a merged copy must read back the same
// text. They did not: a rebuilt part replayed the source's declaration
// (encoding="windows-1252") over UTF-8 bytes, so text read back as mojibake,
// and a duplicate parsed its own rebuilt copy with a decoder that rejects the
// declaration and came out empty.
func TestWindows1252PackageRoundTrip(t *testing.T) {
	p := CreateWidescreen()
	layout, err := p.LayoutByType(LayoutTitleAndContent)
	if err != nil {
		t.Fatal(err)
	}
	layout.SetName("Titre et contenu – é")
	s := p.AddSlideFromLayout(layout)
	for _, ph := range s.Placeholders() {
		ph.SetText("Café ‘quoted’ €5")
	}
	p.Properties.Title = "Résumé"
	plain, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	src, nonASCII, err := testutil.WithWindows1252(plain)
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range []string{"ppt/slides/slide1.xml", "ppt/slideLayouts/slideLayout2.xml", "docProps/core.xml"} {
		if !slices.Contains(nonASCII, part) {
			t.Fatalf("fixture has no non-ASCII %s: %v", part, nonASCII)
		}
	}
	wantText := s.Text()
	if wantText == "" {
		t.Fatal("fixture slide has no text")
	}

	check := func(label string, data []byte, slides int) {
		t.Helper()
		deck, err := OpenReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			t.Fatalf("%s: %v", label, err)
		}
		defer func() { _ = deck.Close() }()
		if n := len(deck.Slides()); n != slides {
			t.Fatalf("%s: %d slides, want %d", label, n, slides)
		}
		for i, sl := range deck.Slides() {
			if got := sl.Text(); got != wantText {
				t.Errorf("%s: slide %d text %q, want %q", label, i+1, got, wantText)
			}
		}
		l, err := deck.LayoutByType(LayoutTitleAndContent)
		if err != nil || l.Name() != "Titre et contenu – é" {
			t.Errorf("%s: layout name %q (%v)", label, l.Name(), err)
		}
		if deck.Properties.Title != "Résumé" {
			t.Errorf("%s: title %q", label, deck.Properties.Title)
		}
	}
	check("source", src, 1)

	open := func() *Presentation {
		t.Helper()
		deck, err := OpenReader(bytes.NewReader(src), int64(len(src)))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = deck.Close() })
		return deck
	}
	// An untouched save keeps every part's bytes, also after reading the
	// slides, which makes the save rebuild them.
	for _, read := range []bool{false, true} {
		deck := open()
		if read {
			_ = deck.Text()
		}
		untouched, err := deck.SaveBytes()
		if err != nil {
			t.Fatal(err)
		}
		if want, got := zipParts(t, src), zipParts(t, untouched); !maps.EqualFunc(want, got, bytes.Equal) {
			for name := range want {
				if !bytes.Equal(want[name], got[name]) {
					t.Errorf("untouched save (read=%v) changed %s", read, name)
				}
			}
		}
	}

	deck := open()
	deck.Slides()[0].Duplicate()
	deck.Properties.Subject = "Ü"
	edited, err := deck.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	check("duplicated", edited, 2)

	merged := open()
	if err = merged.AppendSlidesFrom(open()); err != nil {
		t.Fatal(err)
	}
	out, err := merged.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	check("merged", out, 2)
}
