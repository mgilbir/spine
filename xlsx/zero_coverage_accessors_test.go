package xlsx

import (
	"bytes"
	"testing"

	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// The accessors below all read one field off a parsed model, which is exactly
// the shape where a copy-paste picks the neighbouring field and nothing
// notices. Every fixture here is therefore built so the neighbouring field
// holds a DIFFERENT value: a non-square image (so HeightEMU returning the width
// fails), a table whose name and displayName differ, a slicer and a timeline
// whose cache names differ from each other and from their own names, and a
// pivot table whose location differs from its source range.

// A non-square image: Width/Height and their EMU forms must each report their
// own axis, and the point conversion must be the documented 96-DPI one. A
// square fixture would pass with the two axes swapped.
func TestImageDimensionAccessorsReportTheirOwnAxis(t *testing.T) {
	const widthPx, heightPx = 200, 50
	const wantWidthEMU = int64(widthPx) * emuPerPixel  // 1905000
	const wantHeightEMU = int64(heightPx) * emuPerPixel // 476250

	w := Create()
	defer func() { _ = w.Close() }()
	s := addSheetT(w, "Pics")
	if err := s.AddImage("B2", testPNG(t, 20, 10), ImageOptions{
		WidthPx:  widthPx,
		HeightPx: heightPx,
		AltText:  "a wide rectangle",
	}); err != nil {
		t.Fatalf("AddImage: %v", err)
	}

	check := func(t *testing.T, stage string, img *Image) {
		t.Helper()
		if got := img.WidthEMU(); got != wantWidthEMU {
			t.Errorf("%s: WidthEMU() = %d, want %d", stage, got, wantWidthEMU)
		}
		if got := img.HeightEMU(); got != wantHeightEMU {
			t.Errorf("%s: HeightEMU() = %d, want %d", stage, got, wantHeightEMU)
		}
		if img.WidthEMU() == img.HeightEMU() {
			t.Errorf("%s: WidthEMU and HeightEMU agree on a 200x50 image", stage)
		}
		// 200 px at 96 DPI = 150 pt; 50 px = 37.5 pt.
		if got := img.Width(); got != 150 {
			t.Errorf("%s: Width() = %v pt, want 150", stage, got)
		}
		if got := img.Height(); got != 37.5 {
			t.Errorf("%s: Height() = %v pt, want 37.5", stage, got)
		}
		if img.AnchorCell() != "B2" {
			t.Errorf("%s: AnchorCell() = %q, want B2", stage, img.AnchorCell())
		}
	}

	pending := s.Images()
	if len(pending) != 1 {
		t.Fatalf("Images() before save = %d, want 1", len(pending))
	}
	check(t, "before save", pending[0])
	// An image added this session has no package part yet; PartName reporting
	// something here would be a fabricated path.
	if got := pending[0].PartName(); got != "" {
		t.Errorf("PartName() before save = %q, want empty", got)
	}

	re, _ := saveReopenSheetBytes(t, w)
	opened := re.Images()
	if len(opened) != 1 {
		t.Fatalf("Images() after reopen = %d, want 1", len(opened))
	}
	check(t, "after reopen", opened[0])
	if got := opened[0].PartName(); got != "/xl/media/image1.png" {
		t.Errorf("PartName() after reopen = %q, want /xl/media/image1.png", got)
	}
	if got := opened[0].ContentType(); got != "image/png" {
		t.Errorf("ContentType() = %q, want image/png", got)
	}
	if got := opened[0].AltText(); got != "a wide rectangle" {
		t.Errorf("AltText() = %q, want %q", got, "a wide rectangle")
	}
}

// emuToPoints is the shared conversion behind Width/Height; pin the exact
// factor (72 pt per inch over 914400 EMU per inch) rather than the round-trip
// of whatever it currently is.
func TestEmuToPoints(t *testing.T) {
	cases := []struct {
		emu  int64
		want float64
	}{
		{0, 0},
		{914400, 72},   // one inch
		{9525, 0.75},   // one pixel at 96 DPI
		{457200, 36},   // half an inch
		{-914400, -72}, // defensive: no clamping surprises
	}
	for _, tc := range cases {
		if got := emuToPoints(tc.emu); got != tc.want {
			t.Errorf("emuToPoints(%d) = %v, want %v", tc.emu, got, tc.want)
		}
	}
}

// tableNameFixtureSheet is a worksheet referencing a table part whose name and
// displayName deliberately differ, so Name and DisplayName cannot both pass by
// reading the same field.
func tableNameFixture(t *testing.T) []byte {
	t.Helper()
	rels := func(entries string) string {
		return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
			`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` + entries + `</Relationships>`
	}
	sheetXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
		`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">` +
		`<sheetData/><tableParts count="1"><tablePart xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" r:id="rId2"/></tableParts></worksheet>`
	return buildFixtureXlsxParts(t, []struct{ name, data string }{
		{"[Content_Types].xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
			`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
			`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
			`<Default Extension="xml" ContentType="application/xml"/>` +
			`<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>` +
			`<Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>` +
			`<Override PartName="/xl/tables/table1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.table+xml"/>` +
			`</Types>`},
		{"_rels/.rels", rels(`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>`)},
		{"xl/workbook.xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
			`<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Sheet1" sheetId="1" r:id="rId1"/></sheets></workbook>`},
		{"xl/_rels/workbook.xml.rels", rels(`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>`)},
		{"xl/worksheets/sheet1.xml", sheetXML},
		{"xl/worksheets/_rels/sheet1.xml.rels", rels(`<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/table" Target="../tables/table1.xml"/>`)},
		{"xl/tables/table1.xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
			`<table xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" id="1" name="internal_name" displayName="Sales_2026" ref="A1:C9"/>`},
	})
}

// Table.DisplayName reports displayName, not name. Excel keeps the two
// separate (name is the part-internal identifier, displayName is what formulas
// and the UI use), so a fixture where they match cannot tell them apart.
func TestTableDisplayNameIsDistinctFromName(t *testing.T) {
	data := tableNameFixture(t)
	w, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = w.Close() }()

	tables := w.Tables()
	if len(tables) != 1 {
		t.Fatalf("Tables() = %d, want 1", len(tables))
	}
	tbl := tables[0]
	if got := tbl.DisplayName(); got != "Sales_2026" {
		t.Errorf("DisplayName() = %q, want Sales_2026", got)
	}
	if got := tbl.Name(); got != "internal_name" {
		t.Errorf("Name() = %q, want internal_name", got)
	}
	if got := tbl.Range(); got != "A1:C9" {
		t.Errorf("Range() = %q, want A1:C9", got)
	}
}

// PivotTable.Location reports the range the table occupies on its sheet, which
// must not be confused with the source range it draws from — the fixture makes
// the two differ.
func TestPivotTableLocationIsNotTheSourceRange(t *testing.T) {
	w := Create()
	defer func() { _ = w.Close() }()
	s := addSheetT(w, "Data")
	rows := [][]interface{}{
		{"Region", "Units"},
		{"North", 3},
		{"South", 5},
		{"North", 2},
	}
	for r, row := range rows {
		for c, v := range row {
			ref, err := CellRef(r+1, c+1)
			if err != nil {
				t.Fatalf("CellRef: %v", err)
			}
			if err := s.SetCellValue(ref, v); err != nil {
				t.Fatalf("SetCellValue: %v", err)
			}
		}
	}
	pt, err := s.AddPivotTable("A1:B4", "E10", PivotOptions{
		RowFields:   []string{"Region"},
		ValueFields: []PivotValueField{{Field: "Units", Aggregation: PivotSum}},
	})
	if err != nil {
		t.Fatalf("AddPivotTable: %v", err)
	}

	loc := pt.Location()
	if loc == "" {
		t.Fatal("Location() is empty")
	}
	if loc == pt.SourceRange() {
		t.Errorf("Location() = SourceRange() = %q; the accessor is reading the source", loc)
	}
	// The pivot was anchored at E10, so its location must start there.
	if got := loc[:3]; got != "E10" {
		t.Errorf("Location() = %q, want a range anchored at E10", loc)
	}
	if got := pt.SourceRange(); got != "A1:B4" {
		t.Errorf("SourceRange() = %q, want A1:B4", got)
	}
}

// Slicer.Cache and Timeline.Cache report the cache each control draws from.
// The fixture's two caches have different names, and neither matches its
// control's own name, so an accessor returning Name or the other control's
// cache fails.
func TestSlicerAndTimelineCacheAccessors(t *testing.T) {
	pkg := slicerFixture(t)
	wb, err := OpenReader(bytes.NewReader(pkg), int64(len(pkg)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	defer func() { _ = wb.Close() }()

	slicers := wb.Slicers()
	if len(slicers) != 1 {
		t.Fatalf("Slicers() = %d, want 1", len(slicers))
	}
	timelines := wb.Timelines()
	if len(timelines) != 1 {
		t.Fatalf("Timelines() = %d, want 1", len(timelines))
	}
	sl, tl := slicers[0], timelines[0]

	if sl.Cache() == "" || tl.Cache() == "" {
		t.Fatalf("cache names are empty: slicer %q timeline %q", sl.Cache(), tl.Cache())
	}
	if sl.Cache() == tl.Cache() {
		t.Errorf("slicer and timeline report the same cache %q", sl.Cache())
	}
	if sl.Cache() == sl.Name() {
		t.Errorf("Slicer.Cache() = Slicer.Name() = %q; the accessor is reading the name", sl.Cache())
	}
	if tl.Cache() == tl.Name() {
		t.Errorf("Timeline.Cache() = Timeline.Name() = %q; the accessor is reading the name", tl.Cache())
	}
	// Timeline.PivotTables resolves through the timeline's own cache: getting
	// the wrong cache would surface the slicer's controlled tables here.
	pts := tl.PivotTables()
	if len(pts) != 1 || pts[0] != "PivotTable1" {
		t.Errorf("Timeline.PivotTables() = %v, want [PivotTable1]", pts)
	}
}

// The style deduplicator compares fills field by field. Gradient fills are the
// only fills whose comparison walks optional float pointers, and two gradients
// differing in exactly one of them must not collapse onto one style — that is a
// silently wrong document, not a crash.
func TestGradientFillEqualityDistinguishesEachOptionalField(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	base := func() *oxml.CT_GradientFill {
		return &oxml.CT_GradientFill{
			Type:   "linear",
			Degree: f(90), Left: f(0.1), Right: f(0.2), Top: f(0.3), Bottom: f(0.4),
			Stop: []oxml.CT_GradientStop{
				{Position: 0, Color: oxml.CT_Color{Rgb: "FFFF0000"}},
				{Position: 1, Color: oxml.CT_Color{Rgb: "FF0000FF"}},
			},
		}
	}
	if !gradientFillEqual(base(), base()) {
		t.Fatal("two identical gradient fills compare unequal")
	}
	if !gradientFillEqual(nil, nil) {
		t.Error("nil gradient fills compare unequal")
	}
	if gradientFillEqual(base(), nil) || gradientFillEqual(nil, base()) {
		t.Error("a gradient fill compares equal to no fill at all")
	}

	mutations := map[string]func(*oxml.CT_GradientFill){
		"Type":          func(g *oxml.CT_GradientFill) { g.Type = "path" },
		"Degree":        func(g *oxml.CT_GradientFill) { g.Degree = f(45) },
		"Left":          func(g *oxml.CT_GradientFill) { g.Left = f(0.9) },
		"Right":         func(g *oxml.CT_GradientFill) { g.Right = f(0.9) },
		"Top":           func(g *oxml.CT_GradientFill) { g.Top = f(0.9) },
		"Bottom":        func(g *oxml.CT_GradientFill) { g.Bottom = f(0.9) },
		"Degree unset":  func(g *oxml.CT_GradientFill) { g.Degree = nil },
		"stop count":    func(g *oxml.CT_GradientFill) { g.Stop = g.Stop[:1] },
		"stop position": func(g *oxml.CT_GradientFill) { g.Stop[1].Position = 0.5 },
		"stop color":    func(g *oxml.CT_GradientFill) { g.Stop[1].Color = oxml.CT_Color{Rgb: "FF00FF00"} },
	}
	for name, mutate := range mutations {
		other := base()
		mutate(other)
		if gradientFillEqual(base(), other) {
			t.Errorf("gradient fills differing in %s compare equal (styles would be deduplicated onto one)", name)
		}
	}
}

// cloneFill must deep-copy the optional gradient floats: a shallow copy that
// shares the pointers lets an edit to the imported style reach back into the
// source workbook's stylesheet.
func TestCloneFillDeepCopiesGradientPointers(t *testing.T) {
	f := func(v float64) *float64 { return &v }
	src := &oxml.CT_Fill{GradientFill: &oxml.CT_GradientFill{
		Type:   "linear",
		Degree: f(90), Left: f(0.1), Right: f(0.2), Top: f(0.3), Bottom: f(0.4),
		Stop: []oxml.CT_GradientStop{{Position: 0, Color: oxml.CT_Color{Rgb: "FFFF0000"}}},
	}}
	clone := cloneFill(src)
	if clone.GradientFill == nil {
		t.Fatal("cloneFill dropped the gradient fill")
	}
	if !gradientFillEqual(src.GradientFill, clone.GradientFill) {
		t.Fatal("cloneFill did not reproduce the gradient fill")
	}

	*src.GradientFill.Degree = 45
	*src.GradientFill.Bottom = 0.99
	src.GradientFill.Stop[0].Position = 0.5

	if got := *clone.GradientFill.Degree; got != 90 {
		t.Errorf("clone Degree = %v after mutating the source, want 90 (pointers are shared)", got)
	}
	if got := *clone.GradientFill.Bottom; got != 0.4 {
		t.Errorf("clone Bottom = %v after mutating the source, want 0.4 (pointers are shared)", got)
	}
	if got := clone.GradientFill.Stop[0].Position; got != 0 {
		t.Errorf("clone stop position = %v after mutating the source, want 0 (slice is shared)", got)
	}
}

// cellAlignmentEqual compares the optional int32 relative indent; two
// alignments differing only there must not deduplicate onto one style.
func TestCellAlignmentEqualityUsesRelativeIndent(t *testing.T) {
	i := func(v int32) *int32 { return &v }
	base := func() *oxml.CT_CellAlignment {
		return &oxml.CT_CellAlignment{Horizontal: "center", RelativeIndent: i(2)}
	}
	if !cellAlignmentEqual(base(), base()) {
		t.Fatal("identical alignments compare unequal")
	}
	other := base()
	other.RelativeIndent = i(3)
	if cellAlignmentEqual(base(), other) {
		t.Error("alignments differing in relativeIndent compare equal")
	}
	other = base()
	other.RelativeIndent = nil
	if cellAlignmentEqual(base(), other) {
		t.Error("an alignment with relativeIndent unset compares equal to one that sets it")
	}
}
