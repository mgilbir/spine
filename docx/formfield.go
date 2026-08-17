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
// body (including inside tables, content controls, hyperlinks and tracked
// changes) and in every header and footer, in document order. Each field is
// reconstructed from its w:fldChar begin/separate/end run sequence and the
// w:ffData definition carried on the begin field character.
//
// The field state machine runs over each part as a whole, so a field whose
// begin and end runs sit in different paragraphs is still read as one field.
func (d *Document) FormFields() []FormField {
	if d.doc() == nil || d.doc().Body == nil {
		return nil
	}
	var out []FormField
	collect := func(paras []*oxml.CT_P) {
		st := &formFieldScan{out: &out}
		for _, p := range paras {
			st.paragraph(p)
		}
		st.finalize()
	}
	collect(d.doc().Body.AllParagraphs())
	for _, hp := range d.sortedHeaderParts() {
		if hp != nil && hp.hdr != nil {
			collect(hp.hdr.AllParagraphs())
		}
	}
	for _, fp := range d.sortedFooterParts() {
		if fp != nil && fp.ftr != nil {
			collect(fp.ftr.AllParagraphs())
		}
	}
	return out
}

// formFieldScan is the w:fldChar state machine: a begin field character
// carrying w:ffData opens a form field, the separate character switches to
// capturing the displayed result, and the end character closes it. The state
// lives across paragraphs so a field split at a paragraph boundary is not lost.
type formFieldScan struct {
	out       *[]FormField
	ff        *oxml.CT_FFData
	capturing bool
	value     strings.Builder
}

// paragraph feeds one paragraph's runs through the scan, descending into
// hyperlinks, tracked changes, inline content controls and simple fields — all
// of which EG_PContent allows a form field's runs to sit in (C498).
func (st *formFieldScan) paragraph(p *oxml.CT_P) {
	if p == nil {
		return
	}
	oxml.VisitContent(p, oxml.ContentVisitor{Run: st.run})
}

// run advances the state machine over a single run.
func (st *formFieldScan) run(r *oxml.CT_R) {
	if r == nil {
		return
	}
	for _, fc := range r.FldChar {
		switch fc.FldCharType {
		case "begin":
			st.finalize() // defensive: close any unterminated prior field
			st.ff = parseFFData(fc)
		case "separate":
			if st.ff != nil {
				st.capturing = true
			}
		case "end":
			st.finalize()
		}
	}
	if st.capturing {
		for _, t := range r.T {
			st.value.WriteString(t.Text)
		}
	}
}

// finalize emits the field currently being scanned, if any, and resets.
func (st *formFieldScan) finalize() {
	if st.ff != nil {
		*st.out = append(*st.out, buildFormField(st.ff, st.value.String()))
	}
	st.ff = nil
	st.capturing = false
	st.value.Reset()
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
		//xmlguard:lenient wrapped is a synthesized wrapper this function just built, parsed straight back; it never came off a package
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

	cp := p.mut()
	cp.AppendR(begin)
	cp.AppendR(instr)
	cp.AppendR(separate)
	cp.AppendR(result)
	cp.AppendR(end)
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
