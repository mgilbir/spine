// This file provides DrawingML XML picture types from dml-main.xsd.

package dml

import (
	"encoding/xml"
	"reflect"
	"strings"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// Blip represents CT_Blip (a:blip) - image reference with effects
type Blip struct {
	Embed         string          `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships embed,attr,omitempty"`
	Link          string          `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships link,attr,omitempty"`
	Cstate        string          `xml:"cstate,attr,omitempty"` // email, screen, print, hqprint
	AlphaBiLevel  *AlphaBiLevel   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main alphaBiLevel,omitempty"`
	AlphaCeiling  *AlphaCeiling   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main alphaCeiling,omitempty"`
	AlphaFloor    *AlphaFloor     `xml:"http://schemas.openxmlformats.org/drawingml/2006/main alphaFloor,omitempty"`
	AlphaInv      *AlphaInv       `xml:"http://schemas.openxmlformats.org/drawingml/2006/main alphaInv,omitempty"`
	AlphaMod      *AlphaMod       `xml:"http://schemas.openxmlformats.org/drawingml/2006/main alphaMod,omitempty"`
	AlphaModFix   *AlphaModFix    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main alphaModFix,omitempty"`
	AlphaRepl     *AlphaRepl      `xml:"http://schemas.openxmlformats.org/drawingml/2006/main alphaRepl,omitempty"`
	BiLevel       *BiLevelXML     `xml:"http://schemas.openxmlformats.org/drawingml/2006/main biLevel,omitempty"`
	Blur          *BlurXML        `xml:"http://schemas.openxmlformats.org/drawingml/2006/main blur,omitempty"`
	ClrChange     *ClrChange      `xml:"http://schemas.openxmlformats.org/drawingml/2006/main clrChange,omitempty"`
	ClrRepl       *ClrRepl        `xml:"http://schemas.openxmlformats.org/drawingml/2006/main clrRepl,omitempty"`
	Duotone       *Duotone        `xml:"http://schemas.openxmlformats.org/drawingml/2006/main duotone,omitempty"`
	FillOverlay   *FillOverlayXML `xml:"http://schemas.openxmlformats.org/drawingml/2006/main fillOverlay,omitempty"`
	Grayscl       *GrayscaleXML   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main grayscl,omitempty"`
	Hsl           *HslXML         `xml:"http://schemas.openxmlformats.org/drawingml/2006/main hsl,omitempty"`
	Lum           *LumXML         `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lum,omitempty"`
	Tint          *TintEffectXML  `xml:"http://schemas.openxmlformats.org/drawingml/2006/main tint,omitempty"`
	ExtLst        *ExtLst         `xml:"http://schemas.openxmlformats.org/drawingml/2006/main extLst,omitempty"`
	CapturedAttrs []xmlb.RootAttr `xml:"-"` // verbatim source attrs; see common/xml.CaptureAttrs
	// CapturedChildren records the source child sequence: the effect children
	// form a repeated xs:choice whose order is significant (effects compose),
	// and grouped singleton fields alone would reorder them on save.
	CapturedChildren *xmlb.ChildCapture `xml:"-"`
}

// OrderedEffects returns the blip's effects in document order, as the
// ordered representation BlipXML keeps; effects set without a captured
// order follow in schema order.
//
// Children the typed fields cannot hold — an element the model does not know,
// or a repeat of an effect already held — sit in CapturedChildren.Raw. They
// take their place in the list as a BlipEffect with only RawName.Local set
// (the element's name without its prefix: the verbatim bytes stay in the
// capture, and Raw here is not filled), so a consumer that applies effects
// sees that something it cannot interpret sits between its neighbours.
// Captured comments and whitespace produce no entry.
func (bl *Blip) OrderedEffects() (effects []*BlipEffect) {
	if bl == nil {
		return nil
	}
	of := func(field int) *BlipEffect {
		switch v := reflect.ValueOf(bl).Elem().Field(field).Interface().(type) {
		case *AlphaBiLevel:
			if v != nil {
				return &BlipEffect{AlphaBiLevel: v}
			}
		case *AlphaCeiling:
			if v != nil {
				return &BlipEffect{AlphaCeiling: v}
			}
		case *AlphaFloor:
			if v != nil {
				return &BlipEffect{AlphaFloor: v}
			}
		case *AlphaInv:
			if v != nil {
				return &BlipEffect{AlphaInv: v}
			}
		case *AlphaMod:
			if v != nil {
				return &BlipEffect{AlphaMod: v}
			}
		case *AlphaModFix:
			if v != nil {
				return &BlipEffect{AlphaModFix: v}
			}
		case *AlphaRepl:
			if v != nil {
				return &BlipEffect{AlphaRepl: v}
			}
		case *BiLevelXML:
			if v != nil {
				return &BlipEffect{BiLevel: v}
			}
		case *BlurXML:
			if v != nil {
				return &BlipEffect{Blur: v}
			}
		case *ClrChange:
			if v != nil {
				return &BlipEffect{ClrChange: v}
			}
		case *ClrRepl:
			if v != nil {
				return &BlipEffect{ClrRepl: v}
			}
		case *Duotone:
			if v != nil {
				return &BlipEffect{Duotone: v}
			}
		case *FillOverlayXML:
			if v != nil {
				return &BlipEffect{FillOverlay: v}
			}
		case *GrayscaleXML:
			if v != nil {
				return &BlipEffect{Grayscl: v}
			}
		case *HslXML:
			if v != nil {
				return &BlipEffect{Hsl: v}
			}
		case *LumXML:
			if v != nil {
				return &BlipEffect{Lum: v}
			}
		case *TintEffectXML:
			if v != nil {
				return &BlipEffect{Tint: v}
			}
		}
		return nil
	}
	n := reflect.TypeOf(*bl).NumField()
	seen := make([]bool, n)
	if c := bl.CapturedChildren; c != nil {
		for _, ref := range c.Order {
			if ref.Field < 0 {
				if ref.Index >= 0 && ref.Index < len(c.Raw) {
					if local := rawElementLocalName(c.Raw[ref.Index]); local != "" {
						effects = append(effects, &BlipEffect{RawName: xml.Name{Local: local}})
					}
				}
				continue
			}
			if ref.Field >= n {
				continue
			}
			if e := of(ref.Field); e != nil && !seen[ref.Field] {
				effects = append(effects, e)
				seen[ref.Field] = true
			}
		}
	}
	for i := 0; i < n; i++ {
		if e := of(i); e != nil && !seen[i] {
			effects = append(effects, e)
		}
	}
	return effects
}

// UnmarshalXML captures the element's verbatim attribute list (source
// attribute order and any unmodeled attributes) and child sequence while
// decoding the children into the struct fields.
func (bl *Blip) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	bl.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	for _, attr := range start.Attr {
		switch {
		case attr.Name.Local == "embed" && attr.Name.Space == xmlb.NSOfficeDocumentRels:
			bl.Embed = attr.Value
		case attr.Name.Local == "link" && attr.Name.Space == xmlb.NSOfficeDocumentRels:
			bl.Link = attr.Value
		case attr.Name.Local == "cstate" && attr.Name.Space == "":
			bl.Cstate = attr.Value
		}
	}
	return xmlb.UnmarshalOrderedChildren(d, bl)
}

// BlipFill represents CT_BlipFillProperties (a:blipFill) - complete blip fill
type BlipFill struct {
	Dpi           *uint32         `xml:"dpi,attr,omitempty"`
	RotWithShape  *bool           `xml:"rotWithShape,attr,omitempty"`
	Blip          *Blip           `xml:"http://schemas.openxmlformats.org/drawingml/2006/main blip,omitempty"`
	SrcRect       *SrcRect        `xml:"http://schemas.openxmlformats.org/drawingml/2006/main srcRect,omitempty"`
	Tile          *Tile           `xml:"http://schemas.openxmlformats.org/drawingml/2006/main tile,omitempty"`
	Stretch       *Stretch        `xml:"http://schemas.openxmlformats.org/drawingml/2006/main stretch,omitempty"`
	CapturedAttrs []xmlb.RootAttr `xml:"-"` // verbatim source attrs; see xml_bool_capture.go
}

// --- Image Compression Settings ---

// ImageCompressionState values for Blip.Cstate:
// email - low quality suitable for email attachments
// screen - medium quality suitable for web pages
// print - high quality suitable for printing
// hqprint - highest quality for professional printing

// --- Cropping Types ---

// CropRect represents cropping rectangle (offsets from edges).
// NOTE: duplicates RelRect/SrcRect (CT_RelativeRect); consolidating the
// RelRect family is a deeper refactor, deliberately deferred.
type CropRect struct {
	L Percentage `xml:"l,attr,omitempty"` // left offset
	T Percentage `xml:"t,attr,omitempty"` // top offset
	R Percentage `xml:"r,attr,omitempty"` // right offset
	B Percentage `xml:"b,attr,omitempty"` // bottom offset
}

// rawElementLocalName returns the local name of the element a captured raw
// child starts with, or "" when the child is not an element (whitespace or
// text, a comment, a processing instruction).
func rawElementLocalName(raw []byte) string {
	if len(raw) < 2 || raw[0] != '<' || raw[1] == '!' || raw[1] == '?' || raw[1] == '/' {
		return ""
	}
	end := 1
	for end < len(raw) {
		if c := raw[end]; c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == '/' || c == '>' {
			break
		}
		end++
	}
	name := string(raw[1:end])
	if i := strings.LastIndexByte(name, ':'); i >= 0 {
		name = name[i+1:]
	}
	return name
}
