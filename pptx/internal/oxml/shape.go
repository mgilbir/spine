package oxml

import (
	"encoding/xml"

	"github.com/mgilbir/spine/common/dml"
	xmlb "github.com/mgilbir/spine/common/xml"
)

// Shape represents a shape element (p:sp) in a slide.
// useBgFill defaults to false, so bool+omitempty carries every non-default
// value (C29 rule); CapturedAttrs additionally keeps a producer's explicit
// useBgFill="0" and any unmodeled attribute, which the shape tree's
// always-remarshal path would otherwise delete (C420).
type Shape struct {
	XMLName   xml.Name       `xml:"http://schemas.openxmlformats.org/presentationml/2006/main sp"`
	UseBgFill bool           `xml:"useBgFill,attr,omitempty"`
	NvSpPr    *NvSpPr        `xml:"nvSpPr"`
	SpPr      *dml.SpPr      `xml:"spPr"`
	Style     *dml.Style     `xml:"style,omitempty"`
	TxBody    *dml.TxBody    `xml:"txBody,omitempty"`
	ExtLst    *ExtensionList `xml:"extLst,omitempty"`

	CapturedAttrs []xmlb.RootAttr `xml:"-"` // verbatim source attrs; see common/xml.CaptureAttrs
}

// UnmarshalXML captures the element's verbatim attribute list (source
// attribute order, unmodeled attributes and an explicit useBgFill="0") before
// decoding through the struct tags; the reflection marshaler replays it.
func (sp *Shape) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	sp.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias Shape
	return d.DecodeElement((*alias)(sp), &start)
}

// NvSpPr contains non-visual shape properties.
type NvSpPr struct {
	CNvPr   *dml.CNvPr   `xml:"cNvPr"`
	CNvSpPr *dml.CNvSpPr `xml:"cNvSpPr"`
	NvPr    *NvPr        `xml:"nvPr"`
}

// NvPr contains application-specific non-visual properties.
// This is CT_ApplicationNonVisualDrawingProps in the PML XSD.
// isPhoto/userDrawn are XSD default-false, so an explicit "0" would be deleted
// by omitempty on the always-remarshaled shape-tree path — CapturedAttrs keeps
// it (C420).
type NvPr struct {
	CapturedAttrs []xmlb.RootAttr     `xml:"-"` // verbatim source attrs; see common/xml.CaptureAttrs
	IsPhoto       bool                `xml:"isPhoto,attr,omitempty"`
	UserDrawn     bool                `xml:"userDrawn,attr,omitempty"`
	Ph            *Placeholder        `xml:"ph,omitempty"`
	AudioCd       *dml.AudioCD        `xml:"http://schemas.openxmlformats.org/drawingml/2006/main audioCd,omitempty"`
	WavAudioFile  *dml.EmbeddedWAVXML `xml:"http://schemas.openxmlformats.org/drawingml/2006/main wavAudioFile,omitempty"`
	AudioFile     *dml.AudioFile      `xml:"http://schemas.openxmlformats.org/drawingml/2006/main audioFile,omitempty"`
	VideoFile     *dml.VideoFile      `xml:"http://schemas.openxmlformats.org/drawingml/2006/main videoFile,omitempty"`
	QuickTimeFile *dml.QuickTimeFile  `xml:"http://schemas.openxmlformats.org/drawingml/2006/main quickTimeFile,omitempty"`
	CustDataLst   *CustDataLst        `xml:"custDataLst,omitempty"`
	ExtLst        *ExtensionList      `xml:"extLst,omitempty"`
}

// UnmarshalXML captures the element's verbatim attribute list before decoding
// through the struct tags; the reflection marshaler replays it.
func (nv *NvPr) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	nv.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias NvPr
	return d.DecodeElement((*alias)(nv), &start)
}

// CustDataLst specifies customer data references (p:custDataLst).
// This is CT_CustomerDataList in the PML XSD.
type CustDataLst struct {
	CustData []*CustData `xml:"custData,omitempty"`
	Tags     *TagsData   `xml:"tags,omitempty"`
}

// CustData references a customer data part (p:custData).
// This is CT_CustomerData in the PML XSD.
type CustData struct {
	Id string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
}

// TagsData references a tags part (p:tags).
// This is CT_TagsData in the PML XSD.
type TagsData struct {
	Id string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
}

// Placeholder specifies placeholder information.
// This is CT_Placeholder in the PML XSD.
type Placeholder struct {
	Type            string          `xml:"type,attr,omitempty"`
	Orient          string          `xml:"orient,attr,omitempty"`
	Sz              string          `xml:"sz,attr,omitempty"`
	Idx             uint32          `xml:"idx,attr,omitempty"`
	HasCustomPrompt bool            `xml:"hasCustomPrompt,attr,omitempty"`
	CapturedAttrs   []xmlb.RootAttr `xml:"-"` // verbatim source attrs; see common/xml.CaptureAttrs
}

// UnmarshalXML captures the element's verbatim attribute list (source
// attribute order and any unmodeled attributes) before decoding through the
// struct tags; the reflection marshaler replays it.
func (ph *Placeholder) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	ph.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias Placeholder
	return d.DecodeElement((*alias)(ph), &start)
}
