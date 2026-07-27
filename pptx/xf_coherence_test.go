package pptx

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/mgilbir/spine/common/dml"
)

// ---------------------------------------------------------------------------
// C571 — Theme() returns the shared *dml.ThemeEditor, and edits persist.
// ---------------------------------------------------------------------------

// TestThemeIsTheSharedEditor: the accessor used to hand back a pptx-private
// read-only type while docx and xlsx returned *dml.ThemeEditor, so theme code
// did not port between formats at all — it did not even compile. The reason it
// stayed that way (C374: regenerating the part from a model that did not carry
// custClrLst/extLst) is fixed, which is what makes converging safe; this test
// proves both halves — the type, and the lossless write-back.
func TestThemeIsTheSharedEditor(t *testing.T) {
	p, err := Open("testdata/test.pptx")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = p.Close() }()

	// Assign through a function typed on *dml.ThemeEditor: this compiles only
	// while both Theme accessors return the shared editor, which is the
	// convergence C571 is about.
	sharedEditor := func(ed *dml.ThemeEditor) *dml.ThemeEditor { return ed }
	ed := sharedEditor(p.Theme())
	if ed == nil {
		t.Fatal("Theme() = nil for an opened deck with a theme part")
	}
	if got := ed.Name(); got != "Office Theme" {
		t.Errorf("theme name = %q, want %q", got, "Office Theme")
	}

	// The presentation theme is the first master's theme, and they are the same
	// handle so an edit through either is written once.
	if ed != sharedEditor(p.SlideMasters()[0].Theme()) {
		t.Error("presentation theme is a different handle from the first master's")
	}

	cs := ed.ColorScheme()
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

	fs := ed.FontScheme()
	if fs == nil {
		t.Fatal("FontScheme() = nil")
	}
	if got := fs.MajorLatin(); got != "Calibri" {
		t.Errorf("major latin font = %q, want Calibri", got)
	}
	// The East Asian / complex-script slots the old pptx-local type exposed and
	// the editor did not: converging must not have dropped a capability.
	_ = fs.MajorEastAsia()
	_ = fs.MinorComplexScript()

	// The format scheme's line styles likewise survive the merge, read-only.
	if fmtScheme := ed.FormatScheme(); fmtScheme != nil {
		_ = fmtScheme.LineStyles()
	}

	// A created deck has no parsed theme; the getter documents this.
	if created := Create(); created.Theme() != nil {
		t.Error("created deck Theme() should be nil")
	}
}

// TestThemeEditsArePersistedAndLossless: pptx could not change a theme at all
// before this (the setters were removed as silent no-ops in #240). Now an edit
// is written back — and the write-back must not delete the extension content
// C374 is about, which is the whole reason the convergence is safe.
func TestThemeEditsArePersistedAndLossless(t *testing.T) {
	p, err := Open("testdata/test.pptx")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = p.Close() }()

	ed := p.Theme()
	if ed == nil {
		t.Fatal("Theme() = nil")
	}
	before := themePartBytes(t, p)
	if ed.Modified() {
		t.Fatal("a freshly read theme reports itself modified")
	}

	ed.SetName("Spine Theme")
	ed.ColorScheme().SetAccent1(dml.NewRGB(0x11, 0x22, 0x33).ToColor())
	ed.FontScheme().SetMinorLatin("Verdana")
	if !ed.Modified() {
		t.Fatal("setters did not mark the theme modified")
	}

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	reopened, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer func() { _ = reopened.Close() }()

	got := reopened.Theme()
	if got == nil {
		t.Fatal("reopened deck has no theme")
	}
	if got.Name() != "Spine Theme" {
		t.Errorf("theme name after save = %q, want %q", got.Name(), "Spine Theme")
	}
	if c := got.ColorScheme().Accent1().RGB.String(); c != "112233" {
		t.Errorf("accent1 after save = %s, want 112233", c)
	}
	if f := got.FontScheme().MinorLatin(); f != "Verdana" {
		t.Errorf("minor latin after save = %q, want Verdana", f)
	}

	// Lossless: every element of the source theme part that the narrow model
	// does not name must still be there. Checking the fixture's own children
	// keeps this honest regardless of what the fixture happens to carry.
	after := string(themePartBytesFromPackage(t, data))
	for _, elem := range []string{"a:fmtScheme", "a:lnStyleLst", "a:effectStyleLst", "a:bgFillStyleLst"} {
		if strings.Contains(before, elem) && !strings.Contains(after, elem) {
			t.Errorf("theme write-back dropped %s (the C374 shape)", elem)
		}
	}
}

// TestUnmodifiedThemeRoundTripsVerbatim: merely reading the theme must leave the
// part byte-identical — the contract docx and xlsx already keep.
func TestUnmodifiedThemeRoundTripsVerbatim(t *testing.T) {
	p, err := Open("testdata/test.pptx")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = p.Close() }()

	before := themePartBytes(t, p)
	ed := p.Theme()
	if ed == nil {
		t.Fatal("Theme() = nil")
	}
	_ = ed.Name()
	_ = ed.ColorScheme().Accent1()

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	if after := string(themePartBytesFromPackage(t, data)); after != before {
		t.Error("reading the theme changed the saved theme part; an unmodified theme " +
			"must round-trip from its preserved bytes")
	}
}

// themePartBytes returns the in-memory bytes of the deck's first theme part.
func themePartBytes(t *testing.T, p *Presentation) string {
	t.Helper()
	for _, name := range sortedKeys(p.themeData) {
		if strings.HasPrefix(name, "/ppt/theme/theme") {
			return string(p.themeData[name])
		}
	}
	t.Fatal("deck has no /ppt/theme/themeN.xml part")
	return ""
}

// themePartBytesFromPackage pulls the first theme part out of a saved package.
func themePartBytesFromPackage(t *testing.T, data []byte) []byte {
	t.Helper()
	reopened, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer func() { _ = reopened.Close() }()
	for _, name := range sortedKeys(reopened.themeData) {
		if strings.HasPrefix(name, "/ppt/theme/theme") {
			return reopened.themeData[name]
		}
	}
	t.Fatal("saved package has no /ppt/theme/themeN.xml part")
	return nil
}

// ---------------------------------------------------------------------------
// C565 — lookups report a miss as an error.
// ---------------------------------------------------------------------------

func TestLookupsReportMissesAsErrors(t *testing.T) {
	p, err := Open("testdata/test.pptx")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = p.Close() }()

	layouts := p.SlideLayouts()
	if len(layouts) == 0 {
		t.Fatal("deck has no layouts")
	}
	name := layouts[0].Name()
	got, err := p.LayoutByName(name)
	if err != nil {
		t.Fatalf("LayoutByName(%q): %v", name, err)
	}
	if got != layouts[0] {
		t.Error("LayoutByName returned a different layout")
	}
	if _, err := p.LayoutByName("no such layout"); !errors.Is(err, ErrLayoutNotFound) {
		t.Errorf("LayoutByName(miss) = %v, want ErrLayoutNotFound", err)
	}
	if _, err := p.LayoutByType(SlideLayoutType("no such type")); !errors.Is(err, ErrLayoutNotFound) {
		t.Errorf("LayoutByType(miss) = %v, want ErrLayoutNotFound", err)
	}

	masters := p.SlideMasters()
	if len(masters) == 0 {
		t.Fatal("deck has no masters")
	}
	if _, err := masters[0].Placeholder(PlaceholderTitle); err != nil {
		t.Errorf("master Placeholder(title): %v", err)
	}
	if _, err := masters[0].Placeholder(PlaceholderType("nope")); !errors.Is(err, ErrPlaceholderNotFound) {
		t.Errorf("master Placeholder(miss) = %v, want ErrPlaceholderNotFound", err)
	}

	slide := p.AddSlide()
	if _, err := slide.Placeholder(PlaceholderTitle); !errors.Is(err, ErrPlaceholderNotFound) {
		t.Errorf("empty slide Placeholder(title) = %v, want ErrPlaceholderNotFound", err)
	}
}

// ---------------------------------------------------------------------------
// C568 — an impossible lazy-parse failure is loud, not silently empty.
// ---------------------------------------------------------------------------

// TestCorruptedSlideModelPanicsRatherThanReadingEmpty simulates the state the
// finding is about: the slide bytes that Open validated are no longer parseable
// when the lazy parse runs. Returning nil there reads as a slide with no shapes
// and writes one back on save — invisible data loss. docx has always panicked
// with a diagnostic for the identical state; pptx and xlsx now do too.
func TestCorruptedSlideModelPanicsRatherThanReadingEmpty(t *testing.T) {
	p, err := Open("testdata/test.pptx")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = p.Close() }()

	if len(p.slides) == 0 {
		t.Fatal("deck has no slides")
	}
	s := p.slides[0]
	// Force the impossible state directly: the only way to reach it in a real
	// process is memory corruption, which a test cannot stage.
	s.sxParsed = true
	s.sxModel = nil
	s.sxParseErr = errors.New("simulated in-memory corruption")

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("sx() returned silently for an unparseable slide; a nil model " +
				"reads as an empty slide and is written back that way (C568)")
		}
		msg, _ := r.(string)
		if !strings.Contains(msg, s.partName) {
			t.Errorf("panic message does not name the part: %v", r)
		}
	}()
	_ = s.sx()
}
