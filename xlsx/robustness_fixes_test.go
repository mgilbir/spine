package xlsx

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/mgilbir/spine/chart"
	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// TestAddChartMarshalFailureNoOrphanSheet verifies that when AddChart fails to
// marshal the chart (here: a chart with no series), no orphan hidden ChartData
// sheet is left in the workbook.
func TestAddChartMarshalFailureNoOrphanSheet(t *testing.T) {
	wb := Create()
	s := wb.AddSheet("Sheet1")
	before := wb.SheetCount()

	// A chart with no series fails to marshal.
	bad := chart.NewColumn().SetTitle("bad")
	if err := s.AddChart("E2", bad); err == nil {
		t.Fatal("AddChart with a series-less chart unexpectedly succeeded")
	}

	if got := wb.SheetCount(); got != before {
		t.Errorf("sheet count = %d after failed AddChart, want %d (orphan data sheet left)", got, before)
	}
	if _, err := wb.SheetByName("ChartData1"); err == nil {
		t.Errorf("orphan ChartData1 sheet left after failed AddChart")
	}
}

// TestAddChartAnchorGridClamp verifies the single-cell default chart span is
// clamped to the worksheet grid maxima.
func TestAddChartAnchorGridClamp(t *testing.T) {
	// Right-edge anchor: to-column clamps to XFD (0-based maxExcelColumns-1).
	_, fromRow, toCol, toRow, err := parseChartAnchor("XFA1")
	if err != nil {
		t.Fatalf("parseChartAnchor(XFA1): %v", err)
	}
	if toCol != maxExcelColumns-1 {
		t.Errorf("toCol = %d, want %d (clamped to grid)", toCol, maxExcelColumns-1)
	}
	if toRow != fromRow+defaultChartRows {
		t.Errorf("toRow = %d, want %d (row not clamped near top)", toRow, fromRow+defaultChartRows)
	}

	// Bottom-edge anchor: to-row clamps to the last row.
	_, _, _, toRow2, err := parseChartAnchor(FormatCellRef(maxExcelRows, 1))
	if err != nil {
		t.Fatalf("parseChartAnchor bottom-edge: %v", err)
	}
	if toRow2 != maxExcelRows-1 {
		t.Errorf("toRow = %d, want %d (clamped to last row)", toRow2, maxExcelRows-1)
	}

	// A mid-grid anchor keeps the full default span.
	fromCol, fRow, tCol, tRow, err := parseChartAnchor("B2")
	if err != nil {
		t.Fatalf("parseChartAnchor(B2): %v", err)
	}
	if tCol != fromCol+defaultChartCols || tRow != fRow+defaultChartRows {
		t.Errorf("mid-grid span = [%d,%d]->[%d,%d], want full default", fromCol, fRow, tCol, tRow)
	}
}

// TestReplacedOpenedHyperlinkRelNotReemitted verifies that replacing a
// hyperlink that was loaded from an opened workbook drops its old external
// relationship from the saved sheet .rels (rather than leaking a stale URL).
func TestReplacedOpenedHyperlinkRelNotReemitted(t *testing.T) {
	w := Create()
	s := w.AddSheet("S")
	c, err := s.Cell("A1")
	if err != nil {
		t.Fatal(err)
	}
	c.SetString("link")
	c.SetHyperlink("https://old.example.com/OLD")

	// Reopen so the hyperlink relationship is loaded from the file (kept in
	// w.relationships[partName], not the pending list).
	rw := reopen(t, w)
	rs := firstSheet(t, rw)
	rc, err := rs.Cell("A1")
	if err != nil {
		t.Fatal(err)
	}
	rc.SetHyperlink("https://new.example.com/NEW") // replace

	out, err := rw.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}

	zr, err := zip.NewReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatal(err)
	}
	var oldSeen, newSeen bool
	for _, f := range zr.File {
		if !strings.HasSuffix(f.Name, ".rels") {
			continue
		}
		rdr, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, _ := io.ReadAll(rdr)
		rdr.Close()
		if strings.Contains(string(b), "old.example.com/OLD") {
			oldSeen = true
		}
		if strings.Contains(string(b), "new.example.com/NEW") {
			newSeen = true
		}
	}
	if oldSeen {
		t.Errorf("saved .rels still references the replaced (old) hyperlink target")
	}
	if !newSeen {
		t.Errorf("saved .rels missing the new hyperlink target")
	}
}

// TestSetColWidthCarvesAllColsGroups verifies SetColWidth carves the target
// column out of a covering entry even when that entry lives in a later <cols>
// group, leaving no overlapping/stale entry.
func TestSetColWidthCarvesAllColsGroups(t *testing.T) {
	wb := Create()
	s := wb.AddSheet("S")
	w10, w20 := 10.0, 20.0
	// Two separate <cols> groups; the 2nd covers the target column (4).
	s.ws().Cols = []oxml.CT_Cols{
		{Col: []oxml.CT_Col{{Min: 1, Max: 2, Width: &w10}}},
		{Col: []oxml.CT_Col{{Min: 3, Max: 5, Width: &w20}}},
	}

	if err := s.SetColWidth(4, 30); err != nil {
		t.Fatal(err)
	}

	// Count entries covering column 4 across every group: must be exactly one,
	// with the new width.
	covering := 0
	var got *oxml.CT_Col
	for gi := range s.ws().Cols {
		for j := range s.ws().Cols[gi].Col {
			e := &s.ws().Cols[gi].Col[j]
			if e.Min <= 4 && 4 <= e.Max {
				covering++
				got = e
			}
		}
	}
	if covering != 1 {
		t.Fatalf("column 4 covered by %d entries, want exactly 1", covering)
	}
	if got.Width == nil || *got.Width != 30 {
		t.Errorf("carved entry width = %v, want 30", got.Width)
	}
	if got.Min != 4 || got.Max != 4 {
		t.Errorf("carved entry = [%d,%d], want [4,4]", got.Min, got.Max)
	}
}

// TestOrphanThreadedReplySurfaced verifies that a threaded reply whose parentId
// matches no thread root (e.g. the root was deleted) is surfaced as a top-level
// comment rather than being silently dropped.
func TestOrphanThreadedReplySurfaced(t *testing.T) {
	w := &Workbook{}
	w.persons = &oxml.CT_PersonList{}
	w.personsLoaded = true
	s := &Sheet{workbook: w}
	s.comments = &sheetComments{
		loaded:       true,
		threadedPart: "/xl/threadedComments/threadedComment1.xml",
		threaded: &oxml.CT_ThreadedComments{Comments: []oxml.CT_ThreadedComment{
			{Ref: "A1", ID: "{root}", Text: "the root"},
			// Orphan reply: parentId points at a nonexistent root.
			{Ref: "A1", ID: "{orphan}", ParentID: "{ghost}", Text: "orphaned reply text"},
		}},
	}
	w.sheets = []*Sheet{s}

	var texts []string
	for _, c := range s.Comments() {
		texts = append(texts, c.Text())
	}

	found := false
	for _, txt := range texts {
		if txt == "orphaned reply text" {
			found = true
		}
	}
	if !found {
		t.Errorf("orphan reply text not surfaced; got comment texts %v", texts)
	}
}

// TestAddPivotTableNoPhantomCells verifies that scanning the source range while
// building a pivot table does not add phantom empty cells/rows to the source
// sheet model.
func TestAddPivotTableNoPhantomCells(t *testing.T) {
	wb := Create()
	data := wb.AddSheet("Data")
	put := func(ref string, v interface{}) {
		c, err := data.Cell(ref)
		if err != nil {
			t.Fatal(err)
		}
		c.SetValue(v)
	}
	put("A1", "Region")
	put("B1", "Product")
	put("C1", "Sales")
	put("A2", "North")
	put("B2", "A")
	put("C2", 10.0)
	put("A3", "North") // B3 intentionally left blank
	put("C3", 20.0)
	put("A4", "South")
	put("B4", "A")
	put("C4", 30.0)
	put("A5", "South")
	put("B5", "B")
	put("C5", 40.0)
	wb.AddSheet("Report")

	countCells := func(s *Sheet) (rows, cells int) {
		for i := range s.ws().SheetData.Row {
			rows++
			cells += len(s.ws().SheetData.Row[i].C)
		}
		return
	}
	beforeRows, beforeCells := countCells(data)

	report, err := wb.SheetByName("Report")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := report.AddPivotTable("Data!A1:C5", "A3", PivotOptions{
		RowFields:   []string{"Region", "Product"},
		ValueFields: []PivotValueField{{Field: "Sales", Aggregation: PivotSum}},
	}); err != nil {
		t.Fatalf("AddPivotTable: %v", err)
	}

	afterRows, afterCells := countCells(data)
	if afterRows != beforeRows || afterCells != beforeCells {
		t.Errorf("source sheet model changed: rows %d->%d, cells %d->%d (phantom entries created)",
			beforeRows, afterRows, beforeCells, afterCells)
	}
}

// TestStyleNoOpDoesNotDirty verifies that a NewCellStyle / AddNumberFormat call
// which resolves to an already-existing record does not mark styles modified
// (which would force styles.xml regeneration and break byte-identical
// round-trip on producer-formatted files).
func TestStyleNoOpDoesNotDirty(t *testing.T) {
	wb := Create()
	sm := wb.Styles()

	idx, err := sm.NewCellStyle(CellStyle{Format: "0.00"})
	if err != nil {
		t.Fatal(err)
	}
	nfID := sm.AddNumberFormat("0.000") // custom (not built-in)

	// Observe subsequent no-ops from a clean state.
	wb.stylesDirty = false

	idx2, err := sm.NewCellStyle(CellStyle{Format: "0.00"})
	if err != nil {
		t.Fatal(err)
	}
	if idx2 != idx {
		t.Errorf("NewCellStyle dedup index = %d, want %d", idx2, idx)
	}
	if wb.stylesDirty {
		t.Errorf("NewCellStyle no-op set stylesDirty")
	}

	if got := sm.AddNumberFormat("0.000"); got != nfID {
		t.Errorf("AddNumberFormat dedup id = %d, want %d", got, nfID)
	}
	if wb.stylesDirty {
		t.Errorf("AddNumberFormat no-op (existing custom) set stylesDirty")
	}

	if got := sm.AddNumberFormat("0.00"); got != 2 {
		t.Errorf("AddNumberFormat built-in id = %d, want 2", got)
	}
	if wb.stylesDirty {
		t.Errorf("AddNumberFormat no-op (built-in) set stylesDirty")
	}

	// A genuinely new record must still dirty.
	if _, err := sm.NewCellStyle(CellStyle{Format: "0.0000"}); err != nil {
		t.Fatal(err)
	}
	if !wb.stylesDirty {
		t.Errorf("new style did not set stylesDirty")
	}
}

// TestParseCellRefMixedCase verifies ParseCellRef accepts column prefixes with
// mixed letter case, matching Excel's leniency.
func TestParseCellRefMixedCase(t *testing.T) {
	cases := []struct {
		ref      string
		row, col int
	}{
		{"Aa1", 1, 27},  // "AA" -> column 27
		{"aB3", 3, 28},  // "AB" -> column 28
		{"A1", 1, 1},    // baseline upper
		{"a1", 1, 1},    // baseline lower
		{"zZ100", 100, 702},
	}
	for _, c := range cases {
		row, col, err := ParseCellRef(c.ref)
		if err != nil {
			t.Errorf("ParseCellRef(%q) returned error: %v", c.ref, err)
			continue
		}
		if row != c.row || col != c.col {
			t.Errorf("ParseCellRef(%q) = (row %d, col %d), want (row %d, col %d)", c.ref, row, col, c.row, c.col)
		}
	}
}

// TestSheetsReturnsCopy verifies that mutating the slice returned by Sheets()
// does not affect the workbook's internal sheet order or per-sheet index.
func TestSheetsReturnsCopy(t *testing.T) {
	wb := Create()
	a := wb.AddSheet("Alpha")
	b := wb.AddSheet("Beta")
	c := wb.AddSheet("Gamma")
	_ = a
	_ = b
	_ = c

	got := wb.Sheets()
	if len(got) != 3 {
		t.Fatalf("Sheets() len = %d, want 3", len(got))
	}

	// Reverse and truncate the returned slice.
	got[0], got[2] = got[2], got[0]
	got = got[:1]
	_ = got

	// The workbook must be unaffected.
	after := wb.Sheets()
	if len(after) != 3 {
		t.Fatalf("workbook sheet count changed to %d, want 3", len(after))
	}
	wantOrder := []string{"Alpha", "Beta", "Gamma"}
	for i, name := range wantOrder {
		if after[i].Name() != name {
			t.Errorf("sheet[%d] = %q, want %q", i, after[i].Name(), name)
		}
		if after[i].index != i {
			t.Errorf("sheet[%d].index = %d, want %d", i, after[i].index, i)
		}
	}
}
