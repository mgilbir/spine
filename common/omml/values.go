// This file provides the simple property-value element types of the math
// schema: elements whose payload is a single (qualified) m:val attribute.

package omml

import (
	"encoding/xml"
	"strconv"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// valState records how a source wrote — or did not write — a val-carrying
// element's attribute, so a re-emit reproduces the document instead of
// inventing markup. Without it emitVal could not tell "attribute absent" from
// "attribute present and zero/empty": a bare <m:count/> came back as
// <m:count m:val="0"/>, a value outside CT_Integer255's 1..255 range that the
// source never contained, and a leniently-accepted unqualified val= was
// rewritten to m:val=.
//
// The zero valState means "built programmatically, not parsed", which keeps
// the canonical emission for models assembled in code.
type valState struct {
	// parsed is set when the element was decoded from a document.
	parsed bool
	// present is set when that element carried a val attribute.
	present bool
	// unqualified is set when the source wrote val= rather than m:val=.
	unqualified bool
	// atParse is the value's rendering immediately after parse. A later
	// rendering that differs means the caller changed the value, which is
	// authoritative over the capture — otherwise the capture would make a
	// modeled value impossible to set (the audit's T-D trap).
	atParse string
	// lexical holds the source's attribute text when it did not parse into
	// the field's Go type (a producer bug such as an unsigned rSp written
	// "-1"). It is re-emitted verbatim while the typed value is unchanged, so
	// one malformed number neither aborts the math zone nor rewrites it.
	lexical string
}

// capture reads the element's val attribute — m:val per the schema's qualified
// attribute form, with an unqualified val accepted leniently — records how it
// was written, and skips to the element's end.
func (s *valState) capture(d *xml.Decoder, start xml.StartElement) (string, error) {
	s.parsed = true
	val := ""
	for _, a := range start.Attr {
		if a.Name.Local != "val" {
			continue
		}
		switch a.Name.Space {
		case NS:
			s.present, s.unqualified, val = true, false, a.Value
		case "":
			s.present, s.unqualified, val = true, true, a.Value
		}
	}
	return val, d.Skip()
}

// valAttr is capture for the string-valued types, recording the parsed
// rendering in one step.
func (s *valState) valAttr(d *xml.Decoder, start xml.StartElement) (string, error) {
	val, err := s.capture(d, start)
	s.atParse = val
	return val, err
}

// render returns the attribute text to write for a value whose current typed
// rendering is cur, and whether to write the attribute at all.
func (s *valState) render(cur string, required bool) (string, bool) {
	if s.parsed && !s.present && cur == s.atParse {
		// The source wrote a bare element and nothing changed since: keep it
		// bare rather than materialize an attribute it never had.
		return "", false
	}
	if !required && cur == "" && !s.present {
		return "", false
	}
	if s.lexical != "" && cur == s.atParse {
		// Unparseable source form, still unmodified: re-emit it verbatim.
		return s.lexical, true
	}
	return cur, true
}

// emitVal writes a self-closing element carrying the element's val attribute,
// honoring the source's presence and qualification (see valState). When
// required is false and the value is empty the attribute is omitted entirely
// (the schema marks it optional).
func emitVal(b *xmlb.Builder, ns, localName, val string, required bool, st *valState) {
	text, ok := st.render(val, required)
	if !ok {
		b.EmptyElement(ns, localName)
		return
	}
	if st.unqualified {
		b.EmptyElement(ns, localName, xmlb.Attr{Name: "val", Value: text})
		return
	}
	b.EmptyElement(ns, localName, xmlb.Attr{Namespace: NS, Name: "val", Value: text})
}

// intVal parses an integer val attribute. A value that is not a Go int is not
// an error: the field keeps its zero and the source text is retained on the
// valState for verbatim re-emission. Rejecting it would make every math zone
// in the paragraph unreadable over one malformed attribute, while the same
// document's unknown *elements* are tolerated and raw-captured.
func intVal(d *xml.Decoder, start xml.StartElement, st *valState) (int, error) {
	s, err := st.capture(d, start)
	if err != nil {
		return 0, err
	}
	if s == "" {
		st.atParse = "0"
		return 0, nil
	}
	n, convErr := strconv.Atoi(s)
	if convErr != nil {
		st.lexical = s
		st.atParse = "0"
		return 0, nil
	}
	st.atParse = strconv.Itoa(n)
	return n, nil
}

// OnOff represents CT_OnOff, a boolean toggle. An empty Val means the
// attribute was absent (which the schema treats as "on").
type OnOff struct {
	Val string // "1", "0", "true", "false", "on", "off", or ""

	val valState
}

// UnmarshalXML implements xml.Unmarshaler.
func (v *OnOff) UnmarshalXML(d *xml.Decoder, start xml.StartElement) (err error) {
	v.Val, err = v.val.valAttr(d, start)
	return err
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *OnOff) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	emitVal(b, ns, localName, v.Val, false, &v.val)
}

// Char represents CT_Char, a single-character value (operator, accent, or
// delimiter character). The val attribute is required and may legitimately
// be empty (e.g. a cases construct's absent begin delimiter).
type Char struct {
	Val string

	val valState
}

// UnmarshalXML implements xml.Unmarshaler.
func (v *Char) UnmarshalXML(d *xml.Decoder, start xml.StartElement) (err error) {
	v.Val, err = v.val.valAttr(d, start)
	return err
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *Char) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	emitVal(b, ns, localName, v.Val, true, &v.val)
}

// TopBot represents CT_TopBot: "top" or "bot".
type TopBot struct {
	Val string

	val valState
}

// UnmarshalXML implements xml.Unmarshaler.
func (v *TopBot) UnmarshalXML(d *xml.Decoder, start xml.StartElement) (err error) {
	v.Val, err = v.val.valAttr(d, start)
	return err
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *TopBot) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	emitVal(b, ns, localName, v.Val, true, &v.val)
}

// Shape represents CT_Shp, the delimiter shape: "centered" or "match".
type Shape struct {
	Val string

	val valState
}

// UnmarshalXML implements xml.Unmarshaler.
func (v *Shape) UnmarshalXML(d *xml.Decoder, start xml.StartElement) (err error) {
	v.Val, err = v.val.valAttr(d, start)
	return err
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *Shape) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	emitVal(b, ns, localName, v.Val, true, &v.val)
}

// YAlign represents CT_YAlign, a vertical alignment: "top", "center", "bot".
type YAlign struct {
	Val string

	val valState
}

// UnmarshalXML implements xml.Unmarshaler.
func (v *YAlign) UnmarshalXML(d *xml.Decoder, start xml.StartElement) (err error) {
	v.Val, err = v.val.valAttr(d, start)
	return err
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *YAlign) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	emitVal(b, ns, localName, v.Val, true, &v.val)
}

// XAlign represents CT_XAlign, a horizontal alignment: "left", "center",
// "right".
type XAlign struct {
	Val string

	val valState
}

// UnmarshalXML implements xml.Unmarshaler.
func (v *XAlign) UnmarshalXML(d *xml.Decoder, start xml.StartElement) (err error) {
	v.Val, err = v.val.valAttr(d, start)
	return err
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *XAlign) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	emitVal(b, ns, localName, v.Val, true, &v.val)
}

// FType represents CT_FType, the fraction type: "bar", "skw", "lin", "noBar".
type FType struct {
	Val string

	val valState
}

// UnmarshalXML implements xml.Unmarshaler.
func (v *FType) UnmarshalXML(d *xml.Decoder, start xml.StartElement) (err error) {
	v.Val, err = v.val.valAttr(d, start)
	return err
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *FType) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	emitVal(b, ns, localName, v.Val, true, &v.val)
}

// LimLoc represents CT_LimLoc, the n-ary limit location: "undOvr" or
// "subSup".
type LimLoc struct {
	Val string

	val valState
}

// UnmarshalXML implements xml.Unmarshaler.
func (v *LimLoc) UnmarshalXML(d *xml.Decoder, start xml.StartElement) (err error) {
	v.Val, err = v.val.valAttr(d, start)
	return err
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *LimLoc) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	emitVal(b, ns, localName, v.Val, true, &v.val)
}

// Script represents CT_Script, the math script style: "roman", "script",
// "fraktur", "double-struck", "sans-serif", "monospace".
type Script struct {
	Val string

	val valState
}

// UnmarshalXML implements xml.Unmarshaler.
func (v *Script) UnmarshalXML(d *xml.Decoder, start xml.StartElement) (err error) {
	v.Val, err = v.val.valAttr(d, start)
	return err
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *Script) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	emitVal(b, ns, localName, v.Val, false, &v.val)
}

// Style represents CT_Style, the run emphasis style: "p" (plain), "b"
// (bold), "i" (italic), "bi" (bold-italic).
type Style struct {
	Val string

	val valState
}

// UnmarshalXML implements xml.Unmarshaler.
func (v *Style) UnmarshalXML(d *xml.Decoder, start xml.StartElement) (err error) {
	v.Val, err = v.val.valAttr(d, start)
	return err
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *Style) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	emitVal(b, ns, localName, v.Val, false, &v.val)
}

// MathJc represents CT_OMathJc, math paragraph justification: "left",
// "right", "center", "centerGroup".
type MathJc struct {
	Val string

	val valState
}

// UnmarshalXML implements xml.Unmarshaler.
func (v *MathJc) UnmarshalXML(d *xml.Decoder, start xml.StartElement) (err error) {
	v.Val, err = v.val.valAttr(d, start)
	return err
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *MathJc) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	emitVal(b, ns, localName, v.Val, false, &v.val)
}

// MathFont represents CT_String as used by m:mathFont.
type MathFont struct {
	Val string

	val valState
}

// UnmarshalXML implements xml.Unmarshaler.
func (v *MathFont) UnmarshalXML(d *xml.Decoder, start xml.StartElement) (err error) {
	v.Val, err = v.val.valAttr(d, start)
	return err
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *MathFont) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	emitVal(b, ns, localName, v.Val, false, &v.val)
}

// BreakBin represents CT_BreakBin, line-break placement on binary operators:
// "before", "after", "repeat".
type BreakBin struct {
	Val string

	val valState
}

// UnmarshalXML implements xml.Unmarshaler.
func (v *BreakBin) UnmarshalXML(d *xml.Decoder, start xml.StartElement) (err error) {
	v.Val, err = v.val.valAttr(d, start)
	return err
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *BreakBin) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	emitVal(b, ns, localName, v.Val, false, &v.val)
}

// BreakBinSub represents CT_BreakBinSub, the duplicated-operator form for
// subtraction when wrapping: "--", "-+", "+-".
type BreakBinSub struct {
	Val string

	val valState
}

// UnmarshalXML implements xml.Unmarshaler.
func (v *BreakBinSub) UnmarshalXML(d *xml.Decoder, start xml.StartElement) (err error) {
	v.Val, err = v.val.valAttr(d, start)
	return err
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *BreakBinSub) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	emitVal(b, ns, localName, v.Val, false, &v.val)
}

// TwipsMeasure represents CT_TwipsMeasure (s:ST_TwipsMeasure): an unsigned
// twips count or a unit-suffixed measurement ("720", "0.5in"). Kept as a
// string to round-trip both lexical forms.
type TwipsMeasure struct {
	Val string

	val valState
}

// UnmarshalXML implements xml.Unmarshaler.
func (v *TwipsMeasure) UnmarshalXML(d *xml.Decoder, start xml.StartElement) (err error) {
	v.Val, err = v.val.valAttr(d, start)
	return err
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *TwipsMeasure) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	emitVal(b, ns, localName, v.Val, true, &v.val)
}

// SpacingRule represents CT_SpacingRule, a row/column spacing rule (0–4).
type SpacingRule struct {
	Val int

	val valState
}

// UnmarshalXML implements xml.Unmarshaler.
func (v *SpacingRule) UnmarshalXML(d *xml.Decoder, start xml.StartElement) (err error) {
	v.Val, err = intVal(d, start, &v.val)
	return err
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *SpacingRule) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	emitVal(b, ns, localName, strconv.Itoa(v.Val), true, &v.val)
}

// UnSignedInteger represents CT_UnSignedInteger.
type UnSignedInteger struct {
	Val uint32

	val valState
}

// UnmarshalXML implements xml.Unmarshaler. A val that is not an unsigned
// 32-bit integer — a producer writing "-1" for a spacing that the schema
// makes unsigned — leaves the field zero and is re-emitted verbatim rather
// than failing the whole math zone (see intVal).
func (v *UnSignedInteger) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	s, err := v.val.capture(d, start)
	if err != nil {
		return err
	}
	if s == "" {
		v.Val = 0
		v.val.atParse = "0"
		return nil
	}
	n, convErr := strconv.ParseUint(s, 10, 32)
	if convErr != nil {
		v.Val = 0
		v.val.lexical = s
		v.val.atParse = "0"
		return nil
	}
	v.Val = uint32(n)
	v.val.atParse = strconv.FormatUint(n, 10)
	return nil
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *UnSignedInteger) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	emitVal(b, ns, localName, strconv.FormatUint(uint64(v.Val), 10), true, &v.val)
}

// Integer represents CT_Integer2, an argument-size increment (-2 to 2).
type Integer struct {
	Val int

	val valState
}

// UnmarshalXML implements xml.Unmarshaler.
func (v *Integer) UnmarshalXML(d *xml.Decoder, start xml.StartElement) (err error) {
	v.Val, err = intVal(d, start, &v.val)
	return err
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *Integer) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	emitVal(b, ns, localName, strconv.Itoa(v.Val), true, &v.val)
}

// Integer255 represents CT_Integer255, a count in 1–255 (matrix column
// counts).
type Integer255 struct {
	Val int

	val valState
}

// UnmarshalXML implements xml.Unmarshaler.
func (v *Integer255) UnmarshalXML(d *xml.Decoder, start xml.StartElement) (err error) {
	v.Val, err = intVal(d, start, &v.val)
	return err
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *Integer255) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	emitVal(b, ns, localName, strconv.Itoa(v.Val), true, &v.val)
}

// Break represents CT_ManualBreak (m:brk), a manual line break in an
// equation. AlnAt (1–255) aligns the wrapped line at the given operator;
// 0 means the attribute is absent.
type Break struct {
	AlnAt int

	val valState
}

// UnmarshalXML implements xml.Unmarshaler. An alnAt that is not a number
// leaves AlnAt zero and is re-emitted verbatim: a malformed attribute on one
// break must not make every math zone in the paragraph unreadable.
func (v *Break) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	v.val.parsed = true
	v.val.atParse = "0"
	for _, a := range start.Attr {
		if a.Name.Local != "alnAt" {
			continue
		}
		switch a.Name.Space {
		case NS:
			v.val.present, v.val.unqualified = true, false
		case "":
			v.val.present, v.val.unqualified = true, true
		default:
			continue
		}
		n, err := strconv.Atoi(a.Value)
		if err != nil {
			v.val.lexical = a.Value
			continue
		}
		v.AlnAt = n
		v.val.atParse = strconv.Itoa(n)
	}
	return d.Skip()
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *Break) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	text, ok := v.val.render(strconv.Itoa(v.AlnAt), false)
	if !ok || (text == "0" && !v.val.present) {
		b.EmptyElement(ns, localName)
		return
	}
	if v.val.unqualified {
		b.EmptyElement(ns, localName, xmlb.Attr{Name: "alnAt", Value: text})
		return
	}
	b.EmptyElement(ns, localName, xmlb.Attr{Namespace: NS, Name: "alnAt", Value: text})
}
