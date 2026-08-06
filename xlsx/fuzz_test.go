package xlsx

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mgilbir/spine/internal/fuzzseed"
)

// buildValidXlsxFuzzSeed creates a small valid workbook in-process so no
// corpus binaries need committing.
func buildValidXlsxFuzzSeed(f testing.TB) []byte {
	f.Helper()
	w := Create()
	sheet := addSheetT(w, "Sheet1")
	if err := sheet.SetCellValue("A1", "fuzz"); err != nil {
		f.Fatalf("building valid xlsx seed: %v", err)
	}
	if err := sheet.SetCellValue("B2", 42); err != nil {
		f.Fatalf("building valid xlsx seed: %v", err)
	}
	// Pinned so the fixture is byte-stable across builds; a fixture that moves
	// cannot be reproduced from a crasher. See fuzzseed.FixtureModified.
	w.Properties.Created = fuzzseed.FixtureModified
	w.Properties.Modified = fuzzseed.FixtureModified

	valid, err := w.SaveBytes()
	if err != nil {
		f.Fatalf("building valid xlsx seed: %v", err)
	}
	return valid
}

// fuzzExerciseXlsx opens the bytes as a workbook and, on success, walks a
// bounded slice of the model and round-trips it (SaveBytes then re-open).
// Any panic is a bug; errors are expected and fine.
func fuzzExerciseXlsx(data []byte) {
	w, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return
	}
	defer func() { _ = w.Close() }()

	for i, s := range w.Sheets() {
		if i >= 8 {
			break
		}
		_ = s.Name()
		rows, cols := s.Rows(), s.Cols()
		if rows > 12 {
			rows = 12
		}
		if cols > 12 {
			cols = 12
		}
		for r := 1; r <= rows; r++ {
			for c := 1; c <= cols; c++ {
				cell, err := s.CellByRowCol(r, c)
				if err != nil {
					continue
				}
				_ = cell.Value()
				_ = cell.String()
				_ = cell.Type()
				_ = cell.Formula()
			}
		}
	}

	out, err := w.SaveBytes()
	if err != nil {
		return
	}
	w2, err := OpenReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		return
	}
	_ = w2.Sheets()
	_ = w2.Close()
}

// FuzzOpenXlsx feeds arbitrary bytes to the workbook opener.
func FuzzOpenXlsx(f *testing.F) {
	valid := buildValidXlsxFuzzSeed(f)

	f.Add(valid)
	f.Add([]byte{})
	f.Add([]byte("PK\x03\x04"))
	f.Add(valid[:len(valid)/2])
	corrupt := append([]byte(nil), valid...)
	corrupt[len(corrupt)/2] ^= 0xFF
	f.Add(corrupt)
	// A package that claims to be a workbook but with a hostile main part
	// and a worksheet full of malformed cell references.
	f.Add(fuzzseed.BuildZip([][2]string{
		{"[Content_Types].xml", `<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.ws()+xml"/></Types>`},
		{"_rels/.rels", `<?xml version="1.0"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`},
		{"xl/_rels/workbook.xml.rels", `<?xml version="1.0"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/></Relationships>`},
		{"xl/workbook.xml", `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="s" sheetId="1" r:id="rId1"/></sheets></workbook>`},
		{"xl/worksheets/sheet1.xml", `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><dimension ref="A1:ZZZZZZ99999999999999"/><sheetData><row r="0"><c r=""><v>1</v></c><c r="!!"><v>2</v></c><c r="A99999999999999999999"><v>3</v></c></row></sheetData><mergeCells count="1"><mergeCell ref=":"/></mergeCells></worksheet>`},
	}))
	// A handful of small real files when the gitignored corpus is present;
	// never committed.
	for _, seed := range fuzzseed.CorpusSeeds("xlsx", 5, 256<<10) {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		fuzzExerciseXlsx(data)
	})
}

// FuzzXlsxWorksheetXML packs the fuzz bytes into the first worksheet part
// of an otherwise-valid workbook, so the SpreadsheetML parsers (cell refs,
// dimensions, merges) see hostile XML directly instead of the fuzzer having
// to invent whole valid zip archives.
func FuzzXlsxWorksheetXML(f *testing.F) {
	valid := buildValidXlsxFuzzSeed(f)
	const sheetPart = "xl/worksheets/sheet1.xml"
	orig := fuzzseed.ZipEntry(valid, sheetPart)
	if orig == nil {
		f.Fatalf("valid xlsx seed has no %s", sheetPart)
	}

	f.Add(orig)
	f.Add([]byte{})
	f.Add([]byte("not xml"))
	f.Add(orig[:len(orig)/2])
	// Malformed cell references, row numbers, and merge ranges.
	f.Add([]byte(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><dimension ref="A1:ZZZZZZ99999999999999"/><sheetData><row r="0"><c r=""><v>1</v></c><c r="!!"><v>2</v></c><c r="A99999999999999999999"><v>3</v></c><c r="XFD1048577"><v>4</v></c></row><row><c><v>5</v></c></row></sheetData><mergeCells count="9999"><mergeCell ref=":"/><mergeCell ref="B2:A1"/></mergeCells></worksheet>`))
	// Shared-string index far out of range and hostile types.
	f.Add([]byte(`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData><row r="1"><c r="A1" t="s"><v>99999999</v></c><c r="B1" t="s"><v>-1</v></c><c r="C1" t="b"><v>maybe</v></c><c r="D1" t="n"><v>1e99999</v></c></row></sheetData></worksheet>`))
	// Deep nesting.
	f.Add([]byte(strings.Repeat("<x>", 300) + strings.Repeat("</x>", 300)))

	f.Fuzz(func(t *testing.T, data []byte) {
		wrapped := fuzzseed.ReplaceZipEntry(valid, sheetPart, data)
		if wrapped == nil {
			t.Skip("seed package unreadable")
		}
		fuzzExerciseXlsx(wrapped)
	})
}

// fuzzReparseXlsx saves a workbook and re-opens the bytes, walking the tables
// and conditional formats of every sheet so the write-then-read path is
// exercised. Any panic is a bug; errors are expected and fine.
func fuzzReparseXlsx(w *Workbook) {
	out, err := w.SaveBytes()
	if err != nil {
		return
	}
	w2, err := OpenReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		return
	}
	defer func() { _ = w2.Close() }()
	for _, s := range w2.Sheets() {
		for _, tbl := range s.Tables() {
			_ = tbl.Name()
			_ = tbl.Range()
			_ = tbl.Columns()
			_, _ = tbl.Style()
		}
		_ = s.ConditionalFormats()
	}
}

// FuzzXlsxAddTable fuzzes Sheet.AddTable: the range string, the column-name
// override (comma-separated), the totals-row flag, the style name and banding,
// and the table name. It creates a fresh workbook with a small grid, adds the
// table, then saves and re-opens. No panic; a self-consistent read-back.
func FuzzXlsxAddTable(f *testing.F) {
	f.Add("A1:C3", "Name,Age,City", false, "TableStyleMedium2", true, "MyTable")
	f.Add("A1:A2", "", true, "", false, "")
	f.Add("", "", false, "", false, "")
	f.Add("A1", "x", false, "", false, "")
	f.Add("A1:B1", "a,b,c", false, "", false, "R1C1")
	f.Add("A1:XFD1048576", "a,b", false, "", false, "T")
	f.Add(":", ",,,", true, "s", true, "1A")
	f.Add("C3:A1", "  ,  ", true, "TableStyleLight1", false, "Sales 2020")

	f.Fuzz(func(t *testing.T, cellRange, cols string, totals bool, style string, stripes bool, name string) {
		w := Create()
		s := addSheetT(w, "Sheet1")
		// A small header + data grid so ranges resolve to real cells and the
		// header write-back path runs against existing content.
		for r := 1; r <= 4; r++ {
			for c := 1; c <= 4; c++ {
				_ = s.SetCellValue(FormatCellRef(r, c), "v")
			}
		}

		var columns []string
		if cols != "" {
			columns = strings.Split(cols, ",")
		}
		opts := TableOptions{
			Name:      name,
			Columns:   columns,
			TotalsRow: totals,
			Style:     TableStyle{Name: style, ShowRowStripes: stripes, ShowColumnStripes: !stripes},
		}
		if totals {
			opts.ColumnTotals = map[string]TotalsColumn{
				"v": {Function: "sum", Label: "Total"},
			}
		}
		tbl, err := s.AddTable(cellRange, opts)
		if err != nil {
			return
		}
		_ = tbl.Range()
		_ = tbl.Columns()
		fuzzReparseXlsx(w)
	})
}

// FuzzXlsxAddConditionalFormat fuzzes Sheet.AddConditionalFormat: the range (or
// space-separated range list), a rule-kind selector, comparison operator,
// formula operands, colors, search text, rank and time period. It builds one
// rule of the selected kind from the fuzzed parameters, adds it, then saves and
// re-opens.
func FuzzXlsxAddConditionalFormat(f *testing.F) {
	f.Add("B2:B10", uint8(0), "greaterThan", "100", "", "F8696B", "63BE7B", "done", uint32(10), "today")
	f.Add("A1:A10 C1:C10", uint8(6), "min", "", "50", "FFAABBCC", "", "x", uint32(0), "num")
	f.Add("", uint8(3), "containsText", "", "", "", "", "", uint32(0), "")
	f.Add(":", uint8(5), "percentile", "!!", "??", "zzzzzz", "#GGG", "", uint32(1), "max")
	f.Add("A1", uint8(1), "between", "1", "9", "", "", "", uint32(3), "yesterday")
	f.Add("A1:A5", uint8(7), "percent", "0", "100", "", "", "3TrafficLights1", uint32(0), "num")
	f.Add("A1:A5", uint8(8), "", "", "", "", "", "", uint32(0), "thisMonth")

	f.Fuzz(func(t *testing.T, cellRange string, sel uint8, op, f1, f2, color1, color2, text string, rank uint32, period string) {
		w := Create()
		s := addSheetT(w, "Sheet1")
		style := DifferentialStyle{Fill: &FillStyle{FgColor: color1}}

		var rule ConditionalRule
		switch sel % 10 {
		case 0:
			rule = NewCellIsRule(op, style, f1)
		case 1:
			rule = NewCellIsRule(op, style, f1, f2)
		case 2:
			rule = NewExpressionRule(f1, style)
		case 3:
			rule = NewTextRule(op, text, style)
		case 4:
			rule = NewTop10Rule(rank, sel&1 == 0, sel&2 == 0, style)
		case 5:
			rule = NewColorScaleRule(
				ColorScalePoint{Type: op, Value: f1, Color: color1},
				ColorScalePoint{Type: period, Value: f2, Color: color2},
			)
		case 6:
			rule = NewDataBarRule(color1,
				ConditionalValueObject{Type: op, Value: f1},
				ConditionalValueObject{Type: period, Value: f2})
		case 7:
			rule = NewIconSetRule(text,
				ConditionalValueObject{Type: op, Value: f1},
				ConditionalValueObject{Type: period, Value: f2})
		case 8:
			rule = NewTimePeriodRule(period, style)
		case 9:
			rule = NewAboveAverageRule(sel&1 == 0, style)
		}
		if sel&4 == 0 {
			rule = rule.StopIfTrue()
		}
		if err := s.AddConditionalFormat(cellRange, rule); err != nil {
			return
		}
		fuzzReparseXlsx(w)
	})
}
