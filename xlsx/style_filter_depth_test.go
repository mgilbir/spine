package xlsx

import (
	"bytes"
	"testing"
)

// reopenWorkbook round-trips a workbook through an in-memory save and reopen.
func reopenWorkbook(t *testing.T, wb *Workbook) *Workbook {
	t.Helper()
	buf, err := wb.WriteToBuffer()
	if err != nil {
		t.Fatalf("WriteToBuffer: %v", err)
	}
	reopened, err := OpenReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	return reopened
}

func TestNamedStyle_RoundTrip(t *testing.T) {
	wb := Create()
	sheet := addSheetT(wb, "Sheet1")
	sm := wb.Styles()

	good := BuiltinStyleGood
	if _, err := sm.AddNamedStyle(NamedStyle{
		Name:      "Good",
		BuiltinId: &good,
		Style: CellStyle{
			Font: &FontStyle{Color: "006100"},
			Fill: &FillStyle{Pattern: "solid", FgColor: "C6EFCE"},
		},
	}); err != nil {
		t.Fatalf("AddNamedStyle: %v", err)
	}
	if _, err := sm.AddNamedStyle(NamedStyle{
		Name:  "MyCustom",
		Style: CellStyle{Font: &FontStyle{Bold: true, Size: 18}},
	}); err != nil {
		t.Fatalf("AddNamedStyle custom: %v", err)
	}

	cell, _ := sheet.Cell("A1")
	cell.SetValue("ok")
	if err := cell.SetNamedStyle("Good"); err != nil {
		t.Fatalf("SetNamedStyle: %v", err)
	}

	// Applying an unknown style must fail.
	if err := cell.SetNamedStyle("Nope"); err == nil {
		t.Fatal("expected error applying unknown named style")
	}

	wb2 := reopenWorkbook(t, wb)
	sm2 := wb2.Styles()
	names := sm2.NamedStyles()

	byName := map[string]NamedStyle{}
	for _, ns := range names {
		byName[ns.Name] = ns
	}
	if _, ok := byName["Normal"]; !ok {
		t.Error("expected default Normal style to survive")
	}
	g, ok := byName["Good"]
	if !ok {
		t.Fatal("Good named style missing after round-trip")
	}
	if g.BuiltinId == nil || *g.BuiltinId != BuiltinStyleGood {
		t.Errorf("Good builtinId = %v, want %d", g.BuiltinId, BuiltinStyleGood)
	}
	if g.Style.Fill == nil || g.Style.Fill.FgColor != "C6EFCE" {
		t.Errorf("Good fill = %+v, want FgColor C6EFCE", g.Style.Fill)
	}
	if g.Style.Font == nil || g.Style.Font.Color != "006100" {
		t.Errorf("Good font color = %+v, want 006100", g.Style.Font)
	}
	c, ok := byName["MyCustom"]
	if !ok {
		t.Fatal("MyCustom named style missing")
	}
	if c.Style.Font == nil || !c.Style.Font.Bold || c.Style.Font.Size != 18 {
		t.Errorf("MyCustom font = %+v, want bold size 18", c.Style.Font)
	}

	// The applied cell carries a style whose xf links back to the named style.
	s2, _ := wb2.SheetByName("Sheet1")
	a1, _ := s2.Cell("A1")
	if a1.StyleIndex() == nil {
		t.Fatal("expected style index on A1 after round-trip")
	}
	xfID, ok := sm2.NamedStyleXfId("Good")
	if !ok {
		t.Fatal("NamedStyleXfId(Good) not found")
	}
	if xfID == 0 {
		t.Error("expected non-zero xfId for Good")
	}
}

func TestNamedStyle_DedupByName(t *testing.T) {
	wb := Create()
	sm := wb.Styles()
	id1, err := sm.AddNamedStyle(NamedStyle{Name: "Dup", Style: CellStyle{Font: &FontStyle{Bold: true}}})
	if err != nil {
		t.Fatal(err)
	}
	id2, err := sm.AddNamedStyle(NamedStyle{Name: "Dup", Style: CellStyle{Font: &FontStyle{Italic: true}}})
	if err != nil {
		t.Fatal(err)
	}
	if id1 != id2 {
		t.Errorf("re-adding a named style changed xfId: %d != %d", id1, id2)
	}
}

func TestGradientFill_RoundTrip(t *testing.T) {
	wb := Create()
	sheet := addSheetT(wb, "Sheet1")
	sm := wb.Styles()

	idx, err := sm.NewCellStyle(CellStyle{
		Fill: &FillStyle{Gradient: &GradientFill{
			Degree: 90,
			Stops: []GradientStop{
				{Position: 0, Color: "FFFFFF"},
				{Position: 1, Color: "4472C4"},
			},
		}},
	})
	if err != nil {
		t.Fatalf("NewCellStyle: %v", err)
	}
	cell, _ := sheet.Cell("A1")
	cell.SetValue("g")
	cell.SetStyleIndex(idx)

	wb2 := reopenWorkbook(t, wb)
	s2, _ := wb2.SheetByName("Sheet1")
	a1, _ := s2.Cell("A1")
	cs, err := wb2.Styles().GetCellStyle(*a1.StyleIndex())
	if err != nil {
		t.Fatal(err)
	}
	if cs.Fill == nil || cs.Fill.Gradient == nil {
		t.Fatalf("expected gradient fill, got %+v", cs.Fill)
	}
	g := cs.Fill.Gradient
	if g.Degree != 90 {
		t.Errorf("degree = %v, want 90", g.Degree)
	}
	if len(g.Stops) != 2 {
		t.Fatalf("stops = %d, want 2", len(g.Stops))
	}
	if g.Stops[0].Color != "FFFFFF" || g.Stops[1].Color != "4472C4" {
		t.Errorf("stop colors = %v", g.Stops)
	}
	if g.Stops[1].Position != 1 {
		t.Errorf("stop[1] position = %v, want 1", g.Stops[1].Position)
	}
}

func TestDiagonalBorder_RoundTrip(t *testing.T) {
	wb := Create()
	sheet := addSheetT(wb, "Sheet1")
	sm := wb.Styles()

	idx, err := sm.NewCellStyle(CellStyle{
		Border: &BorderStyle{
			Diagonal:     &BorderSide{Style: "thin", Color: "FF0000"},
			DiagonalUp:   true,
			DiagonalDown: true,
		},
	})
	if err != nil {
		t.Fatalf("NewCellStyle: %v", err)
	}
	cell, _ := sheet.Cell("A1")
	cell.SetValue("d")
	cell.SetStyleIndex(idx)

	wb2 := reopenWorkbook(t, wb)
	s2, _ := wb2.SheetByName("Sheet1")
	a1, _ := s2.Cell("A1")
	cs, err := wb2.Styles().GetCellStyle(*a1.StyleIndex())
	if err != nil {
		t.Fatal(err)
	}
	if cs.Border == nil || cs.Border.Diagonal == nil {
		t.Fatalf("expected diagonal border, got %+v", cs.Border)
	}
	if cs.Border.Diagonal.Style != "thin" || cs.Border.Diagonal.Color != "FF0000" {
		t.Errorf("diagonal = %+v", cs.Border.Diagonal)
	}
	if !cs.Border.DiagonalUp || !cs.Border.DiagonalDown {
		t.Errorf("diagonalUp=%v diagonalDown=%v, want both true", cs.Border.DiagonalUp, cs.Border.DiagonalDown)
	}
}

func TestAlignmentExtras_RoundTrip(t *testing.T) {
	wb := Create()
	sheet := addSheetT(wb, "Sheet1")
	sm := wb.Styles()

	idx, err := sm.NewCellStyle(CellStyle{
		Alignment: &AlignmentStyle{
			Horizontal:      "center",
			ShrinkToFit:     true,
			JustifyLastLine: true,
			ReadingOrder:    2,
		},
	})
	if err != nil {
		t.Fatalf("NewCellStyle: %v", err)
	}
	cell, _ := sheet.Cell("A1")
	cell.SetValue("a")
	cell.SetStyleIndex(idx)

	wb2 := reopenWorkbook(t, wb)
	s2, _ := wb2.SheetByName("Sheet1")
	a1, _ := s2.Cell("A1")
	cs, err := wb2.Styles().GetCellStyle(*a1.StyleIndex())
	if err != nil {
		t.Fatal(err)
	}
	if cs.Alignment == nil {
		t.Fatal("expected alignment")
	}
	a := cs.Alignment
	if !a.ShrinkToFit || !a.JustifyLastLine || a.ReadingOrder != 2 {
		t.Errorf("alignment extras = %+v", a)
	}
}

func TestFilterColumns_ValueList_RoundTrip(t *testing.T) {
	wb := Create()
	sheet := addSheetT(wb, "Sheet1")
	if err := sheet.SetAutoFilter("A1:C10"); err != nil {
		t.Fatal(err)
	}
	if err := sheet.SetFilterColumn(FilterColumn{
		ColID:  1,
		Values: []string{"apple", "pear"},
		Blank:  true,
	}); err != nil {
		t.Fatalf("SetFilterColumn: %v", err)
	}

	wb2 := reopenWorkbook(t, wb)
	s2, _ := wb2.SheetByName("Sheet1")
	cols := s2.FilterColumns()
	if len(cols) != 1 {
		t.Fatalf("filter columns = %d, want 1", len(cols))
	}
	fc := cols[0]
	if fc.ColID != 1 {
		t.Errorf("colID = %d, want 1", fc.ColID)
	}
	if len(fc.Values) != 2 || fc.Values[0] != "apple" || fc.Values[1] != "pear" {
		t.Errorf("values = %v", fc.Values)
	}
	if !fc.Blank {
		t.Error("expected blank true")
	}
}

func TestFilterColumns_Custom_RoundTrip(t *testing.T) {
	wb := Create()
	sheet := addSheetT(wb, "Sheet1")
	if err := sheet.SetAutoFilter("A1:C10"); err != nil {
		t.Fatal(err)
	}
	if err := sheet.SetFilterColumn(FilterColumn{
		ColID:     0,
		CustomAnd: true,
		Custom: []CustomFilter{
			{Operator: FilterGreaterThan, Value: "10"},
			{Operator: FilterLessThanOrEqual, Value: "100"},
		},
	}); err != nil {
		t.Fatalf("SetFilterColumn: %v", err)
	}

	// SetFilterColumn requires an auto-filter.
	bare := addSheetT(Create(), "X")
	if err := bare.SetFilterColumn(FilterColumn{ColID: 0, Values: []string{"a"}}); err == nil {
		t.Error("expected error setting filter column without auto-filter")
	}

	wb2 := reopenWorkbook(t, wb)
	s2, _ := wb2.SheetByName("Sheet1")
	cols := s2.FilterColumns()
	if len(cols) != 1 {
		t.Fatalf("filter columns = %d, want 1", len(cols))
	}
	fc := cols[0]
	if !fc.CustomAnd {
		t.Error("expected customAnd true")
	}
	if len(fc.Custom) != 2 {
		t.Fatalf("custom = %d, want 2", len(fc.Custom))
	}
	if fc.Custom[0].Operator != FilterGreaterThan || fc.Custom[0].Value != "10" {
		t.Errorf("custom[0] = %+v", fc.Custom[0])
	}
	if fc.Custom[1].Operator != FilterLessThanOrEqual || fc.Custom[1].Value != "100" {
		t.Errorf("custom[1] = %+v", fc.Custom[1])
	}

	// Replacing a column filter in place keeps a single entry.
	if err := s2.SetFilterColumn(FilterColumn{ColID: 0, Values: []string{"z"}}); err != nil {
		t.Fatal(err)
	}
	if got := s2.FilterColumns(); len(got) != 1 || len(got[0].Values) != 1 {
		t.Errorf("after replace, cols = %+v", got)
	}
	s2.ClearFilterColumns()
	if got := s2.FilterColumns(); got != nil {
		t.Errorf("after clear, cols = %+v, want nil", got)
	}
	if _, ok := s2.AutoFilterRange(); !ok {
		t.Error("clearing columns should keep the auto-filter range")
	}
}

func TestSortState_RoundTrip(t *testing.T) {
	wb := Create()
	sheet := addSheetT(wb, "Sheet1")
	if err := sheet.SetSortState(SortState{
		Ref:           "A2:C100",
		CaseSensitive: true,
		Conditions: []SortCondition{
			{Ref: "B2:B100", Descending: true},
			{Ref: "C2:C100", SortBy: SortByCellColor},
		},
	}); err != nil {
		t.Fatalf("SetSortState: %v", err)
	}
	if err := sheet.SetSortState(SortState{}); err == nil {
		t.Error("expected error for sort state without Ref")
	}

	wb2 := reopenWorkbook(t, wb)
	s2, _ := wb2.SheetByName("Sheet1")
	ss, ok := s2.SortState()
	if !ok {
		t.Fatal("expected sort state after round-trip")
	}
	if ss.Ref != "A2:C100" || !ss.CaseSensitive {
		t.Errorf("sort state = %+v", ss)
	}
	if len(ss.Conditions) != 2 {
		t.Fatalf("conditions = %d, want 2", len(ss.Conditions))
	}
	if !ss.Conditions[0].Descending || ss.Conditions[0].Ref != "B2:B100" {
		t.Errorf("condition[0] = %+v", ss.Conditions[0])
	}
	if ss.Conditions[1].SortBy != SortByCellColor {
		t.Errorf("condition[1] sortBy = %q", ss.Conditions[1].SortBy)
	}

	s2.RemoveSortState()
	if _, ok := s2.SortState(); ok {
		t.Error("expected no sort state after removal")
	}
}

func TestDefinedName_RichAttrs_RoundTrip(t *testing.T) {
	wb := Create()
	addSheetT(wb, "Sheet1")
	if err := wb.AddDefinedNameFull(DefinedName{
		Name:        "SecretRange",
		Value:       "Sheet1!$A$1:$B$2",
		SheetIndex:  -1,
		Hidden:      true,
		Comment:     "internal use",
		Description: "a hidden helper range",
	}); err != nil {
		t.Fatalf("AddDefinedNameFull: %v", err)
	}
	if err := wb.AddDefinedName("Visible", "Sheet1!$C$1"); err != nil {
		t.Fatal(err)
	}

	wb2 := reopenWorkbook(t, wb)
	names := wb2.DefinedNames()
	byName := map[string]DefinedName{}
	for _, dn := range names {
		byName[dn.Name] = dn
	}
	secret, ok := byName["SecretRange"]
	if !ok {
		t.Fatal("SecretRange missing after round-trip")
	}
	if !secret.Hidden {
		t.Error("expected Hidden true")
	}
	if secret.Comment != "internal use" {
		t.Errorf("comment = %q", secret.Comment)
	}
	if secret.Description != "a hidden helper range" {
		t.Errorf("description = %q", secret.Description)
	}

	// Removal.
	if !wb2.RemoveDefinedName("SecretRange") {
		t.Error("RemoveDefinedName returned false")
	}
	if wb2.RemoveDefinedName("SecretRange") {
		t.Error("second RemoveDefinedName should return false")
	}
	remaining := wb2.DefinedNames()
	if len(remaining) != 1 || remaining[0].Name != "Visible" {
		t.Errorf("remaining names = %+v", remaining)
	}
}

func TestRemoveDefinedNameScoped(t *testing.T) {
	wb := Create()
	addSheetT(wb, "Sheet1")
	addSheetT(wb, "Sheet2")
	if err := wb.AddDefinedNameScoped("Local", "Sheet2!$A$1", 1); err != nil {
		t.Fatal(err)
	}
	// Wrong scope must not remove it.
	if wb.RemoveDefinedName("Local") {
		t.Error("workbook-scoped removal should not touch a sheet-scoped name")
	}
	if !wb.RemoveDefinedNameScoped("Local", 1) {
		t.Error("expected scoped removal to succeed")
	}
	if len(wb.DefinedNames()) != 0 {
		t.Errorf("expected no defined names, got %d", len(wb.DefinedNames()))
	}
}
