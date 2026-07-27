package oxml

// Bookmark remapping over a whole body, used when merging one document's
// content into another.
//
// A bookmark's identity is a numeric w:id shared by its start and end markers
// plus a name carried on the start marker, and both id spaces are per-document.
// Appending one body into another therefore has to renumber: two documents both
// carrying Word's _GoBack bookmark (very often id 0) otherwise end up with two
// bookmarkStart/bookmarkEnd pairs sharing an id, which mispairs, and with two
// bookmarks sharing a name, which makes an internal hyperlink ambiguous (C503).
//
// The markers sit at every level EG_ContentRowContent and EG_PContent reach —
// body, table, row, cell, block SDT, paragraph, and inside a paragraph's
// hyperlinks, tracked changes and inline SDTs — so the walk is the shared block
// visitor plus the shared paragraph-content visitor rather than another
// hand-rolled descent.

// BookmarkRemap renames and renumbers a body's bookmarks.
//
// ID maps an existing w:id to its replacement; it is applied to both the start
// and the end marker so the pair stays matched. Name maps an existing bookmark
// name to its replacement. Either may be nil, in which case that half is left
// alone; a mapping function returning "" leaves the value unchanged.
type BookmarkRemap struct {
	ID   func(string) string
	Name func(string) string
}

// RemapBookmarks rewrites every bookmark marker reachable from the body.
func (body *CT_Body) RemapBookmarks(remap BookmarkRemap) {
	if body == nil {
		return
	}
	remap.applyStarts(body.BookmarkStart)
	remap.applyEnds(body.BookmarkEnd)
	visitBlockContent(body.childOrder, body.P, body.Tbl, body.SdtBlock, blockVisitor{
		Para: func(p *CT_P) {
			if p == nil {
				return
			}
			VisitContent(p, ContentVisitor{
				BookmarkStart: remap.applyStart,
				BookmarkEnd:   remap.applyEnd,
			})
		},
		Tbl: func(tbl *CT_Tbl) {
			remap.applyStarts(tbl.BookmarkStart)
			remap.applyEnds(tbl.BookmarkEnd)
		},
		Row: func(tr *CT_Tr) {
			remap.applyStarts(tr.BookmarkStart)
			remap.applyEnds(tr.BookmarkEnd)
		},
		Cell: func(tc *CT_Tc) {
			remap.applyStarts(tc.BookmarkStart)
			remap.applyEnds(tc.BookmarkEnd)
		},
		Sdt: func(s *CT_SdtBlock) {
			if s.SdtContent != nil {
				remap.applyStarts(s.SdtContent.BookmarkStart)
				remap.applyEnds(s.SdtContent.BookmarkEnd)
			}
		},
	})
}

// BookmarkNames returns every bookmark name declared in the body, in document
// order, so a merge can tell which of the source's names collide.
func (body *CT_Body) BookmarkNames() []string {
	var out []string
	body.RemapBookmarks(BookmarkRemap{
		Name: func(name string) string {
			out = append(out, name)
			return ""
		},
	})
	return out
}

func (r BookmarkRemap) applyStart(bs *CT_BookmarkStart) {
	if bs == nil {
		return
	}
	if r.ID != nil && bs.Id != "" {
		if v := r.ID(bs.Id); v != "" {
			bs.Id = v
		}
	}
	if r.Name != nil && bs.Name != "" {
		if v := r.Name(bs.Name); v != "" {
			bs.Name = v
		}
	}
}

func (r BookmarkRemap) applyEnd(be *CT_BookmarkEnd) {
	if be == nil || r.ID == nil || be.Id == "" {
		return
	}
	if v := r.ID(be.Id); v != "" {
		be.Id = v
	}
}

func (r BookmarkRemap) applyStarts(list []*CT_BookmarkStart) {
	for _, bs := range list {
		r.applyStart(bs)
	}
}

func (r BookmarkRemap) applyEnds(list []*CT_BookmarkEnd) {
	for _, be := range list {
		r.applyEnd(be)
	}
}
