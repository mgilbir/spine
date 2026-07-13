package oxml

import (
	"encoding/xml"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// CT_RPr represents run properties (w:rPr).
type CT_RPr struct {
	RStyle *CT_String `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main rStyle,omitempty"`
	// AlternateContent captures mc:AlternateContent inside w:rPr (Word wraps
	// w:rFonts in one for markup-compat, e.g. emoji fonts via w16se). It sits
	// at the rFonts slot to match where Word emits it. Raw capture, like the
	// run-level AlternateContent.
	AlternateContent []*CT_RawElement       `xml:"http://schemas.openxmlformats.org/markup-compatibility/2006 AlternateContent,omitempty"`
	RFonts           *CT_Fonts              `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main rFonts,omitempty"`
	B                *CT_OnOff              `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main b,omitempty"`
	BCs              *CT_OnOff              `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main bCs,omitempty"`
	I                *CT_OnOff              `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main i,omitempty"`
	ICs              *CT_OnOff              `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main iCs,omitempty"`
	Caps             *CT_OnOff              `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main caps,omitempty"`
	SmallCaps        *CT_OnOff              `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main smallCaps,omitempty"`
	Strike           *CT_OnOff              `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main strike,omitempty"`
	Dstrike          *CT_OnOff              `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main dstrike,omitempty"`
	Outline          *CT_OnOff              `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main outline,omitempty"`
	Shadow           *CT_OnOff              `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main shadow,omitempty"`
	Emboss           *CT_OnOff              `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main emboss,omitempty"`
	Imprint          *CT_OnOff              `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main imprint,omitempty"`
	NoProof          *CT_OnOff              `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main noProof,omitempty"`
	SnapToGrid       *CT_OnOff              `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main snapToGrid,omitempty"`
	Vanish           *CT_OnOff              `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main vanish,omitempty"`
	WebHidden        *CT_OnOff              `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main webHidden,omitempty"`
	Color            *CT_Color              `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main color,omitempty"`
	Spacing          *CT_SignedTwipsMeasure `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main spacing,omitempty"`
	W                *CT_TextScale          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main w,omitempty"`
	Kern             *CT_HpsMeasure         `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main kern,omitempty"`
	Position         *CT_SignedHpsMeasure   `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main position,omitempty"`
	Sz               *CT_HpsMeasure         `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main sz,omitempty"`
	SzCs             *CT_HpsMeasure         `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main szCs,omitempty"`
	Highlight        *CT_Highlight          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main highlight,omitempty"`
	U                *CT_Underline          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main u,omitempty"`
	Effect           *CT_String             `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main effect,omitempty"`
	Bdr              *CT_Border             `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main bdr,omitempty"`
	Shd              *CT_Shd                `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main shd,omitempty"`
	FitText          *CT_FitText            `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main fitText,omitempty"`
	VertAlign        *CT_VerticalAlignRun   `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main vertAlign,omitempty"`
	Rtl              *CT_OnOff              `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main rtl,omitempty"`
	Cs               *CT_OnOff              `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main cs,omitempty"`
	Em               *CT_Em                 `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main em,omitempty"`
	Lang             *CT_Lang               `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main lang,omitempty"`
	SpecVanish       *CT_OnOff              `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main specVanish,omitempty"`
	OMatch           *CT_OnOff              `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main oMath,omitempty"`
	RPrChange        *CT_RPrChange          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main rPrChange,omitempty"`
	// Ligatures models w14:ligatures (Word 2010 extension). Word emits it as
	// the last w:rPr child (corpus: 6,530/6,530 instances precede </w:rPr>),
	// so it sits last here; previously it was silently dropped.
	Ligatures *CT_Word2010Val `xml:"http://schemas.microsoft.com/office/word/2010/wordml ligatures,omitempty"`
}

// CT_Word2010Val is a Word 2010 extension element carrying a single w14:val
// attribute (e.g. w14:ligatures).
type CT_Word2010Val struct {
	Val string `xml:"http://schemas.microsoft.com/office/word/2010/wordml val,attr"`
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

// CT_FtnEdnRef represents a footnote/endnote reference. Word emits
// w:customMarkFollows before w:id.
type CT_FtnEdnRef struct {
	CustomMarkFollows string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main customMarkFollows,attr,omitempty"`
	Id                string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main id,attr"`
}

// CT_Markup represents a markup element carrying only a w:id attribute
// (w:commentReference — the run-level anchor tying a comment range to its
// entry in comments.xml).
type CT_Markup struct {
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
	T                     []*CT_Text       `xml:"-"`
	Br                    []*CT_Br         `xml:"-"`
	Tab                   []*CT_Empty      `xml:"-"`
	Cr                    []*CT_Empty      `xml:"-"`
	Sym                   []*CT_Sym        `xml:"-"`
	Drawing               []*CT_Drawing    `xml:"-"`
	FtnRef                []*CT_FtnEdnRef  `xml:"-"`
	EndnoteRef            []*CT_FtnEdnRef  `xml:"-"`
	LastRenderedPageBreak []*CT_Empty      `xml:"-"`
	NoBreakHyphen         []*CT_Empty      `xml:"-"`
	SoftHyphen            []*CT_Empty      `xml:"-"`
	FldChar               []*CT_FldChar    `xml:"-"`
	InstrText             []*CT_Text       `xml:"-"`
	DelText               []*CT_Text       `xml:"-"` // w:delText - tracked-deletion text
	CommentReference      []*CT_Markup     `xml:"-"`
	Ptab                  []*CT_Ptab       `xml:"-"`
	Pict                  []*CT_RawElement `xml:"-"` // w:pict - VML content, raw
	Object                []*CT_RawElement `xml:"-"` // w:object - OLE wrapper, raw
	// AlternateContent holds mc:AlternateContent children (drawings with VML
	// fallbacks, textboxes). Captured raw like Pict/Object: Word emits
	// mc:Choice/mc:Fallback with inline xmlns declarations and no xmlns=""
	// reset, which the typed common/oxml AlternateContent would not reproduce.
	AlternateContent []*CT_RawElement `xml:"-"`
	childOrder       []runChildRef
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
	runChildDelText
	runChildCommentReference
	runChildPtab
	runChildPict
	runChildObject
	runChildAlternateContent
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
			case "delText":
				v := &CT_Text{}
				if err := d.DecodeElement(v, &t); err != nil {
					return err
				}
				r.childOrder = append(r.childOrder, runChildRef{runChildDelText, len(r.DelText)})
				r.DelText = append(r.DelText, v)
			case "commentReference":
				v := &CT_Markup{}
				if err := d.DecodeElement(v, &t); err != nil {
					return err
				}
				r.childOrder = append(r.childOrder, runChildRef{runChildCommentReference, len(r.CommentReference)})
				r.CommentReference = append(r.CommentReference, v)
			case "ptab":
				v := &CT_Ptab{}
				if err := d.DecodeElement(v, &t); err != nil {
					return err
				}
				r.childOrder = append(r.childOrder, runChildRef{runChildPtab, len(r.Ptab)})
				r.Ptab = append(r.Ptab, v)
			case "pict":
				v := &CT_RawElement{}
				if err := d.DecodeElement(v, &t); err != nil {
					return err
				}
				r.childOrder = append(r.childOrder, runChildRef{runChildPict, len(r.Pict)})
				r.Pict = append(r.Pict, v)
			case "object":
				v := &CT_RawElement{}
				if err := d.DecodeElement(v, &t); err != nil {
					return err
				}
				r.childOrder = append(r.childOrder, runChildRef{runChildObject, len(r.Object)})
				r.Object = append(r.Object, v)
			case "AlternateContent":
				v := &CT_RawElement{}
				if err := d.DecodeElement(v, &t); err != nil {
					return err
				}
				r.childOrder = append(r.childOrder, runChildRef{runChildAlternateContent, len(r.AlternateContent)})
				r.AlternateContent = append(r.AlternateContent, v)
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
	// Word's attribute order: rsidDel, rsidR, rsidRPr (not the XSD order).
	var attrs []xmlb.Attr
	if r.RsidDel != "" {
		attrs = append(attrs, xmlb.Attr{Namespace: xmlb.NSWordprocessingML, Name: "rsidDel", Value: r.RsidDel})
	}
	if r.RsidR != "" {
		attrs = append(attrs, xmlb.Attr{Namespace: xmlb.NSWordprocessingML, Name: "rsidR", Value: r.RsidR})
	}
	if r.RsidRPr != "" {
		attrs = append(attrs, xmlb.Attr{Namespace: xmlb.NSWordprocessingML, Name: "rsidRPr", Value: r.RsidRPr})
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
			case runChildDelText:
				if ref.index < len(r.DelText) {
					marshalText(b, ns, "delText", r.DelText[ref.index])
				}
			case runChildCommentReference:
				if ref.index < len(r.CommentReference) {
					b.MarshalElement(ns, "commentReference", r.CommentReference[ref.index])
				}
			case runChildPtab:
				if ref.index < len(r.Ptab) {
					b.MarshalElement(ns, "ptab", r.Ptab[ref.index])
				}
			case runChildPict:
				if ref.index < len(r.Pict) {
					r.Pict[ref.index].MarshalToBuilder(b, ns, "pict")
				}
			case runChildObject:
				if ref.index < len(r.Object) {
					r.Object[ref.index].MarshalToBuilder(b, ns, "object")
				}
			case runChildAlternateContent:
				if ref.index < len(r.AlternateContent) {
					r.AlternateContent[ref.index].MarshalToBuilder(b, xmlb.NSMarkupCompatibility, "AlternateContent")
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

// backfillChildOrder records any existing untracked children in childOrder,
// grouped by kind in slice order. A run built programmatically (e.g. SetTexts
// on an empty order writes r.T without tracking) has typed children but an
// empty childOrder; the first tracked append would otherwise flip marshaling
// to the childOrder-only path and silently drop them.
func (r *CT_R) backfillChildOrder() {
	if len(r.childOrder) > 0 {
		return
	}
	for i := range r.T {
		r.childOrder = append(r.childOrder, runChildRef{runChildT, i})
	}
	for i := range r.Br {
		r.childOrder = append(r.childOrder, runChildRef{runChildBr, i})
	}
	for i := range r.Tab {
		r.childOrder = append(r.childOrder, runChildRef{runChildTab, i})
	}
	for i := range r.Cr {
		r.childOrder = append(r.childOrder, runChildRef{runChildCr, i})
	}
	for i := range r.Sym {
		r.childOrder = append(r.childOrder, runChildRef{runChildSym, i})
	}
	for i := range r.Drawing {
		r.childOrder = append(r.childOrder, runChildRef{runChildDrawing, i})
	}
	for i := range r.FtnRef {
		r.childOrder = append(r.childOrder, runChildRef{runChildFtnRef, i})
	}
	for i := range r.EndnoteRef {
		r.childOrder = append(r.childOrder, runChildRef{runChildEndnoteRef, i})
	}
	for i := range r.LastRenderedPageBreak {
		r.childOrder = append(r.childOrder, runChildRef{runChildLastRenderedPageBreak, i})
	}
	for i := range r.NoBreakHyphen {
		r.childOrder = append(r.childOrder, runChildRef{runChildNoBreakHyphen, i})
	}
	for i := range r.SoftHyphen {
		r.childOrder = append(r.childOrder, runChildRef{runChildSoftHyphen, i})
	}
	for i := range r.FldChar {
		r.childOrder = append(r.childOrder, runChildRef{runChildFldChar, i})
	}
	for i := range r.InstrText {
		r.childOrder = append(r.childOrder, runChildRef{runChildInstrText, i})
	}
	for i := range r.DelText {
		r.childOrder = append(r.childOrder, runChildRef{runChildDelText, i})
	}
	for i := range r.CommentReference {
		r.childOrder = append(r.childOrder, runChildRef{runChildCommentReference, i})
	}
	for i := range r.Ptab {
		r.childOrder = append(r.childOrder, runChildRef{runChildPtab, i})
	}
	for i := range r.Pict {
		r.childOrder = append(r.childOrder, runChildRef{runChildPict, i})
	}
	for i := range r.Object {
		r.childOrder = append(r.childOrder, runChildRef{runChildObject, i})
	}
	for i := range r.AlternateContent {
		r.childOrder = append(r.childOrder, runChildRef{runChildAlternateContent, i})
	}
}

// AppendDrawing appends a drawing to the run, maintaining child order.
// Existing untracked children are backfilled into the order first.
func (r *CT_R) AppendDrawing(d *CT_Drawing) {
	r.backfillChildOrder()
	r.childOrder = append(r.childOrder, runChildRef{runChildDrawing, len(r.Drawing)})
	r.Drawing = append(r.Drawing, d)
}

// AppendBr appends a break to the run, maintaining child order (see AppendDrawing).
func (r *CT_R) AppendBr(br *CT_Br) {
	r.backfillChildOrder()
	r.childOrder = append(r.childOrder, runChildRef{runChildBr, len(r.Br)})
	r.Br = append(r.Br, br)
}

// AppendTab appends a tab to the run, maintaining child order (see AppendDrawing).
func (r *CT_R) AppendTab() {
	r.backfillChildOrder()
	r.childOrder = append(r.childOrder, runChildRef{runChildTab, len(r.Tab)})
	r.Tab = append(r.Tab, &CT_Empty{})
}

// AppendFldChar appends a field character (w:fldChar begin/separate/end) to
// the run, maintaining child order (see AppendDrawing).
func (r *CT_R) AppendFldChar(fc *CT_FldChar) {
	r.backfillChildOrder()
	r.childOrder = append(r.childOrder, runChildRef{runChildFldChar, len(r.FldChar)})
	r.FldChar = append(r.FldChar, fc)
}

// AppendInstrText appends field instruction text (w:instrText) to the run,
// maintaining child order (see AppendDrawing).
func (r *CT_R) AppendInstrText(t *CT_Text) {
	r.backfillChildOrder()
	r.childOrder = append(r.childOrder, runChildRef{runChildInstrText, len(r.InstrText)})
	r.InstrText = append(r.InstrText, t)
}

// SetTexts replaces the run's text elements with ts, keeping childOrder
// consistent so stale text references neither drop nor duplicate content. On
// a tracked run the new texts take the position of the first existing text
// reference and all other text references are removed; a run with no recorded
// order stays untracked (the fallback marshal writes texts directly).
func (r *CT_R) SetTexts(ts []*CT_Text) {
	r.T = ts
	if len(r.childOrder) == 0 {
		return
	}
	newOrder := make([]runChildRef, 0, len(r.childOrder)+len(ts))
	inserted := false
	for _, ref := range r.childOrder {
		if ref.kind == runChildT {
			if !inserted {
				for i := range ts {
					newOrder = append(newOrder, runChildRef{runChildT, i})
				}
				inserted = true
			}
			continue
		}
		newOrder = append(newOrder, ref)
	}
	if !inserted {
		for i := range ts {
			newOrder = append(newOrder, runChildRef{runChildT, i})
		}
	}
	r.childOrder = newOrder
}

// ClearContent removes all content children from the run, including the
// recorded child order, so later appends do not resolve stale references to
// the new content and duplicate it.
func (r *CT_R) ClearContent() {
	r.T = nil
	r.Br = nil
	r.Tab = nil
	r.Cr = nil
	r.Sym = nil
	r.Drawing = nil
	r.FtnRef = nil
	r.EndnoteRef = nil
	r.LastRenderedPageBreak = nil
	r.NoBreakHyphen = nil
	r.SoftHyphen = nil
	r.FldChar = nil
	r.InstrText = nil
	r.DelText = nil
	r.CommentReference = nil
	r.Ptab = nil
	r.Pict = nil
	r.Object = nil
	r.AlternateContent = nil
	r.childOrder = nil
}

// marshalText writes a text element with xml:space handling.
func marshalText(b *xmlb.Builder, ns, localName string, t *CT_Text) {
	var attrs []xmlb.Attr
	if t.Space != "" {
		attrs = append(attrs, xmlb.Attr{Name: "xml:space", Value: t.Space})
	}
	b.WriteElement(ns, localName, t.Text, attrs...)
}
