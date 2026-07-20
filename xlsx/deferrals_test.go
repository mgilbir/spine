package xlsx

import (
	"strings"
	"testing"
)

// --- Formula authoring ------------------------------------------------------

// SetArrayFormula writes a t="array" master with a ref spanning the spill
// range, and it survives a save/reopen round-trip.
func TestSetArrayFormulaRoundTrip(t *testing.T) {
	w := Create()
	s := w.AddSheet("Sheet1")
	c, err := s.Cell("C1")
	if err != nil {
		t.Fatal(err)
	}
	c.SetArrayFormula("A1:A3*B1:B3", "C1:C3")

	out, err := marshalWorksheetXML(s.worksheet)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(out), `<f t="array" ref="C1:C3">A1:A3*B1:B3</f>`) {
		t.Errorf("array formula not emitted as expected:\n%s", out)
	}

	rc, err := firstSheet(t, reopen(t, w)).Cell("C1")
	if err != nil {
		t.Fatal(err)
	}
	if rc.cell.F == nil || rc.cell.F.T != "array" || rc.cell.F.Ref != "C1:C3" {
		t.Errorf("reopened array formula = %+v", rc.cell.F)
	}
	if rc.Formula() != "A1:A3*B1:B3" {
		t.Errorf("reopened formula = %q", rc.Formula())
	}
}

// SetDynamicArrayFormula marks the master with aca/ca, and those flags round-trip.
func TestSetDynamicArrayFormulaRoundTrip(t *testing.T) {
	w := Create()
	s := w.AddSheet("Sheet1")
	c, err := s.Cell("D2")
	if err != nil {
		t.Fatal(err)
	}
	c.SetDynamicArrayFormula("SORT(A2:A10)", "")

	out, err := marshalWorksheetXML(s.worksheet)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(out), `<f t="array" aca="1" ref="D2" ca="1">SORT(A2:A10)</f>`) {
		t.Errorf("dynamic-array formula not emitted as expected:\n%s", out)
	}

	rc, err := firstSheet(t, reopen(t, w)).Cell("D2")
	if err != nil {
		t.Fatal(err)
	}
	f := rc.cell.F
	if f == nil || f.T != "array" || f.Aca == nil || !*f.Aca || f.Ca == nil || !*f.Ca {
		t.Errorf("reopened dynamic-array flags not preserved: %+v", f)
	}
}

// SetSharedFormula sets a master with ref+si and fills the range with follower
// stubs sharing the index; it round-trips and passes validation.
func TestSetSharedFormulaRoundTrip(t *testing.T) {
	w := Create()
	s := w.AddSheet("Sheet1")
	c, err := s.Cell("A1")
	if err != nil {
		t.Fatal(err)
	}
	if err := c.SetSharedFormula("B1*2", "A1:A3"); err != nil {
		t.Fatalf("SetSharedFormula: %v", err)
	}

	// Master carries the formula, ref and si; followers are empty si-only stubs.
	if c.cell.F.T != "shared" || c.cell.F.Ref != "A1:A3" || c.cell.F.Si == nil {
		t.Fatalf("master formula = %+v", c.cell.F)
	}
	si := *c.cell.F.Si
	for _, ref := range []string{"A2", "A3"} {
		fc, err := s.Cell(ref)
		if err != nil {
			t.Fatal(err)
		}
		if fc.cell.F == nil || fc.cell.F.T != "shared" || fc.cell.F.Ref != "" ||
			fc.cell.F.Si == nil || *fc.cell.F.Si != si || fc.cell.F.Value != "" {
			t.Errorf("follower %s = %+v", ref, fc.cell.F)
		}
	}

	if rep := w.Validate(); rep.HasErrors() {
		t.Fatalf("validate reported errors: %v", rep)
	}

	rs := firstSheet(t, reopen(t, w))
	rc, err := rs.Cell("A1")
	if err != nil {
		t.Fatal(err)
	}
	if rc.cell.F == nil || rc.cell.F.Ref != "A1:A3" || rc.cell.F.Si == nil {
		t.Errorf("reopened master = %+v", rc.cell.F)
	}
}

// A shared-formula master must be the top-left of the range.
func TestSetSharedFormulaRejectsNonAnchor(t *testing.T) {
	w := Create()
	s := w.AddSheet("Sheet1")
	c, _ := s.Cell("B2")
	if err := c.SetSharedFormula("X", "A1:C3"); err == nil {
		t.Fatal("SetSharedFormula on a non-anchor cell must error")
	}
}

// --- Data validation advanced attributes ------------------------------------

func TestDataValidationErrorStyleImeMode(t *testing.T) {
	w := Create()
	s := w.AddSheet("Sheet1")
	if err := s.AddDataValidation(DataValidation{
		Range:      "A1:A10",
		Type:       "whole",
		Operator:   "between",
		Formula1:   "1",
		Formula2:   "10",
		ErrorStyle: ValidationErrorWarning,
		ImeMode:    "off",
	}); err != nil {
		t.Fatalf("AddDataValidation: %v", err)
	}

	out, err := marshalWorksheetXML(s.worksheet)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(out), `errorStyle="warning"`) || !strings.Contains(string(out), `imeMode="off"`) {
		t.Errorf("errorStyle/imeMode not emitted:\n%s", out)
	}

	dvs := firstSheet(t, reopen(t, w)).DataValidations()
	if len(dvs) != 1 {
		t.Fatalf("reopened DataValidations len = %d", len(dvs))
	}
	if dvs[0].ErrorStyle != "warning" || dvs[0].ImeMode != "off" {
		t.Errorf("reopened dv = %+v", dvs[0])
	}
}

// --- Comment rich text ------------------------------------------------------

func TestCommentRichTextRoundTrip(t *testing.T) {
	w := Create()
	s := w.AddSheet("Sheet1")
	runs := []TextRun{
		{Text: "Total: ", Font: &FontStyle{Bold: true}},
		{Text: "see notes", Font: &FontStyle{Italic: true, Color: "FF0000"}},
	}
	cm := s.AddNoteRichText("A1", "Reviewer", runs)
	if cm.Text() != "Total: see notes" {
		t.Errorf("plain Text() = %q", cm.Text())
	}
	if got := cm.RichText(); len(got) != 2 || got[0].Font == nil || !got[0].Font.Bold {
		t.Errorf("RichText() = %+v", got)
	}

	rs := firstSheet(t, reopen(t, w))
	rcm := rs.Comments()
	if len(rcm) != 1 {
		t.Fatalf("reopened comments len = %d", len(rcm))
	}
	if rcm[0].Text() != "Total: see notes" {
		t.Errorf("reopened Text() = %q", rcm[0].Text())
	}
	got := rcm[0].RichText()
	if len(got) != 2 {
		t.Fatalf("reopened RichText() len = %d", len(got))
	}
	if got[0].Font == nil || !got[0].Font.Bold {
		t.Errorf("reopened run[0] lost bold: %+v", got[0].Font)
	}
	if got[1].Font == nil || !got[1].Font.Italic || !strings.EqualFold(got[1].Font.Color, "FFFF0000") {
		t.Errorf("reopened run[1] formatting: %+v", got[1].Font)
	}
}

// SetRichText on a threaded comment formats the legacy fallback note while the
// thread body stays the flattened plain text; plain Text() keeps working.
func TestCommentSetRichTextThreaded(t *testing.T) {
	w := Create()
	s := w.AddSheet("Sheet1")
	cm := s.AddComment("A1", "Alice", "placeholder")
	cm.SetRichText([]TextRun{
		{Text: "Bold", Font: &FontStyle{Bold: true}},
		{Text: " and plain"},
	})
	if cm.Text() != "Bold and plain" {
		t.Errorf("Text() after SetRichText = %q", cm.Text())
	}

	rs := firstSheet(t, reopen(t, w))
	for _, c := range rs.Comments() {
		if c.Ref() == "A1" && c.Text() != "Bold and plain" {
			t.Errorf("reopened threaded Text() = %q", c.Text())
		}
	}
}

// --- Sparkline mutate / color / markers / delete ----------------------------

func TestSparklineMutateColorsMarkers(t *testing.T) {
	w := Create()
	s := w.AddSheet("Sheet1")
	g, err := s.AddSparklineGroup(SparklineOptions{
		Type: SparklineLine,
		Data: []SparklineData{{DataRange: "Sheet1!A1:D1", LocationCell: "E1"}},
	})
	if err != nil {
		t.Fatalf("AddSparklineGroup: %v", err)
	}
	g.SetSeriesColor("376092")
	g.SetNegativeColor("FF0000")
	g.SetMarkersColor("00B050")
	g.SetMarkers(true)
	g.SetHigh(true)

	rg := firstSheet(t, reopen(t, w)).Sparklines()
	if len(rg) != 1 {
		t.Fatalf("reopened groups len = %d", len(rg))
	}
	if !strings.EqualFold(rg[0].SeriesColor(), "FF376092") {
		t.Errorf("series color = %q", rg[0].SeriesColor())
	}
	if !rg[0].Markers() {
		t.Error("markers not preserved")
	}
	if rg[0].g.ColorNegative == nil || !strings.EqualFold(rg[0].g.ColorNegative.Rgb, "FFFF0000") {
		t.Errorf("negative color = %+v", rg[0].g.ColorNegative)
	}
	if rg[0].g.ColorMarkers == nil || !strings.EqualFold(rg[0].g.ColorMarkers.Rgb, "FF00B050") {
		t.Errorf("markers color = %+v", rg[0].g.ColorMarkers)
	}
	if rg[0].g.High == nil || !*rg[0].g.High {
		t.Error("high flag not preserved")
	}
}

func TestSparklineDelete(t *testing.T) {
	w := Create()
	s := w.AddSheet("Sheet1")
	if _, err := s.AddSparklineGroup(SparklineOptions{
		Data: []SparklineData{{DataRange: "Sheet1!A1:D1", LocationCell: "E1"}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddSparklineGroup(SparklineOptions{
		Data: []SparklineData{{DataRange: "Sheet1!A2:D2", LocationCell: "E2"}},
	}); err != nil {
		t.Fatal(err)
	}

	groups := s.Sparklines()
	if len(groups) != 2 {
		t.Fatalf("groups len = %d, want 2", len(groups))
	}
	groups[0].Delete()

	after := firstSheet(t, reopen(t, w)).Sparklines()
	if len(after) != 1 {
		t.Fatalf("after delete groups len = %d, want 1", len(after))
	}
	if after[0].Sparklines()[0].LocationCell != "E2" {
		t.Errorf("wrong group survived: %+v", after[0].Sparklines())
	}

	// Deleting the last group (re-fetched from the live sheet) removes the
	// sparkline extension entirely.
	remaining := s.Sparklines()
	if len(remaining) != 1 {
		t.Fatalf("live sheet groups len = %d, want 1", len(remaining))
	}
	remaining[0].Delete()
	if s.worksheet.ExtLst != nil && findSparklineExt(s.worksheet.ExtLst) != nil {
		t.Error("deleting the last group must remove the sparkline extension")
	}
	rs := firstSheet(t, reopen(t, w))
	if len(rs.Sparklines()) != 0 {
		t.Errorf("sparklines should be empty after deleting all")
	}
}
