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
// position of each start marker): the body first, then each header and each
// footer in part-name order. Word's built-in _GoBack bookmark and other markers
// are all included.
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
// across: every body paragraph in document order (including table- and
// SDT-nested ones), then every header and footer paragraph, headers and footers
// ordered by part name so the result is deterministic.
//
// Header/footer paragraphs belong here because Paragraph.AddBookmark is
// callable on them (Header.AddParagraph hands out exactly such a paragraph);
// leaving them out made Bookmarks() contradict its own "every bookmark in the
// document" and made Bookmark.Text() return "" for a header bookmark (C408).
func (d *Document) allBookmarkParagraphs() []*oxml.CT_P {
	var out []*oxml.CT_P
	if d.doc() != nil && d.doc().Body != nil {
		out = append(out, d.doc().Body.AllParagraphs()...)
	}
	for _, hp := range d.sortedHeaderParts() {
		if hp != nil && hp.hdr != nil {
			out = append(out, hp.hdr.AllParagraphs()...)
		}
	}
	for _, fp := range d.sortedFooterParts() {
		if fp != nil && fp.ftr != nil {
			out = append(out, fp.ftr.AllParagraphs()...)
		}
	}
	return out
}

// AddBookmark brackets the whole paragraph with a bookmark of the given name,
// allocating the next free numeric id. An internal hyperlink can target it by
// name.
func (p *Paragraph) AddBookmark(name string) *Bookmark {
	id := p.document.nextBookmarkID()
	p.touch()
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
	start.paragraph.touch()
	end.paragraph.touch()
	start.paragraph.p.InsertBookmarkStartBeforeRun(start.r, id, name)
	end.paragraph.p.InsertBookmarkEndAfterRun(end.r, id)
	return &Bookmark{document: d, name: name, id: id}
}

// nextBookmarkID returns the next free numeric bookmark id as a string.
//
// The scan covers the body (MaxBookmarkID, which reaches body-, table-, row-,
// cell- and SDT-level markers) *and* every header and footer part. Scanning the
// body alone meant two AddBookmark calls on paragraphs of two session-added
// headers each got bodyMax+1 — the same id, so the two ranges aliased (C408).
func (d *Document) nextBookmarkID() string {
	if d.doc() == nil {
		return "0"
	}
	max := oxml.MaxBookmarkID(d.doc().Body)
	consider := func(starts []*oxml.CT_BookmarkStart) {
		for _, bs := range starts {
			if bs == nil {
				continue
			}
			if v, err := strconv.Atoi(bs.Id); err == nil && v > max {
				max = v
			}
		}
	}
	for _, hp := range d.sortedHeaderParts() {
		if hp == nil || hp.hdr == nil {
			continue
		}
		consider(hp.hdr.BookmarkStart)
		for _, p := range hp.hdr.AllParagraphs() {
			if p != nil {
				consider(p.BookmarkStart)
			}
		}
	}
	for _, fp := range d.sortedFooterParts() {
		if fp == nil || fp.ftr == nil {
			continue
		}
		consider(fp.ftr.BookmarkStart)
		for _, p := range fp.ftr.AllParagraphs() {
			if p != nil {
				consider(p.BookmarkStart)
			}
		}
	}
	return strconv.Itoa(max + 1)
}
