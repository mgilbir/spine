package oxml

import (
	"encoding/xml"
	"strings"

	coxml "github.com/mgilbir/spine/common/oxml"
	xmlb "github.com/mgilbir/spine/common/xml"
)

// CT_PPr represents paragraph properties (w:pPr).
type CT_PPr struct {
	PStyle            *CT_String           `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main pStyle,omitempty"`
	KeepNext          *CT_OnOff            `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main keepNext,omitempty"`
	KeepLines         *CT_OnOff            `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main keepLines,omitempty"`
	PageBreakBefore   *CT_OnOff            `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main pageBreakBefore,omitempty"`
	FramePr           *CT_FramePr          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main framePr,omitempty"`
	WidowControl      *CT_OnOff            `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main widowControl,omitempty"`
	NumPr             *CT_NumPr            `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main numPr,omitempty"`
	SuppressLineNumbers *CT_OnOff          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main suppressLineNumbers,omitempty"`
	PBdr              *CT_PBdr             `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main pBdr,omitempty"`
	Shd               *CT_Shd              `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main shd,omitempty"`
	Tabs              *CT_Tabs             `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main tabs,omitempty"`
	SuppressAutoHyphens *CT_OnOff          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main suppressAutoHyphens,omitempty"`
	Kinsoku           *CT_OnOff            `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main kinsoku,omitempty"`
	WordWrap          *CT_OnOff            `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main wordWrap,omitempty"`
	OverflowPunct     *CT_OnOff            `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main overflowPunct,omitempty"`
	TopLinePunct      *CT_OnOff            `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main topLinePunct,omitempty"`
	AutoSpaceDE       *CT_OnOff            `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main autoSpaceDE,omitempty"`
	AutoSpaceDN       *CT_OnOff            `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main autoSpaceDN,omitempty"`
	Bidi              *CT_OnOff            `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main bidi,omitempty"`
	AdjustRightInd    *CT_OnOff            `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main adjustRightInd,omitempty"`
	SnapToGrid        *CT_OnOff            `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main snapToGrid,omitempty"`
	Spacing           *CT_Spacing          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main spacing,omitempty"`
	Ind               *CT_Ind              `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main ind,omitempty"`
	ContextualSpacing *CT_OnOff            `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main contextualSpacing,omitempty"`
	MirrorIndents     *CT_OnOff            `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main mirrorIndents,omitempty"`
	SuppressOverlap   *CT_OnOff            `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main suppressOverlap,omitempty"`
	Jc                *CT_Jc               `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main jc,omitempty"`
	TextDirection     *CT_String           `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main textDirection,omitempty"`
	TextAlignment     *CT_String           `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main textAlignment,omitempty"`
	OutlineLvl        *CT_DecimalNumber    `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main outlineLvl,omitempty"`
	DivId             *CT_DecimalNumber    `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main divId,omitempty"`
	RPr               *CT_RPr              `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main rPr,omitempty"`
	SectPr            *CT_SectPr           `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main sectPr,omitempty"`
	PPrChange         *CT_PPrChange        `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main pPrChange,omitempty"`
}

// pChildKind identifies paragraph content child element types.
type pChildKind int

const (
	pChildR pChildKind = iota
	pChildHyperlink
	pChildBookmarkStart
	pChildBookmarkEnd
	pChildProofErr
	pChildPermStart
	pChildPermEnd
	pChildIns
	pChildDel
	pChildFldSimple
	pChildSdtRun
	pChildCommentRangeStart
	pChildCommentRangeEnd
	pChildOMath
	pChildOMathPara
	pChildAlternateContent
)

// pChildRef references a paragraph content child.
type pChildRef struct {
	kind  pChildKind
	index int
}

// Text returns the paragraph's visible text, walking child content in document
// order (runs, hyperlinks, simple fields, tracked insertions, and structured
// document tags), so text nested in a hyperlink or field is not lost. Deleted
// (w:del) content is excluded as it is not visible text.
func (p *CT_P) Text() string {
	var sb strings.Builder
	if len(p.childOrder) > 0 {
		for _, ref := range p.childOrder {
			switch ref.kind {
			case pChildR:
				if ref.index < len(p.R) {
					writeRunText(&sb, p.R[ref.index])
				}
			case pChildHyperlink:
				if ref.index < len(p.Hyperlink) {
					writeRunsText(&sb, p.Hyperlink[ref.index].R)
				}
			case pChildIns:
				if ref.index < len(p.Ins) {
					writeRunsText(&sb, p.Ins[ref.index].R)
				}
			case pChildFldSimple:
				if ref.index < len(p.FldSimple) {
					writeRunsText(&sb, p.FldSimple[ref.index].R)
				}
			case pChildSdtRun:
				if ref.index < len(p.SdtRun) && p.SdtRun[ref.index].SdtContent != nil {
					writeRunsText(&sb, p.SdtRun[ref.index].SdtContent.R)
				}
			}
		}
		return sb.String()
	}
	// No recorded child order (e.g. a programmatically built paragraph): read
	// the top-level runs.
	writeRunsText(&sb, p.R)
	return sb.String()
}

func writeRunText(sb *strings.Builder, r *CT_R) {
	if r == nil {
		return
	}
	for _, t := range r.T {
		sb.WriteString(t.Text)
	}
}

func writeRunsText(sb *strings.Builder, runs []*CT_R) {
	for _, r := range runs {
		writeRunText(sb, r)
	}
}

// CT_P represents a paragraph (w:p).
type CT_P struct {
	RsidR           string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main rsidR,attr,omitempty"`
	RsidRPr         string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main rsidRPr,attr,omitempty"`
	RsidRDefault    string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main rsidRDefault,attr,omitempty"`
	RsidP           string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main rsidP,attr,omitempty"`
	RsidDel         string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main rsidDel,attr,omitempty"`
	ParaId          string `xml:"-"` // w14:paraId - custom unmarshal
	TextId          string `xml:"-"` // w14:textId - custom unmarshal

	PPr             *CT_PPr              `xml:"-"`
	R               []*CT_R              `xml:"-"`
	Hyperlink       []*CT_Hyperlink      `xml:"-"`
	BookmarkStart   []*CT_BookmarkStart  `xml:"-"`
	BookmarkEnd     []*CT_BookmarkEnd    `xml:"-"`
	ProofErr        []*CT_ProofErr       `xml:"-"`
	PermStart       []*CT_PermStart      `xml:"-"`
	PermEnd         []*CT_PermEnd        `xml:"-"`
	Ins             []*CT_RunTrackChange `xml:"-"`
	Del             []*CT_RunTrackChange `xml:"-"`
	FldSimple       []*CT_SimpleField    `xml:"-"`
	SdtRun          []*CT_SdtRun         `xml:"-"`
	CommentRangeStart []*CT_CommentRangeStart `xml:"-"`
	CommentRangeEnd   []*CT_CommentRangeEnd   `xml:"-"`
	OMath           [][]byte             `xml:"-"` // raw m:oMath elements
	OMathPara       [][]byte             `xml:"-"` // raw m:oMathPara elements
	AlternateContent []*coxml.AlternateContent `xml:"-"`
	childOrder      []pChildRef
}

// UnmarshalXML implements custom unmarshaling for CT_P to preserve child order.
func (p *CT_P) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		switch {
		case attr.Name.Local == "rsidR" && (attr.Name.Space == NsWml || attr.Name.Space == ""):
			p.RsidR = attr.Value
		case attr.Name.Local == "rsidRPr" && (attr.Name.Space == NsWml || attr.Name.Space == ""):
			p.RsidRPr = attr.Value
		case attr.Name.Local == "rsidRDefault" && (attr.Name.Space == NsWml || attr.Name.Space == ""):
			p.RsidRDefault = attr.Value
		case attr.Name.Local == "rsidP" && (attr.Name.Space == NsWml || attr.Name.Space == ""):
			p.RsidP = attr.Value
		case attr.Name.Local == "rsidDel" && (attr.Name.Space == NsWml || attr.Name.Space == ""):
			p.RsidDel = attr.Value
		case attr.Name.Local == "paraId":
			p.ParaId = attr.Value
		case attr.Name.Local == "textId":
			p.TextId = attr.Value
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
			case "pPr":
				p.PPr = &CT_PPr{}
				if err := d.DecodeElement(p.PPr, &t); err != nil {
					return err
				}
			case "r":
				v := &CT_R{}
				if err := d.DecodeElement(v, &t); err != nil {
					return err
				}
				p.childOrder = append(p.childOrder, pChildRef{pChildR, len(p.R)})
				p.R = append(p.R, v)
			case "hyperlink":
				v := &CT_Hyperlink{}
				if err := d.DecodeElement(v, &t); err != nil {
					return err
				}
				p.childOrder = append(p.childOrder, pChildRef{pChildHyperlink, len(p.Hyperlink)})
				p.Hyperlink = append(p.Hyperlink, v)
			case "bookmarkStart":
				v := &CT_BookmarkStart{}
				if err := d.DecodeElement(v, &t); err != nil {
					return err
				}
				p.childOrder = append(p.childOrder, pChildRef{pChildBookmarkStart, len(p.BookmarkStart)})
				p.BookmarkStart = append(p.BookmarkStart, v)
			case "bookmarkEnd":
				v := &CT_BookmarkEnd{}
				if err := d.DecodeElement(v, &t); err != nil {
					return err
				}
				p.childOrder = append(p.childOrder, pChildRef{pChildBookmarkEnd, len(p.BookmarkEnd)})
				p.BookmarkEnd = append(p.BookmarkEnd, v)
			case "proofErr":
				v := &CT_ProofErr{}
				if err := d.DecodeElement(v, &t); err != nil {
					return err
				}
				p.childOrder = append(p.childOrder, pChildRef{pChildProofErr, len(p.ProofErr)})
				p.ProofErr = append(p.ProofErr, v)
			case "permStart":
				v := &CT_PermStart{}
				if err := d.DecodeElement(v, &t); err != nil {
					return err
				}
				p.childOrder = append(p.childOrder, pChildRef{pChildPermStart, len(p.PermStart)})
				p.PermStart = append(p.PermStart, v)
			case "permEnd":
				v := &CT_PermEnd{}
				if err := d.DecodeElement(v, &t); err != nil {
					return err
				}
				p.childOrder = append(p.childOrder, pChildRef{pChildPermEnd, len(p.PermEnd)})
				p.PermEnd = append(p.PermEnd, v)
			case "ins":
				v := &CT_RunTrackChange{}
				if err := d.DecodeElement(v, &t); err != nil {
					return err
				}
				p.childOrder = append(p.childOrder, pChildRef{pChildIns, len(p.Ins)})
				p.Ins = append(p.Ins, v)
			case "del":
				v := &CT_RunTrackChange{}
				if err := d.DecodeElement(v, &t); err != nil {
					return err
				}
				p.childOrder = append(p.childOrder, pChildRef{pChildDel, len(p.Del)})
				p.Del = append(p.Del, v)
			case "fldSimple":
				v := &CT_SimpleField{}
				if err := d.DecodeElement(v, &t); err != nil {
					return err
				}
				p.childOrder = append(p.childOrder, pChildRef{pChildFldSimple, len(p.FldSimple)})
				p.FldSimple = append(p.FldSimple, v)
			case "sdt":
				v := &CT_SdtRun{}
				if err := d.DecodeElement(v, &t); err != nil {
					return err
				}
				p.childOrder = append(p.childOrder, pChildRef{pChildSdtRun, len(p.SdtRun)})
				p.SdtRun = append(p.SdtRun, v)
			case "commentRangeStart":
				v := &CT_CommentRangeStart{}
				if err := d.DecodeElement(v, &t); err != nil {
					return err
				}
				p.childOrder = append(p.childOrder, pChildRef{pChildCommentRangeStart, len(p.CommentRangeStart)})
				p.CommentRangeStart = append(p.CommentRangeStart, v)
			case "commentRangeEnd":
				v := &CT_CommentRangeEnd{}
				if err := d.DecodeElement(v, &t); err != nil {
					return err
				}
				p.childOrder = append(p.childOrder, pChildRef{pChildCommentRangeEnd, len(p.CommentRangeEnd)})
				p.CommentRangeEnd = append(p.CommentRangeEnd, v)
			case "oMath":
				var raw struct {
					Content []byte `xml:",innerxml"`
				}
				if err := d.DecodeElement(&raw, &t); err != nil {
					return err
				}
				p.childOrder = append(p.childOrder, pChildRef{pChildOMath, len(p.OMath)})
				p.OMath = append(p.OMath, raw.Content)
			case "oMathPara":
				var raw struct {
					Content []byte `xml:",innerxml"`
				}
				if err := d.DecodeElement(&raw, &t); err != nil {
					return err
				}
				p.childOrder = append(p.childOrder, pChildRef{pChildOMathPara, len(p.OMathPara)})
				p.OMathPara = append(p.OMathPara, raw.Content)
			case "AlternateContent":
				v := &coxml.AlternateContent{}
				if err := d.DecodeElement(v, &t); err != nil {
					return err
				}
				p.childOrder = append(p.childOrder, pChildRef{pChildAlternateContent, len(p.AlternateContent)})
				p.AlternateContent = append(p.AlternateContent, v)
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

// MarshalToBuilder implements xmlb.BuilderMarshaler for CT_P.
func (p *CT_P) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	var attrs []xmlb.Attr
	if p.ParaId != "" {
		attrs = append(attrs, xmlb.Attr{Namespace: xmlb.NSWord2010, Name: "paraId", Value: p.ParaId})
	}
	if p.TextId != "" {
		attrs = append(attrs, xmlb.Attr{Namespace: xmlb.NSWord2010, Name: "textId", Value: p.TextId})
	}
	if p.RsidR != "" {
		attrs = append(attrs, xmlb.Attr{Namespace: xmlb.NSWordprocessingML, Name: "rsidR", Value: p.RsidR})
	}
	if p.RsidRPr != "" {
		attrs = append(attrs, xmlb.Attr{Namespace: xmlb.NSWordprocessingML, Name: "rsidRPr", Value: p.RsidRPr})
	}
	if p.RsidRDefault != "" {
		attrs = append(attrs, xmlb.Attr{Namespace: xmlb.NSWordprocessingML, Name: "rsidRDefault", Value: p.RsidRDefault})
	}
	if p.RsidP != "" {
		attrs = append(attrs, xmlb.Attr{Namespace: xmlb.NSWordprocessingML, Name: "rsidP", Value: p.RsidP})
	}
	if p.RsidDel != "" {
		attrs = append(attrs, xmlb.Attr{Namespace: xmlb.NSWordprocessingML, Name: "rsidDel", Value: p.RsidDel})
	}
	b.StartElement(ns, localName, attrs...)

	if p.PPr != nil {
		b.MarshalElement(ns, "pPr", p.PPr)
	}

	if len(p.childOrder) > 0 {
		for _, ref := range p.childOrder {
			switch ref.kind {
			case pChildR:
				if ref.index < len(p.R) {
					p.R[ref.index].MarshalToBuilder(b, ns, "r")
				}
			case pChildHyperlink:
				if ref.index < len(p.Hyperlink) {
					p.Hyperlink[ref.index].MarshalToBuilder(b, ns, "hyperlink")
				}
			case pChildBookmarkStart:
				if ref.index < len(p.BookmarkStart) {
					b.MarshalElement(ns, "bookmarkStart", p.BookmarkStart[ref.index])
				}
			case pChildBookmarkEnd:
				if ref.index < len(p.BookmarkEnd) {
					b.MarshalElement(ns, "bookmarkEnd", p.BookmarkEnd[ref.index])
				}
			case pChildProofErr:
				if ref.index < len(p.ProofErr) {
					b.MarshalElement(ns, "proofErr", p.ProofErr[ref.index])
				}
			case pChildPermStart:
				if ref.index < len(p.PermStart) {
					b.MarshalElement(ns, "permStart", p.PermStart[ref.index])
				}
			case pChildPermEnd:
				if ref.index < len(p.PermEnd) {
					b.MarshalElement(ns, "permEnd", p.PermEnd[ref.index])
				}
			case pChildIns:
				if ref.index < len(p.Ins) {
					p.Ins[ref.index].MarshalToBuilder(b, ns, "ins")
				}
			case pChildDel:
				if ref.index < len(p.Del) {
					p.Del[ref.index].MarshalToBuilder(b, ns, "del")
				}
			case pChildFldSimple:
				if ref.index < len(p.FldSimple) {
					p.FldSimple[ref.index].MarshalToBuilder(b, ns, "fldSimple")
				}
			case pChildSdtRun:
				if ref.index < len(p.SdtRun) {
					p.SdtRun[ref.index].MarshalToBuilder(b, ns, "sdt")
				}
			case pChildCommentRangeStart:
				if ref.index < len(p.CommentRangeStart) {
					b.MarshalElement(ns, "commentRangeStart", p.CommentRangeStart[ref.index])
				}
			case pChildCommentRangeEnd:
				if ref.index < len(p.CommentRangeEnd) {
					b.MarshalElement(ns, "commentRangeEnd", p.CommentRangeEnd[ref.index])
				}
			case pChildOMath:
				if ref.index < len(p.OMath) {
					b.StartElement("http://schemas.openxmlformats.org/officeDocument/2006/math", "oMath")
					b.WriteRaw(p.OMath[ref.index])
					b.EndElement("http://schemas.openxmlformats.org/officeDocument/2006/math", "oMath")
				}
			case pChildOMathPara:
				if ref.index < len(p.OMathPara) {
					b.StartElement("http://schemas.openxmlformats.org/officeDocument/2006/math", "oMathPara")
					b.WriteRaw(p.OMathPara[ref.index])
					b.EndElement("http://schemas.openxmlformats.org/officeDocument/2006/math", "oMathPara")
				}
			case pChildAlternateContent:
				if ref.index < len(p.AlternateContent) {
					p.AlternateContent[ref.index].MarshalToBuilder(b, ns, "AlternateContent")
				}
			}
		}
	} else {
		for _, v := range p.R {
			v.MarshalToBuilder(b, ns, "r")
		}
	}

	b.EndElement(ns, localName)
}

// unmarshalPContent is a shared helper for unmarshaling paragraph-level content children
// (used by CT_P, CT_Hyperlink, CT_RunTrackChange, CT_SimpleField).
func unmarshalPContent(d *xml.Decoder,
	r *[]*CT_R, hyperlink *[]*CT_Hyperlink,
	bookmarkStart *[]*CT_BookmarkStart, bookmarkEnd *[]*CT_BookmarkEnd,
	proofErr *[]*CT_ProofErr, permStart *[]*CT_PermStart, permEnd *[]*CT_PermEnd,
	ins *[]*CT_RunTrackChange, del *[]*CT_RunTrackChange,
	fldSimple *[]*CT_SimpleField, sdtRun *[]*CT_SdtRun,
	childOrder *[]pChildRef,
) error {
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "r":
				v := &CT_R{}
				if err := d.DecodeElement(v, &t); err != nil {
					return err
				}
				if r != nil {
					*childOrder = append(*childOrder, pChildRef{pChildR, len(*r)})
					*r = append(*r, v)
				}
			case "hyperlink":
				if hyperlink != nil {
					v := &CT_Hyperlink{}
					if err := d.DecodeElement(v, &t); err != nil {
						return err
					}
					*childOrder = append(*childOrder, pChildRef{pChildHyperlink, len(*hyperlink)})
					*hyperlink = append(*hyperlink, v)
				} else {
					if err := d.Skip(); err != nil {
						return err
					}
				}
			case "bookmarkStart":
				if bookmarkStart != nil {
					v := &CT_BookmarkStart{}
					if err := d.DecodeElement(v, &t); err != nil {
						return err
					}
					*childOrder = append(*childOrder, pChildRef{pChildBookmarkStart, len(*bookmarkStart)})
					*bookmarkStart = append(*bookmarkStart, v)
				} else {
					if err := d.Skip(); err != nil {
						return err
					}
				}
			case "bookmarkEnd":
				if bookmarkEnd != nil {
					v := &CT_BookmarkEnd{}
					if err := d.DecodeElement(v, &t); err != nil {
						return err
					}
					*childOrder = append(*childOrder, pChildRef{pChildBookmarkEnd, len(*bookmarkEnd)})
					*bookmarkEnd = append(*bookmarkEnd, v)
				} else {
					if err := d.Skip(); err != nil {
						return err
					}
				}
			case "proofErr":
				if proofErr != nil {
					v := &CT_ProofErr{}
					if err := d.DecodeElement(v, &t); err != nil {
						return err
					}
					*childOrder = append(*childOrder, pChildRef{pChildProofErr, len(*proofErr)})
					*proofErr = append(*proofErr, v)
				} else {
					if err := d.Skip(); err != nil {
						return err
					}
				}
			case "permStart":
				if permStart != nil {
					v := &CT_PermStart{}
					if err := d.DecodeElement(v, &t); err != nil {
						return err
					}
					*childOrder = append(*childOrder, pChildRef{pChildPermStart, len(*permStart)})
					*permStart = append(*permStart, v)
				} else {
					if err := d.Skip(); err != nil {
						return err
					}
				}
			case "permEnd":
				if permEnd != nil {
					v := &CT_PermEnd{}
					if err := d.DecodeElement(v, &t); err != nil {
						return err
					}
					*childOrder = append(*childOrder, pChildRef{pChildPermEnd, len(*permEnd)})
					*permEnd = append(*permEnd, v)
				} else {
					if err := d.Skip(); err != nil {
						return err
					}
				}
			case "ins":
				if ins != nil {
					v := &CT_RunTrackChange{}
					if err := d.DecodeElement(v, &t); err != nil {
						return err
					}
					*childOrder = append(*childOrder, pChildRef{pChildIns, len(*ins)})
					*ins = append(*ins, v)
				} else {
					if err := d.Skip(); err != nil {
						return err
					}
				}
			case "del":
				if del != nil {
					v := &CT_RunTrackChange{}
					if err := d.DecodeElement(v, &t); err != nil {
						return err
					}
					*childOrder = append(*childOrder, pChildRef{pChildDel, len(*del)})
					*del = append(*del, v)
				} else {
					if err := d.Skip(); err != nil {
						return err
					}
				}
			case "fldSimple":
				if fldSimple != nil {
					v := &CT_SimpleField{}
					if err := d.DecodeElement(v, &t); err != nil {
						return err
					}
					*childOrder = append(*childOrder, pChildRef{pChildFldSimple, len(*fldSimple)})
					*fldSimple = append(*fldSimple, v)
				} else {
					if err := d.Skip(); err != nil {
						return err
					}
				}
			case "sdt":
				if sdtRun != nil {
					v := &CT_SdtRun{}
					if err := d.DecodeElement(v, &t); err != nil {
						return err
					}
					*childOrder = append(*childOrder, pChildRef{pChildSdtRun, len(*sdtRun)})
					*sdtRun = append(*sdtRun, v)
				} else {
					if err := d.Skip(); err != nil {
						return err
					}
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

// marshalPContent is a shared helper for marshaling paragraph-level content children.
func marshalPContent(b *xmlb.Builder, ns string,
	r []*CT_R, hyperlink []*CT_Hyperlink,
	bookmarkStart []*CT_BookmarkStart, bookmarkEnd []*CT_BookmarkEnd,
	proofErr []*CT_ProofErr, permStart []*CT_PermStart, permEnd []*CT_PermEnd,
	ins []*CT_RunTrackChange, del []*CT_RunTrackChange,
	fldSimple []*CT_SimpleField, sdtRun []*CT_SdtRun,
	childOrder []pChildRef,
) {
	if len(childOrder) > 0 {
		for _, ref := range childOrder {
			switch ref.kind {
			case pChildR:
				if ref.index < len(r) {
					r[ref.index].MarshalToBuilder(b, ns, "r")
				}
			case pChildHyperlink:
				if ref.index < len(hyperlink) {
					hyperlink[ref.index].MarshalToBuilder(b, ns, "hyperlink")
				}
			case pChildBookmarkStart:
				if ref.index < len(bookmarkStart) {
					b.MarshalElement(ns, "bookmarkStart", bookmarkStart[ref.index])
				}
			case pChildBookmarkEnd:
				if ref.index < len(bookmarkEnd) {
					b.MarshalElement(ns, "bookmarkEnd", bookmarkEnd[ref.index])
				}
			case pChildProofErr:
				if ref.index < len(proofErr) {
					b.MarshalElement(ns, "proofErr", proofErr[ref.index])
				}
			case pChildPermStart:
				if ref.index < len(permStart) {
					b.MarshalElement(ns, "permStart", permStart[ref.index])
				}
			case pChildPermEnd:
				if ref.index < len(permEnd) {
					b.MarshalElement(ns, "permEnd", permEnd[ref.index])
				}
			case pChildIns:
				if ref.index < len(ins) {
					ins[ref.index].MarshalToBuilder(b, ns, "ins")
				}
			case pChildDel:
				if ref.index < len(del) {
					del[ref.index].MarshalToBuilder(b, ns, "del")
				}
			case pChildFldSimple:
				if ref.index < len(fldSimple) {
					fldSimple[ref.index].MarshalToBuilder(b, ns, "fldSimple")
				}
			case pChildSdtRun:
				if ref.index < len(sdtRun) {
					sdtRun[ref.index].MarshalToBuilder(b, ns, "sdt")
				}
			}
		}
	} else {
		for _, v := range r {
			v.MarshalToBuilder(b, ns, "r")
		}
	}
}

// AppendR appends a run to this paragraph, maintaining child order so it is
// marshaled even on a paragraph parsed from a file (whose order is already
// populated).
func (p *CT_P) AppendR(r *CT_R) {
	p.childOrder = append(p.childOrder, pChildRef{pChildR, len(p.R)})
	p.R = append(p.R, r)
}
