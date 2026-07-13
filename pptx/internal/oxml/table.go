package oxml

import (
	"encoding/xml"

	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/common/dml/chart"
	"github.com/mgilbir/spine/common/dml/diagram"
	xmlb "github.com/mgilbir/spine/common/xml"
)

// GraphicFrame represents a graphic frame element (p:graphicFrame) that contains tables.
type GraphicFrame struct {
	XMLName          xml.Name          `xml:"http://schemas.openxmlformats.org/presentationml/2006/main graphicFrame"`
	NvGraphicFramePr *NvGraphicFramePr `xml:"nvGraphicFramePr"`
	Xfrm             *dml.Xfrm         `xml:"xfrm"`
	Graphic          *AGraphic         `xml:"http://schemas.openxmlformats.org/drawingml/2006/main graphic"`
	ExtLst           *ExtensionList    `xml:"extLst,omitempty"`
}

// NvGraphicFramePr contains non-visual graphic frame properties.
type NvGraphicFramePr struct {
	CNvPr             *dml.CNvPr         `xml:"cNvPr"`
	CNvGraphicFramePr *CNvGraphicFramePr `xml:"cNvGraphicFramePr"`
	NvPr              *NvPr              `xml:"nvPr"`
}

// CNvGraphicFramePr is CT_NonVisualGraphicFrameProperties, shared with
// common/dml (graphicFrameLocks + extLst).
type CNvGraphicFramePr = dml.CNvGraphicFramePr

// GraphicFrameLocks is CT_GraphicalObjectFrameLocking. It is the shared
// DrawingML type: the previous local model exposed only noGrp and silently
// dropped noChangeAspect, noDrilldown, noSelect, noMove, noResize and extLst.
type GraphicFrameLocks = dml.GraphicFrameLocks

// AGraphic represents a DrawingML graphic container.
type AGraphic struct {
	GraphicData *AGraphicData `xml:"http://schemas.openxmlformats.org/drawingml/2006/main graphicData"`
}

// Graphic data URI constants.
const (
	DiagramGraphicDataURI = "http://schemas.openxmlformats.org/drawingml/2006/diagram"
	ChartGraphicDataURI   = "http://schemas.openxmlformats.org/drawingml/2006/chart"
)

// AGraphicData contains the graphic data with URI identifying the type.
// Known URIs populate exactly one of Table, DiagramRelIds, or ChartRef.
// Unknown URIs (e.g. embedded OLE objects) are preserved verbatim in
// RawContent so their payload survives re-marshaling.
type AGraphicData struct {
	URI           string          `xml:"uri,attr"`
	Table         *ATable         `xml:"http://schemas.openxmlformats.org/drawingml/2006/main tbl,omitempty"`
	DiagramRelIds *diagram.RelIds `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram relIds,omitempty"`
	ChartRef      *chart.RelId    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart chart,omitempty"`

	// Fallback for unknown URIs: raw inner XML plus any xmlns declarations
	// carried on the graphicData element itself.
	RawContent    []byte        `xml:"-"`
	InlineNSDecls []xmlb.NSDecl `xml:"-"`
}

// aGraphicDataNoMethods mirrors AGraphicData without its methods so the
// typed URIs keep using the tag-driven decode/encode paths.
type aGraphicDataNoMethods AGraphicData

// UnmarshalXML implements custom unmarshaling for AGraphicData: known URIs
// decode into typed fields; unknown URIs capture their content verbatim.
func (g *AGraphicData) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	var uri string
	for _, attr := range start.Attr {
		if attr.Name.Space == "" && attr.Name.Local == "uri" {
			uri = attr.Value
		}
	}

	switch uri {
	case TableGraphicDataURI, DiagramGraphicDataURI, ChartGraphicDataURI:
		var tmp aGraphicDataNoMethods
		if err := d.DecodeElement(&tmp, &start); err != nil {
			return err
		}
		*g = AGraphicData(tmp)
		return nil
	}

	// Unknown URI (e.g. OLE objects): preserve raw bytes for round-trip.
	g.URI = uri
	for _, attr := range start.Attr {
		switch {
		case attr.Name.Space == "xmlns":
			g.InlineNSDecls = append(g.InlineNSDecls, xmlb.NSDecl{Prefix: attr.Name.Local, URI: attr.Value})
		case attr.Name.Space == "" && attr.Name.Local == "xmlns":
			g.InlineNSDecls = append(g.InlineNSDecls, xmlb.NSDecl{URI: attr.Value})
		}
	}
	var inner struct {
		Content []byte `xml:",innerxml"`
	}
	if err := d.DecodeElement(&inner, &start); err != nil {
		return err
	}
	g.RawContent = inner.Content
	return nil
}

// MarshalToBuilder implements xmlb.BuilderMarshaler for AGraphicData.
// Typed content marshals through the tag-driven reflection path; unknown
// URIs re-emit their captured raw content.
func (g *AGraphicData) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	if g.Table != nil || g.DiagramRelIds != nil || g.ChartRef != nil ||
		(len(g.RawContent) == 0 && len(g.InlineNSDecls) == 0) {
		b.MarshalElement(ns, localName, (*aGraphicDataNoMethods)(g))
		return
	}

	attrs := []xmlb.Attr{xmlb.StrAttr("uri", g.URI)}
	for _, nsd := range g.InlineNSDecls {
		name := "xmlns"
		if nsd.Prefix != "" {
			name = "xmlns:" + nsd.Prefix
		}
		attrs = append(attrs, xmlb.Attr{Name: name, Value: nsd.URI})
	}
	if len(g.RawContent) == 0 {
		b.EmptyElement(ns, localName, attrs...)
		return
	}
	b.StartElement(ns, localName, attrs...)
	b.WriteRaw(g.RawContent)
	b.EndElement(ns, localName)
}

// TableGraphicDataURI is the URI for table graphic data.
const TableGraphicDataURI = "http://schemas.openxmlformats.org/drawingml/2006/table"

// ATable represents a DrawingML table.
type ATable struct {
	TblPr   *ATblPr   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main tblPr,omitempty"`
	TblGrid *ATblGrid `xml:"http://schemas.openxmlformats.org/drawingml/2006/main tblGrid"`
	Tr      []*ATr    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main tr"`
}

// ATblPr contains table properties (CT_TableProperties). Child order follows
// the XSD sequence: fill choice, effect choice, tableStyle|tableStyleId,
// extLst.
type ATblPr struct {
	Rtl           bool             `xml:"rtl,attr,omitempty"`
	FirstRow      bool             `xml:"firstRow,attr,omitempty"`
	FirstCol      bool             `xml:"firstCol,attr,omitempty"`
	LastRow       bool             `xml:"lastRow,attr,omitempty"`
	LastCol       bool             `xml:"lastCol,attr,omitempty"`
	BandRow       bool             `xml:"bandRow,attr,omitempty"`
	BandCol       bool             `xml:"bandCol,attr,omitempty"`
	NoFill        *dml.NoFillXML   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main noFill,omitempty"`
	SolidFill     *dml.SolidFill   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main solidFill,omitempty"`
	GradFill      *dml.GradFill    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main gradFill,omitempty"`
	BlipFill      *dml.BlipFillXML `xml:"http://schemas.openxmlformats.org/drawingml/2006/main blipFill,omitempty"`
	PattFill      *dml.PattFill    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main pattFill,omitempty"`
	GrpFill       *dml.GrpFill     `xml:"http://schemas.openxmlformats.org/drawingml/2006/main grpFill,omitempty"`
	EffectLst     *dml.EffectLst   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main effectLst,omitempty"`
	EffectDag     *dml.EffectDag   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main effectDag,omitempty"`
	TableStyle    *dml.TableStyle  `xml:"http://schemas.openxmlformats.org/drawingml/2006/main tableStyle,omitempty"`
	TableStyleId  string           `xml:"http://schemas.openxmlformats.org/drawingml/2006/main tableStyleId,omitempty"`
	ExtLst        *dml.ExtLst      `xml:"http://schemas.openxmlformats.org/drawingml/2006/main extLst,omitempty"`
	CapturedAttrs []xmlb.RootAttr  `xml:"-"` // verbatim source attrs; see common/xml.CaptureAttrs
}

// UnmarshalXML captures the element's verbatim attribute list (source
// attribute order and any unmodeled attributes) before decoding through the
// struct tags; the reflection marshaler replays it.
func (atb *ATblPr) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	atb.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias ATblPr
	return d.DecodeElement((*alias)(atb), &start)
}

// ATblGrid contains table grid column definitions.
type ATblGrid struct {
	GridCol []*AGridCol `xml:"http://schemas.openxmlformats.org/drawingml/2006/main gridCol"`
}

// AGridCol represents a grid column with width.
type AGridCol struct {
	W      int64       `xml:"w,attr"`
	ExtLst *dml.ExtLst `xml:"http://schemas.openxmlformats.org/drawingml/2006/main extLst,omitempty"`
}

// ATr represents a table row.
type ATr struct {
	// H is a pointer so an explicit h="0" survives the round trip.
	H      *int64      `xml:"h,attr,omitempty"`
	Tc     []*ATc      `xml:"http://schemas.openxmlformats.org/drawingml/2006/main tc"`
	ExtLst *dml.ExtLst `xml:"http://schemas.openxmlformats.org/drawingml/2006/main extLst,omitempty"`
}

// ATc represents a table cell (CT_TableCell).
// Field order matches XSD sequence: txBody, tcPr, extLst.
type ATc struct {
	TxBody        *dml.TxBody     `xml:"http://schemas.openxmlformats.org/drawingml/2006/main txBody,omitempty"`
	TcPr          *ATcPr          `xml:"http://schemas.openxmlformats.org/drawingml/2006/main tcPr,omitempty"`
	ExtLst        *dml.ExtLst     `xml:"http://schemas.openxmlformats.org/drawingml/2006/main extLst,omitempty"`
	RowSpan       int             `xml:"rowSpan,attr,omitempty"`
	GridSpan      int             `xml:"gridSpan,attr,omitempty"`
	HMerge        bool            `xml:"hMerge,attr,omitempty"`
	VMerge        bool            `xml:"vMerge,attr,omitempty"`
	Id            string          `xml:"id,attr,omitempty"`
	CapturedAttrs []xmlb.RootAttr `xml:"-"` // verbatim source attrs; see common/xml.CaptureAttrs
}

// UnmarshalXML captures the element's verbatim attribute list (source
// attribute order and any unmodeled attributes) before decoding through the
// struct tags; the reflection marshaler replays it.
func (atc *ATc) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	atc.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias ATc
	return d.DecodeElement((*alias)(atc), &start)
}

// ATcPr contains table cell properties (CT_TableCellProperties). Child order
// follows the XSD sequence: lnL, lnR, lnT, lnB, lnTlToBr, lnBlToTr, cell3D,
// fill choice, headers, extLst.
type ATcPr struct {
	MarL          *int64           `xml:"marL,attr,omitempty"`
	MarR          *int64           `xml:"marR,attr,omitempty"`
	MarT          *int64           `xml:"marT,attr,omitempty"`
	MarB          *int64           `xml:"marB,attr,omitempty"`
	Vert          string           `xml:"vert,attr,omitempty"`
	Anchor        string           `xml:"anchor,attr,omitempty"`
	AnchorCtr     *bool            `xml:"anchorCtr,attr,omitempty"`
	HorzOverflow  string           `xml:"horzOverflow,attr,omitempty"`
	LnL           *dml.Ln          `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lnL,omitempty"`
	LnR           *dml.Ln          `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lnR,omitempty"`
	LnT           *dml.Ln          `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lnT,omitempty"`
	LnB           *dml.Ln          `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lnB,omitempty"`
	LnTlToBr      *dml.Ln          `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lnTlToBr,omitempty"`
	LnBlToTr      *dml.Ln          `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lnBlToTr,omitempty"`
	Cell3D        *dml.Cell3D      `xml:"http://schemas.openxmlformats.org/drawingml/2006/main cell3D,omitempty"`
	NoFill        *dml.NoFillXML   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main noFill,omitempty"`
	SolidFill     *dml.SolidFill   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main solidFill,omitempty"`
	GradFill      *dml.GradFill    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main gradFill,omitempty"`
	BlipFill      *dml.BlipFillXML `xml:"http://schemas.openxmlformats.org/drawingml/2006/main blipFill,omitempty"`
	PattFill      *dml.PattFill    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main pattFill,omitempty"`
	GrpFill       *dml.GrpFill     `xml:"http://schemas.openxmlformats.org/drawingml/2006/main grpFill,omitempty"`
	Headers       *dml.Headers     `xml:"http://schemas.openxmlformats.org/drawingml/2006/main headers,omitempty"`
	ExtLst        *dml.ExtLst      `xml:"http://schemas.openxmlformats.org/drawingml/2006/main extLst,omitempty"`
	CapturedAttrs []xmlb.RootAttr  `xml:"-"` // verbatim source attrs; see common/xml.CaptureAttrs
}

// UnmarshalXML captures the element's verbatim attribute list (source
// attribute order and any unmodeled attributes) before decoding through the
// struct tags; the reflection marshaler replays it.
func (atp *ATcPr) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	atp.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias ATcPr
	return d.DecodeElement((*alias)(atp), &start)
}
