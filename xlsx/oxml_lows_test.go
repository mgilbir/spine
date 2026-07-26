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
