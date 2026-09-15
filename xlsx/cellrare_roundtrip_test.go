package xlsx

import (
	"bytes"
	"strings"
	"testing"
)

// TestRareCellFieldsSurviveADirtySave covers the fields the fold moved behind a
// pointer. The corpus barely exercises them — cm appears on 0.01% of its 25.7M
// cells, ph on 88, vm on 7 and extLst on none — so the byte-identity gate is
// not the guard here; this is.
//
// The sheet is dirtied so the save regenerates it from the model rather than
// re-emitting the preserved bytes, which is the only path where losing a field
// would show.
func TestRareCellFieldsSurviveADirtySave(t *testing.T) {
	const body = `<sheetData>` +
		`<row r="1">` +
		`<c r="A1" s="0" t="n" cm="1" vm="2" ph="1"><v>42</v></c>` +
		`<c r="B1" t="n"><v>7</v>` +
		`<extLst><ext uri="{DEADBEEF-0000-0000-0000-000000000000}"><foo:bar xmlns:foo="urn:foo" baz="1"/></ext></extLst>` +
		`</c>` +
		`</row></sheetData>`

	data := buildXLSXWithWorksheet(t, body)
	wb, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	sh, err := wb.Sheet(0)
	if err != nil {
		t.Fatal(err)
	}
	// Dirty the sheet so it is regenerated from the model on save.
	if err := sh.SetCellValue("D4", "touched"); err != nil {
		t.Fatal(err)
	}
	out, err := wb.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}

	wb2, err := OpenReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatal(err)
	}
	sh2, err := wb2.Sheet(0)
	if err != nil {
		t.Fatal(err)
	}
	a1 := sh2.FindCell("A1")
	if a1 == nil {
		t.Fatal("A1 lost")
	}
	if cm := a1.cell.Cm(); cm == nil || *cm != 1 {
		t.Errorf("A1 cm = %v, want 1", cm)
	}
	if vm := a1.cell.Vm(); vm == nil || *vm != 2 {
		t.Errorf("A1 vm = %v, want 2", vm)
	}
	if ph := a1.cell.Ph(); ph == nil || !*ph {
		t.Errorf("A1 ph = %v, want true", ph)
	}
	// s="0" is the case a presence bit exists for: held by value, "absent" and
	// "zero" are the same number, and a cell that never had an s attribute must
	// not gain one. The source gives A1 s="0" and B1 none.
	if v, ok := a1.cell.StyleIndex(); !ok || v != 0 {
		t.Errorf(`A1 style index = %v, %v; want 0, true (source had s="0")`, v, ok)
	}

	b1 := sh2.FindCell("B1")
	if b1 == nil {
		t.Fatal("B1 lost")
	}
	if _, ok := b1.cell.StyleIndex(); ok {
		t.Error("B1 gained a style index it never had in the source")
	}
	if got := b1.cell.ExtRaw(); len(got) != 1 {
		t.Fatalf("B1 extLst: got %d raw children, want 1", len(got))
	} else if !strings.Contains(string(got[0]), "urn:foo") {
		t.Errorf("B1 extLst content lost its namespace: %q", got[0])
	}

}
