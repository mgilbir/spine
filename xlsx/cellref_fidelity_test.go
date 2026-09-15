package xlsx

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"
)

// readPartFromZip returns one part's bytes from a saved package.
func readPartFromZip(pkg []byte, name string) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(pkg), int64(len(pkg)))
	if err != nil {
		return "", err
	}
	for _, f := range zr.File {
		if f.Name != name {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return "", err
		}
		defer func() { _ = rc.Close() }()
		b, err := io.ReadAll(rc)
		return string(b), err
	}
	return "", io.EOF
}

// openWorksheetBody opens a workbook whose single sheet has the given
// <sheetData>, dirties it so a save regenerates the part from the model rather
// than re-emitting the preserved bytes, saves, and returns the regenerated
// sheet XML.
func regeneratedSheetXML(t *testing.T, body string) string {
	t.Helper()
	data := buildXLSXWithWorksheet(t, body)
	wb, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	sh, err := wb.Sheet(0)
	if err != nil {
		t.Fatal(err)
	}
	if err := sh.SetCellValue("Z99", "dirty"); err != nil {
		t.Fatal(err)
	}
	out, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	xml, err := readPartFromZip(out, "xl/worksheets/sheet1.xml")
	if err != nil {
		t.Fatal(err)
	}
	return xml
}

// TestNonCanonicalRefRoundTripsVerbatim is the fallback the derived reference
// depends on. Every one of the corpus's 25.6M references is already canonical,
// so nothing there would notice a cell whose r is spelled differently — this
// does. "A01" must come back as "A01", not rebuilt as "A1".
func TestNonCanonicalRefRoundTripsVerbatim(t *testing.T) {
	got := regeneratedSheetXML(t, `<sheetData><row r="1">`+
		`<c r="A01"><v>1</v></c>`+
		`<c r="b1"><v>2</v></c>`+
		`</row></sheetData>`)

	for _, want := range []string{`r="A01"`, `r="b1"`} {
		if !strings.Contains(got, want) {
			t.Errorf("regenerated sheet lost the original spelling %s\n%s", want, got)
		}
	}
	if strings.Contains(got, `r="A1"`) {
		t.Errorf(`"A01" was rewritten to the canonical "A1"`+"\n%s", got)
	}
}

// TestCellWithoutRefKeepsOmittingIt covers C368 from the other side: r is
// optional, and a cell that had none must not gain one now that the reference
// is rebuilt from a stored position.
func TestCellWithoutRefKeepsOmittingIt(t *testing.T) {
	got := regeneratedSheetXML(t, `<sheetData><row r="1">`+
		`<c><v>1</v></c>`+
		`</row></sheetData>`)

	// The dirtying cell Z99 legitimately carries r; the row 1 cell must not.
	row1 := got
	if i := strings.Index(got, `<row r="1"`); i >= 0 {
		if j := strings.Index(got[i:], "</row>"); j >= 0 {
			row1 = got[i : i+j]
		}
	}
	if strings.Contains(row1, " r=") && strings.Count(row1, " r=") > 1 {
		t.Errorf("a cell that had no r attribute gained one:\n%s", row1)
	}
}

// TestMalformedRefRoundTripsAndStaysUnaddressable pins the third case: text
// that is not a reference at all is kept verbatim, and the cell it belongs to
// is not reachable by position — which is how it behaved when the reference was
// compared as a string.
func TestMalformedRefRoundTripsAndStaysUnaddressable(t *testing.T) {
	data := buildXLSXWithWorksheet(t, `<sheetData><row r="1">`+
		`<c r="A1"><v>1</v></c><c r="notaref"><v>2</v></c>`+
		`</row></sheetData>`)
	wb, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	sh, err := wb.Sheet(0)
	if err != nil {
		t.Fatal(err)
	}
	if got := sh.FindCell("A1"); got == nil || got.String() != "1" {
		t.Fatalf("A1 = %v, want 1", got)
	}
	// The malformed cell must not answer to any position.
	ws := sh.ws()
	for _, c := range ws.SheetData.Row[0].C {
		if c.Ref() == "notaref" {
			if _, _, ok := c.RowCol(); ok {
				t.Error("a cell whose r does not parse became addressable by position")
			}
		}
	}

	if err := sh.SetCellValue("D4", "dirty"); err != nil {
		t.Fatal(err)
	}
	out, err := wb.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	xml, err := readPartFromZip(out, "xl/worksheets/sheet1.xml")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(xml, `r="notaref"`) {
		t.Errorf("malformed reference lost on a dirty save:\n%s", xml)
	}
}
