// Package dml provides miscellaneous DrawingML types for cross-namespace elements.
package dml

// Wsp represents CT_WordprocessingShape (wsp) - used in WordprocessingDrawing
type Wsp struct {
	CNvPr     *CNvPr     `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing cNvPr,omitempty"`
	CNvSpPr   *CNvSpPr   `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing cNvSpPr,omitempty"`
	SpPr      *SpPr      `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing spPr,omitempty"`
	Style     *Style     `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing style,omitempty"`
	TxBody    *TxBody    `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing txBody,omitempty"`
	BodyPr    *BodyPr    `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing bodyPr,omitempty"`
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

// DiagramPageSetup represents CT_PrintSettings pageSetup in diagram context
type DiagramPageSetup struct {
	PaperSize      *uint32 `xml:"paperSize,attr,omitempty"`
	FirstPageNumber *int32 `xml:"firstPageNumber,attr,omitempty"`
	Orientation    string  `xml:"orientation,attr,omitempty"`
	BlackAndWhite  *bool   `xml:"blackAndWhite,attr,omitempty"`
	Draft          *bool   `xml:"draft,attr,omitempty"`
	UseFirstPageNumber *bool `xml:"useFirstPageNumber,attr,omitempty"`
	HorizontalDpi *int32  `xml:"horizontalDpi,attr,omitempty"`
	VerticalDpi   *int32  `xml:"verticalDpi,attr,omitempty"`
	Copies        *uint32 `xml:"copies,attr,omitempty"`
	PaperHeight   string  `xml:"paperHeight,attr,omitempty"`
	PaperWidth    string  `xml:"paperWidth,attr,omitempty"`
}
