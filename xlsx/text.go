package xlsx

import (
	"sort"
	"strings"
)

// Text returns every cell value and cell comment in the workbook as a single
// plain string, with no markup, suitable for search, indexing, or LLM
// ingestion. Each sheet's text (see Sheet.Text) is concatenated in workbook
// order, separated by a blank line.
func (w *Workbook) Text() string {
	var sb strings.Builder
	for _, s := range w.Sheets() {
		sep := ""
		if sb.Len() > 0 {
			sep = "\n\n"
		}
		s.writeText(&sheetText{sb: &sb, sep: sep})
	}
	return sb.String()
}

// Text returns the sheet's cell values and comments as a single plain string.
//
// Cells are laid out row-major: within a populated row the cells are separated
// by a tab ("\t") and positioned at their column (missing interior cells become
// empty fields); rows are separated by "\n". Rows with no populated cells are
// skipped rather than emitted as blank lines. Cell values are resolved the same
// way as Cell.String — shared strings and rich text are flattened to their
// text, while numbers, dates, and booleans come through as their raw stored
// value (Excel serials for dates), not the number-format-applied display text.
//
// Cell comments (legacy notes and threaded comments, including replies) follow
// the grid, one per line, ordered by their anchoring cell.
//
// Because a cell is positioned at its column, a row holding one cell in
// column XFD is 16383 tabs long: the text of a sparse sheet can be far larger
// than the file it came from.
func (s *Sheet) Text() string {
	var sb strings.Builder
	s.writeText(&sheetText{sb: &sb})
	return sb.String()
}

// sheetText writes one sheet's lines into a builder that may already hold
// earlier sheets. sep goes in front of the sheet's first byte, so a sheet that
// writes nothing leaves the builder untouched.
type sheetText struct {
	sb      *strings.Builder
	sep     string
	written bool
}

// appendLine appends text as its own line of the sheet, separated from the
// sheet's earlier content by "\n". Like the strings.Builder it replaces, an
// empty line before any content writes nothing.
func (t *sheetText) appendLine(text []byte) {
	// Grow by doubling. Appending lets a large builder grow by about a
	// quarter each time, and the copies then cost several times the result.
	if need := len(t.sep) + 1 + len(text); t.sb.Cap()-t.sb.Len() < need {
		t.sb.Grow(max(need, t.sb.Len()))
	}
	if t.written {
		t.sb.WriteByte('\n')
	} else {
		if len(text) == 0 {
			return
		}
		t.sb.WriteString(t.sep)
		t.written = true
	}
	t.sb.Write(text)
}

// writeText writes the sheet's grid and comments.
//
// Each row is built straight into one reused line buffer and copied once into
// the output. It once filled a []string as wide as the row's last column, joined
// it, and kept every joined line until the rows were sorted, then Workbook.Text
// copied each sheet again: a sheet of 20000 rows with one cell each in column
// XFD, 100 KB zipped, allocated 7.4 GB to return 328 MB of text (C616).
func (s *Sheet) writeText(out *sheetText) {
	if ws := s.ws(); ws != nil {
		type rowRef struct {
			num uint32
			idx int
		}
		rows := make([]rowRef, 0, len(ws.SheetData.Row))
		for i := range ws.SheetData.Row {
			r := &ws.SheetData.Row[i]
			if rn, ok := rowNumberOf(r); ok && len(r.C) > 0 {
				rows = append(rows, rowRef{num: rn, idx: i})
			}
		}
		sort.SliceStable(rows, func(i, j int) bool { return rows[i].num < rows[j].num })

		type field struct {
			col  int
			text string
		}
		var (
			fields []field
			line   []byte
		)
		for _, rr := range rows {
			fields = fields[:0]
			for _, c := range ws.SheetData.Row[rr.idx].C {
				if _, col, ok := c.RowCol(); ok {
					fields = append(fields, field{col: col, text: (&Cell{sheet: s, cell: c}).String()})
				}
			}
			if len(fields) == 0 {
				continue
			}
			// Two cells naming the same column: the later one wins, as it
			// did when the row was collected into a map.
			sort.SliceStable(fields, func(i, j int) bool { return fields[i].col < fields[j].col })
			line = line[:0]
			at := 1
			for i, f := range fields {
				if i+1 < len(fields) && fields[i+1].col == f.col {
					continue
				}
				for ; at < f.col; at++ {
					line = append(line, '\t')
				}
				line = append(line, f.text...)
			}
			out.appendLine(line)
		}
	}

	for _, cmt := range s.sortedComments() {
		writeCommentText(out, cmt)
	}
}

// sortedComments returns the sheet's top-level comments ordered by anchoring
// cell (row, then column) so Text() is deterministic.
func (s *Sheet) sortedComments() []*Comment {
	comments := s.Comments()
	sort.SliceStable(comments, func(i, j int) bool {
		ri, ci, ei := ParseCellRef(comments[i].Ref())
		rj, cj, ej := ParseCellRef(comments[j].Ref())
		if ei != nil || ej != nil {
			return comments[i].Ref() < comments[j].Ref()
		}
		if ri != rj {
			return ri < rj
		}
		return ci < cj
	})
	return comments
}

// writeCommentText appends a comment's text and, recursively, its replies.
func writeCommentText(out *sheetText, c *Comment) {
	if t := c.Text(); t != "" {
		out.appendLine([]byte(t))
	}
	for _, reply := range c.Replies() {
		writeCommentText(out, reply)
	}
}
