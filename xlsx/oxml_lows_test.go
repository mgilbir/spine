package xlsx

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// A cell in a Japanese phonetic-guide workbook carries ph="1"; the attribute is
// the sixth CT_Cell attribute (r, s, t, cm, vm, ph) and must survive a dirty
// save rather than being silently dropped.
func TestCellPreservesPhoneticAttr(t *testing.T) {
	const src = `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">` +
		`<sheetData><row r="1"><c r="A1" ph="1"><v>0</v></c></row></sheetData>` +
		`</worksheet>`

	var ws oxml.CT_Worksheet
	if err := xml.Unmarshal([]byte(src), &ws); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	data, err := marshalWorksheetXML(&ws)
	if err != nil {
		t.Fatalf("marshalWorksheetXML: %v", err)
	}
	if out := string(data); !strings.Contains(out, `ph="1"`) {
		t.Errorf("cell ph attribute dropped on re-marshal:\n%s", out)
	}
}

// An inline string carrying Japanese phonetic runs (rPh) must round-trip them
// on a dirty save; CT_Rst previously modeled only t, r and phoneticPr, so the
// rPh runs were silently dropped when the sheet was regenerated (C134).
func TestInlineStringPreservesPhoneticRuns(t *testing.T) {
	const src = `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">` +
		`<sheetData><row r="1"><c r="A1" t="inlineStr"><is>` +
		`<t>課</t><rPh sb="0" eb="1"><t>カ</t></rPh><phoneticPr fontId="1"/>` +
		`</is></c></row></sheetData>` +
		`</worksheet>`

	var ws oxml.CT_Worksheet
	if err := xml.Unmarshal([]byte(src), &ws); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	data, err := marshalWorksheetXML(&ws)
	if err != nil {
		t.Fatalf("marshalWorksheetXML: %v", err)
	}
	out := string(data)
	for _, want := range []string{`<rPh sb="0" eb="1">`, `<t>カ</t>`} {
		if !strings.Contains(out, want) {
			t.Errorf("inline string lost phonetic run %q:\n%s", want, out)
		}
	}
}
