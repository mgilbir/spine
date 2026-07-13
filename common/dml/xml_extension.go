// Package dml provides DrawingML XML extension types from dml-main.xsd.
// These types handle forward/backward compatibility with Office document versions.
package dml

import (
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
}

// CompatExt represents compatibility extension wrapper
type CompatExt struct {
	SpId string `xml:"spId,attr,omitempty"`
}

// NonVisualDrawingPropsExtension represents extension for non-visual drawing properties
type NonVisualDrawingPropsExtension struct {
	CreationId *CreationId `xml:"http://schemas.microsoft.com/office/drawing/2014/main creationId,omitempty"`
}

// --- a16 extensions (Drawing 2014) ---

// CreationId represents a16:creationId extension element
type CreationId struct {
	Id string `xml:"id,attr,omitempty"`
}

// A16ColId represents a16:colId extension element (table column identifier)
type A16ColId struct {
	Val uint32 `xml:"val,attr"`
}

// A16RowId represents a16:rowId extension element (table row identifier)
type A16RowId struct {
	Val uint32 `xml:"val,attr"`
}

// --- a14 extensions (Drawing 2010) ---

// A14UseLocalDpi represents a14:useLocalDpi extension element. The val
// attribute is xsd:boolean (e.g. val="0" or val="true"), so it must be modeled
// as a bool — an int32 fails to parse the lexical "true"/"false" forms.
type A14UseLocalDpi struct {
	Val *bool `xml:"val,attr,omitempty"`
}

// A14ShadowObscured represents a14:shadowObscured extension element. The val
// attribute is xsd:boolean; see A14UseLocalDpi.
type A14ShadowObscured struct {
	Val *bool `xml:"val,attr,omitempty"`
}

// A14HiddenFill represents a14:hiddenFill extension element.
// Contains fill properties (CT_FillProperties) from the DML namespace.
type A14HiddenFill struct {
	SolidFill *SolidFill `xml:"http://schemas.openxmlformats.org/drawingml/2006/main solidFill,omitempty"`
	GradFill  *GradFill  `xml:"http://schemas.openxmlformats.org/drawingml/2006/main gradFill,omitempty"`
	PattFill  *PattFill  `xml:"http://schemas.openxmlformats.org/drawingml/2006/main pattFill,omitempty"`
	NoFill    *NoFillXML `xml:"http://schemas.openxmlformats.org/drawingml/2006/main noFill,omitempty"`
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
}

// A14HiddenEffects represents a14:hiddenEffects extension element.
// Contains an effect list (CT_EffectList).
type A14HiddenEffects struct {
	EffectLst *EffectLst `xml:"http://schemas.openxmlformats.org/drawingml/2006/main effectLst,omitempty"`
}

// A14ImgProps represents a14:imgProps extension element.
// Contains image processing properties.
type A14ImgProps struct {
	ImgLayer *A14ImgLayer `xml:"http://schemas.microsoft.com/office/drawing/2010/main imgLayer,omitempty"`
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
}

// UnmarshalXML decodes the known effect kinds into typed fields and falls back
// to capturing the raw inner XML for unmodeled effects, so they survive
// re-marshal instead of being deleted.
func (v *A14ImgEffect) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
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
	}
	return nil
}

// A14Saturation represents a14:saturation element.
type A14Saturation struct {
	Sat *int32 `xml:"sat,attr,omitempty"`
}

// A14Brightness represents a14:brightnessContrast element.
type A14Brightness struct {
	Bright   *int32 `xml:"bright,attr,omitempty"`
	Contrast *int32 `xml:"contrast,attr,omitempty"`
}

// A14Sharpen represents a14:sharpenSoften element.
type A14Sharpen struct {
	Amount *int32 `xml:"amount,attr,omitempty"`
}

// A14ColorTemp represents a14:colorTemperature element.
type A14ColorTemp struct {
	ColorTemp *int32 `xml:"colorTemp,attr,omitempty"`
}

// --- Other extensions ---

// ASvgBlip represents asvg:svgBlip extension element.
type ASvgBlip struct {
	Embed string `xml:"-"` // r:embed attribute (namespaced)
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
	for _, attr := range start.Attr {
		switch {
		case attr.Name.Local == "embed" && attr.Name.Space == xmlb.NSOfficeDocumentRels:
			v.Embed = attr.Value
		}
	}
	return d.Skip()
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
	for _, attr := range start.Attr {
		if attr.Name.Local == "uri" {
			e.URI = attr.Value
		}
	}

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
		// Unknown extension - preserve raw bytes
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
	b.StartElement(ns, localName, xmlb.StrAttr("uri", e.URI))

	switch {
	case e.CreationId != nil:
		b.EmptyElementInlineNS(nsA16, xmlb.PrefixDrawing2014, "creationId",
			xmlb.StrAttr("id", e.CreationId.Id))

	case e.ColId != nil:
		b.EmptyElementInlineNS(nsA16, xmlb.PrefixDrawing2014, "colId",
			xmlb.UintAttr("val", e.ColId.Val))

	case e.RowId != nil:
		b.EmptyElementInlineNS(nsA16, xmlb.PrefixDrawing2014, "rowId",
			xmlb.UintAttr("val", e.RowId.Val))

	case e.UseLocalDpi != nil:
		marshalA14Simple(b, "useLocalDpi", e.UseLocalDpi.Val)

	case e.ShadowObscured != nil:
		marshalA14Simple(b, "shadowObscured", e.ShadowObscured.Val)

	case e.HiddenFill != nil:
		b.StartElementInlineNS(nsA14, xmlb.PrefixDrawing2010, "hiddenFill")
		b.MarshalChildren(nsA, e.HiddenFill)
		b.EndElementInlineNS(xmlb.PrefixDrawing2010, "hiddenFill")
		b.ResetNamespaceDeclaration(nsA14)

	case e.HiddenLine != nil:
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
		b.StartElementInlineNS(nsA14, xmlb.PrefixDrawing2010, "hiddenEffects")
		b.MarshalChildren(nsA, e.HiddenEffects)
		b.EndElementInlineNS(xmlb.PrefixDrawing2010, "hiddenEffects")
		b.ResetNamespaceDeclaration(nsA14)

	case e.ImgProps != nil:
		b.StartElementInlineNS(nsA14, xmlb.PrefixDrawing2010, "imgProps")
		marshalImgLayer(b, e.ImgProps.ImgLayer)
		b.EndElementInlineNS(xmlb.PrefixDrawing2010, "imgProps")
		b.ResetNamespaceDeclaration(nsA14)

	case e.SvgBlip != nil:
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

	b.EndElement(ns, localName)
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
		b.StartElement(nsA14, "imgEffect")
		if len(eff.Raw) > 0 {
			// Unmodeled effect (e.g. artistic effects): re-emit the
			// captured raw bytes, as the unknown-URI Ext fallback does.
			b.WriteRaw(eff.Raw)
		} else {
			b.MarshalChildren(nsA14, eff)
		}
		b.EndElement(nsA14, "imgEffect")
	}
	b.EndElement(nsA14, "imgLayer")
}
