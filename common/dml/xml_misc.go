// This file provides miscellaneous DrawingML types for cross-namespace elements.

package dml

import xmlb "github.com/mgilbir/spine/common/xml"

// Wsp represents CT_WordprocessingShape (wps:wsp).
//
// The type is NOT from dml-wordprocessingDrawing.xsd, which the doc comment
// used to claim: that schema's target namespace
// (.../drawingml/2006/wordprocessingDrawing) defines the wp:inline/wp:anchor
// wrappers and contains no wsp element at all. CT_WordprocessingShape lives in
// the Microsoft wordprocessingShape 2010 namespace, which is what Word writes
// and what this library's own docx shape/text-box/WordArt builders emit
// (docx.buildWspXML). Tagging the children under the 2006 URI meant a real
// wps:wsp parsed into an empty struct (C486).
//
// Children follow the schema (cNvPr, cNvSpPr|cNvCnPr, spPr, style, extLst,
// bodyPr). The txbx/linkedTxbx choice is not modeled: CT_TextboxInfo contains
// WML block-level content (w:txbxContent), which needs WML types this package
// does not depend on.
type Wsp struct {
	NormalEastAsianFlow *bool           `xml:"normalEastAsianFlow,attr,omitempty"`
	CNvPr               *CNvPr          `xml:"http://schemas.microsoft.com/office/word/2010/wordprocessingShape cNvPr,omitempty"`
	CNvSpPr             *CNvSpPr        `xml:"http://schemas.microsoft.com/office/word/2010/wordprocessingShape cNvSpPr,omitempty"`
	CNvCnPr             *CNvCxnSpPr     `xml:"http://schemas.microsoft.com/office/word/2010/wordprocessingShape cNvCnPr,omitempty"`
	SpPr                *SpPr           `xml:"http://schemas.microsoft.com/office/word/2010/wordprocessingShape spPr,omitempty"`
	Style               *Style          `xml:"http://schemas.microsoft.com/office/word/2010/wordprocessingShape style,omitempty"`
	ExtLst              *ExtLst         `xml:"http://schemas.microsoft.com/office/word/2010/wordprocessingShape extLst,omitempty"`
	BodyPr              *BodyPr         `xml:"http://schemas.microsoft.com/office/word/2010/wordprocessingShape bodyPr,omitempty"`
	CapturedAttrs       []xmlb.RootAttr `xml:"-"` // verbatim source attrs; see xml_bool_capture.go
}

// DiagramBg represents CT_BackgroundFormatting (dgm:bg) in diagram context
type DiagramBg struct {
	NoFill    *NoFillXML `xml:"http://schemas.openxmlformats.org/drawingml/2006/main noFill,omitempty"`
	SolidFill *SolidFill `xml:"http://schemas.openxmlformats.org/drawingml/2006/main solidFill,omitempty"`
	GradFill  *GradFill  `xml:"http://schemas.openxmlformats.org/drawingml/2006/main gradFill,omitempty"`
	BlipFill  *BlipFillXML `xml:"http://schemas.openxmlformats.org/drawingml/2006/main blipFill,omitempty"`
	PattFill  *PattFill  `xml:"http://schemas.openxmlformats.org/drawingml/2006/main pattFill,omitempty"`
	GrpFill   *GrpFill   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main grpFill,omitempty"`
	EffectLst *EffectLst `xml:"http://schemas.openxmlformats.org/drawingml/2006/main effectLst,omitempty"`
	EffectDag *EffectDag `xml:"http://schemas.openxmlformats.org/drawingml/2006/main effectDag,omitempty"`
}
