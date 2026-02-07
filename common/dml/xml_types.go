// Package dml provides DrawingML types.
// This file contains XML serialization types from dml-main.xsd.
// These types are used for marshaling/unmarshaling OOXML documents.
package dml

import (
	"encoding/xml"
	"fmt"

	xmlb "github.com/mgilbir/spine/common/xml"
)

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
	Hue int32      `xml:"hue,attr"`
	Sat Percentage `xml:"sat,attr"`
	Lum Percentage `xml:"lum,attr"`
}

// PrstClr represents CT_PresetColor (a:prstClr) for XML serialization.
// Supports EG_ColorTransform child elements (tint, shade, satMod, etc.).
type PrstClr struct {
	Val    string           `xml:"val,attr"`
	Tint   *ColorTransform  `xml:"http://schemas.openxmlformats.org/drawingml/2006/main tint,omitempty"`
	Shade  *ColorTransform  `xml:"http://schemas.openxmlformats.org/drawingml/2006/main shade,omitempty"`
	Comp   *ColorTransform  `xml:"http://schemas.openxmlformats.org/drawingml/2006/main comp,omitempty"`
	Inv    *ColorTransform  `xml:"http://schemas.openxmlformats.org/drawingml/2006/main inv,omitempty"`
	Gray   *ColorTransform  `xml:"http://schemas.openxmlformats.org/drawingml/2006/main gray,omitempty"`
	Alpha  *ColorTransform  `xml:"http://schemas.openxmlformats.org/drawingml/2006/main alpha,omitempty"`
	AlphaOff *ColorTransform `xml:"http://schemas.openxmlformats.org/drawingml/2006/main alphaOff,omitempty"`
	AlphaMod *ColorTransform `xml:"http://schemas.openxmlformats.org/drawingml/2006/main alphaMod,omitempty"`
	Hue    *ColorTransform  `xml:"http://schemas.openxmlformats.org/drawingml/2006/main hue,omitempty"`
	HueOff *ColorTransform  `xml:"http://schemas.openxmlformats.org/drawingml/2006/main hueOff,omitempty"`
	HueMod *ColorTransform  `xml:"http://schemas.openxmlformats.org/drawingml/2006/main hueMod,omitempty"`
	Sat    *ColorTransform  `xml:"http://schemas.openxmlformats.org/drawingml/2006/main sat,omitempty"`
	SatOff *ColorTransform  `xml:"http://schemas.openxmlformats.org/drawingml/2006/main satOff,omitempty"`
	SatMod *ColorTransform  `xml:"http://schemas.openxmlformats.org/drawingml/2006/main satMod,omitempty"`
	Lum    *ColorTransform  `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lum,omitempty"`
	LumOff *ColorTransform  `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lumOff,omitempty"`
	LumMod *ColorTransform  `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lumMod,omitempty"`
}

// ScRgbClr represents CT_ScRgbColor (a:scrgbClr) for XML serialization
type ScRgbClr struct {
	R Percentage `xml:"r,attr"`
	G Percentage `xml:"g,attr"`
	B Percentage `xml:"b,attr"`
}

// clrTransformKind identifies a color transform element type.
type clrTransformKind int

const (
	ctTint clrTransformKind = iota
	ctShade
	ctComp
	ctInv
	ctGray
	ctAlpha
	ctAlphaMod
	ctAlphaOff
	ctHue
	ctHueMod
	ctHueOff
	ctSat
	ctSatMod
	ctSatOff
	ctLum
	ctLumMod
	ctLumOff
)

// clrTransformRef references a color transform by kind and index.
type clrTransformRef struct {
	kind  clrTransformKind
	index int
}

// SchemeClrTransform represents CT_SchemeColor with color transforms (EG_ColorTransform).
// Uses custom UnmarshalXML/MarshalToBuilder to preserve child element order (xs:choice maxOccurs="unbounded").
type SchemeClrTransform struct {
	Val      string            `xml:"val,attr"`
	Tint     []*ColorTransform `xml:"-"`
	Shade    []*ColorTransform `xml:"-"`
	Comp     []*ColorTransform `xml:"-"`
	Inv      []*ColorTransform `xml:"-"`
	Gray     []*ColorTransform `xml:"-"`
	Alpha    []*ColorTransform `xml:"-"`
	AlphaMod []*ColorTransform `xml:"-"`
	AlphaOff []*ColorTransform `xml:"-"`
	Hue      []*ColorTransform `xml:"-"`
	HueMod   []*ColorTransform `xml:"-"`
	HueOff   []*ColorTransform `xml:"-"`
	Sat      []*ColorTransform `xml:"-"`
	SatMod   []*ColorTransform `xml:"-"`
	SatOff   []*ColorTransform `xml:"-"`
	Lum      []*ColorTransform `xml:"-"`
	LumMod   []*ColorTransform `xml:"-"`
	LumOff   []*ColorTransform `xml:"-"`
	xfOrder  []clrTransformRef // tracks interleaved transform order
}

// clrTransformNameMap maps element local names to their kind.
var clrTransformNameMap = map[string]clrTransformKind{
	"tint": ctTint, "shade": ctShade, "comp": ctComp, "inv": ctInv, "gray": ctGray,
	"alpha": ctAlpha, "alphaMod": ctAlphaMod, "alphaOff": ctAlphaOff,
	"hue": ctHue, "hueMod": ctHueMod, "hueOff": ctHueOff,
	"sat": ctSat, "satMod": ctSatMod, "satOff": ctSatOff,
	"lum": ctLum, "lumMod": ctLumMod, "lumOff": ctLumOff,
}

// clrTransformKindName maps kind back to element local name.
var clrTransformKindName = map[clrTransformKind]string{
	ctTint: "tint", ctShade: "shade", ctComp: "comp", ctInv: "inv", ctGray: "gray",
	ctAlpha: "alpha", ctAlphaMod: "alphaMod", ctAlphaOff: "alphaOff",
	ctHue: "hue", ctHueMod: "hueMod", ctHueOff: "hueOff",
	ctSat: "sat", ctSatMod: "satMod", ctSatOff: "satOff",
	ctLum: "lum", ctLumMod: "lumMod", ctLumOff: "lumOff",
}

// UnmarshalXML implements custom unmarshaling to preserve color transform order.
func (s *SchemeClrTransform) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		if attr.Name.Local == "val" {
			s.Val = attr.Value
		}
	}
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			kind, ok := clrTransformNameMap[t.Name.Local]
			if !ok {
				if err := d.Skip(); err != nil {
					return err
				}
				continue
			}
			ct := &ColorTransform{}
			if err := d.DecodeElement(ct, &t); err != nil {
				return err
			}
			slice := s.sliceForKind(kind)
			s.xfOrder = append(s.xfOrder, clrTransformRef{kind, len(*slice)})
			*slice = append(*slice, ct)
		case xml.EndElement:
			return nil
		}
	}
}

// sliceForKind returns a pointer to the slice for a given color transform kind.
func (s *SchemeClrTransform) sliceForKind(kind clrTransformKind) *[]*ColorTransform {
	switch kind {
	case ctTint:
		return &s.Tint
	case ctShade:
		return &s.Shade
	case ctComp:
		return &s.Comp
	case ctInv:
		return &s.Inv
	case ctGray:
		return &s.Gray
	case ctAlpha:
		return &s.Alpha
	case ctAlphaMod:
		return &s.AlphaMod
	case ctAlphaOff:
		return &s.AlphaOff
	case ctHue:
		return &s.Hue
	case ctHueMod:
		return &s.HueMod
	case ctHueOff:
		return &s.HueOff
	case ctSat:
		return &s.Sat
	case ctSatMod:
		return &s.SatMod
	case ctSatOff:
		return &s.SatOff
	case ctLum:
		return &s.Lum
	case ctLumMod:
		return &s.LumMod
	case ctLumOff:
		return &s.LumOff
	default:
		return &s.Tint // shouldn't happen
	}
}

// MarshalToBuilder implements xmlb.BuilderMarshaler to preserve color transform order.
func (s *SchemeClrTransform) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	if len(s.xfOrder) == 0 && !s.hasAnyTransforms() {
		b.EmptyElement(ns, localName, xmlb.StrAttr("val", s.Val))
		return
	}
	b.StartElement(ns, localName, xmlb.StrAttr("val", s.Val))
	if len(s.xfOrder) > 0 {
		for _, ref := range s.xfOrder {
			name := clrTransformKindName[ref.kind]
			slice := s.sliceForKind(ref.kind)
			if ref.index < len(*slice) {
				ct := (*slice)[ref.index]
				b.EmptyElement(ns, name, xmlb.Int32Attr("val", int32(ct.Val)))
			}
		}
	} else {
		// No order tracking - write all non-nil transforms
		s.writeAllTransforms(b, ns)
	}
	b.EndElement(ns, localName)
}

// hasAnyTransforms returns true if any color transforms are set.
func (s *SchemeClrTransform) hasAnyTransforms() bool {
	return len(s.Tint) > 0 || len(s.Shade) > 0 || len(s.Comp) > 0 || len(s.Inv) > 0 ||
		len(s.Gray) > 0 || len(s.Alpha) > 0 || len(s.AlphaMod) > 0 || len(s.AlphaOff) > 0 ||
		len(s.Hue) > 0 || len(s.HueMod) > 0 || len(s.HueOff) > 0 ||
		len(s.Sat) > 0 || len(s.SatMod) > 0 || len(s.SatOff) > 0 ||
		len(s.Lum) > 0 || len(s.LumMod) > 0 || len(s.LumOff) > 0
}

// writeAllTransforms writes all transforms in a default order (no ordering preserved).
func (s *SchemeClrTransform) writeAllTransforms(b *xmlb.Builder, ns string) {
	allSlices := []struct {
		name  string
		slice []*ColorTransform
	}{
		{"tint", s.Tint}, {"shade", s.Shade}, {"comp", s.Comp}, {"inv", s.Inv}, {"gray", s.Gray},
		{"alpha", s.Alpha}, {"alphaMod", s.AlphaMod}, {"alphaOff", s.AlphaOff},
		{"hue", s.Hue}, {"hueMod", s.HueMod}, {"hueOff", s.HueOff},
		{"sat", s.Sat}, {"satMod", s.SatMod}, {"satOff", s.SatOff},
		{"lum", s.Lum}, {"lumMod", s.LumMod}, {"lumOff", s.LumOff},
	}
	for _, entry := range allSlices {
		for _, ct := range entry.slice {
			b.EmptyElement(ns, entry.name, xmlb.Int32Attr("val", int32(ct.Val)))
		}
	}
}

// MarshalXML implements xml.Marshaler for Go's encoding/xml (used by tests).
func (s SchemeClrTransform) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "val"}, Value: s.Val})
	type entry struct {
		name  string
		slice []*ColorTransform
	}
	all := []entry{
		{"tint", s.Tint}, {"shade", s.Shade}, {"comp", s.Comp}, {"inv", s.Inv}, {"gray", s.Gray},
		{"alpha", s.Alpha}, {"alphaMod", s.AlphaMod}, {"alphaOff", s.AlphaOff},
		{"hue", s.Hue}, {"hueMod", s.HueMod}, {"hueOff", s.HueOff},
		{"sat", s.Sat}, {"satMod", s.SatMod}, {"satOff", s.SatOff},
		{"lum", s.Lum}, {"lumMod", s.LumMod}, {"lumOff", s.LumOff},
	}
	hasChildren := false
	for _, a := range all {
		if len(a.slice) > 0 {
			hasChildren = true
			break
		}
	}
	if !hasChildren {
		return e.EncodeElement(struct{}{}, start)
	}
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	if len(s.xfOrder) > 0 {
		for _, ref := range s.xfOrder {
			name := clrTransformKindName[ref.kind]
			slice := s.sliceForKind(ref.kind)
			if ref.index < len(*slice) {
				ct := (*slice)[ref.index]
				elem := xml.StartElement{Name: xml.Name{Local: name}}
				elem.Attr = append(elem.Attr, xml.Attr{Name: xml.Name{Local: "val"}, Value: fmt.Sprintf("%d", ct.Val)})
				if err := e.EncodeElement(struct{}{}, elem); err != nil {
					return err
				}
			}
		}
	} else {
		for _, a := range all {
			for _, ct := range a.slice {
				elem := xml.StartElement{Name: xml.Name{Local: a.name}}
				elem.Attr = append(elem.Attr, xml.Attr{Name: xml.Name{Local: "val"}, Value: fmt.Sprintf("%d", ct.Val)})
				if err := e.EncodeElement(struct{}{}, elem); err != nil {
					return err
				}
			}
		}
	}
	return e.EncodeToken(start.End())
}

// ColorTransform represents a color transform (a:tint, a:shade, a:alpha, etc.)
type ColorTransform struct {
	Val Percentage `xml:"val,attr"`
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
	ScRgbClr  *ScRgbClr           `xml:"http://schemas.openxmlformats.org/drawingml/2006/main scrgbClr,omitempty"`
	SrgbClr   *SrgbClr            `xml:"http://schemas.openxmlformats.org/drawingml/2006/main srgbClr,omitempty"`
	HslClr    *HslClr             `xml:"http://schemas.openxmlformats.org/drawingml/2006/main hslClr,omitempty"`
	SysClr    *SystemClr          `xml:"http://schemas.openxmlformats.org/drawingml/2006/main sysClr,omitempty"`
	SchemeClr *SchemeClrTransform `xml:"http://schemas.openxmlformats.org/drawingml/2006/main schemeClr,omitempty"`
	PrstClr   *PrstClr            `xml:"http://schemas.openxmlformats.org/drawingml/2006/main prstClr,omitempty"`
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
	Dpi          *int32   `xml:"dpi,attr,omitempty"`
	RotWithShape *bool    `xml:"rotWithShape,attr,omitempty"`
	Blip         *BlipXML `xml:"http://schemas.openxmlformats.org/drawingml/2006/main blip,omitempty"`
	SrcRect      *RelRect `xml:"http://schemas.openxmlformats.org/drawingml/2006/main srcRect,omitempty"`
	Tile         *TileXML `xml:"http://schemas.openxmlformats.org/drawingml/2006/main tile,omitempty"`
	Stretch      *StretchXML `xml:"http://schemas.openxmlformats.org/drawingml/2006/main stretch,omitempty"`
}

// BlipXML represents CT_Blip (a:blip)
type BlipXML struct {
	Embed  string  `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships embed,attr,omitempty"`
	Link   string  `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships link,attr,omitempty"`
	Cstate string  `xml:"cstate,attr,omitempty"`
	ExtLst *ExtLst `xml:"http://schemas.openxmlformats.org/drawingml/2006/main extLst,omitempty"`
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
