package chart_test

// These tests exercise chart.EmbeddedWorkbook. chart builds the minimal
// workbook itself, so these could live in package chart; they stay in an
// external test package to open the result with the xlsx reader (an independent
// check that the bytes are a valid .xlsx) without an import cycle, since xlsx
// imports chart.

import (
	"bytes"
	"math"
	"strconv"
	"testing"

	"github.com/mgilbir/spine/chart"
	"github.com/mgilbir/spine/xlsx"
)

func absDiff(a, b float64) float64 { return math.Abs(a - b) }

// TestEmbeddedWorkbook builds the embedded workbook, opens it with the xlsx
// package, and verifies the values match the chart data and the returned
// layout references line up with the chart's own references.
func TestEmbeddedWorkbook(t *testing.T) {
	cats := []string{"Q1", "Q2", "Q3"}
	s1 := []float64{10, 20, 30}
	s2 := []float64{40, 50, 60}
	c := chart.NewColumn().SetCategories(cats)
	c.AddSeries("North", s1)
	c.AddSeries("South", s2)

	data, layout, err := c.EmbeddedWorkbook()
	if err != nil {
		t.Fatalf("EmbeddedWorkbook: %v", err)
	}
	if layout.Sheet != "Sheet1" {
		t.Errorf("layout sheet: got %q", layout.Sheet)
	}
	if layout.CategoriesRef != "Sheet1!$A$2:$A$4" {
		t.Errorf("categories ref: got %q", layout.CategoriesRef)
	}

	wb, err := xlsx.OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open embedded workbook: %v", err)
	}
	sheet, err := wb.SheetByName("Sheet1")
	if err != nil {
		t.Fatalf("SheetByName: %v", err)
	}

	cell := func(ref string) string {
		v, err := sheet.CellValue(ref)
		if err != nil {
			t.Fatalf("GetCellValue(%s): %v", ref, err)
		}
		return v
	}
	// Categories in column A rows 2..4.
	for i, want := range cats {
		if got := cell("A" + strconv.Itoa(i+2)); got != want {
			t.Errorf("A%d: got %q want %q", i+2, got, want)
		}
	}
	// Series names in row 1.
	if got := cell("B1"); got != "North" {
		t.Errorf("B1: got %q", got)
	}
	if got := cell("C1"); got != "South" {
		t.Errorf("C1: got %q", got)
	}
	// Series values.
	checkNum := func(ref string, want float64) {
		f, err := strconv.ParseFloat(cell(ref), 64)
		if err != nil {
			t.Fatalf("parse %s: %v", ref, err)
		}
		if absDiff(f, want) > 1e-9 {
			t.Errorf("%s: got %v want %v", ref, f, want)
		}
	}
	for i, v := range s1 {
		checkNum("B"+strconv.Itoa(i+2), v)
	}
	for i, v := range s2 {
		checkNum("C"+strconv.Itoa(i+2), v)
	}
}

// TestEmbeddedWorkbookMatchesChartRefs verifies the embedded-workbook layout's
// references equal the references emitted in the chart.xml, so the two line up.
func TestEmbeddedWorkbookMatchesChartRefs(t *testing.T) {
	c := chart.NewLine().SetCategories([]string{"a", "b"}).SetDataRef("Sheet1")
	c.AddSeries("S1", []float64{1, 2})

	xmlBytes, err := c.MarshalChartXML()
	if err != nil {
		t.Fatalf("MarshalChartXML: %v", err)
	}
	_, layout, err := c.EmbeddedWorkbook()
	if err != nil {
		t.Fatalf("EmbeddedWorkbook: %v", err)
	}
	refs := []string{layout.CategoriesRef, layout.Series[0].NameRef, layout.Series[0].ValuesRef}
	for _, ref := range refs {
		if !bytes.Contains(xmlBytes, []byte("<c:f>"+ref+"</c:f>")) {
			t.Errorf("chart.xml missing layout ref %q", ref)
		}
	}
}

func TestEmbeddedWorkbookScatter(t *testing.T) {
	x := []float64{1, 2, 3}
	y := []float64{9, 8, 7}
	c := chart.NewScatter()
	c.AddXYSeries("S", x, y)

	data, layout, err := c.EmbeddedWorkbook()
	if err != nil {
		t.Fatalf("EmbeddedWorkbook: %v", err)
	}
	if layout.Series[0].XValuesRef != "Sheet1!$A$2:$A$4" {
		t.Errorf("scatter x ref: got %q", layout.Series[0].XValuesRef)
	}
	wb, err := xlsx.OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	sheet, _ := wb.SheetByName("Sheet1")
	for i, want := range x {
		v, _ := sheet.CellValue("A" + strconv.Itoa(i+2))
		f, _ := strconv.ParseFloat(v, 64)
		if absDiff(f, want) > 1e-9 {
			t.Errorf("A%d: got %v want %v", i+2, f, want)
		}
	}
}
