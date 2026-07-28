// This file provides SpreadsheetDrawing types from dml-spreadsheetDrawing.xsd.
// These types represent xdr: namespace elements used for drawings in SML.

package dml

import xmlb "github.com/mgilbir/spine/common/xml"

// XDRTwoCellAnchor represents CT_TwoCellAnchor (xdr:twoCellAnchor)
type XDRTwoCellAnchor struct {
	EditAs     string          `xml:"editAs,attr,omitempty"`
	From       *XDRMarker      `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing from,omitempty"`
	To         *XDRMarker      `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing to,omitempty"`
	Sp         *XDRSp          `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing sp,omitempty"`
	GrpSp      *XDRGrpSp       `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing grpSp,omitempty"`
	Pic        *XDRPic         `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing pic,omitempty"`
	CxnSp      *XDRCxnSp       `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing cxnSp,omitempty"`
	ClientData *XDRClientData  `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing clientData,omitempty"`
}

// XDRMarker represents CT_Marker (xdr:from, xdr:to)
type XDRMarker struct {
	Col    int64 `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing col"`
	ColOff int64 `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing colOff"`
	Row    int64 `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing row"`
	RowOff int64 `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing rowOff"`
}

// XDRSp represents CT_Shape (xdr:sp)
type XDRSp struct {
	Macro         string          `xml:"macro,attr,omitempty"`
	TextLink      string          `xml:"textlink,attr,omitempty"`
	FLocksText    *bool           `xml:"fLocksText,attr,omitempty"`
	FPublished    *bool           `xml:"fPublished,attr,omitempty"`
	NvSpPr        *XDRNvSpPr      `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing nvSpPr,omitempty"`
	SpPr          *SpPr           `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing spPr,omitempty"`
	Style         *Style          `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing style,omitempty"`
	TxBody        *TxBody         `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing txBody,omitempty"`
	CapturedAttrs []xmlb.RootAttr `xml:"-"` // verbatim source attrs; see xml_bool_capture.go
}

// XDRPic represents CT_Picture (xdr:pic)
type XDRPic struct {
	Macro         string          `xml:"macro,attr,omitempty"`
	FPublished    *bool           `xml:"fPublished,attr,omitempty"`
	NvPicPr       *XDRNvPicPr     `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing nvPicPr,omitempty"`
	BlipFill      *BlipFillXML    `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing blipFill,omitempty"`
	SpPr          *SpPr           `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing spPr,omitempty"`
	Style         *Style          `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing style,omitempty"`
	CapturedAttrs []xmlb.RootAttr `xml:"-"` // verbatim source attrs; see xml_bool_capture.go
}

// XDRGrpSp represents CT_GroupShape (xdr:grpSp)
type XDRGrpSp struct {
	NvGrpSpPr *XDRNvGrpSpPr `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing nvGrpSpPr,omitempty"`
	GrpSpPr   *GrpSpPr      `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing grpSpPr,omitempty"`
	Sp        []*XDRSp      `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing sp,omitempty"`
	Pic       []*XDRPic     `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing pic,omitempty"`
	GrpSp     []*XDRGrpSp   `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing grpSp,omitempty"`
	CxnSp     []*XDRCxnSp   `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing cxnSp,omitempty"`
}

// XDRCxnSp represents CT_Connector (xdr:cxnSp)
type XDRCxnSp struct {
	Macro         string          `xml:"macro,attr,omitempty"`
	FPublished    *bool           `xml:"fPublished,attr,omitempty"`
	NvCxnSpPr     *XDRNvCxnSpPr   `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing nvCxnSpPr,omitempty"`
	SpPr          *SpPr           `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing spPr,omitempty"`
	Style         *Style          `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing style,omitempty"`
	CapturedAttrs []xmlb.RootAttr `xml:"-"` // verbatim source attrs; see xml_bool_capture.go
}

// XDRClientData represents CT_AnchorClientData (xdr:clientData)
type XDRClientData struct {
	FLocksWithSheet  *bool           `xml:"fLocksWithSheet,attr,omitempty"`
	FPrintsWithSheet *bool           `xml:"fPrintsWithSheet,attr,omitempty"`
	CapturedAttrs    []xmlb.RootAttr `xml:"-"` // verbatim source attrs; see xml_bool_capture.go
}

// XDRNvSpPr represents non-visual shape properties (xdr:nvSpPr)
type XDRNvSpPr struct {
	CNvPr   *CNvPr   `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing cNvPr,omitempty"`
	CNvSpPr *CNvSpPr `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing cNvSpPr,omitempty"`
}

// XDRNvPicPr represents non-visual picture properties (xdr:nvPicPr)
type XDRNvPicPr struct {
	CNvPr    *CNvPr    `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing cNvPr,omitempty"`
	CNvPicPr *CNvPicPr `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing cNvPicPr,omitempty"`
}

// XDRNvGrpSpPr represents non-visual group shape properties (xdr:nvGrpSpPr)
type XDRNvGrpSpPr struct {
	CNvPr      *CNvPr      `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing cNvPr,omitempty"`
	CNvGrpSpPr *CNvGrpSpPr `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing cNvGrpSpPr,omitempty"`
}

// XDRNvCxnSpPr represents non-visual connector properties (xdr:nvCxnSpPr)
type XDRNvCxnSpPr struct {
	CNvPr      *CNvPr      `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing cNvPr,omitempty"`
	CNvCxnSpPr *CNvCxnSpPr `xml:"http://schemas.openxmlformats.org/drawingml/2006/spreadsheetDrawing cNvCxnSpPr,omitempty"`
}
