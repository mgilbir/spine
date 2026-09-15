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
				t.Errorf("cell %q allocated a rare block though it sets none of cm/vm/ph/extLst", c.Ref())
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
	c := NewCell("A1")
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

// TestStyleIndexDistinguishesZeroFromAbsent is the property the presence bit
// exists for. As a *uint32, "no s attribute" was nil and s="0" was a pointer to
// zero; held by value those collapse unless the bit is kept, and emitting s="0"
// on a cell that never had it changes the file.
func TestStyleIndexDistinguishesZeroFromAbsent(t *testing.T) {
	var c CT_Cell
	if _, ok := c.StyleIndex(); ok {
		t.Error("a fresh cell reports a style index")
	}
	if c.HasStyle() {
		t.Error("a fresh cell reports HasStyle")
	}

	c.SetStyleIndex(0)
	v, ok := c.StyleIndex()
	if !ok || v != 0 {
		t.Errorf("after SetStyleIndex(0): got %v, %v; want 0, true", v, ok)
	}
	if !c.HasStyle() {
		t.Error("s=\"0\" must read as present")
	}

	c.ClearStyleIndex()
	if _, ok := c.StyleIndex(); ok {
		t.Error("ClearStyleIndex left the index present")
	}
}

// TestStyleIndexZeroRoundTrips pins the same distinction through the XML, which
// is where it actually costs something: a cell written with s="0" must come
// back with it, and one written without s must not gain it.
func TestStyleIndexZeroRoundTrips(t *testing.T) {
	ws := &CT_Worksheet{}
	body := `<worksheet><sheetData><row r="1">` +
		`<c r="A1" s="0"><v>1</v></c><c r="B1"><v>2</v></c>` +
		`</row></sheetData></worksheet>`
	if err := xmlb.UnmarshalWithSource([]byte(body), ws); err != nil {
		t.Fatal(err)
	}
	cells := ws.SheetData.Row[0].C
	if v, ok := cells[0].StyleIndex(); !ok || v != 0 {
		t.Errorf(`A1 (s="0"): got %v, %v; want 0, true`, v, ok)
	}
	if _, ok := cells[1].StyleIndex(); ok {
		t.Error("B1 has no s attribute but reports a style index")
	}
}

// TestClearRefRemovesTheReference pins the contract Ref's rSet check exists
// for: a cell that had a position and then had its reference cleared must emit
// no r attribute, rather than rebuilding one from the position it still
// remembers.
func TestClearRefRemovesTheReference(t *testing.T) {
	c := NewCell("C7")
	if got := c.Ref(); got != "C7" {
		t.Fatalf("Ref() = %q, want C7", got)
	}
	if row, col, ok := c.RowCol(); !ok || row != 7 || col != 3 {
		t.Fatalf("RowCol() = %d,%d,%v; want 7,3,true", row, col, ok)
	}

	c.ClearRef()
	if got := c.Ref(); got != "" {
		t.Errorf("Ref() after ClearRef = %q, want \"\"", got)
	}
	if c.HasRef() {
		t.Error("HasRef() after ClearRef is true")
	}
	if _, _, ok := c.RowCol(); ok {
		t.Error("RowCol() after ClearRef still reports a position")
	}
}

// TestRefIsCanonicalMatchesFormatting pins the shortcut SetRef takes. It
// decides canonicity by inspecting the text instead of formatting the position
// and comparing, which cost an allocation per cell on the parse path; the two
// must agree for every reference, or a cell either loses its original spelling
// or keeps a redundant copy of it.
func TestRefIsCanonicalMatchesFormatting(t *testing.T) {
	refs := []string{
		"A1", "Z9", "AA1", "XFD1", "XFD1048576", "B10", "A100",
		"A01", "A0", "a1", "aB3", "Ab3", "A1 ", " A1", "A+1", "A-1",
		"A1x", "1A", "", "A", "1", "AAAA1", "A1048577",
	}
	for _, ref := range refs {
		row, col, err := ParseRefString(ref)
		want := err == nil && CellRefString(row, col) == ref
		if got := err == nil && refIsCanonical(ref); got != want {
			t.Errorf("refIsCanonical(%q) = %v, but formatting says %v", ref, got, want)
		}
	}
}

// TestColumnLettersRoundTrips guards the rewritten formatter against the
// parser across the whole column range.
func TestColumnLettersRoundTrips(t *testing.T) {
	for col := 1; col <= MaxCol; col++ {
		letters := ColumnLetters(col)
		_, got, err := ParseRefString(letters + "1")
		if err != nil || got != col {
			t.Fatalf("ColumnLetters(%d) = %q, which parses back to %d (err %v)", col, letters, got, err)
		}
	}
	if got := ColumnLetters(0); got != "" {
		t.Errorf("ColumnLetters(0) = %q, want \"\"", got)
	}
	if got := ColumnLetters(MaxCol); got != "XFD" {
		t.Errorf("ColumnLetters(MaxCol) = %q, want XFD", got)
	}
}

// TestSchemaCellTypesNeedNoRareBlock is the premise the enum rests on. Every
// ST_CellType value must map onto the enum itself; one that fell through to
// cellTypeOther would still round-trip — the verbatim fallback sees to that —
// but it would allocate a rare block for a perfectly ordinary cell, which is
// the cost the enum exists to avoid. Fidelity tests cannot see that difference.
func TestSchemaCellTypesNeedNoRareBlock(t *testing.T) {
	for _, ty := range []string{"", "b", "d", "e", "inlineStr", "n", "s", "str"} {
		c := &CT_Cell{}
		c.SetType(ty)
		if c.rare != nil {
			t.Errorf("t=%q allocated a rare block; it should be carried by the enum", ty)
		}
		if got := c.Type(); got != ty {
			t.Errorf("t=%q read back as %q", ty, got)
		}
	}

	// And a value outside the set must use the block, not be silently dropped.
	c := &CT_Cell{}
	c.SetType("bogus")
	if c.rare == nil || c.rare.T != "bogus" {
		t.Error("an unknown type was not kept verbatim")
	}
	if got := c.Type(); got != "bogus" {
		t.Errorf("unknown type read back as %q, want \"bogus\"", got)
	}
}
