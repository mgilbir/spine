package docx

import "github.com/mgilbir/spine/docx/internal/oxml"

// ContentControlType identifies the control kind of a content control
// (structured document tag). The empty value reports a plain container with no
// explicit control-type child (a rich-text SDT).
type ContentControlType string

const (
	// ContentControlUnspecified is a container SDT with no control-type child.
	ContentControlUnspecified ContentControlType = ""
	// ContentControlRichText is a rich-text control (w:richText).
	ContentControlRichText ContentControlType = "richText"
	// ContentControlText is a plain-text control (w:text).
	ContentControlText ContentControlType = "text"
	// ContentControlDropDownList is a drop-down list control (w:dropDownList).
	ContentControlDropDownList ContentControlType = "dropDownList"
	// ContentControlComboBox is a combo-box control (w:comboBox).
	ContentControlComboBox ContentControlType = "comboBox"
	// ContentControlCheckbox is a Word 2010 checkbox control (w14:checkbox).
	ContentControlCheckbox ContentControlType = "checkbox"
	// ContentControlDate is a date-picker control (w:date).
	ContentControlDate ContentControlType = "date"
	// ContentControlPicture is a picture control (w:picture).
	ContentControlPicture ContentControlType = "picture"
)

// ContentControlOption is one selectable item of a drop-down or combo-box
// content control.
type ContentControlOption struct {
	// DisplayText is the label shown to the user.
	DisplayText string
	// Value is the underlying stored value.
	Value string
}

// ContentControl is a structured document tag (w:sdt) — Word's content control.
// It wraps either a block-level control (w:sdt in the body, spanning whole
// paragraphs or tables) or an inline control (w:sdtRun, within a paragraph).
// Exactly one of the underlying representations is set.
type ContentControl struct {
	doc   *Document
	block *oxml.CT_SdtBlock
	run   *oxml.CT_SdtRun
}

// ContentControls returns every content control in the document body in
// document order, including inline controls, controls nested inside other
// controls, and controls inside tables.
func (d *Document) ContentControls() []*ContentControl {
	if d.doc() == nil || d.doc().Body == nil {
		return nil
	}
	refs := d.doc().Body.ContentControls()
	out := make([]*ContentControl, 0, len(refs))
	for _, ref := range refs {
		out = append(out, &ContentControl{doc: d, block: ref.Block, run: ref.Run})
	}
	return out
}

// IsInline reports whether the control is an inline (run-level) control rather
// than a block-level one.
func (c *ContentControl) IsInline() bool { return c.run != nil }

// props returns the control's properties, or nil when absent.
func (c *ContentControl) props() *oxml.CT_SdtPr {
	if c.run != nil {
		return c.run.SdtPr
	}
	if c.block != nil {
		return c.block.SdtPr
	}
	return nil
}

// ensureProps returns the control's properties, creating them if absent.
func (c *ContentControl) ensureProps() *oxml.CT_SdtPr {
	if c.run != nil {
		return c.run.EnsurePr()
	}
	return c.block.EnsurePr()
}

// Tag returns the control's programmatic tag (w:tag), or "" when unset.
func (c *ContentControl) Tag() string { return c.props().TagValue() }

// SetTag sets the control's tag (w:tag). Passing "" removes it.
func (c *ContentControl) SetTag(tag string) { c.ensureProps().SetTag(tag) }

// Alias returns the control's friendly name (w:alias), or "" when unset.
func (c *ContentControl) Alias() string { return c.props().AliasValue() }

// SetAlias sets the control's friendly name (w:alias). Passing "" removes it.
func (c *ContentControl) SetAlias(alias string) { c.ensureProps().SetAlias(alias) }

// ID returns the control's w:id value, or "" when unset.
func (c *ContentControl) ID() string { return c.props().IDValue() }

// Type returns the control's kind. It reports ContentControlUnspecified for a
// plain rich-text container with no explicit control-type child.
func (c *ContentControl) Type() ContentControlType {
	pr := c.props()
	if pr == nil || pr.Control == nil {
		return ContentControlUnspecified
	}
	return ContentControlType(pr.Control.Local)
}

// Value returns the control's current text (the concatenated text of its
// content runs).
func (c *ContentControl) Value() string {
	if c.run != nil {
		return c.run.ContentText()
	}
	return c.block.ContentText()
}

// SetValue replaces the control's content with a single run of text. Existing
// content (and its run formatting) is discarded.
func (c *ContentControl) SetValue(text string) {
	if c.run != nil {
		c.run.SetContentText(text)
		return
	}
	c.block.SetContentText(text)
}

// Options returns the selectable items of a drop-down or combo-box control, or
// nil for other kinds.
func (c *ContentControl) Options() []ContentControlOption {
	pr := c.props()
	if pr == nil || pr.Control == nil {
		return nil
	}
	items := pr.Control.ListItems()
	if len(items) == 0 {
		return nil
	}
	out := make([]ContentControlOption, len(items))
	for i, it := range items {
		out[i] = ContentControlOption{DisplayText: it.DisplayText, Value: it.Value}
	}
	return out
}

// DateFormat returns the display format of a date control (w:dateFormat), or ""
// for other kinds.
func (c *ContentControl) DateFormat() string {
	pr := c.props()
	if pr == nil || pr.Control == nil {
		return ""
	}
	return pr.Control.DateFormat()
}

// Checked reports the state of a Word 2010 checkbox control. The second return
// value is false when the control is not a checkbox or has no checked child.
func (c *ContentControl) Checked() (checked, ok bool) {
	pr := c.props()
	if pr == nil || pr.Control == nil {
		return false, false
	}
	return pr.Control.Checked()
}

// AddContentControl appends a block-level rich-text content control to the
// document body, carrying the given tag and holding value as its content. The
// returned handle can further adjust the tag, alias, or value.
func (d *Document) AddContentControl(tag, value string) *ContentControl {
	if d.doc().Body == nil {
		d.doc().Body = &oxml.CT_Body{}
	}
	block := newRichTextBlock(tag, value)
	d.doc().Body.AppendSdtBlock(block)
	return &ContentControl{doc: d, block: block}
}

// AddContentControl appends an inline rich-text content control to the
// paragraph, carrying the given tag and holding value as its content.
func (p *Paragraph) AddContentControl(tag, value string) *ContentControl {
	pr := &oxml.CT_SdtPr{}
	if tag != "" {
		pr.SetTag(tag)
	}
	pr.SetControl(string(ContentControlRichText), oxml.NsWml)

	sr := &oxml.CT_SdtRun{SdtPr: pr}
	sr.SetContentText(value)
	p.mut().AppendSdtRun(sr)
	return &ContentControl{doc: p.document, run: sr}
}

// newRichTextBlock builds a block-level rich-text SDT with one paragraph of
// text.
func newRichTextBlock(tag, value string) *oxml.CT_SdtBlock {
	pr := &oxml.CT_SdtPr{}
	if tag != "" {
		pr.SetTag(tag)
	}
	pr.SetControl(string(ContentControlRichText), oxml.NsWml)

	block := &oxml.CT_SdtBlock{SdtPr: pr}
	block.SetContentText(value)
	return block
}
