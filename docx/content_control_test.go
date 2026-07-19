package docx

import (
	"bytes"
	"strings"
	"testing"
)

// findControl returns the first content control whose tag matches, or nil.
func findControl(ctrls []*ContentControl, tag string) *ContentControl {
	for _, c := range ctrls {
		if c.Tag() == tag {
			return c
		}
	}
	return nil
}

// TestContentControlSdtPrByteIdentical is the core fidelity guard for the
// sdtPr retype: a block SDT carrying an unmodeled run-property child, a data
// binding, the typed tag/id, and a drop-down control must round-trip its
// document.xml fragment byte-for-byte.
func TestContentControlSdtPrByteIdentical(t *testing.T) {
	sdt := `<w:sdt>` +
		`<w:sdtPr>` +
		`<w:rPr><w:b/><w:color w:val="FF0000"/></w:rPr>` +
		`<w:alias w:val="Choice"/>` +
		`<w:tag w:val="dd"/>` +
		`<w:id w:val="123456"/>` +
		`<w:lock w:val="sdtContentLocked"/>` +
		`<w:dataBinding w:xpath="/root[1]/pick[1]" w:storeItemID="{FOO}"/>` +
		`<w:dropDownList w:lastValue="A">` +
		`<w:listItem w:displayText="Alpha" w:value="A"/>` +
		`<w:listItem w:displayText="Beta" w:value="B"/>` +
		`</w:dropDownList>` +
		`</w:sdtPr>` +
		`<w:sdtEndPr/>` +
		`<w:sdtContent><w:p><w:r><w:t>Alpha</w:t></w:r></w:p></w:sdtContent>` +
		`</w:sdt>`
	fixture := fixtureWithDocument(t, fixtureWNS, `<w:body>`+sdt+`</w:body>`)
	out := openSave(t, fixture)
	if !strings.Contains(out, sdt) {
		t.Fatalf("sdt fragment not byte-identical after round-trip.\nwant substring:\n%s\ngot:\n%s", sdt, out)
	}
}

// TestContentControlInlineByteIdentical guards the inline (w:sdtRun) path and
// its empty w:sdtEndPr self-close.
func TestContentControlInlineByteIdentical(t *testing.T) {
	sdt := `<w:sdt>` +
		`<w:sdtPr><w:rPr><w:sz w:val="22"/></w:rPr><w:tag w:val="cite"/><w:id w:val="-1"/>` +
		`<w:placeholder><w:docPart w:val="X"/></w:placeholder></w:sdtPr>` +
		`<w:sdtEndPr/>` +
		`<w:sdtContent><w:r><w:t>text</w:t></w:r></w:sdtContent>` +
		`</w:sdt>`
	fixture := fixtureWithDocument(t, fixtureWNS, `<w:body><w:p>`+sdt+`</w:p></w:body>`)
	out := openSave(t, fixture)
	if !strings.Contains(out, sdt) {
		t.Fatalf("inline sdt fragment not byte-identical.\nwant substring:\n%s\ngot:\n%s", sdt, out)
	}
}

// TestContentControlCheckboxByteIdentical guards the w14:checkbox control
// (foreign-namespace element captured verbatim).
func TestContentControlCheckboxByteIdentical(t *testing.T) {
	sdt := `<w:sdt><w:sdtPr><w:tag w:val="cb"/>` +
		`<w14:checkbox><w14:checked w14:val="1"/><w14:checkedState w14:val="2612"/><w14:uncheckedState w14:val="2610"/></w14:checkbox>` +
		`</w:sdtPr><w:sdtContent><w:p><w:r><w:t>x</w:t></w:r></w:p></w:sdtContent></w:sdt>`
	fixture := fixtureWithDocument(t, fixtureWNS14, `<w:body>`+sdt+`</w:body>`)
	out := openSave(t, fixture)
	if !strings.Contains(out, sdt) {
		t.Fatalf("checkbox sdt not byte-identical.\nwant substring:\n%s\ngot:\n%s", sdt, out)
	}
}

// TestContentControlReadTypedFields verifies the typed API surface reads a
// drop-down control's tag, alias, id, type, options, and value.
func TestContentControlReadTypedFields(t *testing.T) {
	sdt := `<w:sdt><w:sdtPr>` +
		`<w:alias w:val="Pick one"/><w:tag w:val="dd"/><w:id w:val="42"/>` +
		`<w:dropDownList><w:listItem w:displayText="Alpha" w:value="A"/><w:listItem w:displayText="Beta" w:value="B"/></w:dropDownList>` +
		`</w:sdtPr><w:sdtContent><w:p><w:r><w:t>Alpha</w:t></w:r></w:p></w:sdtContent></w:sdt>`
	fixture := fixtureWithDocument(t, fixtureWNS, `<w:body>`+sdt+`</w:body>`)
	doc, err := OpenReader(bytes.NewReader(fixture), int64(len(fixture)))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()

	ctrls := doc.ContentControls()
	c := findControl(ctrls, "dd")
	if c == nil {
		t.Fatalf("content control with tag dd not found; got %d controls", len(ctrls))
	}
	if got := c.Alias(); got != "Pick one" {
		t.Errorf("Alias = %q, want %q", got, "Pick one")
	}
	if got := c.ID(); got != "42" {
		t.Errorf("ID = %q, want %q", got, "42")
	}
	if got := c.Type(); got != ContentControlDropDownList {
		t.Errorf("Type = %q, want %q", got, ContentControlDropDownList)
	}
	if got := c.Value(); got != "Alpha" {
		t.Errorf("Value = %q, want %q", got, "Alpha")
	}
	opts := c.Options()
	if len(opts) != 2 || opts[0].DisplayText != "Alpha" || opts[0].Value != "A" || opts[1].Value != "B" {
		t.Errorf("Options = %+v, want [Alpha/A Beta/B]", opts)
	}
	if c.IsInline() {
		t.Error("block control reported as inline")
	}
}

// TestContentControlDateAndCheckbox verifies the date-format and checkbox
// accessors parse their preserved control children.
func TestContentControlDateAndCheckbox(t *testing.T) {
	body := `<w:body>` +
		`<w:sdt><w:sdtPr><w:tag w:val="d"/><w:date w:fullDate="2020-01-01T00:00:00Z"><w:dateFormat w:val="M/d/yyyy"/><w:lid w:val="en-US"/></w:date></w:sdtPr>` +
		`<w:sdtContent><w:p><w:r><w:t>1/1/2020</w:t></w:r></w:p></w:sdtContent></w:sdt>` +
		`<w:sdt><w:sdtPr><w:tag w:val="c"/><w14:checkbox><w14:checked w14:val="1"/></w14:checkbox></w:sdtPr>` +
		`<w:sdtContent><w:p><w:r><w:t>x</w:t></w:r></w:p></w:sdtContent></w:sdt>` +
		`</w:body>`
	fixture := fixtureWithDocument(t, fixtureWNS14, body)
	doc, err := OpenReader(bytes.NewReader(fixture), int64(len(fixture)))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc.Close() }()

	ctrls := doc.ContentControls()
	d := findControl(ctrls, "d")
	if d == nil || d.Type() != ContentControlDate {
		t.Fatalf("date control not found/typed: %+v", d)
	}
	if got := d.DateFormat(); got != "M/d/yyyy" {
		t.Errorf("DateFormat = %q, want %q", got, "M/d/yyyy")
	}
	cb := findControl(ctrls, "c")
	if cb == nil || cb.Type() != ContentControlCheckbox {
		t.Fatalf("checkbox control not found/typed: %+v", cb)
	}
	checked, ok := cb.Checked()
	if !ok || !checked {
		t.Errorf("Checked = (%v,%v), want (true,true)", checked, ok)
	}
}

// TestContentControlSetTagAlias verifies mutating tag/alias survives a
// round-trip and preserves the surrounding unmodeled children.
func TestContentControlSetTagAlias(t *testing.T) {
	sdt := `<w:sdt><w:sdtPr><w:rPr><w:b/></w:rPr><w:tag w:val="old"/><w:id w:val="7"/></w:sdtPr>` +
		`<w:sdtContent><w:p><w:r><w:t>hi</w:t></w:r></w:p></w:sdtContent></w:sdt>`
	fixture := fixtureWithDocument(t, fixtureWNS, `<w:body>`+sdt+`</w:body>`)
	doc, err := OpenReader(bytes.NewReader(fixture), int64(len(fixture)))
	if err != nil {
		t.Fatal(err)
	}
	c := findControl(doc.ContentControls(), "old")
	if c == nil {
		t.Fatal("control not found")
	}
	c.SetTag("new")
	c.SetAlias("Friendly")
	c.SetValue("bye")

	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	_ = doc.Close()
	doc2, err := OpenReader(bytes.NewReader(saved), int64(len(saved)))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = doc2.Close() }()

	c2 := findControl(doc2.ContentControls(), "new")
	if c2 == nil {
		t.Fatalf("control with new tag not found after save")
	}
	if got := c2.Alias(); got != "Friendly" {
		t.Errorf("Alias = %q, want Friendly", got)
	}
	if got := c2.Value(); got != "bye" {
		t.Errorf("Value = %q, want bye", got)
	}
	data, _ := zipEntry(t, saved, "word/document.xml")
	if !bytes.Contains(data, []byte(`<w:rPr><w:b/></w:rPr>`)) {
		t.Errorf("unmodeled rPr child lost after tag edit:\n%s", data)
	}
	if !bytes.Contains(data, []byte(`<w:id w:val="7"/>`)) {
		t.Errorf("id child lost after tag edit:\n%s", data)
	}
}

// TestContentControlCreateBlock verifies programmatic block-level creation.
func TestContentControlCreateBlock(t *testing.T) {
	doc := Create()
	doc.AddParagraphWithText("intro")
	c := doc.AddContentControl("mytag", "hello")
	c.SetAlias("My Field")

	data, err := doc.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = reopened.Close() }()

	got := findControl(reopened.ContentControls(), "mytag")
	if got == nil {
		t.Fatalf("created content control not found after reopen")
	}
	if got.Alias() != "My Field" {
		t.Errorf("Alias = %q, want My Field", got.Alias())
	}
	if got.Type() != ContentControlRichText {
		t.Errorf("Type = %q, want richText", got.Type())
	}
	if got.Value() != "hello" {
		t.Errorf("Value = %q, want hello", got.Value())
	}
}

// TestContentControlCreateInline verifies programmatic inline creation inside a
// paragraph.
func TestContentControlCreateInline(t *testing.T) {
	doc := Create()
	p := doc.AddParagraph()
	p.AddRun().SetText("before ")
	c := p.AddContentControl("inlinetag", "value")
	if !c.IsInline() {
		t.Error("inline control not reported as inline")
	}

	data, err := doc.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = reopened.Close() }()

	got := findControl(reopened.ContentControls(), "inlinetag")
	if got == nil {
		t.Fatalf("created inline content control not found after reopen")
	}
	if !got.IsInline() {
		t.Error("reopened control not inline")
	}
	if got.Value() != "value" {
		t.Errorf("Value = %q, want value", got.Value())
	}
}
