package xlsx

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mgilbir/spine/internal/testutil"
)

// testXlsxFiles contains all XLSX files to test for round-trip fidelity.
var testXlsxFiles = []struct {
	name              string
	path              string
	description       string
	skipByteIdentical bool // true if byte-identical round-trip is not expected
}{
	{
		name:        "minimal",
		path:        "testdata/minimal.xlsx",
		description: "Minimal XLSX with 2 sheets and basic cell types",
	},
	{
		name:        "world_bank",
		path:        "testdata/world_bank.xlsx",
		description: "World Bank country classification dataset",
	},
	{
		name:        "excelize_test",
		path:        "testdata/excelize_test.xlsx",
		description: "Excelize library test file",
	},
	{
		name:        "excelize_test2",
		path:        "testdata/excelize_test2.xlsx",
		description: "Excelize library test file 2",
	},
	{
		name:        "excelize_test3",
		path:        "testdata/excelize_test3.xlsx",
		description: "Excelize library test file 3 (merge cells)",
	},
	{
		name:        "financial_sample",
		path:        "testdata/financial_sample.xlsx",
		description: "Financial sample dataset",
	},
	{
		name:        "fred_data",
		path:        "testdata/fred_data.xlsx",
		description: "FRED economic data export",
	},
	{
		name:        "abs_australia",
		path:        "testdata/abs_australia.xlsx",
		description: "Australian Bureau of Statistics dataset",
	},
}

// TestCreateAndReopen verifies that creating, saving, and reopening a workbook
// preserves its structural content.
func TestCreateAndReopen(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "created.xlsx")

	// Create a new workbook
	wb := Create()
	s1 := wb.AddSheet("Sheet1")
	if err := s1.SetCellValue("A1", "Hello"); err != nil {
		t.Fatalf("SetCellValue error: %v", err)
	}
	if err := s1.SetCellValue("B1", 42); err != nil {
		t.Fatalf("SetCellValue error: %v", err)
	}
	if err := s1.SetCellValue("A2", true); err != nil {
		t.Fatalf("SetCellValue error: %v", err)
	}
	if err := s1.SetCellValue("B2", 3.14); err != nil {
		t.Fatalf("SetCellValue error: %v", err)
	}

	s2 := wb.AddSheet("Sheet2")
	if err := s2.SetCellValue("A1", "World"); err != nil {
		t.Fatalf("SetCellValue error: %v", err)
	}

	if err := wb.Save(tmpFile); err != nil {
		t.Fatalf("Failed to save: %v", err)
	}

	// Reopen
	wb2, err := Open(tmpFile)
	if err != nil {
		t.Fatalf("Failed to reopen: %v", err)
	}
	defer func() { _ = wb2.Close() }()

	if wb2.SheetCount() != 2 {
		t.Fatalf("Expected 2 sheets, got %d", wb2.SheetCount())
	}

	sheet1, err := wb2.Sheet(0)
	if err != nil {
		t.Fatalf("Failed to get sheet 0: %v", err)
	}
	if sheet1.Name() != "Sheet1" {
		t.Errorf("Sheet 0 name = %q, want %q", sheet1.Name(), "Sheet1")
	}

	val, err := sheet1.GetCellValue("A1")
	if err != nil {
		t.Fatalf("Failed to get A1: %v", err)
	}
	if val != "Hello" {
		t.Errorf("A1 = %q, want %q", val, "Hello")
	}

	val, err = sheet1.GetCellValue("B1")
	if err != nil {
		t.Fatalf("Failed to get B1: %v", err)
	}
	if val != "42" {
		t.Errorf("B1 = %q, want %q", val, "42")
	}

	sheet2, err := wb2.Sheet(1)
	if err != nil {
		t.Fatalf("Failed to get sheet 1: %v", err)
	}
	if sheet2.Name() != "Sheet2" {
		t.Errorf("Sheet 1 name = %q, want %q", sheet2.Name(), "Sheet2")
	}

	val, err = sheet2.GetCellValue("A1")
	if err != nil {
		t.Fatalf("Failed to get Sheet2 A1: %v", err)
	}
	if val != "World" {
		t.Errorf("Sheet2 A1 = %q, want %q", val, "World")
	}
}

// TestRoundTrip tests that opening and saving XLSX files produces valid output
// that can be re-opened and has the same structure.
func TestRoundTrip(t *testing.T) {
	for _, tc := range testXlsxFiles {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := os.Stat(tc.path); os.IsNotExist(err) {
				t.Skip("Test file not found:", tc.path)
			}

			// Open the original file
			w1, err := Open(tc.path)
			if err != nil {
				t.Fatalf("Failed to open original %s: %v", tc.path, err)
			}

			// Capture original structure
			origSheetCount := w1.SheetCount()
			origSheetNames := make([]string, origSheetCount)
			for i, s := range w1.Sheets() {
				origSheetNames[i] = s.Name()
			}

			// Save to a temp file
			tmpFile := filepath.Join(t.TempDir(), "roundtrip.xlsx")
			if err := w1.Save(tmpFile); err != nil {
				_ = w1.Close()
				t.Fatalf("Failed to save: %v", err)
			}
			_ = w1.Close()

			// Re-open the saved file to verify it's valid
			w2, err := Open(tmpFile)
			if err != nil {
				t.Fatalf("Failed to re-open saved file: %v", err)
			}
			defer func() { _ = w2.Close() }()

			// Verify structure is preserved
			if w2.SheetCount() != origSheetCount {
				t.Errorf("Sheet count changed: got %d, want %d", w2.SheetCount(), origSheetCount)
			}
			for i, s := range w2.Sheets() {
				if i < len(origSheetNames) && s.Name() != origSheetNames[i] {
					t.Errorf("Sheet %d name changed: got %q, want %q", i, s.Name(), origSheetNames[i])
				}
			}
		})
	}
}

// TestRoundTripByteIdentical tests byte-for-byte round-trip fidelity.
// Every part in the original XLSX must appear in the round-tripped output
// with identical content.
func TestRoundTripByteIdentical(t *testing.T) {
	for _, tc := range testXlsxFiles {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := os.Stat(tc.path); os.IsNotExist(err) {
				t.Skip("Test file not found:", tc.path)
			}
			if tc.skipByteIdentical {
				t.Skip("Byte-identical round-trip not expected:", tc.description)
			}

			// Open the original file
			w, err := Open(tc.path)
			if err != nil {
				t.Fatalf("Failed to open %s: %v", tc.path, err)
			}

			// Save to a temp file
			tmpFile := filepath.Join(t.TempDir(), "roundtrip.xlsx")
			if err := w.Save(tmpFile); err != nil {
				_ = w.Close()
				t.Fatalf("Failed to save: %v", err)
			}
			_ = w.Close()

			// Compare the two files
			missing, extra, changed := testutil.CompareZipFiles(t, tc.path, tmpFile)

			if len(missing) > 0 {
				t.Errorf("%d missing parts:", len(missing))
				for _, name := range missing {
					t.Errorf("  MISSING: %s", name)
				}
			}
			if len(extra) > 0 {
				t.Errorf("%d extra parts:", len(extra))
				for _, name := range extra {
					t.Errorf("  EXTRA: %s", name)
				}
			}
			if len(changed) > 0 {
				origParts, _ := testutil.ReadZipParts(tc.path)
				rtParts, _ := testutil.ReadZipParts(tmpFile)
				t.Errorf("%d changed parts:", len(changed))
				for _, name := range changed {
					origSize := len(origParts[name])
					rtSize := len(rtParts[name])
					t.Errorf("  CHANGED: %s (%d -> %d bytes)", name, origSize, rtSize)
					testutil.ShowDiff(t, name, origParts[name], rtParts[name])
				}
			}
		})
	}
}

// TestCellRef tests the CellRef function.
func TestCellRef(t *testing.T) {
	tests := []struct {
		row, col int
		want     string
		wantErr  bool
	}{
		{1, 1, "A1", false},
		{1, 2, "B1", false},
		{1, 26, "Z1", false},
		{1, 27, "AA1", false},
		{1, 702, "ZZ1", false},
		{10, 3, "C10", false},
		{100, 1, "A100", false},
		{0, 1, "", true},
		{1, 0, "", true},
	}

	for _, tt := range tests {
		got, err := CellRef(tt.row, tt.col)
		if tt.wantErr {
			if err == nil {
				t.Errorf("CellRef(%d, %d) = %q, want error", tt.row, tt.col, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("CellRef(%d, %d) error: %v", tt.row, tt.col, err)
			continue
		}
		if got != tt.want {
			t.Errorf("CellRef(%d, %d) = %q, want %q", tt.row, tt.col, got, tt.want)
		}
	}
}

// TestParseCellRef tests the ParseCellRef function.
func TestParseCellRef(t *testing.T) {
	tests := []struct {
		ref          string
		wantRow      int
		wantCol      int
		wantErr      bool
	}{
		{"A1", 1, 1, false},
		{"B1", 1, 2, false},
		{"Z1", 1, 26, false},
		{"AA1", 1, 27, false},
		{"ZZ1", 1, 702, false},
		{"C10", 10, 3, false},
		{"A100", 100, 1, false},
		{"", 0, 0, true},
		{"1", 0, 0, true},
		{"A", 0, 0, true},
	}

	for _, tt := range tests {
		row, col, err := ParseCellRef(tt.ref)
		if tt.wantErr {
			if err == nil {
				t.Errorf("ParseCellRef(%q) = (%d, %d), want error", tt.ref, row, col)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseCellRef(%q) error: %v", tt.ref, err)
			continue
		}
		if row != tt.wantRow || col != tt.wantCol {
			t.Errorf("ParseCellRef(%q) = (%d, %d), want (%d, %d)", tt.ref, row, col, tt.wantRow, tt.wantCol)
		}
	}
}
