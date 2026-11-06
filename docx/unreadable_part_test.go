package docx_test

import (
	"archive/zip"
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/mgilbir/spine/docx"
	"github.com/mgilbir/spine/internal/testutil"
	"github.com/mgilbir/spine/opc"
)

// unreadableFixture is a small document — body, styles, relationships and
// core properties — every entry of which the sweep below makes unreadable in
// turn.
func unreadableFixture(t *testing.T) []byte {
	t.Helper()
	d := docx.Create()
	d.AddParagraphWithText("body text that must survive")
	d.Properties.Title = "fixture"
	data, err := d.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := docx.OpenReader(bytes.NewReader(data), int64(len(data))); err != nil {
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
// an unreadable word/_rels/document.xml.rels or docProps/core.xml opened
// cleanly and the next save dropped the part.
func TestOpen_UnreadablePartFailsOpen(t *testing.T) {
	pkg := unreadableFixture(t)
	names := zipEntryNames(t, pkg)
	for _, want := range []string{"docProps/core.xml", "word/_rels/document.xml.rels"} {
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
			_, err := docx.OpenReader(bytes.NewReader(bad), int64(len(bad)))
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
			_, err := docx.OpenReader(bytes.NewReader(bad), int64(len(bad)))
			if !errors.Is(err, zip.ErrChecksum) {
				t.Errorf("open error = %v, want one wrapping zip.ErrChecksum", err)
			}
		})
	}
}

// TestOpen_MissingCorePropertiesStillOpens keeps the leniency for a part that
// is genuinely absent: a dangling core-properties relationship opens and saves.
func TestOpen_MissingCorePropertiesStillOpens(t *testing.T) {
	without := testutil.WithoutEntry(t, unreadableFixture(t), "docProps/core.xml")
	d, err := docx.OpenReader(bytes.NewReader(without), int64(len(without)))
	if err != nil {
		t.Fatalf("open with core.xml absent: %v", err)
	}
	if _, err := d.SaveBytes(); err != nil {
		t.Fatalf("save with core.xml absent: %v", err)
	}
}

// TestSave_MainPartUnreadableAfterOpenFailsSave: the main document part is
// not retained after Open validates it; an untouched body is re-read from the
// source at save. When that re-read fails (the source was truncated or
// replaced underneath the open document) the save must fail, not regenerate
// the body from a model that was never built.
func TestSave_MainPartUnreadableAfterOpenFailsSave(t *testing.T) {
	pkg := unreadableFixture(t)
	src := testutil.NewSwitchableReaderAt(pkg)
	d, err := docx.OpenReader(src, src.Size())
	if err != nil {
		t.Fatal(err)
	}
	src.Fail()
	out, err := d.SaveBytes()
	if err == nil {
		t.Fatalf("save succeeded (%d bytes) after the main part became unreadable: "+
			"the body was regenerated instead of preserved", len(out))
	}
	if !errors.Is(err, testutil.ErrInjectedRead) {
		t.Errorf("save error = %v, want one wrapping the read failure", err)
	}
	if !strings.Contains(err.Error(), "/word/document.xml") {
		t.Errorf("save error does not name the main part: %v", err)
	}
}
