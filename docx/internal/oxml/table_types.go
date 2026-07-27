package oxml

import (
	"encoding/xml"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// CT_TblPrEx represents table property exceptions (w:tblPrEx).
// These override table properties for specific rows.
type CT_TblPrEx struct {
	TblW           *CT_TblWidth      `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main tblW,omitempty"`
	Jc             *CT_Jc            `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main jc,omitempty"`
	TblCellSpacing *CT_TblWidth      `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main tblCellSpacing,omitempty"`
	TblInd         *CT_TblWidth      `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main tblInd,omitempty"`
	TblBorders     *CT_TblBorders    `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main tblBorders,omitempty"`
	Shd            *CT_Shd           `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main shd,omitempty"`
	TblLayout      *CT_TblLayout     `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main tblLayout,omitempty"`
	TblCellMar     *CT_TblCellMar    `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main tblCellMar,omitempty"`
	TblLook        *CT_TblLook       `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main tblLook,omitempty"`
	TblPrExChange  *CT_TblPrExChange `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main tblPrExChange,omitempty"`
}

// CT_TblPrExChange represents a table property exception change tracking entry.
type CT_TblPrExChange struct {
	ID      string      `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main id,attr,omitempty"`
	Author  string      `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main author,attr,omitempty"`
	Date    string      `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main date,attr,omitempty"`
	TblPrEx *CT_TblPrEx `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main tblPrEx,omitempty"`
	// CapturedAttrs preserves the verbatim source attribute list (Word 2021+
	// writes an unmodeled w16du:dateUtc on every revision record); the
	// reflection marshaler replays it (C411).
	CapturedAttrs []xmlb.RootAttr `xml:"-"`
}

// UnmarshalXML captures the element's verbatim attribute list before decoding
// through the struct tags; the reflection marshaler replays it.
func (c *CT_TblPrExChange) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	c.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias CT_TblPrExChange
	return d.DecodeElement((*alias)(c), &start)
}
