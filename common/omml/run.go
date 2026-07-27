// This file provides the math run types: CT_R (m:r), its math run
// properties CT_RPR (m:rPr), the run text CT_Text (m:t), and the
// control-character properties CT_CtrlPr (m:ctrlPr).

package omml

import (
	"encoding/xml"
	"strings"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// RunChild is one ordered content child of a math run: *Text for m:t, or
// *Raw for WordprocessingML run content (w:br, w:tab, ...) and the optional
// w:rPr character formatting, which common/omml preserves verbatim.
type RunChild interface {
	emitRunChild(b *xmlb.Builder)
}

// Text represents CT_Text (m:t), the literal text of a math run.
type Text struct {
	// Space carries the xml:space attribute; "preserve" keeps significant
	// leading/trailing whitespace (mirrors w:t handling).
	Space string
	Value string

	// CapturedAttrs is the verbatim source attribute list. m:t is the single
	// most common math element, and modeling only xml:space silently dropped
	// every producer extension and any future schema attribute on it — with
	// no raw-capture fallback, unlike every other element in this package.
	// nil means the value was built programmatically.
	CapturedAttrs []xmlb.RootAttr `xml:"-"`
	// CapturedEmptyTag records whether an empty m:t was written <m:t/> or
	// <m:t></m:t>; producers mix both forms within one part.
	CapturedEmptyTag xmlb.EmptyTagStyle `xml:"-"`
}

// UnmarshalXML implements xml.Unmarshaler.
func (t *Text) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	t.CapturedEmptyTag = xmlb.CaptureEmptyTagStyle(d)
	t.CapturedAttrs = xmlb.CaptureAttrs(start.Attr)
	for _, a := range start.Attr {
		if a.Name.Local == "space" && a.Name.Space == xmlNamespace {
			t.Space = a.Value
		}
	}
	var inner struct {
		Value string `xml:",chardata"`
	}
	if err := d.DecodeElement(&inner, &start); err != nil {
		return err
	}
	t.Value = inner.Value
	return nil
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (t *Text) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	var attrs []xmlb.Attr
	if t.Space != "" {
		attrs = append(attrs, xmlb.Attr{Name: "xml:space", Value: t.Space})
	}
	if captured := t.CapturedAttrs; captured != nil {
		if t.Space == "" {
			// Space cleared after parse: a captured attribute with no modeled
			// match replays verbatim, so the stale xml:space has to go or the
			// modeled zero could never win.
			captured = withoutAttr(captured, xmlNamespace, "space")
		}
		attrs = b.ReplayCapturedAttrs(captured, attrs)
	}
	if t.Value == "" && t.CapturedEmptyTag != xmlb.EmptyTagUnknown {
		b.EmptyElementStyled(t.CapturedEmptyTag, ns, localName, attrs...)
		return
	}
	b.WriteElement(ns, localName, t.Value, attrs...)
}

func (t *Text) emitRunChild(b *xmlb.Builder) { t.MarshalToBuilder(b, NS, "t") }

// withoutAttr returns captured with the attribute in (space, local) removed.
// The input is not modified.
func withoutAttr(captured []xmlb.RootAttr, space, local string) []xmlb.RootAttr {
	out := make([]xmlb.RootAttr, 0, len(captured))
	for _, ra := range captured {
		if !ra.IsNS && ra.Space == space && ra.LocalName == local {
			continue
		}
		out = append(out, ra)
	}
	return out
}

// Run represents CT_R (m:r), a math run. RPr holds the math run properties
// (m:rPr). Items holds the ordered run content: *Text for m:t and *Raw for
// everything else — including the optional WordprocessingML w:rPr (which the
// schema places right after m:rPr) and w:EG_RunInnerContent children such as
// w:br.
type Run struct {
	RPr   *RunPr
	Items []RunChild
}

// UnmarshalXML implements xml.Unmarshaler.
func (r *Run) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.CharData:
			// Character data directly in m:r (text belongs in m:t) is
			// non-conformant; keep it in position rather than drop it.
			if text := captureCharData(t); text != nil {
				r.Items = append(r.Items, text)
			}
		case xml.StartElement:
			if t.Name.Space == NS {
				switch t.Name.Local {
				case "rPr":
					// A repeated m:rPr is non-conformant; the second used to
					// overwrite the first silently. Keep the first and let the
					// duplicate fall through to the raw capture (C471).
					if r.RPr != nil {
						break
					}
					v := &RunPr{}
					if err := d.DecodeElement(v, &t); err != nil {
						return err
					}
					r.RPr = v
					continue
				case "t":
					v := &Text{}
					if err := d.DecodeElement(v, &t); err != nil {
						return err
					}
					r.Items = append(r.Items, v)
					continue
				}
			}
			raw := &Raw{}
			if err := raw.UnmarshalXML(d, t); err != nil {
				return err
			}
			r.Items = append(r.Items, raw)
		case xml.EndElement:
			return nil
		}
	}
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (r *Run) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	if r.RPr == nil && len(r.Items) == 0 {
		b.EmptyElement(ns, localName)
		return
	}
	b.StartElement(ns, localName)
	if r.RPr != nil {
		r.RPr.MarshalToBuilder(b, NS, "rPr")
	}
	for _, it := range r.Items {
		it.emitRunChild(b)
	}
	b.EndElement(ns, localName)
}

func (r *Run) emitMath(b *xmlb.Builder) { r.MarshalToBuilder(b, NS, "r") }

// Text returns the concatenated m:t content of the run.
func (r *Run) Text() string {
	var sb strings.Builder
	for _, it := range r.Items {
		if t, ok := it.(*Text); ok {
			sb.WriteString(t.Value)
		}
	}
	return sb.String()
}

// RunPr represents CT_RPR (m:rPr), the math run properties.
type RunPr struct {
	Lit *OnOff  // literal: render exactly as entered
	Nor *OnOff  // normal (non-math) text
	Scr *Script // script style
	Sty *Style  // emphasis style
	Brk *Break  // manual break
	Aln *OnOff  // alignment point

	extra []extraChild
}

var runPrFields = seqFields(RunPr{},
	"lit=Lit", "nor=Nor", "scr=Scr", "sty=Sty", "brk=Brk", "aln=Aln")

// UnmarshalXML implements xml.Unmarshaler.
func (v *RunPr) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	return unmarshalSeq(d, v, runPrFields, &v.extra)
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *RunPr) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	marshalSeq(b, ns, localName, v, runPrFields, v.extra)
}

// CtrlPr represents CT_CtrlPr (m:ctrlPr), the formatting of a structure's
// control characters (fraction bars, delimiters, ...). Its schema content is
// WordprocessingML (w:rPr, or tracked w:ins / w:del), which common/omml
// raw-captures verbatim and in order.
type CtrlPr struct {
	Items []*Raw
}

// UnmarshalXML implements xml.Unmarshaler.
func (v *CtrlPr) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			raw := &Raw{}
			if err := raw.UnmarshalXML(d, t); err != nil {
				return err
			}
			v.Items = append(v.Items, raw)
		case xml.EndElement:
			return nil
		}
	}
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *CtrlPr) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	if len(v.Items) == 0 {
		b.EmptyElement(ns, localName)
		return
	}
	b.StartElement(ns, localName)
	for _, it := range v.Items {
		it.marshal(b)
	}
	b.EndElement(ns, localName)
}
