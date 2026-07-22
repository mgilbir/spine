package docx

import (
	"bytes"
	"os"
	"testing"
)

// TestAddBuildingBlockNewGlossary authors a building block on a created
// document with no glossary part and reads it back after a round-trip.
func TestAddBuildingBlockNewGlossary(t *testing.T) {
	doc := Create()
	if err := doc.AddBuildingBlock(BuildingBlockDef{
		Name:        "MyBlock",
		Gallery:     "AutoText",
		Category:    "General",
		Types:       []string{"autoTxt"},
		Description: "A reusable snippet",
		GUID:        "{1A953087-BC75-4687-B086-946FDC359BC7}",
	}); err != nil {
		t.Fatal(err)
	}

	re, _ := reopen(t, doc)
	defer re.Close() //nolint:errcheck

	if !re.HasGlossary() {
		t.Fatal("HasGlossary = false after authoring a building block")
	}
	blocks := re.BuildingBlocks()
	if len(blocks) != 1 {
		t.Fatalf("BuildingBlocks len = %d, want 1", len(blocks))
	}
	b := blocks[0]
	if b.Name() != "MyBlock" {
		t.Errorf("Name = %q, want MyBlock", b.Name())
	}
	if b.Gallery() != "AutoText" {
		t.Errorf("Gallery = %q, want AutoText", b.Gallery())
	}
	if b.Category() != "General" {
		t.Errorf("Category = %q, want General", b.Category())
	}
	if len(b.Types()) != 1 || b.Types()[0] != "autoTxt" {
		t.Errorf("Types = %v, want [autoTxt]", b.Types())
	}
	if b.Description() != "A reusable snippet" {
		t.Errorf("Description = %q", b.Description())
	}
	if b.GUID() != "{1A953087-BC75-4687-B086-946FDC359BC7}" {
		t.Errorf("GUID = %q", b.GUID())
	}
	if rep := re.Validate(); rep.HasErrors() {
		t.Errorf("Validate reported errors: %v", rep)
	}
}

// TestAddBuildingBlockGeneratesGUID checks a fresh GUID is assigned when none is
// supplied.
func TestAddBuildingBlockGeneratesGUID(t *testing.T) {
	doc := Create()
	if err := doc.AddBuildingBlock(BuildingBlockDef{Name: "NoGuid", Gallery: "placeholder"}); err != nil {
		t.Fatal(err)
	}
	re, _ := reopen(t, doc)
	defer re.Close() //nolint:errcheck
	blocks := re.BuildingBlocks()
	if len(blocks) != 1 {
		t.Fatalf("BuildingBlocks len = %d, want 1", len(blocks))
	}
	if g := blocks[0].GUID(); len(g) != 38 || g[0] != '{' || g[37] != '}' {
		t.Errorf("generated GUID = %q, want brace-wrapped 36-char UUID", g)
	}
}

// TestAddBuildingBlockEmptyName rejects an unnamed building block.
func TestAddBuildingBlockEmptyName(t *testing.T) {
	doc := Create()
	if err := doc.AddBuildingBlock(BuildingBlockDef{Gallery: "AutoText"}); err == nil {
		t.Fatal("AddBuildingBlock with empty name = nil error, want error")
	}
}

// TestAddBuildingBlockAppendsToExisting appends a building block to a document
// that already has a glossary, preserving the existing docPart verbatim.
func TestAddBuildingBlockAppendsToExisting(t *testing.T) {
	orig, err := os.ReadFile("testdata/chart.docx")
	if err != nil {
		t.Fatal(err)
	}
	origGlossary, ok := zipEntry(t, orig, "word/glossary/document.xml")
	if !ok {
		t.Fatal("fixture missing glossary part")
	}

	doc, err := Open("testdata/chart.docx")
	if err != nil {
		t.Fatal(err)
	}
	if err := doc.AddBuildingBlock(BuildingBlockDef{
		Name:    "Appended",
		Gallery: "AutoText",
		Types:   []string{"autoTxt"},
		GUID:    "{22222222-2222-2222-2222-222222222222}",
	}); err != nil {
		t.Fatal(err)
	}

	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	newGlossary, ok := zipEntry(t, saved, "word/glossary/document.xml")
	if !ok {
		t.Fatal("glossary part missing after save")
	}

	// The original docPart's bytes must still be present verbatim (only the new
	// docPart is spliced in ahead of </w:docParts>).
	origDocPart := origGlossary[bytes.Index(origGlossary, []byte("<w:docPart>")):bytes.Index(origGlossary, []byte("</w:docPart>"))]
	if !bytes.Contains(newGlossary, origDocPart) {
		t.Fatal("original docPart bytes not preserved after appending a building block")
	}

	re, err := OpenReader(bytes.NewReader(saved), int64(len(saved)))
	if err != nil {
		t.Fatal(err)
	}
	defer re.Close() //nolint:errcheck
	blocks := re.BuildingBlocks()
	if len(blocks) != 2 {
		t.Fatalf("BuildingBlocks len = %d, want 2", len(blocks))
	}
	if blocks[0].Name() != "DefaultPlaceholder_-1854013440" {
		t.Errorf("first block name = %q, want the original", blocks[0].Name())
	}
	if blocks[1].Name() != "Appended" {
		t.Errorf("second block name = %q, want Appended", blocks[1].Name())
	}
	if rep := re.Validate(); rep.HasErrors() {
		t.Errorf("Validate reported errors: %v", rep)
	}
}

// TestGlossaryUnmodifiedRoundTripsByteIdentical guards that opening a document
// with a glossary and NOT authoring anything leaves the part byte-identical.
func TestGlossaryUnmodifiedRoundTripsByteIdentical(t *testing.T) {
	orig, err := os.ReadFile("testdata/chart.docx")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := Open("testdata/chart.docx")
	if err != nil {
		t.Fatal(err)
	}
	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	a, _ := zipEntry(t, orig, "word/glossary/document.xml")
	b, ok := zipEntry(t, saved, "word/glossary/document.xml")
	if !ok || !bytes.Equal(a, b) {
		t.Fatal("unmodified glossary part not byte-identical after round-trip")
	}
}
