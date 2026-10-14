package main

import (
	"strings"

	"github.com/mgilbir/spine/pptx"
	"github.com/mgilbir/spine/xlsx"
)

// parseCellSelector splits a SHEET!CELL selector into its sheet name and A1
// reference. The sheet name may be wrapped in single quotes; a quoted name
// containing a single quote is not supported in slice 1 and selects nothing.
// Structural problems are invalid_selector; a missing or malformed cell
// reference is invalid_range.
func parseCellSelector(sel string) (sheet, ref string, selectsNothing bool, err error) {
	var name string
	if strings.HasPrefix(sel, "'") {
		end := strings.Index(sel[1:], "'")
		if end < 0 {
			return "", "", false, errf("invalid_selector", "selector %q has an unterminated quoted sheet name", sel)
		}
		name = sel[1 : 1+end]
		rest := sel[2+end:]
		if !strings.HasPrefix(rest, "!") {
			return "", "", false, errf("invalid_selector", "selector %q must be SHEET!CELL, e.g. Sheet1!B2", sel)
		}
		if strings.Contains(name, "'") {
			// A doubled '' inside a quoted name is not supported in slice 1;
			// such names select nothing.
			return name, "", true, nil
		}
		return name, rest[1:], false, nil
	}
	i := strings.Index(sel, "!")
	if i < 0 {
		return "", "", false, errf("invalid_selector", "selector %q must be SHEET!CELL, e.g. Sheet1!B2", sel)
	}
	name, ref = sel[:i], sel[i+1:]
	if name == "" {
		return "", "", false, errf("invalid_selector", "selector %q has an empty sheet name", sel)
	}
	if strings.ContainsAny(name, "[]'") {
		return "", "", false, errf("invalid_selector", "sheet name %q may not contain '[', ']' or '''", name)
	}
	if ref == "" {
		return "", "", false, errf("invalid_range", "selector %q has no cell reference", sel)
	}
	return name, ref, false, nil
}

// resolveSheet finds a sheet by exact, byte-for-byte name match. An unknown
// name is not_found, with the closest sheet name by case-insensitive prefix
// match added to the message when there is one (diagnostic only: the hint
// never selects).
func resolveSheet(wb *xlsx.Workbook, name string) (*xlsx.Sheet, error) {
	for _, s := range wb.Sheets() {
		if s.Name() == name {
			return s, nil
		}
	}
	msg := "sheet not found: " + quoted(name)
	if closest := closestSheetName(name, wb); closest != "" {
		msg += " (closest is " + quoted(closest) + ")"
	}
	return nil, errf("not_found", "%s", msg)
}

// closestSheetName returns the workbook sheet whose name is a case-insensitive
// prefix of name or of which name is a prefix, preferring the longest match.
func closestSheetName(name string, wb *xlsx.Workbook) string {
	lower := strings.ToLower(name)
	best := ""
	for _, s := range wb.Sheets() {
		sn := strings.ToLower(s.Name())
		if !strings.HasPrefix(lower, sn) && !strings.HasPrefix(sn, lower) {
			continue
		}
		if len(sn) > len(best) {
			best = s.Name()
		}
	}
	return best
}

// pptxSelector is a parsed PPTX text target.
type pptxSelector struct {
	slide  int    // 1-based
	kind   string // "slide" | "title" | "body" | "shape"
	shape  uint32 // stored shape id, for kind == "shape"
	remark string // the selector as given, for output
}

// parsePptxSelector parses slide/<N>, slide/<N>/title, slide/<N>/body and
// slide/<N>/shape/<ID>.
func parsePptxSelector(sel string) (*pptxSelector, error) {
	parts := strings.Split(sel, "/")
	if len(parts) < 2 || parts[0] != "slide" {
		return nil, errf("invalid_selector", "selector %q must be slide/<N>, slide/<N>/title, slide/<N>/body or slide/<N>/shape/<ID>", sel)
	}
	n, err := intFlag(parts[1])
	if err != nil {
		return nil, errf("invalid_selector", "selector %q has a slide number that is not an integer", sel)
	}
	p := &pptxSelector{slide: n, kind: "slide", remark: sel}
	if len(parts) == 2 {
		return p, nil
	}
	switch parts[2] {
	case "title", "body":
		if len(parts) != 3 {
			return nil, errf("invalid_selector", "selector %q is not a valid text target", sel)
		}
		p.kind = parts[2]
		return p, nil
	case "shape":
		if len(parts) != 4 {
			return nil, errf("invalid_selector", "selector %q is not a valid text target", sel)
		}
		id, err := intFlag(parts[3])
		if err != nil {
			return nil, errf("invalid_selector", "selector %q has a shape id that is not an integer", sel)
		}
		p.kind = "shape"
		p.shape = uint32(id)
		return p, nil
	default:
		return nil, errf("invalid_selector", "selector %q is not a valid text target", sel)
	}
}

// resolveSlide returns the 1-based slide, or not_found.
func resolveSlide(p *pptx.Presentation, n int) (*pptx.Slide, error) {
	slides := p.Slides()
	if n < 1 || n > len(slides) {
		return nil, errf("not_found", "slide %d not found; the presentation has %d slides", n, len(slides))
	}
	return slides[n-1], nil
}

// resolveShape returns the top-level shape with the stored id, or not_found
// with the spec's pointer to `spine ls`.
func resolveShape(s *pptx.Slide, slideNum int, id uint32) (pptx.Shape, error) {
	if id == 0 {
		return nil, errf("not_found", "shape 0 not found on slide %d; run `spine ls <file> slide/%d` for ids", slideNum, slideNum)
	}
	for _, sh := range s.Shapes() {
		if sh.ID() == id {
			return sh, nil
		}
	}
	return nil, errf("not_found", "shape %d not found on slide %d; run `spine ls <file> slide/%d` for ids", id, slideNum, slideNum)
}
