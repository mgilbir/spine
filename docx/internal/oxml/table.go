package oxml

import (
	"encoding/xml"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// CT_TblPr represents table properties (w:tblPr).
type CT_TblPr struct {
	TblStyle            *CT_String        `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main tblStyle,omitempty"`
	TblpPr              *CT_TblPPr        `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main tblpPr,omitempty"`
	TblOverlap          *CT_String        `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main tblOverlap,omitempty"`
	BidiVisual          *CT_OnOff         `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main bidiVisual,omitempty"`
	TblStyleRowBandSize *CT_DecimalNumber `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main tblStyleRowBandSize,omitempty"`
	TblStyleColBandSize *CT_DecimalNumber `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main tblStyleColBandSize,omitempty"`
	TblW                *CT_TblWidth      `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main tblW,omitempty"`
	Jc                  *CT_Jc            `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main jc,omitempty"`
	TblCellSpacing      *CT_TblWidth      `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main tblCellSpacing,omitempty"`
	TblInd              *CT_TblWidth      `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main tblInd,omitempty"`
	TblBorders          *CT_TblBorders    `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main tblBorders,omitempty"`
	Shd                 *CT_Shd           `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main shd,omitempty"`
	TblLayout           *CT_TblLayout     `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main tblLayout,omitempty"`
	TblCellMar          *CT_TblCellMar    `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main tblCellMar,omitempty"`
	TblLook             *CT_TblLook       `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main tblLook,omitempty"`
	TblCaption          *CT_String        `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main tblCaption,omitempty"`
	TblDescription      *CT_String        `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main tblDescription,omitempty"`
	TblPrChange         *CT_TblPrChange   `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main tblPrChange,omitempty"`
}

// CT_TblPPr represents table positioning properties.
type CT_TblPPr struct {
	LeftFromText   string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main leftFromText,attr,omitempty"`
	RightFromText  string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main rightFromText,attr,omitempty"`
	TopFromText    string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main topFromText,attr,omitempty"`
	BottomFromText string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main bottomFromText,attr,omitempty"`
	VertAnchor     string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main vertAnchor,attr,omitempty"`
	HorzAnchor     string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main horzAnchor,attr,omitempty"`
	TblpXSpec      string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main tblpXSpec,attr,omitempty"`
	TblpYSpec      string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main tblpYSpec,attr,omitempty"`
	TblpX          string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main tblpX,attr,omitempty"`
	TblpY          string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main tblpY,attr,omitempty"`
}

// CT_TblLayout represents table layout mode.
type CT_TblLayout struct {
	Type string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main type,attr,omitempty"`
}

// CT_TblCellMar represents table cell margins.
type CT_TblCellMar struct {
	Top    *CT_TblWidth `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main top,omitempty"`
	Left   *CT_TblWidth `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main left,omitempty"`
	Bottom *CT_TblWidth `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main bottom,omitempty"`
	Right  *CT_TblWidth `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main right,omitempty"`
}

// CT_TblLook represents conditional formatting flags for a table.
type CT_TblLook struct {
	Val         string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr,omitempty"`
	FirstRow    string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main firstRow,attr,omitempty"`
	LastRow     string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main lastRow,attr,omitempty"`
	FirstColumn string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main firstColumn,attr,omitempty"`
	LastColumn  string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main lastColumn,attr,omitempty"`
	NoHBand     string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main noHBand,attr,omitempty"`
	NoVBand     string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main noVBand,attr,omitempty"`
}

// CT_TblGrid represents table grid definition.
type CT_TblGrid struct {
	GridCol []CT_GridCol `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main gridCol"`
}

// CT_GridCol represents a single grid column.
type CT_GridCol struct {
	W string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main w,attr,omitempty"`
}

// CT_Cnf represents conditional-formatting properties (w:cnfStyle, CT_Cnf).
// Word writes w:val plus twelve explicit ST_OnOff attributes; they are kept
// as strings so explicit zeros and lexical forms round-trip.
type CT_Cnf struct {
	Val                 string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr,omitempty"`
	FirstRow            string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main firstRow,attr,omitempty"`
	LastRow             string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main lastRow,attr,omitempty"`
	FirstColumn         string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main firstColumn,attr,omitempty"`
	LastColumn          string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main lastColumn,attr,omitempty"`
	OddVBand            string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main oddVBand,attr,omitempty"`
	EvenVBand           string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main evenVBand,attr,omitempty"`
	OddHBand            string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main oddHBand,attr,omitempty"`
	EvenHBand           string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main evenHBand,attr,omitempty"`
	FirstRowFirstColumn string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main firstRowFirstColumn,attr,omitempty"`
	FirstRowLastColumn  string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main firstRowLastColumn,attr,omitempty"`
	LastRowFirstColumn  string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main lastRowFirstColumn,attr,omitempty"`
	LastRowLastColumn   string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main lastRowLastColumn,attr,omitempty"`
}

// CT_TrPr represents table row properties.
type CT_TrPr struct {
	CnfStyle         *CT_Cnf            `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main cnfStyle,omitempty"`
	DivId            *CT_DecimalNumber  `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main divId,omitempty"`
	GridBefore       *CT_DecimalNumber  `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main gridBefore,omitempty"`
	GridAfter        *CT_DecimalNumber  `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main gridAfter,omitempty"`
	WBefore          *CT_TblWidth       `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main wBefore,omitempty"`
	WAfter           *CT_TblWidth       `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main wAfter,omitempty"`
	CantSplit        *CT_OnOff          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main cantSplit,omitempty"`
	TrHeight         *CT_Height         `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main trHeight,omitempty"`
	TblHeader        *CT_OnOff          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main tblHeader,omitempty"`
	TblCellSpacing   *CT_TblWidth       `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main tblCellSpacing,omitempty"`
	Jc               *CT_Jc             `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main jc,omitempty"`
	Hidden           *CT_OnOff          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main hidden,omitempty"`
	Ins              *CT_TrackChange    `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main ins,omitempty"`
	Del              *CT_TrackChange    `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main del,omitempty"`
	TrPrChange       *CT_TrPrChange     `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main trPrChange,omitempty"`
	CapturedEmptyTag xmlb.EmptyTagStyle `xml:"-"` // empty-element style; see common/xml.CaptureEmptyTagStyle
}

// UnmarshalXML captures the element's empty-tag style before decoding
// through the struct tags.
func (tp2 *CT_TrPr) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	tp2.CapturedEmptyTag = xmlb.CaptureEmptyTagStyle(d)
	type alias CT_TrPr
	return d.DecodeElement((*alias)(tp2), &start)
}

// CT_Height represents a height measurement.
type CT_Height struct {
	Val           string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr,omitempty"`
	HRule         string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main hRule,attr,omitempty"`
	CapturedAttrs []xmlb.RootAttr `xml:"-"` // verbatim source attrs; see common/xml.CaptureAttrs
}

// UnmarshalXML captures the element's verbatim attribute list (source
// attribute order and any unmodeled attributes) before decoding through the
// struct tags; the reflection marshaler replays it.
func (h *CT_Height) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	h.CapturedAttrs = xmlb.CaptureAttrs(start.Attr)
	type alias CT_Height
	return d.DecodeElement((*alias)(h), &start)
}

// CT_TrackChange represents a simple track change marker (no content).
type CT_TrackChange struct {
	Id            string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main id,attr"`
	Author        string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main author,attr"`
	Date          string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main date,attr,omitempty"`
	CapturedAttrs []xmlb.RootAttr `xml:"-"` // verbatim source attrs; see common/xml.CaptureAttrs
}

// UnmarshalXML captures the element's verbatim attribute list (source
// attribute order and any unmodeled attributes) before decoding through the
// struct tags; the reflection marshaler replays it.
func (tc2 *CT_TrackChange) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	tc2.CapturedAttrs = xmlb.CaptureAttrs(start.Attr)
	type alias CT_TrackChange
	return d.DecodeElement((*alias)(tc2), &start)
}

// CT_TcPr represents table cell properties.
type CT_TcPr struct {
	CnfStyle      *CT_Cnf           `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main cnfStyle,omitempty"`
	TcW           *CT_TblWidth      `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main tcW,omitempty"`
	GridSpan      *CT_DecimalNumber `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main gridSpan,omitempty"`
	HMerge        *CT_String        `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main hMerge,omitempty"`
	VMerge        *CT_String        `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main vMerge,omitempty"`
	TcBorders     *CT_TcBorders     `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main tcBorders,omitempty"`
	Shd           *CT_Shd           `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main shd,omitempty"`
	NoWrap        *CT_OnOff         `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main noWrap,omitempty"`
	TcMar         *CT_TblCellMar    `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main tcMar,omitempty"`
	TextDirection *CT_String        `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main textDirection,omitempty"`
	TcFitText     *CT_OnOff         `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main tcFitText,omitempty"`
	VAlign        *CT_String        `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main vAlign,omitempty"`
	HideMark      *CT_OnOff         `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main hideMark,omitempty"`
	CellIns       *CT_TrackChange   `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main cellIns,omitempty"`
	CellDel       *CT_TrackChange   `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main cellDel,omitempty"`
	CellMerge     *CT_CellMerge     `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main cellMerge,omitempty"`
	TcPrChange    *CT_TcPrChange    `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main tcPrChange,omitempty"`
}

// CT_CellMerge represents cell merge revision information.
type CT_CellMerge struct {
	Id         string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main id,attr,omitempty"`
	Author     string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main author,attr,omitempty"`
	Date       string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main date,attr,omitempty"`
	VMerge     string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main vMerge,attr,omitempty"`
	VMergeOrig string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main vMergeOrig,attr,omitempty"`
}

// CT_Tc represents a table cell (w:tc).
type CT_Tc struct {
	TcPr          *CT_TcPr              `xml:"-"`
	P             []*CT_P               `xml:"-"`
	Tbl           []*CT_Tbl             `xml:"-"`
	SdtBlock      []*CT_SdtBlock        `xml:"-"`
	BookmarkStart []*CT_BookmarkStart   `xml:"-"`
	BookmarkEnd   []*CT_BookmarkEnd     `xml:"-"`
	Raw           []*CT_RawNamedElement `xml:"-"`
	childOrder    []bodyChildRef
}

// UnmarshalXML implements custom unmarshaling for CT_Tc.
func (tc *CT_Tc) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "tcPr":
				tc.TcPr = &CT_TcPr{}
				if err := d.DecodeElement(tc.TcPr, &t); err != nil {
					return err
				}
			default:
				if err := unmarshalBodyChild(d, &t, &tc.P, &tc.Tbl, &tc.SdtBlock, &tc.BookmarkStart, &tc.BookmarkEnd, &tc.Raw, &tc.childOrder); err != nil {
					return err
				}
			}
		case xml.EndElement:
			return nil
		}
	}
}

// MarshalToBuilder implements xmlb.BuilderMarshaler for CT_Tc.
func (tc *CT_Tc) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	b.StartElement(ns, localName)
	if tc.TcPr != nil {
		b.MarshalElement(ns, "tcPr", tc.TcPr)
	}
	marshalBodyContent(b, ns, tc.P, tc.Tbl, tc.SdtBlock, tc.BookmarkStart, tc.BookmarkEnd, tc.Raw, tc.childOrder)
	b.EndElement(ns, localName)
}

// trChildKind identifies table row children.
type trChildKind int

const (
	trChildTc trChildKind = iota
	trChildBookmarkStart
	trChildBookmarkEnd
	trChildCustomXmlCell
	trChildSdtCell
	trChildIns
	trChildDel
	trChildRaw
)

// isRawRowChild reports whether a row-level child element the model does not
// type must be preserved verbatim instead of skipped: row-level w:customXml
// (which wraps whole cells) and the tracked-move range markers.
func isRawRowChild(local string) bool {
	switch local {
	case "customXml",
		"moveFromRangeStart", "moveFromRangeEnd", "moveToRangeStart", "moveToRangeEnd":
		return true
	}
	return false
}

// trChildRef references a table row child.
type trChildRef struct {
	kind  trChildKind
	index int
}

// CT_Tr represents a table row (w:tr).
type CT_Tr struct {
	// CapturedAttrs preserves the verbatim source attribute list; replayed on
	// marshal (producer rsid order varies, e.g. rsidDel is not modeled).
	CapturedAttrs []xmlb.RootAttr `xml:"-"`
	RsidR         string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main rsidR,attr,omitempty"`
	RsidRPr       string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main rsidRPr,attr,omitempty"`
	RsidTr        string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main rsidTr,attr,omitempty"`
	ParaId        string          `xml:"-"` // w14:paraId
	TextId        string          `xml:"-"` // w14:textId
	// TblPrEx (w:tblPrEx, table property exceptions for this row) is preserved
	// raw; it precedes trPr in the schema and is re-emitted in that position.
	TblPrEx       *CT_RawElement        `xml:"-"`
	TrPr          *CT_TrPr              `xml:"-"`
	Tc            []*CT_Tc              `xml:"-"`
	BookmarkStart []*CT_BookmarkStart   `xml:"-"`
	BookmarkEnd   []*CT_BookmarkEnd     `xml:"-"`
	SdtCell       []*CT_SdtBlock        `xml:"-"`
	Ins           []*CT_RunTrackChange  `xml:"-"`
	Del           []*CT_RunTrackChange  `xml:"-"`
	Raw           []*CT_RawNamedElement `xml:"-"`
	childOrder    []trChildRef
}

// UnmarshalXML implements custom unmarshaling for CT_Tr.
func (tr *CT_Tr) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	tr.CapturedAttrs = xmlb.CaptureAttrs(start.Attr)
	for _, attr := range start.Attr {
		switch attr.Name.Local {
		case "rsidR":
			tr.RsidR = attr.Value
		case "rsidRPr":
			tr.RsidRPr = attr.Value
		case "rsidTr":
			tr.RsidTr = attr.Value
		case "paraId":
			tr.ParaId = attr.Value
		case "textId":
			tr.TextId = attr.Value
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
			case "tblPrEx":
				tr.TblPrEx = &CT_RawElement{}
				if err := d.DecodeElement(tr.TblPrEx, &t); err != nil {
					return err
				}
			case "trPr":
				tr.TrPr = &CT_TrPr{}
				if err := d.DecodeElement(tr.TrPr, &t); err != nil {
					return err
				}
			case "tc":
				v := &CT_Tc{}
				if err := d.DecodeElement(v, &t); err != nil {
					return err
				}
				tr.childOrder = append(tr.childOrder, trChildRef{trChildTc, len(tr.Tc)})
				tr.Tc = append(tr.Tc, v)
			case "bookmarkStart":
				v := &CT_BookmarkStart{}
				if err := d.DecodeElement(v, &t); err != nil {
					return err
				}
				tr.childOrder = append(tr.childOrder, trChildRef{trChildBookmarkStart, len(tr.BookmarkStart)})
				tr.BookmarkStart = append(tr.BookmarkStart, v)
			case "bookmarkEnd":
				v := &CT_BookmarkEnd{}
				if err := d.DecodeElement(v, &t); err != nil {
					return err
				}
				tr.childOrder = append(tr.childOrder, trChildRef{trChildBookmarkEnd, len(tr.BookmarkEnd)})
				tr.BookmarkEnd = append(tr.BookmarkEnd, v)
			case "sdt":
				v := &CT_SdtBlock{}
				if err := d.DecodeElement(v, &t); err != nil {
					return err
				}
				tr.childOrder = append(tr.childOrder, trChildRef{trChildSdtCell, len(tr.SdtCell)})
				tr.SdtCell = append(tr.SdtCell, v)
			case "ins":
				v := &CT_RunTrackChange{}
				if err := d.DecodeElement(v, &t); err != nil {
					return err
				}
				tr.childOrder = append(tr.childOrder, trChildRef{trChildIns, len(tr.Ins)})
				tr.Ins = append(tr.Ins, v)
			case "del":
				v := &CT_RunTrackChange{}
				if err := d.DecodeElement(v, &t); err != nil {
					return err
				}
				tr.childOrder = append(tr.childOrder, trChildRef{trChildDel, len(tr.Del)})
				tr.Del = append(tr.Del, v)
			default:
				if isRawRowChild(t.Name.Local) {
					v := &CT_RawNamedElement{}
					if err := d.DecodeElement(v, &t); err != nil {
						return err
					}
					tr.childOrder = append(tr.childOrder, trChildRef{trChildRaw, len(tr.Raw)})
					tr.Raw = append(tr.Raw, v)
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

// MarshalToBuilder implements xmlb.BuilderMarshaler for CT_Tr.
func (tr *CT_Tr) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	// Word's attribute order: rsidR, rsidRPr, w14:paraId, w14:textId, rsidTr.
	var attrs []xmlb.Attr
	if tr.RsidR != "" {
		attrs = append(attrs, xmlb.Attr{Namespace: xmlb.NSWordprocessingML, Name: "rsidR", Value: tr.RsidR})
	}
	if tr.RsidRPr != "" {
		attrs = append(attrs, xmlb.Attr{Namespace: xmlb.NSWordprocessingML, Name: "rsidRPr", Value: tr.RsidRPr})
	}
	if tr.ParaId != "" {
		attrs = append(attrs, xmlb.Attr{Namespace: xmlb.NSWord2010, Name: "paraId", Value: tr.ParaId})
	}
	if tr.TextId != "" {
		attrs = append(attrs, xmlb.Attr{Namespace: xmlb.NSWord2010, Name: "textId", Value: tr.TextId})
	}
	if tr.RsidTr != "" {
		attrs = append(attrs, xmlb.Attr{Namespace: xmlb.NSWordprocessingML, Name: "rsidTr", Value: tr.RsidTr})
	}
	if tr.CapturedAttrs != nil {
		attrs = b.ReplayCapturedAttrs(tr.CapturedAttrs, attrs)
	}
	b.StartElement(ns, localName, attrs...)

	if tr.TblPrEx != nil {
		tr.TblPrEx.MarshalToBuilder(b, ns, "tblPrEx")
	}
	if tr.TrPr != nil {
		b.MarshalElement(ns, "trPr", tr.TrPr)
	}

	if len(tr.childOrder) > 0 {
		for _, ref := range tr.childOrder {
			switch ref.kind {
			case trChildTc:
				if ref.index < len(tr.Tc) {
					tr.Tc[ref.index].MarshalToBuilder(b, ns, "tc")
				}
			case trChildBookmarkStart:
				if ref.index < len(tr.BookmarkStart) {
					b.MarshalElement(ns, "bookmarkStart", tr.BookmarkStart[ref.index])
				}
			case trChildBookmarkEnd:
				if ref.index < len(tr.BookmarkEnd) {
					b.MarshalElement(ns, "bookmarkEnd", tr.BookmarkEnd[ref.index])
				}
			case trChildSdtCell:
				if ref.index < len(tr.SdtCell) {
					b.MarshalElement(ns, "sdt", tr.SdtCell[ref.index])
				}
			case trChildIns:
				if ref.index < len(tr.Ins) {
					tr.Ins[ref.index].MarshalToBuilder(b, ns, "ins")
				}
			case trChildDel:
				if ref.index < len(tr.Del) {
					tr.Del[ref.index].MarshalToBuilder(b, ns, "del")
				}
			case trChildRaw:
				if ref.index < len(tr.Raw) {
					tr.Raw[ref.index].MarshalNamed(b, ns)
				}
			}
		}
	} else {
		for _, v := range tr.Tc {
			v.MarshalToBuilder(b, ns, "tc")
		}
	}

	b.EndElement(ns, localName)
}

// tblChildKind identifies table child element types.
type tblChildKind int

const (
	tblChildTr tblChildKind = iota
	tblChildBookmarkStart
	tblChildBookmarkEnd
)

// tblChildRef references a table child.
type tblChildRef struct {
	kind  tblChildKind
	index int
}

// backfillChildOrder records any existing untracked children in childOrder,
// grouped by kind in slice order, so the first tracked append on a table
// built programmatically does not flip marshaling to the childOrder-only path
// and drop them.
func (tbl *CT_Tbl) backfillChildOrder() {
	if len(tbl.childOrder) > 0 {
		return
	}
	for i := range tbl.Tr {
		tbl.childOrder = append(tbl.childOrder, tblChildRef{tblChildTr, i})
	}
	for i := range tbl.BookmarkStart {
		tbl.childOrder = append(tbl.childOrder, tblChildRef{tblChildBookmarkStart, i})
	}
	for i := range tbl.BookmarkEnd {
		tbl.childOrder = append(tbl.childOrder, tblChildRef{tblChildBookmarkEnd, i})
	}
}

// AppendRow appends a row and updates child ordering for round-trip edits.
// Existing untracked rows are backfilled into the order first.
func (tbl *CT_Tbl) AppendRow(tr *CT_Tr) {
	tbl.backfillChildOrder()
	tbl.childOrder = append(tbl.childOrder, tblChildRef{tblChildTr, len(tbl.Tr)})
	tbl.Tr = append(tbl.Tr, tr)
}

// backfillChildOrder records any existing untracked children in childOrder,
// grouped by kind in slice order (see CT_Tbl.backfillChildOrder).
func (tr *CT_Tr) backfillChildOrder() {
	if len(tr.childOrder) > 0 {
		return
	}
	for i := range tr.Tc {
		tr.childOrder = append(tr.childOrder, trChildRef{trChildTc, i})
	}
	for i := range tr.BookmarkStart {
		tr.childOrder = append(tr.childOrder, trChildRef{trChildBookmarkStart, i})
	}
	for i := range tr.BookmarkEnd {
		tr.childOrder = append(tr.childOrder, trChildRef{trChildBookmarkEnd, i})
	}
	for i := range tr.SdtCell {
		tr.childOrder = append(tr.childOrder, trChildRef{trChildSdtCell, i})
	}
	for i := range tr.Ins {
		tr.childOrder = append(tr.childOrder, trChildRef{trChildIns, i})
	}
	for i := range tr.Del {
		tr.childOrder = append(tr.childOrder, trChildRef{trChildDel, i})
	}
	for i := range tr.Raw {
		tr.childOrder = append(tr.childOrder, trChildRef{trChildRaw, i})
	}
}

// AppendCell appends a cell and updates child ordering for round-trip edits.
// Existing untracked cells are backfilled into the order first.
func (tr *CT_Tr) AppendCell(tc *CT_Tc) {
	tr.backfillChildOrder()
	tr.childOrder = append(tr.childOrder, trChildRef{trChildTc, len(tr.Tc)})
	tr.Tc = append(tr.Tc, tc)
}

// CT_Tbl represents a table (w:tbl).
type CT_Tbl struct {
	TblPr         *CT_TblPr           `xml:"-"`
	TblGrid       *CT_TblGrid         `xml:"-"`
	Tr            []*CT_Tr            `xml:"-"`
	BookmarkStart []*CT_BookmarkStart `xml:"-"`
	BookmarkEnd   []*CT_BookmarkEnd   `xml:"-"`
	childOrder    []tblChildRef
}

// UnmarshalXML implements custom unmarshaling for CT_Tbl.
func (tbl *CT_Tbl) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "tblPr":
				tbl.TblPr = &CT_TblPr{}
				if err := d.DecodeElement(tbl.TblPr, &t); err != nil {
					return err
				}
			case "tblGrid":
				tbl.TblGrid = &CT_TblGrid{}
				if err := d.DecodeElement(tbl.TblGrid, &t); err != nil {
					return err
				}
			case "tr":
				v := &CT_Tr{}
				if err := d.DecodeElement(v, &t); err != nil {
					return err
				}
				tbl.childOrder = append(tbl.childOrder, tblChildRef{tblChildTr, len(tbl.Tr)})
				tbl.Tr = append(tbl.Tr, v)
			case "bookmarkStart":
				v := &CT_BookmarkStart{}
				if err := d.DecodeElement(v, &t); err != nil {
					return err
				}
				tbl.childOrder = append(tbl.childOrder, tblChildRef{tblChildBookmarkStart, len(tbl.BookmarkStart)})
				tbl.BookmarkStart = append(tbl.BookmarkStart, v)
			case "bookmarkEnd":
				v := &CT_BookmarkEnd{}
				if err := d.DecodeElement(v, &t); err != nil {
					return err
				}
				tbl.childOrder = append(tbl.childOrder, tblChildRef{tblChildBookmarkEnd, len(tbl.BookmarkEnd)})
				tbl.BookmarkEnd = append(tbl.BookmarkEnd, v)
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

// MarshalToBuilder implements xmlb.BuilderMarshaler for CT_Tbl.
func (tbl *CT_Tbl) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	b.StartElement(ns, localName)

	if tbl.TblPr != nil {
		b.MarshalElement(ns, "tblPr", tbl.TblPr)
	}
	if tbl.TblGrid != nil {
		b.MarshalElement(ns, "tblGrid", tbl.TblGrid)
	}

	if len(tbl.childOrder) > 0 {
		for _, ref := range tbl.childOrder {
			switch ref.kind {
			case tblChildTr:
				if ref.index < len(tbl.Tr) {
					tbl.Tr[ref.index].MarshalToBuilder(b, ns, "tr")
				}
			case tblChildBookmarkStart:
				if ref.index < len(tbl.BookmarkStart) {
					b.MarshalElement(ns, "bookmarkStart", tbl.BookmarkStart[ref.index])
				}
			case tblChildBookmarkEnd:
				if ref.index < len(tbl.BookmarkEnd) {
					b.MarshalElement(ns, "bookmarkEnd", tbl.BookmarkEnd[ref.index])
				}
			}
		}
	} else {
		for _, v := range tbl.Tr {
			v.MarshalToBuilder(b, ns, "tr")
		}
	}

	b.EndElement(ns, localName)
}

// AppendP appends a paragraph to this table cell, maintaining child order so it
// is marshaled even on a cell parsed from a file. Existing untracked children
// (e.g. a seed paragraph assigned directly) are backfilled into the order first.
func (tc *CT_Tc) AppendP(p *CT_P) {
	backfillBodyChildOrder(&tc.childOrder, tc.P, tc.Tbl, tc.SdtBlock, tc.BookmarkStart, tc.BookmarkEnd, tc.Raw)
	appendBodyP(&tc.P, &tc.childOrder, p)
}
