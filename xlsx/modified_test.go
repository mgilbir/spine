package xlsx

import (
	"archive/zip"
	"bytes"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/mgilbir/spine/common/dml"
)

// The two halves of the "stamp dcterms:modified if and only if the content
// changed" contract, pinned together on purpose: a suite that only tests one
// direction does not notice the sense being inverted. Every test that could be
// satisfied by a clock that happens not to tick sleeps past a second boundary
// first — dcterms:modified is written at RFC3339 (one second) resolution, which
// is exactly how a per-save stamp hid for three audits as a 1-in-300 flake.

// openFixture opens testdata/minimal.xlsx from its bytes and reports the stored
// dcterms:modified, so a test asserts against the value on disk rather than one
// an earlier step in the same test produced.
func openFixture(t *testing.T) (*Workbook, []byte, time.Time) {
	t.Helper()
	src, err := os.ReadFile("testdata/minimal.xlsx")
	if err != nil {
		t.Fatalf("reading fixture: %v", err)
	}
	wb, err := OpenReader(bytes.NewReader(src), int64(len(src)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	t.Cleanup(func() { _ = wb.Close() })
	return wb, src, wb.Properties.Modified
}

// reopenModified saves data, reopens it, and returns the stored
// dcterms:modified — the value that actually reached the package, not the
// in-memory field.
func reopenModified(t *testing.T, data []byte) time.Time {
	t.Helper()
	wb, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer wb.Close() //nolint:errcheck
	return wb.Properties.Modified
}

// TestUntouchedSaveDoesNotStampModified is the determinism half: opening a
// workbook and saving it without touching anything must leave dcterms:modified
// exactly as it was and reproduce the package byte-for-byte, however much wall
// time passes between the saves.
func TestUntouchedSaveDoesNotStampModified(t *testing.T) {
	wb, src, before := openFixture(t)

	first, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("first SaveBytes: %v", err)
	}
	// Cross a second boundary: a per-save stamp cannot survive this.
	time.Sleep(1100 * time.Millisecond)
	second, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("second SaveBytes: %v", err)
	}

	if !bytes.Equal(first, second) {
		t.Error("re-saving an untouched workbook produced different bytes")
	}
	if !wb.Properties.Modified.Equal(before) {
		t.Errorf("saving an untouched workbook moved Properties.Modified: %v -> %v", before, wb.Properties.Modified)
	}
	if got := reopenModified(t, first); !got.Equal(before) {
		t.Errorf("saving an untouched workbook stamped dcterms:modified: %v -> %v", before, got)
	}
	if d := packageDiff(t, src, first); len(d) != 0 {
		t.Errorf("saving an untouched workbook changed parts: %v", d)
	}
}

// TestReadOnlyAccessDoesNotStampModified is the trap the rule has to avoid.
// xlsx has two shapes of it: Sheet.Cell is a materializing accessor by design
// (C425), so a pure read creates <row>/<c> entries in the model, and
// Workbook.Styles materialized a default stylesheet — and set stylesDirty —
// until PR #257. Keying the stamp off "will this part be regenerated" would
// bump dcterms:modified for a caller who only read the workbook.
func TestReadOnlyAccessDoesNotStampModified(t *testing.T) {
	wb, src, before := openFixture(t)

	// Everything a reader would touch, including the two materializing paths.
	callEveryAccessor(t, reflect.ValueOf(wb), "Workbook")
	_ = wb.Styles().NamedStyles()
	_ = wb.Theme()
	_ = wb.CustomProperties()
	_ = wb.DefinedNames()
	for _, sheet := range wb.Sheets() {
		callEveryAccessor(t, reflect.ValueOf(sheet), "Sheet")
		if c, err := sheet.Cell("Z99"); err == nil && c != nil {
			callEveryAccessor(t, reflect.ValueOf(c), "Cell")
			_ = c.Value()
		}
		_, _ = sheet.CellValue("A1")
		_ = sheet.Comments()
	}

	if wb.contentChanged() {
		t.Error("read-only access recorded a content edit")
	}

	time.Sleep(1100 * time.Millisecond)
	data, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	if got := reopenModified(t, data); !got.Equal(before) {
		t.Errorf("read-only access bumped dcterms:modified: %v -> %v", before, got)
	}
	if d := packageDiff(t, src, data); len(d) != 0 {
		t.Errorf("read-only access changed the saved package: %v", d)
	}
}

// TestEditStampsModified is the other half: an edit that really changes the
// workbook records when it was written.
func TestEditStampsModified(t *testing.T) {
	wb, _, before := openFixture(t)
	sheet, err := wb.Sheet(0)
	if err != nil {
		t.Fatalf("Sheet(0): %v", err)
	}
	if err := sheet.SetCellValue("D4", "edited"); err != nil {
		t.Fatalf("SetCellValue: %v", err)
	}

	data, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	got := reopenModified(t, data)
	if !got.After(before) {
		t.Errorf("edit did not stamp dcterms:modified: was %v, still %v", before, got)
	}
}

// TestWorkbookLevelEditStampsModified covers the mutators that need no
// regeneration flag because workbook.xml is regenerated from the model on every
// save. They are exactly the ones nothing else records, so they are exactly the
// ones a flags-only derivation would miss.
func TestWorkbookLevelEditStampsModified(t *testing.T) {
	cases := map[string]func(t *testing.T, wb *Workbook){
		"AddDefinedName": func(t *testing.T, wb *Workbook) {
			if err := wb.AddDefinedName("Probe", "Sheet1!$A$1"); err != nil {
				t.Fatalf("AddDefinedName: %v", err)
			}
		},
		"Sheet.SetName": func(t *testing.T, wb *Workbook) {
			s, err := wb.Sheet(0)
			if err != nil {
				t.Fatalf("Sheet(0): %v", err)
			}
			if err := s.SetName("Renamed"); err != nil {
				t.Fatalf("SetName: %v", err)
			}
		},
		"Sheet.SetPrintArea": func(t *testing.T, wb *Workbook) {
			s, err := wb.Sheet(0)
			if err != nil {
				t.Fatalf("Sheet(0): %v", err)
			}
			if err := s.SetPrintArea("A1:B2"); err != nil {
				t.Fatalf("SetPrintArea: %v", err)
			}
		},
		"SetActiveSheet": func(t *testing.T, wb *Workbook) {
			if err := wb.SetActiveSheet(1); err != nil {
				t.Fatalf("SetActiveSheet: %v", err)
			}
		},
		"Protect": func(t *testing.T, wb *Workbook) {
			wb.Protect(WorkbookProtectionOptions{LockStructure: true})
		},
		"SetCustomProperty": func(t *testing.T, wb *Workbook) {
			if err := wb.SetCustomProperty("Probe", "value"); err != nil {
				t.Fatalf("SetCustomProperty: %v", err)
			}
		},
	}

	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			wb, _, before := openFixture(t)
			mutate(t, wb)
			data, err := wb.SaveBytes()
			if err != nil {
				t.Fatalf("SaveBytes: %v", err)
			}
			if got := reopenModified(t, data); !got.After(before) {
				t.Errorf("%s did not stamp dcterms:modified: was %v, still %v", name, before, got)
			}
		})
	}
}

// TestEditedWorkbookSaveIsStillIdempotent pins the interaction between the two
// halves: the edit stamps once, and re-saving without touching the workbook
// again must reproduce the same bytes — including the same stamp — rather than
// stamping afresh. A latching flag fails this; so does an unconditional stamp.
func TestEditedWorkbookSaveIsStillIdempotent(t *testing.T) {
	wb, _, _ := openFixture(t)
	sheet, err := wb.Sheet(0)
	if err != nil {
		t.Fatalf("Sheet(0): %v", err)
	}
	if err := sheet.SetCellValue("D4", "edited"); err != nil {
		t.Fatalf("SetCellValue: %v", err)
	}

	first, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("first SaveBytes: %v", err)
	}
	time.Sleep(1100 * time.Millisecond)
	second, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("second SaveBytes: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Error("re-saving an already-saved edit produced different bytes")
	}
}

// TestSecondEditStampsAgain: after a save has stamped, a further edit must
// stamp again. This is what a latching boolean cannot do — after the first save
// it still reads "changed", so the second edit is indistinguishable from the
// first and the save either re-stamps every time or never again.
func TestSecondEditStampsAgain(t *testing.T) {
	wb, _, _ := openFixture(t)
	sheet, err := wb.Sheet(0)
	if err != nil {
		t.Fatalf("Sheet(0): %v", err)
	}
	if err := sheet.SetCellValue("D4", "first"); err != nil {
		t.Fatalf("SetCellValue: %v", err)
	}
	first, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("first SaveBytes: %v", err)
	}
	afterFirst := reopenModified(t, first)

	time.Sleep(1100 * time.Millisecond)
	if err := sheet.SetCellValue("D5", "second"); err != nil {
		t.Fatalf("SetCellValue: %v", err)
	}
	second, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("second SaveBytes: %v", err)
	}
	if got := reopenModified(t, second); !got.After(afterFirst) {
		t.Errorf("second edit did not re-stamp: %v then %v", afterFirst, got)
	}
}

// TestExplicitModifiedIsRespected: assigning Properties.Modified is itself a
// property edit, so the save must write the caller's value rather than
// overwriting it with the save time — even when the workbook was also edited.
func TestExplicitModifiedIsRespected(t *testing.T) {
	wb, _, _ := openFixture(t)
	want := time.Date(2001, 2, 3, 4, 5, 6, 0, time.UTC)
	wb.Properties.Modified = want
	sheet, err := wb.Sheet(0)
	if err != nil {
		t.Fatalf("Sheet(0): %v", err)
	}
	if err := sheet.SetCellValue("D4", "edited"); err != nil {
		t.Fatalf("SetCellValue: %v", err)
	}

	data, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	if got := reopenModified(t, data); !got.Equal(want) {
		t.Errorf("explicit Properties.Modified overwritten: want %v, got %v", want, got)
	}
}

// TestRejectedEditDoesNotStamp: a mutator that returns an error changed
// nothing, so it must not move the timestamp either. These are the calls PR
// #257 fixed to stop dirtying the sheet before validating (C544); the stamp
// inherits that fix by deriving from the same flag.
func TestRejectedEditDoesNotStamp(t *testing.T) {
	cases := map[string]func(c *Cell) error{
		"SetStyle with an out-of-range rotation": func(c *Cell) error {
			return c.SetStyle(CellStyle{Alignment: &AlignmentStyle{Rotation: 400}})
		},
		"SetStyle with a negative number-format id": func(c *Cell) error {
			return c.SetStyle(CellStyle{NumberFormatID: -3})
		},
		"SetNamedStyle with an unknown name": func(c *Cell) error {
			return c.SetNamedStyle("NoSuchStyle")
		},
	}

	for name, call := range cases {
		t.Run(name, func(t *testing.T) {
			wb, src, before := openFixture(t)
			sheet, err := wb.Sheet(0)
			if err != nil {
				t.Fatalf("Sheet(0): %v", err)
			}
			cell, err := sheet.Cell("A1")
			if err != nil {
				t.Fatalf("Cell: %v", err)
			}
			if err := call(cell); err == nil {
				t.Fatal("expected the invalid style to be rejected")
			}
			time.Sleep(1100 * time.Millisecond)
			data, err := wb.SaveBytes()
			if err != nil {
				t.Fatalf("SaveBytes: %v", err)
			}
			if got := reopenModified(t, data); !got.Equal(before) {
				t.Errorf("a rejected mutator stamped dcterms:modified: %v -> %v", before, got)
			}
			if d := packageDiff(t, src, data); len(d) != 0 {
				t.Errorf("a rejected mutator changed the saved package: %v", d)
			}
		})
	}
}

// TestOpaqueSheetEditDoesNotStamp: markDirty refuses opaque
// (chartsheet/dialogsheet/macrosheet) sheets because their bytes are preserved
// verbatim and the edit is discarded (C241/C423). An edit that is discarded is
// not a change to the saved workbook, so it must not stamp either — which it
// cannot, because the counter is bumped inside that same guard.
func TestOpaqueSheetEditDoesNotStamp(t *testing.T) {
	src := buildChartsheetWithNoteWorkbook(t)
	wb, err := OpenReader(bytes.NewReader(src), int64(len(src)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	defer wb.Close() //nolint:errcheck
	before := wb.Properties.Modified

	chart, err := wb.SheetByName("Chart")
	if err != nil {
		t.Fatalf("SheetByName(Chart): %v", err)
	}
	notes := chart.Comments()
	if len(notes) != 1 {
		t.Fatalf("Comments() on the chartsheet = %d, want 1", len(notes))
	}
	notes[0].SetRichText([]TextRun{{Text: "edited"}})

	if wb.contentChanged() {
		t.Error("an edit to an opaque sheet — which the save discards — recorded a content change")
	}
	data, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	if got := reopenModified(t, data); !got.Equal(before) {
		t.Errorf("an edit to an opaque sheet stamped dcterms:modified: %v -> %v", before, got)
	}
}

// TestCreatedWorkbookStampsOnContentThenHoldsStill covers the created-workbook
// side: Create leaves the properties empty, adding content records the write
// time, and saving again with nothing further changed reproduces those bytes.
func TestCreatedWorkbookStampsOnContentThenHoldsStill(t *testing.T) {
	wb := Create()
	sheet := addSheetT(wb, "Sheet1")
	if err := sheet.SetCellValue("A1", "hello"); err != nil {
		t.Fatalf("SetCellValue: %v", err)
	}

	first, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("first SaveBytes: %v", err)
	}
	if stamped := reopenModified(t, first); stamped.IsZero() {
		t.Error("a created workbook with content saved without a dcterms:modified stamp")
	}

	time.Sleep(1100 * time.Millisecond)
	second, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("second SaveBytes: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Error("re-saving an unchanged created workbook produced different bytes")
	}
}

// TestNoCorePartWorkbookGainsNoCorePart: a package that stores no
// /docProps/core.xml must not grow one just because a cell was edited. Some
// producers keep core properties elsewhere or omit them, and the stamp is not a
// reason to change the package shape.
func TestNoCorePartWorkbookGainsNoCorePart(t *testing.T) {
	src := buildBookXlsxWithout(t, "docProps/core.xml")
	wb, err := OpenReader(bytes.NewReader(src), int64(len(src)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	defer wb.Close() //nolint:errcheck
	before := wb.Properties.Modified

	sheet, err := wb.Sheet(0)
	if err != nil {
		t.Fatalf("Sheet(0): %v", err)
	}
	if err := sheet.SetCellValue("D4", "edited"); err != nil {
		t.Fatalf("SetCellValue: %v", err)
	}
	data, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	if !wb.Properties.Modified.Equal(before) {
		t.Errorf("stamped a workbook that has no core properties part: %v -> %v", before, wb.Properties.Modified)
	}
	if _, ok := zipPartMap(t, data)["docProps/core.xml"]; ok {
		t.Error("editing a cell added a docProps/core.xml the source did not have")
	}
}

// corePartIndex returns the zip entry index of docProps/core.xml, or -1.
func corePartIndex(t *testing.T, data []byte) int {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("reading package: %v", err)
	}
	for i, f := range zr.File {
		if f.Name == "docProps/core.xml" {
			return i
		}
	}
	return -1
}

// TestEditedCorePropsKeepTheirPartPosition: a regenerated docProps/core.xml is
// written where the round-trip save puts the preserved one, not appended by
// Close after everything else. Leaving it to Close moved the part to the end of
// the archive, so edit -> reopen -> save produced a different entry order for
// identical content — the pptx regression this change was written not to
// repeat.
func TestEditedCorePropsKeepTheirPartPosition(t *testing.T) {
	wb, src, _ := openFixture(t)
	sheet, err := wb.Sheet(0)
	if err != nil {
		t.Fatalf("Sheet(0): %v", err)
	}
	if err := sheet.SetCellValue("D4", "edited"); err != nil {
		t.Fatalf("SetCellValue: %v", err)
	}
	edited, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	// The baseline is where an unmodified round-trip puts the part: a save that
	// stamps must not relocate it.
	clean, err := OpenReader(bytes.NewReader(src), int64(len(src)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	defer clean.Close() //nolint:errcheck
	untouched, err := clean.SaveBytes()
	if err != nil {
		t.Fatalf("baseline SaveBytes: %v", err)
	}

	want, got := corePartIndex(t, untouched), corePartIndex(t, edited)
	if want < 0 {
		t.Fatal("the unmodified round-trip wrote no docProps/core.xml; the guard would be vacuous")
	}
	if got != want {
		t.Errorf("a stamped save moved docProps/core.xml from entry %d to entry %d; "+
			"regenerated core properties must be written in place of the preserved part, "+
			"not appended by Close", want, got)
	}

	// And the content itself must be stable across the reopen: the second save
	// preserves the part where it found it, so a moved part would also mean the
	// two archives disagree about more than the timestamp.
	reopened, err := OpenReader(bytes.NewReader(edited), int64(len(edited)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer reopened.Close() //nolint:errcheck
	again, err := reopened.SaveBytes()
	if err != nil {
		t.Fatalf("re-save: %v", err)
	}
	if d := packageDiff(t, edited, again); len(d) != 0 {
		t.Errorf("re-saving the reopened result of an edit changed parts: %v", d)
	}
	if got := corePartIndex(t, again); got != want {
		t.Errorf("the reopened save put docProps/core.xml at entry %d, want %d", got, want)
	}
}

// buildThemedXlsx assembles a workbook that carries a theme part and core
// properties, the two things the theme half of the stamp needs.
func buildThemedXlsx(t *testing.T) []byte {
	t.Helper()
	const theme = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" name="Office Theme">` +
		`<a:themeElements><a:clrScheme name="Office">` +
		`<a:dk1><a:sysClr val="windowText" lastClr="000000"/></a:dk1>` +
		`<a:lt1><a:sysClr val="window" lastClr="FFFFFF"/></a:lt1>` +
		`<a:dk2><a:srgbClr val="44546A"/></a:dk2><a:lt2><a:srgbClr val="E7E6E6"/></a:lt2>` +
		`<a:accent1><a:srgbClr val="4472C4"/></a:accent1><a:accent2><a:srgbClr val="ED7D31"/></a:accent2>` +
		`<a:accent3><a:srgbClr val="A5A5A5"/></a:accent3><a:accent4><a:srgbClr val="FFC000"/></a:accent4>` +
		`<a:accent5><a:srgbClr val="5B9BD5"/></a:accent5><a:accent6><a:srgbClr val="70AD47"/></a:accent6>` +
		`<a:hlink><a:srgbClr val="0563C1"/></a:hlink><a:folHlink><a:srgbClr val="954F72"/></a:folHlink>` +
		`</a:clrScheme><a:fontScheme name="Office">` +
		`<a:majorFont><a:latin typeface="Calibri Light"/><a:ea typeface=""/><a:cs typeface=""/></a:majorFont>` +
		`<a:minorFont><a:latin typeface="Calibri"/><a:ea typeface=""/><a:cs typeface=""/></a:minorFont>` +
		`</a:fontScheme><a:fmtScheme name="Office"><a:fillStyleLst><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:fillStyleLst>` +
		`<a:lnStyleLst><a:ln w="6350"><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:ln></a:lnStyleLst>` +
		`<a:effectStyleLst><a:effectStyle><a:effectLst/></a:effectStyle></a:effectStyleLst>` +
		`<a:bgFillStyleLst><a:solidFill><a:schemeClr val="phClr"/></a:solidFill></a:bgFillStyleLst>` +
		`</a:fmtScheme></a:themeElements><a:objectDefaults/><a:extraClrSchemeLst/></a:theme>`

	return buildFixtureXlsxParts(t, []struct{ name, data string }{
		{"[Content_Types].xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
			`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
			`<Default Extension="xml" ContentType="application/xml"/>` +
			`<Override PartName="/xl/workbook.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.sheet.main+xml"/>` +
			`<Override PartName="/xl/worksheets/sheet1.xml" ContentType="application/vnd.openxmlformats-officedocument.spreadsheetml.worksheet+xml"/>` +
			`<Override PartName="/xl/theme/theme1.xml" ContentType="application/vnd.openxmlformats-officedocument.theme+xml"/>` +
			`<Override PartName="/docProps/core.xml" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/>` +
			`</Types>`},
		{"_rels/.rels", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="xl/workbook.xml"/>` +
			`<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="docProps/core.xml"/>` +
			`</Relationships>`},
		{"docProps/core.xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" ` +
			`xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:dcterms="http://purl.org/dc/terms/" ` +
			`xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance">` +
			`<dcterms:modified xsi:type="dcterms:W3CDTF">2001-02-03T04:05:06Z</dcterms:modified></cp:coreProperties>`},
		{"xl/workbook.xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<workbook xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">` +
			`<sheets><sheet name="Sheet1" sheetId="1" r:id="rId1"/></sheets></workbook>`},
		{"xl/_rels/workbook.xml.rels", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet" Target="worksheets/sheet1.xml"/>` +
			`<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme" Target="theme/theme1.xml"/>` +
			`</Relationships>`},
		{"xl/worksheets/sheet1.xml", `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData><row r="1"><c r="A1"><v>1</v></c></row></sheetData></worksheet>`},
		{"xl/theme/theme1.xml", theme},
	})
}

// TestThemeEditStampsThenHoldsStill covers the one signal that cannot be a
// counter bump at the mutator: dml.ThemeEditor lives in another package and
// exposes only a latching Modified bit, so the theme edit is recognized at save
// time by comparing the re-serialized theme against the stored bytes. Reading
// the theme must not stamp, editing it must, and a repeat save must not stamp
// again — which the latching bit alone would do.
func TestThemeEditStampsThenHoldsStill(t *testing.T) {
	src := buildThemedXlsx(t)

	read, err := OpenReader(bytes.NewReader(src), int64(len(src)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	defer read.Close() //nolint:errcheck
	before := read.Properties.Modified
	if before.IsZero() {
		t.Fatal("fixture carries no dcterms:modified; the guard would be vacuous")
	}
	if th := read.Theme(); th == nil {
		t.Fatal("Theme() = nil on a workbook that has a theme part")
	} else if th.ColorScheme() == nil {
		t.Fatal("ColorScheme() = nil")
	}
	data, err := read.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	if got := reopenModified(t, data); !got.Equal(before) {
		t.Errorf("reading the theme stamped dcterms:modified: %v -> %v", before, got)
	}

	wb, err := OpenReader(bytes.NewReader(src), int64(len(src)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	defer wb.Close() //nolint:errcheck
	green, err := dml.ParseRGB("00FF00")
	if err != nil {
		t.Fatalf("ParseRGB: %v", err)
	}
	wb.Theme().ColorScheme().SetAccent2(green.ToColor())

	first, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("first SaveBytes: %v", err)
	}
	if got := reopenModified(t, first); !got.After(before) {
		t.Errorf("a theme edit did not stamp dcterms:modified: was %v, still %v", before, got)
	}

	time.Sleep(1100 * time.Millisecond)
	second, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("second SaveBytes: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Error("re-saving an already-saved theme edit produced different bytes")
	}
}
