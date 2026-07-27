package docx

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
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
// for the highest existing tracked-change id so authored revisions never
// collide with ones already present.
//
// The scan covers the body *and* every header and footer part, plus the
// paragraph-level w:sectPrChange records that mark mid-document section breaks.
// Scanning the body alone meant that authoring into a document whose header
// carries tracked changes — which Revisions() enumerates, so they are visibly
// part of the same id space — reused those ids (C496, the revision twin of the
// bookmark C408).
func (d *Document) nextRevisionID() int {
	if !d.revisionIDInit {
		d.revisionIDVal = d.maxRevisionID()
		d.revisionIDInit = true
	}
	d.revisionIDVal++
	return d.revisionIDVal
}

// maxRevisionID returns the highest numeric tracked-change id anywhere in the
// document: the body (including its structural and section revisions), the
// paragraph-level section-property changes the body scan does not reach, and
// every header and footer part.
func (d *Document) maxRevisionID() int {
	maxID := 0
	consider := func(s string) {
		if n, err := strconv.Atoi(strings.TrimSpace(s)); err == nil && n > maxID {
			maxID = n
		}
	}
	if d.doc() != nil && d.doc().Body != nil {
		maxID = oxml.MaxRevisionID(d.doc().Body)
		for _, p := range d.doc().Body.AllParagraphs() {
			if p != nil && p.PPr != nil && p.PPr.SectPr != nil && p.PPr.SectPr.SectPrChange != nil {
				consider(p.PPr.SectPr.SectPrChange.Id)
			}
		}
	}
	for _, hp := range d.sortedHeaderParts() {
		if hp != nil && hp.hdr != nil {
			consider(strconv.Itoa(oxml.MaxHdrFtrRevisionID(hp.hdr)))
		}
	}
	for _, fp := range d.sortedFooterParts() {
		if fp != nil && fp.ftr != nil {
			consider(strconv.Itoa(oxml.MaxHdrFtrRevisionID(fp.ftr)))
		}
	}
	return maxID
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
	p.mut().AppendIns(block)
	return &Run{paragraph: p, r: r}
}

// MarkInserted wraps an existing run in a tracked insertion (w:ins) attributed
// to author, dated to the current time (UTC). The run must be a top-level run of
// its paragraph (as returned by Paragraph.Runs or Paragraph.AddRun); on a run
// that is not a direct paragraph child (for example one from Hyperlink.Runs) the
// call is a no-op that leaves the run unchanged. It returns the run so calls can
// be chained. The result reads back through Document.Revisions and is transformed
// by Accept/Reject. Use MarkInsertedWithDate for a fixed timestamp.
func (r *Run) MarkInserted(author string) *Run {
	return r.MarkInsertedWithDate(author, time.Now())
}

// MarkInsertedWithDate is MarkInserted with an explicit revision timestamp
// (recorded in UTC), for deterministic output.
func (r *Run) MarkInsertedWithDate(author string, date time.Time) *Run {
	id := strconv.Itoa(r.paragraph.document.nextRevisionID())
	oxml.WrapRunInsertion(r.mutParagraph(), r.r, id, author, formatRevisionDate(date))
	return r
}

// MarkDeleted wraps an existing run in a tracked deletion (w:del) attributed to
// author, dated to the current time (UTC), converting the run's text (w:t) to
// deletion text (w:delText). The run must be a top-level run of its paragraph; on
// a run that is not a direct paragraph child (for example one from
// Hyperlink.Runs) the call is a no-op that leaves the run unchanged — its text is
// not converted to w:delText, so no schema-invalid w:delText is emitted outside a
// w:del. It returns the run so calls can be chained. The result reads back through
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
	oxml.WrapRunDeletion(r.mutParagraph(), r.r, id, author, formatRevisionDate(date))
	return r
}

// AddMoveFromRun appends the source half of a tracked move to the paragraph: a
// run carrying text wrapped in w:moveFrom and bracketed by move range markers
// carrying name, all attributed to author and dated to the current time (UTC).
// Author the matching destination with AddMoveToRun using the same name. The
// move reads back through Document.Revisions (as RevisionMoveFrom) and is
// transformed by Accept/Reject. Use AddMoveFromRunWithDate for a fixed
// timestamp.
func (p *Paragraph) AddMoveFromRun(author, name, text string) {
	p.AddMoveFromRunWithDate(author, name, text, time.Now())
}

// AddMoveFromRunWithDate is AddMoveFromRun with an explicit revision timestamp
// (recorded in UTC), for deterministic output.
func (p *Paragraph) AddMoveFromRunWithDate(author, name, text string, date time.Time) {
	p.addMove("moveFrom", author, name, text, date)
}

// AddMoveToRun appends the destination half of a tracked move to the paragraph:
// a run carrying text wrapped in w:moveTo and bracketed by move range markers
// carrying name, attributed to author and dated to the current time (UTC). Pair
// it with AddMoveFromRun using the same name. The move reads back through
// Document.Revisions (as RevisionMoveTo) and is transformed by Accept/Reject.
// Use AddMoveToRunWithDate for a fixed timestamp.
func (p *Paragraph) AddMoveToRun(author, name, text string) {
	p.AddMoveToRunWithDate(author, name, text, time.Now())
}

// AddMoveToRunWithDate is AddMoveToRun with an explicit revision timestamp
// (recorded in UTC), for deterministic output.
func (p *Paragraph) AddMoveToRunWithDate(author, name, text string, date time.Time) {
	p.addMove("moveTo", author, name, text, date)
}

// addMove appends a tracked-move range marker, content wrapper, and closing
// range marker (kind is "moveFrom" or "moveTo") for a single run of text. The
// range markers share one revision id; the content wrapper gets its own.
func (p *Paragraph) addMove(kind, author, name, text string, date time.Time) {
	rangeID := strconv.Itoa(p.document.nextRevisionID())
	contentID := strconv.Itoa(p.document.nextRevisionID())
	ds := formatRevisionDate(date)
	r := &oxml.CT_R{T: []*oxml.CT_Text{{Space: "preserve", Text: text}}}
	cp := p.mut()
	cp.AppendRaw(oxml.NewMoveRangeStart(kind+"RangeStart", rangeID, author, ds, name))
	cp.AppendRaw(oxml.NewMoveBlock(kind, contentID, author, ds, r))
	cp.AppendRaw(oxml.NewMoveRangeEnd(kind+"RangeEnd", rangeID))
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
	// RevisionMoveFrom is the source half of a tracked move (w:moveFrom): the
	// content in the location text was moved away from. Accept drops it (the
	// text left this location); Reject restores it as normal text.
	RevisionMoveFrom RevisionType = "moveFrom"
	// RevisionMoveTo is the destination half of a tracked move (w:moveTo): the
	// content in the location text was moved to. Accept keeps it as normal text
	// (the text arrived here); Reject removes it.
	RevisionMoveTo RevisionType = "moveTo"

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
// Revisions are enumerated over the main document body and the header and footer
// parts, including content nested in tables, hyperlinks, fields, and structured
// document tags. Tracked moves (w:moveFrom/w:moveTo) are enumerated and
// transformed; their range markers (w:moveFromRangeStart, ...) are preserved but
// left in place across accept/reject.
type Revision struct {
	kind     RevisionType
	author   string
	date     string
	text     string
	moveName string

	// Transform targets. Exactly the set relevant to kind is populated.
	container oxml.RevContainer // holds block/moveRaw, for insertion/deletion/move
	block     *oxml.CT_RunTrackChange
	run       *oxml.CT_R               // for RevisionRunFormat
	para      *oxml.CT_P               // for RevisionParagraphFormat
	moveRaw   *oxml.CT_RawNamedElement // for RevisionMoveFrom/RevisionMoveTo
	notify    func()                   // marks the owning part modified (header/footer)
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

// MoveName returns the paired move name recorded on the enclosing range marker
// for a tracked-move revision (RevisionMoveFrom/RevisionMoveTo), linking the
// source and destination halves. It is empty for other revision types and when
// the name could not be resolved.
func (r *Revision) MoveName() string { return r.moveName }

// Editable reports whether Accept and Reject can transform this revision.
// Read-only revision types (section/table/row/cell property changes, cell
// merges, row/cell insertions and deletions) return false.
func (r *Revision) Editable() bool {
	switch r.kind {
	case RevisionInsertion, RevisionDeletion, RevisionRunFormat, RevisionParagraphFormat,
		RevisionMoveFrom, RevisionMoveTo:
		return true
	}
	return false
}

// Accept applies the revision to the document, transforming its content:
// an insertion becomes normal text, a deletion is removed, and a property
// change keeps its new properties (dropping the change record). It returns an
// error for a read-only revision type (see Editable), and ErrRevisionStale when
// the revision's content is no longer where it was enumerated.
func (r *Revision) Accept() error {
	var done bool
	switch r.kind {
	case RevisionInsertion:
		done = oxml.AcceptInsertion(r.container, r.block)
	case RevisionDeletion:
		done = oxml.AcceptDeletion(r.container, r.block)
	case RevisionRunFormat:
		done = r.hasRunChange()
		oxml.AcceptRunFormat(r.run)
	case RevisionParagraphFormat:
		done = r.hasParaChange()
		oxml.AcceptParagraphFormat(r.para)
	case RevisionMoveFrom:
		// Accepting a move drops the source content.
		done = oxml.DropMoveBlock(r.container, r.moveRaw)
	case RevisionMoveTo:
		// Accepting a move keeps the destination content as normal text.
		done = oxml.UnwrapMoveBlock(r.container, r.moveRaw)
	default:
		return fmt.Errorf("docx: revision type %q is read-only and cannot be accepted", r.kind)
	}
	return r.finish(done)
}

// Reject discards the revision, transforming its content: an insertion is
// removed, a deletion is restored as normal text, and a property change reverts
// to the recorded old properties (dropping the change record). It returns an
// error for a read-only revision type (see Editable), and ErrRevisionStale when
// the revision's content is no longer where it was enumerated.
func (r *Revision) Reject() error {
	var done bool
	switch r.kind {
	case RevisionInsertion:
		done = oxml.RejectInsertion(r.container, r.block)
	case RevisionDeletion:
		done = oxml.RejectDeletion(r.container, r.block)
	case RevisionRunFormat:
		done = r.hasRunChange()
		oxml.RejectRunFormat(r.run)
	case RevisionParagraphFormat:
		done = r.hasParaChange()
		oxml.RejectParagraphFormat(r.para)
	case RevisionMoveFrom:
		// Rejecting a move restores the source content as normal text.
		done = oxml.UnwrapMoveBlock(r.container, r.moveRaw)
	case RevisionMoveTo:
		// Rejecting a move drops the destination content.
		done = oxml.DropMoveBlock(r.container, r.moveRaw)
	default:
		return fmt.Errorf("docx: revision type %q is read-only and cannot be rejected", r.kind)
	}
	return r.finish(done)
}

// finish reports the outcome of a transform: on success it flags the owning
// part modified, on failure it reports the revision stale and flags nothing.
//
// The transforms return false when the revision's block is no longer a direct
// child of its container — exactly the invalidation the type's godoc warns
// about, since accepting one revision rebuilds its container and detaches the
// handles of the others enumerated alongside it. Discarding that bool made
// Accept() report success having done nothing, *and* flag a header part for
// regeneration on the strength of a no-op (C494).
func (r *Revision) finish(done bool) error {
	if !done {
		return ErrRevisionStale
	}
	r.markModified()
	return nil
}

// hasRunChange reports whether the run still carries the property-change record
// this revision refers to.
func (r *Revision) hasRunChange() bool {
	return r.run != nil && r.run.RPr != nil && r.run.RPr.RPrChange != nil
}

// hasParaChange reports whether the paragraph still carries the property-change
// record this revision refers to.
func (r *Revision) hasParaChange() bool {
	return r.para != nil && r.para.PPr != nil && r.para.PPr.PPrChange != nil
}

// markModified flags the revision's owning part (a header or footer) for
// regeneration on save. Body revisions carry no notifier: document.xml is always
// regenerated.
func (r *Revision) markModified() {
	if r.notify != nil {
		r.notify()
	}
}

// Revisions returns every tracked change in the document body and in every
// header and footer part, in document order, descending into tables,
// hyperlinks, fields, and structured document tags. Insertions, deletions,
// run/paragraph property changes, and tracked moves (w:moveFrom/w:moveTo) are
// editable (Accept/Reject transform the document); table, row, cell, and
// section revisions are reported read-only. Header and footer revisions follow
// the body revisions, ordered by part name.
//
// Structural revisions are enumerated through the same block descent that
// allocates their ids, so a table revision inside a block-level content control
// is reported (it used to be allocated an id it never surfaced), and a
// w:sectPrChange on a paragraph-level w:sectPr — any mid-document section break
// edited under track changes — is reported alongside the body-level one (C495).
//
// The returned Revision values reference live document structures. Accept or
// Reject on one, and any editing between enumerating and applying, can
// invalidate others in the slice; re-read Revisions after a batch of edits.
func (d *Document) Revisions() []*Revision {
	var out []*Revision
	if d.doc() != nil && d.doc().Body != nil {
		for _, p := range d.doc().Body.AllParagraphs() {
			out = collectParagraphAndMoves(out, p, nil)
		}
		out = appendStructuralRevisions(out, d.doc().Body.StructuralRevisions())
	}
	out = d.collectHdrFtrRevisions(out)
	return out
}

// appendStructuralRevisions maps the internal structural-revision records onto
// the public read-only Revision types.
func appendStructuralRevisions(out []*Revision, revs []oxml.StructRevision) []*Revision {
	for _, sr := range revs {
		out = append(out, &Revision{
			kind:   structRevisionType(sr.Kind),
			author: sr.Author,
			date:   sr.Date,
		})
	}
	return out
}

// structRevisionType maps an internal structural-revision kind to its public
// RevisionType.
func structRevisionType(k oxml.StructRevKind) RevisionType {
	switch k {
	case oxml.StructRevTableFormat:
		return RevisionTableFormat
	case oxml.StructRevRowInsertion:
		return RevisionRowInsertion
	case oxml.StructRevRowDeletion:
		return RevisionRowDeletion
	case oxml.StructRevRowFormat:
		return RevisionRowFormat
	case oxml.StructRevCellInsertion:
		return RevisionCellInsertion
	case oxml.StructRevCellDeletion:
		return RevisionCellDeletion
	case oxml.StructRevCellMerge:
		return RevisionCellMerge
	case oxml.StructRevCellFormat:
		return RevisionCellFormat
	}
	return RevisionSectionFormat
}

// collectParagraphAndMoves appends a paragraph's tracked changes followed by its
// tracked moves, tagging each with notify (nil for the body, a part-modified
// callback for headers and footers).
func collectParagraphAndMoves(out []*Revision, p *oxml.CT_P, notify func()) []*Revision {
	for _, rr := range oxml.CollectParagraphRevisions(p) {
		rev := newRevision(rr)
		rev.notify = notify
		out = append(out, rev)
	}
	for _, rm := range oxml.CollectContainerMoves(nil, p) {
		out = append(out, newMoveRevision(rm, notify))
	}
	return out
}

// collectHdrFtrRevisions appends the tracked changes in every header and footer
// part, in a deterministic part-name order. Accept/Reject on the returned
// revisions flags the owning part for regeneration on save.
func (d *Document) collectHdrFtrRevisions(out []*Revision) []*Revision {
	for _, name := range sortedKeys(d.headers) {
		hp := d.headers[name]
		if hp == nil || hp.hdr == nil {
			continue
		}
		partName := name
		notify := func() { d.markHdrFtrModified(partName) }
		for _, p := range hp.hdr.AllParagraphs() {
			out = collectParagraphAndMoves(out, p, notify)
		}
		out = appendStructuralRevisions(out, hp.hdr.StructuralRevisions())
	}
	for _, name := range sortedKeys(d.footers) {
		fp := d.footers[name]
		if fp == nil || fp.ftr == nil {
			continue
		}
		partName := name
		notify := func() { d.markHdrFtrModified(partName) }
		for _, p := range fp.ftr.AllParagraphs() {
			out = collectParagraphAndMoves(out, p, notify)
		}
		out = appendStructuralRevisions(out, fp.ftr.StructuralRevisions())
	}
	return out
}

// AcceptAllRevisions accepts every editable revision in the document body and
// in every header and footer: insertions become normal text, deletions are
// removed, run/paragraph property changes keep their new properties, and
// tracked moves are resolved (the source content is dropped, the destination
// kept as normal text; the w:moveFrom/w:moveTo range markers are preserved in
// place).
//
// Read-only revision types are left untouched: table, row and cell property
// changes, row and cell insertions and deletions, cell merges, and section
// property changes — including the w:sectPrChange on a mid-document section
// break. Those are reported by Revisions but never transformed here, so a
// document AcceptAllRevisions has processed can still enumerate revisions.
//
// It returns an error only if a future transform reports one; today it always
// succeeds.
func (d *Document) AcceptAllRevisions() error {
	if d.doc() != nil && d.doc().Body != nil {
		for _, p := range d.doc().Body.AllParagraphs() {
			oxml.AcceptParagraphFormat(p)
			oxml.AcceptAllInContainer(p)
		}
	}
	d.forEachHdrFtr(func(hf *oxml.CT_HdrFtr, notify func()) {
		if !hdrFtrHasRevisions(hf) {
			return
		}
		for _, p := range hf.AllParagraphs() {
			oxml.AcceptParagraphFormat(p)
			oxml.AcceptAllInContainer(p)
		}
		notify()
	})
	return nil
}

// RejectAllRevisions rejects every editable revision in the document body:
// insertions are removed, deletions restored as normal text, and run/paragraph
// property changes reverted to their recorded old properties. Read-only
// revision types are left untouched. It returns an error only if a future
// transform reports one; today it always succeeds.
func (d *Document) RejectAllRevisions() error {
	if d.doc() != nil && d.doc().Body != nil {
		for _, p := range d.doc().Body.AllParagraphs() {
			oxml.RejectParagraphFormat(p)
			oxml.RejectAllInContainer(p)
		}
	}
	d.forEachHdrFtr(func(hf *oxml.CT_HdrFtr, notify func()) {
		if !hdrFtrHasRevisions(hf) {
			return
		}
		for _, p := range hf.AllParagraphs() {
			oxml.RejectParagraphFormat(p)
			oxml.RejectAllInContainer(p)
		}
		notify()
	})
	return nil
}

// forEachHdrFtr calls fn for every parsed header and footer model in a
// deterministic part-name order, passing a callback that flags that part for
// regeneration on save.
func (d *Document) forEachHdrFtr(fn func(hf *oxml.CT_HdrFtr, notify func())) {
	for _, name := range sortedKeys(d.headers) {
		hp := d.headers[name]
		if hp == nil || hp.hdr == nil {
			continue
		}
		partName := name
		fn(hp.hdr, func() { d.markHdrFtrModified(partName) })
	}
	for _, name := range sortedKeys(d.footers) {
		fp := d.footers[name]
		if fp == nil || fp.ftr == nil {
			continue
		}
		partName := name
		fn(fp.ftr, func() { d.markHdrFtrModified(partName) })
	}
}

// hdrFtrHasRevisions reports whether a header or footer carries any editable
// tracked change (insertion, deletion, run/paragraph format, or move), so
// AcceptAll/RejectAll only flag parts they actually rewrite.
func hdrFtrHasRevisions(hf *oxml.CT_HdrFtr) bool {
	for _, p := range hf.AllParagraphs() {
		if len(oxml.CollectParagraphRevisions(p)) > 0 {
			return true
		}
		if len(oxml.CollectContainerMoves(nil, p)) > 0 {
			return true
		}
	}
	return false
}

// sortedKeys returns the keys of a string-keyed map in ascending order, giving
// header/footer traversal a deterministic order independent of map iteration.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
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

// newMoveRevision maps an oxml.RawMove enumerated from paragraph content to the
// public Revision type, tagging it with notify (nil for the body).
func newMoveRevision(rm oxml.RawMove, notify func()) *Revision {
	rev := &Revision{
		author:    rm.Author,
		date:      rm.Date,
		text:      rm.Text,
		moveName:  rm.Name,
		container: rm.Container,
		moveRaw:   rm.Raw,
		notify:    notify,
	}
	if rm.Kind == oxml.MoveTo {
		rev.kind = RevisionMoveTo
	} else {
		rev.kind = RevisionMoveFrom
	}
	return rev
}

