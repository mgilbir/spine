package oxml

import (
	"encoding/xml"
	"strings"

	coxml "github.com/mgilbir/spine/common/oxml"
	xmlb "github.com/mgilbir/spine/common/xml"
)

// CT_PPr represents paragraph properties (w:pPr).
type CT_PPr struct {
	PStyle              *CT_String        `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main pStyle,omitempty"`
	KeepNext            *CT_OnOff         `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main keepNext,omitempty"`
	KeepLines           *CT_OnOff         `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main keepLines,omitempty"`
	PageBreakBefore     *CT_OnOff         `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main pageBreakBefore,omitempty"`
	FramePr             *CT_FramePr       `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main framePr,omitempty"`
	WidowControl        *CT_OnOff         `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main widowControl,omitempty"`
	NumPr               *CT_NumPr         `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main numPr,omitempty"`
	SuppressLineNumbers *CT_OnOff         `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main suppressLineNumbers,omitempty"`
	PBdr                *CT_PBdr          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main pBdr,omitempty"`
	Shd                 *CT_Shd           `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main shd,omitempty"`
	Tabs                *CT_Tabs          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main tabs,omitempty"`
	SuppressAutoHyphens *CT_OnOff         `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main suppressAutoHyphens,omitempty"`
	Kinsoku             *CT_OnOff         `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main kinsoku,omitempty"`
	WordWrap            *CT_OnOff         `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main wordWrap,omitempty"`
	OverflowPunct       *CT_OnOff         `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main overflowPunct,omitempty"`
	TopLinePunct        *CT_OnOff         `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main topLinePunct,omitempty"`
	AutoSpaceDE         *CT_OnOff         `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main autoSpaceDE,omitempty"`
	AutoSpaceDN         *CT_OnOff         `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main autoSpaceDN,omitempty"`
	Bidi                *CT_OnOff         `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main bidi,omitempty"`
	AdjustRightInd      *CT_OnOff         `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main adjustRightInd,omitempty"`
	SnapToGrid          *CT_OnOff         `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main snapToGrid,omitempty"`
	Spacing             *CT_Spacing       `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main spacing,omitempty"`
	Ind                 *CT_Ind           `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main ind,omitempty"`
	ContextualSpacing   *CT_OnOff         `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main contextualSpacing,omitempty"`
	MirrorIndents       *CT_OnOff         `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main mirrorIndents,omitempty"`
	SuppressOverlap     *CT_OnOff         `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main suppressOverlap,omitempty"`
	Jc                  *CT_Jc            `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main jc,omitempty"`
	TextDirection       *CT_String        `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main textDirection,omitempty"`
	TextAlignment       *CT_String        `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main textAlignment,omitempty"`
	OutlineLvl          *CT_DecimalNumber `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main outlineLvl,omitempty"`
	DivId               *CT_DecimalNumber `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main divId,omitempty"`
	// CnfStyle sits at its XSD position — after every other pPrBase child and
	// immediately before w:rPr — which is also where Word emits it (corpus:
	// the element following w:cnfStyle is always w:rPr content or nothing).
	CnfStyle  *CT_Cnf       `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main cnfStyle,omitempty"`
	RPr       *CT_RPr       `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main rPr,omitempty"`
	SectPr    *CT_SectPr    `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main sectPr,omitempty"`
	PPrChange *CT_PPrChange `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main pPrChange,omitempty"`
	// CapturedChildren records the source child sequence (see CT_RPr):
	// producer child order, unmodeled children, and duplicated toggles all
	// replay verbatim.
	CapturedChildren *xmlb.ChildCapture `xml:"-"`
	// CapturedEmptyTag records how an empty w:pPr was written in the source.
	CapturedEmptyTag xmlb.EmptyTagStyle `xml:"-"`
}

// UnmarshalXML captures the element's empty-tag style and child sequence
// while decoding the children into the struct fields.
func (pp *CT_PPr) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	pp.CapturedEmptyTag = xmlb.CaptureEmptyTagStyle(d)
	return xmlb.UnmarshalOrderedChildren(d, pp)
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
	pChildRaw
)

// isRawPChild reports whether an inline (paragraph-content) child element the
// model does not type must be preserved verbatim instead of skipped: inline
// w:customXml and w:smartTag (whose runs would otherwise lose their text),
// tracked-move containers w:moveTo/w:moveFrom (whose loss deletes the moved
// text entirely), and the move range markers. The captured content is opaque
// to the model: Text() does not descend into it and SetText() removes it along
// with the other content children.
func isRawPChild(local string) bool {
	switch local {
	case "customXml", "smartTag", "moveTo", "moveFrom",
		"moveFromRangeStart", "moveFromRangeEnd", "moveToRangeStart", "moveToRangeEnd",
		"customXmlInsRangeStart", "customXmlInsRangeEnd",
		"customXmlDelRangeStart", "customXmlDelRangeEnd",
		"customXmlMoveFromRangeStart", "customXmlMoveFromRangeEnd",
		"customXmlMoveToRangeStart", "customXmlMoveToRangeEnd",
		"br",
		"contentPart",
		"commentRangeStart", "commentRangeEnd":
		// w:br is only valid inside w:r, but LibreOffice-era exports place it
		// directly in w:p; dropping it merged the surrounding lines.
		// w:contentPart (EG_PContent, a CT_Rel referencing an ink/customXML
		// part by r:id) is untyped by the model; preserving it verbatim keeps
		// the in-body ink reference so a paragraph carrying only a contentPart
		// no longer round-trips as an empty <w:p/>.
		// Comment ranges appear inside w:ins/w:del and run-level SDT content,
		// which type them nowhere (CT_P handles its own before this).
		return true
	}
	return false
}

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
	// CapturedAttrs preserves the verbatim source attribute list (producer
	// rsid/paraId order varies; unmodeled attributes survive); replayed on
	// marshal.
	CapturedAttrs []xmlb.RootAttr `xml:"-"`
	RsidR         string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main rsidR,attr,omitempty"`
	RsidRPr       string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main rsidRPr,attr,omitempty"`
	RsidRDefault  string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main rsidRDefault,attr,omitempty"`
	RsidP         string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main rsidP,attr,omitempty"`
	RsidDel       string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main rsidDel,attr,omitempty"`
	ParaId        string          `xml:"-"` // w14:paraId - custom unmarshal
	TextId        string          `xml:"-"` // w14:textId - custom unmarshal

	PPr               *CT_PPr                   `xml:"-"`
	R                 []*CT_R                   `xml:"-"`
	Hyperlink         []*CT_Hyperlink           `xml:"-"`
	BookmarkStart     []*CT_BookmarkStart       `xml:"-"`
	BookmarkEnd       []*CT_BookmarkEnd         `xml:"-"`
	ProofErr          []*CT_ProofErr            `xml:"-"`
	PermStart         []*CT_PermStart           `xml:"-"`
	PermEnd           []*CT_PermEnd             `xml:"-"`
	Ins               []*CT_RunTrackChange      `xml:"-"`
	Del               []*CT_RunTrackChange      `xml:"-"`
	FldSimple         []*CT_SimpleField         `xml:"-"`
	SdtRun            []*CT_SdtRun              `xml:"-"`
	CommentRangeStart []*CT_CommentRangeStart   `xml:"-"`
	CommentRangeEnd   []*CT_CommentRangeEnd     `xml:"-"`
	OMath             [][]byte                  `xml:"-"` // raw m:oMath elements
	OMathPara         [][]byte                  `xml:"-"` // raw m:oMathPara elements
	AlternateContent  []*coxml.AlternateContent `xml:"-"`
	Raw               []*CT_RawNamedElement     `xml:"-"` // see isRawPChild
	// CapturedEmptyTag records how an empty w:p was written in the source
	// (<w:p/> vs <w:p></w:p>; producers mix both forms in one part).
	CapturedEmptyTag xmlb.EmptyTagStyle `xml:"-"`
	childOrder       []pChildRef
}

// isEmpty reports whether the paragraph has no children to write.
func (p *CT_P) isEmpty() bool {
	return p.PPr == nil && len(p.childOrder) == 0 && len(p.R) == 0 &&
		len(p.Hyperlink) == 0 && len(p.BookmarkStart) == 0 && len(p.BookmarkEnd) == 0 &&
		len(p.ProofErr) == 0 && len(p.PermStart) == 0 && len(p.PermEnd) == 0 &&
		len(p.Ins) == 0 && len(p.Del) == 0 && len(p.FldSimple) == 0 &&
		len(p.SdtRun) == 0 && len(p.CommentRangeStart) == 0 && len(p.CommentRangeEnd) == 0 &&
		len(p.OMath) == 0 && len(p.OMathPara) == 0 && len(p.AlternateContent) == 0 && len(p.Raw) == 0
}

// UnmarshalXML implements custom unmarshaling for CT_P to preserve child order.
func (p *CT_P) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	p.CapturedEmptyTag = xmlb.CaptureEmptyTagStyle(d)
	p.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
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
				if p.PPr != nil {
					// A duplicated w:pPr (invalid but seen in the wild) is
					// preserved verbatim at its position; the first stays
					// the typed model.
					v := &CT_RawNamedElement{}
					if err := d.DecodeElement(v, &t); err != nil {
						return err
					}
					p.childOrder = append(p.childOrder, pChildRef{pChildRaw, len(p.Raw)})
					p.Raw = append(p.Raw, v)
					continue
				}
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
				if isRawPChild(t.Name.Local) {
					v := &CT_RawNamedElement{}
					if err := d.DecodeElement(v, &t); err != nil {
						return err
					}
					p.childOrder = append(p.childOrder, pChildRef{pChildRaw, len(p.Raw)})
					p.Raw = append(p.Raw, v)
					continue
				}
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
	// Word emits w:rsidDel between w:rsidRPr and w:rsidRDefault (corpus:
	// every Word-authored w:p carrying rsidDel uses this position).
	if p.RsidDel != "" {
		attrs = append(attrs, xmlb.Attr{Namespace: xmlb.NSWordprocessingML, Name: "rsidDel", Value: p.RsidDel})
	}
	if p.RsidRDefault != "" {
		attrs = append(attrs, xmlb.Attr{Namespace: xmlb.NSWordprocessingML, Name: "rsidRDefault", Value: p.RsidRDefault})
	}
	if p.RsidP != "" {
		attrs = append(attrs, xmlb.Attr{Namespace: xmlb.NSWordprocessingML, Name: "rsidP", Value: p.RsidP})
	}
	if p.CapturedAttrs != nil {
		attrs = b.ReplayCapturedAttrs(p.CapturedAttrs, attrs)
	}
	if p.isEmpty() {
		b.EmptyElementStyled(p.CapturedEmptyTag, ns, localName, attrs...)
		return
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
					marshalMathElement(b, "oMath", p.OMath[ref.index])
				}
			case pChildOMathPara:
				if ref.index < len(p.OMathPara) {
					marshalMathElement(b, "oMathPara", p.OMathPara[ref.index])
				}
			case pChildAlternateContent:
				if ref.index < len(p.AlternateContent) {
					p.AlternateContent[ref.index].MarshalToBuilder(b, ns, "AlternateContent")
				}
			case pChildRaw:
				if ref.index < len(p.Raw) {
					p.Raw[ref.index].MarshalNamed(b, ns)
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

// marshalMathElement writes a captured m:oMath / m:oMathPara element with its
// raw inner content. The math namespace prefix is resolved through the
// builder's namespace table (registered by NewWordprocessingMLBuilder), so the
// element is emitted prefixed instead of as unprefixed <oMath>. When the root
// element did not declare the math namespace, the declaration is emitted
// inline on the element itself; the Builder scopes it to the element, so a
// later sibling gets its own declaration — the output stays well-formed in
// every context.
func marshalMathElement(b *xmlb.Builder, localName string, raw []byte) {
	if b.IsNamespaceDeclared(xmlb.NSMath) {
		b.StartElement(xmlb.NSMath, localName)
		b.WriteRaw(raw)
		b.EndElement(xmlb.NSMath, localName)
		return
	}
	b.StartElementInlineNS(xmlb.NSMath, xmlb.PrefixMath, localName)
	b.WriteRaw(raw)
	b.EndElementInlineNS(xmlb.PrefixMath, localName)
}

// unmarshalPContent is a shared helper for unmarshaling paragraph-level content children
// (used by CT_P, CT_Hyperlink, CT_RunTrackChange, CT_SimpleField).
func unmarshalPContent(d *xml.Decoder,
	r *[]*CT_R, hyperlink *[]*CT_Hyperlink,
	bookmarkStart *[]*CT_BookmarkStart, bookmarkEnd *[]*CT_BookmarkEnd,
	proofErr *[]*CT_ProofErr, permStart *[]*CT_PermStart, permEnd *[]*CT_PermEnd,
	ins *[]*CT_RunTrackChange, del *[]*CT_RunTrackChange,
	fldSimple *[]*CT_SimpleField, sdtRun *[]*CT_SdtRun,
	raw *[]*CT_RawNamedElement,
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
				if raw != nil && isRawPChild(t.Name.Local) {
					v := &CT_RawNamedElement{}
					if err := d.DecodeElement(v, &t); err != nil {
						return err
					}
					*childOrder = append(*childOrder, pChildRef{pChildRaw, len(*raw)})
					*raw = append(*raw, v)
					continue
				}
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
	raw []*CT_RawNamedElement,
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
			case pChildRaw:
				if ref.index < len(raw) {
					raw[ref.index].MarshalNamed(b, ns)
				}
			}
		}
	} else {
		for _, v := range r {
			v.MarshalToBuilder(b, ns, "r")
		}
	}
}

// backfillChildOrder records any existing untracked children in childOrder,
// grouped by kind in slice order. A paragraph built programmatically (e.g.
// SetRuns on an empty order writes p.R without tracking) has typed children
// but an empty childOrder; the first tracked append would otherwise flip
// marshaling to the childOrder-only path and silently drop them.
func (p *CT_P) backfillChildOrder() {
	if len(p.childOrder) > 0 {
		return
	}
	for i := range p.R {
		p.childOrder = append(p.childOrder, pChildRef{pChildR, i})
	}
	for i := range p.Hyperlink {
		p.childOrder = append(p.childOrder, pChildRef{pChildHyperlink, i})
	}
	for i := range p.BookmarkStart {
		p.childOrder = append(p.childOrder, pChildRef{pChildBookmarkStart, i})
	}
	for i := range p.BookmarkEnd {
		p.childOrder = append(p.childOrder, pChildRef{pChildBookmarkEnd, i})
	}
	for i := range p.ProofErr {
		p.childOrder = append(p.childOrder, pChildRef{pChildProofErr, i})
	}
	for i := range p.PermStart {
		p.childOrder = append(p.childOrder, pChildRef{pChildPermStart, i})
	}
	for i := range p.PermEnd {
		p.childOrder = append(p.childOrder, pChildRef{pChildPermEnd, i})
	}
	for i := range p.Ins {
		p.childOrder = append(p.childOrder, pChildRef{pChildIns, i})
	}
	for i := range p.Del {
		p.childOrder = append(p.childOrder, pChildRef{pChildDel, i})
	}
	for i := range p.FldSimple {
		p.childOrder = append(p.childOrder, pChildRef{pChildFldSimple, i})
	}
	for i := range p.SdtRun {
		p.childOrder = append(p.childOrder, pChildRef{pChildSdtRun, i})
	}
	for i := range p.CommentRangeStart {
		p.childOrder = append(p.childOrder, pChildRef{pChildCommentRangeStart, i})
	}
	for i := range p.CommentRangeEnd {
		p.childOrder = append(p.childOrder, pChildRef{pChildCommentRangeEnd, i})
	}
	for i := range p.OMath {
		p.childOrder = append(p.childOrder, pChildRef{pChildOMath, i})
	}
	for i := range p.OMathPara {
		p.childOrder = append(p.childOrder, pChildRef{pChildOMathPara, i})
	}
	for i := range p.AlternateContent {
		p.childOrder = append(p.childOrder, pChildRef{pChildAlternateContent, i})
	}
	for i := range p.Raw {
		p.childOrder = append(p.childOrder, pChildRef{pChildRaw, i})
	}
}

// ClearContent removes every content child of the paragraph (runs,
// hyperlinks, SDTs, tracked changes, fields, comment/bookmark markers, math,
// AlternateContent, and raw-preserved inline elements), resetting the recorded
// child order so later appends do not resolve stale references. Paragraph
// properties (PPr) are kept.
func (p *CT_P) ClearContent() {
	p.R = nil
	p.Hyperlink = nil
	p.BookmarkStart = nil
	p.BookmarkEnd = nil
	p.ProofErr = nil
	p.PermStart = nil
	p.PermEnd = nil
	p.Ins = nil
	p.Del = nil
	p.FldSimple = nil
	p.SdtRun = nil
	p.CommentRangeStart = nil
	p.CommentRangeEnd = nil
	p.OMath = nil
	p.OMathPara = nil
	p.AlternateContent = nil
	p.Raw = nil
	p.childOrder = nil
}

// AppendR appends a run to this paragraph, maintaining child order so it is
// marshaled even on a paragraph parsed from a file (whose order is already
// populated). Existing untracked children are backfilled into the order first
// so they are not dropped by the childOrder-gated marshal.
func (p *CT_P) AppendR(r *CT_R) {
	p.backfillChildOrder()
	p.childOrder = append(p.childOrder, pChildRef{pChildR, len(p.R)})
	p.R = append(p.R, r)
}

// AppendIns appends an insertion block (w:ins) to this paragraph, maintaining
// child order like AppendR so it survives the childOrder-gated marshal of
// paragraphs parsed from a file. Existing untracked children are backfilled
// into the order first so they are not dropped.
func (p *CT_P) AppendIns(block *CT_RunTrackChange) {
	p.backfillChildOrder()
	p.childOrder = append(p.childOrder, pChildRef{pChildIns, len(p.Ins)})
	p.Ins = append(p.Ins, block)
}

// AppendDel appends a deletion block (w:del) to this paragraph, maintaining
// child order like AppendIns.
func (p *CT_P) AppendDel(block *CT_RunTrackChange) {
	p.backfillChildOrder()
	p.childOrder = append(p.childOrder, pChildRef{pChildDel, len(p.Del)})
	p.Del = append(p.Del, block)
}

// AppendOMath appends a raw-captured m:oMath child — the element's inner XML
// with m:/w:-prefixed names, exactly the form UnmarshalXML captures and the
// omml marshaler produces — maintaining child order. Existing untracked
// children are backfilled first so they are not dropped by the
// childOrder-gated marshal.
func (p *CT_P) AppendOMath(raw []byte) {
	p.backfillChildOrder()
	p.childOrder = append(p.childOrder, pChildRef{pChildOMath, len(p.OMath)})
	p.OMath = append(p.OMath, raw)
}

// AppendOMathPara appends a raw-captured m:oMathPara child (see AppendOMath).
func (p *CT_P) AppendOMathPara(raw []byte) {
	p.backfillChildOrder()
	p.childOrder = append(p.childOrder, pChildRef{pChildOMathPara, len(p.OMathPara)})
	p.OMathPara = append(p.OMathPara, raw)
}

// AppendFldSimple appends a simple field (w:fldSimple) to this paragraph,
// maintaining child order like AppendR so it survives the childOrder-gated
// marshal of paragraphs parsed from a file.
func (p *CT_P) AppendFldSimple(f *CT_SimpleField) {
	p.backfillChildOrder()
	p.childOrder = append(p.childOrder, pChildRef{pChildFldSimple, len(p.FldSimple)})
	p.FldSimple = append(p.FldSimple, f)
}

// SetRuns replaces the paragraph's top-level runs with rs, keeping childOrder
// consistent so stale run references neither drop nor duplicate content. On a
// tracked paragraph the new runs take the position of the first existing run
// reference and all other run references are removed; a paragraph with no
// recorded order stays untracked (the fallback marshal writes runs directly).
func (p *CT_P) SetRuns(rs []*CT_R) {
	p.R = rs
	if len(p.childOrder) == 0 {
		return
	}
	newOrder := make([]pChildRef, 0, len(p.childOrder)+len(rs))
	inserted := false
	for _, ref := range p.childOrder {
		if ref.kind == pChildR {
			if !inserted {
				for i := range rs {
					newOrder = append(newOrder, pChildRef{pChildR, i})
				}
				inserted = true
			}
			continue
		}
		newOrder = append(newOrder, ref)
	}
	if !inserted {
		for i := range rs {
			newOrder = append(newOrder, pChildRef{pChildR, i})
		}
	}
	p.childOrder = newOrder
}
