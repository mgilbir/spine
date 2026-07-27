package oxml

import (
	"strings"
	"testing"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// C352: whitespace between <w:background/> and <w:body/> must round-trip in its
// source position. The unmarshal slot used to flip only at w:body, so the
// inter-child whitespace was captured before the background and replayed there,
// drifting pretty-printed documents that carry a background.
func TestCTDocument_BackgroundBodyWhitespaceSlot(t *testing.T) {
	src := "<w:document xmlns:w=\"http://schemas.openxmlformats.org/wordprocessingml/2006/main\">\n  " +
		"<w:background w:color=\"FFFFFF\"/>\n  " +
		"<w:body/>\n</w:document>"

	var doc CT_Document
	if err := xmlb.UnmarshalWithSource([]byte(src), &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	b := xmlb.NewWordprocessingMLBuilder()
	doc.MarshalToBuilder(b, xmlb.NSWordprocessingML, "document")
	out := string(b.Bytes())

	idxBg := strings.Index(out, "<w:background")
	idxBody := strings.Index(out, "<w:body")
	if idxBg < 0 || idxBody < 0 {
		t.Fatalf("missing background/body in output: %q", out)
	}
	if idxBg > idxBody {
		t.Fatalf("background must precede body: %q", out)
	}
	// Exactly WS1 ("\n  ") precedes the background: WS2 must not be hoisted here
	// (pre-fix, both WS1 and WS2 landed before the background => "\n  \n  ").
	before := out[:idxBg]
	if !strings.HasSuffix(before, "\n  ") || strings.HasSuffix(before, "\n  \n  ") {
		t.Errorf("whitespace before background not exactly WS1: %q", out)
	}
	// WS2 ("\n  ") must sit between background and body, immediately before
	// <w:body (pre-fix the background close abutted <w:body with no whitespace).
	if !strings.HasSuffix(out[:idxBody], "\n  ") {
		t.Errorf("inter-child whitespace not between background and body: %q", out)
	}
}
