package xlsx

import (
	"bytes"
	"strings"
	"testing"
)

func setCell(t *testing.T, s *Sheet, ref string, val interface{}) {
	t.Helper()
	if err := s.SetCellValue(ref, val); err != nil {
		t.Fatalf("SetCellValue(%s): %v", ref, err)
	}
}

func TestCopySheetFrom(t *testing.T) {
	src := Create()
	s := addSheetT(src, "Data")
	setCell(t, s, "A1", "Name")
	setCell(t, s, "B1", "Score")
	setCell(t, s, "A2", "Alice")
	setCell(t, s, "B2", 42)
	setCell(t, s, "B3", 8)
	if c, err := s.Cell("B4"); err == nil {
		c.SetFormula("SUM(B2:B3)")
	} else {
		t.Fatalf("Cell B4: %v", err)
	}
	// A styled cell to exercise style index remapping.
	if c, err := s.Cell("A1"); err == nil {
		if err := c.SetStyle(CellStyle{Format: "0.00"}); err != nil {
			t.Fatalf("SetStyle: %v", err)
		}
	}
	if err := s.MergeCells("A1", "B1"); err != nil {
		t.Fatalf("MergeCells: %v", err)
	}
	if err := s.SetColWidth(1, 20); err != nil {
		t.Fatalf("SetColWidth: %v", err)
	}

	dst := Create()
	setCell(t, addSheetT(dst, "Existing"), "A1", "keep")

	newSheet, err := dst.CopySheetFrom(src, "Data")
	if err != nil {
		t.Fatalf("CopySheetFrom: %v", err)
	}
	if newSheet.Name() != "Data" {
		t.Fatalf("new sheet name = %q, want Data", newSheet.Name())
	}
	if got := dst.SheetCount(); got != 2 {
		t.Fatalf("SheetCount = %d, want 2", got)
	}
	if r := dst.Validate(); r.HasErrors() {
		t.Fatalf("Validate after copy: %v", r)
	}

	data, err := dst.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	re, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	if r := re.Validate(); r.HasErrors() {
		t.Fatalf("Validate reopened: %v", r)
	}

	got, err := re.SheetByName("Data")
	if err != nil {
		t.Fatalf("SheetByName after reopen: %v", err)
	}
	assertCell(t, got, "A1", "Name")
	assertCell(t, got, "A2", "Alice")
	assertCell(t, got, "B2", "42")
	// The original "Existing" sheet is untouched.
	existing, err := re.SheetByName("Existing")
	if err != nil {
		t.Fatalf("SheetByName Existing: %v", err)
	}
	assertCell(t, existing, "A1", "keep")

	// The merged range came across.
	if got.ws().MergeCells == nil || len(got.ws().MergeCells.MergeCell) == 0 {
		t.Fatalf("merged range not copied")
	}
}

func assertCell(t *testing.T, s *Sheet, ref, want string) {
	t.Helper()
	got, err := s.GetCellValue(ref)
	if err != nil {
		t.Fatalf("GetCellValue(%s): %v", ref, err)
	}
	if got != want {
		t.Errorf("cell %s = %q, want %q", ref, got, want)
	}
}

func TestCopySheetFromCreatedImage(t *testing.T) {
	// Source sheet carries an image added this session plus a two-cell anchor.
	src := Create()
	s := addSheetT(src, "Pics")
	setCell(t, s, "A1", "header")
	if err := s.AddImage("B2", testPNG(t, 24, 16), ImageOptions{}); err != nil {
		t.Fatalf("AddImage: %v", err)
	}
	if err := s.AddImage("D4", testPNG(t, 12, 12), ImageOptions{ToCell: "F8"}); err != nil {
		t.Fatalf("AddImage two-cell: %v", err)
	}

	dst := Create()
	setCell(t, addSheetT(dst, "Keep"), "A1", "keep")
	if _, err := dst.CopySheetFrom(src, "Pics"); err != nil {
		t.Fatalf("CopySheetFrom: %v", err)
	}
	if r := dst.Validate(); r.HasErrors() {
		t.Fatalf("Validate after copy: %v", r)
	}

	data, err := dst.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	re, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	if r := re.Validate(); r.HasErrors() {
		t.Fatalf("Validate reopened: %v", r)
	}
	got, err := re.SheetByName("Pics")
	if err != nil {
		t.Fatalf("SheetByName: %v", err)
	}
	if n := len(got.Images()); n != 2 {
		t.Fatalf("copied sheet has %d images, want 2", n)
	}
	names := zipNames(t, data)
	drawings, media := 0, 0
	for n := range names {
		if strings.HasPrefix(n, "xl/drawings/drawing") && strings.HasSuffix(n, ".xml") {
			drawings++
		}
		if strings.HasPrefix(n, "xl/media/image") {
			media++
		}
	}
	if drawings != 1 || media != 2 {
		t.Errorf("drawings=%d media=%d, want 1 drawing and 2 media; parts=%v", drawings, media, names)
	}
}

func TestCopySheetFromOpenedImage(t *testing.T) {
	// Build a source workbook, save, and reopen so its image lives in the
	// opened source's preserved drawing/media parts (not src.images).
	seed := Create()
	ss := addSheetT(seed, "Photos")
	setCell(t, ss, "A1", "x")
	if err := ss.AddImage("C3", testPNG(t, 30, 20), ImageOptions{}); err != nil {
		t.Fatalf("AddImage: %v", err)
	}
	seedBytes, err := seed.SaveBytes()
	if err != nil {
		t.Fatalf("seed SaveBytes: %v", err)
	}
	src, err := OpenReader(bytes.NewReader(seedBytes), int64(len(seedBytes)))
	if err != nil {
		t.Fatalf("open source: %v", err)
	}

	dst := Create()
	setCell(t, addSheetT(dst, "Keep"), "A1", "keep")
	if _, err := dst.CopySheetFrom(src, "Photos"); err != nil {
		t.Fatalf("CopySheetFrom: %v", err)
	}
	if r := dst.Validate(); r.HasErrors() {
		t.Fatalf("Validate after copy: %v", r)
	}
	data, err := dst.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	re, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	if r := re.Validate(); r.HasErrors() {
		t.Fatalf("Validate reopened: %v", r)
	}
	got, err := re.SheetByName("Photos")
	if err != nil {
		t.Fatalf("SheetByName: %v", err)
	}
	imgs := got.Images()
	if len(imgs) != 1 {
		t.Fatalf("copied sheet has %d images, want 1", len(imgs))
	}
	if imgs[0].AnchorCell() != "C3" {
		t.Errorf("anchor = %q, want C3", imgs[0].AnchorCell())
	}
}

func TestCopySheetFromDuplicateName(t *testing.T) {
	src := Create()
	setCell(t, addSheetT(src, "Report"), "A1", "x")

	dst := Create()
	setCell(t, addSheetT(dst, "Report"), "A1", "orig")

	newSheet, err := dst.CopySheetFrom(src, "Report")
	if err != nil {
		t.Fatalf("CopySheetFrom: %v", err)
	}
	if newSheet.Name() == "Report" {
		t.Fatalf("expected a de-duplicated sheet name, got %q", newSheet.Name())
	}
	if got := dst.SheetCount(); got != 2 {
		t.Fatalf("SheetCount = %d, want 2", got)
	}
	if r := dst.Validate(); r.HasErrors() {
		t.Fatalf("Validate: %v", r)
	}
}

func TestCopySheetFromMissing(t *testing.T) {
	src := Create()
	setCell(t, addSheetT(src, "A"), "A1", "x")
	dst := Create()
	setCell(t, addSheetT(dst, "B"), "A1", "y")
	if _, err := dst.CopySheetFrom(src, "Nope"); err != ErrSheetNotFound {
		t.Fatalf("err = %v, want ErrSheetNotFound", err)
	}
}

func TestCopySheetFromNil(t *testing.T) {
	dst := Create()
	setCell(t, addSheetT(dst, "A"), "A1", "x")
	if _, err := dst.CopySheetFrom(nil, "A"); err != ErrNilWorkbook {
		t.Fatalf("err = %v, want ErrNilWorkbook", err)
	}
}
