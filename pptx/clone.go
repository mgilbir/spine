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
