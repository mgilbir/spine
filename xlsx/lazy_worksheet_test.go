package xlsx

import (
	"bytes"
	"testing"
)

// Opening a workbook must not eagerly build a worksheet model per sheet: the
// model is parsed lazily on first access, so a workbook that is inspected
// lightly or round-tripped unmodified holds only the raw sheet bytes. This
// pins that behavior (a regression would reintroduce the large-workbook memory
// cost).
func TestWorksheetParsedLazily(t *testing.T) {
	wb := Create()
	sh := addSheetT(wb, "Data")
	if err := sh.SetCellValue("A1", "hello"); err != nil {
		t.Fatalf("SetCellValue: %v", err)
	}
	data, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	reopened, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	s, err := reopened.Sheet(0)
	if err != nil {
		t.Fatalf("Sheet(0): %v", err)
	}
	// Not yet parsed: the model is nil until an accessor materializes it.
	if s.wsModel != nil {
		t.Errorf("worksheet model built eagerly at open; want lazy (nil until accessed)")
	}
	// A read accessor triggers the lazy parse and returns the real content.
	got, err := s.GetCellValue("A1")
	if err != nil {
		t.Fatalf("GetCellValue: %v", err)
	}
	if got != "hello" {
		t.Errorf("GetCellValue(A1) = %q, want %q", got, "hello")
	}
	if s.wsModel == nil {
		t.Errorf("worksheet model still nil after accessing a cell; lazy parse did not run")
	}
	// Accessing (parsing) an unmodified sheet must not mark it dirty, so it
	// still round-trips from its raw bytes.
	if s.dirty {
		t.Errorf("read access marked the sheet dirty; would force needless regeneration")
	}
}

// A clean (unmodified) round-trip of an opened workbook must produce identical
// bytes whether or not a sheet was accessed: lazy parsing plus the
// raw-passthrough of clean sheets must make read access invisible to the saved
// output. (Byte-identity against real producer files is covered by the corpus
// round-trip sweep; here we pin that access does not perturb the result.)
func TestLazyReadAccessDoesNotPerturbRoundTrip(t *testing.T) {
	wb := Create()
	sh := addSheetT(wb, "S")
	_ = sh.SetCellValue("A1", "x")
	_ = sh.SetCellValue("B2", "y")
	orig, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	// Open and save without touching any sheet.
	r1, err := OpenReader(bytes.NewReader(orig), int64(len(orig)))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	out1, err := r1.SaveBytes()
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	// Open, read a cell (forcing a lazy parse), then save.
	r2, err := OpenReader(bytes.NewReader(orig), int64(len(orig)))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	s2, _ := r2.Sheet(0)
	if got, _ := s2.GetCellValue("A1"); got != "x" {
		t.Fatalf("GetCellValue(A1) = %q, want x", got)
	}
	out2, err := r2.SaveBytes()
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	if !bytes.Equal(out1, out2) {
		t.Errorf("read access perturbed the clean round-trip output (%d vs %d bytes)", len(out1), len(out2))
	}
}
