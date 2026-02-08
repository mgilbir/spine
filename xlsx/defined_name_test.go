package xlsx

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAddDefinedName(t *testing.T) {
	wb := Create()
	wb.AddSheet("Sheet1")

	if err := wb.AddDefinedName("TotalRevenue", "Sheet1!$B$100"); err != nil {
		t.Fatal(err)
	}

	names := wb.DefinedNames()
	if len(names) != 1 {
		t.Fatalf("expected 1 defined name, got %d", len(names))
	}
	if names[0].Name != "TotalRevenue" {
		t.Errorf("Name = %s, want TotalRevenue", names[0].Name)
	}
	if names[0].Value != "Sheet1!$B$100" {
		t.Errorf("Value = %s, want Sheet1!$B$100", names[0].Value)
	}
	if names[0].SheetIndex != -1 {
		t.Errorf("SheetIndex = %d, want -1", names[0].SheetIndex)
	}
}

func TestAddDefinedNameScoped(t *testing.T) {
	wb := Create()
	wb.AddSheet("Sheet1")
	wb.AddSheet("Sheet2")

	if err := wb.AddDefinedNameScoped("LocalName", "Sheet2!$A$1:$C$10", 1); err != nil {
		t.Fatal(err)
	}

	names := wb.DefinedNames()
	if len(names) != 1 {
		t.Fatalf("expected 1 defined name, got %d", len(names))
	}
	if names[0].SheetIndex != 1 {
		t.Errorf("SheetIndex = %d, want 1", names[0].SheetIndex)
	}
}

func TestAddDefinedNameScopedInvalidIndex(t *testing.T) {
	wb := Create()
	wb.AddSheet("Sheet1")

	err := wb.AddDefinedNameScoped("Bad", "Sheet1!$A$1", 5)
	if err != ErrSheetIndex {
		t.Errorf("expected ErrSheetIndex, got %v", err)
	}
}

func TestMultipleDefinedNames(t *testing.T) {
	wb := Create()
	wb.AddSheet("Sheet1")

	_ = wb.AddDefinedName("Name1", "Sheet1!$A$1")
	_ = wb.AddDefinedName("Name2", "Sheet1!$B$1")
	_ = wb.AddDefinedNameScoped("Name3", "Sheet1!$C$1", 0)

	names := wb.DefinedNames()
	if len(names) != 3 {
		t.Fatalf("expected 3 defined names, got %d", len(names))
	}
}

func TestDefinedNameSaveAndReopen(t *testing.T) {
	wb := Create()
	sheet := wb.AddSheet("Data")

	cell, _ := sheet.Cell("B100")
	cell.SetValue(42195.50)

	if err := wb.AddDefinedName("TotalRevenue", "Data!$B$100"); err != nil {
		t.Fatal(err)
	}

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "defined_names.xlsx")
	if err := wb.Save(path); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() == 0 {
		t.Fatal("output file is empty")
	}

	// Reopen and verify
	wb2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer wb2.Close() //nolint:errcheck

	names := wb2.DefinedNames()
	if len(names) != 1 {
		t.Fatalf("expected 1 defined name, got %d", len(names))
	}
	if names[0].Name != "TotalRevenue" {
		t.Errorf("Name = %s, want TotalRevenue", names[0].Name)
	}
	if names[0].Value != "Data!$B$100" {
		t.Errorf("Value = %s, want Data!$B$100", names[0].Value)
	}
}

func TestNoDefinedNames(t *testing.T) {
	wb := Create()
	wb.AddSheet("Sheet1")

	names := wb.DefinedNames()
	if names != nil {
		t.Errorf("expected nil defined names, got %v", names)
	}
}

func TestDefinedNameRoundTrip(t *testing.T) {
	wb := Create()
	wb.AddSheet("Sheet1")
	_ = wb.AddDefinedName("MyRange", "Sheet1!$A$1:$D$10")

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "dn_roundtrip.xlsx")
	if err := wb.Save(path); err != nil {
		t.Fatal(err)
	}

	wb2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer wb2.Close() //nolint:errcheck

	// Save again
	path2 := filepath.Join(tmpDir, "dn_roundtrip2.xlsx")
	if err := wb2.Save(path2); err != nil {
		t.Fatal(err)
	}

	// Reopen and verify
	wb3, err := Open(path2)
	if err != nil {
		t.Fatal(err)
	}
	defer wb3.Close() //nolint:errcheck

	names := wb3.DefinedNames()
	if len(names) != 1 {
		t.Fatalf("expected 1 defined name after round-trip, got %d", len(names))
	}
	if names[0].Name != "MyRange" {
		t.Errorf("Name = %s, want MyRange", names[0].Name)
	}

	info, _ := os.Stat(path2)
	if info.Size() == 0 {
		t.Fatal("round-trip file is empty")
	}
}
