package xlsx

import (
	"bytes"
	"strings"
	"testing"
)

// An empty (0-byte) xl/styles.xml is tolerated at Open as an empty stylesheet,
// matching Excel — real Common Crawl files ship a 0-byte styles part. The
// workbook opens, its worksheet is readable, and the empty part is surfaced as
// a styles-empty warning rather than swallowed silently.
func TestOpenToleratesEmptyStylesPart(t *testing.T) {
	data := buildFidelityTestXlsx(t, mutatorTestSheetBare,
		map[string]string{"xl/styles.xml": ""},
		stylesFixtureOverride, stylesFixtureRel)

	wb, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("Open errored on an empty styles.xml: %v", err)
	}
	defer wb.Close() //nolint:errcheck

	if wb.SheetCount() != 1 {
		t.Fatalf("SheetCount = %d, want 1", wb.SheetCount())
	}
	cell, err := wb.Sheets()[0].Cell("A1")
	if err != nil {
		t.Fatalf("Cell A1: %v", err)
	}
	if got := cell.String(); got != "1" {
		t.Errorf("A1 = %q, want %q", got, "1")
	}

	rep := wb.Validate()
	if rep.HasErrors() {
		t.Fatalf("Validate reported errors on a tolerated empty styles part: %v", rep.Errors())
	}
	var found bool
	for _, w := range rep.Warnings() {
		if w.Code == codeStylesEmpty && w.Part == "/xl/styles.xml" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a %q warning for the empty styles part, got: %v", codeStylesEmpty, rep.Warnings())
	}
}

// A zero-modification save of a workbook with a 0-byte styles.xml re-emits the
// empty part byte-for-byte: tolerating it at Open must not regenerate styles
// from the empty model.
func TestEmptyStylesPartZeroModSaveByteIdentical(t *testing.T) {
	data := buildFidelityTestXlsx(t, mutatorTestSheetBare,
		map[string]string{"xl/styles.xml": ""},
		stylesFixtureOverride, stylesFixtureRel)

	wb, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	out, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if got := readZipPart(t, out, "xl/styles.xml"); len(got) != 0 {
		t.Errorf("empty styles.xml not preserved: got %d bytes (%q)", len(got), got)
	}
}

// Guard the boundary: a NON-empty but malformed styles.xml is genuine
// corruption and must still fail Open. Only the empty/EOF case is tolerated.
func TestOpenErrorsOnCorruptStylesPart(t *testing.T) {
	corrupt := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
		`<styleSheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><fonts count="1"><font><sz val="11"/>` // truncated
	data := buildFidelityTestXlsx(t, mutatorTestSheetBare,
		map[string]string{"xl/styles.xml": corrupt},
		stylesFixtureOverride, stylesFixtureRel)

	_, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err == nil {
		t.Fatal("Open succeeded on a non-empty but malformed styles.xml")
	}
	if !strings.Contains(err.Error(), "/xl/styles.xml") {
		t.Errorf("error does not name the failing part: %v", err)
	}
}
