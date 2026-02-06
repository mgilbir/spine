package chart

import "github.com/mgilbir/spine/common/dml"

// --- Axes ---

// ValAx represents CT_ValAx (c:valAx) - value axis
type ValAx struct {
	AxId         *UnsignedInt `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart axId,omitempty"`
	Scaling      *Scaling     `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart scaling,omitempty"`
	Delete       *Boolean     `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart delete,omitempty"`
	AxPos        *AxPos       `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart axPos,omitempty"`
	MajorGridlines *ChartLines `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart majorGridlines,omitempty"`
	MinorGridlines *ChartLines `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart minorGridlines,omitempty"`
	Title        *Title       `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart title,omitempty"`
	NumFmt       *NumFmt      `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart numFmt,omitempty"`
	MajorTickMark *TickMark   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart majorTickMark,omitempty"`
	MinorTickMark *TickMark   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart minorTickMark,omitempty"`
	TickLblPos   *TickLblPos  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart tickLblPos,omitempty"`
	SpPr         *dml.SpPr    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart spPr,omitempty"`
	TxPr         *dml.TxBody  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart txPr,omitempty"`
	CrossAx      *UnsignedInt `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart crossAx,omitempty"`
	Crosses      *Crosses     `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart crosses,omitempty"`
	CrossesAt    *Double      `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart crossesAt,omitempty"`
	CrossBetween *CrossBetween `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart crossBetween,omitempty"`
	MajorUnit    *Double      `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart majorUnit,omitempty"`
	MinorUnit    *Double      `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart minorUnit,omitempty"`
	DispUnits    *DispUnits   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart dispUnits,omitempty"`
	ExtLst       *dml.ExtLst  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart extLst,omitempty"`
}

// CatAx represents CT_CatAx (c:catAx) - category axis
type CatAx struct {
	AxId         *UnsignedInt `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart axId,omitempty"`
	Scaling      *Scaling     `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart scaling,omitempty"`
	Delete       *Boolean     `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart delete,omitempty"`
	AxPos        *AxPos       `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart axPos,omitempty"`
	MajorGridlines *ChartLines `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart majorGridlines,omitempty"`
	MinorGridlines *ChartLines `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart minorGridlines,omitempty"`
	Title        *Title       `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart title,omitempty"`
	NumFmt       *NumFmt      `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart numFmt,omitempty"`
	MajorTickMark *TickMark   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart majorTickMark,omitempty"`
	MinorTickMark *TickMark   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart minorTickMark,omitempty"`
	TickLblPos   *TickLblPos  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart tickLblPos,omitempty"`
	SpPr         *dml.SpPr    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart spPr,omitempty"`
	TxPr         *dml.TxBody  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart txPr,omitempty"`
	CrossAx      *UnsignedInt `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart crossAx,omitempty"`
	Crosses      *Crosses     `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart crosses,omitempty"`
	CrossesAt    *Double      `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart crossesAt,omitempty"`
	Auto         *Boolean     `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart auto,omitempty"`
	LblAlgn      *LblAlgn     `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart lblAlgn,omitempty"`
	LblOffset    *LblOffset   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart lblOffset,omitempty"`
	TickLblSkip  *Skip        `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart tickLblSkip,omitempty"`
	TickMarkSkip *Skip        `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart tickMarkSkip,omitempty"`
	NoMultiLvlLbl *Boolean    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart noMultiLvlLbl,omitempty"`
	ExtLst       *dml.ExtLst  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart extLst,omitempty"`
}

// DateAx represents CT_DateAx (c:dateAx) - date axis
type DateAx struct {
	AxId         *UnsignedInt `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart axId,omitempty"`
	Scaling      *Scaling     `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart scaling,omitempty"`
	Delete       *Boolean     `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart delete,omitempty"`
	AxPos        *AxPos       `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart axPos,omitempty"`
	MajorGridlines *ChartLines `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart majorGridlines,omitempty"`
	MinorGridlines *ChartLines `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart minorGridlines,omitempty"`
	Title        *Title       `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart title,omitempty"`
	NumFmt       *NumFmt      `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart numFmt,omitempty"`
	MajorTickMark *TickMark   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart majorTickMark,omitempty"`
	MinorTickMark *TickMark   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart minorTickMark,omitempty"`
	TickLblPos   *TickLblPos  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart tickLblPos,omitempty"`
	SpPr         *dml.SpPr    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart spPr,omitempty"`
	TxPr         *dml.TxBody  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart txPr,omitempty"`
	CrossAx      *UnsignedInt `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart crossAx,omitempty"`
	Crosses      *Crosses     `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart crosses,omitempty"`
	CrossesAt    *Double      `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart crossesAt,omitempty"`
	Auto         *Boolean     `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart auto,omitempty"`
	LblOffset    *LblOffset   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart lblOffset,omitempty"`
	BaseTimeUnit *TimeUnit    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart baseTimeUnit,omitempty"`
	MajorUnit    *Double      `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart majorUnit,omitempty"`
	MajorTimeUnit *TimeUnit   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart majorTimeUnit,omitempty"`
	MinorUnit    *Double      `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart minorUnit,omitempty"`
	MinorTimeUnit *TimeUnit   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart minorTimeUnit,omitempty"`
	ExtLst       *dml.ExtLst  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart extLst,omitempty"`
}

// SerAx represents CT_SerAx (c:serAx) - series axis (3D charts)
type SerAx struct {
	AxId         *UnsignedInt `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart axId,omitempty"`
	Scaling      *Scaling     `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart scaling,omitempty"`
	Delete       *Boolean     `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart delete,omitempty"`
	AxPos        *AxPos       `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart axPos,omitempty"`
	MajorGridlines *ChartLines `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart majorGridlines,omitempty"`
	MinorGridlines *ChartLines `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart minorGridlines,omitempty"`
	Title        *Title       `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart title,omitempty"`
	NumFmt       *NumFmt      `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart numFmt,omitempty"`
	MajorTickMark *TickMark   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart majorTickMark,omitempty"`
	MinorTickMark *TickMark   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart minorTickMark,omitempty"`
	TickLblPos   *TickLblPos  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart tickLblPos,omitempty"`
	SpPr         *dml.SpPr    `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart spPr,omitempty"`
	TxPr         *dml.TxBody  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart txPr,omitempty"`
	CrossAx      *UnsignedInt `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart crossAx,omitempty"`
	Crosses      *Crosses     `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart crosses,omitempty"`
	CrossesAt    *Double      `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart crossesAt,omitempty"`
	TickLblSkip  *Skip        `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart tickLblSkip,omitempty"`
	TickMarkSkip *Skip        `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart tickMarkSkip,omitempty"`
	ExtLst       *dml.ExtLst  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart extLst,omitempty"`
}

// Scaling represents CT_Scaling (c:scaling) - axis scaling
type Scaling struct {
	LogBase     *LogBase     `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart logBase,omitempty"`
	Orientation *Orientation `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart orientation,omitempty"`
	Max         *Double      `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart max,omitempty"`
	Min         *Double      `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart min,omitempty"`
	ExtLst      *dml.ExtLst  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart extLst,omitempty"`
}

// DispUnits represents CT_DispUnits (c:dispUnits) - display units
type DispUnits struct {
	BuiltInUnit *BuiltInUnit `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart builtInUnit,omitempty"`
	CustUnit    *Double      `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart custUnit,omitempty"`
	DispUnitsLbl *DispUnitsLbl `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart dispUnitsLbl,omitempty"`
	ExtLst      *dml.ExtLst  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart extLst,omitempty"`
}

// DispUnitsLbl represents CT_DispUnitsLbl
type DispUnitsLbl struct {
	Layout *Layout     `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart layout,omitempty"`
	Tx     *ChartText  `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart tx,omitempty"`
	SpPr   *dml.SpPr   `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart spPr,omitempty"`
	TxPr   *dml.TxBody `xml:"http://schemas.openxmlformats.org/drawingml/2006/chart txPr,omitempty"`
}
