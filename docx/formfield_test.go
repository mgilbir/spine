package docx

import (
	"bytes"
	"strings"
	"testing"
)

// TestAddFormFieldTextRoundTrip creates a text form field and verifies both the
// serialized w:ffData and that FormFields() reads it back after a round trip.
func TestAddFormFieldText(t *testing.T) {
	doc := Create()
	p := doc.AddParagraph()
	p.AddText("Name: ")
	run := p.AddFormField(FormFieldOptions{
		Type:        FormFieldText,
		Name:        "FullName",
		DefaultText: "Jane Doe",
		MaxLength:   40,
		HelpText:    "Type your full name",
	})
	if run == nil {
		t.Fatal("AddFormField returned nil run")
	}
	run.SetBold(true)

	data, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	body := zipPart(t, data, "word/document.xml")
	for _, want := range []string{
		`w:fldCharType="begin"`,
		`<w:ffData>`,
		`<w:name w:val="FullName"/>`,
		`<w:textInput>`,
		`<w:default w:val="Jane Doe"/>`,
		`<w:maxLength w:val="40"/>`,
		` FORMTEXT `,
		`w:fldCharType="separate"`,
		`w:fldCharType="end"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("document.xml missing %q\n%s", want, body)
		}
	}

	doc2, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	ffs := doc2.FormFields()
	if len(ffs) != 1 {
		t.Fatalf("FormFields() = %d, want 1", len(ffs))
	}
	got := ffs[0]
	if got.Name != "FullName" || got.Type != FormFieldText {
		t.Errorf("got %+v", got)
	}
	if got.Value != "Jane Doe" {
		t.Errorf("Value = %q, want Jane Doe", got.Value)
	}
	if got.HelpText != "Type your full name" {
		t.Errorf("HelpText = %q", got.HelpText)
	}
}

// TestAddFormFieldCheckBox covers checkbox creation and read-back.
func TestAddFormFieldCheckBox(t *testing.T) {
	doc := Create()
	p := doc.AddParagraph()
	p.AddFormField(FormFieldOptions{Type: FormFieldCheckBox, Name: "Agree", Checked: true})

	data, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	body := zipPart(t, data, "word/document.xml")
	for _, want := range []string{`<w:checkBox>`, `<w:checked w:val="1"/>`, ` FORMCHECKBOX `} {
		if !strings.Contains(body, want) {
			t.Errorf("document.xml missing %q\n%s", want, body)
		}
	}

	doc2, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	ffs := doc2.FormFields()
	if len(ffs) != 1 || ffs[0].Type != FormFieldCheckBox {
		t.Fatalf("FormFields() = %+v", ffs)
	}
	if !ffs[0].Checked || ffs[0].Value != "true" {
		t.Errorf("checkbox not read as checked: %+v", ffs[0])
	}
}

// TestAddFormFieldDropDown covers dropdown creation, entry list, and selection.
func TestAddFormFieldDropDown(t *testing.T) {
	doc := Create()
	p := doc.AddParagraph()
	p.AddFormField(FormFieldOptions{
		Type:     FormFieldDropDown,
		Name:     "Color",
		Entries:  []string{"Red", "Green", "Blue"},
		Selected: 2,
	})

	data, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	body := zipPart(t, data, "word/document.xml")
	for _, want := range []string{
		`<w:ddList>`,
		`<w:result w:val="2"/>`,
		`<w:listEntry w:val="Red"/>`,
		`<w:listEntry w:val="Blue"/>`,
		` FORMDROPDOWN `,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("document.xml missing %q\n%s", want, body)
		}
	}

	doc2, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	ffs := doc2.FormFields()
	if len(ffs) != 1 || ffs[0].Type != FormFieldDropDown {
		t.Fatalf("FormFields() = %+v", ffs)
	}
	got := ffs[0]
	if len(got.Entries) != 3 || got.Selected != 2 || got.Value != "Blue" {
		t.Errorf("dropdown mismatch: %+v", got)
	}
}

// TestFormFieldsEmpty confirms a plain document reports no form fields and that
// an ordinary PAGE field is not mistaken for a form field.
func TestFormFieldsEmpty(t *testing.T) {
	doc := Create()
	p := doc.AddParagraph()
	p.AddText("plain ")
	p.AddField(FieldPage)

	data, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	doc2, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if ffs := doc2.FormFields(); len(ffs) != 0 {
		t.Errorf("FormFields() = %+v, want none", ffs)
	}
}
