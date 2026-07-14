// This file provides DrawingML XML theme types from dml-main.xsd.

package dml

import (
	"encoding/xml"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// Theme represents CT_OfficeStyleSheet (a:theme)
type Theme struct {
	Name              string             `xml:"name,attr,omitempty"`
	ThemeElements     *ThemeElements     `xml:"http://schemas.openxmlformats.org/drawingml/2006/main themeElements,omitempty"`
	ObjectDefaults    *ObjectDefaults    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main objectDefaults,omitempty"`
	ExtraClrSchemeLst *ExtraClrSchemeLst `xml:"http://schemas.openxmlformats.org/drawingml/2006/main extraClrSchemeLst,omitempty"`
}

// ThemeElements represents CT_BaseStyles (a:themeElements)
type ThemeElements struct {
	ClrScheme  *ClrScheme  `xml:"http://schemas.openxmlformats.org/drawingml/2006/main clrScheme,omitempty"`
	FontScheme *FontScheme `xml:"http://schemas.openxmlformats.org/drawingml/2006/main fontScheme,omitempty"`
	FmtScheme  *FmtScheme  `xml:"http://schemas.openxmlformats.org/drawingml/2006/main fmtScheme,omitempty"`
}

// ClrScheme represents CT_ColorScheme (a:clrScheme)
type ClrScheme struct {
	Name     string       `xml:"name,attr"`
	Dk1      *ColorChoice `xml:"http://schemas.openxmlformats.org/drawingml/2006/main dk1,omitempty"`
	Lt1      *ColorChoice `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lt1,omitempty"`
	Dk2      *ColorChoice `xml:"http://schemas.openxmlformats.org/drawingml/2006/main dk2,omitempty"`
	Lt2      *ColorChoice `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lt2,omitempty"`
	Accent1  *ColorChoice `xml:"http://schemas.openxmlformats.org/drawingml/2006/main accent1,omitempty"`
	Accent2  *ColorChoice `xml:"http://schemas.openxmlformats.org/drawingml/2006/main accent2,omitempty"`
	Accent3  *ColorChoice `xml:"http://schemas.openxmlformats.org/drawingml/2006/main accent3,omitempty"`
	Accent4  *ColorChoice `xml:"http://schemas.openxmlformats.org/drawingml/2006/main accent4,omitempty"`
	Accent5  *ColorChoice `xml:"http://schemas.openxmlformats.org/drawingml/2006/main accent5,omitempty"`
	Accent6  *ColorChoice `xml:"http://schemas.openxmlformats.org/drawingml/2006/main accent6,omitempty"`
	Hlink    *ColorChoice `xml:"http://schemas.openxmlformats.org/drawingml/2006/main hlink,omitempty"`
	FolHlink *ColorChoice `xml:"http://schemas.openxmlformats.org/drawingml/2006/main folHlink,omitempty"`
}

// FontScheme represents CT_FontScheme (a:fontScheme)
type FontScheme struct {
	Name      string          `xml:"name,attr"`
	MajorFont *FontCollection `xml:"http://schemas.openxmlformats.org/drawingml/2006/main majorFont,omitempty"`
	MinorFont *FontCollection `xml:"http://schemas.openxmlformats.org/drawingml/2006/main minorFont,omitempty"`
}

// FontCollection represents CT_FontCollection (a:majorFont, a:minorFont)
type FontCollection struct {
	Latin *TextFont           `xml:"http://schemas.openxmlformats.org/drawingml/2006/main latin,omitempty"`
	Ea    *TextFont           `xml:"http://schemas.openxmlformats.org/drawingml/2006/main ea,omitempty"`
	Cs    *TextFont           `xml:"http://schemas.openxmlformats.org/drawingml/2006/main cs,omitempty"`
	Font  []*SupplementalFont `xml:"http://schemas.openxmlformats.org/drawingml/2006/main font,omitempty"`
}

// SupplementalFont represents CT_SupplementalFont (a:font)
type SupplementalFont struct {
	Script   string `xml:"script,attr"`
	Typeface string `xml:"typeface,attr"`
}

// FmtScheme represents CT_StyleMatrix (a:fmtScheme)
type FmtScheme struct {
	Name           string          `xml:"name,attr,omitempty"`
	FillStyleLst   *FillStyleLst   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main fillStyleLst,omitempty"`
	LnStyleLst     *LnStyleLst     `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lnStyleLst,omitempty"`
	EffectStyleLst *EffectStyleLst `xml:"http://schemas.openxmlformats.org/drawingml/2006/main effectStyleLst,omitempty"`
	BgFillStyleLst *BgFillStyleLst `xml:"http://schemas.openxmlformats.org/drawingml/2006/main bgFillStyleLst,omitempty"`
}

// FillStyleLst represents CT_FillStyleList (a:fillStyleLst)
type FillStyleLst struct {
	NoFill    []*NoFillXML   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main noFill,omitempty"`
	SolidFill []*SolidFill   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main solidFill,omitempty"`
	GradFill  []*GradFill    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main gradFill,omitempty"`
	BlipFill  []*BlipFillXML `xml:"http://schemas.openxmlformats.org/drawingml/2006/main blipFill,omitempty"`
	PattFill  []*PattFill    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main pattFill,omitempty"`
	GrpFill   []*GrpFill     `xml:"http://schemas.openxmlformats.org/drawingml/2006/main grpFill,omitempty"`
}

// LnStyleLst represents CT_LineStyleList (a:lnStyleLst)
type LnStyleLst struct {
	Ln []*Ln `xml:"http://schemas.openxmlformats.org/drawingml/2006/main ln,omitempty"`
}

// EffectStyleLst represents CT_EffectStyleList (a:effectStyleLst)
type EffectStyleLst struct {
	EffectStyle []*EffectStyle `xml:"http://schemas.openxmlformats.org/drawingml/2006/main effectStyle,omitempty"`
}

// EffectStyle represents CT_EffectStyleItem (a:effectStyle)
type EffectStyle struct {
	EffectLst *EffectLst `xml:"http://schemas.openxmlformats.org/drawingml/2006/main effectLst,omitempty"`
	EffectDag *EffectDag `xml:"http://schemas.openxmlformats.org/drawingml/2006/main effectDag,omitempty"`
	Scene3d   *Scene3d   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main scene3d,omitempty"`
	Sp3d      *Sp3d      `xml:"http://schemas.openxmlformats.org/drawingml/2006/main sp3d,omitempty"`
}

// BgFillStyleLst represents CT_BackgroundFillStyleList (a:bgFillStyleLst)
type BgFillStyleLst struct {
	NoFill    []*NoFillXML   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main noFill,omitempty"`
	SolidFill []*SolidFill   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main solidFill,omitempty"`
	GradFill  []*GradFill    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main gradFill,omitempty"`
	BlipFill  []*BlipFillXML `xml:"http://schemas.openxmlformats.org/drawingml/2006/main blipFill,omitempty"`
	PattFill  []*PattFill    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main pattFill,omitempty"`
	GrpFill   []*GrpFill     `xml:"http://schemas.openxmlformats.org/drawingml/2006/main grpFill,omitempty"`
}

// ObjectDefaults represents CT_ObjectStyleDefaults (a:objectDefaults)
type ObjectDefaults struct {
	SpDef *DefaultShapeDefinition `xml:"http://schemas.openxmlformats.org/drawingml/2006/main spDef,omitempty"`
	LnDef *DefaultShapeDefinition `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lnDef,omitempty"`
	TxDef *DefaultShapeDefinition `xml:"http://schemas.openxmlformats.org/drawingml/2006/main txDef,omitempty"`
}

// DefaultShapeDefinition represents CT_DefaultShapeDefinition (a:spDef, a:lnDef, a:txDef)
type DefaultShapeDefinition struct {
	SpPr     *SpPr     `xml:"http://schemas.openxmlformats.org/drawingml/2006/main spPr,omitempty"`
	BodyPr   *BodyPr   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main bodyPr,omitempty"`
	LstStyle *LstStyle `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lstStyle,omitempty"`
	Style    *Style    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main style,omitempty"`
}

// ExtraClrSchemeLst represents CT_ColorSchemeList (a:extraClrSchemeLst)
type ExtraClrSchemeLst struct {
	ExtraClrScheme []*ExtraClrScheme `xml:"http://schemas.openxmlformats.org/drawingml/2006/main extraClrScheme,omitempty"`
}

// ExtraClrScheme represents CT_ColorSchemeAndMapping (a:extraClrScheme)
type ExtraClrScheme struct {
	ClrScheme *ClrScheme `xml:"http://schemas.openxmlformats.org/drawingml/2006/main clrScheme,omitempty"`
	ClrMap    *ClrMap    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main clrMap,omitempty"`
}

// ClrMap represents CT_ColorMapping (a:clrMap)
type ClrMap struct {
	Bg1           string          `xml:"bg1,attr,omitempty"`
	Tx1           string          `xml:"tx1,attr,omitempty"`
	Bg2           string          `xml:"bg2,attr,omitempty"`
	Tx2           string          `xml:"tx2,attr,omitempty"`
	Accent1       string          `xml:"accent1,attr,omitempty"`
	Accent2       string          `xml:"accent2,attr,omitempty"`
	Accent3       string          `xml:"accent3,attr,omitempty"`
	Accent4       string          `xml:"accent4,attr,omitempty"`
	Accent5       string          `xml:"accent5,attr,omitempty"`
	Accent6       string          `xml:"accent6,attr,omitempty"`
	Hlink         string          `xml:"hlink,attr,omitempty"`
	FolHlink      string          `xml:"folHlink,attr,omitempty"`
	CapturedAttrs []xmlb.RootAttr `xml:"-"` // verbatim source attrs; see common/xml.CaptureAttrs
}

// UnmarshalXML captures the element's verbatim attribute list (source
// attribute order and any unmodeled attributes) before decoding through the
// struct tags; the reflection marshaler replays it.
func (cm *ClrMap) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	cm.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias ClrMap
	return d.DecodeElement((*alias)(cm), &start)
}

// ClrMapOvr represents CT_ColorMappingOverride (a:clrMapOvr)
type ClrMapOvr struct {
	MasterClrMapping   *MasterClrMapping `xml:"http://schemas.openxmlformats.org/drawingml/2006/main masterClrMapping,omitempty"`
	OverrideClrMapping *ClrMap           `xml:"http://schemas.openxmlformats.org/drawingml/2006/main overrideClrMapping,omitempty"`
}

// MasterClrMapping represents CT_EmptyElement (a:masterClrMapping)
type MasterClrMapping struct{}
