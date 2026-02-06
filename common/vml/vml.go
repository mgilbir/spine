// Package vml provides Vector Markup Language types from vml-main.xsd.
// VML is the legacy drawing format used in older Office documents.
// These types implement the v: namespace elements.
package vml

// --- Core Shape Types ---

// Group represents the <v:group> element - a container for shapes
type Group struct {
	ID          string `xml:"id,attr,omitempty"`
	Style       string `xml:"style,attr,omitempty"`
	CoordOrigin string `xml:"coordorigin,attr,omitempty"`
	CoordSize   string `xml:"coordsize,attr,omitempty"`
	WrapCoords  string `xml:"wrapcoords,attr,omitempty"`
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
}

// Shape represents the <v:shape> element - a generic shape
type Shape struct {
	ID           string `xml:"id,attr,omitempty"`
	Type         string `xml:"type,attr,omitempty"`
	Style        string `xml:"style,attr,omitempty"`
	Filled       string `xml:"filled,attr,omitempty"`       // t, f
	FillColor    string `xml:"fillcolor,attr,omitempty"`
	Stroked      string `xml:"stroked,attr,omitempty"`      // t, f
	StrokeColor  string `xml:"strokecolor,attr,omitempty"`
	StrokeWeight string `xml:"strokeweight,attr,omitempty"`
	CoordOrigin  string `xml:"coordorigin,attr,omitempty"`
	CoordSize    string `xml:"coordsize,attr,omitempty"`
	Path         string `xml:"path,attr,omitempty"`
	WrapCoords   string `xml:"wrapcoords,attr,omitempty"`
	Adj          string `xml:"adj,attr,omitempty"`
	Alt          string `xml:"alt,attr,omitempty"`
	HRef         string `xml:"href,attr,omitempty"`
	Target       string `xml:"target,attr,omitempty"`
	Title        string `xml:"title,attr,omitempty"`
	Opacity      string `xml:"opacity,attr,omitempty"`
	ChromaKey    string `xml:"chromakey,attr,omitempty"`
	Spt          int32  `xml:"spt,attr,omitempty"`
	ConnectorType string `xml:"connectortype,attr,omitempty"`
	// Office-specific attributes
	OSpid       string `xml:"urn:schemas-microsoft-com:office:office spid,attr,omitempty"`
	OConnectorType string `xml:"urn:schemas-microsoft-com:office:office connectortype,attr,omitempty"`
	OInsetMode  string `xml:"urn:schemas-microsoft-com:office:office insetmode,attr,omitempty"`
	// Child elements
	Fill       *Fill       `xml:"urn:schemas-microsoft-com:vml fill,omitempty"`
	Stroke     *Stroke     `xml:"urn:schemas-microsoft-com:vml stroke,omitempty"`
	Shadow     *Shadow     `xml:"urn:schemas-microsoft-com:vml shadow,omitempty"`
	Textbox    *Textbox    `xml:"urn:schemas-microsoft-com:vml textbox,omitempty"`
	TextPath   *TextPath   `xml:"urn:schemas-microsoft-com:vml textpath,omitempty"`
	ImageData  *ImageData  `xml:"urn:schemas-microsoft-com:vml imagedata,omitempty"`
	Formulas   *Formulas   `xml:"urn:schemas-microsoft-com:vml formulas,omitempty"`
	Handles    *Handles    `xml:"urn:schemas-microsoft-com:vml handles,omitempty"`
	PathEl     *PathEl     `xml:"urn:schemas-microsoft-com:vml path,omitempty"`
	// Office elements
	Lock       *Lock       `xml:"urn:schemas-microsoft-com:office:office lock,omitempty"`
	Callout    *Callout    `xml:"urn:schemas-microsoft-com:office:office callout,omitempty"`
	Extrusion  *Extrusion  `xml:"urn:schemas-microsoft-com:office:office extrusion,omitempty"`
	SignatureLine *SignatureLine `xml:"urn:schemas-microsoft-com:office:office signatureline,omitempty"`
	Wrap       *Wrap       `xml:"urn:schemas-microsoft-com:office:word wrap,omitempty"`
	AnchorLock *AnchorLock `xml:"urn:schemas-microsoft-com:office:word anchorlock,omitempty"`
	ClientData *ClientData `xml:"urn:schemas-microsoft-com:office:excel ClientData,omitempty"`
}

// Shapetype represents the <v:shapetype> element - a reusable shape template
type Shapetype struct {
	ID           string `xml:"id,attr,omitempty"`
	CoordSize    string `xml:"coordsize,attr,omitempty"`
	Spt          int32  `xml:"spt,attr,omitempty"`
	Adj          string `xml:"adj,attr,omitempty"`
	Path         string `xml:"path,attr,omitempty"`
	Filled       string `xml:"filled,attr,omitempty"`
	Stroked      string `xml:"stroked,attr,omitempty"`
	OPreferRelative string `xml:"urn:schemas-microsoft-com:office:office preferrelative,attr,omitempty"`
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
	ID          string `xml:"id,attr,omitempty"`
	Style       string `xml:"style,attr,omitempty"`
	Filled      string `xml:"filled,attr,omitempty"`
	FillColor   string `xml:"fillcolor,attr,omitempty"`
	Stroked     string `xml:"stroked,attr,omitempty"`
	StrokeColor string `xml:"strokecolor,attr,omitempty"`
	StrokeWeight string `xml:"strokeweight,attr,omitempty"`
	OSpid       string `xml:"urn:schemas-microsoft-com:office:office spid,attr,omitempty"`
	OInsetMode  string `xml:"urn:schemas-microsoft-com:office:office insetmode,attr,omitempty"`
	Fill        *Fill     `xml:"urn:schemas-microsoft-com:vml fill,omitempty"`
	Stroke      *Stroke   `xml:"urn:schemas-microsoft-com:vml stroke,omitempty"`
	Shadow      *Shadow   `xml:"urn:schemas-microsoft-com:vml shadow,omitempty"`
	Textbox     *Textbox  `xml:"urn:schemas-microsoft-com:vml textbox,omitempty"`
	Lock        *Lock     `xml:"urn:schemas-microsoft-com:office:office lock,omitempty"`
	ClientData  *ClientData `xml:"urn:schemas-microsoft-com:office:excel ClientData,omitempty"`
}

// RoundRect represents the <v:roundrect> element
type RoundRect struct {
	ID          string `xml:"id,attr,omitempty"`
	Style       string `xml:"style,attr,omitempty"`
	ArcSize     string `xml:"arcsize,attr,omitempty"` // percentage, e.g. "10923f"
	Filled      string `xml:"filled,attr,omitempty"`
	FillColor   string `xml:"fillcolor,attr,omitempty"`
	Stroked     string `xml:"stroked,attr,omitempty"`
	StrokeColor string `xml:"strokecolor,attr,omitempty"`
	Fill        *Fill    `xml:"urn:schemas-microsoft-com:vml fill,omitempty"`
	Stroke      *Stroke  `xml:"urn:schemas-microsoft-com:vml stroke,omitempty"`
	Shadow      *Shadow  `xml:"urn:schemas-microsoft-com:vml shadow,omitempty"`
	Textbox     *Textbox `xml:"urn:schemas-microsoft-com:vml textbox,omitempty"`
}

// Oval represents the <v:oval> element
type Oval struct {
	ID          string `xml:"id,attr,omitempty"`
	Style       string `xml:"style,attr,omitempty"`
	Filled      string `xml:"filled,attr,omitempty"`
	FillColor   string `xml:"fillcolor,attr,omitempty"`
	Stroked     string `xml:"stroked,attr,omitempty"`
	StrokeColor string `xml:"strokecolor,attr,omitempty"`
	Fill        *Fill    `xml:"urn:schemas-microsoft-com:vml fill,omitempty"`
	Stroke      *Stroke  `xml:"urn:schemas-microsoft-com:vml stroke,omitempty"`
	Shadow      *Shadow  `xml:"urn:schemas-microsoft-com:vml shadow,omitempty"`
	Textbox     *Textbox `xml:"urn:schemas-microsoft-com:vml textbox,omitempty"`
}

// Line represents the <v:line> element
type Line struct {
	ID          string `xml:"id,attr,omitempty"`
	Style       string `xml:"style,attr,omitempty"`
	From        string `xml:"from,attr,omitempty"` // x,y
	To          string `xml:"to,attr,omitempty"`   // x,y
	StrokeColor string `xml:"strokecolor,attr,omitempty"`
	StrokeWeight string `xml:"strokeweight,attr,omitempty"`
	Stroke      *Stroke `xml:"urn:schemas-microsoft-com:vml stroke,omitempty"`
	Shadow      *Shadow `xml:"urn:schemas-microsoft-com:vml shadow,omitempty"`
}

// Polyline represents the <v:polyline> element
type Polyline struct {
	ID          string `xml:"id,attr,omitempty"`
	Style       string `xml:"style,attr,omitempty"`
	Points      string `xml:"points,attr,omitempty"` // space-separated x,y pairs
	Filled      string `xml:"filled,attr,omitempty"`
	StrokeColor string `xml:"strokecolor,attr,omitempty"`
	Fill        *Fill   `xml:"urn:schemas-microsoft-com:vml fill,omitempty"`
	Stroke      *Stroke `xml:"urn:schemas-microsoft-com:vml stroke,omitempty"`
}

// Curve represents the <v:curve> element - a bezier curve
type Curve struct {
	ID          string `xml:"id,attr,omitempty"`
	Style       string `xml:"style,attr,omitempty"`
	From        string `xml:"from,attr,omitempty"`
	Control1    string `xml:"control1,attr,omitempty"`
	Control2    string `xml:"control2,attr,omitempty"`
	To          string `xml:"to,attr,omitempty"`
	StrokeColor string `xml:"strokecolor,attr,omitempty"`
	Stroke      *Stroke `xml:"urn:schemas-microsoft-com:vml stroke,omitempty"`
}

// Arc represents the <v:arc> element
type Arc struct {
	ID          string `xml:"id,attr,omitempty"`
	Style       string `xml:"style,attr,omitempty"`
	StartAngle  string `xml:"startAngle,attr,omitempty"`
	EndAngle    string `xml:"endAngle,attr,omitempty"`
	Filled      string `xml:"filled,attr,omitempty"`
	FillColor   string `xml:"fillcolor,attr,omitempty"`
	Fill        *Fill   `xml:"urn:schemas-microsoft-com:vml fill,omitempty"`
	Stroke      *Stroke `xml:"urn:schemas-microsoft-com:vml stroke,omitempty"`
}

// ImageEl represents the <v:image> element
type ImageEl struct {
	ID        string `xml:"id,attr,omitempty"`
	Style     string `xml:"style,attr,omitempty"`
	Src       string `xml:"src,attr,omitempty"`
	CropTop   string `xml:"croptop,attr,omitempty"`
	CropBottom string `xml:"cropbottom,attr,omitempty"`
	CropLeft  string `xml:"cropleft,attr,omitempty"`
	CropRight string `xml:"cropright,attr,omitempty"`
}

// --- Fill, Stroke, Shadow ---

// Fill represents the <v:fill> element
type Fill struct {
	ID          string `xml:"id,attr,omitempty"`
	Type        string `xml:"type,attr,omitempty"`        // solid, gradient, gradientRadial, tile, pattern, frame
	On          string `xml:"on,attr,omitempty"`           // t, f
	Color       string `xml:"color,attr,omitempty"`
	Color2      string `xml:"color2,attr,omitempty"`
	Opacity     string `xml:"opacity,attr,omitempty"`      // 0.0-1.0 or percentage
	Opacity2    string `xml:"opacity2,attr,omitempty"`
	Src         string `xml:"src,attr,omitempty"`           // image source
	Size        string `xml:"size,attr,omitempty"`
	Origin      string `xml:"origin,attr,omitempty"`       // for patterns
	Position    string `xml:"position,attr,omitempty"`
	Aspect      string `xml:"aspect,attr,omitempty"`       // ignore, atleast, atmost
	AlignShape  string `xml:"alignshape,attr,omitempty"`   // t, f
	Focus       string `xml:"focus,attr,omitempty"`        // gradient focus: 0-100%
	FocusSize   string `xml:"focussize,attr,omitempty"`
	FocusPosition string `xml:"focusposition,attr,omitempty"`
	Method      string `xml:"method,attr,omitempty"`       // none, linear, sigma, any
	Angle       string `xml:"angle,attr,omitempty"`        // gradient angle
	Colors      string `xml:"colors,attr,omitempty"`       // gradient stop colors
	Rotate      string `xml:"rotate,attr,omitempty"`       // t, f
	// Relationship attributes
	RID         string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr,omitempty"`
	OTitle      string `xml:"urn:schemas-microsoft-com:office:office title,attr,omitempty"`
	ODetectMouseClick string `xml:"urn:schemas-microsoft-com:office:office detectmouseclick,attr,omitempty"`
	RecolorTarget string `xml:"recolortarget,attr,omitempty"`
}

// Stroke represents the <v:stroke> element
type Stroke struct {
	ID           string `xml:"id,attr,omitempty"`
	On           string `xml:"on,attr,omitempty"`           // t, f
	Weight       string `xml:"weight,attr,omitempty"`
	Color        string `xml:"color,attr,omitempty"`
	Color2       string `xml:"color2,attr,omitempty"`
	Opacity      string `xml:"opacity,attr,omitempty"`
	LineStyle    string `xml:"linestyle,attr,omitempty"`     // single, thinThin, thinThick, thickThin, thickBetweenThin
	MiterLimit   string `xml:"miterlimit,attr,omitempty"`
	JoinStyle    string `xml:"joinstyle,attr,omitempty"`     // round, bevel, miter
	EndCap       string `xml:"endcap,attr,omitempty"`        // flat, square, round
	DashStyle    string `xml:"dashstyle,attr,omitempty"`     // solid, shortdash, shortdot, shortdashdot, shortdashdotdot, dot, dash, longdash, dashdot, longdashdot, longdashdotdot
	FillType     string `xml:"filltype,attr,omitempty"`
	Src          string `xml:"src,attr,omitempty"`
	ImageAlignShape string `xml:"imagealignshape,attr,omitempty"`
	ImageSize    string `xml:"imagesize,attr,omitempty"`
	StartArrow   string `xml:"startarrow,attr,omitempty"`    // none, block, classic, diamond, oval, open
	StartArrowWidth string `xml:"startarrowwidth,attr,omitempty"` // narrow, medium, wide
	StartArrowLength string `xml:"startarrowlength,attr,omitempty"` // short, medium, long
	EndArrow     string `xml:"endarrow,attr,omitempty"`
	EndArrowWidth string `xml:"endarrowwidth,attr,omitempty"`
	EndArrowLength string `xml:"endarrowlength,attr,omitempty"`
	InsetPen     string `xml:"insetpen,attr,omitempty"`      // t, f
	RID          string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr,omitempty"`
}

// Shadow represents the <v:shadow> element
type Shadow struct {
	ID       string `xml:"id,attr,omitempty"`
	On       string `xml:"on,attr,omitempty"`       // t, f
	Type     string `xml:"type,attr,omitempty"`      // single, double, emboss, perspective
	Color    string `xml:"color,attr,omitempty"`
	Color2   string `xml:"color2,attr,omitempty"`
	Opacity  string `xml:"opacity,attr,omitempty"`
	Offset   string `xml:"offset,attr,omitempty"`    // x,y
	Offset2  string `xml:"offset2,attr,omitempty"`
	Origin   string `xml:"origin,attr,omitempty"`
	Matrix   string `xml:"matrix,attr,omitempty"`
	Obscured string `xml:"obscured,attr,omitempty"` // t, f
}

// --- Text Elements ---

// Textbox represents the <v:textbox> element
type Textbox struct {
	ID        string `xml:"id,attr,omitempty"`
	Style     string `xml:"style,attr,omitempty"`
	Inset     string `xml:"inset,attr,omitempty"` // margins: "left,top,right,bottom"
	SingleClick string `xml:"urn:schemas-microsoft-com:office:office singleclick,attr,omitempty"`
	InsetMode string `xml:"insetmode,attr,omitempty"` // auto, custom
	InnerXML  []byte `xml:",innerxml"`
}

// TextPath represents the <v:textpath> element - text along a path (WordArt)
type TextPath struct {
	ID         string `xml:"id,attr,omitempty"`
	On         string `xml:"on,attr,omitempty"`
	FitShape   string `xml:"fitshape,attr,omitempty"`
	FitPath    string `xml:"fitpath,attr,omitempty"`
	Trim       string `xml:"trim,attr,omitempty"`
	XScale     string `xml:"xscale,attr,omitempty"`
	String     string `xml:"string,attr,omitempty"`
	Style      string `xml:"style,attr,omitempty"`
}

// --- Image ---

// ImageData represents the <v:imagedata> element
type ImageData struct {
	ID           string `xml:"id,attr,omitempty"`
	Src          string `xml:"src,attr,omitempty"`
	CropTop      string `xml:"croptop,attr,omitempty"`
	CropBottom   string `xml:"cropbottom,attr,omitempty"`
	CropLeft     string `xml:"cropleft,attr,omitempty"`
	CropRight    string `xml:"cropright,attr,omitempty"`
	Gain         string `xml:"gain,attr,omitempty"`
	BlackLevel   string `xml:"blacklevel,attr,omitempty"`
	Gamma        string `xml:"gamma,attr,omitempty"`
	GrayScale    string `xml:"grayscale,attr,omitempty"`   // t, f
	BiLevel      string `xml:"bilevel,attr,omitempty"`     // t, f
	ChromaKey    string `xml:"chromakey,attr,omitempty"`
	EmbossColor  string `xml:"embosscolor,attr,omitempty"`
	RecolorTarget string `xml:"recolortarget,attr,omitempty"`
	OTitle       string `xml:"urn:schemas-microsoft-com:office:office title,attr,omitempty"`
	RID          string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr,omitempty"`
	ORelID       string `xml:"urn:schemas-microsoft-com:office:office relid,attr,omitempty"`
}

// --- Path and Formulas ---

// PathEl represents the <v:path> element (named PathEl to avoid conflict with shape path attribute)
type PathEl struct {
	ID               string `xml:"id,attr,omitempty"`
	V                string `xml:"v,attr,omitempty"` // path data using VML path commands
	Limo             string `xml:"limo,attr,omitempty"`
	TextBoxRect      string `xml:"textboxrect,attr,omitempty"`
	FillOk           string `xml:"fillok,attr,omitempty"`           // t, f
	StrokeOk         string `xml:"strokeok,attr,omitempty"`         // t, f
	ShadowOk         string `xml:"shadowok,attr,omitempty"`         // t, f
	ArrowOk          string `xml:"arrowok,attr,omitempty"`          // t, f
	GradientShapeOk  string `xml:"gradientshapeok,attr,omitempty"`  // t, f
	TextPathOk       string `xml:"textpathok,attr,omitempty"`       // t, f
	InsetPenOk       string `xml:"insetpenok,attr,omitempty"`       // t, f
	OConnectType     string `xml:"urn:schemas-microsoft-com:office:office connecttype,attr,omitempty"` // none, rect, segments, custom
	OConnectLocs     string `xml:"urn:schemas-microsoft-com:office:office connectlocs,attr,omitempty"`
	OConnectAngles   string `xml:"urn:schemas-microsoft-com:office:office connectangles,attr,omitempty"`
	OExtrusionOk     string `xml:"urn:schemas-microsoft-com:office:office extrusionok,attr,omitempty"`
}

// Formulas represents the <v:formulas> element
type Formulas struct {
	F []*Formula `xml:"urn:schemas-microsoft-com:vml f,omitempty"`
}

// Formula represents the <v:f> element
type Formula struct {
	Eqn string `xml:"eqn,attr,omitempty"` // formula expression
}

// Handles represents the <v:handles> element
type Handles struct {
	H []*Handle `xml:"urn:schemas-microsoft-com:vml h,omitempty"`
}

// Handle represents the <v:h> element - adjustment handle
type Handle struct {
	Position   string `xml:"position,attr,omitempty"`
	Polar      string `xml:"polar,attr,omitempty"`
	Map        string `xml:"map,attr,omitempty"`
	InvX       string `xml:"invx,attr,omitempty"`       // t, f
	InvY       string `xml:"invy,attr,omitempty"`       // t, f
	Switch     string `xml:"switch,attr,omitempty"`     // t, f
	XRange     string `xml:"xrange,attr,omitempty"`
	YRange     string `xml:"yrange,attr,omitempty"`
	RadiusRange string `xml:"radiusrange,attr,omitempty"`
}

// --- Office VML Extensions (o:) ---

// Lock represents the <o:lock> element - shape locking
type Lock struct {
	Ext           string `xml:"urn:schemas-microsoft-com:vml ext,attr,omitempty"` // "edit"
	AspectRatio   string `xml:"aspectratio,attr,omitempty"`   // t, f
	Text          string `xml:"text,attr,omitempty"`          // t, f
	Rotation      string `xml:"rotation,attr,omitempty"`      // t, f
	Cropping      string `xml:"cropping,attr,omitempty"`      // t, f
	Verticies     string `xml:"verticies,attr,omitempty"`     // t, f
	AdjustHandles string `xml:"adjusthandles,attr,omitempty"` // t, f
	Grouping      string `xml:"grouping,attr,omitempty"`      // t, f
	Ungrouping    string `xml:"ungrouping,attr,omitempty"`    // t, f
	Selection     string `xml:"selection,attr,omitempty"`     // t, f
	Position      string `xml:"position,attr,omitempty"`      // t, f
	ShapeType     string `xml:"shapetype,attr,omitempty"`     // t, f
}

// Callout represents the <o:callout> element
type Callout struct {
	Ext          string `xml:"urn:schemas-microsoft-com:vml ext,attr,omitempty"`
	On           string `xml:"on,attr,omitempty"`           // t, f
	Type         string `xml:"type,attr,omitempty"`
	Gap          string `xml:"gap,attr,omitempty"`
	Angle        string `xml:"angle,attr,omitempty"`        // any, 30, 45, 60, 90, auto
	Drop         string `xml:"drop,attr,omitempty"`
	DropAuto     string `xml:"dropauto,attr,omitempty"`     // t, f
	Distance     string `xml:"distance,attr,omitempty"`
	LengthSpecified string `xml:"lengthspecified,attr,omitempty"` // t, f
	Length       string `xml:"length,attr,omitempty"`
	AccentBar    string `xml:"accentbar,attr,omitempty"`    // t, f
	TextBorder   string `xml:"textborder,attr,omitempty"`   // t, f
	MiniGo       string `xml:"minusx,attr,omitempty"`
	MinusY       string `xml:"minusy,attr,omitempty"`
}

// Extrusion represents the <o:extrusion> element - 3D extrusion
type Extrusion struct {
	Ext             string `xml:"urn:schemas-microsoft-com:vml ext,attr,omitempty"`
	On              string `xml:"on,attr,omitempty"`              // t, f
	Type            string `xml:"type,attr,omitempty"`             // parallel, perspective
	Render          string `xml:"render,attr,omitempty"`           // solid, wireFrame, boundingCube
	ViewPointOrigin string `xml:"viewpointorigin,attr,omitempty"`
	ViewPoint       string `xml:"viewpoint,attr,omitempty"`
	Plane           string `xml:"plane,attr,omitempty"`            // XY, ZX, YZ
	SkewAngle       string `xml:"skewangle,attr,omitempty"`
	SkewAmt         string `xml:"skewamt,attr,omitempty"`
	ForeDepth       string `xml:"foredepth,attr,omitempty"`
	BackDepth       string `xml:"backdepth,attr,omitempty"`
	Orientation     string `xml:"orientation,attr,omitempty"`
	OrientationAngle string `xml:"orientationangle,attr,omitempty"`
	LockRotationCenter string `xml:"lockrotationcenter,attr,omitempty"` // t, f
	AutoRotationCenter string `xml:"autorotationcenter,attr,omitempty"` // t, f
	RotationCenter  string `xml:"rotationcenter,attr,omitempty"`
	RotationAngle   string `xml:"rotationangle,attr,omitempty"`
	Color           string `xml:"color,attr,omitempty"`
	Shininess       string `xml:"shininess,attr,omitempty"`
	Specularity     string `xml:"specularity,attr,omitempty"`
	Diffusity       string `xml:"diffusity,attr,omitempty"`
	Metal           string `xml:"metal,attr,omitempty"`           // t, f
	Edge            string `xml:"edge,attr,omitempty"`
	FaceAt          string `xml:"facet,attr,omitempty"`
	LightFace       string `xml:"lightface,attr,omitempty"`       // t, f
	Brightness      string `xml:"brightness,attr,omitempty"`
	LightPosition   string `xml:"lightposition,attr,omitempty"`
	LightLevel      string `xml:"lightlevel,attr,omitempty"`
	LightHarsh      string `xml:"lightharsh,attr,omitempty"`      // t, f
	LightPosition2  string `xml:"lightposition2,attr,omitempty"`
	LightLevel2     string `xml:"lightlevel2,attr,omitempty"`
	LightHarsh2     string `xml:"lightharsh2,attr,omitempty"`     // t, f
}

// SignatureLine represents the <o:signatureline> element
type SignatureLine struct {
	Ext                   string `xml:"urn:schemas-microsoft-com:vml ext,attr,omitempty"`
	ID                    string `xml:"id,attr,omitempty"`
	ProvID                string `xml:"provid,attr,omitempty"`
	SigProvURL            string `xml:"sigprovurl,attr,omitempty"`
	IsSigned              string `xml:"issignatureline,attr,omitempty"`        // t, f
	IsSignedDateTimeSet   string `xml:"issigneddatetimeset,attr,omitempty"`   // t, f
	SignatureSetupCertSrc string `xml:"sigsetupallowcomments,attr,omitempty"` // t, f
	ShowSignDate          string `xml:"showsigndate,attr,omitempty"`          // t, f
	ShowSignTitle         string `xml:"showsigntitle,attr,omitempty"`         // t, f
	SuggestedSigner       string `xml:"suggestedsigner,attr,omitempty"`
	SuggestedSigner2      string `xml:"suggestedsigner2,attr,omitempty"`
	SuggestedSignerEmail  string `xml:"suggestedsigneremail,attr,omitempty"`
	SignInstructions       string `xml:"signinginstructions,attr,omitempty"`
	AddlXml               string `xml:"addlxml,attr,omitempty"`
	SigProvDetails         string `xml:"sigprovdetails,attr,omitempty"`
}

// --- Word VML Extensions (w10:) ---

// Wrap represents the <w10:wrap> element - text wrapping for Word
type Wrap struct {
	Type     string `xml:"type,attr,omitempty"`     // none, topAndBottom, square, tight, through
	Side     string `xml:"side,attr,omitempty"`     // both, left, right, largest
	AnchorX  string `xml:"anchorx,attr,omitempty"`  // margin, page, text, char
	AnchorY  string `xml:"anchory,attr,omitempty"`  // margin, page, text, line
}

// AnchorLock represents the <w10:anchorlock> element
type AnchorLock struct{}

// BorderTop represents the <w10:bordertop> element
type BorderTop struct {
	Type  string `xml:"type,attr,omitempty"`
	Width int32  `xml:"width,attr,omitempty"`
	Color string `xml:"color,attr,omitempty"`
}

// BorderBottom represents the <w10:borderbottom> element
type BorderBottom struct {
	Type  string `xml:"type,attr,omitempty"`
	Width int32  `xml:"width,attr,omitempty"`
	Color string `xml:"color,attr,omitempty"`
}

// BorderLeft represents the <w10:borderleft> element
type BorderLeft struct {
	Type  string `xml:"type,attr,omitempty"`
	Width int32  `xml:"width,attr,omitempty"`
	Color string `xml:"color,attr,omitempty"`
}

// BorderRight represents the <w10:borderright> element
type BorderRight struct {
	Type  string `xml:"type,attr,omitempty"`
	Width int32  `xml:"width,attr,omitempty"`
	Color string `xml:"color,attr,omitempty"`
}

// --- Excel VML Extensions (x:) ---

// ClientData represents the <x:ClientData> element - Excel form control data
type ClientData struct {
	ObjectType     string `xml:"ObjectType,attr,omitempty"` // Note, Drop, Check, Button, GBox, Spin, List, Radio, Scroll, Edit, Label, Dialog, Rect, Shape, Group, Pict
	MoveWithCells  string `xml:"urn:schemas-microsoft-com:office:excel MoveWithCells,omitempty"`
	SizeWithCells  string `xml:"urn:schemas-microsoft-com:office:excel SizeWithCells,omitempty"`
	Anchor         string `xml:"urn:schemas-microsoft-com:office:excel Anchor,omitempty"`
	AutoFill       string `xml:"urn:schemas-microsoft-com:office:excel AutoFill,omitempty"`
	Row            *int32 `xml:"urn:schemas-microsoft-com:office:excel Row,omitempty"`
	Column         *int32 `xml:"urn:schemas-microsoft-com:office:excel Column,omitempty"`
	Visible        string `xml:"urn:schemas-microsoft-com:office:excel Visible,omitempty"`
	FmlaMacro      string `xml:"urn:schemas-microsoft-com:office:excel FmlaMacro,omitempty"`
	TextHAlign     string `xml:"urn:schemas-microsoft-com:office:excel TextHAlign,omitempty"`
	TextVAlign     string `xml:"urn:schemas-microsoft-com:office:excel TextVAlign,omitempty"`
	Val            *int32 `xml:"urn:schemas-microsoft-com:office:excel Val,omitempty"`
	Min            *int32 `xml:"urn:schemas-microsoft-com:office:excel Min,omitempty"`
	Max            *int32 `xml:"urn:schemas-microsoft-com:office:excel Max,omitempty"`
	Inc            *int32 `xml:"urn:schemas-microsoft-com:office:excel Inc,omitempty"`
	Page           *int32 `xml:"urn:schemas-microsoft-com:office:excel Page,omitempty"`
	Dx             *int32 `xml:"urn:schemas-microsoft-com:office:excel Dx,omitempty"`
	MultiLine      string `xml:"urn:schemas-microsoft-com:office:excel MultiLine,omitempty"`
	VScroll        string `xml:"urn:schemas-microsoft-com:office:excel VScroll,omitempty"`
	NoThreeD       string `xml:"urn:schemas-microsoft-com:office:excel NoThreeD,omitempty"`
	NoThreeD2      string `xml:"urn:schemas-microsoft-com:office:excel NoThreeD2,omitempty"`
	Checked        *int32 `xml:"urn:schemas-microsoft-com:office:excel Checked,omitempty"`
	FmlaLink       string `xml:"urn:schemas-microsoft-com:office:excel FmlaLink,omitempty"`
	FmlaRange      string `xml:"urn:schemas-microsoft-com:office:excel FmlaRange,omitempty"`
	WidthMin       *int32 `xml:"urn:schemas-microsoft-com:office:excel WidthMin,omitempty"`
	Sel            *int32 `xml:"urn:schemas-microsoft-com:office:excel Sel,omitempty"`
	SelType        string `xml:"urn:schemas-microsoft-com:office:excel SelType,omitempty"`
	DropStyle      string `xml:"urn:schemas-microsoft-com:office:excel DropStyle,omitempty"`
	DropLines      *int32 `xml:"urn:schemas-microsoft-com:office:excel DropLines,omitempty"`
	LCT            string `xml:"urn:schemas-microsoft-com:office:excel LCT,omitempty"`
}
