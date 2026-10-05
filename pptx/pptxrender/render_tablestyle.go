package pptxrender

import (
	"fmt"
	"strings"

	"github.com/mgilbir/spine/common/dml"
	xmlb "github.com/mgilbir/spine/common/xml"
	core "github.com/mgilbir/spine/internal/render"
	"github.com/mgilbir/spine/pptx/internal/oxml"
	"github.com/mgilbir/spine/render"
)

// Table style parts, lowest precedence first: where parts overlap on a cell,
// the later one in a:tblStyle's schema order wins, per property.
const (
	partWhole = iota
	partBand1H
	partBand2H
	partBand1V
	partBand2V
	partLastCol
	partFirstCol
	partLastRow
	partSECell
	partSWCell
	partFirstRow
	partNECell
	partNWCell
)

// tableStyler resolves what a table style gives each cell of a table: fill,
// text and the four borders. Rows and columns are grid positions; a merged
// cell takes its style from its first grid cell.
type tableStyler struct {
	style      *dml.TableStyle
	rows, cols int
	parts      []tableStylePart // lowest precedence first
}

type tableStylePart struct {
	rank  int
	style *dml.TablePartStyle
	// in reports whether the part applies to a grid cell: its region.
	in func(r, c int) bool
}

// newTableStyler lays a style's parts over a table with the options its
// a:tblPr switches on.
func newTableStyler(ts *dml.TableStyle, pr *oxml.ATblPr, rows, cols int) *tableStyler {
	t := &tableStyler{style: ts, rows: rows, cols: cols}
	firstRow, lastRow := pr.FirstRow && rows > 0, pr.LastRow && rows > 0
	firstCol, lastCol := pr.FirstCol && cols > 0, pr.LastCol && cols > 0
	// Banding runs over the body: the header and total rows and the first
	// and last columns are not counted.
	bodyRows, bodyCols := [2]int{0, rows - 1}, [2]int{0, cols - 1}
	if firstRow {
		bodyRows[0]++
	}
	if lastRow {
		bodyRows[1]--
	}
	if firstCol {
		bodyCols[0]++
	}
	if lastCol {
		bodyCols[1]--
	}
	bandH := func(parity int) func(r, c int) bool {
		return func(r, c int) bool {
			return pr.BandRow && r >= bodyRows[0] && r <= bodyRows[1] && (r-bodyRows[0])%2 == parity
		}
	}
	bandV := func(parity int) func(r, c int) bool {
		return func(r, c int) bool {
			return pr.BandCol && c >= bodyCols[0] && c <= bodyCols[1] && (c-bodyCols[0])%2 == parity
		}
	}
	for _, p := range []struct {
		rank  int
		style *dml.TablePartStyle
		in    func(r, c int) bool
	}{
		{partWhole, ts.WholeTbl, func(r, c int) bool { return true }},
		{partBand1H, ts.Band1H, bandH(0)},
		{partBand2H, ts.Band2H, bandH(1)},
		{partBand1V, ts.Band1V, bandV(0)},
		{partBand2V, ts.Band2V, bandV(1)},
		{partLastCol, ts.LastCol, func(r, c int) bool { return lastCol && c == cols-1 }},
		{partFirstCol, ts.FirstCol, func(r, c int) bool { return firstCol && c == 0 }},
		{partLastRow, ts.LastRow, func(r, c int) bool { return lastRow && r == rows-1 }},
		{partSECell, ts.SeCell, func(r, c int) bool { return lastRow && lastCol && r == rows-1 && c == cols-1 }},
		{partSWCell, ts.SwCell, func(r, c int) bool { return lastRow && firstCol && r == rows-1 && c == 0 }},
		{partFirstRow, ts.FirstRow, func(r, c int) bool { return firstRow && r == 0 }},
		{partNECell, ts.NeCell, func(r, c int) bool { return firstRow && lastCol && r == 0 && c == cols-1 }},
		{partNWCell, ts.NwCell, func(r, c int) bool { return firstRow && firstCol && r == 0 && c == 0 }},
	} {
		if p.style != nil {
			t.parts = append(t.parts, tableStylePart{rank: p.rank, style: p.style, in: p.in})
		}
	}
	return t
}

// fill is the solid fill the style gives a cell, with set false where none
// of its parts says; none is true when a part says the cell has no fill.
func (t *tableStyler) fill(r, c int) (solid *dml.SolidFill, none bool) {
	for i := len(t.parts) - 1; i >= 0; i-- {
		p := t.parts[i]
		if !p.in(r, c) || p.style.TcStyle == nil {
			continue
		}
		if f := p.style.TcStyle.SolidFill; f != nil {
			return f, false
		}
		if p.style.TcStyle.NoFill != nil {
			return nil, true
		}
	}
	return nil, false
}

// textStyle is a cell's text properties from its style: bold and a color,
// each nil where the style leaves them to the text.
type textStyle struct {
	bold  *bool
	color *dml.SolidFill
}

func (t *tableStyler) text(r, c int) textStyle {
	var out textStyle
	for i := len(t.parts) - 1; i >= 0; i-- {
		p := t.parts[i]
		if !p.in(r, c) || p.style.TcTxStyle == nil {
			continue
		}
		tx := p.style.TcTxStyle
		if out.bold == nil {
			switch tx.B {
			case "on":
				v := true
				out.bold = &v
			case "off":
				v := false
				out.bold = &v
			}
		}
		if out.color == nil {
			switch {
			case tx.SchemeClr != nil:
				out.color = &dml.SolidFill{SchemeClr: tx.SchemeClr}
			case tx.SrgbClr != nil:
				out.color = &dml.SolidFill{SrgbClr: tx.SrgbClr}
			case tx.SysClr != nil:
				out.color = &dml.SolidFill{SysClr: tx.SysClr}
			case tx.PrstClr != nil:
				out.color = &dml.SolidFill{PrstClr: tx.PrstClr}
			}
		}
	}
	return out
}

// Sides of a cell, as indexes.
const (
	sideLeft = iota
	sideTop
	sideRight
	sideBottom
)

// edge is the border the style gives one side of a grid cell, and the rank
// of the part it comes from; nil where no part says. A part's border named
// for the sides of its region (left, top, ...) applies where the cell is at
// the region's edge, and its inside borders where a neighbor in the same
// region lies beyond.
func (t *tableStyler) edge(r, c, side int) (*dml.Ln, int) {
	dr, dc := [4]int{0, -1, 0, 1}[side], [4]int{-1, 0, 1, 0}[side]
	nr, nc := r+dr, c+dc
	neighbor := nr >= 0 && nr < t.rows && nc >= 0 && nc < t.cols
	for i := len(t.parts) - 1; i >= 0; i-- {
		p := t.parts[i]
		if !p.in(r, c) || p.style.TcStyle == nil || p.style.TcStyle.TcBdr == nil {
			continue
		}
		b := p.style.TcStyle.TcBdr
		inside := neighbor && p.in(nr, nc)
		var l *dml.ThemeableLineStyle
		switch {
		case side == sideLeft && inside:
			l = b.InsideV
		case side == sideLeft:
			l = b.Left
		case side == sideRight && inside:
			l = b.InsideV
		case side == sideRight:
			l = b.Right
		case side == sideTop && inside:
			l = b.InsideH
		case side == sideTop:
			l = b.Top
		case side == sideBottom && inside:
			l = b.InsideH
		default:
			l = b.Bottom
		}
		if l != nil && l.Ln != nil {
			return l.Ln, p.rank
		}
	}
	return nil, 0
}

// tableStyleLookup is how a table's style id resolves.
type tableStyleLookup struct {
	style *dml.TableStyle // a built-in style, or nil
	// leftOut is why the table is drawn without a style, nil when it has
	// none or the style is built in.
	leftOut error
}

// resolveTableStyle finds a table's style: PowerPoint's built-in styles are
// known by their ids, unless the deck's ppt/tableStyles.xml defines the id,
// whose definition this renderer does not read.
func (c *renderColors) resolveTableStyle(pr *oxml.ATblPr) (tableStyleLookup, error) {
	if pr == nil || (pr.TableStyle == nil && pr.TableStyleId == "") {
		return tableStyleLookup{}, nil
	}
	left := tableStyleLookup{leftOut: fmt.Errorf("%w: table style left out", render.ErrUnsupported)}
	if pr.TableStyle != nil {
		return left, nil
	}
	id := strings.ToUpper(strings.TrimSpace(pr.TableStyleId))
	ts := builtinTableStyle(id)
	if ts == nil {
		return left, nil
	}
	defined, err := c.deckTableStyle(id)
	if err != nil {
		return tableStyleLookup{}, err
	}
	if defined {
		return left, nil
	}
	return tableStyleLookup{style: ts}, nil
}

// deckTableStyle reports whether the deck's ppt/tableStyles.xml defines a
// style id.
func (c *renderColors) deckTableStyle(id string) (bool, error) {
	if !c.tableStylesLoaded {
		c.tableStylesLoaded = true
		c.tableStyleIDs, c.tableStylesErr = c.parseTableStyles()
	}
	return c.tableStyleIDs[id], c.tableStylesErr
}

func (c *renderColors) parseTableStyles() (map[string]bool, error) {
	data := c.slide.presentation.tableStyles
	if len(data) == 0 {
		return nil, nil
	}
	if err := c.budget.CheckXML(c.ctx, data, func(node core.XMLNode) error {
		if len(node.Path) == 1 && !node.Text && (node.Name.Space != nsA || node.Name.Local != "tblStyleLst") {
			return fmt.Errorf("%w: table styles XML root", render.ErrInvalid)
		}
		return nil
	}); err != nil {
		return nil, fmt.Errorf("pptx: table styles: %w", err)
	}
	var lst dml.TblStyleLst
	if err := xmlb.UnmarshalWithSource(data, &lst); err != nil {
		return nil, fmt.Errorf("%w: table styles: %w", render.ErrInvalid, err)
	}
	ids := make(map[string]bool, len(lst.TblStyle))
	for _, s := range lst.TblStyle {
		if s != nil {
			ids[strings.ToUpper(strings.TrimSpace(s.StyleId))] = true
		}
	}
	return ids, nil
}
