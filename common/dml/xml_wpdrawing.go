// Package dml provides WordprocessingDrawing types from dml-wordprocessingDrawing.xsd.
// These types represent wp: namespace elements used for inline/anchored drawings in WML.
package dml

// WPAnchor represents CT_Anchor (wp:anchor) - anchored drawing object.
// relativeHeight, behindDoc, locked, layoutInCell and allowOverlap are
// required by the XSD, so they carry no omitempty: their zero values must be
// emitted rather than dropped.
type WPAnchor struct {
	DistT          *uint32 `xml:"distT,attr,omitempty"`
	DistB          *uint32 `xml:"distB,attr,omitempty"`
	DistL          *uint32 `xml:"distL,attr,omitempty"`
	DistR          *uint32 `xml:"distR,attr,omitempty"`
	SimplePos2     *bool   `xml:"simplePos,attr,omitempty"`
	RelativeHeight uint32  `xml:"relativeHeight,attr"`
	BehindDoc      bool    `xml:"behindDoc,attr"`
	Locked         bool    `xml:"locked,attr"`
	LayoutInCell   bool    `xml:"layoutInCell,attr"`
	Hidden         bool    `xml:"hidden,attr,omitempty"`
	AllowOverlap   bool    `xml:"allowOverlap,attr"`

	SimplePos         *WPPoint2D         `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing simplePos,omitempty"`
	PositionH         *WPPosH            `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing positionH,omitempty"`
	PositionV         *WPPosV            `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing positionV,omitempty"`
	Extent            *ExtXML            `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing extent,omitempty"`
	EffectExtent      *WPEffectExtent    `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing effectExtent,omitempty"`
	WrapNone          *WPWrapNone        `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing wrapNone,omitempty"`
	WrapSquare        *WPWrapSquare      `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing wrapSquare,omitempty"`
	WrapTight         *WPWrapTight       `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing wrapTight,omitempty"`
	WrapThrough       *WPWrapThrough     `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing wrapThrough,omitempty"`
	WrapTopAndBottom  *WPWrapTopBottom   `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing wrapTopAndBottom,omitempty"`
	DocPr             *CNvPr             `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing docPr,omitempty"`
	CNvGraphicFramePr *CNvGraphicFramePr `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing cNvGraphicFramePr,omitempty"`
	Graphic           *Graphic           `xml:"http://schemas.openxmlformats.org/drawingml/2006/main graphic,omitempty"`
}

// WPInline represents CT_Inline (wp:inline) - inline drawing object
type WPInline struct {
	DistT *uint32 `xml:"distT,attr,omitempty"`
	DistB *uint32 `xml:"distB,attr,omitempty"`
	DistL *uint32 `xml:"distL,attr,omitempty"`
	DistR *uint32 `xml:"distR,attr,omitempty"`

	Extent            *ExtXML            `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing extent,omitempty"`
	EffectExtent      *WPEffectExtent    `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing effectExtent,omitempty"`
	DocPr             *CNvPr             `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing docPr,omitempty"`
	CNvGraphicFramePr *CNvGraphicFramePr `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing cNvGraphicFramePr,omitempty"`
	Graphic           *Graphic           `xml:"http://schemas.openxmlformats.org/drawingml/2006/main graphic,omitempty"`
}

// WPEffectExtent represents CT_EffectExtent (wp:effectExtent)
type WPEffectExtent struct {
	L int64 `xml:"l,attr"`
	T int64 `xml:"t,attr"`
	R int64 `xml:"r,attr"`
	B int64 `xml:"b,attr"`
}

// WPWrapPolygon represents CT_WrapPath (wp:wrapPolygon)
type WPWrapPolygon struct {
	Edited bool         `xml:"edited,attr,omitempty"`
	Start  *WPPoint2D   `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing start,omitempty"`
	LineTo []*WPPoint2D `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing lineTo,omitempty"`
}

// WPPoint2D represents CT_Point2D (wp:start, wp:lineTo)
type WPPoint2D struct {
	X int64 `xml:"x,attr"`
	Y int64 `xml:"y,attr"`
}

// WPPosH represents CT_PosH (wp:positionH)
type WPPosH struct {
	RelativeFrom string  `xml:"relativeFrom,attr,omitempty"`
	Align        *string `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing align,omitempty"`
	PosOffset    *string `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing posOffset,omitempty"`
}

// WPPosV represents CT_PosV (wp:positionV)
type WPPosV struct {
	RelativeFrom string  `xml:"relativeFrom,attr,omitempty"`
	Align        *string `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing align,omitempty"`
	PosOffset    *string `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing posOffset,omitempty"`
}

// WPWrapNone represents CT_WrapNone (wp:wrapNone)
type WPWrapNone struct{}

// WPWrapSquare represents CT_WrapSquare (wp:wrapSquare)
type WPWrapSquare struct {
	WrapText     string          `xml:"wrapText,attr,omitempty"`
	DistT        *uint32         `xml:"distT,attr,omitempty"`
	DistB        *uint32         `xml:"distB,attr,omitempty"`
	DistL        *uint32         `xml:"distL,attr,omitempty"`
	DistR        *uint32         `xml:"distR,attr,omitempty"`
	EffectExtent *WPEffectExtent `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing effectExtent,omitempty"`
}

// WPWrapTight represents CT_WrapTight (wp:wrapTight)
type WPWrapTight struct {
	WrapText    string         `xml:"wrapText,attr,omitempty"`
	DistL       *uint32        `xml:"distL,attr,omitempty"`
	DistR       *uint32        `xml:"distR,attr,omitempty"`
	WrapPolygon *WPWrapPolygon `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing wrapPolygon,omitempty"`
}

// WPWrapThrough represents CT_WrapThrough (wp:wrapThrough)
type WPWrapThrough struct {
	WrapText    string         `xml:"wrapText,attr,omitempty"`
	DistL       *uint32        `xml:"distL,attr,omitempty"`
	DistR       *uint32        `xml:"distR,attr,omitempty"`
	WrapPolygon *WPWrapPolygon `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing wrapPolygon,omitempty"`
}

// WPWrapTopBottom represents CT_WrapTopBottom (wp:wrapTopAndBottom)
type WPWrapTopBottom struct {
	DistT        *uint32         `xml:"distT,attr,omitempty"`
	DistB        *uint32         `xml:"distB,attr,omitempty"`
	EffectExtent *WPEffectExtent `xml:"http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing effectExtent,omitempty"`
}
