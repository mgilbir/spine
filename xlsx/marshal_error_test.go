package xlsx

import (
	"strings"
	"testing"

	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// C187: a Builder error during part marshaling must surface from Save instead
// of shipping a malformed part into the package. The trigger is an attribute
// whose namespace has no registered prefix (the loud writeQName path from
// C147) — here x14ac:dyDescent on a workbook created from scratch, whose
// builder declares only the SpreadsheetML, relationships and
// markup-compatibility namespaces.
//
// The trigger used to be CT_BookView.ExtAttrs, a bespoke extension-attribute
// slice that C429 replaced with the CapturedAttrs convention (captured
// attributes replay with literal names, so they cannot reach writeQName).
func TestSave_MarshalErrorSurfaces(t *testing.T) {
	wb := Create()
	sheet := addSheetT(wb, "Sheet1")
	dyDescent := 0.25
	sheet.ws().SheetFormatPr = &oxml.CT_SheetFormatPr{
		DefaultRowHeight: 15,
		DyDescent:        &dyDescent,
	}

	_, err := wb.SaveBytes()
	if err == nil {
		t.Fatal("Save succeeded despite a marshal error")
	}
	if !strings.Contains(err.Error(), "no prefix registered") {
		t.Errorf("unexpected error: %v", err)
	}
}
