package docx

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
)

// documentXML saves the document and returns the decompressed word/document.xml.
func documentXML(t *testing.T, d *Document) string {
	t.Helper()
	data, err := d.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("zip.NewReader: %v", err)
	}
	for _, f := range zr.File {
		if f.Name != "word/document.xml" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open document.xml: %v", err)
		}
		defer func() { _ = rc.Close() }()
		var buf bytes.Buffer
		if _, err := buf.ReadFrom(rc); err != nil {
			t.Fatalf("read document.xml: %v", err)
		}
		return buf.String()
	}
	t.Fatal("word/document.xml not found")
	return ""
}

// --- Section details ---

func TestSectionPageBordersRoundTrip(t *testing.T) {
	doc := Create()
	doc.AddParagraphWithText("body")
	sec := doc.DefaultSection()
	if _, ok := sec.PageBorders(); ok {
		t.Fatal("fresh section should have no page borders")
	}
	sec.SetPageBorders(PageBorders{
		OffsetFrom: "page",
		Top:        &Border{Style: "single", Width: 1, Color: "FF0000"},
		Bottom:     &Border{Style: "double", Width: 2, Color: "0000FF"},
	})

	got := saveReopenDoc(t, doc).DefaultSection()
	pb, ok := got.PageBorders()
	if !ok {
		t.Fatal("PageBorders missing after round-trip")
	}
	if pb.OffsetFrom != "page" {
		t.Errorf("OffsetFrom = %q, want page", pb.OffsetFrom)
	}
	if pb.Top == nil || pb.Top.Style != "single" || pb.Top.Color != "FF0000" {
		t.Errorf("Top = %+v", pb.Top)
	}
	if pb.Bottom == nil || pb.Bottom.Style != "double" || pb.Bottom.Width != 2 {
		t.Errorf("Bottom = %+v", pb.Bottom)
	}
	if pb.Left != nil {
		t.Errorf("Left = %+v, want nil", pb.Left)
	}
	got.ClearPageBorders()
	if _, ok := got.PageBorders(); ok {
		t.Error("PageBorders present after ClearPageBorders")
	}
}

func TestSectionLineNumberingRoundTrip(t *testing.T) {
	doc := Create()
	doc.AddParagraphWithText("body")
	sec := doc.DefaultSection()
	sec.SetLineNumbering(LineNumbering{CountBy: 5, Start: 1, Distance: 12, Restart: "newPage"})

	got := saveReopenDoc(t, doc).DefaultSection()
	ln, ok := got.LineNumbering()
	if !ok {
		t.Fatal("LineNumbering missing after round-trip")
	}
	if ln.CountBy != 5 || ln.Start != 1 || ln.Distance != 12 || ln.Restart != "newPage" {
		t.Errorf("LineNumbering = %+v", ln)
	}
	got.ClearLineNumbering()
	if _, ok := got.LineNumbering(); ok {
		t.Error("LineNumbering present after clear")
	}
}

func TestSectionVerticalAlignmentRoundTrip(t *testing.T) {
	doc := Create()
	doc.AddParagraphWithText("body")
	doc.DefaultSection().SetVerticalAlignment("center")

	got := saveReopenDoc(t, doc).DefaultSection()
	if got.VerticalAlignment() != "center" {
		t.Errorf("VerticalAlignment = %q, want center", got.VerticalAlignment())
	}
	got.SetVerticalAlignment("")
	if got.VerticalAlignment() != "" {
		t.Errorf("VerticalAlignment = %q after clear", got.VerticalAlignment())
	}
}

func TestSectionPaperSourceRoundTrip(t *testing.T) {
	doc := Create()
	doc.AddParagraphWithText("body")
	doc.DefaultSection().SetPaperSource(4, 7)

	got := saveReopenDoc(t, doc).DefaultSection()
	first, other, ok := got.PaperSource()
	if !ok || first != 4 || other != 7 {
		t.Errorf("PaperSource = (%d, %d, %v)", first, other, ok)
	}
}

func TestSectionDocumentGridRoundTrip(t *testing.T) {
	doc := Create()
	doc.AddParagraphWithText("body")
	doc.DefaultSection().SetDocumentGrid(DocumentGrid{Type: "lines", LinePitch: 360})

	got := saveReopenDoc(t, doc).DefaultSection()
	dg, ok := got.DocumentGrid()
	if !ok || dg.Type != "lines" || dg.LinePitch != 360 {
		t.Errorf("DocumentGrid = %+v, ok=%v", dg, ok)
	}
}

func TestSectionNotePropertiesRoundTrip(t *testing.T) {
	doc := Create()
	doc.AddParagraphWithText("body")
	sec := doc.DefaultSection()
	start := 2
	sec.SetFootnoteProperties(NoteProperties{
		Position:     "beneathText",
		NumberFormat: "lowerRoman",
		NumberStart:  &start,
		Restart:      "eachPage",
	})
	sec.SetEndnoteProperties(NoteProperties{Position: "docEnd", NumberFormat: "decimal"})

	got := saveReopenDoc(t, doc).DefaultSection()
	fp, ok := got.FootnoteProperties()
	if !ok {
		t.Fatal("FootnoteProperties missing")
	}
	if fp.Position != "beneathText" || fp.NumberFormat != "lowerRoman" ||
		fp.NumberStart == nil || *fp.NumberStart != 2 || fp.Restart != "eachPage" {
		t.Errorf("FootnoteProperties = %+v", fp)
	}
	ep, ok := got.EndnoteProperties()
	if !ok || ep.Position != "docEnd" || ep.NumberFormat != "decimal" {
		t.Errorf("EndnoteProperties = %+v, ok=%v", ep, ok)
	}
}

// --- Document settings ---

func TestDefaultTabStopRoundTrip(t *testing.T) {
	doc := Create()
	doc.AddParagraphWithText("body")
	if _, ok := doc.DefaultTabStop(); ok {
		t.Fatal("fresh document should have no default tab stop")
	}
	doc.SetDefaultTabStop(18)

	got := saveReopenDoc(t, doc)
	pts, ok := got.DefaultTabStop()
	if !ok || pts != 18 {
		t.Errorf("DefaultTabStop = (%v, %v), want (18, true)", pts, ok)
	}
}

func TestEvenAndOddHeadersRoundTrip(t *testing.T) {
	doc := Create()
	doc.AddParagraphWithText("body")
	if doc.EvenAndOddHeaders() {
		t.Fatal("fresh document should not declare even/odd headers")
	}
	doc.SetEvenAndOddHeaders(true)

	got := saveReopenDoc(t, doc)
	if !got.EvenAndOddHeaders() {
		t.Error("EvenAndOddHeaders = false after round-trip")
	}
	got.SetEvenAndOddHeaders(false)
	if got.EvenAndOddHeaders() {
		t.Error("EvenAndOddHeaders = true after disabling")
	}
}

func TestZoomRoundTrip(t *testing.T) {
	doc := Create()
	doc.AddParagraphWithText("body")
	doc.SetZoom(150)

	got := saveReopenDoc(t, doc)
	pct, ok := got.Zoom()
	if !ok || pct != 150 {
		t.Errorf("Zoom = (%d, %v), want (150, true)", pct, ok)
	}
}

func TestDocumentVariablesRoundTrip(t *testing.T) {
	doc := Create()
	doc.AddParagraphWithText("body")
	doc.SetDocumentVariable("Author", "Ada <Lovelace>")
	doc.SetDocumentVariable("Year", "1843")
	doc.SetDocumentVariable("Author", "Ada Lovelace") // update in place

	got := saveReopenDoc(t, doc)
	if v, ok := got.DocumentVariable("Author"); !ok || v != "Ada Lovelace" {
		t.Errorf("Author = (%q, %v)", v, ok)
	}
	if v, ok := got.DocumentVariable("Year"); !ok || v != "1843" {
		t.Errorf("Year = (%q, %v)", v, ok)
	}
	vars := got.DocumentVariables()
	if len(vars) != 2 {
		t.Fatalf("DocumentVariables len = %d, want 2", len(vars))
	}
	if !got.RemoveDocumentVariable("Year") {
		t.Error("RemoveDocumentVariable(Year) = false")
	}
	got2 := saveReopenDoc(t, got)
	if _, ok := got2.DocumentVariable("Year"); ok {
		t.Error("Year still present after removal")
	}
	if len(got2.DocumentVariables()) != 1 {
		t.Errorf("DocumentVariables len = %d after removal, want 1", len(got2.DocumentVariables()))
	}
}

// --- Table depth ---

func TestTableVerticalMergeRoundTrip(t *testing.T) {
	doc := Create()
	tbl := doc.AddTable(2, 2)
	rows := tbl.Rows()
	rows[0].Cells()[0].SetVerticalMerge(VerticalMergeRestart)
	rows[1].Cells()[0].SetVerticalMerge(VerticalMergeContinue)

	got := saveReopenDoc(t, doc).Tables()[0]
	grows := got.Rows()
	if m := grows[0].Cells()[0].VerticalMerge(); m != VerticalMergeRestart {
		t.Errorf("row0 VerticalMerge = %q, want restart", m)
	}
	if m := grows[1].Cells()[0].VerticalMerge(); m != VerticalMergeContinue {
		t.Errorf("row1 VerticalMerge = %q, want continue", m)
	}
	if m := grows[0].Cells()[1].VerticalMerge(); m != "" {
		t.Errorf("unmerged cell VerticalMerge = %q, want empty", m)
	}

	// A continued cell must serialize as a bare <w:vMerge/> (no w:val).
	xml := documentXML(t, doc)
	if !strings.Contains(xml, "<w:vMerge/>") && !strings.Contains(xml, "<w:vMerge />") {
		t.Error("continued cell did not emit a bare w:vMerge")
	}
	if !strings.Contains(xml, `<w:vMerge w:val="restart"/>`) {
		t.Error("restart cell did not emit w:vMerge w:val=restart")
	}
}

func TestTableLookRoundTrip(t *testing.T) {
	doc := Create()
	tbl := doc.AddTable(1, 1)
	tbl.SetTableLook(TableLook{FirstRow: true, LastColumn: true, NoVBand: true})

	got := saveReopenDoc(t, doc).Tables()[0]
	look, ok := got.TableLook()
	if !ok {
		t.Fatal("TableLook missing after round-trip")
	}
	if !look.FirstRow || !look.LastColumn || !look.NoVBand {
		t.Errorf("TableLook = %+v", look)
	}
	if look.LastRow || look.FirstColumn || look.NoHBand {
		t.Errorf("TableLook has unexpected flags: %+v", look)
	}
}

func TestTableLayoutIndentAlignmentRoundTrip(t *testing.T) {
	doc := Create()
	tbl := doc.AddTable(1, 1)
	tbl.SetLayout(TableLayoutFixed)
	tbl.SetIndent(15)
	tbl.SetAlignment(AlignmentCenter)

	got := saveReopenDoc(t, doc).Tables()[0]
	if got.Layout() != TableLayoutFixed {
		t.Errorf("Layout = %q, want fixed", got.Layout())
	}
	if ind, ok := got.Indent(); !ok || ind != 15 {
		t.Errorf("Indent = (%v, %v), want (15, true)", ind, ok)
	}
	if got.Alignment() != AlignmentCenter {
		t.Errorf("Alignment = %v, want center", got.Alignment())
	}
}

func TestTableAndCellGettersRoundTrip(t *testing.T) {
	doc := Create()
	tbl := doc.AddTable(1, 1)
	tbl.SetBorders(TableBorders{
		Top:     &Border{Style: "single", Width: 1, Color: "000000"},
		InsideH: &Border{Style: "dotted", Width: 0.5, Color: "AAAAAA"},
	})
	tbl.SetWidth(300)
	cell := tbl.Rows()[0].Cells()[0]
	cell.SetShading("FFFF00")
	cell.SetWidth(120)
	cell.SetVerticalAlignment("center")
	cell.SetGridSpan(2)

	got := saveReopenDoc(t, doc).Tables()[0]
	b, ok := got.Borders()
	if !ok || b.Top == nil || b.Top.Style != "single" || b.InsideH == nil || b.InsideH.Color != "AAAAAA" {
		t.Errorf("Borders = %+v, ok=%v", b, ok)
	}
	if w, ok := got.Width(); !ok || w != 300 {
		t.Errorf("Width = (%v, %v), want (300, true)", w, ok)
	}
	gcell := got.Rows()[0].Cells()[0]
	if gcell.Shading() != "FFFF00" {
		t.Errorf("cell Shading = %q, want FFFF00", gcell.Shading())
	}
	if w, ok := gcell.Width(); !ok || w != 120 {
		t.Errorf("cell Width = (%v, %v), want (120, true)", w, ok)
	}
	if gcell.VerticalAlignment() != "center" {
		t.Errorf("cell VerticalAlignment = %q, want center", gcell.VerticalAlignment())
	}
	if gcell.GridSpan() != 2 {
		t.Errorf("cell GridSpan = %d, want 2", gcell.GridSpan())
	}
}

// --- Paragraph borders & shading ---

func TestParagraphBordersShadingRoundTrip(t *testing.T) {
	doc := Create()
	p := doc.AddParagraphWithText("boxed")
	p.SetBorders(ParagraphBorders{
		Top:    &Border{Style: "single", Width: 1, Color: "000000"},
		Bottom: &Border{Style: "single", Width: 1, Color: "000000"},
	})
	p.SetShading("E0E0E0")

	got := saveReopenDoc(t, doc).Paragraphs()[0]
	pb, ok := got.Borders()
	if !ok || pb.Top == nil || pb.Bottom == nil {
		t.Errorf("Borders = %+v, ok=%v", pb, ok)
	}
	if got.Shading() != "E0E0E0" {
		t.Errorf("Shading = %q, want E0E0E0", got.Shading())
	}
	got.ClearBorders()
	got.ClearShading()
	if _, ok := got.Borders(); ok {
		t.Error("Borders present after ClearBorders")
	}
	if got.Shading() != "" {
		t.Error("Shading present after ClearShading")
	}
}

// --- Run style & symbol ---

func TestRunSetStyleRoundTrip(t *testing.T) {
	doc := Create()
	p := doc.AddParagraph()
	r := p.AddRun()
	r.SetText("styled")
	r.SetStyle("Emphasis")

	got := saveReopenDoc(t, doc).Paragraphs()[0].Runs()[0]
	if got.Style() != "Emphasis" {
		t.Errorf("Style = %q, want Emphasis", got.Style())
	}
	got.SetStyle("")
	if got.Style() != "" {
		t.Errorf("Style = %q after clear", got.Style())
	}
}

func TestRunAddSymbolRoundTrip(t *testing.T) {
	doc := Create()
	p := doc.AddParagraph()
	r := p.AddRun()
	r.AddSymbol("Wingdings", "F0E0")

	s := documentXML(t, doc)
	if !strings.Contains(s, `w:font="Wingdings"`) || !strings.Contains(s, `w:char="F0E0"`) {
		t.Error("saved document does not contain the w:sym glyph")
	}
	// Reopen to confirm the symbol survives a parse/marshal cycle.
	if s2 := documentXML(t, saveReopenDoc(t, doc)); !strings.Contains(s2, `w:char="F0E0"`) {
		t.Error("w:sym did not survive a round-trip")
	}
}
