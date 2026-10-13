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

// forEachEncoding runs fn on the package with the named parts re-encoded as
// Windows-1252 and as UTF-16 of either byte order.
func forEachEncoding(t *testing.T, data []byte, parts []string, fn func(t *testing.T, wb *Workbook)) {
	t.Helper()
	for _, enc := range []struct {
		name   string
		encode func() ([]byte, error)
	}{
		{"windows-1252", func() ([]byte, error) { b, _, err := testutil.WithWindows1252(data, parts...); return b, err }},
		{"UTF-16LE", func() ([]byte, error) { return testutil.WithUTF16(data, false, parts...) }},
		{"UTF-16BE", func() ([]byte, error) { return testutil.WithUTF16(data, true, parts...) }},
	} {
		t.Run(enc.name, func(t *testing.T) {
			src, err := enc.encode()
			if err != nil {
				t.Fatal(err)
			}
			wb, err := OpenReader(bytes.NewReader(src), int64(len(src)))
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = wb.Close() }()
			fn(t, wb)
		})
	}
}

func TestFormControlsReadEncodedParts(t *testing.T) {
	forEachEncoding(t, buildXLSXWithControls(t), []string{"xl/worksheets/sheet1.xml"}, func(t *testing.T, wb *Workbook) {
		controls := wb.Sheets()[0].FormControls()
		if len(controls) != 1 || controls[0].Name != "Check Box 1" || controls[0].CtrlPropPart != "/xl/ctrlProps/ctrlProp1.xml" {
			t.Fatalf("FormControls = %+v", controls)
		}
	})
}

func TestAddCommentKeepsEncodedLegacyDrawing(t *testing.T) {
	forEachEncoding(t, buildXLSXWithFormControl(t), []string{"xl/drawings/vmlDrawing1.vml"}, func(t *testing.T, wb *Workbook) {
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
		if fcs := s2.FormControls(); len(fcs) != 1 || fcs[0].Type != FormControlCheckBox || fcs[0].Name != "Check Box 1" {
			t.Errorf("FormControls after save = %+v, want the checkbox alone", fcs)
		}
		if cs := s2.Comments(); len(cs) != 1 || cs[0].Text() != "à vérifier" {
			t.Errorf("Comments after save = %+v", cs)
		}
	})
}

func TestSlicersAndTimelinesReadEncodedParts(t *testing.T) {
	forEachEncoding(t, slicerFixture(t), []string{"xl/slicers/slicer1.xml", "xl/slicerCaches/slicerCache1.xml", "xl/timelines/timeline1.xml", "xl/timelineCaches/timelineCache1.xml"}, func(t *testing.T, wb *Workbook) {
		if sl := wb.Slicers(); len(sl) != 1 || sl[0].SourceField() != "Region" || sl[0].Caption() != "Region Filter" {
			t.Errorf("Slicers = %v", sl)
		}
		if tl := wb.Timelines(); len(tl) != 1 || tl[0].SourceField() != "Date" || tl[0].Caption() != "Date Range" {
			t.Errorf("Timelines = %v", tl)
		}
	})
}

func TestAddChartToEncodedDrawing(t *testing.T) {
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
	forEachEncoding(t, fuzzseed.EditZip(created, [][2]string{{"xl/drawings/drawing1.xml", drawing}}), []string{"xl/drawings/drawing1.xml", "xl/drawings/drawing1.xml"}, func(t *testing.T, wb *Workbook) {
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
	})
}
