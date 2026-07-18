package oxml

// AppendFtnRef appends a footnote reference (w:footnoteReference) to the run,
// maintaining child order (see AppendDrawing).
func (r *CT_R) AppendFtnRef(ref *CT_FtnEdnRef) {
	r.backfillChildOrder()
	r.childOrder = append(r.childOrder, runChildRef{runChildFtnRef, len(r.FtnRef)})
	r.FtnRef = append(r.FtnRef, ref)
}

// AppendEndnoteRef appends an endnote reference (w:endnoteReference) to the run,
// maintaining child order (see AppendDrawing).
func (r *CT_R) AppendEndnoteRef(ref *CT_FtnEdnRef) {
	r.backfillChildOrder()
	r.childOrder = append(r.childOrder, runChildRef{runChildEndnoteRef, len(r.EndnoteRef)})
	r.EndnoteRef = append(r.EndnoteRef, ref)
}

// newNoteRefMarkRun builds the note body's leading run: the reference mark
// (w:footnoteRef or w:endnoteRef) styled with the given character style, exactly
// as Word emits it at the start of a note body.
func newNoteRefMarkRun(markLocal, styleID string) *CT_R {
	r := &CT_R{RPr: &CT_RPr{RStyle: &CT_String{Val: styleID}}}
	r.Raw = []*CT_RawNamedElement{{Local: markLocal, Space: NsWml}}
	r.childOrder = []runChildRef{{runChildRaw, 0}}
	return r
}

// NewNoteBody builds a note body paragraph: the reference-mark run followed by a
// run carrying the note text. markLocal is "footnoteRef" or "endnoteRef";
// paraStyle and refStyle are the paragraph and character style ids.
func NewNoteBody(markLocal, paraStyle, refStyle, text string) *CT_P {
	p := &CT_P{PPr: &CT_PPr{PStyle: &CT_String{Val: paraStyle}}}
	p.AppendR(newNoteRefMarkRun(markLocal, refStyle))
	p.AppendR(&CT_R{T: []*CT_Text{{Space: "preserve", Text: " " + text}}})
	return p
}

// newSeparatorNote builds one of the two mandatory separator notes Word writes
// at the head of a footnotes/endnotes part (w:separator or
// w:continuationSeparator inside a single run).
func newSeparatorNote(id, noteType, markLocal string) *CT_FtnEdn {
	r := &CT_R{}
	r.Raw = []*CT_RawNamedElement{{Local: markLocal, Space: NsWml}}
	r.childOrder = []runChildRef{{runChildRaw, 0}}
	p := &CT_P{}
	p.AppendR(r)
	n := &CT_FtnEdn{Type: noteType, Id: id}
	n.AppendP(p)
	return n
}

// StandardFootnotes returns the two separator notes (id -1 and 0) Word requires
// at the head of a newly created footnotes part.
func StandardFootnotes() []*CT_FtnEdn {
	return []*CT_FtnEdn{
		newSeparatorNote("-1", "separator", "separator"),
		newSeparatorNote("0", "continuationSeparator", "continuationSeparator"),
	}
}

// StandardEndnotes returns the two separator notes (id -1 and 0) Word requires
// at the head of a newly created endnotes part.
func StandardEndnotes() []*CT_FtnEdn {
	return []*CT_FtnEdn{
		newSeparatorNote("-1", "separator", "separator"),
		newSeparatorNote("0", "continuationSeparator", "continuationSeparator"),
	}
}
