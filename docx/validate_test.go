package docx

import (
	"testing"

	"github.com/mgilbir/spine/common/validate"
	"github.com/mgilbir/spine/docx/internal/oxml"
)

func hasCode(r validate.Report, code string, sev validate.Severity) bool {
	for _, e := range r {
		if e.Code == code && e.Severity == sev {
			return true
		}
	}
	return false
}

func numPrParagraph(numID int) *oxml.CT_P {
	return &oxml.CT_P{PPr: &oxml.CT_PPr{NumPr: &oxml.CT_NumPr{NumId: &oxml.CT_DecimalNumber{Val: numID}}}}
}

// numPr referencing a numId absent from numbering.xml is a warning (C26): Word
// tolerates a dangling numPr, and real Office files ship empty numbering parts.
func TestValidate_NumberingMissing(t *testing.T) {
	num := &oxml.CT_Numbering{ParsedNumIDs: []string{"1", "2"}}
	body := &oxml.CT_Body{P: []*oxml.CT_P{numPrParagraph(99)}}
	d := &Document{document: &oxml.CT_Document{Body: body}, numbering: num}
	r := d.Validate()
	if r.HasErrors() {
		t.Fatalf("numbering-missing must not be an error (Office tolerates it): %v", r.Errors())
	}
	if !hasCode(r, codeNumberingMissing, validate.SeverityWarning) {
		t.Fatalf("expected numbering-missing warning, got: %v", r)
	}

	// A defined numId produces no numbering finding at all.
	body.P = []*oxml.CT_P{numPrParagraph(2)}
	if r := d.Validate(); hasCode(r, codeNumberingMissing, validate.SeverityWarning) {
		t.Fatalf("did not expect numbering-missing for defined numId, got: %v", r)
	}
}

// With no numbering part, a numPr is a warning, not an error.
func TestValidate_NumberingAbsentIsWarning(t *testing.T) {
	body := &oxml.CT_Body{P: []*oxml.CT_P{numPrParagraph(3)}}
	d := &Document{document: &oxml.CT_Document{Body: body}}
	r := d.Validate()
	if r.HasErrors() {
		t.Fatalf("numPr with no numbering part must not be an error: %v", r.Errors())
	}
	if !hasCode(r, codeNumberingMissing, validate.SeverityWarning) {
		t.Fatalf("expected numbering-missing warning, got: %v", r)
	}
}

// A header reference to a missing relationship is a dangling-rel error.
func TestValidate_HeaderRefDangling(t *testing.T) {
	body := &oxml.CT_Body{SectPr: &oxml.CT_SectPr{
		HeaderReference: []*oxml.CT_HdrFtrRef{{Type: "default", RID: "rId99"}},
	}}
	d := &Document{document: &oxml.CT_Document{Body: body}}
	if r := d.Validate(); !hasCode(r, validate.CodeDanglingRel, validate.SeverityError) {
		t.Fatalf("expected dangling-rel error for missing header rel, got: %v", r)
	}
}
