// Package diagram provides DrawingML Diagram types from dml-diagram*.xsd.
// These types represent SmartArt/diagram structures in OOXML documents.
//
// The diagram schemas cover:
//   - dml-diagramDefinition.xsd: Data model (points, connections)
//   - dml-diagramLayoutDefinition.xsd: Layout algorithms and constraints
//   - dml-diagramColorTransform.xsd: Color transform definitions
//   - dml-diagramStyle.xsd: Style definitions
//
// Namespace: http://schemas.openxmlformats.org/drawingml/2006/diagram (dgm:)
package diagram

import (
	"github.com/mgilbir/spine/common/dml"
	xmlb "github.com/mgilbir/spine/common/xml"
)

// XML namespace constants
const (
	NsDiagram = "http://schemas.openxmlformats.org/drawingml/2006/diagram"
)

// --- Data Model (dgm:dataModel) ---

// DataModel represents CT_DataModel (dgm:dataModel) - root data element
type DataModel struct {
	PtLst    *PtLst    `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram ptLst,omitempty"`
	CxnLst   *CxnLst   `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram cxnLst,omitempty"`
	Bg       *dml.SpPr  `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram bg,omitempty"`
	Whole    *dml.SpPr  `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram whole,omitempty"`
	ExtLst   *dml.ExtLst `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram extLst,omitempty"`
}

// PtLst represents CT_PtList (dgm:ptLst) - list of data points
type PtLst struct {
	Pt []*Pt `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram pt,omitempty"`
}

// Pt represents CT_Pt (dgm:pt) - a data point in the diagram
type Pt struct {
	ModelId string      `xml:"modelId,attr"`
	Type    string      `xml:"type,attr,omitempty"` // ST_PtType: node, asst, doc, pres, parTrans, sibTrans
	CxnId   string      `xml:"cxnId,attr,omitempty"`
	PrSet   *PrSet      `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram prSet,omitempty"`
	SpPr    *dml.SpPr   `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram spPr,omitempty"`
	T       *dml.TxBody `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram t,omitempty"`
	ExtLst  *dml.ExtLst `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram extLst,omitempty"`
}

// PrSet represents CT_ElemPropSet (dgm:prSet) - element property set.
//
// CT_ElemPropSet is not attribute-only: it carries a dgm:presLayoutVars
// (CT_LayoutVariablePropertySet) and a dgm:style (a:CT_ShapeStyle), both of
// which a parse→marshal silently discarded until C485. PowerPoint writes
// presLayoutVars on presentation points and style on manually restyled nodes.
type PrSet struct {
	PresLayoutVars *VarLst    `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram presLayoutVars,omitempty"`
	Style          *dml.Style `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram style,omitempty"`

	PresAssocID   string `xml:"presAssocID,attr,omitempty"`
	PresName      string `xml:"presName,attr,omitempty"`
	PresStyleLbl  string `xml:"presStyleLbl,attr,omitempty"`
	PresStyleIdx  int32  `xml:"presStyleIdx,attr,omitempty"`
	PresStyleCnt  int32  `xml:"presStyleCnt,attr,omitempty"`
	LoTypeId      string `xml:"loTypeId,attr,omitempty"`
	LoCatId       string `xml:"loCatId,attr,omitempty"`
	QsTypeId      string `xml:"qsTypeId,attr,omitempty"`
	QsCatId       string `xml:"qsCatId,attr,omitempty"`
	CsTypeId      string `xml:"csTypeId,attr,omitempty"`
	CsCatId       string `xml:"csCatId,attr,omitempty"`
	Coherent3DOff bool   `xml:"coherent3DOff,attr,omitempty"`
	PhldrT        string `xml:"phldrT,attr,omitempty"`
	Phldr         bool   `xml:"phldr,attr,omitempty"`
	CustAng       int32  `xml:"custAng,attr,omitempty"`
	CustFlipVert  bool   `xml:"custFlipVert,attr,omitempty"`
	CustFlipHor   bool   `xml:"custFlipHor,attr,omitempty"`
	CustSzX       int32  `xml:"custSzX,attr,omitempty"`
	CustSzY       int32  `xml:"custSzY,attr,omitempty"`
	CustScaleX    int32  `xml:"custScaleX,attr,omitempty"`
	CustScaleY    int32  `xml:"custScaleY,attr,omitempty"`
	CustT         bool   `xml:"custT,attr,omitempty"`
	CustLinFactX  int32  `xml:"custLinFactX,attr,omitempty"`
	CustLinFactY  int32  `xml:"custLinFactY,attr,omitempty"`
	CustLinFactNeighborX int32 `xml:"custLinFactNeighborX,attr,omitempty"`
	CustLinFactNeighborY int32 `xml:"custLinFactNeighborY,attr,omitempty"`
	CustRadScaleRad int32 `xml:"custRadScaleRad,attr,omitempty"`
	CustRadScaleInc int32 `xml:"custRadScaleInc,attr,omitempty"`
}

// CxnLst represents CT_CxnList (dgm:cxnLst) - list of connections
type CxnLst struct {
	Cxn []*Cxn `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram cxn,omitempty"`
}

// Cxn represents CT_Cxn (dgm:cxn) - a connection between points
type Cxn struct {
	ModelId   string      `xml:"modelId,attr"`
	Type      string      `xml:"type,attr,omitempty"` // ST_CxnType: parOf, presOf, presParOf, unknownRelationship
	SrcId     string      `xml:"srcId,attr"`
	DestId    string      `xml:"destId,attr"`
	SrcOrd    uint32      `xml:"srcOrd,attr"`
	DestOrd   uint32      `xml:"destOrd,attr"`
	ParTransId string     `xml:"parTransId,attr,omitempty"`
	SibTransId string     `xml:"sibTransId,attr,omitempty"`
	PresId    string      `xml:"presId,attr,omitempty"`
	ExtLst    *dml.ExtLst `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram extLst,omitempty"`
}

// SampleData represents CT_SampleData (dgm:sampData) - sample data for layout preview
type SampleData struct {
	UseDef    bool       `xml:"useDef,attr,omitempty"`
	DataModel *DataModel `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram dataModel,omitempty"`
}

// StyleData represents CT_SampleData (dgm:styleData) - style data for preview
type StyleData struct {
	UseDef    bool       `xml:"useDef,attr,omitempty"`
	DataModel *DataModel `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram dataModel,omitempty"`
}

// CategoryData represents CT_SampleData (dgm:clrData) - color data for preview
type CategoryData struct {
	UseDef    bool       `xml:"useDef,attr,omitempty"`
	DataModel *DataModel `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram dataModel,omitempty"`
}

// RelIds represents CT_RelIds (dgm:relIds) - relationship IDs for diagram parts
type RelIds struct {
	Dm  string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships dm,attr"`
	Lo  string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships lo,attr"`
	Qs  string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships qs,attr"`
	Cs  string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships cs,attr"`
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
// Adds explicit xmlns:dgm and xmlns:r declarations as required by OOXML for graphicData content.
func (r *RelIds) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	b.EmptyElement(ns, localName,
		xmlb.Attr{Name: "xmlns:dgm", Value: xmlb.NSDrawingMLDiagram},
		xmlb.Attr{Name: "xmlns:r", Value: xmlb.NSOfficeDocumentRels},
		xmlb.Attr{Namespace: xmlb.NSOfficeDocumentRels, Name: "dm", Value: r.Dm},
		xmlb.Attr{Namespace: xmlb.NSOfficeDocumentRels, Name: "lo", Value: r.Lo},
		xmlb.Attr{Namespace: xmlb.NSOfficeDocumentRels, Name: "qs", Value: r.Qs},
		xmlb.Attr{Namespace: xmlb.NSOfficeDocumentRels, Name: "cs", Value: r.Cs},
	)
}
