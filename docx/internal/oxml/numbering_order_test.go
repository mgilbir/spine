package oxml

import (
	"encoding/xml"
	"strings"
	"testing"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// C352: CT_Numbering must emit its raw-preserved children in source document
// order. The earlier grouped emit (numPicBullet, then all abstractNum, then all
// num) reordered producers that interleaved the child kinds, breaking the
// byte-for-byte promise in the type's header comment.
func TestCTNumbering_PreservesChildOrder(t *testing.T) {
	// numPicBullet deliberately sits after the abstractNum here; the grouped
	// emit hoisted it to the front.
	src := `<w:numbering xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` +
		`<w:abstractNum w:abstractNumId="0"><w:lvl w:ilvl="0"/></w:abstractNum>` +
		`<w:numPicBullet w:numPicBulletId="0"/>` +
		`<w:num w:numId="1"><w:abstractNumId w:val="0"/></w:num>` +
		`</w:numbering>`
	var n CT_Numbering
	if err := xml.Unmarshal([]byte(src), &n); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	b := xmlb.NewWordprocessingMLBuilder()
	n.MarshalToBuilder(b, xmlb.NSWordprocessingML, "numbering")
	out := string(b.Bytes())

	iAbstract := strings.Index(out, "<w:abstractNum ")
	iPicBullet := strings.Index(out, "<w:numPicBullet")
	iNum := strings.Index(out, "<w:num ")
	if iAbstract < 0 || iPicBullet < 0 || iNum < 0 {
		t.Fatalf("missing child in output: %q", out)
	}
	if iAbstract >= iPicBullet || iPicBullet >= iNum {
		t.Errorf("children not in source order (want abstractNum, numPicBullet, num): %q", out)
	}
}
