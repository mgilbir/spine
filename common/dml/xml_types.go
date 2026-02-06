// Package dml provides DrawingML types.
// This file contains XML serialization types from dml-main.xsd.
// These types are used for marshaling/unmarshaling OOXML documents.
package dml

// XML namespace constants
const (
	NsDrawingML     = "http://schemas.openxmlformats.org/drawingml/2006/main"
	NsRelationships = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
)

// --- Color XML Types ---

// SrgbClr represents CT_SRgbColor (a:srgbClr) for XML serialization
type SrgbClr struct {
	Val    string          `xml:"val,attr"`
	Tint   *ColorTransform `xml:"http://schemas.openxmlformats.org/drawingml/2006/main tint,omitempty"`
	Shade  *ColorTransform `xml:"http://schemas.openxmlformats.org/drawingml/2006/main shade,omitempty"`
	SatMod *ColorTransform `xml:"http://schemas.openxmlformats.org/drawingml/2006/main satMod,omitempty"`
	Alpha  *ColorTransform `xml:"http://schemas.openxmlformats.org/drawingml/2006/main alpha,omitempty"`
	LumMod *ColorTransform `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lumMod,omitempty"`
	LumOff *ColorTransform `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lumOff,omitempty"`
}

// SchemeClr represents CT_SchemeColor (a:schemeClr) for XML serialization
type SchemeClr struct {
	Val string `xml:"val,attr"`
}

// SystemClr represents CT_SystemColor (a:sysClr) for XML serialization
type SystemClr struct {
	Val     string          `xml:"val,attr"`
	LastClr string          `xml:"lastClr,attr,omitempty"`
	Tint    *ColorTransform `xml:"http://schemas.openxmlformats.org/drawingml/2006/main tint,omitempty"`
	Shade   *ColorTransform `xml:"http://schemas.openxmlformats.org/drawingml/2006/main shade,omitempty"`
	SatMod  *ColorTransform `xml:"http://schemas.openxmlformats.org/drawingml/2006/main satMod,omitempty"`
	Alpha   *ColorTransform `xml:"http://schemas.openxmlformats.org/drawingml/2006/main alpha,omitempty"`
	LumMod  *ColorTransform `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lumMod,omitempty"`
	LumOff  *ColorTransform `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lumOff,omitempty"`
}

// HslClr represents CT_HslColor (a:hslClr) for XML serialization
type HslClr struct {
	Hue int32 `xml:"hue,attr"`
	Sat int32 `xml:"sat,attr"`
	Lum int32 `xml:"lum,attr"`
}

// PrstClr represents CT_PresetColor (a:prstClr) for XML serialization
type PrstClr struct {
	Val string `xml:"val,attr"`
}

// ScRgbClr represents CT_ScRgbColor (a:scrgbClr) for XML serialization
type ScRgbClr struct {
	R int32 `xml:"r,attr"`
	G int32 `xml:"g,attr"`
	B int32 `xml:"b,attr"`
}

// SchemeClrTransform represents CT_SchemeColor with color transforms
type SchemeClrTransform struct {
	Val    string          `xml:"val,attr"`
	Tint   *ColorTransform `xml:"http://schemas.openxmlformats.org/drawingml/2006/main tint,omitempty"`
	Shade  *ColorTransform `xml:"http://schemas.openxmlformats.org/drawingml/2006/main shade,omitempty"`
	SatMod *ColorTransform `xml:"http://schemas.openxmlformats.org/drawingml/2006/main satMod,omitempty"`
	Alpha  *ColorTransform `xml:"http://schemas.openxmlformats.org/drawingml/2006/main alpha,omitempty"`
	LumMod *ColorTransform `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lumMod,omitempty"`
	LumOff *ColorTransform `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lumOff,omitempty"`
}

// ColorTransform represents a color transform (a:tint, a:shade, a:alpha, etc.)
type ColorTransform struct {
	Val int32 `xml:"val,attr"`
}

// ColorChoice represents EG_ColorChoice for XML serialization
type ColorChoice struct {
	SrgbClr   *SrgbClr            `xml:"http://schemas.openxmlformats.org/drawingml/2006/main srgbClr,omitempty"`
	SchemeClr *SchemeClrTransform `xml:"http://schemas.openxmlformats.org/drawingml/2006/main schemeClr,omitempty"`
	SysClr    *SystemClr          `xml:"http://schemas.openxmlformats.org/drawingml/2006/main sysClr,omitempty"`
	PrstClr   *PrstClr            `xml:"http://schemas.openxmlformats.org/drawingml/2006/main prstClr,omitempty"`
	HslClr    *HslClr             `xml:"http://schemas.openxmlformats.org/drawingml/2006/main hslClr,omitempty"`
	ScrgbClr  *ScRgbClr           `xml:"http://schemas.openxmlformats.org/drawingml/2006/main scrgbClr,omitempty"`
}

// --- Fill XML Types ---

// SolidFill represents CT_SolidColorFillProperties (a:solidFill)
type SolidFill struct {
	SrgbClr   *SrgbClr            `xml:"http://schemas.openxmlformats.org/drawingml/2006/main srgbClr,omitempty"`
	SchemeClr *SchemeClrTransform `xml:"http://schemas.openxmlformats.org/drawingml/2006/main schemeClr,omitempty"`
}

// GradFill represents CT_GradientFillProperties (a:gradFill)
type GradFill struct {
	RotWithShape bool     `xml:"rotWithShape,attr,omitempty"`
	GsLst        *GsLst   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main gsLst,omitempty"`
	Lin          *Lin     `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lin,omitempty"`
	PathShade    *PathXML `xml:"http://schemas.openxmlformats.org/drawingml/2006/main path,omitempty"`
	TileRect     *RelRect `xml:"http://schemas.openxmlformats.org/drawingml/2006/main tileRect,omitempty"`
}

// GsLst represents CT_GradientStopList (a:gsLst)
type GsLst struct {
	Gs []*Gs `xml:"http://schemas.openxmlformats.org/drawingml/2006/main gs"`
}

// Gs represents CT_GradientStop (a:gs)
type Gs struct {
	Pos       int32               `xml:"pos,attr"`
	SrgbClr   *SrgbClr            `xml:"http://schemas.openxmlformats.org/drawingml/2006/main srgbClr,omitempty"`
	SchemeClr *SchemeClrTransform `xml:"http://schemas.openxmlformats.org/drawingml/2006/main schemeClr,omitempty"`
}

// Lin represents CT_LinearShadeProperties (a:lin)
type Lin struct {
	Ang    int32 `xml:"ang,attr,omitempty"`
	Scaled bool  `xml:"scaled,attr,omitempty"`
}

// PathXML represents CT_PathShadeProperties (a:path)
type PathXML struct {
	Path       string   `xml:"path,attr,omitempty"`
	FillToRect *RelRect `xml:"http://schemas.openxmlformats.org/drawingml/2006/main fillToRect,omitempty"`
}

// PattFill represents CT_PatternFillProperties (a:pattFill)
type PattFill struct {
	Prst  string       `xml:"prst,attr,omitempty"`
	FgClr *ColorChoice `xml:"http://schemas.openxmlformats.org/drawingml/2006/main fgClr,omitempty"`
	BgClr *ColorChoice `xml:"http://schemas.openxmlformats.org/drawingml/2006/main bgClr,omitempty"`
}

// BlipFillXML represents CT_BlipFillProperties (a:blipFill)
type BlipFillXML struct {
	Dpi          int32    `xml:"dpi,attr,omitempty"`
	RotWithShape bool     `xml:"rotWithShape,attr,omitempty"`
	Blip         *BlipXML `xml:"http://schemas.openxmlformats.org/drawingml/2006/main blip,omitempty"`
	SrcRect      *RelRect `xml:"http://schemas.openxmlformats.org/drawingml/2006/main srcRect,omitempty"`
	Tile         *TileXML `xml:"http://schemas.openxmlformats.org/drawingml/2006/main tile,omitempty"`
	Stretch      *StretchXML `xml:"http://schemas.openxmlformats.org/drawingml/2006/main stretch,omitempty"`
}

// BlipXML represents CT_Blip (a:blip)
type BlipXML struct {
	Embed  string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships embed,attr,omitempty"`
	Link   string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships link,attr,omitempty"`
	Cstate string `xml:"cstate,attr,omitempty"`
}

// TileXML represents CT_TileInfoProperties (a:tile)
type TileXML struct {
	Tx   int64  `xml:"tx,attr,omitempty"`
	Ty   int64  `xml:"ty,attr,omitempty"`
	Sx   int32  `xml:"sx,attr,omitempty"`
	Sy   int32  `xml:"sy,attr,omitempty"`
	Flip string `xml:"flip,attr,omitempty"`
	Algn string `xml:"algn,attr,omitempty"`
}

// StretchXML represents CT_StretchInfoProperties (a:stretch)
type StretchXML struct {
	FillRect *RelRect `xml:"http://schemas.openxmlformats.org/drawingml/2006/main fillRect,omitempty"`
}

// NoFillXML represents CT_NoFillProperties (a:noFill)
type NoFillXML struct{}

// GrpFill represents CT_GroupFillProperties (a:grpFill)
type GrpFill struct{}

// RelRect represents CT_RelativeRect
type RelRect struct {
	L int32 `xml:"l,attr,omitempty"`
	T int32 `xml:"t,attr,omitempty"`
	R int32 `xml:"r,attr,omitempty"`
	B int32 `xml:"b,attr,omitempty"`
}
