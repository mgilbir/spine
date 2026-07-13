package pptx

import (
	"testing"
)

// C84: master and layout Placeholders return the placeholder shapes parsed
// from the master/layout parts of an opened deck (previously nil stubs).
func TestMasterAndLayoutPlaceholders_LoadedDeck(t *testing.T) {
	p, err := Open("testdata/test.pptx")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = p.Close() }()

	masters := p.SlideMasters()
	if len(masters) == 0 {
		t.Fatal("no slide masters loaded")
	}
	phs := masters[0].Placeholders()
	if len(phs) == 0 {
		t.Fatal("master Placeholders() is empty")
	}
	// test.pptx's master carries title, body, dt, ftr, sldNum placeholders.
	types := map[PlaceholderType]uint32{}
	for _, ph := range phs {
		types[ph.PlaceholderType()] = ph.Index()
	}
	if _, ok := types[PlaceholderTitle]; !ok {
		t.Errorf("master has no title placeholder, got %v", types)
	}
	if idx, ok := types[PlaceholderSlideNumber]; !ok || idx != 4 {
		t.Errorf("master sldNum placeholder idx = %d (found=%v), want 4", idx, ok)
	}
	if ph := masters[0].GetPlaceholder(PlaceholderTitle); ph == nil {
		t.Error("master GetPlaceholder(title) = nil")
	}

	layouts := p.SlideLayouts()
	if len(layouts) == 0 {
		t.Fatal("no slide layouts loaded")
	}
	layoutPhs := layouts[0].Placeholders()
	if len(layoutPhs) == 0 {
		t.Error("layout Placeholders() is empty")
	}
	found := false
	for _, ph := range layoutPhs {
		if ph.PlaceholderType() == PlaceholderCenteredTitle || ph.PlaceholderType() == PlaceholderTitle {
			found = true
		}
	}
	if !found {
		t.Errorf("layout 1 has no title placeholder among %d placeholders", len(layoutPhs))
	}
}

// C84: Theme is parsed from the master's theme part on open, exposing the
// color and font schemes read-only.
func TestTheme_LoadedDeck(t *testing.T) {
	p, err := Open("testdata/test.pptx")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = p.Close() }()

	th := p.Theme()
	if th == nil {
		t.Fatal("Theme() = nil for an opened deck with a theme part")
	}
	if th != p.SlideMasters()[0].Theme() {
		t.Error("presentation theme differs from the first master's theme")
	}
	if got := th.Name(); got != "Office Theme" {
		t.Errorf("theme name = %q, want %q", got, "Office Theme")
	}

	cs := th.ColorScheme()
	if cs == nil {
		t.Fatal("ColorScheme() = nil")
	}
	if got := cs.Name(); got != "Office" {
		t.Errorf("color scheme name = %q, want %q", got, "Office")
	}
	// Fixture values from testdata/test.pptx ppt/theme/theme1.xml.
	if got := cs.Accent1().RGB.String(); got != "4F81BD" {
		t.Errorf("accent1 = %s, want 4F81BD", got)
	}
	if got := cs.Dark2().RGB.String(); got != "1F497D" {
		t.Errorf("dark2 = %s, want 1F497D", got)
	}
	// dk1 is a sysClr slot resolved via lastClr.
	if got := cs.Dark1().RGB.String(); got != "000000" {
		t.Errorf("dark1 = %s, want 000000", got)
	}
	if got := cs.Hyperlink().RGB.String(); got != "0000FF" {
		t.Errorf("hyperlink = %s, want 0000FF", got)
	}

	fs := th.FontScheme()
	if fs == nil {
		t.Fatal("FontScheme() = nil")
	}
	if got := fs.MajorLatin(); got != "Calibri" {
		t.Errorf("major latin font = %q, want Calibri", got)
	}

	// A created deck has no parsed theme; the getter documents this.
	if created := Create(); created.Theme() != nil {
		t.Error("created deck Theme() should be nil")
	}
}
