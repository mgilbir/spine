package docxrender

import (
	"fmt"
	"math"

	"github.com/mgilbir/forme/layout"
	"github.com/mgilbir/spine/render"
)

// Tables.
//
// A w:tbl is translated into one block, in two steps. Translation (this file)
// reads the table's structure: properties, grid, rows, cells, the merges, and
// the cells' content, which is translated by the ordinary block translators
// into blocks of their own (paragraphs, nested tables) under the formatting the
// table style gives the cell. The block's markup is written later by finish
// (render_table_layout.go), when the width the table is laid out in is known: a
// table's percentage width and its nested tables' widths depend on where it
// sits, which is not known while the body is walked (a section's page geometry
// comes with the section's last paragraph).
//
// forme draws the table as a CSS table with collapsing borders and fixed column
// widths. Everything Word decides, the renderer decides before layout and hands
// forme as numbers: the columns, the cell margins (as padding), every border
// (with Word's conflict rule applied between neighbours), shading, row heights.
// forme lays the rows out, and pagination (render_table_paginate.go) cuts them.

const (
	// wordMaxGridCols bounds the columns of a table (Word's own limit is 63).
	wordMaxGridCols = 256
	// wordMaxTableDepth bounds nested tables.
	wordMaxTableDepth = 16
	// wordMaxTableSlots bounds rows times columns, the size of the grid the
	// borders are resolved over.
	wordMaxTableSlots = 1 << 20
)

func init() {
	wordRegisterBlock("tbl", (*wordFlow).table)
}

// wordCellCtx is the context of the blocks inside a table cell.
type wordCellCtx struct {
	// base is the paragraph level the cell's table style gives, which paragraph
	// styles are resolved over; nil is the document defaults.
	base *wordParaLevel
	// autofit is set in a table whose columns Word may widen for their content:
	// text is then not broken inside a word, so a column that is too narrow for
	// a word is reported by layout (unbreakable text) rather than hidden.
	autofit bool
}

// cellBase is the paragraph level to resolve over, nil outside table cells.
func (f *wordFlow) cellBase() *wordParaLevel {
	if f.cell == nil {
		return nil
	}
	return f.cell.base
}

// wordTblCell is a cell of a table: one w:tc that is drawn (continuations of a
// vertical merge are folded into the cell above).
type wordTblCell struct {
	node *wordNode
	pr   wordTcPr
	// row and col are the cell's first row and grid column; span and rowSpan
	// its extent.
	row, col, span, rowSpan int
	// mask is the set of table style layers that apply to the cell.
	mask   uint16
	blocks []*wordBlock

	// Resolved by finish.
	id       int
	bd       [4]wordBorder
	mar      [4]float64
	fill     wordOpt[wordRGB]
	vAlign   string
	contentW float64
}

// wordTblRow is a row of a table.
type wordTblRow struct {
	pr    wordTrPr
	ex    wordTblPr
	cells []*wordTblCell
	// keepNext is set when every paragraph in the row asks to stay with the next.
	keepNext bool
	id       int
}

// wordTable is a translated table.
type wordTable struct {
	r *wordRenderer
	// direct is the table's own properties; pr is direct over the table
	// style's.
	direct, pr wordTblPr
	ts         *wordTableStyle
	// grid holds the column widths in pixels; a negative width is unknown.
	grid  []float64
	ncols int
	rows  []*wordTblRow
	plan  *wordTablePlan
	// block is the block the table is drawn in; nested is set for a table
	// inside a cell, which pagination does not see.
	block  *wordBlock
	nested bool
	// cols are the column widths drawn, set by finish.
	cols []float64
}

// table translates a w:tbl.
func (f *wordFlow) table(n *wordNode) error {
	r := f.r
	if f.depth >= wordMaxTableDepth {
		return render.ErrLimit
	}
	direct, err := r.parseTblPr(n.child("tblPr"))
	if err != nil {
		return err
	}
	ts, err := r.styles.table(direct.style)
	if err != nil {
		return err
	}
	var whole wordTblLayer
	if ts != nil {
		whole = ts.layers[wordLayerWhole]
	}
	t := &wordTable{r: r, direct: direct, pr: direct.over(whole.tbl), ts: ts, nested: f.depth > 0}
	if err = r.issues(t.pr.issues); err != nil {
		return err
	}
	if t.pr.floating {
		if err = r.approximate("floating tables (drawn in the text flow)"); err != nil {
			return err
		}
	}
	if t.pr.bidi {
		if err = r.approximate("right-to-left tables"); err != nil {
			return err
		}
	}
	if err = t.readGrid(n.child("tblGrid")); err != nil {
		return err
	}
	var trs []*wordNode
	if err = f.tableRows(n, &trs, 0); err != nil {
		return err
	}
	if err = t.structure(trs); err != nil {
		return err
	}
	if len(t.rows) == 0 {
		return nil
	}
	if err = t.cellContent(f); err != nil {
		return err
	}
	return t.addBlock(f)
}

// readGrid reads w:tblGrid.
func (t *wordTable) readGrid(g *wordNode) error {
	if g == nil {
		return nil
	}
	for _, c := range g.children {
		if !c.is("gridCol") {
			continue
		}
		if len(t.grid) >= wordMaxGridCols {
			return render.ErrLimit
		}
		w := -1.0
		if v, ok := c.attr("w"); ok {
			px, err := wordLength(v, "w:gridCol")
			if err != nil {
				return err
			}
			w = math.Max(px, 0)
		}
		t.grid = append(t.grid, w)
	}
	return nil
}

// tableRows collects the w:tr of a table or of a content control inside one.
func (f *wordFlow) tableRows(parent *wordNode, out *[]*wordNode, depth int) error {
	if depth > 8 {
		return render.ErrLimit
	}
	for _, c := range parent.children {
		if err := f.r.ctx.Err(); err != nil {
			return err
		}
		if c.space != nsW {
			if err := f.r.leaveOut("table element " + wordKey(c)); err != nil {
				return err
			}
			continue
		}
		switch c.name {
		case "tr":
			if err := f.r.charge(1); err != nil {
				return err
			}
			*out = append(*out, c)
		case "sdt":
			if content := c.child("sdtContent"); content != nil {
				if err := f.tableRows(content, out, depth+1); err != nil {
					return err
				}
			}
		case "customXml":
			if err := f.tableRows(c, out, depth+1); err != nil {
				return err
			}
		case "tblPr", "tblGrid", "bookmarkStart", "bookmarkEnd", "permStart", "permEnd", "proofErr", "sdtPr", "sdtEndPr", "customXmlPr":
		default:
			if err := f.r.leaveOut("table element w:" + c.name); err != nil {
				return err
			}
		}
	}
	return nil
}

// cellNodes collects the w:tc of a row, through content controls.
func (r *wordRenderer) cellNodes(parent *wordNode, out *[]*wordNode, depth int) error {
	if depth > 8 {
		return render.ErrLimit
	}
	for _, c := range parent.children {
		if c.space != nsW {
			if err := r.leaveOut("row element " + wordKey(c)); err != nil {
				return err
			}
			continue
		}
		switch c.name {
		case "tc":
			if len(*out) >= wordMaxGridCols {
				return render.ErrLimit
			}
			*out = append(*out, c)
		case "sdt":
			if content := c.child("sdtContent"); content != nil {
				if err := r.cellNodes(content, out, depth+1); err != nil {
					return err
				}
			}
		case "customXml":
			if err := r.cellNodes(c, out, depth+1); err != nil {
				return err
			}
		case "trPr", "tblPrEx", "bookmarkStart", "bookmarkEnd", "permStart", "permEnd", "proofErr", "sdtPr", "sdtEndPr", "customXmlPr":
		default:
			if err := r.leaveOut("row element w:" + c.name); err != nil {
				return err
			}
		}
	}
	return nil
}

// structure reads the rows and cells and resolves the merges.
func (t *wordTable) structure(trs []*wordNode) error {
	r := t.r
	// open[c] is the cell a vertical merge in grid column c continues.
	open := make([]*wordTblCell, 0, len(t.grid))
	maxCol := len(t.grid)
	for _, tr := range trs {
		if err := r.ctx.Err(); err != nil {
			return err
		}
		pr, err := r.parseTrPr(tr.child("trPr"))
		if err != nil {
			return err
		}
		ex, err := r.parseTblPr(tr.child("tblPrEx"))
		if err != nil {
			return err
		}
		if err = r.issues(pr.issues); err != nil {
			return err
		}
		if err = r.issues(ex.issues); err != nil {
			return err
		}
		if pr.hidden {
			continue
		}
		var tcs []*wordNode
		if err = r.cellNodes(tr, &tcs, 0); err != nil {
			return err
		}
		if len(tcs) == 0 {
			// A row without cells is not a row.
			continue
		}
		ri := len(t.rows)
		row := &wordTblRow{pr: pr, ex: ex}
		next := make([]*wordTblCell, max(len(open), pr.gridBefore))
		col := pr.gridBefore
		var prev *wordTblCell
		for _, tc := range tcs {
			tcpr, err := r.parseTcPr(tc.child("tcPr"))
			if err != nil {
				return err
			}
			if err = r.issues(tcpr.issues); err != nil {
				return err
			}
			if col+tcpr.span > wordMaxGridCols {
				return render.ErrLimit
			}
			if len(next) < col+tcpr.span {
				next = append(next, make([]*wordTblCell, col+tcpr.span-len(next))...)
			}
			if tcpr.hMerge == "continue" && prev != nil && prev.row == ri && prev.col+prev.span == col {
				prev.span += tcpr.span
				if err = t.foldedContent(tc, "horizontally merged"); err != nil {
					return err
				}
				if next[prev.col] == prev {
					for k := col; k < col+tcpr.span; k++ {
						next[k] = prev
					}
				}
				col += tcpr.span
				continue
			}
			if tcpr.vMerge == "continue" && col < len(open) {
				if above := open[col]; above != nil && above.col == col && above.span == tcpr.span && above.row+above.rowSpan == ri {
					above.rowSpan++
					if err = t.foldedContent(tc, "vertically merged"); err != nil {
						return err
					}
					for k := col; k < col+tcpr.span; k++ {
						next[k] = above
					}
					col += tcpr.span
					continue
				}
			}
			if err = r.charge(2); err != nil {
				return err
			}
			cell := &wordTblCell{node: tc, pr: tcpr, row: ri, col: col, span: tcpr.span, rowSpan: 1}
			if tcpr.vMerge == "restart" {
				for k := col; k < col+tcpr.span; k++ {
					next[k] = cell
				}
			}
			row.cells = append(row.cells, cell)
			prev = cell
			col += tcpr.span
		}
		maxCol = max(maxCol, col+pr.gridAfter)
		open = next
		t.rows = append(t.rows, row)
	}
	t.ncols = maxCol
	if len(t.rows) == 0 || t.ncols == 0 {
		t.rows = nil
		return nil
	}
	if len(t.rows)*t.ncols > wordMaxTableSlots {
		return render.ErrLimit
	}
	for len(t.grid) < t.ncols {
		t.grid = append(t.grid, -1)
	}
	return nil
}

// foldedContent reports content in a cell that a merge hides.
func (t *wordTable) foldedContent(tc *wordNode, what string) error {
	stack := []*wordNode{tc}
	steps := 0
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if steps++; steps > 1<<16 {
			return render.ErrLimit
		}
		if n.space == nsW {
			switch n.name {
			case "t":
				for _, c := range n.text {
					if c != ' ' && c != '\t' && c != '\n' && c != '\r' {
						return t.r.leaveOut("content of " + what + " cells")
					}
				}
			case "drawing", "pict", "object", "tbl":
				return t.r.leaveOut("content of " + what + " cells")
			}
		}
		stack = append(stack, n.children...)
	}
	return nil
}

// layerMask returns the table style layers that apply to a cell.
func (t *wordTable) layerMask(c *wordTblCell) uint16 {
	look := t.pr.look.v
	if !t.pr.look.set {
		// Word's default for a table that says nothing: header row, first
		// column and banded rows.
		look = wordTblLook{firstRow: true, firstCol: true, noVBand: true}
	}
	nrows, lastRow := len(t.rows), c.row+c.rowSpan-1
	left := t.rows[c.row].pr.gridBefore
	right := t.ncols - t.rows[c.row].pr.gridAfter
	var m uint16
	set := func(l int) { m |= 1 << l }
	set(wordLayerWhole)
	firstRow, lastRowOn := look.firstRow && c.row == 0, look.lastRow && lastRow == nrows-1
	firstCol, lastCol := look.firstCol && c.col == left, look.lastCol && c.col+c.span == right
	if !look.noHBand {
		first, end := 0, nrows
		if look.firstRow {
			first = 1
		}
		if look.lastRow {
			end--
		}
		if c.row >= first && c.row < end {
			size := 1
			if t.pr.rowBand.set {
				size = t.pr.rowBand.v
			}
			if ((c.row-first)/size)%2 == 0 {
				set(wordLayerBand1Horz)
			} else {
				set(wordLayerBand2Horz)
			}
		}
	}
	if !look.noVBand {
		first, end := 0, t.ncols
		if look.firstCol {
			first = 1
		}
		if look.lastCol {
			end--
		}
		if c.col >= first && c.col < end {
			size := 1
			if t.pr.colBand.set {
				size = t.pr.colBand.v
			}
			if ((c.col-first)/size)%2 == 0 {
				set(wordLayerBand1Vert)
			} else {
				set(wordLayerBand2Vert)
			}
		}
	}
	if firstCol {
		set(wordLayerFirstCol)
	}
	if lastCol {
		set(wordLayerLastCol)
	}
	if firstRow {
		set(wordLayerFirstRow)
	}
	if lastRowOn {
		set(wordLayerLastRow)
	}
	switch {
	case firstRow && firstCol:
		set(wordLayerNW)
	case firstRow && lastCol:
		set(wordLayerNE)
	case lastRowOn && firstCol:
		set(wordLayerSW)
	case lastRowOn && lastCol:
		set(wordLayerSE)
	}
	return m
}

// wordBaseKey identifies the paragraph level a table style gives a cell.
type wordBaseKey struct {
	ts   *wordTableStyle
	mask uint16
}

// cellBase resolves the paragraph and run formatting the layers of a table style
// give a cell, over the document defaults.
func (s *wordStyles) cellBase(ts *wordTableStyle, mask uint16) *wordParaLevel {
	if ts == nil {
		return nil
	}
	key := wordBaseKey{ts, mask}
	if lv := s.bases[key]; lv != nil {
		return lv
	}
	lv := &wordParaLevel{ppr: s.docPPr, rpr: s.docRPr}
	for l := range wordLayers {
		if mask&(1<<l) != 0 && ts.layers[l].set {
			lv.ppr = ts.layers[l].ppr.over(lv.ppr)
			lv.rpr = ts.layers[l].rpr.over(lv.rpr)
		}
	}
	if s.bases == nil {
		s.bases = map[wordBaseKey]*wordParaLevel{}
	}
	s.bases[key] = lv
	return lv
}

// cellContent translates the content of every cell.
func (t *wordTable) cellContent(f *wordFlow) error {
	r := t.r
	for _, row := range t.rows {
		all, kept := 0, 0
		for _, c := range row.cells {
			c.mask = t.layerMask(c)
			sub := &wordFlow{r: r, depth: f.depth + 1, cell: &wordCellCtx{base: r.styles.cellBase(t.ts, c.mask), autofit: !t.fixed()}}
			content := &wordNode{space: nsW, name: "tc"}
			for _, k := range c.node.children {
				if !k.is("tcPr") {
					content.children = append(content.children, k)
				}
			}
			if err := sub.blocksOf(content); err != nil {
				return err
			}
			c.blocks = sub.cur
			for _, b := range c.blocks {
				if b.kind == "p" {
					all++
					if b.keepNext {
						kept++
					}
				}
			}
		}
		row.keepNext = all > 0 && all == kept
	}
	return nil
}

// addBlock adds the table to the flow as one block.
func (t *wordTable) addBlock(f *wordFlow) error {
	b := &wordBlock{kind: "tbl", plan: &wordTablePlan{}}
	t.plan, t.block = b.plan, b
	for _, row := range t.rows {
		for _, c := range row.cells {
			for _, cb := range c.blocks {
				b.nested = append(b.nested, cb)
				b.nested = append(b.nested, cb.nested...)
			}
		}
	}
	b.finish = t.finish
	b.inspect = t.inspect
	b.units = func(*layout.Fragment) []wordUnit { return t.plan.flat }
	return f.add(b)
}

// wordErrTable is returned for a table the layout did not place as planned.
func wordErrTable(what string) error {
	return fmt.Errorf("%w: docx: table: %s", render.ErrUnsupported, what)
}
