package docx

import "github.com/mgilbir/spine/docx/internal/oxml"

// The mutation accessors below are the one place a feature mutator reaches the
// underlying paragraph or run model.
//
// A header or footer opened from a package round-trips as preserved raw bytes
// unless its part is flagged modified, so an edit made through a live handle
// into such a part is silently dropped on save unless the mutator calls touch()
// (C266). Property setters were covered because they all funnel through
// ensurePPr/ensureRPr, and the content mutators on Paragraph and Run were
// covered one by one — but every mutator the *feature* APIs added
// (AddInsertedRun, MarkInserted/MarkDeleted, the tracked moves, AddField,
// AddMergeField, AddCitation, AddFormField, AddContentControl, AddComment,
// AddFootnote/AddEndnote, AddSignatureLine, RemoveListStyle) forgot it, and each
// new feature API had to remember independently (C406).
//
// Some of those were latent when the audit was written and are not any more:
// PR #235 added Document.Headers()/Footers() and Header.Paragraphs(), which
// hands out live header paragraphs, so `doc.Header(sec, HeaderDefault).
// Paragraphs()[0].AddField(FieldPage)` is now an ordinary call. Worse, the
// companion part is *not* gated the same way: Run.AddFootnote appends the note
// to footnotes.xml (which is regenerated) and the reference into the run (which
// is masked), so the document ends up with an orphan note and a lost reference.
//
// Going through mut() rather than the field makes the flagging structural: a
// mutator that reaches p.mut() cannot forget, and one that reaches p.p is
// declaring itself a read.

// mut returns the paragraph's underlying model, flagging the header or footer
// part that owns it for regeneration on save. Every mutator must reach the
// model through it; direct use of p.p is for reads only.
func (p *Paragraph) mut() *oxml.CT_P {
	p.touch()
	return p.p
}

// mut returns the run's underlying model, flagging the owning header or footer
// part for regeneration on save (see Paragraph.mut).
func (r *Run) mut() *oxml.CT_R {
	r.touch()
	return r.r
}

// mutParagraph returns the model of the paragraph containing this run, flagging
// the owning header or footer part. Run-level mutators that splice content into
// the run's *parent* (a footnote reference run, a tracked-change wrapper) use it
// rather than reaching through r.paragraph.p.
func (r *Run) mutParagraph() *oxml.CT_P {
	r.touch()
	return r.paragraph.p
}
