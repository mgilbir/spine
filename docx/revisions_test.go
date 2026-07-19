package docx

import (
	"bytes"
	"strings"
	"testing"
)

// openDocFixture opens a body-only fixture and returns the live document.
func openDocFixture(t *testing.T, body string) *Document {
	t.Helper()
	fixture := fixtureWithDocument(t, fixtureWNS, body)
	doc, err := OpenReader(bytes.NewReader(fixture), int64(len(fixture)))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	return doc
}

// saveDocXML saves a document and returns its regenerated document.xml.
func saveDocXML(t *testing.T, doc *Document) string {
	t.Helper()
	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if _, err := OpenReader(bytes.NewReader(saved), int64(len(saved))); err != nil {
		t.Fatalf("saved document does not reopen: %v", err)
	}
	data, ok := zipEntry(t, saved, "word/document.xml")
	if !ok {
		t.Fatal("word/document.xml missing from saved package")
	}
	return string(data)
}

const (
	insBody   = `<w:body><w:p><w:r><w:t xml:space="preserve">keep </w:t></w:r><w:ins w:id="1" w:author="Ann" w:date="2021-01-02T03:04:05Z"><w:r><w:t>added</w:t></w:r></w:ins></w:p></w:body>`
	delBody   = `<w:body><w:p><w:r><w:t xml:space="preserve">keep </w:t></w:r><w:del w:id="2" w:author="Bob" w:date="2021-02-02T00:00:00Z"><w:r><w:delText>gone</w:delText></w:r></w:del></w:p></w:body>`
	rPrBody   = `<w:body><w:p><w:r><w:rPr><w:b/><w:rPrChange w:id="3" w:author="Cid" w:date="2021-03-03T00:00:00Z"><w:rPr><w:i/></w:rPr></w:rPrChange></w:rPr><w:t>styled</w:t></w:r></w:p></w:body>`
	pPrBody   = `<w:body><w:p><w:pPr><w:jc w:val="center"/><w:pPrChange w:id="4" w:author="Dee"><w:pPr><w:jc w:val="left"/></w:pPr></w:pPrChange></w:pPr><w:r><w:t>para</w:t></w:r></w:p></w:body>`
	tableBody = `<w:body><w:tbl><w:tblPr><w:tblStyle w:val="Grid"/><w:tblPrChange w:id="9" w:author="Eve"><w:tblPr/></w:tblPrChange></w:tblPr><w:tr><w:trPr><w:ins w:id="10" w:author="Eve"/></w:trPr><w:tc><w:tcPr><w:cellMerge w:id="11" w:author="Eve" w:vMerge="cont"/></w:tcPr><w:p><w:r><w:t>c</w:t></w:r></w:p></w:tc></w:tr></w:tbl></w:body>`
)

// TestRevisionsRoundTripFidelity is the critical fidelity guarantee: a document
// with tracked changes that is opened and saved WITHOUT accept/reject must
// round-trip byte-identically.
func TestRevisionsRoundTripFidelity(t *testing.T) {
	for _, body := range []string{insBody, delBody, rPrBody, pPrBody, tableBody} {
		want := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
			`<w:document ` + fixtureWNS + `>` + body + `</w:document>`
		doc := openDocFixture(t, body)
		got := saveDocXML(t, doc)
		if got != want {
			t.Fatalf("round-trip not byte-identical.\nwant:\n%s\ngot:\n%s", want, got)
		}
	}
}

func TestRevisionsEnumeration(t *testing.T) {
	body := `<w:body>` +
		strings.TrimPrefix(strings.TrimSuffix(insBody, `</w:body>`), `<w:body>`) +
		strings.TrimPrefix(strings.TrimSuffix(delBody, `</w:body>`), `<w:body>`) +
		strings.TrimPrefix(strings.TrimSuffix(rPrBody, `</w:body>`), `<w:body>`) +
		strings.TrimPrefix(strings.TrimSuffix(pPrBody, `</w:body>`), `<w:body>`) +
		`</w:body>`
	doc := openDocFixture(t, body)
	revs := doc.Revisions()
	if len(revs) != 4 {
		t.Fatalf("want 4 revisions, got %d", len(revs))
	}
	type want struct {
		kind         RevisionType
		author, text string
		editable     bool
	}
	wants := []want{
		{RevisionInsertion, "Ann", "added", true},
		{RevisionDeletion, "Bob", "gone", true},
		{RevisionRunFormat, "Cid", "styled", true},
		{RevisionParagraphFormat, "Dee", "para", true},
	}
	for i, w := range wants {
		r := revs[i]
		if r.Type() != w.kind || r.Author() != w.author || r.Text() != w.text || r.Editable() != w.editable {
			t.Errorf("rev %d: got (%s,%s,%q,%v), want (%s,%s,%q,%v)",
				i, r.Type(), r.Author(), r.Text(), r.Editable(), w.kind, w.author, w.text, w.editable)
		}
	}
	// The insertion carries its recorded date.
	if revs[0].Date() != "2021-01-02T03:04:05Z" {
		t.Errorf("insertion date: got %q", revs[0].Date())
	}
}

func TestAcceptInsertion(t *testing.T) {
	doc := openDocFixture(t, insBody)
	if err := doc.Revisions()[0].Accept(); err != nil {
		t.Fatal(err)
	}
	out := saveDocXML(t, doc)
	if strings.Contains(out, "w:ins") {
		t.Errorf("accepted insertion still wrapped:\n%s", out)
	}
	if !strings.Contains(out, `<w:r><w:t>added</w:t></w:r>`) {
		t.Errorf("accepted insertion content not promoted to run:\n%s", out)
	}
	// The unmodified surrounding run is preserved.
	if !strings.Contains(out, `<w:t xml:space="preserve">keep </w:t>`) {
		t.Errorf("surrounding text lost:\n%s", out)
	}
}

func TestRejectInsertion(t *testing.T) {
	doc := openDocFixture(t, insBody)
	if err := doc.Revisions()[0].Reject(); err != nil {
		t.Fatal(err)
	}
	out := saveDocXML(t, doc)
	if strings.Contains(out, "w:ins") || strings.Contains(out, "added") {
		t.Errorf("rejected insertion content not removed:\n%s", out)
	}
	if !strings.Contains(out, `keep `) {
		t.Errorf("surrounding text lost:\n%s", out)
	}
}

func TestAcceptDeletion(t *testing.T) {
	doc := openDocFixture(t, delBody)
	if err := doc.Revisions()[0].Accept(); err != nil {
		t.Fatal(err)
	}
	out := saveDocXML(t, doc)
	if strings.Contains(out, "w:del") || strings.Contains(out, "gone") {
		t.Errorf("accepted deletion content not removed:\n%s", out)
	}
}

func TestRejectDeletion(t *testing.T) {
	doc := openDocFixture(t, delBody)
	if err := doc.Revisions()[0].Reject(); err != nil {
		t.Fatal(err)
	}
	out := saveDocXML(t, doc)
	if strings.Contains(out, "w:del") || strings.Contains(out, "delText") {
		t.Errorf("rejected deletion still marked:\n%s", out)
	}
	// The deleted text is restored as normal run text.
	if !strings.Contains(out, `<w:t>gone</w:t>`) {
		t.Errorf("rejected deletion text not restored as w:t:\n%s", out)
	}
}

func TestAcceptRunFormat(t *testing.T) {
	doc := openDocFixture(t, rPrBody)
	if err := doc.Revisions()[0].Accept(); err != nil {
		t.Fatal(err)
	}
	out := saveDocXML(t, doc)
	if strings.Contains(out, "rPrChange") {
		t.Errorf("accepted run-format change not dropped:\n%s", out)
	}
	// The new formatting (bold) is kept; the old (italic) is gone.
	if !strings.Contains(out, "<w:b/>") || strings.Contains(out, "<w:i/>") {
		t.Errorf("accept did not keep new run properties:\n%s", out)
	}
}

func TestRejectRunFormat(t *testing.T) {
	doc := openDocFixture(t, rPrBody)
	if err := doc.Revisions()[0].Reject(); err != nil {
		t.Fatal(err)
	}
	out := saveDocXML(t, doc)
	if strings.Contains(out, "rPrChange") {
		t.Errorf("rejected run-format change not dropped:\n%s", out)
	}
	// The old formatting (italic) is restored; the new (bold) is gone.
	if !strings.Contains(out, "<w:i/>") || strings.Contains(out, "<w:b/>") {
		t.Errorf("reject did not restore old run properties:\n%s", out)
	}
	if !strings.Contains(out, "styled") {
		t.Errorf("run text lost on reject:\n%s", out)
	}
}

func TestAcceptParagraphFormat(t *testing.T) {
	doc := openDocFixture(t, pPrBody)
	if err := doc.Revisions()[0].Accept(); err != nil {
		t.Fatal(err)
	}
	out := saveDocXML(t, doc)
	if strings.Contains(out, "pPrChange") {
		t.Errorf("accepted paragraph-format change not dropped:\n%s", out)
	}
	if !strings.Contains(out, `<w:jc w:val="center"/>`) {
		t.Errorf("accept did not keep new paragraph properties:\n%s", out)
	}
}

func TestRejectParagraphFormat(t *testing.T) {
	doc := openDocFixture(t, pPrBody)
	if err := doc.Revisions()[0].Reject(); err != nil {
		t.Fatal(err)
	}
	out := saveDocXML(t, doc)
	if strings.Contains(out, "pPrChange") {
		t.Errorf("rejected paragraph-format change not dropped:\n%s", out)
	}
	if !strings.Contains(out, `<w:jc w:val="left"/>`) || strings.Contains(out, `w:val="center"`) {
		t.Errorf("reject did not restore old paragraph properties:\n%s", out)
	}
}

func TestAcceptAllRevisions(t *testing.T) {
	body := `<w:body><w:p>` +
		`<w:ins w:id="1" w:author="A"><w:r><w:t>ins </w:t></w:r></w:ins>` +
		`<w:del w:id="2" w:author="A"><w:r><w:delText>del</w:delText></w:r></w:del>` +
		`<w:r><w:t> plain</w:t></w:r></w:p></w:body>`
	doc := openDocFixture(t, body)
	if err := doc.AcceptAllRevisions(); err != nil {
		t.Fatal(err)
	}
	out := saveDocXML(t, doc)
	if strings.Contains(out, "w:ins") || strings.Contains(out, "w:del") {
		t.Errorf("accept-all left revision markers:\n%s", out)
	}
	// Insertion kept, deletion removed, plain kept.
	if !strings.Contains(out, "ins ") || strings.Contains(out, "del") || !strings.Contains(out, " plain") {
		t.Errorf("accept-all produced wrong content:\n%s", out)
	}
	if len(doc.Revisions()) != 0 {
		t.Errorf("revisions remain after accept-all: %d", len(doc.Revisions()))
	}
}

func TestRejectAllRevisions(t *testing.T) {
	body := `<w:body><w:p>` +
		`<w:ins w:id="1" w:author="A"><w:r><w:t>ins </w:t></w:r></w:ins>` +
		`<w:del w:id="2" w:author="A"><w:r><w:delText>del</w:delText></w:r></w:del>` +
		`<w:r><w:t> plain</w:t></w:r></w:p></w:body>`
	doc := openDocFixture(t, body)
	if err := doc.RejectAllRevisions(); err != nil {
		t.Fatal(err)
	}
	out := saveDocXML(t, doc)
	if strings.Contains(out, "w:ins") || strings.Contains(out, "w:del") || strings.Contains(out, "delText") {
		t.Errorf("reject-all left revision markers:\n%s", out)
	}
	// Insertion removed, deletion restored, plain kept.
	if strings.Contains(out, "ins ") || !strings.Contains(out, "del") || !strings.Contains(out, " plain") {
		t.Errorf("reject-all produced wrong content:\n%s", out)
	}
	if len(doc.Revisions()) != 0 {
		t.Errorf("revisions remain after reject-all: %d", len(doc.Revisions()))
	}
}

// TestTableRevisionsReadOnly verifies structural table revisions are reported
// but not transformable.
func TestTableRevisionsReadOnly(t *testing.T) {
	doc := openDocFixture(t, tableBody)
	var seen []RevisionType
	for _, r := range doc.Revisions() {
		seen = append(seen, r.Type())
		if r.Editable() {
			continue
		}
		if err := r.Accept(); err == nil {
			t.Errorf("read-only revision %s accepted without error", r.Type())
		}
		if err := r.Reject(); err == nil {
			t.Errorf("read-only revision %s rejected without error", r.Type())
		}
	}
	// Table property change, row insertion, and cell merge are all reported.
	wantKinds := map[RevisionType]bool{
		RevisionTableFormat: true, RevisionRowInsertion: true, RevisionCellMerge: true,
	}
	for k := range wantKinds {
		found := false
		for _, s := range seen {
			if s == k {
				found = true
			}
		}
		if !found {
			t.Errorf("expected to enumerate %s; saw %v", k, seen)
		}
	}
}

// TestRevisionInTableCell confirms enumeration and transform reach content
// nested in table cells.
func TestRevisionInTableCell(t *testing.T) {
	body := `<w:body><w:tbl><w:tblPr/><w:tr><w:tc><w:tcPr/>` +
		`<w:p><w:ins w:id="1" w:author="A"><w:r><w:t>celltext</w:t></w:r></w:ins></w:p>` +
		`</w:tc></w:tr></w:tbl></w:body>`
	doc := openDocFixture(t, body)
	revs := doc.Revisions()
	if len(revs) != 1 || revs[0].Type() != RevisionInsertion {
		t.Fatalf("want 1 insertion in cell, got %#v", revs)
	}
	if err := revs[0].Accept(); err != nil {
		t.Fatal(err)
	}
	out := saveDocXML(t, doc)
	if strings.Contains(out, "w:ins") || !strings.Contains(out, "celltext") {
		t.Errorf("cell insertion not accepted correctly:\n%s", out)
	}
}
