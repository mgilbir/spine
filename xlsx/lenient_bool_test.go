package xlsx

import (
	"strings"
	"testing"

	xmlb "github.com/mgilbir/spine/common/xml"
	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// C322: BoolLex hard-errored on any spelling but 1/0/true/false, and it backs
// workbookPr/calcPr/bookView booleans, so a single out-of-schema ST_OnOff value
// (date1904="on") failed the whole Open. Parsing must be lenient and preserve
// the original spelling on round-trip.
func TestWorkbookLenientOnOffBooleans(t *testing.T) {
	const src = `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"` +
		` xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">` +
		`<workbookPr date1904="on"/>` +
		`<bookViews><workbookView showHorizontalScroll="off" xWindow="0"/></bookViews>` +
		`<sheets><sheet name="S" sheetId="1" r:id="rId1"/></sheets>` +
		`</workbook>`

	var wb oxml.CT_Workbook
	if err := xmlb.UnmarshalWithSource([]byte(src), &wb); err != nil {
		t.Fatalf("Open failed on odd boolean spelling: %v", err)
	}
	data, err := marshalWorkbookXML(&wb)
	if err != nil {
		t.Fatalf("marshalWorkbookXML: %v", err)
	}
	out := string(data)
	if !strings.Contains(out, `date1904="on"`) {
		t.Errorf("date1904 spelling not preserved:\n%s", out)
	}
	if !strings.Contains(out, `showHorizontalScroll="off"`) {
		t.Errorf("showHorizontalScroll spelling not preserved:\n%s", out)
	}
}
