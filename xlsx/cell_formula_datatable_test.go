package xlsx

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// C273: CT_CellFormula modeled only t/aca/ref/ca/si, so a data-table formula
// (<f t="dataTable" ...>) lost its r1/r2/dt2D/dtr/del1/del2/bx attributes on a
// dirty save, corrupting the what-if table. CapturedAttrs must round-trip the
// unmodeled attributes.
func TestCellFormulaDataTableAttrsPreserved(t *testing.T) {
	const src = `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">` +
		`<sheetData>` +
		`<row r="1"><c r="A1"><v>1</v></c></row>` +
		`<row r="2">` +
		`<c r="A2"><v>2</v></c>` +
		`<c r="B2"><f t="dataTable" ref="B2:C3" dt2D="1" dtr="0" r1="A1" r2="A2" del1="0" del2="0" bx="1"/></c>` +
		`</row>` +
		`</sheetData>` +
		`</worksheet>`

	var ws oxml.CT_Worksheet
	if err := xml.Unmarshal([]byte(src), &ws); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// Simulate a dirty save touching an unrelated cell: re-marshal from model.
	data, err := marshalWorksheetXML(&ws)
	if err != nil {
		t.Fatalf("marshalWorksheetXML: %v", err)
	}
	out := string(data)
	for _, want := range []string{
		`t="dataTable"`, `ref="B2:C3"`, `dt2D="1"`, `dtr="0"`,
		`r1="A1"`, `r2="A2"`, `del1="0"`, `del2="0"`, `bx="1"`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("data-table formula lost %s:\n%s", want, out)
		}
	}
}
