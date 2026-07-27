package xlsx

import (
	"archive/zip"
	"bytes"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/mgilbir/spine/chart"
	"github.com/mgilbir/spine/opc"
)

// ---------------------------------------------------------------------------
// C422 — DeleteSheet cascade must reach OLE embeddings, ctrlProps and slicers
// ---------------------------------------------------------------------------

// buildXLSXWithSheetPrivateParts builds a two-sheet workbook whose first sheet
// owns an embedded OLE object, a control-properties part and a slicer, each
// wired through a worksheet relationship and carrying a content-type override.
// The second sheet owns nothing, so deleting the first must take all three
// parts (and their overrides) with it.
func buildXLSXWithSheetPrivateParts(t *testing.T) []byte {
	t.Helper()
	const (
		ctOLE      = "application/vnd.openxmlformats-officedocument.oleObject"
		ctCtrlProp = "application/vnd.ms-excel.controlproperties+xml"
		ctSlicer   = "application/vnd.ms-excel.slicer+xml"
	)
	parts := map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
			`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
			`<Default Extension="xml" ContentType="application/xml"/>` +
			`<Default Extension="bin" ContentType="` + ctOLE + `"/>` +
			`<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>` +
			`<Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>` +
			`<Override PartName="/xl/worksheets/sheet2.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>` +
			`<Override PartName="/xl/ctrlProps/ctrlProp1.xml" ContentType="` + ctCtrlProp + `"/>` +
			`<Override PartName="/xl/slicers/slicer1.xml" ContentType="` + ctSlicer + `"/>` +
			`</Types>`,
		"_rels/.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId1" Type="` + opc.RelTypeOfficeDocument + `" Target="xl/workbook.xml"/></Relationships>`,
		"xl/workbook.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">` +
			`<sheets><sheet name="Objects" sheetId="1" r:id="rId1"/><sheet name="Keep" sheetId="2" r:id="rId2"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId1" Type="` + opc.RelTypeWorksheet + `" Target="worksheets/sheet1.xml"/>` +
			`<Relationship Id="rId2" Type="` + opc.RelTypeWorksheet + `" Target="worksheets/sheet2.xml"/>` +
			`</Relationships>`,
		"xl/worksheets/sheet1.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">` +
			`<sheetData/></worksheet>`,
		"xl/worksheets/sheet2.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">` +
			`<sheetData/></worksheet>`,
		"xl/worksheets/_rels/sheet1.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rIdOle" Type="` + opc.RelTypeOLEObject + `" Target="../embeddings/oleObject1.bin"/>` +
			`<Relationship Id="rIdCtrl" Type="` + relTypeCtrlProp + `" Target="../ctrlProps/ctrlProp1.xml"/>` +
			`<Relationship Id="rIdSlicer" Type="` + opc.RelTypeSlicer + `" Target="../slicers/slicer1.xml"/>` +
			`</Relationships>`,
		"xl/embeddings/oleObject1.bin": strings.Repeat("OLE-PAYLOAD", 64),
		"xl/ctrlProps/ctrlProp1.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<formControlPr xmlns="http://schemas.microsoft.com/office/spreadsheetml/2009/9/main" objectType="CheckBox"/>`,
		"xl/slicers/slicer1.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<slicers xmlns="http://schemas.microsoft.com/office/spreadsheetml/2009/9/main"/>`,
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range parts {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("zip create %s: %v", name, err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatalf("zip write %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return buf.Bytes()
}

// TestDeleteSheetDropsOLEControlAndSlicerParts guards C422: sheetPrivateRelTypes
// seeded only drawing/table/vml/comments, so a deleted sheet's OLE embedding
// (often megabytes), ctrlProps part and slicer survived in preservedParts with
// their content-type overrides but no incoming relationship.
func TestDeleteSheetDropsOLEControlAndSlicerParts(t *testing.T) {
	src := buildXLSXWithSheetPrivateParts(t)
	wb, err := OpenReader(bytes.NewReader(src), int64(len(src)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	if err := wb.DeleteSheet(0); err != nil {
		t.Fatalf("DeleteSheet: %v", err)
	}
	out, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	names := zipNames(t, out)
	for _, gone := range []string{
		"xl/embeddings/oleObject1.bin",
		"xl/ctrlProps/ctrlProp1.xml",
		"xl/slicers/slicer1.xml",
	} {
		if names[gone] {
			t.Errorf("%s survived DeleteSheet: orphan part with no incoming relationship", gone)
		}
	}

	// The overrides must go with the parts, or the package carries a
	// content-type override naming a part that does not exist.
	ct := string(readZipPart(t, out, "[Content_Types].xml"))
	for _, gone := range []string{"/xl/ctrlProps/ctrlProp1.xml", "/xl/slicers/slicer1.xml"} {
		if strings.Contains(ct, gone) {
			t.Errorf("[Content_Types].xml still overrides removed part %s", gone)
		}
	}

	// The surviving sheet is untouched.
	if !names["xl/worksheets/sheet2.xml"] {
		t.Error("surviving sheet part was removed")
	}
}

// ---------------------------------------------------------------------------
// C423 — feature mutators on an opaque sheet must report, not silently discard
// ---------------------------------------------------------------------------

// opaqueSheet returns the chartsheet from buildChartsheetWorkbook.
func opaqueSheet(t *testing.T) (*Workbook, *Sheet) {
	t.Helper()
	src := buildChartsheetWorkbook(t)
	wb, err := OpenReader(bytes.NewReader(src), int64(len(src)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	sh, err := wb.SheetByName("Chart")
	if err != nil {
		t.Fatalf("SheetByName(Chart): %v", err)
	}
	return wb, sh
}

// TestOpaqueSheetFeatureMutatorsReport guards C423: every feature mutator that
// relies on markDirty returned success on a chartsheet while markDirty
// deliberately refused it, so the change was discarded at save. A chartsheet
// genuinely supports protection and page setup in Excel, which made
// chartsheet.Protect(...) succeeding-then-vanishing a particularly sharp trap.
func TestOpaqueSheetFeatureMutatorsReport(t *testing.T) {
	_, sh := opaqueSheet(t)

	t.Run("Protect", func(t *testing.T) {
		if err := sh.Protect(SheetProtectionOptions{}); !errors.Is(err, ErrNotWorksheet) {
			t.Errorf("Protect = %v, want ErrNotWorksheet", err)
		}
	})
	t.Run("SetPageSetup", func(t *testing.T) {
		if err := sh.SetPageSetup(PageSetup{Orientation: OrientationLandscape}); !errors.Is(err, ErrNotWorksheet) {
			t.Errorf("SetPageSetup = %v, want ErrNotWorksheet", err)
		}
	})
	t.Run("SetPrintOptions", func(t *testing.T) {
		if err := sh.SetPrintOptions(PrintOptions{}); !errors.Is(err, ErrNotWorksheet) {
			t.Errorf("SetPrintOptions = %v, want ErrNotWorksheet", err)
		}
	})
	t.Run("SetHeaderFooter", func(t *testing.T) {
		if err := sh.SetHeaderFooter(HeaderFooter{OddHeader: "&Cx"}); !errors.Is(err, ErrNotWorksheet) {
			t.Errorf("SetHeaderFooter = %v, want ErrNotWorksheet", err)
		}
	})
	t.Run("AddScenario", func(t *testing.T) {
		err := sh.AddScenario(Scenario{Name: "S", Inputs: []ScenarioInput{{Cell: "A1", Value: "1"}}})
		if !errors.Is(err, ErrNotWorksheet) {
			t.Errorf("AddScenario = %v, want ErrNotWorksheet", err)
		}
	})
	t.Run("AddSparklineGroup", func(t *testing.T) {
		_, err := sh.AddSparklineGroup(SparklineOptions{
			Data: []SparklineData{{DataRange: "Data!A1:D1", LocationCell: "E1"}},
		})
		if !errors.Is(err, ErrNotWorksheet) {
			t.Errorf("AddSparklineGroup = %v, want ErrNotWorksheet", err)
		}
	})
	t.Run("SetFilterColumn", func(t *testing.T) {
		if err := sh.SetFilterColumn(FilterColumn{ColID: 0}); !errors.Is(err, ErrNotWorksheet) {
			t.Errorf("SetFilterColumn = %v, want ErrNotWorksheet", err)
		}
	})
	t.Run("SetSortState", func(t *testing.T) {
		if err := sh.SetSortState(SortState{Ref: "A1:B2"}); !errors.Is(err, ErrNotWorksheet) {
			t.Errorf("SetSortState = %v, want ErrNotWorksheet", err)
		}
	})
	t.Run("AddTable", func(t *testing.T) {
		if _, err := sh.AddTable("A1:B3", TableOptions{}); !errors.Is(err, ErrNotWorksheet) {
			t.Errorf("AddTable = %v, want ErrNotWorksheet", err)
		}
	})
	t.Run("AddConditionalFormat", func(t *testing.T) {
		rule := NewCellIsRule(CondOpGreaterThan, DifferentialStyle{}, "1")
		if err := sh.AddConditionalFormat("A1:A5", rule); !errors.Is(err, ErrNotWorksheet) {
			t.Errorf("AddConditionalFormat = %v, want ErrNotWorksheet", err)
		}
	})
	t.Run("AddOLEObject", func(t *testing.T) {
		err := sh.AddOLEObject(OLEObjectSpec{Data: []byte("x"), ProgID: "Package"})
		if !errors.Is(err, ErrNotWorksheet) {
			t.Errorf("AddOLEObject = %v, want ErrNotWorksheet", err)
		}
	})
	t.Run("AddComment", func(t *testing.T) {
		if c := sh.AddComment("A1", "me", "hi"); c != nil {
			t.Error("AddComment on an opaque sheet returned a comment that will not be saved")
		}
	})
	// Swept alongside the findings named in C423: same shape, same file family.
	t.Run("SetPageMargins", func(t *testing.T) {
		if err := sh.SetPageMargins(PageMargins{Left: 1}); !errors.Is(err, ErrNotWorksheet) {
			t.Errorf("SetPageMargins = %v, want ErrNotWorksheet", err)
		}
	})
	t.Run("AddNote", func(t *testing.T) {
		if c := sh.AddNote("A1", "me", "hi"); c != nil {
			t.Error("AddNote on an opaque sheet returned a note that will not be saved")
		}
	})
	t.Run("AddNoteRichText", func(t *testing.T) {
		if c := sh.AddNoteRichText("A1", "me", []TextRun{{Text: "hi"}}); c != nil {
			t.Error("AddNoteRichText on an opaque sheet returned a note that will not be saved")
		}
	})
}

// TestOpaqueSheetIsNeverDirtied pins the invariant behind C423: no feature
// mutator, accepted or refused, may leave an opaque sheet marked dirty — a
// dirty opaque sheet is treated as a rewritable worksheet on save (C241) and
// perturbs dropCalcChain/needRelsRebuild.
func TestOpaqueSheetIsNeverDirtied(t *testing.T) {
	_, sh := opaqueSheet(t)

	_ = sh.Protect(SheetProtectionOptions{})
	_ = sh.SetPageSetup(PageSetup{Orientation: OrientationLandscape})
	_ = sh.SetPageMargins(PageMargins{Left: 1})
	_ = sh.SetPrintOptions(PrintOptions{})
	_ = sh.SetHeaderFooter(HeaderFooter{OddHeader: "&Cx"})
	_ = sh.AddScenario(Scenario{Name: "S", Inputs: []ScenarioInput{{Cell: "A1", Value: "1"}}})
	_ = sh.SetFilterColumn(FilterColumn{ColID: 0})
	_ = sh.SetSortState(SortState{Ref: "A1:B2"})
	_ = sh.AddImage("A1", testPNG(t, 4, 4), ImageOptions{})
	_, _ = sh.AddTable("A1:B3", TableOptions{})
	_, _ = sh.AddSparklineGroup(SparklineOptions{
		Data: []SparklineData{{DataRange: "Data!A1:D1", LocationCell: "E1"}},
	})
	sh.AddComment("A1", "me", "hi")
	sh.AddNote("A1", "me", "hi")
	sh.Unprotect()
	sh.ClearFilterColumns()
	sh.RemoveSortState()

	if sh.dirty {
		t.Error("an opaque sheet was left marked dirty by a feature mutator")
	}
}

// TestOpaqueSheetAddImageAndChartReport guards the sharpest half of C423:
// AddImage and AddChart accepted the attachment, set s.dirty directly (bypassing
// the opaque guard in markDirty), and were then skipped wholesale by
// saveOpenedSheetAttachments — the image was silently dropped.
func TestOpaqueSheetAddImageAndChartReport(t *testing.T) {
	_, sh := opaqueSheet(t)

	if err := sh.AddImage("A1", testPNG(t, 4, 4), ImageOptions{}); !errors.Is(err, ErrNotWorksheet) {
		t.Errorf("AddImage = %v, want ErrNotWorksheet", err)
	}
	c := chart.NewBar()
	c.AddSeries("S", []float64{1})
	if err := sh.AddChart("A1", c); !errors.Is(err, ErrNotWorksheet) {
		t.Errorf("AddChart = %v, want ErrNotWorksheet", err)
	}
	// The refused calls must not have queued anything or dirtied the sheet.
	if len(sh.images) != 0 || len(sh.charts) != 0 {
		t.Errorf("refused attachments were still queued: %d images, %d charts", len(sh.images), len(sh.charts))
	}
	if sh.dirty {
		t.Error("opaque sheet was marked dirty, which perturbs dropCalcChain/needRelsRebuild")
	}
}

// TestAddImageRoutesDirtyThroughMarkDirty pins that AddImage no longer sets
// s.dirty directly. On a worksheet the flag must still be set.
func TestAddImageRoutesDirtyThroughMarkDirty(t *testing.T) {
	w := Create()
	s := w.AddSheet("S")
	if err := s.AddImage("B2", testPNG(t, 4, 4), ImageOptions{}); err != nil {
		t.Fatalf("AddImage: %v", err)
	}
	if !s.dirty {
		t.Error("AddImage on a worksheet did not mark the sheet dirty")
	}
}

// ---------------------------------------------------------------------------
// C427 — a two-cell anchor must not claim editAs="oneCell"
// ---------------------------------------------------------------------------

// TestTwoCellAnchorEditAs guards C427: ImageOptions.ToCell documents an image
// that "moves and resizes with the cells" and picXML emits a 0x0 a:ext for
// Excel to derive the size from the anchors, but the anchor said
// editAs="oneCell", which per ST_EditAs means move-but-do-not-resize.
func TestTwoCellAnchorEditAs(t *testing.T) {
	w := Create()
	s := w.AddSheet("S")
	if err := s.AddImage("B2", testPNG(t, 4, 4), ImageOptions{ToCell: "E10"}); err != nil {
		t.Fatalf("AddImage: %v", err)
	}
	out, err := w.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	drawing := string(readZipPart(t, out, "xl/drawings/drawing1.xml"))
	if strings.Contains(drawing, `editAs="oneCell"`) {
		t.Error(`two-cell anchor emitted editAs="oneCell", which pins the size and contradicts ToCell's documented resize-with-cells`)
	}
	if !strings.Contains(drawing, `<xdr:twoCellAnchor editAs="twoCell">`) {
		t.Errorf("want a twoCell-edited twoCellAnchor, got:\n%s", drawing)
	}
}

// ---------------------------------------------------------------------------
// C428 — a sparkline handle must survive a later AddSparklineGroup
// ---------------------------------------------------------------------------

// TestSparklineHandleSurvivesAppend guards C428: handles held a
// *CT_SparklineGroup pointing into the shared Groups slice, so the append in
// AddSparklineGroup reallocated the backing array and every previously returned
// handle was left pointing into dead memory — setters mutated it, flushSparklines
// re-marshaled from the live slice, and the change vanished with no error.
func TestSparklineHandleSurvivesAppend(t *testing.T) {
	w := Create()
	s := w.AddSheet("S")

	first, err := s.AddSparklineGroup(SparklineOptions{
		Data: []SparklineData{{DataRange: "S!A1:D1", LocationCell: "E1"}},
	})
	if err != nil {
		t.Fatalf("AddSparklineGroup: %v", err)
	}

	// Grow the slice well past its initial capacity so a stale pointer is
	// certainly detached.
	for i := 2; i <= 8; i++ {
		if _, err := s.AddSparklineGroup(SparklineOptions{
			Data: []SparklineData{{DataRange: "S!A1:D1", LocationCell: FormatCellRef(i, 5)}},
		}); err != nil {
			t.Fatalf("AddSparklineGroup %d: %v", i, err)
		}
	}

	// A setter on the handle returned before the appends must still land.
	first.SetSeriesColor("376092")
	first.SetHigh(true)

	groups := firstSheet(t, reopen(t, w)).Sparklines()
	if len(groups) != 8 {
		t.Fatalf("reopened groups = %d, want 8", len(groups))
	}
	var found *SparklineGroup
	for _, g := range groups {
		if sp := g.Sparklines(); len(sp) == 1 && sp[0].LocationCell == "E1" {
			found = g
		}
	}
	if found == nil {
		t.Fatal("the E1 group is missing after reopen")
	}
	if !strings.EqualFold(found.SeriesColor(), "FF376092") {
		t.Errorf("series color set through the pre-append handle = %q, want FF376092 (the write was discarded)", found.SeriesColor())
	}
	if m := found.resolve(); m == nil || m.High == nil || !*m.High {
		t.Error("high flag set through the pre-append handle was discarded")
	}
}

// TestSparklineHandleDeleteThenSetIsNoOp pins that a handle on a deleted group
// resolves to nothing rather than writing into another group's memory.
func TestSparklineHandleDeleteThenSetIsNoOp(t *testing.T) {
	w := Create()
	s := w.AddSheet("S")
	a, err := s.AddSparklineGroup(SparklineOptions{
		Data: []SparklineData{{DataRange: "S!A1:D1", LocationCell: "E1"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.AddSparklineGroup(SparklineOptions{
		Data: []SparklineData{{DataRange: "S!A2:D2", LocationCell: "E2"}},
	})
	if err != nil {
		t.Fatal(err)
	}

	a.Delete()
	a.SetSeriesColor("FF0000") // must not touch b

	if b.resolve() == nil {
		t.Fatal("the surviving handle stopped resolving after another group was deleted")
	}
	if got := b.SeriesColor(); got != "" {
		t.Errorf("write through a deleted handle landed on the surviving group: color = %q", got)
	}
	if sp := b.Sparklines(); len(sp) != 1 || sp[0].LocationCell != "E2" {
		t.Errorf("surviving handle resolves to the wrong group: %+v", sp)
	}
}

// ---------------------------------------------------------------------------
// C432 — adding one group must not strip unmodeled content from the others
// ---------------------------------------------------------------------------

// sparklineExtXLSX builds a workbook whose sheet carries a sparkline extension
// with one group that has an xr2:uid attribute and an unmodeled child, neither
// of which CT_SparklineGroup models.
func sparklineExtXLSX(t *testing.T) []byte {
	t.Helper()
	src, err := os.ReadFile("testdata/minimal.xlsx")
	if err != nil {
		t.Fatalf("read minimal.xlsx: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(src), int64(len(src)))
	if err != nil {
		t.Fatalf("zip open: %v", err)
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open %s: %v", f.Name, err)
		}
		content := new(bytes.Buffer)
		if _, err := content.ReadFrom(rc); err != nil {
			t.Fatalf("read %s: %v", f.Name, err)
		}
		_ = rc.Close()
		s := content.String()
		if f.Name == "xl/worksheets/sheet1.xml" {
			ext := `<extLst><ext uri="{05C60535-1F16-4fd2-B633-F4F36F0B64E0}" xmlns:x14="http://schemas.microsoft.com/office/spreadsheetml/2009/9/main">` +
				`<x14:sparklineGroups xmlns:xm="http://schemas.microsoft.com/office/excel/2006/main">` +
				`<x14:sparklineGroup type="column" xr2:uid="{DEADBEEF-0000-4000-8000-000000000001}" displayEmptyCellsAs="gap" xmlns:xr2="http://schemas.microsoft.com/office/spreadsheetml/2015/revision2">` +
				`<x14:sparklines><x14:sparkline><xm:f>Sheet1!A1:D1</xm:f><xm:sqref>E1</xm:sqref></x14:sparkline></x14:sparklines>` +
				`</x14:sparklineGroup></x14:sparklineGroups></ext></extLst>`
			s = strings.Replace(s, "</worksheet>", ext+"</worksheet>", 1)
		}
		w, err := zw.Create(f.Name)
		if err != nil {
			t.Fatalf("create %s: %v", f.Name, err)
		}
		if _, err := w.Write([]byte(s)); err != nil {
			t.Fatalf("write %s: %v", f.Name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return buf.Bytes()
}

// TestAddSparklineGroupPreservesExistingGroups guards C432: flushSparklines
// overwrote ext.RawContent with a full re-marshal of a model that has no
// attribute or child passthrough, so adding ONE group stripped xr2:uid (and
// every other unmodeled attribute) from every pre-existing group.
func TestAddSparklineGroupPreservesExistingGroups(t *testing.T) {
	src := sparklineExtXLSX(t)
	wb, err := OpenReader(bytes.NewReader(src), int64(len(src)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	sh, err := wb.Sheet(0)
	if err != nil {
		t.Fatalf("Sheet(0): %v", err)
	}
	if _, err := sh.AddSparklineGroup(SparklineOptions{
		Data: []SparklineData{{DataRange: "Sheet1!A2:D2", LocationCell: "E2"}},
	}); err != nil {
		t.Fatalf("AddSparklineGroup: %v", err)
	}
	out, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	sheet := string(readZipPart(t, out, "xl/worksheets/sheet1.xml"))

	if !strings.Contains(sheet, `xr2:uid="{DEADBEEF-0000-4000-8000-000000000001}"`) {
		t.Errorf("adding a sparkline group stripped the pre-existing group's xr2:uid:\n%s", sheet)
	}
	if !strings.Contains(sheet, `displayEmptyCellsAs="gap"`) {
		t.Errorf("adding a sparkline group stripped the pre-existing group's displayEmptyCellsAs:\n%s", sheet)
	}
	// The new group is present too.
	if !strings.Contains(sheet, "<xm:sqref>E2</xm:sqref>") {
		t.Errorf("the newly added sparkline group is missing:\n%s", sheet)
	}
}

// TestEditedSparklineGroupRegenerates pins the other half of C432: a group the
// caller actually edited must be regenerated (so the edit lands), not replayed
// from its captured source.
func TestEditedSparklineGroupRegenerates(t *testing.T) {
	src := sparklineExtXLSX(t)
	wb, err := OpenReader(bytes.NewReader(src), int64(len(src)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	sh, err := wb.Sheet(0)
	if err != nil {
		t.Fatalf("Sheet(0): %v", err)
	}
	groups := sh.Sparklines()
	if len(groups) != 1 {
		t.Fatalf("Sparklines() = %d, want 1", len(groups))
	}
	groups[0].SetSeriesColor("FF0000")

	out, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	sheet := string(readZipPart(t, out, "xl/worksheets/sheet1.xml"))
	if !strings.Contains(sheet, "colorSeries") {
		t.Errorf("the edit to a parsed sparkline group was not written:\n%s", sheet)
	}
}

// ---------------------------------------------------------------------------
// C534 — cell-reference-lookalike sheet names must be quoted
// ---------------------------------------------------------------------------

// TestPrintAreaQuotesReferenceLookalikeSheetNames guards C534: quoteSheetName
// only quoted non-identifier names, so a sheet legally named "Q1" produced
// Q1!$A$1:$D$20 in _xlnm.Print_Area, which Excel reads as a reference to cell
// Q1, not as a sheet.
func TestPrintAreaQuotesReferenceLookalikeSheetNames(t *testing.T) {
	cases := []struct {
		sheet string
		want  string
	}{
		{"Q1", "'Q1'!$A$1:$D$20"},
		{"XFD1", "'XFD1'!$A$1:$D$20"},
		{"R1C1", "'R1C1'!$A$1:$D$20"},
		{"R", "'R'!$A$1:$D$20"},
		{"TRUE", "'TRUE'!$A$1:$D$20"},
		// FY2024 is column FY, row 2024 — a real cell inside the grid, so Excel
		// quotes it too.
		{"FY2024", "'FY2024'!$A$1:$D$20"},
		// Not references, so these must stay bare: XFE is past the last column
		// (XFD), DATA2024 is past it by a wide margin, and a bare "Q" has no
		// row number.
		{"XFE1", "XFE1!$A$1:$D$20"},
		{"Data2024", "Data2024!$A$1:$D$20"},
		{"Q", "Q!$A$1:$D$20"},
		{"Sales", "Sales!$A$1:$D$20"},
		// Already quoted by the old rule; must stay quoted.
		{"My Sheet", "'My Sheet'!$A$1:$D$20"},
	}
	for _, tc := range cases {
		t.Run(tc.sheet, func(t *testing.T) {
			w := Create()
			s := w.AddSheet(tc.sheet)
			if err := s.SetPrintArea("A1:D20"); err != nil {
				t.Fatalf("SetPrintArea: %v", err)
			}
			var got string
			for _, dn := range w.DefinedNames() {
				if dn.Name == printAreaName {
					got = dn.Value
				}
			}
			if got != tc.want {
				t.Errorf("print area = %q, want %q", got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// C535 — table and pivot names must meet Excel's rules
// ---------------------------------------------------------------------------

// TestAddTableRejectsInvalidNames guards C535: validateTableName checked only
// empty/whitespace/A1-lookalike/first-character, so names Excel refuses on open
// went straight into tableN.xml.
func TestAddTableRejectsInvalidNames(t *testing.T) {
	bad := []struct{ name, why string }{
		{"Sales)Q1", "illegal interior character"},
		{"Sales-Q1", "illegal interior character"},
		{"Sales!Q1", "illegal interior character"},
		{strings.Repeat("A", 256), "longer than 255 characters"},
		{"R1C1", "R1C1-style reference"},
		{"R", "bare R"},
		{"C", "bare C"},
		{"A1", "A1-style reference"},
		{"1Table", "must not start with a digit"},
		{"", "empty (explicit empty means generated, so this is covered elsewhere)"},
	}
	for _, tc := range bad {
		if tc.name == "" {
			continue // an empty Name means "generate one"
		}
		t.Run(tc.why+"/"+truncateForName(tc.name), func(t *testing.T) {
			w := Create()
			s := w.AddSheet("S")
			seedTableRange(t, s)
			if _, err := s.AddTable("A1:C4", TableOptions{Name: tc.name}); err == nil {
				t.Errorf("AddTable(%q) succeeded; Excel rejects it (%s)", tc.name, tc.why)
			}
		})
	}
}

// TestAddTableAcceptsValidNames keeps the tightened validator from rejecting
// names Excel allows.
func TestAddTableAcceptsValidNames(t *testing.T) {
	for _, name := range []string{"Sales", "Sales_Q1", "Sales.Q1", "_Private", "\\Escaped", "Tabelle1", "Ünïcøde"} {
		t.Run(name, func(t *testing.T) {
			w := Create()
			s := w.AddSheet("S")
			seedTableRange(t, s)
			if _, err := s.AddTable("A1:C4", TableOptions{Name: name}); err != nil {
				t.Errorf("AddTable(%q) = %v, want success", name, err)
			}
		})
	}
}

// TestAddTableRejectsDefinedNameCollision guards the other half of C535: tables
// and defined names share one namespace, but tableNameExists checked only other
// tables.
func TestAddTableRejectsDefinedNameCollision(t *testing.T) {
	w := Create()
	s := w.AddSheet("S")
	seedTableRange(t, s)
	if err := w.AddDefinedName("Sales", "S!$A$1"); err != nil {
		t.Fatalf("AddDefinedName: %v", err)
	}
	if _, err := s.AddTable("A1:C4", TableOptions{Name: "Sales"}); err == nil {
		t.Error("AddTable succeeded with a name already taken by a defined name; Excel refuses to open such a workbook")
	}
}

// TestAddPivotTableValidatesName guards the AddPivotTable half of C535, which
// checked uniqueness but not syntax at all.
func TestAddPivotTableValidatesName(t *testing.T) {
	w := Create()
	s := w.AddSheet("S")
	seedPivotSource(t, s)
	for _, name := range []string{"My Pivot", "A1", "R1C1", "Pivot)1"} {
		t.Run(name, func(t *testing.T) {
			_, err := s.AddPivotTable("A1:C4", "F1", PivotOptions{
				Name:        name,
				RowFields:   []string{"Region"},
				ValueFields: []PivotValueField{{Field: "Amount"}},
			})
			if err == nil {
				t.Errorf("AddPivotTable(%q) succeeded; Excel rejects the name", name)
			}
		})
	}
}

func truncateForName(s string) string {
	if len(s) > 12 {
		return s[:12]
	}
	return s
}

func seedTableRange(t *testing.T, s *Sheet) {
	t.Helper()
	for i, h := range []string{"A", "B", "C"} {
		if err := s.SetCellValue(FormatCellRef(1, i+1), h); err != nil {
			t.Fatal(err)
		}
	}
	for r := 2; r <= 4; r++ {
		for c := 1; c <= 3; c++ {
			if err := s.SetCellValue(FormatCellRef(r, c), float64(r*c)); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func seedPivotSource(t *testing.T, s *Sheet) {
	t.Helper()
	if err := s.SetCellValue("A1", "Region"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetCellValue("B1", "Amount"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetCellValue("C1", "Extra"); err != nil {
		t.Fatal(err)
	}
	for r := 2; r <= 4; r++ {
		if err := s.SetCellValue(FormatCellRef(r, 1), "R"); err != nil {
			t.Fatal(err)
		}
		if err := s.SetCellValue(FormatCellRef(r, 2), float64(r)); err != nil {
			t.Fatal(err)
		}
		if err := s.SetCellValue(FormatCellRef(r, 3), "x"); err != nil {
			t.Fatal(err)
		}
	}
}

// ---------------------------------------------------------------------------
// C536 — SortState reads the nested state, so Remove must remove it
// ---------------------------------------------------------------------------

// nestedSortStateXLSX builds a workbook whose sheet carries its sort state
// inside <autoFilter>, which is what Excel writes for a sorted auto-filter.
func nestedSortStateXLSX(t *testing.T) []byte {
	t.Helper()
	src, err := os.ReadFile("testdata/minimal.xlsx")
	if err != nil {
		t.Fatalf("read minimal.xlsx: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(src), int64(len(src)))
	if err != nil {
		t.Fatalf("zip open: %v", err)
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open %s: %v", f.Name, err)
		}
		content := new(bytes.Buffer)
		if _, err := content.ReadFrom(rc); err != nil {
			t.Fatalf("read %s: %v", f.Name, err)
		}
		_ = rc.Close()
		s := content.String()
		if f.Name == "xl/worksheets/sheet1.xml" {
			af := `<autoFilter ref="A1:C10">` +
				`<sortState ref="A2:C10"><sortCondition ref="B2:B10"/></sortState>` +
				`</autoFilter>`
			s = strings.Replace(s, "</worksheet>", af+"</worksheet>", 1)
		}
		w, err := zw.Create(f.Name)
		if err != nil {
			t.Fatalf("create %s: %v", f.Name, err)
		}
		if _, err := w.Write([]byte(s)); err != nil {
			t.Fatalf("write %s: %v", f.Name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return buf.Bytes()
}

// TestRemoveSortStateRemovesNestedState guards C536: SortState() falls back to
// the autoFilter-nested sortState, but RemoveSortState only nilled the
// worksheet-level element, so Remove left SortState() still returning ok.
func TestRemoveSortStateRemovesNestedState(t *testing.T) {
	src := nestedSortStateXLSX(t)
	wb, err := OpenReader(bytes.NewReader(src), int64(len(src)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	sh, err := wb.Sheet(0)
	if err != nil {
		t.Fatalf("Sheet(0): %v", err)
	}
	if _, ok := sh.SortState(); !ok {
		t.Fatal("fixture has no readable sort state")
	}
	sh.RemoveSortState()
	if ss, ok := sh.SortState(); ok {
		t.Errorf("SortState() still reports ok=%v (%+v) after RemoveSortState", ok, ss)
	}
	// The auto-filter range itself survives.
	if sh.ws().AutoFilter == nil {
		t.Error("RemoveSortState removed the auto-filter as well")
	}
}

// TestSetSortStatePreservesUnmodeledChildren guards the finding #225 handed
// over: SetSortState rebuilt CT_SortState from the narrow public model, dropping
// the extLst (x14 sort-by-color conditions) that #225 now preserves.
func TestSetSortStatePreservesUnmodeledChildren(t *testing.T) {
	src, err := os.ReadFile("testdata/minimal.xlsx")
	if err != nil {
		t.Fatalf("read minimal.xlsx: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(src), int64(len(src)))
	if err != nil {
		t.Fatalf("zip open: %v", err)
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open %s: %v", f.Name, err)
		}
		content := new(bytes.Buffer)
		if _, err := content.ReadFrom(rc); err != nil {
			t.Fatalf("read %s: %v", f.Name, err)
		}
		_ = rc.Close()
		s := content.String()
		if f.Name == "xl/worksheets/sheet1.xml" {
			ss := `<sortState ref="A2:C10"><sortCondition ref="B2:B10"/>` +
				`<extLst><ext uri="{SORT-EXT}"><x14:sortCondition xmlns:x14="http://schemas.microsoft.com/office/spreadsheetml/2009/9/main" ref="C2:C10"/></ext></extLst>` +
				`</sortState>`
			s = strings.Replace(s, "</worksheet>", ss+"</worksheet>", 1)
		}
		w, err := zw.Create(f.Name)
		if err != nil {
			t.Fatalf("create %s: %v", f.Name, err)
		}
		if _, err := w.Write([]byte(s)); err != nil {
			t.Fatalf("write %s: %v", f.Name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}

	pkg := buf.Bytes()
	wb, err := OpenReader(bytes.NewReader(pkg), int64(len(pkg)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	sh, err := wb.Sheet(0)
	if err != nil {
		t.Fatalf("Sheet(0): %v", err)
	}
	if err := sh.SetSortState(SortState{Ref: "A2:C20", Conditions: []SortCondition{{Ref: "B2:B20"}}}); err != nil {
		t.Fatalf("SetSortState: %v", err)
	}
	out, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	sheet := string(readZipPart(t, out, "xl/worksheets/sheet1.xml"))
	if !strings.Contains(sheet, "{SORT-EXT}") {
		t.Errorf("SetSortState dropped the sortState extLst that the parse preserves:\n%s", sheet)
	}
	if !strings.Contains(sheet, `ref="A2:C20"`) {
		t.Errorf("SetSortState did not apply the new ref:\n%s", sheet)
	}
}

// ---------------------------------------------------------------------------
// C538 — SetAutoFilter / AddDataValidation must validate their ranges
// ---------------------------------------------------------------------------

// TestSetAutoFilterValidatesRange guards half of C538: SetAutoFilter accepted
// any string, and an invalid ref on <autoFilter> triggers an Excel repair
// prompt with no warning from this package at all.
func TestSetAutoFilterValidatesRange(t *testing.T) {
	w := Create()
	s := w.AddSheet("S")
	for _, bad := range []string{"banana", "", "A1:", ":B2", "A0", "1:1:1"} {
		t.Run("bad/"+bad, func(t *testing.T) {
			if err := s.SetAutoFilter(bad); err == nil {
				t.Errorf("SetAutoFilter(%q) succeeded; the ref reaches <autoFilter> and Excel offers to repair", bad)
			}
		})
	}
	for _, good := range []string{"A1:F1", "a1:f10", "B2"} {
		t.Run("good/"+good, func(t *testing.T) {
			if err := s.SetAutoFilter(good); err != nil {
				t.Errorf("SetAutoFilter(%q) = %v, want success", good, err)
			}
		})
	}
}

// TestAddDataValidationValidatesRange guards the other half of C538: an invalid
// Range was caught only later, as a non-blocking save warning.
func TestAddDataValidationValidatesRange(t *testing.T) {
	w := Create()
	s := w.AddSheet("S")
	for _, bad := range []string{"banana", "", "   ", "A1:", "Z0"} {
		t.Run("bad/"+bad, func(t *testing.T) {
			err := s.AddDataValidation(DataValidation{Range: bad, Type: "list", Formula1: `"a,b"`})
			if err == nil {
				t.Errorf("AddDataValidation(Range=%q) succeeded, unlike every sibling API that parses its range", bad)
			}
		})
	}
	// A space-separated sqref is legal and must still be accepted.
	if err := s.AddDataValidation(DataValidation{Range: "A1:A5 C1:C5", Type: "list", Formula1: `"a,b"`}); err != nil {
		t.Errorf("AddDataValidation with a multi-range sqref = %v, want success", err)
	}
}

// ---------------------------------------------------------------------------
// C539 — addComment must not dereference a nil workbook
// ---------------------------------------------------------------------------

// TestAddCommentWithoutWorkbookDoesNotPanic guards C539: addComment
// dereferenced s.workbook (through ensurePerson) without the nil guard its
// sibling writers have.
func TestAddCommentWithoutWorkbookDoesNotPanic(t *testing.T) {
	s := &Sheet{name: "detached"}
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("AddComment on a workbook-less sheet panicked: %v", r)
		}
	}()
	if c := s.AddComment("A1", "me", "hi"); c != nil {
		t.Error("AddComment on a workbook-less sheet returned a comment")
	}
}

// ---------------------------------------------------------------------------
// C540 — a comment edit must not reset every note box's geometry
// ---------------------------------------------------------------------------

// notesVMLXLSX builds a workbook with two legacy comments whose VML note shapes
// carry hand-set geometry and fill colors.
func notesVMLXLSX(t *testing.T) []byte {
	t.Helper()
	const ctVML = "application/vnd.openxmlformats-officedocument.vmlDrawing"
	const ctComments = "application/vnd.openxmlformats-officedocument.spreadsheetml.comments+xml"

	src, err := os.ReadFile("testdata/minimal.xlsx")
	if err != nil {
		t.Fatalf("read minimal.xlsx: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(src), int64(len(src)))
	if err != nil {
		t.Fatalf("zip open: %v", err)
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	write := func(name, content string) {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open %s: %v", f.Name, err)
		}
		content := new(bytes.Buffer)
		if _, err := content.ReadFrom(rc); err != nil {
			t.Fatalf("read %s: %v", f.Name, err)
		}
		_ = rc.Close()
		s := content.String()
		switch f.Name {
		case "[Content_Types].xml":
			ov := `<Override PartName="/xl/drawings/vmlDrawing1.vml" ContentType="` + ctVML + `"/>` +
				`<Override PartName="/xl/comments1.xml" ContentType="` + ctComments + `"/>`
			s = strings.Replace(s, "</Types>", ov+"</Types>", 1)
		case "xl/worksheets/sheet1.xml":
			s = strings.Replace(s, "</worksheet>", `<legacyDrawing r:id="rIdVml"/></worksheet>`, 1)
		}
		write(f.Name, s)
	}
	write("xl/worksheets/_rels/sheet1.xml.rels",
		`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`+
			`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`+
			`<Relationship Id="rIdVml" Type="`+opc.RelTypeVMLDrawing+`" Target="../drawings/vmlDrawing1.vml"/>`+
			`<Relationship Id="rIdCmt" Type="`+opc.RelTypeComments+`" Target="../comments1.xml"/>`+
			`</Relationships>`)
	write("xl/comments1.xml",
		`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`+
			`<comments xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">`+
			`<authors><author>Ada</author></authors><commentList>`+
			`<comment ref="A1" authorId="0"><text><t>first</t></text></comment>`+
			`<comment ref="B2" authorId="0"><text><t>second</t></text></comment>`+
			`</commentList></comments>`)
	// Two note shapes, each with distinctive geometry and fill the library's
	// generator would never produce.
	write("xl/drawings/vmlDrawing1.vml",
		`<xml xmlns:v="urn:schemas-microsoft-com:vml" xmlns:o="urn:schemas-microsoft-com:office:office" xmlns:x="urn:schemas-microsoft-com:office:excel">`+
			`<o:shapelayout v:ext="edit"><o:idmap v:ext="edit" data="1"/></o:shapelayout>`+
			`<v:shapetype id="_x0000_t202" coordsize="21600,21600" o:spt="202" path="m0,0l0,21600,21600,21600,21600,0xe"><v:stroke joinstyle="miter"/><v:path gradientshapeok="t" o:connecttype="rect"/></v:shapetype>`+
			`<v:shape id="_x0000_s1025" type="#_x0000_t202" style='position:absolute;margin-left:400pt;margin-top:200pt;width:333pt;height:222pt;z-index:1;visibility:hidden' fillcolor="#c0ffee">`+
			`<x:ClientData ObjectType="Note"><x:MoveWithCells/><x:SizeWithCells/><x:AutoFill>False</x:AutoFill><x:Row>0</x:Row><x:Column>0</x:Column></x:ClientData></v:shape>`+
			`<v:shape id="_x0000_s1026" type="#_x0000_t202" style='position:absolute;margin-left:500pt;margin-top:300pt;width:444pt;height:55pt;z-index:2;visibility:hidden' fillcolor="#beefed">`+
			`<x:ClientData ObjectType="Note"><x:MoveWithCells/><x:SizeWithCells/><x:AutoFill>False</x:AutoFill><x:Row>1</x:Row><x:Column>1</x:Column></x:ClientData></v:shape>`+
			`</xml>`)
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return buf.Bytes()
}

// TestAddCommentPreservesExistingNoteGeometry guards C540: composeLegacyVML
// dropped every existing Note shape and re-rendered them from
// buildCommentVMLShapes' fixed 108x59.25pt / #ffffe1 template, so adding one
// comment reset the position, size and color of every note already on the sheet.
func TestAddCommentPreservesExistingNoteGeometry(t *testing.T) {
	src := notesVMLXLSX(t)
	wb, err := OpenReader(bytes.NewReader(src), int64(len(src)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	sh, err := wb.Sheet(0)
	if err != nil {
		t.Fatalf("Sheet(0): %v", err)
	}
	if c := sh.AddComment("D4", "Ada", "third"); c == nil {
		t.Fatal("AddComment returned nil")
	}
	out, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	vml := string(readZipPart(t, out, "xl/drawings/vmlDrawing1.vml"))

	for _, want := range []string{
		"margin-left:400pt", "width:333pt", "height:222pt", `fillcolor="#c0ffee"`,
		"margin-left:500pt", "width:444pt", `fillcolor="#beefed"`,
	} {
		if !strings.Contains(vml, want) {
			t.Errorf("adding a comment reset an existing note box: %q missing from\n%s", want, vml)
		}
	}
	// The new comment still gets a generated note box.
	if strings.Count(vml, `ObjectType="Note"`) != 3 {
		t.Errorf("want 3 note shapes (2 preserved + 1 new), got %d:\n%s",
			strings.Count(vml, `ObjectType="Note"`), vml)
	}
	// The new shape must not collide with the preserved shape ids.
	if strings.Count(vml, `id="_x0000_s1025"`) != 1 || strings.Count(vml, `id="_x0000_s1026"`) != 1 {
		t.Errorf("preserved note shape ids were duplicated or renumbered:\n%s", vml)
	}
}

// ---------------------------------------------------------------------------
// C541 — a totals row must actually appear in the sheet
// ---------------------------------------------------------------------------

// TestAddTableWritesTotalsRowCells guards C541: the totals functions were
// recorded in tableN.xml but no SUBTOTAL formula or label was written into the
// sheet's totals-row cells, so the row rendered blank until the user toggled
// the totals row off and on.
func TestAddTableWritesTotalsRowCells(t *testing.T) {
	w := Create()
	s := w.AddSheet("S")
	if err := s.SetCellValue("A1", "Region"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetCellValue("B1", "Amount"); err != nil {
		t.Fatal(err)
	}
	for r := 2; r <= 4; r++ {
		if err := s.SetCellValue(FormatCellRef(r, 1), "R"); err != nil {
			t.Fatal(err)
		}
		if err := s.SetCellValue(FormatCellRef(r, 2), float64(r)); err != nil {
			t.Fatal(err)
		}
	}
	// A1:B5 = header + 3 data rows + totals row.
	if _, err := s.AddTable("A1:B5", TableOptions{
		Name:      "Sales",
		TotalsRow: true,
		ColumnTotals: map[string]TotalsColumn{
			"Region": {Label: "Total"},
			"Amount": {Function: "sum"},
		},
	}); err != nil {
		t.Fatalf("AddTable: %v", err)
	}

	label, err := s.GetCellValue("A5")
	if err != nil {
		t.Fatal(err)
	}
	if label != "Total" {
		t.Errorf("totals-row label cell A5 = %q, want %q", label, "Total")
	}
	cell, err := s.Cell("B5")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := cell.Formula(), "SUBTOTAL(109,Sales[Amount])"; got != want {
		t.Errorf("totals-row formula B5 = %q, want %q", got, want)
	}
}

// TestAddTableTotalsRowNeedsDataRow guards the off-by-one in C541: a 2-row
// range with TotalsRow yielded a table with a header and a totals row and zero
// data rows, which Excel never creates.
func TestAddTableTotalsRowNeedsDataRow(t *testing.T) {
	w := Create()
	s := w.AddSheet("S")
	seedTableRange(t, s)
	if _, err := s.AddTable("A1:C2", TableOptions{TotalsRow: true}); err == nil {
		t.Error("AddTable accepted a 2-row range with a totals row, leaving the table with no data rows")
	}
	// Three rows (header + data + totals) is the minimum and must be accepted.
	if _, err := s.AddTable("A1:C3", TableOptions{Name: "OK", TotalsRow: true}); err != nil {
		t.Errorf("AddTable with header+data+totals = %v, want success", err)
	}
}

// ---------------------------------------------------------------------------
// C542 — iconSet thresholds
// ---------------------------------------------------------------------------

// TestIconSetThresholdCount guards C542: explicit thresholds were emitted
// regardless of how many the icon set takes, and Excel prompts to repair a file
// whose cfvo count does not match the icon count.
func TestIconSetThresholdCount(t *testing.T) {
	w := Create()
	s := w.AddSheet("S")

	// Two thresholds for a 3-icon set.
	rule := NewIconSetRule("3TrafficLights1",
		ConditionalValueObject{Type: "percent", Value: "0"},
		ConditionalValueObject{Type: "percent", Value: "50"},
	)
	if err := s.AddConditionalFormat("A1:A10", rule); err == nil {
		t.Error("iconSet accepted 2 thresholds for a 3-icon set; Excel offers to repair the file")
	}

	// Five for a 4-icon set.
	rule = NewIconSetRule("4Arrows",
		ConditionalValueObject{Type: "percent", Value: "0"},
		ConditionalValueObject{Type: "percent", Value: "20"},
		ConditionalValueObject{Type: "percent", Value: "40"},
		ConditionalValueObject{Type: "percent", Value: "60"},
		ConditionalValueObject{Type: "percent", Value: "80"},
	)
	if err := s.AddConditionalFormat("B1:B10", rule); err == nil {
		t.Error("iconSet accepted 5 thresholds for a 4-icon set")
	}

	// The right count still works.
	rule = NewIconSetRule("3TrafficLights1",
		ConditionalValueObject{Type: "percent", Value: "0"},
		ConditionalValueObject{Type: "percent", Value: "40"},
		ConditionalValueObject{Type: "percent", Value: "80"},
	)
	if err := s.AddConditionalFormat("C1:C10", rule); err != nil {
		t.Errorf("iconSet with the matching threshold count = %v, want success", err)
	}
}

// TestIconSetDefaultThresholdsMatchExcel guards the rounding half of C542.
func TestIconSetDefaultThresholdsMatchExcel(t *testing.T) {
	cases := []struct {
		set  string
		want []string
	}{
		{"3TrafficLights1", []string{"0", "33", "67"}},
		{"4Arrows", []string{"0", "25", "50", "75"}},
		{"5Rating", []string{"0", "20", "40", "60", "80"}},
	}
	for _, tc := range cases {
		t.Run(tc.set, func(t *testing.T) {
			w := Create()
			s := w.AddSheet("S")
			if err := s.AddConditionalFormat("A1:A10", NewIconSetRule(tc.set)); err != nil {
				t.Fatalf("AddConditionalFormat: %v", err)
			}
			_, cf := firstCF(t, w)
			is := cf.Rules[0].IconSet
			if is == nil || len(is.Values) != len(tc.want) {
				t.Fatalf("iconSet = %+v", is)
			}
			for i, want := range tc.want {
				if is.Values[i].Value != want {
					t.Errorf("threshold %d = %q, want %q (Excel's own rounding)", i, is.Values[i].Value, want)
				}
			}
		})
	}
}

// TestCfvoTypeValidation guards the last part of C542: NewDataBarRule accepted
// any cfvo Type string unvalidated.
func TestCfvoTypeValidation(t *testing.T) {
	w := Create()
	s := w.AddSheet("S")

	bad := NewDataBarRule("638EC6",
		ConditionalValueObject{Type: "bogus", Value: "0"},
		ConditionalValueObject{Type: "max"},
	)
	if err := s.AddConditionalFormat("A1:A10", bad); err == nil {
		t.Error("dataBar accepted an unknown cfvo type; it reaches the part and Excel offers to repair")
	}

	bad = NewIconSetRule("3TrafficLights1",
		ConditionalValueObject{Type: "percent", Value: "0"},
		ConditionalValueObject{Type: "nope", Value: "33"},
		ConditionalValueObject{Type: "percent", Value: "67"},
	)
	if err := s.AddConditionalFormat("B1:B10", bad); err == nil {
		t.Error("iconSet accepted an unknown cfvo type")
	}

	// Every legal type is accepted.
	for _, typ := range []string{"num", "percent", "max", "min", "formula", "percentile"} {
		ok := NewDataBarRule("638EC6",
			ConditionalValueObject{Type: typ, Value: "1"},
			ConditionalValueObject{Type: "max"},
		)
		if err := s.AddConditionalFormat("D1:D10", ok); err != nil {
			t.Errorf("dataBar with cfvo type %q = %v, want success", typ, err)
		}
	}
}

// ---------------------------------------------------------------------------
// C543 — a pivot may not be laid out over its own source
// ---------------------------------------------------------------------------

// TestAddPivotTableRejectsSourceOverlap guards C543: with refreshOnLoad set,
// Excel's rebuild writes the pivot over the very cells its cache reads.
func TestAddPivotTableRejectsSourceOverlap(t *testing.T) {
	w := Create()
	s := w.AddSheet("S")
	seedPivotSource(t, s)

	// Anchoring at B2 puts the layout squarely inside A1:C4.
	if _, err := s.AddPivotTable("A1:C4", "B2", PivotOptions{
		RowFields:   []string{"Region"},
		ValueFields: []PivotValueField{{Field: "Amount"}},
	}); err == nil {
		t.Error("AddPivotTable placed a pivot over its own source range; Excel's UI refuses this and a refresh destroys the data")
	}

	// Clear of the source is fine.
	if _, err := s.AddPivotTable("A1:C4", "F1", PivotOptions{
		RowFields:   []string{"Region"},
		ValueFields: []PivotValueField{{Field: "Amount"}},
	}); err != nil {
		t.Errorf("AddPivotTable clear of its source = %v, want success", err)
	}
}

// TestAddPivotTableAllowsOverlapOnOtherSheet pins that the overlap check is
// scoped to the source sheet: the same box on a different sheet is legal.
func TestAddPivotTableAllowsOverlapOnOtherSheet(t *testing.T) {
	w := Create()
	src := w.AddSheet("Src")
	seedPivotSource(t, src)
	dst := w.AddSheet("Dst")

	if _, err := dst.AddPivotTable("Src!A1:C4", "B2", PivotOptions{
		RowFields:   []string{"Region"},
		ValueFields: []PivotValueField{{Field: "Amount"}},
	}); err != nil {
		t.Errorf("AddPivotTable on another sheet = %v, want success", err)
	}
}

// ---------------------------------------------------------------------------
// C554c — ensureDrawingInChildOrder must rank unknown:N entries
// ---------------------------------------------------------------------------

// TestEnsureDrawingInChildOrderRanksUnknownEntries guards C554c: the hand-rolled
// duplicate of CT_Worksheet.EnsureChildOrder recognized only three of the
// elements that follow <drawing> and could not rank the "unknown:N" entries that
// stand for unmodeled children, so on a worksheet whose only post-drawing child
// was an unknown element the reference was appended after it, out of schema
// order. Excel is strict about worksheet child order.
func TestEnsureDrawingInChildOrderRanksUnknownEntries(t *testing.T) {
	// picture follows drawing in the CT_Worksheet sequence and has no typed
	// model, so it is captured as an unknown child.
	src, err := os.ReadFile("testdata/minimal.xlsx")
	if err != nil {
		t.Fatalf("read minimal.xlsx: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(src), int64(len(src)))
	if err != nil {
		t.Fatalf("zip open: %v", err)
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open %s: %v", f.Name, err)
		}
		content := new(bytes.Buffer)
		if _, err := content.ReadFrom(rc); err != nil {
			t.Fatalf("read %s: %v", f.Name, err)
		}
		_ = rc.Close()
		s := content.String()
		if f.Name == "xl/worksheets/sheet1.xml" {
			s = strings.Replace(s, "</worksheet>", `<picture r:id="rIdPic"/></worksheet>`, 1)
		}
		w, err := zw.Create(f.Name)
		if err != nil {
			t.Fatalf("create %s: %v", f.Name, err)
		}
		if _, err := w.Write([]byte(s)); err != nil {
			t.Fatalf("write %s: %v", f.Name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}

	pkg := buf.Bytes()
	wb, err := OpenReader(bytes.NewReader(pkg), int64(len(pkg)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	sh, err := wb.Sheet(0)
	if err != nil {
		t.Fatalf("Sheet(0): %v", err)
	}
	if err := sh.AddImage("A1", testPNG(t, 4, 4), ImageOptions{}); err != nil {
		t.Fatalf("AddImage: %v", err)
	}
	out, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	sheet := string(readZipPart(t, out, "xl/worksheets/sheet1.xml"))

	di := strings.Index(sheet, "<drawing")
	pi := strings.Index(sheet, "<picture")
	if di < 0 {
		t.Fatalf("no <drawing> emitted:\n%s", sheet)
	}
	if pi < 0 {
		t.Fatalf("the unmodeled <picture> child was dropped:\n%s", sheet)
	}
	if di > pi {
		t.Errorf("<drawing> emitted after <picture>, violating the CT_Worksheet sequence:\n%s", sheet)
	}
}

// ---------------------------------------------------------------------------
// Byte identity: a zero-modification open->save must not be perturbed by the
// per-group sparkline capture (C432) or the note-shape preservation (C540).
// ---------------------------------------------------------------------------

// TestUntouchedFeaturePartsStayByteIdentical pins that the fixtures exercising
// C432 and C540 round-trip verbatim when nothing is changed. Both fixes are
// about preserving content, so a regression here would mean the fix itself
// started rewriting untouched files.
func TestUntouchedFeaturePartsStayByteIdentical(t *testing.T) {
	cases := []struct {
		name  string
		build func(*testing.T) []byte
		parts []string
	}{
		{"sparklines", sparklineExtXLSX, []string{"xl/worksheets/sheet1.xml"}},
		{"legacy notes", notesVMLXLSX, []string{
			"xl/worksheets/sheet1.xml", "xl/drawings/vmlDrawing1.vml", "xl/comments1.xml",
		}},
		{"nested sortState", nestedSortStateXLSX, []string{"xl/worksheets/sheet1.xml"}},
		{"sheet-private parts", buildXLSXWithSheetPrivateParts, []string{
			"xl/worksheets/sheet1.xml", "xl/embeddings/oleObject1.bin",
			"xl/ctrlProps/ctrlProp1.xml", "xl/slicers/slicer1.xml",
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := tc.build(t)
			wb, err := OpenReader(bytes.NewReader(src), int64(len(src)))
			if err != nil {
				t.Fatalf("OpenReader: %v", err)
			}
			out, err := wb.SaveBytes()
			if err != nil {
				t.Fatalf("SaveBytes: %v", err)
			}
			for _, part := range tc.parts {
				want := readZipPart(t, src, part)
				got := readZipPart(t, out, part)
				if !bytes.Equal(want, got) {
					t.Errorf("%s changed on a zero-modification save:\n want: %s\n  got: %s", part, want, got)
				}
			}
		})
	}
}

// TestRepeatedSaveConverges pins that a second SaveBytes reproduces the first
// for the two preservation fixes. Both compose their output against the opened
// package's original bytes, and writeSheetComments drops the preserved legacy
// VML once it has been superseded — so a second save found nothing to compose
// against, regenerated the drawing from scratch and discarded the form-control
// shapes and hand-positioned note boxes the first save had just preserved.
// (Pre-existing: the base fails this too, less visibly, because it preserved
// less to begin with.)
func TestRepeatedSaveConverges(t *testing.T) {
	t.Run("sparklines", func(t *testing.T) {
		src := sparklineExtXLSX(t)
		wb, err := OpenReader(bytes.NewReader(src), int64(len(src)))
		if err != nil {
			t.Fatalf("OpenReader: %v", err)
		}
		sh, err := wb.Sheet(0)
		if err != nil {
			t.Fatalf("Sheet(0): %v", err)
		}
		if _, err := sh.AddSparklineGroup(SparklineOptions{
			Data: []SparklineData{{DataRange: "Sheet1!A2:D2", LocationCell: "E2"}},
		}); err != nil {
			t.Fatalf("AddSparklineGroup: %v", err)
		}
		assertSaveConverges(t, wb, "xl/worksheets/sheet1.xml")
	})
	t.Run("legacy notes", func(t *testing.T) {
		src := notesVMLXLSX(t)
		wb, err := OpenReader(bytes.NewReader(src), int64(len(src)))
		if err != nil {
			t.Fatalf("OpenReader: %v", err)
		}
		sh, err := wb.Sheet(0)
		if err != nil {
			t.Fatalf("Sheet(0): %v", err)
		}
		if c := sh.AddComment("D4", "Ada", "third"); c == nil {
			t.Fatal("AddComment returned nil")
		}
		assertSaveConverges(t, wb, "xl/drawings/vmlDrawing1.vml", "xl/comments1.xml")
	})
}

// assertSaveConverges saves twice and requires the named parts to be identical.
func assertSaveConverges(t *testing.T, wb *Workbook, parts ...string) {
	t.Helper()
	first, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("first SaveBytes: %v", err)
	}
	second, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("second SaveBytes: %v", err)
	}
	for _, part := range parts {
		a := readZipPart(t, first, part)
		b := readZipPart(t, second, part)
		if !bytes.Equal(a, b) {
			t.Errorf("%s differs between the first and second save:\n first (%d bytes): %s\nsecond (%d bytes): %s",
				part, len(a), a, len(b), b)
		}
	}
}
