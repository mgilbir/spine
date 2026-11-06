package xlsx_test

import (
	"archive/zip"
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/mgilbir/spine/internal/testutil"
	"github.com/mgilbir/spine/opc"
	"github.com/mgilbir/spine/xlsx"
)

// unreadableFixture is a small workbook — workbook, worksheet, relationships
// and core properties — every entry of which the sweep below makes unreadable
// in turn.
func unreadableFixture(t *testing.T) []byte {
	t.Helper()
	wb := xlsx.Create()
	sh, err := wb.AddSheet("Data")
	if err != nil {
		t.Fatal(err)
	}
	if err := sh.SetCellValue("A1", "kept"); err != nil {
		t.Fatal(err)
	}
	wb.Properties.Title = "fixture"
	data, err := wb.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := xlsx.OpenReader(bytes.NewReader(data), int64(len(data))); err != nil {
		t.Fatalf("intact fixture does not open: %v", err)
	}
	return data
}

func zipEntryNames(t *testing.T, pkg []byte) []string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(pkg), int64(len(pkg)))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, f := range zr.File {
		names = append(names, f.Name)
	}
	return names
}

// TestOpen_UnreadablePartFailsOpen: every part of the package, made present
// but unreadable, must fail the open with the read error in the chain. Before,
// an unreadable docProps/core.xml opened cleanly and the next save dropped it,
// and an unreadable xl/_rels/workbook.xml.rels surfaced as a misleading
// dangling-relationship error.
func TestOpen_UnreadablePartFailsOpen(t *testing.T) {
	pkg := unreadableFixture(t)
	names := zipEntryNames(t, pkg)
	for _, want := range []string{"docProps/core.xml", "xl/_rels/workbook.xml.rels"} {
		found := false
		for _, n := range names {
			found = found || n == want
		}
		if !found {
			t.Fatalf("fixture lacks %s; the cases below would not cover it (entries: %v)", want, names)
		}
	}
	for _, name := range names {
		t.Run(name+"/oversized", func(t *testing.T) {
			bad := testutil.WithOversizedEntry(t, pkg, name)
			_, err := xlsx.OpenReader(bytes.NewReader(bad), int64(len(bad)))
			if err == nil {
				t.Fatal("open succeeded with an unreadable part")
			}
			if !errors.Is(err, opc.ErrDecompressionLimit) {
				t.Errorf("error does not wrap opc.ErrDecompressionLimit: %v", err)
			}
			if !strings.Contains(err.Error(), name) {
				t.Errorf("error does not name the unreadable part: %v", err)
			}
		})
		t.Run(name+"/badcrc", func(t *testing.T) {
			bad := testutil.WithBadCRC(t, pkg, name)
			_, err := xlsx.OpenReader(bytes.NewReader(bad), int64(len(bad)))
			if !errors.Is(err, zip.ErrChecksum) {
				t.Errorf("open error = %v, want one wrapping zip.ErrChecksum", err)
			}
		})
	}
}

// TestOpen_UnreadableWorkbookRelsNamesTheReadFailure pins the misleading
// message: an unreadable workbook relationships part used to be reported as a
// sheet referencing a relationship "which does not exist".
func TestOpen_UnreadableWorkbookRelsNamesTheReadFailure(t *testing.T) {
	bad := testutil.WithOversizedEntry(t, unreadableFixture(t), "xl/_rels/workbook.xml.rels")
	_, err := xlsx.OpenReader(bytes.NewReader(bad), int64(len(bad)))
	if err == nil {
		t.Fatal("open succeeded")
	}
	if strings.Contains(err.Error(), "does not exist") {
		t.Errorf("error blames a missing relationship instead of the read failure: %v", err)
	}
	if !strings.Contains(err.Error(), "/xl/_rels/workbook.xml.rels") || !errors.Is(err, opc.ErrDecompressionLimit) {
		t.Errorf("error = %v, want the read failure of /xl/_rels/workbook.xml.rels", err)
	}
}

// TestOpen_MissingCorePropertiesStillOpens keeps the leniency for a part that
// is genuinely absent: a dangling core-properties relationship opens, and the
// save does not invent the part's original content.
func TestOpen_MissingCorePropertiesStillOpens(t *testing.T) {
	without := testutil.WithoutEntry(t, unreadableFixture(t), "docProps/core.xml")
	wb, err := xlsx.OpenReader(bytes.NewReader(without), int64(len(without)))
	if err != nil {
		t.Fatalf("open with core.xml absent: %v", err)
	}
	if _, err := wb.SaveBytes(); err != nil {
		t.Fatalf("save with core.xml absent: %v", err)
	}
}
