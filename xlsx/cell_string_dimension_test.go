package xlsx

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

// C129: literal strings must be stored as inline strings, not t="str" (the
// cached-formula-result type).
func TestSetStringUsesInlineString(t *testing.T) {
	wb := Create()
	sheet := addSheetT(wb, "Sheet1")
	cell, err := sheet.Cell("A1")
	if err != nil {
		t.Fatal(err)
	}
	cell.SetString("hello")

	if cell.cell.T != "inlineStr" {
		t.Errorf("cell type = %q, want inlineStr", cell.cell.T)
	}
	if cell.cell.V != nil {
		t.Errorf("cell has <v> %q; inline strings use <is>", *cell.cell.V)
	}
	if cell.String() != "hello" {
		t.Errorf("String() = %q, want hello", cell.String())
	}
	if cell.Type() != CellTypeString {
		t.Errorf("Type() = %v, want CellTypeString", cell.Type())
	}

	// Overwriting the string with a number must drop the inline string.
	cell.SetValue(3)
	if cell.cell.Is != nil {
		t.Error("numeric overwrite left the inline string in place")
	}
}

// C129: strings with leading/trailing whitespace must survive a save/reopen
// round-trip via xml:space="preserve".
func TestSetStringPreservesEdgeWhitespace(t *testing.T) {
	wb := Create()
	sheet := addSheetT(wb, "Sheet1")
	if err := sheet.SetCellValue("A1", "  padded  "); err != nil {
		t.Fatal(err)
	}
	saved, err := wb.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}

	sheetXML := string(readZipPart(t, saved, "xl/worksheets/sheet1.xml"))
	if !strings.Contains(sheetXML, `xml:space="preserve"`) {
		t.Errorf("sheet XML lacks xml:space=\"preserve\":\n%s", sheetXML)
	}

	reopened, err := OpenReader(bytes.NewReader(saved), int64(len(saved)))
	if err != nil {
		t.Fatal(err)
	}
	s2, err := reopened.Sheet(0)
	if err != nil {
		t.Fatal(err)
	}
	got, err := s2.GetCellValue("A1")
	if err != nil {
		t.Fatal(err)
	}
	if got != "  padded  " {
		t.Errorf("round-tripped value = %q, want %q", got, "  padded  ")
	}
}

// C117: cell writes past the recorded used range must refresh the dimension
// element of the regenerated sheet.
func TestDimensionUpdatedAfterCellWrite(t *testing.T) {
	sheetXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
		`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><dimension ref="A1:B2"/><sheetData><row r="1"><c r="A1"><v>1</v></c><c r="B1"><v>2</v></c></row><row r="2"><c r="A2"><v>3</v></c><c r="B2"><v>4</v></c></row></sheetData></worksheet>`
	data := buildMutatorTestXlsx(t, sheetXML)
	wb, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	sheet, err := wb.Sheet(0)
	if err != nil {
		t.Fatal(err)
	}
	if err := sheet.SetCellValue("Z99", 42); err != nil {
		t.Fatal(err)
	}
	saved, err := wb.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	out := string(readZipPart(t, saved, "xl/worksheets/sheet1.xml"))
	if !strings.Contains(out, `<dimension ref="A1:Z99"/>`) {
		t.Errorf("dimension not refreshed after writing Z99:\n%s", out)
	}
}

// C117: an untouched sheet must keep its original (even stale) bytes.
func TestDimensionUntouchedSheetKeepsBytes(t *testing.T) {
	sheetXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
		`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><dimension ref="A1:B2"/><sheetData/></worksheet>`
	data := buildMutatorTestXlsx(t, sheetXML)
	wb, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	saved, err := wb.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	got := string(readZipPart(t, saved, "xl/worksheets/sheet1.xml"))
	if got != sheetXML {
		t.Errorf("untouched sheet regenerated:\n got: %s\nwant: %s", got, sheetXML)
	}
}

// C132: numeric cells styled with a built-in date number format must report
// CellTypeDate (previously unreachable) and expose the time via Value().
func TestCellTypeDateDetection(t *testing.T) {
	wb := Create()
	sheet := addSheetT(wb, "Sheet1")
	cell, err := sheet.Cell("A1")
	if err != nil {
		t.Fatal(err)
	}
	when := time.Date(2026, 7, 12, 0, 0, 0, 0, time.UTC)
	cell.SetTime(when)
	if err := cell.SetStyle(CellStyle{Format: "mm-dd-yy"}); err != nil { // builtin id 14
		t.Fatal(err)
	}

	if got := cell.Type(); got != CellTypeDate {
		t.Fatalf("Type() = %v, want CellTypeDate", got)
	}
	v, ok := cell.Value().(time.Time)
	if !ok {
		t.Fatalf("Value() = %T, want time.Time", cell.Value())
	}
	if !v.Equal(when) {
		t.Errorf("Value() = %v, want %v", v, when)
	}

	// A plain numeric cell must stay CellTypeNumber.
	num, err := sheet.Cell("B1")
	if err != nil {
		t.Fatal(err)
	}
	num.SetValue(42)
	if got := num.Type(); got != CellTypeNumber {
		t.Errorf("plain numeric Type() = %v, want CellTypeNumber", got)
	}
}
