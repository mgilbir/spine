// Package dml provides DrawingML XML effect types from dml-main.xsd.
package dml

// EffectLst represents CT_EffectList (a:effectLst)
type EffectLst struct {
	Blur        *BlurXML        `xml:"http://schemas.openxmlformats.org/drawingml/2006/main blur,omitempty"`
	FillOverlay *FillOverlayXML `xml:"http://schemas.openxmlformats.org/drawingml/2006/main fillOverlay,omitempty"`
	Glow        *GlowXML        `xml:"http://schemas.openxmlformats.org/drawingml/2006/main glow,omitempty"`
	InnerShdw   *InnerShdw      `xml:"http://schemas.openxmlformats.org/drawingml/2006/main innerShdw,omitempty"`
	OuterShdw   *OuterShdw      `xml:"http://schemas.openxmlformats.org/drawingml/2006/main outerShdw,omitempty"`
	PrstShdw    *PrstShdw       `xml:"http://schemas.openxmlformats.org/drawingml/2006/main prstShdw,omitempty"`
	Reflection  *ReflectionXML  `xml:"http://schemas.openxmlformats.org/drawingml/2006/main reflection,omitempty"`
	SoftEdge    *SoftEdgeXML    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main softEdge,omitempty"`
}

// OuterShdw represents CT_OuterShadowEffect (a:outerShdw)
type OuterShdw struct {
	BlurRad      *int64              `xml:"blurRad,attr,omitempty"`
	Dist         *int64              `xml:"dist,attr,omitempty"`
	Dir          *int32              `xml:"dir,attr,omitempty"`
	Sx           *int32              `xml:"sx,attr,omitempty"`
	Sy           *int32              `xml:"sy,attr,omitempty"`
	Kx           *int32              `xml:"kx,attr,omitempty"`
	Ky           *int32              `xml:"ky,attr,omitempty"`
	Algn         string              `xml:"algn,attr,omitempty"`
	RotWithShape *bool               `xml:"rotWithShape,attr,omitempty"`
	ScRgbClr     *ScRgbClr           `xml:"http://schemas.openxmlformats.org/drawingml/2006/main scrgbClr,omitempty"`
	SrgbClr      *SrgbClr            `xml:"http://schemas.openxmlformats.org/drawingml/2006/main srgbClr,omitempty"`
	HslClr       *HslClr             `xml:"http://schemas.openxmlformats.org/drawingml/2006/main hslClr,omitempty"`
	SysClr       *SystemClr          `xml:"http://schemas.openxmlformats.org/drawingml/2006/main sysClr,omitempty"`
	SchemeClr    *SchemeClrTransform `xml:"http://schemas.openxmlformats.org/drawingml/2006/main schemeClr,omitempty"`
	PrstClr      *PrstClr            `xml:"http://schemas.openxmlformats.org/drawingml/2006/main prstClr,omitempty"`
}

// InnerShdw represents CT_InnerShadowEffect (a:innerShdw)
type InnerShdw struct {
	BlurRad   *int64              `xml:"blurRad,attr,omitempty"`
	Dist      *int64              `xml:"dist,attr,omitempty"`
	Dir       *int32              `xml:"dir,attr,omitempty"`
	SrgbClr   *SrgbClr            `xml:"http://schemas.openxmlformats.org/drawingml/2006/main srgbClr,omitempty"`
	SchemeClr *SchemeClrTransform `xml:"http://schemas.openxmlformats.org/drawingml/2006/main schemeClr,omitempty"`
}

// ReflectionXML represents CT_ReflectionEffect (a:reflection)
type ReflectionXML struct {
	BlurRad      *int64 `xml:"blurRad,attr,omitempty"`
	StA          *int32 `xml:"stA,attr,omitempty"`
	StPos        *int32 `xml:"stPos,attr,omitempty"`
	EndA         *int32 `xml:"endA,attr,omitempty"`
	EndPos       *int32 `xml:"endPos,attr,omitempty"`
	Dist         *int64 `xml:"dist,attr,omitempty"`
	Dir          *int32 `xml:"dir,attr,omitempty"`
	FadeDir      *int32 `xml:"fadeDir,attr,omitempty"`
	Sx           *int32 `xml:"sx,attr,omitempty"`
	Sy           *int32 `xml:"sy,attr,omitempty"`
	Kx           *int32 `xml:"kx,attr,omitempty"`
	Ky           *int32 `xml:"ky,attr,omitempty"`
	Algn         string `xml:"algn,attr,omitempty"`
	RotWithShape *bool  `xml:"rotWithShape,attr,omitempty"`
}

// GlowXML represents CT_GlowEffect (a:glow)
type GlowXML struct {
	Rad       int64               `xml:"rad,attr,omitempty"`
	SrgbClr   *SrgbClr            `xml:"http://schemas.openxmlformats.org/drawingml/2006/main srgbClr,omitempty"`
	SchemeClr *SchemeClrTransform `xml:"http://schemas.openxmlformats.org/drawingml/2006/main schemeClr,omitempty"`
}

// SoftEdgeXML represents CT_SoftEdgesEffect (a:softEdge)
type SoftEdgeXML struct {
	Rad int64 `xml:"rad,attr"`
}

// BlurXML represents CT_BlurEffect (a:blur)
type BlurXML struct {
	Rad  int64 `xml:"rad,attr,omitempty"`
	Grow bool  `xml:"grow,attr,omitempty"`
}

// PrstShdw represents CT_PresetShadowEffect (a:prstShdw)
type PrstShdw struct {
	Prst      string              `xml:"prst,attr"`
	Dist      int64               `xml:"dist,attr,omitempty"`
	Dir       int32               `xml:"dir,attr,omitempty"`
	SrgbClr   *SrgbClr            `xml:"http://schemas.openxmlformats.org/drawingml/2006/main srgbClr,omitempty"`
	SchemeClr *SchemeClrTransform `xml:"http://schemas.openxmlformats.org/drawingml/2006/main schemeClr,omitempty"`
}

// EffectContainer represents CT_EffectContainer (a:cont)
type EffectContainer struct {
	Type string   `xml:"type,attr,omitempty"`
	Name string   `xml:"name,attr,omitempty"`
	Blur *BlurXML `xml:"http://schemas.openxmlformats.org/drawingml/2006/main blur,omitempty"`
}

// EffectRef represents CT_EffectReference (a:effect)
type EffectRef struct {
	Ref string `xml:"ref,attr"`
}

// AlphaMod represents CT_AlphaModulateEffect (a:alphaMod)
type AlphaMod struct {
	Cont *EffectContainer `xml:"http://schemas.openxmlformats.org/drawingml/2006/main cont,omitempty"`
}

// AlphaModFix represents CT_AlphaModulateFixedEffect (a:alphaModFix)
type AlphaModFix struct {
	Amt int32 `xml:"amt,attr,omitempty"`
}

// BiLevelXML represents CT_BiLevelEffect (a:biLevel)
type BiLevelXML struct {
	Thresh int32 `xml:"thresh,attr"`
}

// ClrChange represents CT_ColorChangeEffect (a:clrChange)
type ClrChange struct {
	UseA    bool         `xml:"useA,attr,omitempty"`
	ClrFrom *ColorChoice `xml:"http://schemas.openxmlformats.org/drawingml/2006/main clrFrom,omitempty"`
	ClrTo   *ColorChoice `xml:"http://schemas.openxmlformats.org/drawingml/2006/main clrTo,omitempty"`
}

// ClrRepl represents CT_ColorReplaceEffect (a:clrRepl)
type ClrRepl struct {
	SrgbClr   *SrgbClr            `xml:"http://schemas.openxmlformats.org/drawingml/2006/main srgbClr,omitempty"`
	SchemeClr *SchemeClrTransform `xml:"http://schemas.openxmlformats.org/drawingml/2006/main schemeClr,omitempty"`
}

// Duotone represents CT_DuotoneEffect (a:duotone)
type Duotone struct {
	Colors []*ColorChoice `xml:"http://schemas.openxmlformats.org/drawingml/2006/main srgbClr"`
}

// FillOverlayXML represents CT_FillOverlayEffect (a:fillOverlay)
type FillOverlayXML struct {
	Blend     string     `xml:"blend,attr"`
	SolidFill *SolidFill `xml:"http://schemas.openxmlformats.org/drawingml/2006/main solidFill,omitempty"`
	GradFill  *GradFill  `xml:"http://schemas.openxmlformats.org/drawingml/2006/main gradFill,omitempty"`
}

// GrayscaleXML represents CT_GrayscaleEffect (a:grayscl)
type GrayscaleXML struct{}

// HslXML represents CT_HSLEffect (a:hsl)
type HslXML struct {
	Hue int32 `xml:"hue,attr,omitempty"`
	Sat int32 `xml:"sat,attr,omitempty"`
	Lum int32 `xml:"lum,attr,omitempty"`
}

// LumXML represents CT_LuminanceEffect (a:lum)
type LumXML struct {
	Bright   int32 `xml:"bright,attr,omitempty"`
	Contrast int32 `xml:"contrast,attr,omitempty"`
}

// TintEffectXML represents CT_TintEffect (a:tint)
type TintEffectXML struct {
	Hue int32 `xml:"hue,attr,omitempty"`
	Amt int32 `xml:"amt,attr,omitempty"`
}

// AlphaOutset represents CT_AlphaOutsetEffect (a:alphaOutset)
type AlphaOutset struct {
	Rad int64 `xml:"rad,attr,omitempty"`
}

// AlphaBiLevel represents CT_AlphaBiLevelEffect (a:alphaBiLevel)
type AlphaBiLevel struct {
	Thresh int32 `xml:"thresh,attr"`
}

// AlphaCeiling represents CT_AlphaCeilingEffect (a:alphaCeiling)
type AlphaCeiling struct{}

// AlphaFloor represents CT_AlphaFloorEffect (a:alphaFloor)
type AlphaFloor struct{}

// AlphaInv represents CT_AlphaInverseEffect (a:alphaInv)
type AlphaInv struct {
	SrgbClr *SrgbClr `xml:"http://schemas.openxmlformats.org/drawingml/2006/main srgbClr,omitempty"`
}

// AlphaRepl represents CT_AlphaReplaceEffect (a:alphaRepl)
type AlphaRepl struct {
	A int32 `xml:"a,attr"`
}

// EffectDag represents CT_EffectContainer (a:effectDag)
type EffectDag struct {
	Type string   `xml:"type,attr,omitempty"`
	Name string   `xml:"name,attr,omitempty"`
	Cont *EffectContainer `xml:"http://schemas.openxmlformats.org/drawingml/2006/main cont,omitempty"`
}
