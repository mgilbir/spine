package oxml

import "encoding/xml"

// CT_GlossaryDocument represents a glossary document part (w:glossaryDocument).
type CT_GlossaryDocument struct {
	XMLName xml.Name       `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main glossaryDocument"`
	DocParts []*CT_DocPart `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main docPart,omitempty"`
}

// CT_DocPart represents a single document part / building block (w:docPart).
type CT_DocPart struct {
	DocPartPr   *CT_DocPartPr `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main docPartPr,omitempty"`
	DocPartBody *CT_Body      `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main docPartBody,omitempty"`
}

// CT_DocPartPr represents document part properties (w:docPartPr).
type CT_DocPartPr struct {
	Name        *CT_DocPartName     `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main name,omitempty"`
	Style       *CT_String          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main style,omitempty"`
	Category    *CT_DocPartCategory `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main category,omitempty"`
	Types       *CT_DocPartTypes    `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main types,omitempty"`
	Behaviors   *CT_DocPartBehaviors `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main behaviors,omitempty"`
	Description *CT_String          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main description,omitempty"`
	Guid        *CT_String          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main guid,omitempty"`
}

// CT_DocPartName represents a document part name (w:name).
type CT_DocPartName struct {
	Val       string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr,omitempty"`
	Decorated string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main decorated,attr,omitempty"`
}

// CT_DocPartCategory represents a document part category (w:category).
type CT_DocPartCategory struct {
	Name    *CT_String `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main name,omitempty"`
	Gallery *CT_String `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main gallery,omitempty"`
}

// CT_DocPartTypes represents document part types (w:types).
type CT_DocPartTypes struct {
	All  string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main all,attr,omitempty"`
	Type []*CT_DocPartType `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main type,omitempty"`
}

// CT_DocPartType represents a single document part type.
type CT_DocPartType struct {
	Val string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr,omitempty"`
}

// CT_DocPartBehaviors represents document part behaviors (w:behaviors).
type CT_DocPartBehaviors struct {
	Behavior []*CT_DocPartBehavior `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main behavior,omitempty"`
}

// CT_DocPartBehavior represents a single behavior.
type CT_DocPartBehavior struct {
	Val string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr,omitempty"`
}
