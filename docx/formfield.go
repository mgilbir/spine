package docx

import (
	"encoding/xml"
	"strconv"
	"strings"

	"github.com/mgilbir/spine/docx/internal/oxml"
)

// FormFieldType classifies a legacy Word form field (the kind inserted from the
// Developer > Legacy Tools palette), encoded as a complex field whose begin
// w:fldChar carries a w:ffData definition.
type FormFieldType string

const (
	// FormFieldText is a text-input form field (w:textInput, FORMTEXT).
	FormFieldText FormFieldType = "text"
	// FormFieldCheckBox is a checkbox form field (w:checkBox, FORMCHECKBOX).
	FormFieldCheckBox FormFieldType = "checkbox"
	// FormFieldDropDown is a drop-down list form field (w:ddList, FORMDROPDOWN).
	FormFieldDropDown FormFieldType = "dropdown"
)

// FormField is a legacy Word form field extracted from a document. It pairs the
// field's w:ffData definition (name, kind, options) with the current result the
// field displays. Extraction is read-only and leaves the underlying runs
// byte-for-byte unchanged on a subsequent save.
type FormField struct {
	// Name is the field's bookmark name (w:ffData/w:name/@w:val), used by macros
	// and REF fields to address the field. It may be empty.
	Name string
	// Type is the field kind: text, checkbox, or dropdown.
	Type FormFieldType
	// Value is the field's current result: the displayed text for a text field,
	// "true"/"false" for a checkbox, or the selected entry for a dropdown.
	Value string
	// Checked reports a checkbox field's state; always false for other kinds.
	Checked bool
	// Entries lists a dropdown field's choices in order; nil for other kinds.
	Entries []string
	// Selected is a dropdown field's selected index into Entries; 0 otherwise.
	Selected int
	// HelpText and StatusText are the optional help/status strings (w:helpText,
	// w:statusText); empty when the field declares none.
	HelpText   string
	StatusText string
}

// FormFields returns the legacy form fields present anywhere in the document
// body (including inside tables) and in every header and footer, in document
// order. Each field is reconstructed from its w:fldChar begin/separate/end run
// sequence and the w:ffData definition carried on the begin field character.
func (d *Document) FormFields() []FormField {
	if d.document == nil || d.document.Body == nil {
		return nil
	}
	var out []FormField
	for _, p := range d.document.Body.AllParagraphs() {
		collectParagraphFormFields(p, &out)
	}
	for _, name := range sortedKeys(d.headers) {
		if hp := d.headers[name]; hp != nil && hp.hdr != nil {
			for _, p := range hp.hdr.AllParagraphs() {
				collectParagraphFormFields(p, &out)
			}
		}
	}
	for _, name := range sortedKeys(d.footers) {
		if fp := d.footers[name]; fp != nil && fp.ftr != nil {
			for _, p := range fp.ftr.AllParagraphs() {
				collectParagraphFormFields(p, &out)
			}
		}
	}
	return out
}

// collectParagraphFormFields appends the form fields found in a paragraph,
// scanning its top-level runs and the runs inside any hyperlinks.
func collectParagraphFormFields(p *oxml.CT_P, out *[]FormField) {
	if p == nil {
		return
	}
	collectRunFormFields(p.R, out)
	for _, h := range p.Hyperlink {
		if h != nil {
			collectRunFormFields(h.R, out)
		}
	}
}

// collectRunFormFields runs the w:fldChar state machine over a run sequence: a
// begin field character carrying w:ffData opens a form field, the separate
// character switches to capturing the displayed result, and the end character
// closes it.
func collectRunFormFields(runs []*oxml.CT_R, out *[]FormField) {
	var (
		ff        *oxml.CT_FFData
		capturing bool
		value     strings.Builder
	)
	finalize := func() {
		if ff != nil {
			*out = append(*out, buildFormField(ff, value.String()))
		}
		ff = nil
		capturing = false
		value.Reset()
	}
	for _, r := range runs {
		if r == nil {
			continue
		}
		for _, fc := range r.FldChar {
			switch fc.FldCharType {
			case "begin":
				finalize() // defensive: close any unterminated prior field
				ff = parseFFData(fc)
			case "separate":
				if ff != nil {
					capturing = true
				}
			case "end":
				finalize()
			}
		}
		if capturing {
			for _, t := range r.T {
				value.WriteString(t.Text)
			}
		}
	}
	finalize()
}

// parseFFData decodes the w:ffData definition preserved raw on a begin field
// character, or nil when the character carries none (so it is an ordinary
// complex field such as PAGE or a hyperlink, not a form field).
func parseFFData(fc *oxml.CT_FldChar) *oxml.CT_FFData {
	if fc == nil {
		return nil
	}
	for _, rn := range fc.Raw {
		if rn == nil || rn.Local != "ffData" {
			continue
		}
		// Reconstruct a self-contained element around the preserved inner XML so
		// the standard decoder can resolve the w: prefix its children use.
		wrapped := []byte(`<w:ffData xmlns:w="` + oxml.NsWml + `">` + string(rn.RawContent) + `</w:ffData>`)
		var ff oxml.CT_FFData
		if err := xml.Unmarshal(wrapped, &ff); err != nil {
			return nil
		}
		return &ff
	}
	return nil
}

// buildFormField assembles a FormField from a parsed w:ffData definition and the
// text captured between the field's separate and end characters.
func buildFormField(ff *oxml.CT_FFData, resultText string) FormField {
	out := FormField{HelpText: ffText(ff.HelpText), StatusText: ffStatusText(ff.StatusText)}
	if ff.Name != nil {
		out.Name = ff.Name.Val
	}
	switch {
	case ff.CheckBox != nil:
		out.Type = FormFieldCheckBox
		checked := ff.CheckBox.Checked.IsOn()
		if ff.CheckBox.Checked == nil {
			checked = ff.CheckBox.Default.IsOn()
		}
		out.Checked = checked
		out.Value = strconv.FormatBool(checked)
	case ff.DdList != nil:
		out.Type = FormFieldDropDown
		for _, e := range ff.DdList.ListEntry {
			out.Entries = append(out.Entries, e.Val)
		}
		sel := 0
		if ff.DdList.Result != nil {
			sel = ff.DdList.Result.Val
		} else if ff.DdList.Default != nil {
			sel = ff.DdList.Default.Val
		}
		out.Selected = sel
		if sel >= 0 && sel < len(out.Entries) {
			out.Value = out.Entries[sel]
		}
	default:
		out.Type = FormFieldText
		out.Value = resultText
		if out.Value == "" && ff.TextInput != nil && ff.TextInput.Default != nil {
			out.Value = ff.TextInput.Default.Val
		}
	}
	return out
}

func ffText(h *oxml.CT_FFHelpText) string {
	if h == nil {
		return ""
	}
	return h.Val
}

func ffStatusText(s *oxml.CT_FFStatusText) string {
	if s == nil {
		return ""
	}
	return s.Val
}

// --- authoring ---

// FormFieldOptions configures a legacy form field created with AddFormField.
// The zero value is a nameless, enabled text field.
type FormFieldOptions struct {
	// Type selects the field kind; the empty value means FormFieldText.
	Type FormFieldType
	// Name is the field's bookmark name (w:ffData/w:name). Optional.
	Name string
	// HelpText and StatusText populate the optional w:helpText/w:statusText
	// (help-key and status-bar prompts). Optional.
	HelpText   string
	StatusText string

	// DefaultText is a text field's initial value (w:textInput/w:default) and
	// the result shown until the user edits the field.
	DefaultText string
	// MaxLength caps a text field's length (w:textInput/w:maxLength); 0 means no
	// limit.
	MaxLength int

	// Checked sets a checkbox field's initial state.
	Checked bool

	// Entries lists a dropdown field's choices in order (w:ddList/w:listEntry).
	Entries []string
	// Selected is the dropdown's selected index into Entries.
	Selected int
}

// AddFormField appends a legacy Word form field to the paragraph and returns the
// run that holds the field's displayed result (format it to style the field).
// The field is emitted as the standard begin/separate/end w:fldChar run
// sequence with a w:ffData definition on the begin character, so Word treats it
// as a fillable form field and Document.FormFields() reports it after a
// save/open round trip.
func (p *Paragraph) AddFormField(opts FormFieldOptions) *Run {
	typ := opts.Type
	if typ == "" {
		typ = FormFieldText
	}

	begin := &oxml.CT_R{}
	begin.AppendFldChar(&oxml.CT_FldChar{
		FldCharType: "begin",
		Raw:         []*oxml.CT_RawNamedElement{ffDataElement(typ, opts)},
	})

	instr := &oxml.CT_R{}
	instr.AppendInstrText(&oxml.CT_Text{Space: "preserve", Text: " " + formFieldInstr(typ) + " "})

	separate := &oxml.CT_R{}
	separate.AppendFldChar(&oxml.CT_FldChar{FldCharType: "separate"})

	result := &oxml.CT_R{}
	result.SetTexts([]*oxml.CT_Text{{Space: "preserve", Text: formFieldResult(typ, opts)}})

	end := &oxml.CT_R{}
	end.AppendFldChar(&oxml.CT_FldChar{FldCharType: "end"})

	p.p.AppendR(begin)
	p.p.AppendR(instr)
	p.p.AppendR(separate)
	p.p.AppendR(result)
	p.p.AppendR(end)
	return &Run{paragraph: p, r: result}
}

// formFieldInstr returns the field instruction keyword for a form-field kind.
func formFieldInstr(t FormFieldType) string {
	switch t {
	case FormFieldCheckBox:
		return "FORMCHECKBOX"
	case FormFieldDropDown:
		return "FORMDROPDOWN"
	default:
		return "FORMTEXT"
	}
}

// formFieldResult returns the text shown between the separate and end field
// characters for a newly created field. Word displays five non-breaking spaces
// for an empty text field, the selected entry for a dropdown, and nothing for a
// checkbox (which renders from the checkBox definition).
func formFieldResult(t FormFieldType, opts FormFieldOptions) string {
	switch t {
	case FormFieldCheckBox:
		return ""
	case FormFieldDropDown:
		if opts.Selected >= 0 && opts.Selected < len(opts.Entries) {
			return opts.Entries[opts.Selected]
		}
		return ""
	default:
		if opts.DefaultText != "" {
			return opts.DefaultText
		}
		return "     "
	}
}

// ffDataElement builds the w:ffData definition child for a begin field
// character, serialized to the verbatim inner XML the raw run marshaler emits.
func ffDataElement(t FormFieldType, opts FormFieldOptions) *oxml.CT_RawNamedElement {
	var b strings.Builder
	if opts.Name != "" {
		b.WriteString(`<w:name w:val="` + xmlEscapeAttr(opts.Name) + `"/>`)
	}
	b.WriteString(`<w:enabled/>`)
	b.WriteString(`<w:calcOnExit w:val="0"/>`)
	if opts.HelpText != "" {
		b.WriteString(`<w:helpText w:type="text" w:val="` + xmlEscapeAttr(opts.HelpText) + `"/>`)
	}
	if opts.StatusText != "" {
		b.WriteString(`<w:statusText w:type="text" w:val="` + xmlEscapeAttr(opts.StatusText) + `"/>`)
	}
	switch t {
	case FormFieldCheckBox:
		b.WriteString(`<w:checkBox><w:sizeAuto/>`)
		if opts.Checked {
			b.WriteString(`<w:default w:val="1"/><w:checked w:val="1"/>`)
		} else {
			b.WriteString(`<w:default w:val="0"/>`)
		}
		b.WriteString(`</w:checkBox>`)
	case FormFieldDropDown:
		b.WriteString(`<w:ddList>`)
		if opts.Selected > 0 {
			b.WriteString(`<w:result w:val="` + strconv.Itoa(opts.Selected) + `"/>`)
		}
		for _, e := range opts.Entries {
			b.WriteString(`<w:listEntry w:val="` + xmlEscapeAttr(e) + `"/>`)
		}
		b.WriteString(`</w:ddList>`)
	default:
		b.WriteString(`<w:textInput>`)
		if opts.DefaultText != "" {
			b.WriteString(`<w:default w:val="` + xmlEscapeAttr(opts.DefaultText) + `"/>`)
		}
		if opts.MaxLength > 0 {
			b.WriteString(`<w:maxLength w:val="` + strconv.Itoa(opts.MaxLength) + `"/>`)
		}
		b.WriteString(`</w:textInput>`)
	}
	return &oxml.CT_RawNamedElement{
		Local:         "ffData",
		Space:         oxml.NsWml,
		CT_RawElement: oxml.CT_RawElement{RawContent: []byte(b.String())},
	}
}
