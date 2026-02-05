package oxml

import (
	"encoding/xml"
)

// Shape represents a shape element (p:sp) in a slide.
type Shape struct {
	XMLName   xml.Name               `xml:"http://schemas.openxmlformats.org/presentationml/2006/main sp"`
	NvSpPr    *NvSpPr                `xml:"nvSpPr"`
	SpPr      *SpPr                  `xml:"spPr"`
	TxBody    *TxBody                `xml:"txBody,omitempty"`
}

// NvSpPr contains non-visual shape properties.
type NvSpPr struct {
	CNvPr   *CNvPr   `xml:"cNvPr"`
	CNvSpPr *CNvSpPr `xml:"cNvSpPr"`
	NvPr    *NvPr    `xml:"nvPr"`
}

// CNvPr contains common non-visual properties.
type CNvPr struct {
	ID    uint32 `xml:"id,attr"`
	Name  string `xml:"name,attr"`
	Descr string `xml:"descr,attr,omitempty"`
	Title string `xml:"title,attr,omitempty"`
}

// CNvSpPr contains non-visual shape drawing properties.
type CNvSpPr struct {
	TxBox bool `xml:"txBox,attr,omitempty"`
}

// NvPr contains non-visual properties.
type NvPr struct {
	Ph *PlaceholderRef `xml:"ph,omitempty"`
}

// PlaceholderRef references a placeholder.
type PlaceholderRef struct {
	Type string `xml:"type,attr,omitempty"`
	Idx  uint32 `xml:"idx,attr,omitempty"`
}

// SpPr contains shape properties.
type SpPr struct {
	Xfrm     *Xfrm     `xml:"http://schemas.openxmlformats.org/drawingml/2006/main xfrm,omitempty"`
	PrstGeom *PrstGeom `xml:"http://schemas.openxmlformats.org/drawingml/2006/main prstGeom,omitempty"`
	SolidFill *ASolidFill `xml:"http://schemas.openxmlformats.org/drawingml/2006/main solidFill,omitempty"`
	Ln       *Ln       `xml:"http://schemas.openxmlformats.org/drawingml/2006/main ln,omitempty"`
}

// Xfrm contains 2D transform properties.
type Xfrm struct {
	Off *Off `xml:"http://schemas.openxmlformats.org/drawingml/2006/main off,omitempty"`
	Ext *Ext `xml:"http://schemas.openxmlformats.org/drawingml/2006/main ext,omitempty"`
	Rot int  `xml:"rot,attr,omitempty"`
}

// Off represents an offset.
type Off struct {
	X int64 `xml:"x,attr"`
	Y int64 `xml:"y,attr"`
}

// Ext represents extents (size).
type Ext struct {
	Cx int64 `xml:"cx,attr"`
	Cy int64 `xml:"cy,attr"`
}

// PrstGeom represents preset geometry.
type PrstGeom struct {
	Prst  string `xml:"prst,attr"`
	AvLst *AvLst `xml:"http://schemas.openxmlformats.org/drawingml/2006/main avLst,omitempty"`
}

// AvLst is an adjust value list (typically empty for preset shapes).
type AvLst struct{}

// ASolidFill represents a solid fill in DrawingML.
type ASolidFill struct {
	SrgbClr   *ASrgbClr   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main srgbClr,omitempty"`
	SchemeClr *ASchemeClr `xml:"http://schemas.openxmlformats.org/drawingml/2006/main schemeClr,omitempty"`
}

// ASrgbClr represents an RGB color.
type ASrgbClr struct {
	Val string `xml:"val,attr"`
}

// ASchemeClr represents a scheme/theme color.
type ASchemeClr struct {
	Val string `xml:"val,attr"`
}

// Ln represents line properties.
type Ln struct {
	W         int32       `xml:"w,attr,omitempty"`
	SolidFill *ASolidFill `xml:"http://schemas.openxmlformats.org/drawingml/2006/main solidFill,omitempty"`
	NoFill    *NoFill     `xml:"http://schemas.openxmlformats.org/drawingml/2006/main noFill,omitempty"`
}

// NoFill represents no fill.
type NoFill struct{}

// TxBody represents a text body.
type TxBody struct {
	BodyPr   *BodyPr   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main bodyPr"`
	LstStyle *LstStyle `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lstStyle,omitempty"`
	P        []*AP     `xml:"http://schemas.openxmlformats.org/drawingml/2006/main p"`
}

// BodyPr contains body properties.
type BodyPr struct {
	Wrap    string `xml:"wrap,attr,omitempty"`
	Anchor  string `xml:"anchor,attr,omitempty"`
	LIns    int64  `xml:"lIns,attr,omitempty"`
	TIns    int64  `xml:"tIns,attr,omitempty"`
	RIns    int64  `xml:"rIns,attr,omitempty"`
	BIns    int64  `xml:"bIns,attr,omitempty"`
	RtlCol  bool   `xml:"rtlCol,attr,omitempty"`
}

// LstStyle is a list style (typically empty).
type LstStyle struct{}

// AP represents a paragraph in DrawingML.
type AP struct {
	PPr *APPr `xml:"http://schemas.openxmlformats.org/drawingml/2006/main pPr,omitempty"`
	R   []*AR `xml:"http://schemas.openxmlformats.org/drawingml/2006/main r,omitempty"`
	EndParaRPr *ARPr `xml:"http://schemas.openxmlformats.org/drawingml/2006/main endParaRPr,omitempty"`
}

// APPr contains paragraph properties.
type APPr struct {
	Algn    string `xml:"algn,attr,omitempty"`
	Lvl     int    `xml:"lvl,attr,omitempty"`
	MarL    int64  `xml:"marL,attr,omitempty"`
	Indent  int64  `xml:"indent,attr,omitempty"`
	DefRPr  *ARPr  `xml:"http://schemas.openxmlformats.org/drawingml/2006/main defRPr,omitempty"`
	BuNone  *BuNone `xml:"http://schemas.openxmlformats.org/drawingml/2006/main buNone,omitempty"`
	BuChar  *BuChar `xml:"http://schemas.openxmlformats.org/drawingml/2006/main buChar,omitempty"`
	BuAutoNum *BuAutoNum `xml:"http://schemas.openxmlformats.org/drawingml/2006/main buAutoNum,omitempty"`
}

// BuNone indicates no bullet.
type BuNone struct{}

// BuChar represents a character bullet.
type BuChar struct {
	Char string `xml:"char,attr"`
}

// BuAutoNum represents auto-numbered bullets.
type BuAutoNum struct {
	Type string `xml:"type,attr"`
}

// AR represents a run of text.
type AR struct {
	RPr *ARPr  `xml:"http://schemas.openxmlformats.org/drawingml/2006/main rPr,omitempty"`
	T   string `xml:"http://schemas.openxmlformats.org/drawingml/2006/main t"`
}

// ARPr contains run properties.
type ARPr struct {
	Lang      string      `xml:"lang,attr,omitempty"`
	Sz        int32       `xml:"sz,attr,omitempty"`
	B         *bool       `xml:"b,attr,omitempty"`
	I         *bool       `xml:"i,attr,omitempty"`
	U         string      `xml:"u,attr,omitempty"`
	Strike    string      `xml:"strike,attr,omitempty"`
	Baseline  int32       `xml:"baseline,attr,omitempty"`
	SolidFill *ASolidFill `xml:"http://schemas.openxmlformats.org/drawingml/2006/main solidFill,omitempty"`
	Latin     *Latin      `xml:"http://schemas.openxmlformats.org/drawingml/2006/main latin,omitempty"`
}

// Latin represents a Latin font.
type Latin struct {
	Typeface string `xml:"typeface,attr"`
}
