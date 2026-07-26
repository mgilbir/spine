package xlsx

import (
	"strings"
	"testing"

	xmlb "github.com/mgilbir/spine/common/xml"
	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// C321: CT_SheetView modeled only pane/selection, so a dirty save dropped a
// <pivotSelection> child (and an inner extLst). The unmodeled children must be
// captured raw and re-emitted, with pane/selection still preserved.
func TestSheetViewPivotSelectionPreserved(t *testing.T) {
	const src = `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">` +
		`<sheetViews>` +
		`<sheetView tabSelected="1" workbookViewId="0">` +
		`<pane xSplit="1" ySplit="1" topLeftCell="B2" activePane="bottomRight" state="frozen"/>` +
		`<selection pane="bottomRight" activeCell="B2" sqref="B2"/>` +
		`<pivotSelection pane="bottomRight" showHeader="1" activeRow="1" activeCol="1" sqref="B2"/>` +
		`</sheetView>` +
		`</sheetViews>` +
		`<sheetData/>` +
		`</worksheet>`

	var ws oxml.CT_Worksheet
	if err := xmlb.UnmarshalWithSource([]byte(src), &ws); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// Model still exposes pane/selection for reads.
	sv := ws.SheetViews.SheetView[0]
	if sv.Pane == nil || sv.Pane.TopLeftCell != "B2" {
		t.Fatalf("pane not decoded: %+v", sv.Pane)
	}
	if len(sv.Selection) != 1 || sv.Selection[0].SqRef != "B2" {
		t.Fatalf("selection not decoded: %+v", sv.Selection)
	}
	data, err := marshalWorksheetXML(&ws)
	if err != nil {
		t.Fatalf("marshalWorksheetXML: %v", err)
	}
	out := string(data)
	if !strings.Contains(out, `<pivotSelection pane="bottomRight" showHeader="1" activeRow="1" activeCol="1" sqref="B2"/>`) {
		t.Errorf("pivotSelection dropped on dirty save:\n%s", out)
	}
	if !strings.Contains(out, `<pane xSplit="1"`) || !strings.Contains(out, `<selection pane="bottomRight"`) {
		t.Errorf("pane/selection not preserved:\n%s", out)
	}
	// pivotSelection must follow selection (source order).
	if i, j := strings.Index(out, "<selection"), strings.Index(out, "<pivotSelection"); i < 0 || j < 0 || i > j {
		t.Errorf("child order not preserved:\n%s", out)
	}
}

// C320: CT_BookView skipped its children, losing an inner <extLst>. It must be
// captured and re-emitted on a regenerated workbook.xml.
func TestBookViewExtLstPreserved(t *testing.T) {
	const src = `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"` +
		` xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"` +
		` xmlns:x15="http://schemas.microsoft.com/office/spreadsheetml/2010/11/main">` +
		`<bookViews>` +
		`<workbookView xWindow="0" yWindow="0" windowWidth="100" windowHeight="100">` +
		`<extLst><ext uri="{ABC}"><x15:workbookView/></ext></extLst>` +
		`</workbookView>` +
		`</bookViews>` +
		`<sheets><sheet name="S" sheetId="1" r:id="rId1"/></sheets>` +
		`</workbook>`

	var wb oxml.CT_Workbook
	if err := xmlb.UnmarshalWithSource([]byte(src), &wb); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	data, err := marshalWorkbookXML(&wb)
	if err != nil {
		t.Fatalf("marshalWorkbookXML: %v", err)
	}
	out := string(data)
	if !strings.Contains(out, `<extLst><ext uri="{ABC}"><x15:workbookView/></ext></extLst>`) {
		t.Errorf("workbookView extLst child dropped:\n%s", out)
	}
}

// C323: a present-but-empty <extLst/> at workbook level is recorded in
// ChildOrder but marshalWorkbookExtLst early-returned on it, so a zero-mod save
// of the always-regenerated workbook.xml dropped it.
func TestWorkbookEmptyExtLstPreserved(t *testing.T) {
	const src = `<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"` +
		` xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">` +
		`<sheets><sheet name="S" sheetId="1" r:id="rId1"/></sheets>` +
		`<extLst/>` +
		`</workbook>`

	var wb oxml.CT_Workbook
	if err := xmlb.UnmarshalWithSource([]byte(src), &wb); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	data, err := marshalWorkbookXML(&wb)
	if err != nil {
		t.Fatalf("marshalWorkbookXML: %v", err)
	}
	if !strings.Contains(string(data), `<extLst/>`) {
		t.Errorf("empty workbook extLst dropped:\n%s", string(data))
	}
}
