package xlsx

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// Runtime companions to the AST guards in mutation_flag_guard_test.go. The AST
// guards catch a mutator that forgets to flag; these catch the mirror image —
// a path that flags when nothing changed, so a part that could have been
// preserved is regenerated (C425/C544). Once dcterms:modified is derived from
// these same flags, an over-flagging read becomes a spurious timestamp bump as
// well as needless regeneration.

// zipPartMap returns every part of an OPC package keyed by zip entry name.
func zipPartMap(t *testing.T, data []byte) map[string][]byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("reading package: %v", err)
	}
	out := make(map[string][]byte, len(zr.File))
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("opening %s: %v", f.Name, err)
		}
		b, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("reading %s: %v", f.Name, err)
		}
		out[f.Name] = b
	}
	return out
}

// packageDiff names the parts that differ between two packages: "added:",
// "removed:" or "changed:" per entry, sorted.
func packageDiff(t *testing.T, before, after []byte) []string {
	t.Helper()
	a, b := zipPartMap(t, before), zipPartMap(t, after)
	var diffs []string
	for name, data := range a {
		other, ok := b[name]
		switch {
		case !ok:
			diffs = append(diffs, "removed:"+name)
		case !bytes.Equal(data, other):
			diffs = append(diffs, "changed:"+name)
		}
	}
	for name := range b {
		if _, ok := a[name]; !ok {
			diffs = append(diffs, "added:"+name)
		}
	}
	sort.Strings(diffs)
	return diffs
}

// ---------------------------------------------------------------------------
// Direction 2: reading must not flag
// ---------------------------------------------------------------------------

// accessorMutatorPrefixes name the method prefixes that identify a mutator.
// Everything else on the public surface is treated as a read, and a read must
// leave the saved package byte-for-byte identical.
var accessorMutatorPrefixes = []string{
	"Add", "Append", "Apply", "Clear", "Close", "Copy", "Delete", "Freeze",
	"Group", "Import", "Merge", "Move", "New", "Protect", "Remove", "Replace",
	"Reply", "Resolve", "Save", "Set", "Split", "Unfreeze", "Ungroup",
	"Unmerge", "Unprotect", "Write",
}

func isAccessorName(name string) bool {
	for _, p := range accessorMutatorPrefixes {
		if strings.HasPrefix(name, p) {
			return false
		}
	}
	return true
}

// synthesizeAccessorArg builds a plausible argument of type rt. Cell-reference
// and index shaped parameters get values that actually resolve, so the
// accessors do real work rather than bailing out at the first validation.
func synthesizeAccessorArg(rt reflect.Type) reflect.Value {
	switch rt.Kind() {
	case reflect.String:
		return reflect.ValueOf("A1").Convert(rt)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return reflect.ValueOf(int64(1)).Convert(rt)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return reflect.ValueOf(uint64(1)).Convert(rt)
	case reflect.Float32, reflect.Float64:
		return reflect.ValueOf(1.0).Convert(rt)
	case reflect.Bool:
		return reflect.ValueOf(false).Convert(rt)
	default:
		return reflect.Zero(rt)
	}
}

// callEveryAccessor invokes every exported read-shaped method on v. A newly
// added accessor is picked up automatically, which is the point: this guard
// fails when a NEW read starts dirtying state, not only when a known one does.
func callEveryAccessor(t *testing.T, v reflect.Value, label string) {
	t.Helper()
	rt := v.Type()
	names := make([]string, 0, rt.NumMethod())
	for i := 0; i < rt.NumMethod(); i++ {
		names = append(names, rt.Method(i).Name)
	}
	sort.Strings(names)
	for _, name := range names {
		if !isAccessorName(name) {
			continue
		}
		m := v.MethodByName(name)
		mt := m.Type()
		if mt.IsVariadic() {
			continue
		}
		args := make([]reflect.Value, mt.NumIn())
		for i := range args {
			args[i] = synthesizeAccessorArg(mt.In(i))
		}
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("%s.%s panicked on a read-only call: %v", label, name, r)
				}
			}()
			m.Call(args)
		}()
	}
}

// TestReadOnlyAccessorsPreserveEveryPart opens a package, calls every exported
// read-shaped method on Workbook, Sheet and Cell, then saves. Reading must not
// change one byte of the package.
//
// This reproduced C425's shape on Workbook.Styles: materializing the default
// stylesheet for a package with no styles part set stylesDirty, so a plain
// wb.Styles().NamedStyles() grew xl/styles.xml, a content-type override and a
// workbook relationship out of nothing.
func TestReadOnlyAccessorsPreserveEveryPart(t *testing.T) {
	for _, path := range []string{
		"testdata/minimal.xlsx",
		"testdata/threaded_comments.xlsx",
	} {
		t.Run(path, func(t *testing.T) {
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("reading fixture: %v", err)
			}
			wb, err := OpenReader(bytes.NewReader(src), int64(len(src)))
			if err != nil {
				t.Fatalf("OpenReader: %v", err)
			}
			defer wb.Close() //nolint:errcheck

			// Establish the baseline: this package must already round-trip
			// part-identically, or the assertion below would be vacuous.
			base, err := wb.SaveBytes()
			if err != nil {
				t.Fatalf("baseline save: %v", err)
			}
			if d := packageDiff(t, src, base); len(d) != 0 {
				t.Fatalf("fixture does not round-trip unmodified (%v); the read guard needs a clean baseline", d)
			}

			wb2, err := OpenReader(bytes.NewReader(src), int64(len(src)))
			if err != nil {
				t.Fatalf("OpenReader: %v", err)
			}
			defer wb2.Close() //nolint:errcheck

			callEveryAccessor(t, reflect.ValueOf(wb2), "Workbook")
			for _, sheet := range wb2.Sheets() {
				callEveryAccessor(t, reflect.ValueOf(sheet), "Sheet")
				if c := sheet.FindCell("A1"); c != nil {
					callEveryAccessor(t, reflect.ValueOf(c), "Cell")
				}
				// Cell() materializes by design (C425); it still must not flag.
				if _, err := sheet.Cell("Z99"); err != nil && err != ErrNotWorksheet {
					t.Fatalf("Cell: %v", err)
				}
				if sheet.dirty {
					t.Errorf("sheet %q was marked dirty by read-only accessors", sheet.Name())
				}
			}
			if wb2.stylesDirty {
				t.Error("read-only accessors set stylesDirty")
			}
			if wb2.sheetsDirty {
				t.Error("read-only accessors set sheetsDirty")
			}

			out, err := wb2.SaveBytes()
			if err != nil {
				t.Fatalf("save after accessor sweep: %v", err)
			}
			if d := packageDiff(t, src, out); len(d) != 0 {
				t.Errorf("read-only accessor sweep changed the saved package: %v", d)
			}
		})
	}
}

// TestFailedCellStyleMutatorLeavesSheetClean guards the C544 shape on the two
// cell-style mutators that flagged before validating: a rejected call changed
// nothing yet dirtied the sheet, so an untouched worksheet was regenerated on
// save (and, once modification timestamps are derived from these flags, stamped
// as edited).
func TestFailedCellStyleMutatorLeavesSheetClean(t *testing.T) {
	src, err := os.ReadFile("testdata/minimal.xlsx")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}

	cases := []struct {
		name string
		call func(c *Cell) error
	}{
		{
			name: "SetStyle with an out-of-range rotation",
			call: func(c *Cell) error {
				return c.SetStyle(CellStyle{Alignment: &AlignmentStyle{Rotation: 400}})
			},
		},
		{
			name: "SetStyle with a negative number-format id",
			call: func(c *Cell) error {
				return c.SetStyle(CellStyle{NumberFormatID: -3})
			},
		},
		{
			name: "SetNamedStyle with an unknown name",
			call: func(c *Cell) error { return c.SetNamedStyle("NoSuchStyle") },
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			wb, err := OpenReader(bytes.NewReader(src), int64(len(src)))
			if err != nil {
				t.Fatalf("OpenReader: %v", err)
			}
			defer wb.Close() //nolint:errcheck
			sheet, err := wb.Sheet(0)
			if err != nil {
				t.Fatalf("Sheet(0): %v", err)
			}
			cell, err := sheet.Cell("A1")
			if err != nil {
				t.Fatalf("Cell: %v", err)
			}
			if err := tc.call(cell); err == nil {
				t.Fatal("expected the invalid style to be rejected")
			}
			if sheet.dirty {
				t.Error("a rejected style mutator marked the sheet dirty")
			}
			if wb.stylesDirty {
				t.Error("a rejected style mutator marked styles dirty")
			}
			out, err := wb.SaveBytes()
			if err != nil {
				t.Fatalf("save: %v", err)
			}
			if d := packageDiff(t, src, out); len(d) != 0 {
				t.Errorf("a rejected style mutator changed the saved package: %v", d)
			}
		})
	}
}

// TestStylesAccessorDoesNotAddStylesPart pins the narrower fact behind the
// accessor sweep: reading styles from a package that carries no styles part
// must not grow one.
func TestStylesAccessorDoesNotAddStylesPart(t *testing.T) {
	src, err := os.ReadFile("testdata/minimal.xlsx")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	wb, err := OpenReader(bytes.NewReader(src), int64(len(src)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	defer wb.Close() //nolint:errcheck

	_ = wb.Styles().NamedStyles()
	if _, ok := wb.Styles().NamedStyleXfId("Normal"); ok {
		t.Log("fixture happens to define Normal; the read is still a read")
	}
	if wb.stylesDirty {
		t.Error("reading styles set stylesDirty")
	}
	out, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if d := packageDiff(t, src, out); len(d) != 0 {
		t.Errorf("reading styles changed the saved package: %v", d)
	}

	// The manager must still flag as soon as it actually appends a record.
	wb2, err := OpenReader(bytes.NewReader(src), int64(len(src)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	defer wb2.Close() //nolint:errcheck
	if _, err := wb2.Styles().NewCellStyle(CellStyle{Font: &FontStyle{Bold: true}}); err != nil {
		t.Fatalf("NewCellStyle: %v", err)
	}
	if !wb2.stylesDirty {
		t.Fatal("appending a cell format did not set stylesDirty")
	}
	out2, err := wb2.SaveBytes()
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if got := readZipPart(t, out2, "xl/styles.xml"); len(got) == 0 {
		t.Error("a real style edit did not produce xl/styles.xml")
	}
}

// ---------------------------------------------------------------------------
// Direction 1: mutating must flag — and must persist through save/reopen
// ---------------------------------------------------------------------------

// TestSheetMutatorsPersistThroughSaveReopen is the runtime counterpart of the
// AST guard: every entry mutates an OPENED workbook (the case where an
// unflagged mutator is silently discarded, because a clean sheet round-trips
// from its preserved bytes), saves, reopens and asserts the change survived.
func TestSheetMutatorsPersistThroughSaveReopen(t *testing.T) {
	src, err := os.ReadFile("testdata/minimal.xlsx")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}

	cases := []struct {
		name   string
		mutate func(t *testing.T, s *Sheet)
		verify func(t *testing.T, s *Sheet)
	}{
		{
			name:   "SetCellValue",
			mutate: func(t *testing.T, s *Sheet) { mustNoErr(t, s.SetCellValue("D4", "persisted")) },
			verify: func(t *testing.T, s *Sheet) {
				if got, _ := s.CellValue("D4"); got != "persisted" {
					t.Errorf("D4 = %q, want %q", got, "persisted")
				}
			},
		},
		{
			name:   "MergeCells",
			mutate: func(t *testing.T, s *Sheet) { mustNoErr(t, s.MergeCells("B2", "C3")) },
			verify: func(t *testing.T, s *Sheet) {
				if got := s.MergedCells(); len(got) != 1 || got[0] != "B2:C3" {
					t.Errorf("MergedCells = %v, want [B2:C3]", got)
				}
			},
		},
		{
			name:   "SetColWidth",
			mutate: func(t *testing.T, s *Sheet) { mustNoErr(t, s.SetColWidth(2, 33.5)) },
			verify: func(t *testing.T, s *Sheet) {
				if w, ok := s.ColumnWidth(2); !ok || w != 33.5 {
					t.Errorf("ColumnWidth(2) = %v, %v; want 33.5, true", w, ok)
				}
			},
		},
		{
			name:   "SetRowHeight",
			mutate: func(t *testing.T, s *Sheet) { mustNoErr(t, s.SetRowHeight(1, 42)) },
			verify: func(t *testing.T, s *Sheet) {
				if h, ok := s.RowHeight(1); !ok || h != 42 {
					t.Errorf("RowHeight(1) = %v, %v; want 42, true", h, ok)
				}
			},
		},
		{
			name:   "FreezePanes",
			mutate: func(t *testing.T, s *Sheet) { mustNoErr(t, s.FreezePanes("B2")) },
			verify: func(t *testing.T, s *Sheet) {
				if cols, rows, ok := s.FrozenPanes(); !ok || cols != 1 || rows != 1 {
					t.Errorf("FrozenPanes = %d, %d, %v; want 1, 1, true", cols, rows, ok)
				}
			},
		},
		{
			name:   "SetAutoFilter",
			mutate: func(t *testing.T, s *Sheet) { mustNoErr(t, s.SetAutoFilter("A1:B2")) },
			verify: func(t *testing.T, s *Sheet) {
				if got, ok := s.AutoFilterRange(); !ok || got != "A1:B2" {
					t.Errorf("AutoFilterRange = %q, %v; want A1:B2, true", got, ok)
				}
			},
		},
		{
			name: "Protect",
			mutate: func(t *testing.T, s *Sheet) {
				mustNoErr(t, s.Protect(SheetProtectionOptions{Password: "secret"}))
			},
			verify: func(t *testing.T, s *Sheet) {
				if p := s.Protection(); p == nil || !p.Enabled() || !p.HasPassword() {
					t.Errorf("Protection = %+v, want sheet-protected", p)
				}
			},
		},
		{
			name: "SetPageSetup",
			mutate: func(t *testing.T, s *Sheet) {
				scale := uint32(80)
				mustNoErr(t, s.SetPageSetup(PageSetup{Orientation: "landscape", Scale: &scale}))
			},
			verify: func(t *testing.T, s *Sheet) {
				ps, ok := s.PageSetup()
				if !ok || ps.Orientation != "landscape" || ps.Scale == nil || *ps.Scale != 80 {
					t.Errorf("PageSetup = %+v, %v; want landscape/80", ps, ok)
				}
			},
		},
		{
			name: "SetHeaderFooter",
			mutate: func(t *testing.T, s *Sheet) {
				mustNoErr(t, s.SetHeaderFooter(HeaderFooter{OddHeader: "&Ctitle"}))
			},
			verify: func(t *testing.T, s *Sheet) {
				hf, ok := s.HeaderFooter()
				if !ok || hf.OddHeader != "&Ctitle" {
					t.Errorf("HeaderFooter = %+v, %v; want OddHeader &Ctitle", hf, ok)
				}
			},
		},
		{
			name: "AddDataValidation",
			mutate: func(t *testing.T, s *Sheet) {
				mustNoErr(t, s.AddDataValidation(DataValidation{
					Range: "A1", Type: "list", Formula1: `"a,b"`,
				}))
			},
			verify: func(t *testing.T, s *Sheet) {
				if got := s.DataValidations(); len(got) != 1 {
					t.Errorf("DataValidations = %d entries, want 1", len(got))
				}
			},
		},
		{
			name:   "SetZoom",
			mutate: func(t *testing.T, s *Sheet) { s.SetZoom(150) },
			verify: func(t *testing.T, s *Sheet) {
				if s.ws().SheetViews == nil || len(s.ws().SheetViews.SheetView) == 0 ||
					s.ws().SheetViews.SheetView[0].ZoomScale == nil ||
					*s.ws().SheetViews.SheetView[0].ZoomScale != 150 {
					t.Error("zoom did not survive the round-trip")
				}
			},
		},
		{
			name:   "SetTabColor",
			mutate: func(t *testing.T, s *Sheet) { s.SetTabColor("FF00FF00") },
			verify: func(t *testing.T, s *Sheet) {
				pr := s.ws().SheetPr
				if pr == nil || pr.TabColor == nil || pr.TabColor.Rgb != "FF00FF00" {
					t.Error("tab color did not survive the round-trip")
				}
			},
		},
		{
			name:   "SetRightToLeft",
			mutate: func(t *testing.T, s *Sheet) { s.SetRightToLeft(true) },
			verify: func(t *testing.T, s *Sheet) {
				if !s.RightToLeft() {
					t.Error("rightToLeft did not survive the round-trip")
				}
			},
		},
		{
			name:   "SetOutlineSummary",
			mutate: func(t *testing.T, s *Sheet) { s.SetOutlineSummary(false, false) },
			verify: func(t *testing.T, s *Sheet) {
				below, right := s.OutlineSummary()
				if below || right {
					t.Errorf("OutlineSummary = %v, %v; want false, false", below, right)
				}
			},
		},
		{
			name:   "GroupRows",
			mutate: func(t *testing.T, s *Sheet) { mustNoErr(t, s.GroupRows(1, 2)) },
			verify: func(t *testing.T, s *Sheet) {
				if lvl := s.RowOutlineLevel(1); lvl != 1 {
					t.Errorf("RowOutlineLevel(1) = %d, want 1", lvl)
				}
			},
		},
		{
			name:   "SetName",
			mutate: func(t *testing.T, s *Sheet) { mustNoErr(t, s.SetName("Renamed")) },
			verify: func(t *testing.T, s *Sheet) {
				if s.Name() != "Renamed" {
					t.Errorf("Name = %q, want Renamed", s.Name())
				}
			},
		},
		{
			name:   "SetPrintArea",
			mutate: func(t *testing.T, s *Sheet) { mustNoErr(t, s.SetPrintArea("A1:B2")) },
			verify: func(t *testing.T, s *Sheet) {
				if got := s.PrintArea(); !strings.Contains(got, "$A$1:$B$2") {
					t.Errorf("PrintArea = %q, want it to contain $A$1:$B$2", got)
				}
			},
		},
		{
			name:   "SetVisibility",
			mutate: func(t *testing.T, s *Sheet) { mustNoErr(t, s.SetVisibility(SheetHidden)) },
			verify: func(t *testing.T, s *Sheet) {
				if s.Visibility() != SheetHidden {
					t.Errorf("Visibility = %q, want hidden", s.Visibility())
				}
			},
		},
		{
			name: "Cell.SetHyperlink",
			mutate: func(t *testing.T, s *Sheet) {
				c, err := s.Cell("A1")
				if err != nil {
					t.Fatalf("Cell: %v", err)
				}
				c.SetHyperlink("https://example.com/")
			},
			verify: func(t *testing.T, s *Sheet) {
				links := s.Hyperlinks()
				if len(links) != 1 || links[0].URL() != "https://example.com/" {
					t.Errorf("Hyperlinks = %v, want one link to example.com", links)
				}
			},
		},
		{
			name: "Cell.SetStyle",
			mutate: func(t *testing.T, s *Sheet) {
				c, err := s.Cell("A1")
				if err != nil {
					t.Fatalf("Cell: %v", err)
				}
				if err := c.SetStyle(CellStyle{Font: &FontStyle{Bold: true}}); err != nil {
					t.Fatalf("SetStyle: %v", err)
				}
			},
			verify: func(t *testing.T, s *Sheet) {
				c := s.FindCell("A1")
				if c == nil || c.StyleIndex() == nil || *c.StyleIndex() == 0 {
					t.Error("cell style did not survive the round-trip")
				}
			},
		},
		{
			name: "AddNote",
			mutate: func(t *testing.T, s *Sheet) {
				if s.AddNote("B5", "Ann", "hi") == nil {
					t.Fatal("AddNote returned nil")
				}
			},
			verify: func(t *testing.T, s *Sheet) {
				found := false
				for _, c := range s.Comments() {
					if c.Ref() == "B5" && c.Text() == "hi" {
						found = true
					}
				}
				if !found {
					t.Error("note did not survive the round-trip")
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			wb, err := OpenReader(bytes.NewReader(src), int64(len(src)))
			if err != nil {
				t.Fatalf("OpenReader: %v", err)
			}
			sheet, err := wb.Sheet(0)
			if err != nil {
				t.Fatalf("Sheet(0): %v", err)
			}
			tc.mutate(t, sheet)
			out, err := wb.SaveBytes()
			if err != nil {
				t.Fatalf("save: %v", err)
			}
			_ = wb.Close()

			re, err := OpenReader(bytes.NewReader(out), int64(len(out)))
			if err != nil {
				t.Fatalf("reopen: %v", err)
			}
			defer re.Close() //nolint:errcheck
			reSheet, err := re.Sheet(0)
			if err != nil {
				t.Fatalf("reopened Sheet(0): %v", err)
			}
			tc.verify(t, reSheet)
		})
	}
}

func mustNoErr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("mutator returned an error: %v", err)
	}
}

// buildChartsheetWithNoteWorkbook returns a package whose chartsheet (an opaque
// sheet: preserved verbatim, never regenerated from a worksheet model) owns a
// legacy comments part, plus a calcChain the save path drops for any dirty
// sheet.
func buildChartsheetWithNoteWorkbook(t *testing.T) []byte {
	t.Helper()
	parts := map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/><Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/><Override PartName="/xl/chartsheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.chartsheet+xml"/><Override PartName="/xl/comments1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.comments+xml"/><Override PartName="/xl/calcChain.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.calcChain+xml"/></Types>`,
		"_rels/.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/></Relationships>`,
		"xl/workbook.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Data" sheetId="1" r:id="rId1"/><sheet name="Chart" sheetId="2" r:id="rId2"/></sheets></workbook>`,
		"xl/_rels/workbook.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/><Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/chartsheet" Target="chartsheets/sheet1.xml"/><Relationship Id="rId3" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/calcChain" Target="calcChain.xml"/></Relationships>`,
		"xl/worksheets/sheet1.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheetData><row r="1"><c r="A1"><f>1+1</f><v>2</v></c></row></sheetData></worksheet>`,
		"xl/chartsheets/sheet1.xml": chartsheetPartXML,
		"xl/chartsheets/_rels/sheet1.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/comments" Target="../comments1.xml"/></Relationships>`,
		"xl/comments1.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<comments xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><authors><author>Ann</author></authors><commentList><comment ref="A1" authorId="0"><text><r><t>note</t></r></text></comment></commentList></comments>`,
		"xl/calcChain.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<calcChain xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><c r="A1" i="1"/></calcChain>`,
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, name := range sortedKeys(parts) {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("zip create %s: %v", name, err)
		}
		if _, err := w.Write([]byte(parts[name])); err != nil {
			t.Fatalf("zip write %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("zip close: %v", err)
	}
	return buf.Bytes()
}

// TestOpaqueSheetCommentEditDoesNotFlagSheet guards the invariant markDirty
// encodes. markCommentsDirty used to assign s.dirty directly, bypassing the
// opaque-sheet guard exactly as AddImage's two raw assignments did (C423). A
// Comment handle reaches it from an opaque sheet, because Sheet.Comments loads
// whatever comment parts the sheet's relationships name without an opaque
// check — and the save path's sheet loops that decide whether to rebuild the
// workbook relationships do not re-check opaque, so the flag leaked out as a
// rewritten xl/_rels/workbook.xml.rels for an edit that is then discarded.
func TestOpaqueSheetCommentEditDoesNotFlagSheet(t *testing.T) {
	src := buildChartsheetWithNoteWorkbook(t)

	wb, err := OpenReader(bytes.NewReader(src), int64(len(src)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	base, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("baseline save: %v", err)
	}
	_ = wb.Close()
	if d := packageDiff(t, src, base); len(d) != 0 {
		t.Fatalf("fixture does not round-trip unmodified (%v)", d)
	}

	wb2, err := OpenReader(bytes.NewReader(src), int64(len(src)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	defer wb2.Close() //nolint:errcheck
	chart, err := wb2.SheetByName("Chart")
	if err != nil {
		t.Fatalf("SheetByName(Chart): %v", err)
	}
	if !chart.opaque {
		t.Fatal("fixture sheet Chart is not opaque; the guard would be vacuous")
	}
	notes := chart.Comments()
	if len(notes) != 1 {
		t.Fatalf("Comments() on the chartsheet = %d, want 1", len(notes))
	}

	notes[0].SetRichText([]TextRun{{Text: "edited"}})

	if chart.dirty {
		t.Error("editing a comment on an opaque sheet marked the sheet dirty; " +
			"markDirty refuses opaque sheets and every writer must go through it")
	}

	out, err := wb2.SaveBytes()
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	before := zipPartMap(t, src)
	after := zipPartMap(t, out)
	for _, name := range []string{"xl/_rels/workbook.xml.rels", "xl/chartsheets/sheet1.xml"} {
		if !bytes.Equal(before[name], after[name]) {
			t.Errorf("%s was regenerated by an edit to an opaque sheet", name)
		}
	}
}
