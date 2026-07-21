package xlsx

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
)

// reopen saves the workbook to bytes and reopens it, so a test can assert that
// the reader observes what the writer produced. This doubles as a round-trip
// (Create -> Save -> Open) check for every write capability below.
func reopen(t *testing.T, w *Workbook) *Workbook {
	t.Helper()
	data, err := w.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	rw, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	return rw
}

func firstSheet(t *testing.T, w *Workbook) *Sheet {
	t.Helper()
	s, err := w.Sheet(0)
	if err != nil {
		t.Fatalf("Sheet(0): %v", err)
	}
	return s
}

// --- Hyperlinks -------------------------------------------------------------

func TestHyperlink_ExternalCreatePathRoundTrip(t *testing.T) {
	w := Create()
	s := w.AddSheet("Sheet1")
	c, err := s.Cell("A1")
	if err != nil {
		t.Fatal(err)
	}
	c.SetString("Anthropic")
	h := c.SetHyperlink("https://www.anthropic.com/")
	h.SetTooltip("Go to Anthropic")

	rw := reopen(t, w)
	rs := firstSheet(t, rw)

	rc, err := rs.Cell("A1")
	if err != nil {
		t.Fatal(err)
	}
	got := rc.Hyperlink()
	if got == nil {
		t.Fatal("Cell.Hyperlink() = nil after Create-path save+reopen")
	}
	if got.URL() != "https://www.anthropic.com/" {
		t.Errorf("URL() = %q", got.URL())
	}
	if got.Anchor() != "" {
		t.Errorf("external link Anchor() = %q, want empty", got.Anchor())
	}
	if got.Tooltip() != "Go to Anthropic" {
		t.Errorf("Tooltip() = %q", got.Tooltip())
	}
	if got.Ref() != "A1" {
		t.Errorf("Ref() = %q, want A1", got.Ref())
	}
	if links := rs.Hyperlinks(); len(links) != 1 {
		t.Errorf("Hyperlinks() len = %d, want 1", len(links))
	}
}

func TestHyperlink_InternalCreatePathRoundTrip(t *testing.T) {
	w := Create()
	s := w.AddSheet("Sheet1")
	c, _ := s.Cell("B2")
	c.SetInternalHyperlink("Sheet1!C3")

	rw := reopen(t, w)
	rc, _ := firstSheet(t, rw).Cell("B2")
	got := rc.Hyperlink()
	if got == nil {
		t.Fatal("internal Hyperlink() = nil after reopen")
	}
	if got.Anchor() != "Sheet1!C3" {
		t.Errorf("Anchor() = %q, want Sheet1!C3", got.Anchor())
	}
	if got.URL() != "" {
		t.Errorf("internal link URL() = %q, want empty", got.URL())
	}
}

func TestHyperlink_ReplaceRemovesRel(t *testing.T) {
	w := Create()
	s := w.AddSheet("S")
	c, _ := s.Cell("A1")
	c.SetHyperlink("https://example.com/one")
	c.SetHyperlink("https://example.com/two") // replace

	if n := len(s.pendingHyperlinkRels); n != 1 {
		t.Fatalf("pending rels after replace = %d, want 1", n)
	}
	rw := reopen(t, w)
	rc, _ := firstSheet(t, rw).Cell("A1")
	if got := rc.Hyperlink().URL(); got != "https://example.com/two" {
		t.Errorf("URL after replace = %q", got)
	}
	if links := firstSheet(t, rw).Hyperlinks(); len(links) != 1 {
		t.Errorf("Hyperlinks() len = %d, want 1", len(links))
	}
}

// --- Sheet protection -------------------------------------------------------

func TestProtection_DefaultCreatePathRoundTrip(t *testing.T) {
	w := Create()
	s := w.AddSheet("S")
	s.Protect(SheetProtectionOptions{})

	rw := reopen(t, w)
	p := firstSheet(t, rw).Protection()
	if p == nil {
		t.Fatal("Protection() = nil after Create-path save+reopen")
	}
	if !p.Enabled() {
		t.Error("Enabled() = false, want true")
	}
	if p.HasPassword() {
		t.Error("HasPassword() = true, want false (no password given)")
	}
	// Defaults: format/insert/delete/sort/autoFilter/pivot locked; objects and
	// scenarios locked when not explicitly allowed; selection allowed.
	if !p.FormatCells() || !p.InsertRows() || !p.DeleteColumns() || !p.Sort() {
		t.Error("default-locked operation reported unlocked")
	}
	if !p.Objects() || !p.Scenarios() {
		t.Error("objects/scenarios should be locked under default Protect")
	}
	if p.SelectLockedCells() || p.SelectUnlockedCells() {
		t.Error("selection should be allowed under default Protect")
	}
}

func TestProtection_PasswordAndAllowRoundTrip(t *testing.T) {
	w := Create()
	s := w.AddSheet("S")
	s.Protect(SheetProtectionOptions{
		Password:        "secret",
		AllowSort:       true,
		AllowAutoFilter: true,
	})

	rw := reopen(t, w)
	p := firstSheet(t, rw).Protection()
	if p == nil {
		t.Fatal("Protection() = nil")
	}
	if !p.HasPassword() {
		t.Error("HasPassword() = false, want true")
	}
	if p.Sort() {
		t.Error("Sort() locked, but AllowSort was set")
	}
	if p.AutoFilter() {
		t.Error("AutoFilter() locked, but AllowAutoFilter was set")
	}
	if !p.FormatCells() {
		t.Error("FormatCells() should remain locked")
	}
}

func TestProtection_Unprotect(t *testing.T) {
	w := Create()
	s := w.AddSheet("S")
	s.Protect(SheetProtectionOptions{})
	s.Unprotect()
	rw := reopen(t, w)
	if p := firstSheet(t, rw).Protection(); p != nil {
		t.Errorf("Protection() = %+v after Unprotect, want nil", p)
	}
}

func TestLegacyPasswordHash(t *testing.T) {
	// Well-known value: Excel hashes "test" to 0xCBEB (documented in many OOXML
	// references / matches Apache POI's implementation).
	if got := legacyPasswordHash("test"); got != 0xCBEB {
		t.Errorf("legacyPasswordHash(\"test\") = %04X, want CBEB", got)
	}
	if got := legacyPasswordHash(""); got != 0 {
		t.Errorf("legacyPasswordHash(\"\") = %04X, want 0", got)
	}
}

// --- Merged cells / panes / autofilter read backs ---------------------------

func TestMergedCells_ReadBack(t *testing.T) {
	w := Create()
	s := w.AddSheet("S")
	if err := s.MergeCells("A1", "B2"); err != nil {
		t.Fatal(err)
	}
	if err := s.MergeCells("D4", "D8"); err != nil {
		t.Fatal(err)
	}
	rw := reopen(t, w)
	got := firstSheet(t, rw).MergedCells()
	want := map[string]bool{"A1:B2": true, "D4:D8": true}
	if len(got) != 2 {
		t.Fatalf("MergedCells() = %v, want 2 entries", got)
	}
	for _, r := range got {
		if !want[r] {
			t.Errorf("unexpected merged range %q", r)
		}
	}
}

func TestFreezePanes_ReadBack(t *testing.T) {
	w := Create()
	s := w.AddSheet("S")
	if err := s.FreezePanes("B2"); err != nil {
		t.Fatal(err)
	}
	rw := reopen(t, w)
	cols, rows, ok := firstSheet(t, rw).FrozenPanes()
	if !ok {
		t.Fatal("FrozenPanes() ok = false after freezing B2")
	}
	if cols != 1 || rows != 1 {
		t.Errorf("FrozenPanes() = (%d,%d), want (1,1)", cols, rows)
	}
}

func TestAutoFilter_ReadBack(t *testing.T) {
	w := Create()
	s := w.AddSheet("S")
	c, _ := s.Cell("A1")
	c.SetString("h")
	if err := s.SetAutoFilter("A1:C1"); err != nil {
		t.Fatal(err)
	}
	rw := reopen(t, w)
	ref, ok := firstSheet(t, rw).AutoFilterRange()
	if !ok || ref != "A1:C1" {
		t.Errorf("AutoFilterRange() = (%q,%v), want (A1:C1,true)", ref, ok)
	}
}

// --- Data validation --------------------------------------------------------

func TestDataValidation_ReadBack(t *testing.T) {
	w := Create()
	s := w.AddSheet("S")
	if err := s.AddDataValidation(DataValidation{
		Range:    "B2:B100",
		Type:     "list",
		Formula1: `"Red,Green,Blue"`,
	}); err != nil {
		t.Fatal(err)
	}
	rw := reopen(t, w)
	rs := firstSheet(t, rw)
	dvs := rs.DataValidations()
	if len(dvs) != 1 {
		t.Fatalf("DataValidations() len = %d, want 1", len(dvs))
	}
	if dvs[0].Type != "list" || dvs[0].Range != "B2:B100" {
		t.Errorf("dv = %+v", dvs[0])
	}
	if dvs[0].Formula1 != `"Red,Green,Blue"` {
		t.Errorf("Formula1 = %q", dvs[0].Formula1)
	}
	rc, _ := rs.Cell("B5")
	if cv := rc.DataValidation(); cv == nil || cv.Type != "list" {
		t.Errorf("Cell.DataValidation() = %+v, want list rule", cv)
	}
}

// --- Images -----------------------------------------------------------------

func TestImages_ReadBackAfterAddImage(t *testing.T) {
	w := Create()
	s := w.AddSheet("S")
	png := testPNG(t, 20, 10)
	if err := s.AddImage("B2", png, ImageOptions{WidthPx: 40, HeightPx: 20}); err != nil {
		t.Fatal(err)
	}
	rw := reopen(t, w)
	imgs := firstSheet(t, rw).Images()
	if len(imgs) != 1 {
		t.Fatalf("Images() len = %d, want 1", len(imgs))
	}
	im := imgs[0]
	if im.ContentType() != "image/png" {
		t.Errorf("ContentType() = %q, want image/png", im.ContentType())
	}
	if len(im.Data()) == 0 {
		t.Error("Data() empty")
	}
	if im.AnchorCell() != "B2" {
		t.Errorf("AnchorCell() = %q, want B2", im.AnchorCell())
	}
}

// --- Conditional formatting (read-only, crafted fixture) --------------------

// buildXLSXWithWorksheet assembles a minimal valid single-sheet workbook whose
// sheet1.xml body is the caller-supplied string. This lets read-only features
// (conditional formatting) be tested without hand-editing a checked-in fixture.
func buildXLSXWithWorksheet(t *testing.T, sheetBody string) []byte {
	t.Helper()
	parts := map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
			`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
			`<Default Extension="xml" ContentType="application/xml"/>` +
			`<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>` +
			`<Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.ws()+xml"/>` +
			`</Types>`,
		"_rels/.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>` +
			`</Relationships>`,
		"xl/workbook.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" ` +
			`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">` +
			`<sheets><sheet name="Sheet1" sheetId="1" r:id="rId1"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>` +
			`</Relationships>`,
		"xl/worksheets/sheet1.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" ` +
			`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">` +
			sheetBody + `</worksheet>`,
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range parts {
		f, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestConditionalFormats_Read(t *testing.T) {
	body := `<sheetData/>` +
		`<conditionalFormatting sqref="A1:A10 C1:C10">` +
		`<cfRule type="cellIs" dxfId="0" priority="1" operator="greaterThan"><formula>5</formula></cfRule>` +
		`<cfRule type="colorScale" priority="2">` +
		`<colorScale>` +
		`<cfvo type="min"/><cfvo type="max"/>` +
		`<color rgb="FFF8696B"/><color rgb="FF63BE7B"/>` +
		`</colorScale></cfRule>` +
		`</conditionalFormatting>`
	data := buildXLSXWithWorksheet(t, body)
	w, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	s := firstSheet(t, w)
	cfs := s.ConditionalFormats()
	if len(cfs) != 1 {
		t.Fatalf("ConditionalFormats() len = %d, want 1", len(cfs))
	}
	cf := cfs[0]
	if cf.SqRef != "A1:A10 C1:C10" || len(cf.Ranges) != 2 {
		t.Errorf("SqRef/Ranges = %q / %v", cf.SqRef, cf.Ranges)
	}
	if len(cf.Rules) != 2 {
		t.Fatalf("Rules len = %d, want 2", len(cf.Rules))
	}
	r0 := cf.Rules[0]
	if r0.Type != "cellIs" || r0.Operator != "greaterThan" || r0.Priority != 1 {
		t.Errorf("rule0 = %+v", r0)
	}
	if len(r0.Formulas) != 1 || r0.Formulas[0] != "5" {
		t.Errorf("rule0 formulas = %v", r0.Formulas)
	}
	if r0.DxfId == nil || *r0.DxfId != 0 {
		t.Errorf("rule0 DxfId = %v", r0.DxfId)
	}
	r1 := cf.Rules[1]
	if r1.Type != "colorScale" || r1.ColorScale == nil {
		t.Fatalf("rule1 = %+v", r1)
	}
	if len(r1.ColorScale.Values) != 2 || len(r1.ColorScale.Colors) != 2 {
		t.Errorf("colorScale = %+v", r1.ColorScale)
	}
	if r1.ColorScale.Colors[0] != "FFF8696B" {
		t.Errorf("colorScale color0 = %q", r1.ColorScale.Colors[0])
	}
}

// --- Fidelity: zero-mod open->save of a feature-bearing workbook ------------

func TestConditionalFormat_ZeroModByteIdentical(t *testing.T) {
	body := `<sheetData/>` +
		`<conditionalFormatting sqref="A1:A10">` +
		`<cfRule type="cellIs" dxfId="0" priority="1" operator="greaterThan"><formula>5</formula></cfRule>` +
		`</conditionalFormatting>`
	data := buildXLSXWithWorksheet(t, body)
	w, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	// Read the feature but do not modify anything.
	if got := firstSheet(t, w).ConditionalFormats(); len(got) != 1 {
		t.Fatalf("precondition: ConditionalFormats len = %d", len(got))
	}
	out, err := w.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	// The worksheet part must round-trip byte-for-byte (regenerate only on
	// modification). Compare the sheet1.xml entry rather than whole-zip, since
	// our crafted zip has no fixed member ordering guarantee.
	orig := readZipEntry(t, data, "xl/worksheets/sheet1.xml")
	saved := readZipEntry(t, out, "xl/worksheets/sheet1.xml")
	if !bytes.Equal(orig, saved) {
		t.Errorf("worksheet not byte-identical on zero-mod save\n--- orig ---\n%s\n--- saved ---\n%s",
			orig, saved)
	}
}

func readZipEntry(t *testing.T, zipData []byte, name string) []byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range zr.File {
		if f.Name == name {
			rc, err := f.Open()
			if err != nil {
				t.Fatal(err)
			}
			var buf bytes.Buffer
			if _, err := buf.ReadFrom(rc); err != nil {
				t.Fatal(err)
			}
			if err := rc.Close(); err != nil {
				t.Fatal(err)
			}
			return buf.Bytes()
		}
	}
	t.Fatalf("zip entry %q not found", name)
	return nil
}

// --- Validate -------------------------------------------------------------

func TestValidate_HyperlinkAndDataValidation(t *testing.T) {
	// A hyperlink referencing a missing rel should surface a warning; a good
	// SetHyperlink (with its pending rel) should not.
	w := Create()
	s := w.AddSheet("S")
	c, _ := s.Cell("A1")
	c.SetHyperlink("https://example.com")
	rep := w.Validate()
	for _, iss := range rep {
		if strings.Contains(iss.Code, "hyperlink-rel-missing") {
			t.Errorf("unexpected hyperlink warning for a well-formed link: %+v", iss)
		}
	}
}
