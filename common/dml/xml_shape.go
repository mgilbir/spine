// This file provides DrawingML XML shape property types from dml-main.xsd.

package dml

import (
	"encoding/xml"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// SpPr represents CT_ShapeProperties (a:spPr)
type SpPr struct {
	BwMode    string       `xml:"bwMode,attr,omitempty"`
	Xfrm      *Xfrm        `xml:"http://schemas.openxmlformats.org/drawingml/2006/main xfrm,omitempty"`
	CustGeom  *CustGeom    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main custGeom,omitempty"`
	PrstGeom  *PrstGeom    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main prstGeom,omitempty"`
	NoFill    *NoFillXML   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main noFill,omitempty"`
	SolidFill *SolidFill   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main solidFill,omitempty"`
	GradFill  *GradFill    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main gradFill,omitempty"`
	BlipFill  *BlipFillXML `xml:"http://schemas.openxmlformats.org/drawingml/2006/main blipFill,omitempty"`
	PattFill  *PattFill    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main pattFill,omitempty"`
	GrpFill   *GrpFill     `xml:"http://schemas.openxmlformats.org/drawingml/2006/main grpFill,omitempty"`
	Ln        *Ln          `xml:"http://schemas.openxmlformats.org/drawingml/2006/main ln,omitempty"`
	EffectLst *EffectLst   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main effectLst,omitempty"`
	EffectDag *EffectDag   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main effectDag,omitempty"`
	Scene3d   *Scene3d     `xml:"http://schemas.openxmlformats.org/drawingml/2006/main scene3d,omitempty"`
	Sp3d      *Sp3d        `xml:"http://schemas.openxmlformats.org/drawingml/2006/main sp3d,omitempty"`
	ExtLst    *ExtLst      `xml:"http://schemas.openxmlformats.org/drawingml/2006/main extLst,omitempty"`
}

// GrpSpPr represents CT_GroupShapeProperties (a:grpSpPr)
type GrpSpPr struct {
	BwMode    string       `xml:"bwMode,attr,omitempty"`
	Xfrm      *GrpXfrm     `xml:"http://schemas.openxmlformats.org/drawingml/2006/main xfrm,omitempty"`
	NoFill    *NoFillXML   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main noFill,omitempty"`
	SolidFill *SolidFill   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main solidFill,omitempty"`
	GradFill  *GradFill    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main gradFill,omitempty"`
	BlipFill  *BlipFillXML `xml:"http://schemas.openxmlformats.org/drawingml/2006/main blipFill,omitempty"`
	PattFill  *PattFill    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main pattFill,omitempty"`
	GrpFill   *GrpFill     `xml:"http://schemas.openxmlformats.org/drawingml/2006/main grpFill,omitempty"`
	EffectLst *EffectLst   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main effectLst,omitempty"`
	EffectDag *EffectDag   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main effectDag,omitempty"`
	Scene3d   *Scene3d     `xml:"http://schemas.openxmlformats.org/drawingml/2006/main scene3d,omitempty"`
	ExtLst    *ExtLst      `xml:"http://schemas.openxmlformats.org/drawingml/2006/main extLst,omitempty"`
}

// Cxn represents CT_Connection (a:stCxn, a:endCxn)
type Cxn struct {
	Id  uint32 `xml:"id,attr"`
	Idx uint32 `xml:"idx,attr"`
}

// CxnSpLocks represents CT_ConnectorLocking (a:cxnSpLocks). Every AG_Locking
// flag defaults to false, so bool+omitempty cannot express an explicit
// noGrp="0"; CapturedAttrs replays it, matching SpLocks/PicLocks.
type CxnSpLocks struct {
	NoGrp              bool            `xml:"noGrp,attr,omitempty"`
	NoSelect           bool            `xml:"noSelect,attr,omitempty"`
	NoRot              bool            `xml:"noRot,attr,omitempty"`
	NoChangeAspect     bool            `xml:"noChangeAspect,attr,omitempty"`
	NoMove             bool            `xml:"noMove,attr,omitempty"`
	NoResize           bool            `xml:"noResize,attr,omitempty"`
	NoEditPoints       bool            `xml:"noEditPoints,attr,omitempty"`
	NoAdjustHandles    bool            `xml:"noAdjustHandles,attr,omitempty"`
	NoChangeArrowheads bool            `xml:"noChangeArrowheads,attr,omitempty"`
	NoChangeShapeType  bool            `xml:"noChangeShapeType,attr,omitempty"`
	ExtLst             *ExtLst         `xml:"http://schemas.openxmlformats.org/drawingml/2006/main extLst,omitempty"`
	CapturedAttrs      []xmlb.RootAttr `xml:"-"` // verbatim source attrs; see common/xml.CaptureAttrs
}

// UnmarshalXML captures the element's verbatim attribute list (source
// attribute order, unmodeled attributes, explicit zero values) before decoding
// through the struct tags; the reflection marshaler replays it.
func (clk *CxnSpLocks) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	clk.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias CxnSpLocks
	return d.DecodeElement((*alias)(clk), &start)
}

// PicLocks represents CT_PictureLocking (a:picLocks): the full AG_Locking
// attribute group plus noCrop and the extLst child.
type PicLocks struct {
	NoGrp              bool            `xml:"noGrp,attr,omitempty"`
	NoSelect           bool            `xml:"noSelect,attr,omitempty"`
	NoRot              bool            `xml:"noRot,attr,omitempty"`
	NoChangeAspect     bool            `xml:"noChangeAspect,attr,omitempty"`
	NoMove             bool            `xml:"noMove,attr,omitempty"`
	NoResize           bool            `xml:"noResize,attr,omitempty"`
	NoEditPoints       bool            `xml:"noEditPoints,attr,omitempty"`
	NoAdjustHandles    bool            `xml:"noAdjustHandles,attr,omitempty"`
	NoChangeArrowheads bool            `xml:"noChangeArrowheads,attr,omitempty"`
	NoChangeShapeType  bool            `xml:"noChangeShapeType,attr,omitempty"`
	NoCrop             bool            `xml:"noCrop,attr,omitempty"`
	ExtLst             *ExtLst         `xml:"http://schemas.openxmlformats.org/drawingml/2006/main extLst,omitempty"`
	CapturedAttrs      []xmlb.RootAttr `xml:"-"` // verbatim source attrs; see common/xml.CaptureAttrs
}

// UnmarshalXML captures the element's verbatim attribute list (source
// attribute order and any unmodeled attributes) before decoding through the
// struct tags; the reflection marshaler replays it.
func (plk *PicLocks) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	plk.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias PicLocks
	return d.DecodeElement((*alias)(plk), &start)
}

// SpLocks represents CT_ShapeLocking (a:spLocks)
type SpLocks struct {
	NoGrp              bool            `xml:"noGrp,attr,omitempty"`
	NoSelect           bool            `xml:"noSelect,attr,omitempty"`
	NoRot              bool            `xml:"noRot,attr,omitempty"`
	NoChangeAspect     bool            `xml:"noChangeAspect,attr,omitempty"`
	NoMove             bool            `xml:"noMove,attr,omitempty"`
	NoResize           bool            `xml:"noResize,attr,omitempty"`
	NoEditPoints       bool            `xml:"noEditPoints,attr,omitempty"`
	NoAdjustHandles    bool            `xml:"noAdjustHandles,attr,omitempty"`
	NoChangeArrowheads bool            `xml:"noChangeArrowheads,attr,omitempty"`
	NoChangeShapeType  bool            `xml:"noChangeShapeType,attr,omitempty"`
	NoTextEdit         bool            `xml:"noTextEdit,attr,omitempty"`
	ExtLst             *ExtLst         `xml:"http://schemas.openxmlformats.org/drawingml/2006/main extLst,omitempty"`
	CapturedAttrs      []xmlb.RootAttr `xml:"-"` // verbatim source attrs; see common/xml.CaptureAttrs
}

// UnmarshalXML captures the element's verbatim attribute list (source
// attribute order and any unmodeled attributes) before decoding through the
// struct tags; the reflection marshaler replays it.
func (slk *SpLocks) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	slk.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias SpLocks
	return d.DecodeElement((*alias)(slk), &start)
}

// GrpSpLocks represents CT_GroupLocking (a:grpSpLocks). See CxnSpLocks for the
// CapturedAttrs rationale.
type GrpSpLocks struct {
	NoGrp          bool            `xml:"noGrp,attr,omitempty"`
	NoUngrp        bool            `xml:"noUngrp,attr,omitempty"`
	NoSelect       bool            `xml:"noSelect,attr,omitempty"`
	NoRot          bool            `xml:"noRot,attr,omitempty"`
	NoChangeAspect bool            `xml:"noChangeAspect,attr,omitempty"`
	NoMove         bool            `xml:"noMove,attr,omitempty"`
	NoResize       bool            `xml:"noResize,attr,omitempty"`
	ExtLst         *ExtLst         `xml:"http://schemas.openxmlformats.org/drawingml/2006/main extLst,omitempty"`
	CapturedAttrs  []xmlb.RootAttr `xml:"-"` // verbatim source attrs; see common/xml.CaptureAttrs
}

// UnmarshalXML captures the element's verbatim attribute list (source
// attribute order, unmodeled attributes, explicit zero values) before decoding
// through the struct tags; the reflection marshaler replays it.
func (glk *GrpSpLocks) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	glk.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias GrpSpLocks
	return d.DecodeElement((*alias)(glk), &start)
}

// Style represents CT_ShapeStyle (a:style)
type Style struct {
	LnRef     *LnRef          `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lnRef,omitempty"`
	FillRef   *FillRef        `xml:"http://schemas.openxmlformats.org/drawingml/2006/main fillRef,omitempty"`
	EffectRef *StyleEffectRef `xml:"http://schemas.openxmlformats.org/drawingml/2006/main effectRef,omitempty"`
	FontRef   *FontRef        `xml:"http://schemas.openxmlformats.org/drawingml/2006/main fontRef,omitempty"`
}

// CNvPr represents CT_NonVisualDrawingProps (a:cNvPr)
type CNvPr struct {
	Id            uint32          `xml:"id,attr"`
	Name          string          `xml:"name,attr"`
	Descr         string          `xml:"descr,attr,omitempty"`
	Title         string          `xml:"title,attr,omitempty"`
	Hidden        bool            `xml:"hidden,attr,omitempty"`
	HlinkClick    *HlinkXML       `xml:"http://schemas.openxmlformats.org/drawingml/2006/main hlinkClick,omitempty"`
	HlinkHover    *HlinkXML       `xml:"http://schemas.openxmlformats.org/drawingml/2006/main hlinkHover,omitempty"`
	ExtLst        *ExtLst         `xml:"http://schemas.openxmlformats.org/drawingml/2006/main extLst,omitempty"`
	CapturedAttrs []xmlb.RootAttr `xml:"-"` // verbatim source attrs; see common/xml.CaptureAttrs
}

// UnmarshalXML captures the element's verbatim attribute list (source
// attribute order and any unmodeled attributes) before decoding through the
// struct tags; the reflection marshaler replays it.
func (cn *CNvPr) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	cn.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias CNvPr
	return d.DecodeElement((*alias)(cn), &start)
}

// CNvSpPr represents CT_NonVisualDrawingShapeProps (a:cNvSpPr)
type CNvSpPr struct {
	TxBox         bool            `xml:"txBox,attr,omitempty"`
	SpLocks       *SpLocks        `xml:"http://schemas.openxmlformats.org/drawingml/2006/main spLocks,omitempty"`
	ExtLst        *ExtLst         `xml:"http://schemas.openxmlformats.org/drawingml/2006/main extLst,omitempty"`
	CapturedAttrs []xmlb.RootAttr `xml:"-"` // verbatim source attrs; see common/xml.CaptureAttrs
}

// UnmarshalXML captures the element's verbatim attribute list (boolean
// lexical forms like txBox="true") before decoding through the struct tags;
// the reflection marshaler replays it.
func (cs *CNvSpPr) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	cs.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias CNvSpPr
	return d.DecodeElement((*alias)(cs), &start)
}

// CNvPicPr represents CT_NonVisualPictureProperties (a:cNvPicPr).
// preferRelativeResize defaults to TRUE in the XSD, so it must be a pointer:
// bool+omitempty would drop an explicit preferRelativeResize="0" and silently
// flip the picture back to relative resizing (C29 rule).
type CNvPicPr struct {
	PreferRelativeResize *bool           `xml:"preferRelativeResize,attr,omitempty"`
	PicLocks             *PicLocks       `xml:"http://schemas.openxmlformats.org/drawingml/2006/main picLocks,omitempty"`
	ExtLst               *ExtLst         `xml:"http://schemas.openxmlformats.org/drawingml/2006/main extLst,omitempty"`
	CapturedAttrs        []xmlb.RootAttr `xml:"-"` // verbatim source attrs; see xml_bool_capture.go
}

// CNvGrpSpPr represents CT_NonVisualGroupDrawingShapeProps (a:cNvGrpSpPr)
type CNvGrpSpPr struct {
	GrpSpLocks *GrpSpLocks `xml:"http://schemas.openxmlformats.org/drawingml/2006/main grpSpLocks,omitempty"`
	ExtLst     *ExtLst     `xml:"http://schemas.openxmlformats.org/drawingml/2006/main extLst,omitempty"`
}

// GraphicFrameLocks represents CT_GraphicalObjectFrameLocking
// (a:graphicFrameLocks). See CxnSpLocks for the CapturedAttrs rationale.
type GraphicFrameLocks struct {
	NoGrp          bool            `xml:"noGrp,attr,omitempty"`
	NoDrilldown    bool            `xml:"noDrilldown,attr,omitempty"`
	NoSelect       bool            `xml:"noSelect,attr,omitempty"`
	NoChangeAspect bool            `xml:"noChangeAspect,attr,omitempty"`
	NoMove         bool            `xml:"noMove,attr,omitempty"`
	NoResize       bool            `xml:"noResize,attr,omitempty"`
	ExtLst         *ExtLst         `xml:"http://schemas.openxmlformats.org/drawingml/2006/main extLst,omitempty"`
	CapturedAttrs  []xmlb.RootAttr `xml:"-"` // verbatim source attrs; see common/xml.CaptureAttrs
}

// UnmarshalXML captures the element's verbatim attribute list (source
// attribute order, unmodeled attributes, explicit zero values) before decoding
// through the struct tags; the reflection marshaler replays it.
func (gfl *GraphicFrameLocks) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	gfl.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias GraphicFrameLocks
	return d.DecodeElement((*alias)(gfl), &start)
}

// CNvGraphicFramePr represents CT_NonVisualGraphicFrameProperties
// (cNvGraphicFramePr). Its child a:graphicFrameLocks lives in the DrawingML
// main namespace regardless of the parent's namespace.
type CNvGraphicFramePr struct {
	GraphicFrameLocks *GraphicFrameLocks `xml:"http://schemas.openxmlformats.org/drawingml/2006/main graphicFrameLocks,omitempty"`
	ExtLst            *ExtLst            `xml:"http://schemas.openxmlformats.org/drawingml/2006/main extLst,omitempty"`
}

// CNvCxnSpPr represents CT_NonVisualConnectorProperties (a:cNvCxnSpPr)
type CNvCxnSpPr struct {
	CxnSpLocks *CxnSpLocks `xml:"http://schemas.openxmlformats.org/drawingml/2006/main cxnSpLocks,omitempty"`
	StCxn      *Cxn        `xml:"http://schemas.openxmlformats.org/drawingml/2006/main stCxn,omitempty"`
	EndCxn     *Cxn        `xml:"http://schemas.openxmlformats.org/drawingml/2006/main endCxn,omitempty"`
	ExtLst     *ExtLst     `xml:"http://schemas.openxmlformats.org/drawingml/2006/main extLst,omitempty"`
}
