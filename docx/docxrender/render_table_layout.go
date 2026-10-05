package docxrender

import (
	"math"
	"strconv"
	"strings"

	"github.com/mgilbir/spine/render"
)

// Writing a table's markup.
//
// forme lays the table out as CSS: border-collapse with fixed columns. Where
// Word and CSS differ, the renderer computes Word's answer and hands CSS a
// table in which the two agree:
//
//   - Columns are the table grid (autofit tables included: Word stores the
//     widths it computed), set as <col> widths; the table's width is their sum.
//   - Cell margins are padding. CSS puts half of each grid line's border inside
//     the cell before the padding; Word measures the margin from the grid line,
//     so the padding is the margin less that half (never negative).
//   - Every cell's borders are resolved here: the cell's own cascade (cell,
//     row exception, table, table style layers), then, where two neighbours
//     share an edge one for one, Word's conflict rule (the heavier line wins,
//     then the darker colour, then the first cell). Both cells are handed the
//     winner, so CSS's own conflict resolution has nothing to decide.
//   - A grid line is as wide as the widest border on it, as in CSS.

// wordMargin is the default cell margin of a table that sets none.
const wordNoMargin = 0.0

// borderPosition is the table border that applies to a side of a cell: the
// outer border at the edge of the table, the inside border elsewhere.
func (t *wordTable) borderPosition(c *wordTblCell, side int) int {
	row := t.rows[c.row]
	switch side {
	case wordTop:
		if c.row == 0 {
			return wordTop
		}
		return wordInsideH
	case wordBottom:
		if c.row+c.rowSpan == len(t.rows) {
			return wordBottom
		}
		return wordInsideH
	case wordLeft:
		if c.col == row.pr.gridBefore {
			return wordLeft
		}
		return wordInsideV
	}
	if c.col+c.span == t.ncols-row.pr.gridAfter {
		return wordRight
	}
	return wordInsideV
}

// layers lists the layers of a cell, highest priority first.
func (t *wordTable) layers(c *wordTblCell) []*wordTblLayer {
	if t.ts == nil {
		return nil
	}
	var out []*wordTblLayer
	for l := wordLayers - 1; l >= 0; l-- {
		if c.mask&(1<<l) != 0 && t.ts.layers[l].set {
			out = append(out, &t.ts.layers[l])
		}
	}
	return out
}

// borderOf resolves one border of a cell through the cascade: the cell's own,
// the row's exception, the table's, then the style layers.
func (t *wordTable) borderOf(c *wordTblCell, side int, layers []*wordTblLayer) wordBorder {
	pos := t.borderPosition(c, side)
	if o := c.pr.borders[side]; o.set {
		return o.v
	}
	if o := t.rows[c.row].ex.borders[pos]; o.set {
		return o.v
	}
	if o := t.direct.borders[pos]; o.set {
		return o.v
	}
	for _, l := range layers {
		if o := l.tc.borders[side]; o.set {
			return o.v
		}
		if o := l.tbl.borders[pos]; o.set {
			return o.v
		}
	}
	return wordBorder{none: true}
}

// marginOf resolves a cell margin: the cell's, the row exception's, the
// table's, then the style layers'.
func (t *wordTable) marginOf(c *wordTblCell, side int, layers []*wordTblLayer) float64 {
	if o := c.pr.mar[side]; o.set {
		return o.v
	}
	if o := t.rows[c.row].ex.mar[side]; o.set {
		return o.v
	}
	if o := t.direct.mar[side]; o.set {
		return o.v
	}
	for _, l := range layers {
		if o := l.tc.mar[side]; o.set {
			return o.v
		}
		if o := l.tbl.mar[side]; o.set {
			return o.v
		}
	}
	return wordNoMargin
}

// shadingOf resolves a cell's fill.
func (t *wordTable) shadingOf(c *wordTblCell, layers []*wordTblLayer) wordOpt[wordShading] {
	if c.pr.shd.set {
		return c.pr.shd
	}
	if s := t.rows[c.row].ex.shd; s.set {
		return s
	}
	if t.direct.shd.set {
		return t.direct.shd
	}
	for _, l := range layers {
		if l.tc.shd.set {
			return l.tc.shd
		}
		if l.tbl.shd.set {
			return l.tbl.shd
		}
	}
	return wordOpt[wordShading]{}
}

// wordBrightness orders colours by darkness as Word does in a border conflict.
func wordBrightness(c wordRGB) int { return int(c.r) + int(c.b) + 2*int(c.g) }

// wordBorderConflict is Word's rule for two borders on one edge: the heavier
// line wins; at equal weight the darker colour; then the first (the cell to the
// left or above).
func wordBorderConflict(first, second wordBorder) wordBorder {
	a, b := first.weight(), second.weight()
	switch {
	case a > b:
		return first
	case b > a:
		return second
	case first.none:
		return first
	case wordBrightness(second.color) < wordBrightness(first.color):
		return second
	}
	return first
}

func wordSameBorder(a, b wordBorder) bool {
	if a.none || b.none {
		return a.none == b.none
	}
	return a == b
}

// checkCellWidths reports cells whose preferred width (w:tcW) differs from the
// grid columns they span. Word sizes a table from the preferred widths and
// stores the grid as what it computed, so a document whose cells say otherwise
// is drawn by the grid, which is not what Word draws.
func (t *wordTable) checkCellWidths() error {
	x := make([]float64, t.ncols+1)
	for i, w := range t.cols {
		x[i+1] = x[i] + w
	}
	for _, row := range t.rows {
		for _, c := range row.cells {
			if c.pr.width.set && c.pr.width.v.typ == "dxa" && math.Abs(c.pr.width.v.v-(x[c.col+c.span]-x[c.col])) > 1 {
				return t.r.approximate("cell widths different from the table grid (drawn by the grid)")
			}
		}
	}
	return nil
}

// resolveCells computes the borders, margins and fill of every cell.
func (t *wordTable) resolveCells() error {
	r := t.r
	for _, row := range t.rows {
		for _, c := range row.cells {
			layers := t.layers(c)
			for _, l := range layers {
				if err := r.issues(l.tbl.issues); err != nil {
					return err
				}
				if err := r.issues(l.tc.issues); err != nil {
					return err
				}
			}
			for side := range 4 {
				c.bd[side] = t.borderOf(c, side, layers)
				c.mar[side] = t.marginOf(c, side, layers)
			}
			if s := t.shadingOf(c, layers); s.set && !s.v.none {
				c.fill = wordSome(s.v.fill)
			}
			c.vAlign = "top"
			if c.pr.vAlign.set {
				c.vAlign = c.pr.vAlign.v
			} else {
				for _, l := range layers {
					if l.tc.vAlign.set {
						c.vAlign = l.tc.vAlign.v
						break
					}
				}
			}
			if c.pr.noWrap && !t.fixed() {
				if err := r.approximate("cell text that must not wrap"); err != nil {
					return err
				}
			}
		}
	}
	return t.pairBorders()
}

// fixed reports a fixed layout.
func (t *wordTable) fixed() bool { return t.pr.layout.set && t.pr.layout.v == "fixed" }

// pairBorders applies the conflict rule between neighbouring cells.
func (t *wordTable) pairBorders() error {
	occ := make([]*wordTblCell, len(t.rows)*t.ncols)
	for _, row := range t.rows {
		for _, c := range row.cells {
			for i := c.row; i < c.row+c.rowSpan; i++ {
				for j := c.col; j < c.col+c.span; j++ {
					occ[i*t.ncols+j] = c
				}
			}
		}
	}
	conflict := false
	for _, row := range t.rows {
		for _, c := range row.cells {
			if c.col+c.span < t.ncols {
				var distinct []*wordTblCell
				for i := c.row; i < c.row+c.rowSpan; i++ {
					if n := occ[i*t.ncols+c.col+c.span]; n != nil && (len(distinct) == 0 || distinct[len(distinct)-1] != n) {
						distinct = append(distinct, n)
					}
				}
				if len(distinct) == 1 && distinct[0].row == c.row && distinct[0].rowSpan == c.rowSpan {
					w := wordBorderConflict(c.bd[wordRight], distinct[0].bd[wordLeft])
					c.bd[wordRight], distinct[0].bd[wordLeft] = w, w
				} else {
					for _, n := range distinct {
						conflict = conflict || !wordSameBorder(c.bd[wordRight], n.bd[wordLeft])
					}
				}
			}
			if c.row+c.rowSpan < len(t.rows) {
				var distinct []*wordTblCell
				for j := c.col; j < c.col+c.span; j++ {
					if n := occ[(c.row+c.rowSpan)*t.ncols+j]; n != nil && (len(distinct) == 0 || distinct[len(distinct)-1] != n) {
						distinct = append(distinct, n)
					}
				}
				if len(distinct) == 1 && distinct[0].col == c.col && distinct[0].span == c.span {
					w := wordBorderConflict(c.bd[wordBottom], distinct[0].bd[wordTop])
					c.bd[wordBottom], distinct[0].bd[wordTop] = w, w
				} else {
					for _, n := range distinct {
						conflict = conflict || !wordSameBorder(c.bd[wordBottom], n.bd[wordTop])
					}
				}
			}
		}
	}
	if conflict {
		return t.r.approximate("conflicting borders between cells of different sizes")
	}
	return nil
}

// columns resolves the column widths in pixels.
func (t *wordTable) columns(avail float64) ([]float64, error) {
	r := t.r
	cols := append([]float64(nil), t.grid...)
	preferred := -1.0
	if t.pr.width.set {
		switch w := t.pr.width.v; w.typ {
		case "dxa":
			preferred = w.v
		case "pct":
			preferred = w.v * avail
		}
	}
	unknown := 0
	for _, w := range cols {
		if w < 0 {
			unknown++
		}
	}
	if unknown > 0 {
		for _, row := range t.rows {
			for _, c := range row.cells {
				if c.span == 1 && cols[c.col] < 0 && c.pr.width.set && c.pr.width.v.typ == "dxa" {
					cols[c.col] = math.Max(c.pr.width.v.v, 0)
				}
			}
		}
		unknown, known := 0, 0.0
		for _, w := range cols {
			if w < 0 {
				unknown++
			} else {
				known += w
			}
		}
		if unknown > 0 {
			if err := r.approximate("table grid without column widths"); err != nil {
				return nil, err
			}
			total := avail
			if preferred >= 0 {
				total = preferred
			}
			share := math.Max(total-known, 0) / float64(unknown)
			for i, w := range cols {
				if w < 0 {
					cols[i] = share
				}
			}
		}
	}
	sum := 0.0
	for _, w := range cols {
		sum += w
	}
	if preferred >= 0 && math.Abs(preferred-sum) > 1 {
		what := "table width different from its grid"
		if !t.fixed() {
			what = "autofit table width different from its grid"
			if sum > 0 {
				scale := preferred / sum
				for i := range cols {
					cols[i] *= scale
				}
			}
		}
		if err := r.approximate(what); err != nil {
			return nil, err
		}
	}
	return cols, nil
}

// finish writes the markup of the table, laid out in avail pixels.
func (t *wordTable) finish(avail float64) error {
	r := t.r
	cols, err := t.columns(avail)
	if err != nil {
		return err
	}
	t.cols = cols
	if err = t.checkCellWidths(); err != nil {
		return err
	}
	if err = t.resolveCells(); err != nil {
		return err
	}
	// The width of every grid line: the widest border on it.
	vline := make([]float64, t.ncols+1)
	hline := make([]float64, len(t.rows)+1)
	for _, row := range t.rows {
		for _, c := range row.cells {
			vline[c.col] = math.Max(vline[c.col], c.bd[wordLeft].width)
			vline[c.col+c.span] = math.Max(vline[c.col+c.span], c.bd[wordRight].width)
			hline[c.row] = math.Max(hline[c.row], c.bd[wordTop].width)
			hline[c.row+c.rowSpan] = math.Max(hline[c.row+c.rowSpan], c.bd[wordBottom].width)
		}
	}
	x := make([]float64, t.ncols+1)
	for i, w := range cols {
		x[i+1] = x[i] + w
	}
	width := x[t.ncols]
	if width > wordMaxLength {
		return render.ErrLimit
	}

	// Position: the left grid line.
	left := t.pr.ind.v
	switch t.pr.jc.v {
	case "center":
		left = (avail - width) / 2
	case "right", "end":
		left = avail - width
	default:
		if t.r.compat < 15 && len(t.rows[0].cells) > 0 {
			// Before Word 2013 the table is placed so that the text of its
			// first cell, not its border, lines up with the margin.
			left -= t.marginOf(t.rows[0].cells[0], 1, t.layers(t.rows[0].cells[0]))
		}
	}

	if err = r.charge(2 + len(cols) + len(t.rows)); err != nil {
		return err
	}
	var sb strings.Builder
	var tc wordCSS
	tc.add("border-collapse", "collapse")
	tc.add("table-layout", "fixed")
	tc.px("width", width)
	tc.px("margin-left", left-vline[0]/2)
	tc.add("margin-top", "0")
	tc.add("margin-bottom", "0")
	sb.WriteString(`<table style="` + tc.String() + `"><colgroup>`)
	for _, w := range cols {
		var c wordCSS
		c.px("width", w)
		sb.WriteString(`<col style="` + c.String() + `">`)
	}
	sb.WriteString("</colgroup>")
	for _, row := range t.rows {
		r.nextID++
		row.id = r.nextID
		var rc wordCSS
		if row.pr.rule != "auto" && row.pr.height > 0 {
			rc.px("height", row.pr.height)
		}
		sb.WriteString(`<tr id="r` + strconv.Itoa(row.id) + `" style="` + rc.String() + `">`)
		if row.pr.gridBefore > 0 {
			if err = r.charge(1); err != nil {
				return err
			}
			sb.WriteString(`<td colspan="` + strconv.Itoa(row.pr.gridBefore) + `" style="padding:0;border:0 none"></td>`)
		}
		for _, c := range row.cells {
			if err = t.writeCell(&sb, c, x, vline, hline); err != nil {
				return err
			}
		}
		sb.WriteString("</tr>")
	}
	sb.WriteString("</table>")
	t.block.inner = sb.String()
	return nil
}

// wordRGBValue is a CSS colour value.
func wordRGBValue(c wordRGB) string {
	return "rgb(" + strconv.Itoa(int(c.r)) + "," + strconv.Itoa(int(c.g)) + "," + strconv.Itoa(int(c.b)) + ")"
}

var wordSideNames = [4]string{"top", "left", "bottom", "right"}

// writeCell writes one cell.
func (t *wordTable) writeCell(sb *strings.Builder, c *wordTblCell, x, vline, hline []float64) error {
	r := t.r
	if err := r.charge(2); err != nil {
		return err
	}
	r.nextID++
	c.id = r.nextID
	lw := [4]float64{hline[c.row], vline[c.col], hline[c.row+c.rowSpan], vline[c.col+c.span]}
	var inset [4]float64
	var css wordCSS
	for side := range 4 {
		pad := math.Max(c.mar[side]-lw[side]/2, 0)
		inset[side] = pad + lw[side]/2
		css.px("padding-"+wordSideNames[side], pad)
		b := c.bd[side]
		if b.none || b.width <= 0 {
			css.add("border-"+wordSideNames[side]+"-style", "none")
			continue
		}
		css.add("border-"+wordSideNames[side], wordPx(b.width)+" "+b.style+" "+wordRGBValue(b.color))
	}
	if c.fill.set {
		css.rgb("background-color", c.fill.v)
	}
	switch c.vAlign {
	case "center":
		css.add("vertical-align", "middle")
	case "bottom":
		css.add("vertical-align", "bottom")
	default:
		css.add("vertical-align", "top")
	}
	c.contentW = math.Max(x[c.col+c.span]-x[c.col]-inset[1]-inset[3], 0)
	sb.WriteString(`<td id="c` + strconv.Itoa(c.id) + `" colspan="` + strconv.Itoa(c.span) + `" rowspan="` + strconv.Itoa(c.rowSpan) + `" style="` + css.String() + `">`)
	wordResolveSpacing(c.blocks)
	for _, b := range c.blocks {
		if b.continuation || b.pageBreakBefore {
			if err := r.approximate("page breaks inside table cells"); err != nil {
				return err
			}
		}
		if b.finish != nil {
			if err := b.finish(c.contentW); err != nil {
				return err
			}
		}
		var wc wordCSS
		if b.before > 0 {
			wc.px("padding-top", b.before)
		}
		if b.after > 0 {
			wc.px("padding-bottom", b.after)
		}
		id := ""
		if b.tab != nil {
			r.nextID++
			b.id = r.nextID
			id = ` id="t` + strconv.Itoa(b.id) + `"`
		}
		if wc.String() == "" && id == "" {
			sb.WriteString(b.inner)
			continue
		}
		if err := r.charge(1); err != nil {
			return err
		}
		sb.WriteString(`<div` + id + ` style="` + wc.String() + `">` + b.inner + `</div>`)
	}
	sb.WriteString("</td>")
	return nil
}
