package chart

import (
	"github.com/mgilbir/spine/common/dml"
	xmlb "github.com/mgilbir/spine/common/xml"
)

// --- Simple Value Types ---

// Boolean represents CT_Boolean
type Boolean struct {
	Val bool `xml:"val,attr"`
}

// UnsignedInt represents CT_UnsignedInt
type UnsignedInt struct {
	Val uint32 `xml:"val,attr"`
}

// Double represents CT_Double
type Double struct {
	Val float64 `xml:"val,attr"`
}

// String represents CT_String
type String struct {
	Val string `xml:"val,attr"`
}

// Style represents CT_Style - chart style
type Style struct {
	Val uint32 `xml:"val,attr"` // 1-48
}

// RelId represents CT_RelId - relationship reference
type RelId struct {
	Id string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
// Adds explicit xmlns:c and xmlns:r declarations as required by OOXML for graphicData content.
func (r *RelId) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	b.EmptyElement(ns, localName,
		xmlb.Attr{Name: "xmlns:c", Value: xmlb.NSDrawingMLChart},
		xmlb.Attr{Name: "xmlns:r", Value: xmlb.NSOfficeDocumentRels},
		xmlb.Attr{Namespace: xmlb.NSOfficeDocumentRels, Name: "id", Value: r.Id},
	)
}

// --- Enum-like Types ---

// BarDir represents CT_BarDir - bar chart direction
type BarDir struct {
	Val string `xml:"val,attr"` // bar, col
}

// BarGrouping represents CT_BarGrouping
type BarGrouping struct {
	Val string `xml:"val,attr"` // percentStacked, clustered, standard, stacked
}

// BarShape represents CT_Shape - bar shape
type BarShape struct {
	Val string `xml:"val,attr"` // cone, coneToMax, box, cylinder, pyramid, pyramidToMax
}

// Grouping represents CT_Grouping - chart grouping
type Grouping struct {
	Val string `xml:"val,attr"` // percentStacked, standard, stacked
}

// ScatterStyle represents CT_ScatterStyle
type ScatterStyle struct {
	Val string `xml:"val,attr"` // none, line, lineMarker, marker, smooth, smoothMarker
}

// RadarStyle represents CT_RadarStyle
type RadarStyle struct {
	Val string `xml:"val,attr"` // standard, marker, filled
}

// OfPieType represents CT_OfPieType
type OfPieType struct {
	Val string `xml:"val,attr"` // pie, bar
}

// SplitType represents CT_SplitType
type SplitType struct {
	Val string `xml:"val,attr"` // auto, cust, percent, pos, val
}

// AxPos represents CT_AxPos - axis position
type AxPos struct {
	Val string `xml:"val,attr"` // b, l, r, t
}

// Crosses represents CT_Crosses - where axes cross
type Crosses struct {
	Val string `xml:"val,attr"` // autoZero, max, min
}

// CrossBetween represents CT_CrossBetween
type CrossBetween struct {
	Val string `xml:"val,attr"` // between, midCat
}

// TickMark represents CT_TickMark
type TickMark struct {
	Val string `xml:"val,attr"` // cross, in, none, out
}

// TickLblPos represents CT_TickLblPos
type TickLblPos struct {
	Val string `xml:"val,attr"` // high, low, nextTo, none
}

// Orientation represents CT_Orientation - axis orientation
type Orientation struct {
	Val string `xml:"val,attr"` // maxMin, minMax
}

// TimeUnit represents CT_TimeUnit
type TimeUnit struct {
	Val string `xml:"val,attr"` // days, months, years
}

// LblAlgn represents CT_LblAlgn - label alignment
type LblAlgn struct {
	Val string `xml:"val,attr"` // ctr, l, r
}

// LblOffset represents CT_LblOffset
type LblOffset struct {
	Val uint32 `xml:"val,attr"` // 0-1000
}

// BuiltInUnit represents CT_BuiltInUnit
type BuiltInUnit struct {
	Val string `xml:"val,attr"` // hundreds, thousands, tenThousands, hundredThousands, millions, tenMillions, hundredMillions, billions, trillions
}

// DispBlanksAs represents CT_DispBlanksAs
type DispBlanksAs struct {
	Val string `xml:"val,attr"` // span, gap, zero
}

// MarkerStyle represents CT_MarkerStyle
type MarkerStyle struct {
	Val string `xml:"val,attr"` // circle, dash, diamond, dot, none, picture, plus, square, star, triangle, x, auto
}

// MarkerSize represents CT_MarkerSize
type MarkerSize struct {
	Val uint32 `xml:"val,attr"` // 2-72
}

// GapAmount represents CT_GapAmount
type GapAmount struct {
	Val uint32 `xml:"val,attr"` // 0-500
}

// Overlap represents CT_Overlap
type Overlap struct {
	Val int32 `xml:"val,attr"` // -100 to 100
}

// HoleSize represents CT_HoleSize (doughnut hole)
type HoleSize struct {
	Val uint32 `xml:"val,attr"` // 1-90
}

// BubbleScale represents CT_BubbleScale
type BubbleScale struct {
	Val uint32 `xml:"val,attr"` // 0-300
}

// SizeRepresents represents CT_SizeRepresents
type SizeRepresents struct {
	Val string `xml:"val,attr"` // area, w
}

// SecondPieSize represents CT_SecondPieSize
type SecondPieSize struct {
	Val uint32 `xml:"val,attr"` // 5-200
}

// Skip represents CT_Skip
type Skip struct {
	Val uint32 `xml:"val,attr"`
}

// LogBase represents CT_LogBase
type LogBase struct {
	Val float64 `xml:"val,attr"` // 2-1000
}

// --- Data Labels ---

// DataLabels represents CT_DLbls (c:dLbls) - data labels for a series
type DataLabels struct {
	DLbl         []*DataLabel `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart dLbl,omitempty"`
	Delete       *Boolean     `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart delete,omitempty"`
	NumFmt       *NumFmt      `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart numFmt,omitempty"`
	SpPr         *dml.SpPr    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart spPr,omitempty"`
	TxPr         *dml.TxBody  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart txPr,omitempty"`
	DLblPos      *DLblPos     `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart dLblPos,omitempty"`
	ShowLegendKey *Boolean    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart showLegendKey,omitempty"`
	ShowVal      *Boolean     `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart showVal,omitempty"`
	ShowCatName  *Boolean     `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart showCatName,omitempty"`
	ShowSerName  *Boolean     `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart showSerName,omitempty"`
	ShowPercent  *Boolean     `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart showPercent,omitempty"`
	ShowBubbleSize *Boolean   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart showBubbleSize,omitempty"`
	Separator    string       `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart separator,omitempty"`
	ShowLeaderLines *Boolean  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart showLeaderLines,omitempty"`
	LeaderLines  *ChartLines  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart leaderLines,omitempty"`
	ExtLst       *ExtLst  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart extLst,omitempty"`
}

// DataLabel represents CT_DLbl (c:dLbl) - individual data label
type DataLabel struct {
	Idx          *UnsignedInt `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart idx,omitempty"`
	Layout       *Layout      `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart layout,omitempty"`
	Tx           *ChartText   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart tx,omitempty"`
	NumFmt       *NumFmt      `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart numFmt,omitempty"`
	SpPr         *dml.SpPr    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart spPr,omitempty"`
	TxPr         *dml.TxBody  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart txPr,omitempty"`
	DLblPos      *DLblPos     `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart dLblPos,omitempty"`
	ShowLegendKey *Boolean    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart showLegendKey,omitempty"`
	ShowVal      *Boolean     `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart showVal,omitempty"`
	ShowCatName  *Boolean     `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart showCatName,omitempty"`
	ShowSerName  *Boolean     `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart showSerName,omitempty"`
	ShowPercent  *Boolean     `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart showPercent,omitempty"`
	ShowBubbleSize *Boolean   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart showBubbleSize,omitempty"`
	Separator    string       `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart separator,omitempty"`
	ExtLst       *ExtLst  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart extLst,omitempty"`
}

// DLblPos represents CT_DLblPos
type DLblPos struct {
	Val string `xml:"val,attr"` // bestFit, b, ctr, inBase, inEnd, l, outEnd, r, t
}

// NumFmt represents CT_NumFmt (c:numFmt). sourceLinked defaults to true, so it
// is a pointer: an explicit false must be emitted rather than omitted (which
// readers treat as true).
type NumFmt struct {
	FormatCode   string `xml:"formatCode,attr"`
	SourceLinked *bool  `xml:"sourceLinked,attr,omitempty"`
}

// --- Legend ---

// Legend represents CT_Legend (c:legend)
type Legend struct {
	LegendPos   *LegendPos    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart legendPos,omitempty"`
	LegendEntry []*LegendEntry `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart legendEntry,omitempty"`
	Layout      *Layout       `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart layout,omitempty"`
	Overlay     *Boolean      `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart overlay,omitempty"`
	SpPr        *dml.SpPr     `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart spPr,omitempty"`
	TxPr        *dml.TxBody   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart txPr,omitempty"`
	ExtLst      *ExtLst   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart extLst,omitempty"`
}

// LegendPos represents CT_LegendPos
type LegendPos struct {
	Val string `xml:"val,attr"` // b, l, r, t, tr
}

// LegendEntry represents CT_LegendEntry
type LegendEntry struct {
	Idx    *UnsignedInt `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart idx,omitempty"`
	Delete *Boolean     `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart delete,omitempty"`
	TxPr   *dml.TxBody  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart txPr,omitempty"`
	ExtLst *ExtLst  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart extLst,omitempty"`
}

// --- Layout ---

// Layout represents CT_Layout (c:layout)
type Layout struct {
	ManualLayout *ManualLayout `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart manualLayout,omitempty"`
	ExtLst       *ExtLst   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart extLst,omitempty"`
}

// ManualLayout represents CT_ManualLayout
type ManualLayout struct {
	LayoutTarget *LayoutTarget `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart layoutTarget,omitempty"`
	XMode        *LayoutMode   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart xMode,omitempty"`
	YMode        *LayoutMode   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart yMode,omitempty"`
	WMode        *LayoutMode   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart wMode,omitempty"`
	HMode        *LayoutMode   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart hMode,omitempty"`
	X            *Double       `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart x,omitempty"`
	Y            *Double       `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart y,omitempty"`
	W            *Double       `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart w,omitempty"`
	H            *Double       `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart h,omitempty"`
	ExtLst       *ExtLst   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart extLst,omitempty"`
}

// LayoutTarget represents CT_LayoutTarget
type LayoutTarget struct {
	Val string `xml:"val,attr"` // inner, outer
}

// LayoutMode represents CT_LayoutMode
type LayoutMode struct {
	Val string `xml:"val,attr"` // edge, factor
}

// --- Miscellaneous ---

// View3D represents CT_View3D (c:view3D)
type View3D struct {
	RotX       *RotX       `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart rotX,omitempty"`
	HPercent   *HPercent   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart hPercent,omitempty"`
	RotY       *RotY       `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart rotY,omitempty"`
	DepthPercent *DepthPercent `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart depthPercent,omitempty"`
	RAngAx     *Boolean    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart rAngAx,omitempty"`
	Perspective *Perspective `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart perspective,omitempty"`
	ExtLst     *ExtLst `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart extLst,omitempty"`
}

type RotX struct{ Val int32 `xml:"val,attr"` }           // -90 to 90
type RotY struct{ Val uint32 `xml:"val,attr"` }          // 0 to 360
type HPercent struct{ Val uint32 `xml:"val,attr"` }      // 5-500
type DepthPercent struct{ Val uint32 `xml:"val,attr"` }  // 20-2000
type Perspective struct{ Val uint32 `xml:"val,attr"` }   // 0-240

// Surface represents CT_Surface (c:floor, c:sideWall, c:backWall)
type Surface struct {
	Thickness *UnsignedInt `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart thickness,omitempty"`
	SpPr      *dml.SpPr    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart spPr,omitempty"`
	PictureOptions *PictureOptions `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart pictureOptions,omitempty"`
	ExtLst    *ExtLst  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart extLst,omitempty"`
}

// PictureOptions represents CT_PictureOptions
type PictureOptions struct {
	ApplyToFront *Boolean    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart applyToFront,omitempty"`
	ApplyToSides *Boolean    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart applyToSides,omitempty"`
	ApplyToEnd   *Boolean    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart applyToEnd,omitempty"`
	PictureFormat *PictureFormat `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart pictureFormat,omitempty"`
	PictureStackUnit *Double `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart pictureStackUnit,omitempty"`
}

// PictureFormat represents CT_PictureFormat
type PictureFormat struct {
	Val string `xml:"val,attr"` // stretch, stack, stackScale
}

// ChartLines represents CT_ChartLines (c:dropLines, etc.)
type ChartLines struct {
	SpPr *dml.SpPr `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart spPr,omitempty"`
}

// UpDownBars represents CT_UpDownBars
type UpDownBars struct {
	GapWidth *GapAmount  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart gapWidth,omitempty"`
	UpBars   *UpDownBar  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart upBars,omitempty"`
	DownBars *UpDownBar  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart downBars,omitempty"`
	ExtLst   *ExtLst `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart extLst,omitempty"`
}

// UpDownBar represents CT_UpDownBar
type UpDownBar struct {
	SpPr *dml.SpPr `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart spPr,omitempty"`
}

// Trendline represents CT_Trendline (c:trendline)
type Trendline struct {
	Name          string        `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart name,omitempty"`
	SpPr          *dml.SpPr     `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart spPr,omitempty"`
	TrendlineType *TrendlineType `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart trendlineType,omitempty"`
	Order         *UnsignedInt  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart order,omitempty"`
	Period        *UnsignedInt  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart period,omitempty"`
	Forward       *Double       `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart forward,omitempty"`
	Backward      *Double       `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart backward,omitempty"`
	Intercept     *Double       `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart intercept,omitempty"`
	DispRSqr      *Boolean      `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart dispRSqr,omitempty"`
	DispEq        *Boolean      `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart dispEq,omitempty"`
	TrendlineLbl  *TrendlineLbl `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart trendlineLbl,omitempty"`
	ExtLst        *ExtLst   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart extLst,omitempty"`
}

// TrendlineType represents CT_TrendlineType
type TrendlineType struct {
	Val string `xml:"val,attr"` // exp, linear, log, movingAvg, poly, power
}

// TrendlineLbl represents CT_TrendlineLbl
type TrendlineLbl struct {
	Layout *Layout     `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart layout,omitempty"`
	Tx     *ChartText  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart tx,omitempty"`
	NumFmt *NumFmt     `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart numFmt,omitempty"`
	SpPr   *dml.SpPr   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart spPr,omitempty"`
	TxPr   *dml.TxBody `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart txPr,omitempty"`
	ExtLst *ExtLst `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart extLst,omitempty"`
}

// ErrBars represents CT_ErrBars (c:errBars) - error bars
type ErrBars struct {
	ErrDir    *ErrDir      `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart errDir,omitempty"`
	ErrBarType *ErrBarType `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart errBarType,omitempty"`
	ErrValType *ErrValType `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart errValType,omitempty"`
	NoEndCap  *Boolean     `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart noEndCap,omitempty"`
	Plus      *NumDataSource `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart plus,omitempty"`
	Minus     *NumDataSource `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart minus,omitempty"`
	Val       *Double      `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart val,omitempty"`
	SpPr      *dml.SpPr    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart spPr,omitempty"`
	ExtLst    *ExtLst  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart extLst,omitempty"`
}

type ErrDir struct{ Val string `xml:"val,attr"` }      // x, y
type ErrBarType struct{ Val string `xml:"val,attr"` }   // both, minus, plus
type ErrValType struct{ Val string `xml:"val,attr"` }   // cust, fixedVal, percentage, stdDev, stdErr

// DataTable represents CT_DTable (c:dTable)
type DataTable struct {
	ShowHorzBorder *Boolean    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart showHorzBorder,omitempty"`
	ShowVertBorder *Boolean    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart showVertBorder,omitempty"`
	ShowOutline    *Boolean    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart showOutline,omitempty"`
	ShowKeys       *Boolean    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart showKeys,omitempty"`
	SpPr           *dml.SpPr   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart spPr,omitempty"`
	TxPr           *dml.TxBody `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart txPr,omitempty"`
	ExtLst         *ExtLst `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart extLst,omitempty"`
}

// PivotSource represents CT_PivotSource
type PivotSource struct {
	Name   string       `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart name,omitempty"`
	FmtId  *UnsignedInt `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart fmtId,omitempty"`
	ExtLst *ExtLst  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart extLst,omitempty"`
}

// PivotFormats represents CT_PivotFmts
type PivotFormats struct {
	PivotFmt []*PivotFormat `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart pivotFmt,omitempty"`
}

// PivotFormat represents CT_PivotFmt
type PivotFormat struct {
	Idx    *UnsignedInt `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart idx,omitempty"`
	SpPr   *dml.SpPr    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart spPr,omitempty"`
	TxPr   *dml.TxBody  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart txPr,omitempty"`
	Marker *Marker      `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart marker,omitempty"`
	DLbl   *DataLabel   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart dLbl,omitempty"`
	ExtLst *ExtLst  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart extLst,omitempty"`
}

// Protection represents CT_Protection
type Protection struct {
	ChartObject      *Boolean `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart chartObject,omitempty"`
	Data             *Boolean `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart data,omitempty"`
	Formatting       *Boolean `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart formatting,omitempty"`
	Selection        *Boolean `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart selection,omitempty"`
	UserInterface    *Boolean `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart userInterface,omitempty"`
}

// ExternalData represents CT_ExternalData
type ExternalData struct {
	Id         string   `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
	AutoUpdate *Boolean `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart autoUpdate,omitempty"`
}

// PrintSettings represents CT_PrintSettings
type PrintSettings struct {
	HeaderFooter *HeaderFooterChart `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart headerFooter,omitempty"`
	PageMargins  *PageMargins      `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart pageMargins,omitempty"`
	PageSetup    *PageSetup        `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart pageSetup,omitempty"`
}

// HeaderFooterChart represents CT_HeaderFooter (chart-specific)
type HeaderFooterChart struct {
	AlignWithMargins bool   `xml:"alignWithMargins,attr,omitempty"`
	DifferentOddEven bool  `xml:"differentOddEven,attr,omitempty"`
	DifferentFirst   bool  `xml:"differentFirst,attr,omitempty"`
	OddHeader        string `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart oddHeader,omitempty"`
	OddFooter        string `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart oddFooter,omitempty"`
	EvenHeader       string `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart evenHeader,omitempty"`
	EvenFooter       string `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart evenFooter,omitempty"`
	FirstHeader      string `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart firstHeader,omitempty"`
	FirstFooter      string `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart firstFooter,omitempty"`
}

// PageMargins represents CT_PageMargins
type PageMargins struct {
	L      float64 `xml:"l,attr"`
	R      float64 `xml:"r,attr"`
	T      float64 `xml:"t,attr"`
	B      float64 `xml:"b,attr"`
	Header float64 `xml:"header,attr"`
	Footer float64 `xml:"footer,attr"`
}

// PageSetup represents CT_PageSetup
type PageSetup struct {
	PaperSize          uint32 `xml:"paperSize,attr,omitempty"`
	PaperHeight        string `xml:"paperHeight,attr,omitempty"`
	PaperWidth         string `xml:"paperWidth,attr,omitempty"`
	FirstPageNumber    uint32 `xml:"firstPageNumber,attr,omitempty"`
	Orientation        string `xml:"orientation,attr,omitempty"` // default, portrait, landscape
	BlackAndWhite      bool   `xml:"blackAndWhite,attr,omitempty"`
	Draft              bool   `xml:"draft,attr,omitempty"`
	UseFirstPageNumber bool   `xml:"useFirstPageNumber,attr,omitempty"`
	HorizontalDpi      int32  `xml:"horizontalDpi,attr,omitempty"`
	VerticalDpi        int32  `xml:"verticalDpi,attr,omitempty"`
	Copies             uint32 `xml:"copies,attr,omitempty"`
}

// BandFormats represents CT_BandFmts
type BandFormats struct {
	BandFmt []*BandFormat `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart bandFmt,omitempty"`
}

// BandFormat represents CT_BandFmt
type BandFormat struct {
	Idx  *UnsignedInt `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart idx,omitempty"`
	SpPr *dml.SpPr    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart spPr,omitempty"`
}

// CustSplit represents CT_CustSplit (custom split for ofPieChart)
type CustSplit struct {
	SecondPiePt []*UnsignedInt `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart secondPiePt,omitempty"`
}
