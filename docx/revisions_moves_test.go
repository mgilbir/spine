package docx

import (
	"strings"
	"testing"
)

// moveBody is a body carrying a complete tracked move: a w:moveFrom source in
// one paragraph and a matching w:moveTo destination in another, each bracketed
// by move range markers sharing the name "m1".
const moveBody = `<w:body>` +
	`<w:p>` +
	`<w:moveFromRangeStart w:id="1" w:author="Ann" w:date="2021-01-02T03:04:05Z" w:name="m1"/>` +
	`<w:moveFrom w:id="2" w:author="Ann" w:date="2021-01-02T03:04:05Z"><w:r><w:t>moved text</w:t></w:r></w:moveFrom>` +
	`<w:moveFromRangeEnd w:id="1"/>` +
	`</w:p>` +
	`<w:p><w:r><w:t xml:space="preserve">before </w:t></w:r>` +
	`<w:moveToRangeStart w:id="3" w:author="Ann" w:date="2021-01-02T03:04:05Z" w:name="m1"/>` +
	`<w:moveTo w:id="4" w:author="Ann" w:date="2021-01-02T03:04:05Z"><w:r><w:t>moved text</w:t></w:r></w:moveTo>` +
	`<w:moveToRangeEnd w:id="3"/>` +
	`</w:p>` +
	`</w:body>`

// TestTrackedMoveRoundTripFidelity verifies a document carrying a tracked move
// that is opened and saved without accept/reject round-trips byte-identically.
func TestTrackedMoveRoundTripFidelity(t *testing.T) {
	want := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
		`<w:document ` + fixtureWNS + `>` + moveBody + `</w:document>`
	doc := openDocFixture(t, moveBody)
	if got := saveDocXML(t, doc); got != want {
		t.Fatalf("tracked move round-trip not byte-identical.\nwant:\n%s\ngot:\n%s", want, got)
	}
}

// TestTrackedMoveEnumeration verifies both halves of a move are enumerated with
// the right type, author, text, and paired name.
func TestTrackedMoveEnumeration(t *testing.T) {
	doc := openDocFixture(t, moveBody)
	revs := doc.Revisions()
	if len(revs) != 2 {
		t.Fatalf("want 2 move revisions, got %d: %+v", len(revs), revs)
	}
	from, to := revs[0], revs[1]
	if from.Type() != RevisionMoveFrom || to.Type() != RevisionMoveTo {
		t.Fatalf("wrong move types: %s, %s", from.Type(), to.Type())
	}
	for _, r := range revs {
		if r.Author() != "Ann" || r.Text() != "moved text" || r.MoveName() != "m1" {
			t.Errorf("move metadata wrong: author=%q text=%q name=%q", r.Author(), r.Text(), r.MoveName())
		}
		if !r.Editable() {
			t.Errorf("move revision should be editable")
		}
	}
}

// TestTrackedMoveAccept accepts a move: the destination content stays as normal
// text and the source content is dropped.
func TestTrackedMoveAccept(t *testing.T) {
	doc := openDocFixture(t, moveBody)
	if err := doc.AcceptAllRevisions(); err != nil {
		t.Fatal(err)
	}
	out := saveDocXML(t, doc)
	if strings.Contains(out, "w:moveFrom>") || strings.Contains(out, "<w:moveTo ") {
		t.Errorf("move wrappers survived accept:\n%s", out)
	}
	// Destination kept: "before moved text"; source dropped.
	if doc.Body() != "before moved text" {
		t.Errorf("accepted move text wrong: %q", doc.Body())
	}
}

// TestTrackedMoveReject rejects a move: the source content is restored and the
// destination content is dropped.
func TestTrackedMoveReject(t *testing.T) {
	doc := openDocFixture(t, moveBody)
	if err := doc.RejectAllRevisions(); err != nil {
		t.Fatal(err)
	}
	out := saveDocXML(t, doc)
	if strings.Contains(out, "w:moveFrom>") || strings.Contains(out, "<w:moveTo ") {
		t.Errorf("move wrappers survived reject:\n%s", out)
	}
	// Source restored, destination dropped: "moved text" (para 1) then "before "
	// (para 2), joined by Body's paragraph separator.
	if doc.Body() != "moved text\nbefore " {
		t.Errorf("rejected move text wrong: %q", doc.Body())
	}
}

// TestTrackedMoveAcceptOne accepts only the destination half via the per-revision
// API, keeping the moveTo text and leaving the source untouched.
func TestTrackedMoveAcceptOne(t *testing.T) {
	doc := openDocFixture(t, moveBody)
	revs := doc.Revisions()
	to := revs[1]
	if to.Type() != RevisionMoveTo {
		t.Fatalf("expected moveTo second, got %s", to.Type())
	}
	if err := to.Accept(); err != nil {
		t.Fatal(err)
	}
	out := saveDocXML(t, doc)
	if strings.Contains(out, "<w:moveTo ") {
		t.Errorf("accepted moveTo still wrapped:\n%s", out)
	}
	if !strings.Contains(out, "<w:moveFrom ") {
		t.Errorf("untouched moveFrom should remain:\n%s", out)
	}
}

// TestAuthorTrackedMoveRoundTrip authors a move, saves, reopens, enumerates, and
// accepts it.
func TestAuthorTrackedMoveRoundTrip(t *testing.T) {
	doc := Create()
	src := doc.AddParagraphWithText("source ")
	src.AddMoveFromRunWithDate("Ann", "mv", "cut", fixedRevDate)
	dst := doc.AddParagraphWithText("dest ")
	dst.AddMoveToRunWithDate("Ann", "mv", "cut", fixedRevDate)

	xml := saveDocXML(t, doc)
	if !strings.Contains(xml, `<w:moveFromRangeStart w:id="1" w:author="Ann" w:date="`+fixedRevDateStr+`" w:name="mv"/>`) {
		t.Fatalf("authored moveFromRangeStart wrong:\n%s", xml)
	}
	if !strings.Contains(xml, `<w:moveFrom w:id="2" w:author="Ann" w:date="`+fixedRevDateStr+`"><w:r><w:t xml:space="preserve">cut</w:t></w:r></w:moveFrom>`) {
		t.Fatalf("authored moveFrom wrong:\n%s", xml)
	}
	if !strings.Contains(xml, `<w:moveToRangeStart w:id="3" w:author="Ann" w:date="`+fixedRevDateStr+`" w:name="mv"/>`) {
		t.Fatalf("authored moveToRangeStart wrong:\n%s", xml)
	}
	if !strings.Contains(xml, `<w:moveTo w:id="4" w:author="Ann" w:date="`+fixedRevDateStr+`"><w:r><w:t xml:space="preserve">cut</w:t></w:r></w:moveTo>`) {
		t.Fatalf("authored moveTo wrong:\n%s", xml)
	}

	fresh := reopenDoc(t, doc)
	revs := fresh.Revisions()
	if len(revs) != 2 || revs[0].Type() != RevisionMoveFrom || revs[1].Type() != RevisionMoveTo {
		t.Fatalf("reopened move revisions wrong: %+v", revs)
	}
	if revs[0].MoveName() != "mv" || revs[1].Text() != "cut" {
		t.Errorf("reopened move metadata wrong: name=%q text=%q", revs[0].MoveName(), revs[1].Text())
	}
	if err := fresh.AcceptAllRevisions(); err != nil {
		t.Fatal(err)
	}
	if got := fresh.Body(); got != "source \ndest cut" {
		t.Errorf("accepted authored move text wrong: %q", got)
	}
}
