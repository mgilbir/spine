package diagram

import (
	"encoding/xml"

	"github.com/mgilbir/spine/common/dml"
	xmlb "github.com/mgilbir/spine/common/xml"
)

// CT_Colors (dgm:fillClrLst and its five siblings) is a repeated
// a:EG_ColorChoice whose children are POSITIONAL: the color transform walks
// them in order and applies meth/hueDir across the sequence, so both the set of
// kinds and their order are load-bearing. Modeling only srgbClr and schemeClr
// deleted the other four kinds outright and regrouped what was left by kind —
// the same defect shape as the theme's fill style matrix (C401), one schema
// over. All six kinds are modeled and the cross-kind document order is captured
// and replayed; a programmatically built list falls back to grouped order.

// clrKind identifies an a:EG_ColorChoice element kind.
type clrKind int

const (
	kindScRgbClr clrKind = iota
	kindSrgbClr
	kindHslClr
	kindSysClr
	kindSchemeClr
	kindPrstClr
)

// clrKindName maps a color kind to its element local name.
var clrKindName = map[clrKind]string{
	kindScRgbClr: "scrgbClr", kindSrgbClr: "srgbClr", kindHslClr: "hslClr",
	kindSysClr: "sysClr", kindSchemeClr: "schemeClr", kindPrstClr: "prstClr",
}

// clrNameKind is the reverse of clrKindName.
var clrNameKind = map[string]clrKind{
	"scrgbClr": kindScRgbClr, "srgbClr": kindSrgbClr, "hslClr": kindHslClr,
	"sysClr": kindSysClr, "schemeClr": kindSchemeClr, "prstClr": kindPrstClr,
}

// clrRef references a color by kind and index within its per-kind slice.
type clrRef struct {
	kind  clrKind
	index int
}

// length returns how many colors the list holds for a kind.
func (c *ColorList) length(kind clrKind) int {
	switch kind {
	case kindScRgbClr:
		return len(c.ScRgbClr)
	case kindSrgbClr:
		return len(c.SrgbClr)
	case kindHslClr:
		return len(c.HslClr)
	case kindSysClr:
		return len(c.SysClr)
	case kindSchemeClr:
		return len(c.SchemeClr)
	case kindPrstClr:
		return len(c.PrstClr)
	}
	return 0
}

// at returns the color referenced by ref, or nil when out of range.
func (c *ColorList) at(ref clrRef) interface{} {
	switch ref.kind {
	case kindScRgbClr:
		if ref.index < len(c.ScRgbClr) {
			return c.ScRgbClr[ref.index]
		}
	case kindSrgbClr:
		if ref.index < len(c.SrgbClr) {
			return c.SrgbClr[ref.index]
		}
	case kindHslClr:
		if ref.index < len(c.HslClr) {
			return c.HslClr[ref.index]
		}
	case kindSysClr:
		if ref.index < len(c.SysClr) {
			return c.SysClr[ref.index]
		}
	case kindSchemeClr:
		if ref.index < len(c.SchemeClr) {
			return c.SchemeClr[ref.index]
		}
	case kindPrstClr:
		if ref.index < len(c.PrstClr) {
			return c.PrstClr[ref.index]
		}
	}
	return nil
}

// decodeColor decodes one color of the given kind into its slice.
func (c *ColorList) decodeColor(d *xml.Decoder, kind clrKind, start *xml.StartElement) error {
	switch kind {
	case kindScRgbClr:
		v := &dml.ScRgbClr{}
		if err := d.DecodeElement(v, start); err != nil {
			return err
		}
		c.ScRgbClr = append(c.ScRgbClr, v)
	case kindSrgbClr:
		v := &dml.SrgbClr{}
		if err := d.DecodeElement(v, start); err != nil {
			return err
		}
		c.SrgbClr = append(c.SrgbClr, v)
	case kindHslClr:
		v := &dml.HslClr{}
		if err := d.DecodeElement(v, start); err != nil {
			return err
		}
		c.HslClr = append(c.HslClr, v)
	case kindSysClr:
		v := &dml.SystemClr{}
		if err := d.DecodeElement(v, start); err != nil {
			return err
		}
		c.SysClr = append(c.SysClr, v)
	case kindSchemeClr:
		v := &dml.SchemeClrTransform{}
		if err := d.DecodeElement(v, start); err != nil {
			return err
		}
		c.SchemeClr = append(c.SchemeClr, v)
	case kindPrstClr:
		v := &dml.PrstClr{}
		if err := d.DecodeElement(v, start); err != nil {
			return err
		}
		c.PrstClr = append(c.PrstClr, v)
	}
	return nil
}

// UnmarshalXML decodes a CT_Colors element, preserving the cross-kind document
// order of its colors.
func (c *ColorList) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		if attr.Name.Space != "" && attr.Name.Space != NsDiagram {
			continue
		}
		switch attr.Name.Local {
		case "meth":
			c.Meth = attr.Value
		case "hueDir":
			c.HueDir = attr.Value
		}
	}
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			kind, ok := clrNameKind[t.Name.Local]
			if !ok {
				if err := d.Skip(); err != nil {
					return err
				}
				continue
			}
			c.clrOrder = append(c.clrOrder, clrRef{kind, c.length(kind)})
			if err := c.decodeColor(d, kind, &t); err != nil {
				return err
			}
		case xml.EndElement:
			return nil
		}
	}
}

// groupedRefs builds refs in grouped XSD order, the fallback for a
// programmatically built list.
func (c *ColorList) groupedRefs() []clrRef {
	var refs []clrRef
	for _, kind := range []clrKind{kindScRgbClr, kindSrgbClr, kindHslClr, kindSysClr, kindSchemeClr, kindPrstClr} {
		for i := 0; i < c.length(kind); i++ {
			refs = append(refs, clrRef{kind, i})
		}
	}
	return refs
}

// orderedRefs returns the captured document order, extended with colors
// appended after parse so a programmatic addition is not dropped.
func (c *ColorList) orderedRefs() []clrRef {
	if len(c.clrOrder) == 0 {
		return c.groupedRefs()
	}
	seen := make(map[clrRef]bool, len(c.clrOrder))
	refs := make([]clrRef, 0, len(c.clrOrder))
	for _, ref := range c.clrOrder {
		if ref.index < c.length(ref.kind) {
			refs = append(refs, ref)
			seen[ref] = true
		}
	}
	for _, ref := range c.groupedRefs() {
		if !seen[ref] {
			refs = append(refs, ref)
		}
	}
	return refs
}

// MarshalXML implements xml.Marshaler, emitting the colors in document order.
func (c ColorList) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	if c.Meth != "" {
		start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "meth"}, Value: c.Meth})
	}
	if c.HueDir != "" {
		start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "hueDir"}, Value: c.HueDir})
	}
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	for _, ref := range (&c).orderedRefs() {
		v := (&c).at(ref)
		if v == nil {
			continue
		}
		name := xml.Name{Space: dml.NsDrawingML, Local: clrKindName[ref.kind]}
		if err := e.EncodeElement(v, xml.StartElement{Name: name}); err != nil {
			return err
		}
	}
	return e.EncodeToken(start.End())
}

// MarshalToBuilder implements xmlb.BuilderMarshaler, emitting the colors in
// document order.
func (c ColorList) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	var attrs []xmlb.Attr
	if c.Meth != "" {
		attrs = append(attrs, xmlb.StrAttr("meth", c.Meth))
	}
	if c.HueDir != "" {
		attrs = append(attrs, xmlb.StrAttr("hueDir", c.HueDir))
	}
	refs := (&c).orderedRefs()
	if len(refs) == 0 {
		b.EmptyElement(ns, localName, attrs...)
		return
	}
	b.StartElement(ns, localName, attrs...)
	for _, ref := range refs {
		if v := (&c).at(ref); v != nil {
			b.MarshalElement(dml.NsDrawingML, clrKindName[ref.kind], v)
		}
	}
	b.EndElement(ns, localName)
}
