package docx

import (
	"strconv"

	"github.com/mgilbir/spine/docx/internal/oxml"
)

// Footnote is a footnote or endnote. The ID/Text/Paragraphs accessors work on
// both note kinds; Document.Footnotes and Document.Endnotes return the two
// families separately, and Run.AddFootnote / Run.AddEndnote anchor them.
type Footnote struct {
	document *Document
	note     *oxml.CT_FtnEdn
}

// ID returns the note's w:id (the value a footnoteReference/endnoteReference
// points at).
func (f *Footnote) ID() string { return f.note.Id }

// Text returns the note body text, with paragraphs joined by newlines.
func (f *Footnote) Text() string { return f.note.Text() }

// Paragraphs returns the note body paragraphs.
func (f *Footnote) Paragraphs() []*Paragraph {
	out := make([]*Paragraph, 0, len(f.note.P))
	for _, p := range f.note.P {
		out = append(out, &Paragraph{document: f.document, p: p})
	}
	return out
}

// isSeparatorNote reports whether a note is one of the mandatory separator /
// continuationSeparator notes Word keeps at the head of the part (not a user
// note).
func isSeparatorNote(n *oxml.CT_FtnEdn) bool {
	return n != nil && (n.Type == "separator" || n.Type == "continuationSeparator")
}

// Footnotes returns the document's footnotes in document (part) order,
// excluding the mandatory separator and continuationSeparator notes.
func (d *Document) Footnotes() []*Footnote {
	if d.footnotes == nil {
		return nil
	}
	var out []*Footnote
	for _, n := range d.footnotes.Footnote {
		if isSeparatorNote(n) {
			continue
		}
		out = append(out, &Footnote{document: d, note: n})
	}
	return out
}

// Endnotes returns the document's endnotes in document (part) order, excluding
// the mandatory separator and continuationSeparator notes.
func (d *Document) Endnotes() []*Footnote {
	if d.endnotes == nil {
		return nil
	}
	var out []*Footnote
	for _, n := range d.endnotes.Endnote {
		if isSeparatorNote(n) {
			continue
		}
		out = append(out, &Footnote{document: d, note: n})
	}
	return out
}

// --- write ---

// AddFootnote inserts a footnote reference in the run stream right after this
// run and appends the note (with the given body text) to word/footnotes.xml,
// creating that part — with its relationship, content-type override, and the
// mandatory separator notes — if the document did not already have it.
func (r *Run) AddFootnote(text string) *Footnote {
	d := r.paragraph.document
	d.ensureFootnotes()
	id := strconv.Itoa(d.footnotes.MaxID() + 1)

	note := &oxml.CT_FtnEdn{Id: id}
	note.AppendP(oxml.NewNoteBody("footnoteRef", "FootnoteText", "FootnoteReference", text))
	d.footnotes.Footnote = append(d.footnotes.Footnote, note)

	r.anchorNoteRef(id, false)
	d.footnotesModified = true
	return &Footnote{document: d, note: note}
}

// AddEndnote inserts an endnote reference in the run stream right after this run
// and appends the note to word/endnotes.xml, creating that part if absent (see
// AddFootnote).
func (r *Run) AddEndnote(text string) *Footnote {
	d := r.paragraph.document
	d.ensureEndnotes()
	id := strconv.Itoa(d.endnotes.MaxID() + 1)

	note := &oxml.CT_FtnEdn{Id: id}
	note.AppendP(oxml.NewNoteBody("endnoteRef", "EndnoteText", "EndnoteReference", text))
	d.endnotes.Endnote = append(d.endnotes.Endnote, note)

	r.anchorNoteRef(id, true)
	d.endnotesModified = true
	return &Footnote{document: d, note: note}
}

// anchorNoteRef places a footnote/endnote reference for id in the run stream.
// It prefers a dedicated reference run (styled FootnoteReference/
// EndnoteReference) inserted right after this run, matching Word; if this run is
// not a direct child of its paragraph (e.g. nested in a hyperlink) the
// reference is appended to this run instead so it is never dropped.
func (r *Run) anchorNoteRef(id string, endnote bool) {
	style := "FootnoteReference"
	if endnote {
		style = "EndnoteReference"
	}
	refRun := &oxml.CT_R{RPr: &oxml.CT_RPr{RStyle: &oxml.CT_String{Val: style}}}
	if endnote {
		refRun.AppendEndnoteRef(&oxml.CT_FtnEdnRef{Id: id})
	} else {
		refRun.AppendFtnRef(&oxml.CT_FtnEdnRef{Id: id})
	}
	if r.paragraph.p.InsertRunAfter(r.r, refRun) {
		return
	}
	if endnote {
		r.r.AppendEndnoteRef(&oxml.CT_FtnEdnRef{Id: id})
	} else {
		r.r.AppendFtnRef(&oxml.CT_FtnEdnRef{Id: id})
	}
}

// ensureFootnotes creates the footnotes model with the mandatory separator
// notes when the document has none yet, so the write API works on a created or
// opened document without a footnotes part.
func (d *Document) ensureFootnotes() {
	if d.footnotes == nil {
		d.footnotes = &oxml.CT_Footnotes{Footnote: oxml.StandardFootnotes()}
	}
}

// ensureEndnotes is the endnote counterpart of ensureFootnotes.
func (d *Document) ensureEndnotes() {
	if d.endnotes == nil {
		d.endnotes = &oxml.CT_Endnotes{Endnote: oxml.StandardEndnotes()}
	}
}
