package xlsx

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mgilbir/spine/chart"
	xmlb "github.com/mgilbir/spine/common/xml"
	"github.com/mgilbir/spine/internal/fuzzseed"
	"github.com/mgilbir/spine/internal/testutil"
)

// The readers below walk a part with a token loop of their own, or splice new
// markup into its bytes. They used a decoder with no charset support, so on a
// part declaring a code page they failed and, being best-effort, said nothing:
// form controls lost their names, slicers and timelines read as absent,
// adding a comment beside a form control regenerated the legacy drawing from
// scratch and dropped the control, and a chart added to a sheet with a drawing
// took a shape id the drawing already used.

func openWindows1252(t *testing.T, data []byte, parts ...string) *Workbook {
	t.Helper()
	src, _, err := testutil.WithWindows1252(data, parts...)
	if err != nil {
		t.Fatal(err)
	}
	wb, err := OpenReader(bytes.NewReader(src), int64(len(src)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = wb.Close() })
	return wb
}

func TestFormControlsReadWindows1252(t *testing.T) {
	wb := openWindows1252(t, buildXLSXWithControls(t), "xl/worksheets/sheet1.xml")
	controls := wb.Sheets()[0].FormControls()
	if len(controls) != 1 || controls[0].Name != "Check Box 1" || controls[0].CtrlPropPart != "/xl/ctrlProps/ctrlProp1.xml" {
		t.Fatalf("FormControls = %+v", controls)
	}
}

func TestAddCommentKeepsWindows1252LegacyDrawing(t *testing.T) {
	wb := openWindows1252(t, buildXLSXWithFormControl(t), "xl/drawings/vmlDrawing1.vml")
	s, _ := wb.Sheet(0)
	s.AddComment("D4", "Relecteur", "à vérifier")
	out, err := wb.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	vml, err := xmlb.UTF8(readZipPart(t, out, "xl/drawings/vmlDrawing1.vml"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(vml, []byte(`ObjectType="Checkbox"`)) || !bytes.Contains(vml, []byte(`ObjectType="Note"`)) {
		t.Errorf("legacy drawing lost the control or the note:\n%s", vml)
	}
	re, err := OpenReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = re.Close() }()
	s2, _ := re.Sheet(0)
	found := false
	for _, fc := range s2.FormControls() {
		found = found || fc.Type == FormControlCheckBox && fc.Name == "Check Box 1"
	}
	if !found {
		t.Errorf("FormControls after save = %+v", s2.FormControls())
	}
	if cs := s2.Comments(); len(cs) != 1 || cs[0].Text() != "à vérifier" {
		t.Errorf("Comments after save = %+v", cs)
	}
}

func TestSlicersAndTimelinesReadWindows1252(t *testing.T) {
	wb := openWindows1252(t, slicerFixture(t), "xl/slicers/slicer1.xml", "xl/slicerCaches/slicerCache1.xml", "xl/timelines/timeline1.xml", "xl/timelineCaches/timelineCache1.xml")
	if sl := wb.Slicers(); len(sl) != 1 || sl[0].SourceField() != "Region" || sl[0].Caption() != "Region Filter" {
		t.Errorf("Slicers = %v", sl)
	}
	if tl := wb.Timelines(); len(tl) != 1 || tl[0].SourceField() != "Date" || tl[0].Caption() != "Date Range" {
		t.Errorf("Timelines = %v", tl)
	}
}

func TestAddChartToWindows1252Drawing(t *testing.T) {
	seed := Create()
	sheet := addSheetT(seed, "Data")
	c1 := chart.NewColumn().SetTitle("First").SetCategories([]string{"a", "b"})
	c1.AddSeries("S1", []float64{1, 2})
	if err := sheet.AddChart(c1, "B2"); err != nil {
		t.Fatal(err)
	}
	created, err := seed.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	drawing := string(fuzzseed.ZipEntry(created, "xl/drawings/drawing1.xml"))
	if !strings.Contains(drawing, `<xdr:cNvPr id="1" `) {
		t.Fatalf("fixture drawing has no shape 1:\n%s", drawing)
	}
	drawing = strings.Replace(drawing, "</xdr:wsDr>", "<!-- tableau de bord – été --></xdr:wsDr>", 1)
	wb := openWindows1252(t, fuzzseed.EditZip(created, [][2]string{{"xl/drawings/drawing1.xml", drawing}}), "xl/drawings/drawing1.xml")
	s, err := wb.SheetByName("Data")
	if err != nil {
		t.Fatal(err)
	}
	c2 := chart.NewLine().SetTitle("Second").SetCategories([]string{"a", "b"})
	c2.AddSeries("S2", []float64{3, 4})
	if err = s.AddChart(c2, "H2"); err != nil {
		t.Fatal(err)
	}
	out, err := wb.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	merged, err := xmlb.UTF8(readZipPart(t, out, "xl/drawings/drawing1.xml"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Count(merged, []byte(`<xdr:cNvPr id="1" `)) != 1 || !bytes.Contains(merged, []byte("<!-- tableau de bord – été -->")) {
		t.Errorf("merged drawing:\n%s", merged)
	}
	re, err := OpenReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = re.Close() }()
	rs, _ := re.SheetByName("Data")
	if n := len(rs.Charts()); n != 2 {
		t.Errorf("Charts after save = %d, want 2", n)
	}
}
