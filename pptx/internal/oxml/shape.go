package oxml

import (
	"encoding/xml"

	"github.com/mgilbir/spine/common/dml"
)

// Shape represents a shape element (p:sp) in a slide.
type Shape struct {
	XMLName xml.Name    `xml:"http://schemas.openxmlformats.org/presentationml/2006/main sp"`
	NvSpPr  *NvSpPr     `xml:"nvSpPr"`
	SpPr    *dml.SpPr   `xml:"spPr"`
	Style   *dml.Style  `xml:"style,omitempty"`
	TxBody  *dml.TxBody `xml:"txBody,omitempty"`
}

// NvSpPr contains non-visual shape properties.
type NvSpPr struct {
	CNvPr   *dml.CNvPr   `xml:"cNvPr"`
	CNvSpPr *dml.CNvSpPr `xml:"cNvSpPr"`
	NvPr    *NvPr        `xml:"nvPr"`
}

// NvPr contains application-specific non-visual properties.
// This is CT_ApplicationNonVisualDrawingProps in the PML XSD.
type NvPr struct {
	IsPhoto       bool                `xml:"isPhoto,attr,omitempty"`
	UserDrawn     bool                `xml:"userDrawn,attr,omitempty"`
	Ph            *Placeholder        `xml:"ph,omitempty"`
	AudioCd       *dml.AudioCD        `xml:"http://schemas.openxmlformats.org/drawingml/2006/main audioCd,omitempty"`
	WavAudioFile  *dml.EmbeddedWAVXML `xml:"http://schemas.openxmlformats.org/drawingml/2006/main wavAudioFile,omitempty"`
	AudioFile     *dml.AudioFile      `xml:"http://schemas.openxmlformats.org/drawingml/2006/main audioFile,omitempty"`
	VideoFile     *dml.VideoFile      `xml:"http://schemas.openxmlformats.org/drawingml/2006/main videoFile,omitempty"`
	QuickTimeFile *dml.QuickTimeFile  `xml:"http://schemas.openxmlformats.org/drawingml/2006/main quickTimeFile,omitempty"`
	ExtLst        *ExtensionList      `xml:"extLst,omitempty"`
}

// Placeholder specifies placeholder information.
// This is CT_Placeholder in the PML XSD.
type Placeholder struct {
	Type            string `xml:"type,attr,omitempty"`
	Orient          string `xml:"orient,attr,omitempty"`
	Sz              string `xml:"sz,attr,omitempty"`
	Idx             uint32 `xml:"idx,attr,omitempty"`
	HasCustomPrompt bool   `xml:"hasCustomPrompt,attr,omitempty"`
}
