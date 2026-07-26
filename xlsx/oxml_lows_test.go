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

// An unparsable row index (r="abc") must not be coerced to the schema-invalid
// r="0": the numeric parse discarded its error while assigning the pointer
// unconditionally, so a garbage r attribute produced <row r="0">. The field is
// now left unset when the value does not parse.
func TestRowSkipsUnparsableIndex(t *testing.T) {
	const src = `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">` +
		`<sheetData><row r="abc"><c r="A1"><v>0</v></c></row></sheetData>` +
		`</worksheet>`

	var ws oxml.CT_Worksheet
	if err := xml.Unmarshal([]byte(src), &ws); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(ws.SheetData.Row) != 1 {
		t.Fatalf("expected 1 row, got %d", len(ws.SheetData.Row))
	}
	if r := ws.SheetData.Row[0].R; r != nil {
		t.Errorf("unparsable r=\"abc\" set R=%d, want nil", *r)
	}
	data, err := marshalWorksheetXML(&ws)
	if err != nil {
		t.Fatalf("marshalWorksheetXML: %v", err)
	}
	if out := string(data); strings.Contains(out, `r="0"`) {
		t.Errorf("unparsable row index emitted schema-invalid r=\"0\":\n%s", out)
	}
}

// A default-namespace declaration on the ext element itself
// (<ext xmlns="..." uri="...">) must survive a dirty save. CT_Extension only
// captured prefixed xmlns declarations (Space=="xmlns"), so the default form
// (Space=="" && Local=="xmlns") was dropped on replay.
func TestExtensionPreservesDefaultNamespaceDecl(t *testing.T) {
	const src = `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">` +
		`<sheetData/>` +
		`<extLst><ext xmlns="urn:example:ext" uri="{ABC}"><child/></ext></extLst>` +
		`</worksheet>`

	var ws oxml.CT_Worksheet
	if err := xml.Unmarshal([]byte(src), &ws); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	data, err := marshalWorksheetXML(&ws)
	if err != nil {
		t.Fatalf("marshalWorksheetXML: %v", err)
	}
	if out := string(data); !strings.Contains(out, `<ext xmlns="urn:example:ext" uri="{ABC}">`) {
		t.Errorf("ext default-namespace declaration dropped on re-marshal:\n%s", out)
	}
}

// A present-but-empty <numFmts count="0"/> must survive a style-edit
// regeneration: the marshaler gated emission on a non-empty NumFmt slice, so
// the element vanished when styles.xml was rebuilt.
func TestEmptyNumFmtsSurvivesStyleRegeneration(t *testing.T) {
	const src = `<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">` +
		`<numFmts count="0"/>` +
		`<fonts count="1"><font><sz val="11"/></font></fonts>` +
		`</styleSheet>`

	var ss oxml.CT_Stylesheet
	if err := xml.Unmarshal([]byte(src), &ss); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	data, err := marshalStylesheetXML(&ss)
	if err != nil {
		t.Fatalf("marshalStylesheetXML: %v", err)
	}
	out := string(data)
	if !strings.Contains(out, "numFmts") || !strings.Contains(out, `count="0"`) {
		t.Errorf("empty numFmts dropped on style regeneration:\n%s", out)
	}
}
