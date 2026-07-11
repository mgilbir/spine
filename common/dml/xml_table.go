// Package dml provides DrawingML XML table types from dml-main.xsd.
package dml

// Tbl represents CT_Table (a:tbl)
type Tbl struct {
	TblPr   *TblPr   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main tblPr,omitempty"`
	TblGrid *TblGrid `xml:"http://schemas.openxmlformats.org/drawingml/2006/main tblGrid,omitempty"`
	Tr      []*Tr    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main tr,omitempty"`
}

// TblPr represents CT_TableProperties (a:tblPr)
type TblPr struct {
	Rtl          bool         `xml:"rtl,attr,omitempty"`
	FirstRow     bool         `xml:"firstRow,attr,omitempty"`
	FirstCol     bool         `xml:"firstCol,attr,omitempty"`
	LastRow      bool         `xml:"lastRow,attr,omitempty"`
	LastCol      bool         `xml:"lastCol,attr,omitempty"`
	BandRow      bool         `xml:"bandRow,attr,omitempty"`
	BandCol      bool         `xml:"bandCol,attr,omitempty"`
	NoFill       *NoFillXML   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main noFill,omitempty"`
	SolidFill    *SolidFill   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main solidFill,omitempty"`
	GradFill     *GradFill    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main gradFill,omitempty"`
	BlipFill     *BlipFillXML `xml:"http://schemas.openxmlformats.org/drawingml/2006/main blipFill,omitempty"`
	PattFill     *PattFill    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main pattFill,omitempty"`
	GrpFill      *GrpFill     `xml:"http://schemas.openxmlformats.org/drawingml/2006/main grpFill,omitempty"`
	EffectLst    *EffectLst   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main effectLst,omitempty"`
	EffectDag    *EffectDag   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main effectDag,omitempty"`
	TableStyleId string       `xml:"http://schemas.openxmlformats.org/drawingml/2006/main tableStyleId,omitempty"`
	ExtLst       *ExtLst      `xml:"http://schemas.openxmlformats.org/drawingml/2006/main extLst,omitempty"`
}

// TblGrid represents CT_TableGrid (a:tblGrid)
type TblGrid struct {
	GridCol []*GridCol `xml:"http://schemas.openxmlformats.org/drawingml/2006/main gridCol,omitempty"`
}

// GridCol represents CT_TableCol (a:gridCol)
type GridCol struct {
	W int64 `xml:"w,attr"`
}

// Tr represents CT_TableRow (a:tr)
type Tr struct {
	H  int64 `xml:"h,attr,omitempty"`
	Tc []*Tc `xml:"http://schemas.openxmlformats.org/drawingml/2006/main tc,omitempty"`
}

// Tc represents CT_TableCell (a:tc)
type Tc struct {
	RowSpan  int32   `xml:"rowSpan,attr,omitempty"`
	GridSpan int32   `xml:"gridSpan,attr,omitempty"`
	HMerge   bool    `xml:"hMerge,attr,omitempty"`
	VMerge   bool    `xml:"vMerge,attr,omitempty"`
	Id       string  `xml:"id,attr,omitempty"`
	TxBody   *TxBody `xml:"http://schemas.openxmlformats.org/drawingml/2006/main txBody,omitempty"`
	TcPr     *TcPr   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main tcPr,omitempty"`
}

// TcPr represents CT_TableCellProperties (a:tcPr)
type TcPr struct {
	MarL         *int64       `xml:"marL,attr,omitempty"`
	MarR         *int64       `xml:"marR,attr,omitempty"`
	MarT         *int64       `xml:"marT,attr,omitempty"`
	MarB         *int64       `xml:"marB,attr,omitempty"`
	Vert         string       `xml:"vert,attr,omitempty"`
	Anchor       string       `xml:"anchor,attr,omitempty"`
	AnchorCtr    *bool        `xml:"anchorCtr,attr,omitempty"`
	HorzOverflow string       `xml:"horzOverflow,attr,omitempty"`
	LnL          *Ln          `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lnL,omitempty"`
	LnR          *Ln          `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lnR,omitempty"`
	LnT          *Ln          `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lnT,omitempty"`
	LnB          *Ln          `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lnB,omitempty"`
	LnTlToBr     *Ln          `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lnTlToBr,omitempty"`
	LnBlToTr     *Ln          `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lnBlToTr,omitempty"`
	Cell3D       *Cell3D      `xml:"http://schemas.openxmlformats.org/drawingml/2006/main cell3D,omitempty"`
	NoFill       *NoFillXML   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main noFill,omitempty"`
	SolidFill    *SolidFill   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main solidFill,omitempty"`
	GradFill     *GradFill    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main gradFill,omitempty"`
	BlipFill     *BlipFillXML `xml:"http://schemas.openxmlformats.org/drawingml/2006/main blipFill,omitempty"`
	PattFill     *PattFill    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main pattFill,omitempty"`
	GrpFill      *GrpFill     `xml:"http://schemas.openxmlformats.org/drawingml/2006/main grpFill,omitempty"`
	ExtLst       *ExtLst      `xml:"http://schemas.openxmlformats.org/drawingml/2006/main extLst,omitempty"`
}

// Cell3D represents CT_Cell3D (a:cell3D)
type Cell3D struct {
	PrstMaterial string    `xml:"prstMaterial,attr,omitempty"`
	Bevel        *Bevel3d  `xml:"http://schemas.openxmlformats.org/drawingml/2006/main bevel,omitempty"`
	LightRig     *LightRig `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lightRig,omitempty"`
}

// TableStyle represents CT_TableStyle (a:tblStyle)
type TableStyle struct {
	StyleId   string          `xml:"styleId,attr"`
	StyleName string          `xml:"styleName,attr"`
	TblBg     *TableBgStyle   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main tblBg,omitempty"`
	WholeTbl  *TablePartStyle `xml:"http://schemas.openxmlformats.org/drawingml/2006/main wholeTbl,omitempty"`
	Band1H    *TablePartStyle `xml:"http://schemas.openxmlformats.org/drawingml/2006/main band1H,omitempty"`
	Band2H    *TablePartStyle `xml:"http://schemas.openxmlformats.org/drawingml/2006/main band2H,omitempty"`
	Band1V    *TablePartStyle `xml:"http://schemas.openxmlformats.org/drawingml/2006/main band1V,omitempty"`
	Band2V    *TablePartStyle `xml:"http://schemas.openxmlformats.org/drawingml/2006/main band2V,omitempty"`
	LastCol   *TablePartStyle `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lastCol,omitempty"`
	FirstCol  *TablePartStyle `xml:"http://schemas.openxmlformats.org/drawingml/2006/main firstCol,omitempty"`
	LastRow   *TablePartStyle `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lastRow,omitempty"`
	SeCell    *TablePartStyle `xml:"http://schemas.openxmlformats.org/drawingml/2006/main seCell,omitempty"`
	SwCell    *TablePartStyle `xml:"http://schemas.openxmlformats.org/drawingml/2006/main swCell,omitempty"`
	FirstRow  *TablePartStyle `xml:"http://schemas.openxmlformats.org/drawingml/2006/main firstRow,omitempty"`
	NeCell    *TablePartStyle `xml:"http://schemas.openxmlformats.org/drawingml/2006/main neCell,omitempty"`
	NwCell    *TablePartStyle `xml:"http://schemas.openxmlformats.org/drawingml/2006/main nwCell,omitempty"`
	ExtLst    *ExtLst         `xml:"http://schemas.openxmlformats.org/drawingml/2006/main extLst,omitempty"`
}

// TableBgStyle represents CT_TableBackgroundStyle (a:tblBg)
type TableBgStyle struct {
	NoFill    *NoFillXML   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main noFill,omitempty"`
	SolidFill *SolidFill   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main solidFill,omitempty"`
	GradFill  *GradFill    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main gradFill,omitempty"`
	BlipFill  *BlipFillXML `xml:"http://schemas.openxmlformats.org/drawingml/2006/main blipFill,omitempty"`
	PattFill  *PattFill    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main pattFill,omitempty"`
	GrpFill   *GrpFill     `xml:"http://schemas.openxmlformats.org/drawingml/2006/main grpFill,omitempty"`
	EffectLst *EffectLst   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main effectLst,omitempty"`
	EffectDag *EffectDag   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main effectDag,omitempty"`
}

// TablePartStyle represents CT_TablePartStyle (a:wholeTbl, a:band1H, etc.)
type TablePartStyle struct {
	TcTxStyle *TcTxStyle `xml:"http://schemas.openxmlformats.org/drawingml/2006/main tcTxStyle,omitempty"`
	TcStyle   *TcStyle   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main tcStyle,omitempty"`
}

// TcTxStyle represents CT_TableStyleTextStyle (a:tcTxStyle). Its a:font child
// is a CT_FontCollection (latin/ea/cs typefaces), distinct from the a:fontRef
// which is a CT_FontReference.
// Its color is an EG_ColorChoice, so all six color kinds are modeled.
type TcTxStyle struct {
	B         string              `xml:"b,attr,omitempty"`
	I         string              `xml:"i,attr,omitempty"`
	Font      *FontCollection     `xml:"http://schemas.openxmlformats.org/drawingml/2006/main font,omitempty"`
	FontRef   *FontRef            `xml:"http://schemas.openxmlformats.org/drawingml/2006/main fontRef,omitempty"`
	ScRgbClr  *ScRgbClr           `xml:"http://schemas.openxmlformats.org/drawingml/2006/main scrgbClr,omitempty"`
	SrgbClr   *SrgbClr            `xml:"http://schemas.openxmlformats.org/drawingml/2006/main srgbClr,omitempty"`
	HslClr    *HslClr             `xml:"http://schemas.openxmlformats.org/drawingml/2006/main hslClr,omitempty"`
	SysClr    *SystemClr          `xml:"http://schemas.openxmlformats.org/drawingml/2006/main sysClr,omitempty"`
	SchemeClr *SchemeClrTransform `xml:"http://schemas.openxmlformats.org/drawingml/2006/main schemeClr,omitempty"`
	PrstClr   *PrstClr            `xml:"http://schemas.openxmlformats.org/drawingml/2006/main prstClr,omitempty"`
	ExtLst    *ExtLst             `xml:"http://schemas.openxmlformats.org/drawingml/2006/main extLst,omitempty"`
}

// TcStyle represents CT_TableStyleCellStyle (a:tcStyle)
type TcStyle struct {
	TcBdr     *TcBdr       `xml:"http://schemas.openxmlformats.org/drawingml/2006/main tcBdr,omitempty"`
	NoFill    *NoFillXML   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main noFill,omitempty"`
	SolidFill *SolidFill   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main solidFill,omitempty"`
	GradFill  *GradFill    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main gradFill,omitempty"`
	BlipFill  *BlipFillXML `xml:"http://schemas.openxmlformats.org/drawingml/2006/main blipFill,omitempty"`
	PattFill  *PattFill    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main pattFill,omitempty"`
	GrpFill   *GrpFill     `xml:"http://schemas.openxmlformats.org/drawingml/2006/main grpFill,omitempty"`
	Cell3D    *Cell3D      `xml:"http://schemas.openxmlformats.org/drawingml/2006/main cell3D,omitempty"`
}

// TcBdr represents CT_TableCellBorderStyle (a:tcBdr)
type TcBdr struct {
	Left    *ThemeableLineStyle `xml:"http://schemas.openxmlformats.org/drawingml/2006/main left,omitempty"`
	Right   *ThemeableLineStyle `xml:"http://schemas.openxmlformats.org/drawingml/2006/main right,omitempty"`
	Top     *ThemeableLineStyle `xml:"http://schemas.openxmlformats.org/drawingml/2006/main top,omitempty"`
	Bottom  *ThemeableLineStyle `xml:"http://schemas.openxmlformats.org/drawingml/2006/main bottom,omitempty"`
	InsideH *ThemeableLineStyle `xml:"http://schemas.openxmlformats.org/drawingml/2006/main insideH,omitempty"`
	InsideV *ThemeableLineStyle `xml:"http://schemas.openxmlformats.org/drawingml/2006/main insideV,omitempty"`
	Tl2br   *ThemeableLineStyle `xml:"http://schemas.openxmlformats.org/drawingml/2006/main tl2br,omitempty"`
	Tr2bl   *ThemeableLineStyle `xml:"http://schemas.openxmlformats.org/drawingml/2006/main tr2bl,omitempty"`
}

// ThemeableLineStyle represents CT_ThemeableLineStyle
type ThemeableLineStyle struct {
	Ln    *Ln    `xml:"http://schemas.openxmlformats.org/drawingml/2006/main ln,omitempty"`
	LnRef *LnRef `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lnRef,omitempty"`
}
