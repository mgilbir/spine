package xlsx

import (
	"strings"
	"testing"
)

// removeChartDataSheet is AddChart's rollback: when writing a chart's data into
// the hidden data sheet fails, the sheet that was just appended has to be
// detached again. Its real risk is not the removal itself but the bookkeeping
// around it — the workbook keeps TWO parallel sheet lists (the handles and the
// workbook part's <sheets>) plus a positional index on each handle that scopes
// sheet-local defined names. Undoing only part of that leaves a workbook whose
// handles and part disagree, or whose later sheets are scoped one position off.
//
// The failing AddChart path cannot be provoked from a test without a chart of
// over a million data points (the only way cursors.cell errors), so the
// rollback is driven directly here and asserted on observable behavior
// afterwards: the surviving sheets keep their identity, their scoped names land
// on the right sheet, and the saved package has no orphan hidden sheet.

// After a rollback the workbook must look exactly as it did before the data
// sheet was appended: same handles, same workbook-part entries, same indices —
// and a defined name scoped to the sheet that followed the removed one must
// still resolve to that sheet.
func TestRemoveChartDataSheetRestoresSheetBookkeeping(t *testing.T) {
	w := Create()
	defer func() { _ = w.Close() }()
	first := addSheetT(w, "First")
	second := addSheetT(w, "Second")
	if err := first.SetCellValue("A1", "keep me"); err != nil {
		t.Fatalf("SetCellValue: %v", err)
	}

	beforeNames := sheetNamesOf(w)
	beforePartNames := workbookPartSheetNames(w)

	data := w.addChartDataSheet()
	if data.state != "hidden" {
		t.Fatalf("chart data sheet state = %q, want hidden", data.state)
	}
	if len(sheetNamesOf(w)) != len(beforeNames)+1 {
		t.Fatalf("addChartDataSheet did not append a sheet")
	}

	w.removeChartDataSheet(data)

	if got := sheetNamesOf(w); !stringsEqual(got, beforeNames) {
		t.Errorf("sheet handles after rollback = %v, want %v", got, beforeNames)
	}
	if got := workbookPartSheetNames(w); !stringsEqual(got, beforePartNames) {
		t.Errorf("workbook part sheets after rollback = %v, want %v", got, beforePartNames)
	}
	for i, s := range w.sheets {
		if s.index != i {
			t.Errorf("sheet %q has index %d after rollback, want %d", s.name, s.index, i)
		}
	}

	// Behavioral check on the index bookkeeping: a sheet-scoped defined name
	// set after the rollback must be scoped to the sheet that owns it. A stale
	// index would scope it to a different sheet (or off the end).
	if err := second.SetPrintTitles("$1:$1", ""); err != nil {
		t.Fatalf("SetPrintTitles: %v", err)
	}
	out, err := w.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	wbXML := string(readZipPart(t, out, "xl/workbook.xml"))
	if strings.Contains(wbXML, "ChartData") {
		t.Errorf("the rolled-back chart data sheet is still in the saved workbook:\n%s", wbXML)
	}
	if !strings.Contains(wbXML, `localSheetId="1"`) {
		t.Errorf("the scoped print-titles name is not scoped to the second sheet:\n%s", wbXML)
	}
	if strings.Contains(wbXML, `localSheetId="2"`) {
		t.Errorf("a defined name is scoped past the end of the sheet list:\n%s", wbXML)
	}

	// The reopened workbook has only the two real sheets, and the first one
	// still holds its content.
	re, _ := saveReopenSheetBytes(t, w)
	if got, _ := re.GetCellValue("A1"); got != "keep me" {
		t.Errorf("first sheet A1 after the rollback = %q, want %q", got, "keep me")
	}
}

// removeChartDataSheet renumbers the sheets that follow the one it removes.
// Today's only caller rolls back the sheet it just appended, so the removal is
// always at the tail and the renumbering never has anything to do — which means
// a regression there would be invisible until the day something removes a sheet
// from the middle. This pins the invariant the loop exists for: after a
// removal, every handle's index equals its position, since that index is what
// scopes sheet-local defined names.
func TestRemoveChartDataSheetRenumbersFollowingSheets(t *testing.T) {
	w := Create()
	defer func() { _ = w.Close() }()
	first := addSheetT(w, "First")
	addSheetT(w, "Second")
	addSheetT(w, "Third")

	w.removeChartDataSheet(first)

	if got := sheetNamesOf(w); !stringsEqual(got, []string{"Second", "Third"}) {
		t.Fatalf("sheets after removing the head = %v, want [Second Third]", got)
	}
	for i, s := range w.sheets {
		if s.index != i {
			t.Errorf("sheet %q has index %d, want %d — the sheets after the removed one were not renumbered",
				s.name, s.index, i)
		}
	}
	// The index is the defined-name scope, so a stale one is observable.
	if err := w.sheets[1].SetPrintTitles("$1:$1", ""); err != nil {
		t.Fatalf("SetPrintTitles: %v", err)
	}
	out, err := w.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	wbXML := string(readZipPart(t, out, "xl/workbook.xml"))
	if !strings.Contains(wbXML, `localSheetId="1"`) {
		t.Errorf("Third's print titles are not scoped to index 1:\n%s", wbXML)
	}
}

// Rolling back a sheet that is not in the workbook must change nothing: the
// loop's identity match is what keeps a second rollback (or a rollback of a
// sheet from another workbook) from truncating the sheet list.
func TestRemoveChartDataSheetIgnoresForeignSheets(t *testing.T) {
	w := Create()
	defer func() { _ = w.Close() }()
	addSheetT(w, "First")
	addSheetT(w, "Second")
	before := sheetNamesOf(w)

	data := w.addChartDataSheet()
	w.removeChartDataSheet(data)
	w.removeChartDataSheet(data) // second rollback of the same sheet

	other := Create()
	defer func() { _ = other.Close() }()
	foreign := addSheetT(other, "Elsewhere")
	w.removeChartDataSheet(foreign)

	if got := sheetNamesOf(w); !stringsEqual(got, before) {
		t.Errorf("sheets after redundant/foreign rollbacks = %v, want %v", got, before)
	}
	if got := workbookPartSheetNames(w); !stringsEqual(got, before) {
		t.Errorf("workbook part sheets after redundant/foreign rollbacks = %v, want %v", got, before)
	}
}

func sheetNamesOf(w *Workbook) []string {
	out := make([]string, 0, len(w.sheets))
	for _, s := range w.sheets {
		out = append(out, s.name)
	}
	return out
}

func workbookPartSheetNames(w *Workbook) []string {
	out := make([]string, 0, len(w.workbook.Sheets.Sheet))
	for _, s := range w.workbook.Sheets.Sheet {
		out = append(out, s.Name)
	}
	return out
}

func stringsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
