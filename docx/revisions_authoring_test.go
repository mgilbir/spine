package docx

import (
	"strings"
	"testing"
	"time"
)

// fixedRevDate is a stable timestamp so authored markup is deterministic.
var fixedRevDate = time.Date(2021, 5, 6, 7, 8, 9, 0, time.UTC)

const fixedRevDateStr = "2021-05-06T07:08:09Z"

// TestAuthorInsertedRunRoundTrip authors an insertion, saves, reopens, and
// verifies it is enumerated and that AcceptAllRevisions keeps the text but drops
// the marker.
func TestAuthorInsertedRunRoundTrip(t *testing.T) {
	doc := Create()
	p := doc.AddParagraphWithText("keep ")
	run := p.AddInsertedRunWithDate("Ann", "added", fixedRevDate)
	if run == nil || run.Text() != "added" {
		t.Fatalf("AddInsertedRun returned unexpected run: %+v", run)
	}

	// The authored markup is well-formed w:ins wrapping a run with w:t.
	xml := saveDocXML(t, doc)
	if !strings.Contains(xml, `<w:ins w:id="1" w:author="Ann" w:date="`+fixedRevDateStr+`"><w:r><w:t xml:space="preserve">added</w:t></w:r></w:ins>`) {
		t.Fatalf("authored insertion markup wrong:\n%s", xml)
	}

	// It reads back as an editable insertion.
	fresh := reopenDoc(t, doc)
	revs := fresh.Revisions()
	if len(revs) != 1 {
		t.Fatalf("want 1 revision after reopen, got %d", len(revs))
	}
	r := revs[0]
	if r.Type() != RevisionInsertion || r.Author() != "Ann" || r.Text() != "added" || r.Date() != fixedRevDateStr {
		t.Fatalf("reopened insertion wrong: (%s,%s,%q,%s)", r.Type(), r.Author(), r.Text(), r.Date())
	}

	// Accepting keeps the text and drops the marker.
	if err := fresh.AcceptAllRevisions(); err != nil {
		t.Fatal(err)
	}
	out := saveDocXML(t, fresh)
	if strings.Contains(out, "w:ins") {
		t.Errorf("accepted insertion still wrapped:\n%s", out)
	}
	if fresh.Body() != "keep added" {
		t.Errorf("accepted text wrong: %q", fresh.Body())
	}
}

// TestMarkInsertedWrapsExistingRun authors an insertion by wrapping an existing
// run, then verifies round-trip and reject (removal).
func TestMarkInsertedWrapsExistingRun(t *testing.T) {
	doc := Create()
	p := doc.AddParagraph()
	r := p.AddRun()
	r.SetText("wrapped")
	if got := r.MarkInsertedWithDate("Bob", fixedRevDate); got != r {
		t.Fatal("MarkInserted should return the same run for chaining")
	}

	xml := saveDocXML(t, doc)
	if !strings.Contains(xml, `<w:ins w:id="1" w:author="Bob" w:date="`+fixedRevDateStr+`"><w:r><w:t xml:space="preserve">wrapped</w:t></w:r></w:ins>`) {
		t.Fatalf("wrapped insertion markup wrong:\n%s", xml)
	}

	fresh := reopenDoc(t, doc)
	revs := fresh.Revisions()
	if len(revs) != 1 || revs[0].Type() != RevisionInsertion || revs[0].Text() != "wrapped" {
		t.Fatalf("reopened insertion wrong: %+v", revs)
	}
	// Rejecting removes the inserted content.
	if err := fresh.RejectAllRevisions(); err != nil {
		t.Fatal(err)
	}
	out := saveDocXML(t, fresh)
	if strings.Contains(out, "w:ins") || strings.Contains(out, "wrapped") {
		t.Errorf("rejected insertion not removed:\n%s", out)
	}
}

// TestMarkDeletedRoundTrip authors a deletion, verifies the w:del/w:delText
// structure, and checks accept (removal) and reject (restoration).
func TestMarkDeletedRoundTrip(t *testing.T) {
	build := func() *Document {
		doc := Create()
		p := doc.AddParagraphWithText("remove me")
		p.Runs()[0].MarkDeletedWithDate("Cid", fixedRevDate)
		return doc
	}

	// Authored markup: w:del wrapping a run whose text became w:delText.
	xml := saveDocXML(t, build())
	if !strings.Contains(xml, `<w:del w:id="1" w:author="Cid" w:date="`+fixedRevDateStr+`"><w:r><w:delText xml:space="preserve">remove me</w:delText></w:r></w:del>`) {
		t.Fatalf("authored deletion markup wrong:\n%s", xml)
	}

	// Reads back as an editable deletion.
	fresh := reopenDoc(t, build())
	revs := fresh.Revisions()
	if len(revs) != 1 || revs[0].Type() != RevisionDeletion || revs[0].Author() != "Cid" || revs[0].Text() != "remove me" {
		t.Fatalf("reopened deletion wrong: %+v", revs)
	}

	// Accepting the deletion removes the content.
	accept := reopenDoc(t, build())
	if err := accept.AcceptAllRevisions(); err != nil {
		t.Fatal(err)
	}
	if out := saveDocXML(t, accept); strings.Contains(out, "w:del") || strings.Contains(out, "remove me") {
		t.Errorf("accepted deletion not removed:\n%s", out)
	}

	// Rejecting the deletion restores the text as a normal run.
	reject := reopenDoc(t, build())
	if err := reject.RejectAllRevisions(); err != nil {
		t.Fatal(err)
	}
	out := saveDocXML(t, reject)
	if strings.Contains(out, "w:del") || strings.Contains(out, "delText") {
		t.Errorf("rejected deletion still marked:\n%s", out)
	}
	if reject.Body() != "remove me" {
		t.Errorf("rejected deletion text not restored: %q", reject.Body())
	}
}

// TestAuthoredRevisionIDsMonotonic verifies each authored revision gets a unique
// id, starting above any id already present in the document.
func TestAuthoredRevisionIDsMonotonic(t *testing.T) {
	// Existing document already carries an insertion with w:id="1".
	doc := openDocFixture(t, insBody)
	p := doc.Paragraphs()[0]
	p.AddInsertedRunWithDate("Ann", "x", fixedRevDate) // -> id 2
	p.AddInsertedRunWithDate("Ann", "y", fixedRevDate) // -> id 3

	xml := saveDocXML(t, doc)
	for _, id := range []string{`w:id="1"`, `w:id="2"`, `w:id="3"`} {
		if strings.Count(xml, id) != 1 {
			t.Errorf("expected exactly one %s, got %d\n%s", id, strings.Count(xml, id), xml)
		}
	}

	// A fresh document starts numbering at 1.
	fresh := Create()
	fp := fresh.AddParagraph()
	fr := fp.AddRun()
	fr.SetText("a")
	fr.MarkInsertedWithDate("Bob", fixedRevDate)
	if out := saveDocXML(t, fresh); !strings.Contains(out, `w:id="1"`) {
		t.Errorf("fresh document should start ids at 1:\n%s", out)
	}
}

// TestAuthoredInsertionDefaultDate verifies the default (now) date variant emits
// a parseable ISO-8601 UTC timestamp.
func TestAuthoredInsertionDefaultDate(t *testing.T) {
	doc := Create()
	p := doc.AddParagraph()
	before := time.Now().UTC().Add(-2 * time.Second)
	p.AddInsertedRun("Ann", "now")
	after := time.Now().UTC().Add(2 * time.Second)

	rev := reopenDoc(t, doc).Revisions()[0]
	got, err := time.Parse(revisionDateFormat, rev.Date())
	if err != nil {
		t.Fatalf("default date not ISO-8601: %q (%v)", rev.Date(), err)
	}
	if got.Before(before.Truncate(time.Second)) || got.After(after) {
		t.Errorf("default date %v outside [%v, %v]", got, before, after)
	}
}
