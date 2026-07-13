// Package dml provides miscellaneous DrawingML types for cross-namespace elements.
package dml

// Wsp represents CT_WordprocessingShape (wsp) from dml-wordprocessingDrawing.xsd.
// Children follow the XSD (cNvPr, cNvSpPr|cNvCnPr, spPr, style, extLst, bodyPr),
// qualified in the wordprocessingDrawing namespace. The txbx/linkedTxbx choice
// is not modeled: CT_TextboxInfo contains WML block-level content (w:txbxContent),
// which needs WML types this package does not depend on.
type Wsp struct {
	NormalEastAsianFlow *bool       `xml:"normalEastAsianFlow,attr,omitempty"`
	CNvPr               *CNvPr      `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing cNvPr,omitempty"`
	CNvSpPr             *CNvSpPr    `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing cNvSpPr,omitempty"`
	CNvCnPr             *CNvCxnSpPr `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing cNvCnPr,omitempty"`
	SpPr                *SpPr       `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing spPr,omitempty"`
	Style               *Style      `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing style,omitempty"`
	ExtLst              *ExtLst     `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing extLst,omitempty"`
	BodyPr              *BodyPr     `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing bodyPr,omitempty"`
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
