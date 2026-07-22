package xlsx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"os"
	"strings"
	"testing"

	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// --- part-level parser tests ------------------------------------------------

const slicerCacheXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
	`<slicerCacheDefinition xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:x14="http://schemas.microsoft.com/office/spreadsheetml/2009/9/main" name="Slicer_Region" sourceName="Region">` +
	`<pivotTables><pivotTable tabId="1" name="PivotTable1"/></pivotTables>` +
	`<data><tabular pivotCacheId="1"><items count="2"><i x="0"/><i x="1"/></items></tabular></data>` +
	`</slicerCacheDefinition>`

const slicerXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
	`<slicers xmlns="http://schemas.microsoft.com/office/spreadsheetml/2009/9/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">` +
	`<slicer name="Region" cache="Slicer_Region" caption="Region Filter" rowHeight="241300" columnCount="2"/>` +
	`</slicers>`

const timelineCacheXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
	`<timelineCacheDefinition xmlns="http://schemas.microsoft.com/office/spreadsheetml/2010/11/main" name="NativeTimeline_Date" sourceName="Date">` +
	`<pivotTables><pivotTable tabId="1" name="PivotTable1"/></pivotTables>` +
	`<state minimalRefreshVersion="6" lastRefreshVersion="6" filterType="years"/>` +
	`</timelineCacheDefinition>`

const timelineXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
	`<timelines xmlns="http://schemas.microsoft.com/office/spreadsheetml/2010/11/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">` +
	`<timeline name="Date" cache="NativeTimeline_Date" caption="Date Range" level="2" selectionLevel="2"/>` +
	`</timelines>`

func TestParseSlicerCacheDefinition(t *testing.T) {
	def, err := oxml.ParseSlicerCacheDefinition([]byte(slicerCacheXML))
	if err != nil {
		t.Fatalf("ParseSlicerCacheDefinition: %v", err)
	}
	if def.Name != "Slicer_Region" || def.SourceName != "Region" {
		t.Fatalf("name/source = %q/%q", def.Name, def.SourceName)
	}
	if !def.HasPivotCacheID || def.PivotCacheID != 1 {
		t.Errorf("pivotCacheId = %d (has=%v)", def.PivotCacheID, def.HasPivotCacheID)
	}
	if len(def.PivotTables) != 1 || def.PivotTables[0].Name != "PivotTable1" || def.PivotTables[0].TabID != 1 {
		t.Errorf("pivotTables = %+v", def.PivotTables)
	}
}

func TestParseSlicers(t *testing.T) {
	slicers, err := oxml.ParseSlicers([]byte(slicerXML))
	if err != nil {
		t.Fatalf("ParseSlicers: %v", err)
	}
	if len(slicers) != 1 {
		t.Fatalf("len = %d", len(slicers))
	}
	s := slicers[0]
	if s.Name != "Region" || s.Cache != "Slicer_Region" || s.Caption != "Region Filter" {
		t.Errorf("slicer = %+v", s)
	}
	if !s.HasColumnCount || s.ColumnCount != 2 {
		t.Errorf("columnCount = %d (has=%v)", s.ColumnCount, s.HasColumnCount)
	}
}

func TestParseTimelineCacheAndTimelines(t *testing.T) {
	def, err := oxml.ParseTimelineCacheDefinition([]byte(timelineCacheXML))
	if err != nil {
		t.Fatalf("ParseTimelineCacheDefinition: %v", err)
	}
	if def.Name != "NativeTimeline_Date" || def.SourceName != "Date" {
		t.Fatalf("name/source = %q/%q", def.Name, def.SourceName)
	}
	if len(def.PivotTables) != 1 || def.PivotTables[0].Name != "PivotTable1" {
		t.Errorf("pivotTables = %+v", def.PivotTables)
	}
	tls, err := oxml.ParseTimelines([]byte(timelineXML))
	if err != nil {
		t.Fatalf("ParseTimelines: %v", err)
	}
	if len(tls) != 1 || tls[0].Name != "Date" || tls[0].Cache != "NativeTimeline_Date" {
		t.Fatalf("timelines = %+v", tls)
	}
	if !tls[0].HasLevel || tls[0].Level != 2 {
		t.Errorf("level = %d (has=%v)", tls[0].Level, tls[0].HasLevel)
	}
}

// --- extension round-trip at the marshal level ------------------------------

// A worksheet's slicer/timeline extLst references (unknown-URI extensions) must
// survive a re-marshal of the worksheet byte-for-byte.
func TestSlicerTimelineWorksheetExtVerbatim(t *testing.T) {
	const ext = `<extLst>` +
		`<ext uri="{A8765BA9-456A-4DAB-B4F3-ACF838C121DE}" xmlns:x14="http://schemas.microsoft.com/office/spreadsheetml/2009/9/main"><x14:slicerList><x14:slicer r:id="rId1"/></x14:slicerList></ext>` +
		`<ext uri="{7E03D99C-DC04-49D9-9315-930204A7B6E9}" xmlns:x15="http://schemas.microsoft.com/office/spreadsheetml/2010/11/main"><x15:timelineRefs><x15:timelineRef r:id="rId2"/></x15:timelineRefs></ext>` +
		`</extLst>`
	wsXML := `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">` +
		`<sheetData/>` + ext + `</worksheet>`
	var ws oxml.CT_Worksheet
	if err := xml.Unmarshal([]byte(wsXML), &ws); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out, err := marshalWorksheetXML(&ws)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(out), ext) {
		t.Errorf("slicer/timeline extLst not preserved verbatim:\n%s", out)
	}
}

// --- full open -> read -> save round-trip -----------------------------------

// slicerFixture rebuilds minimal.xlsx into a workbook carrying a slicer and a
// timeline: the definition parts, their cache parts, the worksheet/workbook
// relationships and the worksheet/workbook extLst references.
func slicerFixture(t *testing.T) []byte {
	t.Helper()
	src, err := os.ReadFile("testdata/minimal.xlsx")
	if err != nil {
		t.Fatalf("read minimal.xlsx: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(src), int64(len(src)))
	if err != nil {
		t.Fatalf("zip open: %v", err)
	}

	const wsExt = `<extLst>` +
		`<ext uri="{A8765BA9-456A-4DAB-B4F3-ACF838C121DE}" xmlns:x14="http://schemas.microsoft.com/office/spreadsheetml/2009/9/main"><x14:slicerList><x14:slicer r:id="rId1"/></x14:slicerList></ext>` +
		`<ext uri="{7E03D99C-DC04-49D9-9315-930204A7B6E9}" xmlns:x15="http://schemas.microsoft.com/office/spreadsheetml/2010/11/main"><x15:timelineRefs><x15:timelineRef r:id="rId2"/></x15:timelineRefs></ext>` +
		`</extLst>`
	const wbExt = `<extLst>` +
		`<ext uri="{BBE1A952-AA13-448E-AADC-164F8A28A991}" xmlns:x14="http://schemas.microsoft.com/office/spreadsheetml/2009/9/main"><x14:slicerCaches><x14:slicerCache r:id="rId3"/></x14:slicerCaches></ext>` +
		`<ext uri="{C9C9C9C9-8B10-4B87-9E0D-2C0D3A3B0A0F}" xmlns:x15="http://schemas.microsoft.com/office/spreadsheetml/2010/11/main"><x15:timelineCacheRefs><x15:timelineCacheRef r:id="rId4"/></x15:timelineCacheRefs></ext>` +
		`</extLst>`

	sheetRels := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rId1" Type="http://schemas.microsoft.com/office/2007/relationships/slicer" Target="../slicers/slicer1.xml"/>` +
		`<Relationship Id="rId2" Type="http://schemas.microsoft.com/office/2011/relationships/timeline" Target="../timelines/timeline1.xml"/>` +
		`</Relationships>`

	overrides := map[string]string{
		"xl/worksheets/_rels/sheet1.xml.rels":  sheetRels,
		"xl/slicers/slicer1.xml":               slicerXML,
		"xl/slicerCaches/slicerCache1.xml":     slicerCacheXML,
		"xl/timelines/timeline1.xml":           timelineXML,
		"xl/timelineCaches/timelineCache1.xml": timelineCacheXML,
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	seen := map[string]bool{}
	writeEntry := func(name, content string) {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create %s: %v", name, err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
		seen[name] = true
	}

	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open %s: %v", f.Name, err)
		}
		b := new(bytes.Buffer)
		if _, err := b.ReadFrom(rc); err != nil {
			t.Fatalf("read %s: %v", f.Name, err)
		}
		_ = rc.Close()
		s := b.String()
		switch f.Name {
		case "xl/worksheets/sheet1.xml":
			s = strings.Replace(s, "</worksheet>", wsExt+"</worksheet>", 1)
		case "xl/workbook.xml":
			s = strings.Replace(s, "</workbook>", wbExt+"</workbook>", 1)
		case "xl/_rels/workbook.xml.rels":
			extra := `<Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/slicerCache" Target="slicerCaches/slicerCache1.xml"/>` +
				`<Relationship Id="rId4" Type="http://schemas.microsoft.com/office/2011/relationships/timelineCache" Target="timelineCaches/timelineCache1.xml"/>`
			s = strings.Replace(s, "</Relationships>", extra+"</Relationships>", 1)
		case "[Content_Types].xml":
			extra := `<Override PartName="/xl/slicers/slicer1.xml" ContentType="application/vnd.ms-excel.slicer+xml"/>` +
				`<Override PartName="/xl/slicerCaches/slicerCache1.xml" ContentType="application/vnd.ms-excel.slicerCache+xml"/>` +
				`<Override PartName="/xl/timelines/timeline1.xml" ContentType="application/vnd.ms-excel.timeline+xml"/>` +
				`<Override PartName="/xl/timelineCaches/timelineCache1.xml" ContentType="application/vnd.ms-excel.timelineCache+xml"/>`
			s = strings.Replace(s, "</Types>", extra+"</Types>", 1)
		}
		writeEntry(f.Name, s)
	}
	for name, content := range overrides {
		if !seen[name] {
			writeEntry(name, content)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return buf.Bytes()
}

// Slicers and timelines from an opened workbook are read with their source
// field, caption and controlled pivot tables resolved through the cache parts.
func TestSlicerTimelineRead(t *testing.T) {
	pkg := slicerFixture(t)
	wb, err := OpenReader(bytes.NewReader(pkg), int64(len(pkg)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	defer func() { _ = wb.Close() }()

	slicers := wb.Slicers()
	if len(slicers) != 1 {
		t.Fatalf("Slicers() len = %d, want 1", len(slicers))
	}
	sl := slicers[0]
	if sl.Name() != "Region" || sl.Caption() != "Region Filter" || sl.SourceField() != "Region" {
		t.Errorf("slicer = name=%q caption=%q field=%q", sl.Name(), sl.Caption(), sl.SourceField())
	}
	if sl.ColumnCount() != 2 || sl.SheetName() != "Sheet1" {
		t.Errorf("slicer columnCount=%d sheet=%q", sl.ColumnCount(), sl.SheetName())
	}
	if pts := sl.PivotTables(); len(pts) != 1 || pts[0] != "PivotTable1" {
		t.Errorf("slicer pivotTables = %v", pts)
	}

	timelines := wb.Timelines()
	if len(timelines) != 1 {
		t.Fatalf("Timelines() len = %d, want 1", len(timelines))
	}
	tl := timelines[0]
	if tl.Name() != "Date" || tl.Caption() != "Date Range" || tl.SourceField() != "Date" {
		t.Errorf("timeline = name=%q caption=%q field=%q", tl.Name(), tl.Caption(), tl.SourceField())
	}
	if tl.Level() != 2 || tl.SheetName() != "Sheet1" {
		t.Errorf("timeline level=%d sheet=%q", tl.Level(), tl.SheetName())
	}

	// Sheet-scoped accessors agree.
	s := firstSheet(t, wb)
	if len(s.Slicers()) != 1 || len(s.Timelines()) != 1 {
		t.Errorf("sheet slicers=%d timelines=%d", len(s.Slicers()), len(s.Timelines()))
	}
}

// A workbook with slicers/timelines round-trips: the definition and cache parts
// are preserved byte-for-byte and the worksheet/workbook extLst references
// survive the save.
func TestSlicerTimelineRoundTrip(t *testing.T) {
	pkg := slicerFixture(t)
	wb, err := OpenReader(bytes.NewReader(pkg), int64(len(pkg)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	defer func() { _ = wb.Close() }()

	out, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	// The definition/cache parts are preserved byte-for-byte.
	for name, want := range map[string]string{
		"xl/slicers/slicer1.xml":               slicerXML,
		"xl/slicerCaches/slicerCache1.xml":     slicerCacheXML,
		"xl/timelines/timeline1.xml":           timelineXML,
		"xl/timelineCaches/timelineCache1.xml": timelineCacheXML,
	} {
		got := string(readZipEntry(t, out, name))
		if got != want {
			t.Errorf("%s not preserved:\ngot:  %s\nwant: %s", name, got, want)
		}
	}

	// The worksheet extLst passes through verbatim (the sheet was never
	// modified) and the workbook extLst survives its regeneration.
	sheet1 := string(readZipEntry(t, out, "xl/worksheets/sheet1.xml"))
	if !strings.Contains(sheet1, `<x14:slicer r:id="rId1"/>`) || !strings.Contains(sheet1, `<x15:timelineRef r:id="rId2"/>`) {
		t.Errorf("worksheet slicer/timeline refs lost:\n%s", sheet1)
	}
	workbook := string(readZipEntry(t, out, "xl/workbook.xml"))
	if !strings.Contains(workbook, `<x14:slicerCache r:id="rId3"/>`) || !strings.Contains(workbook, `<x15:timelineCacheRef r:id="rId4"/>`) {
		t.Errorf("workbook slicer/timeline cache refs lost:\n%s", workbook)
	}
}
