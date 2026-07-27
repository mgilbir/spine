package pptx

import (
	"bytes"
	"testing"
)

// Audit section 5 ("margins getters lie by omission"): a parsed body with no
// inset attributes inherits ~91440/45720, but materializes as a zero
// TextMargins that is indistinguishable from an explicit zero. Margins alone
// cannot say which; MarginsSet can.
func TestTextFrameMarginsSet_DistinguishesInheritedFromExplicit(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	s.AddTextBox().TextFrame().SetText("x")
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}

	// The same deck with every inset attribute removed from the body.
	stripped := rewriteZipPart(t, data, "ppt/slides/slide1.xml", func(xml []byte) []byte {
		out := bytes.Replace(xml, []byte(` lIns="91440"`), nil, 1)
		out = bytes.Replace(out, []byte(` tIns="45720"`), nil, 1)
		out = bytes.Replace(out, []byte(` rIns="91440"`), nil, 1)
		return bytes.Replace(out, []byte(` bIns="45720"`), nil, 1)
	})

	_, box := openBox(t, stripped)
	tf := box.TextFrame()
	if tf.MarginsSet() {
		t.Errorf("a body with no insets must report MarginsSet false, got margins %+v", tf.Margins())
	}
	if m := tf.Margins(); m != (TextMargins{}) {
		t.Errorf("expected the zero placeholder margins, got %+v", m)
	}

	// The original, whose insets are present.
	_, box2 := openBox(t, data)
	if !box2.TextFrame().MarginsSet() {
		t.Error("a body carrying explicit insets must report MarginsSet true")
	}

	// A frame created through the API always writes its four insets.
	if !NewTextFrame().MarginsSet() {
		t.Error("a newly created text frame must report MarginsSet true")
	}
}
