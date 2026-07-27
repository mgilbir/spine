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

// C506: the C352 document-order rewrite made emitNum run only after the raw
// loop, so on a part with no w:num of its own a session-added w:num (and
// w:abstractNum) landed after a raw w:numIdMacAtCleanup — which the schema
// places last in CT_Numbering. numIdMacAtCleanup is a must-precede trigger just
// like num.
func TestCTNumbering_SessionDefsPrecedeNumIdMacAtCleanup(t *testing.T) {
	src := `<w:numbering xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` +
		`<w:numPicBullet w:numPicBulletId="0"/>` +
		`<w:numIdMacAtCleanup w:val="9"/>` +
		`</w:numbering>`
	var n CT_Numbering
	if err := xmlb.UnmarshalWithSource([]byte(src), &n); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	n.AbstractNum = append(n.AbstractNum, &CT_AbstractNum{AbstractNumId: "3"})
	n.Num = append(n.Num, &CT_Num{NumId: "7"})

	b := xmlb.NewWordprocessingMLBuilder()
	n.MarshalToBuilder(b, xmlb.NSWordprocessingML, "numbering")
	out := string(b.Bytes())

	iAbstract := strings.Index(out, "<w:abstractNum ")
	iNum := strings.Index(out, "<w:num ")
	iCleanup := strings.Index(out, "<w:numIdMacAtCleanup")
	if iAbstract < 0 || iNum < 0 || iCleanup < 0 {
		t.Fatalf("missing child in output: %q", out)
	}
	if iAbstract >= iNum || iNum >= iCleanup {
		t.Errorf("schema order violated (want abstractNum < num < numIdMacAtCleanup): %q", out)
	}
}
