package docx

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAddHeader(t *testing.T) {
	doc := Create()
	hdr := doc.AddHeader(HeaderDefault)
	hdr.AddParagraphWithText("Header Text")

	if doc.document.Body.SectPr == nil {
		t.Fatal("expected SectPr to be set")
	}
	if len(doc.document.Body.SectPr.HeaderReference) != 1 {
		t.Fatalf("expected 1 header reference, got %d", len(doc.document.Body.SectPr.HeaderReference))
	}
	ref := doc.document.Body.SectPr.HeaderReference[0]
	if ref.Type != "default" {
		t.Errorf("Type = %s, want default", ref.Type)
	}
	if ref.RID == "" {
		t.Error("expected RID to be set")
	}
}

func TestAddFooter(t *testing.T) {
	doc := Create()
	ftr := doc.AddFooter(FooterDefault)
	ftr.AddParagraphWithText("Footer Text")

	if len(doc.document.Body.SectPr.FooterReference) != 1 {
		t.Fatalf("expected 1 footer reference, got %d", len(doc.document.Body.SectPr.FooterReference))
	}
	ref := doc.document.Body.SectPr.FooterReference[0]
	if ref.Type != "default" {
		t.Errorf("Type = %s, want default", ref.Type)
	}
}

func TestAddFirstPageHeader(t *testing.T) {
	doc := Create()
	doc.AddHeader(HeaderFirst)

	// First page header should enable TitlePg
	if doc.document.Body.SectPr.TitlePg == nil {
		t.Error("expected TitlePg to be set for first page header")
	}
}

func TestHeaderFooterSave(t *testing.T) {
	doc := Create()

	hdr := doc.AddHeader(HeaderDefault)
	hdr.AddParagraphWithText("My Header")

	ftr := doc.AddFooter(FooterDefault)
	ftr.AddParagraphWithText("Page Footer")

	doc.AddParagraphWithText("Document body text")

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "hdrftr.docx")
	if err := doc.Save(path); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() == 0 {
		t.Fatal("output file is empty")
	}
}

func TestMultipleHeadersFooters(t *testing.T) {
	doc := Create()

	doc.AddHeader(HeaderDefault)
	doc.AddHeader(HeaderFirst)
	doc.AddFooter(FooterDefault)
	doc.AddFooter(FooterFirst)

	if len(doc.document.Body.SectPr.HeaderReference) != 2 {
		t.Fatalf("expected 2 header references, got %d", len(doc.document.Body.SectPr.HeaderReference))
	}
	if len(doc.document.Body.SectPr.FooterReference) != 2 {
		t.Fatalf("expected 2 footer references, got %d", len(doc.document.Body.SectPr.FooterReference))
	}

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "multi_hdrftr.docx")
	if err := doc.Save(path); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Size() == 0 {
		t.Fatal("output file is empty")
	}
}

func TestHeaderFooterReopen(t *testing.T) {
	doc := Create()
	hdr := doc.AddHeader(HeaderDefault)
	hdr.AddParagraphWithText("Test Header")
	doc.AddParagraphWithText("Body content")

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "reopen_hdr.docx")
	if err := doc.Save(path); err != nil {
		t.Fatal(err)
	}

	doc2, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer doc2.Close() //nolint:errcheck

	// Document should have at least the body paragraph
	paras := doc2.Paragraphs()
	if len(paras) == 0 {
		t.Fatal("expected at least 1 paragraph")
	}
}
