package xlsx

import (
	"archive/zip"
	"bytes"
	"testing"
)

// fuzzXlsxZip assembles an in-memory zip archive from name/body pairs.
func fuzzXlsxZip(entries [][2]string) []byte {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range entries {
		w, err := zw.Create(e[0])
		if err != nil {
			continue
		}
		_, _ = w.Write([]byte(e[1]))
	}
	_ = zw.Close()
	return buf.Bytes()
}

// FuzzOpenXlsx feeds arbitrary bytes to the workbook opener and, when a
// package opens, walks a bounded slice of the model and round-trips it
// (SaveBytes then re-open). Any panic is a bug; errors are expected.
func FuzzOpenXlsx(f *testing.F) {
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

	f.Add(valid)
	f.Add([]byte{})
	f.Add([]byte("PK\x03\x04"))
	f.Add(valid[:len(valid)/2])
	corrupt := append([]byte(nil), valid...)
	corrupt[len(corrupt)/2] ^= 0xFF
	f.Add(corrupt)
	// A package that claims to be a workbook but with a hostile main part
	// and a worksheet full of malformed cell references.
	f.Add(fuzzXlsxZip([][2]string{
		{"[Content_Types].xml", `<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/></Types>`},
		{"_rels/.rels", `<?xml version="1.0"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`},
		{"xl/_rels/workbook.xml.rels", `<?xml version="1.0"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/></Relationships>`},
		{"xl/workbook.xml", `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="s" sheetId="1" r:id="rId1"/></sheets></workbook>`},
		{"xl/worksheets/sheet1.xml", `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><dimension ref="A1:ZZZZZZ99999999999999"/><sheetData><row r="0"><c r=""><v>1</v></c><c r="!!"><v>2</v></c><c r="A99999999999999999999"><v>3</v></c></row></sheetData><mergeCells count="1"><mergeCell ref=":"/></mergeCells></worksheet>`},
	}))

	f.Fuzz(func(t *testing.T, data []byte) {
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
	})
}
