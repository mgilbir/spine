package oxml

import (
	"testing"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// TestOrdinaryCellsAllocateNoRareBlock is the premise the fold rests on: the
// fields behind CT_Cell.rare are so seldom set that paying 8 bytes for a
// pointer beats paying 48 for the fields. If an ordinary cell started
// allocating the block, the change would cost memory instead of saving it.
func TestOrdinaryCellsAllocateNoRareBlock(t *testing.T) {
	ws := &CT_Worksheet{}
	const body = `<worksheet><sheetData>` +
		`<row r="1"><c r="A1" t="n"><v>1</v></c><c r="B1" s="3"><v>2</v></c>` +
		`<c r="C1" t="inlineStr"><is><t>x</t></is></c>` +
		`<c r="D1"><f>SUM(A1:B1)</f><v>3</v></c></row>` +
		`</sheetData></worksheet>`
	if err := xmlb.UnmarshalWithSource([]byte(body), ws); err != nil {
		t.Fatal(err)
	}
	n := 0
	for i := range ws.SheetData.Row {
		for _, c := range ws.SheetData.Row[i].C {
			n++
			if c.rare != nil {
				t.Errorf("cell %q allocated a rare block though it sets none of cm/vm/ph/extLst", c.R)
			}
		}
	}
	if n != 4 {
		t.Fatalf("parsed %d cells, want 4", n)
	}
}

// TestSetNilOnAbsentRareStaysAbsent pins the other half: clearing a field the
// cell never had must not create the block.
func TestSetNilOnAbsentRareStaysAbsent(t *testing.T) {
	c := &CT_Cell{R: "A1"}
	c.SetCm(nil)
	c.SetVm(nil)
	c.SetPh(nil)
	if c.rare != nil {
		t.Error("clearing absent fields allocated a rare block")
	}
	if c.Cm() != nil || c.Vm() != nil || c.Ph() != nil || c.ExtRaw() != nil {
		t.Error("accessors on a cell with no rare block must read as absent")
	}

	one := uint32(1)
	c.SetCm(&one)
	if c.rare == nil || c.Cm() == nil || *c.Cm() != 1 {
		t.Fatalf("SetCm did not take: rare=%v cm=%v", c.rare, c.Cm())
	}
}
