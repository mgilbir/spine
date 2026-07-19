package docx

import (
	"bytes"
	"os"
	"testing"

	"github.com/mgilbir/spine/common/dml"
)

func TestThemeReadDocx(t *testing.T) {
	doc, err := Open("testdata/chart.docx")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = doc.Close() }()

	theme := doc.Theme()
	if theme == nil {
		t.Fatal("Theme() = nil, want a theme handle for chart.docx")
	}
	cs := theme.ColorScheme()
	if cs == nil {
		t.Fatal("ColorScheme() = nil")
	}
	if got := cs.Accent1().RGB.String(); got == "" {
		t.Errorf("Accent1() RGB empty")
	}
	fs := theme.FontScheme()
	if fs == nil {
		t.Fatal("FontScheme() = nil")
	}
	if fs.MajorLatin() == "" {
		t.Errorf("MajorLatin() empty, want a typeface")
	}
}

// TestThemeUnmodifiedRoundTripDocx verifies the theme part is byte-identical
// when the theme is read but not modified.
func TestThemeUnmodifiedRoundTripDocx(t *testing.T) {
	orig, err := os.ReadFile("testdata/chart.docx")
	if err != nil {
		t.Fatal(err)
	}
	origTheme := zipPart(t, orig, "word/theme/theme1.xml")

	doc, err := Open("testdata/chart.docx")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = doc.Close() }()

	if theme := doc.Theme(); theme == nil {
		t.Fatal("Theme() = nil")
	} else if theme.Modified() {
		t.Fatal("Theme reports modified after read-only access")
	}

	out, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	savedTheme := zipPart(t, out, "word/theme/theme1.xml")
	if origTheme != savedTheme {
		t.Errorf("unmodified theme part not byte-identical:\n orig %d bytes\nsaved %d bytes", len(origTheme), len(savedTheme))
	}
}

// TestThemeModifiedRoundTripDocx verifies edits persist and the theme part
// re-serializes to valid, reparseable XML on save.
func TestThemeModifiedRoundTripDocx(t *testing.T) {
	doc, err := Open("testdata/chart.docx")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	theme := doc.Theme()
	if theme == nil {
		t.Fatal("Theme() = nil")
	}
	red, _ := dml.ParseRGB("FF0000")
	theme.ColorScheme().SetAccent1(red.ToColor())
	theme.FontScheme().SetMajorLatin("Comic Sans MS")
	if !theme.Modified() {
		t.Fatal("Theme not marked modified after setters")
	}

	out, err := doc.SaveBytes()
	_ = doc.Close()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	doc2, err := OpenReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer func() { _ = doc2.Close() }()
	theme2 := doc2.Theme()
	if theme2 == nil {
		t.Fatal("reopened Theme() = nil")
	}
	if got := theme2.ColorScheme().Accent1().RGB.String(); got != "FF0000" {
		t.Errorf("Accent1 after round-trip = %q, want FF0000", got)
	}
	if got := theme2.FontScheme().MajorLatin(); got != "Comic Sans MS" {
		t.Errorf("MajorLatin after round-trip = %q, want Comic Sans MS", got)
	}
}

// TestThemeCreatedIsNilDocx verifies a document created from scratch (no theme
// part) reports a nil theme, matching pptx's behavior.
func TestThemeCreatedIsNilDocx(t *testing.T) {
	doc := Create()
	if theme := doc.Theme(); theme != nil {
		t.Errorf("Theme() on created document = %v, want nil", theme)
	}
}
