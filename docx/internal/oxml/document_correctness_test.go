package oxml

import (
	"encoding/xml"
	"testing"
)

// C58: w:conformance and mc:Ignorable are distinct attributes and must not be
// conflated.
func TestCTDocument_ConformanceVsIgnorable(t *testing.T) {
	src := `<w:document ` +
		`xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" ` +
		`xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006" ` +
		`w:conformance="strict" mc:Ignorable="w14"><w:body/></w:document>`

	var doc CT_Document
	if err := xml.Unmarshal([]byte(src), &doc); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if doc.Conformance != "strict" {
		t.Errorf("Conformance = %q, want strict (w:conformance dropped)", doc.Conformance)
	}
	if doc.Ignorable != "w14" {
		t.Errorf("Ignorable = %q, want w14", doc.Ignorable)
	}
}

// C63: paragraph text includes content nested in hyperlinks (and other run
// containers), in document order.
func TestCTP_TextIncludesHyperlink(t *testing.T) {
	p := &CT_P{
		R: []*CT_R{{T: []*CT_Text{{Text: "before "}}}},
		Hyperlink: []*CT_Hyperlink{
			{R: []*CT_R{{T: []*CT_Text{{Text: "link"}}}}},
		},
		childOrder: []pChildRef{
			{kind: pChildR, index: 0},
			{kind: pChildHyperlink, index: 0},
		},
	}
	if got := p.Text(); got != "before link" {
		t.Errorf("Text() = %q, want %q (hyperlink text lost)", got, "before link")
	}
}

// C63: a programmatically built paragraph (no recorded child order) still reads
// its runs.
func TestCTP_TextNoChildOrder(t *testing.T) {
	p := &CT_P{R: []*CT_R{{T: []*CT_Text{{Text: "plain"}}}}}
	if got := p.Text(); got != "plain" {
		t.Errorf("Text() = %q, want plain", got)
	}
}
