package pptx

import (
	"bytes"
	"testing"

	"github.com/mgilbir/spine/common/dml"
)

// reopenFirstTextFrame saves the deck, reopens it, and returns the text frame of
// the first shape on the first slide.
func reopenFirstTextFrame(t *testing.T, p *Presentation) *TextFrame {
	t.Helper()
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	p2 := openBytes(t, data)
	shapes := p2.Slides()[0].Shapes()
	if len(shapes) == 0 {
		t.Fatal("reopened slide has no shapes")
	}
	tb, ok := shapes[0].(*TextBox)
	if !ok {
		t.Fatalf("first shape is %T, want *TextBox", shapes[0])
	}
	return tb.TextFrame()
}

func TestTextFrame_AutofitRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name    string
		autofit AutofitType
		elem    string
	}{
		{"shape", AutofitShape, "<a:spAutoFit"},
		{"normal", AutofitNormal, "<a:normAutofit"},
		{"none", AutofitNone, "<a:noAutofit"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := Create()
			box := p.AddSlide().AddTextBox()
			box.TextFrame().SetText("content")
			box.TextFrame().SetAutofit(tc.autofit)

			data, err := p.SaveBytes()
			if err != nil {
				t.Fatalf("SaveBytes: %v", err)
			}
			slideXML := zipPart(t, data, "ppt/slides/slide1.xml")
			if !bytes.Contains(slideXML, []byte(tc.elem)) {
				t.Errorf("slide1.xml missing %s:\n%s", tc.elem, slideXML)
			}

			p2 := openBytes(t, data)
			tf := p2.Slides()[0].Shapes()[0].(*TextBox).TextFrame()
			if got := tf.Autofit(); got != tc.autofit {
				t.Errorf("Autofit() = %v, want %v", got, tc.autofit)
			}
		})
	}
}

// TestTextFrame_AutofitPreservedOnAnchorEdit verifies a parsed autofit survives
// an edit that only touches the anchor (autofit is not marked dirty).
func TestTextFrame_AutofitPreservedOnAnchorEdit(t *testing.T) {
	p := Create()
	box := p.AddSlide().AddTextBox()
	box.TextFrame().SetText("content")
	box.TextFrame().SetAutofit(AutofitNormal)
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	p2 := openBytes(t, data)
	tf := p2.Slides()[0].Shapes()[0].(*TextBox).TextFrame()
	tf.SetAnchor("ctr") // only the anchor changes; autofit stays
	data2, err := p2.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes 2: %v", err)
	}
	slideXML := zipPart(t, data2, "ppt/slides/slide1.xml")
	if !bytes.Contains(slideXML, []byte("<a:normAutofit")) {
		t.Errorf("normAutofit lost after anchor edit:\n%s", slideXML)
	}
}

func TestParagraph_BulletAutoNumberRoundTrip(t *testing.T) {
	p := Create()
	box := p.AddSlide().AddTextBox()
	tf := box.TextFrame()
	para := tf.AddParagraph()
	para.AddRun().SetText("item")
	para.SetBulletAutoNumber(AutoNumberRomanUcPeriod, 3)

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	slideXML := zipPart(t, data, "ppt/slides/slide1.xml")
	if !bytes.Contains(slideXML, []byte(`<a:buAutoNum type="romanUcPeriod" startAt="3"`)) {
		t.Errorf("buAutoNum missing/wrong:\n%s", slideXML)
	}

	tf2 := reopenFirstTextFrame(t, p)
	rp := tf2.Paragraphs()[0]
	if rp.Bullet() != BulletNumber {
		t.Errorf("Bullet() = %v, want BulletNumber", rp.Bullet())
	}
	if rp.BulletAutoNumberScheme() != AutoNumberRomanUcPeriod {
		t.Errorf("scheme = %q, want romanUcPeriod", rp.BulletAutoNumberScheme())
	}
	if rp.BulletAutoNumberStart() != 3 {
		t.Errorf("startAt = %d, want 3", rp.BulletAutoNumberStart())
	}
}

func TestParagraph_BulletStylingRoundTrip(t *testing.T) {
	p := Create()
	box := p.AddSlide().AddTextBox()
	tf := box.TextFrame()
	para := tf.AddParagraph()
	para.AddRun().SetText("item")
	para.SetBulletChar("•")
	para.SetBulletColor(dml.RGB{R: 0xFF, G: 0x00, B: 0x00}.ToColor())
	para.SetBulletSizePercent(75000)
	para.SetBulletFont("Wingdings")

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	slideXML := zipPart(t, data, "ppt/slides/slide1.xml")
	for _, want := range []string{`<a:buClr>`, `<a:buSzPct val="75000"`, `<a:buFont typeface="Wingdings"`, `<a:buChar char="`} {
		if !bytes.Contains(slideXML, []byte(want)) {
			t.Errorf("slide1.xml missing %s:\n%s", want, slideXML)
		}
	}

	tf2 := reopenFirstTextFrame(t, p)
	rp := tf2.Paragraphs()[0]
	if rp.BulletColor() == nil {
		t.Fatal("BulletColor lost on round trip")
	}
	if got := rp.BulletSizePercent(); got != 75000 {
		t.Errorf("BulletSizePercent = %d, want 75000", got)
	}
	if got := rp.BulletFont(); got != "Wingdings" {
		t.Errorf("BulletFont = %q, want Wingdings", got)
	}
}

func TestParagraph_IndentAndTabsRoundTrip(t *testing.T) {
	p := Create()
	box := p.AddSlide().AddTextBox()
	tf := box.TextFrame()
	para := tf.AddParagraph()
	para.AddRun().SetText("item")
	para.SetMarginLeft(dml.EMU(457200))
	para.SetIndent(dml.EMU(-457200))
	para.AddTabStop(TabStop{Position: dml.EMU(914400), Align: TabAlignCenter})
	para.AddTabStop(TabStop{Position: dml.EMU(1828800), Align: TabAlignDecimal})

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	slideXML := zipPart(t, data, "ppt/slides/slide1.xml")
	for _, want := range []string{`marL="457200"`, `indent="-457200"`, `<a:tabLst>`, `<a:tab pos="914400" algn="ctr"`, `algn="dec"`} {
		if !bytes.Contains(slideXML, []byte(want)) {
			t.Errorf("slide1.xml missing %s:\n%s", want, slideXML)
		}
	}

	tf2 := reopenFirstTextFrame(t, p)
	rp := tf2.Paragraphs()[0]
	if got := rp.MarginLeft(); got != dml.EMU(457200) {
		t.Errorf("MarginLeft = %d, want 457200", got)
	}
	if got := rp.Indent(); got != dml.EMU(-457200) {
		t.Errorf("Indent = %d, want -457200", got)
	}
	tabs := rp.TabStops()
	if len(tabs) != 2 {
		t.Fatalf("TabStops len = %d, want 2", len(tabs))
	}
	if tabs[0].Position != dml.EMU(914400) || tabs[0].Align != TabAlignCenter {
		t.Errorf("tab 0 = %+v", tabs[0])
	}
	if tabs[1].Align != TabAlignDecimal {
		t.Errorf("tab 1 align = %q, want dec", tabs[1].Align)
	}
}
