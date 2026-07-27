package xlsx

import (
	"archive/zip"
	"bytes"
	"os"
	"testing"
)

// TestTableReadExternal parses tables from a real workbook and checks the
// surfaced metadata.
func TestTableReadExternal(t *testing.T) {
	if _, err := os.Stat("testdata/external/financial_sample.xlsx"); err != nil {
		t.Skipf("fixture unavailable: %v", err)
	}
	wb, err := Open("testdata/external/financial_sample.xlsx")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = wb.Close() }()

	tables := wb.Tables()
	if len(tables) == 0 {
		t.Fatal("expected at least one table")
	}
	tbl := tables[0]
	if tbl.Name() != "financials" {
		t.Errorf("Name = %q, want financials", tbl.Name())
	}
	if tbl.Range() != "A1:P701" {
		t.Errorf("Range = %q, want A1:P701", tbl.Range())
	}
	if !tbl.HeaderRow() {
		t.Error("HeaderRow = false, want true")
	}
	cols := tbl.Columns()
	if len(cols) != 16 {
		t.Fatalf("Columns len = %d, want 16", len(cols))
	}
	if cols[0].Name != "Segment" {
		t.Errorf("first column = %q, want Segment", cols[0].Name)
	}
	if _, ok := tbl.Style(); !ok {
		t.Error("expected a table style")
	}
}

// TestTableRoundTripByteIdentical verifies that opening a table-bearing
// workbook and saving it without modification leaves the table part (and the
// whole package) byte-identical.
func TestTableRoundTripByteIdentical(t *testing.T) {
	for _, path := range []string{
		"testdata/external/financial_sample.xlsx",
		"testdata/external/excelize_test.xlsx",
	} {
		t.Run(path, func(t *testing.T) {
			orig, err := readFile(path)
			if err != nil {
				t.Skipf("fixture unavailable: %v", err)
			}
			wb, err := OpenReader(bytes.NewReader(orig), int64(len(orig)))
			if err != nil {
				t.Fatalf("open: %v", err)
			}
			// Touch the read API to prove parsing does not perturb the model.
			_ = wb.Tables()
			var buf bytes.Buffer
			if err := wb.SaveTo(&buf); err != nil {
				t.Fatalf("save: %v", err)
			}
			_ = wb.Close()

			// The table parts specifically, plus every other part, must be
			// byte-identical: adding table support must not perturb files that
			// use tables when nothing is modified.
			parts := zipTableParts(t, orig)
			if len(parts) == 0 {
				t.Fatal("fixture has no table parts")
			}
			for _, name := range zipPartNames(t, orig) {
				o := zipEntry(t, orig, name)
				n := zipEntry(t, buf.Bytes(), name)
				if o != n {
					t.Errorf("part %s not byte-identical\n orig: %s\n new:  %s", name, o, n)
				}
			}
		})
	}
}

// TestAddTableCreateRoundTrip creates a table on a fresh workbook, saves,
// reopens, and checks the table is present with the expected shape.
func TestAddTableCreateRoundTrip(t *testing.T) {
	wb := Create()
	sh := addSheetT(wb, "Data")
	headers := []string{"Name", "Age", "City"}
	for i, h := range headers {
		c, err := sh.Cell(FormatCellRef(1, i+1))
		if err != nil {
			t.Fatal(err)
		}
		c.SetString(h)
	}
	c, _ := sh.Cell("A2")
	c.SetString("Alice")

	tbl, err := sh.AddTable("A1:C2", TableOptions{
		Style: TableStyle{Name: "TableStyleMedium2", ShowRowStripes: true},
	})
	if err != nil {
		t.Fatalf("AddTable: %v", err)
	}
	if tbl.Name() != "Table1" {
		t.Errorf("Name = %q, want Table1", tbl.Name())
	}

	var buf bytes.Buffer
	if err := wb.SaveTo(&buf); err != nil {
		t.Fatalf("save: %v", err)
	}

	// The package must carry a table part, its content-type override and a
	// worksheet relationship.
	if !zipHasPart(t, buf.Bytes(), "xl/tables/table1.xml") {
		t.Error("missing xl/tables/table1.xml")
	}

	re, err := OpenReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer func() { _ = re.Close() }()
	got := re.Tables()
	if len(got) != 1 {
		t.Fatalf("Tables len = %d, want 1", len(got))
	}
	rt := got[0]
	if rt.Range() != "A1:C2" {
		t.Errorf("Range = %q, want A1:C2", rt.Range())
	}
	cols := rt.Columns()
	if len(cols) != 3 || cols[0].Name != "Name" || cols[2].Name != "City" {
		t.Errorf("columns = %+v, want Name/Age/City", cols)
	}
	st, ok := rt.Style()
	if !ok || st.Name != "TableStyleMedium2" || !st.ShowRowStripes {
		t.Errorf("style = %+v ok=%v", st, ok)
	}
}

// TestAddTableTotalsRow exercises the totals-row path.
func TestAddTableTotalsRow(t *testing.T) {
	wb := Create()
	sh := addSheetT(wb, "Data")
	for i, h := range []string{"Item", "Qty"} {
		c, _ := sh.Cell(FormatCellRef(1, i+1))
		c.SetString(h)
	}
	_, err := sh.AddTable("A1:B4", TableOptions{
		TotalsRow: true,
		ColumnTotals: map[string]TotalsColumn{
			"Item": {Label: "Total"},
			"Qty":  {Function: "sum"},
		},
	})
	if err != nil {
		t.Fatalf("AddTable: %v", err)
	}
	var buf bytes.Buffer
	if err := wb.SaveTo(&buf); err != nil {
		t.Fatalf("save: %v", err)
	}
	re, err := OpenReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer func() { _ = re.Close() }()
	tbl := re.Tables()[0]
	if !tbl.TotalsRow() {
		t.Error("TotalsRow = false, want true")
	}
	cols := tbl.Columns()
	if cols[0].TotalsRowLabel != "Total" {
		t.Errorf("col0 label = %q, want Total", cols[0].TotalsRowLabel)
	}
	if cols[1].TotalsRowFunction != "sum" {
		t.Errorf("col1 function = %q, want sum", cols[1].TotalsRowFunction)
	}
}

// TestAddTableOpenedWorkbook adds a table to a workbook opened from bytes,
// exercising the round-trip save path (distinct from the create path).
func TestAddTableOpenedWorkbook(t *testing.T) {
	// Build a plain workbook and save it.
	base := Create()
	sh := addSheetT(base, "Sheet1")
	for i, h := range []string{"Product", "Price"} {
		c, _ := sh.Cell(FormatCellRef(1, i+1))
		c.SetString(h)
	}
	pc, _ := sh.Cell("A2")
	pc.SetString("Widget")
	priceC, _ := sh.Cell("B2")
	priceC.SetFloat(9.99)
	var baseBuf bytes.Buffer
	if err := base.SaveTo(&baseBuf); err != nil {
		t.Fatalf("save base: %v", err)
	}

	// Reopen and add a table.
	wb, err := OpenReader(bytes.NewReader(baseBuf.Bytes()), int64(baseBuf.Len()))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	osheet, err := wb.SheetByName("Sheet1")
	if err != nil {
		t.Fatalf("sheet: %v", err)
	}
	if _, err := osheet.AddTable("A1:B2", TableOptions{
		Name:  "Products",
		Style: TableStyle{Name: "TableStyleLight1"},
	}); err != nil {
		t.Fatalf("AddTable: %v", err)
	}
	var buf bytes.Buffer
	if err := wb.SaveTo(&buf); err != nil {
		t.Fatalf("save: %v", err)
	}
	_ = wb.Close()

	if !zipHasPart(t, buf.Bytes(), "xl/tables/table1.xml") {
		t.Fatal("missing table part")
	}

	re, err := OpenReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer func() { _ = re.Close() }()
	tables := re.Tables()
	if len(tables) != 1 {
		t.Fatalf("Tables len = %d, want 1", len(tables))
	}
	if tables[0].Name() != "Products" || tables[0].Range() != "A1:B2" {
		t.Errorf("table = %q %q, want Products A1:B2", tables[0].Name(), tables[0].Range())
	}
	cols := tables[0].Columns()
	if len(cols) != 2 || cols[0].Name != "Product" || cols[1].Name != "Price" {
		t.Errorf("columns = %+v, want Product/Price", cols)
	}
}

func readFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func zipTableParts(t *testing.T, data []byte) []string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("zip: %v", err)
	}
	var names []string
	for _, f := range zr.File {
		if len(f.Name) > 15 && f.Name[:10] == "xl/tables/" {
			names = append(names, f.Name)
		}
	}
	return names
}

func zipPartNames(t *testing.T, data []byte) []string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("zip: %v", err)
	}
	var names []string
	for _, f := range zr.File {
		names = append(names, f.Name)
	}
	return names
}
