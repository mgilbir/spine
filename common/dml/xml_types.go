// This file provides DrawingML types.
// This file contains XML serialization types from dml-main.xsd.
// These types are used for marshaling/unmarshaling OOXML documents.

package dml

import (
	"bytes"
	"encoding/xml"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// XML namespace constants
const (
	NsDrawingML     = "http://schemas.openxmlformats.org/drawingml/2006/main"
	NsRelationships = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
)

// --- Color XML Types ---

// SrgbClr represents CT_SRgbColor (a:srgbClr) for XML serialization.
// All EG_ColorTransform kinds are modeled (single occurrence each) so a
// transform child is never silently deleted on re-marshal.
type SrgbClr struct {
	Val      string             `xml:"val,attr"`
	Tint     *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main tint,omitempty"`
	Shade    *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main shade,omitempty"`
	SatMod   *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main satMod,omitempty"`
	Alpha    *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main alpha,omitempty"`
	LumMod   *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lumMod,omitempty"`
	LumOff   *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lumOff,omitempty"`
	Comp     *EmptyClrTransform `xml:"http://schemas.openxmlformats.org/drawingml/2006/main comp,omitempty"`
	Inv      *EmptyClrTransform `xml:"http://schemas.openxmlformats.org/drawingml/2006/main inv,omitempty"`
	Gray     *EmptyClrTransform `xml:"http://schemas.openxmlformats.org/drawingml/2006/main gray,omitempty"`
	AlphaOff *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main alphaOff,omitempty"`
	AlphaMod *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main alphaMod,omitempty"`
	Hue      *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main hue,omitempty"`
	HueOff   *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main hueOff,omitempty"`
	HueMod   *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main hueMod,omitempty"`
	Sat      *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main sat,omitempty"`
	SatOff   *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main satOff,omitempty"`
	Lum      *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lum,omitempty"`
	Red      *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main red,omitempty"`
	RedOff   *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main redOff,omitempty"`
	RedMod   *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main redMod,omitempty"`
	Green    *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main green,omitempty"`
	GreenOff *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main greenOff,omitempty"`
	GreenMod *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main greenMod,omitempty"`
	Blue     *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main blue,omitempty"`
	BlueOff  *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main blueOff,omitempty"`
	BlueMod  *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main blueMod,omitempty"`
	Gamma    *EmptyClrTransform `xml:"http://schemas.openxmlformats.org/drawingml/2006/main gamma,omitempty"`
	InvGamma *EmptyClrTransform `xml:"http://schemas.openxmlformats.org/drawingml/2006/main invGamma,omitempty"`
	xfOrder  []clrTransformKind // captured source order of the transform children
}

// SystemClr represents CT_SystemColor (a:sysClr) for XML serialization.
// All EG_ColorTransform kinds are modeled (single occurrence each); see SrgbClr.
type SystemClr struct {
	Val      string             `xml:"val,attr"`
	LastClr  string             `xml:"lastClr,attr,omitempty"`
	Tint     *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main tint,omitempty"`
	Shade    *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main shade,omitempty"`
	SatMod   *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main satMod,omitempty"`
	Alpha    *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main alpha,omitempty"`
	LumMod   *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lumMod,omitempty"`
	LumOff   *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lumOff,omitempty"`
	Comp     *EmptyClrTransform `xml:"http://schemas.openxmlformats.org/drawingml/2006/main comp,omitempty"`
	Inv      *EmptyClrTransform `xml:"http://schemas.openxmlformats.org/drawingml/2006/main inv,omitempty"`
	Gray     *EmptyClrTransform `xml:"http://schemas.openxmlformats.org/drawingml/2006/main gray,omitempty"`
	AlphaOff *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main alphaOff,omitempty"`
	AlphaMod *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main alphaMod,omitempty"`
	Hue      *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main hue,omitempty"`
	HueOff   *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main hueOff,omitempty"`
	HueMod   *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main hueMod,omitempty"`
	Sat      *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main sat,omitempty"`
	SatOff   *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main satOff,omitempty"`
	Lum      *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lum,omitempty"`
	Red      *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main red,omitempty"`
	RedOff   *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main redOff,omitempty"`
	RedMod   *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main redMod,omitempty"`
	Green    *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main green,omitempty"`
	GreenOff *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main greenOff,omitempty"`
	GreenMod *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main greenMod,omitempty"`
	Blue     *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main blue,omitempty"`
	BlueOff  *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main blueOff,omitempty"`
	BlueMod  *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main blueMod,omitempty"`
	Gamma    *EmptyClrTransform `xml:"http://schemas.openxmlformats.org/drawingml/2006/main gamma,omitempty"`
	InvGamma *EmptyClrTransform `xml:"http://schemas.openxmlformats.org/drawingml/2006/main invGamma,omitempty"`
	xfOrder  []clrTransformKind // captured source order of the transform children
}

// HslClr represents CT_HslColor (a:hslClr) for XML serialization.
// All EG_ColorTransform kinds are modeled (single occurrence each); see SrgbClr.
type HslClr struct {
	Hue      int32              `xml:"hue,attr"`
	Sat      Percentage         `xml:"sat,attr"`
	Lum      Percentage         `xml:"lum,attr"`
	Tint     *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main tint,omitempty"`
	Shade    *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main shade,omitempty"`
	Comp     *EmptyClrTransform `xml:"http://schemas.openxmlformats.org/drawingml/2006/main comp,omitempty"`
	Inv      *EmptyClrTransform `xml:"http://schemas.openxmlformats.org/drawingml/2006/main inv,omitempty"`
	Gray     *EmptyClrTransform `xml:"http://schemas.openxmlformats.org/drawingml/2006/main gray,omitempty"`
	Alpha    *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main alpha,omitempty"`
	AlphaOff *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main alphaOff,omitempty"`
	AlphaMod *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main alphaMod,omitempty"`
	HueXf    *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main hue,omitempty"`
	HueOff   *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main hueOff,omitempty"`
	HueMod   *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main hueMod,omitempty"`
	SatXf    *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main sat,omitempty"`
	SatOff   *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main satOff,omitempty"`
	SatMod   *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main satMod,omitempty"`
	LumXf    *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lum,omitempty"`
	LumOff   *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lumOff,omitempty"`
	LumMod   *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lumMod,omitempty"`
	Red      *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main red,omitempty"`
	RedOff   *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main redOff,omitempty"`
	RedMod   *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main redMod,omitempty"`
	Green    *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main green,omitempty"`
	GreenOff *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main greenOff,omitempty"`
	GreenMod *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main greenMod,omitempty"`
	Blue     *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main blue,omitempty"`
	BlueOff  *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main blueOff,omitempty"`
	BlueMod  *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main blueMod,omitempty"`
	Gamma    *EmptyClrTransform `xml:"http://schemas.openxmlformats.org/drawingml/2006/main gamma,omitempty"`
	InvGamma *EmptyClrTransform `xml:"http://schemas.openxmlformats.org/drawingml/2006/main invGamma,omitempty"`
	xfOrder  []clrTransformKind // captured source order of the transform children
}

// PrstClr represents CT_PresetColor (a:prstClr) for XML serialization.
// All EG_ColorTransform kinds are modeled (single occurrence each); see SrgbClr.
type PrstClr struct {
	Val      string             `xml:"val,attr"`
	Tint     *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main tint,omitempty"`
	Shade    *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main shade,omitempty"`
	Comp     *EmptyClrTransform `xml:"http://schemas.openxmlformats.org/drawingml/2006/main comp,omitempty"`
	Inv      *EmptyClrTransform `xml:"http://schemas.openxmlformats.org/drawingml/2006/main inv,omitempty"`
	Gray     *EmptyClrTransform `xml:"http://schemas.openxmlformats.org/drawingml/2006/main gray,omitempty"`
	Alpha    *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main alpha,omitempty"`
	AlphaOff *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main alphaOff,omitempty"`
	AlphaMod *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main alphaMod,omitempty"`
	Hue      *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main hue,omitempty"`
	HueOff   *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main hueOff,omitempty"`
	HueMod   *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main hueMod,omitempty"`
	Sat      *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main sat,omitempty"`
	SatOff   *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main satOff,omitempty"`
	SatMod   *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main satMod,omitempty"`
	Lum      *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lum,omitempty"`
	LumOff   *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lumOff,omitempty"`
	LumMod   *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lumMod,omitempty"`
	Red      *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main red,omitempty"`
	RedOff   *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main redOff,omitempty"`
	RedMod   *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main redMod,omitempty"`
	Green    *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main green,omitempty"`
	GreenOff *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main greenOff,omitempty"`
	GreenMod *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main greenMod,omitempty"`
	Blue     *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main blue,omitempty"`
	BlueOff  *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main blueOff,omitempty"`
	BlueMod  *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main blueMod,omitempty"`
	Gamma    *EmptyClrTransform `xml:"http://schemas.openxmlformats.org/drawingml/2006/main gamma,omitempty"`
	InvGamma *EmptyClrTransform `xml:"http://schemas.openxmlformats.org/drawingml/2006/main invGamma,omitempty"`
	xfOrder  []clrTransformKind // captured source order of the transform children
}

// ScRgbClr represents CT_ScRgbColor (a:scrgbClr) for XML serialization.
// All EG_ColorTransform kinds are modeled (single occurrence each); see SrgbClr.
type ScRgbClr struct {
	R        Percentage         `xml:"r,attr"`
	G        Percentage         `xml:"g,attr"`
	B        Percentage         `xml:"b,attr"`
	Tint     *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main tint,omitempty"`
	Shade    *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main shade,omitempty"`
	Comp     *EmptyClrTransform `xml:"http://schemas.openxmlformats.org/drawingml/2006/main comp,omitempty"`
	Inv      *EmptyClrTransform `xml:"http://schemas.openxmlformats.org/drawingml/2006/main inv,omitempty"`
	Gray     *EmptyClrTransform `xml:"http://schemas.openxmlformats.org/drawingml/2006/main gray,omitempty"`
	Alpha    *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main alpha,omitempty"`
	AlphaOff *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main alphaOff,omitempty"`
	AlphaMod *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main alphaMod,omitempty"`
	Hue      *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main hue,omitempty"`
	HueOff   *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main hueOff,omitempty"`
	HueMod   *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main hueMod,omitempty"`
	Sat      *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main sat,omitempty"`
	SatOff   *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main satOff,omitempty"`
	SatMod   *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main satMod,omitempty"`
	Lum      *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lum,omitempty"`
	LumOff   *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lumOff,omitempty"`
	LumMod   *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lumMod,omitempty"`
	Red      *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main red,omitempty"`
	RedOff   *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main redOff,omitempty"`
	RedMod   *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main redMod,omitempty"`
	Green    *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main green,omitempty"`
	GreenOff *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main greenOff,omitempty"`
	GreenMod *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main greenMod,omitempty"`
	Blue     *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main blue,omitempty"`
	BlueOff  *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main blueOff,omitempty"`
	BlueMod  *ColorTransform    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main blueMod,omitempty"`
	Gamma    *EmptyClrTransform `xml:"http://schemas.openxmlformats.org/drawingml/2006/main gamma,omitempty"`
	InvGamma *EmptyClrTransform `xml:"http://schemas.openxmlformats.org/drawingml/2006/main invGamma,omitempty"`
	xfOrder  []clrTransformKind // captured source order of the transform children
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
	ctRed
	ctRedMod
	ctRedOff
	ctGreen
	ctGreenMod
	ctGreenOff
	ctBlue
	ctBlueMod
	ctBlueOff
	ctGamma
	ctInvGamma
)

// clrTransformArgless marks the EG_ColorTransform kinds whose complex types are
// EMPTY in the XSD (CT_ComplementTransform, CT_InverseTransform,
// CT_GrayscaleTransform, CT_GammaTransform, CT_InverseGammaTransform): they
// carry no val attribute and must be emitted as bare empty elements.
var clrTransformArgless = map[clrTransformKind]bool{
	ctComp: true, ctInv: true, ctGray: true, ctGamma: true, ctInvGamma: true,
}

// clrTransformAllKinds lists every kind in XSD declaration order, used for the
// grouped fallback when no interleaved order was captured.
var clrTransformAllKinds = []clrTransformKind{
	ctTint, ctShade, ctComp, ctInv, ctGray,
	ctAlpha, ctAlphaOff, ctAlphaMod,
	ctHue, ctHueOff, ctHueMod,
	ctSat, ctSatOff, ctSatMod,
	ctLum, ctLumOff, ctLumMod,
	ctRed, ctRedOff, ctRedMod,
	ctGreen, ctGreenOff, ctGreenMod,
	ctBlue, ctBlueOff, ctBlueMod,
	ctGamma, ctInvGamma,
}

// clrTransformRef references a color transform by kind and index.
type clrTransformRef struct {
	kind  clrTransformKind
	index int
}

// SchemeClrTransform represents CT_SchemeColor with color transforms (EG_ColorTransform).
// All 28 EG_ColorTransform kinds are modeled. Uses custom UnmarshalXML/MarshalToBuilder
// to preserve child element order (xs:choice maxOccurs="unbounded").
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
	Red      []*ColorTransform `xml:"-"`
	RedMod   []*ColorTransform `xml:"-"`
	RedOff   []*ColorTransform `xml:"-"`
	Green    []*ColorTransform `xml:"-"`
	GreenMod []*ColorTransform `xml:"-"`
	GreenOff []*ColorTransform `xml:"-"`
	Blue     []*ColorTransform `xml:"-"`
	BlueMod  []*ColorTransform `xml:"-"`
	BlueOff  []*ColorTransform `xml:"-"`
	Gamma    []*ColorTransform `xml:"-"`
	InvGamma []*ColorTransform `xml:"-"`
	xfOrder  []clrTransformRef // tracks interleaved transform order
}

// clrTransformNameMap maps element local names to their kind.
var clrTransformNameMap = map[string]clrTransformKind{
	"tint": ctTint, "shade": ctShade, "comp": ctComp, "inv": ctInv, "gray": ctGray,
	"alpha": ctAlpha, "alphaMod": ctAlphaMod, "alphaOff": ctAlphaOff,
	"hue": ctHue, "hueMod": ctHueMod, "hueOff": ctHueOff,
	"sat": ctSat, "satMod": ctSatMod, "satOff": ctSatOff,
	"lum": ctLum, "lumMod": ctLumMod, "lumOff": ctLumOff,
	"red": ctRed, "redMod": ctRedMod, "redOff": ctRedOff,
	"green": ctGreen, "greenMod": ctGreenMod, "greenOff": ctGreenOff,
	"blue": ctBlue, "blueMod": ctBlueMod, "blueOff": ctBlueOff,
	"gamma": ctGamma, "invGamma": ctInvGamma,
}

// clrTransformKindName maps kind back to element local name.
var clrTransformKindName = map[clrTransformKind]string{
	ctTint: "tint", ctShade: "shade", ctComp: "comp", ctInv: "inv", ctGray: "gray",
	ctAlpha: "alpha", ctAlphaMod: "alphaMod", ctAlphaOff: "alphaOff",
	ctHue: "hue", ctHueMod: "hueMod", ctHueOff: "hueOff",
	ctSat: "sat", ctSatMod: "satMod", ctSatOff: "satOff",
	ctLum: "lum", ctLumMod: "lumMod", ctLumOff: "lumOff",
	ctRed: "red", ctRedMod: "redMod", ctRedOff: "redOff",
	ctGreen: "green", ctGreenMod: "greenMod", ctGreenOff: "greenOff",
	ctBlue: "blue", ctBlueMod: "blueMod", ctBlueOff: "blueOff",
	ctGamma: "gamma", ctInvGamma: "invGamma",
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
	case ctRed:
		return &s.Red
	case ctRedMod:
		return &s.RedMod
	case ctRedOff:
		return &s.RedOff
	case ctGreen:
		return &s.Green
	case ctGreenMod:
		return &s.GreenMod
	case ctGreenOff:
		return &s.GreenOff
	case ctBlue:
		return &s.Blue
	case ctBlueMod:
		return &s.BlueMod
	case ctBlueOff:
		return &s.BlueOff
	case ctGamma:
		return &s.Gamma
	case ctInvGamma:
		return &s.InvGamma
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
			slice := s.sliceForKind(ref.kind)
			if ref.index < len(*slice) {
				writeClrTransform(b, ns, ref.kind, (*slice)[ref.index])
			}
		}
	} else {
		// No order tracking - write all non-nil transforms
		s.writeAllTransforms(b, ns)
	}
	b.EndElement(ns, localName)
}

// writeClrTransform writes a single color transform element. Arg-less kinds
// (comp, inv, gray, gamma, invGamma) have EMPTY complex types and carry no
// val attribute.
func writeClrTransform(b *xmlb.Builder, ns string, kind clrTransformKind, ct *ColorTransform) {
	name := clrTransformKindName[kind]
	if clrTransformArgless[kind] {
		b.EmptyElement(ns, name)
		return
	}
	b.EmptyElement(ns, name, xmlb.StrAttr("val", ct.Val.AttrValue()))
}

// hasAnyTransforms returns true if any color transforms are set.
func (s *SchemeClrTransform) hasAnyTransforms() bool {
	for _, kind := range clrTransformAllKinds {
		if len(*s.sliceForKind(kind)) > 0 {
			return true
		}
	}
	return false
}

// writeAllTransforms writes all transforms in a default order (no ordering preserved).
func (s *SchemeClrTransform) writeAllTransforms(b *xmlb.Builder, ns string) {
	for _, kind := range clrTransformAllKinds {
		for _, ct := range *s.sliceForKind(kind) {
			writeClrTransform(b, ns, kind, ct)
		}
	}
}

// encodeClrTransform writes a single color transform via encoding/xml,
// mirroring writeClrTransform for the non-Builder serializer.
func encodeClrTransform(e *xml.Encoder, kind clrTransformKind, ct *ColorTransform) error {
	elem := xml.StartElement{Name: xml.Name{Local: clrTransformKindName[kind]}}
	if !clrTransformArgless[kind] {
		elem.Attr = append(elem.Attr, xml.Attr{Name: xml.Name{Local: "val"}, Value: ct.Val.AttrValue()})
	}
	return e.EncodeElement(struct{}{}, elem)
}

// MarshalXML implements xml.Marshaler for Go's encoding/xml (used by tests).
func (s SchemeClrTransform) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "val"}, Value: s.Val})
	if !s.hasAnyTransforms() {
		return e.EncodeElement(struct{}{}, start)
	}
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	if len(s.xfOrder) > 0 {
		for _, ref := range s.xfOrder {
			slice := s.sliceForKind(ref.kind)
			if ref.index < len(*slice) {
				if err := encodeClrTransform(e, ref.kind, (*slice)[ref.index]); err != nil {
					return err
				}
			}
		}
	} else {
		for _, kind := range clrTransformAllKinds {
			for _, ct := range *s.sliceForKind(kind) {
				if err := encodeClrTransform(e, kind, ct); err != nil {
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

// EmptyClrTransform represents the arg-less color transforms (a:comp, a:inv,
// a:gray, a:gamma, a:invGamma). Their complex types are EMPTY in the XSD: no
// attributes, no content.
type EmptyClrTransform struct{}

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

// GradFill represents CT_GradientFillProperties (a:gradFill).
// rotWithShape is optional with no XSD default, so it is a pointer: an explicit
// "0" must round-trip instead of being deleted (flipping "explicitly false"
// to "unspecified", which some renderers default to true).
type GradFill struct {
	Flip         string   `xml:"flip,attr,omitempty"`
	RotWithShape *bool    `xml:"rotWithShape,attr,omitempty"`
	GsLst        *GsLst   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main gsLst,omitempty"`
	Lin          *Lin     `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lin,omitempty"`
	PathShade    *PathXML `xml:"http://schemas.openxmlformats.org/drawingml/2006/main path,omitempty"`
	TileRect     *RelRect `xml:"http://schemas.openxmlformats.org/drawingml/2006/main tileRect,omitempty"`
}

// GsLst represents CT_GradientStopList (a:gsLst)
type GsLst struct {
	Gs []*Gs `xml:"http://schemas.openxmlformats.org/drawingml/2006/main gs"`
}

// Gs represents CT_GradientStop (a:gs). It carries exactly one EG_ColorChoice
// child; all six color kinds must be supported or a stop using, e.g., a scheme
// or system color would be dropped, leaving <a:gs> with no color child.
type Gs struct {
	Pos       Percentage          `xml:"pos,attr"`
	ScRgbClr  *ScRgbClr           `xml:"http://schemas.openxmlformats.org/drawingml/2006/main scrgbClr,omitempty"`
	SrgbClr   *SrgbClr            `xml:"http://schemas.openxmlformats.org/drawingml/2006/main srgbClr,omitempty"`
	HslClr    *HslClr             `xml:"http://schemas.openxmlformats.org/drawingml/2006/main hslClr,omitempty"`
	SysClr    *SystemClr          `xml:"http://schemas.openxmlformats.org/drawingml/2006/main sysClr,omitempty"`
	SchemeClr *SchemeClrTransform `xml:"http://schemas.openxmlformats.org/drawingml/2006/main schemeClr,omitempty"`
	PrstClr   *PrstClr            `xml:"http://schemas.openxmlformats.org/drawingml/2006/main prstClr,omitempty"`
}

// Lin represents CT_LinearShadeProperties (a:lin).
// scaled is optional with no XSD default, so it is a pointer; see GradFill.
type Lin struct {
	// Ang is a pointer so an explicit ang="0" survives the round trip.
	Ang    *int32 `xml:"ang,attr,omitempty"`
	Scaled *bool  `xml:"scaled,attr,omitempty"`
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
	Dpi          *int32      `xml:"dpi,attr,omitempty"`
	RotWithShape *bool       `xml:"rotWithShape,attr,omitempty"`
	Blip         *BlipXML    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main blip,omitempty"`
	SrcRect      *RelRect    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main srcRect,omitempty"`
	Tile         *TileXML    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main tile,omitempty"`
	Stretch      *StretchXML `xml:"http://schemas.openxmlformats.org/drawingml/2006/main stretch,omitempty"`
}

// BlipXML represents CT_Blip (a:blip). Its effect children form a repeated
// xs:choice, so they are kept as an ordered list: every typed effect kind is
// modeled, and anything else is captured raw so no child is dropped on save.
// (See also Blip in xml_picture.go — a grouped duplicate kept for spectest;
// consolidating the two is a deeper refactor.)
type BlipXML struct {
	Embed   string        `xml:"-"` // r:embed attribute (namespaced)
	Link    string        `xml:"-"` // r:link attribute (namespaced)
	Cstate  string        `xml:"-"`
	Effects []*BlipEffect `xml:"-"`
	ExtLst  *ExtLst       `xml:"-"`
}

// BlipEffect is one effect child of CT_Blip. Exactly one field is set. The 17
// typed kinds cover the XSD choice; RawName/RawAttrs/Raw preserve any other
// child (typed dispatch must never be lossier than raw capture).
type BlipEffect struct {
	AlphaBiLevel *AlphaBiLevel
	AlphaCeiling *AlphaCeiling
	AlphaFloor   *AlphaFloor
	AlphaInv     *AlphaInv
	AlphaMod     *AlphaMod
	AlphaModFix  *AlphaModFix
	AlphaRepl    *AlphaRepl
	BiLevel      *BiLevelXML
	Blur         *BlurXML
	ClrChange    *ClrChange
	ClrRepl      *ClrRepl
	Duotone      *Duotone
	FillOverlay  *FillOverlayXML
	Grayscl      *GrayscaleXML
	Hsl          *HslXML
	Lum          *LumXML
	Tint         *TintEffectXML

	RawName  xml.Name
	RawAttrs []xml.Attr
	Raw      []byte // inner XML of an unmodeled child
}

// UnmarshalXML implements custom unmarshaling for BlipXML, preserving the
// order of effect children and capturing unmodeled ones raw.
func (v *BlipXML) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		switch {
		case attr.Name.Local == "embed" && attr.Name.Space == xmlb.NSOfficeDocumentRels:
			v.Embed = attr.Value
		case attr.Name.Local == "link" && attr.Name.Space == xmlb.NSOfficeDocumentRels:
			v.Link = attr.Value
		case attr.Name.Local == "cstate":
			v.Cstate = attr.Value
		}
	}
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			eff := &BlipEffect{}
			var target interface{}
			switch t.Name.Local {
			case "alphaBiLevel":
				eff.AlphaBiLevel = &AlphaBiLevel{}
				target = eff.AlphaBiLevel
			case "alphaCeiling":
				eff.AlphaCeiling = &AlphaCeiling{}
				target = eff.AlphaCeiling
			case "alphaFloor":
				eff.AlphaFloor = &AlphaFloor{}
				target = eff.AlphaFloor
			case "alphaInv":
				eff.AlphaInv = &AlphaInv{}
				target = eff.AlphaInv
			case "alphaMod":
				eff.AlphaMod = &AlphaMod{}
				target = eff.AlphaMod
			case "alphaModFix":
				eff.AlphaModFix = &AlphaModFix{}
				target = eff.AlphaModFix
			case "alphaRepl":
				eff.AlphaRepl = &AlphaRepl{}
				target = eff.AlphaRepl
			case "biLevel":
				eff.BiLevel = &BiLevelXML{}
				target = eff.BiLevel
			case "blur":
				eff.Blur = &BlurXML{}
				target = eff.Blur
			case "clrChange":
				eff.ClrChange = &ClrChange{}
				target = eff.ClrChange
			case "clrRepl":
				eff.ClrRepl = &ClrRepl{}
				target = eff.ClrRepl
			case "duotone":
				eff.Duotone = &Duotone{}
				target = eff.Duotone
			case "fillOverlay":
				eff.FillOverlay = &FillOverlayXML{}
				target = eff.FillOverlay
			case "grayscl":
				eff.Grayscl = &GrayscaleXML{}
				target = eff.Grayscl
			case "hsl":
				eff.Hsl = &HslXML{}
				target = eff.Hsl
			case "lum":
				eff.Lum = &LumXML{}
				target = eff.Lum
			case "tint":
				eff.Tint = &TintEffectXML{}
				target = eff.Tint
			case "extLst":
				v.ExtLst = &ExtLst{}
				if err := d.DecodeElement(v.ExtLst, &t); err != nil {
					return err
				}
				continue
			default:
				// Unmodeled child: capture name, attributes and inner XML raw.
				eff.RawName = t.Name
				eff.RawAttrs = append([]xml.Attr(nil), t.Attr...)
				var inner struct {
					Content []byte `xml:",innerxml"`
				}
				if err := d.DecodeElement(&inner, &t); err != nil {
					return err
				}
				eff.Raw = inner.Content
				v.Effects = append(v.Effects, eff)
				continue
			}
			if err := d.DecodeElement(target, &t); err != nil {
				return err
			}
			v.Effects = append(v.Effects, eff)
		case xml.EndElement:
			return nil
		}
	}
}

// MarshalToBuilder implements xmlb.BuilderMarshaler, emitting effect children
// in their captured order.
func (v *BlipXML) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	var attrs []xmlb.Attr
	if v.Embed != "" {
		attrs = append(attrs, xmlb.RelAttr("embed", v.Embed))
	}
	if v.Link != "" {
		attrs = append(attrs, xmlb.RelAttr("link", v.Link))
	}
	if v.Cstate != "" {
		attrs = append(attrs, xmlb.StrAttr("cstate", v.Cstate))
	}
	if len(v.Effects) == 0 && v.ExtLst == nil {
		b.EmptyElement(ns, localName, attrs...)
		return
	}
	b.StartElement(ns, localName, attrs...)
	for _, eff := range v.Effects {
		eff.marshalToBuilder(b, ns)
	}
	if v.ExtLst != nil {
		b.MarshalElement(ns, "extLst", v.ExtLst)
	}
	b.EndElement(ns, localName)
}

// typedChild returns the element local name and value for the typed kind set
// on the effect, or ok=false when the effect is a raw capture.
func (v *BlipEffect) typedChild() (name string, value interface{}, ok bool) {
	switch {
	case v.AlphaBiLevel != nil:
		return "alphaBiLevel", v.AlphaBiLevel, true
	case v.AlphaCeiling != nil:
		return "alphaCeiling", v.AlphaCeiling, true
	case v.AlphaFloor != nil:
		return "alphaFloor", v.AlphaFloor, true
	case v.AlphaInv != nil:
		return "alphaInv", v.AlphaInv, true
	case v.AlphaMod != nil:
		return "alphaMod", v.AlphaMod, true
	case v.AlphaModFix != nil:
		return "alphaModFix", v.AlphaModFix, true
	case v.AlphaRepl != nil:
		return "alphaRepl", v.AlphaRepl, true
	case v.BiLevel != nil:
		return "biLevel", v.BiLevel, true
	case v.Blur != nil:
		return "blur", v.Blur, true
	case v.ClrChange != nil:
		return "clrChange", v.ClrChange, true
	case v.ClrRepl != nil:
		return "clrRepl", v.ClrRepl, true
	case v.Duotone != nil:
		return "duotone", v.Duotone, true
	case v.FillOverlay != nil:
		return "fillOverlay", v.FillOverlay, true
	case v.Grayscl != nil:
		return "grayscl", v.Grayscl, true
	case v.Hsl != nil:
		return "hsl", v.Hsl, true
	case v.Lum != nil:
		return "lum", v.Lum, true
	case v.Tint != nil:
		return "tint", v.Tint, true
	}
	return "", nil, false
}

func (v *BlipEffect) marshalToBuilder(b *xmlb.Builder, ns string) {
	if name, value, ok := v.typedChild(); ok {
		b.MarshalElement(ns, name, value)
		return
	}
	if v.RawName.Local == "" {
		return
	}
	rns := v.RawName.Space
	if rns == "" {
		rns = ns
	}
	// Replay xmlns declarations the element carried as literal attributes so
	// prefixes used by the element name, its attributes, or Raw stay bound.
	// If a declaration binds the element's own namespace, emit through the
	// inline-NS path so the Builder resolves the element name to that prefix
	// (an unregistered namespace would otherwise be an error).
	var attrs []xmlb.Attr
	inlinePrefix := ""
	hasInline := false
	for _, a := range v.RawAttrs {
		switch {
		case a.Name.Space == "xmlns":
			if a.Value == rns && !hasInline {
				inlinePrefix, hasInline = a.Name.Local, true
				continue // StartElementInlineNS writes this declaration itself
			}
			attrs = append(attrs, xmlb.Attr{Name: "xmlns:" + a.Name.Local, Value: a.Value})
		case a.Name.Space == "" && a.Name.Local == "xmlns":
			attrs = append(attrs, xmlb.Attr{Name: "xmlns", Value: a.Value})
		default:
			attrs = append(attrs, xmlb.Attr{Namespace: a.Name.Space, Name: a.Name.Local, Value: a.Value})
		}
	}
	if hasInline && inlinePrefix != "" {
		if len(v.Raw) == 0 {
			b.EmptyElementInlineNS(rns, inlinePrefix, v.RawName.Local, attrs...)
			return
		}
		b.StartElementInlineNS(rns, inlinePrefix, v.RawName.Local, attrs...)
		b.WriteRaw(v.Raw)
		b.EndElementInlineNS(inlinePrefix, v.RawName.Local)
		b.ResetNamespaceDeclaration(rns)
		return
	}
	if len(v.Raw) == 0 {
		b.EmptyElement(rns, v.RawName.Local, attrs...)
		return
	}
	b.StartElement(rns, v.RawName.Local, attrs...)
	b.WriteRaw(v.Raw)
	b.EndElement(rns, v.RawName.Local)
}

// MarshalXML implements xml.Marshaler for BlipXML (encoding/xml path), keeping
// effect children in order. Raw-captured children are replayed as tokens.
func (v *BlipXML) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	if v.Embed != "" {
		start.Attr = append(start.Attr, xml.Attr{
			Name: xml.Name{Space: xmlb.NSOfficeDocumentRels, Local: "embed"}, Value: v.Embed})
	}
	if v.Link != "" {
		start.Attr = append(start.Attr, xml.Attr{
			Name: xml.Name{Space: xmlb.NSOfficeDocumentRels, Local: "link"}, Value: v.Link})
	}
	if v.Cstate != "" {
		start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "cstate"}, Value: v.Cstate})
	}
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	for _, eff := range v.Effects {
		if err := eff.marshalXML(e); err != nil {
			return err
		}
	}
	if v.ExtLst != nil {
		if err := e.EncodeElement(v.ExtLst, xml.StartElement{Name: xml.Name{Space: NsDrawingML, Local: "extLst"}}); err != nil {
			return err
		}
	}
	return e.EncodeToken(start.End())
}

func (v *BlipEffect) marshalXML(e *xml.Encoder) error {
	if name, value, ok := v.typedChild(); ok {
		return e.EncodeElement(value, xml.StartElement{Name: xml.Name{Space: NsDrawingML, Local: name}})
	}
	if v.RawName.Local == "" {
		return nil
	}
	elem := xml.StartElement{Name: v.RawName}
	for _, a := range v.RawAttrs {
		if a.Name.Space == "xmlns" || (a.Name.Space == "" && a.Name.Local == "xmlns") {
			continue
		}
		elem.Attr = append(elem.Attr, a)
	}
	if err := e.EncodeToken(elem); err != nil {
		return err
	}
	// Replay the captured inner XML as tokens: encoding/xml has no raw-write
	// API, so re-tokenize (prefixes bound outside the fragment cannot be
	// resolved and end the replay early rather than failing the marshal).
	if len(v.Raw) > 0 {
		sub := xml.NewDecoder(bytes.NewReader(v.Raw))
		for {
			tok, err := sub.Token()
			if err != nil {
				break
			}
			if err := e.EncodeToken(fixupRawToken(tok)); err != nil {
				return err
			}
		}
	}
	return e.EncodeToken(elem.End())
}

// fixupRawToken strips prefix-only namespace hints Go's decoder leaves on
// re-tokenized fragments so the encoder does not reject them.
func fixupRawToken(tok xml.Token) xml.Token {
	if se, ok := tok.(xml.StartElement); ok {
		attrs := se.Attr[:0]
		for _, a := range se.Attr {
			if a.Name.Space == "xmlns" || (a.Name.Space == "" && a.Name.Local == "xmlns") {
				continue
			}
			attrs = append(attrs, a)
		}
		se.Attr = attrs
		return se
	}
	return tok
}

// TileXML represents CT_TileInfoProperties (a:tile)
type TileXML struct {
	// Tx/Ty are pointers so explicit tx="0" ty="0" survive the round trip.
	Tx   *int64     `xml:"tx,attr,omitempty"`
	Ty   *int64     `xml:"ty,attr,omitempty"`
	Sx   Percentage `xml:"sx,attr,omitempty"`
	Sy   Percentage `xml:"sy,attr,omitempty"`
	Flip string     `xml:"flip,attr,omitempty"`
	Algn string     `xml:"algn,attr,omitempty"`
}

// StretchXML represents CT_StretchInfoProperties (a:stretch)
type StretchXML struct {
	FillRect *RelRect `xml:"http://schemas.openxmlformats.org/drawingml/2006/main fillRect,omitempty"`
}

// NoFillXML represents CT_NoFillProperties (a:noFill)
type NoFillXML struct{}

// GrpFill represents CT_GroupFillProperties (a:grpFill)
type GrpFill struct{}

// RelRect represents CT_RelativeRect. Its edges are ST_Percentage values:
// transitional producers write them as "n%" strings (e.g. fillToRect
// t="50%"), which must parse and re-emit verbatim.
type RelRect struct {
	L             Percentage      `xml:"l,attr,omitempty"`
	T             Percentage      `xml:"t,attr,omitempty"`
	R             Percentage      `xml:"r,attr,omitempty"`
	B             Percentage      `xml:"b,attr,omitempty"`
	CapturedAttrs []xmlb.RootAttr `xml:"-"` // verbatim source attrs; see common/xml.CaptureAttrs
}

// UnmarshalXML captures the element's verbatim attribute list (source
// attribute order and any unmodeled attributes) before decoding through the
// struct tags; the reflection marshaler replays it.
func (rr *RelRect) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	rr.CapturedAttrs = xmlb.CaptureAttrs(start.Attr)
	type alias RelRect
	return d.DecodeElement((*alias)(rr), &start)
}
