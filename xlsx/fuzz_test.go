package xlsx

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mgilbir/spine/internal/fuzzseed"
)

// buildValidXlsxFuzzSeed creates a small valid workbook in-process so no
// corpus binaries need committing.
func buildValidXlsxFuzzSeed(f *testing.F) []byte {
	f.Helper()
	w := Create()
	sheet := w.AddSheet("Sheet1")
	if err := sheet.SetCellValue("A1", "fuzz"); err != nil {
		f.Fatalf("building valid xlsx seed: %v", err)
	}
	if err := sheet.SetCellValue("B2", 42); err != nil {
		f.Fatalf("building valid xlsx seed: %v", err)
	}
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
		{"[Content_Types].xml", `<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/></Types>`},
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
