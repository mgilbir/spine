// This file provides DrawingML XML text types from dml-main.xsd.

package dml

import (
	"encoding/xml"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// TxBody represents CT_TextBody (a:txBody)
type TxBody struct {
	BodyPr   *BodyPr   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main bodyPr,omitempty"`
	LstStyle *LstStyle `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lstStyle,omitempty"`
	P        []*P      `xml:"http://schemas.openxmlformats.org/drawingml/2006/main p"`
}

// BodyPr represents CT_TextBodyProperties (a:bodyPr)
type BodyPr struct {
	// Rot is a pointer so an explicit rot="0" in the source survives the
	// round trip (omitempty on a value field dropped it).
	Rot              *int32 `xml:"rot,attr,omitempty"`
	SpcFirstLastPara *bool  `xml:"spcFirstLastPara,attr,omitempty"`
	VertOverflow     string `xml:"vertOverflow,attr,omitempty"`
	HorzOverflow     string `xml:"horzOverflow,attr,omitempty"`
	Vert             string `xml:"vert,attr,omitempty"`
	Wrap             string `xml:"wrap,attr,omitempty"`
	LIns             *int64 `xml:"lIns,attr,omitempty"`
	TIns             *int64 `xml:"tIns,attr,omitempty"`
	RIns             *int64 `xml:"rIns,attr,omitempty"`
	BIns             *int64 `xml:"bIns,attr,omitempty"`
	NumCol           int32  `xml:"numCol,attr,omitempty"`
	// SpcCol is a pointer so an explicit spcCol="0" survives the round trip.
	SpcCol        *int32          `xml:"spcCol,attr,omitempty"`
	RtlCol        *bool           `xml:"rtlCol,attr,omitempty"`
	FromWordArt   *bool           `xml:"fromWordArt,attr,omitempty"`
	Anchor        string          `xml:"anchor,attr,omitempty"`
	AnchorCtr     *bool           `xml:"anchorCtr,attr,omitempty"`
	ForceAA       *bool           `xml:"forceAA,attr,omitempty"`
	UpRight       *bool           `xml:"upright,attr,omitempty"`
	CompatLnSpc   *bool           `xml:"compatLnSpc,attr,omitempty"`
	PrstTxWarp    *PrstTxWarp     `xml:"http://schemas.openxmlformats.org/drawingml/2006/main prstTxWarp,omitempty"`
	NoAutofit     *NoAutofit      `xml:"http://schemas.openxmlformats.org/drawingml/2006/main noAutofit,omitempty"`
	NormAutofit   *NormAutofit    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main normAutofit,omitempty"`
	SpAutoFit     *SpAutoFit      `xml:"http://schemas.openxmlformats.org/drawingml/2006/main spAutoFit,omitempty"`
	Scene3d       *Scene3d        `xml:"http://schemas.openxmlformats.org/drawingml/2006/main scene3d,omitempty"`
	Sp3d          *Sp3d           `xml:"http://schemas.openxmlformats.org/drawingml/2006/main sp3d,omitempty"`
	FlatTx        *FlatTx         `xml:"http://schemas.openxmlformats.org/drawingml/2006/main flatTx,omitempty"`
	ExtLst        *ExtLst         `xml:"http://schemas.openxmlformats.org/drawingml/2006/main extLst,omitempty"`
	CapturedAttrs []xmlb.RootAttr `xml:"-"` // verbatim source attrs; see common/xml.CaptureAttrs
}

// UnmarshalXML captures the element's verbatim attribute list (source
// attribute order and any unmodeled attributes) before decoding through the
// struct tags; the reflection marshaler replays it.
func (bp *BodyPr) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	bp.CapturedAttrs = xmlb.CaptureAttrs(start.Attr)
	type alias BodyPr
	return d.DecodeElement((*alias)(bp), &start)
}

// PrstTxWarp represents CT_PresetTextShape (a:prstTxWarp)
type PrstTxWarp struct {
	Prst  string `xml:"prst,attr"`
	AvLst *AvLst `xml:"http://schemas.openxmlformats.org/drawingml/2006/main avLst,omitempty"`
}

// NoAutofit represents CT_TextNoAutofit (a:noAutofit)
type NoAutofit struct{}

// NormAutofit represents CT_TextNormalAutofit (a:normAutofit)
type NormAutofit struct {
	FontScale      Percentage `xml:"fontScale,attr,omitempty"`
	LnSpcReduction Percentage `xml:"lnSpcReduction,attr,omitempty"`
}

// SpAutoFit represents CT_TextShapeAutofit (a:spAutoFit)
type SpAutoFit struct{}

// FlatTx represents CT_FlatText (a:flatTx)
type FlatTx struct {
	Z int64 `xml:"z,attr,omitempty"`
}

// LstStyle represents CT_TextListStyle (a:lstStyle)
type LstStyle struct {
	DefPPr  *PPr `xml:"http://schemas.openxmlformats.org/drawingml/2006/main defPPr,omitempty"`
	Lvl1pPr *PPr `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lvl1pPr,omitempty"`
	Lvl2pPr *PPr `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lvl2pPr,omitempty"`
	Lvl3pPr *PPr `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lvl3pPr,omitempty"`
	Lvl4pPr *PPr `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lvl4pPr,omitempty"`
	Lvl5pPr *PPr `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lvl5pPr,omitempty"`
	Lvl6pPr *PPr `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lvl6pPr,omitempty"`
	Lvl7pPr *PPr `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lvl7pPr,omitempty"`
	Lvl8pPr *PPr `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lvl8pPr,omitempty"`
	Lvl9pPr *PPr `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lvl9pPr,omitempty"`
}

// P represents CT_TextParagraph (a:p).
// It implements custom unmarshal/marshal to preserve interleaved R/Br/Fld order
// (per XSD: xs:choice maxOccurs="unbounded").
type P struct {
	PPr        *PPr        `xml:"http://schemas.openxmlformats.org/drawingml/2006/main pPr,omitempty"`
	R          []*R        `xml:"-"`
	Br         []*Br       `xml:"-"`
	Fld        []*Fld      `xml:"-"`
	EndParaRPr *RPr        `xml:"http://schemas.openxmlformats.org/drawingml/2006/main endParaRPr,omitempty"`
	childOrder []pChildRef // tracks interleaved child order
}

type pChildKind int

const (
	pChildR pChildKind = iota
	pChildBr
	pChildFld
)

type pChildRef struct {
	kind  pChildKind
	index int
}

// ResetRunOrder rebuilds the paragraph child order as run-only content.
// This is useful after code mutates the run slice on paragraphs that do not
// contain line breaks or fields.
func (p *P) ResetRunOrder() {
	p.childOrder = p.childOrder[:0]
	for i := range p.R {
		p.childOrder = append(p.childOrder, pChildRef{pChildR, i})
	}
}

// MapRunSegments rewrites the paragraph's runs segment by segment: fn is
// called once for each maximal sequence of consecutive a:r children
// (uninterrupted by a:br or a:fld) in the preserved child order, and its
// return value replaces that segment's runs. The run slice and child order
// are rebuilt accordingly; br and fld children stay in place. A paragraph
// without recorded child order is treated as a single run segment.
func (p *P) MapRunSegments(fn func(runs []*R) []*R) {
	if len(p.childOrder) == 0 {
		p.R = fn(p.R)
		if len(p.Br) == 0 && len(p.Fld) == 0 {
			p.ResetRunOrder()
		}
		return
	}

	var (
		newR     []*R
		newOrder []pChildRef
		segment  []*R
	)
	flush := func() {
		if len(segment) == 0 {
			return
		}
		for _, r := range fn(segment) {
			newOrder = append(newOrder, pChildRef{pChildR, len(newR)})
			newR = append(newR, r)
		}
		segment = nil
	}
	for _, ref := range p.childOrder {
		switch ref.kind {
		case pChildR:
			if ref.index < len(p.R) {
				segment = append(segment, p.R[ref.index])
			}
		default:
			flush()
			newOrder = append(newOrder, ref)
		}
	}
	flush()
	p.R = newR
	p.childOrder = newOrder
}

// UnmarshalXML implements custom unmarshaling for P to preserve child order.
func (p *P) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "pPr":
				p.PPr = &PPr{}
				if err := d.DecodeElement(p.PPr, &t); err != nil {
					return err
				}
			case "r":
				r := &R{}
				if err := d.DecodeElement(r, &t); err != nil {
					return err
				}
				p.childOrder = append(p.childOrder, pChildRef{pChildR, len(p.R)})
				p.R = append(p.R, r)
			case "br":
				br := &Br{}
				if err := d.DecodeElement(br, &t); err != nil {
					return err
				}
				p.childOrder = append(p.childOrder, pChildRef{pChildBr, len(p.Br)})
				p.Br = append(p.Br, br)
			case "fld":
				fld := &Fld{}
				if err := d.DecodeElement(fld, &t); err != nil {
					return err
				}
				p.childOrder = append(p.childOrder, pChildRef{pChildFld, len(p.Fld)})
				p.Fld = append(p.Fld, fld)
			case "endParaRPr":
				p.EndParaRPr = &RPr{}
				if err := d.DecodeElement(p.EndParaRPr, &t); err != nil {
					return err
				}
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
func (p *P) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	b.StartElement(ns, localName)

	if p.PPr != nil {
		b.MarshalElement(ns, "pPr", p.PPr)
	}

	if len(p.childOrder) > 0 {
		for _, ref := range p.childOrder {
			switch ref.kind {
			case pChildR:
				if ref.index < len(p.R) {
					b.MarshalElement(ns, "r", p.R[ref.index])
				}
			case pChildBr:
				if ref.index < len(p.Br) {
					b.MarshalElement(ns, "br", p.Br[ref.index])
				}
			case pChildFld:
				if ref.index < len(p.Fld) {
					b.MarshalElement(ns, "fld", p.Fld[ref.index])
				}
			}
		}
	} else {
		for _, r := range p.R {
			b.MarshalElement(ns, "r", r)
		}
		for _, br := range p.Br {
			b.MarshalElement(ns, "br", br)
		}
		for _, fld := range p.Fld {
			b.MarshalElement(ns, "fld", fld)
		}
	}

	if p.EndParaRPr != nil {
		b.MarshalElement(ns, "endParaRPr", p.EndParaRPr)
	}

	b.EndElement(ns, localName)
}

// MarshalXML implements xml.Marshaler for P, ensuring interleaved R/Br/Fld children
// are serialized even though they use xml:"-" struct tags.
func (p *P) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	if err := e.EncodeToken(start); err != nil {
		return err
	}

	ns := "http://schemas.openxmlformats.org/drawingml/2006/main"

	if p.PPr != nil {
		if err := e.EncodeElement(p.PPr, xml.StartElement{Name: xml.Name{Space: ns, Local: "pPr"}}); err != nil {
			return err
		}
	}

	if len(p.childOrder) > 0 {
		for _, ref := range p.childOrder {
			switch ref.kind {
			case pChildR:
				if ref.index < len(p.R) {
					if err := e.EncodeElement(p.R[ref.index], xml.StartElement{Name: xml.Name{Space: ns, Local: "r"}}); err != nil {
						return err
					}
				}
			case pChildBr:
				if ref.index < len(p.Br) {
					if err := e.EncodeElement(p.Br[ref.index], xml.StartElement{Name: xml.Name{Space: ns, Local: "br"}}); err != nil {
						return err
					}
				}
			case pChildFld:
				if ref.index < len(p.Fld) {
					if err := e.EncodeElement(p.Fld[ref.index], xml.StartElement{Name: xml.Name{Space: ns, Local: "fld"}}); err != nil {
						return err
					}
				}
			}
		}
	} else {
		for _, r := range p.R {
			if err := e.EncodeElement(r, xml.StartElement{Name: xml.Name{Space: ns, Local: "r"}}); err != nil {
				return err
			}
		}
		for _, br := range p.Br {
			if err := e.EncodeElement(br, xml.StartElement{Name: xml.Name{Space: ns, Local: "br"}}); err != nil {
				return err
			}
		}
		for _, fld := range p.Fld {
			if err := e.EncodeElement(fld, xml.StartElement{Name: xml.Name{Space: ns, Local: "fld"}}); err != nil {
				return err
			}
		}
	}

	if p.EndParaRPr != nil {
		if err := e.EncodeElement(p.EndParaRPr, xml.StartElement{Name: xml.Name{Space: ns, Local: "endParaRPr"}}); err != nil {
			return err
		}
	}

	return e.EncodeToken(start.End())
}

// PPr represents CT_TextParagraphProperties (a:pPr)
type PPr struct {
	MarL          *int32          `xml:"marL,attr,omitempty"`
	MarR          *int32          `xml:"marR,attr,omitempty"`
	Lvl           *int32          `xml:"lvl,attr,omitempty"`
	Indent        *int32          `xml:"indent,attr,omitempty"`
	Algn          string          `xml:"algn,attr,omitempty"`
	DefTabSz      *int32          `xml:"defTabSz,attr,omitempty"`
	Rtl           *bool           `xml:"rtl,attr,omitempty"`
	EaLnBrk       *bool           `xml:"eaLnBrk,attr,omitempty"`
	FontAlgn      string          `xml:"fontAlgn,attr,omitempty"`
	LatinLnBrk    *bool           `xml:"latinLnBrk,attr,omitempty"`
	HangingPunct  *bool           `xml:"hangingPunct,attr,omitempty"`
	LnSpc         *LnSpc          `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lnSpc,omitempty"`
	SpcBef        *SpcBef         `xml:"http://schemas.openxmlformats.org/drawingml/2006/main spcBef,omitempty"`
	SpcAft        *SpcAft         `xml:"http://schemas.openxmlformats.org/drawingml/2006/main spcAft,omitempty"`
	BuClrTx       *BuClrTx        `xml:"http://schemas.openxmlformats.org/drawingml/2006/main buClrTx,omitempty"`
	BuClr         *BuClr          `xml:"http://schemas.openxmlformats.org/drawingml/2006/main buClr,omitempty"`
	BuSzTx        *BuSzTx         `xml:"http://schemas.openxmlformats.org/drawingml/2006/main buSzTx,omitempty"`
	BuSzPct       *BuSzPct        `xml:"http://schemas.openxmlformats.org/drawingml/2006/main buSzPct,omitempty"`
	BuSzPts       *BuSzPts        `xml:"http://schemas.openxmlformats.org/drawingml/2006/main buSzPts,omitempty"`
	BuFontTx      *BuFontTx       `xml:"http://schemas.openxmlformats.org/drawingml/2006/main buFontTx,omitempty"`
	BuFont        *BuFont         `xml:"http://schemas.openxmlformats.org/drawingml/2006/main buFont,omitempty"`
	BuNone        *BuNone         `xml:"http://schemas.openxmlformats.org/drawingml/2006/main buNone,omitempty"`
	BuAutoNum     *BuAutoNum      `xml:"http://schemas.openxmlformats.org/drawingml/2006/main buAutoNum,omitempty"`
	BuChar        *BuChar         `xml:"http://schemas.openxmlformats.org/drawingml/2006/main buChar,omitempty"`
	BuBlip        *BuBlip         `xml:"http://schemas.openxmlformats.org/drawingml/2006/main buBlip,omitempty"`
	TabLst        *TabLst         `xml:"http://schemas.openxmlformats.org/drawingml/2006/main tabLst,omitempty"`
	DefRPr        *RPr            `xml:"http://schemas.openxmlformats.org/drawingml/2006/main defRPr,omitempty"`
	ExtLst        *ExtLst         `xml:"http://schemas.openxmlformats.org/drawingml/2006/main extLst,omitempty"`
	CapturedAttrs []xmlb.RootAttr `xml:"-"` // verbatim source attrs; see common/xml.CaptureAttrs
}

// UnmarshalXML captures the element's verbatim attribute list (source
// attribute order and any unmodeled attributes) before decoding through the
// struct tags; the reflection marshaler replays it.
func (pp *PPr) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	pp.CapturedAttrs = xmlb.CaptureAttrs(start.Attr)
	type alias PPr
	return d.DecodeElement((*alias)(pp), &start)
}

// R represents CT_RegularTextRun (a:r)
type R struct {
	RPr *RPr   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main rPr,omitempty"`
	T   string `xml:"http://schemas.openxmlformats.org/drawingml/2006/main t"`
}

// RPr represents CT_TextCharacterProperties (a:rPr)
type RPr struct {
	Kumimoji       *bool           `xml:"kumimoji,attr,omitempty"`
	Lang           string          `xml:"lang,attr,omitempty"`
	AltLang        string          `xml:"altLang,attr,omitempty"`
	Sz             int32           `xml:"sz,attr,omitempty"`
	B              *bool           `xml:"b,attr,omitempty"`
	I              *bool           `xml:"i,attr,omitempty"`
	U              string          `xml:"u,attr,omitempty"`
	Strike         string          `xml:"strike,attr,omitempty"`
	Kern           *int32          `xml:"kern,attr,omitempty"`
	Cap            string          `xml:"cap,attr,omitempty"`
	Spc            *int32          `xml:"spc,attr,omitempty"`
	NormalizeH     *bool           `xml:"normalizeH,attr,omitempty"`
	Baseline       *int32          `xml:"baseline,attr,omitempty"`
	NoProof        *bool           `xml:"noProof,attr,omitempty"`
	Dirty          *bool           `xml:"dirty,attr,omitempty"`
	Err            *bool           `xml:"err,attr,omitempty"`
	SmtClean       *bool           `xml:"smtClean,attr,omitempty"`
	SmtId          uint32          `xml:"smtId,attr,omitempty"`
	Bmk            string          `xml:"bmk,attr,omitempty"`
	Ln             *Ln             `xml:"http://schemas.openxmlformats.org/drawingml/2006/main ln,omitempty"`
	NoFill         *NoFillXML      `xml:"http://schemas.openxmlformats.org/drawingml/2006/main noFill,omitempty"`
	SolidFill      *SolidFill      `xml:"http://schemas.openxmlformats.org/drawingml/2006/main solidFill,omitempty"`
	GradFill       *GradFill       `xml:"http://schemas.openxmlformats.org/drawingml/2006/main gradFill,omitempty"`
	BlipFill       *BlipFillXML    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main blipFill,omitempty"`
	PattFill       *PattFill       `xml:"http://schemas.openxmlformats.org/drawingml/2006/main pattFill,omitempty"`
	GrpFill        *GrpFill        `xml:"http://schemas.openxmlformats.org/drawingml/2006/main grpFill,omitempty"`
	EffectLst      *EffectLst      `xml:"http://schemas.openxmlformats.org/drawingml/2006/main effectLst,omitempty"`
	EffectDag      *EffectDag      `xml:"http://schemas.openxmlformats.org/drawingml/2006/main effectDag,omitempty"`
	Highlight      *ColorChoice    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main highlight,omitempty"`
	ULnTx          *ULnTx          `xml:"http://schemas.openxmlformats.org/drawingml/2006/main uLnTx,omitempty"`
	ULn            *Ln             `xml:"http://schemas.openxmlformats.org/drawingml/2006/main uLn,omitempty"`
	UFillTx        *UFillTx        `xml:"http://schemas.openxmlformats.org/drawingml/2006/main uFillTx,omitempty"`
	UFill          *UFill          `xml:"http://schemas.openxmlformats.org/drawingml/2006/main uFill,omitempty"`
	Latin          *TextFont       `xml:"http://schemas.openxmlformats.org/drawingml/2006/main latin,omitempty"`
	Ea             *TextFont       `xml:"http://schemas.openxmlformats.org/drawingml/2006/main ea,omitempty"`
	Cs             *TextFont       `xml:"http://schemas.openxmlformats.org/drawingml/2006/main cs,omitempty"`
	Sym            *TextFont       `xml:"http://schemas.openxmlformats.org/drawingml/2006/main sym,omitempty"`
	HlinkClick     *HlinkXML       `xml:"http://schemas.openxmlformats.org/drawingml/2006/main hlinkClick,omitempty"`
	HlinkMouseOver *HlinkXML       `xml:"http://schemas.openxmlformats.org/drawingml/2006/main hlinkMouseOver,omitempty"`
	Rtl            *TextRtl        `xml:"http://schemas.openxmlformats.org/drawingml/2006/main rtl,omitempty"`
	ExtLst         *ExtLst         `xml:"http://schemas.openxmlformats.org/drawingml/2006/main extLst,omitempty"`
	CapturedAttrs  []xmlb.RootAttr `xml:"-"` // verbatim source attrs; see common/xml.CaptureAttrs
}

// UnmarshalXML captures the element's verbatim attribute list (source
// attribute order and any unmodeled attributes) before decoding through the
// struct tags; the reflection marshaler replays it.
func (rp *RPr) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	rp.CapturedAttrs = xmlb.CaptureAttrs(start.Attr)
	type alias RPr
	return d.DecodeElement((*alias)(rp), &start)
}

// TextRtl represents the a:rtl element (CT_Boolean): the value is carried in a
// val attribute (<a:rtl val="1"/>), not as element chardata. Modeling it as a
// bare *bool serialized as <a:rtl>true</a:rtl>, which is schema-invalid and
// fails to parse real input (the value lives in the attribute, so chardata is
// empty).
type TextRtl struct {
	Val bool `xml:"val,attr"`
}

// Br represents CT_TextLineBreak (a:br)
type Br struct {
	RPr *RPr `xml:"http://schemas.openxmlformats.org/drawingml/2006/main rPr,omitempty"`
}

// Fld represents CT_TextField (a:fld). id is required by the XSD (ST_Guid),
// so it carries no omitempty: an API-constructed field must not silently emit
// a schema-invalid <a:fld> without it.
type Fld struct {
	Id   string `xml:"id,attr"`
	Type string `xml:"type,attr,omitempty"`
	RPr  *RPr   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main rPr,omitempty"`
	PPr  *PPr   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main pPr,omitempty"`
	T    string `xml:"http://schemas.openxmlformats.org/drawingml/2006/main t,omitempty"`
}

// TextFont represents CT_TextFont (a:latin, a:ea, a:cs, a:sym)
type TextFont struct {
	Typeface      string          `xml:"typeface,attr"`
	Panose        string          `xml:"panose,attr,omitempty"`
	PitchFamily   *int32          `xml:"pitchFamily,attr,omitempty"`
	Charset       *int32          `xml:"charset,attr,omitempty"`
	CapturedAttrs []xmlb.RootAttr `xml:"-"` // verbatim source attrs; see common/xml.CaptureAttrs
}

// UnmarshalXML captures the element's verbatim attribute list (source
// attribute order and any unmodeled attributes) before decoding through the
// struct tags; the reflection marshaler replays it.
func (tf *TextFont) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	tf.CapturedAttrs = xmlb.CaptureAttrs(start.Attr)
	type alias TextFont
	return d.DecodeElement((*alias)(tf), &start)
}

// HlinkXML represents CT_Hyperlink (a:hlinkClick, a:hlinkMouseOver)
type HlinkXML struct {
	Id             *string         `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr,omitempty"`
	InvalidUrl     string          `xml:"invalidUrl,attr,omitempty"`
	Action         string          `xml:"action,attr,omitempty"`
	TgtFrame       string          `xml:"tgtFrame,attr,omitempty"`
	Tooltip        string          `xml:"tooltip,attr,omitempty"`
	History        *bool           `xml:"history,attr,omitempty"`
	HighlightClick *bool           `xml:"highlightClick,attr,omitempty"`
	EndSnd         *bool           `xml:"endSnd,attr,omitempty"`
	Snd            *EmbeddedWAVXML `xml:"http://schemas.openxmlformats.org/drawingml/2006/main snd,omitempty"`
	ExtLst         *ExtLst         `xml:"http://schemas.openxmlformats.org/drawingml/2006/main extLst,omitempty"`
}

// EmbeddedWAVXML represents CT_EmbeddedWAVAudioFile (a:snd)
type EmbeddedWAVXML struct {
	Embed string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships embed,attr,omitempty"`
	Name  string `xml:"name,attr,omitempty"`
}

// LnSpc represents CT_TextSpacing for line spacing (a:lnSpc)
type LnSpc struct {
	SpcPct *SpcPct `xml:"http://schemas.openxmlformats.org/drawingml/2006/main spcPct,omitempty"`
	SpcPts *SpcPts `xml:"http://schemas.openxmlformats.org/drawingml/2006/main spcPts,omitempty"`
}

// SpcBef represents CT_TextSpacing for space before (a:spcBef)
type SpcBef struct {
	SpcPct *SpcPct `xml:"http://schemas.openxmlformats.org/drawingml/2006/main spcPct,omitempty"`
	SpcPts *SpcPts `xml:"http://schemas.openxmlformats.org/drawingml/2006/main spcPts,omitempty"`
}

// SpcAft represents CT_TextSpacing for space after (a:spcAft)
type SpcAft struct {
	SpcPct *SpcPct `xml:"http://schemas.openxmlformats.org/drawingml/2006/main spcPct,omitempty"`
	SpcPts *SpcPts `xml:"http://schemas.openxmlformats.org/drawingml/2006/main spcPts,omitempty"`
}

// SpcPct represents CT_TextSpacingPercent (a:spcPct)
type SpcPct struct {
	Val Percentage `xml:"val,attr"`
}

// SpcPts represents CT_TextSpacingPoint (a:spcPts)
type SpcPts struct {
	Val int32 `xml:"val,attr"`
}

// BuClrTx represents CT_TextBulletColorFollowText (a:buClrTx)
type BuClrTx struct{}

// BuClr represents CT_Color (a:buClr). Its content is an EG_ColorChoice, so
// all six color kinds are modeled: a parsed hsl/sys/prst/scrgb bullet color
// must survive re-marshal instead of collapsing to an empty element.
type BuClr struct {
	ScRgbClr  *ScRgbClr           `xml:"http://schemas.openxmlformats.org/drawingml/2006/main scrgbClr,omitempty"`
	SrgbClr   *SrgbClr            `xml:"http://schemas.openxmlformats.org/drawingml/2006/main srgbClr,omitempty"`
	HslClr    *HslClr             `xml:"http://schemas.openxmlformats.org/drawingml/2006/main hslClr,omitempty"`
	SysClr    *SystemClr          `xml:"http://schemas.openxmlformats.org/drawingml/2006/main sysClr,omitempty"`
	SchemeClr *SchemeClrTransform `xml:"http://schemas.openxmlformats.org/drawingml/2006/main schemeClr,omitempty"`
	PrstClr   *PrstClr            `xml:"http://schemas.openxmlformats.org/drawingml/2006/main prstClr,omitempty"`
}

// BuSzTx represents CT_TextBulletSizeFollowText (a:buSzTx)
type BuSzTx struct{}

// BuSzPct represents CT_TextBulletSizePercent (a:buSzPct)
type BuSzPct struct {
	Val Percentage `xml:"val,attr"`
}

// BuSzPts represents CT_TextBulletSizePoint (a:buSzPts)
type BuSzPts struct {
	Val int32 `xml:"val,attr"`
}

// BuFontTx represents CT_TextBulletTypefaceFollowText (a:buFontTx)
type BuFontTx struct{}

// BuFont represents CT_TextFont (a:buFont)
type BuFont struct {
	Typeface      string          `xml:"typeface,attr"`
	Panose        string          `xml:"panose,attr,omitempty"`
	PitchFamily   *int32          `xml:"pitchFamily,attr,omitempty"`
	Charset       *int32          `xml:"charset,attr,omitempty"`
	CapturedAttrs []xmlb.RootAttr `xml:"-"` // verbatim source attrs; see common/xml.CaptureAttrs
}

// UnmarshalXML captures the element's verbatim attribute list (source
// attribute order and any unmodeled attributes) before decoding through the
// struct tags; the reflection marshaler replays it.
func (bf *BuFont) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	bf.CapturedAttrs = xmlb.CaptureAttrs(start.Attr)
	type alias BuFont
	return d.DecodeElement((*alias)(bf), &start)
}

// BuNone represents CT_TextNoBullet (a:buNone)
type BuNone struct{}

// BuAutoNum represents CT_TextAutonumberBullet (a:buAutoNum)
type BuAutoNum struct {
	Type    string `xml:"type,attr"`
	StartAt int32  `xml:"startAt,attr,omitempty"`
}

// BuChar represents CT_TextCharBullet (a:buChar)
type BuChar struct {
	Char string `xml:"char,attr"`
}

// BuBlip represents CT_TextBlipBullet (a:buBlip)
type BuBlip struct {
	Blip *BlipXML `xml:"http://schemas.openxmlformats.org/drawingml/2006/main blip,omitempty"`
}

// TabLst represents CT_TextTabStopList (a:tabLst)
type TabLst struct {
	Tab []*Tab `xml:"http://schemas.openxmlformats.org/drawingml/2006/main tab,omitempty"`
}

// Tab represents CT_TextTabStop (a:tab)
type Tab struct {
	// Pos is a pointer so an explicit pos="0" survives the round trip.
	Pos  *int32 `xml:"pos,attr,omitempty"`
	Algn string `xml:"algn,attr,omitempty"`
}

// ULnTx represents CT_TextUnderlineLineFollowText (a:uLnTx)
type ULnTx struct{}

// UFillTx represents CT_TextUnderlineFillFollowText (a:uFillTx)
type UFillTx struct{}

// UFill represents CT_TextUnderlineFillGroupWrapper (a:uFill)
type UFill struct {
	NoFill    *NoFillXML   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main noFill,omitempty"`
	SolidFill *SolidFill   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main solidFill,omitempty"`
	GradFill  *GradFill    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main gradFill,omitempty"`
	BlipFill  *BlipFillXML `xml:"http://schemas.openxmlformats.org/drawingml/2006/main blipFill,omitempty"`
	PattFill  *PattFill    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main pattFill,omitempty"`
	GrpFill   *GrpFill     `xml:"http://schemas.openxmlformats.org/drawingml/2006/main grpFill,omitempty"`
}

// --- Additional Text Body Types ---

// TcTxStyle already defined in xml_table.go

// DefPPr represents CT_TextParagraphProperties for default paragraph (a:defPPr)
type DefPPr = PPr

// --- Text Field Types ---

// FldType represents field type values
// Common values: slidenum, datetime, datetime1-datetime13

// --- Text Anchor Types ---

// TextAnchoringType values: t, ctr, b, just, dist
// TextVerticalType values: horz, vert, vert270, wordArtVert, eaVert, mongolianVert, wordArtVertRtl

// --- Preset Text Warp Types ---
// textNoShape, textPlain, textStop, textTriangle, textTriangleInverted, textChevron, textChevronInverted,
// textRingInside, textRingOutside, textArchUp, textArchDown, textCircle, textButton, textArchUpPour,
// textArchDownPour, textCirclePour, textButtonPour, textCurveUp, textCurveDown, textCanUp, textCanDown,
// textWave1, textWave2, textDoubleWave1, textWave4, textInflate, textDeflate, textInflateBottom,
// textDeflateBottom, textInflateTop, textDeflateTop, textDeflateInflate, textDeflateInflateDeflate,
// textFadeRight, textFadeLeft, textFadeUp, textFadeDown, textSlantUp, textSlantDown, textCascadeUp,
// textCascadeDown
