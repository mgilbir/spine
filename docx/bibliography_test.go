package docx

import (
	"bytes"
	"strings"
	"testing"
)

// TestAddSourceAndCitationRoundTrip is the headline scenario: add a source,
// cite it, save, reopen, and read both back.
func TestAddSourceAndCitationRoundTrip(t *testing.T) {
	doc := Create()
	if err := doc.AddSource(Source{
		Tag:       "Smi20",
		Type:      SourceBook,
		Author:    "Smith, John",
		Title:     "A Fine Book",
		Year:      "2020",
		City:      "New York",
		Publisher: "ACME",
	}); err != nil {
		t.Fatalf("AddSource: %v", err)
	}
	p := doc.AddParagraph()
	p.AddText("As shown ")
	p.AddCitation("Smi20")

	data, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	// The sources part must exist and carry the source.
	sources := zipPart(t, data, "word/bibliography/sources.xml")
	for _, want := range []string{
		`<b:Sources`,
		`<b:Source>`,
		`<b:Tag>Smi20</b:Tag>`,
		`<b:SourceType>Book</b:SourceType>`,
		`<b:Title>A Fine Book</b:Title>`,
		`<b:Year>2020</b:Year>`,
		`<b:Last>Smith</b:Last>`,
		`<b:First>John</b:First>`,
	} {
		if !strings.Contains(sources, want) {
			t.Errorf("sources.xml missing %q\n%s", want, sources)
		}
	}

	// The citation field must be in the body.
	body := zipPart(t, data, "word/document.xml")
	if !strings.Contains(body, `<w:fldSimple w:instr=" CITATION Smi20 "`) {
		t.Errorf("document.xml missing CITATION field\n%s", body)
	}
	if !strings.Contains(body, `(Smith, 2020)`) {
		t.Errorf("document.xml missing citation placeholder\n%s", body)
	}

	// The document.xml relationships must reference the bibliography part.
	rels := zipPart(t, data, "word/_rels/document.xml.rels")
	if !strings.Contains(rels, "bibliography/sources.xml") {
		t.Errorf("document.xml.rels missing bibliography relationship\n%s", rels)
	}

	// Reopen and read the source back.
	doc2, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	got := doc2.Sources()
	if len(got) != 1 {
		t.Fatalf("Sources() = %d, want 1", len(got))
	}
	s := got[0]
	if s.Tag != "Smi20" || s.Title != "A Fine Book" || s.Year != "2020" || s.Author != "Smith, John" || s.Type != "Book" {
		t.Errorf("read source mismatch: %+v", s)
	}
	if rep := doc2.Validate(); rep.HasErrors() {
		t.Errorf("Validate reported errors: %v", rep)
	}
}

// TestSourcesSurviveReopenResave: sources added, saved, reopened and re-saved
// must round-trip.
func TestSourcesSurviveReopenResave(t *testing.T) {
	doc := Create()
	if err := doc.AddSource(Source{Tag: "Doe19", Author: "Doe, Jane", Title: "T", Year: "2019"}); err != nil {
		t.Fatalf("AddSource: %v", err)
	}
	data, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	doc2, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	// Mutate something unrelated, then save.
	doc2.AddParagraphWithText("appended")
	data2, err := doc2.SaveBytes()
	if err != nil {
		t.Fatalf("second SaveBytes: %v", err)
	}
	doc3, err := OpenReader(bytes.NewReader(data2), int64(len(data2)))
	if err != nil {
		t.Fatalf("second reopen: %v", err)
	}
	if got := doc3.Sources(); len(got) != 1 || got[0].Tag != "Doe19" {
		t.Errorf("source lost on reopen+save: %+v", got)
	}
}

// TestAddSourceReplacesByTag: re-adding a source with an existing tag updates
// it in place rather than duplicating.
func TestAddSourceReplacesByTag(t *testing.T) {
	doc := Create()
	_ = doc.AddSource(Source{Tag: "K1", Title: "Old", Year: "2000"})
	_ = doc.AddSource(Source{Tag: "K1", Title: "New", Year: "2001"})
	got := doc.Sources()
	if len(got) != 1 {
		t.Fatalf("expected 1 source after replace, got %d", len(got))
	}
	if got[0].Title != "New" || got[0].Year != "2001" {
		t.Errorf("source not replaced: %+v", got[0])
	}
}

// TestAddSourceEmptyTagRejected: an untagged source is rejected.
func TestAddSourceEmptyTagRejected(t *testing.T) {
	doc := Create()
	if err := doc.AddSource(Source{Title: "No tag"}); err == nil {
		t.Fatal("expected error for empty tag")
	}
}

// TestRemoveSource removes by tag.
func TestRemoveSource(t *testing.T) {
	doc := Create()
	_ = doc.AddSource(Source{Tag: "A"})
	_ = doc.AddSource(Source{Tag: "B"})
	if !doc.RemoveSource("A") {
		t.Fatal("RemoveSource(A) = false")
	}
	if doc.RemoveSource("A") {
		t.Fatal("RemoveSource(A) twice = true")
	}
	got := doc.Sources()
	if len(got) != 1 || got[0].Tag != "B" {
		t.Errorf("after remove: %+v", got)
	}
}

// TestCorporateAuthor: an author with no comma is a corporate author.
func TestCorporateAuthor(t *testing.T) {
	doc := Create()
	_ = doc.AddSource(Source{Tag: "Org1", Author: "World Health Organization", Title: "Report", Year: "2021"})
	data, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	sources := zipPart(t, data, "word/bibliography/sources.xml")
	if !strings.Contains(sources, `<b:Corporate>World Health Organization</b:Corporate>`) {
		t.Errorf("missing corporate author\n%s", sources)
	}
	doc2, _ := OpenReader(bytes.NewReader(data), int64(len(data)))
	if got := doc2.Sources(); len(got) != 1 || got[0].Author != "World Health Organization" {
		t.Errorf("corporate author round-trip: %+v", got)
	}
}

// TestMultipleAuthors: several "Last, First" names separated by ";".
func TestMultipleAuthors(t *testing.T) {
	doc := Create()
	_ = doc.AddSource(Source{Tag: "M2", Author: "Smith, John; Doe, Jane", Title: "T", Year: "2022"})
	data, _ := doc.SaveBytes()
	doc2, _ := OpenReader(bytes.NewReader(data), int64(len(data)))
	got := doc2.Sources()
	if len(got) != 1 || got[0].Author != "Smith, John; Doe, Jane" {
		t.Errorf("multi-author round-trip: %+v", got)
	}
}

// TestCitationUnknownSourcePlaceholder: citing a tag with no source shows the
// tag as the placeholder.
func TestCitationUnknownSourcePlaceholder(t *testing.T) {
	doc := Create()
	p := doc.AddParagraph()
	p.AddCitation("Ghost")
	data, _ := doc.SaveBytes()
	body := zipPart(t, data, "word/document.xml")
	if !strings.Contains(body, `>(Ghost)<`) {
		t.Errorf("unknown-source placeholder missing\n%s", body)
	}
}
