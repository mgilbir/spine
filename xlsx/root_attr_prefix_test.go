package xlsx

import (
	"strings"
	"testing"

	xmlb "github.com/mgilbir/spine/common/xml"
	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// C324: root attribute prefix resolution scanned only the declarations seen so
// far, so a namespaced attribute whose xmlns declaration follows it on the same
// tag lost its prefix (and namespace) on a dirty save. The worksheet root must
// resolve it against every declaration on the element.
func TestWorksheetRootAttrPrefixDeclaredLater(t *testing.T) {
	const src = `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"` +
		` foo:tag="v" xmlns:foo="urn:foo">` +
		`<sheetData/></worksheet>`

	var ws oxml.CT_Worksheet
	if err := xmlb.Unmarshal([]byte(src), &ws); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	data, err := marshalWorksheetXML(&ws)
	if err != nil {
		t.Fatalf("marshalWorksheetXML: %v", err)
	}
	if !strings.Contains(string(data), `foo:tag="v"`) {
		t.Errorf("namespaced root attr lost its prefix:\n%s", string(data))
	}
}

// C324: same gap on the stylesheet root.
func TestStylesheetRootAttrPrefixDeclaredLater(t *testing.T) {
	const src = `<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"` +
		` foo:tag="v" xmlns:foo="urn:foo">` +
		`<fonts count="0"/></styleSheet>`

	var ss oxml.CT_Stylesheet
	if err := xmlb.Unmarshal([]byte(src), &ss); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	data, err := marshalStylesheetXML(&ss)
	if err != nil {
		t.Fatalf("marshalStylesheetXML: %v", err)
	}
	if !strings.Contains(string(data), `foo:tag="v"`) {
		t.Errorf("namespaced root attr lost its prefix:\n%s", string(data))
	}
}
