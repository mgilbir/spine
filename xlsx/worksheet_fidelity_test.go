package xlsx

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// C14: unknown worksheet children (oleObjects, controls, customSheetViews, ...)
// must survive a re-marshal of a dirty sheet rather than being dropped.
func TestWorksheetPreservesUnknownChildren(t *testing.T) {
	const src = `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"` +
		` xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006" mc:Ignorable="x14ac">` +
		`<dimension ref="A1"/>` +
		`<sheetData><row r="1"><c r="A1" t="str"><v>hi</v></c></row></sheetData>` +
		`<oleObjects><oleObject progId="Excel.Sheet"><objectPr/></oleObject></oleObjects>` +
		`<customSheetViews><customSheetView guid="{123}"/></customSheetViews>` +
		`</worksheet>`

	var ws oxml.CT_Worksheet
	if err := xml.Unmarshal([]byte(src), &ws); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	data, merr := marshalWorksheetXML(&ws)
	if merr != nil {
		t.Fatalf("marshalWorksheetXML: %v", merr)
	}
	out := string(data)

	for _, want := range []string{
		"<oleObjects>", `progId="Excel.Sheet"`, "<objectPr", // unknown subtree preserved verbatim
		"<customSheetViews>", `guid="{123}"`,
		`mc:Ignorable="x14ac"`, // root attribute preserved
		`r="A1"`,               // known content still emitted
	} {
		if !strings.Contains(out, want) {
			t.Errorf("re-marshaled sheet is missing %q:\n%s", want, out)
		}
	}

	// The unknown children must appear after sheetData (original order preserved).
	if i, j := strings.Index(out, "</sheetData>"), strings.Index(out, "<oleObjects>"); i < 0 || j < 0 || i > j {
		t.Errorf("unknown children not in original position:\n%s", out)
	}
}

// C201: an unknown child that declares its namespace prefix inline (rather
// than at the root) must be re-encoded with its element and attribute
// prefixes intact — stripping them would silently move the content into the
// default sml namespace.
func TestWorksheetPreservesUnknownChildWithInlineNamespace(t *testing.T) {
	const src = `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">` +
		`<sheetData/>` +
		`<foo:custom xmlns:foo="urn:foo" foo:val="1"><foo:inner/></foo:custom>` +
		`</worksheet>`

	var ws oxml.CT_Worksheet
	if err := xml.Unmarshal([]byte(src), &ws); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	data, merr := marshalWorksheetXML(&ws)
	if merr != nil {
		t.Fatalf("marshalWorksheetXML: %v", merr)
	}
	out := string(data)

	if !strings.Contains(out, `<foo:custom xmlns:foo="urn:foo" foo:val="1"><foo:inner/></foo:custom>`) {
		t.Errorf("inline-namespaced unknown child not re-encoded byte-faithfully:\n%s", out)
	}
	for _, banned := range []string{"<custom", ` val="1"`} {
		if strings.Contains(out, banned) {
			t.Errorf("unknown child lost its prefix (%q found):\n%s", banned, out)
		}
	}
}
