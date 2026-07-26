package docx

import (
	"sort"
	"strings"

	"github.com/mgilbir/spine/docx/internal/oxml"
)

// Text returns every piece of body text in the document as a single plain
// string, with no markup, suitable for search, indexing, or LLM ingestion.
//
// The text is assembled deterministically in this order:
//
//   - the document body, in document order: paragraphs (including the text of
//     runs nested in hyperlinks, simple fields, tracked insertions, and inline
//     content controls), tables, and block-level content controls;
//   - each header and each footer (ordered by part name);
//   - footnotes, then endnotes;
//   - text boxes (from the body and from headers/footers).
//
// Paragraphs are separated by "\n". Within a table, cells are separated by a
// tab ("\t") and rows by "\n"; a cell's own paragraphs are joined by a single
// space so each table row stays on one line. Tracked deletions and math
// (oMath) text are not included, mirroring the per-element accessors.
func (d *Document) Text() string {
	var sb strings.Builder

	if d.doc() != nil {
		writeBlocksText(&sb, d, d.doc().Body.TextBlocks())
	}

	for _, hp := range d.sortedHeaderParts() {
		if hp != nil {
			writeBlocksText(&sb, d, hp.hdr.TextBlocks())
		}
	}
	for _, fp := range d.sortedFooterParts() {
		if fp != nil {
			writeBlocksText(&sb, d, fp.ftr.TextBlocks())
		}
	}

	for _, fn := range d.Footnotes() {
		appendLine(&sb, fn.Text())
	}
	for _, en := range d.Endnotes() {
		appendLine(&sb, en.Text())
	}

	for _, tb := range d.TextBoxes() {
		appendLine(&sb, tb.Text())
	}

	return sb.String()
}

// writeBlocksText renders a slice of ordered body/header/footer blocks into sb:
// a paragraph as its text, a table as tab-separated cells / newline-separated
// rows, and a block-level content control as its content paragraphs.
func writeBlocksText(sb *strings.Builder, d *Document, blocks []oxml.TextBlock) {
	for _, blk := range blocks {
		switch {
		case blk.P != nil:
			appendLine(sb, (&Paragraph{document: d, p: blk.P}).Text())
		case blk.Tbl != nil:
			writeTableText(sb, d, blk.Tbl)
		case blk.Sdt != nil:
			for _, p := range blk.Sdt.ContentParagraphs() {
				appendLine(sb, (&Paragraph{document: d, p: p}).Text())
			}
		}
	}
}

// writeTableText renders a table as one line per row with tab-separated cells.
func writeTableText(sb *strings.Builder, d *Document, tbl *oxml.CT_Tbl) {
	t := &Table{document: d, tbl: tbl}
	for _, row := range t.Rows() {
		cells := row.Cells()
		texts := make([]string, len(cells))
		for i, c := range cells {
			texts[i] = cellText(c)
		}
		appendLine(sb, strings.Join(texts, "\t"))
	}
}

// cellText joins a cell's paragraph texts with a single space so the cell
// occupies one tab-separated field on its row's line.
func cellText(c *TableCell) string {
	paras := c.Paragraphs()
	parts := make([]string, 0, len(paras))
	for _, p := range paras {
		parts = append(parts, p.Text())
	}
	return strings.Join(parts, " ")
}

// appendLine appends s as its own line, separating it from prior content with a
// "\n". Empty strings still produce a line so table/paragraph structure is
// preserved.
func appendLine(sb *strings.Builder, s string) {
	if sb.Len() > 0 {
		sb.WriteByte('\n')
	}
	sb.WriteString(s)
}

// sortedHeaderParts returns the header parts ordered by part name so
// Text() is deterministic (the headers map has no inherent order).
func (d *Document) sortedHeaderParts() []*headerPart {
	keys := make([]string, 0, len(d.headers))
	for k := range d.headers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]*headerPart, 0, len(keys))
	for _, k := range keys {
		out = append(out, d.headers[k])
	}
	return out
}

// sortedFooterParts returns the footer parts ordered by part name.
func (d *Document) sortedFooterParts() []*footerPart {
	keys := make([]string, 0, len(d.footers))
	for k := range d.footers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]*footerPart, 0, len(keys))
	for _, k := range keys {
		out = append(out, d.footers[k])
	}
	return out
}
