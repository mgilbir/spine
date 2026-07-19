package docx

import (
	"fmt"
	"strconv"
	"time"

	"github.com/mgilbir/spine/docx/internal/oxml"
)

// revisionDateFormat is the ISO-8601 form Word records in a revision's w:date
// attribute (UTC, second precision, "Z" suffix), e.g. 2021-01-02T03:04:05Z.
const revisionDateFormat = "2006-01-02T15:04:05Z"

// formatRevisionDate renders t as a w:date attribute value in UTC.
func formatRevisionDate(t time.Time) string {
	return t.UTC().Format(revisionDateFormat)
}

// nextRevisionID returns the next unused revision id (w:id) for the document,
// monotonically increasing across the document's lifetime. The first call scans
// the body for the highest existing tracked-change id so authored revisions
// never collide with ones already present.
func (d *Document) nextRevisionID() int {
	if !d.revisionIDInit {
		if d.document != nil {
			d.revisionIDVal = oxml.MaxRevisionID(d.document.Body)
		}
		d.revisionIDInit = true
	}
	d.revisionIDVal++
	return d.revisionIDVal
}

// --- Authoring tracked changes ---

// AddInsertedRun appends a tracked insertion to the paragraph: a run carrying
// text wrapped in a w:ins element attributed to author, dated to the current
// time (UTC). The returned Run wraps the inserted run, so the caller can format
// it further (bold, color, ...). The insertion is enumerated by
// Document.Revisions and transformed by Accept/Reject: accepting keeps the text
// as a normal run, rejecting removes it. For deterministic output (tests), use
// AddInsertedRunWithDate.
func (p *Paragraph) AddInsertedRun(author, text string) *Run {
	return p.AddInsertedRunWithDate(author, text, time.Now())
}

// AddInsertedRunWithDate is AddInsertedRun with an explicit revision timestamp,
// recorded in the w:date attribute (converted to UTC). Passing a fixed date
// makes the emitted markup deterministic.
func (p *Paragraph) AddInsertedRunWithDate(author, text string, date time.Time) *Run {
	r := &oxml.CT_R{T: []*oxml.CT_Text{{Space: "preserve", Text: text}}}
	block := &oxml.CT_RunTrackChange{
		Id:     strconv.Itoa(p.document.nextRevisionID()),
		Author: author,
		Date:   formatRevisionDate(date),
	}
	block.AppendR(r)
	p.p.AppendIns(block)
	return &Run{paragraph: p, r: r}
}

// MarkInserted wraps an existing run in a tracked insertion (w:ins) attributed
// to author, dated to the current time (UTC). The run must be a top-level run of
// its paragraph (as returned by Paragraph.Runs or Paragraph.AddRun). It returns
// the run so calls can be chained. The result reads back through
// Document.Revisions and is transformed by Accept/Reject. Use MarkInsertedWithDate
// for a fixed timestamp.
func (r *Run) MarkInserted(author string) *Run {
	return r.MarkInsertedWithDate(author, time.Now())
}

// MarkInsertedWithDate is MarkInserted with an explicit revision timestamp
// (recorded in UTC), for deterministic output.
func (r *Run) MarkInsertedWithDate(author string, date time.Time) *Run {
	id := strconv.Itoa(r.paragraph.document.nextRevisionID())
	oxml.WrapRunInsertion(r.paragraph.p, r.r, id, author, formatRevisionDate(date))
	return r
}

// MarkDeleted wraps an existing run in a tracked deletion (w:del) attributed to
// author, dated to the current time (UTC), converting the run's text (w:t) to
// deletion text (w:delText). The run must be a top-level run of its paragraph.
// It returns the run so calls can be chained. The result reads back through
// Document.Revisions and is transformed by Accept/Reject: accepting removes the
// text, rejecting restores it as a normal run. Use MarkDeletedWithDate for a
// fixed timestamp.
func (r *Run) MarkDeleted(author string) *Run {
	return r.MarkDeletedWithDate(author, time.Now())
}

// MarkDeletedWithDate is MarkDeleted with an explicit revision timestamp
// (recorded in UTC), for deterministic output.
func (r *Run) MarkDeletedWithDate(author string, date time.Time) *Run {
	id := strconv.Itoa(r.paragraph.document.nextRevisionID())
	oxml.WrapRunDeletion(r.paragraph.p, r.r, id, author, formatRevisionDate(date))
	return r
}

// RevisionType names the kind of a tracked change (a Word revision).
type RevisionType string

const (
	// RevisionInsertion is inserted content (w:ins). Accept keeps it as normal
	// text; Reject removes it.
	RevisionInsertion RevisionType = "insertion"
	// RevisionDeletion is deleted content (w:del/w:delText). Accept removes it;
	// Reject restores it as normal text.
	RevisionDeletion RevisionType = "deletion"
	// RevisionRunFormat is a run-property change (w:rPrChange). Accept keeps the
	// new formatting; Reject reverts to the recorded old formatting.
	RevisionRunFormat RevisionType = "runFormat"
	// RevisionParagraphFormat is a paragraph-property change (w:pPrChange).
	// Accept keeps the new properties; Reject reverts to the recorded old ones.
	RevisionParagraphFormat RevisionType = "paragraphFormat"

	// The following revision types are read-only: Revisions reports them, but
	// Accept and Reject return an error because transforming them safely is out
	// of scope. They are preserved byte-for-byte across a round trip.

	// RevisionSectionFormat is a section-property change (w:sectPrChange).
	RevisionSectionFormat RevisionType = "sectionFormat"
	// RevisionTableFormat is a table-property change (w:tblPrChange).
	RevisionTableFormat RevisionType = "tableFormat"
	// RevisionRowFormat is a table-row-property change (w:trPrChange).
	RevisionRowFormat RevisionType = "rowFormat"
	// RevisionCellFormat is a table-cell-property change (w:tcPrChange).
	RevisionCellFormat RevisionType = "cellFormat"
	// RevisionRowInsertion is an inserted table row (w:trPr/w:ins).
	RevisionRowInsertion RevisionType = "rowInsertion"
	// RevisionRowDeletion is a deleted table row (w:trPr/w:del).
	RevisionRowDeletion RevisionType = "rowDeletion"
	// RevisionCellInsertion is an inserted table cell (w:tcPr/w:cellIns).
	RevisionCellInsertion RevisionType = "cellInsertion"
	// RevisionCellDeletion is a deleted table cell (w:tcPr/w:cellDel).
	RevisionCellDeletion RevisionType = "cellDeletion"
	// RevisionCellMerge is a cell-merge revision (w:tcPr/w:cellMerge).
	RevisionCellMerge RevisionType = "cellMerge"
)

// Revision is a single tracked change in a document: an insertion, deletion, or
// property change made with Word's Track Changes turned on. Read its metadata
// with Author, Date, Type, and Text; apply or discard it with Accept or Reject.
//
// Revisions are enumerated over the main document body, including content nested
// in tables, hyperlinks, fields, and structured document tags. Tracked moves
// (w:moveFrom/w:moveTo) are preserved across a round trip but are not enumerated
// or transformed.
type Revision struct {
	kind   RevisionType
	author string
	date   string
	text   string

	// Transform targets. Exactly the set relevant to kind is populated.
	container oxml.RevContainer // holds block, for insertion/deletion
	block     *oxml.CT_RunTrackChange
	run       *oxml.CT_R // for RevisionRunFormat
	para      *oxml.CT_P // for RevisionParagraphFormat
}

// Author returns the author recorded on the revision (w:author), or an empty
// string when none was recorded.
func (r *Revision) Author() string { return r.author }

// Date returns the timestamp recorded on the revision (w:date, an ISO-8601
// string), or an empty string when none was recorded.
func (r *Revision) Date() string { return r.date }

// Type returns the revision's kind.
func (r *Revision) Type() RevisionType { return r.kind }

// Text returns the text affected by the revision: the inserted or deleted text
// for insertions and deletions, and the run or paragraph text for property
// changes. Property changes on tables/rows/cells/sections report an empty
// string.
func (r *Revision) Text() string { return r.text }

// Editable reports whether Accept and Reject can transform this revision.
// Read-only revision types (section/table/row/cell property changes, cell
// merges, row/cell insertions and deletions) return false.
func (r *Revision) Editable() bool {
	switch r.kind {
	case RevisionInsertion, RevisionDeletion, RevisionRunFormat, RevisionParagraphFormat:
		return true
	}
	return false
}

// Accept applies the revision to the document, transforming its content:
// an insertion becomes normal text, a deletion is removed, and a property
// change keeps its new properties (dropping the change record). It returns an
// error for a read-only revision type (see Editable).
func (r *Revision) Accept() error {
	switch r.kind {
	case RevisionInsertion:
		oxml.AcceptInsertion(r.container, r.block)
	case RevisionDeletion:
		oxml.AcceptDeletion(r.container, r.block)
	case RevisionRunFormat:
		oxml.AcceptRunFormat(r.run)
	case RevisionParagraphFormat:
		oxml.AcceptParagraphFormat(r.para)
	default:
		return fmt.Errorf("docx: revision type %q is read-only and cannot be accepted", r.kind)
	}
	return nil
}

// Reject discards the revision, transforming its content: an insertion is
// removed, a deletion is restored as normal text, and a property change reverts
// to the recorded old properties (dropping the change record). It returns an
// error for a read-only revision type (see Editable).
func (r *Revision) Reject() error {
	switch r.kind {
	case RevisionInsertion:
		oxml.RejectInsertion(r.container, r.block)
	case RevisionDeletion:
		oxml.RejectDeletion(r.container, r.block)
	case RevisionRunFormat:
		oxml.RejectRunFormat(r.run)
	case RevisionParagraphFormat:
		oxml.RejectParagraphFormat(r.para)
	default:
		return fmt.Errorf("docx: revision type %q is read-only and cannot be rejected", r.kind)
	}
	return nil
}

// Revisions returns every tracked change in the document body in document
// order, descending into tables, hyperlinks, fields, and structured document
// tags. Insertions, deletions, and run/paragraph property changes are editable
// (Accept/Reject transform the document); table, row, cell, and section
// revisions are reported read-only.
//
// The returned Revision values reference live document structures. Accept or
// Reject on one, and any editing between enumerating and applying, can
// invalidate others in the slice; re-read Revisions after a batch of edits.
func (d *Document) Revisions() []*Revision {
	if d.document == nil || d.document.Body == nil {
		return nil
	}
	var out []*Revision
	for _, p := range d.document.Body.AllParagraphs() {
		for _, rr := range oxml.CollectParagraphRevisions(p) {
			out = append(out, newRevision(rr))
		}
	}
	for _, tbl := range d.document.Body.Tbl {
		out = collectTableRevisions(out, tbl)
	}
	if sc := d.document.Body.SectPr; sc != nil && sc.SectPrChange != nil {
		out = append(out, &Revision{
			kind:   RevisionSectionFormat,
			author: sc.SectPrChange.Author,
			date:   sc.SectPrChange.Date,
		})
	}
	return out
}

// AcceptAllRevisions accepts every editable revision in the document body:
// insertions become normal text, deletions are removed, and run/paragraph
// property changes keep their new properties. Read-only revision types
// (table/row/cell/section changes, cell merges) are left untouched. It returns
// an error only if a future transform reports one; today it always succeeds.
func (d *Document) AcceptAllRevisions() error {
	if d.document == nil || d.document.Body == nil {
		return nil
	}
	for _, p := range d.document.Body.AllParagraphs() {
		oxml.AcceptParagraphFormat(p)
		oxml.AcceptAllInContainer(p)
	}
	return nil
}

// RejectAllRevisions rejects every editable revision in the document body:
// insertions are removed, deletions restored as normal text, and run/paragraph
// property changes reverted to their recorded old properties. Read-only
// revision types are left untouched. It returns an error only if a future
// transform reports one; today it always succeeds.
func (d *Document) RejectAllRevisions() error {
	if d.document == nil || d.document.Body == nil {
		return nil
	}
	for _, p := range d.document.Body.AllParagraphs() {
		oxml.RejectParagraphFormat(p)
		oxml.RejectAllInContainer(p)
	}
	return nil
}

// newRevision maps an oxml.RawRevision enumerated from paragraph content to the
// public Revision type.
func newRevision(rr oxml.RawRevision) *Revision {
	rev := &Revision{
		author:    rr.Author,
		date:      rr.Date,
		text:      rr.Text,
		container: rr.Container,
		block:     rr.Block,
		run:       rr.Run,
		para:      rr.Para,
	}
	switch rr.Kind {
	case oxml.RevKindInsertion:
		rev.kind = RevisionInsertion
	case oxml.RevKindDeletion:
		rev.kind = RevisionDeletion
	case oxml.RevKindRunFormat:
		rev.kind = RevisionRunFormat
	case oxml.RevKindParagraphFormat:
		rev.kind = RevisionParagraphFormat
	}
	return rev
}

// collectTableRevisions appends a table's read-only structural revisions
// (table/row/cell property changes, row/cell insertions and deletions, and cell
// merges), descending into nested tables.
func collectTableRevisions(out []*Revision, tbl *oxml.CT_Tbl) []*Revision {
	if tbl == nil {
		return out
	}
	if tbl.TblPr != nil && tbl.TblPr.TblPrChange != nil {
		out = append(out, &Revision{
			kind:   RevisionTableFormat,
			author: tbl.TblPr.TblPrChange.Author,
			date:   tbl.TblPr.TblPrChange.Date,
		})
	}
	for _, tr := range tbl.Tr {
		if tr == nil {
			continue
		}
		if tr.TrPr != nil {
			if tr.TrPr.Ins != nil {
				out = append(out, &Revision{kind: RevisionRowInsertion, author: tr.TrPr.Ins.Author, date: tr.TrPr.Ins.Date})
			}
			if tr.TrPr.Del != nil {
				out = append(out, &Revision{kind: RevisionRowDeletion, author: tr.TrPr.Del.Author, date: tr.TrPr.Del.Date})
			}
			if tr.TrPr.TrPrChange != nil {
				out = append(out, &Revision{kind: RevisionRowFormat, author: tr.TrPr.TrPrChange.Author, date: tr.TrPr.TrPrChange.Date})
			}
		}
		for _, tc := range tr.Tc {
			if tc == nil {
				continue
			}
			if tc.TcPr != nil {
				if tc.TcPr.CellIns != nil {
					out = append(out, &Revision{kind: RevisionCellInsertion, author: tc.TcPr.CellIns.Author, date: tc.TcPr.CellIns.Date})
				}
				if tc.TcPr.CellDel != nil {
					out = append(out, &Revision{kind: RevisionCellDeletion, author: tc.TcPr.CellDel.Author, date: tc.TcPr.CellDel.Date})
				}
				if tc.TcPr.CellMerge != nil {
					out = append(out, &Revision{kind: RevisionCellMerge, author: tc.TcPr.CellMerge.Author, date: tc.TcPr.CellMerge.Date})
				}
				if tc.TcPr.TcPrChange != nil {
					out = append(out, &Revision{kind: RevisionCellFormat, author: tc.TcPr.TcPrChange.Author, date: tc.TcPr.TcPrChange.Date})
				}
			}
			for _, nested := range tc.Tbl {
				out = collectTableRevisions(out, nested)
			}
		}
	}
	return out
}
