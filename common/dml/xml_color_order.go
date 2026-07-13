package dml

import (
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
func unmarshalClrColor(d *xml.Decoder, start xml.StartElement, setAttr func(xml.Attr) error, slots []clrXfSlot, order *[]clrTransformKind) error {
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
			slot := byKind[kind]
			wasSet := slot.isSet()
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
			// A duplicate kind keeps the single-occurrence model's last-wins
			// value but is recorded once, so the order replay emits it once.
			if !wasSet {
				*order = append(*order, kind)
			}
		case xml.EndElement:
			return nil
		}
	}
}

// marshalClrColor writes a color element with its attributes and transform
// children, replaying the captured source order when present.
func marshalClrColor(b *xmlb.Builder, ns, localName string, attrs []xmlb.Attr, slots []clrXfSlot, order []clrTransformKind) {
	any := false
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
	}, c.srgbSlots(), &c.xfOrder)
}

// MarshalToBuilder implements xmlb.BuilderMarshaler, replaying transform order.
func (c *SrgbClr) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	marshalClrColor(b, ns, localName, []xmlb.Attr{xmlb.StrAttr("val", c.Val)}, c.srgbSlots(), c.xfOrder)
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
	}, c.sysSlots(), &c.xfOrder)
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
	marshalClrColor(b, ns, localName, attrs, c.sysSlots(), c.xfOrder)
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
	}, c.hslSlots(), &c.xfOrder)
}

// MarshalToBuilder implements xmlb.BuilderMarshaler, replaying transform order.
func (c *HslClr) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	attrs := []xmlb.Attr{
		xmlb.Int32Attr("hue", c.Hue),
		xmlb.StrAttr("sat", c.Sat.AttrValue()),
		xmlb.StrAttr("lum", c.Lum.AttrValue()),
	}
	marshalClrColor(b, ns, localName, attrs, c.hslSlots(), c.xfOrder)
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
	}, c.prstSlots(), &c.xfOrder)
}

// MarshalToBuilder implements xmlb.BuilderMarshaler, replaying transform order.
func (c *PrstClr) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	marshalClrColor(b, ns, localName, []xmlb.Attr{xmlb.StrAttr("val", c.Val)}, c.prstSlots(), c.xfOrder)
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
	}, c.scrgbSlots(), &c.xfOrder)
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
	marshalClrColor(b, ns, localName, attrs, c.scrgbSlots(), c.xfOrder)
}
