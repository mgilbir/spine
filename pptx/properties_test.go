package pptx

import (
	"archive/zip"
	"bytes"
	"io"
	"testing"
)

// TestPropertiesEditPersistsAcrossSave verifies that core-property edits made
// after Open survive a save (C10 companion: pptx regenerates core.xml from
// the model, and that behavior must keep matching docx/xlsx).
func TestPropertiesEditPersistsAcrossSave(t *testing.T) {
	p, err := Open("testdata/minimal.pptx")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() { _ = p.Close() }()

	p.Properties.Title = "Edited Title"
	p.Properties.Creator = "Edited Creator"

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes() error = %v", err)
	}

	p2, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("reopen error = %v", err)
	}
	defer func() { _ = p2.Close() }()

	if p2.Properties.Title != "Edited Title" {
		t.Errorf("Title after save/reopen = %q, want %q", p2.Properties.Title, "Edited Title")
	}
	if p2.Properties.Creator != "Edited Creator" {
		t.Errorf("Creator after save/reopen = %q, want %q", p2.Properties.Creator, "Edited Creator")
	}
}

// TestSaveTwiceIdenticalAndContentTypesUnmutated verifies that repeated saves
// produce identical bytes and do not mutate the reader's shared ContentTypes
// (C53: writers must operate on a clone).
func TestSaveTwiceIdenticalAndContentTypesUnmutated(t *testing.T) {
	p, err := Open("testdata/minimal.pptx")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() { _ = p.Close() }()

	overridesBefore := len(p.reader.ContentTypes.Overrides)

	first, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("first SaveBytes() error = %v", err)
	}
	second, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("second SaveBytes() error = %v", err)
	}

	if !bytes.Equal(first, second) {
		t.Error("two sequential saves produced different bytes")
	}
	if got := len(p.reader.ContentTypes.Overrides); got != overridesBefore {
		t.Errorf("reader ContentTypes overrides = %d after saves, want %d (unmutated)", got, overridesBefore)
	}
}

// TestUnmodifiedCorePropertiesPreserveRawBytes verifies that a
// zero-modification save writes the source docProps/core.xml verbatim
// (producer formatting varies; regenerating the part drifts) while an edit
// still regenerates it.
func TestUnmodifiedCorePropertiesPreserveRawBytes(t *testing.T) {
	p, err := Open("testdata/minimal.pptx")
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() { _ = p.Close() }()
	if p.corePartRaw == nil {
		t.Skip("fixture has no docProps/core.xml")
	}

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes() error = %v", err)
	}
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("zip: %v", err)
	}
	var saved []byte
	for _, f := range zr.File {
		if f.Name == "docProps/core.xml" {
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("open part: %v", err)
			}
			saved, err = io.ReadAll(rc)
			rc.Close()
			if err != nil {
				t.Fatalf("read part: %v", err)
			}
		}
	}
	if !bytes.Equal(saved, p.corePartRaw.Data) {
		t.Errorf("unmodified save regenerated docProps/core.xml:\ngot  %q\nwant %q", saved, p.corePartRaw.Data)
	}
}
