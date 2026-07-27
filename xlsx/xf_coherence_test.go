package xlsx

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// C440 — AddSheet no longer rewrites the caller's sheet name.
// ---------------------------------------------------------------------------

func TestAddSheetRejectsInvalidAndDuplicateNames(t *testing.T) {
	longName := strings.Repeat("x", 32)

	for _, tc := range []struct {
		name string
		arg  string
	}{
		{"forbidden characters", `Bad[Name]:x`},
		{"too long", longName},
		{"empty", ""},
		{"leading apostrophe", "'quoted"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wb := Create()
			got, err := wb.AddSheet(tc.arg)
			if err == nil {
				t.Fatalf("AddSheet(%q) accepted an illegal name and produced %q; "+
					"ValidateSheetName rejects it and Sheet.SetName rejects it, so AddSheet "+
					"must too (C440)", tc.arg, got.Name())
			}
			if wb.SheetCount() != 0 {
				t.Errorf("AddSheet(%q) failed but still added %d sheet(s)", tc.arg, wb.SheetCount())
			}
		})
	}

	t.Run("duplicate", func(t *testing.T) {
		wb := Create()
		if _, err := wb.AddSheet("Data"); err != nil {
			t.Fatal(err)
		}
		got, err := wb.AddSheet("Data")
		if !errors.Is(err, ErrDuplicateSheetName) {
			t.Fatalf("second AddSheet(%q): got (%v, %v), want ErrDuplicateSheetName", "Data", got, err)
		}
		// Case-insensitively, as Excel compares them.
		if _, err := wb.AddSheet("DATA"); !errors.Is(err, ErrDuplicateSheetName) {
			t.Errorf(`AddSheet("DATA") after "Data": got %v, want ErrDuplicateSheetName`, err)
		}
		if wb.SheetCount() != 1 {
			t.Errorf("workbook has %d sheets after two rejected adds, want 1", wb.SheetCount())
		}
	})

	// The symptom the finding leads with: the sheet you asked for is not the
	// sheet you can find afterwards.
	t.Run("accepted name is findable", func(t *testing.T) {
		wb := Create()
		s, err := wb.AddSheet("Data")
		if err != nil {
			t.Fatal(err)
		}
		found, err := wb.SheetByName("Data")
		if err != nil {
			t.Fatalf("SheetByName(%q): %v", "Data", err)
		}
		if found != s {
			t.Error("SheetByName returned a different sheet than AddSheet")
		}
	})
}

// TestUniqueSheetNameCoercesExplicitly: the coercion AddSheet used to apply
// silently is still available, now as something the caller asks for by name.
func TestUniqueSheetNameCoercesExplicitly(t *testing.T) {
	wb := Create()
	if _, err := wb.AddSheet(wb.UniqueSheetName("Data")); err != nil {
		t.Fatal(err)
	}
	second := wb.UniqueSheetName("Data")
	if second == "Data" {
		t.Fatal("UniqueSheetName returned a name that is already taken")
	}
	s2, err := wb.AddSheet(second)
	if err != nil {
		t.Fatalf("AddSheet(%q): %v", second, err)
	}
	if s2.Name() != second {
		t.Errorf("sheet name = %q, want %q", s2.Name(), second)
	}

	// Illegal characters are stripped and the result is a name AddSheet accepts.
	derived := wb.UniqueSheetName(`a[b]c*?/\:` + strings.Repeat("z", 40))
	if err := ValidateSheetName(derived); err != nil {
		t.Errorf("UniqueSheetName produced an invalid name %q: %v", derived, err)
	}
	if _, err := wb.AddSheet(derived); err != nil {
		t.Errorf("AddSheet(UniqueSheetName(...)) = %v, want nil", err)
	}
}

// CopySheetFrom still promises a unique name with a suffix, so making AddSheet
// strict must not have broken it.
func TestCopySheetFromStillUniquifies(t *testing.T) {
	src := Create()
	s := addSheetT(src, "Data")
	if err := s.SetCellValue("A1", "from source"); err != nil {
		t.Fatal(err)
	}

	dst := Create()
	addSheetT(dst, "Data")

	copied, err := dst.CopySheetFrom(src, "Data")
	if err != nil {
		t.Fatalf("CopySheetFrom: %v", err)
	}
	if copied.Name() == "Data" {
		t.Error("CopySheetFrom reused the taken name")
	}
	got, err := copied.CellValue("A1")
	if err != nil {
		t.Fatal(err)
	}
	if got != "from source" {
		t.Errorf("copied A1 = %q, want %q", got, "from source")
	}
}

// ---------------------------------------------------------------------------
// C569 — the whole-file merge xlsx was missing.
// ---------------------------------------------------------------------------

func TestAppendSheetsFrom(t *testing.T) {
	src := Create()
	for _, name := range []string{"Alpha", "Beta"} {
		s := addSheetT(src, name)
		if err := s.SetCellValue("A1", name); err != nil {
			t.Fatal(err)
		}
	}

	dst := Create()
	addSheetT(dst, "Alpha") // forces the uniquifying path for one of the two

	added, err := dst.AppendSheetsFrom(src)
	if err != nil {
		t.Fatalf("AppendSheetsFrom: %v", err)
	}
	if len(added) != 2 {
		t.Fatalf("appended %d sheets, want 2", len(added))
	}
	if dst.SheetCount() != 3 {
		t.Fatalf("destination has %d sheets, want 3", dst.SheetCount())
	}
	for i, want := range []string{"Alpha", "Beta"} {
		got, err := added[i].CellValue("A1")
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("appended sheet %d A1 = %q, want %q", i, got, want)
		}
	}

	// The guards match docx's Append and pptx's AppendSlidesFrom.
	if _, err := dst.AppendSheetsFrom(nil); !errors.Is(err, ErrNilWorkbook) {
		t.Errorf("AppendSheetsFrom(nil) = %v, want ErrNilWorkbook", err)
	}
	if _, err := dst.AppendSheetsFrom(dst); err == nil {
		t.Error("AppendSheetsFrom(itself) returned no error")
	}
}

// ---------------------------------------------------------------------------
// C570 — Open holds no OS file handle, as docx.Open and pptx.Open do not.
// ---------------------------------------------------------------------------

func TestOpenRetainsNoFileHandle(t *testing.T) {
	wb, err := Open("testdata/minimal.xlsx")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = wb.Close() }()

	if wb.reader == nil {
		t.Fatal("opened workbook has no package reader")
	}
	// opc.ReadCloser.file is the *os.File Open used to hold until Close; a
	// reader built over in-memory bytes leaves it nil. Read it reflectively
	// (unexported in another package) rather than adding an accessor for a
	// test: IsNil does not require Interface().
	f := reflect.ValueOf(wb.reader).Elem().FieldByName("file")
	if !f.IsValid() {
		t.Fatal("opc.ReadCloser no longer has a file field; update this test")
	}
	if !f.IsNil() {
		t.Error("xlsx.Open retained an OS file handle until Close. docx.Open and " +
			"pptx.Open slurp and hold none, so callers trained by those leak a " +
			"descriptor per workbook here (C570). Nothing needs the handle: Open " +
			"already reads every part into memory.")
	}

	// And the workbook is fully usable, i.e. the handle really was surplus.
	if wb.SheetCount() == 0 {
		t.Fatal("opened workbook has no sheets")
	}
	if _, err := wb.SaveBytes(); err != nil {
		t.Fatalf("SaveBytes after a handle-free open: %v", err)
	}
}

// ---------------------------------------------------------------------------
// C565 / C567 — the alias and Get-prefix cleanups behave identically.
// ---------------------------------------------------------------------------

func TestRemoveSheetIsDeleteSheet(t *testing.T) {
	wb := Create()
	addSheetT(wb, "One")
	addSheetT(wb, "Two")
	if err := wb.RemoveSheet(0); err != nil {
		t.Fatalf("RemoveSheet: %v", err)
	}
	if wb.SheetCount() != 1 {
		t.Fatalf("sheet count = %d, want 1", wb.SheetCount())
	}
	if _, err := wb.SheetByName("One"); !errors.Is(err, ErrSheetNotFound) {
		t.Errorf("removed sheet still resolves: %v", err)
	}
	if err := wb.RemoveSheet(9); !errors.Is(err, ErrSheetIndex) {
		t.Errorf("RemoveSheet(out of range) = %v, want ErrSheetIndex", err)
	}
}

func TestGetLessAccessorsMatchTheirGetForms(t *testing.T) {
	wb := Create()
	s := addSheetT(wb, "S")
	if err := s.SetCellValue("A1", "v"); err != nil {
		t.Fatal(err)
	}
	a, err1 := s.CellValue("A1")
	b, err2 := s.GetCellValue("A1") //nolint:staticcheck // deliberately comparing the deprecated form
	if a != b || (err1 == nil) != (err2 == nil) {
		t.Errorf("CellValue = (%q, %v), GetCellValue = (%q, %v)", a, err1, b, err2)
	}

	sm := wb.Styles()
	c1, e1 := sm.CellStyleAt(0)
	c2, e2 := sm.GetCellStyle(0) //nolint:staticcheck // deliberately comparing the deprecated form
	if e1 != nil || e2 != nil {
		t.Fatalf("CellStyleAt/GetCellStyle errors: %v / %v", e1, e2)
	}
	if !reflect.DeepEqual(c1, c2) {
		t.Error("CellStyleAt and GetCellStyle disagree")
	}
}

// ---------------------------------------------------------------------------
// C568 — an impossible lazy-parse failure is loud, not silently empty.
// ---------------------------------------------------------------------------

// TestCorruptedWorksheetModelPanicsRatherThanReadingEmpty simulates the state
// the finding is about: the sheet bytes Open validated (loadSheets parses every
// non-opaque sheet up front and discards the model) are no longer parseable
// when the lazy parse runs. Returning nil there reads as a sheet with no cells
// and writes one back on save — invisible data loss. docx has always panicked
// with a diagnostic for the identical state; xlsx and pptx now do too.
func TestCorruptedWorksheetModelPanicsRatherThanReadingEmpty(t *testing.T) {
	wb, err := Open("testdata/minimal.xlsx")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = wb.Close() }()

	s, err := wb.Sheet(0)
	if err != nil {
		t.Fatal(err)
	}
	// Force the impossible state directly: the only way to reach it in a real
	// process is memory corruption, which a test cannot stage.
	s.wsParsed = true
	s.wsModel = nil
	s.wsParseErr = errors.New("simulated in-memory corruption")

	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("ws() returned silently for an unparseable worksheet; a nil model " +
				"reads as an empty sheet and is written back that way (C568)")
		}
		msg, _ := r.(string)
		if !strings.Contains(msg, s.partName) {
			t.Errorf("panic message does not name the part: %v", r)
		}
	}()
	_ = s.ws()
}
