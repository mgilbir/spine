package docx

import (
	"fmt"

	"github.com/mgilbir/spine/docx/internal/oxml"
)

// TOCOptions configures AddTableOfContents. The zero value builds a TOC over
// heading levels 1-3.
type TOCOptions struct {
	// MinLevel is the first heading (outline) level included, 1-9.
	// Zero means 1.
	MinLevel int
	// MaxLevel is the last heading (outline) level included, 1-9.
	// Zero means 3.
	MaxLevel int
}

// AddTableOfContents appends a table of contents built from the document's
// heading styles (see AddHeading), wrapped in a structured document tag so
// Word offers its "Update Table" affordance.
//
// The TOC is a Word field: its entries are computed by Word, not by this
// library. The field is marked dirty so Word recalculates it when the
// document is opened (depending on settings, Word may prompt before
// updating); until then the placeholder paragraph is shown.
func (d *Document) AddTableOfContents(opts TOCOptions) error {
	min, max := opts.MinLevel, opts.MaxLevel
	if min == 0 {
		min = 1
	}
	if max == 0 {
		max = 3
	}
	if min < 1 || min > 9 || max < 1 || max > 9 || min > max {
		return fmt.Errorf("docx: AddTableOfContents: heading levels must satisfy 1 <= MinLevel <= MaxLevel <= 9, got %d..%d", min, max)
	}

	// The complex-field form Word itself writes: begin (dirty), the TOC
	// instruction, separate, the cached result (a placeholder until Word
	// computes the entries), end.
	// \o defines the heading-level range, \h makes entries hyperlinks,
	// \z hides tab leaders in web view, \u uses applied outline levels.
	instr := fmt.Sprintf(` TOC \o "%d-%d" \h \z \u `, min, max)

	begin := &oxml.CT_R{}
	begin.AppendFldChar(&oxml.CT_FldChar{FldCharType: "begin", Dirty: "true"})

	instrRun := &oxml.CT_R{}
	instrRun.AppendInstrText(&oxml.CT_Text{Space: "preserve", Text: instr})

	separate := &oxml.CT_R{}
	separate.AppendFldChar(&oxml.CT_FldChar{FldCharType: "separate"})

	placeholder := &oxml.CT_R{}
	placeholder.SetTexts([]*oxml.CT_Text{{
		Space: "preserve",
		Text:  "Table of contents (update fields to populate).",
	}})

	end := &oxml.CT_R{}
	end.AppendFldChar(&oxml.CT_FldChar{FldCharType: "end"})

	fieldPara := &oxml.CT_P{}
	fieldPara.AppendR(begin)
	fieldPara.AppendR(instrRun)
	fieldPara.AppendR(separate)
	fieldPara.AppendR(placeholder)
	fieldPara.AppendR(end)

	content := &oxml.CT_SdtContentBlock{}
	content.AppendP(fieldPara)

	sdt := &oxml.CT_SdtBlock{
		// The docPartObj gallery marks this SDT as a Table of Contents
		// building block, which is what makes Word show the TOC frame with
		// its update control.
		SdtPr: &oxml.CT_SdtPr{RawContent: []byte(
			`<w:docPartObj><w:docPartGallery w:val="Table of Contents"/><w:docPartUnique/></w:docPartObj>`,
		)},
		SdtContent: content,
	}

	if d.document.Body == nil {
		d.document.Body = &oxml.CT_Body{}
	}
	d.document.Body.AppendSdtBlock(sdt)
	return nil
}
