// This file provides the simple property-value element types of the math
// schema: elements whose payload is a single (qualified) m:val attribute.

package omml

import (
	"encoding/xml"
	"fmt"
	"strconv"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// valAttr returns the element's val attribute — m:val per the schema's
// qualified attribute form, with an unqualified val accepted leniently — and
// skips to the element's end.
func valAttr(d *xml.Decoder, start xml.StartElement) (string, error) {
	val := ""
	for _, a := range start.Attr {
		if a.Name.Local == "val" && (a.Name.Space == NS || a.Name.Space == "") {
			val = a.Value
		}
	}
	return val, d.Skip()
}

// emitVal writes a self-closing element carrying an m:val attribute. When
// required is false and val is empty, the attribute is omitted entirely
// (the schema marks it optional).
func emitVal(b *xmlb.Builder, ns, localName, val string, required bool) {
	if !required && val == "" {
		b.EmptyElement(ns, localName)
		return
	}
	b.EmptyElement(ns, localName, xmlb.Attr{Namespace: NS, Name: "val", Value: val})
}

// intVal parses a required integer val attribute.
func intVal(d *xml.Decoder, start xml.StartElement) (int, error) {
	s, err := valAttr(d, start)
	if err != nil {
		return 0, err
	}
	if s == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("omml: %s val: %w", start.Name.Local, err)
	}
	return n, nil
}

// OnOff represents CT_OnOff, a boolean toggle. An empty Val means the
// attribute was absent (which the schema treats as "on").
type OnOff struct {
	Val string // "1", "0", "true", "false", "on", "off", or ""
}

// UnmarshalXML implements xml.Unmarshaler.
func (v *OnOff) UnmarshalXML(d *xml.Decoder, start xml.StartElement) (err error) {
	v.Val, err = valAttr(d, start)
	return err
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *OnOff) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	emitVal(b, ns, localName, v.Val, false)
}

// Char represents CT_Char, a single-character value (operator, accent, or
// delimiter character). The val attribute is required and may legitimately
// be empty (e.g. a cases construct's absent begin delimiter).
type Char struct {
	Val string
}

// UnmarshalXML implements xml.Unmarshaler.
func (v *Char) UnmarshalXML(d *xml.Decoder, start xml.StartElement) (err error) {
	v.Val, err = valAttr(d, start)
	return err
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *Char) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	emitVal(b, ns, localName, v.Val, true)
}

// TopBot represents CT_TopBot: "top" or "bot".
type TopBot struct {
	Val string
}

// UnmarshalXML implements xml.Unmarshaler.
func (v *TopBot) UnmarshalXML(d *xml.Decoder, start xml.StartElement) (err error) {
	v.Val, err = valAttr(d, start)
	return err
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *TopBot) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	emitVal(b, ns, localName, v.Val, true)
}

// Shape represents CT_Shp, the delimiter shape: "centered" or "match".
type Shape struct {
	Val string
}

// UnmarshalXML implements xml.Unmarshaler.
func (v *Shape) UnmarshalXML(d *xml.Decoder, start xml.StartElement) (err error) {
	v.Val, err = valAttr(d, start)
	return err
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *Shape) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	emitVal(b, ns, localName, v.Val, true)
}

// YAlign represents CT_YAlign, a vertical alignment: "top", "center", "bot".
type YAlign struct {
	Val string
}

// UnmarshalXML implements xml.Unmarshaler.
func (v *YAlign) UnmarshalXML(d *xml.Decoder, start xml.StartElement) (err error) {
	v.Val, err = valAttr(d, start)
	return err
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *YAlign) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	emitVal(b, ns, localName, v.Val, true)
}

// XAlign represents CT_XAlign, a horizontal alignment: "left", "center",
// "right".
type XAlign struct {
	Val string
}

// UnmarshalXML implements xml.Unmarshaler.
func (v *XAlign) UnmarshalXML(d *xml.Decoder, start xml.StartElement) (err error) {
	v.Val, err = valAttr(d, start)
	return err
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *XAlign) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	emitVal(b, ns, localName, v.Val, true)
}

// FType represents CT_FType, the fraction type: "bar", "skw", "lin", "noBar".
type FType struct {
	Val string
}

// UnmarshalXML implements xml.Unmarshaler.
func (v *FType) UnmarshalXML(d *xml.Decoder, start xml.StartElement) (err error) {
	v.Val, err = valAttr(d, start)
	return err
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *FType) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	emitVal(b, ns, localName, v.Val, true)
}

// LimLoc represents CT_LimLoc, the n-ary limit location: "undOvr" or
// "subSup".
type LimLoc struct {
	Val string
}

// UnmarshalXML implements xml.Unmarshaler.
func (v *LimLoc) UnmarshalXML(d *xml.Decoder, start xml.StartElement) (err error) {
	v.Val, err = valAttr(d, start)
	return err
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *LimLoc) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	emitVal(b, ns, localName, v.Val, true)
}

// Script represents CT_Script, the math script style: "roman", "script",
// "fraktur", "double-struck", "sans-serif", "monospace".
type Script struct {
	Val string
}

// UnmarshalXML implements xml.Unmarshaler.
func (v *Script) UnmarshalXML(d *xml.Decoder, start xml.StartElement) (err error) {
	v.Val, err = valAttr(d, start)
	return err
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *Script) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	emitVal(b, ns, localName, v.Val, false)
}

// Style represents CT_Style, the run emphasis style: "p" (plain), "b"
// (bold), "i" (italic), "bi" (bold-italic).
type Style struct {
	Val string
}

// UnmarshalXML implements xml.Unmarshaler.
func (v *Style) UnmarshalXML(d *xml.Decoder, start xml.StartElement) (err error) {
	v.Val, err = valAttr(d, start)
	return err
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *Style) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	emitVal(b, ns, localName, v.Val, false)
}

// MathJc represents CT_OMathJc, math paragraph justification: "left",
// "right", "center", "centerGroup".
type MathJc struct {
	Val string
}

// UnmarshalXML implements xml.Unmarshaler.
func (v *MathJc) UnmarshalXML(d *xml.Decoder, start xml.StartElement) (err error) {
	v.Val, err = valAttr(d, start)
	return err
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *MathJc) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	emitVal(b, ns, localName, v.Val, false)
}

// MathFont represents CT_String as used by m:mathFont.
type MathFont struct {
	Val string
}

// UnmarshalXML implements xml.Unmarshaler.
func (v *MathFont) UnmarshalXML(d *xml.Decoder, start xml.StartElement) (err error) {
	v.Val, err = valAttr(d, start)
	return err
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *MathFont) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	emitVal(b, ns, localName, v.Val, false)
}

// BreakBin represents CT_BreakBin, line-break placement on binary operators:
// "before", "after", "repeat".
type BreakBin struct {
	Val string
}

// UnmarshalXML implements xml.Unmarshaler.
func (v *BreakBin) UnmarshalXML(d *xml.Decoder, start xml.StartElement) (err error) {
	v.Val, err = valAttr(d, start)
	return err
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *BreakBin) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	emitVal(b, ns, localName, v.Val, false)
}

// BreakBinSub represents CT_BreakBinSub, the duplicated-operator form for
// subtraction when wrapping: "--", "-+", "+-".
type BreakBinSub struct {
	Val string
}

// UnmarshalXML implements xml.Unmarshaler.
func (v *BreakBinSub) UnmarshalXML(d *xml.Decoder, start xml.StartElement) (err error) {
	v.Val, err = valAttr(d, start)
	return err
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *BreakBinSub) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	emitVal(b, ns, localName, v.Val, false)
}

// TwipsMeasure represents CT_TwipsMeasure (s:ST_TwipsMeasure): an unsigned
// twips count or a unit-suffixed measurement ("720", "0.5in"). Kept as a
// string to round-trip both lexical forms.
type TwipsMeasure struct {
	Val string
}

// UnmarshalXML implements xml.Unmarshaler.
func (v *TwipsMeasure) UnmarshalXML(d *xml.Decoder, start xml.StartElement) (err error) {
	v.Val, err = valAttr(d, start)
	return err
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *TwipsMeasure) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	emitVal(b, ns, localName, v.Val, true)
}

// SpacingRule represents CT_SpacingRule, a row/column spacing rule (0–4).
type SpacingRule struct {
	Val int
}

// UnmarshalXML implements xml.Unmarshaler.
func (v *SpacingRule) UnmarshalXML(d *xml.Decoder, start xml.StartElement) (err error) {
	v.Val, err = intVal(d, start)
	return err
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *SpacingRule) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	emitVal(b, ns, localName, strconv.Itoa(v.Val), true)
}

// UnSignedInteger represents CT_UnSignedInteger.
type UnSignedInteger struct {
	Val uint32
}

// UnmarshalXML implements xml.Unmarshaler.
func (v *UnSignedInteger) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	s, err := valAttr(d, start)
	if err != nil {
		return err
	}
	if s == "" {
		v.Val = 0
		return nil
	}
	n, err := strconv.ParseUint(s, 10, 32)
	if err != nil {
		return fmt.Errorf("omml: %s val: %w", start.Name.Local, err)
	}
	v.Val = uint32(n)
	return nil
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *UnSignedInteger) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	emitVal(b, ns, localName, strconv.FormatUint(uint64(v.Val), 10), true)
}

// Integer represents CT_Integer2, an argument-size increment (-2 to 2).
type Integer struct {
	Val int
}

// UnmarshalXML implements xml.Unmarshaler.
func (v *Integer) UnmarshalXML(d *xml.Decoder, start xml.StartElement) (err error) {
	v.Val, err = intVal(d, start)
	return err
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *Integer) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	emitVal(b, ns, localName, strconv.Itoa(v.Val), true)
}

// Integer255 represents CT_Integer255, a count in 1–255 (matrix column
// counts).
type Integer255 struct {
	Val int
}

// UnmarshalXML implements xml.Unmarshaler.
func (v *Integer255) UnmarshalXML(d *xml.Decoder, start xml.StartElement) (err error) {
	v.Val, err = intVal(d, start)
	return err
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *Integer255) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	emitVal(b, ns, localName, strconv.Itoa(v.Val), true)
}

// Break represents CT_ManualBreak (m:brk), a manual line break in an
// equation. AlnAt (1–255) aligns the wrapped line at the given operator;
// 0 means the attribute is absent.
type Break struct {
	AlnAt int
}

// UnmarshalXML implements xml.Unmarshaler.
func (v *Break) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, a := range start.Attr {
		if a.Name.Local == "alnAt" && (a.Name.Space == NS || a.Name.Space == "") {
			n, err := strconv.Atoi(a.Value)
			if err != nil {
				return fmt.Errorf("omml: brk alnAt: %w", err)
			}
			v.AlnAt = n
		}
	}
	return d.Skip()
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *Break) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	if v.AlnAt == 0 {
		b.EmptyElement(ns, localName)
		return
	}
	b.EmptyElement(ns, localName, xmlb.Attr{Namespace: NS, Name: "alnAt", Value: strconv.Itoa(v.AlnAt)})
}
