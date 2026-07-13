// This file provides ChartDrawing types from dml-chartDrawing.xsd.
// These types represent cdr: namespace elements used for drawings in charts.

package dml

// CDRRelSizeAnchor represents CT_RelSizeAnchor (cdr:relSizeAnchor)
type CDRRelSizeAnchor struct {
	From *CDRMarker `xml:"http://schemas.openxmlformats.org/drawingml/2006/chartDrawing from,omitempty"`
	To   *CDRMarker `xml:"http://schemas.openxmlformats.org/drawingml/2006/chartDrawing to,omitempty"`
	Sp   *CDRSp     `xml:"http://schemas.openxmlformats.org/drawingml/2006/chartDrawing sp,omitempty"`
	Pic  *CDRPic    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chartDrawing pic,omitempty"`
	GrpSp *CDRGrpSp `xml:"http://schemas.openxmlformats.org/drawingml/2006/chartDrawing grpSp,omitempty"`
	CxnSp *CDRCxnSp `xml:"http://schemas.openxmlformats.org/drawingml/2006/chartDrawing cxnSp,omitempty"`
}

// CDRMarker represents CT_Marker (cdr:from, cdr:to)
type CDRMarker struct {
	X float64 `xml:"http://schemas.openxmlformats.org/drawingml/2006/chartDrawing x"`
	Y float64 `xml:"http://schemas.openxmlformats.org/drawingml/2006/chartDrawing y"`
}

// CDRSp represents CT_Shape (cdr:sp)
type CDRSp struct {
	Macro    string       `xml:"macro,attr,omitempty"`
	TextLink string       `xml:"textlink,attr,omitempty"`
	FLocksText *bool      `xml:"fLocksText,attr,omitempty"`
	FPublished *bool      `xml:"fPublished,attr,omitempty"`
	NvSpPr   *CDRNvSpPr   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chartDrawing nvSpPr,omitempty"`
	SpPr     *SpPr        `xml:"http://schemas.openxmlformats.org/drawingml/2006/chartDrawing spPr,omitempty"`
	Style    *Style       `xml:"http://schemas.openxmlformats.org/drawingml/2006/chartDrawing style,omitempty"`
	TxBody   *TxBody      `xml:"http://schemas.openxmlformats.org/drawingml/2006/chartDrawing txBody,omitempty"`
}

// CDRPic represents CT_Picture (cdr:pic)
type CDRPic struct {
	Macro      string        `xml:"macro,attr,omitempty"`
	FPublished *bool         `xml:"fPublished,attr,omitempty"`
	NvPicPr    *CDRNvPicPr   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chartDrawing nvPicPr,omitempty"`
	BlipFill   *BlipFillXML  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chartDrawing blipFill,omitempty"`
	SpPr       *SpPr         `xml:"http://schemas.openxmlformats.org/drawingml/2006/chartDrawing spPr,omitempty"`
	Style      *Style        `xml:"http://schemas.openxmlformats.org/drawingml/2006/chartDrawing style,omitempty"`
}

// CDRGrpSp represents CT_GroupShape (cdr:grpSp)
type CDRGrpSp struct {
	NvGrpSpPr *CDRNvGrpSpPr `xml:"http://schemas.openxmlformats.org/drawingml/2006/chartDrawing nvGrpSpPr,omitempty"`
	GrpSpPr   *GrpSpPr      `xml:"http://schemas.openxmlformats.org/drawingml/2006/chartDrawing grpSpPr,omitempty"`
	Sp        []*CDRSp      `xml:"http://schemas.openxmlformats.org/drawingml/2006/chartDrawing sp,omitempty"`
	Pic       []*CDRPic     `xml:"http://schemas.openxmlformats.org/drawingml/2006/chartDrawing pic,omitempty"`
	GrpSp     []*CDRGrpSp   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chartDrawing grpSp,omitempty"`
	CxnSp     []*CDRCxnSp   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chartDrawing cxnSp,omitempty"`
}

// CDRCxnSp represents CT_Connector (cdr:cxnSp)
type CDRCxnSp struct {
	Macro      string          `xml:"macro,attr,omitempty"`
	FPublished *bool           `xml:"fPublished,attr,omitempty"`
	NvCxnSpPr  *CDRNvCxnSpPr  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chartDrawing nvCxnSpPr,omitempty"`
	SpPr       *SpPr           `xml:"http://schemas.openxmlformats.org/drawingml/2006/chartDrawing spPr,omitempty"`
	Style      *Style          `xml:"http://schemas.openxmlformats.org/drawingml/2006/chartDrawing style,omitempty"`
}

// CDRNvSpPr represents non-visual shape properties (cdr:nvSpPr)
type CDRNvSpPr struct {
	CNvPr   *CNvPr   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chartDrawing cNvPr,omitempty"`
	CNvSpPr *CNvSpPr `xml:"http://schemas.openxmlformats.org/drawingml/2006/chartDrawing cNvSpPr,omitempty"`
}

// CDRNvPicPr represents non-visual picture properties (cdr:nvPicPr)
type CDRNvPicPr struct {
	CNvPr    *CNvPr    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chartDrawing cNvPr,omitempty"`
	CNvPicPr *CNvPicPr `xml:"http://schemas.openxmlformats.org/drawingml/2006/chartDrawing cNvPicPr,omitempty"`
}

// CDRNvGrpSpPr represents non-visual group shape properties (cdr:nvGrpSpPr)
type CDRNvGrpSpPr struct {
	CNvPr      *CNvPr      `xml:"http://schemas.openxmlformats.org/drawingml/2006/chartDrawing cNvPr,omitempty"`
	CNvGrpSpPr *CNvGrpSpPr `xml:"http://schemas.openxmlformats.org/drawingml/2006/chartDrawing cNvGrpSpPr,omitempty"`
}

// CDRNvCxnSpPr represents non-visual connector properties (cdr:nvCxnSpPr)
type CDRNvCxnSpPr struct {
	CNvPr      *CNvPr      `xml:"http://schemas.openxmlformats.org/drawingml/2006/chartDrawing cNvPr,omitempty"`
	CNvCxnSpPr *CNvCxnSpPr `xml:"http://schemas.openxmlformats.org/drawingml/2006/chartDrawing cNvCxnSpPr,omitempty"`
}
