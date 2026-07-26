package docx

import (
	"bytes"
	"strings"
	"testing"
)

// openRegenSave opens a fixture, materializes the body (so the save regenerates
// document.xml from the model instead of passing the original bytes through
// verbatim), saves, and returns the regenerated document.xml. This forces the
// marshal path that a real modification save takes — the only path on which the
// content-model preservation bugs surface.
func openRegenSave(t *testing.T, fixture []byte) string {
	t.Helper()
	doc, err := OpenReader(bytes.NewReader(fixture), int64(len(fixture)))
	if err != nil {
		t.Fatal(err)
	}
	// Touch the body to materialize the model, then save. The saved package must
	// reopen cleanly.
	_ = doc.Paragraphs()
	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatal(err)
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

// C259: m:oMath / m:oMathPara nested inside w:ins, w:del, w:hyperlink,
// w:fldSimple, or a run-level w:sdt hit the shared paragraph-content path, which
// had no math case, so d.Skip() deleted the equation whenever document.xml was
// regenerated (any modification save). Word writes exactly this layout for an
// equation inserted with track-changes on. The math must survive byte-for-byte
// inside each wrapper.
func TestNestedOMathPreservedOnSave(t *testing.T) {
	const eq = `<m:oMath><m:r><m:t>x</m:t></m:r></m:oMath>`
	const eqPara = `<m:oMathPara><m:oMath><m:r><m:t>y</m:t></m:r></m:oMath></m:oMathPara>`
	body := `<w:body>` +
		`<w:p><w:ins w:id="1" w:author="a">` + eq + `</w:ins></w:p>` +
		`<w:p><w:del w:id="2" w:author="a">` + eqPara + `</w:del></w:p>` +
		`<w:p><w:hyperlink w:anchor="b">` + eq + `</w:hyperlink></w:p>` +
		`<w:p><w:fldSimple w:instr=" REF x ">` + eq + `</w:fldSimple></w:p>` +
		`<w:sectPr/></w:body>`
	fixture := fixtureWithDocument(t, fixtureWNS+" "+mathNSDecl, body)
	doc := openRegenSave(t, fixture)

	for _, want := range []string{
		`<w:ins w:id="1" w:author="a">` + eq + `</w:ins>`,
		`<w:del w:id="2" w:author="a">` + eqPara + `</w:del>`,
		`<w:hyperlink w:anchor="b">` + eq + `</w:hyperlink>`,
		`<w:fldSimple w:instr=" REF x ">` + eq + `</w:fldSimple>`,
	} {
		if !strings.Contains(doc, want) {
			t.Errorf("saved document.xml lost nested math: %s\ngot: %s", want, doc)
		}
	}
	if strings.Contains(doc, "<oMath") {
		t.Error("oMath re-emitted unprefixed")
	}
}
