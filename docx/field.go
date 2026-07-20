package docx

import (
	"strings"

	"github.com/mgilbir/spine/docx/internal/oxml"
)

// FieldType is a Word field instruction for Paragraph.AddField. The
// predefined constants cover standard document furniture; any other value is
// passed through verbatim as the field instruction, so callers can emit
// further field types (e.g. "DATE \\@ \"yyyy-MM-dd\"") without new API.
type FieldType string

const (
	// FieldPage renders the current page number ({ PAGE }).
	FieldPage FieldType = "PAGE"
	// FieldNumPages renders the total page count ({ NUMPAGES }).
	FieldNumPages FieldType = "NUMPAGES"
)

// AddField appends a simple field (w:fldSimple) with the given instruction to
// the paragraph, together with a cached-result run that Word replaces when it
// recalculates fields (page fields recalculate automatically on
// repagination). The returned Run holds the cached result; format it to style
// the field's rendered text:
//
//	p := footer.AddParagraph()
//	p.AddText("Page ")
//	p.AddField(docx.FieldPage)
//	p.AddText(" of ")
//	p.AddField(docx.FieldNumPages)
func (p *Paragraph) AddField(t FieldType) *Run {
	// Word brackets instructions with spaces; keep that convention so the
	// instruction parses identically to fields Word itself inserts.
	fld := &oxml.CT_SimpleField{Instr: " " + string(t) + " "}
	result := &oxml.CT_R{}
	result.SetTexts([]*oxml.CT_Text{{Space: "preserve", Text: "1"}})
	fld.R = append(fld.R, result)
	p.p.AppendFldSimple(fld)
	return &Run{paragraph: p, r: result}
}

// AddMergeField appends a MERGEFIELD simple field (w:fldSimple) that inserts
// the named data-source column when the document is merged, together with a
// placeholder result run showing «name» until Word performs the merge. The
// returned Run holds the placeholder result; format it to style the merged
// value. A name containing whitespace is quoted in the field instruction so it
// parses as a single field argument, matching what Word writes.
//
//	p := doc.AddParagraph()
//	p.AddText("Dear ")
//	p.AddMergeField("FirstName")
//	p.AddText(",")
func (p *Paragraph) AddMergeField(name string) *Run {
	arg := name
	if strings.ContainsAny(name, " \t\r\n") {
		arg = `"` + name + `"`
	}
	fld := &oxml.CT_SimpleField{Instr: " MERGEFIELD " + arg + ` \* MERGEFORMAT `}
	result := &oxml.CT_R{}
	result.SetTexts([]*oxml.CT_Text{{Space: "preserve", Text: "«" + name + "»"}})
	fld.R = append(fld.R, result)
	p.p.AppendFldSimple(fld)
	return &Run{paragraph: p, r: result}
}

// AddText appends a new run containing text to the paragraph and returns it.
// It is shorthand for AddRun followed by SetText.
func (p *Paragraph) AddText(text string) *Run {
	r := p.AddRun()
	r.SetText(text)
	return r
}
