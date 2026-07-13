// This file provides Picture namespace types from dml-picture.xsd.
// These types represent pic: namespace elements.

package dml

// PicPic represents CT_Picture (pic:pic) - standalone picture element
type PicPic struct {
	NvPicPr  *PicNvPicPr  `xml:"http://schemas.openxmlformats.org/drawingml/2006/picture nvPicPr,omitempty"`
	BlipFill *BlipFillXML `xml:"http://schemas.openxmlformats.org/drawingml/2006/picture blipFill,omitempty"`
	SpPr     *SpPr        `xml:"http://schemas.openxmlformats.org/drawingml/2006/picture spPr,omitempty"`
}

// PicNvPicPr represents CT_PictureNonVisual (pic:nvPicPr)
type PicNvPicPr struct {
	CNvPr    *CNvPr    `xml:"http://schemas.openxmlformats.org/drawingml/2006/picture cNvPr,omitempty"`
	CNvPicPr *CNvPicPr `xml:"http://schemas.openxmlformats.org/drawingml/2006/picture cNvPicPr,omitempty"`
}
