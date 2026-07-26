package pptx

import (
	"strings"
	"testing"

	"github.com/mgilbir/spine/common/enum"
)

// C194: SetAnchor on a shape whose parsed body has no inset attributes must not
// write lIns/tIns/rIns/bIns — the insets are still inherited. Only the anchor
// changes.
func TestSetAnchorKeepsInheritedInsets(t *testing.T) {
	shape := `<p:sp><p:nvSpPr><p:cNvPr id="2" name="TextBox 1"/>` +
		`<p:cNvSpPr txBox="1"/><p:nvPr/></p:nvSpPr>` +
		`<p:spPr/><p:txBody><a:bodyPr/><a:lstStyle/>` +
		`<a:p><a:r><a:rPr lang="en-US"/><a:t>Body</a:t></a:r></a:p>` +
		`</p:txBody></p:sp>`
	data := buildPPTXWithSpTreeBody(t, shape)

	pres := openDeck(t, data)
	firstTextBox(t, pres).TextFrame().SetAnchor(enum.TextAnchorMiddle)

	out, err := pres.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	xml := string(zipPart(t, out, "ppt/slides/slide1.xml"))
	if !strings.Contains(xml, `anchor="ctr"`) {
		t.Errorf("anchor not written:\n%s", xml)
	}
	for _, ins := range []string{"lIns=", "tIns=", "rIns=", "bIns="} {
		if strings.Contains(xml, ins) {
			t.Errorf("SetAnchor wrote inherited inset %q (should stay inherited):\n%s", ins, xml)
		}
	}
}
