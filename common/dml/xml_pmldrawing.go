// This file provides lightweight PML shape types for spec test validation.
// These mirror the PML internal types (pptx/internal/oxml) but are public
// and live in the DML package since PML shapes reference DML types extensively.

package dml

import xmlb "github.com/mgilbir/spine/common/xml"

// PMLShape represents CT_Shape (p:sp) - PML shape for spec testing
type PMLShape struct {
	NvSpPr *PMLNvSpPr `xml:"http://schemas.openxmlformats.org/presentationml/2006/main nvSpPr,omitempty"`
	SpPr   *SpPr      `xml:"http://schemas.openxmlformats.org/presentationml/2006/main spPr,omitempty"`
	Style  *Style     `xml:"http://schemas.openxmlformats.org/presentationml/2006/main style,omitempty"`
	TxBody *TxBody    `xml:"http://schemas.openxmlformats.org/presentationml/2006/main txBody,omitempty"`
}

// PMLPicture represents CT_Picture (p:pic) - PML picture for spec testing
type PMLPicture struct {
	NvPicPr  *PMLNvPicPr  `xml:"http://schemas.openxmlformats.org/presentationml/2006/main nvPicPr,omitempty"`
	BlipFill *BlipFillXML `xml:"http://schemas.openxmlformats.org/presentationml/2006/main blipFill,omitempty"`
	SpPr     *SpPr        `xml:"http://schemas.openxmlformats.org/presentationml/2006/main spPr,omitempty"`
	Style    *Style       `xml:"http://schemas.openxmlformats.org/presentationml/2006/main style,omitempty"`
}

// PMLGroupShape represents CT_GroupShape (p:grpSp) - PML group shape for spec testing
type PMLGroupShape struct {
	NvGrpSpPr *PMLNvGrpSpPr `xml:"http://schemas.openxmlformats.org/presentationml/2006/main nvGrpSpPr,omitempty"`
	GrpSpPr   *GrpSpPr      `xml:"http://schemas.openxmlformats.org/presentationml/2006/main grpSpPr,omitempty"`
	Sp        []*PMLShape   `xml:"http://schemas.openxmlformats.org/presentationml/2006/main sp,omitempty"`
	Pic       []*PMLPicture `xml:"http://schemas.openxmlformats.org/presentationml/2006/main pic,omitempty"`
	GrpSp     []*PMLGroupShape `xml:"http://schemas.openxmlformats.org/presentationml/2006/main grpSp,omitempty"`
}

// PMLShapeTree represents CT_GroupShape (p:spTree) - PML shape tree for spec testing
type PMLShapeTree struct {
	NvGrpSpPr *PMLNvGrpSpPr `xml:"http://schemas.openxmlformats.org/presentationml/2006/main nvGrpSpPr,omitempty"`
	GrpSpPr   *GrpSpPr      `xml:"http://schemas.openxmlformats.org/presentationml/2006/main grpSpPr,omitempty"`
	Sp        []*PMLShape   `xml:"http://schemas.openxmlformats.org/presentationml/2006/main sp,omitempty"`
	Pic       []*PMLPicture `xml:"http://schemas.openxmlformats.org/presentationml/2006/main pic,omitempty"`
	GrpSp     []*PMLGroupShape `xml:"http://schemas.openxmlformats.org/presentationml/2006/main grpSp,omitempty"`
}

// PMLNvSpPr represents non-visual shape properties (p:nvSpPr)
type PMLNvSpPr struct {
	CNvPr   *CNvPr   `xml:"http://schemas.openxmlformats.org/presentationml/2006/main cNvPr,omitempty"`
	CNvSpPr *CNvSpPr `xml:"http://schemas.openxmlformats.org/presentationml/2006/main cNvSpPr,omitempty"`
	NvPr    *PMLNvPr `xml:"http://schemas.openxmlformats.org/presentationml/2006/main nvPr,omitempty"`
}

// PMLNvPicPr represents non-visual picture properties (p:nvPicPr)
type PMLNvPicPr struct {
	CNvPr    *CNvPr    `xml:"http://schemas.openxmlformats.org/presentationml/2006/main cNvPr,omitempty"`
	CNvPicPr *CNvPicPr `xml:"http://schemas.openxmlformats.org/presentationml/2006/main cNvPicPr,omitempty"`
	NvPr     *PMLNvPr  `xml:"http://schemas.openxmlformats.org/presentationml/2006/main nvPr,omitempty"`
}

// PMLNvGrpSpPr represents non-visual group shape properties (p:nvGrpSpPr)
type PMLNvGrpSpPr struct {
	CNvPr      *CNvPr      `xml:"http://schemas.openxmlformats.org/presentationml/2006/main cNvPr,omitempty"`
	CNvGrpSpPr *CNvGrpSpPr `xml:"http://schemas.openxmlformats.org/presentationml/2006/main cNvGrpSpPr,omitempty"`
	NvPr       *PMLNvPr    `xml:"http://schemas.openxmlformats.org/presentationml/2006/main nvPr,omitempty"`
}

// PMLNvPr represents CT_ApplicationNonVisualDrawingProps (p:nvPr)
type PMLNvPr struct {
	IsPhoto       bool            `xml:"isPhoto,attr,omitempty"`
	UserDrawn     bool            `xml:"userDrawn,attr,omitempty"`
	CapturedAttrs []xmlb.RootAttr `xml:"-"` // verbatim source attrs; see xml_bool_capture.go
}
