package xlsx

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"

	"github.com/mgilbir/spine/internal/testutil"
)

// buildPrefixedRootXLSX crafts a minimal workbook whose SpreadsheetML parts
// use a namespace-prefixed root (<x:workbook>, <x:worksheet>), as written by
// some non-Excel generators (Common Crawl corpus, 16 files).
func buildPrefixedRootXLSX(t *testing.T) []byte {
	t.Helper()

	files := map[string]string{
		"[Content_Types].xml":        `<?xml version="1.0" encoding="utf-8"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml" /><Default Extension="xml" ContentType="application/xml" /><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml" /><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.ws()+xml" /></Types>`,
		"_rels/.rels":                `<?xml version="1.0" encoding="utf-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="/xl/workbook.xml" Id="rId1" /></Relationships>`,
		"xl/workbook.xml":            `<?xml version="1.0" encoding="utf-8"?><x:workbook xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:x="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><x:sheets><x:sheet name="Sheet1" sheetId="1" r:id="rId2" /></x:sheets></x:workbook>`,
		"xl/_rels/workbook.xml.rels": `<?xml version="1.0" encoding="utf-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="/xl/worksheets/sheet1.xml" Id="rId2" /></Relationships>`,
		"xl/worksheets/sheet1.xml":   `<?xml version="1.0" encoding="utf-8"?><x:worksheet xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:x="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><x:sheetData><x:row r="1"><x:c r="A1" t="inlineStr"><x:is><x:t>hello</x:t></x:is></x:c></x:row></x:sheetData></x:worksheet>`,
	}

	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, name := range []string{"[Content_Types].xml", "_rels/.rels", "xl/workbook.xml", "xl/_rels/workbook.xml.rels", "xl/worksheets/sheet1.xml"} {
		fw, err := w.Create(name)
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		if _, err := fw.Write([]byte(files[name])); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}

// A workbook whose root elements are namespace-prefixed must round-trip to
// well-formed XML that reopens: the regenerated workbook.xml has to emit its
// root open tag with the same prefix its declarations bind for the
// SpreadsheetML namespace, not an unprefixed open tag closed by a prefixed
// close tag.
func TestPrefixedRootWorkbookRoundTrip(t *testing.T) {
	fixture := buildPrefixedRootXLSX(t)

	wb, err := OpenReader(bytes.NewReader(fixture), int64(len(fixture)))
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	if got, err := wb.Sheets()[0].GetCellValue("A1"); err != nil || got != "hello" {
		t.Fatalf("A1 = %q, %v; want hello", got, err)
	}
	out, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	parts, err := testutil.ReadZipPartsBytes(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	wbXML := string(parts["xl/workbook.xml"])
	if !strings.Contains(wbXML, "<x:workbook ") {
		t.Errorf("regenerated workbook.xml lost the prefixed root:\n%s", wbXML)
	}
	if !strings.Contains(wbXML, "</x:workbook>") {
		t.Errorf("regenerated workbook.xml close tag not prefixed:\n%s", wbXML)
	}

	wb2, err := OpenReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatalf("reopen saved workbook: %v", err)
	}
	if got, err := wb2.Sheets()[0].GetCellValue("A1"); err != nil || got != "hello" {
		t.Errorf("reopened A1 = %q, %v; want hello", got, err)
	}
}

// The dirty-worksheet path regenerates worksheet XML through the same
// preserved-root-attribute machinery; an edited prefixed workbook must also
// reopen with the edit intact.
func TestPrefixedRootWorksheetEditRoundTrip(t *testing.T) {
	fixture := buildPrefixedRootXLSX(t)

	wb, err := OpenReader(bytes.NewReader(fixture), int64(len(fixture)))
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	if err := wb.Sheets()[0].SetCellValue("A1", "edited"); err != nil {
		t.Fatalf("SetCellValue: %v", err)
	}
	out, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	parts, err := testutil.ReadZipPartsBytes(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	wsXML := string(parts["xl/worksheets/sheet1.xml"])
	if !strings.Contains(wsXML, "<x:worksheet ") || !strings.Contains(wsXML, "</x:worksheet>") {
		t.Errorf("regenerated worksheet lost the prefixed root:\n%s", wsXML)
	}

	wb2, err := OpenReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatalf("reopen edited workbook: %v", err)
	}
	if got, err := wb2.Sheets()[0].GetCellValue("A1"); err != nil || got != "edited" {
		t.Errorf("reopened A1 = %q, %v; want edited", got, err)
	}
}
