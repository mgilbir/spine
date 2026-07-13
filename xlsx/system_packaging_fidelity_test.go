package xlsx

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"

	"github.com/mgilbir/spine/internal/testutil"
)

// buildSystemPackagingXLSX crafts a minimal workbook in the style of
// System.IO.Packaging producers (Common Crawl corpus): " />" self-closing
// tags in [Content_Types].xml, an inline xmlns:r declaration on the sheet
// element, mixed-style unknown workbook children, and no docProps/core.xml.
func buildSystemPackagingXLSX(t *testing.T) []byte {
	t.Helper()

	files := map[string]string{
		"[Content_Types].xml":        `<?xml version="1.0" encoding="utf-8"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml" /><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml" /><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml" /></Types>`,
		"_rels/.rels":                `<?xml version="1.0" encoding="utf-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="/xl/workbook.xml" Id="rId1" /></Relationships>`,
		"xl/workbook.xml":            `<?xml version="1.0" encoding="utf-8"?><x:workbook xmlns:x="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><xr:revisionPtr revIDLastSave="0" xmlns:xr="http://schemas.microsoft.com/office/spreadsheetml/2014/revision" /><x:sheets><x:sheet name="Sheet1" sheetId="1" r:id="rId2" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" /></x:sheets></x:workbook>`,
		"xl/_rels/workbook.xml.rels": `<?xml version="1.0" encoding="utf-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="/xl/worksheets/sheet1.xml" Id="rId2" /></Relationships>`,
		"xl/worksheets/sheet1.xml":   `<?xml version="1.0" encoding="utf-8"?><x:worksheet xmlns:x="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><x:sheetData><x:row r="1"><x:c r="A1" t="inlineStr"><x:is><x:t>hello</x:t></x:is></x:c></x:row></x:sheetData></x:worksheet>`,
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

// A zero-modification save of a System.IO.Packaging-style workbook must not
// synthesize docProps/core.xml, must reproduce the source's " />" style in
// the regenerated [Content_Types].xml, must keep the sheet's inline xmlns:r
// declaration, and must re-emit unknown workbook children verbatim.
func TestSystemPackagingStyleRoundTrip(t *testing.T) {
	fixture := buildSystemPackagingXLSX(t)

	wb, err := OpenReader(bytes.NewReader(fixture), int64(len(fixture)))
	if err != nil {
		t.Fatalf("open fixture: %v", err)
	}
	out, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	if _, err := OpenReader(bytes.NewReader(out), int64(len(out))); err != nil {
		t.Fatalf("saved workbook does not reopen: %v", err)
	}

	parts, err := testutil.ReadZipPartsBytes(out)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	if _, ok := parts["docProps/core.xml"]; ok {
		t.Errorf("save synthesized docProps/core.xml the source never had")
	}

	ct := string(parts["[Content_Types].xml"])
	if !strings.Contains(ct, `ContentType="application/vnd.openxmlformats-package.relationships+xml" />`) {
		t.Errorf("[Content_Types].xml lost the source's \" />\" self-closing style:\n%s", ct)
	}

	wbXML := string(parts["xl/workbook.xml"])
	if !strings.Contains(wbXML, `<x:sheet name="Sheet1" sheetId="1" r:id="rId2" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" />`) {
		t.Errorf("sheet inline xmlns:r declaration not preserved:\n%s", wbXML)
	}
	if !strings.Contains(wbXML, `<xr:revisionPtr revIDLastSave="0" xmlns:xr="http://schemas.microsoft.com/office/spreadsheetml/2014/revision" />`) {
		t.Errorf("unknown workbook child not re-emitted verbatim:\n%s", wbXML)
	}
}
