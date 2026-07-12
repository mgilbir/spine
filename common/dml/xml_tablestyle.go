// This file provides DrawingML table style list type from dml-main.xsd.

package dml

// TblStyleLst represents CT_TableStyleList (a:tblStyleLst)
type TblStyleLst struct {
	Def      string        `xml:"def,attr,omitempty"`
	TblStyle []*TableStyle `xml:"http://schemas.openxmlformats.org/drawingml/2006/main tblStyle,omitempty"`
}
