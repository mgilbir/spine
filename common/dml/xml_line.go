// This file provides DrawingML XML line/stroke types from dml-main.xsd.

package dml

import (
	"encoding/xml"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// Ln represents CT_LineProperties (a:ln)
type Ln struct {
	W             *int64          `xml:"w,attr,omitempty"`
	Cap           string          `xml:"cap,attr,omitempty"`
	Cmpd          string          `xml:"cmpd,attr,omitempty"`
	Algn          string          `xml:"algn,attr,omitempty"`
	NoFill        *NoFillXML      `xml:"http://schemas.openxmlformats.org/drawingml/2006/main noFill,omitempty"`
	SolidFill     *SolidFill      `xml:"http://schemas.openxmlformats.org/drawingml/2006/main solidFill,omitempty"`
	GradFill      *GradFill       `xml:"http://schemas.openxmlformats.org/drawingml/2006/main gradFill,omitempty"`
	PattFill      *PattFill       `xml:"http://schemas.openxmlformats.org/drawingml/2006/main pattFill,omitempty"`
	PrstDash      *PrstDash       `xml:"http://schemas.openxmlformats.org/drawingml/2006/main prstDash,omitempty"`
	CustDash      *CustDash       `xml:"http://schemas.openxmlformats.org/drawingml/2006/main custDash,omitempty"`
	Round         *Round          `xml:"http://schemas.openxmlformats.org/drawingml/2006/main round,omitempty"`
	Bevel         *Bevel          `xml:"http://schemas.openxmlformats.org/drawingml/2006/main bevel,omitempty"`
	Miter         *Miter          `xml:"http://schemas.openxmlformats.org/drawingml/2006/main miter,omitempty"`
	HeadEnd       *LineEnd        `xml:"http://schemas.openxmlformats.org/drawingml/2006/main headEnd,omitempty"`
	TailEnd       *LineEnd        `xml:"http://schemas.openxmlformats.org/drawingml/2006/main tailEnd,omitempty"`
	ExtLst        *ExtLst         `xml:"http://schemas.openxmlformats.org/drawingml/2006/main extLst,omitempty"`
	CapturedAttrs []xmlb.RootAttr `xml:"-"` // verbatim source attrs; see common/xml.CaptureAttrs
}

// UnmarshalXML captures the element's verbatim attribute list (source
// attribute order and any unmodeled attributes) before decoding through the
// struct tags; the reflection marshaler replays it.
func (ln *Ln) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	ln.CapturedAttrs = xmlb.CaptureAttrs(start.Attr)
	type alias Ln
	return d.DecodeElement((*alias)(ln), &start)
}

// PrstDash represents CT_PresetLineDashProperties (a:prstDash)
type PrstDash struct {
	Val string `xml:"val,attr,omitempty"`
}

// CustDash represents CT_DashStopList (a:custDash)
type CustDash struct {
	Ds []*Ds `xml:"http://schemas.openxmlformats.org/drawingml/2006/main ds,omitempty"`
}

// Ds represents CT_DashStop (a:ds)
type Ds struct {
	D  int32 `xml:"d,attr"`
	Sp int32 `xml:"sp,attr"`
}

// Round represents CT_LineJoinRound (a:round)
type Round struct{}

// Bevel represents CT_LineJoinBevel (a:bevel)
type Bevel struct {
	// CapturedAttrs preserves unmodeled attributes; see common/xml.CaptureAttrs.
	CapturedAttrs []xmlb.RootAttr `xml:"-"`
}

// UnmarshalXML captures the element's verbatim attribute list.
func (bv2 *Bevel) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	bv2.CapturedAttrs = xmlb.CaptureAttrs(start.Attr)
	type alias Bevel
	return d.DecodeElement((*alias)(bv2), &start)
}

// Miter represents CT_LineJoinMiterProperties (a:miter)
type Miter struct {
	Lim           int32           `xml:"lim,attr,omitempty"`
	CapturedAttrs []xmlb.RootAttr `xml:"-"` // verbatim source attrs; see common/xml.CaptureAttrs
}

// UnmarshalXML captures the element's verbatim attribute list (source
// attribute order and any unmodeled attributes) before decoding through the
// struct tags; the reflection marshaler replays it.
func (mi *Miter) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	mi.CapturedAttrs = xmlb.CaptureAttrs(start.Attr)
	type alias Miter
	return d.DecodeElement((*alias)(mi), &start)
}

// LineEnd represents CT_LineEndProperties (a:headEnd, a:tailEnd)
type LineEnd struct {
	Type          string          `xml:"type,attr,omitempty"`
	W             string          `xml:"w,attr,omitempty"`
	Len           string          `xml:"len,attr,omitempty"`
	CapturedAttrs []xmlb.RootAttr `xml:"-"` // verbatim source attrs; see common/xml.CaptureAttrs
}

// UnmarshalXML captures the element's verbatim attribute list (source
// attribute order and any unmodeled attributes) before decoding through the
// struct tags; the reflection marshaler replays it.
func (le *LineEnd) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	le.CapturedAttrs = xmlb.CaptureAttrs(start.Attr)
	type alias LineEnd
	return d.DecodeElement((*alias)(le), &start)
}

// LnRef represents CT_StyleMatrixReference (a:lnRef)
type LnRef struct {
	Idx       uint32              `xml:"idx,attr"`
	ScrgbClr  *ScRgbClr           `xml:"http://schemas.openxmlformats.org/drawingml/2006/main scrgbClr,omitempty"`
	SrgbClr   *SrgbClr            `xml:"http://schemas.openxmlformats.org/drawingml/2006/main srgbClr,omitempty"`
	HslClr    *HslClr             `xml:"http://schemas.openxmlformats.org/drawingml/2006/main hslClr,omitempty"`
	SysClr    *SystemClr          `xml:"http://schemas.openxmlformats.org/drawingml/2006/main sysClr,omitempty"`
	SchemeClr *SchemeClrTransform `xml:"http://schemas.openxmlformats.org/drawingml/2006/main schemeClr,omitempty"`
	PrstClr   *PrstClr            `xml:"http://schemas.openxmlformats.org/drawingml/2006/main prstClr,omitempty"`
}

// FillRef represents CT_StyleMatrixReference (a:fillRef)
type FillRef struct {
	Idx       uint32              `xml:"idx,attr"`
	ScrgbClr  *ScRgbClr           `xml:"http://schemas.openxmlformats.org/drawingml/2006/main scrgbClr,omitempty"`
	SrgbClr   *SrgbClr            `xml:"http://schemas.openxmlformats.org/drawingml/2006/main srgbClr,omitempty"`
	HslClr    *HslClr             `xml:"http://schemas.openxmlformats.org/drawingml/2006/main hslClr,omitempty"`
	SysClr    *SystemClr          `xml:"http://schemas.openxmlformats.org/drawingml/2006/main sysClr,omitempty"`
	SchemeClr *SchemeClrTransform `xml:"http://schemas.openxmlformats.org/drawingml/2006/main schemeClr,omitempty"`
	PrstClr   *PrstClr            `xml:"http://schemas.openxmlformats.org/drawingml/2006/main prstClr,omitempty"`
}

// StyleEffectRef represents CT_StyleMatrixReference (a:effectRef) for style references
type StyleEffectRef struct {
	Idx       uint32              `xml:"idx,attr"`
	ScrgbClr  *ScRgbClr           `xml:"http://schemas.openxmlformats.org/drawingml/2006/main scrgbClr,omitempty"`
	SrgbClr   *SrgbClr            `xml:"http://schemas.openxmlformats.org/drawingml/2006/main srgbClr,omitempty"`
	HslClr    *HslClr             `xml:"http://schemas.openxmlformats.org/drawingml/2006/main hslClr,omitempty"`
	SysClr    *SystemClr          `xml:"http://schemas.openxmlformats.org/drawingml/2006/main sysClr,omitempty"`
	SchemeClr *SchemeClrTransform `xml:"http://schemas.openxmlformats.org/drawingml/2006/main schemeClr,omitempty"`
	PrstClr   *PrstClr            `xml:"http://schemas.openxmlformats.org/drawingml/2006/main prstClr,omitempty"`
}

// FontRef represents CT_FontReference (a:fontRef)
type FontRef struct {
	Idx       string              `xml:"idx,attr"`
	ScrgbClr  *ScRgbClr           `xml:"http://schemas.openxmlformats.org/drawingml/2006/main scrgbClr,omitempty"`
	SrgbClr   *SrgbClr            `xml:"http://schemas.openxmlformats.org/drawingml/2006/main srgbClr,omitempty"`
	HslClr    *HslClr             `xml:"http://schemas.openxmlformats.org/drawingml/2006/main hslClr,omitempty"`
	SysClr    *SystemClr          `xml:"http://schemas.openxmlformats.org/drawingml/2006/main sysClr,omitempty"`
	SchemeClr *SchemeClrTransform `xml:"http://schemas.openxmlformats.org/drawingml/2006/main schemeClr,omitempty"`
	PrstClr   *PrstClr            `xml:"http://schemas.openxmlformats.org/drawingml/2006/main prstClr,omitempty"`
}
