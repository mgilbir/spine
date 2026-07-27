package oxml

import (
	"encoding/xml"
	"testing"
)

// C352: an undeclared r: prefix on r:id must still resolve. Go's decoder leaves
// the literal prefix ("r") as the attribute namespace and never yields
// Local=="r:id", so the old Local=="r:id" branch was dead and the id was
// silently dropped. The corrected Space=="r" branch recovers it.
func TestCTHyperlink_RIDUndeclaredPrefix(t *testing.T) {
	// No xmlns:r declaration, so the decoder reports {Space:"r", Local:"id"}.
	src := `<w:hyperlink ` +
		`xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" ` +
		`r:id="rId5"/>`
	var h CT_Hyperlink
	if err := xml.Unmarshal([]byte(src), &h); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if h.RID != "rId5" {
		t.Errorf("RID = %q, want rId5 (undeclared r:id dropped)", h.RID)
	}
}

// C352: the declared-prefix case (r resolves to the relationships URI) keeps
// working through the NsRelationships branch.
func TestCTHyperlink_RIDDeclaredPrefix(t *testing.T) {
	src := `<w:hyperlink ` +
		`xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" ` +
		`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" ` +
		`r:id="rId7"/>`
	var h CT_Hyperlink
	if err := xml.Unmarshal([]byte(src), &h); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if h.RID != "rId7" {
		t.Errorf("RID = %q, want rId7", h.RID)
	}
}

// C352: same fix for CT_HdrFtrRef (section.go), reached through CT_SectPr.
func TestCTHdrFtrRef_RIDUndeclaredPrefix(t *testing.T) {
	src := `<w:sectPr ` +
		`xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` +
		`<w:headerReference w:type="default" r:id="rId9"/>` +
		`</w:sectPr>`
	var sp CT_SectPr
	if err := xml.Unmarshal([]byte(src), &sp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(sp.HeaderReference) != 1 {
		t.Fatalf("HeaderReference count = %d, want 1", len(sp.HeaderReference))
	}
	if sp.HeaderReference[0].RID != "rId9" {
		t.Errorf("RID = %q, want rId9 (undeclared r:id dropped)", sp.HeaderReference[0].RID)
	}
}
