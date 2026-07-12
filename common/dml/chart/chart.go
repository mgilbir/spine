// Package chart provides DrawingML chart types from dml-chart.xsd.
// These types implement the c: namespace elements for chart definitions.
package chart

import "github.com/mgilbir/spine/common/dml"

const NsChart = "http://schemas.openxmlformats.org/drawingml/2006/chart"

// --- Root Element ---

// ChartSpace represents CT_ChartSpace (c:chartSpace) - the root element of a chart part
type ChartSpace struct {
	Date1904     *Boolean          `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart date1904,omitempty"`
	Lang         *String           `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart lang,omitempty"`
	RoundedCorners *Boolean        `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart roundedCorners,omitempty"`
	Style        *Style            `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart style,omitempty"`
	ClrMapOvr    *dml.ClrMapOvr    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart clrMapOvr,omitempty"`
	PivotSource  *PivotSource      `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart pivotSource,omitempty"`
	Protection   *Protection       `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart protection,omitempty"`
	Chart        *Chart            `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart chart,omitempty"`
	SpPr         *dml.SpPr         `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart spPr,omitempty"`
	TxPr         *dml.TxBody       `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart txPr,omitempty"`
	ExternalData *ExternalData     `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart externalData,omitempty"`
	PrintSettings *PrintSettings   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart printSettings,omitempty"`
	UserShapes   *RelId            `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart userShapes,omitempty"`
	ExtLst       *ExtLst       `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart extLst,omitempty"`
}

// Chart represents CT_Chart (c:chart)
type Chart struct {
	Title        *Title          `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart title,omitempty"`
	AutoTitleDeleted *Boolean    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart autoTitleDeleted,omitempty"`
	PivotFmts    *PivotFormats   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart pivotFmts,omitempty"`
	View3D       *View3D         `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart view3D,omitempty"`
	Floor        *Surface        `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart floor,omitempty"`
	SideWall     *Surface        `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart sideWall,omitempty"`
	BackWall     *Surface        `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart backWall,omitempty"`
	PlotArea     *PlotArea       `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart plotArea,omitempty"`
	Legend       *Legend         `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart legend,omitempty"`
	PlotVisOnly  *Boolean        `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart plotVisOnly,omitempty"`
	DispBlanksAs *DispBlanksAs   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart dispBlanksAs,omitempty"`
	ShowDLblsOverMax *Boolean    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart showDLblsOverMax,omitempty"`
	ExtLst       *ExtLst     `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart extLst,omitempty"`
}

// --- Title ---

// Title represents CT_Title (c:title)
type Title struct {
	Tx      *ChartText    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart tx,omitempty"`
	Layout  *Layout       `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart layout,omitempty"`
	Overlay *Boolean      `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart overlay,omitempty"`
	SpPr    *dml.SpPr     `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart spPr,omitempty"`
	TxPr    *dml.TxBody   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart txPr,omitempty"`
	ExtLst  *ExtLst   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart extLst,omitempty"`
}

// ChartText represents CT_Tx (c:tx) - chart text source
type ChartText struct {
	Rich   *dml.TxBody   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart rich,omitempty"`
	StrRef *StrRef       `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart strRef,omitempty"`
}

// --- Plot Area ---

// PlotArea represents CT_PlotArea (c:plotArea)
type PlotArea struct {
	Layout       *Layout         `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart layout,omitempty"`
	// Chart type groups
	AreaChart    []*AreaChart    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart areaChart,omitempty"`
	Area3DChart  []*Area3DChart  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart area3DChart,omitempty"`
	BarChart     []*BarChart     `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart barChart,omitempty"`
	Bar3DChart   []*Bar3DChart   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart bar3DChart,omitempty"`
	BubbleChart  []*BubbleChart  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart bubbleChart,omitempty"`
	DoughnutChart []*DoughnutChart `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart doughnutChart,omitempty"`
	LineChart    []*LineChart    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart lineChart,omitempty"`
	Line3DChart  []*Line3DChart  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart line3DChart,omitempty"`
	OfPieChart   []*OfPieChart   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart ofPieChart,omitempty"`
	PieChart     []*PieChart     `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart pieChart,omitempty"`
	Pie3DChart   []*Pie3DChart   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart pie3DChart,omitempty"`
	RadarChart   []*RadarChart   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart radarChart,omitempty"`
	ScatterChart []*ScatterChart `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart scatterChart,omitempty"`
	StockChart   []*StockChart   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart stockChart,omitempty"`
	SurfaceChart []*SurfaceChart `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart surfaceChart,omitempty"`
	Surface3DChart []*Surface3DChart `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart surface3DChart,omitempty"`
	// Axes
	ValAx    []*ValAx     `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart valAx,omitempty"`
	CatAx    []*CatAx     `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart catAx,omitempty"`
	DateAx   []*DateAx    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart dateAx,omitempty"`
	SerAx    []*SerAx     `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart serAx,omitempty"`
	DTable   *DataTable   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart dTable,omitempty"`
	SpPr     *dml.SpPr    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart spPr,omitempty"`
	ExtLst   *ExtLst  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart extLst,omitempty"`
}

// --- Chart Types ---

// BarChart represents CT_BarChart (c:barChart)
type BarChart struct {
	BarDir    *BarDir      `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart barDir,omitempty"`
	Grouping  *BarGrouping `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart grouping,omitempty"`
	VaryColors *Boolean    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart varyColors,omitempty"`
	Ser       []*BarSer    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart ser,omitempty"`
	DLbls     *DataLabels  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart dLbls,omitempty"`
	GapWidth  *GapAmount   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart gapWidth,omitempty"`
	Overlap   *Overlap     `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart overlap,omitempty"`
	SerLines  []*ChartLines `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart serLines,omitempty"`
	AxId      []*UnsignedInt `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart axId,omitempty"`
	ExtLst    *ExtLst  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart extLst,omitempty"`
}

// Bar3DChart represents CT_Bar3DChart (c:bar3DChart)
type Bar3DChart struct {
	BarDir    *BarDir      `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart barDir,omitempty"`
	Grouping  *BarGrouping `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart grouping,omitempty"`
	VaryColors *Boolean    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart varyColors,omitempty"`
	Ser       []*BarSer    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart ser,omitempty"`
	DLbls     *DataLabels  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart dLbls,omitempty"`
	GapWidth  *GapAmount   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart gapWidth,omitempty"`
	GapDepth  *GapAmount   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart gapDepth,omitempty"`
	Shape     *BarShape    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart shape,omitempty"`
	AxId      []*UnsignedInt `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart axId,omitempty"`
	ExtLst    *ExtLst  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart extLst,omitempty"`
}

// LineChart represents CT_LineChart (c:lineChart)
type LineChart struct {
	Grouping   *Grouping    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart grouping,omitempty"`
	VaryColors *Boolean     `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart varyColors,omitempty"`
	Ser        []*LineSer   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart ser,omitempty"`
	DLbls      *DataLabels  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart dLbls,omitempty"`
	DropLines  *ChartLines  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart dropLines,omitempty"`
	HiLowLines *ChartLines  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart hiLowLines,omitempty"`
	UpDownBars *UpDownBars  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart upDownBars,omitempty"`
	Marker     *Boolean     `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart marker,omitempty"`
	Smooth     *Boolean     `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart smooth,omitempty"`
	AxId       []*UnsignedInt `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart axId,omitempty"`
	ExtLst     *ExtLst  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart extLst,omitempty"`
}

// Line3DChart represents CT_Line3DChart (c:line3DChart)
type Line3DChart struct {
	Grouping   *Grouping    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart grouping,omitempty"`
	VaryColors *Boolean     `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart varyColors,omitempty"`
	Ser        []*LineSer   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart ser,omitempty"`
	DLbls      *DataLabels  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart dLbls,omitempty"`
	DropLines  *ChartLines  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart dropLines,omitempty"`
	GapDepth   *GapAmount   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart gapDepth,omitempty"`
	AxId       []*UnsignedInt `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart axId,omitempty"`
	ExtLst     *ExtLst  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart extLst,omitempty"`
}

// PieChart represents CT_PieChart (c:pieChart)
type PieChart struct {
	VaryColors *Boolean     `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart varyColors,omitempty"`
	Ser        []*PieSer    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart ser,omitempty"`
	DLbls      *DataLabels  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart dLbls,omitempty"`
	FirstSliceAng *UnsignedInt `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart firstSliceAng,omitempty"`
	ExtLst     *ExtLst  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart extLst,omitempty"`
}

// Pie3DChart represents CT_Pie3DChart (c:pie3DChart)
type Pie3DChart struct {
	VaryColors *Boolean     `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart varyColors,omitempty"`
	Ser        []*PieSer    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart ser,omitempty"`
	DLbls      *DataLabels  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart dLbls,omitempty"`
	ExtLst     *ExtLst  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart extLst,omitempty"`
}

// DoughnutChart represents CT_DoughnutChart (c:doughnutChart)
type DoughnutChart struct {
	VaryColors   *Boolean     `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart varyColors,omitempty"`
	Ser          []*PieSer    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart ser,omitempty"`
	DLbls        *DataLabels  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart dLbls,omitempty"`
	FirstSliceAng *UnsignedInt `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart firstSliceAng,omitempty"`
	HoleSize     *HoleSize    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart holeSize,omitempty"`
	ExtLst       *ExtLst  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart extLst,omitempty"`
}

// AreaChart represents CT_AreaChart (c:areaChart)
type AreaChart struct {
	Grouping   *Grouping    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart grouping,omitempty"`
	VaryColors *Boolean     `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart varyColors,omitempty"`
	Ser        []*AreaSer   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart ser,omitempty"`
	DLbls      *DataLabels  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart dLbls,omitempty"`
	DropLines  *ChartLines  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart dropLines,omitempty"`
	AxId       []*UnsignedInt `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart axId,omitempty"`
	ExtLst     *ExtLst  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart extLst,omitempty"`
}

// Area3DChart represents CT_Area3DChart (c:area3DChart)
type Area3DChart struct {
	Grouping   *Grouping    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart grouping,omitempty"`
	VaryColors *Boolean     `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart varyColors,omitempty"`
	Ser        []*AreaSer   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart ser,omitempty"`
	DLbls      *DataLabels  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart dLbls,omitempty"`
	DropLines  *ChartLines  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart dropLines,omitempty"`
	GapDepth   *GapAmount   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart gapDepth,omitempty"`
	AxId       []*UnsignedInt `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart axId,omitempty"`
	ExtLst     *ExtLst  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart extLst,omitempty"`
}

// ScatterChart represents CT_ScatterChart (c:scatterChart)
type ScatterChart struct {
	ScatterStyle *ScatterStyle `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart scatterStyle,omitempty"`
	VaryColors   *Boolean      `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart varyColors,omitempty"`
	Ser          []*ScatterSer `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart ser,omitempty"`
	DLbls        *DataLabels   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart dLbls,omitempty"`
	AxId         []*UnsignedInt `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart axId,omitempty"`
	ExtLst       *ExtLst   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart extLst,omitempty"`
}

// BubbleChart represents CT_BubbleChart (c:bubbleChart)
type BubbleChart struct {
	VaryColors    *Boolean      `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart varyColors,omitempty"`
	Ser           []*BubbleSer  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart ser,omitempty"`
	DLbls         *DataLabels   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart dLbls,omitempty"`
	Bubble3D      *Boolean      `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart bubble3D,omitempty"`
	BubbleScale   *BubbleScale  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart bubbleScale,omitempty"`
	ShowNegBubbles *Boolean     `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart showNegBubbles,omitempty"`
	SizeRepresents *SizeRepresents `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart sizeRepresents,omitempty"`
	AxId          []*UnsignedInt `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart axId,omitempty"`
	ExtLst        *ExtLst   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart extLst,omitempty"`
}

// RadarChart represents CT_RadarChart (c:radarChart)
type RadarChart struct {
	RadarStyle *RadarStyle  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart radarStyle,omitempty"`
	VaryColors *Boolean     `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart varyColors,omitempty"`
	Ser        []*RadarSer  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart ser,omitempty"`
	DLbls      *DataLabels  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart dLbls,omitempty"`
	AxId       []*UnsignedInt `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart axId,omitempty"`
	ExtLst     *ExtLst  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart extLst,omitempty"`
}

// StockChart represents CT_StockChart (c:stockChart)
type StockChart struct {
	Ser        []*LineSer   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart ser,omitempty"`
	DLbls      *DataLabels  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart dLbls,omitempty"`
	DropLines  *ChartLines  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart dropLines,omitempty"`
	HiLowLines *ChartLines  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart hiLowLines,omitempty"`
	UpDownBars *UpDownBars  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart upDownBars,omitempty"`
	AxId       []*UnsignedInt `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart axId,omitempty"`
	ExtLst     *ExtLst  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart extLst,omitempty"`
}

// SurfaceChart represents CT_SurfaceChart (c:surfaceChart)
type SurfaceChart struct {
	Wireframe *Boolean       `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart wireframe,omitempty"`
	Ser       []*SurfaceSer  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart ser,omitempty"`
	BandFmts  *BandFormats   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart bandFmts,omitempty"`
	AxId      []*UnsignedInt `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart axId,omitempty"`
	ExtLst    *ExtLst    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart extLst,omitempty"`
}

// Surface3DChart represents CT_Surface3DChart (c:surface3DChart)
type Surface3DChart struct {
	Wireframe *Boolean       `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart wireframe,omitempty"`
	Ser       []*SurfaceSer  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart ser,omitempty"`
	BandFmts  *BandFormats   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart bandFmts,omitempty"`
	AxId      []*UnsignedInt `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart axId,omitempty"`
	ExtLst    *ExtLst    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart extLst,omitempty"`
}

// OfPieChart represents CT_OfPieChart (c:ofPieChart) - pie of pie / bar of pie
type OfPieChart struct {
	OfPieType   *OfPieType   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart ofPieType,omitempty"`
	VaryColors  *Boolean     `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart varyColors,omitempty"`
	Ser         []*PieSer    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart ser,omitempty"`
	DLbls       *DataLabels  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart dLbls,omitempty"`
	GapWidth    *GapAmount   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart gapWidth,omitempty"`
	SplitType   *SplitType   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart splitType,omitempty"`
	SplitPos    *Double      `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart splitPos,omitempty"`
	CustSplit   *CustSplit   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart custSplit,omitempty"`
	SecondPieSize *SecondPieSize `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart secondPieSize,omitempty"`
	SerLines    []*ChartLines `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart serLines,omitempty"`
	ExtLst      *ExtLst  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart extLst,omitempty"`
}
