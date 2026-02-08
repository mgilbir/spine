package oxml

import (
	"encoding/xml"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// CT_DrawingHF represents header/footer drawing settings.
type CT_DrawingHF struct {
	RID string `xml:"-"` // r:id attr

	Lho  string `xml:"lho,attr,omitempty"`
	Lhe  string `xml:"lhe,attr,omitempty"`
	Lhf  string `xml:"lhf,attr,omitempty"`
	Cho  string `xml:"cho,attr,omitempty"`
	Che  string `xml:"che,attr,omitempty"`
	Chf  string `xml:"chf,attr,omitempty"`
	Rho  string `xml:"rho,attr,omitempty"`
	Rhe  string `xml:"rhe,attr,omitempty"`
	Rhf  string `xml:"rhf,attr,omitempty"`
	Lfo  string `xml:"lfo,attr,omitempty"`
	Lfe  string `xml:"lfe,attr,omitempty"`
	Lff  string `xml:"lff,attr,omitempty"`
	Cfo  string `xml:"cfo,attr,omitempty"`
	Cfe  string `xml:"cfe,attr,omitempty"`
	Cff  string `xml:"cff,attr,omitempty"`
	Rfo  string `xml:"rfo,attr,omitempty"`
	Rfe  string `xml:"rfe,attr,omitempty"`
	Rff  string `xml:"rff,attr,omitempty"`
}

// UnmarshalXML implements custom unmarshaling for CT_DrawingHF to handle r:id.
func (d *CT_DrawingHF) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		switch {
		case attr.Name.Local == "id" && (attr.Name.Space == nsR || attr.Name.Space == "r"):
			d.RID = attr.Value
		case attr.Name.Local == "lho":
			d.Lho = attr.Value
		case attr.Name.Local == "lhe":
			d.Lhe = attr.Value
		case attr.Name.Local == "lhf":
			d.Lhf = attr.Value
		case attr.Name.Local == "cho":
			d.Cho = attr.Value
		case attr.Name.Local == "che":
			d.Che = attr.Value
		case attr.Name.Local == "chf":
			d.Chf = attr.Value
		case attr.Name.Local == "rho":
			d.Rho = attr.Value
		case attr.Name.Local == "rhe":
			d.Rhe = attr.Value
		case attr.Name.Local == "rhf":
			d.Rhf = attr.Value
		case attr.Name.Local == "lfo":
			d.Lfo = attr.Value
		case attr.Name.Local == "lfe":
			d.Lfe = attr.Value
		case attr.Name.Local == "lff":
			d.Lff = attr.Value
		case attr.Name.Local == "cfo":
			d.Cfo = attr.Value
		case attr.Name.Local == "cfe":
			d.Cfe = attr.Value
		case attr.Name.Local == "cff":
			d.Cff = attr.Value
		case attr.Name.Local == "rfo":
			d.Rfo = attr.Value
		case attr.Name.Local == "rfe":
			d.Rfe = attr.Value
		case attr.Name.Local == "rff":
			d.Rff = attr.Value
		}
	}
	return dec.Skip()
}

// MarshalToBuilder implements xmlb.BuilderMarshaler for CT_DrawingHF.
func (d *CT_DrawingHF) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	var attrs []xmlb.Attr
	if d.RID != "" {
		attrs = append(attrs, xmlb.Attr{Namespace: nsR, Name: "id", Value: d.RID})
	}
	for _, pair := range [][2]string{
		{"lho", d.Lho}, {"lhe", d.Lhe}, {"lhf", d.Lhf},
		{"cho", d.Cho}, {"che", d.Che}, {"chf", d.Chf},
		{"rho", d.Rho}, {"rhe", d.Rhe}, {"rhf", d.Rhf},
		{"lfo", d.Lfo}, {"lfe", d.Lfe}, {"lff", d.Lff},
		{"cfo", d.Cfo}, {"cfe", d.Cfe}, {"cff", d.Cff},
		{"rfo", d.Rfo}, {"rfe", d.Rfe}, {"rff", d.Rff},
	} {
		if pair[1] != "" {
			attrs = append(attrs, xmlb.StrAttr(pair[0], pair[1]))
		}
	}
	b.EmptyElement(ns, localName, attrs...)
}

// CT_CommentPr represents comment properties.
type CT_CommentPr struct {
	Locked          string `xml:"locked,attr,omitempty"`
	DefaultSize     string `xml:"defaultSize,attr,omitempty"`
	Print           string `xml:"print,attr,omitempty"`
	Disabled        string `xml:"disabled,attr,omitempty"`
	AutoFill        string `xml:"autoFill,attr,omitempty"`
	AutoLine        string `xml:"autoLine,attr,omitempty"`
	AltText         string `xml:"altText,attr,omitempty"`
	TextHAlign      string `xml:"textHAlign,attr,omitempty"`
	TextVAlign      string `xml:"textVAlign,attr,omitempty"`
	LockText        string `xml:"lockText,attr,omitempty"`
	JustLastX       string `xml:"justLastX,attr,omitempty"`
	AutoScale       string `xml:"autoScale,attr,omitempty"`
}

// CT_ControlPr represents control properties.
type CT_ControlPr struct {
	RID         string `xml:"-"` // r:id attr
	Locked      string `xml:"locked,attr,omitempty"`
	DefaultSize string `xml:"defaultSize,attr,omitempty"`
	Print       string `xml:"print,attr,omitempty"`
	Disabled    string `xml:"disabled,attr,omitempty"`
	AutoFill    string `xml:"autoFill,attr,omitempty"`
	AutoLine    string `xml:"autoLine,attr,omitempty"`
	AltText     string `xml:"altText,attr,omitempty"`
	LinkedCell  string `xml:"linkedCell,attr,omitempty"`
	ListFillRange string `xml:"listFillRange,attr,omitempty"`
}

// UnmarshalXML implements custom unmarshaling for CT_ControlPr to handle r:id.
func (c *CT_ControlPr) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		switch {
		case attr.Name.Local == "id" && (attr.Name.Space == nsR || attr.Name.Space == "r"):
			c.RID = attr.Value
		case attr.Name.Local == "locked":
			c.Locked = attr.Value
		case attr.Name.Local == "defaultSize":
			c.DefaultSize = attr.Value
		case attr.Name.Local == "print":
			c.Print = attr.Value
		case attr.Name.Local == "disabled":
			c.Disabled = attr.Value
		case attr.Name.Local == "autoFill":
			c.AutoFill = attr.Value
		case attr.Name.Local == "autoLine":
			c.AutoLine = attr.Value
		case attr.Name.Local == "altText":
			c.AltText = attr.Value
		case attr.Name.Local == "linkedCell":
			c.LinkedCell = attr.Value
		case attr.Name.Local == "listFillRange":
			c.ListFillRange = attr.Value
		}
	}
	return d.Skip()
}

// MarshalToBuilder implements xmlb.BuilderMarshaler for CT_ControlPr.
func (c *CT_ControlPr) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	var attrs []xmlb.Attr
	if c.RID != "" {
		attrs = append(attrs, xmlb.Attr{Namespace: nsR, Name: "id", Value: c.RID})
	}
	for _, pair := range [][2]string{
		{"locked", c.Locked}, {"defaultSize", c.DefaultSize},
		{"print", c.Print}, {"disabled", c.Disabled},
		{"autoFill", c.AutoFill}, {"autoLine", c.AutoLine},
		{"altText", c.AltText}, {"linkedCell", c.LinkedCell},
		{"listFillRange", c.ListFillRange},
	} {
		if pair[1] != "" {
			attrs = append(attrs, xmlb.StrAttr(pair[0], pair[1]))
		}
	}
	b.EmptyElement(ns, localName, attrs...)
}

// CT_ObjectPr represents embedded object properties.
type CT_ObjectPr struct {
	Locked      string `xml:"locked,attr,omitempty"`
	DefaultSize string `xml:"defaultSize,attr,omitempty"`
	Print       string `xml:"print,attr,omitempty"`
	Disabled    string `xml:"disabled,attr,omitempty"`
	UIObject    string `xml:"uiObject,attr,omitempty"`
	AutoFill    string `xml:"autoFill,attr,omitempty"`
	AutoLine    string `xml:"autoLine,attr,omitempty"`
	AltText     string `xml:"altText,attr,omitempty"`
	Macro       string `xml:"macro,attr,omitempty"`
}

// CT_SharedItems represents shared items in a pivot cache.
type CT_SharedItems struct {
	ContainsString       string `xml:"containsString,attr,omitempty"`
	ContainsBlank        string `xml:"containsBlank,attr,omitempty"`
	ContainsNumber       string `xml:"containsNumber,attr,omitempty"`
	ContainsInteger      string `xml:"containsInteger,attr,omitempty"`
	ContainsSemiMixedTypes string `xml:"containsSemiMixedTypes,attr,omitempty"`
	ContainsMixedTypes   string `xml:"containsMixedTypes,attr,omitempty"`
	ContainsNonDate      string `xml:"containsNonDate,attr,omitempty"`
	ContainsDate         string `xml:"containsDate,attr,omitempty"`
	MinValue             string `xml:"minValue,attr,omitempty"`
	MaxValue             string `xml:"maxValue,attr,omitempty"`
	MinDate              string `xml:"minDate,attr,omitempty"`
	MaxDate              string `xml:"maxDate,attr,omitempty"`
	Count                string `xml:"count,attr,omitempty"`
	LongText             string `xml:"longText,attr,omitempty"`
}

// UnmarshalXML implements custom unmarshaling for CT_SharedItems to skip unknown children.
func (si *CT_SharedItems) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		switch attr.Name.Local {
		case "containsString":
			si.ContainsString = attr.Value
		case "containsBlank":
			si.ContainsBlank = attr.Value
		case "containsNumber":
			si.ContainsNumber = attr.Value
		case "containsInteger":
			si.ContainsInteger = attr.Value
		case "containsSemiMixedTypes":
			si.ContainsSemiMixedTypes = attr.Value
		case "containsMixedTypes":
			si.ContainsMixedTypes = attr.Value
		case "containsNonDate":
			si.ContainsNonDate = attr.Value
		case "containsDate":
			si.ContainsDate = attr.Value
		case "minValue":
			si.MinValue = attr.Value
		case "maxValue":
			si.MaxValue = attr.Value
		case "minDate":
			si.MinDate = attr.Value
		case "maxDate":
			si.MaxDate = attr.Value
		case "count":
			si.Count = attr.Value
		case "longText":
			si.LongText = attr.Value
		}
	}
	// Skip child elements (m, s, n, d, b, e)
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch tok.(type) {
		case xml.StartElement:
			if err := d.Skip(); err != nil {
				return err
			}
		case xml.EndElement:
			return nil
		}
	}
}

// CT_DataBinding represents XML map data binding.
type CT_DataBinding struct {
	ConnectionID     string `xml:"ConnectionID,attr,omitempty"`
	FileBinding      string `xml:"FileBinding,attr,omitempty"`
	DataBindingLoadMode string `xml:"DataBindingLoadMode,attr,omitempty"`
	DataBindingName  string `xml:"DataBindingName,attr,omitempty"`
	FileBindingName  string `xml:"FileBindingName,attr,omitempty"`
}

// CT_ProtectedRanges represents a collection of protected ranges.
type CT_ProtectedRanges struct {
	ProtectedRange []CT_ProtectedRange `xml:"protectedRange,omitempty"`
}

// CT_ProtectedRange represents a single protected range.
type CT_ProtectedRange struct {
	Sqref         string `xml:"sqref,attr,omitempty"`
	Name          string `xml:"name,attr,omitempty"`
	SecurityDescriptor string `xml:"securityDescriptor,attr,omitempty"`
	Password      string `xml:"password,attr,omitempty"`
	HashValue     string `xml:"hashValue,attr,omitempty"`
	SaltValue     string `xml:"saltValue,attr,omitempty"`
	SpinCount     string `xml:"spinCount,attr,omitempty"`
	AlgorithmName string `xml:"algorithmName,attr,omitempty"`
}

// CT_Table represents a table definition (table root element).
type CT_Table struct {
	ID             string            `xml:"id,attr,omitempty"`
	Name           string            `xml:"name,attr,omitempty"`
	DisplayName    string            `xml:"displayName,attr,omitempty"`
	Ref            string            `xml:"ref,attr,omitempty"`
	TotalsRowShown string            `xml:"totalsRowShown,attr,omitempty"`
	TotalsRowCount string            `xml:"totalsRowCount,attr,omitempty"`
	HeaderRowCount string            `xml:"headerRowCount,attr,omitempty"`
	AutoFilter     *CT_AutoFilter    `xml:"autoFilter,omitempty"`
	TableColumns   *CT_TableColumns  `xml:"tableColumns,omitempty"`
	TableStyleInfo *CT_TableStyleInfo `xml:"tableStyleInfo,omitempty"`
}

// CT_TableColumns represents table columns.
type CT_TableColumns struct {
	Count       string           `xml:"count,attr,omitempty"`
	TableColumn []CT_TableColumn `xml:"tableColumn,omitempty"`
}

// CT_TableColumn represents a single table column.
type CT_TableColumn struct {
	ID          string `xml:"id,attr,omitempty"`
	UniqueName  string `xml:"uniqueName,attr,omitempty"`
	Name        string `xml:"name,attr,omitempty"`
	DataDxfId   string `xml:"dataDxfId,attr,omitempty"`
	TotalsRowFunction string `xml:"totalsRowFunction,attr,omitempty"`
	TotalsRowLabel string `xml:"totalsRowLabel,attr,omitempty"`
}

// CT_TableStyleInfo represents table style info.
type CT_TableStyleInfo struct {
	Name              string `xml:"name,attr,omitempty"`
	ShowFirstColumn   string `xml:"showFirstColumn,attr,omitempty"`
	ShowLastColumn    string `xml:"showLastColumn,attr,omitempty"`
	ShowRowStripes    string `xml:"showRowStripes,attr,omitempty"`
	ShowColumnStripes string `xml:"showColumnStripes,attr,omitempty"`
}
