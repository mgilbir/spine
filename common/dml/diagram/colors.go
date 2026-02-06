package diagram

import "github.com/mgilbir/spine/common/dml"

// --- Color Transform Definition (dgm:colorsDef) ---

// ColorsDef represents CT_ColorTransform (dgm:colorsDef) - color transform definition root
type ColorsDef struct {
	UniqueId string             `xml:"uniqueId,attr,omitempty"`
	MinVer   string             `xml:"minVer,attr,omitempty"`
	Title    []*DiagTitle       `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram title,omitempty"`
	Desc     []*DiagDesc        `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram desc,omitempty"`
	CatLst   *CatLst            `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram catLst,omitempty"`
	StyleLbl []*CTStyleLabel    `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram styleLbl,omitempty"`
	ExtLst   *dml.ExtLst        `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram extLst,omitempty"`
}

// CTStyleLabel represents CT_CTStyleLabel (dgm:styleLbl) - color transform style label
type CTStyleLabel struct {
	Name          string        `xml:"name,attr"`
	FillClrLst    *ColorList    `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram fillClrLst,omitempty"`
	LinClrLst     *ColorList    `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram linClrLst,omitempty"`
	EffectClrLst  *ColorList    `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram effectClrLst,omitempty"`
	TxLinClrLst   *ColorList    `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram txLinClrLst,omitempty"`
	TxFillClrLst  *ColorList    `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram txFillClrLst,omitempty"`
	TxEffectClrLst *ColorList   `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram txEffectClrLst,omitempty"`
	ExtLst        *dml.ExtLst   `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram extLst,omitempty"`
}

// ColorList represents CT_Colors (dgm:fillClrLst, etc.) - list of colors
type ColorList struct {
	Meth      string                `xml:"meth,attr,omitempty"` // ST_ClrAppMethod: span, cycle, repeat
	HueDir    string                `xml:"hueDir,attr,omitempty"` // ST_HueDir: cw, ccw
	SrgbClr   []*dml.SrgbClr       `xml:"http://schemas.openxmlformats.org/drawingml/2006/main srgbClr,omitempty"`
	SchemeClr []*dml.SchemeClrTransform `xml:"http://schemas.openxmlformats.org/drawingml/2006/main schemeClr,omitempty"`
}

// ColorsDefHdr represents CT_ColorTransformHeader (dgm:colorsDefHdr)
type ColorsDefHdr struct {
	UniqueId string       `xml:"uniqueId,attr"`
	MinVer   string       `xml:"minVer,attr,omitempty"`
	ResId    string       `xml:"resId,attr,omitempty"`
	Title    []*DiagTitle `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram title,omitempty"`
	Desc     []*DiagDesc  `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram desc,omitempty"`
	CatLst   *CatLst      `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram catLst,omitempty"`
	ExtLst   *dml.ExtLst  `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram extLst,omitempty"`
}

// ColorsDefHdrLst represents CT_ColorTransformHeaderLst (dgm:colorsDefHdrLst)
type ColorsDefHdrLst struct {
	ColorsDefHdr []*ColorsDefHdr `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram colorsDefHdr,omitempty"`
}
