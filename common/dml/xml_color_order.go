package dml

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"strconv"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// This file gives the five pointer-based color types (SrgbClr, SystemClr,
// HslClr, PrstClr, ScRgbClr) the same source-order fidelity that
// SchemeClrTransform gets from its xfOrder capture: EG_ColorTransform is an
// xs:choice whose children apply IN ORDER, and producers freely interleave
// them (e.g. <a:gamma/><a:shade .../><a:invGamma/>), so re-emitting them in
// struct-field order both drifts bytes and changes the rendered color. Each
// type keeps its single-occurrence pointer fields; parsing records the
// transform order and marshaling replays it, falling back to field order for
// programmatically built values.

// clrXfSlot references one single-occurrence transform field of a
// pointer-based color type. Exactly one of val/empty is non-nil, matching the
// XSD: valued transforms carry a val attribute, arg-less transforms (comp,
// inv, gray, gamma, invGamma) have EMPTY complex types.
type clrXfSlot struct {
	kind  clrTransformKind
	val   **ColorTransform
	empty **EmptyClrTransform
}

// isSet reports whether the slot's field holds a parsed transform.
func (s clrXfSlot) isSet() bool {
	if s.val != nil {
		return *s.val != nil
	}
	return *s.empty != nil
}

// unmarshalClrColor parses a color element: attributes via setAttr, transform
// children into slots, recording their document order in *order.
// clrRawKindBase marks order entries that reference a raw-preserved
// duplicate transform: kind clrRawKindBase+i replays raws[i] verbatim.
const clrRawKindBase clrTransformKind = 1 << 16

func unmarshalClrColor(d *xml.Decoder, start xml.StartElement, setAttr func(xml.Attr) error, slots []clrXfSlot, order *[]clrTransformKind, raws *[][]byte) error {
	for _, attr := range start.Attr {
		if err := setAttr(attr); err != nil {
			return err
		}
	}
	byKind := make(map[clrTransformKind]clrXfSlot, len(slots))
	for _, s := range slots {
		byKind[s.kind] = s
	}
	for {
		pre := d.InputOffset()
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			kind, ok := clrTransformNameMap[t.Name.Local]
			if !ok {
				// An unmodeled child (a future transform, an mc:AlternateContent
				// wrapper, an extension element): preserve it verbatim at its
				// position rather than skipping it. Typed dispatch must never be
				// lossier than raw capture — the package's own rule, which the
				// bare d.Skip here violated.
				if err := d.Skip(); err != nil {
					return err
				}
				if raw := xmlb.RawTokenBytes(d, pre); raw != nil {
					*order = append(*order, clrRawKindBase+clrTransformKind(len(*raws)))
					*raws = append(*raws, bytes.Clone(raw))
				}
				continue
			}
			slot := byKind[kind]
			if slot.isSet() {
				// A duplicated transform kind: the single-occurrence model
				// keeps the first value; preserve the duplicate verbatim at
				// its position (first-wins matches the order replay, which
				// previously silently dropped the repeat).
				if err := d.Skip(); err != nil {
					return err
				}
				if raw := xmlb.RawTokenBytes(d, pre); raw != nil {
					// Clone: retaining a sub-slice of the part source would pin
					// the whole part in memory for the model's lifetime (C282).
					*order = append(*order, clrRawKindBase+clrTransformKind(len(*raws)))
					*raws = append(*raws, bytes.Clone(raw))
				}
				continue
			}
			if slot.val != nil {
				ct := &ColorTransform{}
				if err := d.DecodeElement(ct, &t); err != nil {
					return err
				}
				*slot.val = ct
			} else {
				if err := d.Skip(); err != nil {
					return err
				}
				*slot.empty = &EmptyClrTransform{}
			}
			*order = append(*order, kind)
		case xml.EndElement:
			return nil
		}
	}
}

// marshalClrColor writes a color element with its attributes and transform
// children, replaying the captured source order when present.
func marshalClrColor(b *xmlb.Builder, ns, localName string, attrs []xmlb.Attr, slots []clrXfSlot, order []clrTransformKind, raws [][]byte) {
	any := len(raws) > 0
	for _, s := range slots {
		if s.isSet() {
			any = true
			break
		}
	}
	if !any {
		b.EmptyElement(ns, localName, attrs...)
		return
	}
	b.StartElement(ns, localName, attrs...)
	if len(order) > 0 {
		byKind := make(map[clrTransformKind]clrXfSlot, len(slots))
		for _, s := range slots {
			byKind[s.kind] = s
		}
		for _, kind := range order {
			if kind >= clrRawKindBase {
				if i := int(kind - clrRawKindBase); i < len(raws) {
					b.WriteRaw(raws[i])
				}
				continue
			}
			writeClrXfSlot(b, ns, byKind[kind])
		}
	} else {
		for _, s := range slots {
			if s.isSet() {
				writeClrXfSlot(b, ns, s)
			}
		}
	}
	b.EndElement(ns, localName)
}

// writeClrXfSlot writes one transform element for a set slot; unset slots
// (e.g. an order entry whose field was cleared) are skipped.
func writeClrXfSlot(b *xmlb.Builder, ns string, s clrXfSlot) {
	if s.val == nil && s.empty == nil {
		// Zero slot (an order entry with no matching field); nothing to write.
		return
	}
	name := clrTransformKindName[s.kind]
	if s.val != nil {
		if *s.val == nil {
			return
		}
		b.EmptyElement(ns, name, xmlb.StrAttr("val", (*s.val).Val.AttrValue()))
		return
	}
	if *s.empty == nil {
		return
	}
	b.EmptyElement(ns, name)
}

// encodeClrColor is the encoding/xml counterpart of marshalClrColor: it writes
// the color element's attributes and its transform children in the captured
// source order, replaying raw-preserved duplicates as tokens.
//
// Without it these five types would fall back to struct-tag marshaling on the
// stdlib path — dropping every captured duplicate transform (xfRaws is
// unreachable from tags) and re-emitting the rest in field order rather than
// document order, which changes the rendered color. The Builder is the
// production serializer, so this exists to stop the spectest and diagram
// round-trip suites from asserting a fidelity the stdlib path did not have
// (the same asymmetry C341 fixed for Ext).
func encodeClrColor(e *xml.Encoder, start xml.StartElement, attrs []xml.Attr, slots []clrXfSlot, order []clrTransformKind, raws [][]byte) error {
	start.Attr = append(start.Attr, attrs...)
	any := len(raws) > 0
	for _, s := range slots {
		if s.isSet() {
			any = true
			break
		}
	}
	if !any {
		return e.EncodeElement(struct{}{}, start)
	}
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	emit := func(s clrXfSlot) error {
		if s.val == nil && s.empty == nil {
			return nil
		}
		name := xml.Name{Local: clrTransformKindName[s.kind]}
		if s.val != nil {
			if *s.val == nil {
				return nil
			}
			return e.EncodeElement(struct{}{}, xml.StartElement{Name: name,
				Attr: []xml.Attr{{Name: xml.Name{Local: "val"}, Value: (*s.val).Val.AttrValue()}}})
		}
		if *s.empty == nil {
			return nil
		}
		return e.EncodeElement(struct{}{}, xml.StartElement{Name: name})
	}
	if len(order) > 0 {
		byKind := make(map[clrTransformKind]clrXfSlot, len(slots))
		for _, s := range slots {
			byKind[s.kind] = s
		}
		for _, kind := range order {
			if kind >= clrRawKindBase {
				i := int(kind - clrRawKindBase)
				if i < len(raws) {
					if err := encodeRawFragment(e, raws[i]); err != nil {
						return err
					}
				}
				continue
			}
			if err := emit(byKind[kind]); err != nil {
				return err
			}
		}
	} else {
		for _, s := range slots {
			if s.isSet() {
				if err := emit(s); err != nil {
					return err
				}
			}
		}
	}
	return e.EncodeToken(start.End())
}

// encodeRawFragment replays captured raw bytes as tokens: encoding/xml has no
// raw-write API (mirrors BlipEffect.marshalXML). A captured transform is always
// a DrawingML sibling of the color element, and its "a:" prefix is bound
// outside the fragment, so the unresolvable prefix is dropped and the element
// inherits the enclosing namespace rather than being emitted with a bogus
// xmlns="a".
func encodeRawFragment(e *xml.Encoder, raw []byte) error {
	sub := xml.NewDecoder(bytes.NewReader(raw))
	for {
		tok, err := sub.Token()
		if err != nil {
			return nil // truncated fragment: stop replaying, do not fail the marshal
		}
		if err := e.EncodeToken(stripFragmentNS(fixupRawToken(tok))); err != nil {
			return err
		}
	}
}

// stripFragmentNS clears the namespace Go's decoder derived from an unbound
// prefix on a re-tokenized fragment.
func stripFragmentNS(tok xml.Token) xml.Token {
	switch t := tok.(type) {
	case xml.StartElement:
		t.Name.Space = ""
		for i := range t.Attr {
			t.Attr[i].Name.Space = ""
		}
		return t
	case xml.EndElement:
		t.Name.Space = ""
		return t
	}
	return tok
}

// srgbSlots returns the transform slots of an SrgbClr in field order.
func (c *SrgbClr) srgbSlots() []clrXfSlot {
	return []clrXfSlot{
		{ctTint, &c.Tint, nil}, {ctShade, &c.Shade, nil}, {ctSatMod, &c.SatMod, nil},
		{ctAlpha, &c.Alpha, nil}, {ctLumMod, &c.LumMod, nil}, {ctLumOff, &c.LumOff, nil},
		{ctComp, nil, &c.Comp}, {ctInv, nil, &c.Inv}, {ctGray, nil, &c.Gray},
		{ctAlphaOff, &c.AlphaOff, nil}, {ctAlphaMod, &c.AlphaMod, nil},
		{ctHue, &c.Hue, nil}, {ctHueOff, &c.HueOff, nil}, {ctHueMod, &c.HueMod, nil},
		{ctSat, &c.Sat, nil}, {ctSatOff, &c.SatOff, nil},
		{ctLum, &c.Lum, nil},
		{ctRed, &c.Red, nil}, {ctRedOff, &c.RedOff, nil}, {ctRedMod, &c.RedMod, nil},
		{ctGreen, &c.Green, nil}, {ctGreenOff, &c.GreenOff, nil}, {ctGreenMod, &c.GreenMod, nil},
		{ctBlue, &c.Blue, nil}, {ctBlueOff, &c.BlueOff, nil}, {ctBlueMod, &c.BlueMod, nil},
		{ctGamma, nil, &c.Gamma}, {ctInvGamma, nil, &c.InvGamma},
	}
}

// UnmarshalXML implements xml.Unmarshaler, capturing transform order.
func (c *SrgbClr) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	return unmarshalClrColor(d, start, func(attr xml.Attr) error {
		if attr.Name.Local == "val" {
			c.Val = attr.Value
		}
		return nil
	}, c.srgbSlots(), &c.xfOrder, &c.xfRaws)
}

// MarshalXML implements xml.Marshaler, mirroring MarshalToBuilder on the
// encoding/xml path; see encodeClrColor.
func (c *SrgbClr) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	attrs := []xml.Attr{{Name: xml.Name{Local: "val"}, Value: c.Val}}
	return encodeClrColor(e, start, attrs, c.srgbSlots(), c.xfOrder, c.xfRaws)
}

// MarshalToBuilder implements xmlb.BuilderMarshaler, replaying transform order.
func (c *SrgbClr) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	marshalClrColor(b, ns, localName, []xmlb.Attr{xmlb.StrAttr("val", c.Val)}, c.srgbSlots(), c.xfOrder, c.xfRaws)
}

// sysSlots returns the transform slots of a SystemClr in field order.
func (c *SystemClr) sysSlots() []clrXfSlot {
	return []clrXfSlot{
		{ctTint, &c.Tint, nil}, {ctShade, &c.Shade, nil}, {ctSatMod, &c.SatMod, nil},
		{ctAlpha, &c.Alpha, nil}, {ctLumMod, &c.LumMod, nil}, {ctLumOff, &c.LumOff, nil},
		{ctComp, nil, &c.Comp}, {ctInv, nil, &c.Inv}, {ctGray, nil, &c.Gray},
		{ctAlphaOff, &c.AlphaOff, nil}, {ctAlphaMod, &c.AlphaMod, nil},
		{ctHue, &c.Hue, nil}, {ctHueOff, &c.HueOff, nil}, {ctHueMod, &c.HueMod, nil},
		{ctSat, &c.Sat, nil}, {ctSatOff, &c.SatOff, nil},
		{ctLum, &c.Lum, nil},
		{ctRed, &c.Red, nil}, {ctRedOff, &c.RedOff, nil}, {ctRedMod, &c.RedMod, nil},
		{ctGreen, &c.Green, nil}, {ctGreenOff, &c.GreenOff, nil}, {ctGreenMod, &c.GreenMod, nil},
		{ctBlue, &c.Blue, nil}, {ctBlueOff, &c.BlueOff, nil}, {ctBlueMod, &c.BlueMod, nil},
		{ctGamma, nil, &c.Gamma}, {ctInvGamma, nil, &c.InvGamma},
	}
}

// UnmarshalXML implements xml.Unmarshaler, capturing transform order.
func (c *SystemClr) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	c.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	return unmarshalClrColor(d, start, func(attr xml.Attr) error {
		switch attr.Name.Local {
		case "val":
			c.Val = attr.Value
		case "lastClr":
			c.LastClr = attr.Value
		}
		return nil
	}, c.sysSlots(), &c.xfOrder, &c.xfRaws)
}

// MarshalXML implements xml.Marshaler, mirroring MarshalToBuilder on the
// encoding/xml path; see encodeClrColor.
func (c *SystemClr) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	attrs := []xml.Attr{{Name: xml.Name{Local: "val"}, Value: c.Val}}
	if c.LastClr != "" {
		attrs = append(attrs, xml.Attr{Name: xml.Name{Local: "lastClr"}, Value: c.LastClr})
	}
	return encodeClrColor(e, start, attrs, c.sysSlots(), c.xfOrder, c.xfRaws)
}

// MarshalToBuilder implements xmlb.BuilderMarshaler, replaying transform order.
func (c *SystemClr) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	attrs := []xmlb.Attr{xmlb.StrAttr("val", c.Val)}
	if c.LastClr != "" {
		attrs = append(attrs, xmlb.StrAttr("lastClr", c.LastClr))
	}
	if c.CapturedAttrs != nil {
		attrs = b.ReplayCapturedAttrs(c.CapturedAttrs, attrs)
	}
	marshalClrColor(b, ns, localName, attrs, c.sysSlots(), c.xfOrder, c.xfRaws)
}

// hslSlots returns the transform slots of an HslClr in field order.
func (c *HslClr) hslSlots() []clrXfSlot {
	return []clrXfSlot{
		{ctTint, &c.Tint, nil}, {ctShade, &c.Shade, nil},
		{ctComp, nil, &c.Comp}, {ctInv, nil, &c.Inv}, {ctGray, nil, &c.Gray},
		{ctAlpha, &c.Alpha, nil}, {ctAlphaOff, &c.AlphaOff, nil}, {ctAlphaMod, &c.AlphaMod, nil},
		{ctHue, &c.HueXf, nil}, {ctHueOff, &c.HueOff, nil}, {ctHueMod, &c.HueMod, nil},
		{ctSat, &c.SatXf, nil}, {ctSatOff, &c.SatOff, nil}, {ctSatMod, &c.SatMod, nil},
		{ctLum, &c.LumXf, nil}, {ctLumOff, &c.LumOff, nil}, {ctLumMod, &c.LumMod, nil},
		{ctRed, &c.Red, nil}, {ctRedOff, &c.RedOff, nil}, {ctRedMod, &c.RedMod, nil},
		{ctGreen, &c.Green, nil}, {ctGreenOff, &c.GreenOff, nil}, {ctGreenMod, &c.GreenMod, nil},
		{ctBlue, &c.Blue, nil}, {ctBlueOff, &c.BlueOff, nil}, {ctBlueMod, &c.BlueMod, nil},
		{ctGamma, nil, &c.Gamma}, {ctInvGamma, nil, &c.InvGamma},
	}
}

// UnmarshalXML implements xml.Unmarshaler, capturing transform order.
func (c *HslClr) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	return unmarshalClrColor(d, start, func(attr xml.Attr) error {
		switch attr.Name.Local {
		case "hue":
			n, err := strconv.ParseInt(attr.Value, 10, 32)
			if err != nil {
				return fmt.Errorf("dml.HslClr: parsing hue %q: %w", attr.Value, err)
			}
			c.Hue = int32(n)
		case "sat":
			return c.Sat.UnmarshalXMLAttr(attr)
		case "lum":
			return c.Lum.UnmarshalXMLAttr(attr)
		}
		return nil
	}, c.hslSlots(), &c.xfOrder, &c.xfRaws)
}

// MarshalXML implements xml.Marshaler, mirroring MarshalToBuilder on the
// encoding/xml path; see encodeClrColor.
func (c *HslClr) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	attrs := []xml.Attr{
		{Name: xml.Name{Local: "hue"}, Value: strconv.FormatInt(int64(c.Hue), 10)},
		{Name: xml.Name{Local: "sat"}, Value: c.Sat.AttrValue()},
		{Name: xml.Name{Local: "lum"}, Value: c.Lum.AttrValue()},
	}
	return encodeClrColor(e, start, attrs, c.hslSlots(), c.xfOrder, c.xfRaws)
}

// MarshalToBuilder implements xmlb.BuilderMarshaler, replaying transform order.
func (c *HslClr) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	attrs := []xmlb.Attr{
		xmlb.Int32Attr("hue", c.Hue),
		xmlb.StrAttr("sat", c.Sat.AttrValue()),
		xmlb.StrAttr("lum", c.Lum.AttrValue()),
	}
	marshalClrColor(b, ns, localName, attrs, c.hslSlots(), c.xfOrder, c.xfRaws)
}

// prstSlots returns the transform slots of a PrstClr in field order.
func (c *PrstClr) prstSlots() []clrXfSlot {
	return []clrXfSlot{
		{ctTint, &c.Tint, nil}, {ctShade, &c.Shade, nil},
		{ctComp, nil, &c.Comp}, {ctInv, nil, &c.Inv}, {ctGray, nil, &c.Gray},
		{ctAlpha, &c.Alpha, nil}, {ctAlphaOff, &c.AlphaOff, nil}, {ctAlphaMod, &c.AlphaMod, nil},
		{ctHue, &c.Hue, nil}, {ctHueOff, &c.HueOff, nil}, {ctHueMod, &c.HueMod, nil},
		{ctSat, &c.Sat, nil}, {ctSatOff, &c.SatOff, nil}, {ctSatMod, &c.SatMod, nil},
		{ctLum, &c.Lum, nil}, {ctLumOff, &c.LumOff, nil}, {ctLumMod, &c.LumMod, nil},
		{ctRed, &c.Red, nil}, {ctRedOff, &c.RedOff, nil}, {ctRedMod, &c.RedMod, nil},
		{ctGreen, &c.Green, nil}, {ctGreenOff, &c.GreenOff, nil}, {ctGreenMod, &c.GreenMod, nil},
		{ctBlue, &c.Blue, nil}, {ctBlueOff, &c.BlueOff, nil}, {ctBlueMod, &c.BlueMod, nil},
		{ctGamma, nil, &c.Gamma}, {ctInvGamma, nil, &c.InvGamma},
	}
}

// UnmarshalXML implements xml.Unmarshaler, capturing transform order.
func (c *PrstClr) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	return unmarshalClrColor(d, start, func(attr xml.Attr) error {
		if attr.Name.Local == "val" {
			c.Val = attr.Value
		}
		return nil
	}, c.prstSlots(), &c.xfOrder, &c.xfRaws)
}

// MarshalXML implements xml.Marshaler, mirroring MarshalToBuilder on the
// encoding/xml path; see encodeClrColor.
func (c *PrstClr) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	attrs := []xml.Attr{{Name: xml.Name{Local: "val"}, Value: c.Val}}
	return encodeClrColor(e, start, attrs, c.prstSlots(), c.xfOrder, c.xfRaws)
}

// MarshalToBuilder implements xmlb.BuilderMarshaler, replaying transform order.
func (c *PrstClr) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	marshalClrColor(b, ns, localName, []xmlb.Attr{xmlb.StrAttr("val", c.Val)}, c.prstSlots(), c.xfOrder, c.xfRaws)
}

// scrgbSlots returns the transform slots of a ScRgbClr in field order.
func (c *ScRgbClr) scrgbSlots() []clrXfSlot {
	return []clrXfSlot{
		{ctTint, &c.Tint, nil}, {ctShade, &c.Shade, nil},
		{ctComp, nil, &c.Comp}, {ctInv, nil, &c.Inv}, {ctGray, nil, &c.Gray},
		{ctAlpha, &c.Alpha, nil}, {ctAlphaOff, &c.AlphaOff, nil}, {ctAlphaMod, &c.AlphaMod, nil},
		{ctHue, &c.Hue, nil}, {ctHueOff, &c.HueOff, nil}, {ctHueMod, &c.HueMod, nil},
		{ctSat, &c.Sat, nil}, {ctSatOff, &c.SatOff, nil}, {ctSatMod, &c.SatMod, nil},
		{ctLum, &c.Lum, nil}, {ctLumOff, &c.LumOff, nil}, {ctLumMod, &c.LumMod, nil},
		{ctRed, &c.Red, nil}, {ctRedOff, &c.RedOff, nil}, {ctRedMod, &c.RedMod, nil},
		{ctGreen, &c.Green, nil}, {ctGreenOff, &c.GreenOff, nil}, {ctGreenMod, &c.GreenMod, nil},
		{ctBlue, &c.Blue, nil}, {ctBlueOff, &c.BlueOff, nil}, {ctBlueMod, &c.BlueMod, nil},
		{ctGamma, nil, &c.Gamma}, {ctInvGamma, nil, &c.InvGamma},
	}
}

// UnmarshalXML implements xml.Unmarshaler, capturing transform order.
func (c *ScRgbClr) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	c.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	return unmarshalClrColor(d, start, func(attr xml.Attr) error {
		switch attr.Name.Local {
		case "r":
			return c.R.UnmarshalXMLAttr(attr)
		case "g":
			return c.G.UnmarshalXMLAttr(attr)
		case "b":
			return c.B.UnmarshalXMLAttr(attr)
		}
		return nil
	}, c.scrgbSlots(), &c.xfOrder, &c.xfRaws)
}

// MarshalXML implements xml.Marshaler, mirroring MarshalToBuilder on the
// encoding/xml path; see encodeClrColor.
func (c *ScRgbClr) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	attrs := []xml.Attr{
		{Name: xml.Name{Local: "r"}, Value: c.R.AttrValue()},
		{Name: xml.Name{Local: "g"}, Value: c.G.AttrValue()},
		{Name: xml.Name{Local: "b"}, Value: c.B.AttrValue()},
	}
	return encodeClrColor(e, start, attrs, c.scrgbSlots(), c.xfOrder, c.xfRaws)
}

// MarshalToBuilder implements xmlb.BuilderMarshaler, replaying transform order.
func (c *ScRgbClr) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	attrs := []xmlb.Attr{
		xmlb.StrAttr("r", c.R.AttrValue()),
		xmlb.StrAttr("g", c.G.AttrValue()),
		xmlb.StrAttr("b", c.B.AttrValue()),
	}
	if c.CapturedAttrs != nil {
		attrs = b.ReplayCapturedAttrs(c.CapturedAttrs, attrs)
	}
	marshalClrColor(b, ns, localName, attrs, c.scrgbSlots(), c.xfOrder, c.xfRaws)
}
