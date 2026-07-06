package pptx

import "github.com/mgilbir/spine/common/dml"

// CloneRow deep-copies the row at srcIndex — cell fills, borders, alignment,
// spans, and full text formatting — and inserts the copy at dstIndex. It
// returns the inserted row. A deck can carry one styled prototype row that
// callers clone per data row, keeping all styling in the document itself.
func (t *Table) CloneRow(srcIndex, dstIndex int) *TableRow {
	if srcIndex < 0 || srcIndex >= len(t.rows) {
		return nil
	}
	row := t.rows[srcIndex].clone()
	if dstIndex < 0 {
		dstIndex = 0
	}
	if dstIndex >= len(t.rows) {
		t.rows = append(t.rows, row)
	} else {
		t.rows = append(t.rows[:dstIndex], append([]*TableRow{row}, t.rows[dstIndex:]...)...)
	}
	return row
}

func (r *TableRow) clone() *TableRow {
	out := &TableRow{
		cells:  make([]*TableCell, len(r.cells)),
		height: r.height,
	}
	for i, c := range r.cells {
		out.cells[i] = c.clone()
	}
	return out
}

func (c *TableCell) clone() *TableCell {
	if c == nil {
		return nil
	}
	return &TableCell{
		textFrame:    c.textFrame.clone(),
		fill:         cloneColor(c.fill),
		borderLeft:   cloneBorder(c.borderLeft),
		borderRight:  cloneBorder(c.borderRight),
		borderTop:    cloneBorder(c.borderTop),
		borderBottom: cloneBorder(c.borderBottom),
		vertAlign:    c.vertAlign,
		rowSpan:      c.rowSpan,
		colSpan:      c.colSpan,
		hMerge:       c.hMerge,
		vMerge:       c.vMerge,
	}
}

func (tf *TextFrame) clone() *TextFrame {
	if tf == nil {
		return nil
	}
	out := &TextFrame{
		paragraphs: make([]*Paragraph, len(tf.paragraphs)),
		anchor:     tf.anchor,
		wrap:       tf.wrap,
		margins:    tf.margins,
	}
	for i, p := range tf.paragraphs {
		out.paragraphs[i] = p.clone()
	}
	return out
}

func (p *Paragraph) clone() *Paragraph {
	out := &Paragraph{
		runs:        make([]*Run, len(p.runs)),
		alignment:   p.alignment,
		level:       p.level,
		lineSpacing: p.lineSpacing,
		spaceBefore: p.spaceBefore,
		spaceAfter:  p.spaceAfter,
		bulletType:  p.bulletType,
		bulletChar:  p.bulletChar,
	}
	for i, r := range p.runs {
		out.runs[i] = r.clone()
	}
	return out
}

func (r *Run) clone() *Run {
	out := *r
	out.color = cloneColor(r.color)
	out.highlight = cloneColor(r.highlight)
	return &out
}

func cloneColor(c *dml.Color) *dml.Color {
	if c == nil {
		return nil
	}
	out := *c
	return &out
}

func cloneBorder(b *TableBorder) *TableBorder {
	if b == nil {
		return nil
	}
	out := *b
	return &out
}

// CloneColumn deep-copies the column at srcIndex — every row's cell styling
// plus the column width — and inserts the copy at dstIndex. It reports
// whether srcIndex was valid. This is the column-wise counterpart of CloneRow
// for tables whose column count depends on the data (the document carries
// one styled prototype column).
func (t *Table) CloneColumn(srcIndex, dstIndex int) bool {
	if srcIndex < 0 || srcIndex >= len(t.colWidths) {
		return false
	}
	if dstIndex < 0 {
		dstIndex = 0
	}
	if dstIndex > len(t.colWidths) {
		dstIndex = len(t.colWidths)
	}

	width := t.colWidths[srcIndex]
	t.colWidths = append(t.colWidths[:dstIndex], append([]dml.EMU{width}, t.colWidths[dstIndex:]...)...)

	for _, row := range t.rows {
		var cell *TableCell
		if srcIndex < len(row.cells) {
			cell = row.cells[srcIndex].clone()
		} else {
			cell = NewTableCell()
		}
		if dstIndex >= len(row.cells) {
			row.cells = append(row.cells, cell)
		} else {
			row.cells = append(row.cells[:dstIndex], append([]*TableCell{cell}, row.cells[dstIndex:]...)...)
		}
	}
	return true
}
