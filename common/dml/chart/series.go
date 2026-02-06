package chart

import "github.com/mgilbir/spine/common/dml"

// --- Series Types ---

// BarSer represents CT_BarSer (c:ser in barChart)
type BarSer struct {
	Idx           *UnsignedInt `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart idx,omitempty"`
	Order         *UnsignedInt `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart order,omitempty"`
	Tx            *SerTx       `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart tx,omitempty"`
	SpPr          *dml.SpPr    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart spPr,omitempty"`
	InvertIfNegative *Boolean  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart invertIfNegative,omitempty"`
	PictureOptions *PictureOptions `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart pictureOptions,omitempty"`
	DPt           []*DataPoint `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart dPt,omitempty"`
	DLbls         *DataLabels  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart dLbls,omitempty"`
	Trendline     []*Trendline `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart trendline,omitempty"`
	ErrBars       *ErrBars     `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart errBars,omitempty"`
	Cat           *AxDataSource `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart cat,omitempty"`
	Val           *NumDataSource `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart val,omitempty"`
	Shape         *BarShape    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart shape,omitempty"`
	ExtLst        *dml.ExtLst  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart extLst,omitempty"`
}

// LineSer represents CT_LineSer (c:ser in lineChart)
type LineSer struct {
	Idx      *UnsignedInt `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart idx,omitempty"`
	Order    *UnsignedInt `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart order,omitempty"`
	Tx       *SerTx       `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart tx,omitempty"`
	SpPr     *dml.SpPr    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart spPr,omitempty"`
	Marker   *Marker      `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart marker,omitempty"`
	DPt      []*DataPoint `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart dPt,omitempty"`
	DLbls    *DataLabels  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart dLbls,omitempty"`
	Trendline []*Trendline `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart trendline,omitempty"`
	ErrBars  *ErrBars     `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart errBars,omitempty"`
	Cat      *AxDataSource `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart cat,omitempty"`
	Val      *NumDataSource `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart val,omitempty"`
	Smooth   *Boolean     `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart smooth,omitempty"`
	ExtLst   *dml.ExtLst  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart extLst,omitempty"`
}

// PieSer represents CT_PieSer (c:ser in pieChart)
type PieSer struct {
	Idx        *UnsignedInt `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart idx,omitempty"`
	Order      *UnsignedInt `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart order,omitempty"`
	Tx         *SerTx       `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart tx,omitempty"`
	SpPr       *dml.SpPr    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart spPr,omitempty"`
	Explosion  *UnsignedInt `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart explosion,omitempty"`
	DPt        []*DataPoint `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart dPt,omitempty"`
	DLbls      *DataLabels  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart dLbls,omitempty"`
	Cat        *AxDataSource `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart cat,omitempty"`
	Val        *NumDataSource `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart val,omitempty"`
	ExtLst     *dml.ExtLst  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart extLst,omitempty"`
}

// AreaSer represents CT_AreaSer (c:ser in areaChart)
type AreaSer struct {
	Idx        *UnsignedInt  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart idx,omitempty"`
	Order      *UnsignedInt  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart order,omitempty"`
	Tx         *SerTx        `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart tx,omitempty"`
	SpPr       *dml.SpPr     `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart spPr,omitempty"`
	PictureOptions *PictureOptions `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart pictureOptions,omitempty"`
	DPt        []*DataPoint  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart dPt,omitempty"`
	DLbls      *DataLabels   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart dLbls,omitempty"`
	Trendline  []*Trendline  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart trendline,omitempty"`
	ErrBars    []*ErrBars    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart errBars,omitempty"`
	Cat        *AxDataSource `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart cat,omitempty"`
	Val        *NumDataSource `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart val,omitempty"`
	ExtLst     *dml.ExtLst   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart extLst,omitempty"`
}

// ScatterSer represents CT_ScatterSer (c:ser in scatterChart)
type ScatterSer struct {
	Idx        *UnsignedInt  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart idx,omitempty"`
	Order      *UnsignedInt  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart order,omitempty"`
	Tx         *SerTx        `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart tx,omitempty"`
	SpPr       *dml.SpPr     `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart spPr,omitempty"`
	Marker     *Marker       `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart marker,omitempty"`
	DPt        []*DataPoint  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart dPt,omitempty"`
	DLbls      *DataLabels   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart dLbls,omitempty"`
	Trendline  []*Trendline  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart trendline,omitempty"`
	ErrBars    []*ErrBars    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart errBars,omitempty"`
	XVal       *AxDataSource `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart xVal,omitempty"`
	YVal       *NumDataSource `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart yVal,omitempty"`
	Smooth     *Boolean      `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart smooth,omitempty"`
	ExtLst     *dml.ExtLst   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart extLst,omitempty"`
}

// BubbleSer represents CT_BubbleSer (c:ser in bubbleChart)
type BubbleSer struct {
	Idx           *UnsignedInt  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart idx,omitempty"`
	Order         *UnsignedInt  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart order,omitempty"`
	Tx            *SerTx        `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart tx,omitempty"`
	SpPr          *dml.SpPr     `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart spPr,omitempty"`
	InvertIfNegative *Boolean   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart invertIfNegative,omitempty"`
	DPt           []*DataPoint  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart dPt,omitempty"`
	DLbls         *DataLabels   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart dLbls,omitempty"`
	Trendline     []*Trendline  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart trendline,omitempty"`
	ErrBars       []*ErrBars    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart errBars,omitempty"`
	XVal          *AxDataSource `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart xVal,omitempty"`
	YVal          *NumDataSource `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart yVal,omitempty"`
	BubbleSize    *NumDataSource `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart bubbleSize,omitempty"`
	Bubble3D      *Boolean      `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart bubble3D,omitempty"`
	ExtLst        *dml.ExtLst   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart extLst,omitempty"`
}

// RadarSer represents CT_RadarSer (c:ser in radarChart)
type RadarSer struct {
	Idx    *UnsignedInt  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart idx,omitempty"`
	Order  *UnsignedInt  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart order,omitempty"`
	Tx     *SerTx        `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart tx,omitempty"`
	SpPr   *dml.SpPr     `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart spPr,omitempty"`
	Marker *Marker       `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart marker,omitempty"`
	DPt    []*DataPoint  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart dPt,omitempty"`
	DLbls  *DataLabels   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart dLbls,omitempty"`
	Cat    *AxDataSource `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart cat,omitempty"`
	Val    *NumDataSource `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart val,omitempty"`
	ExtLst *dml.ExtLst   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart extLst,omitempty"`
}

// SurfaceSer represents CT_SurfaceSer (c:ser in surfaceChart)
type SurfaceSer struct {
	Idx    *UnsignedInt  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart idx,omitempty"`
	Order  *UnsignedInt  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart order,omitempty"`
	Tx     *SerTx        `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart tx,omitempty"`
	SpPr   *dml.SpPr     `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart spPr,omitempty"`
	Cat    *AxDataSource `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart cat,omitempty"`
	Val    *NumDataSource `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart val,omitempty"`
	ExtLst *dml.ExtLst   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart extLst,omitempty"`
}

// SerTx represents CT_SerTx - series text (name)
type SerTx struct {
	StrRef *StrRef `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart strRef,omitempty"`
	V      string  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart v,omitempty"`
}

// DataPoint represents CT_DPt (c:dPt) - individual data point formatting
type DataPoint struct {
	Idx           *UnsignedInt   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart idx,omitempty"`
	InvertIfNegative *Boolean    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart invertIfNegative,omitempty"`
	Marker        *Marker        `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart marker,omitempty"`
	Bubble3D      *Boolean       `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart bubble3D,omitempty"`
	Explosion     *UnsignedInt   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart explosion,omitempty"`
	SpPr          *dml.SpPr      `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart spPr,omitempty"`
	PictureOptions *PictureOptions `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart pictureOptions,omitempty"`
	ExtLst        *dml.ExtLst    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart extLst,omitempty"`
}

// Marker represents CT_Marker (c:marker) - data point marker
type Marker struct {
	Symbol *MarkerStyle `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart symbol,omitempty"`
	Size   *MarkerSize  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart size,omitempty"`
	SpPr   *dml.SpPr    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart spPr,omitempty"`
	ExtLst *dml.ExtLst  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart extLst,omitempty"`
}

// --- Data Sources ---

// AxDataSource represents CT_AxDataSource (c:cat, c:xVal) - axis data
type AxDataSource struct {
	MultiLvlStrRef *MultiLvlStrRef `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart multiLvlStrRef,omitempty"`
	NumRef   *NumRef  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart numRef,omitempty"`
	NumLit   *NumData `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart numLit,omitempty"`
	StrRef   *StrRef  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart strRef,omitempty"`
	StrLit   *StrData `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart strLit,omitempty"`
}

// NumDataSource represents CT_NumDataSource (c:val, c:yVal) - numeric data
type NumDataSource struct {
	NumRef *NumRef  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart numRef,omitempty"`
	NumLit *NumData `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart numLit,omitempty"`
}

// NumRef represents CT_NumRef (c:numRef) - numeric reference
type NumRef struct {
	F        string   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart f,omitempty"` // formula
	NumCache *NumData `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart numCache,omitempty"`
	ExtLst   *dml.ExtLst `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart extLst,omitempty"`
}

// NumData represents CT_NumData (c:numCache, c:numLit)
type NumData struct {
	FormatCode string    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart formatCode,omitempty"`
	PtCount    *UnsignedInt `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart ptCount,omitempty"`
	Pt         []*NumVal `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart pt,omitempty"`
	ExtLst     *dml.ExtLst `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart extLst,omitempty"`
}

// NumVal represents CT_NumVal (c:pt) - a numeric value
type NumVal struct {
	Idx      uint32 `xml:"idx,attr"`
	FormatCode string `xml:"formatCode,attr,omitempty"`
	V        string `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart v,omitempty"`
}

// StrRef represents CT_StrRef (c:strRef) - string reference
type StrRef struct {
	F        string   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart f,omitempty"`
	StrCache *StrData `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart strCache,omitempty"`
	ExtLst   *dml.ExtLst `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart extLst,omitempty"`
}

// StrData represents CT_StrData (c:strCache, c:strLit)
type StrData struct {
	PtCount *UnsignedInt `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart ptCount,omitempty"`
	Pt      []*StrVal    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart pt,omitempty"`
	ExtLst  *dml.ExtLst  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart extLst,omitempty"`
}

// StrVal represents CT_StrVal (c:pt) - a string value
type StrVal struct {
	Idx uint32 `xml:"idx,attr"`
	V   string `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart v,omitempty"`
}

// MultiLvlStrRef represents CT_MultiLvlStrRef - multi-level string reference
type MultiLvlStrRef struct {
	F              string         `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart f,omitempty"`
	MultiLvlStrCache *MultiLvlStrData `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart multiLvlStrCache,omitempty"`
	ExtLst         *dml.ExtLst    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart extLst,omitempty"`
}

// MultiLvlStrData represents CT_MultiLvlStrData
type MultiLvlStrData struct {
	PtCount *UnsignedInt `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart ptCount,omitempty"`
	Lvl     []*Level     `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart lvl,omitempty"`
	ExtLst  *dml.ExtLst  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart extLst,omitempty"`
}

// Level represents CT_Lvl
type Level struct {
	Pt []*StrVal `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart pt,omitempty"`
}
