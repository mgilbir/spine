package diagram

import "github.com/mgilbir/spine/common/dml"

// --- Style Definition (dgm:styleDef) ---

// StyleDef represents CT_StyleDefinition (dgm:styleDef) - style definition root
type StyleDef struct {
	UniqueId string          `xml:"uniqueId,attr,omitempty"`
	MinVer   string          `xml:"minVer,attr,omitempty"`
	Title    []*DiagTitle    `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram title,omitempty"`
	Desc     []*DiagDesc     `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram desc,omitempty"`
	CatLst   *CatLst         `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram catLst,omitempty"`
	StyleLbl []*StyleDefLabel `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram styleLbl,omitempty"`
	Scene3d  *dml.Scene3d    `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram scene3d,omitempty"`
	ExtLst   *dml.ExtLst     `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram extLst,omitempty"`
}

// StyleDefLabel represents CT_StyleLabel (dgm:styleLbl) - style label with visual properties
type StyleDefLabel struct {
	Name   string      `xml:"name,attr"`
	Scene3d *dml.Scene3d `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram scene3d,omitempty"`
	Sp3d   *dml.Sp3d   `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram sp3d,omitempty"`
	TxPr   *DiagTxPr   `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram txPr,omitempty"`
	Style  *DiagStyle  `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram style,omitempty"`
	ExtLst *dml.ExtLst `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram extLst,omitempty"`
}

// DiagTxPr represents CT_TextProps (dgm:txPr) - text properties for diagram styles
type DiagTxPr struct {
	Sp3d      *dml.Sp3d       `xml:"http://schemas.openxmlformats.org/drawingml/2006/main sp3d,omitempty"`
	FlatTx    *dml.FlatTx     `xml:"http://schemas.openxmlformats.org/drawingml/2006/main flatTx,omitempty"`
}

// DiagStyle represents CT_ShapeStyle (dgm:style) - shape style reference for diagram
type DiagStyle struct {
	LnRef     *StyleMatrixRef `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lnRef,omitempty"`
	FillRef   *StyleMatrixRef `xml:"http://schemas.openxmlformats.org/drawingml/2006/main fillRef,omitempty"`
	EffectRef *StyleMatrixRef `xml:"http://schemas.openxmlformats.org/drawingml/2006/main effectRef,omitempty"`
	FontRef   *FontReference  `xml:"http://schemas.openxmlformats.org/drawingml/2006/main fontRef,omitempty"`
}

// StyleMatrixRef represents CT_StyleMatrixReference (a:lnRef, a:fillRef, a:effectRef)
type StyleMatrixRef struct {
	Idx       uint32                    `xml:"idx,attr"`
	SrgbClr   *dml.SrgbClr             `xml:"http://schemas.openxmlformats.org/drawingml/2006/main srgbClr,omitempty"`
	SchemeClr *dml.SchemeClrTransform   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main schemeClr,omitempty"`
}

// FontReference represents CT_FontReference (a:fontRef)
type FontReference struct {
	Idx       string                    `xml:"idx,attr"` // ST_FontCollectionIndex: major, minor, none
	SrgbClr   *dml.SrgbClr             `xml:"http://schemas.openxmlformats.org/drawingml/2006/main srgbClr,omitempty"`
	SchemeClr *dml.SchemeClrTransform   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main schemeClr,omitempty"`
}

// StyleDefHdr represents CT_StyleDefinitionHeader (dgm:styleDefHdr)
type StyleDefHdr struct {
	UniqueId string       `xml:"uniqueId,attr"`
	MinVer   string       `xml:"minVer,attr,omitempty"`
	ResId    string       `xml:"resId,attr,omitempty"`
	Title    []*DiagTitle `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram title,omitempty"`
	Desc     []*DiagDesc  `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram desc,omitempty"`
	CatLst   *CatLst      `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram catLst,omitempty"`
	ExtLst   *dml.ExtLst  `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram extLst,omitempty"`
}

// StyleDefHdrLst represents CT_StyleDefinitionHeaderLst (dgm:styleDefHdrLst)
type StyleDefHdrLst struct {
	StyleDefHdr []*StyleDefHdr `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram styleDefHdr,omitempty"`
}
