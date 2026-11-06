package xlsx

import (
	"bytes"
	"fmt"
	"math/rand/v2"
	"runtime"
	"sort"
	"strings"
	"testing"
)

// referenceSheetText is Sheet.Text as it was before C616, kept to prove the
// streaming rewrite produces the same bytes.
func referenceSheetText(s *Sheet) string {
	var sb strings.Builder
	appendLine := func(text string) {
		if sb.Len() > 0 {
			sb.WriteByte('\n')
		}
		sb.WriteString(text)
	}
	if s.ws() != nil {
		type rowLine struct {
			num  uint32
			text string
		}
		var lines []rowLine
		for i := range s.ws().SheetData.Row {
			r := &s.ws().SheetData.Row[i]
			rn, ok := rowNumberOf(r)
			if !ok || len(r.C) == 0 {
				continue
			}
			maxCol := 0
			cols := make(map[int]string, len(r.C))
			for _, c := range r.C {
				_, col, ok := c.RowCol()
				if !ok {
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
			appendLine(ln.text)
		}
	}
	var walk func(c *Comment)
	walk = func(c *Comment) {
		if t := c.Text(); t != "" {
			appendLine(t)
		}
		for _, r := range c.Replies() {
			walk(r)
		}
	}
	for _, c := range s.sortedComments() {
		walk(c)
	}
	return sb.String()
}

func referenceWorkbookText(w *Workbook) string {
	var sb strings.Builder
	for _, s := range w.Sheets() {
		t := referenceSheetText(s)
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

// TestTextMatchesReference checks the streaming Text against the original on
// sheets built to hit its edge cases: rows out of order, duplicated, or with no
// r attribute; cells out of column order, sharing a column, or with no usable
// reference; empty values in the first line; empty sheets between full ones.
func TestTextMatchesReference(t *testing.T) {
	rng := rand.New(rand.NewPCG(1, 2))
	colName := func(c int) string { return columnLetters(c) }
	randSheet := func() string {
		var sb strings.Builder
		sb.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`)
		// Up to 40 rows over 6 numbers: past 12 elements the row sort is no
		// longer an insertion sort, so only a stable sort keeps duplicates in
		// file order.
		for range rng.IntN(40) {
			row := 1 + rng.IntN(6)
			rAttr := fmt.Sprintf(` r="%d"`, row)
			if rng.IntN(5) == 0 {
				rAttr = ""
			}
			fmt.Fprintf(&sb, `<row%s>`, rAttr)
			for range rng.IntN(20) { // over 6 columns: same-column cells past 12
				ref := fmt.Sprintf("%s%d", colName(1+rng.IntN(6)), row)
				switch rng.IntN(8) {
				case 0:
					ref = "bogus"
				case 1:
					ref = ""
				}
				val := []string{"", "x", "a b", "7"}[rng.IntN(4)]
				if ref == "" {
					fmt.Fprintf(&sb, `<c t="inlineStr"><is><t>%s</t></is></c>`, val)
				} else {
					fmt.Fprintf(&sb, `<c r="%s" t="inlineStr"><is><t>%s</t></is></c>`, ref, val)
				}
			}
			sb.WriteString(`</row>`)
		}
		sb.WriteString(`</sheetData></worksheet>`)
		return sb.String()
	}
	for i := range 400 {
		sheets := make([]string, 1+rng.IntN(3))
		for j := range sheets {
			sheets[j] = randSheet()
		}
		data := buildMutatorTestXlsx(t, sheets...)
		w, err := OpenReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			t.Fatalf("case %d: open: %v", i, err)
		}
		for j, s := range w.Sheets() {
			if got, want := s.Text(), referenceSheetText(s); got != want {
				t.Fatalf("case %d sheet %d:\n got %q\nwant %q\nxml %s", i, j, got, want, sheets[j])
			}
		}
		if got, want := w.Text(), referenceWorkbookText(w); got != want {
			t.Fatalf("case %d workbook:\n got %q\nwant %q", i, got, want)
		}
	}

	// The parts fixture adds shared strings, rich text and comments.
	base := buildXlsxPartsFixture()
	w, err := OpenReader(bytes.NewReader(base), int64(len(base)))
	if err != nil {
		t.Fatal(err)
	}
	if got, want := w.Text(), referenceWorkbookText(w); got != want || got == "" {
		t.Fatalf("parts fixture:\n got %q\nwant %q", got, want)
	}
}

// TestTextSparseRowsAllocation bounds what Text allocates for a sheet whose
// rows each hold one cell in the last column. The output is inherently 16 KB a
// row; the old code allocated about 22 times that (C616).
func TestTextSparseRowsAllocation(t *testing.T) {
	const rows = 2000
	var sb strings.Builder
	sb.WriteString(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>`)
	for i := 1; i <= rows; i++ {
		fmt.Fprintf(&sb, `<row r="%d"><c r="XFD%d" t="inlineStr"><is><t>x</t></is></c></row>`, i, i)
	}
	sb.WriteString(`</sheetData></worksheet>`)
	data := buildMutatorTestXlsx(t, sb.String())
	w, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	out := w.Text()
	runtime.ReadMemStats(&after)
	alloc := after.TotalAlloc - before.TotalAlloc
	// Each row is 16383 tabs and "x", and rows are joined by "\n".
	if want := rows*MaxCol + rows - 1; len(out) != want {
		t.Fatalf("len(Text) = %d, want %d", len(out), want)
	}
	// A doubling builder allocates under 2x the result; allow 4x.
	if limit := uint64(4 * len(out)); alloc > limit {
		t.Fatalf("Text allocated %d bytes for %d bytes of text (limit %d)", alloc, len(out), limit)
	}
	t.Logf("Text allocated %.1fx its %d-byte result", float64(alloc)/float64(len(out)), len(out))
}
