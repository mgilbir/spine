package oxml

import (
	"encoding/xml"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// CT_FFData represents form field data (w:ffData).
type CT_FFData struct {
	Name     *CT_FFName     `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main name,omitempty"`
	Enabled  *CT_OnOff      `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main enabled,omitempty"`
	CalcOnExit *CT_OnOff    `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main calcOnExit,omitempty"`
	HelpText *CT_FFHelpText `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main helpText,omitempty"`
	StatusText *CT_FFStatusText `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main statusText,omitempty"`
	CheckBox   *CT_FFCheckBox  `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main checkBox,omitempty"`
	DdList     *CT_FFDDList    `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main ddList,omitempty"`
	TextInput  *CT_FFTextInput `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main textInput,omitempty"`
	EntryMacro *CT_MacroName   `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main entryMacro,omitempty"`
	ExitMacro  *CT_MacroName   `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main exitMacro,omitempty"`
}

// CT_FFName represents a form field name (w:name inside ffData).
type CT_FFName struct {
	Val string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr,omitempty"`
}

// CT_FFCheckBox represents a checkbox form field (w:checkBox).
type CT_FFCheckBox struct {
	Size      *CT_HpsMeasure `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main size,omitempty"`
	SizeAuto  *CT_OnOff      `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main sizeAuto,omitempty"`
	Default   *CT_OnOff      `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main default,omitempty"`
	Checked   *CT_OnOff      `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main checked,omitempty"`
}

// CT_FFDDList represents a drop-down list form field (w:ddList).
type CT_FFDDList struct {
	Result    *CT_DecimalNumber `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main result,omitempty"`
	Default   *CT_DecimalNumber `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main default,omitempty"`
	ListEntry []CT_String       `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main listEntry,omitempty"`
}

// CT_FFTextInput represents a text input form field (w:textInput).
type CT_FFTextInput struct {
	Type      *CT_String `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main type,omitempty"`
	Default   *CT_String `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main default,omitempty"`
	Format    *CT_String `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main format,omitempty"`
	MaxLength *CT_DecimalNumber `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main maxLength,omitempty"`
}

// CT_FFHelpText represents form field help text.
type CT_FFHelpText struct {
	Type string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main type,attr,omitempty"`
	Val  string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr,omitempty"`
}

// CT_FFStatusText represents form field status text.
type CT_FFStatusText struct {
	Type string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main type,attr,omitempty"`
	Val  string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr,omitempty"`
}

// CT_MacroName represents a macro name.
type CT_MacroName struct {
	Val string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr,omitempty"`
}

// CT_Control represents an embedded control (w:control).
type CT_Control struct {
	RID     string `xml:"-"` // r:id attr
	Name    string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main name,attr,omitempty"`
	ShapeID string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main shapeid,attr,omitempty"`
}

// UnmarshalXML implements custom unmarshaling for CT_Control to handle r:id attributes.
func (c *CT_Control) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		switch {
		case attr.Name.Local == "name":
			c.Name = attr.Value
		case attr.Name.Local == "shapeid":
			c.ShapeID = attr.Value
		case attr.Name.Local == "id" && attr.Name.Space == NsRelationships:
			c.RID = attr.Value
		case attr.Name.Local == "id" && attr.Name.Space == "r":
			c.RID = attr.Value
		}
	}
	return d.Skip()
}

// MarshalToBuilder implements xmlb.BuilderMarshaler for CT_Control.
func (c *CT_Control) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	var attrs []xmlb.Attr
	if c.RID != "" {
		attrs = append(attrs, xmlb.Attr{Namespace: NsRelationships, Name: "id", Value: c.RID})
	}
	if c.Name != "" {
		attrs = append(attrs, xmlb.StrAttr("name", c.Name))
	}
	if c.ShapeID != "" {
		attrs = append(attrs, xmlb.StrAttr("shapeid", c.ShapeID))
	}
	b.EmptyElement(ns, localName, attrs...)
}
