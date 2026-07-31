package xlsx

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mgilbir/spine/common/crypto"
	"github.com/mgilbir/spine/opc"
)

// This file covers three unexercised inverse/wrapper operations:
//
//   - Cell.Clear, whose failure mode is a partial clear: dropping the value but
//     leaving the type behind (an empty cell still typed t="s" indexes into the
//     shared string table), or nilling a shared-formula master and leaving its
//     followers as si-only stubs with no master anywhere, which is the C176
//     repair-prompt bug.
//   - Sheet.ClearPrintTitles, where the in-memory getter cannot distinguish
//     "removed" from "written empty", so the assertions are on the serialized
//     workbook part.
//   - Open with a password, and SaveEncrypted: the path wrappers around the
//     tested reader and writer cores.

// clearFixtureSheet carries one cell of each shape Clear has to reset: an
// inline string (Is), a formula with a cached value (F+V), a typed boolean, and
// a plain number.
const clearFixtureSheet = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
	`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData>` +
	`<row r="1">` +
	`<c r="A1" t="inlineStr"><is><t>hello</t></is></c>` +
	`<c r="B1"><f>1+1</f><v>2</v></c>` +
	`<c r="C1" t="b"><v>1</v></c>` +
	`<c r="D1"><v>7</v></c>` +
	`</row></sheetData></worksheet>`

// Clear resets every part of a cell — value, formula, type and inline string —
// and the result survives the save: a cleared cell must not keep a type that
// reinterprets the emptiness (t="s" indexes the shared string table, t="b"
// makes it FALSE), nor a stale cached formula value.
func TestCellClearResetsValueTypeAndFormula(t *testing.T) {
	data := buildMutatorTestXlsx(t, clearFixtureSheet)
	w, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = w.Close() }()
	s := w.Sheets()[0]

	for _, ref := range []string{"A1", "B1", "C1", "D1"} {
		c, err := s.Cell(ref)
		if err != nil {
			t.Fatalf("Cell(%s): %v", ref, err)
		}
		if c.IsEmpty() {
			t.Fatalf("%s is empty before Clear, so the fixture proves nothing", ref)
		}
		c.Clear()
		if !c.IsEmpty() {
			t.Errorf("%s: IsEmpty() = false after Clear", ref)
		}
	}

	out, err := w.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	sheetXML := string(readZipPart(t, out, "xl/worksheets/sheet1.xml"))
	for _, leftover := range []string{`t="inlineStr"`, `<is>`, `<f>`, `t="b"`, `<v>`} {
		if strings.Contains(sheetXML, leftover) {
			t.Errorf("cleared cells still serialize %s:\n%s", leftover, sheetXML)
		}
	}

	re, err := OpenReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer func() { _ = re.Close() }()
	for _, ref := range []string{"A1", "B1", "C1", "D1"} {
		if got, _ := re.Sheets()[0].GetCellValue(ref); got != "" {
			t.Errorf("%s after Clear + save/reopen = %q, want empty", ref, got)
		}
	}
}

// Clear must not touch its neighbours: the cells around the cleared one keep
// their values, types and formulas.
func TestCellClearLeavesOtherCellsAlone(t *testing.T) {
	data := buildMutatorTestXlsx(t, clearFixtureSheet)
	w, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = w.Close() }()
	s := w.Sheets()[0]

	c, err := s.Cell("B1")
	if err != nil {
		t.Fatalf("Cell: %v", err)
	}
	c.Clear()

	re, _ := saveReopenSheetBytes(t, w)
	if got, _ := re.GetCellValue("A1"); got != "hello" {
		t.Errorf("A1 = %q after clearing B1, want hello", got)
	}
	if got, _ := re.GetCellValue("D1"); got != "7" {
		t.Errorf("D1 = %q after clearing B1, want 7", got)
	}
}

// Clearing the master of a shared-formula group must materialize its followers
// as plain formulas first. Nilling the master alone would leave `<f t="shared"
// si="0"/>` stubs pointing at a master that no longer exists — spec-invalid,
// and Excel opens the file with a repair prompt (C176).
func TestCellClearOnSharedFormulaMasterMaterializesFollowers(t *testing.T) {
	data := buildMutatorTestXlsx(t, sharedFormulaSheet)
	w, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer func() { _ = w.Close() }()

	c, err := w.Sheets()[0].Cell("B1")
	if err != nil {
		t.Fatalf("Cell: %v", err)
	}
	c.Clear()

	out, err := w.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	sheetXML := string(readZipPart(t, out, "xl/worksheets/sheet1.xml"))
	if strings.Contains(sheetXML, `t="shared"`) || strings.Contains(sheetXML, `si="0"`) {
		t.Errorf("shared-formula bookkeeping survived Clear of the master:\n%s", sheetXML)
	}
	// The follower with an empty stub gets the master's formula translated by
	// one row; the one that carried its own text keeps it.
	if !strings.Contains(sheetXML, `<c r="B2"><f>A2*2</f>`) {
		t.Errorf("follower B2 was not materialized by Clear:\n%s", sheetXML)
	}
	if !strings.Contains(sheetXML, `<c r="B3"><f>A3*2</f>`) {
		t.Errorf("follower B3 lost its formula:\n%s", sheetXML)
	}
	re, err := OpenReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer func() { _ = re.Close() }()
	if got, _ := re.Sheets()[0].GetCellValue("B1"); got != "" {
		t.Errorf("cleared master B1 = %q, want empty", got)
	}
}

// ---------------------------------------------------------------------------
// ClearPrintTitles
// ---------------------------------------------------------------------------

// ClearPrintTitles removes the sheet's _xlnm.Print_Titles defined name from the
// workbook part rather than blanking its value: an empty defined name is a
// different document, and PrintTitles() == "" cannot tell the two apart. It
// must also be scoped — clearing one sheet's titles leaves the other sheet's
// titles and any user-defined name in place.
func TestClearPrintTitlesRemovesTheDefinedName(t *testing.T) {
	w := Create()
	defer func() { _ = w.Close() }()
	s1 := addSheetT(w, "First")
	s2 := addSheetT(w, "Second")

	if err := s1.SetPrintTitles("$1:$1", "$A:$A"); err != nil {
		t.Fatalf("SetPrintTitles: %v", err)
	}
	if err := s2.SetPrintTitles("$2:$2", ""); err != nil {
		t.Fatalf("SetPrintTitles: %v", err)
	}
	if err := w.AddDefinedName("Rates", "First!$B$1"); err != nil {
		t.Fatalf("AddDefinedName: %v", err)
	}
	if s1.PrintTitles() == "" {
		t.Fatal("PrintTitles empty before the clear, so the fixture proves nothing")
	}

	before, err := w.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	if n := strings.Count(string(readZipPart(t, before, "xl/workbook.xml")), printTitlesName); n != 2 {
		t.Fatalf("workbook has %d Print_Titles names before the clear, want 2", n)
	}

	s1.ClearPrintTitles()

	if got := s1.PrintTitles(); got != "" {
		t.Errorf("PrintTitles() after ClearPrintTitles = %q, want empty", got)
	}
	if got := s2.PrintTitles(); got == "" {
		t.Error("clearing the first sheet's print titles also cleared the second sheet's")
	}

	out, err := w.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	wbXML := string(readZipPart(t, out, "xl/workbook.xml"))
	if n := strings.Count(wbXML, printTitlesName); n != 1 {
		t.Errorf("workbook has %d Print_Titles names after clearing one, want 1:\n%s", n, wbXML)
	}
	if !strings.Contains(wbXML, `localSheetId="1"`) {
		t.Errorf("the surviving Print_Titles lost its sheet scope:\n%s", wbXML)
	}
	if !strings.Contains(wbXML, `name="Rates"`) {
		t.Errorf("ClearPrintTitles removed an unrelated defined name:\n%s", wbXML)
	}

	re, err := OpenReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer func() { _ = re.Close() }()
	if got := re.Sheets()[0].PrintTitles(); got != "" {
		t.Errorf("reopened first sheet PrintTitles = %q, want empty", got)
	}
	if got := re.Sheets()[1].PrintTitles(); got == "" {
		t.Error("reopened second sheet lost its print titles")
	}
}

// Clearing the only defined name leaves no empty <definedNames> container
// behind, and clearing titles that were never set is a no-op.
func TestClearPrintTitlesLeavesNoEmptyContainer(t *testing.T) {
	w := Create()
	defer func() { _ = w.Close() }()
	s := addSheetT(w, "Only")

	s.ClearPrintTitles() // never set: must not panic or invent anything

	if err := s.SetPrintTitles("$1:$1", ""); err != nil {
		t.Fatalf("SetPrintTitles: %v", err)
	}
	s.ClearPrintTitles()

	out, err := w.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	wbXML := string(readZipPart(t, out, "xl/workbook.xml"))
	if strings.Contains(wbXML, "definedName") {
		t.Errorf("an empty definedNames container survived the clear:\n%s", wbXML)
	}
}

// ---------------------------------------------------------------------------
// OpenEncrypted / SaveEncrypted (the file-path wrappers)
// ---------------------------------------------------------------------------

// SaveEncrypted writes a real encrypted container to the path and OpenEncrypted
// reads it back with the same content. The file must be a CFB container, not a
// plain zip: a wrapper that fell through to the unencrypted writer would still
// round-trip through Open and only be caught by looking at the bytes.
func TestSaveOpenEncryptedFilePathRoundTrip(t *testing.T) {
	w := Create()
	s := addSheetT(w, "Secret")
	if err := s.SetCellValue("A1", "classified"); err != nil {
		t.Fatalf("SetCellValue: %v", err)
	}
	path := filepath.Join(t.TempDir(), "enc.xlsx")
	if err := w.SaveEncrypted(path, "pa55word"); err != nil {
		t.Fatalf("SaveEncrypted: %v", err)
	}
	_ = w.Close()

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if len(raw) < 8 || !bytes.Equal(raw[:8], []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}) {
		t.Fatalf("saved file is not a CFB container (first bytes %x)", raw[:min(8, len(raw))])
	}
	if bytes.HasPrefix(raw, []byte("PK")) {
		t.Fatal("SaveEncrypted wrote a plain zip")
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm&0o200 == 0 || perm&0o111 != 0 {
		t.Errorf("saved file mode = %v, want owner-writable and non-executable", perm)
	}

	re, err := Open(path, opc.WithPassword("pa55word"))
	if err != nil {
		t.Fatalf("Open with a password: %v", err)
	}
	defer func() { _ = re.Close() }()
	if got, _ := re.Sheets()[0].GetCellValue("A1"); got != "classified" {
		t.Errorf("A1 through the encrypted round trip = %q, want classified", got)
	}
	if name := re.Sheets()[0].Name(); name != "Secret" {
		t.Errorf("sheet name through the encrypted round trip = %q, want Secret", name)
	}
}

// The wrong password is reported as crypto.ErrWrongPassword rather than a parse
// failure, and a missing path surfaces the filesystem error instead of a nil
// workbook.
func TestOpenEncryptedPathErrors(t *testing.T) {
	w := Create()
	s := addSheetT(w, "S")
	if err := s.SetCellValue("A1", 1); err != nil {
		t.Fatalf("SetCellValue: %v", err)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "enc.xlsx")
	if err := w.SaveEncrypted(path, "right"); err != nil {
		t.Fatalf("SaveEncrypted: %v", err)
	}
	_ = w.Close()

	if _, err := Open(path, opc.WithPassword("wrong")); !errors.Is(err, crypto.ErrWrongPassword) {
		t.Errorf("Open with the wrong password: err = %v, want crypto.ErrWrongPassword", err)
	}
	if _, err := Open(path); !errors.Is(err, opc.ErrEncrypted) {
		t.Errorf("Open of an encrypted file without a password: err = %v, want opc.ErrEncrypted", err)
	}
	missing := filepath.Join(dir, "nope.xlsx")
	wb, err := Open(missing, opc.WithPassword("right"))
	if err == nil {
		t.Error("Open on a missing path returned no error")
		_ = wb.Close()
	} else if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Open on a missing path: err = %v, want a not-exist error", err)
	}
	if wb != nil {
		t.Error("Open returned a workbook alongside an error")
	}
}

// A rejected SaveEncrypted must not leave a file behind: the encryption failure
// has to be decided before anything is written, or the caller is left with a
// truncated or plaintext file at the destination path.
func TestSaveEncryptedFailureWritesNoFile(t *testing.T) {
	w := Create()
	defer func() { _ = w.Close() }()
	s := addSheetT(w, "S")
	if err := s.SetCellValue("A1", 1); err != nil {
		t.Fatalf("SetCellValue: %v", err)
	}
	path := filepath.Join(t.TempDir(), "empty-password.xlsx")
	if err := w.SaveEncrypted(path, ""); err == nil {
		t.Fatal("SaveEncrypted with an empty password returned no error")
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("a rejected SaveEncrypted left a file at the destination (stat err = %v)", err)
	}
}
