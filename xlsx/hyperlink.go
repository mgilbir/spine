package xlsx

import (
	"fmt"
	"strings"

	"github.com/mgilbir/spine/opc"
	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// Hyperlink is a cell hyperlink. The read and write surface is shared across the
// docx, xlsx, and pptx packages: URL, Anchor, and Tooltip have the same meaning
// in each. In xlsx a hyperlink is anchored to a cell (or cell range); the
// xlsx-specific anchor cell is available via Ref.
//
// A hyperlink is either external (URL is set, Anchor is "") or internal (Anchor
// is a cell/range reference within the workbook such as "Sheet2!A1", URL is "").
type Hyperlink struct {
	sheet *Sheet
	hl    *oxml.CT_Hyperlink // backing model element (nil-safe accessors below)
	url   string             // resolved external target ("" for an internal link)
}

// URL returns the external target of the hyperlink, or "" if the hyperlink is
// internal (points at a location within the workbook).
func (h *Hyperlink) URL() string {
	if h == nil {
		return ""
	}
	return h.url
}

// Anchor returns the internal target of the hyperlink — a cell or range
// reference within the workbook, such as "Sheet2!A1" — or "" if the hyperlink is
// external.
func (h *Hyperlink) Anchor() string {
	if h == nil || h.hl == nil {
		return ""
	}
	if h.url != "" {
		return ""
	}
	return h.hl.Location
}

// Tooltip returns the hyperlink's tooltip (screen-tip) text, or "" if none.
func (h *Hyperlink) Tooltip() string {
	if h == nil || h.hl == nil {
		return ""
	}
	return h.hl.Tooltip
}

// Ref returns the cell reference the hyperlink is anchored to (e.g. "A1"). This
// is the xlsx-specific anchor, an addition to the shared hyperlink surface.
func (h *Hyperlink) Ref() string {
	if h == nil || h.hl == nil {
		return ""
	}
	return h.hl.Ref
}

// SetTooltip sets the hyperlink's tooltip (screen-tip) text and marks the sheet
// dirty so a save persists it.
func (h *Hyperlink) SetTooltip(tooltip string) {
	if h == nil || h.hl == nil {
		return
	}
	h.hl.Tooltip = tooltip
	if h.sheet != nil {
		h.sheet.markDirty()
	}
}

// Hyperlinks returns every hyperlink on the sheet, in document order. The
// returned slice is nil when the sheet has none.
func (s *Sheet) Hyperlinks() []*Hyperlink {
	if s.ws() == nil || s.ws().Hyperlinks == nil {
		return nil
	}
	links := s.ws().Hyperlinks.Hyperlink
	if len(links) == 0 {
		return nil
	}
	out := make([]*Hyperlink, 0, len(links))
	for i := range links {
		out = append(out, s.newHyperlink(&links[i]))
	}
	return out
}

// Hyperlink returns the hyperlink anchored to this cell, or nil if the cell has
// none. When a hyperlink covers a range that includes the cell, that hyperlink
// is returned.
func (c *Cell) Hyperlink() *Hyperlink {
	if c.sheet == nil || c.sheet.ws() == nil || c.sheet.ws().Hyperlinks == nil {
		return nil
	}
	links := c.sheet.ws().Hyperlinks.Hyperlink
	for i := range links {
		if strings.EqualFold(links[i].Ref, c.cell.R) || sqrefContains(links[i].Ref, c.cell.R) {
			return c.sheet.newHyperlink(&links[i])
		}
	}
	return nil
}

// newHyperlink builds a Hyperlink view over a model element, resolving the
// external target from the sheet relationships (or the pending set for links
// added this session).
func (s *Sheet) newHyperlink(hl *oxml.CT_Hyperlink) *Hyperlink {
	h := &Hyperlink{sheet: s, hl: hl}
	if hl.RID != "" {
		h.url = s.resolveHyperlinkURL(hl.RID)
	}
	return h
}

// resolveHyperlinkURL resolves a hyperlink relationship id to its external
// target, checking both the loaded sheet relationships and the pending
// (this-session) external hyperlink relationships.
func (s *Sheet) resolveHyperlinkURL(rid string) string {
	if s.workbook != nil {
		for _, rel := range s.workbook.relationships[s.partName] {
			if rel != nil && rel.ID == rid {
				return rel.Target
			}
		}
	}
	for _, rel := range s.pendingHyperlinkRels {
		if rel != nil && rel.ID == rid {
			return rel.Target
		}
	}
	return ""
}

// SetHyperlink sets an external hyperlink on the cell pointing at url (e.g.
// "https://example.com"), replacing any existing hyperlink on the cell. The link
// is written as <hyperlink ref=... r:id=.../> with an External relationship in
// the sheet's .rels. It returns the new Hyperlink so a tooltip can be attached.
//
// Works on both created and opened workbooks; a save wires the relationship and
// re-marshals the worksheet with the hyperlink.
func (c *Cell) SetHyperlink(url string) *Hyperlink {
	s := c.sheet
	s.removeHyperlinkForRef(c.cell.R)

	rid := s.nextHyperlinkRID()
	hl := oxml.CT_Hyperlink{Ref: c.cell.R, RID: rid}
	s.appendHyperlink(hl)
	s.pendingHyperlinkRels = append(s.pendingHyperlinkRels, &opc.Relationship{
		ID:         rid,
		Type:       opc.RelTypeHyperlink,
		Target:     url,
		TargetMode: opc.TargetModeExternal,
	})
	s.markDirty()
	return &Hyperlink{sheet: s, hl: s.lastHyperlink(), url: url}
}

// SetInternalHyperlink sets an internal hyperlink on the cell pointing at a
// location within the workbook (e.g. "Sheet2!A1"), replacing any existing
// hyperlink on the cell. Internal links carry a location attribute and need no
// relationship. It returns the new Hyperlink so a tooltip can be attached.
func (c *Cell) SetInternalHyperlink(location string) *Hyperlink {
	s := c.sheet
	s.removeHyperlinkForRef(c.cell.R)

	hl := oxml.CT_Hyperlink{Ref: c.cell.R, Location: location}
	s.appendHyperlink(hl)
	s.markDirty()
	return &Hyperlink{sheet: s, hl: s.lastHyperlink()}
}

// appendHyperlink appends a hyperlink to the worksheet model, creating the
// container and splicing it into the child order when absent.
func (s *Sheet) appendHyperlink(hl oxml.CT_Hyperlink) {
	s.ensureWorksheet()
	if s.ws().Hyperlinks == nil {
		s.ws().Hyperlinks = &oxml.CT_Hyperlinks{}
	}
	s.ws().EnsureChildOrder("hyperlinks")
	s.ws().Hyperlinks.Hyperlink = append(s.ws().Hyperlinks.Hyperlink, hl)
}

// lastHyperlink returns a pointer to the most recently appended hyperlink.
func (s *Sheet) lastHyperlink() *oxml.CT_Hyperlink {
	hl := s.ws().Hyperlinks.Hyperlink
	return &hl[len(hl)-1]
}

// removeHyperlinkForRef drops any existing hyperlink anchored exactly to ref
// (case-insensitive) along with its pending external relationship, so replacing
// a cell's hyperlink does not leave a duplicate or an orphaned relationship.
func (s *Sheet) removeHyperlinkForRef(ref string) {
	if s.ws() == nil || s.ws().Hyperlinks == nil {
		return
	}
	links := s.ws().Hyperlinks.Hyperlink
	kept := links[:0]
	for _, hl := range links {
		if strings.EqualFold(hl.Ref, ref) {
			if hl.RID != "" {
				s.removePendingHyperlinkRel(hl.RID)
			}
			continue
		}
		kept = append(kept, hl)
	}
	s.ws().Hyperlinks.Hyperlink = kept
	if len(kept) == 0 {
		s.ws().Hyperlinks = nil
	}
}

// removePendingHyperlinkRel drops the pending external relationship with the
// given id.
func (s *Sheet) removePendingHyperlinkRel(rid string) {
	kept := s.pendingHyperlinkRels[:0]
	for _, rel := range s.pendingHyperlinkRels {
		if rel != nil && rel.ID == rid {
			continue
		}
		kept = append(kept, rel)
	}
	s.pendingHyperlinkRels = kept
}

// nextHyperlinkRID allocates a sheet-relationship id not used by an existing
// relationship, an existing hyperlink in the model, or a pending hyperlink rel.
func (s *Sheet) nextHyperlinkRID() string {
	used := make(map[string]struct{})
	if s.workbook != nil {
		for _, rel := range s.workbook.relationships[s.partName] {
			if rel != nil {
				used[rel.ID] = struct{}{}
			}
		}
	}
	for _, rel := range s.pendingHyperlinkRels {
		if rel != nil {
			used[rel.ID] = struct{}{}
		}
	}
	if s.ws() != nil && s.ws().Hyperlinks != nil {
		for _, hl := range s.ws().Hyperlinks.Hyperlink {
			if hl.RID != "" {
				used[hl.RID] = struct{}{}
			}
		}
	}
	return fmt.Sprintf("rId%d", nextRelationshipID(used))
}

// sheetsHavePendingHyperlinkRels reports whether any sheet has external
// hyperlink relationships that still need to be written to a sheet .rels.
func (w *Workbook) sheetsHavePendingHyperlinkRels() bool {
	for _, sheet := range w.sheets {
		if len(sheet.pendingHyperlinkRels) > 0 {
			return true
		}
	}
	return false
}
