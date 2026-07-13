package oxml

import (
	"encoding/xml"
	"strings"
	"testing"

	coxml "github.com/mgilbir/spine/common/oxml"
	xmlb "github.com/mgilbir/spine/common/xml"
)

// C223: multiple root-level mc:AlternateContent siblings must not collapse to
// the last one, and each must re-emit at its original position relative to
// the typed children.
func TestSlideRoot_MultipleAlternateContentInPosition(t *testing.T) {
	src := `<p:sld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"` +
		` xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"` +
		` xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">` +
		`<p:cSld><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr/></p:spTree></p:cSld>` +
		`<mc:AlternateContent xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006" xmlns:p14="http://schemas.microsoft.com/office/powerpoint/2010/main">` +
		`<mc:Choice Requires="p14"><p14:marker val="1"/></mc:Choice><mc:Fallback xmlns=""></mc:Fallback></mc:AlternateContent>` +
		`<p:transition><p:fade/></p:transition>` +
		`<p:timing><p:tnLst><p:par><p:cTn id="1" dur="indefinite" restart="never" nodeType="tmRoot"/></p:par></p:tnLst></p:timing>` +
		`<mc:AlternateContent xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006" xmlns:p14="http://schemas.microsoft.com/office/powerpoint/2010/main">` +
		`<mc:Choice Requires="p14"><p14:marker val="2"/></mc:Choice><mc:Fallback xmlns=""></mc:Fallback></mc:AlternateContent>` +
		`</p:sld>`

	var sld Slide
	if err := xml.Unmarshal([]byte(src), &sld); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(sld.AlternateContent) != 2 {
		t.Fatalf("AlternateContent count = %d, want 2", len(sld.AlternateContent))
	}

	b := xmlb.NewPresentationMLBuilder()
	sld.MarshalRootToBuilder(b)
	if err := b.Finish(); err != nil {
		t.Fatalf("builder: %v", err)
	}
	out := b.String()

	m1 := strings.Index(out, `<p14:marker val="1"/>`)
	m2 := strings.Index(out, `<p14:marker val="2"/>`)
	tr := strings.Index(out, "<p:transition>")
	tm := strings.Index(out, "<p:timing>")
	if m1 < 0 || m2 < 0 {
		t.Fatalf("AlternateContent sibling(s) lost:\n%s", out)
	}
	if m1 >= tr || tr >= tm || tm >= m2 {
		t.Errorf("AlternateContent positions not preserved (m1=%d tr=%d tm=%d m2=%d):\n%s", m1, tr, tm, m2, out)
	}
}

// A slide with no parsed anchors places programmatically added
// AlternateContent at the historical position (after the transition).
func TestSlideRoot_DefaultACPosition(t *testing.T) {
	sld := Slide{
		CSld:       &CommonSlideData{SpTree: &ShapeTree{}},
		Transition: &Transition{Fade: &OptionalBlackTransition{}},
		Timing:     &Timing{},
		AlternateContent: []*AlternateContent{
			{Choices: []coxml.AlternateContentChoice{{Requires: "p14", Content: []byte(`<p14:marker val="9"/>`)}}},
		},
	}
	b := xmlb.NewPresentationMLBuilder()
	sld.MarshalRootToBuilder(b)
	if err := b.Finish(); err != nil {
		t.Fatalf("builder: %v", err)
	}
	out := b.String()
	ac := strings.Index(out, "<mc:AlternateContent")
	tr := strings.Index(out, "<p:transition>")
	tm := strings.Index(out, "<p:timing")
	if ac < 0 {
		t.Fatalf("AlternateContent lost:\n%s", out)
	}
	if tr >= ac || ac >= tm {
		t.Errorf("default AC position not between transition and timing (tr=%d ac=%d tm=%d):\n%s", tr, ac, tm, out)
	}
}

// A timing element set programmatically after parse (media autoplay) is still
// emitted even though it was not part of the parsed child sequence.
func TestSlideRoot_ProgrammaticTimingStillEmitted(t *testing.T) {
	src := `<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main">` +
		`<p:cSld><p:spTree><p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr/></p:spTree></p:cSld>` +
		`</p:sld>`
	var sld Slide
	if err := xml.Unmarshal([]byte(src), &sld); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	sld.Timing = &Timing{}

	b := xmlb.NewPresentationMLBuilder()
	sld.MarshalRootToBuilder(b)
	if err := b.Finish(); err != nil {
		t.Fatalf("builder: %v", err)
	}
	if !strings.Contains(b.String(), "<p:timing") {
		t.Errorf("programmatically set timing dropped:\n%s", b.String())
	}
}
