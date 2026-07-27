// This file provides DrawingML XML effect types from dml-main.xsd.

package dml

import (
	"bytes"
	"encoding/xml"

	xmlb "github.com/mgilbir/spine/common/xml"
)

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
	BlurRad       *int64              `xml:"blurRad,attr,omitempty"`
	Dist          *int64              `xml:"dist,attr,omitempty"`
	Dir           *int32              `xml:"dir,attr,omitempty"`
	Sx            *Percentage         `xml:"sx,attr,omitempty"`
	Sy            *Percentage         `xml:"sy,attr,omitempty"`
	Kx            *int32              `xml:"kx,attr,omitempty"`
	Ky            *int32              `xml:"ky,attr,omitempty"`
	Algn          string              `xml:"algn,attr,omitempty"`
	RotWithShape  *bool               `xml:"rotWithShape,attr,omitempty"`
	ScRgbClr      *ScRgbClr           `xml:"http://schemas.openxmlformats.org/drawingml/2006/main scrgbClr,omitempty"`
	SrgbClr       *SrgbClr            `xml:"http://schemas.openxmlformats.org/drawingml/2006/main srgbClr,omitempty"`
	HslClr        *HslClr             `xml:"http://schemas.openxmlformats.org/drawingml/2006/main hslClr,omitempty"`
	SysClr        *SystemClr          `xml:"http://schemas.openxmlformats.org/drawingml/2006/main sysClr,omitempty"`
	SchemeClr     *SchemeClrTransform `xml:"http://schemas.openxmlformats.org/drawingml/2006/main schemeClr,omitempty"`
	PrstClr       *PrstClr            `xml:"http://schemas.openxmlformats.org/drawingml/2006/main prstClr,omitempty"`
	CapturedAttrs []xmlb.RootAttr     `xml:"-"` // verbatim source attrs; see common/xml.CaptureAttrs
}

// UnmarshalXML captures the element's verbatim attribute list (source
// attribute order and any unmodeled attributes) before decoding through the
// struct tags; the reflection marshaler replays it.
func (osh *OuterShdw) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	osh.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias OuterShdw
	return d.DecodeElement((*alias)(osh), &start)
}

// InnerShdw represents CT_InnerShadowEffect (a:innerShdw). Its color is an
// EG_ColorChoice, so all six color kinds are modeled: a parsed hsl/sys/prst/
// scrgb color must survive re-marshal instead of collapsing to an empty,
// schema-invalid element.
type InnerShdw struct {
	BlurRad   *int64              `xml:"blurRad,attr,omitempty"`
	Dist      *int64              `xml:"dist,attr,omitempty"`
	Dir       *int32              `xml:"dir,attr,omitempty"`
	ScRgbClr  *ScRgbClr           `xml:"http://schemas.openxmlformats.org/drawingml/2006/main scrgbClr,omitempty"`
	SrgbClr   *SrgbClr            `xml:"http://schemas.openxmlformats.org/drawingml/2006/main srgbClr,omitempty"`
	HslClr    *HslClr             `xml:"http://schemas.openxmlformats.org/drawingml/2006/main hslClr,omitempty"`
	SysClr    *SystemClr          `xml:"http://schemas.openxmlformats.org/drawingml/2006/main sysClr,omitempty"`
	SchemeClr *SchemeClrTransform `xml:"http://schemas.openxmlformats.org/drawingml/2006/main schemeClr,omitempty"`
	PrstClr   *PrstClr            `xml:"http://schemas.openxmlformats.org/drawingml/2006/main prstClr,omitempty"`
}

// ReflectionXML represents CT_ReflectionEffect (a:reflection)
type ReflectionXML struct {
	BlurRad       *int64          `xml:"blurRad,attr,omitempty"`
	StA           *Percentage     `xml:"stA,attr,omitempty"`
	StPos         *Percentage     `xml:"stPos,attr,omitempty"`
	EndA          *Percentage     `xml:"endA,attr,omitempty"`
	EndPos        *Percentage     `xml:"endPos,attr,omitempty"`
	Dist          *int64          `xml:"dist,attr,omitempty"`
	Dir           *int32          `xml:"dir,attr,omitempty"`
	FadeDir       *int32          `xml:"fadeDir,attr,omitempty"`
	Sx            *Percentage     `xml:"sx,attr,omitempty"`
	Sy            *Percentage     `xml:"sy,attr,omitempty"`
	Kx            *int32          `xml:"kx,attr,omitempty"`
	Ky            *int32          `xml:"ky,attr,omitempty"`
	Algn          string          `xml:"algn,attr,omitempty"`
	RotWithShape  *bool           `xml:"rotWithShape,attr,omitempty"`
	CapturedAttrs []xmlb.RootAttr `xml:"-"` // verbatim source attrs; see common/xml.CaptureAttrs
}

// UnmarshalXML captures the element's verbatim attribute list (source
// attribute order, unmodeled attributes, explicit zero values) before
// decoding through the struct tags; the reflection marshaler replays it.
func (rf *ReflectionXML) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	rf.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias ReflectionXML
	return d.DecodeElement((*alias)(rf), &start)
}

// GlowXML represents CT_GlowEffect (a:glow). All six EG_ColorChoice kinds are
// modeled; see InnerShdw.
type GlowXML struct {
	Rad       int64               `xml:"rad,attr,omitempty"`
	ScRgbClr  *ScRgbClr           `xml:"http://schemas.openxmlformats.org/drawingml/2006/main scrgbClr,omitempty"`
	SrgbClr   *SrgbClr            `xml:"http://schemas.openxmlformats.org/drawingml/2006/main srgbClr,omitempty"`
	HslClr    *HslClr             `xml:"http://schemas.openxmlformats.org/drawingml/2006/main hslClr,omitempty"`
	SysClr    *SystemClr          `xml:"http://schemas.openxmlformats.org/drawingml/2006/main sysClr,omitempty"`
	SchemeClr *SchemeClrTransform `xml:"http://schemas.openxmlformats.org/drawingml/2006/main schemeClr,omitempty"`
	PrstClr   *PrstClr            `xml:"http://schemas.openxmlformats.org/drawingml/2006/main prstClr,omitempty"`
}

// SoftEdgeXML represents CT_SoftEdgesEffect (a:softEdge)
type SoftEdgeXML struct {
	Rad int64 `xml:"rad,attr"`
}

// BlurXML represents CT_BlurEffect (a:blur). grow defaults to true, so it is a
// pointer: an explicit false must be emitted rather than omitted (which readers
// treat as true).
type BlurXML struct {
	Rad  int64 `xml:"rad,attr,omitempty"`
	Grow *bool `xml:"grow,attr,omitempty"`
}

// PrstShdw represents CT_PresetShadowEffect (a:prstShdw). All six
// EG_ColorChoice kinds are modeled; see InnerShdw.
type PrstShdw struct {
	Prst      string              `xml:"prst,attr"`
	Dist      int64               `xml:"dist,attr,omitempty"`
	Dir       int32               `xml:"dir,attr,omitempty"`
	ScRgbClr  *ScRgbClr           `xml:"http://schemas.openxmlformats.org/drawingml/2006/main scrgbClr,omitempty"`
	SrgbClr   *SrgbClr            `xml:"http://schemas.openxmlformats.org/drawingml/2006/main srgbClr,omitempty"`
	HslClr    *HslClr             `xml:"http://schemas.openxmlformats.org/drawingml/2006/main hslClr,omitempty"`
	SysClr    *SystemClr          `xml:"http://schemas.openxmlformats.org/drawingml/2006/main sysClr,omitempty"`
	SchemeClr *SchemeClrTransform `xml:"http://schemas.openxmlformats.org/drawingml/2006/main schemeClr,omitempty"`
	PrstClr   *PrstClr            `xml:"http://schemas.openxmlformats.org/drawingml/2006/main prstClr,omitempty"`
}

// EffectContainer represents CT_EffectContainer (a:cont). The XSD content is a
// repeated xs:choice over ~30 effect kinds; only Blur and a nested Cont are
// typed. Every other child is captured raw and replayed in document order so a
// re-marshal never deletes an unmodeled effect (e.g. a:alphaModFix): typed
// dispatch must never be lossier than raw capture (see BlipEffect).
type EffectContainer struct {
	Type string   `xml:"-"`
	Name string   `xml:"-"`
	Blur *BlurXML `xml:"-"` // first a:blur child; also settable programmatically
	// Children holds the ordered effect children (typed Blur/Cont plus raw
	// captures of every other kind); nil when built only via the Blur field.
	Children []*effectChild `xml:"-"`
}

// effectChild is one child of a CT_EffectContainer (a:cont / a:effectDag).
// Blur and Cont are the typed kinds; RawName/RawAttrs/Raw preserve any other
// effect kind verbatim (mirrors BlipEffect).
type effectChild struct {
	Blur     *BlurXML
	Cont     *EffectContainer
	RawName  xml.Name
	RawAttrs []xml.Attr
	Raw      []byte // inner XML of an unmodeled child
}

// UnmarshalXML captures the type/name attributes and decodes the effect
// children, keeping unmodeled kinds as raw bytes in document order.
func (ec *EffectContainer) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		if attr.Name.Space != "" {
			continue
		}
		switch attr.Name.Local {
		case "type":
			ec.Type = attr.Value
		case "name":
			ec.Name = attr.Value
		}
	}
	children, err := decodeEffectChildren(d)
	if err != nil {
		return err
	}
	ec.Children = children
	ec.Blur = firstBlur(children)
	return nil
}

// effectiveChildren returns the captured child list, or a synthesized single
// blur child when the container was built programmatically via the Blur field.
func (ec *EffectContainer) effectiveChildren() []*effectChild {
	if len(ec.Children) > 0 {
		return ec.Children
	}
	if ec.Blur != nil {
		return []*effectChild{{Blur: ec.Blur}}
	}
	return nil
}

// MarshalToBuilder implements xmlb.BuilderMarshaler, emitting the effect
// children in their captured document order.
func (ec *EffectContainer) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	marshalEffectContainer(b, ns, localName, ec.Type, ec.Name, ec.effectiveChildren())
}

// MarshalXML implements xml.Marshaler for the encoding/xml path.
func (ec *EffectContainer) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	return encodeEffectContainer(e, start, ec.Type, ec.Name, ec.effectiveChildren())
}

// decodeEffectChildren reads the children of a CT_EffectContainer, decoding
// a:blur and a:cont into typed fields and capturing everything else raw.
func decodeEffectChildren(d *xml.Decoder) ([]*effectChild, error) {
	var children []*effectChild
	for {
		tok, err := d.Token()
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			c := &effectChild{}
			switch t.Name.Local {
			case "blur":
				c.Blur = &BlurXML{}
				if err := d.DecodeElement(c.Blur, &t); err != nil {
					return nil, err
				}
			case "cont":
				c.Cont = &EffectContainer{}
				if err := d.DecodeElement(c.Cont, &t); err != nil {
					return nil, err
				}
			default:
				c.RawName = t.Name
				c.RawAttrs = append([]xml.Attr(nil), t.Attr...)
				var inner struct {
					Content []byte `xml:",innerxml"`
				}
				if err := d.DecodeElement(&inner, &t); err != nil {
					return nil, err
				}
				c.Raw = inner.Content
			}
			children = append(children, c)
		case xml.EndElement:
			return children, nil
		}
	}
}

// firstBlur returns the first typed a:blur child, or nil.
func firstBlur(children []*effectChild) *BlurXML {
	for _, c := range children {
		if c.Blur != nil {
			return c.Blur
		}
	}
	return nil
}

// firstCont returns the first typed a:cont child, or nil.
func firstCont(children []*effectChild) *EffectContainer {
	for _, c := range children {
		if c.Cont != nil {
			return c.Cont
		}
	}
	return nil
}

// marshalEffectContainer emits a CT_EffectContainer element (a:cont /
// a:effectDag) with its type/name attributes and ordered children.
func marshalEffectContainer(b *xmlb.Builder, ns, localName, typ, name string, children []*effectChild) {
	var attrs []xmlb.Attr
	if typ != "" {
		attrs = append(attrs, xmlb.StrAttr("type", typ))
	}
	if name != "" {
		attrs = append(attrs, xmlb.StrAttr("name", name))
	}
	if len(children) == 0 {
		b.EmptyElement(ns, localName, attrs...)
		return
	}
	b.StartElement(ns, localName, attrs...)
	for _, c := range children {
		c.marshalToBuilder(b, ns)
	}
	b.EndElement(ns, localName)
}

// encodeEffectContainer is the encoding/xml counterpart of
// marshalEffectContainer.
func encodeEffectContainer(e *xml.Encoder, start xml.StartElement, typ, name string, children []*effectChild) error {
	if typ != "" {
		start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "type"}, Value: typ})
	}
	if name != "" {
		start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "name"}, Value: name})
	}
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	for _, c := range children {
		if err := c.marshalXML(e); err != nil {
			return err
		}
	}
	return e.EncodeToken(start.End())
}

// marshalToBuilder emits one effect child: the typed Blur/Cont, or the raw
// capture replayed with its xmlns declarations restored so its prefixes stay
// bound (mirrors BlipEffect.marshalToBuilder).
func (c *effectChild) marshalToBuilder(b *xmlb.Builder, ns string) {
	switch {
	case c.Blur != nil:
		b.MarshalElement(ns, "blur", c.Blur)
		return
	case c.Cont != nil:
		b.MarshalElement(ns, "cont", c.Cont)
		return
	}
	if c.RawName.Local == "" {
		return
	}
	rns := c.RawName.Space
	if rns == "" {
		rns = ns
	}
	var attrs []xmlb.Attr
	inlinePrefix := ""
	hasInline := false
	for _, a := range c.RawAttrs {
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
		if len(c.Raw) == 0 {
			b.EmptyElementInlineNS(rns, inlinePrefix, c.RawName.Local, attrs...)
			return
		}
		b.StartElementInlineNS(rns, inlinePrefix, c.RawName.Local, attrs...)
		b.WriteRaw(c.Raw)
		b.EndElementInlineNS(inlinePrefix, c.RawName.Local)
		b.ResetNamespaceDeclaration(rns)
		return
	}
	if len(c.Raw) == 0 {
		b.EmptyElement(rns, c.RawName.Local, attrs...)
		return
	}
	b.StartElement(rns, c.RawName.Local, attrs...)
	b.WriteRaw(c.Raw)
	b.EndElement(rns, c.RawName.Local)
}

// marshalXML is the encoding/xml counterpart of effectChild.marshalToBuilder.
func (c *effectChild) marshalXML(e *xml.Encoder) error {
	switch {
	case c.Blur != nil:
		return e.EncodeElement(c.Blur, xml.StartElement{Name: xml.Name{Space: NsDrawingML, Local: "blur"}})
	case c.Cont != nil:
		return e.EncodeElement(c.Cont, xml.StartElement{Name: xml.Name{Space: NsDrawingML, Local: "cont"}})
	}
	if c.RawName.Local == "" {
		return nil
	}
	elem := xml.StartElement{Name: c.RawName}
	for _, a := range c.RawAttrs {
		if a.Name.Space == "xmlns" || (a.Name.Space == "" && a.Name.Local == "xmlns") {
			continue
		}
		elem.Attr = append(elem.Attr, a)
	}
	if err := e.EncodeToken(elem); err != nil {
		return err
	}
	if len(c.Raw) > 0 {
		sub := xml.NewDecoder(bytes.NewReader(c.Raw))
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

// EffectRef represents CT_EffectReference (a:effect)
type EffectRef struct {
	Ref string `xml:"ref,attr"`
}

// AlphaMod represents CT_AlphaModulateEffect (a:alphaMod)
type AlphaMod struct {
	Cont *EffectContainer `xml:"http://schemas.openxmlformats.org/drawingml/2006/main cont,omitempty"`
}

// AlphaModFix represents CT_AlphaModulateFixedEffect (a:alphaModFix). The amt
// attribute defaults to 100000 (100%) in the XSD, so it is a pointer: an
// explicit amt="0" (0%) must be emitted rather than omitted. A non-pointer
// Percentage with omitempty would drop the strict integer form "0" (whose Val
// is 0 and orig is empty, so IsZeroAttr reports zero), silently reinterpreting
// an explicit 0% as the 100% default.
type AlphaModFix struct {
	Amt *Percentage `xml:"amt,attr,omitempty"`
}

// BiLevelXML represents CT_BiLevelEffect (a:biLevel)
type BiLevelXML struct {
	Thresh Percentage `xml:"thresh,attr"`
}

// ClrChange represents CT_ColorChangeEffect (a:clrChange). useA defaults to
// true in the XSD, so it is a pointer: an explicit useA="0" must be emitted
// rather than omitted (which readers treat as true).
type ClrChange struct {
	UseA    *bool        `xml:"useA,attr,omitempty"`
	ClrFrom *ColorChoice `xml:"http://schemas.openxmlformats.org/drawingml/2006/main clrFrom,omitempty"`
	ClrTo   *ColorChoice `xml:"http://schemas.openxmlformats.org/drawingml/2006/main clrTo,omitempty"`
}

// ClrRepl represents CT_ColorReplaceEffect (a:clrRepl). All six
// EG_ColorChoice kinds are modeled; see InnerShdw.
type ClrRepl struct {
	ScRgbClr  *ScRgbClr           `xml:"http://schemas.openxmlformats.org/drawingml/2006/main scrgbClr,omitempty"`
	SrgbClr   *SrgbClr            `xml:"http://schemas.openxmlformats.org/drawingml/2006/main srgbClr,omitempty"`
	HslClr    *HslClr             `xml:"http://schemas.openxmlformats.org/drawingml/2006/main hslClr,omitempty"`
	SysClr    *SystemClr          `xml:"http://schemas.openxmlformats.org/drawingml/2006/main sysClr,omitempty"`
	SchemeClr *SchemeClrTransform `xml:"http://schemas.openxmlformats.org/drawingml/2006/main schemeClr,omitempty"`
	PrstClr   *PrstClr            `xml:"http://schemas.openxmlformats.org/drawingml/2006/main prstClr,omitempty"`
}

// clrChoiceKind identifies an EG_ColorChoice element kind.
type clrChoiceKind int

const (
	ccScRgbClr clrChoiceKind = iota
	ccSrgbClr
	ccHslClr
	ccSysClr
	ccSchemeClr
	ccPrstClr
)

// clrChoiceKindName maps a color choice kind to its element local name.
var clrChoiceKindName = map[clrChoiceKind]string{
	ccScRgbClr: "scrgbClr", ccSrgbClr: "srgbClr", ccHslClr: "hslClr",
	ccSysClr: "sysClr", ccSchemeClr: "schemeClr", ccPrstClr: "prstClr",
}

// clrChoiceRef references a color by kind and index within its per-kind slice.
type clrChoiceRef struct {
	kind  clrChoiceKind
	index int
}

// Duotone represents CT_DuotoneEffect (a:duotone), exactly two EG_ColorChoice
// colors given as direct children. The two colors are POSITIONAL (dark then
// light), so custom unmarshal/marshal preserves their cross-kind document
// order: a <a:prstClr/><a:srgbClr/> pair must not re-emit inverted.
type Duotone struct {
	ScRgbClr  []*ScRgbClr           `xml:"-"`
	SrgbClr   []*SrgbClr            `xml:"-"`
	HslClr    []*HslClr             `xml:"-"`
	SysClr    []*SystemClr          `xml:"-"`
	SchemeClr []*SchemeClrTransform `xml:"-"`
	PrstClr   []*PrstClr            `xml:"-"`
	clrOrder  []clrChoiceRef        // tracks interleaved color order
}

// UnmarshalXML implements custom unmarshaling for Duotone, preserving the
// document order of its color children across kinds.
func (v *Duotone) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "scrgbClr":
				c := &ScRgbClr{}
				if err := d.DecodeElement(c, &t); err != nil {
					return err
				}
				v.clrOrder = append(v.clrOrder, clrChoiceRef{ccScRgbClr, len(v.ScRgbClr)})
				v.ScRgbClr = append(v.ScRgbClr, c)
			case "srgbClr":
				c := &SrgbClr{}
				if err := d.DecodeElement(c, &t); err != nil {
					return err
				}
				v.clrOrder = append(v.clrOrder, clrChoiceRef{ccSrgbClr, len(v.SrgbClr)})
				v.SrgbClr = append(v.SrgbClr, c)
			case "hslClr":
				c := &HslClr{}
				if err := d.DecodeElement(c, &t); err != nil {
					return err
				}
				v.clrOrder = append(v.clrOrder, clrChoiceRef{ccHslClr, len(v.HslClr)})
				v.HslClr = append(v.HslClr, c)
			case "sysClr":
				c := &SystemClr{}
				if err := d.DecodeElement(c, &t); err != nil {
					return err
				}
				v.clrOrder = append(v.clrOrder, clrChoiceRef{ccSysClr, len(v.SysClr)})
				v.SysClr = append(v.SysClr, c)
			case "schemeClr":
				c := &SchemeClrTransform{}
				if err := d.DecodeElement(c, &t); err != nil {
					return err
				}
				v.clrOrder = append(v.clrOrder, clrChoiceRef{ccSchemeClr, len(v.SchemeClr)})
				v.SchemeClr = append(v.SchemeClr, c)
			case "prstClr":
				c := &PrstClr{}
				if err := d.DecodeElement(c, &t); err != nil {
					return err
				}
				v.clrOrder = append(v.clrOrder, clrChoiceRef{ccPrstClr, len(v.PrstClr)})
				v.PrstClr = append(v.PrstClr, c)
			default:
				if err := d.Skip(); err != nil {
					return err
				}
			}
		case xml.EndElement:
			return nil
		}
	}
}

// colorForRef returns the color value referenced by ref, or nil when out of range.
func (v *Duotone) colorForRef(ref clrChoiceRef) interface{} {
	switch ref.kind {
	case ccScRgbClr:
		if ref.index < len(v.ScRgbClr) {
			return v.ScRgbClr[ref.index]
		}
	case ccSrgbClr:
		if ref.index < len(v.SrgbClr) {
			return v.SrgbClr[ref.index]
		}
	case ccHslClr:
		if ref.index < len(v.HslClr) {
			return v.HslClr[ref.index]
		}
	case ccSysClr:
		if ref.index < len(v.SysClr) {
			return v.SysClr[ref.index]
		}
	case ccSchemeClr:
		if ref.index < len(v.SchemeClr) {
			return v.SchemeClr[ref.index]
		}
	case ccPrstClr:
		if ref.index < len(v.PrstClr) {
			return v.PrstClr[ref.index]
		}
	}
	return nil
}

// groupedRefs builds refs in grouped XSD order, the fallback when no document
// order was captured (e.g. a programmatically built Duotone).
func (v *Duotone) groupedRefs() []clrChoiceRef {
	var refs []clrChoiceRef
	for i := range v.ScRgbClr {
		refs = append(refs, clrChoiceRef{ccScRgbClr, i})
	}
	for i := range v.SrgbClr {
		refs = append(refs, clrChoiceRef{ccSrgbClr, i})
	}
	for i := range v.HslClr {
		refs = append(refs, clrChoiceRef{ccHslClr, i})
	}
	for i := range v.SysClr {
		refs = append(refs, clrChoiceRef{ccSysClr, i})
	}
	for i := range v.SchemeClr {
		refs = append(refs, clrChoiceRef{ccSchemeClr, i})
	}
	for i := range v.PrstClr {
		refs = append(refs, clrChoiceRef{ccPrstClr, i})
	}
	return refs
}

// orderedRefs returns the captured document order, or the grouped fallback.
func (v *Duotone) orderedRefs() []clrChoiceRef {
	if len(v.clrOrder) > 0 {
		return v.clrOrder
	}
	return v.groupedRefs()
}

// MarshalToBuilder implements xmlb.BuilderMarshaler, emitting the two colors
// in their original document order.
func (v *Duotone) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	refs := v.orderedRefs()
	if len(refs) == 0 {
		b.EmptyElement(ns, localName)
		return
	}
	b.StartElement(ns, localName)
	for _, ref := range refs {
		if c := v.colorForRef(ref); c != nil {
			b.MarshalElement(ns, clrChoiceKindName[ref.kind], c)
		}
	}
	b.EndElement(ns, localName)
}

// MarshalXML implements xml.Marshaler for Duotone (encoding/xml path),
// mirroring MarshalToBuilder.
func (v *Duotone) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	for _, ref := range v.orderedRefs() {
		c := v.colorForRef(ref)
		if c == nil {
			continue
		}
		name := xml.Name{Space: NsDrawingML, Local: clrChoiceKindName[ref.kind]}
		if err := e.EncodeElement(c, xml.StartElement{Name: name}); err != nil {
			return err
		}
	}
	return e.EncodeToken(start.End())
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
	Hue int32      `xml:"hue,attr,omitempty"`
	Sat Percentage `xml:"sat,attr,omitempty"`
	Lum Percentage `xml:"lum,attr,omitempty"`
}

// LumXML represents CT_LuminanceEffect (a:lum)
type LumXML struct {
	Bright   Percentage `xml:"bright,attr,omitempty"`
	Contrast Percentage `xml:"contrast,attr,omitempty"`
}

// TintEffectXML represents CT_TintEffect (a:tint)
type TintEffectXML struct {
	Hue int32      `xml:"hue,attr,omitempty"`
	Amt Percentage `xml:"amt,attr,omitempty"`
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

// AlphaInv represents CT_AlphaInverseEffect (a:alphaInv). All six
// EG_ColorChoice kinds are modeled; see InnerShdw.
type AlphaInv struct {
	ScRgbClr  *ScRgbClr           `xml:"http://schemas.openxmlformats.org/drawingml/2006/main scrgbClr,omitempty"`
	SrgbClr   *SrgbClr            `xml:"http://schemas.openxmlformats.org/drawingml/2006/main srgbClr,omitempty"`
	HslClr    *HslClr             `xml:"http://schemas.openxmlformats.org/drawingml/2006/main hslClr,omitempty"`
	SysClr    *SystemClr          `xml:"http://schemas.openxmlformats.org/drawingml/2006/main sysClr,omitempty"`
	SchemeClr *SchemeClrTransform `xml:"http://schemas.openxmlformats.org/drawingml/2006/main schemeClr,omitempty"`
	PrstClr   *PrstClr            `xml:"http://schemas.openxmlformats.org/drawingml/2006/main prstClr,omitempty"`
}

// AlphaRepl represents CT_AlphaReplaceEffect (a:alphaRepl)
type AlphaRepl struct {
	A int32 `xml:"a,attr"`
}

// EffectDag represents CT_EffectContainer (a:effectDag). Same content model as
// EffectContainer; see it for the raw-capture rationale. Unmodeled effect
// children are preserved instead of being dropped on re-marshal.
type EffectDag struct {
	Type string           `xml:"-"`
	Name string           `xml:"-"`
	Cont *EffectContainer `xml:"-"` // first a:cont child; also settable programmatically
	// Children holds the ordered effect children (typed Blur/Cont plus raw
	// captures); nil when built only via the Cont field.
	Children []*effectChild `xml:"-"`
}

// UnmarshalXML captures the type/name attributes and decodes the effect
// children, keeping unmodeled kinds as raw bytes in document order.
func (ed *EffectDag) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		if attr.Name.Space != "" {
			continue
		}
		switch attr.Name.Local {
		case "type":
			ed.Type = attr.Value
		case "name":
			ed.Name = attr.Value
		}
	}
	children, err := decodeEffectChildren(d)
	if err != nil {
		return err
	}
	ed.Children = children
	ed.Cont = firstCont(children)
	return nil
}

// effectiveChildren returns the captured child list, or a synthesized single
// cont child when the dag was built programmatically via the Cont field.
func (ed *EffectDag) effectiveChildren() []*effectChild {
	if len(ed.Children) > 0 {
		return ed.Children
	}
	if ed.Cont != nil {
		return []*effectChild{{Cont: ed.Cont}}
	}
	return nil
}

// MarshalToBuilder implements xmlb.BuilderMarshaler, emitting the effect
// children in their captured document order.
func (ed *EffectDag) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	marshalEffectContainer(b, ns, localName, ed.Type, ed.Name, ed.effectiveChildren())
}

// MarshalXML implements xml.Marshaler for the encoding/xml path.
func (ed *EffectDag) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	return encodeEffectContainer(e, start, ed.Type, ed.Name, ed.effectiveChildren())
}
