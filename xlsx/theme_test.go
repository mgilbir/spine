package xlsx

import (
	"bytes"
	"os"
	"testing"

	"github.com/mgilbir/spine/common/dml"
)

const themeFixture = "testdata/external/excelize_test.xlsx"

func TestThemeReadXlsx(t *testing.T) {
	if _, err := os.Stat(themeFixture); err != nil {
		t.Skip("external fixture absent")
	}
	wb, err := Open(themeFixture)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = wb.Close() }()

	theme := wb.Theme()
	if theme == nil {
		t.Fatal("Theme() = nil, want a theme handle")
	}
	if theme.ColorScheme() == nil {
		t.Fatal("ColorScheme() = nil")
	}
	if theme.FontScheme() == nil {
		t.Fatal("FontScheme() = nil")
	}
	if theme.FontScheme().MinorLatin() == "" {
		t.Errorf("MinorLatin() empty, want a typeface")
	}
}

// TestThemeUnmodifiedRoundTripXlsx verifies the theme part is byte-identical
// when read but not modified.
func TestThemeUnmodifiedRoundTripXlsx(t *testing.T) {
	if _, err := os.Stat(themeFixture); err != nil {
		t.Skip("external fixture absent")
	}
	orig, err := os.ReadFile(themeFixture)
	if err != nil {
		t.Fatal(err)
	}
	origParts := readAllZipParts(t, orig)
	origTheme, ok := origParts["xl/theme/theme1.xml"]
	if !ok {
		t.Fatal("fixture has no xl/theme/theme1.xml")
	}

	wb, err := Open(themeFixture)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = wb.Close() }()

	if theme := wb.Theme(); theme == nil {
		t.Fatal("Theme() = nil")
	} else if theme.Modified() {
		t.Fatal("Theme reports modified after read-only access")
	}

	out, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	savedTheme, ok := readAllZipParts(t, out)["xl/theme/theme1.xml"]
	if !ok {
		t.Fatal("saved output has no theme part")
	}
	if !bytes.Equal(origTheme, savedTheme) {
		t.Errorf("unmodified theme part not byte-identical:\n orig %d bytes\nsaved %d bytes", len(origTheme), len(savedTheme))
	}
}

// TestThemeModifiedRoundTripXlsx verifies edits persist across a save.
func TestThemeModifiedRoundTripXlsx(t *testing.T) {
	if _, err := os.Stat(themeFixture); err != nil {
		t.Skip("external fixture absent")
	}
	wb, err := Open(themeFixture)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	theme := wb.Theme()
	if theme == nil {
		t.Fatal("Theme() = nil")
	}
	green, _ := dml.ParseRGB("00FF00")
	theme.ColorScheme().SetAccent2(green.ToColor())
	theme.FontScheme().SetMinorLatin("Verdana")
	if !theme.Modified() {
		t.Fatal("Theme not marked modified after setters")
	}

	out, err := wb.SaveBytes()
	_ = wb.Close()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	wb2, err := openBytes(t, out)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer func() { _ = wb2.Close() }()
	theme2 := wb2.Theme()
	if theme2 == nil {
		t.Fatal("reopened Theme() = nil")
	}
	if got := theme2.ColorScheme().Accent2().RGB.String(); got != "00FF00" {
		t.Errorf("Accent2 after round-trip = %q, want 00FF00", got)
	}
	if got := theme2.FontScheme().MinorLatin(); got != "Verdana" {
		t.Errorf("MinorLatin after round-trip = %q, want Verdana", got)
	}
}

// TestThemeCreatedIsNilXlsx verifies a workbook created from scratch (no theme
// part) reports a nil theme, matching pptx's behavior.
func TestThemeCreatedIsNilXlsx(t *testing.T) {
	wb := Create()
	if theme := wb.Theme(); theme != nil {
		t.Errorf("Theme() on created workbook = %v, want nil", theme)
	}
}
