package xlsx

import (
	"github.com/mgilbir/spine/internal/imagesniff"
)

// isSVG reports whether data looks like an SVG document (an <svg> root,
// possibly preceded by an XML declaration, BOM, or doctype/comments). It is the
// shared internal/imagesniff predicate, so docx, xlsx and pptx agree on what
// counts as an SVG (C387/C441).
func isSVG(data []byte) bool { return imagesniff.IsSVG(data) }

// svgIntrinsicSize returns the intrinsic pixel size of an SVG from its root
// width/height, falling back to the viewBox, then to the CSS default 300x150.
// The parse is shared with pptx (C387), which needs the same answer to size a
// freshly added SVG picture.
func svgIntrinsicSize(data []byte) (w, h int) { return imagesniff.SVGSizePx(data) }
