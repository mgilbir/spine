package docx

import (
	"bytes"
	"os"
	"testing"
)

// TestBuildingBlocksReadFromChart reads the single placeholder building block
// carried by the chart fixture's glossary part.
func TestBuildingBlocksReadFromChart(t *testing.T) {
	doc, err := Open("testdata/chart.docx")
	if err != nil {
		t.Fatal(err)
	}
	defer doc.Close() //nolint:errcheck

	if !doc.HasGlossary() {
		t.Fatal("HasGlossary = false, want true")
	}
	blocks := doc.BuildingBlocks()
	if len(blocks) != 1 {
		t.Fatalf("BuildingBlocks len = %d, want 1", len(blocks))
	}
	b := blocks[0]
	if b.Name() != "DefaultPlaceholder_-1854013440" {
		t.Fatalf("Name = %q", b.Name())
	}
	if b.Gallery() != "placeholder" {
		t.Fatalf("Gallery = %q", b.Gallery())
	}
	if b.Category() != "General" {
		t.Fatalf("Category = %q", b.Category())
	}
	if len(b.Types()) != 1 || b.Types()[0] != "bbPlcHdr" {
		t.Fatalf("Types = %v", b.Types())
	}
	if b.GUID() != "{1A953087-BC75-4687-B086-946FDC359BC7}" {
		t.Fatalf("GUID = %q", b.GUID())
	}
}

// TestBuildingBlocksNone reports no glossary for a document without one.
func TestBuildingBlocksNone(t *testing.T) {
	fixture := fixtureWithDocument(t, fixtureWNS, `<w:body><w:p/></w:body>`)
	doc, err := OpenReader(bytes.NewReader(fixture), int64(len(fixture)))
	if err != nil {
		t.Fatal(err)
	}
	if doc.HasGlossary() {
		t.Fatal("HasGlossary = true for a document with no glossary")
	}
	if b := doc.BuildingBlocks(); b != nil {
		t.Fatalf("BuildingBlocks = %v, want nil", b)
	}
}

// TestGlossaryPreservedByteIdentical guards that reading building blocks leaves
// the glossary part bytes untouched across a round-trip.
func TestGlossaryPreservedByteIdentical(t *testing.T) {
	orig, err := os.ReadFile("testdata/chart.docx")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := Open("testdata/chart.docx")
	if err != nil {
		t.Fatal(err)
	}
	_ = doc.BuildingBlocks()
	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	a, _ := zipEntry(t, orig, "word/glossary/document.xml")
	b, ok := zipEntry(t, saved, "word/glossary/document.xml")
	if !ok {
		t.Fatal("glossary part missing after save")
	}
	if !bytes.Equal(a, b) {
		t.Fatal("glossary/document.xml not byte-identical after round-trip")
	}
}
