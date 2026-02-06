package oxml

import (
	"encoding/xml"

	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/common/dml/chart"
	"github.com/mgilbir/spine/common/dml/diagram"
)

// GraphicFrame represents a graphic frame element (p:graphicFrame) that contains tables.
type GraphicFrame struct {
	XMLName          xml.Name          `xml:"http://schemas.openxmlformats.org/presentationml/2006/main graphicFrame"`
	NvGraphicFramePr *NvGraphicFramePr `xml:"nvGraphicFramePr"`
	Xfrm             *dml.Xfrm        `xml:"xfrm"`
	Graphic          *AGraphic         `xml:"http://schemas.openxmlformats.org/drawingml/2006/main graphic"`
	ExtLst           *ExtensionList    `xml:"extLst,omitempty"`
}

// NvGraphicFramePr contains non-visual graphic frame properties.
type NvGraphicFramePr struct {
	CNvPr             *dml.CNvPr         `xml:"cNvPr"`
	CNvGraphicFramePr *CNvGraphicFramePr `xml:"cNvGraphicFramePr"`
	NvPr              *NvPr              `xml:"nvPr"`
}

// CNvGraphicFramePr contains non-visual graphic frame drawing properties.
type CNvGraphicFramePr struct {
	GraphicFrameLocks *GraphicFrameLocks `xml:"http://schemas.openxmlformats.org/drawingml/2006/main graphicFrameLocks,omitempty"`
}

// GraphicFrameLocks contains graphic frame locking properties.
type GraphicFrameLocks struct {
	NoGrp bool `xml:"noGrp,attr,omitempty"`
}

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
// Exactly one of Table, DiagramRelIds, or ChartRef will be populated based on URI.
type AGraphicData struct {
	URI           string           `xml:"uri,attr"`
	Table         *ATable          `xml:"http://schemas.openxmlformats.org/drawingml/2006/main tbl,omitempty"`
	DiagramRelIds *diagram.RelIds  `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram relIds,omitempty"`
	ChartRef      *chart.RelId     `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart chart,omitempty"`
}

// TableGraphicDataURI is the URI for table graphic data.
const TableGraphicDataURI = "http://schemas.openxmlformats.org/drawingml/2006/table"

// ATable represents a DrawingML table.
type ATable struct {
	TblPr   *ATblPr  `xml:"http://schemas.openxmlformats.org/drawingml/2006/main tblPr,omitempty"`
	TblGrid *ATblGrid `xml:"http://schemas.openxmlformats.org/drawingml/2006/main tblGrid"`
	Tr      []*ATr   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main tr"`
}

// ATblPr contains table properties.
type ATblPr struct {
	FirstRow     bool   `xml:"firstRow,attr,omitempty"`
	FirstCol     bool   `xml:"firstCol,attr,omitempty"`
	LastRow      bool   `xml:"lastRow,attr,omitempty"`
	LastCol      bool   `xml:"lastCol,attr,omitempty"`
	BandRow      bool   `xml:"bandRow,attr,omitempty"`
	BandCol      bool   `xml:"bandCol,attr,omitempty"`
	TableStyleId string `xml:"http://schemas.openxmlformats.org/drawingml/2006/main tableStyleId,omitempty"`
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
	H      int64       `xml:"h,attr,omitempty"`
	Tc     []*ATc      `xml:"http://schemas.openxmlformats.org/drawingml/2006/main tc"`
	ExtLst *dml.ExtLst `xml:"http://schemas.openxmlformats.org/drawingml/2006/main extLst,omitempty"`
}

// ATc represents a table cell (CT_TableCell).
// Field order matches XSD sequence: txBody, tcPr, extLst.
type ATc struct {
	TxBody   *dml.TxBody `xml:"http://schemas.openxmlformats.org/drawingml/2006/main txBody,omitempty"`
	TcPr     *ATcPr      `xml:"http://schemas.openxmlformats.org/drawingml/2006/main tcPr,omitempty"`
	RowSpan  int          `xml:"rowSpan,attr,omitempty"`
	GridSpan int          `xml:"gridSpan,attr,omitempty"`
	HMerge   bool         `xml:"hMerge,attr,omitempty"`
	VMerge   bool         `xml:"vMerge,attr,omitempty"`
}

// ATcPr contains table cell properties.
type ATcPr struct {
	MarL      *int64          `xml:"marL,attr,omitempty"`
	MarR      *int64          `xml:"marR,attr,omitempty"`
	MarT      *int64          `xml:"marT,attr,omitempty"`
	MarB      *int64          `xml:"marB,attr,omitempty"`
	Anchor    string          `xml:"anchor,attr,omitempty"`
	LnL       *dml.Ln         `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lnL,omitempty"`
	LnR       *dml.Ln         `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lnR,omitempty"`
	LnT       *dml.Ln         `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lnT,omitempty"`
	LnB       *dml.Ln         `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lnB,omitempty"`
	SolidFill *dml.SolidFill  `xml:"http://schemas.openxmlformats.org/drawingml/2006/main solidFill,omitempty"`
	NoFill    *dml.NoFillXML  `xml:"http://schemas.openxmlformats.org/drawingml/2006/main noFill,omitempty"`
}
