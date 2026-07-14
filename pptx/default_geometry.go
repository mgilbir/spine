package pptx

import "github.com/mgilbir/spine/common/dml"

// Default-layout placeholder geometry is derived from the slide size so a
// created deck is internally consistent: on a 4:3 deck (10"×7.5") and a 16:9
// deck (13.33"×7.5") alike, every baked placeholder fits within the slide and
// spans the content width (C139). Previously the master and some layouts hard-
// coded widescreen extents (12.33" wide), which overflowed a 4:3 slide.

// phMargin is the horizontal (and top) margin around placeholders: 0.5 inch.
const phMargin = dml.EMU(457200)

// phRect is a placeholder rectangle in EMU.
type phRect struct {
	x, y, cx, cy dml.EMU
}

// contentWidth is the slide width minus a margin on each side.
func contentWidth(w dml.EMU) dml.EMU {
	cw := w - 2*phMargin
	if cw < phMargin {
		cw = w // pathological tiny slide: fill it
	}
	return cw
}

// titleRect is the standard top title band.
func titleRect(w, h dml.EMU) phRect {
	return phRect{x: phMargin, y: h / 20, cx: contentWidth(w), cy: h * 4 / 25}
}

// bodyRect is the large content area below the title.
func bodyRect(w, h dml.EMU) phRect {
	y := h * 7 / 30
	return phRect{x: phMargin, y: y, cx: contentWidth(w), cy: h - y - h/20}
}

// centeredTitleRect is the vertically-centered title used by the title layout.
func centeredTitleRect(w, h dml.EMU) phRect {
	return phRect{x: phMargin, y: h * 5 / 16, cx: contentWidth(w), cy: h / 5}
}

// subtitleRect sits below the centered title.
func subtitleRect(w, h dml.EMU) phRect {
	return phRect{x: phMargin, y: h * 8 / 15, cx: contentWidth(w), cy: h * 2 / 15}
}

// sectionTitleRect / sectionTextRect are the section-header layout bands.
func sectionTitleRect(w, h dml.EMU) phRect {
	return phRect{x: phMargin, y: h / 3, cx: contentWidth(w), cy: h / 5}
}

func sectionTextRect(w, h dml.EMU) phRect {
	return phRect{x: phMargin, y: h * 3 / 5, cx: contentWidth(w), cy: h / 5}
}

// leftContentRect / rightContentRect are the two side-by-side content areas of
// the two-content layout, split around a center gutter.
func leftContentRect(w, h dml.EMU) phRect {
	gutter := phMargin
	half := (contentWidth(w) - gutter) / 2
	body := bodyRect(w, h)
	return phRect{x: phMargin, y: body.y, cx: half, cy: body.cy}
}

func rightContentRect(w, h dml.EMU) phRect {
	gutter := phMargin
	half := (contentWidth(w) - gutter) / 2
	body := bodyRect(w, h)
	return phRect{x: phMargin + half + gutter, y: body.y, cx: half, cy: body.cy}
}

// apply sets a placeholder's position and size from a rect.
func (r phRect) apply(ph *PlaceholderShape) {
	ph.SetPosition(r.x, r.y)
	ph.SetSize(r.cx, r.cy)
}
