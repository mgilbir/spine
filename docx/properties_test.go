package docx

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"testing"
)

// readZipPart extracts one part from an in-memory zip archive.
func readZipPart(t *testing.T, data []byte, name string) []byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("zip.NewReader() error = %v", err)
	}
	for _, f := range zr.File {
		if f.Name != name {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open %s: %v", name, err)
		}
		defer func() { _ = rc.Close() }()
		b, err := io.ReadAll(rc)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		return b
	}
	t.Fatalf("part %s not found in archive", name)
	return nil
}

// TestPropertiesEditPersistsAcrossSave verifies that core-property edits made
// after Open survive a save (C10: the preserved raw core.xml must not shadow
// the regenerated one).
func TestPropertiesEditPersistsAcrossSave(t *testing.T) {
	d, err := Open("testdata/svg_test.docx")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() { _ = d.Close() }()

	d.Properties.Title = "Edited Title"
	d.Properties.Creator = "Edited Creator"

	data, err := d.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes() error = %v", err)
	}

	d2, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("reopen error = %v", err)
	}
	defer func() { _ = d2.Close() }()

	if d2.Properties.Title != "Edited Title" {
		t.Errorf("Title after save/reopen = %q, want %q", d2.Properties.Title, "Edited Title")
	}
	if d2.Properties.Creator != "Edited Creator" {
		t.Errorf("Creator after save/reopen = %q, want %q", d2.Properties.Creator, "Edited Creator")
	}
}

// TestUnmodifiedSaveKeepsCoreXMLByteIdentical verifies that when properties
// are not touched, the preserved raw core.xml is written verbatim (C10:
// unchanged documents keep byte-identity).
func TestUnmodifiedSaveKeepsCoreXMLByteIdentical(t *testing.T) {
	const path = "testdata/svg_test.docx"
	orig, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}

	d, err := Open(path)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() { _ = d.Close() }()

	data, err := d.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes() error = %v", err)
	}

	origCore := readZipPart(t, orig, "docProps/core.xml")
	newCore := readZipPart(t, data, "docProps/core.xml")
	if !bytes.Equal(origCore, newCore) {
		t.Errorf("core.xml changed on unmodified save:\noriginal: %s\nsaved:    %s", origCore, newCore)
	}
}

// readAllZipParts extracts every part of an in-memory zip archive.
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

// TestSaveTwiceIdenticalAndContentTypesUnmutated verifies that repeated saves
// produce equivalent packages (same parts, same bytes per part; archive entry
// order follows map iteration and is not asserted) and do not mutate the
// reader's shared ContentTypes (C53: writers must operate on a clone).
func TestSaveTwiceIdenticalAndContentTypesUnmutated(t *testing.T) {
	d, err := Open("testdata/svg_test.docx")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() { _ = d.Close() }()

	overridesBefore := len(d.reader.ContentTypes.Overrides)

	first, err := d.SaveBytes()
	if err != nil {
		t.Fatalf("first SaveBytes() error = %v", err)
	}
	second, err := d.SaveBytes()
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
	if got := len(d.reader.ContentTypes.Overrides); got != overridesBefore {
		t.Errorf("reader ContentTypes overrides = %d after saves, want %d (unmutated)", got, overridesBefore)
	}
}
