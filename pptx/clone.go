package pptx

import (
	"encoding/xml"

	"github.com/mgilbir/spine/common/dml"
)

// CloneRow deep-copies the row at srcIndex — cell fills, borders, alignment,
// spans, and full text formatting — and inserts the copy at dstIndex. It
// returns the inserted row. A deck can carry one styled prototype row that
// callers clone per data row, keeping all styling in the document itself.
//
// Cells that participate in merges are copied as-is; cloning across a merge
// boundary is the caller's responsibility. On tables loaded from a file,
// call SyncXML after the mutations so they reach the slide XML.
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
	t.structDirty = true
	return row
}

func (r *TableRow) clone() *TableRow {
	if r == nil {
		return nil
	}
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

// clone deep-copies the text frame.
//
// It starts from a value copy so a field added to TextFrame is carried over by
// default; only the reference-typed fields need explicit deep copies below.
// Enumerating fields instead is what silently dropped autofit here and seven
// paragraph properties in Paragraph.clone (C415) — the clone helpers predate
// the rich-text wave that added them.
func (tf *TextFrame) clone() *TextFrame {
	if tf == nil {
		return nil
	}
	out := *tf
	out.paragraphs = make([]*Paragraph, len(tf.paragraphs))
	for i, p := range tf.paragraphs {
		out.paragraphs[i] = p.clone()
	}
	return &out
}

// clone deep-copies the paragraph. Like TextFrame.clone it starts from a value
// copy; TestParagraphClone_CopiesEveryField guards the reference-typed fields.
func (p *Paragraph) clone() *Paragraph {
	if p == nil {
		return nil
	}
	out := *p
	out.runs = make([]*Run, len(p.runs))
	for i, r := range p.runs {
		out.runs[i] = r.clone()
	}
	out.bulletColor = cloneColor(p.bulletColor)
	out.marL = cloneInt32Ptr(p.marL)
	out.indent = cloneInt32Ptr(p.indent)
	if p.tabStops != nil {
		out.tabStops = append([]TabStop(nil), p.tabStops...)
	}
	return &out
}

// cloneInt32Ptr returns a fresh pointer to the same value, so the clone and the
// original never share a cell.
func cloneInt32Ptr(v *int32) *int32 {
	if v == nil {
		return nil
	}
	c := *v
	return &c
}

func (r *Run) clone() *Run {
	out := *r
	out.color = cloneColor(r.color)
	out.highlight = cloneColor(r.highlight)
	// Deep-copy the hyperlink so the clone shares no state with the original:
	// each clone must allocate its own relationship on save (a shared relID is
	// filled once and the second placement is skipped, dangling), and editing
	// the clone's link must not mutate the original. markDirty is rebound to
	// this run so a later tooltip/target edit still re-flushes it.
	out.hyperlink = r.hyperlink.cloneReset(func() { out.dirty = true })
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
// one styled prototype column). The merge and SyncXML notes on CloneRow
// apply here too.
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
	t.structDirty = true
	return true
}

// CloneShape deep-copies a shape for prototype-based repetition (text boxes
// and auto shapes; other shape kinds return nil). The clone shares no state
// with the original: text frames are deep-copied and the drawing properties
// are copied via their XML round-trip. Add the clone to a slide with
// Slide.AddShape; on slides loaded from a file it is appended to the parsed
// shape tree with a fresh id, leaving the existing content untouched.
func CloneShape(shape Shape) Shape {
	switch s := shape.(type) {
	case *TextBox:
		c := &TextBox{
			BaseShape: cloneBaseShape(s.BaseShape),
			textFrame: s.textFrame.clone(),
			spPr:      cloneSpPr(s.spPr),
		}
		rebindShapeHyperlink(&c.BaseShape)
		return c
	case *AutoShape:
		c := &AutoShape{
			BaseShape:      cloneBaseShape(s.BaseShape),
			presetGeometry: s.presetGeometry,
			textFrame:      s.textFrame.clone(),
			spPr:           cloneSpPr(s.spPr),
		}
		rebindShapeHyperlink(&c.BaseShape)
		return c
	}
	return nil
}

// cloneBaseShape copies the base fields but drops the source node identity:
// the clone is a new shape and must not alias the original's parsed node, or
// id-matched in-place updates would hit the original. The shape-level hyperlink
// is deep-copied with its per-placement state reset (see Hyperlink.cloneReset)
// so the clone allocates its own relationship and edits to it do not touch the
// original; markDirty is rebound to the clone by rebindShapeHyperlink once the
// enclosing shape's final address is known.
func cloneBaseShape(b BaseShape) BaseShape {
	b.sourceID = 0
	b.hyperlink = b.hyperlink.cloneReset(nil)
	return b
}

// rebindShapeHyperlink points a cloned shape's hyperlink markDirty at the
// shape's own dirty flag, so a tooltip/target edit re-flushes the clone rather
// than a stale copy. cloneBaseShape returns the BaseShape by value, so the
// binding can only be made after it is embedded in the final shape.
func rebindShapeHyperlink(b *BaseShape) {
	if b.hyperlink != nil {
		b.hyperlink.markDirty = func() { b.dirty = true }
	}
}

// cloneSpPr deep-copies shape drawing properties through their XML encoding
// (the structs exist for that round-trip). The error paths are unreachable
// for properties that were themselves parsed or built through these structs;
// returning empty properties keeps the clone usable rather than aliasing the
// source's pointer fields.
func cloneSpPr(src dml.SpPr) dml.SpPr {
	data, err := xml.Marshal(&src)
	if err != nil {
		return dml.SpPr{}
	}
	var out dml.SpPr
	if err := xml.Unmarshal(data, &out); err != nil {
		return dml.SpPr{}
	}
	return out
}
