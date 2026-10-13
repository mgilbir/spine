package xlsx

import (
	"bytes"
	"maps"
	"slices"
	"strings"
	"testing"

	xmlb "github.com/mgilbir/spine/common/xml"
	"github.com/mgilbir/spine/internal/fuzzseed"
	"github.com/mgilbir/spine/internal/testutil"
	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// TestEncodedPackageRoundTrip covers a workbook whose parts are not UTF-8: in
// a single-byte code page, or in UTF-16, which OPC requires readers to accept
// and the library could not open. The text reads correctly; an untouched save keeps
// every part's bytes (workbook.xml is rebuilt on every save and must still
// prefer the source when nothing changed); and an edited save must read back
// the same text.
func TestEncodedPackageRoundTrip(t *testing.T) {
	w := Create()
	s, err := w.AddSheet("Données")
	if err != nil {
		t.Fatal(err)
	}
	if err = s.SetCellValue("A1", "Café ‘quoted’ €5"); err != nil {
		t.Fatal(err)
	}
	w.Properties.Title = "Résumé"
	created, err := w.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	// Excel keeps strings in a shared string table.
	sheet := string(fuzzseed.ZipEntry(created, "xl/worksheets/sheet1.xml"))
	rels := string(fuzzseed.ZipEntry(created, "xl/_rels/workbook.xml.rels"))
	types := string(fuzzseed.ZipEntry(created, "[Content_Types].xml"))
	plain := fuzzseed.EditZip(created, [][2]string{
		{"xl/worksheets/sheet1.xml", strings.Replace(sheet, `</row></sheetData>`, `</row><row r="2"><c r="A2" t="s"><v>0</v></c></row></sheetData>`, 1)},
		{"xl/_rels/workbook.xml.rels", strings.Replace(rels, "</Relationships>", `<Relationship Id="rId9" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/sharedStrings" Target="sharedStrings.xml"/></Relationships>`, 1)},
		{"[Content_Types].xml", strings.Replace(types, "</Types>", `<Override PartName="/xl/sharedStrings.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sharedStrings+xml"/></Types>`, 1)},
		{"xl/sharedStrings.xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" + `<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" count="1" uniqueCount="1"><si><t>Zoë</t></si></sst>`},
	})
	for _, enc := range []struct {
		name   string
		encode func([]byte) ([]byte, error)
	}{
		{"windows-1252", func(b []byte) ([]byte, error) {
			src, nonASCII, err := testutil.WithWindows1252(b)
			for _, part := range []string{"xl/workbook.xml", "xl/worksheets/sheet1.xml", "xl/sharedStrings.xml", "docProps/core.xml"} {
				if err == nil && !slices.Contains(nonASCII, part) {
					t.Fatalf("fixture has no non-ASCII %s: %v", part, nonASCII)
				}
			}
			return src, err
		}},
		{"UTF-16LE", func(b []byte) ([]byte, error) { return testutil.WithUTF16(b, false) }},
		{"UTF-16BE", func(b []byte) ([]byte, error) { return testutil.WithUTF16(b, true) }},
	} {
		t.Run(enc.name, func(t *testing.T) {
			src, err := enc.encode(plain)
			if err != nil {
				t.Fatal(err)
			}
			encodedRoundTrip(t, src)
		})
	}
}

func encodedRoundTrip(t *testing.T, src []byte) {
	check := func(label string, data []byte, cells map[string]string) {
		t.Helper()
		book, err := OpenReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			t.Fatalf("%s: %v", label, err)
		}
		defer func() { _ = book.Close() }()
		sheet, err := book.SheetByName("Données")
		if err != nil {
			t.Fatalf("%s: %v", label, err)
		}
		for ref, want := range cells {
			if got, err := sheet.GetCellValue(ref); err != nil || got != want {
				t.Errorf("%s: %s = %q (%v), want %q", label, ref, got, err, want)
			}
		}
		if book.Properties.Title != "Résumé" {
			t.Errorf("%s: title %q", label, book.Properties.Title)
		}
	}
	cells := map[string]string{"A1": "Café ‘quoted’ €5", "A2": "Zoë"}
	check("source", src, cells)

	// An untouched save keeps every part's bytes, also after reading the cells.
	for _, read := range []bool{false, true} {
		book, err := OpenReader(bytes.NewReader(src), int64(len(src)))
		if err != nil {
			t.Fatal(err)
		}
		if read {
			_ = book.Text()
		}
		untouched, err := book.SaveBytes()
		_ = book.Close()
		if err != nil {
			t.Fatal(err)
		}
		if want, got := unzipParts(t, src), unzipParts(t, untouched); !maps.EqualFunc(want, got, bytes.Equal) {
			for name := range want {
				if !bytes.Equal(want[name], got[name]) {
					t.Errorf("untouched save (read=%v) changed %s", read, name)
				}
			}
		}
	}

	book, err := OpenReader(bytes.NewReader(src), int64(len(src)))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = book.Close() }()

	sheet1, err := book.SheetByName("Données")
	if err != nil {
		t.Fatal(err)
	}
	if err = sheet1.SetCellValue("B1", "Ça va"); err != nil {
		t.Fatal(err)
	}
	if _, err = book.AddSheet("Résultats"); err != nil {
		t.Fatal(err)
	}
	book.Properties.Subject = "Ü"
	edited, err := book.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	cells["B1"] = "Ça va"
	check("edited", edited, cells)
	reopened, err := OpenReader(bytes.NewReader(edited), int64(len(edited)))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = reopened.Close() }()
	if _, err = reopened.SheetByName("Résultats"); err != nil {
		t.Errorf("edited: added sheet: %v", err)
	}
}

// A workbookView's inner content is kept unless it is XML white space; a
// no-break space is text, which strings.TrimSpace classed as white space and
// dropped from the always-regenerated workbook.xml.
func TestBookViewKeepsNoBreakSpaceContent(t *testing.T) {
	src := []byte(`<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">` +
		"<bookViews><workbookView activeTab=\"0\"> </workbookView></bookViews>" +
		`<sheets><sheet name="S" sheetId="1" r:id="rId1"/></sheets></workbook>`)
	var wb oxml.CT_Workbook
	wb.RawSource = src
	if err := xmlb.UnmarshalWithSource(src, &wb); err != nil {
		t.Fatal(err)
	}
	out, err := marshalWorkbookXML(&wb)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out, []byte(" </workbookView>")) {
		t.Errorf("no-break space content dropped: %s", out)
	}
}
