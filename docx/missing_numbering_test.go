package docx

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mgilbir/spine/common/validate"
)

// numberingRelXML is a document.xml.rels entry pointing at a numbering part
// that is not present in the package — the dangling-numbering-rel class real
// Common Crawl files exhibit. Word opens such files, ignoring the dead rel.
const numberingRelXML = `<Relationship Id="rId5" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/numbering" Target="numbering.xml"/>`

// A referenced-but-absent numbering part is OPTIONAL: Open tolerates it (unlike
// a missing header/footer), the document's paragraphs stay readable, and the
// tolerated defect surfaces as a warning rather than an Open error.
func TestOpenToleratesMissingNumberingPart(t *testing.T) {
	fixture := fixtureWithDocRels(t, numberingRelXML, nil)

	doc, err := OpenReader(bytes.NewReader(fixture), int64(len(fixture)))
	if err != nil {
		t.Fatalf("Open errored on a dangling numbering rel: %v", err)
	}
	defer doc.Close() //nolint:errcheck

	if got := len(doc.Paragraphs()); got != 1 {
		t.Fatalf("Paragraphs() = %d, want 1", got)
	}

	rep := doc.Validate()
	if rep.HasErrors() {
		t.Fatalf("Validate reported errors on a tolerated dangling numbering rel: %v", rep.Errors())
	}
	// The dangling numbering rel is surfaced as a rel-target-missing warning.
	if !hasCode(rep, validate.CodeRelTargetMissing, validate.SeverityWarning) {
		t.Errorf("expected a %q warning for the dangling numbering rel, got: %v",
			validate.CodeRelTargetMissing, rep.Warnings())
	}
}

// A zero-modification save preserves the dangling numbering rel byte-for-byte:
// tolerating it at Open must not drop the dead relationship on save.
func TestMissingNumberingRelZeroModSavePreservesRel(t *testing.T) {
	fixture := fixtureWithDocRels(t, numberingRelXML, nil)

	doc, err := OpenReader(bytes.NewReader(fixture), int64(len(fixture)))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	origRels := readZipPart(t, fixture, "word/_rels/document.xml.rels")
	savedRels := readZipPart(t, saved, "word/_rels/document.xml.rels")
	if !bytes.Equal(origRels, savedRels) {
		t.Errorf("document.xml.rels changed on zero-modification save:\nwant: %s\ngot:  %s", origRels, savedRels)
	}
	if !strings.Contains(string(savedRels), `Target="numbering.xml"`) {
		t.Errorf("dangling numbering rel dropped on save:\n%s", savedRels)
	}

	// The saved package reopens cleanly (still tolerating the dangling rel).
	if _, err := OpenReader(bytes.NewReader(saved), int64(len(saved))); err != nil {
		t.Fatalf("saved document does not reopen: %v", err)
	}
}

// Negative test: the essential main document.xml part is not relaxed. A package
// missing document.xml must still fail Open.
func TestOpenErrorsOnMissingDocumentPart(t *testing.T) {
	fixture := buildFixtureDocx(t, map[string]string{
		"[Content_Types].xml": fixtureContentTypes,
		"_rels/.rels":         fixtureRootRels,
		// no word/document.xml
	})
	if _, err := OpenReader(bytes.NewReader(fixture), int64(len(fixture))); err == nil {
		t.Fatal("Open succeeded on a package missing the main document.xml part")
	}
}
