package xlsx

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// C272: a dirty worksheet whose <extLst> carries its own xmlns declaration
// (e.g. <extLst xmlns:x14="...">) must re-emit that declaration, or the
// x14:-prefixed extension children reference an undeclared prefix (malformed
// XML). The workbook path already replayed CT_ExtensionList.CapturedAttrs; the
// worksheet path dropped them.
func TestWorksheetExtLstCapturedAttrsReplayed(t *testing.T) {
	const src = `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">` +
		`<sheetData><row r="1"><c r="A1"><v>1</v></c></row></sheetData>` +
		`<extLst xmlns:x14="http://schemas.microsoft.com/office/spreadsheetml/2009/9/main">` +
		`<ext uri="{05C60535-1F16-4fd2-B633-F4F36F0B64E0}"><x14:sparklineGroups/></ext>` +
		`</extLst>` +
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
	if !strings.Contains(out, `<extLst xmlns:x14="http://schemas.microsoft.com/office/spreadsheetml/2009/9/main">`) {
		t.Errorf("worksheet extLst dropped its xmlns:x14 declaration:\n%s", out)
	}
	// The x14 prefix used by the child must be declared somewhere the child can
	// see it; parse the fragment strictly to confirm it is namespace-well-formed.
	if err := xml.Unmarshal(data, new(struct {
		XMLName xml.Name
	})); err != nil {
		t.Errorf("re-parse of marshaled worksheet failed: %v", err)
	}
}

// C272: same gap on the stylesheet path.
func TestStylesheetExtLstCapturedAttrsReplayed(t *testing.T) {
	const src = `<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">` +
		`<extLst xmlns:x14="http://schemas.microsoft.com/office/spreadsheetml/2009/9/main">` +
		`<ext uri="{EB79DEF2-80B8-43e5-95BD-54CBDDF9020C}"><x14:slicerStyles defaultSlicerStyle="SlicerStyleLight1"/></ext>` +
		`</extLst>` +
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
	if !strings.Contains(out, `<extLst xmlns:x14="http://schemas.microsoft.com/office/spreadsheetml/2009/9/main">`) {
		t.Errorf("stylesheet extLst dropped its xmlns:x14 declaration:\n%s", out)
	}
}
