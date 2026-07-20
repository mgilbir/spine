package xlsx

import (
	"strings"
	"testing"
)

// rangeAreaTooLarge reports whether a "A1:D10" range spans more cells than a
// fuzz iteration should materialize. AddPivotTable scans the declared source
// rectangle (allocating per column and per data row), so a pathological range
// like "A1:XFD1048576" would exhaust memory regardless of how few cells the
// sheet actually holds. Skipping such ranges keeps the fuzzer on genuine
// bounds/nil/validation bugs rather than deliberate out-of-memory requests; it
// does not change library behavior.
func rangeAreaTooLarge(rng string) bool {
	a, b, ok := strings.Cut(rng, ":")
	if !ok {
		return false
	}
	r1, c1, e1 := ParseCellRef(strings.TrimSpace(strings.TrimLeft(a, "'")))
	r2, c2, e2 := ParseCellRef(strings.TrimSpace(b))
	if e1 != nil || e2 != nil {
		return false
	}
	dr := r2 - r1
	if dr < 0 {
		dr = -dr
	}
	dc := c2 - c1
	if dc < 0 {
		dc = -dc
	}
	return int64(dr+1)*int64(dc+1) > 100000
}

// FuzzXlsxAddPivotTable fuzzes Sheet.AddPivotTable: the source range, the anchor
// cell, and which source columns land on the row, column, value, and filter
// axes with which aggregation. It builds a small labeled grid, adds the pivot,
// then saves and re-opens, reading the pivot tables back. No panic; a
// self-consistent read-back. Errors (bad ranges, non-numeric aggregations,
// duplicate axis fields) are expected and fine.
func FuzzXlsxAddPivotTable(f *testing.F) {
	// range, anchor, row, col, value, filter field selectors, aggregation selector
	f.Add("A1:C5", "E1", uint8(0), uint8(3), uint8(1), uint8(9), uint8(0))
	f.Add("A1:D6", "A10", uint8(0), uint8(9), uint8(2), uint8(9), uint8(3))
	f.Add("", "A1", uint8(9), uint8(9), uint8(0), uint8(9), uint8(0))
	f.Add(":", "", uint8(0), uint8(1), uint8(2), uint8(3), uint8(5))
	f.Add("A1:B2", "ZZ99", uint8(0), uint8(0), uint8(0), uint8(9), uint8(2))
	f.Add("Data!A1:C5", "E1", uint8(0), uint8(1), uint8(2), uint8(0), uint8(1))
	f.Add("A1:C5", "E1", uint8(2), uint8(2), uint8(2), uint8(2), uint8(6))

	f.Fuzz(func(t *testing.T, srcRange, anchor string, rowSel, colSel, valSel, filterSel, aggSel uint8) {
		if rangeAreaTooLarge(srcRange) {
			t.Skip("range too large for a fuzz iteration")
		}
		w := Create()
		s := w.AddSheet("Data")
		// A labeled header row and a few data rows so field names resolve.
		headers := []string{"Region", "Product", "Units", "Price"}
		for c, h := range headers {
			_ = s.SetCellValue(FormatCellRef(1, c+1), h)
		}
		for r := 2; r <= 6; r++ {
			_ = s.SetCellValue(FormatCellRef(r, 1), []string{"N", "S", "E", "W"}[r%4])
			_ = s.SetCellValue(FormatCellRef(r, 2), []string{"A", "B"}[r%2])
			_ = s.SetCellValue(FormatCellRef(r, 3), r*3)
			_ = s.SetCellValue(FormatCellRef(r, 4), float64(r)*1.5)
		}

		pick := func(sel uint8) []string {
			if int(sel) >= len(headers) {
				return nil
			}
			return []string{headers[sel]}
		}
		aggs := []PivotAggregation{
			PivotSum, PivotCount, PivotCountNum, PivotAverage,
			PivotMax, PivotMin, PivotProduct, "", "bogus",
		}
		opts := PivotOptions{
			RowFields:    pick(rowSel),
			ColumnFields: pick(colSel),
			Filters:      pick(filterSel),
		}
		if vf := pick(valSel); vf != nil {
			opts.ValueFields = []PivotValueField{{
				Field:       vf[0],
				Aggregation: aggs[int(aggSel)%len(aggs)],
			}}
		}

		if _, err := s.AddPivotTable(srcRange, anchor, opts); err != nil {
			return
		}
		fuzzReparseXlsx(w)
	})
}

// FuzzXlsxAddSparklineGroup fuzzes Sheet.AddSparklineGroup: the sparkline type,
// the series color (including malformed hex), and one or two (data range,
// location cell) mappings drawn from fuzzed strings. It adds the group, then
// saves and re-opens, reading the sparklines back. No panic; a self-consistent
// read-back.
func FuzzXlsxAddSparklineGroup(f *testing.F) {
	f.Add("line", "376092", "Sheet1!A1:D1", "E1", "Sheet1!A2:D2", "E2")
	f.Add("column", "FF376092", "A1:D1", "E1", "", "")
	f.Add("winloss", "", "A1:A10", "B1", "junk", "  ")
	f.Add("bogus", "zzz", "", "", "", "")
	f.Add("", "#GGG", "Sheet1!A1", "A1", "Sheet1!B1", "B1")
	f.Add("line", "12", "'My Sheet'!A1:C1", "D1", "A1:C1", "D2")

	f.Fuzz(func(t *testing.T, typ, color, d1, l1, d2, l2 string) {
		w := Create()
		s := w.AddSheet("Sheet1")
		for c := 1; c <= 4; c++ {
			_ = s.SetCellValue(FormatCellRef(1, c), c*7)
			_ = s.SetCellValue(FormatCellRef(2, c), c*3)
		}

		opts := SparklineOptions{
			Type:        typ,
			SeriesColor: color,
			Data:        []SparklineData{{DataRange: d1, LocationCell: l1}},
		}
		if d2 != "" || l2 != "" {
			opts.Data = append(opts.Data, SparklineData{DataRange: d2, LocationCell: l2})
		}

		g, err := s.AddSparklineGroup(opts)
		if err != nil {
			return
		}
		_ = g.Type()
		_ = g.SeriesColor()
		_ = g.Sparklines()
		// A second group exercises the merge-into-existing-extension path.
		_, _ = s.AddSparklineGroup(SparklineOptions{
			Type: typ,
			Data: []SparklineData{{DataRange: d1, LocationCell: l1}},
		})
		fuzzReparseXlsx(w)
	})
}
