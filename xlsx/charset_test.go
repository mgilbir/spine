package xlsx

import (
	"archive/zip"
	"bytes"
	"testing"
)

// buildCharsetXLSX crafts a minimal workbook whose worksheet part declares a
// non-UTF-8 charset in its XML prolog. Wild files harvested from the web carry
// us-ascii/Windows-1252/ISO-8859-1 declarations (Office opens them); the
// library must decode them rather than reject the part outright.
func buildCharsetXLSX(t *testing.T, sheetPart []byte) []byte {
	t.Helper()

	ct := `<?xml version="1.0" encoding="utf-8"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml" /><Default Extension="xml" ContentType="application/xml" /><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml" /><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.ws()+xml" /></Types>`
	rootRels := `<?xml version="1.0" encoding="utf-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="/xl/workbook.xml" Id="rId1" /></Relationships>`
	workbook := `<?xml version="1.0" encoding="utf-8"?><workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Sheet1" sheetId="1" r:id="rId2" /></sheets></workbook>`
	wbRels := `<?xml version="1.0" encoding="utf-8"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="/xl/worksheets/sheet1.xml" Id="rId2" /></Relationships>`

	order := []string{"[Content_Types].xml", "_rels/.rels", "xl/workbook.xml", "xl/_rels/workbook.xml.rels", "xl/worksheets/sheet1.xml"}
	files := map[string][]byte{
		"[Content_Types].xml":        []byte(ct),
		"_rels/.rels":                []byte(rootRels),
		"xl/workbook.xml":            []byte(workbook),
		"xl/_rels/workbook.xml.rels": []byte(wbRels),
		"xl/worksheets/sheet1.xml":   sheetPart,
	}

	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for _, name := range order {
		fw, err := w.Create(name)
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		if _, err := fw.Write(files[name]); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}

func TestOpenNonUTF8Worksheet(t *testing.T) {
	tests := []struct {
		name  string
		sheet []byte
		want  string
	}{
		{
			name:  "us-ascii",
			sheet: []byte(`<?xml version="1.0" encoding="us-ascii"?><worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData><row r="1"><c r="A1" t="inlineStr"><is><t>hello</t></is></c></row></sheetData></worksheet>`),
			want:  "hello",
		},
		{
			// 0x92 is the Windows-1252 right single quotation mark (U+2019).
			name:  "windows-1252",
			sheet: []byte("<?xml version=\"1.0\" encoding=\"Windows-1252\"?><worksheet xmlns=\"http://schemas.openxmlformats.org/spreadsheetml/2006/main\"><sheetData><row r=\"1\"><c r=\"A1\" t=\"inlineStr\"><is><t>John\x92s</t></is></c></row></sheetData></worksheet>"),
			want:  "John’s",
		},
		{
			// 0xE9 is é in ISO-8859-1 (U+00E9).
			name:  "iso-8859-1",
			sheet: []byte("<?xml version=\"1.0\" encoding=\"ISO-8859-1\"?><worksheet xmlns=\"http://schemas.openxmlformats.org/spreadsheetml/2006/main\"><sheetData><row r=\"1\"><c r=\"A1\" t=\"inlineStr\"><is><t>caf\xe9</t></is></c></row></sheetData></worksheet>"),
			want:  "café",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := buildCharsetXLSX(t, tt.sheet)
			wb, err := OpenReader(bytes.NewReader(fixture), int64(len(fixture)))
			if err != nil {
				t.Fatalf("open %s fixture: %v", tt.name, err)
			}
			got, err := wb.Sheets()[0].GetCellValue("A1")
			if err != nil {
				t.Fatalf("GetCellValue: %v", err)
			}
			if got != tt.want {
				t.Errorf("A1 = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestNonUTF8PartPreservedByteIdentical verifies the fidelity claim: a
// non-UTF-8 worksheet part is preserved as raw bytes and re-emitted unchanged
// on a zero-mod save, even though the model decoded its text through the
// CharsetReader.
func TestNonUTF8PartPreservedByteIdentical(t *testing.T) {
	sheet := []byte("<?xml version=\"1.0\" encoding=\"Windows-1252\"?><worksheet xmlns=\"http://schemas.openxmlformats.org/spreadsheetml/2006/main\"><sheetData><row r=\"1\"><c r=\"A1\" t=\"inlineStr\"><is><t>John\x92s</t></is></c></row></sheetData></worksheet>")
	fixture := buildCharsetXLSX(t, sheet)

	wb, err := OpenReader(bytes.NewReader(fixture), int64(len(fixture)))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	out, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatalf("zip read: %v", err)
	}
	var found bool
	for _, f := range zr.File {
		if f.Name != "xl/worksheets/sheet1.xml" {
			continue
		}
		found = true
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open part: %v", err)
		}
		var b bytes.Buffer
		if _, err := b.ReadFrom(rc); err != nil {
			t.Fatalf("read part: %v", err)
		}
		_ = rc.Close()
		if !bytes.Equal(b.Bytes(), sheet) {
			t.Errorf("preserved worksheet bytes changed on zero-mod save:\n got %q\nwant %q", b.Bytes(), sheet)
		}
	}
	if !found {
		t.Fatal("worksheet part missing from output")
	}
}
