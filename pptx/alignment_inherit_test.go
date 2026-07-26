package pptx

import (
	"strings"
	"testing"

	"github.com/mgilbir/spine/common/enum"
)

// C262: SetText on a centered title placeholder must not write algn="l" — the
// paragraph inherits the layout's centering. An explicit SetAlignment still
// emits algn.
func TestSetTextInheritsAlignment(t *testing.T) {
	shape := `<p:sp><p:nvSpPr><p:cNvPr id="2" name="Title 1"/>` +
		`<p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr>` +
		`<p:nvPr><p:ph type="ctrTitle"/></p:nvPr></p:nvSpPr>` +
		`<p:spPr/><p:txBody><a:bodyPr/><a:lstStyle/>` +
		`<a:p><a:pPr algn="ctr"/><a:r><a:rPr lang="en-US"/><a:t>Original</a:t></a:r></a:p>` +
		`</p:txBody></p:sp>`
	data := buildPPTXWithSpTreeBody(t, shape)

	pres := openDeck(t, data)
	var ph *PlaceholderShape
	for _, sh := range pres.Slides()[0].Shapes() {
		if p, ok := sh.(*PlaceholderShape); ok {
			ph = p
			break
		}
	}
	if ph == nil {
		t.Fatal("no placeholder materialized")
	}
	ph.TextFrame().SetText("Replaced")

	out, err := pres.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	xml := string(zipPart(t, out, "ppt/slides/slide1.xml"))
	if !strings.Contains(xml, "Replaced") {
		t.Fatalf("text not replaced:\n%s", xml)
	}
	if strings.Contains(xml, `algn="l"`) {
		t.Errorf("SetText clobbered inherited alignment with algn=\"l\":\n%s", xml)
	}

	// An explicit SetAlignment(left) must still emit algn="l".
	pres2 := openDeck(t, data)
	for _, sh := range pres2.Slides()[0].Shapes() {
		if p, ok := sh.(*PlaceholderShape); ok {
			p.TextFrame().SetText("Left")
			p.TextFrame().Paragraphs()[0].SetAlignment(enum.TextAlignLeft)
			break
		}
	}
	out2, err := pres2.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	if xml2 := string(zipPart(t, out2, "ppt/slides/slide1.xml")); !strings.Contains(xml2, `algn="l"`) {
		t.Errorf("explicit SetAlignment(left) did not emit algn=\"l\":\n%s", xml2)
	}
}
