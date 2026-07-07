// Package dml provides DrawingML XML picture types from dml-main.xsd.
package dml

// Blip represents CT_Blip (a:blip) - image reference with effects
type Blip struct {
	Embed      string      `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships embed,attr,omitempty"`
	Link       string      `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships link,attr,omitempty"`
	Cstate     string      `xml:"cstate,attr,omitempty"` // email, screen, print, hqprint
	AlphaBiLevel *AlphaBiLevel `xml:"http://schemas.openxmlformats.org/drawingml/2006/main alphaBiLevel,omitempty"`
	AlphaCeiling *AlphaCeiling `xml:"http://schemas.openxmlformats.org/drawingml/2006/main alphaCeiling,omitempty"`
	AlphaFloor   *AlphaFloor   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main alphaFloor,omitempty"`
	AlphaInv     *AlphaInv     `xml:"http://schemas.openxmlformats.org/drawingml/2006/main alphaInv,omitempty"`
	AlphaMod     *AlphaMod     `xml:"http://schemas.openxmlformats.org/drawingml/2006/main alphaMod,omitempty"`
	AlphaModFix  *AlphaModFix  `xml:"http://schemas.openxmlformats.org/drawingml/2006/main alphaModFix,omitempty"`
	AlphaRepl    *AlphaRepl    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main alphaRepl,omitempty"`
	BiLevel      *BiLevelXML   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main biLevel,omitempty"`
	Blur         *BlurXML      `xml:"http://schemas.openxmlformats.org/drawingml/2006/main blur,omitempty"`
	ClrChange    *ClrChange    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main clrChange,omitempty"`
	ClrRepl      *ClrRepl      `xml:"http://schemas.openxmlformats.org/drawingml/2006/main clrRepl,omitempty"`
	Duotone      *Duotone      `xml:"http://schemas.openxmlformats.org/drawingml/2006/main duotone,omitempty"`
	FillOverlay  *FillOverlayXML `xml:"http://schemas.openxmlformats.org/drawingml/2006/main fillOverlay,omitempty"`
	Grayscl      *GrayscaleXML `xml:"http://schemas.openxmlformats.org/drawingml/2006/main grayscl,omitempty"`
	Hsl          *HslXML       `xml:"http://schemas.openxmlformats.org/drawingml/2006/main hsl,omitempty"`
	Lum          *LumXML       `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lum,omitempty"`
	Tint         *TintEffectXML `xml:"http://schemas.openxmlformats.org/drawingml/2006/main tint,omitempty"`
	ExtLst       *ExtLst       `xml:"http://schemas.openxmlformats.org/drawingml/2006/main extLst,omitempty"`
}

// BlipFill represents CT_BlipFillProperties (a:blipFill) - complete blip fill
type BlipFill struct {
	Dpi          *uint32  `xml:"dpi,attr,omitempty"`
	RotWithShape *bool    `xml:"rotWithShape,attr,omitempty"`
	Blip         *Blip    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main blip,omitempty"`
	SrcRect      *SrcRect `xml:"http://schemas.openxmlformats.org/drawingml/2006/main srcRect,omitempty"`
	Tile         *Tile    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main tile,omitempty"`
	Stretch      *Stretch `xml:"http://schemas.openxmlformats.org/drawingml/2006/main stretch,omitempty"`
}

// --- Image Compression Settings ---

// ImageCompressionState values for Blip.Cstate:
// email - low quality suitable for email attachments
// screen - medium quality suitable for web pages
// print - high quality suitable for printing
// hqprint - highest quality for professional printing

// --- Cropping Types ---

// CropRect represents cropping rectangle (offsets from edges)
type CropRect struct {
	L int32 `xml:"l,attr,omitempty"` // left offset
	T int32 `xml:"t,attr,omitempty"` // top offset
	R int32 `xml:"r,attr,omitempty"` // right offset
	B int32 `xml:"b,attr,omitempty"` // bottom offset
}

// --- Picture Rendering Hints ---

// PicRenderHints represents rendering hints for pictures
type PicRenderHints struct {
	PreferRelativeResize bool   `xml:"preferRelativeResize,attr,omitempty"`
	DisableLocking       bool   `xml:"noChangeArrowheads,attr,omitempty"`
	DisableAspectRatio   bool   `xml:"noChangeAspect,attr,omitempty"`
}

// --- Linked Picture Types ---

// LinkedPic represents a linked (not embedded) picture
type LinkedPic struct {
	Link string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships link,attr"`
}

// --- Picture Placeholder Types ---

// PicPlaceholder represents a picture placeholder
type PicPlaceholder struct {
	Type string `xml:"type,attr,omitempty"` // clipArt, media, etc.
	Idx  uint32 `xml:"idx,attr,omitempty"`
}
