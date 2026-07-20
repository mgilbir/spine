package xlsx

import (
	"encoding/xml"
	"strings"
	"testing"

	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// A worksheet carrying a real Excel sparkline extension. The x14 prefix is
// declared on the ext, xm on the sparklineGroups element, matching Excel.
const sparklineWorksheetXML = `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006" mc:Ignorable="x14ac" xmlns:x14ac="http://schemas.microsoft.com/office/spreadsheetml/2009/9/ac">` +
	`<dimension ref="A1:E2"/>` +
	`<sheetData><row r="1"><c r="A1"><v>1</v></c><c r="B1"><v>2</v></c><c r="C1"><v>3</v></c><c r="D1"><v>4</v></c></row></sheetData>` +
	`<extLst>` +
	`<ext uri="{05C60535-1F16-4fd2-B633-F4F36F0B64E0}" xmlns:x14="http://schemas.microsoft.com/office/spreadsheetml/2009/9/main">` +
	`<x14:sparklineGroups xmlns:xm="http://schemas.microsoft.com/office/excel/2006/main">` +
	`<x14:sparklineGroup displayEmptyCellsAs="gap" markers="1">` +
	`<x14:colorSeries theme="4" tint="-0.499984740745262"/>` +
	`<x14:colorNegative theme="5"/>` +
	`<x14:colorMarkers theme="4" tint="-0.499984740745262"/>` +
	`<x14:sparklines>` +
	`<x14:sparkline><xm:f>Sheet1!A1:D1</xm:f><xm:sqref>E1</xm:sqref></x14:sparkline>` +
	`</x14:sparklines>` +
	`</x14:sparklineGroup>` +
	`</x14:sparklineGroups>` +
	`</ext>` +
	`</extLst>` +
	`</worksheet>`

// An existing sparkline extension must survive a re-marshal of a dirty sheet
// byte-for-byte: sparklines are captured raw in extLst and only re-serialized
// when a caller adds or modifies one.
func TestSparklinePreservesExistingExtVerbatim(t *testing.T) {
	var ws oxml.CT_Worksheet
	if err := xml.Unmarshal([]byte(sparklineWorksheetXML), &ws); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	data, err := marshalWorksheetXML(&ws)
	if err != nil {
		t.Fatalf("marshalWorksheetXML: %v", err)
	}
	out := string(data)

	want := `<ext uri="{05C60535-1F16-4fd2-B633-F4F36F0B64E0}" xmlns:x14="http://schemas.microsoft.com/office/spreadsheetml/2009/9/main">` +
		`<x14:sparklineGroups xmlns:xm="http://schemas.microsoft.com/office/excel/2006/main">` +
		`<x14:sparklineGroup displayEmptyCellsAs="gap" markers="1">` +
		`<x14:colorSeries theme="4" tint="-0.499984740745262"/>` +
		`<x14:colorNegative theme="5"/>` +
		`<x14:colorMarkers theme="4" tint="-0.499984740745262"/>` +
		`<x14:sparklines>` +
		`<x14:sparkline><xm:f>Sheet1!A1:D1</xm:f><xm:sqref>E1</xm:sqref></x14:sparkline>` +
		`</x14:sparklines>` +
		`</x14:sparklineGroup>` +
		`</x14:sparklineGroups>` +
		`</ext>`
	if !strings.Contains(out, want) {
		t.Errorf("sparkline extension not preserved verbatim:\n%s", out)
	}
}

// Sheet.Sparklines exposes the type, series color and (dataRange, locationCell)
// pairs parsed from an existing sparkline extension.
func TestSparklineRead(t *testing.T) {
	var ws oxml.CT_Worksheet
	if err := xml.Unmarshal([]byte(sparklineWorksheetXML), &ws); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	s := &Sheet{worksheet: &ws}
	groups := s.Sparklines()
	if len(groups) != 1 {
		t.Fatalf("Sparklines() len = %d, want 1", len(groups))
	}
	g := groups[0]
	if g.Type() != SparklineLine {
		t.Errorf("Type() = %q, want line", g.Type())
	}
	pairs := g.Sparklines()
	if len(pairs) != 1 {
		t.Fatalf("group Sparklines() len = %d, want 1", len(pairs))
	}
	if pairs[0].DataRange != "Sheet1!A1:D1" || pairs[0].LocationCell != "E1" {
		t.Errorf("pair = %+v, want {Sheet1!A1:D1 E1}", pairs[0])
	}
}

// Creating a sparkline group on a fresh sheet writes a well-formed extension
// that reopens with the same type, color and mappings.
func TestSparklineAddRoundTrip(t *testing.T) {
	w := Create()
	s := w.AddSheet("Sheet1")
	for _, ref := range []string{"A1", "B1", "C1", "D1"} {
		if _, err := s.Cell(ref); err != nil {
			t.Fatalf("Cell(%s): %v", ref, err)
		}
	}
	g, err := s.AddSparklineGroup(SparklineOptions{
		Type:        SparklineColumn,
		SeriesColor: "376092",
		Data: []SparklineData{
			{DataRange: "Sheet1!A1:D1", LocationCell: "E1"},
			{DataRange: "Sheet1!A2:D2", LocationCell: "E2"},
		},
	})
	if err != nil {
		t.Fatalf("AddSparklineGroup: %v", err)
	}
	if g.Type() != SparklineColumn {
		t.Errorf("Type() = %q, want column", g.Type())
	}

	rw := reopen(t, w)
	groups := firstSheet(t, rw).Sparklines()
	if len(groups) != 1 {
		t.Fatalf("after reopen Sparklines() len = %d, want 1", len(groups))
	}
	rg := groups[0]
	if rg.Type() != SparklineColumn {
		t.Errorf("reopened Type() = %q, want column", rg.Type())
	}
	if !strings.EqualFold(rg.SeriesColor(), "FF376092") {
		t.Errorf("reopened SeriesColor() = %q, want FF376092", rg.SeriesColor())
	}
	pairs := rg.Sparklines()
	if len(pairs) != 2 {
		t.Fatalf("reopened pairs len = %d, want 2", len(pairs))
	}
	if pairs[0].DataRange != "Sheet1!A1:D1" || pairs[0].LocationCell != "E1" {
		t.Errorf("pair[0] = %+v", pairs[0])
	}
	if pairs[1].DataRange != "Sheet1!A2:D2" || pairs[1].LocationCell != "E2" {
		t.Errorf("pair[1] = %+v", pairs[1])
	}
}

// Win/loss maps to Excel's "stacked" type, and adding to a sheet that already
// has a group appends rather than replacing.
func TestSparklineAppendWinLoss(t *testing.T) {
	var ws oxml.CT_Worksheet
	if err := xml.Unmarshal([]byte(sparklineWorksheetXML), &ws); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	s := &Sheet{worksheet: &ws}
	if _, err := s.AddSparklineGroup(SparklineOptions{
		Type: SparklineWinLoss,
		Data: []SparklineData{{DataRange: "Sheet1!A2:D2", LocationCell: "E2"}},
	}); err != nil {
		t.Fatalf("AddSparklineGroup: %v", err)
	}

	groups := s.Sparklines()
	if len(groups) != 2 {
		t.Fatalf("Sparklines() len = %d, want 2 (existing + appended)", len(groups))
	}
	if groups[0].Type() != SparklineLine {
		t.Errorf("existing group Type() = %q, want line", groups[0].Type())
	}
	if groups[1].Type() != SparklineWinLoss {
		t.Errorf("appended group Type() = %q, want winloss", groups[1].Type())
	}

	data, err := marshalWorksheetXML(&ws)
	if err != nil {
		t.Fatalf("marshalWorksheetXML: %v", err)
	}
	out := string(data)
	if !strings.Contains(out, `type="stacked"`) {
		t.Errorf("win/loss group not emitted as stacked:\n%s", out)
	}
	// Both groups live under a single sparklineGroups element in one ext.
	if n := strings.Count(out, "<x14:sparklineGroups"); n != 1 {
		t.Errorf("sparklineGroups count = %d, want 1", n)
	}
	if n := strings.Count(out, "<x14:sparklineGroup "); n != 2 {
		t.Errorf("sparklineGroup count = %d, want 2", n)
	}
}

// A group with no mappings is rejected; no empty sparkline extension is written.
func TestSparklineAddRejectsEmpty(t *testing.T) {
	w := Create()
	s := w.AddSheet("Sheet1")
	if _, err := s.AddSparklineGroup(SparklineOptions{Type: SparklineLine}); err == nil {
		t.Fatal("AddSparklineGroup with no data must error")
	}
	if s.worksheet != nil && s.worksheet.ExtLst != nil && findSparklineExt(s.worksheet.ExtLst) != nil {
		t.Error("rejected AddSparklineGroup must not create a sparkline extension")
	}
}
