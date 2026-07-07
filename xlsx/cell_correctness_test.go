package xlsx

import (
	"math"
	"strings"
	"testing"
	"time"

	xmlb "github.com/mgilbir/spine/common/xml"
	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// C66: rows are emitted in ascending row-number order even when added out of
// order.
func TestRowsSortedOnMarshal(t *testing.T) {
	r5, r1 := uint32(5), uint32(1)
	sd := &oxml.CT_SheetData{Row: []oxml.CT_Row{
		{R: &r5, C: []*oxml.CT_Cell{{R: "A5"}}},
		{R: &r1, C: []*oxml.CT_Cell{{R: "A1"}}},
	}}
	b := xmlb.NewSpreadsheetMLBuilder()
	marshalSheetData(b, sd)
	out := string(b.Bytes())
	i1, i5 := strings.Index(out, `r="1"`), strings.Index(out, `r="5"`)
	if i1 < 0 || i5 < 0 || i1 > i5 {
		t.Errorf("rows not sorted ascending in output: %s", out)
	}
}

// C11: a *Cell handle stays valid after later cells are added to the same row.
func TestCellHandleStableAcrossAppends(t *testing.T) {
	wb := Create()
	s := wb.AddSheet("Sheet1")

	a1, err := s.Cell("A1")
	if err != nil {
		t.Fatal(err)
	}
	// Append several more cells to the same row, forcing slice growth.
	for _, ref := range []string{"B1", "C1", "D1", "E1", "F1", "G1", "H1", "I1"} {
		if _, err := s.Cell(ref); err != nil {
			t.Fatal(err)
		}
	}
	// Write through the original handle; it must not be detached.
	a1.SetString("kept")

	got, err := s.GetCellValue("A1")
	if err != nil {
		t.Fatal(err)
	}
	if got != "kept" {
		t.Errorf("A1 = %q, want %q (stale handle detached)", got, "kept")
	}
}

// C67: NaN/Inf become an error cell, not an invalid <v>NaN</v>.
func TestSetFloat_NaNInf(t *testing.T) {
	wb := Create()
	s := wb.AddSheet("Sheet1")
	for _, v := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		c, _ := s.Cell("A1")
		c.SetFloat(v)
		if c.Type() != CellTypeError {
			t.Errorf("SetFloat(%v): type = %v, want error cell", v, c.Type())
		}
	}
}

// C68: large int64/uint64 keep full precision instead of routing via float64.
func TestSetValue_Int64Precision(t *testing.T) {
	wb := Create()
	s := wb.AddSheet("Sheet1")

	const big int64 = 9007199254740993 // 2^53 + 1, not representable as float64
	c, _ := s.Cell("A1")
	c.SetValue(big)
	if got := c.String(); got != "9007199254740993" {
		t.Errorf("int64 lost precision: got %q", got)
	}

	const bigU uint64 = 18446744073709551615 // max uint64
	c2, _ := s.Cell("A2")
	c2.SetValue(bigU)
	if got := c2.String(); got != "18446744073709551615" {
		t.Errorf("uint64 lost precision: got %q", got)
	}
}

// C69: SetTime is timezone-independent and round-trips through Time().
func TestSetTime_TimezoneIndependent(t *testing.T) {
	wb := Create()
	s := wb.AddSheet("Sheet1")

	plus5 := time.FixedZone("UTC+5", 5*3600)
	d := time.Date(2024, 1, 15, 9, 30, 0, 0, plus5)

	c, _ := s.Cell("A1")
	c.SetTime(d)

	// The stored serial must reflect the wall-clock date (2024-01-15), so the
	// decoded date matches regardless of zone.
	got := c.Time()
	if got.Year() != 2024 || got.Month() != time.January || got.Day() != 15 {
		t.Errorf("SetTime/Time round-trip lost the date: got %v", got)
	}
	if got.Hour() != 9 || got.Minute() != 30 {
		t.Errorf("SetTime/Time round-trip lost the time: got %v", got)
	}
}

// C69: known Excel serial values (leap-day boundary and a modern date).
func TestExcelDateSerials(t *testing.T) {
	cases := []struct {
		t      time.Time
		serial float64
	}{
		{time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC), 1},
		{time.Date(1900, 2, 28, 0, 0, 0, 0, time.UTC), 59},
		{time.Date(1900, 3, 1, 0, 0, 0, 0, time.UTC), 61},
		{time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC), 45292},
	}
	for _, c := range cases {
		if got := timeToExcelDate(c.t); got != c.serial {
			t.Errorf("timeToExcelDate(%v) = %v, want %v", c.t, got, c.serial)
		}
	}
}

// C70: references outside the grid and overflow-shaped strings are rejected.
func TestParseCellRef_Bounds(t *testing.T) {
	valid := []string{"A1", "XFD1", "A1048576"}
	for _, ref := range valid {
		if _, _, err := ParseCellRef(ref); err != nil {
			t.Errorf("ParseCellRef(%q) unexpected error: %v", ref, err)
		}
	}
	invalid := []string{"XFE1", "A1048577", "AAAAAAAAAAAAAA1"}
	for _, ref := range invalid {
		if _, _, err := ParseCellRef(ref); err == nil {
			t.Errorf("ParseCellRef(%q) = nil error, want rejection", ref)
		}
	}
	if got := FormatCellRef(5, 0); got != "" {
		t.Errorf("FormatCellRef(5,0) = %q, want empty (column 0 invalid)", got)
	}
}

// C73: a row without the optional r attribute is addressable and not duplicated.
func TestRowWithoutRAttribute(t *testing.T) {
	wb := Create()
	s := wb.AddSheet("Sheet1")
	s.ensureWorksheet()
	// Simulate a parsed row that omitted r, carrying a cell whose ref implies row 3.
	v := "hello"
	s.worksheet.SheetData.Row = append(s.worksheet.SheetData.Row, oxml.CT_Row{
		C: []*oxml.CT_Cell{{R: "A3", T: "str", V: &v}},
	})

	if got, _ := s.GetCellValue("A3"); got != "hello" {
		t.Errorf("r-less row invisible: GetCellValue(A3) = %q", got)
	}
	before := len(s.worksheet.SheetData.Row)
	if _, err := s.Cell("A3"); err != nil {
		t.Fatal(err)
	}
	if len(s.worksheet.SheetData.Row) != before {
		t.Errorf("Cell(A3) duplicated the r-less row: rows %d -> %d", before, len(s.worksheet.SheetData.Row))
	}
}
