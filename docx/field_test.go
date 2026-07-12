package docx

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
)

// zipPart returns the named entry of a saved package as a string.
func zipPart(t *testing.T, data []byte, name string) string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("reading zip: %v", err)
	}
	for _, f := range zr.File {
		if f.Name == name {
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("opening %s: %v", name, err)
			}
			defer func() { _ = rc.Close() }()
			var b bytes.Buffer
			if _, err := b.ReadFrom(rc); err != nil {
				t.Fatalf("reading %s: %v", name, err)
			}
			return b.String()
		}
	}
	t.Fatalf("part %s not found in package", name)
	return ""
}

// TestFooterPageFields is the feature's headline scenario: "Page X of Y" in a
// footer via PAGE and NUMPAGES simple fields.
func TestFooterPageFields(t *testing.T) {
	doc := Create()
	doc.AddParagraphWithText("body")

	footer := doc.AddFooter(FooterDefault)
	p := footer.AddParagraph()
	p.AddText("Page ")
	p.AddField(FieldPage)
	p.AddText(" of ")
	p.AddField(FieldNumPages)

	data, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	ftr := zipPart(t, data, "word/footer1.xml")
	for _, want := range []string{
		`<w:fldSimple w:instr=" PAGE "`,
		`<w:fldSimple w:instr=" NUMPAGES "`,
	} {
		if !strings.Contains(ftr, want) {
			t.Errorf("footer1.xml missing %s\n%s", want, ftr)
		}
	}
	// Interleaving must hold: text, field, text, field.
	iPage := strings.Index(ftr, `w:instr=" PAGE "`)
	iOf := strings.Index(ftr, ">Page <")
	iNum := strings.Index(ftr, `w:instr=" NUMPAGES "`)
	if iOf >= iPage || iPage >= iNum {
		t.Errorf("field/text order wrong: %q", ftr)
	}

	// The document must reopen cleanly.
	if _, err := OpenReader(bytes.NewReader(data), int64(len(data))); err != nil {
		t.Fatalf("reopen: %v", err)
	}
}

// TestAddFieldReturnsFormattableRun: the cached-result run belongs to the
// field and takes run formatting.
func TestAddFieldReturnsFormattableRun(t *testing.T) {
	doc := Create()
	p := doc.AddParagraph()
	r := p.AddField(FieldPage)
	r.SetBold(true)

	data, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	body := zipPart(t, data, "word/document.xml")
	// The bold run must live inside the fldSimple element.
	fld := body[strings.Index(body, "<w:fldSimple"):]
	fld = fld[:strings.Index(fld, "</w:fldSimple>")]
	if !strings.Contains(fld, "<w:b/>") {
		t.Errorf("field result run lost formatting: %s", fld)
	}
}

// TestAddFieldCustomInstruction: FieldType passes arbitrary instructions
// through, XML-escaped.
func TestAddFieldCustomInstruction(t *testing.T) {
	doc := Create()
	p := doc.AddParagraph()
	p.AddField(FieldType(`DATE \@ "yyyy \"and\" MM"`))

	data, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	body := zipPart(t, data, "word/document.xml")
	if !strings.Contains(body, `DATE \@ &#34;yyyy`) && !strings.Contains(body, `DATE \@ &quot;yyyy`) {
		t.Errorf("custom instruction not present/escaped in document.xml: %s", body)
	}
	if _, err := OpenReader(bytes.NewReader(data), int64(len(data))); err != nil {
		t.Fatalf("reopen after quoted instruction: %v", err)
	}
}

// TestFieldSurvivesOpenedDocumentEdit: fields added to a document, saved,
// reopened and re-saved (the childOrder-gated marshal path) must survive.
func TestFieldSurvivesOpenedDocumentEdit(t *testing.T) {
	doc := Create()
	p := doc.AddParagraph()
	p.AddText("Page ")
	p.AddField(FieldPage)

	data, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	doc2, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	// Mutate something unrelated, then save: the parsed paragraph's field
	// must round-trip through the childOrder-gated marshal.
	doc2.AddParagraphWithText("appended after reopen")
	data2, err := doc2.SaveBytes()
	if err != nil {
		t.Fatalf("second SaveBytes: %v", err)
	}
	body := zipPart(t, data2, "word/document.xml")
	if !strings.Contains(body, `<w:fldSimple w:instr=" PAGE "`) {
		t.Errorf("field lost on reopen+save: %s", body)
	}
	iText := strings.Index(body, ">Page <")
	iFld := strings.Index(body, "<w:fldSimple")
	if iText < 0 || iText >= iFld {
		t.Errorf("text/field order lost on reopen+save")
	}
}

// TestAddFieldAppendedToParsedParagraph: appending a field to a paragraph
// that came from a parsed document must not drop existing content.
func TestAddFieldAppendedToParsedParagraph(t *testing.T) {
	doc := Create()
	doc.AddParagraphWithText("existing text")
	data, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	doc2, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	paras := doc2.Paragraphs()
	if len(paras) == 0 {
		t.Fatal("no paragraphs after reopen")
	}
	paras[0].AddField(FieldNumPages)

	data2, err := doc2.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes after append: %v", err)
	}
	body := zipPart(t, data2, "word/document.xml")
	if !strings.Contains(body, "existing text") {
		t.Errorf("existing text dropped when appending a field to a parsed paragraph")
	}
	if !strings.Contains(body, `w:instr=" NUMPAGES "`) {
		t.Errorf("appended field missing")
	}
}
