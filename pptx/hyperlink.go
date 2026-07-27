package pptx

import (
	"strconv"

	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/opc"
)

// Hyperlink is a hyperlink attached to a text run or a shape. The read accessors
// (URL, Anchor, Tooltip) and SetTooltip are shared verbatim with the docx and
// xlsx hyperlink APIs so the three formats are symmetric; only the anchoring
// differs by format.
//
// In pptx a hyperlink lives in an a:hlinkClick on run properties (a:rPr) or on a
// shape's non-visual properties (p:cNvPr). An external link carries an r:id to
// an External relationship (URL); an internal link carries either an r:id to a
// slide (a slide jump) or a ppaction:// action verb (first/last/next/previous
// slide, end show).
type Hyperlink struct {
	// url is the external target (http(s)://, mailto:, file path); "" for an
	// internal link.
	url string
	// anchor is the internal target: the 1-based slide number of a slide jump,
	// or a ppaction:// verb; "" for an external link.
	anchor  string
	tooltip string

	// --- write backing, consumed at save time by allocateHyperlinkRels ---
	// isExternal marks a link to an external URL, needing an External
	// RelTypeHyperlink relationship on save.
	isExternal bool
	// slideJump marks an internal jump to another slide, needing an Internal
	// RelTypeSlide relationship plus the ppaction://hlinksldjump action.
	slideJump      bool
	slideJumpIndex int
	// action is a raw ppaction:// verb that needs no relationship.
	action string
	// relID backs an external or slide-jump link. At read time it is the source
	// r:id; at write time allocateHyperlinkRels fills it on the first save.
	relID string

	// slide is the owning slide, set for read resolution.
	slide *Slide
	// markDirty flags the owning run/shape so a tooltip/target edit re-flushes
	// into the slide XML; nil until the hyperlink is bound to an owner.
	markDirty func()
}

// The standard PowerPoint slide-show action verbs (a:hlinkClick@action). Pass
// one to SetActionHyperlink; none of them allocate a relationship.
const (
	ActionNextSlide     = "ppaction://hlinkshowjump?jump=nextslide"
	ActionPreviousSlide = "ppaction://hlinkshowjump?jump=previousslide"
	ActionFirstSlide    = "ppaction://hlinkshowjump?jump=firstslide"
	ActionLastSlide     = "ppaction://hlinkshowjump?jump=lastslide"
	ActionEndShow       = "ppaction://hlinkshowjump?jump=endshow"
)

// URL returns the external target of the hyperlink, or "" when the link is
// internal (a slide jump or a ppaction:// verb).
func (h *Hyperlink) URL() string {
	if h == nil {
		return ""
	}
	return h.url
}

// Anchor returns the internal target of the hyperlink — the 1-based destination
// slide number for a slide jump, or the ppaction:// verb — or "" when the link
// is external.
func (h *Hyperlink) Anchor() string {
	if h == nil {
		return ""
	}
	return h.anchor
}

// Tooltip returns the hyperlink's screen-tip text.
func (h *Hyperlink) Tooltip() string {
	if h == nil {
		return ""
	}
	return h.tooltip
}

// SetTooltip sets the hyperlink's screen-tip text.
func (h *Hyperlink) SetTooltip(tooltip string) {
	if h == nil {
		return
	}
	h.tooltip = tooltip
	if h.markDirty != nil {
		h.markDirty()
	}
}

// cloneReset returns a deep copy of the hyperlink for a cloned run or shape.
// The value fields (url, anchor, tooltip, external/slide-jump target) are
// copied, but every per-placement binding is reset: relID is cleared so each
// clone allocates its own relationship on save (a shared relID is filled once
// and later placements are skipped by allocateHyperlinkRels, leaving a dangling
// r:id), the slide back-reference is dropped, and markDirty is rebound via the
// given setter. Returns nil for a nil receiver.
func (h *Hyperlink) cloneReset(markDirty func()) *Hyperlink {
	if h == nil {
		return nil
	}
	out := *h
	out.relID = ""
	out.slide = nil
	out.markDirty = markDirty
	return &out
}

// clearTarget blanks every target binding of a hyperlink whose destination is
// gone — the deletion sweep calls it when the slide a jump pointed at was
// removed (C364). slideJump in particular must be cleared, not just relID: it is
// resolved by *index* at save time, so a jump left flagged would silently
// re-allocate a relationship to whatever slide now occupies the freed position.
// The owner is not marked dirty: the XML side is stripped separately, and
// flagging the shape would force a resync of content the removal did not touch.
func (h *Hyperlink) clearTarget() {
	if h == nil {
		return
	}
	h.url = ""
	h.anchor = ""
	h.isExternal = false
	h.slideJump = false
	h.slideJumpIndex = 0
	h.action = ""
	h.relID = ""
}

// --- constructors used by the run/shape setters ---

func newExternalHyperlink(url string, markDirty func()) *Hyperlink {
	return &Hyperlink{url: url, isExternal: true, markDirty: markDirty}
}

func newActionHyperlink(action string, markDirty func()) *Hyperlink {
	return &Hyperlink{anchor: action, action: action, markDirty: markDirty}
}

func newSlideJumpHyperlink(index int, markDirty func()) *Hyperlink {
	return &Hyperlink{
		slideJump:      true,
		slideJumpIndex: index,
		anchor:         strconv.Itoa(index + 1),
		markDirty:      markDirty,
	}
}

// hyperlinkFromXML materializes a domain Hyperlink from a parsed a:hlinkClick.
// The URL/anchor are left unresolved (they need slide context); resolveHyperlink
// fills them in.
func hyperlinkFromXML(x *dml.HlinkXML) *Hyperlink {
	if x == nil {
		return nil
	}
	h := &Hyperlink{tooltip: x.Tooltip, action: x.Action}
	if x.Id != nil {
		h.relID = *x.Id
	}
	return h
}

// hyperlinkToXML serializes a domain Hyperlink into an a:hlinkClick. The
// relationship id (for external and slide-jump links) must already be allocated.
func hyperlinkToXML(h *Hyperlink) *dml.HlinkXML {
	if h == nil {
		return nil
	}
	x := &dml.HlinkXML{Tooltip: h.tooltip}
	switch {
	case h.isExternal:
		if h.relID != "" {
			id := h.relID
			x.Id = &id
		}
	case h.slideJump:
		// A slide jump needs an allocated RelTypeSlide relationship (r:id).
		// allocateHyperlinkRels leaves relID empty only when the jump target is
		// out of range (see Slide.SetHyperlinkToSlide), so emitting the
		// ppaction://hlinksldjump verb here would produce a dangling jump with no
		// target. Drop the whole hlinkClick instead of writing a broken action.
		if h.relID == "" {
			return nil
		}
		id := h.relID
		x.Id = &id
		x.Action = "ppaction://hlinksldjump"
	case h.action != "":
		x.Action = h.action
	}
	return x
}

// isMediaAction reports whether a parsed hlinkClick is the ppaction://media
// marker PowerPoint puts on embedded video/audio pictures — an implementation
// detail, not a user hyperlink.
func isMediaAction(x *dml.HlinkXML) bool {
	return x != nil && x.Action == "ppaction://media"
}

// resolveHyperlink fills in a materialized hyperlink's URL or anchor from the
// slide's relationships. It never mutates the slide XML, so it is safe on the
// byte-identity read path.
func (s *Slide) resolveHyperlink(h *Hyperlink) {
	if h == nil {
		return
	}
	h.slide = s
	if h.relID != "" {
		if rel := s.relByID(h.relID); rel != nil {
			switch {
			case rel.TargetMode == opc.TargetModeExternal:
				h.url = rel.Target
				h.isExternal = true
			case rel.Type == opc.RelTypeSlide:
				target := opc.ResolvePartName(s.partName, rel.Target)
				if idx := s.presentation.slideIndexByPart(target); idx >= 0 {
					h.slideJump = true
					h.slideJumpIndex = idx
					h.anchor = strconv.Itoa(idx + 1)
				}
			}
			return
		}
	}
	// A ppaction:// verb (no relationship) is reported verbatim as the anchor.
	if h.action != "" && h.anchor == "" {
		h.anchor = h.action
	}
}

// relByID returns this slide's relationship with the given id, or nil.
func (s *Slide) relByID(id string) *opc.Relationship {
	if s.presentation == nil || id == "" {
		return nil
	}
	for _, rel := range s.presentation.relationships[s.partName] {
		if rel != nil && rel.ID == id {
			return rel
		}
	}
	return nil
}

// relTargetPart resolves a slide relationship id to its absolute internal part
// name, or "" when the id is unknown or external.
func (s *Slide) relTargetPart(relID string) string {
	if rel := s.relByID(relID); rel != nil && rel.TargetMode != opc.TargetModeExternal {
		return opc.ResolvePartName(s.partName, rel.Target)
	}
	return ""
}

// forEachHyperlink calls fn for every hyperlink on the slide: each shape-level
// hyperlink and each run-level hyperlink, descending into groups and table
// cells. The walk order is stable (used for both reading and rel allocation).
func (s *Slide) forEachHyperlink(fn func(*Hyperlink)) {
	var walk func(shapes []Shape)
	walk = func(shapes []Shape) {
		for _, shape := range shapes {
			if base := baseShapeOf(shape); base != nil && base.hyperlink != nil {
				fn(base.hyperlink)
			}
			if tf := textFrameOf(shape); tf != nil {
				forEachRunHyperlink(tf, fn)
			}
			switch sh := shape.(type) {
			case *Table:
				for _, row := range sh.rows {
					for _, cell := range row.cells {
						if cell.textFrame != nil {
							forEachRunHyperlink(cell.textFrame, fn)
						}
					}
				}
			case *GroupShape:
				walk(sh.children)
			}
		}
	}
	walk(s.shapeList())
}

// forEachRunHyperlink calls fn for every run hyperlink in a text frame.
func forEachRunHyperlink(tf *TextFrame, fn func(*Hyperlink)) {
	for _, p := range tf.paragraphs {
		for _, r := range p.runs {
			if r.hyperlink != nil {
				fn(r.hyperlink)
			}
		}
	}
}

// textFrameOf returns the text frame of a text-bearing shape, or nil.
func textFrameOf(shape Shape) *TextFrame {
	switch sh := shape.(type) {
	case *TextBox:
		return sh.textFrame
	case *PlaceholderShape:
		return sh.textFrame
	case *AutoShape:
		return sh.textFrame
	}
	return nil
}

// resolveHyperlinks resolves every hyperlink on the slide after materialization.
func (s *Slide) resolveHyperlinks() {
	s.forEachHyperlink(s.resolveHyperlink)
}

// allocateHyperlinkRels allocates the relationships that API-created external and
// slide-jump hyperlinks need, filling in each hyperlink's relID. It is idempotent
// (a hyperlink that already has a relID is skipped), so it is safe to run before
// every save.
func (s *Slide) allocateHyperlinkRels() {
	if s.presentation == nil || s.partName == "" {
		return
	}
	s.forEachHyperlink(func(h *Hyperlink) {
		if h == nil || h.relID != "" {
			return
		}
		switch {
		case h.isExternal && h.url != "":
			h.relID = s.nextRelID()
			s.presentation.relationships[s.partName] = append(s.presentation.relationships[s.partName], &opc.Relationship{
				ID:         h.relID,
				Type:       opc.RelTypeHyperlink,
				Target:     h.url,
				TargetMode: opc.TargetModeExternal,
			})
			h.slide = s
		case h.slideJump:
			target := s.presentation.slidePartByIndex(h.slideJumpIndex)
			if target == "" {
				return
			}
			h.relID = s.nextRelID()
			s.presentation.relationships[s.partName] = append(s.presentation.relationships[s.partName], &opc.Relationship{
				ID:         h.relID,
				Type:       opc.RelTypeSlide,
				Target:     relativeTarget(s.partName, target),
				TargetMode: opc.TargetModeInternal,
			})
			h.slide = s
		}
	})
}

// Hyperlinks returns every hyperlink on the slide: shape-level and run-level, in
// document order, descending into groups and table cells. A slide with no
// hyperlinks returns nil.
func (s *Slide) Hyperlinks() []*Hyperlink {
	var out []*Hyperlink
	s.forEachHyperlink(func(h *Hyperlink) { out = append(out, h) })
	return out
}

// Hyperlinks returns every hyperlink across all slides, slide by slide in slide
// order (see Slide.Hyperlinks).
func (p *Presentation) Hyperlinks() []*Hyperlink {
	var out []*Hyperlink
	for _, s := range p.slides {
		out = append(out, s.Hyperlinks()...)
	}
	return out
}
