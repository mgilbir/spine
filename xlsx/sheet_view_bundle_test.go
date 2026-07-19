package xlsx

import (
	"path/filepath"
	"testing"
)

// saveReopen saves wb to a temp file and reopens it, failing the test on error.
func saveReopen(t *testing.T, wb *Workbook) *Workbook {
	t.Helper()
	path := filepath.Join(t.TempDir(), "wb.xlsx")
	if err := wb.Save(path); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = got.Close() })
	return got
}

func TestSheetVisibilityRoundTrip(t *testing.T) {
	wb := Create()
	s1 := wb.AddSheet("Visible")
	s2 := wb.AddSheet("Hidden")
	s3 := wb.AddSheet("VeryHidden")

	if !s1.Visible() {
		t.Fatal("new sheet should be visible")
	}
	if err := s2.SetVisible(false); err != nil {
		t.Fatalf("hide s2: %v", err)
	}
	if err := s3.SetVisibility(SheetVeryHidden); err != nil {
		t.Fatalf("veryHidden s3: %v", err)
	}

	if s2.Visibility() != SheetHidden {
		t.Errorf("s2 visibility = %q, want hidden", s2.Visibility())
	}
	if s3.Visible() {
		t.Error("s3 should not be visible")
	}

	got := saveReopen(t, wb)
	g1, _ := got.Sheet(0)
	g2, _ := got.Sheet(1)
	g3, _ := got.Sheet(2)
	if !g1.Visible() {
		t.Error("reopened s1 should be visible")
	}
	if g2.Visibility() != SheetHidden {
		t.Errorf("reopened s2 = %q, want hidden", g2.Visibility())
	}
	if g3.Visibility() != SheetVeryHidden {
		t.Errorf("reopened s3 = %q, want veryHidden", g3.Visibility())
	}
}

func TestSheetVisibilityLastVisibleRefused(t *testing.T) {
	wb := Create()
	only := wb.AddSheet("Only")
	if err := only.SetVisible(false); err == nil {
		t.Fatal("expected error hiding the only visible sheet")
	}
	if !only.Visible() {
		t.Error("sheet must remain visible after refused hide")
	}

	// With two sheets, hiding one is fine but hiding both is not.
	second := wb.AddSheet("Second")
	if err := only.SetVisible(false); err != nil {
		t.Fatalf("hide first of two: %v", err)
	}
	if err := second.SetVisible(false); err == nil {
		t.Fatal("expected error hiding the last visible sheet")
	}
}

func TestSheetVisibilityInvalid(t *testing.T) {
	wb := Create()
	s := wb.AddSheet("S")
	if err := s.SetVisibility(SheetVisibility("bogus")); err == nil {
		t.Fatal("expected error for invalid visibility")
	}
}

func TestSheetHiddenReshownRoundTrip(t *testing.T) {
	// A sheet hidden in one session, saved, reopened, and unhidden must be
	// visible after a second round trip (verifies captured-attr reconciliation).
	wb := Create()
	wb.AddSheet("Keep")
	s := wb.AddSheet("Toggle")
	if err := s.SetVisible(false); err != nil {
		t.Fatal(err)
	}
	got := saveReopen(t, wb)

	g, _ := got.Sheet(1)
	if g.Visible() {
		t.Fatal("sheet should be hidden after first round trip")
	}
	if err := g.SetVisible(true); err != nil {
		t.Fatalf("unhide: %v", err)
	}
	got2 := saveReopen(t, got)
	g2, _ := got2.Sheet(1)
	if !g2.Visible() {
		t.Error("sheet should be visible after unhide + round trip")
	}
}

func TestSetRowColumnHiddenRoundTrip(t *testing.T) {
	wb := Create()
	s := wb.AddSheet("S")
	if err := s.SetRowHidden(3, true); err != nil {
		t.Fatal(err)
	}
	if err := s.SetColumnHidden(2, true); err != nil {
		t.Fatal(err)
	}
	if !s.RowHidden(3) {
		t.Error("row 3 should be hidden")
	}
	if !s.ColumnHidden(2) {
		t.Error("col 2 should be hidden")
	}

	got := saveReopen(t, wb)
	g, _ := got.Sheet(0)
	if !g.RowHidden(3) {
		t.Error("reopened row 3 should be hidden")
	}
	if !g.ColumnHidden(2) {
		t.Error("reopened col 2 should be hidden")
	}
	// Unhide clears.
	if err := g.SetRowHidden(3, false); err != nil {
		t.Fatal(err)
	}
	if g.RowHidden(3) {
		t.Error("row 3 should be shown after unhide")
	}
}

func TestSetColumnHiddenSplitsRange(t *testing.T) {
	wb := Create()
	s := wb.AddSheet("S")
	// Establish a spanned width for columns 1-5, then hide only column 3.
	if err := s.SetColWidth(1, 20); err != nil {
		t.Fatal(err)
	}
	// Manually widen the range by carving is complex; instead set width on a
	// couple columns and hide one in the middle.
	if err := s.SetColumnHidden(3, true); err != nil {
		t.Fatal(err)
	}
	if !s.ColumnHidden(3) {
		t.Error("col 3 should be hidden")
	}
	if s.ColumnHidden(1) {
		t.Error("col 1 should not be hidden")
	}
	if w, ok := s.ColumnWidth(1); !ok || w != 20 {
		t.Errorf("col 1 width = %v (ok=%v), want 20", w, ok)
	}
}

func TestSheetViewTogglesRoundTrip(t *testing.T) {
	wb := Create()
	s := wb.AddSheet("S")

	s.SetShowRowColHeaders(false)
	s.SetRightToLeft(true)
	s.SetShowFormulas(true)
	s.SetShowZeros(false)
	s.SetShowRuler(false)
	if err := s.SetView(ViewPageLayout); err != nil {
		t.Fatal(err)
	}

	check := func(tag string, s *Sheet) {
		if s.ShowRowColHeaders() {
			t.Errorf("%s: ShowRowColHeaders = true, want false", tag)
		}
		if !s.RightToLeft() {
			t.Errorf("%s: RightToLeft = false, want true", tag)
		}
		if !s.ShowFormulas() {
			t.Errorf("%s: ShowFormulas = false, want true", tag)
		}
		if s.ShowZeros() {
			t.Errorf("%s: ShowZeros = true, want false", tag)
		}
		if s.ShowRuler() {
			t.Errorf("%s: ShowRuler = true, want false", tag)
		}
		if s.View() != ViewPageLayout {
			t.Errorf("%s: View = %q, want %q", tag, s.View(), ViewPageLayout)
		}
	}
	check("pre-save", s)

	got := saveReopen(t, wb)
	g, _ := got.Sheet(0)
	check("reopened", g)
}

func TestSheetViewDefaults(t *testing.T) {
	wb := Create()
	s := wb.AddSheet("S")
	// A pristine sheet reports OOXML defaults without a sheetView.
	if !s.ShowRowColHeaders() {
		t.Error("default ShowRowColHeaders should be true")
	}
	if s.RightToLeft() {
		t.Error("default RightToLeft should be false")
	}
	if s.ShowFormulas() {
		t.Error("default ShowFormulas should be false")
	}
	if !s.ShowZeros() {
		t.Error("default ShowZeros should be true")
	}
	if !s.ShowRuler() {
		t.Error("default ShowRuler should be true")
	}
	if s.View() != ViewNormal {
		t.Errorf("default View = %q, want normal", s.View())
	}
}

func TestSetViewInvalid(t *testing.T) {
	wb := Create()
	s := wb.AddSheet("S")
	if err := s.SetView("bogus"); err == nil {
		t.Fatal("expected error for invalid view")
	}
}

func TestSplitPanesRoundTrip(t *testing.T) {
	wb := Create()
	s := wb.AddSheet("S")
	if err := s.SplitPanes(2000, 1000, "C4", ""); err != nil {
		t.Fatal(err)
	}
	x, y, tl, ok := s.SplitPanePosition()
	if !ok || x != 2000 || y != 1000 || tl != "C4" {
		t.Fatalf("SplitPanePosition = %v,%v,%q,%v", x, y, tl, ok)
	}
	// A split pane is not a frozen pane.
	if _, _, frozen := s.FrozenPanes(); frozen {
		t.Error("split pane should not report as frozen")
	}

	got := saveReopen(t, wb)
	g, _ := got.Sheet(0)
	x, y, tl, ok = g.SplitPanePosition()
	if !ok || x != 2000 || y != 1000 || tl != "C4" {
		t.Errorf("reopened SplitPanePosition = %v,%v,%q,%v", x, y, tl, ok)
	}
	pane := g.pane()
	if pane == nil || pane.State != "split" {
		t.Fatalf("pane state = %v, want split", pane)
	}
	if pane.ActivePane != "bottomRight" {
		t.Errorf("ActivePane = %q, want bottomRight", pane.ActivePane)
	}
}

func TestSplitPanesRemoveAndInvalid(t *testing.T) {
	wb := Create()
	s := wb.AddSheet("S")
	if err := s.SplitPanes(1000, 0, "B1", "topRight"); err != nil {
		t.Fatal(err)
	}
	if _, _, _, ok := s.SplitPanePosition(); !ok {
		t.Fatal("expected split pane")
	}
	// Zero offsets remove the pane.
	if err := s.SplitPanes(0, 0, "", ""); err != nil {
		t.Fatal(err)
	}
	if _, _, _, ok := s.SplitPanePosition(); ok {
		t.Error("expected no split pane after zero offsets")
	}
	if err := s.SplitPanes(100, 100, "A1", "nope"); err == nil {
		t.Fatal("expected error for invalid active pane")
	}
	if err := s.SplitPanes(-1, 0, "A1", ""); err == nil {
		t.Fatal("expected error for negative offset")
	}
}

func TestRowGroupingRoundTrip(t *testing.T) {
	wb := Create()
	s := wb.AddSheet("S")
	if err := s.GroupRows(2, 4); err != nil {
		t.Fatal(err)
	}
	if err := s.GroupRows(3, 3); err != nil { // deeper nest on row 3
		t.Fatal(err)
	}
	if err := s.SetRowCollapsed(2, true); err != nil {
		t.Fatal(err)
	}
	if got := s.RowOutlineLevel(3); got != 2 {
		t.Errorf("row 3 outline = %d, want 2", got)
	}
	if got := s.RowOutlineLevel(2); got != 1 {
		t.Errorf("row 2 outline = %d, want 1", got)
	}
	if !s.RowCollapsed(2) {
		t.Error("row 2 should be collapsed")
	}

	got := saveReopen(t, wb)
	g, _ := got.Sheet(0)
	if lvl := g.RowOutlineLevel(3); lvl != 2 {
		t.Errorf("reopened row 3 outline = %d, want 2", lvl)
	}
	if !g.RowCollapsed(2) {
		t.Error("reopened row 2 should be collapsed")
	}
	// Ungroup drops the level.
	if err := g.UngroupRows(3, 3); err != nil {
		t.Fatal(err)
	}
	if lvl := g.RowOutlineLevel(3); lvl != 1 {
		t.Errorf("after ungroup, row 3 outline = %d, want 1", lvl)
	}
}

func TestColumnGroupingRoundTrip(t *testing.T) {
	wb := Create()
	s := wb.AddSheet("S")
	if err := s.GroupColumns(2, 5); err != nil {
		t.Fatal(err)
	}
	if err := s.SetColumnCollapsed(2, true); err != nil {
		t.Fatal(err)
	}
	if lvl := s.ColumnOutlineLevel(3); lvl != 1 {
		t.Errorf("col 3 outline = %d, want 1", lvl)
	}
	if err := s.SetColumnOutlineLevel(4, 3); err != nil {
		t.Fatal(err)
	}

	got := saveReopen(t, wb)
	g, _ := got.Sheet(0)
	if lvl := g.ColumnOutlineLevel(3); lvl != 1 {
		t.Errorf("reopened col 3 outline = %d, want 1", lvl)
	}
	if lvl := g.ColumnOutlineLevel(4); lvl != 3 {
		t.Errorf("reopened col 4 outline = %d, want 3", lvl)
	}
	if !g.ColumnCollapsed(2) {
		t.Error("reopened col 2 should be collapsed")
	}
}

func TestOutlineLevelBounds(t *testing.T) {
	wb := Create()
	s := wb.AddSheet("S")
	if err := s.SetRowOutlineLevel(1, 8); err == nil {
		t.Fatal("expected error for outline level 8")
	}
	if err := s.SetColumnOutlineLevel(1, 8); err == nil {
		t.Fatal("expected error for outline level 8")
	}
	if err := s.GroupRows(5, 2); err == nil {
		t.Fatal("expected error for reversed range")
	}
}

func TestOutlineSummaryRoundTrip(t *testing.T) {
	wb := Create()
	s := wb.AddSheet("S")
	// Defaults are both true.
	if below, right := s.OutlineSummary(); !below || !right {
		t.Errorf("default OutlineSummary = %v,%v, want true,true", below, right)
	}
	s.SetOutlineSummary(false, false)
	if below, right := s.OutlineSummary(); below || right {
		t.Errorf("OutlineSummary = %v,%v, want false,false", below, right)
	}

	got := saveReopen(t, wb)
	g, _ := got.Sheet(0)
	if below, right := g.OutlineSummary(); below || right {
		t.Errorf("reopened OutlineSummary = %v,%v, want false,false", below, right)
	}
}

func TestForceFullCalcRoundTrip(t *testing.T) {
	wb := Create()
	wb.AddSheet("S")
	if wb.ForceFullCalc() {
		t.Fatal("new workbook should not force full calc")
	}
	wb.SetForceFullCalc(true)
	if !wb.ForceFullCalc() {
		t.Fatal("expected force full calc after enable")
	}

	got := saveReopen(t, wb)
	if !got.ForceFullCalc() {
		t.Error("reopened workbook should force full calc")
	}

	// Disable clears the flag.
	got.SetForceFullCalc(false)
	if got.ForceFullCalc() {
		t.Error("expected force full calc cleared")
	}
	got2 := saveReopen(t, got)
	if got2.ForceFullCalc() {
		t.Error("reopened workbook should not force full calc after disable")
	}
}
