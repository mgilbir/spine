package docx

import (
	"bytes"
	"testing"
)

// saveReopenDoc saves the document and reopens it from the serialized bytes.
func saveReopenDoc(t *testing.T, d *Document) *Document {
	t.Helper()
	data, err := d.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	d2, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	t.Cleanup(func() { _ = d2.Close() })
	return d2
}

func TestSectionTypeRoundTrip(t *testing.T) {
	doc := Create()
	doc.AddParagraphWithText("body")
	sec := doc.DefaultSection()
	if sec.SectionType() != "" {
		t.Errorf("fresh SectionType = %q, want empty", sec.SectionType())
	}
	sec.SetSectionType(SectionTypeContinuous)

	got := saveReopenDoc(t, doc).DefaultSection()
	if got.SectionType() != SectionTypeContinuous {
		t.Errorf("SectionType = %q, want %q", got.SectionType(), SectionTypeContinuous)
	}
}

func TestTitlePageRoundTrip(t *testing.T) {
	doc := Create()
	doc.AddParagraphWithText("body")
	sec := doc.DefaultSection()
	if sec.TitlePage() {
		t.Error("fresh section should not have a title page")
	}
	sec.SetTitlePage(true)

	got := saveReopenDoc(t, doc).DefaultSection()
	if !got.TitlePage() {
		t.Error("TitlePage = false after round-trip, want true")
	}

	got.SetTitlePage(false)
	if got.TitlePage() {
		t.Error("TitlePage = true after disabling")
	}
}

func TestPageNumberingRoundTrip(t *testing.T) {
	doc := Create()
	doc.AddParagraphWithText("body")
	sec := doc.DefaultSection()
	if _, ok := sec.PageNumbering(); ok {
		t.Error("fresh section should have no page numbering")
	}
	start := 3
	sec.SetPageNumbering(PageNumbering{Format: PageNumberLowerRoman, Start: &start})

	got := saveReopenDoc(t, doc).DefaultSection()
	pn, ok := got.PageNumbering()
	if !ok {
		t.Fatal("PageNumbering not present after round-trip")
	}
	if pn.Format != PageNumberLowerRoman {
		t.Errorf("Format = %q, want %q", pn.Format, PageNumberLowerRoman)
	}
	if pn.Start == nil || *pn.Start != 3 {
		t.Errorf("Start = %v, want 3", pn.Start)
	}

	got.ClearPageNumbering()
	if _, ok := got.PageNumbering(); ok {
		t.Error("PageNumbering still present after ClearPageNumbering")
	}
}

func TestColumnsEqualWidthRoundTrip(t *testing.T) {
	doc := Create()
	doc.AddParagraphWithText("body")
	sec := doc.DefaultSection()
	if _, ok := sec.Columns(); ok {
		t.Error("fresh section should have no columns element")
	}
	sec.SetColumns(Columns{Count: 3, Spacing: 36, Separator: true, EqualWidth: true})

	got := saveReopenDoc(t, doc).DefaultSection()
	cols, ok := got.Columns()
	if !ok {
		t.Fatal("Columns not present after round-trip")
	}
	if cols.Count != 3 {
		t.Errorf("Count = %d, want 3", cols.Count)
	}
	if cols.Spacing != 36 {
		t.Errorf("Spacing = %v, want 36", cols.Spacing)
	}
	if !cols.Separator {
		t.Error("Separator = false, want true")
	}
	if !cols.EqualWidth {
		t.Error("EqualWidth = false, want true")
	}
}

func TestColumnsExplicitWidthsRoundTrip(t *testing.T) {
	doc := Create()
	doc.AddParagraphWithText("body")
	sec := doc.DefaultSection()
	sec.SetColumns(Columns{
		Count:      2,
		EqualWidth: false,
		Cols: []Column{
			{Width: 200, Spacing: 20},
			{Width: 300},
		},
	})

	got := saveReopenDoc(t, doc).DefaultSection()
	cols, ok := got.Columns()
	if !ok {
		t.Fatal("Columns not present after round-trip")
	}
	if cols.EqualWidth {
		t.Error("EqualWidth = true, want false")
	}
	if len(cols.Cols) != 2 {
		t.Fatalf("len(Cols) = %d, want 2", len(cols.Cols))
	}
	if cols.Cols[0].Width != 200 || cols.Cols[0].Spacing != 20 {
		t.Errorf("Cols[0] = %+v, want {200 20}", cols.Cols[0])
	}
	if cols.Cols[1].Width != 300 {
		t.Errorf("Cols[1].Width = %v, want 300", cols.Cols[1].Width)
	}

	got.ClearColumns()
	if _, ok := got.Columns(); ok {
		t.Error("Columns still present after ClearColumns")
	}
}

func TestSectionsEnumeration(t *testing.T) {
	doc := Create()
	doc.AddParagraphWithText("first section")
	first := doc.DefaultSection()
	first.SetSectionType(SectionTypeContinuous)

	// A section break moves the current body section onto the last paragraph and
	// creates a fresh body section.
	doc.AddSectionBreak()
	doc.AddParagraphWithText("second section")

	secs := doc.Sections()
	if len(secs) != 2 {
		t.Fatalf("Sections() returned %d, want 2", len(secs))
	}
	if secs[0].SectionType() != SectionTypeContinuous {
		t.Errorf("first section type = %q, want continuous", secs[0].SectionType())
	}

	// Survives a round-trip.
	got := saveReopenDoc(t, doc).Sections()
	if len(got) != 2 {
		t.Fatalf("Sections() after reopen = %d, want 2", len(got))
	}
	if got[0].SectionType() != SectionTypeContinuous {
		t.Errorf("reopened first section type = %q, want continuous", got[0].SectionType())
	}
}
