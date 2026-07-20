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
		t := s.Text()
		if t == "" {
			continue
		}
		if sb.Len() > 0 {
			sb.WriteString("\n\n")
		}
		sb.WriteString(t)
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
func (s *Sheet) Text() string {
	var sb strings.Builder

	if s.worksheet != nil {
		type rowLine struct {
			num  uint32
			text string
		}
		var lines []rowLine
		for i := range s.worksheet.SheetData.Row {
			r := &s.worksheet.SheetData.Row[i]
			rn, ok := rowNumberOf(r)
			if !ok || len(r.C) == 0 {
				continue
			}
			maxCol := 0
			cols := make(map[int]string, len(r.C))
			for _, c := range r.C {
				_, col, err := ParseCellRef(c.R)
				if err != nil {
					continue
				}
				cols[col] = (&Cell{sheet: s, cell: c}).String()
				if col > maxCol {
					maxCol = col
				}
			}
			if maxCol == 0 {
				continue
			}
			fields := make([]string, maxCol)
			for col, v := range cols {
				fields[col-1] = v
			}
			lines = append(lines, rowLine{num: rn, text: strings.Join(fields, "\t")})
		}
		sort.SliceStable(lines, func(i, j int) bool { return lines[i].num < lines[j].num })
		for _, ln := range lines {
			appendLine(&sb, ln.text)
		}
	}

	for _, cmt := range s.sortedComments() {
		writeCommentText(&sb, cmt)
	}

	return sb.String()
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
func writeCommentText(sb *strings.Builder, c *Comment) {
	if t := c.Text(); t != "" {
		appendLine(sb, t)
	}
	for _, reply := range c.Replies() {
		writeCommentText(sb, reply)
	}
}

// appendLine appends s as its own line, separated from prior content by "\n".
func appendLine(sb *strings.Builder, s string) {
	if sb.Len() > 0 {
		sb.WriteByte('\n')
	}
	sb.WriteString(s)
}
