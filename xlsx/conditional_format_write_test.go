package xlsx

import (
	"bytes"
	"strings"
	"testing"
)

// addCFSheet creates a one-sheet workbook and returns the sheet, ready for
// AddConditionalFormat.
func addCFSheet(t *testing.T) (*Workbook, *Sheet) {
	t.Helper()
	w := Create()
	return w, w.AddSheet("Sheet1")
}

// firstCF reopens the workbook and returns the first sheet's single
// conditional-formatting block, failing if there is not exactly one.
func firstCF(t *testing.T, w *Workbook) (*Workbook, *ConditionalFormat) {
	t.Helper()
	rw := reopen(t, w)
	cfs := firstSheet(t, rw).ConditionalFormats()
	if len(cfs) != 1 {
		t.Fatalf("ConditionalFormats() len = %d, want 1", len(cfs))
	}
	return rw, cfs[0]
}

func TestConditionalFormat_CellIsRoundTrip(t *testing.T) {
	w, s := addCFSheet(t)
	style := DifferentialStyle{
		Font: &FontStyle{Color: "9C0006", Bold: true},
		Fill: &FillStyle{FgColor: "FFC7CE"},
	}
	if err := s.AddConditionalFormat("B2:B10", NewCellIsRule(CondOpGreaterThan, style, "5")); err != nil {
		t.Fatalf("AddConditionalFormat: %v", err)
	}
	rw, cf := firstCF(t, w)
	if cf.SqRef != "B2:B10" {
		t.Errorf("SqRef = %q, want B2:B10", cf.SqRef)
	}
	if len(cf.Rules) != 1 {
		t.Fatalf("Rules len = %d", len(cf.Rules))
	}
	r := cf.Rules[0]
	if r.Type != "cellIs" || r.Operator != "greaterThan" {
		t.Errorf("rule = %+v", r)
	}
	if len(r.Formulas) != 1 || r.Formulas[0] != "5" {
		t.Errorf("formulas = %v", r.Formulas)
	}
	if r.Priority != 1 {
		t.Errorf("priority = %d, want 1", r.Priority)
	}
	if r.DxfId == nil {
		t.Fatal("DxfId is nil")
	}
	// The resolved differential format must report the colors we set.
	df := r.DifferentialFormat()
	if df == nil {
		t.Fatal("DifferentialFormat() = nil")
	}
	if df.Font == nil || !strings.EqualFold(df.Font.Color, "9C0006") || !df.Font.Bold {
		t.Errorf("font = %+v", df.Font)
	}
	if df.Fill == nil || !strings.EqualFold(df.Fill.FgColor, "FFC7CE") {
		t.Errorf("fill = %+v", df.Fill)
	}
	_ = rw

	// The between operator needs two operands.
	if err := firstSheet(t, w).AddConditionalFormat("C1:C2", NewCellIsRule(CondOpBetween, style, "1")); err == nil {
		t.Error("between with one operand: want error")
	}
}

func TestConditionalFormat_TwoAndThreeColorScale(t *testing.T) {
	w, s := addCFSheet(t)
	two := NewColorScaleRule(
		ColorScalePoint{Type: "min", Color: "F8696B"},
		ColorScalePoint{Type: "max", Color: "63BE7B"},
	)
	if err := s.AddConditionalFormat("A1:A10", two); err != nil {
		t.Fatalf("AddConditionalFormat (2-color): %v", err)
	}
	_, cf := firstCF(t, w)
	cs := cf.Rules[0].ColorScale
	if cs == nil || len(cs.Values) != 2 || len(cs.Colors) != 2 {
		t.Fatalf("colorScale = %+v", cs)
	}
	if !strings.EqualFold(cs.Colors[0], "FFF8696B") {
		t.Errorf("color0 = %q", cs.Colors[0])
	}

	// A 3-color scale also round-trips; 1 or 4 points is rejected.
	w2, s2 := addCFSheet(t)
	three := NewColorScaleRule(
		ColorScalePoint{Type: "min", Color: "F8696B"},
		ColorScalePoint{Type: "percentile", Value: "50", Color: "FFEB84"},
		ColorScalePoint{Type: "max", Color: "63BE7B"},
	)
	if err := s2.AddConditionalFormat("A1:A10", three); err != nil {
		t.Fatalf("AddConditionalFormat (3-color): %v", err)
	}
	_, cf2 := firstCF(t, w2)
	if cs2 := cf2.Rules[0].ColorScale; cs2 == nil || len(cs2.Values) != 3 {
		t.Fatalf("3-color = %+v", cs2)
	}
	if err := s2.AddConditionalFormat("B1:B2", NewColorScaleRule(ColorScalePoint{Type: "min"})); err == nil {
		t.Error("1-point color scale: want error")
	}
}

func TestConditionalFormat_DataBar(t *testing.T) {
	w, s := addCFSheet(t)
	rule := NewDataBarRule("638EC6", ConditionalValueObject{}, ConditionalValueObject{})
	if err := s.AddConditionalFormat("D1:D20", rule); err != nil {
		t.Fatalf("AddConditionalFormat: %v", err)
	}
	_, cf := firstCF(t, w)
	db := cf.Rules[0].DataBar
	if db == nil || len(db.Values) != 2 {
		t.Fatalf("dataBar = %+v", db)
	}
	if db.Values[0].Type != "min" || db.Values[1].Type != "max" {
		t.Errorf("bounds = %+v", db.Values)
	}
	if !strings.EqualFold(db.Color, "FF638EC6") {
		t.Errorf("color = %q", db.Color)
	}
}

func TestConditionalFormat_IconSet(t *testing.T) {
	w, s := addCFSheet(t)
	if err := s.AddConditionalFormat("E1:E30", NewIconSetRule("3TrafficLights1")); err != nil {
		t.Fatalf("AddConditionalFormat: %v", err)
	}
	_, cf := firstCF(t, w)
	is := cf.Rules[0].IconSet
	if is == nil || is.IconSet != "3TrafficLights1" {
		t.Fatalf("iconSet = %+v", is)
	}
	// Default thresholds: one per icon, evenly spaced from 0.
	if len(is.Values) != 3 || is.Values[0].Value != "0" || is.Values[1].Value != "33" || is.Values[2].Value != "66" {
		t.Errorf("thresholds = %+v", is.Values)
	}
}

func TestConditionalFormat_TextRule(t *testing.T) {
	w, s := addCFSheet(t)
	style := DifferentialStyle{Fill: &FillStyle{FgColor: "FFC7CE"}}
	if err := s.AddConditionalFormat("A1:A5", NewTextRule(CondTextContains, "err", style)); err != nil {
		t.Fatalf("AddConditionalFormat: %v", err)
	}
	_, cf := firstCF(t, w)
	r := cf.Rules[0]
	if r.Type != "containsText" || r.Text != "err" {
		t.Errorf("rule = %+v", r)
	}
	// A matching formula anchored at the range top-left must be synthesized.
	if len(r.Formulas) != 1 || !strings.Contains(r.Formulas[0], "SEARCH(\"err\",A1)") {
		t.Errorf("formula = %v", r.Formulas)
	}
	if r.DxfId == nil {
		t.Error("DxfId nil")
	}

	// The other three operators map type/operator per Excel's convention.
	cases := []struct {
		op       string
		wantType string
		wantOp   string
		wantFn   string
	}{
		{CondTextNotContains, "notContainsText", "notContains", "ISERROR(SEARCH(\"x\",A1))"},
		{CondTextBeginsWith, "beginsWith", "beginsWith", "LEFT(A1,LEN(\"x\"))=\"x\""},
		{CondTextEndsWith, "endsWith", "endsWith", "RIGHT(A1,LEN(\"x\"))=\"x\""},
	}
	for _, tc := range cases {
		w2, s2 := addCFSheet(t)
		if err := s2.AddConditionalFormat("A1:A5", NewTextRule(tc.op, "x", style)); err != nil {
			t.Fatalf("%s: %v", tc.op, err)
		}
		_, cf2 := firstCF(t, w2)
		got := cf2.Rules[0]
		if got.Type != tc.wantType || got.Operator != tc.wantOp {
			t.Errorf("%s: type/op = %q/%q, want %q/%q", tc.op, got.Type, got.Operator, tc.wantType, tc.wantOp)
		}
		if len(got.Formulas) != 1 || got.Formulas[0] != tc.wantFn {
			t.Errorf("%s: formula = %v, want %q", tc.op, got.Formulas, tc.wantFn)
		}
	}
}

func TestConditionalFormat_Top10AboveAvgDuplicateUniqueExpression(t *testing.T) {
	w, s := addCFSheet(t)
	style := DifferentialStyle{Fill: &FillStyle{FgColor: "FFC7CE"}}
	rules := []ConditionalRule{
		NewTop10Rule(3, false, false, style),
		NewTop10Rule(10, true, true, style),
		NewAboveAverageRule(true, style),
		NewAboveAverageRule(false, style),
		NewDuplicateValuesRule(style),
		NewUniqueValuesRule(style),
		NewExpressionRule("MOD(ROW(),2)=0", style),
	}
	if err := s.AddConditionalFormat("A1:A100", rules...); err != nil {
		t.Fatalf("AddConditionalFormat: %v", err)
	}
	_, cf := firstCF(t, w)
	if len(cf.Rules) != len(rules) {
		t.Fatalf("Rules len = %d, want %d", len(cf.Rules), len(rules))
	}
	got := make([]string, len(cf.Rules))
	for i, r := range cf.Rules {
		got[i] = r.Type
		if r.Priority != i+1 {
			t.Errorf("rule %d priority = %d, want %d", i, r.Priority, i+1)
		}
	}
	want := []string{"top10", "top10", "aboveAverage", "aboveAverage", "duplicateValues", "uniqueValues", "expression"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("rule %d type = %q, want %q", i, got[i], want[i])
		}
	}
	// top10 percent/bottom flags.
	if cf.Rules[0].Rank == nil || *cf.Rules[0].Rank != 3 || cf.Rules[0].Bottom || cf.Rules[0].Percent {
		t.Errorf("top rule = %+v", cf.Rules[0])
	}
	if cf.Rules[1].Rank == nil || *cf.Rules[1].Rank != 10 || !cf.Rules[1].Bottom || !cf.Rules[1].Percent {
		t.Errorf("bottom%% rule = %+v", cf.Rules[1])
	}
	// aboveAverage direction.
	if cf.Rules[2].AboveAverage == nil || !*cf.Rules[2].AboveAverage {
		t.Errorf("above rule = %+v", cf.Rules[2])
	}
	if cf.Rules[3].AboveAverage == nil || *cf.Rules[3].AboveAverage {
		t.Errorf("below rule = %+v", cf.Rules[3])
	}
	// expression formula.
	if len(cf.Rules[6].Formulas) != 1 || cf.Rules[6].Formulas[0] != "MOD(ROW(),2)=0" {
		t.Errorf("expression formula = %v", cf.Rules[6].Formulas)
	}
}

func TestConditionalFormat_TimePeriod(t *testing.T) {
	w, s := addCFSheet(t)
	style := DifferentialStyle{Fill: &FillStyle{FgColor: "FFC7CE"}}
	if err := s.AddConditionalFormat("A1:A10", NewTimePeriodRule("today", style)); err != nil {
		t.Fatalf("AddConditionalFormat: %v", err)
	}
	_, cf := firstCF(t, w)
	r := cf.Rules[0]
	if r.Type != "timePeriod" || r.TimePeriod != "today" {
		t.Errorf("rule = %+v", r)
	}
	if len(r.Formulas) != 1 || !strings.Contains(r.Formulas[0], "TODAY()") {
		t.Errorf("formula = %v", r.Formulas)
	}
}

// TestConditionalFormat_DxfDedup verifies identical differential formats share a
// single dxf entry while distinct ones each get their own.
func TestConditionalFormat_DxfDedup(t *testing.T) {
	w, s := addCFSheet(t)
	red := DifferentialStyle{Fill: &FillStyle{FgColor: "FFC7CE"}}
	green := DifferentialStyle{Fill: &FillStyle{FgColor: "C6EFCE"}}
	// Three rules: two identical (red) and one distinct (green).
	if err := s.AddConditionalFormat("A1:A10",
		NewCellIsRule(CondOpGreaterThan, red, "5"),
		NewCellIsRule(CondOpLessThan, red, "0"),
		NewCellIsRule(CondOpEqual, green, "3"),
	); err != nil {
		t.Fatalf("AddConditionalFormat: %v", err)
	}

	// The stylesheet must hold exactly two dxfs.
	if w.stylesheet == nil || w.stylesheet.Dxfs == nil {
		t.Fatal("no dxfs allocated")
	}
	if n := len(w.stylesheet.Dxfs.Dxf); n != 2 {
		t.Fatalf("dxf count = %d, want 2", n)
	}

	rw, cf := firstCF(t, w)
	_ = rw
	if len(cf.Rules) != 3 {
		t.Fatalf("Rules len = %d", len(cf.Rules))
	}
	d0, d1, d2 := cf.Rules[0].DxfId, cf.Rules[1].DxfId, cf.Rules[2].DxfId
	if d0 == nil || d1 == nil || d2 == nil {
		t.Fatal("a rule has nil DxfId")
	}
	if *d0 != *d1 {
		t.Errorf("identical styles got different dxfIds: %d vs %d", *d0, *d1)
	}
	if *d0 == *d2 {
		t.Errorf("distinct styles share a dxfId: %d", *d0)
	}
}

// TestConditionalFormat_MultipleBlocksCompose verifies successive
// AddConditionalFormat calls on one sheet each produce a block with
// monotonically increasing priorities.
func TestConditionalFormat_MultipleBlocksCompose(t *testing.T) {
	w, s := addCFSheet(t)
	style := DifferentialStyle{Fill: &FillStyle{FgColor: "FFC7CE"}}
	if err := s.AddConditionalFormat("A1:A10", NewCellIsRule(CondOpGreaterThan, style, "5")); err != nil {
		t.Fatal(err)
	}
	if err := s.AddConditionalFormat("B1:B10", NewCellIsRule(CondOpLessThan, style, "0")); err != nil {
		t.Fatal(err)
	}
	rw := reopen(t, w)
	cfs := firstSheet(t, rw).ConditionalFormats()
	if len(cfs) != 2 {
		t.Fatalf("blocks = %d, want 2", len(cfs))
	}
	if cfs[0].Rules[0].Priority != 1 || cfs[1].Rules[0].Priority != 2 {
		t.Errorf("priorities = %d, %d; want 1, 2", cfs[0].Rules[0].Priority, cfs[1].Rules[0].Priority)
	}
}

// TestConditionalFormat_AddToExistingBlockRoundTrip opens a workbook that
// already carries a conditional-format block, adds a new one, and verifies both
// survive a save/reopen — exercising the dirty re-marshal path alongside the
// preserved-bytes zero-mod path covered elsewhere.
func TestConditionalFormat_AddToExistingBlockRoundTrip(t *testing.T) {
	body := `<sheetData/>` +
		`<conditionalFormatting sqref="A1:A10">` +
		`<cfRule type="cellIs" dxfId="0" priority="1" operator="greaterThan"><formula>5</formula></cfRule>` +
		`</conditionalFormatting>`
	data := buildXLSXWithWorksheet(t, body)
	w, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	s := firstSheet(t, w)
	style := DifferentialStyle{Fill: &FillStyle{FgColor: "C6EFCE"}}
	if err := s.AddConditionalFormat("C1:C10", NewCellIsRule(CondOpLessThan, style, "0")); err != nil {
		t.Fatalf("AddConditionalFormat: %v", err)
	}
	rw := reopen(t, w)
	cfs := firstSheet(t, rw).ConditionalFormats()
	if len(cfs) != 2 {
		t.Fatalf("blocks = %d, want 2", len(cfs))
	}
	if cfs[0].SqRef != "A1:A10" || cfs[1].SqRef != "C1:C10" {
		t.Errorf("sqrefs = %q, %q", cfs[0].SqRef, cfs[1].SqRef)
	}
	// The new rule's priority must sit above the existing rule's (1).
	if cfs[1].Rules[0].Priority != 2 {
		t.Errorf("new rule priority = %d, want 2", cfs[1].Rules[0].Priority)
	}
}

func TestConditionalFormat_InvalidInputs(t *testing.T) {
	_, s := addCFSheet(t)
	style := DifferentialStyle{Fill: &FillStyle{FgColor: "FFC7CE"}}
	if err := s.AddConditionalFormat("A1:A10"); err == nil {
		t.Error("no rules: want error")
	}
	if err := s.AddConditionalFormat("not-a-range", NewCellIsRule(CondOpEqual, style, "1")); err == nil {
		t.Error("bad range: want error")
	}
	if err := s.AddConditionalFormat("A1:A10", NewTextRule("bogus", "x", style)); err == nil {
		t.Error("bad text operator: want error")
	}
}
