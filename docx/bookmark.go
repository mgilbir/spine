package docx

import (
	"strconv"

	"github.com/mgilbir/spine/docx/internal/oxml"
)

// Bookmark is a named location or span in a document. A bookmark brackets a
// range of content with a w:bookmarkStart / w:bookmarkEnd pair sharing a w:id;
// internal hyperlinks (AddInternalHyperlink) target it by name.
type Bookmark struct {
	document *Document
	name     string
	id       string
}

// Name returns the bookmark name (the value an internal hyperlink anchors to).
func (b *Bookmark) Name() string { return b.name }

// Text returns the document text between the bookmark's start and end markers,
// or "" for a point bookmark that spans no content.
func (b *Bookmark) Text() string {
	text, _ := oxml.BookmarkText(b.document.allBookmarkParagraphs(), b.id)
	return text
}

// Bookmarks returns every bookmark in the document, in document order (by the
// position of each start marker). Word's built-in _GoBack bookmark and other
// markers are all included.
func (d *Document) Bookmarks() []*Bookmark {
	var out []*Bookmark
	seen := make(map[string]bool)
	for _, p := range d.allBookmarkParagraphs() {
		if p == nil {
			continue
		}
		for _, bs := range p.BookmarkStart {
			if bs == nil || seen[bs.Id] {
				continue
			}
			seen[bs.Id] = true
			out = append(out, &Bookmark{document: d, name: bs.Name, id: bs.Id})
		}
	}
	return out
}

// allBookmarkParagraphs returns the paragraphs a bookmark range can be resolved
// across: every body paragraph in document order (including table-nested ones).
func (d *Document) allBookmarkParagraphs() []*oxml.CT_P {
	if d.doc() == nil || d.doc().Body == nil {
		return nil
	}
	return d.doc().Body.AllParagraphs()
}

// AddBookmark brackets the whole paragraph with a bookmark of the given name,
// allocating the next free numeric id. An internal hyperlink can target it by
// name.
func (p *Paragraph) AddBookmark(name string) *Bookmark {
	id := p.document.nextBookmarkID()
	p.p.AddBookmarkAroundParagraph(id, name)
	return &Bookmark{document: p.document, name: name, id: id}
}

// AddBookmarkOnRange brackets the content from the start run to the end run
// (inclusive) with a bookmark of the given name. The runs may live in the same
// paragraph or in different paragraphs. Returns nil if either run is not a
// direct child run of its paragraph (e.g. a run nested inside a hyperlink),
// leaving the document unchanged so no bookmarkStart is placed without a
// matching bookmarkEnd.
func (d *Document) AddBookmarkOnRange(name string, start, end *Run) *Bookmark {
	if start == nil || end == nil || start.paragraph == nil || end.paragraph == nil {
		return nil
	}
	// Verify both markers can be anchored before mutating either paragraph: the
	// end run being nested (not a direct child) must not leave a dangling
	// bookmarkStart from a half-completed insertion (C296).
	if !start.paragraph.p.HasDirectChildRun(start.r) || !end.paragraph.p.HasDirectChildRun(end.r) {
		return nil
	}
	id := d.nextBookmarkID()
	start.paragraph.p.InsertBookmarkStartBeforeRun(start.r, id, name)
	end.paragraph.p.InsertBookmarkEndAfterRun(end.r, id)
	return &Bookmark{document: d, name: name, id: id}
}

// nextBookmarkID returns the next free numeric bookmark id as a string.
func (d *Document) nextBookmarkID() string {
	return strconv.Itoa(oxml.MaxBookmarkID(d.allBookmarkParagraphs()) + 1)
}
