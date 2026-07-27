package chart_test

// C562: every format's AddChart used to point the caller's own *chart.Chart at
// its host sheet via SetDataRef, so one chart reused across sheets or formats
// ended up with the last host's sheet name in every copy of its references.

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mgilbir/spine/chart"
	"github.com/mgilbir/spine/docx"
	"github.com/mgilbir/spine/pptx"
	"github.com/mgilbir/spine/xlsx"
)

func newSharedChart() *chart.Chart {
	c := chart.NewColumn().SetTitle("Shared").SetCategories([]string{"a", "b"})
	c.AddSeries("S", []float64{1, 2})
	return c
}

// TestAddChartDoesNotMutateCaller checks each format's AddChart leaves the
// caller's chart alone.
func TestAddChartDoesNotMutateCaller(t *testing.T) {
	t.Run("xlsx", func(t *testing.T) {
		c := newSharedChart()
		wb := xlsx.Create()
		sheet := addSheetT(wb, "Host")
		if err := sheet.AddChart(c, "E2"); err != nil {
			t.Fatalf("AddChart: %v", err)
		}
		if c.DataRef != "Sheet1" {
			t.Errorf("caller's DataRef = %q, want the untouched Sheet1", c.DataRef)
		}
	})
	t.Run("docx", func(t *testing.T) {
		// docx sets the ref to the embedded workbook's sheet, which is derived
		// from the chart's own DataRef — so the mutation only shows when the
		// caller left it empty (the default resolves to "Sheet1").
		c := newSharedChart().SetDataRef("")
		doc := docx.Create()
		if err := doc.AddChart(c, 100, 100); err != nil {
			t.Fatalf("AddChart: %v", err)
		}
		if c.DataRef != "" {
			t.Errorf("caller's DataRef = %q, want it left empty", c.DataRef)
		}
	})
	t.Run("pptx", func(t *testing.T) {
		c := newSharedChart().SetDataRef("Mine")
		p := pptx.Create()
		if err := p.AddSlide().AddChart(c, 0, 0, 100, 100); err != nil {
			t.Fatalf("AddChart: %v", err)
		}
		if c.DataRef != "Mine" {
			t.Errorf("caller's DataRef = %q, want the untouched Mine", c.DataRef)
		}
	})
}

// TestChartReusedAcrossSheets checks the concrete symptom: one chart value
// added to two sheets must produce two chart parts, each referencing its own
// data sheet, rather than both referencing the last host's.
func TestChartReusedAcrossSheets(t *testing.T) {
	c := newSharedChart()
	wb := xlsx.Create()
	first := addSheetT(wb, "First")
	second := addSheetT(wb, "Second")
	if err := first.AddChart(c, "E2"); err != nil {
		t.Fatalf("AddChart(first): %v", err)
	}
	if err := second.AddChart(c, "E2"); err != nil {
		t.Fatalf("AddChart(second): %v", err)
	}
	data, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	one := string(zipPart(t, data, "xl/charts/chart1.xml"))
	two := string(zipPart(t, data, "xl/charts/chart2.xml"))
	if !strings.Contains(one, "ChartData1!") {
		t.Errorf("chart1 does not reference its own data sheet:\n%s", one)
	}
	if !strings.Contains(two, "ChartData2!") {
		t.Errorf("chart2 does not reference its own data sheet:\n%s", two)
	}
	if strings.Contains(one, "ChartData2!") {
		t.Errorf("chart1 references the second chart's data sheet:\n%s", one)
	}

	// Both data sheets carry the values.
	wb2, err := xlsx.OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	for _, name := range []string{"ChartData1", "ChartData2"} {
		ds, err := wb2.SheetByName(name)
		if err != nil {
			t.Fatalf("SheetByName(%s): %v", name, err)
		}
		if v, _ := ds.CellValue("B2"); v != "1" {
			t.Errorf("%s!B2 = %q, want 1", name, v)
		}
	}
}

// TestChartEditsAfterAddDoNotLeak checks the copy is a snapshot: editing the
// caller's chart after AddChart cannot change what was already attached.
func TestChartEditsAfterAddDoNotLeak(t *testing.T) {
	c := newSharedChart()
	wb := xlsx.Create()
	sheet := addSheetT(wb, "Host")
	if err := sheet.AddChart(c, "E2"); err != nil {
		t.Fatalf("AddChart: %v", err)
	}
	c.SetTitle("Changed")
	c.SeriesList()[0].Values[0] = 999

	data, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	saved := string(zipPart(t, data, "xl/charts/chart1.xml"))
	if strings.Contains(saved, "Changed") {
		t.Errorf("a post-AddChart title edit reached the saved part:\n%s", saved)
	}
	if strings.Contains(saved, "<c:v>999</c:v>") {
		t.Errorf("a post-AddChart value edit reached the saved part:\n%s", saved)
	}
}

// TestCloneIsDeep checks Clone copies the series values, not the backing array.
func TestCloneIsDeep(t *testing.T) {
	c := newSharedChart()
	cp := c.Clone()
	cp.SetTitle("Copy").SetDataRef("Other")
	cp.SeriesList()[0].Values[0] = 42
	cp.SeriesList()[0].Name = "Renamed"

	if c.Title() != "Shared" || c.DataRef != "Sheet1" {
		t.Errorf("original changed: title=%q dataRef=%q", c.Title(), c.DataRef)
	}
	if got := c.SeriesList()[0]; got.Values[0] != 1 || got.Name != "S" {
		t.Errorf("original series changed: %+v", got)
	}
	if cp.Categories()[0] != "a" {
		t.Errorf("clone lost its categories: %v", cp.Categories())
	}
}
