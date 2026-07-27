// This file provides DrawingML XML extension types from dml-main.xsd.
// These types handle forward/backward compatibility with Office document versions.

package dml

import (
	"bytes"
	"encoding/xml"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// ExtLst represents CT_OfficeArtExtensionList (a:extLst)
// Extension list for future compatibility
type ExtLst struct {
	Ext []*Ext `xml:"http://schemas.openxmlformats.org/drawingml/2006/main ext,omitempty"`
}

// Ext represents CT_OfficeArtExtension (a:ext).
// The XSD defines content as <xsd:any processContents="lax" minOccurs="0" maxOccurs="unbounded"/>.
// Known extension types are parsed into typed fields; unknown extensions use RawContent.
type Ext struct {
	URI string `xml:"uri,attr"`

	// a16 extensions (Drawing 2014)
	CreationId *CreationId `xml:"-"`
	ColId      *A16ColId   `xml:"-"`
	RowId      *A16RowId   `xml:"-"`

	// a14 extensions (Drawing 2010)
	UseLocalDpi    *A14UseLocalDpi    `xml:"-"`
	ShadowObscured *A14ShadowObscured `xml:"-"`
	HiddenFill     *A14HiddenFill     `xml:"-"`
	HiddenLine     *A14HiddenLine     `xml:"-"`
	HiddenEffects  *A14HiddenEffects  `xml:"-"`
	ImgProps       *A14ImgProps       `xml:"-"`

	// Other extensions
	SvgBlip      *ASvgBlip         `xml:"-"`
	ThemeFamily  *Thm15ThemeFamily `xml:"-"`
	DataModelExt *DspDataModelExt  `xml:"-"`

	// Fallback for unrecognized extensions (xsd:any)
	RawContent []byte `xml:"-"`

	// InlineNSDecls preserves xmlns declarations carried on the ext element
	// itself (e.g. <a:ext uri="..." xmlns:foo="urn:foo">). They are re-emitted
	// for unknown-URI extensions so prefixes used by RawContent stay bound.
	InlineNSDecls []xmlb.NSDecl `xml:"-"`
}

// NonVisualDrawingPropsExtension represents extension for non-visual drawing properties
type NonVisualDrawingPropsExtension struct {
	CreationId *CreationId `xml:"http://schemas.microsoft.com/office/drawing/2014/main creationId,omitempty"`
}

// --- a16 extensions (Drawing 2014) ---

// CreationId represents a16:creationId extension element
type CreationId struct {
	Id string `xml:"id,attr,omitempty"`
	// CapturedAttrs preserves the verbatim source attribute list (xmlns
	// declarations interleaved with attributes, e.g. a trailing xmlns="");
	// nil for values built programmatically.
	CapturedAttrs []xmlb.RootAttr `xml:"-"`
}

// A16ColId represents a16:colId extension element (table column identifier)
type A16ColId struct {
	Val           uint32          `xml:"val,attr"`
	CapturedAttrs []xmlb.RootAttr `xml:"-"` // see CreationId.CapturedAttrs
}

// A16RowId represents a16:rowId extension element (table row identifier)
type A16RowId struct {
	Val           uint32          `xml:"val,attr"`
	CapturedAttrs []xmlb.RootAttr `xml:"-"` // see CreationId.CapturedAttrs
}

// --- a14 extensions (Drawing 2010) ---

// A14UseLocalDpi represents a14:useLocalDpi extension element. The val
// attribute is xsd:boolean (e.g. val="0" or val="true"), so it must be modeled
// as a bool — an int32 fails to parse the lexical "true"/"false" forms.
type A14UseLocalDpi struct {
	Val           *bool           `xml:"val,attr,omitempty"`
	CapturedAttrs []xmlb.RootAttr `xml:"-"` // see CreationId.CapturedAttrs
}

// A14ShadowObscured represents a14:shadowObscured extension element. The val
// attribute is xsd:boolean; see A14UseLocalDpi.
type A14ShadowObscured struct {
	Val           *bool           `xml:"val,attr,omitempty"`
	CapturedAttrs []xmlb.RootAttr `xml:"-"` // see CreationId.CapturedAttrs
}

// A14HiddenFill represents a14:hiddenFill extension element.
// Contains fill properties (CT_FillProperties) from the DML namespace.
type A14HiddenFill struct {
	SolidFill *SolidFill   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main solidFill,omitempty"`
	GradFill  *GradFill    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main gradFill,omitempty"`
	PattFill  *PattFill    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main pattFill,omitempty"`
	NoFill    *NoFillXML   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main noFill,omitempty"`
	BlipFill  *BlipFillXML `xml:"http://schemas.openxmlformats.org/drawingml/2006/main blipFill,omitempty"`
	GrpFill   *GrpFill     `xml:"http://schemas.openxmlformats.org/drawingml/2006/main grpFill,omitempty"`

	CapturedAttrs []xmlb.RootAttr `xml:"-"` // see CreationId.CapturedAttrs
}

// UnmarshalXML captures the element's verbatim attribute list before decoding
// children through the struct tags.
func (v *A14HiddenFill) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	v.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias A14HiddenFill
	return d.DecodeElement((*alias)(v), &start)
}

// A14HiddenLine represents a14:hiddenLine extension element.
// Has the same structure as CT_LineProperties (a:ln).
type A14HiddenLine struct {
	W         *int64     `xml:"w,attr,omitempty"`
	Cap       string     `xml:"cap,attr,omitempty"`
	Cmpd      string     `xml:"cmpd,attr,omitempty"`
	Algn      string     `xml:"algn,attr,omitempty"`
	NoFill    *NoFillXML `xml:"http://schemas.openxmlformats.org/drawingml/2006/main noFill,omitempty"`
	SolidFill *SolidFill `xml:"http://schemas.openxmlformats.org/drawingml/2006/main solidFill,omitempty"`
	GradFill  *GradFill  `xml:"http://schemas.openxmlformats.org/drawingml/2006/main gradFill,omitempty"`
	PattFill  *PattFill  `xml:"http://schemas.openxmlformats.org/drawingml/2006/main pattFill,omitempty"`
	PrstDash  *PrstDash  `xml:"http://schemas.openxmlformats.org/drawingml/2006/main prstDash,omitempty"`
	CustDash  *CustDash  `xml:"http://schemas.openxmlformats.org/drawingml/2006/main custDash,omitempty"`
	Round     *Round     `xml:"http://schemas.openxmlformats.org/drawingml/2006/main round,omitempty"`
	Bevel     *Bevel     `xml:"http://schemas.openxmlformats.org/drawingml/2006/main bevel,omitempty"`
	Miter     *Miter     `xml:"http://schemas.openxmlformats.org/drawingml/2006/main miter,omitempty"`
	HeadEnd   *LineEnd   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main headEnd,omitempty"`
	TailEnd   *LineEnd   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main tailEnd,omitempty"`

	CapturedAttrs []xmlb.RootAttr `xml:"-"` // see CreationId.CapturedAttrs
}

// UnmarshalXML captures the element's verbatim attribute list before decoding
// attributes and children through the struct tags.
func (v *A14HiddenLine) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	v.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias A14HiddenLine
	return d.DecodeElement((*alias)(v), &start)
}

// A14HiddenEffects represents a14:hiddenEffects extension element.
// Contains an effect list (CT_EffectList).
type A14HiddenEffects struct {
	EffectLst *EffectLst `xml:"http://schemas.openxmlformats.org/drawingml/2006/main effectLst,omitempty"`

	CapturedAttrs []xmlb.RootAttr `xml:"-"` // see CreationId.CapturedAttrs
}

// UnmarshalXML captures the element's verbatim attribute list before decoding
// children through the struct tags.
func (v *A14HiddenEffects) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	v.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias A14HiddenEffects
	return d.DecodeElement((*alias)(v), &start)
}

// A14ImgProps represents a14:imgProps extension element.
// Contains image processing properties.
type A14ImgProps struct {
	ImgLayer *A14ImgLayer `xml:"http://schemas.microsoft.com/office/drawing/2010/main imgLayer,omitempty"`

	CapturedAttrs []xmlb.RootAttr `xml:"-"` // see CreationId.CapturedAttrs
}

// UnmarshalXML captures the element's verbatim attribute list before decoding
// children through the struct tags.
func (v *A14ImgProps) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	v.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias A14ImgProps
	return d.DecodeElement((*alias)(v), &start)
}

// A14ImgLayer represents a14:imgLayer element.
type A14ImgLayer struct {
	Embed      string          `xml:"-"` // r:embed attribute (namespaced)
	ImgEffects []*A14ImgEffect `xml:"http://schemas.microsoft.com/office/drawing/2010/main imgEffect,omitempty"`
}

// A14ImgEffect represents a14:imgEffect element. Each imgEffect wraps exactly
// one effect child (a choice); the four common adjustment effects are typed.
// Any other effect (a14:artisticChalkSketch and the rest of the ~30 artistic
// effects) is preserved as raw bytes in Raw so the typed imgProps dispatch
// never loses what the unknown-URI raw fallback would have kept.
type A14ImgEffect struct {
	Saturation *A14Saturation `xml:"http://schemas.microsoft.com/office/drawing/2010/main saturation,omitempty"`
	Brightness *A14Brightness `xml:"http://schemas.microsoft.com/office/drawing/2010/main brightnessContrast,omitempty"`
	Sharpen    *A14Sharpen    `xml:"http://schemas.microsoft.com/office/drawing/2010/main sharpenSoften,omitempty"`
	ColorTemp  *A14ColorTemp  `xml:"http://schemas.microsoft.com/office/drawing/2010/main colorTemperature,omitempty"`

	// Raw holds the original inner XML when no typed field matched,
	// mirroring Ext.RawContent for unknown extension URIs.
	Raw []byte `xml:"-"`

	// InlineNSDecls preserves xmlns declarations carried on the imgEffect
	// element itself, re-emitted with Raw so its prefixes stay bound
	// (mirroring Ext.InlineNSDecls).
	InlineNSDecls []xmlb.NSDecl `xml:"-"`
}

// UnmarshalXML decodes the known effect kinds into typed fields and falls back
// to capturing the raw inner XML for unmodeled effects, so they survive
// re-marshal instead of being deleted.
func (v *A14ImgEffect) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var nsDecls []xmlb.NSDecl
	for _, attr := range start.Attr {
		switch {
		case attr.Name.Space == "xmlns":
			nsDecls = append(nsDecls, xmlb.NSDecl{Prefix: attr.Name.Local, URI: attr.Value})
		case attr.Name.Space == "" && attr.Name.Local == "xmlns":
			nsDecls = append(nsDecls, xmlb.NSDecl{URI: attr.Value})
		}
	}
	var aux struct {
		Saturation *A14Saturation `xml:"http://schemas.microsoft.com/office/drawing/2010/main saturation,omitempty"`
		Brightness *A14Brightness `xml:"http://schemas.microsoft.com/office/drawing/2010/main brightnessContrast,omitempty"`
		Sharpen    *A14Sharpen    `xml:"http://schemas.microsoft.com/office/drawing/2010/main sharpenSoften,omitempty"`
		ColorTemp  *A14ColorTemp  `xml:"http://schemas.microsoft.com/office/drawing/2010/main colorTemperature,omitempty"`
		Inner      []byte         `xml:",innerxml"`
	}
	if err := d.DecodeElement(&aux, &start); err != nil {
		return err
	}
	v.Saturation = aux.Saturation
	v.Brightness = aux.Brightness
	v.Sharpen = aux.Sharpen
	v.ColorTemp = aux.ColorTemp
	if v.Saturation == nil && v.Brightness == nil && v.Sharpen == nil && v.ColorTemp == nil {
		v.Raw = aux.Inner
		v.InlineNSDecls = nsDecls
	}
	return nil
}

// A14Saturation represents a14:saturation element.
type A14Saturation struct {
	Sat           *int32          `xml:"sat,attr,omitempty"`
	CapturedAttrs []xmlb.RootAttr `xml:"-"` // see A14Brightness.CapturedAttrs
}

// UnmarshalXML captures the verbatim attribute list before decoding.
func (v *A14Saturation) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	v.CapturedAttrs = xmlb.CaptureAttrs(start.Attr)
	type alias A14Saturation
	return d.DecodeElement((*alias)(v), &start)
}

// A14Brightness represents a14:brightnessContrast element.
type A14Brightness struct {
	Bright   *int32 `xml:"bright,attr,omitempty"`
	Contrast *int32 `xml:"contrast,attr,omitempty"`
	// CapturedAttrs preserves the verbatim source attribute list so any
	// attribute the model does not type (producers emit off-spec attributes
	// such as amount on brightnessContrast) survives re-marshal. Replayed by
	// the reflection marshaler; nil for programmatic values.
	CapturedAttrs []xmlb.RootAttr `xml:"-"`
}

// UnmarshalXML captures the verbatim attribute list before decoding the typed
// attributes, so unmodeled attributes are not dropped on re-marshal.
func (v *A14Brightness) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	v.CapturedAttrs = xmlb.CaptureAttrs(start.Attr)
	type alias A14Brightness
	return d.DecodeElement((*alias)(v), &start)
}

// A14Sharpen represents a14:sharpenSoften element.
type A14Sharpen struct {
	Amount        *int32          `xml:"amount,attr,omitempty"`
	CapturedAttrs []xmlb.RootAttr `xml:"-"` // see A14Brightness.CapturedAttrs
}

// UnmarshalXML captures the verbatim attribute list before decoding.
func (v *A14Sharpen) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	v.CapturedAttrs = xmlb.CaptureAttrs(start.Attr)
	type alias A14Sharpen
	return d.DecodeElement((*alias)(v), &start)
}

// A14ColorTemp represents a14:colorTemperature element.
type A14ColorTemp struct {
	ColorTemp     *int32          `xml:"colorTemp,attr,omitempty"`
	CapturedAttrs []xmlb.RootAttr `xml:"-"` // see A14Brightness.CapturedAttrs
}

// UnmarshalXML captures the verbatim attribute list before decoding.
func (v *A14ColorTemp) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	v.CapturedAttrs = xmlb.CaptureAttrs(start.Attr)
	type alias A14ColorTemp
	return d.DecodeElement((*alias)(v), &start)
}

// --- Other extensions ---

// ASvgBlip represents asvg:svgBlip extension element.
type ASvgBlip struct {
	Embed         string          `xml:"-"` // r:embed attribute (namespaced)
	CapturedAttrs []xmlb.RootAttr `xml:"-"` // see CreationId.CapturedAttrs
}

// Thm15ThemeFamily represents thm15:themeFamily extension element.
type Thm15ThemeFamily struct {
	Name string `xml:"name,attr,omitempty"`
	Id   string `xml:"id,attr,omitempty"`
	Vid  string `xml:"vid,attr,omitempty"`
}

// DspDataModelExt represents dsp:dataModelExt extension element.
type DspDataModelExt struct {
	RelId  string `xml:"-"` // r:relId attribute (namespaced)
	MinVer string `xml:"minVer,attr,omitempty"`
}

// --- Custom UnmarshalXML for types with namespaced attributes ---

func (v *A14ImgLayer) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		switch {
		case attr.Name.Local == "embed" && attr.Name.Space == xmlb.NSOfficeDocumentRels:
			v.Embed = attr.Value
		}
	}
	type alias A14ImgLayer
	aux := (*alias)(v)
	return d.DecodeElement(aux, &start)
}

func (v *ASvgBlip) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	v.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	for _, attr := range start.Attr {
		switch {
		case attr.Name.Local == "embed" && attr.Name.Space == xmlb.NSOfficeDocumentRels:
			v.Embed = attr.Value
		}
	}
	return d.Skip()
}

// UnmarshalXML captures the element's verbatim attribute list (leaf element).
func (v *CreationId) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	v.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	for _, attr := range start.Attr {
		if attr.Name.Space == "" && attr.Name.Local == "id" {
			v.Id = attr.Value
		}
	}
	return d.Skip()
}

// UnmarshalXML captures the element's verbatim attribute list (leaf element).
func (v *A16ColId) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	v.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias A16ColId
	return d.DecodeElement((*alias)(v), &start)
}

// UnmarshalXML captures the element's verbatim attribute list (leaf element).
func (v *A16RowId) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	v.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias A16RowId
	return d.DecodeElement((*alias)(v), &start)
}

// UnmarshalXML captures the element's verbatim attribute list (leaf element).
func (v *A14UseLocalDpi) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	v.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias A14UseLocalDpi
	return d.DecodeElement((*alias)(v), &start)
}

// UnmarshalXML captures the element's verbatim attribute list (leaf element).
func (v *A14ShadowObscured) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	v.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias A14ShadowObscured
	return d.DecodeElement((*alias)(v), &start)
}

func (v *DspDataModelExt) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		switch {
		case attr.Name.Local == "relId" && attr.Name.Space == xmlb.NSOfficeDocumentRels:
			v.RelId = attr.Value
		case attr.Name.Local == "minVer":
			v.MinVer = attr.Value
		}
	}
	return d.Skip()
}

// --- Custom UnmarshalXML for Ext ---

func (e *Ext) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var nsDecls []xmlb.NSDecl
	for _, attr := range start.Attr {
		switch {
		case attr.Name.Local == "uri" && attr.Name.Space == "":
			e.URI = attr.Value
		case attr.Name.Space == "xmlns":
			nsDecls = append(nsDecls, xmlb.NSDecl{Prefix: attr.Name.Local, URI: attr.Value})
		case attr.Name.Space == "" && attr.Name.Local == "xmlns":
			nsDecls = append(nsDecls, xmlb.NSDecl{URI: attr.Value})
		}
	}
	// Keep ext-level declarations for typed content too: marshal replays them
	// on the a:ext element so a source that declared the extension prefix
	// there (instead of on the child) round-trips.
	e.InlineNSDecls = nsDecls

	switch e.URI {
	case xmlb.ExtURICreationId:
		var w struct {
			V CreationId `xml:"http://schemas.microsoft.com/office/drawing/2014/main creationId"`
		}
		if err := d.DecodeElement(&w, &start); err != nil {
			return err
		}
		e.CreationId = &w.V

	case xmlb.ExtURIColId:
		var w struct {
			V A16ColId `xml:"http://schemas.microsoft.com/office/drawing/2014/main colId"`
		}
		if err := d.DecodeElement(&w, &start); err != nil {
			return err
		}
		e.ColId = &w.V

	case xmlb.ExtURIRowId:
		var w struct {
			V A16RowId `xml:"http://schemas.microsoft.com/office/drawing/2014/main rowId"`
		}
		if err := d.DecodeElement(&w, &start); err != nil {
			return err
		}
		e.RowId = &w.V

	case xmlb.ExtURIUseLocalDpi:
		var w struct {
			V A14UseLocalDpi `xml:"http://schemas.microsoft.com/office/drawing/2010/main useLocalDpi"`
		}
		if err := d.DecodeElement(&w, &start); err != nil {
			return err
		}
		e.UseLocalDpi = &w.V

	case xmlb.ExtURIShadowObscured:
		var w struct {
			V A14ShadowObscured `xml:"http://schemas.microsoft.com/office/drawing/2010/main shadowObscured"`
		}
		if err := d.DecodeElement(&w, &start); err != nil {
			return err
		}
		e.ShadowObscured = &w.V

	case xmlb.ExtURIHiddenFill:
		var w struct {
			V A14HiddenFill `xml:"http://schemas.microsoft.com/office/drawing/2010/main hiddenFill"`
		}
		if err := d.DecodeElement(&w, &start); err != nil {
			return err
		}
		e.HiddenFill = &w.V

	case xmlb.ExtURIHiddenLine:
		var w struct {
			V A14HiddenLine `xml:"http://schemas.microsoft.com/office/drawing/2010/main hiddenLine"`
		}
		if err := d.DecodeElement(&w, &start); err != nil {
			return err
		}
		e.HiddenLine = &w.V

	case xmlb.ExtURIHiddenEffects:
		var w struct {
			V A14HiddenEffects `xml:"http://schemas.microsoft.com/office/drawing/2010/main hiddenEffects"`
		}
		if err := d.DecodeElement(&w, &start); err != nil {
			return err
		}
		e.HiddenEffects = &w.V

	case xmlb.ExtURIImgProps:
		var w struct {
			V A14ImgProps `xml:"http://schemas.microsoft.com/office/drawing/2010/main imgProps"`
		}
		if err := d.DecodeElement(&w, &start); err != nil {
			return err
		}
		e.ImgProps = &w.V

	case xmlb.ExtURISvgBlip:
		var w struct {
			V ASvgBlip `xml:"http://schemas.microsoft.com/office/drawing/2016/SVG/main svgBlip"`
		}
		if err := d.DecodeElement(&w, &start); err != nil {
			return err
		}
		e.SvgBlip = &w.V

	case xmlb.ExtURIThemeFamily:
		var w struct {
			V Thm15ThemeFamily `xml:"http://schemas.microsoft.com/office/thememl/2012/main themeFamily"`
		}
		if err := d.DecodeElement(&w, &start); err != nil {
			return err
		}
		e.ThemeFamily = &w.V

	case xmlb.ExtURIDataModelExt:
		var w struct {
			V DspDataModelExt `xml:"http://schemas.microsoft.com/office/drawing/2008/diagram dataModelExt"`
		}
		if err := d.DecodeElement(&w, &start); err != nil {
			return err
		}
		e.DataModelExt = &w.V

	default:
		// Unknown extension - preserve raw bytes along with any xmlns
		// declarations the ext element carried, so re-emitted content
		// keeps its prefixes bound.
		var inner struct {
			Content []byte `xml:",innerxml"`
		}
		if err := d.DecodeElement(&inner, &start); err != nil {
			return err
		}
		e.RawContent = inner.Content
	}

	return nil
}

// --- MarshalToBuilder for Ext ---

const (
	nsA   = xmlb.NSDrawingML
	nsA14 = xmlb.NSDrawing2010
	nsA16 = xmlb.NSDrawing2014
	nsR   = xmlb.NSOfficeDocumentRels
)

func (e *Ext) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	// Captured ext-level declarations are replayed for typed content too: a
	// source that declared the extension prefix on the a:ext element (rather
	// than on the child) must get it back in the same place. The typed child
	// only synthesizes its own declaration when it carries no capture.
	attrs := xmlb.NSDeclAttrs([]xmlb.Attr{xmlb.StrAttr("uri", e.URI)}, e.InlineNSDecls)
	b.StartElement(ns, localName, attrs...)
	e.marshalContent(b)
	b.EndElement(ns, localName)
}

// MarshalXML implements xml.Marshaler for the encoding/xml path. All content
// fields are xml:"-", so without this the reflection-free stdlib encoder would
// emit <a:ext uri="…"></a:ext> and silently delete the a16/a14 child. The
// element is rendered through the Builder (which owns the extension prefix and
// inline-declaration logic in marshalContent) and its self-contained tokens are
// replayed into the encoder.
func (e *Ext) MarshalXML(enc *xml.Encoder, start xml.StartElement) error {
	b := xmlb.NewBuilder()
	attrs := xmlb.NSDeclAttrs([]xmlb.Attr{xmlb.StrAttr("uri", e.URI)}, e.InlineNSDecls)
	b.StartElementInlineNS(nsA, xmlb.PrefixDrawingML, "ext", attrs...)
	e.marshalContent(b)
	b.EndElementInlineNS(xmlb.PrefixDrawingML, "ext")
	if err := b.Err(); err != nil {
		return err
	}
	dec := xml.NewDecoder(bytes.NewReader(b.Bytes()))
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		if err := enc.EncodeToken(fixupRawToken(tok)); err != nil {
			return err
		}
	}
	return nil
}

// marshalContent writes the a:ext child content (the typed extension element or
// the raw fallback) between the element's start and end tags.
func (e *Ext) marshalContent(b *xmlb.Builder) {
	switch {
	case e.CreationId != nil:
		if raw := e.CreationId.CapturedAttrs; raw != nil {
			b.EmptyElementLiteral(xmlb.RawAttrPrefix(raw, nsA16, xmlb.PrefixDrawing2014), "creationId",
				xmlb.RawAttrListOverride(raw, map[string]string{"id": e.CreationId.Id})...)
			break
		}
		b.EmptyElementInlineNS(nsA16, xmlb.PrefixDrawing2014, "creationId",
			xmlb.StrAttr("id", e.CreationId.Id))

	case e.ColId != nil:
		if raw := e.ColId.CapturedAttrs; raw != nil {
			b.EmptyElementLiteral(xmlb.RawAttrPrefix(raw, nsA16, xmlb.PrefixDrawing2014), "colId",
				xmlb.RawAttrListOverride(raw, map[string]string{"val": xmlb.UintAttr("val", e.ColId.Val).Value})...)
			break
		}
		b.EmptyElementInlineNS(nsA16, xmlb.PrefixDrawing2014, "colId",
			xmlb.UintAttr("val", e.ColId.Val))

	case e.RowId != nil:
		if raw := e.RowId.CapturedAttrs; raw != nil {
			b.EmptyElementLiteral(xmlb.RawAttrPrefix(raw, nsA16, xmlb.PrefixDrawing2014), "rowId",
				xmlb.RawAttrListOverride(raw, map[string]string{"val": xmlb.UintAttr("val", e.RowId.Val).Value})...)
			break
		}
		b.EmptyElementInlineNS(nsA16, xmlb.PrefixDrawing2014, "rowId",
			xmlb.UintAttr("val", e.RowId.Val))

	case e.UseLocalDpi != nil:
		if raw := e.UseLocalDpi.CapturedAttrs; raw != nil {
			b.EmptyElementLiteral(xmlb.RawAttrPrefix(raw, nsA14, xmlb.PrefixDrawing2010), "useLocalDpi",
				xmlb.RawAttrList(raw)...)
			break
		}
		marshalA14Simple(b, "useLocalDpi", e.UseLocalDpi.Val)

	case e.ShadowObscured != nil:
		if raw := e.ShadowObscured.CapturedAttrs; raw != nil {
			b.EmptyElementLiteral(xmlb.RawAttrPrefix(raw, nsA14, xmlb.PrefixDrawing2010), "shadowObscured",
				xmlb.RawAttrList(raw)...)
			break
		}
		marshalA14Simple(b, "shadowObscured", e.ShadowObscured.Val)

	case e.HiddenFill != nil:
		if raw := e.HiddenFill.CapturedAttrs; raw != nil {
			prefix := xmlb.RawAttrPrefix(raw, nsA14, xmlb.PrefixDrawing2010)
			b.StartElementLiteral(prefix, "hiddenFill",
				[]xmlb.NSDecl{{Prefix: prefix, URI: nsA14}}, xmlb.RawAttrList(raw)...)
			b.MarshalChildren(nsA, e.HiddenFill)
			b.EndElementLiteral(prefix, "hiddenFill")
			break
		}
		b.StartElementInlineNS(nsA14, xmlb.PrefixDrawing2010, "hiddenFill")
		b.MarshalChildren(nsA, e.HiddenFill)
		b.EndElementInlineNS(xmlb.PrefixDrawing2010, "hiddenFill")
		b.ResetNamespaceDeclaration(nsA14)

	case e.HiddenLine != nil:
		if raw := e.HiddenLine.CapturedAttrs; raw != nil {
			prefix := xmlb.RawAttrPrefix(raw, nsA14, xmlb.PrefixDrawing2010)
			b.StartElementLiteral(prefix, "hiddenLine",
				[]xmlb.NSDecl{{Prefix: prefix, URI: nsA14}}, xmlb.RawAttrList(raw)...)
			b.MarshalChildren(nsA, e.HiddenLine)
			b.EndElementLiteral(prefix, "hiddenLine")
			break
		}
		var attrs []xmlb.Attr
		if e.HiddenLine.W != nil {
			attrs = append(attrs, xmlb.IntAttr("w", *e.HiddenLine.W))
		}
		if e.HiddenLine.Cap != "" {
			attrs = append(attrs, xmlb.StrAttr("cap", e.HiddenLine.Cap))
		}
		if e.HiddenLine.Cmpd != "" {
			attrs = append(attrs, xmlb.StrAttr("cmpd", e.HiddenLine.Cmpd))
		}
		if e.HiddenLine.Algn != "" {
			attrs = append(attrs, xmlb.StrAttr("algn", e.HiddenLine.Algn))
		}
		b.StartElementInlineNS(nsA14, xmlb.PrefixDrawing2010, "hiddenLine", attrs...)
		b.MarshalChildren(nsA, e.HiddenLine)
		b.EndElementInlineNS(xmlb.PrefixDrawing2010, "hiddenLine")
		b.ResetNamespaceDeclaration(nsA14)

	case e.HiddenEffects != nil:
		if raw := e.HiddenEffects.CapturedAttrs; raw != nil {
			prefix := xmlb.RawAttrPrefix(raw, nsA14, xmlb.PrefixDrawing2010)
			b.StartElementLiteral(prefix, "hiddenEffects",
				[]xmlb.NSDecl{{Prefix: prefix, URI: nsA14}}, xmlb.RawAttrList(raw)...)
			b.MarshalChildren(nsA, e.HiddenEffects)
			b.EndElementLiteral(prefix, "hiddenEffects")
			break
		}
		b.StartElementInlineNS(nsA14, xmlb.PrefixDrawing2010, "hiddenEffects")
		b.MarshalChildren(nsA, e.HiddenEffects)
		b.EndElementInlineNS(xmlb.PrefixDrawing2010, "hiddenEffects")
		b.ResetNamespaceDeclaration(nsA14)

	case e.ImgProps != nil:
		if raw := e.ImgProps.CapturedAttrs; raw != nil {
			prefix := xmlb.RawAttrPrefix(raw, nsA14, xmlb.PrefixDrawing2010)
			b.StartElementLiteral(prefix, "imgProps",
				[]xmlb.NSDecl{{Prefix: prefix, URI: nsA14}}, xmlb.RawAttrList(raw)...)
			marshalImgLayer(b, e.ImgProps.ImgLayer)
			b.EndElementLiteral(prefix, "imgProps")
			break
		}
		b.StartElementInlineNS(nsA14, xmlb.PrefixDrawing2010, "imgProps")
		marshalImgLayer(b, e.ImgProps.ImgLayer)
		b.EndElementInlineNS(xmlb.PrefixDrawing2010, "imgProps")
		b.ResetNamespaceDeclaration(nsA14)

	case e.SvgBlip != nil:
		if raw := e.SvgBlip.CapturedAttrs; raw != nil {
			b.EmptyElementLiteral(xmlb.RawAttrPrefix(raw, xmlb.NSDrawingSVG2016, xmlb.PrefixDrawingSVG2016), "svgBlip",
				xmlb.RawAttrListOverride(raw, map[string]string{"r:embed": e.SvgBlip.Embed})...)
			break
		}
		b.EmptyElementInlineNS(xmlb.NSDrawingSVG2016, xmlb.PrefixDrawingSVG2016, "svgBlip",
			xmlb.RelAttr("embed", e.SvgBlip.Embed))

	case e.ThemeFamily != nil:
		var attrs []xmlb.Attr
		if e.ThemeFamily.Name != "" {
			attrs = append(attrs, xmlb.StrAttr("name", e.ThemeFamily.Name))
		}
		if e.ThemeFamily.Id != "" {
			attrs = append(attrs, xmlb.StrAttr("id", e.ThemeFamily.Id))
		}
		if e.ThemeFamily.Vid != "" {
			attrs = append(attrs, xmlb.StrAttr("vid", e.ThemeFamily.Vid))
		}
		b.EmptyElementInlineNS(xmlb.NSThemeML2012, xmlb.PrefixThemeML2012, "themeFamily", attrs...)

	case e.DataModelExt != nil:
		var attrs []xmlb.Attr
		if e.DataModelExt.RelId != "" {
			attrs = append(attrs, xmlb.RelAttr("relId", e.DataModelExt.RelId))
		}
		if e.DataModelExt.MinVer != "" {
			attrs = append(attrs, xmlb.StrAttr("minVer", e.DataModelExt.MinVer))
		}
		b.EmptyElementInlineNS(xmlb.NSDrawingDiagram2008, xmlb.PrefixDrawingDiagram2008, "dataModelExt", attrs...)

	default:
		if len(e.RawContent) > 0 {
			b.WriteRaw(e.RawContent)
		}
	}
}

// marshalA14Simple writes a simple a14 extension element with an optional val attribute.
func marshalA14Simple(b *xmlb.Builder, localName string, val *bool) {
	if val != nil {
		b.EmptyElementInlineNS(nsA14, xmlb.PrefixDrawing2010, localName,
			xmlb.BoolAttr("val", *val))
	} else {
		b.EmptyElementInlineNS(nsA14, xmlb.PrefixDrawing2010, localName)
	}
}

// marshalImgLayer writes a14:imgLayer with its effects.
func marshalImgLayer(b *xmlb.Builder, layer *A14ImgLayer) {
	if layer == nil {
		return
	}
	var attrs []xmlb.Attr
	if layer.Embed != "" {
		attrs = append(attrs, xmlb.RelAttr("embed", layer.Embed))
	}
	b.StartElement(nsA14, "imgLayer", attrs...)
	for _, eff := range layer.ImgEffects {
		if len(eff.Raw) > 0 {
			// Unmodeled effect (e.g. artistic effects): re-emit the captured
			// raw bytes, as the unknown-URI Ext fallback does, restoring any
			// xmlns declarations the imgEffect element carried.
			b.StartElement(nsA14, "imgEffect", xmlb.NSDeclAttrs(nil, eff.InlineNSDecls)...)
			b.WriteRaw(eff.Raw)
		} else {
			b.StartElement(nsA14, "imgEffect")
			b.MarshalChildren(nsA14, eff)
		}
		b.EndElement(nsA14, "imgEffect")
	}
	b.EndElement(nsA14, "imgLayer")
}
