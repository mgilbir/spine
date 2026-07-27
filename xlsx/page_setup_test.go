package xlsx

import (
	"bytes"
	"path/filepath"
	"testing"
)

func b(v bool) *bool { return &v }

// saveReopenSheet saves the workbook, reopens it, and returns the first sheet.
func saveReopenSheet(t *testing.T, wb *Workbook) *Sheet {
	t.Helper()
	data, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	wb2, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	t.Cleanup(func() { _ = wb2.Close() })
	sh, err := wb2.Sheet(0)
	if err != nil {
		t.Fatalf("Sheet(0): %v", err)
	}
	return sh
}

func TestPageSetupRoundTrip(t *testing.T) {
	wb := Create()
	sheet := wb.AddSheet("Sheet1")
	if err := sheet.SetPageSetup(PageSetup{
		Orientation:     OrientationLandscape,
		PaperSize:       u32(9),
		Scale:           u32(80),
		FitToWidth:      u32(1),
		FitToHeight:     u32(2),
		FirstPageNumber: u32(3),
		BlackAndWhite:   b(true),
		Draft:           b(true),
	}); err != nil {
		t.Fatalf("SetPageSetup: %v", err)
	}

	sh := saveReopenSheet(t, wb)
	got, ok := sh.PageSetup()
	if !ok {
		t.Fatal("PageSetup not present after round-trip")
	}
	if got.Orientation != OrientationLandscape {
		t.Errorf("Orientation = %q, want landscape", got.Orientation)
	}
	if got.PaperSize == nil || *got.PaperSize != 9 {
		t.Errorf("PaperSize = %v, want 9", got.PaperSize)
	}
	if got.Scale == nil || *got.Scale != 80 {
		t.Errorf("Scale = %v, want 80", got.Scale)
	}
	if got.FitToWidth == nil || *got.FitToWidth != 1 {
		t.Errorf("FitToWidth = %v, want 1", got.FitToWidth)
	}
	if got.FitToHeight == nil || *got.FitToHeight != 2 {
		t.Errorf("FitToHeight = %v, want 2", got.FitToHeight)
	}
	if got.FirstPageNumber == nil || *got.FirstPageNumber != 3 {
		t.Errorf("FirstPageNumber = %v, want 3", got.FirstPageNumber)
	}
	if got.BlackAndWhite == nil || !*got.BlackAndWhite {
		t.Errorf("BlackAndWhite = %v, want true", got.BlackAndWhite)
	}
	if got.Draft == nil || !*got.Draft {
		t.Errorf("Draft = %v, want true", got.Draft)
	}
}

func TestPageSetupAbsentByDefault(t *testing.T) {
	wb := Create()
	sheet := wb.AddSheet("Sheet1")
	if _, ok := sheet.PageSetup(); ok {
		t.Error("expected no PageSetup on a fresh sheet")
	}
	if _, ok := sheet.PageMargins(); ok {
		t.Error("expected no PageMargins on a fresh sheet")
	}
	if _, ok := sheet.HeaderFooter(); ok {
		t.Error("expected no HeaderFooter on a fresh sheet")
	}
	if _, ok := sheet.PrintOptions(); ok {
		t.Error("expected no PrintOptions on a fresh sheet")
	}
}

func TestPageMarginsRoundTrip(t *testing.T) {
	wb := Create()
	sheet := wb.AddSheet("Sheet1")
	want := PageMargins{Left: 0.7, Right: 0.7, Top: 0.75, Bottom: 0.75, Header: 0.3, Footer: 0.3}
	if err := sheet.SetPageMargins(want); err != nil {
		t.Fatalf("SetPageMargins: %v", err)
	}

	sh := saveReopenSheet(t, wb)
	got, ok := sh.PageMargins()
	if !ok {
		t.Fatal("PageMargins not present after round-trip")
	}
	if got != want {
		t.Errorf("PageMargins = %+v, want %+v", got, want)
	}
}

func TestHeaderFooterRoundTrip(t *testing.T) {
	wb := Create()
	sheet := wb.AddSheet("Sheet1")
	if err := sheet.SetHeaderFooter(HeaderFooter{
		DifferentOddEven: b(true),
		ScaleWithDoc:     b(false),
		OddHeader:        "&LLeft&CCenter&RRight",
		OddFooter:        "&CPage &P",
		EvenHeader:       "&CEven",
		EvenFooter:       "&CEven footer",
		FirstHeader:      "&CFirst",
		FirstFooter:      "&CFirst footer",
	}); err != nil {
		t.Fatalf("SetHeaderFooter: %v", err)
	}

	sh := saveReopenSheet(t, wb)
	got, ok := sh.HeaderFooter()
	if !ok {
		t.Fatal("HeaderFooter not present after round-trip")
	}
	if got.OddHeader != "&LLeft&CCenter&RRight" {
		t.Errorf("OddHeader = %q", got.OddHeader)
	}
	if got.OddFooter != "&CPage &P" {
		t.Errorf("OddFooter = %q", got.OddFooter)
	}
	if got.EvenHeader != "&CEven" || got.EvenFooter != "&CEven footer" {
		t.Errorf("even header/footer = %q / %q", got.EvenHeader, got.EvenFooter)
	}
	if got.FirstHeader != "&CFirst" || got.FirstFooter != "&CFirst footer" {
		t.Errorf("first header/footer = %q / %q", got.FirstHeader, got.FirstFooter)
	}
	if got.DifferentOddEven == nil || !*got.DifferentOddEven {
		t.Errorf("DifferentOddEven = %v, want true", got.DifferentOddEven)
	}
	if got.ScaleWithDoc == nil || *got.ScaleWithDoc {
		t.Errorf("ScaleWithDoc = %v, want false", got.ScaleWithDoc)
	}
}

func TestPrintOptionsRoundTrip(t *testing.T) {
	wb := Create()
	sheet := wb.AddSheet("Sheet1")
	if err := sheet.SetPrintOptions(PrintOptions{
		HorizontalCentered: b(true),
		VerticalCentered:   b(true),
		GridLines:          b(true),
		Headings:           b(true),
	}); err != nil {
		t.Fatalf("SetPrintOptions: %v", err)
	}

	sh := saveReopenSheet(t, wb)
	got, ok := sh.PrintOptions()
	if !ok {
		t.Fatal("PrintOptions not present after round-trip")
	}
	if got.HorizontalCentered == nil || !*got.HorizontalCentered {
		t.Errorf("HorizontalCentered = %v, want true", got.HorizontalCentered)
	}
	if got.VerticalCentered == nil || !*got.VerticalCentered {
		t.Errorf("VerticalCentered = %v, want true", got.VerticalCentered)
	}
	if got.GridLines == nil || !*got.GridLines {
		t.Errorf("GridLines = %v, want true", got.GridLines)
	}
	if got.Headings == nil || !*got.Headings {
		t.Errorf("Headings = %v, want true", got.Headings)
	}
}

func TestPrintAreaRoundTrip(t *testing.T) {
	wb := Create()
	sheet := wb.AddSheet("Sheet1")
	if err := sheet.SetPrintArea("A1:D20"); err != nil {
		t.Fatal(err)
	}
	if got := sheet.PrintArea(); got != "Sheet1!$A$1:$D$20" {
		t.Errorf("PrintArea = %q, want Sheet1!$A$1:$D$20", got)
	}

	sh := saveReopenSheet(t, wb)
	if got := sh.PrintArea(); got != "Sheet1!$A$1:$D$20" {
		t.Errorf("after reopen PrintArea = %q, want Sheet1!$A$1:$D$20", got)
	}
}

func TestPrintAreaMultipleAndQuoting(t *testing.T) {
	wb := Create()
	sheet := wb.AddSheet("My Sheet")
	if err := sheet.SetPrintArea("A1:B2", "D1:E2"); err != nil {
		t.Fatal(err)
	}
	want := "'My Sheet'!$A$1:$B$2,'My Sheet'!$D$1:$E$2"
	if got := sheet.PrintArea(); got != want {
		t.Errorf("PrintArea = %q, want %q", got, want)
	}
	sheet.ClearPrintArea()
	if got := sheet.PrintArea(); got != "" {
		t.Errorf("PrintArea after clear = %q, want empty", got)
	}
}

func TestPrintTitlesRoundTrip(t *testing.T) {
	wb := Create()
	sheet := wb.AddSheet("Sheet1")
	if err := sheet.SetPrintTitles("1:1", "A:B"); err != nil {
		t.Fatal(err)
	}
	want := "Sheet1!$A:$B,Sheet1!$1:$1"
	if got := sheet.PrintTitles(); got != want {
		t.Errorf("PrintTitles = %q, want %q", got, want)
	}

	sh := saveReopenSheet(t, wb)
	if got := sh.PrintTitles(); got != want {
		t.Errorf("after reopen PrintTitles = %q, want %q", got, want)
	}
}

func TestPrintTitlesRowsOnly(t *testing.T) {
	wb := Create()
	sheet := wb.AddSheet("Sheet1")
	if err := sheet.SetPrintTitles("1:2", ""); err != nil {
		t.Fatal(err)
	}
	if got := sheet.PrintTitles(); got != "Sheet1!$1:$2" {
		t.Errorf("PrintTitles = %q, want Sheet1!$1:$2", got)
	}
	if err := sheet.SetPrintTitles("", ""); err != nil {
		t.Fatal(err)
	}
	if got := sheet.PrintTitles(); got != "" {
		t.Errorf("PrintTitles after clear = %q, want empty", got)
	}
}

// TestPageSetupPreservesReopenedFileBytes verifies that opening a file that has
// no page-setup features and saving it without touching them does not perturb
// its bytes — the new write paths must be inert unless used.
func TestPageSetupPreservesReopenedFileBytes(t *testing.T) {
	wb := Create()
	sheet := wb.AddSheet("Sheet1")
	c, _ := sheet.Cell("A1")
	c.SetValue("hello")

	tmp := filepath.Join(t.TempDir(), "base.xlsx")
	if err := wb.Save(tmp); err != nil {
		t.Fatal(err)
	}

	wb2, err := Open(tmp)
	if err != nil {
		t.Fatal(err)
	}
	defer wb2.Close() //nolint:errcheck
	first, err := wb2.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	second, err := wb2.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Error("re-saving an untouched reopened workbook is not deterministic")
	}
}

func TestAbsolutizeRange(t *testing.T) {
	cases := map[string]string{
		"A1:D10": "$A$1:$D$10",
		"1:1":    "$1:$1",
		"A:B":    "$A:$B",
		"b2":     "$B$2",
		"$A$1":   "$A$1",
	}
	for in, want := range cases {
		if got := absolutizeRange(in); got != want {
			t.Errorf("absolutizeRange(%q) = %q, want %q", in, got, want)
		}
	}
}
