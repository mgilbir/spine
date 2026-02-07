package oxml

import (
	"encoding/xml"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// CT_RPr represents run properties (w:rPr).
type CT_RPr struct {
	RStyle        *CT_String           `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main rStyle,omitempty"`
	RFonts        *CT_Fonts            `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main rFonts,omitempty"`
	B             *CT_OnOff            `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main b,omitempty"`
	BCs           *CT_OnOff            `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main bCs,omitempty"`
	I             *CT_OnOff            `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main i,omitempty"`
	ICs           *CT_OnOff            `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main iCs,omitempty"`
	Caps          *CT_OnOff            `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main caps,omitempty"`
	SmallCaps     *CT_OnOff            `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main smallCaps,omitempty"`
	Strike        *CT_OnOff            `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main strike,omitempty"`
	Dstrike       *CT_OnOff            `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main dstrike,omitempty"`
	Outline       *CT_OnOff            `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main outline,omitempty"`
	Shadow        *CT_OnOff            `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main shadow,omitempty"`
	Emboss        *CT_OnOff            `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main emboss,omitempty"`
	Imprint       *CT_OnOff            `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main imprint,omitempty"`
	NoProof       *CT_OnOff            `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main noProof,omitempty"`
	SnapToGrid    *CT_OnOff            `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main snapToGrid,omitempty"`
	Vanish        *CT_OnOff            `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main vanish,omitempty"`
	WebHidden     *CT_OnOff            `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main webHidden,omitempty"`
	Color         *CT_Color            `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main color,omitempty"`
	Spacing       *CT_SignedTwipsMeasure `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main spacing,omitempty"`
	W             *CT_TextScale        `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main w,omitempty"`
	Kern          *CT_HpsMeasure       `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main kern,omitempty"`
	Position      *CT_SignedHpsMeasure  `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main position,omitempty"`
	Sz            *CT_HpsMeasure       `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main sz,omitempty"`
	SzCs          *CT_HpsMeasure       `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main szCs,omitempty"`
	Highlight     *CT_Highlight        `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main highlight,omitempty"`
	U             *CT_Underline        `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main u,omitempty"`
	Effect        *CT_String           `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main effect,omitempty"`
	Bdr           *CT_Border           `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main bdr,omitempty"`
	Shd           *CT_Shd              `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main shd,omitempty"`
	FitText       *CT_FitText          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main fitText,omitempty"`
	VertAlign     *CT_VerticalAlignRun `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main vertAlign,omitempty"`
	Rtl           *CT_OnOff            `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main rtl,omitempty"`
	Cs            *CT_OnOff            `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main cs,omitempty"`
	Em            *CT_Em               `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main em,omitempty"`
	Lang          *CT_Lang             `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main lang,omitempty"`
	SpecVanish    *CT_OnOff            `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main specVanish,omitempty"`
	OMatch        *CT_OnOff            `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main oMath,omitempty"`
	RPrChange     *CT_RPrChange        `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main rPrChange,omitempty"`
}

// CT_RPrChange represents a revision of run properties.
type CT_RPrChange struct {
	Id     string  `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main id,attr"`
	Author string  `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main author,attr"`
	Date   string  `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main date,attr,omitempty"`
	RPr    *CT_RPr `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main rPr,omitempty"`
}

// CT_Text represents text content with xml:space attribute.
type CT_Text struct {
	Space string `xml:"http://www.w3.org/XML/1998/namespace space,attr,omitempty"`
	Text  string `xml:",chardata"`
}

// CT_Br represents a break element.
type CT_Br struct {
	Type  string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main type,attr,omitempty"`
	Clear string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main clear,attr,omitempty"`
}

// CT_Sym represents a symbol character.
type CT_Sym struct {
	Font string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main font,attr,omitempty"`
	Char string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main char,attr,omitempty"`
}

// CT_FtnEdnRef represents a footnote/endnote reference.
type CT_FtnEdnRef struct {
	Id string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main id,attr"`
}

// CT_R represents a run of content (w:r).
// Uses custom UnmarshalXML/MarshalToBuilder for child ordering.
type CT_R struct {
	RsidRPr string  `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main rsidRPr,attr,omitempty"`
	RsidDel string  `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main rsidDel,attr,omitempty"`
	RsidR   string  `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main rsidR,attr,omitempty"`
	RPr     *CT_RPr `xml:"-"`

	// Content children tracked in order
	T          []*CT_Text      `xml:"-"`
	Br         []*CT_Br        `xml:"-"`
	Tab        []*CT_Empty     `xml:"-"`
	Cr         []*CT_Empty     `xml:"-"`
	Sym        []*CT_Sym       `xml:"-"`
	Drawing    []*CT_Drawing   `xml:"-"`
	FtnRef     []*CT_FtnEdnRef `xml:"-"`
	EndnoteRef []*CT_FtnEdnRef `xml:"-"`
	LastRenderedPageBreak []*CT_Empty `xml:"-"`
	NoBreakHyphen []*CT_Empty `xml:"-"`
	SoftHyphen    []*CT_Empty `xml:"-"`
	FldChar       []*CT_FldChar `xml:"-"`
	InstrText     []*CT_Text    `xml:"-"`
	childOrder []runChildRef
}

// runChildKind identifies a run child element type.
type runChildKind int

const (
	runChildT runChildKind = iota
	runChildBr
	runChildTab
	runChildCr
	runChildSym
	runChildDrawing
	runChildFtnRef
	runChildEndnoteRef
	runChildLastRenderedPageBreak
	runChildNoBreakHyphen
	runChildSoftHyphen
	runChildFldChar
	runChildInstrText
)

// runChildRef references a child element by kind and index.
type runChildRef struct {
	kind  runChildKind
	index int
}

// UnmarshalXML implements custom unmarshaling for CT_R to preserve child order.
func (r *CT_R) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		switch attr.Name.Local {
		case "rsidRPr":
			r.RsidRPr = attr.Value
		case "rsidDel":
			r.RsidDel = attr.Value
		case "rsidR":
			r.RsidR = attr.Value
		}
	}

	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "rPr":
				r.RPr = &CT_RPr{}
				if err := d.DecodeElement(r.RPr, &t); err != nil {
					return err
				}
			case "t":
				v := &CT_Text{}
				if err := d.DecodeElement(v, &t); err != nil {
					return err
				}
				r.childOrder = append(r.childOrder, runChildRef{runChildT, len(r.T)})
				r.T = append(r.T, v)
			case "br":
				v := &CT_Br{}
				if err := d.DecodeElement(v, &t); err != nil {
					return err
				}
				r.childOrder = append(r.childOrder, runChildRef{runChildBr, len(r.Br)})
				r.Br = append(r.Br, v)
			case "tab":
				v := &CT_Empty{}
				if err := d.Skip(); err != nil {
					return err
				}
				r.childOrder = append(r.childOrder, runChildRef{runChildTab, len(r.Tab)})
				r.Tab = append(r.Tab, v)
			case "cr":
				v := &CT_Empty{}
				if err := d.Skip(); err != nil {
					return err
				}
				r.childOrder = append(r.childOrder, runChildRef{runChildCr, len(r.Cr)})
				r.Cr = append(r.Cr, v)
			case "sym":
				v := &CT_Sym{}
				if err := d.DecodeElement(v, &t); err != nil {
					return err
				}
				r.childOrder = append(r.childOrder, runChildRef{runChildSym, len(r.Sym)})
				r.Sym = append(r.Sym, v)
			case "drawing":
				v := &CT_Drawing{}
				if err := d.DecodeElement(v, &t); err != nil {
					return err
				}
				r.childOrder = append(r.childOrder, runChildRef{runChildDrawing, len(r.Drawing)})
				r.Drawing = append(r.Drawing, v)
			case "footnoteReference":
				v := &CT_FtnEdnRef{}
				if err := d.DecodeElement(v, &t); err != nil {
					return err
				}
				r.childOrder = append(r.childOrder, runChildRef{runChildFtnRef, len(r.FtnRef)})
				r.FtnRef = append(r.FtnRef, v)
			case "endnoteReference":
				v := &CT_FtnEdnRef{}
				if err := d.DecodeElement(v, &t); err != nil {
					return err
				}
				r.childOrder = append(r.childOrder, runChildRef{runChildEndnoteRef, len(r.EndnoteRef)})
				r.EndnoteRef = append(r.EndnoteRef, v)
			case "lastRenderedPageBreak":
				v := &CT_Empty{}
				if err := d.Skip(); err != nil {
					return err
				}
				r.childOrder = append(r.childOrder, runChildRef{runChildLastRenderedPageBreak, len(r.LastRenderedPageBreak)})
				r.LastRenderedPageBreak = append(r.LastRenderedPageBreak, v)
			case "noBreakHyphen":
				v := &CT_Empty{}
				if err := d.Skip(); err != nil {
					return err
				}
				r.childOrder = append(r.childOrder, runChildRef{runChildNoBreakHyphen, len(r.NoBreakHyphen)})
				r.NoBreakHyphen = append(r.NoBreakHyphen, v)
			case "softHyphen":
				v := &CT_Empty{}
				if err := d.Skip(); err != nil {
					return err
				}
				r.childOrder = append(r.childOrder, runChildRef{runChildSoftHyphen, len(r.SoftHyphen)})
				r.SoftHyphen = append(r.SoftHyphen, v)
			case "fldChar":
				v := &CT_FldChar{}
				if err := d.DecodeElement(v, &t); err != nil {
					return err
				}
				r.childOrder = append(r.childOrder, runChildRef{runChildFldChar, len(r.FldChar)})
				r.FldChar = append(r.FldChar, v)
			case "instrText":
				v := &CT_Text{}
				if err := d.DecodeElement(v, &t); err != nil {
					return err
				}
				r.childOrder = append(r.childOrder, runChildRef{runChildInstrText, len(r.InstrText)})
				r.InstrText = append(r.InstrText, v)
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

// MarshalToBuilder implements xmlb.BuilderMarshaler to preserve child element order.
func (r *CT_R) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	var attrs []xmlb.Attr
	if r.RsidRPr != "" {
		attrs = append(attrs, xmlb.StrAttr("rsidRPr", r.RsidRPr))
	}
	if r.RsidDel != "" {
		attrs = append(attrs, xmlb.StrAttr("rsidDel", r.RsidDel))
	}
	if r.RsidR != "" {
		attrs = append(attrs, xmlb.StrAttr("rsidR", r.RsidR))
	}
	b.StartElement(ns, localName, attrs...)

	if r.RPr != nil {
		b.MarshalElement(ns, "rPr", r.RPr)
	}

	if len(r.childOrder) > 0 {
		for _, ref := range r.childOrder {
			switch ref.kind {
			case runChildT:
				if ref.index < len(r.T) {
					marshalText(b, ns, "t", r.T[ref.index])
				}
			case runChildBr:
				if ref.index < len(r.Br) {
					b.MarshalElement(ns, "br", r.Br[ref.index])
				}
			case runChildTab:
				if ref.index < len(r.Tab) {
					b.EmptyElement(ns, "tab")
				}
			case runChildCr:
				if ref.index < len(r.Cr) {
					b.EmptyElement(ns, "cr")
				}
			case runChildSym:
				if ref.index < len(r.Sym) {
					b.MarshalElement(ns, "sym", r.Sym[ref.index])
				}
			case runChildDrawing:
				if ref.index < len(r.Drawing) {
					r.Drawing[ref.index].MarshalToBuilder(b, ns, "drawing")
				}
			case runChildFtnRef:
				if ref.index < len(r.FtnRef) {
					b.MarshalElement(ns, "footnoteReference", r.FtnRef[ref.index])
				}
			case runChildEndnoteRef:
				if ref.index < len(r.EndnoteRef) {
					b.MarshalElement(ns, "endnoteReference", r.EndnoteRef[ref.index])
				}
			case runChildLastRenderedPageBreak:
				if ref.index < len(r.LastRenderedPageBreak) {
					b.EmptyElement(ns, "lastRenderedPageBreak")
				}
			case runChildNoBreakHyphen:
				if ref.index < len(r.NoBreakHyphen) {
					b.EmptyElement(ns, "noBreakHyphen")
				}
			case runChildSoftHyphen:
				if ref.index < len(r.SoftHyphen) {
					b.EmptyElement(ns, "softHyphen")
				}
			case runChildFldChar:
				if ref.index < len(r.FldChar) {
					b.MarshalElement(ns, "fldChar", r.FldChar[ref.index])
				}
			case runChildInstrText:
				if ref.index < len(r.InstrText) {
					marshalText(b, ns, "instrText", r.InstrText[ref.index])
				}
			}
		}
	} else {
		for _, v := range r.T {
			marshalText(b, ns, "t", v)
		}
		for _, v := range r.Br {
			b.MarshalElement(ns, "br", v)
		}
	}

	b.EndElement(ns, localName)
}

// marshalText writes a text element with xml:space handling.
func marshalText(b *xmlb.Builder, ns, localName string, t *CT_Text) {
	var attrs []xmlb.Attr
	if t.Space != "" {
		attrs = append(attrs, xmlb.Attr{Name: "xml:space", Value: t.Space})
	}
	b.WriteElement(ns, localName, t.Text, attrs...)
}
