package xlsx

import (
	"strings"
	"testing"

	xmlb "github.com/mgilbir/spine/common/xml"
	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// C187: a Builder error during part marshaling must surface from Save instead
// of shipping a malformed part into the package. The trigger here is an
// extension attribute whose namespace has no registered prefix (the loud
// writeQName path from C147).
func TestSave_MarshalErrorSurfaces(t *testing.T) {
	wb := Create()
	wb.workbook.BookViews = &oxml.CT_BookViews{
		WorkbookView: []oxml.CT_BookView{{
			ExtAttrs: []xmlb.Attr{{Namespace: "urn:example:unregistered", Name: "uid", Value: "1"}},
		}},
	}

	_, err := wb.SaveBytes()
	if err == nil {
		t.Fatal("Save succeeded despite a marshal error")
	}
	if !strings.Contains(err.Error(), "no prefix registered") {
		t.Errorf("unexpected error: %v", err)
	}
}
