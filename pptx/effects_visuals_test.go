package pptx

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mgilbir/spine/common/dml"
)

// --- Shape effects ---

func TestShapeGlowRoundTrip(t *testing.T) {
	as := NewAutoShape(PresetRect)
	as.SetSize(dml.Inches(2), dml.Inches(1))
	as.SetGlow(Glow{Color: dml.NewRGB(0xFF, 0x00, 0x00).ToColor(), Radius: 5})

	got := as.Glow()
	if got == nil {
		t.Fatal("Glow() returned nil after SetGlow")
	}
	if got.Radius != 5 {
		t.Errorf("Glow radius = %v, want 5", got.Radius)
	}

	xml := marshalSlideWithShape(t, as)
	if !strings.Contains(xml, "<a:glow rad=\"63500\"") {
		t.Errorf("a:glow not emitted:\n%s", xml)
	}
}

func TestShapeSoftEdgeRoundTrip(t *testing.T) {
	tb := NewTextBox()
	tb.SetSize(dml.Inches(2), dml.Inches(1))
	tb.SetSoftEdge(SoftEdge{Radius: 3})

	if got := tb.SoftEdge(); got == nil || got.Radius != 3 {
		t.Fatalf("SoftEdge() = %+v, want radius 3", got)
	}

	xml := marshalSlideWithShape(t, tb)
	if !strings.Contains(xml, "<a:softEdge rad=\"38100\"") {
		t.Errorf("a:softEdge not emitted:\n%s", xml)
	}
}

func TestShapeReflectionRoundTrip(t *testing.T) {
	as := NewAutoShape(PresetEllipse)
	as.SetSize(dml.Inches(2), dml.Inches(1))
	as.SetReflection(Reflection{
		BlurRadius:    0,
		StartOpacity:  0.5,
		EndOpacity:    0.003,
		EndPosition:   0.35,
		Direction:     90,
		FadeDirection: 90,
	})

	got := as.Reflection()
	if got == nil {
		t.Fatal("Reflection() returned nil")
	}
	if got.StartOpacity != 0.5 || got.EndPosition != 0.35 {
		t.Errorf("Reflection = %+v, want StartOpacity 0.5 EndPosition 0.35", got)
	}
	if got.Direction != 90 {
		t.Errorf("Reflection Direction = %v, want 90", got.Direction)
	}

	xml := marshalSlideWithShape(t, as)
	if !strings.Contains(xml, "<a:reflection") {
		t.Errorf("a:reflection not emitted:\n%s", xml)
	}
}

func TestShapeBevelRoundTrip(t *testing.T) {
	as := NewAutoShape(PresetRect)
	as.SetSize(dml.Inches(2), dml.Inches(1))
	as.SetBevel(Bevel{Preset: dml.BevelCircle, Width: 6, Height: 6})

	if got := as.Bevel(); got == nil || got.Preset != "circle" {
		t.Fatalf("Bevel() = %+v, want circle", got)
	}

	xml := marshalSlideWithShape(t, as)
	if !strings.Contains(xml, "<a:sp3d>") || !strings.Contains(xml, "<a:bevelT ") {
		t.Errorf("a:sp3d/a:bevelT not emitted:\n%s", xml)
	}
}

func TestShapeThemeGlowColor(t *testing.T) {
	as := NewAutoShape(PresetRect)
	as.SetSize(dml.Inches(2), dml.Inches(1))
	as.SetGlow(Glow{Color: dml.ThemeColorAccent1.ToColor(), Radius: 4})

	xml := marshalSlideWithShape(t, as)
	if !strings.Contains(xml, "<a:schemeClr val=\"accent1\"") {
		t.Errorf("theme glow color not routed to schemeClr:\n%s", xml)
	}
}

// marshalSlideWithShape adds a shape to a fresh deck and returns its slide XML.
func marshalSlideWithShape(t *testing.T, shape Shape) string {
	t.Helper()
	pres := Create()
	slide := pres.AddSlide()
	if err := slide.AddShape(shape); err != nil {
		t.Fatal(err)
	}
	data, err := pres.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	return string(zipPart(t, data, "ppt/slides/slide1.xml"))
}

// --- Background ---

func TestSlideBackgroundSolidFill(t *testing.T) {
	pres := Create()
	slide := pres.AddSlide()
	slide.SetBackgroundFill(dml.NewSolidFill(dml.NewRGB(0x20, 0x30, 0x40).ToColor()))

	if !slide.HasBackground() {
		t.Fatal("HasBackground() false after SetBackgroundFill")
	}
	if c, ok := slide.BackgroundColor(); !ok || c.RGB != dml.NewRGB(0x20, 0x30, 0x40) {
		t.Errorf("BackgroundColor = %v ok=%v, want 203040", c, ok)
	}

	data, err := pres.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	xml := string(zipPart(t, data, "ppt/slides/slide1.xml"))
	if !strings.Contains(xml, "<p:bg><p:bgPr><a:solidFill><a:srgbClr val=\"203040\"") {
		t.Errorf("background solid fill not emitted:\n%s", xml)
	}

	// Reopen and confirm the background survives.
	reopened := openBytes(t, data)
	if !reopened.Slides()[0].HasBackground() {
		t.Error("background lost after reopen")
	}
	if c, ok := reopened.Slides()[0].BackgroundColor(); !ok || c.RGB != dml.NewRGB(0x20, 0x30, 0x40) {
		t.Errorf("reopened BackgroundColor = %v ok=%v, want 203040", c, ok)
	}
}

func TestSlideBackgroundGradientAndClear(t *testing.T) {
	pres := Create()
	slide := pres.AddSlide()
	slide.SetBackgroundFill(dml.NewGradientFill(90,
		dml.GradientStop{Position: 0, Color: dml.NewRGB(0x00, 0x00, 0x00).ToColor()},
		dml.GradientStop{Position: 1, Color: dml.NewRGB(0xFF, 0xFF, 0xFF).ToColor()},
	))

	data, err := pres.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	xml := string(zipPart(t, data, "ppt/slides/slide1.xml"))
	if !strings.Contains(xml, "<p:bg><p:bgPr><a:gradFill>") {
		t.Errorf("background gradient not emitted:\n%s", xml)
	}

	slide.ClearBackground()
	if slide.HasBackground() {
		t.Error("HasBackground() true after ClearBackground")
	}
}

func TestMasterBackgroundSolidFill(t *testing.T) {
	pres := Create()
	master := pres.SlideMasters()[0]
	master.SetBackgroundFill(dml.NewSolidFill(dml.NewRGB(0x11, 0x22, 0x33).ToColor()))

	if c, ok := master.BackgroundColor(); !ok || c.RGB != dml.NewRGB(0x11, 0x22, 0x33) {
		t.Errorf("master BackgroundColor = %v ok=%v", c, ok)
	}

	data, err := pres.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	xml := string(zipPart(t, data, "ppt/slideMasters/slideMaster1.xml"))
	if !strings.Contains(xml, "<a:srgbClr val=\"112233\"") {
		t.Errorf("master background fill not emitted:\n%s", xml)
	}
}

// --- Transition variants and parameters ---

func TestTransitionNewVariants(t *testing.T) {
	cases := []struct {
		typ  TransitionType
		elem string
	}{
		{TransitionCircle, "<p:circle/>"},
		{TransitionNewsflash, "<p:newsflash/>"},
		{TransitionWedge, "<p:wedge/>"},
		{TransitionComb, "<p:comb"},
		{TransitionRandomBar, "<p:randomBar"},
		{TransitionPull, "<p:pull"},
		{TransitionStrips, "<p:strips"},
		{TransitionZoom, "<p:zoom"},
	}
	for _, tc := range cases {
		pres := Create()
		slide := pres.AddSlide()
		slide.SetTransition(Transition{Type: tc.typ, AdvanceOnClick: true})

		if got := slide.Transition(); got == nil || got.Type != tc.typ {
			t.Errorf("type %d: getter = %+v", tc.typ, got)
		}

		data, err := pres.SaveBytes()
		if err != nil {
			t.Fatal(err)
		}
		xml := string(zipPart(t, data, "ppt/slides/slide1.xml"))
		if !strings.Contains(xml, tc.elem) {
			t.Errorf("type %d: %q not in slide XML:\n%s", tc.typ, tc.elem, xml)
		}
	}
}

func TestTransitionDirectionOrientationSpokes(t *testing.T) {
	pres := Create()
	slide := pres.AddSlide()
	slide.SetTransition(Transition{
		Type:      TransitionPull,
		Direction: TransitionDirRightDown,
	})
	if got := slide.Transition(); got.Direction != TransitionDirRightDown {
		t.Errorf("Direction = %q, want rd", got.Direction)
	}

	slide.SetTransition(Transition{Type: TransitionComb, Orientation: TransitionVertical})
	if got := slide.Transition(); got.Orientation != TransitionVertical {
		t.Errorf("Orientation = %q, want vert", got.Orientation)
	}

	slide.SetTransition(Transition{Type: TransitionWheel, Spokes: 8})
	if got := slide.Transition(); got.Spokes != 8 {
		t.Errorf("Spokes = %d, want 8", got.Spokes)
	}

	slide.SetTransition(Transition{Type: TransitionSplit, Orientation: TransitionVertical, Direction: TransitionDirIn})
	got := slide.Transition()
	if got.Orientation != TransitionVertical || got.Direction != TransitionDirIn {
		t.Errorf("Split = %+v, want vert/in", got)
	}

	data, err := pres.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	xml := string(zipPart(t, data, "ppt/slides/slide1.xml"))
	if !strings.Contains(xml, `<p:split orient="vert" dir="in"/>`) {
		t.Errorf("split orient/dir not emitted:\n%s", xml)
	}
}

func TestTransitionThroughBlackAndSound(t *testing.T) {
	pres := Create()
	slide := pres.AddSlide()
	slide.SetTransition(Transition{
		Type:         TransitionFade,
		ThroughBlack: true,
		Sound:        &TransitionSound{StopPreviousSound: true},
	})

	got := slide.Transition()
	if !got.ThroughBlack {
		t.Error("ThroughBlack not read back")
	}
	if got.Sound == nil || !got.Sound.StopPreviousSound {
		t.Errorf("Sound = %+v, want StopPreviousSound", got.Sound)
	}

	data, err := pres.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	xml := string(zipPart(t, data, "ppt/slides/slide1.xml"))
	if !strings.Contains(xml, `<p:fade thruBlk="1"/>`) {
		t.Errorf("fade thruBlk not emitted:\n%s", xml)
	}
	if !strings.Contains(xml, "<p:sndAc><p:endSnd/></p:sndAc>") {
		t.Errorf("sndAc/endSnd not emitted:\n%s", xml)
	}
}

// --- Table cell margins and table style ---

func TestTableCellMarginsAndStyle(t *testing.T) {
	pres := Create()
	slide := pres.AddSlide()
	tbl := slide.AddTable(2, 2)
	tbl.SetSize(dml.Inches(4), dml.Inches(2))
	tbl.SetStyleID("{5C22544A-7EE6-4342-B048-85BDC9FD1C3A}")
	tbl.Cell(0, 0).SetMargins(dml.EMU(1000), dml.EMU(2000), dml.EMU(3000), dml.EMU(4000))

	if tbl.StyleID() != "{5C22544A-7EE6-4342-B048-85BDC9FD1C3A}" {
		t.Errorf("StyleID = %q", tbl.StyleID())
	}
	if l, top, r, b, ok := tbl.Cell(0, 0).Margins(); !ok || l != 1000 || top != 2000 || r != 3000 || b != 4000 {
		t.Errorf("Margins = %v,%v,%v,%v ok=%v", l, top, r, b, ok)
	}

	data, err := pres.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	xml := string(zipPart(t, data, "ppt/slides/slide1.xml"))
	if !strings.Contains(xml, "<a:tableStyleId>{5C22544A-7EE6-4342-B048-85BDC9FD1C3A}</a:tableStyleId>") {
		t.Errorf("tableStyleId not emitted:\n%s", xml)
	}
	if !strings.Contains(xml, `marL="1000"`) || !strings.Contains(xml, `marB="4000"`) {
		t.Errorf("cell margins not emitted:\n%s", xml)
	}

	// Reopen and confirm both survive.
	reopened := openBytes(t, data)
	rt := reopened.Slides()[0].Shapes()[0].(*Table)
	if rt.StyleID() != "{5C22544A-7EE6-4342-B048-85BDC9FD1C3A}" {
		t.Errorf("reopened StyleID = %q", rt.StyleID())
	}
	if l, top, r, b, ok := rt.Cell(0, 0).Margins(); !ok || l != 1000 || top != 2000 || r != 3000 || b != 4000 {
		t.Errorf("reopened Margins = %v,%v,%v,%v ok=%v", l, top, r, b, ok)
	}
}

// --- Embedded fonts and custom shows ---

func TestEmbeddedFontsReadWrite(t *testing.T) {
	pres := Create()
	pres.AddSlide()
	pres.SetEmbedTrueTypeFonts(true)
	pres.SetEmbeddedFonts([]EmbeddedFont{
		{Typeface: "Georgia", Regular: "rIdF1", Bold: "rIdF2"},
	})

	if !pres.EmbedTrueTypeFonts() {
		t.Error("EmbedTrueTypeFonts() false")
	}
	fonts := pres.EmbeddedFonts()
	if len(fonts) != 1 || fonts[0].Typeface != "Georgia" || fonts[0].Regular != "rIdF1" || fonts[0].Bold != "rIdF2" {
		t.Fatalf("EmbeddedFonts = %+v", fonts)
	}

	data, err := pres.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	xml := string(zipPart(t, data, "ppt/presentation.xml"))
	if !strings.Contains(xml, "<p:embeddedFontLst>") ||
		!strings.Contains(xml, `<p:font typeface="Georgia"/>`) ||
		!strings.Contains(xml, `embedTrueTypeFonts="1"`) {
		t.Errorf("embedded font list not emitted:\n%s", xml)
	}

	reopened := openBytes(t, data)
	if got := reopened.EmbeddedFonts(); len(got) != 1 || got[0].Regular != "rIdF1" {
		t.Errorf("reopened EmbeddedFonts = %+v", got)
	}
}

func TestCustomShowsReadWrite(t *testing.T) {
	pres := Create()
	s1 := pres.AddSlide()
	s2 := pres.AddSlide()

	// Save once so the slides receive presentation-level relationship ids.
	data, err := pres.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	_ = s1
	_ = s2

	reopened := openBytes(t, data)
	id := reopened.AddCustomShow("Demo", reopened.Slides()[1], reopened.Slides()[0])
	shows := reopened.CustomShows()
	if len(shows) != 1 || shows[0].Name != "Demo" || shows[0].ID != id {
		t.Fatalf("CustomShows = %+v", shows)
	}
	if len(shows[0].SlideRelIDs) != 2 {
		t.Fatalf("custom show slide ids = %v", shows[0].SlideRelIDs)
	}

	data2, err := reopened.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	xml := string(zipPart(t, data2, "ppt/presentation.xml"))
	if !strings.Contains(xml, `<p:custShow name="Demo"`) || !strings.Contains(xml, "<p:sldLst>") {
		t.Errorf("custom show not emitted:\n%s", xml)
	}

	final := openBytes(t, data2)
	if got := final.CustomShows(); len(got) != 1 || len(got[0].SlideRelIDs) != 2 {
		t.Errorf("reopened CustomShows = %+v", got)
	}
}

// TestEffectsDoNotPerturbUnsetShapes guards the additive requirement: a deck
// that never touches the new accessors round-trips byte-for-byte.
func TestEffectsDoNotPerturbUnsetShapes(t *testing.T) {
	pres := Create()
	slide := pres.AddSlide()
	as := NewAutoShape(PresetRect)
	as.SetSize(dml.Inches(2), dml.Inches(1))
	if err := slide.AddShape(as); err != nil {
		t.Fatal(err)
	}
	data, err := pres.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}

	reopened := openBytes(t, data)
	again, err := reopened.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(zipPart(t, data, "ppt/slides/slide1.xml"), zipPart(t, again, "ppt/slides/slide1.xml")) {
		t.Error("slide XML changed on a no-op round trip")
	}
}
