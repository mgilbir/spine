package oxml

import (
	"encoding/xml"
)

// Picture represents a picture element (p:pic) in a slide.
type Picture struct {
	XMLName   xml.Name   `xml:"http://schemas.openxmlformats.org/presentationml/2006/main pic"`
	NvPicPr   *NvPicPr   `xml:"nvPicPr"`
	BlipFill  *BlipFill  `xml:"http://schemas.openxmlformats.org/presentationml/2006/main blipFill"`
	SpPr      *SpPr      `xml:"spPr"`
}

// NvPicPr contains non-visual picture properties.
type NvPicPr struct {
	CNvPr    *CNvPr    `xml:"cNvPr"`
	CNvPicPr *CNvPicPr `xml:"cNvPicPr"`
	NvPr     *NvPr     `xml:"nvPr"`
}

// CNvPicPr contains non-visual picture drawing properties.
type CNvPicPr struct {
	PicLocks *PicLocks `xml:"http://schemas.openxmlformats.org/drawingml/2006/main picLocks,omitempty"`
}

// PicLocks contains picture locking properties.
type PicLocks struct {
	NoChangeAspect bool `xml:"noChangeAspect,attr,omitempty"`
}

// BlipFill represents a picture fill using a blip (image reference).
type BlipFill struct {
	Blip    *ABlip    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main blip,omitempty"`
	SrcRect *ASrcRect `xml:"http://schemas.openxmlformats.org/drawingml/2006/main srcRect,omitempty"`
	Stretch *AStretch `xml:"http://schemas.openxmlformats.org/drawingml/2006/main stretch,omitempty"`
}

// ABlip represents a DrawingML blip (image reference).
type ABlip struct {
	Embed string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships embed,attr,omitempty"`
	Link  string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships link,attr,omitempty"`
	Cstate string `xml:"cstate,attr,omitempty"`
}

// ASrcRect represents source rectangle for cropping.
type ASrcRect struct {
	L int32 `xml:"l,attr,omitempty"`
	T int32 `xml:"t,attr,omitempty"`
	R int32 `xml:"r,attr,omitempty"`
	B int32 `xml:"b,attr,omitempty"`
}

// AStretch represents stretch fill mode.
type AStretch struct {
	FillRect *AFillRect `xml:"http://schemas.openxmlformats.org/drawingml/2006/main fillRect,omitempty"`
}

// AFillRect represents the fill rectangle.
type AFillRect struct {
	L int32 `xml:"l,attr,omitempty"`
	T int32 `xml:"t,attr,omitempty"`
	R int32 `xml:"r,attr,omitempty"`
	B int32 `xml:"b,attr,omitempty"`
}

// ConnectionShape represents a connector shape (p:cxnSp).
type ConnectionShape struct {
	XMLName    xml.Name    `xml:"http://schemas.openxmlformats.org/presentationml/2006/main cxnSp"`
	NvCxnSpPr  *NvCxnSpPr  `xml:"nvCxnSpPr"`
	SpPr       *SpPr       `xml:"spPr"`
}

// NvCxnSpPr contains non-visual connection shape properties.
type NvCxnSpPr struct {
	CNvPr      *CNvPr      `xml:"cNvPr"`
	CNvCxnSpPr *CNvCxnSpPr `xml:"cNvCxnSpPr"`
	NvPr       *NvPr       `xml:"nvPr"`
}

// CNvCxnSpPr contains non-visual connection shape drawing properties.
type CNvCxnSpPr struct {
	StCxn *ACxn `xml:"http://schemas.openxmlformats.org/drawingml/2006/main stCxn,omitempty"`
	EndCxn *ACxn `xml:"http://schemas.openxmlformats.org/drawingml/2006/main endCxn,omitempty"`
}

// ACxn represents a connection point reference.
type ACxn struct {
	ID  uint32 `xml:"id,attr"`
	Idx uint32 `xml:"idx,attr"`
}

// GroupShape represents a group shape (p:grpSp).
type GroupShape struct {
	XMLName    xml.Name     `xml:"http://schemas.openxmlformats.org/presentationml/2006/main grpSp"`
	NvGrpSpPr  *NvGrpSpPr   `xml:"nvGrpSpPr"`
	GrpSpPr    *GrpSpPr     `xml:"grpSpPr"`
	Shapes     []*Shape     `xml:"http://schemas.openxmlformats.org/presentationml/2006/main sp,omitempty"`
	Pictures   []*Picture   `xml:"http://schemas.openxmlformats.org/presentationml/2006/main pic,omitempty"`
	GraphicFrames []*GraphicFrame `xml:"http://schemas.openxmlformats.org/presentationml/2006/main graphicFrame,omitempty"`
	GroupShapes []*GroupShape `xml:"http://schemas.openxmlformats.org/presentationml/2006/main grpSp,omitempty"`
}

// NvGrpSpPr contains non-visual group shape properties.
type NvGrpSpPr struct {
	CNvPr      *CNvPr      `xml:"cNvPr"`
	CNvGrpSpPr *CNvGrpSpPr `xml:"cNvGrpSpPr"`
	NvPr       *NvPr       `xml:"nvPr"`
}

// CNvGrpSpPr contains non-visual group shape drawing properties.
type CNvGrpSpPr struct{}

// GrpSpPr contains group shape properties.
type GrpSpPr struct {
	Xfrm *GrpXfrm `xml:"http://schemas.openxmlformats.org/drawingml/2006/main xfrm,omitempty"`
}

// GrpXfrm contains group transform properties.
type GrpXfrm struct {
	Off     *Off `xml:"http://schemas.openxmlformats.org/drawingml/2006/main off,omitempty"`
	Ext     *Ext `xml:"http://schemas.openxmlformats.org/drawingml/2006/main ext,omitempty"`
	ChOff   *Off `xml:"http://schemas.openxmlformats.org/drawingml/2006/main chOff,omitempty"`
	ChExt   *Ext `xml:"http://schemas.openxmlformats.org/drawingml/2006/main chExt,omitempty"`
	Rot     int  `xml:"rot,attr,omitempty"`
	FlipH   bool `xml:"flipH,attr,omitempty"`
	FlipV   bool `xml:"flipV,attr,omitempty"`
}
