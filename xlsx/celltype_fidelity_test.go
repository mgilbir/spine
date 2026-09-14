package xlsx

import (
	"fmt"
	"strings"
	"testing"
)

// TestEverySchemaCellTypeRoundTrips walks the whole ST_CellType set through a
// dirty save, which regenerates the worksheet from the model. The enum is only
// sound if every legal value maps back to exactly the text it came from, and
// "d" in particular appears nowhere in the corpus, so nothing else covers it.
func TestEverySchemaCellTypeRoundTrips(t *testing.T) {
	types := []string{"b", "d", "e", "inlineStr", "n", "s", "str"}

	var cells strings.Builder
	for i, ty := range types {
		// The value is irrelevant here; only the t attribute is under test.
		fmt.Fprintf(&cells, `<c r="%s1" t="%s"><v>1</v></c>`, string(rune('A'+i)), ty)
	}
	got := regeneratedSheetXML(t, `<sheetData><row r="1">`+cells.String()+`</row></sheetData>`)

	for _, ty := range types {
		if !strings.Contains(got, fmt.Sprintf(`t="%s"`, ty)) {
			t.Errorf("cell type %q lost on a regenerated worksheet:\n%s", ty, got)
		}
	}
}

// TestUnknownCellTypeRoundTripsVerbatim covers the fallback. A value outside
// the schema set has to be kept as written, including one that differs only in
// case — "S" is not "s" as far as the schema is concerned, and silently folding
// it would rewrite the caller's file.
func TestUnknownCellTypeRoundTripsVerbatim(t *testing.T) {
	got := regeneratedSheetXML(t, `<sheetData><row r="1">`+
		`<c r="A1" t="bogus"><v>1</v></c>`+
		`<c r="B1" t="S"><v>2</v></c>`+
		`</row></sheetData>`)

	for _, want := range []string{`t="bogus"`, `t="S"`} {
		if !strings.Contains(got, want) {
			t.Errorf("unknown cell type lost: wanted %s in\n%s", want, got)
		}
	}
	// "S" must not have been folded onto the shared-string type.
	if strings.Contains(got, `r="B1" t="s"`) {
		t.Errorf(`t="S" was folded onto t="s"`+"\n%s", got)
	}
}

// TestCellWithoutTypeKeepsNone is the other direction: t is optional, and a
// cell that had none must not gain one from the enum's zero value.
func TestCellWithoutTypeKeepsNone(t *testing.T) {
	got := regeneratedSheetXML(t, `<sheetData><row r="1">`+
		`<c r="A1"><v>1</v></c>`+
		`</row></sheetData>`)

	row1 := got
	if i := strings.Index(got, `<row r="1"`); i >= 0 {
		if j := strings.Index(got[i:], "</row>"); j >= 0 {
			row1 = got[i : i+j]
		}
	}
	if strings.Contains(row1, " t=") {
		t.Errorf("a cell with no t attribute gained one:\n%s", row1)
	}
}
