package xlsx

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"testing"
)

// readAllZipParts extracts every part of an in-memory zip archive.
// (readZipPart lives in sheet_mutator_order_test.go.)
func readAllZipParts(t *testing.T, data []byte) map[string][]byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("zip.NewReader() error = %v", err)
	}
	parts := make(map[string][]byte, len(zr.File))
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open %s: %v", f.Name, err)
		}
		b, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("read %s: %v", f.Name, err)
		}
		parts[f.Name] = b
	}
	return parts
}

// TestPropertiesEditPersistsAcrossSave verifies that core-property edits made
// after Open survive a save (C10: the preserved raw core.xml must not shadow
// the regenerated one).
func TestPropertiesEditPersistsAcrossSave(t *testing.T) {
	w, err := Open("testdata/minimal.xlsx")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() { _ = w.Close() }()

	w.Properties.Title = "Edited Title"
	w.Properties.Creator = "Edited Creator"

	data, err := w.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes() error = %v", err)
	}

	w2, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("reopen error = %v", err)
	}
	defer func() { _ = w2.Close() }()

	if w2.Properties.Title != "Edited Title" {
		t.Errorf("Title after save/reopen = %q, want %q", w2.Properties.Title, "Edited Title")
	}
	if w2.Properties.Creator != "Edited Creator" {
		t.Errorf("Creator after save/reopen = %q, want %q", w2.Properties.Creator, "Edited Creator")
	}
}

// TestUnmodifiedSaveKeepsCoreXMLByteIdentical verifies that when properties
// are not touched, the preserved raw core.xml is written verbatim (C10:
// unchanged workbooks keep byte-identity).
func TestUnmodifiedSaveKeepsCoreXMLByteIdentical(t *testing.T) {
	const path = "testdata/minimal.xlsx"
	orig, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	w, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() { _ = w.Close() }()

	data, err := w.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes() error = %v", err)
	}

	origCore := readZipPart(t, orig, "docProps/core.xml")
	newCore := readZipPart(t, data, "docProps/core.xml")
	if !bytes.Equal(origCore, newCore) {
		t.Errorf("core.xml changed on unmodified save:\noriginal: %s\nsaved:    %s", origCore, newCore)
	}
}

// TestSaveTwiceIdenticalAndContentTypesUnmutated verifies that repeated saves
// produce equivalent packages (same parts, same bytes per part; archive entry
// order follows map iteration and is not asserted) and do not mutate the
// workbook's captured ContentTypes (C53: writers must operate on a clone).
func TestSaveTwiceIdenticalAndContentTypesUnmutated(t *testing.T) {
	w, err := Open("testdata/minimal.xlsx")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() { _ = w.Close() }()

	overridesBefore := len(w.contentTypes.Overrides)

	first, err := w.SaveBytes()
	if err != nil {
		t.Fatalf("first SaveBytes() error = %v", err)
	}
	second, err := w.SaveBytes()
	if err != nil {
		t.Fatalf("second SaveBytes() error = %v", err)
	}

	p1, p2 := readAllZipParts(t, first), readAllZipParts(t, second)
	for name, v1 := range p1 {
		v2, ok := p2[name]
		if !ok {
			t.Errorf("part %s missing from second save", name)
			continue
		}
		if !bytes.Equal(v1, v2) {
			t.Errorf("part %s differs between sequential saves", name)
		}
	}
	for name := range p2 {
		if _, ok := p1[name]; !ok {
			t.Errorf("part %s only present in second save", name)
		}
	}
	if got := len(w.contentTypes.Overrides); got != overridesBefore {
		t.Errorf("captured ContentTypes overrides = %d after saves, want %d (unmutated)", got, overridesBefore)
	}
}
