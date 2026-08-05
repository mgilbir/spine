// Package vml provides a read-oriented, spec-coverage model of Vector Markup
// Language — the legacy drawing format of older Office documents — covering
// the v:, o:, w10: and x: namespaces of vml-main.xsd and its companions.
//
// # Scope
//
// This package is for *reading* VML: parsing a legacy drawing part, a Word
// w:pict, or an Excel comment/control shape into typed Go values you can
// inspect. It is deliberately not spine's VML writer. The VML this library
// produces is generated from templates in xlsx/comment_vml.go (comments, form
// controls, OLE pictures), and xlsx reads back the handful of fields it needs
// through its own narrow structs; no format package marshals these types.
//
// The types do marshal — the round-trip is how parse fidelity is proved —
// but three properties a writer would need are outside the model's contract,
// and each is a deliberate boundary rather than an oversight:
//
//   - Child order is normalized to struct-field (schema) order. VML content
//     models are xs:choice groups, so a source that wrote
//     v:fill, v:shadow, v:path, v:textbox re-emits as fill, shadow, textbox,
//     path. Rendering is unaffected; byte fidelity is impossible.
//   - Attribute and element coverage is what the schema documents, with no
//     generic capture convention. An attribute or child this package does not
//     model is dropped, not preserved — unlike common/dml, docx and xlsx,
//     whose CapturedAttrs / CapturedChildren conventions retain the unmodeled.
//   - w:txbxContent holds WordprocessingML, which this package cannot type
//     (common/vml must not depend on docx). It is kept as verbatim inner XML
//     whose prefix bindings live in the enclosing document, so marshal
//     re-declares the OOXML prefixes it recognizes in that content; a
//     producer prefix outside that set cannot be bound by a model that stores
//     the content opaquely.
//
// Within those boundaries the emitted XML is namespace-correct: every element
// carries its schema name and namespace, and unqualified (##local) textbox
// children stay unqualified.
package vml

import (
	"encoding/xml"
	"fmt"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// Namespace URIs of the VML family.
const (
	NSVml   = "urn:schemas-microsoft-com:vml"
	NSOffic = "urn:schemas-microsoft-com:office:office"
	NSWord  = "urn:schemas-microsoft-com:office:word"
	NSExcel = "urn:schemas-microsoft-com:office:excel"
	NSRels  = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
	NSWml   = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"
)

// --- Core Shape Types ---

// Group represents the <v:group> element - a container for shapes
type Group struct {
	XMLName     xml.Name `xml:"urn:schemas-microsoft-com:vml group"`
	ID          string   `xml:"id,attr,omitempty"`
	Style       string   `xml:"style,attr,omitempty"`
	CoordOrigin string   `xml:"coordorigin,attr,omitempty"`
	CoordSize   string   `xml:"coordsize,attr,omitempty"`
	WrapCoords  string   `xml:"wrapcoords,attr,omitempty"`
	// Child shapes
	Shape     []*Shape     `xml:"urn:schemas-microsoft-com:vml shape,omitempty"`
	Group     []*Group     `xml:"urn:schemas-microsoft-com:vml group,omitempty"`
	Shapetype []*Shapetype `xml:"urn:schemas-microsoft-com:vml shapetype,omitempty"`
	Rect      []*Rect      `xml:"urn:schemas-microsoft-com:vml rect,omitempty"`
	Oval      []*Oval      `xml:"urn:schemas-microsoft-com:vml oval,omitempty"`
	Line      []*Line      `xml:"urn:schemas-microsoft-com:vml line,omitempty"`
	Polyline  []*Polyline  `xml:"urn:schemas-microsoft-com:vml polyline,omitempty"`
	Curve     []*Curve     `xml:"urn:schemas-microsoft-com:vml curve,omitempty"`
	Arc       []*Arc       `xml:"urn:schemas-microsoft-com:vml arc,omitempty"`
	Image     []*ImageEl   `xml:"urn:schemas-microsoft-com:vml image,omitempty"`
	RoundRect []*RoundRect `xml:"urn:schemas-microsoft-com:vml roundrect,omitempty"`
	// A group is a shape: it carries the same formatting, text and
	// application-extension children as v:shape. Modeling only its shape
	// children discarded a group's o:lock, w10:wrap, w10:anchorlock,
	// x:ClientData, v:textbox and v:fill/stroke/shadow silently.
	Fill       *Fill       `xml:"urn:schemas-microsoft-com:vml fill,omitempty"`
	Stroke     *Stroke     `xml:"urn:schemas-microsoft-com:vml stroke,omitempty"`
	Shadow     *Shadow     `xml:"urn:schemas-microsoft-com:vml shadow,omitempty"`
	Textbox    *Textbox    `xml:"urn:schemas-microsoft-com:vml textbox,omitempty"`
	Lock       *Lock       `xml:"urn:schemas-microsoft-com:office:office lock,omitempty"`
	Wrap       *Wrap       `xml:"urn:schemas-microsoft-com:office:word wrap,omitempty"`
	AnchorLock *AnchorLock `xml:"urn:schemas-microsoft-com:office:word anchorlock,omitempty"`
	ClientData *ClientData `xml:"urn:schemas-microsoft-com:office:excel ClientData,omitempty"`
}

// Shape represents the <v:shape> element - a generic shape
type Shape struct {
	XMLName       xml.Name `xml:"urn:schemas-microsoft-com:vml shape"`
	ID            string   `xml:"id,attr,omitempty"`
	Type          string   `xml:"type,attr,omitempty"`
	Style         string   `xml:"style,attr,omitempty"`
	Filled        string   `xml:"filled,attr,omitempty"` // t, f
	FillColor     string   `xml:"fillcolor,attr,omitempty"`
	Stroked       string   `xml:"stroked,attr,omitempty"` // t, f
	StrokeColor   string   `xml:"strokecolor,attr,omitempty"`
	StrokeWeight  string   `xml:"strokeweight,attr,omitempty"`
	CoordOrigin   string   `xml:"coordorigin,attr,omitempty"`
	CoordSize     string   `xml:"coordsize,attr,omitempty"`
	Path          string   `xml:"path,attr,omitempty"`
	WrapCoords    string   `xml:"wrapcoords,attr,omitempty"`
	Adj           string   `xml:"adj,attr,omitempty"`
	Alt           string   `xml:"alt,attr,omitempty"`
	HRef          string   `xml:"href,attr,omitempty"`
	Target        string   `xml:"target,attr,omitempty"`
	Title         string   `xml:"title,attr,omitempty"`
	Opacity       string   `xml:"opacity,attr,omitempty"`
	ChromaKey     string   `xml:"chromakey,attr,omitempty"`
	Spt           int32    `xml:"spt,attr,omitempty"`
	ConnectorType string   `xml:"connectortype,attr,omitempty"`
	// Office-specific attributes
	OSpid          string `xml:"urn:schemas-microsoft-com:office:office spid,attr,omitempty"`
	OConnectorType string `xml:"urn:schemas-microsoft-com:office:office connectortype,attr,omitempty"`
	OInsetMode     string `xml:"urn:schemas-microsoft-com:office:office insetmode,attr,omitempty"`
	// o:gfxdata carries the base64 fallback picture Word writes for every
	// shape drawn in a modern client; dropping it loses the shape's rendering
	// for readers that cannot draw VML. o:bwmode and o:allowincell were
	// likewise unmodeled.
	OGfxData        string `xml:"urn:schemas-microsoft-com:office:office gfxdata,attr,omitempty"`
	OBwMode         string `xml:"urn:schemas-microsoft-com:office:office bwmode,attr,omitempty"`
	OBwPure         string `xml:"urn:schemas-microsoft-com:office:office bwpure,attr,omitempty"`
	OBwNormal       string `xml:"urn:schemas-microsoft-com:office:office bwnormal,attr,omitempty"`
	OAllowInCell    string `xml:"urn:schemas-microsoft-com:office:office allowincell,attr,omitempty"`
	OAllowOverlap   string `xml:"urn:schemas-microsoft-com:office:office allowoverlap,attr,omitempty"`
	OPreferRelative string `xml:"urn:schemas-microsoft-com:office:office preferrelative,attr,omitempty"`
	OOLE            string `xml:"urn:schemas-microsoft-com:office:office ole,attr,omitempty"`
	OButton         string `xml:"urn:schemas-microsoft-com:office:office button,attr,omitempty"`
	OSpt            string `xml:"urn:schemas-microsoft-com:office:office spt,attr,omitempty"`
	// Child elements
	Fill      *Fill      `xml:"urn:schemas-microsoft-com:vml fill,omitempty"`
	Stroke    *Stroke    `xml:"urn:schemas-microsoft-com:vml stroke,omitempty"`
	Shadow    *Shadow    `xml:"urn:schemas-microsoft-com:vml shadow,omitempty"`
	Textbox   *Textbox   `xml:"urn:schemas-microsoft-com:vml textbox,omitempty"`
	TextPath  *TextPath  `xml:"urn:schemas-microsoft-com:vml textpath,omitempty"`
	ImageData *ImageData `xml:"urn:schemas-microsoft-com:vml imagedata,omitempty"`
	Formulas  *Formulas  `xml:"urn:schemas-microsoft-com:vml formulas,omitempty"`
	Handles   *Handles   `xml:"urn:schemas-microsoft-com:vml handles,omitempty"`
	PathEl    *PathEl    `xml:"urn:schemas-microsoft-com:vml path,omitempty"`
	// Office elements
	Lock          *Lock          `xml:"urn:schemas-microsoft-com:office:office lock,omitempty"`
	Callout       *Callout       `xml:"urn:schemas-microsoft-com:office:office callout,omitempty"`
	Extrusion     *Extrusion     `xml:"urn:schemas-microsoft-com:office:office extrusion,omitempty"`
	SignatureLine *SignatureLine `xml:"urn:schemas-microsoft-com:office:office signatureline,omitempty"`
	Wrap          *Wrap          `xml:"urn:schemas-microsoft-com:office:word wrap,omitempty"`
	AnchorLock    *AnchorLock    `xml:"urn:schemas-microsoft-com:office:word anchorlock,omitempty"`
	ClientData    *ClientData    `xml:"urn:schemas-microsoft-com:office:excel ClientData,omitempty"`
}

// Shapetype represents the <v:shapetype> element - a reusable shape template
type Shapetype struct {
	XMLName         xml.Name `xml:"urn:schemas-microsoft-com:vml shapetype"`
	ID              string   `xml:"id,attr,omitempty"`
	CoordSize       string   `xml:"coordsize,attr,omitempty"`
	Spt             int32    `xml:"spt,attr,omitempty"`
	Adj             string   `xml:"adj,attr,omitempty"`
	Path            string   `xml:"path,attr,omitempty"`
	Filled          string   `xml:"filled,attr,omitempty"`
	Stroked         string   `xml:"stroked,attr,omitempty"`
	OPreferRelative string   `xml:"urn:schemas-microsoft-com:office:office preferrelative,attr,omitempty"`
	// Child elements
	Stroke   *Stroke   `xml:"urn:schemas-microsoft-com:vml stroke,omitempty"`
	Fill     *Fill     `xml:"urn:schemas-microsoft-com:vml fill,omitempty"`
	Formulas *Formulas `xml:"urn:schemas-microsoft-com:vml formulas,omitempty"`
	PathEl   *PathEl   `xml:"urn:schemas-microsoft-com:vml path,omitempty"`
	TextPath *TextPath `xml:"urn:schemas-microsoft-com:vml textpath,omitempty"`
	Handles  *Handles  `xml:"urn:schemas-microsoft-com:vml handles,omitempty"`
	Lock     *Lock     `xml:"urn:schemas-microsoft-com:office:office lock,omitempty"`
}

// Rect represents the <v:rect> element
type Rect struct {
	XMLName      xml.Name    `xml:"urn:schemas-microsoft-com:vml rect"`
	ID           string      `xml:"id,attr,omitempty"`
	Style        string      `xml:"style,attr,omitempty"`
	Filled       string      `xml:"filled,attr,omitempty"`
	FillColor    string      `xml:"fillcolor,attr,omitempty"`
	Stroked      string      `xml:"stroked,attr,omitempty"`
	StrokeColor  string      `xml:"strokecolor,attr,omitempty"`
	StrokeWeight string      `xml:"strokeweight,attr,omitempty"`
	OSpid        string      `xml:"urn:schemas-microsoft-com:office:office spid,attr,omitempty"`
	OInsetMode   string      `xml:"urn:schemas-microsoft-com:office:office insetmode,attr,omitempty"`
	Fill         *Fill       `xml:"urn:schemas-microsoft-com:vml fill,omitempty"`
	Stroke       *Stroke     `xml:"urn:schemas-microsoft-com:vml stroke,omitempty"`
	Shadow       *Shadow     `xml:"urn:schemas-microsoft-com:vml shadow,omitempty"`
	Textbox      *Textbox    `xml:"urn:schemas-microsoft-com:vml textbox,omitempty"`
	Lock         *Lock       `xml:"urn:schemas-microsoft-com:office:office lock,omitempty"`
	ClientData   *ClientData `xml:"urn:schemas-microsoft-com:office:excel ClientData,omitempty"`
}

// RoundRect represents the <v:roundrect> element
type RoundRect struct {
	XMLName     xml.Name `xml:"urn:schemas-microsoft-com:vml roundrect"`
	ID          string   `xml:"id,attr,omitempty"`
	Style       string   `xml:"style,attr,omitempty"`
	ArcSize     string   `xml:"arcsize,attr,omitempty"` // percentage, e.g. "10923f"
	Filled      string   `xml:"filled,attr,omitempty"`
	FillColor   string   `xml:"fillcolor,attr,omitempty"`
	Stroked     string   `xml:"stroked,attr,omitempty"`
	StrokeColor string   `xml:"strokecolor,attr,omitempty"`
	Fill        *Fill    `xml:"urn:schemas-microsoft-com:vml fill,omitempty"`
	Stroke      *Stroke  `xml:"urn:schemas-microsoft-com:vml stroke,omitempty"`
	Shadow      *Shadow  `xml:"urn:schemas-microsoft-com:vml shadow,omitempty"`
	Textbox     *Textbox `xml:"urn:schemas-microsoft-com:vml textbox,omitempty"`
}

// Oval represents the <v:oval> element
type Oval struct {
	XMLName     xml.Name `xml:"urn:schemas-microsoft-com:vml oval"`
	ID          string   `xml:"id,attr,omitempty"`
	Style       string   `xml:"style,attr,omitempty"`
	Filled      string   `xml:"filled,attr,omitempty"`
	FillColor   string   `xml:"fillcolor,attr,omitempty"`
	Stroked     string   `xml:"stroked,attr,omitempty"`
	StrokeColor string   `xml:"strokecolor,attr,omitempty"`
	Fill        *Fill    `xml:"urn:schemas-microsoft-com:vml fill,omitempty"`
	Stroke      *Stroke  `xml:"urn:schemas-microsoft-com:vml stroke,omitempty"`
	Shadow      *Shadow  `xml:"urn:schemas-microsoft-com:vml shadow,omitempty"`
	Textbox     *Textbox `xml:"urn:schemas-microsoft-com:vml textbox,omitempty"`
}

// Line represents the <v:line> element
type Line struct {
	XMLName      xml.Name `xml:"urn:schemas-microsoft-com:vml line"`
	ID           string   `xml:"id,attr,omitempty"`
	Style        string   `xml:"style,attr,omitempty"`
	From         string   `xml:"from,attr,omitempty"` // x,y
	To           string   `xml:"to,attr,omitempty"`   // x,y
	StrokeColor  string   `xml:"strokecolor,attr,omitempty"`
	StrokeWeight string   `xml:"strokeweight,attr,omitempty"`
	Stroke       *Stroke  `xml:"urn:schemas-microsoft-com:vml stroke,omitempty"`
	Shadow       *Shadow  `xml:"urn:schemas-microsoft-com:vml shadow,omitempty"`
}

// Polyline represents the <v:polyline> element
type Polyline struct {
	XMLName     xml.Name `xml:"urn:schemas-microsoft-com:vml polyline"`
	ID          string   `xml:"id,attr,omitempty"`
	Style       string   `xml:"style,attr,omitempty"`
	Points      string   `xml:"points,attr,omitempty"` // space-separated x,y pairs
	Filled      string   `xml:"filled,attr,omitempty"`
	StrokeColor string   `xml:"strokecolor,attr,omitempty"`
	Fill        *Fill    `xml:"urn:schemas-microsoft-com:vml fill,omitempty"`
	Stroke      *Stroke  `xml:"urn:schemas-microsoft-com:vml stroke,omitempty"`
}

// Curve represents the <v:curve> element - a bezier curve
type Curve struct {
	XMLName     xml.Name `xml:"urn:schemas-microsoft-com:vml curve"`
	ID          string   `xml:"id,attr,omitempty"`
	Style       string   `xml:"style,attr,omitempty"`
	From        string   `xml:"from,attr,omitempty"`
	Control1    string   `xml:"control1,attr,omitempty"`
	Control2    string   `xml:"control2,attr,omitempty"`
	To          string   `xml:"to,attr,omitempty"`
	StrokeColor string   `xml:"strokecolor,attr,omitempty"`
	Stroke      *Stroke  `xml:"urn:schemas-microsoft-com:vml stroke,omitempty"`
}

// Arc represents the <v:arc> element
type Arc struct {
	XMLName    xml.Name `xml:"urn:schemas-microsoft-com:vml arc"`
	ID         string   `xml:"id,attr,omitempty"`
	Style      string   `xml:"style,attr,omitempty"`
	StartAngle string   `xml:"startAngle,attr,omitempty"`
	EndAngle   string   `xml:"endAngle,attr,omitempty"`
	Filled     string   `xml:"filled,attr,omitempty"`
	FillColor  string   `xml:"fillcolor,attr,omitempty"`
	Fill       *Fill    `xml:"urn:schemas-microsoft-com:vml fill,omitempty"`
	Stroke     *Stroke  `xml:"urn:schemas-microsoft-com:vml stroke,omitempty"`
}

// ImageEl represents the <v:image> element
type ImageEl struct {
	XMLName    xml.Name `xml:"urn:schemas-microsoft-com:vml image"`
	ID         string   `xml:"id,attr,omitempty"`
	Style      string   `xml:"style,attr,omitempty"`
	Src        string   `xml:"src,attr,omitempty"`
	CropTop    string   `xml:"croptop,attr,omitempty"`
	CropBottom string   `xml:"cropbottom,attr,omitempty"`
	CropLeft   string   `xml:"cropleft,attr,omitempty"`
	CropRight  string   `xml:"cropright,attr,omitempty"`
}

// --- Fill, Stroke, Shadow ---

// Fill represents the <v:fill> element
type Fill struct {
	XMLName       xml.Name `xml:"urn:schemas-microsoft-com:vml fill"`
	ID            string   `xml:"id,attr,omitempty"`
	Type          string   `xml:"type,attr,omitempty"` // solid, gradient, gradientRadial, tile, pattern, frame
	On            string   `xml:"on,attr,omitempty"`   // t, f
	Color         string   `xml:"color,attr,omitempty"`
	Color2        string   `xml:"color2,attr,omitempty"`
	Opacity       string   `xml:"opacity,attr,omitempty"` // 0.0-1.0 or percentage
	Opacity2      string   `xml:"opacity2,attr,omitempty"`
	Src           string   `xml:"src,attr,omitempty"` // image source
	Size          string   `xml:"size,attr,omitempty"`
	Origin        string   `xml:"origin,attr,omitempty"` // for patterns
	Position      string   `xml:"position,attr,omitempty"`
	Aspect        string   `xml:"aspect,attr,omitempty"`     // ignore, atleast, atmost
	AlignShape    string   `xml:"alignshape,attr,omitempty"` // t, f
	Focus         string   `xml:"focus,attr,omitempty"`      // gradient focus: 0-100%
	FocusSize     string   `xml:"focussize,attr,omitempty"`
	FocusPosition string   `xml:"focusposition,attr,omitempty"`
	Method        string   `xml:"method,attr,omitempty"` // none, linear, sigma, any
	Angle         string   `xml:"angle,attr,omitempty"`  // gradient angle
	Colors        string   `xml:"colors,attr,omitempty"` // gradient stop colors
	Rotate        string   `xml:"rotate,attr,omitempty"` // t, f
	// Relationship attributes
	RID               string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr,omitempty"`
	OTitle            string `xml:"urn:schemas-microsoft-com:office:office title,attr,omitempty"`
	ODetectMouseClick string `xml:"urn:schemas-microsoft-com:office:office detectmouseclick,attr,omitempty"`
	RecolorTarget     string `xml:"recolortarget,attr,omitempty"`
}

// Stroke represents the <v:stroke> element
type Stroke struct {
	XMLName          xml.Name `xml:"urn:schemas-microsoft-com:vml stroke"`
	ID               string   `xml:"id,attr,omitempty"`
	On               string   `xml:"on,attr,omitempty"` // t, f
	Weight           string   `xml:"weight,attr,omitempty"`
	Color            string   `xml:"color,attr,omitempty"`
	Color2           string   `xml:"color2,attr,omitempty"`
	Opacity          string   `xml:"opacity,attr,omitempty"`
	LineStyle        string   `xml:"linestyle,attr,omitempty"` // single, thinThin, thinThick, thickThin, thickBetweenThin
	MiterLimit       string   `xml:"miterlimit,attr,omitempty"`
	JoinStyle        string   `xml:"joinstyle,attr,omitempty"` // round, bevel, miter
	EndCap           string   `xml:"endcap,attr,omitempty"`    // flat, square, round
	DashStyle        string   `xml:"dashstyle,attr,omitempty"` // solid, shortdash, shortdot, shortdashdot, shortdashdotdot, dot, dash, longdash, dashdot, longdashdot, longdashdotdot
	FillType         string   `xml:"filltype,attr,omitempty"`
	Src              string   `xml:"src,attr,omitempty"`
	ImageAlignShape  string   `xml:"imagealignshape,attr,omitempty"`
	ImageSize        string   `xml:"imagesize,attr,omitempty"`
	StartArrow       string   `xml:"startarrow,attr,omitempty"`       // none, block, classic, diamond, oval, open
	StartArrowWidth  string   `xml:"startarrowwidth,attr,omitempty"`  // narrow, medium, wide
	StartArrowLength string   `xml:"startarrowlength,attr,omitempty"` // short, medium, long
	EndArrow         string   `xml:"endarrow,attr,omitempty"`
	EndArrowWidth    string   `xml:"endarrowwidth,attr,omitempty"`
	EndArrowLength   string   `xml:"endarrowlength,attr,omitempty"`
	InsetPen         string   `xml:"insetpen,attr,omitempty"` // t, f
	RID              string   `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr,omitempty"`
}

// Shadow represents the <v:shadow> element
type Shadow struct {
	XMLName  xml.Name `xml:"urn:schemas-microsoft-com:vml shadow"`
	ID       string   `xml:"id,attr,omitempty"`
	On       string   `xml:"on,attr,omitempty"`   // t, f
	Type     string   `xml:"type,attr,omitempty"` // single, double, emboss, perspective
	Color    string   `xml:"color,attr,omitempty"`
	Color2   string   `xml:"color2,attr,omitempty"`
	Opacity  string   `xml:"opacity,attr,omitempty"`
	Offset   string   `xml:"offset,attr,omitempty"` // x,y
	Offset2  string   `xml:"offset2,attr,omitempty"`
	Origin   string   `xml:"origin,attr,omitempty"`
	Matrix   string   `xml:"matrix,attr,omitempty"`
	Obscured string   `xml:"obscured,attr,omitempty"` // t, f
}

// --- Text Elements ---

// Textbox represents the <v:textbox> element.
// Per XSD: choice of w:txbxContent or local-namespace elements.
type Textbox struct {
	XMLName     xml.Name     `xml:"urn:schemas-microsoft-com:vml textbox"`
	ID          string       `xml:"id,attr,omitempty"`
	Style       string       `xml:"style,attr,omitempty"`
	Inset       string       `xml:"inset,attr,omitempty"` // margins: "left,top,right,bottom"
	SingleClick string       `xml:"urn:schemas-microsoft-com:office:office singleclick,attr,omitempty"`
	InsetMode   string       `xml:"insetmode,attr,omitempty"` // auto, custom
	TxbxContent *TxbxContent `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main txbxContent,omitempty"`
	// LocalContent preserves the local-namespace child elements (xsd:any
	// namespace="##local" processContents="skip") in document order. A single
	// RawContent kept only the last child and had no marshal path, so a textbox
	// with multiple local children lost all but one and re-emitted none.
	LocalContent []*textboxChild `xml:"-"`
}

// textboxChild is one local-namespace child of a v:textbox, captured verbatim
// so it survives a round trip in order.
type textboxChild struct {
	Name  xml.Name
	Attrs []xml.Attr
	Inner []byte
}

// marshalXML re-emits the captured child element and its verbatim inner content.
//
// A ##local child is unqualified by definition, and the enclosing v:textbox
// declares the VML namespace as the default (that is how the schema's own
// examples are written), so re-emitting the child bare would silently move it
// into the VML namespace. An explicit xmlns="" undeclares the default for the
// child's subtree, which is what "no namespace" means in XML.
func (c *textboxChild) marshalXML(e *xml.Encoder) error {
	start := xml.StartElement{Name: c.Name}
	if c.Name.Space == "" {
		start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "xmlns"}})
	}
	for _, a := range c.Attrs {
		if a.Name.Space == "xmlns" || a.Name.Local == "xmlns" {
			continue
		}
		start.Attr = append(start.Attr, a)
	}
	aux := struct {
		Inner []byte `xml:",innerxml"`
	}{c.Inner}
	return e.EncodeElement(aux, start)
}

// TxbxContent represents w:txbxContent (CT_TxbxContent): the
// WordprocessingML body content (paragraphs, tables) of a textbox.
//
// common/vml cannot type WML — it must not depend on docx — so the content is
// kept as the verbatim inner XML of the source. Its prefix bindings live in
// the enclosing document, which is a real limitation of an opaque capture:
// see MarshalXML.
type TxbxContent struct {
	RawContent []byte `xml:",innerxml"`
}

// txbxContentPrefixes are the namespace prefixes that appear inside a
// w:txbxContent body in documents Office and the common third-party writers
// produce, with the URI each is conventionally bound to. Marshal declares the
// ones the captured content actually uses.
//
// A fixed table is the honest limit of an ",innerxml" capture: the bytes
// record the prefixes the producer wrote but not what they were bound to,
// because the declarations sit on an ancestor the capture never saw. A
// prefix outside this table therefore cannot be bound, which is one of the
// reasons this package is documented as a reader (see the package comment).
var txbxContentPrefixes = map[string]string{
	"w":    NSWml,
	"w14":  "http://schemas.microsoft.com/office/word/2010/wordml",
	"w15":  "http://schemas.microsoft.com/office/word/2012/wordml",
	"w16":  "http://schemas.microsoft.com/office/word/2018/wordml",
	"wne":  "http://schemas.microsoft.com/office/word/2006/wordml",
	"wp":   "http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing",
	"wps":  "http://schemas.microsoft.com/office/word/2010/wordprocessingShape",
	"wpg":  "http://schemas.microsoft.com/office/word/2010/wordprocessingGroup",
	"wpc":  "http://schemas.microsoft.com/office/word/2010/wordprocessingCanvas",
	"wpi":  "http://schemas.microsoft.com/office/word/2010/wordprocessingInk",
	"a":    "http://schemas.openxmlformats.org/drawingml/2006/main",
	"pic":  "http://schemas.openxmlformats.org/drawingml/2006/picture",
	"a14":  "http://schemas.microsoft.com/office/drawing/2010/main",
	"a16":  "http://schemas.microsoft.com/office/drawing/2014/main",
	"asvg": "http://schemas.microsoft.com/office/drawing/2016/SVG/main",
	"m":    "http://schemas.openxmlformats.org/officeDocument/2006/math",
	"mc":   "http://schemas.openxmlformats.org/markup-compatibility/2006",
	"r":    NSRels,
	"v":    NSVml,
	"o":    NSOffic,
	"w10":  NSWord,
	"x":    NSExcel,
}

// MarshalXML implements xml.Marshaler.
//
// Go's encoder writes a namespaced element name as a *default* declaration
// (xmlns="…"), which binds nothing for the w:-prefixed names inside the
// captured content: the output parsed as "unbound prefix" in every strict
// reader while Go's own lenient decoder read it back happily. Emit the
// element under an explicit w: prefix and declare every recognized prefix the
// captured bytes actually reference, so the subtree carries its own bindings.
func (c *TxbxContent) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = xml.Name{Local: "w:txbxContent"}
	start.Attr = []xml.Attr{{Name: xml.Name{Local: "xmlns:w"}, Value: NSWml}}
	for _, prefix := range usedPrefixes(c.RawContent) {
		uri, ok := txbxContentPrefixes[prefix]
		if !ok || prefix == "w" {
			continue
		}
		start.Attr = append(start.Attr, xml.Attr{
			Name:  xml.Name{Local: "xmlns:" + prefix},
			Value: uri,
		})
	}
	aux := struct {
		Inner []byte `xml:",innerxml"`
	}{c.RawContent}
	return e.EncodeElement(aux, start)
}

// usedPrefixes returns, in first-appearance order, the namespace prefixes the
// raw XML uses on element and attribute names. It lexes rather than parses:
// the content's prefixes are unbound in isolation, so a decoder would report
// them as namespace URIs and lose the distinction from real ones.
func usedPrefixes(raw []byte) []string {
	var out []string
	seen := map[string]bool{}
	inTag := false
	for i := 0; i < len(raw); i++ {
		switch raw[i] {
		case '<':
			inTag = true
			continue
		case '>':
			inTag = false
			continue
		}
		if !inTag {
			continue
		}
		// A prefix is an NCName immediately followed by ':' and another
		// NCName start, preceded by '<', '/' or whitespace (element name) —
		// attribute names are separated by whitespace too.
		if raw[i] != ':' {
			continue
		}
		j := i
		for j > 0 && isNameByte(raw[j-1]) {
			j--
		}
		if j == i || (j > 0 && raw[j-1] != '<' && raw[j-1] != '/' && !isSpaceByte(raw[j-1])) {
			continue
		}
		if i+1 >= len(raw) || !isNameByte(raw[i+1]) {
			continue
		}
		p := string(raw[j:i])
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	return out
}

func isNameByte(c byte) bool {
	return c == '_' || c == '-' || c == '.' ||
		(c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

func isSpaceByte(c byte) bool {
	return c == ' ' || c == '\t' || c == '\r' || c == '\n'
}

func (tb *Textbox) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		switch attr.Name.Local {
		case "id":
			tb.ID = attr.Value
		case "style":
			tb.Style = attr.Value
		case "inset":
			tb.Inset = attr.Value
		case "singleclick":
			if attr.Name.Space == "urn:schemas-microsoft-com:office:office" {
				tb.SingleClick = attr.Value
			}
		case "insetmode":
			tb.InsetMode = attr.Value
		}
	}

	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "txbxContent" && t.Name.Space == "http://schemas.openxmlformats.org/wordprocessingml/2006/main" {
				tb.TxbxContent = &TxbxContent{}
				if err := d.DecodeElement(tb.TxbxContent, &t); err != nil {
					return err
				}
			} else {
				// Local-namespace element: preserve each child verbatim, in
				// order (previously a single RawContent kept only the last).
				//
				// The name has to be one this model can write back. Go's
				// decoder is looser than the Name production and reported the
				// local name "0" for <A:0/>; capturing it verbatim meant
				// marshaling an element literally named 0, which does not
				// reparse. Preserving a child faithfully is the whole point of
				// this branch, so a name that cannot be preserved is refused
				// rather than mangled or silently dropped.
				if !xmlb.IsName(t.Name.Local) {
					return fmt.Errorf("vml: v:textbox child %q is not a valid XML name", t.Name.Local)
				}
				var inner struct {
					Content []byte `xml:",innerxml"`
				}
				child := &textboxChild{Name: t.Name, Attrs: append([]xml.Attr(nil), t.Attr...)}
				if err := d.DecodeElement(&inner, &t); err != nil {
					return err
				}
				child.Inner = inner.Content
				tb.LocalContent = append(tb.LocalContent, child)
			}
		case xml.EndElement:
			return nil
		}
	}
}

// MarshalXML implements xml.Marshaler, re-emitting the textbox attributes, an
// optional w:txbxContent, and every captured local-namespace child in order.
// Without it the reflection encoder dropped LocalContent (xml:"-") entirely.
func (tb *Textbox) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	// A type implementing xml.Marshaler never gets its XMLName consulted: at
	// top level encoding/xml derives the start element from the Go type name,
	// so without this the element was written as <Textbox> in no namespace.
	start.Name = xml.Name{Space: NSVml, Local: "textbox"}
	start.Attr = nil
	if tb.ID != "" {
		start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "id"}, Value: tb.ID})
	}
	if tb.Style != "" {
		start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "style"}, Value: tb.Style})
	}
	if tb.Inset != "" {
		start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "inset"}, Value: tb.Inset})
	}
	if tb.SingleClick != "" {
		start.Attr = append(start.Attr, xml.Attr{
			Name:  xml.Name{Space: "urn:schemas-microsoft-com:office:office", Local: "singleclick"},
			Value: tb.SingleClick,
		})
	}
	if tb.InsetMode != "" {
		start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "insetmode"}, Value: tb.InsetMode})
	}
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	if tb.TxbxContent != nil {
		if err := e.EncodeElement(tb.TxbxContent, xml.StartElement{
			Name: xml.Name{Space: "http://schemas.openxmlformats.org/wordprocessingml/2006/main", Local: "txbxContent"},
		}); err != nil {
			return err
		}
	}
	for _, c := range tb.LocalContent {
		if err := c.marshalXML(e); err != nil {
			return err
		}
	}
	return e.EncodeToken(start.End())
}

// TextPath represents the <v:textpath> element - text along a path (WordArt)
type TextPath struct {
	XMLName  xml.Name `xml:"urn:schemas-microsoft-com:vml textpath"`
	ID       string   `xml:"id,attr,omitempty"`
	On       string   `xml:"on,attr,omitempty"`
	FitShape string   `xml:"fitshape,attr,omitempty"`
	FitPath  string   `xml:"fitpath,attr,omitempty"`
	Trim     string   `xml:"trim,attr,omitempty"`
	XScale   string   `xml:"xscale,attr,omitempty"`
	String   string   `xml:"string,attr,omitempty"`
	Style    string   `xml:"style,attr,omitempty"`
}

// --- Image ---

// ImageData represents the <v:imagedata> element
type ImageData struct {
	XMLName       xml.Name `xml:"urn:schemas-microsoft-com:vml imagedata"`
	ID            string   `xml:"id,attr,omitempty"`
	Src           string   `xml:"src,attr,omitempty"`
	CropTop       string   `xml:"croptop,attr,omitempty"`
	CropBottom    string   `xml:"cropbottom,attr,omitempty"`
	CropLeft      string   `xml:"cropleft,attr,omitempty"`
	CropRight     string   `xml:"cropright,attr,omitempty"`
	Gain          string   `xml:"gain,attr,omitempty"`
	BlackLevel    string   `xml:"blacklevel,attr,omitempty"`
	Gamma         string   `xml:"gamma,attr,omitempty"`
	GrayScale     string   `xml:"grayscale,attr,omitempty"` // t, f
	BiLevel       string   `xml:"bilevel,attr,omitempty"`   // t, f
	ChromaKey     string   `xml:"chromakey,attr,omitempty"`
	EmbossColor   string   `xml:"embosscolor,attr,omitempty"`
	RecolorTarget string   `xml:"recolortarget,attr,omitempty"`
	OTitle        string   `xml:"urn:schemas-microsoft-com:office:office title,attr,omitempty"`
	RID           string   `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr,omitempty"`
	ORelID        string   `xml:"urn:schemas-microsoft-com:office:office relid,attr,omitempty"`
}

// --- Path and Formulas ---

// PathEl represents the <v:path> element (named PathEl to avoid conflict with shape path attribute)
type PathEl struct {
	XMLName         xml.Name `xml:"urn:schemas-microsoft-com:vml path"`
	ID              string   `xml:"id,attr,omitempty"`
	V               string   `xml:"v,attr,omitempty"` // path data using VML path commands
	Limo            string   `xml:"limo,attr,omitempty"`
	TextBoxRect     string   `xml:"textboxrect,attr,omitempty"`
	FillOk          string   `xml:"fillok,attr,omitempty"`                                              // t, f
	StrokeOk        string   `xml:"strokeok,attr,omitempty"`                                            // t, f
	ShadowOk        string   `xml:"shadowok,attr,omitempty"`                                            // t, f
	ArrowOk         string   `xml:"arrowok,attr,omitempty"`                                             // t, f
	GradientShapeOk string   `xml:"gradientshapeok,attr,omitempty"`                                     // t, f
	TextPathOk      string   `xml:"textpathok,attr,omitempty"`                                          // t, f
	InsetPenOk      string   `xml:"insetpenok,attr,omitempty"`                                          // t, f
	OConnectType    string   `xml:"urn:schemas-microsoft-com:office:office connecttype,attr,omitempty"` // none, rect, segments, custom
	OConnectLocs    string   `xml:"urn:schemas-microsoft-com:office:office connectlocs,attr,omitempty"`
	OConnectAngles  string   `xml:"urn:schemas-microsoft-com:office:office connectangles,attr,omitempty"`
	OExtrusionOk    string   `xml:"urn:schemas-microsoft-com:office:office extrusionok,attr,omitempty"`
}

// Formulas represents the <v:formulas> element
type Formulas struct {
	XMLName xml.Name   `xml:"urn:schemas-microsoft-com:vml formulas"`
	F       []*Formula `xml:"urn:schemas-microsoft-com:vml f,omitempty"`
}

// Formula represents the <v:f> element
type Formula struct {
	XMLName xml.Name `xml:"urn:schemas-microsoft-com:vml f"`
	Eqn     string   `xml:"eqn,attr,omitempty"` // formula expression
}

// Handles represents the <v:handles> element
type Handles struct {
	XMLName xml.Name  `xml:"urn:schemas-microsoft-com:vml handles"`
	H       []*Handle `xml:"urn:schemas-microsoft-com:vml h,omitempty"`
}

// Handle represents the <v:h> element - adjustment handle
type Handle struct {
	XMLName     xml.Name `xml:"urn:schemas-microsoft-com:vml h"`
	Position    string   `xml:"position,attr,omitempty"`
	Polar       string   `xml:"polar,attr,omitempty"`
	Map         string   `xml:"map,attr,omitempty"`
	InvX        string   `xml:"invx,attr,omitempty"`   // t, f
	InvY        string   `xml:"invy,attr,omitempty"`   // t, f
	Switch      string   `xml:"switch,attr,omitempty"` // t, f
	XRange      string   `xml:"xrange,attr,omitempty"`
	YRange      string   `xml:"yrange,attr,omitempty"`
	RadiusRange string   `xml:"radiusrange,attr,omitempty"`
}

// --- Office VML Extensions (o:) ---

// Lock represents the <o:lock> element - shape locking
type Lock struct {
	XMLName       xml.Name `xml:"urn:schemas-microsoft-com:office:office lock"`
	Ext           string   `xml:"urn:schemas-microsoft-com:vml ext,attr,omitempty"` // "edit"
	AspectRatio   string   `xml:"aspectratio,attr,omitempty"`                       // t, f
	Text          string   `xml:"text,attr,omitempty"`                              // t, f
	Rotation      string   `xml:"rotation,attr,omitempty"`                          // t, f
	Cropping      string   `xml:"cropping,attr,omitempty"`                          // t, f
	Verticies     string   `xml:"verticies,attr,omitempty"`                         // t, f
	AdjustHandles string   `xml:"adjusthandles,attr,omitempty"`                     // t, f
	Grouping      string   `xml:"grouping,attr,omitempty"`                          // t, f
	Ungrouping    string   `xml:"ungrouping,attr,omitempty"`                        // t, f
	Selection     string   `xml:"selection,attr,omitempty"`                         // t, f
	Position      string   `xml:"position,attr,omitempty"`                          // t, f
	ShapeType     string   `xml:"shapetype,attr,omitempty"`                         // t, f
}

// Callout represents the <o:callout> element
type Callout struct {
	XMLName         xml.Name `xml:"urn:schemas-microsoft-com:office:office callout"`
	Ext             string   `xml:"urn:schemas-microsoft-com:vml ext,attr,omitempty"`
	On              string   `xml:"on,attr,omitempty"` // t, f
	Type            string   `xml:"type,attr,omitempty"`
	Gap             string   `xml:"gap,attr,omitempty"`
	Angle           string   `xml:"angle,attr,omitempty"` // any, 30, 45, 60, 90, auto
	Drop            string   `xml:"drop,attr,omitempty"`
	DropAuto        string   `xml:"dropauto,attr,omitempty"` // t, f
	Distance        string   `xml:"distance,attr,omitempty"`
	LengthSpecified string   `xml:"lengthspecified,attr,omitempty"` // t, f
	Length          string   `xml:"length,attr,omitempty"`
	AccentBar       string   `xml:"accentbar,attr,omitempty"`  // t, f
	TextBorder      string   `xml:"textborder,attr,omitempty"` // t, f
	MinusX          string   `xml:"minusx,attr,omitempty"`
	MinusY          string   `xml:"minusy,attr,omitempty"`
}

// Extrusion represents the <o:extrusion> element - 3D extrusion
type Extrusion struct {
	XMLName            xml.Name `xml:"urn:schemas-microsoft-com:office:office extrusion"`
	Ext                string   `xml:"urn:schemas-microsoft-com:vml ext,attr,omitempty"`
	On                 string   `xml:"on,attr,omitempty"`     // t, f
	Type               string   `xml:"type,attr,omitempty"`   // parallel, perspective
	Render             string   `xml:"render,attr,omitempty"` // solid, wireFrame, boundingCube
	ViewPointOrigin    string   `xml:"viewpointorigin,attr,omitempty"`
	ViewPoint          string   `xml:"viewpoint,attr,omitempty"`
	Plane              string   `xml:"plane,attr,omitempty"` // XY, ZX, YZ
	SkewAngle          string   `xml:"skewangle,attr,omitempty"`
	SkewAmt            string   `xml:"skewamt,attr,omitempty"`
	ForeDepth          string   `xml:"foredepth,attr,omitempty"`
	BackDepth          string   `xml:"backdepth,attr,omitempty"`
	Orientation        string   `xml:"orientation,attr,omitempty"`
	OrientationAngle   string   `xml:"orientationangle,attr,omitempty"`
	LockRotationCenter string   `xml:"lockrotationcenter,attr,omitempty"` // t, f
	AutoRotationCenter string   `xml:"autorotationcenter,attr,omitempty"` // t, f
	RotationCenter     string   `xml:"rotationcenter,attr,omitempty"`
	RotationAngle      string   `xml:"rotationangle,attr,omitempty"`
	Color              string   `xml:"color,attr,omitempty"`
	Shininess          string   `xml:"shininess,attr,omitempty"`
	Specularity        string   `xml:"specularity,attr,omitempty"`
	Diffusity          string   `xml:"diffusity,attr,omitempty"`
	Metal              string   `xml:"metal,attr,omitempty"` // t, f
	Edge               string   `xml:"edge,attr,omitempty"`
	Facet              string   `xml:"facet,attr,omitempty"`
	LightFace          string   `xml:"lightface,attr,omitempty"` // t, f
	Brightness         string   `xml:"brightness,attr,omitempty"`
	LightPosition      string   `xml:"lightposition,attr,omitempty"`
	LightLevel         string   `xml:"lightlevel,attr,omitempty"`
	LightHarsh         string   `xml:"lightharsh,attr,omitempty"` // t, f
	LightPosition2     string   `xml:"lightposition2,attr,omitempty"`
	LightLevel2        string   `xml:"lightlevel2,attr,omitempty"`
	LightHarsh2        string   `xml:"lightharsh2,attr,omitempty"` // t, f
}

// SignatureLine represents the <o:signatureline> element
type SignatureLine struct {
	XMLName                xml.Name `xml:"urn:schemas-microsoft-com:office:office signatureline"`
	Ext                    string   `xml:"urn:schemas-microsoft-com:vml ext,attr,omitempty"`
	ID                     string   `xml:"id,attr,omitempty"`
	ProvID                 string   `xml:"provid,attr,omitempty"`
	SigProvURL             string   `xml:"sigprovurl,attr,omitempty"`
	IsSignatureLine        string   `xml:"issignatureline,attr,omitempty"`        // t, f
	SigningInstructionsSet string   `xml:"signinginstructionsset,attr,omitempty"` // t, f
	AllowComments          string   `xml:"allowcomments,attr,omitempty"`          // t, f
	ShowSignDate           string   `xml:"showsigndate,attr,omitempty"`           // t, f
	ShowSignTitle          string   `xml:"showsigntitle,attr,omitempty"`          // t, f
	SuggestedSigner        string   `xml:"suggestedsigner,attr,omitempty"`
	SuggestedSigner2       string   `xml:"suggestedsigner2,attr,omitempty"`
	SuggestedSignerEmail   string   `xml:"suggestedsigneremail,attr,omitempty"`
	SignInstructions       string   `xml:"signinginstructions,attr,omitempty"`
	AddlXml                string   `xml:"addlxml,attr,omitempty"`
	SigProvDetails         string   `xml:"sigprovdetails,attr,omitempty"`
}

// --- Word VML Extensions (w10:) ---

// Wrap represents the <w10:wrap> element - text wrapping for Word
type Wrap struct {
	XMLName xml.Name `xml:"urn:schemas-microsoft-com:office:word wrap"`
	Type    string   `xml:"type,attr,omitempty"`    // none, topAndBottom, square, tight, through
	Side    string   `xml:"side,attr,omitempty"`    // both, left, right, largest
	AnchorX string   `xml:"anchorx,attr,omitempty"` // margin, page, text, char
	AnchorY string   `xml:"anchory,attr,omitempty"` // margin, page, text, line
}

// AnchorLock represents the <w10:anchorlock> element
type AnchorLock struct {
	XMLName xml.Name `xml:"urn:schemas-microsoft-com:office:word anchorlock"`
}

// BorderTop represents the <w10:bordertop> element
type BorderTop struct {
	XMLName xml.Name `xml:"urn:schemas-microsoft-com:office:word bordertop"`
	Type    string   `xml:"type,attr,omitempty"`
	Width   int32    `xml:"width,attr,omitempty"`
	Color   string   `xml:"color,attr,omitempty"`
}

// BorderBottom represents the <w10:borderbottom> element
type BorderBottom struct {
	XMLName xml.Name `xml:"urn:schemas-microsoft-com:office:word borderbottom"`
	Type    string   `xml:"type,attr,omitempty"`
	Width   int32    `xml:"width,attr,omitempty"`
	Color   string   `xml:"color,attr,omitempty"`
}

// BorderLeft represents the <w10:borderleft> element
type BorderLeft struct {
	XMLName xml.Name `xml:"urn:schemas-microsoft-com:office:word borderleft"`
	Type    string   `xml:"type,attr,omitempty"`
	Width   int32    `xml:"width,attr,omitempty"`
	Color   string   `xml:"color,attr,omitempty"`
}

// BorderRight represents the <w10:borderright> element
type BorderRight struct {
	XMLName xml.Name `xml:"urn:schemas-microsoft-com:office:word borderright"`
	Type    string   `xml:"type,attr,omitempty"`
	Width   int32    `xml:"width,attr,omitempty"`
	Color   string   `xml:"color,attr,omitempty"`
}

// --- Office VML Extensions (o:) ---

// Background represents the <v:background> element - document background
type Background struct {
	XMLName   xml.Name `xml:"urn:schemas-microsoft-com:vml background"`
	ID        string   `xml:"id,attr,omitempty"`
	FillColor string   `xml:"fillcolor,attr,omitempty"`
	Fill      *Fill    `xml:"urn:schemas-microsoft-com:vml fill,omitempty"`
}

// ShapeLayout represents the <o:shapelayout> element - shape layout
type ShapeLayout struct {
	Ext string `xml:"-"` // v:ext attr (cross-namespace)
}

// UnmarshalXML implements custom unmarshaling for ShapeLayout.
func (s *ShapeLayout) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		if attr.Name.Local == "ext" {
			s.Ext = attr.Value
		}
	}
	return d.Skip()
}

// MarshalXML implements custom marshaling for ShapeLayout to output v:ext.
func (s *ShapeLayout) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = xml.Name{Space: NSOffic, Local: "shapelayout"}
	if s.Ext != "" {
		start.Attr = append(start.Attr, xml.Attr{
			Name:  xml.Name{Space: "urn:schemas-microsoft-com:vml", Local: "ext"},
			Value: s.Ext,
		})
	}
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	return e.EncodeToken(start.End())
}

// ShapeDefaults represents the <o:shapedefaults> element - default shape properties
type ShapeDefaults struct {
	Ext       string `xml:"-"` // v:ext attr (cross-namespace)
	SpidMax   string `xml:"spidmax,attr,omitempty"`
	FillColor string `xml:"fillcolor,attr,omitempty"`
}

// UnmarshalXML implements custom unmarshaling for ShapeDefaults.
func (s *ShapeDefaults) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		switch attr.Name.Local {
		case "ext":
			s.Ext = attr.Value
		case "spidmax":
			s.SpidMax = attr.Value
		case "fillcolor":
			s.FillColor = attr.Value
		}
	}
	return d.Skip()
}

// MarshalXML implements custom marshaling for ShapeDefaults to output v:ext.
func (s *ShapeDefaults) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = xml.Name{Space: NSOffic, Local: "shapedefaults"}
	if s.Ext != "" {
		start.Attr = append(start.Attr, xml.Attr{
			Name:  xml.Name{Space: "urn:schemas-microsoft-com:vml", Local: "ext"},
			Value: s.Ext,
		})
	}
	if s.SpidMax != "" {
		start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "spidmax"}, Value: s.SpidMax})
	}
	if s.FillColor != "" {
		start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "fillcolor"}, Value: s.FillColor})
	}
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	return e.EncodeToken(start.End())
}

// ColorMru represents the <o:colormru> element - most recently used colors
type ColorMru struct {
	XMLName xml.Name `xml:"urn:schemas-microsoft-com:office:office colormru"`
	Ext     string   `xml:"urn:schemas-microsoft-com:vml ext,attr,omitempty"`
	Colors  string   `xml:"colors,attr,omitempty"`
}

// IdMap represents the <o:idmap> element - shape ID block mapping
type IdMap struct {
	XMLName xml.Name `xml:"urn:schemas-microsoft-com:office:office idmap"`
	Ext     string   `xml:"urn:schemas-microsoft-com:vml ext,attr,omitempty"`
	Data    string   `xml:"data,attr,omitempty"`
}

// RelationTable represents the <o:relationtable> element - shape relationship table
type RelationTable struct {
	XMLName xml.Name `xml:"urn:schemas-microsoft-com:office:office relationtable"`
	Ext     string   `xml:"urn:schemas-microsoft-com:vml ext,attr,omitempty"`
	Rel     []Rel    `xml:"urn:schemas-microsoft-com:office:office rel,omitempty"`
}

// Rel represents the <o:rel> element - shape relationship
type Rel struct {
	XMLName xml.Name `xml:"urn:schemas-microsoft-com:office:office rel"`
	Ext     string   `xml:"urn:schemas-microsoft-com:vml ext,attr,omitempty"`
	IdSrc   string   `xml:"idsrc,attr,omitempty"`
	IdDest  string   `xml:"iddest,attr,omitempty"`
	IdCntr  string   `xml:"idcntr,attr,omitempty"`
}

// OLEObject represents the <o:OLEObject> element - embedded OLE object
type OLEObject struct {
	Type       string `xml:"-"`
	ProgID     string `xml:"-"`
	ShapeID    string `xml:"-"`
	DrawAspect string `xml:"-"`
	ObjectID   string `xml:"-"`
	RID        string `xml:"-"` // r:id attr
}

// UnmarshalXML implements custom unmarshaling for OLEObject.
func (o *OLEObject) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		switch attr.Name.Local {
		case "Type":
			o.Type = attr.Value
		case "ProgID":
			o.ProgID = attr.Value
		case "ShapeID":
			o.ShapeID = attr.Value
		case "DrawAspect":
			o.DrawAspect = attr.Value
		case "ObjectID":
			o.ObjectID = attr.Value
		case "id":
			o.RID = attr.Value
		}
	}
	return d.Skip()
}

// MarshalXML implements custom marshaling for OLEObject to output all attributes including r:id.
func (o *OLEObject) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Name = xml.Name{Space: NSOffic, Local: "OLEObject"}
	for _, pair := range [][2]string{
		{"Type", o.Type},
		{"ProgID", o.ProgID},
		{"ShapeID", o.ShapeID},
		{"DrawAspect", o.DrawAspect},
		{"ObjectID", o.ObjectID},
	} {
		if pair[1] != "" {
			start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: pair[0]}, Value: pair[1]})
		}
	}
	if o.RID != "" {
		start.Attr = append(start.Attr, xml.Attr{
			Name:  xml.Name{Space: "http://schemas.openxmlformats.org/officeDocument/2006/relationships", Local: "id"},
			Value: o.RID,
		})
	}
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	return e.EncodeToken(start.End())
}

// Diagram represents the <o:diagram> element - VML diagram
type Diagram struct {
	XMLName    xml.Name `xml:"urn:schemas-microsoft-com:office:office diagram"`
	Autoformat string   `xml:"autoformat,attr,omitempty"`
	Reverse    string   `xml:"reverse,attr,omitempty"`
	AutoLayout string   `xml:"autolayout,attr,omitempty"`
	Dgmstyle   string   `xml:"dgmstyle,attr,omitempty"`
}

// Ink represents the <o:ink> element - ink data
type Ink struct {
	XMLName    xml.Name `xml:"urn:schemas-microsoft-com:office:office ink"`
	I          string   `xml:"i,attr,omitempty"`
	Annotation string   `xml:"annotation,attr,omitempty"`
}

// --- Excel VML Extensions (x:) ---

// ClientData represents <x:ClientData> (CT_ClientData in vml-excel.xsd):
// the Excel-specific data of a legacy comment, form control, OLE object or
// picture shape.
//
// Every child is a pointer. CT_ClientData declares its children as bare
// xsd:string / xsd:integer with minOccurs="0", and Excel writes most of them
// as *presence flags* with no content — <x:Visible/> is how an always-shown
// comment is marked, and <x:MoveWithCells/>, <x:NoThreeD/>, <x:VScroll/> are
// written the same way. Under string+omitempty such a child unmarshaled to ""
// and was then indistinguishable from absent, so it was dropped on marshal
// and its meaning lost. A pointer separates the three states the schema has:
// nil is absent, a pointer to "" is a present flag, and a pointer to text is
// a valued child (e.g. AutoFill "False"). The whole child set is converted in
// one pass and pinned by a table-driven test, because a partial conversion is
// exactly how this defect survived its first fix.
//
// Fields are in schema-sequence order, which is also the marshal order.
type ClientData struct {
	XMLName xml.Name `xml:"urn:schemas-microsoft-com:office:excel ClientData"`
	// ObjectType selects the meaning of the children: Note, Drop, Check,
	// Button, GBox, Spin, List, Radio, Scroll, Edit, Label, Dialog, Rect,
	// Shape, Group, Pict, Movie.
	ObjectType    string  `xml:"ObjectType,attr,omitempty"`
	MoveWithCells *string `xml:"urn:schemas-microsoft-com:office:excel MoveWithCells,omitempty"`
	SizeWithCells *string `xml:"urn:schemas-microsoft-com:office:excel SizeWithCells,omitempty"`
	Anchor        *string `xml:"urn:schemas-microsoft-com:office:excel Anchor,omitempty"`
	Locked        *string `xml:"urn:schemas-microsoft-com:office:excel Locked,omitempty"`
	DefaultSize   *string `xml:"urn:schemas-microsoft-com:office:excel DefaultSize,omitempty"`
	PrintObject   *string `xml:"urn:schemas-microsoft-com:office:excel PrintObject,omitempty"`
	Disabled      *string `xml:"urn:schemas-microsoft-com:office:excel Disabled,omitempty"`
	AutoFill      *string `xml:"urn:schemas-microsoft-com:office:excel AutoFill,omitempty"`
	AutoLine      *string `xml:"urn:schemas-microsoft-com:office:excel AutoLine,omitempty"`
	AutoPict      *string `xml:"urn:schemas-microsoft-com:office:excel AutoPict,omitempty"`
	FmlaMacro     *string `xml:"urn:schemas-microsoft-com:office:excel FmlaMacro,omitempty"`
	TextHAlign    *string `xml:"urn:schemas-microsoft-com:office:excel TextHAlign,omitempty"`
	TextVAlign    *string `xml:"urn:schemas-microsoft-com:office:excel TextVAlign,omitempty"`
	LockText      *string `xml:"urn:schemas-microsoft-com:office:excel LockText,omitempty"`
	JustLastX     *string `xml:"urn:schemas-microsoft-com:office:excel JustLastX,omitempty"`
	SecretEdit    *string `xml:"urn:schemas-microsoft-com:office:excel SecretEdit,omitempty"`
	Default       *string `xml:"urn:schemas-microsoft-com:office:excel Default,omitempty"`
	Help          *string `xml:"urn:schemas-microsoft-com:office:excel Help,omitempty"`
	Cancel        *string `xml:"urn:schemas-microsoft-com:office:excel Cancel,omitempty"`
	Dismiss       *string `xml:"urn:schemas-microsoft-com:office:excel Dismiss,omitempty"`
	Accel         *int32  `xml:"urn:schemas-microsoft-com:office:excel Accel,omitempty"`
	Accel2        *int32  `xml:"urn:schemas-microsoft-com:office:excel Accel2,omitempty"`
	Row           *int32  `xml:"urn:schemas-microsoft-com:office:excel Row,omitempty"`
	Column        *int32  `xml:"urn:schemas-microsoft-com:office:excel Column,omitempty"`
	Visible       *string `xml:"urn:schemas-microsoft-com:office:excel Visible,omitempty"`
	RowHidden     *string `xml:"urn:schemas-microsoft-com:office:excel RowHidden,omitempty"`
	ColHidden     *string `xml:"urn:schemas-microsoft-com:office:excel ColHidden,omitempty"`
	// VTEdit is declared xsd:integer, but the spec's own example for
	// 19.4.2.67 writes <x:VTEdit>True</x:VTEdit>. A reader that hard-fails
	// on the specification's example is useless on real files, so the
	// lexical form is kept.
	VTEdit         *string `xml:"urn:schemas-microsoft-com:office:excel VTEdit,omitempty"`
	MultiLine      *string `xml:"urn:schemas-microsoft-com:office:excel MultiLine,omitempty"`
	VScroll        *string `xml:"urn:schemas-microsoft-com:office:excel VScroll,omitempty"`
	ValidIds       *string `xml:"urn:schemas-microsoft-com:office:excel ValidIds,omitempty"`
	FmlaRange      *string `xml:"urn:schemas-microsoft-com:office:excel FmlaRange,omitempty"`
	WidthMin       *int32  `xml:"urn:schemas-microsoft-com:office:excel WidthMin,omitempty"`
	Sel            *int32  `xml:"urn:schemas-microsoft-com:office:excel Sel,omitempty"`
	NoThreeD2      *string `xml:"urn:schemas-microsoft-com:office:excel NoThreeD2,omitempty"`
	SelType        *string `xml:"urn:schemas-microsoft-com:office:excel SelType,omitempty"`
	MultiSel       *string `xml:"urn:schemas-microsoft-com:office:excel MultiSel,omitempty"`
	LCT            *string `xml:"urn:schemas-microsoft-com:office:excel LCT,omitempty"`
	ListItem       *string `xml:"urn:schemas-microsoft-com:office:excel ListItem,omitempty"`
	DropStyle      *string `xml:"urn:schemas-microsoft-com:office:excel DropStyle,omitempty"`
	Colored        *string `xml:"urn:schemas-microsoft-com:office:excel Colored,omitempty"`
	DropLines      *int32  `xml:"urn:schemas-microsoft-com:office:excel DropLines,omitempty"`
	Checked        *int32  `xml:"urn:schemas-microsoft-com:office:excel Checked,omitempty"`
	FmlaLink       *string `xml:"urn:schemas-microsoft-com:office:excel FmlaLink,omitempty"`
	FmlaPict       *string `xml:"urn:schemas-microsoft-com:office:excel FmlaPict,omitempty"`
	NoThreeD       *string `xml:"urn:schemas-microsoft-com:office:excel NoThreeD,omitempty"`
	FirstButton    *string `xml:"urn:schemas-microsoft-com:office:excel FirstButton,omitempty"`
	FmlaGroup      *string `xml:"urn:schemas-microsoft-com:office:excel FmlaGroup,omitempty"`
	Val            *int32  `xml:"urn:schemas-microsoft-com:office:excel Val,omitempty"`
	Min            *int32  `xml:"urn:schemas-microsoft-com:office:excel Min,omitempty"`
	Max            *int32  `xml:"urn:schemas-microsoft-com:office:excel Max,omitempty"`
	Inc            *int32  `xml:"urn:schemas-microsoft-com:office:excel Inc,omitempty"`
	Page           *int32  `xml:"urn:schemas-microsoft-com:office:excel Page,omitempty"`
	Dx             *int32  `xml:"urn:schemas-microsoft-com:office:excel Dx,omitempty"`
	MapOCX         *string `xml:"urn:schemas-microsoft-com:office:excel MapOCX,omitempty"`
	CF             *string `xml:"urn:schemas-microsoft-com:office:excel CF,omitempty"`
	Camera         *string `xml:"urn:schemas-microsoft-com:office:excel Camera,omitempty"`
	RecalcAlways   *string `xml:"urn:schemas-microsoft-com:office:excel RecalcAlways,omitempty"`
	AutoScale      *string `xml:"urn:schemas-microsoft-com:office:excel AutoScale,omitempty"`
	DDE            *string `xml:"urn:schemas-microsoft-com:office:excel DDE,omitempty"`
	UIObj          *string `xml:"urn:schemas-microsoft-com:office:excel UIObj,omitempty"`
	ScriptText     *string `xml:"urn:schemas-microsoft-com:office:excel ScriptText,omitempty"`
	ScriptExtended *string `xml:"urn:schemas-microsoft-com:office:excel ScriptExtended,omitempty"`
	ScriptLanguage *uint32 `xml:"urn:schemas-microsoft-com:office:excel ScriptLanguage,omitempty"`
	ScriptLocation *uint32 `xml:"urn:schemas-microsoft-com:office:excel ScriptLocation,omitempty"`
	FmlaTxbx       *string `xml:"urn:schemas-microsoft-com:office:excel FmlaTxbx,omitempty"`
}
