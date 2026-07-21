package docx

import (
	"strings"
	"testing"
)

// C160: headerReference/footerReference must emit their type attribute as
// w:type, not unprefixed type, and the value must survive a save→reopen→save
// round trip.
func TestHeaderFooterReference_TypeAttributePrefixed(t *testing.T) {
	doc := Create()
	doc.AddHeader(HeaderFirst).AddParagraphWithText("first page header")
	doc.AddFooter(FooterDefault).AddParagraphWithText("footer")

	checkDocumentXML := func(saved []byte) {
		t.Helper()
		docXML, ok := zipEntry(t, saved, "word/document.xml")
		if !ok {
			t.Fatal("word/document.xml missing")
		}
		s := string(docXML)
		if !strings.Contains(s, `<w:headerReference w:type="first"`) {
			t.Errorf("headerReference w:type not emitted; document.xml: %s", s)
		}
		if !strings.Contains(s, `<w:footerReference w:type="default"`) {
			t.Errorf("footerReference w:type not emitted; document.xml: %s", s)
		}
		if strings.Contains(s, ` type="`) {
			t.Errorf("unprefixed type attribute emitted; document.xml: %s", s)
		}
	}

	doc, saved := reopen(t, doc)
	checkDocumentXML(saved)

	// The parsed value must survive and re-emit prefixed on a second cycle.
	refs := doc.doc().Body.SectPr.HeaderReference
	if len(refs) != 1 || refs[0].Type != "first" {
		t.Fatalf("parsed headerReference type = %v, want [first]", refs)
	}
	_, saved = reopen(t, doc)
	checkDocumentXML(saved)
}
