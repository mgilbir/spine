package xlsx

import (
	"strings"

	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// TextRun is one run of formatted text within a rich-text cell. A nil Font
// leaves the run in the cell's default font.
type TextRun struct {
	Text string
	Font *FontStyle
}

// SetRichText sets the cell to a rich (multi-run) string, where each run may
// carry its own font formatting — e.g. a bold label followed by a normal
// value in the same cell:
//
//	cell.SetRichText([]xlsx.TextRun{
//	    {Text: "Total: ", Font: &xlsx.FontStyle{Bold: true}},
//	    {Text: "1,234"},
//	})
//
// The runs are stored inline in the worksheet (an inlineStr cell), so no
// shared-strings table is required.
func (c *Cell) SetRichText(runs []TextRun) {
	c.markSheetDirty()
	c.cell.F = nil
	c.cell.V = nil

	if len(runs) == 0 {
		// An empty rich text is an empty inline string.
		c.cell.T = "inlineStr"
		empty := ""
		c.cell.Is = &oxml.CT_Rst{T: &empty}
		return
	}

	rst := &oxml.CT_Rst{R: make([]oxml.CT_RElt, 0, len(runs))}
	for _, run := range runs {
		rst.R = append(rst.R, oxml.CT_RElt{
			RPr: fontStyleToRPrElt(run.Font),
			T:   run.Text,
		})
	}
	c.cell.T = "inlineStr"
	c.cell.Is = rst
}

// RichText returns the cell's text as formatting runs. A plain string cell (or
// an empty cell) returns a single unformatted run; a rich cell (inline or a
// shared string with runs) returns one TextRun per run. It returns nil for a
// truly empty cell.
func (c *Cell) RichText() []TextRun {
	switch c.cell.T {
	case "inlineStr":
		if c.cell.Is == nil {
			return nil
		}
		if len(c.cell.Is.R) > 0 {
			return reltRunsToTextRuns(c.cell.Is.R)
		}
		if c.cell.Is.T != nil {
			return []TextRun{{Text: *c.cell.Is.T}}
		}
		return nil
	case "s":
		si := c.sharedStringItem()
		if si != nil && len(si.R) > 0 {
			return reltRunsToTextRuns(si.R)
		}
		// Plain shared string.
		if s := c.String(); s != "" {
			return []TextRun{{Text: s}}
		}
		return nil
	default:
		if s := c.String(); s != "" {
			return []TextRun{{Text: s}}
		}
		return nil
	}
}

// sharedStringItem returns the shared-string item this cell references, or nil.
func (c *Cell) sharedStringItem() *oxml.CT_Rst {
	if c.cell.V == nil || c.sheet == nil || c.sheet.workbook == nil {
		return nil
	}
	ss := c.sheet.workbook.sharedStrings
	if ss == nil {
		return nil
	}
	idx := 0
	for _, r := range *c.cell.V {
		if r < '0' || r > '9' {
			return nil
		}
		idx = idx*10 + int(r-'0')
	}
	if idx < 0 || idx >= len(ss.Si) {
		return nil
	}
	return &ss.Si[idx]
}

// reltRunsToTextRuns converts oxml rich-text runs to public TextRuns.
func reltRunsToTextRuns(runs []oxml.CT_RElt) []TextRun {
	out := make([]TextRun, 0, len(runs))
	for i := range runs {
		out = append(out, TextRun{
			Text: runs[i].T,
			Font: rPrEltToFontStyle(runs[i].RPr),
		})
	}
	return out
}

// fontStyleToRPrElt maps a public FontStyle to rich-text run properties
// (CT_RPrElt), mirroring fontStyleToOxml but using the run-property element
// names (rFont rather than name). A nil font yields nil properties.
func fontStyleToRPrElt(fs *FontStyle) *oxml.CT_RPrElt {
	if fs == nil {
		return nil
	}
	rpr := &oxml.CT_RPrElt{}
	set := false
	if fs.Name != "" {
		rpr.RFont = &oxml.CT_FontName{Val: fs.Name}
		set = true
	}
	if fs.Size > 0 {
		rpr.Sz = &oxml.CT_FontSize{Val: fs.Size}
		set = true
	}
	if fs.Bold {
		rpr.B = &oxml.CT_BooleanProperty{}
		set = true
	}
	if fs.Italic {
		rpr.I = &oxml.CT_BooleanProperty{}
		set = true
	}
	if fs.Underline {
		rpr.U = &oxml.CT_UnderlineProperty{Val: "single"}
		set = true
	}
	if fs.Color != "" {
		rpr.Color = &oxml.CT_Color{Rgb: normalizeHexColor(fs.Color)}
		set = true
	}
	if !set {
		return nil
	}
	return rpr
}

// rPrEltToFontStyle reconstructs a public FontStyle from rich-text run
// properties, or nil when the run has none.
func rPrEltToFontStyle(rpr *oxml.CT_RPrElt) *FontStyle {
	if rpr == nil {
		return nil
	}
	fs := &FontStyle{}
	if rpr.RFont != nil {
		fs.Name = rpr.RFont.Val
	}
	if rpr.Sz != nil {
		fs.Size = rpr.Sz.Val
	}
	fs.Bold = boolPropOn(rpr.B)
	fs.Italic = boolPropOn(rpr.I)
	fs.Underline = rpr.U != nil && strings.ToLower(rpr.U.Val) != "none"
	if rpr.Color != nil && rpr.Color.Rgb != "" {
		fs.Color = rpr.Color.Rgb
	}
	return fs
}

// boolPropOn reports whether a boolean run property is on. A present element
// with no val attribute defaults to true (OOXML CT_BooleanProperty semantics).
func boolPropOn(p *oxml.CT_BooleanProperty) bool {
	if p == nil {
		return false
	}
	return p.Val == nil || *p.Val
}
