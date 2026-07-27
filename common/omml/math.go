// Package omml provides typed Office Math Markup Language (OMML) models for
// the m: namespace (shared-math.xsd in ECMA-376): math zones (m:oMath), math
// paragraphs (m:oMathPara), and the full structure set — runs, fractions,
// radicals, scripts, n-ary operators, delimiters, matrices, accents, and the
// bar/box family.
//
// Math zone content is an ordered heterogeneous sequence: an m:oMath child
// list may interleave runs, fractions, and scripts, and that order is the
// equation. The model therefore stores content as a single ordered slice of
// the MathItem union instead of grouping children per kind.
//
// WordprocessingML content that the schema allows inside math (w:rPr run
// formatting, w:ins/w:del in m:ctrlPr, w:EG_PContentMath elements) is
// preserved verbatim and in position as *Raw values — common/omml cannot
// depend on docx internals. Unknown elements anywhere get the same raw
// in-position capture, so a typed round-trip is never lossier than raw
// bytes.
//
// Serialization goes through the common/xml Builder with m:-prefixed names
// (see MarshalToBuilder on every type); parsing uses encoding/xml custom
// unmarshalers.
package omml

import (
	"bytes"
	"encoding/xml"
	"reflect"
	"strings"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// NS is the Office Math (OMML) namespace URI.
const NS = xmlb.NSMath

// MathItem is one element of an ordered math sequence (EG_OMathElements):
// any of the twenty math structures of EG_OMathMathElements (*Run,
// *Fraction, *Radical, ...) or a *Raw holding WordprocessingML content
// (w:EG_PContentMath) or an unknown element, preserved verbatim in position.
type MathItem interface {
	emitMath(b *xmlb.Builder)
}

// decodeMathItem decodes one EG_OMathElements member, falling back to raw
// in-position capture for anything the model does not type.
func decodeMathItem(d *xml.Decoder, t xml.StartElement) (MathItem, error) {
	if t.Name.Space == NS {
		var v MathItem
		switch t.Name.Local {
		case "acc":
			v = &Accent{}
		case "bar":
			v = &Bar{}
		case "box":
			v = &Box{}
		case "borderBox":
			v = &BorderBox{}
		case "d":
			v = &Delimiter{}
		case "eqArr":
			v = &EquationArray{}
		case "f":
			v = &Fraction{}
		case "func":
			v = &Function{}
		case "groupChr":
			v = &GroupChar{}
		case "limLow":
			v = &LimitLow{}
		case "limUpp":
			v = &LimitUpper{}
		case "m":
			v = &Matrix{}
		case "nary":
			v = &NAry{}
		case "phant":
			v = &Phantom{}
		case "rad":
			v = &Radical{}
		case "sPre":
			v = &SubSuperscriptPre{}
		case "sSub":
			v = &Subscript{}
		case "sSubSup":
			v = &SubSuperscript{}
		case "sSup":
			v = &Superscript{}
		case "r":
			v = &Run{}
		}
		if v != nil {
			return v, d.DecodeElement(v, &t)
		}
	}
	r := &Raw{}
	return r, r.UnmarshalXML(d, t)
}

// OMath represents CT_OMath (m:oMath), a math zone. Items is the zone's
// ordered content.
type OMath struct {
	Items []MathItem
}

// UnmarshalXML implements xml.Unmarshaler.
func (m *OMath) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.CharData:
			if text := captureCharData(t); text != nil {
				m.Items = append(m.Items, text)
			}
		case xml.StartElement:
			it, err := decodeMathItem(d, t)
			if err != nil {
				return err
			}
			m.Items = append(m.Items, it)
		case xml.EndElement:
			return nil
		}
	}
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (m *OMath) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	if len(m.Items) == 0 {
		b.EmptyElement(ns, localName)
		return
	}
	b.StartElement(ns, localName)
	m.MarshalContent(b)
	b.EndElement(ns, localName)
}

// MarshalContent writes the zone's children — everything inside the
// m:oMath element, without the element itself. The builder must have the
// math (and, when WML content is present, wordprocessingml) namespaces
// registered; xmlb.NewWordprocessingMLBuilder registers both.
func (m *OMath) MarshalContent(b *xmlb.Builder) {
	for _, it := range m.Items {
		it.emitMath(b)
	}
}

func (m *OMath) emitMath(b *xmlb.Builder) { m.MarshalToBuilder(b, NS, "oMath") }

// Text returns the concatenated m:t content of the zone, walking every
// structure's arguments in document order.
//
// A math structure this model does not type — a future OMML element — lands
// in a *Raw, and its m:t descendants are recovered from the captured bytes so
// extraction degrades rather than silently losing text. Raw *WordprocessingML*
// content (w:EG_PContentMath runs, w:ins/w:del in m:ctrlPr) contributes
// nothing: it is Word run content, not math text, and common/omml cannot
// depend on docx to interpret it.
func (m *OMath) Text() string {
	var sb strings.Builder
	for _, it := range m.Items {
		appendItemText(&sb, it)
	}
	return sb.String()
}

// OMathPara represents CT_OMathPara (m:oMathPara), a math paragraph: one or
// more math zones with paragraph-level justification. Word uses it for
// display (own-line) equations.
type OMathPara struct {
	OMathParaPr *OMathParaPr
	OMath       []*OMath

	extra []extraChild
}

var oMathParaFields = seqFields(OMathPara{}, "oMathParaPr=OMathParaPr", "oMath=OMath")

// UnmarshalXML implements xml.Unmarshaler.
func (p *OMathPara) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	return unmarshalSeq(d, p, oMathParaFields, &p.extra)
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (p *OMathPara) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	marshalSeq(b, ns, localName, p, oMathParaFields, p.extra)
}

// MarshalContent writes the math paragraph's children without the
// m:oMathPara element itself (see OMath.MarshalContent).
func (p *OMathPara) MarshalContent(b *xmlb.Builder) {
	marshalSeqContent(b, reflect.ValueOf(p).Elem(), oMathParaFields, p.extra)
}

// Text returns the concatenated m:t content of all contained math zones.
func (p *OMathPara) Text() string {
	var sb strings.Builder
	for _, m := range p.OMath {
		sb.WriteString(m.Text())
	}
	return sb.String()
}

// OMathParaPr represents CT_OMathParaPr (m:oMathParaPr).
type OMathParaPr struct {
	Jc *MathJc

	extra []extraChild
}

var oMathParaPrFields = seqFields(OMathParaPr{}, "jc=Jc")

// UnmarshalXML implements xml.Unmarshaler.
func (v *OMathParaPr) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	return unmarshalSeq(d, v, oMathParaPrFields, &v.extra)
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *OMathParaPr) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	marshalSeq(b, ns, localName, v, oMathParaPrFields, v.extra)
}

// Element represents CT_OMathArg — the argument elements m:e, m:num, m:den,
// m:sub, m:sup, m:deg, m:lim, and m:fName. Like a math zone it holds an
// ordered content sequence, plus optional argument properties (m:argPr,
// first) and control properties (m:ctrlPr, last).
type Element struct {
	ArgPr  *ArgPr
	Items  []MathItem
	CtrlPr *CtrlPr

	// tail holds content the source placed after m:ctrlPr — a non-conformant
	// duplicate ctrlPr, or an unknown element written last. Items are emitted
	// before m:ctrlPr, so these need their own slot to keep source order.
	tail []MathItem
}

// UnmarshalXML implements xml.Unmarshaler.
func (e *Element) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.CharData:
			if text := captureCharData(t); text != nil {
				e.appendItem(text)
			}
		case xml.StartElement:
			// A repeated m:argPr / m:ctrlPr is non-conformant. Overwriting the
			// field dropped the first occurrence silently; fall through to the
			// in-position raw capture so nothing is lost (C471).
			if t.Name.Space == NS {
				switch {
				case t.Name.Local == "argPr" && e.ArgPr == nil:
					v := &ArgPr{}
					if err := d.DecodeElement(v, &t); err != nil {
						return err
					}
					e.ArgPr = v
					continue
				case t.Name.Local == "ctrlPr" && e.CtrlPr == nil:
					v := &CtrlPr{}
					if err := d.DecodeElement(v, &t); err != nil {
						return err
					}
					e.CtrlPr = v
					continue
				}
			}
			it, err := decodeMathItem(d, t)
			if err != nil {
				return err
			}
			e.appendItem(it)
		case xml.EndElement:
			return nil
		}
	}
}

// appendItem records one content child, keeping anything that follows the
// element's m:ctrlPr in the tail slot so source order survives.
func (e *Element) appendItem(it MathItem) {
	if e.CtrlPr != nil {
		e.tail = append(e.tail, it)
		return
	}
	e.Items = append(e.Items, it)
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (e *Element) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	if e.ArgPr == nil && len(e.Items) == 0 && e.CtrlPr == nil && len(e.tail) == 0 {
		b.EmptyElement(ns, localName)
		return
	}
	b.StartElement(ns, localName)
	if e.ArgPr != nil {
		e.ArgPr.MarshalToBuilder(b, NS, "argPr")
	}
	for _, it := range e.Items {
		it.emitMath(b)
	}
	if e.CtrlPr != nil {
		e.CtrlPr.MarshalToBuilder(b, NS, "ctrlPr")
	}
	for _, it := range e.tail {
		it.emitMath(b)
	}
	b.EndElement(ns, localName)
}

// Text returns the concatenated m:t content of the argument.
func (e *Element) Text() string {
	var sb strings.Builder
	appendElementText(&sb, e)
	return sb.String()
}

// ArgPr represents CT_OMathArgPr (m:argPr).
type ArgPr struct {
	ArgSz *Integer

	extra []extraChild
}

var argPrFields = seqFields(ArgPr{}, "argSz=ArgSz")

// UnmarshalXML implements xml.Unmarshaler.
func (v *ArgPr) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	return unmarshalSeq(d, v, argPrFields, &v.extra)
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *ArgPr) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	marshalSeq(b, ns, localName, v, argPrFields, v.extra)
}

// MathPr represents CT_MathPr (m:mathPr), the document-level math settings
// stored in the WML settings part.
type MathPr struct {
	MathFont   *MathFont
	BrkBin     *BreakBin
	BrkBinSub  *BreakBinSub
	SmallFrac  *OnOff
	DispDef    *OnOff
	LMargin    *TwipsMeasure
	RMargin    *TwipsMeasure
	DefJc      *MathJc
	PreSp      *TwipsMeasure
	PostSp     *TwipsMeasure
	InterSp    *TwipsMeasure
	IntraSp    *TwipsMeasure
	WrapIndent *TwipsMeasure // choice with WrapRight
	WrapRight  *OnOff        // choice with WrapIndent
	IntLim     *LimLoc
	NaryLim    *LimLoc

	extra []extraChild
}

var mathPrFields = seqFields(MathPr{},
	"mathFont=MathFont", "brkBin=BrkBin", "brkBinSub=BrkBinSub",
	"smallFrac=SmallFrac", "dispDef=DispDef", "lMargin=LMargin", "rMargin=RMargin",
	"defJc=DefJc", "preSp=PreSp", "postSp=PostSp", "interSp=InterSp", "intraSp=IntraSp",
	"wrapIndent=WrapIndent", "wrapRight=WrapRight", "intLim=IntLim", "naryLim=NaryLim")

// UnmarshalXML implements xml.Unmarshaler.
func (v *MathPr) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	return unmarshalSeq(d, v, mathPrFields, &v.extra)
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *MathPr) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	marshalSeq(b, ns, localName, v, mathPrFields, v.extra)
}

// appendElementText appends the visible text of an argument element.
func appendElementText(sb *strings.Builder, e *Element) {
	if e == nil {
		return
	}
	for _, it := range e.Items {
		appendItemText(sb, it)
	}
	for _, it := range e.tail {
		appendItemText(sb, it)
	}
}

// appendRawMathText appends the m:t text carried by a raw-captured math
// element — an OMML structure this model does not type. The capture holds the
// element's inner XML verbatim, with the source's prefixes, so the scan
// matches on the local name alone: inside an m:-namespace element a <t> is an
// m:t. Captures outside the math namespace (WordprocessingML content) are not
// math text and are skipped.
func appendRawMathText(sb *strings.Builder, r *Raw) {
	if r.Space != NS || len(r.Content) == 0 {
		return
	}
	d := xml.NewDecoder(bytes.NewReader(r.Content))
	depth := 0
	for {
		tok, err := d.Token()
		if err != nil {
			return
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if depth > 0 || t.Name.Local == "t" {
				depth++
			}
		case xml.EndElement:
			if depth > 0 {
				depth--
			}
		case xml.CharData:
			if depth > 0 {
				sb.Write(t)
			}
		}
	}
}

// appendItemText appends the visible text of one math item, walking its
// arguments in document (schema) order.
func appendItemText(sb *strings.Builder, it MathItem) {
	switch v := it.(type) {
	case *Raw:
		appendRawMathText(sb, v)
	case *Run:
		sb.WriteString(v.Text())
	case *Accent:
		appendElementText(sb, v.E)
	case *Bar:
		appendElementText(sb, v.E)
	case *Box:
		appendElementText(sb, v.E)
	case *BorderBox:
		appendElementText(sb, v.E)
	case *Delimiter:
		for _, e := range v.E {
			appendElementText(sb, e)
		}
	case *EquationArray:
		for _, e := range v.E {
			appendElementText(sb, e)
		}
	case *Fraction:
		appendElementText(sb, v.Num)
		appendElementText(sb, v.Den)
	case *Function:
		appendElementText(sb, v.FName)
		appendElementText(sb, v.E)
	case *GroupChar:
		appendElementText(sb, v.E)
	case *LimitLow:
		appendElementText(sb, v.E)
		appendElementText(sb, v.Lim)
	case *LimitUpper:
		appendElementText(sb, v.E)
		appendElementText(sb, v.Lim)
	case *Matrix:
		for _, mr := range v.MR {
			for _, e := range mr.E {
				appendElementText(sb, e)
			}
		}
	case *NAry:
		appendElementText(sb, v.Sub)
		appendElementText(sb, v.Sup)
		appendElementText(sb, v.E)
	case *Phantom:
		appendElementText(sb, v.E)
	case *Radical:
		appendElementText(sb, v.Deg)
		appendElementText(sb, v.E)
	case *SubSuperscriptPre:
		appendElementText(sb, v.Sub)
		appendElementText(sb, v.Sup)
		appendElementText(sb, v.E)
	case *Subscript:
		appendElementText(sb, v.E)
		appendElementText(sb, v.Sub)
	case *SubSuperscript:
		appendElementText(sb, v.E)
		appendElementText(sb, v.Sub)
		appendElementText(sb, v.Sup)
	case *Superscript:
		appendElementText(sb, v.E)
		appendElementText(sb, v.Sup)
	}
}
