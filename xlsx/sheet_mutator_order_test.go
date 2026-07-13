package xlsx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"strings"
	"testing"

	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// C157: sheet mutators that introduce a child element kind absent from the
// original sheet must insert it into the worksheet's ChildOrder at its schema
// position; otherwise the ChildOrder-gated marshal silently drops it.

const mutatorTestContentTypesFmt = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
	`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>%s</Types>`

const mutatorTestRootRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
	`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`

// buildMutatorTestXlsx assembles a minimal in-memory workbook whose sheets
// have the given worksheet XML bodies (sheet1.xml, sheet2.xml, ...).
func buildMutatorTestXlsx(t *testing.T, sheetXMLs ...string) []byte {
	t.Helper()

	var ctOverrides, wbSheets, wbRelEntries strings.Builder
	for i := range sheetXMLs {
		fmt.Fprintf(&ctOverrides, `<Override PartName="/xl/worksheets/sheet%d.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>`, i+1)
		fmt.Fprintf(&wbSheets, `<sheet name="Sheet%d" sheetId="%d" r:id="rId%d"/>`, i+1, i+1, i+1)
		fmt.Fprintf(&wbRelEntries, `<Relationship Id="rId%d" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet%d.xml"/>`, i+1, i+1)
	}

	wbXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
		`<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets>` +
		wbSheets.String() + `</sheets></workbook>`
	wbRels := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		wbRelEntries.String() + `</Relationships>`

	files := []struct{ name, data string }{
		{"[Content_Types].xml", fmt.Sprintf(mutatorTestContentTypesFmt, ctOverrides.String())},
		{"_rels/.rels", mutatorTestRootRels},
		{"xl/workbook.xml", wbXML},
		{"xl/_rels/workbook.xml.rels", wbRels},
	}
	for i, sheetXML := range sheetXMLs {
		files = append(files, struct{ name, data string }{
			fmt.Sprintf("xl/worksheets/sheet%d.xml", i+1), sheetXML,
		})
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range files {
		w, err := zw.Create(f.name)
		if err != nil {
			t.Fatalf("create %s: %v", f.name, err)
		}
		if _, err := w.Write([]byte(f.data)); err != nil {
			t.Fatalf("write %s: %v", f.name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}

// readZipPart returns the named part's bytes from a zipped xlsx.
func readZipPart(t *testing.T, data []byte, name string) []byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open zip: %v", err)
	}
	for _, f := range zr.File {
		if f.Name == name {
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("open %s: %v", name, err)
			}
			var b bytes.Buffer
			if _, err := b.ReadFrom(rc); err != nil {
				t.Fatalf("read %s: %v", name, err)
			}
			if err := rc.Close(); err != nil {
				t.Fatalf("close %s: %v", name, err)
			}
			return b.Bytes()
		}
	}
	t.Fatalf("part %s not found in output", name)
	return nil
}

// sheet without sheetPr/sheetViews/cols/autoFilter/mergeCells/dataValidations.
const mutatorTestSheetBare = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
	`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData><row r="1"><c r="A1"><v>1</v></c></row></sheetData><pageMargins left="0.7" right="0.7" top="0.75" bottom="0.75" header="0.3" footer="0.3"/></worksheet>`

// TestMutatorsInsertMissingWorksheetChildren applies every mutator that can
// introduce a new child kind to a sheet lacking all of them, and verifies
// each element is emitted at its schema position.
func TestMutatorsInsertMissingWorksheetChildren(t *testing.T) {
	data := buildMutatorTestXlsx(t, mutatorTestSheetBare)
	wb, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	sh := wb.Sheets()[0]

	if err := sh.MergeCells("A1", "B2"); err != nil {
		t.Fatalf("MergeCells: %v", err)
	}
	if err := sh.SetColWidth(1, 25); err != nil {
		t.Fatalf("SetColWidth: %v", err)
	}
	if err := sh.FreezePanes("B2"); err != nil {
		t.Fatalf("FreezePanes: %v", err)
	}
	if err := sh.AddDataValidation(DataValidation{Range: "C1:C10", Type: "whole", Operator: "between", Formula1: "1", Formula2: "9"}); err != nil {
		t.Fatalf("AddDataValidation: %v", err)
	}
	if err := sh.SetAutoFilter("A1:B1"); err != nil {
		t.Fatalf("SetAutoFilter: %v", err)
	}
	sh.SetTabColor("FF0000")

	out, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	sheet := string(readZipPart(t, out, "xl/worksheets/sheet1.xml"))

	// Every introduced element must be present exactly once, in schema order.
	order := []string{
		"<sheetPr", "<sheetViews", "<cols", "<sheetData", "<autoFilter",
		"<mergeCells", "<dataValidations", "<pageMargins",
	}
	prev := -1
	for _, tag := range order {
		i := strings.Index(sheet, tag)
		if i < 0 {
			t.Errorf("saved sheet is missing %s:\n%s", tag, sheet)
			continue
		}
		if strings.Count(sheet, tag) != 1 {
			t.Errorf("saved sheet has duplicate %s:\n%s", tag, sheet)
		}
		if i < prev {
			t.Errorf("element %s out of schema order:\n%s", tag, sheet)
		}
		prev = i
	}

	// The result must still parse as a worksheet.
	var ws oxml.CT_Worksheet
	if err := xml.Unmarshal([]byte(sheet), &ws); err != nil {
		t.Fatalf("saved sheet does not parse: %v", err)
	}
	if ws.MergeCells == nil || len(ws.MergeCells.MergeCell) != 1 || ws.MergeCells.MergeCell[0].Ref != "A1:B2" {
		t.Errorf("mergeCells not round-trippable: %+v", ws.MergeCells)
	}
	if len(ws.Cols) != 1 || len(ws.Cols[0].Col) != 1 || ws.Cols[0].Col[0].Width == nil || *ws.Cols[0].Col[0].Width != 25 {
		t.Errorf("cols not round-trippable: %+v", ws.Cols)
	}
	if ws.SheetViews == nil || len(ws.SheetViews.SheetView) != 1 || ws.SheetViews.SheetView[0].Pane == nil {
		t.Errorf("sheetViews/pane not round-trippable: %+v", ws.SheetViews)
	}
}

// TestMutatorOnSheetWithExistingElementNoDuplicate verifies that a mutator on
// a sheet that already has the element extends it rather than emitting a
// second one.
func TestMutatorOnSheetWithExistingElementNoDuplicate(t *testing.T) {
	const sheetWithMerge = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
		`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData><row r="1"><c r="A1"><v>1</v></c></row></sheetData><mergeCells count="1"><mergeCell ref="D1:E1"/></mergeCells><pageMargins left="0.7" right="0.7" top="0.75" bottom="0.75" header="0.3" footer="0.3"/></worksheet>`

	data := buildMutatorTestXlsx(t, sheetWithMerge)
	wb, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	sh := wb.Sheets()[0]
	if err := sh.MergeCells("A1", "B2"); err != nil {
		t.Fatalf("MergeCells: %v", err)
	}

	out, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	sheet := string(readZipPart(t, out, "xl/worksheets/sheet1.xml"))

	if n := strings.Count(sheet, "<mergeCells"); n != 1 {
		t.Errorf("expected exactly one mergeCells element, got %d:\n%s", n, sheet)
	}
	for _, want := range []string{`ref="D1:E1"`, `ref="A1:B2"`, `count="2"`} {
		if !strings.Contains(sheet, want) {
			t.Errorf("saved sheet is missing %s:\n%s", want, sheet)
		}
	}
}

// TestMutatorKeepsUnknownChildrenOrdered verifies the C14/C157 interaction:
// inserting a new known child must not drop or displace captured unknown
// children, and the insertion point respects unknown children whose schema
// rank is derivable from their element name.
func TestMutatorKeepsUnknownChildrenOrdered(t *testing.T) {
	const src = `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">` +
		`<dimension ref="A1"/>` +
		`<sheetData><row r="1"><c r="A1" t="str"><v>hi</v></c></row></sheetData>` +
		`<customSheetViews><customSheetView guid="{123}"/></customSheetViews>` +
		`<oleObjects><oleObject progId="Excel.Sheet"><objectPr/></oleObject></oleObjects>` +
		`</worksheet>`

	var ws oxml.CT_Worksheet
	if err := xml.Unmarshal([]byte(src), &ws); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	s := &Sheet{worksheet: &ws}
	if err := s.MergeCells("A1", "B2"); err != nil {
		t.Fatalf("MergeCells: %v", err)
	}
	if err := s.SetColWidth(1, 25); err != nil {
		t.Fatalf("SetColWidth: %v", err)
	}

	data, err := marshalWorksheetXML(&ws)
	if err != nil {
		t.Fatalf("marshalWorksheetXML: %v", err)
	}
	out := string(data)

	for _, want := range []string{
		"<customSheetViews>", `guid="{123}"`, // unknown children preserved
		"<oleObjects>", `progId="Excel.Sheet"`, "<objectPr",
		"<cols>", "<mergeCells", // new children emitted
	} {
		if !strings.Contains(out, want) {
			t.Errorf("re-marshaled sheet is missing %q:\n%s", want, out)
		}
	}

	// Schema order: cols < sheetData < customSheetViews < mergeCells < oleObjects.
	idx := func(tag string) int { return strings.Index(out, tag) }
	if idx("<cols>") >= idx("<sheetData>") ||
		idx("<sheetData>") >= idx("<customSheetViews>") ||
		idx("<customSheetViews>") >= idx("<mergeCells") ||
		idx("<mergeCells") >= idx("<oleObjects>") {
		t.Errorf("children not in schema order:\n%s", out)
	}
}

// TestUntouchedSheetStaysByteIdentical verifies that mutating one sheet does
// not perturb the preserved bytes of the others.
func TestUntouchedSheetStaysByteIdentical(t *testing.T) {
	const sheet2XML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
		`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData><row r="1"><c r="A1"><v>42</v></c></row></sheetData><pageMargins left="0.7" right="0.7" top="0.75" bottom="0.75" header="0.3" footer="0.3"/></worksheet>`

	data := buildMutatorTestXlsx(t, mutatorTestSheetBare, sheet2XML)
	wb, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	sh := wb.Sheets()[0]
	if err := sh.MergeCells("A1", "B2"); err != nil {
		t.Fatalf("MergeCells: %v", err)
	}
	if err := sh.SetColWidth(1, 25); err != nil {
		t.Fatalf("SetColWidth: %v", err)
	}

	out, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	got := readZipPart(t, out, "xl/worksheets/sheet2.xml")
	if !bytes.Equal(got, []byte(sheet2XML)) {
		t.Errorf("untouched sheet2.xml changed:\nwant: %s\ngot:  %s", sheet2XML, got)
	}

	sheet1 := string(readZipPart(t, out, "xl/worksheets/sheet1.xml"))
	if !strings.Contains(sheet1, "<mergeCells") || !strings.Contains(sheet1, "<cols>") {
		t.Errorf("mutated sheet1.xml is missing new children:\n%s", sheet1)
	}
}
