package docx

import (
	"os"
	"path/filepath"
	"testing"
)

func TestParagraph_SpaceBefore(t *testing.T) {
	doc := Create()
	p := doc.AddParagraphWithText("Hello")
	p.SetSpaceBefore(12)

	if got := p.SpaceBefore(); got != 12 {
		t.Errorf("SpaceBefore() = %v, want 12", got)
	}
}

func TestParagraph_SpaceAfter(t *testing.T) {
	doc := Create()
	p := doc.AddParagraphWithText("Hello")
	p.SetSpaceAfter(6)

	if got := p.SpaceAfter(); got != 6 {
		t.Errorf("SpaceAfter() = %v, want 6", got)
	}
}

func TestParagraph_LineSpacing(t *testing.T) {
	doc := Create()
	p := doc.AddParagraphWithText("Hello")
	p.SetLineSpacing(1.5)

	if p.p.PPr == nil || p.p.PPr.Spacing == nil {
		t.Fatal("expected spacing to be set")
	}
	if p.p.PPr.Spacing.Line != "360" {
		t.Errorf("Line = %s, want 360", p.p.PPr.Spacing.Line)
	}
	if p.p.PPr.Spacing.LineRule != "auto" {
		t.Errorf("LineRule = %s, want auto", p.p.PPr.Spacing.LineRule)
	}
}

func TestParagraph_LineSpacingExact(t *testing.T) {
	doc := Create()
	p := doc.AddParagraphWithText("Hello")
	p.SetLineSpacingExact(14)

	if p.p.PPr == nil || p.p.PPr.Spacing == nil {
		t.Fatal("expected spacing to be set")
	}
	// 14 points * 20 = 280 twips
	if p.p.PPr.Spacing.Line != "280" {
		t.Errorf("Line = %s, want 280", p.p.PPr.Spacing.Line)
	}
	if p.p.PPr.Spacing.LineRule != "exact" {
		t.Errorf("LineRule = %s, want exact", p.p.PPr.Spacing.LineRule)
	}
}

func TestParagraph_Indentation(t *testing.T) {
	doc := Create()
	p := doc.AddParagraphWithText("Hello")
	p.SetIndentLeft(36)
	p.SetIndentFirstLine(18)

	if p.p.PPr == nil || p.p.PPr.Ind == nil {
		t.Fatal("expected indentation to be set")
	}
	// 36 points * 20 = 720 twips
	if p.p.PPr.Ind.Left != "720" {
		t.Errorf("Left = %s, want 720", p.p.PPr.Ind.Left)
	}
	// 18 points * 20 = 360 twips
	if p.p.PPr.Ind.FirstLine != "360" {
		t.Errorf("FirstLine = %s, want 360", p.p.PPr.Ind.FirstLine)
	}
}

func TestParagraph_HangingIndent(t *testing.T) {
	doc := Create()
	p := doc.AddParagraphWithText("Hello")
	p.SetIndentHanging(18)

	if p.p.PPr.Ind.Hanging != "360" {
		t.Errorf("Hanging = %s, want 360", p.p.PPr.Ind.Hanging)
	}
	// Hanging should clear FirstLine
	if p.p.PPr.Ind.FirstLine != "" {
		t.Errorf("FirstLine should be empty when hanging is set, got %s", p.p.PPr.Ind.FirstLine)
	}
}

func TestParagraph_FlowControl(t *testing.T) {
	doc := Create()
	p := doc.AddParagraphWithText("Hello")

	p.SetKeepWithNext(true)
	if p.p.PPr.KeepNext == nil {
		t.Error("expected keepNext to be set")
	}

	p.SetKeepTogether(true)
	if p.p.PPr.KeepLines == nil {
		t.Error("expected keepLines to be set")
	}

	p.SetPageBreakBefore(true)
	if p.p.PPr.PageBreakBefore == nil {
		t.Error("expected pageBreakBefore to be set")
	}

	// Turn off
	p.SetKeepWithNext(false)
	if p.p.PPr.KeepNext != nil {
		t.Error("expected keepNext to be nil")
	}
}

func TestSection_PageSize(t *testing.T) {
	doc := Create()
	sec := doc.DefaultSection()

	sec.SetPageSize(PageSizeA4())
	w, h := sec.PageSize()

	// Allow small floating point differences due to twips conversion
	if w < 595 || w > 596 {
		t.Errorf("width = %v, want ~595.3", w)
	}
	if h < 841 || h > 842 {
		t.Errorf("height = %v, want ~841.9", h)
	}
}

func TestSection_Orientation(t *testing.T) {
	doc := Create()
	sec := doc.DefaultSection()

	sec.SetPageSize(PageSizeLetter())
	sec.SetOrientation(OrientationLandscape)

	if sec.Orientation() != OrientationLandscape {
		t.Error("expected landscape")
	}

	// Dimensions should be swapped
	w, h := sec.PageSize()
	if w < 791 || w > 793 {
		t.Errorf("landscape width = %v, want ~792", w)
	}
	if h < 611 || h > 613 {
		t.Errorf("landscape height = %v, want ~612", h)
	}
}

func TestSection_Margins(t *testing.T) {
	doc := Create()
	sec := doc.DefaultSection()

	sec.SetMargins(PageMargins{Top: 72, Bottom: 72, Left: 72, Right: 72})
	m := sec.Margins()

	if m.Top != 72 {
		t.Errorf("Top = %v, want 72", m.Top)
	}
	if m.Left != 72 {
		t.Errorf("Left = %v, want 72", m.Left)
	}
}

func TestBulletList(t *testing.T) {
	doc := Create()
	bullets := doc.AddBulletList()

	items := []string{"Item 1", "Item 2", "Item 3"}
	for _, item := range items {
		p := doc.AddParagraphWithText(item)
		p.SetListStyle(bullets, 0)
	}

	// Verify list was applied
	paras := doc.Paragraphs()
	for i, p := range paras {
		if p.p.PPr == nil || p.p.PPr.NumPr == nil {
			t.Errorf("paragraph %d: expected NumPr to be set", i)
			continue
		}
		if p.p.PPr.NumPr.NumId == nil {
			t.Errorf("paragraph %d: expected NumId to be set", i)
		}
	}
}

func TestNumberedList(t *testing.T) {
	doc := Create()
	numbered := doc.AddNumberedList()

	p := doc.AddParagraphWithText("First")
	p.SetListStyle(numbered, 0)

	if p.p.PPr.NumPr.Ilvl.Val != 0 {
		t.Errorf("level = %d, want 0", p.p.PPr.NumPr.Ilvl.Val)
	}
}

func TestRemoveListStyle(t *testing.T) {
	doc := Create()
	bullets := doc.AddBulletList()

	p := doc.AddParagraphWithText("Item")
	p.SetListStyle(bullets, 0)
	p.RemoveListStyle()

	if p.p.PPr.NumPr != nil {
		t.Error("expected NumPr to be nil after RemoveListStyle")
	}
}

func TestDocx_SaveWithFormatting(t *testing.T) {
	doc := Create()

	// Set up section
	sec := doc.DefaultSection()
	sec.SetPageSize(PageSizeA4())
	sec.SetMargins(PageMargins{Top: 72, Bottom: 72, Left: 72, Right: 72})

	// Add heading
	h := doc.AddHeading("Test Document", 1)
	h.SetSpaceAfter(12)

	// Add body paragraph
	p := doc.AddParagraphWithText("This is a test paragraph.")
	p.SetSpaceBefore(6)
	p.SetSpaceAfter(6)
	p.SetLineSpacing(1.15)
	p.SetIndentFirstLine(36)

	// Add bullet list
	bullets := doc.AddBulletList()
	for _, item := range []string{"Item A", "Item B", "Item C"} {
		li := doc.AddParagraphWithText(item)
		li.SetListStyle(bullets, 0)
	}

	// Save
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "formatted.docx")
	if err := doc.Save(path); err != nil {
		t.Fatal(err)
	}

	// Verify file exists
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() == 0 {
		t.Fatal("output file is empty")
	}

	// Reopen and verify
	doc2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer doc2.Close() //nolint:errcheck

	paras := doc2.Paragraphs()
	if len(paras) < 5 {
		t.Fatalf("expected at least 5 paragraphs, got %d", len(paras))
	}
}

func TestSectionBreak(t *testing.T) {
	doc := Create()

	doc.AddParagraphWithText("Page 1 content")

	sec2 := doc.AddSectionBreak()
	sec2.SetPageSize(PageSizeLetter())
	sec2.SetOrientation(OrientationLandscape)

	doc.AddParagraphWithText("Page 2 content (landscape)")

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "sections.docx")
	if err := doc.Save(path); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() == 0 {
		t.Fatal("output file is empty")
	}
}
