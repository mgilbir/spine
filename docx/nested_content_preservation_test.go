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

// C260: w:dir / w:bdo are EG_PContent bidi-embedding wrappers holding run
// content. Untyped by the model, they hit the default d.Skip() in
// CT_P.UnmarshalXML, so the visible text inside was deleted on any regeneration.
// The wrapper and its runs must survive verbatim.
func TestBidiWrapperPreservedOnSave(t *testing.T) {
	const dir = `<w:dir w:val="rtl"><w:r><w:t>shalom</w:t></w:r></w:dir>`
	const bdo = `<w:bdo w:val="ltr"><w:r><w:t>abc</w:t></w:r></w:bdo>`
	body := `<w:body>` +
		`<w:p>` + dir + `</w:p>` +
		`<w:p>` + bdo + `</w:p>` +
		`<w:sectPr/></w:body>`
	fixture := fixtureWithDocument(t, fixtureWNS, body)
	doc := openRegenSave(t, fixture)

	for _, want := range []string{dir, bdo, "shalom", "abc"} {
		if !strings.Contains(doc, want) {
			t.Errorf("saved document.xml lost bidi wrapper content: %s\ngot: %s", want, doc)
		}
	}
}

// C261: CT_SimpleField bound its lock attribute to a non-existent w:lock, so
// w:fldLock="true" (which pins a field against recalculation) was stripped on
// save, silently making a locked field editable; it also dropped the w:fldData
// custom field-data child and any unmodeled attribute. The correct w:fldLock
// mapping, raw-preserved fldData, and CapturedAttrs must all survive.
func TestSimpleFieldLockAndFldDataPreservedOnSave(t *testing.T) {
	const fldData = `<w:fldData xml:space="preserve">QQ==</w:fldData>`
	field := `<w:fldSimple w:instr=" REF _Ref1 " w:fldLock="true" w:dirty="true">` +
		fldData +
		`<w:r><w:t>cached</w:t></w:r></w:fldSimple>`
	body := `<w:body><w:p>` + field + `</w:p><w:sectPr/></w:body>`
	fixture := fixtureWithDocument(t, fixtureWNS, body)
	doc := openRegenSave(t, fixture)

	if !strings.Contains(doc, `w:fldLock="true"`) {
		t.Errorf("w:fldLock dropped (locked field became editable):\n%s", doc)
	}
	if !strings.Contains(doc, fldData) {
		t.Errorf("w:fldData child dropped:\n%s", doc)
	}
	if !strings.Contains(doc, "cached") {
		t.Errorf("field result run dropped:\n%s", doc)
	}
	// The whole field, including the source attribute order (instr, fldLock,
	// dirty), must round-trip verbatim.
	if !strings.Contains(doc, field) {
		t.Errorf("fldSimple not preserved verbatim:\nwant %s\ngot  %s", field, doc)
	}
}
