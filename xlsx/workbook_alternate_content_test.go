package xlsx

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// C319: a root-level mc:AlternateContent is repeatable. A single-pointer field
// collapsed two distinct blocks to the last while both ChildOrder entries
// remained, so a zero-mod save of an always-regenerated workbook.xml
// duplicated one block and dropped the other. Both must survive, in order.
func TestWorkbookRepeatedAlternateContentPreserved(t *testing.T) {
	const src = `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"` +
		` xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"` +
		` xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006"` +
		` xmlns:x15="http://schemas.microsoft.com/office/spreadsheetml/2010/11/main">` +
		`<mc:AlternateContent><mc:Choice Requires="x15"><x15:AAA/></mc:Choice></mc:AlternateContent>` +
		`<sheets><sheet name="S" sheetId="1" r:id="rId1"/></sheets>` +
		`<mc:AlternateContent><mc:Choice Requires="x15"><x15:BBB/></mc:Choice></mc:AlternateContent>` +
		`</workbook>`

	var wb oxml.CT_Workbook
	if err := xml.Unmarshal([]byte(src), &wb); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	data, err := marshalWorkbookXML(&wb)
	if err != nil {
		t.Fatalf("marshalWorkbookXML: %v", err)
	}
	out := string(data)
	if strings.Count(out, "x15:AAA") != 1 {
		t.Errorf("first AlternateContent block not emitted exactly once:\n%s", out)
	}
	if strings.Count(out, "x15:BBB") != 1 {
		t.Errorf("second AlternateContent block not emitted exactly once:\n%s", out)
	}
	if i, j := strings.Index(out, "x15:AAA"), strings.Index(out, "x15:BBB"); i < 0 || j < 0 || i > j {
		t.Errorf("AlternateContent blocks out of order:\n%s", out)
	}
}
