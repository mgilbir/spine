// This file provides DrawingML XML 3D types from dml-main.xsd.

package dml

import (
	"encoding/xml"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// Scene3d represents CT_Scene3D (a:scene3d)
type Scene3d struct {
	Camera   *Camera   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main camera,omitempty"`
	LightRig *LightRig `xml:"http://schemas.openxmlformats.org/drawingml/2006/main lightRig,omitempty"`
	Backdrop *Backdrop `xml:"http://schemas.openxmlformats.org/drawingml/2006/main backdrop,omitempty"`
}

// Camera represents CT_Camera (a:camera). zoom is ST_PositivePercentage, so it
// is a Percentage (an int32 rejects the transitional "50%" form); fov is
// ST_FOVAngle — an angle in 60000ths of a degree, not a percentage — and stays
// an integer. CapturedAttrs keeps an explicit zoom="0", which omitempty would
// otherwise drop and silently restore to the 100000 default.
type Camera struct {
	Prst          string          `xml:"prst,attr"`
	Fov           int32           `xml:"fov,attr,omitempty"`
	Zoom          Percentage      `xml:"zoom,attr,omitempty"`
	Rot           *Rot3d          `xml:"http://schemas.openxmlformats.org/drawingml/2006/main rot,omitempty"`
	CapturedAttrs []xmlb.RootAttr `xml:"-"` // verbatim source attrs; see common/xml.CaptureAttrs
}

// UnmarshalXML captures the element's verbatim attribute list (source
// attribute order, unmodeled attributes, explicit zero values) before decoding
// through the struct tags; the reflection marshaler replays it.
func (cam *Camera) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	cam.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias Camera
	return d.DecodeElement((*alias)(cam), &start)
}

// LightRig represents CT_LightRig (a:lightRig)
type LightRig struct {
	Rig string `xml:"rig,attr"`
	Dir string `xml:"dir,attr"`
	Rot *Rot3d `xml:"http://schemas.openxmlformats.org/drawingml/2006/main rot,omitempty"`
}

// Backdrop represents CT_Backdrop (a:backdrop)
type Backdrop struct {
	Anchor *Point3d  `xml:"http://schemas.openxmlformats.org/drawingml/2006/main anchor,omitempty"`
	Norm   *Vector3d `xml:"http://schemas.openxmlformats.org/drawingml/2006/main norm,omitempty"`
	Up     *Vector3d `xml:"http://schemas.openxmlformats.org/drawingml/2006/main up,omitempty"`
}

// Rot3d represents CT_SphereCoords (a:rot)
type Rot3d struct {
	Lat int32 `xml:"lat,attr"`
	Lon int32 `xml:"lon,attr"`
	Rev int32 `xml:"rev,attr"`
}

// Point3d represents CT_Point3D (a:anchor)
type Point3d struct {
	X int64 `xml:"x,attr"`
	Y int64 `xml:"y,attr"`
	Z int64 `xml:"z,attr"`
}

// Vector3d represents CT_Vector3D (a:norm, a:up)
type Vector3d struct {
	Dx int64 `xml:"dx,attr"`
	Dy int64 `xml:"dy,attr"`
	Dz int64 `xml:"dz,attr"`
}

// Sp3d represents CT_Shape3D (a:sp3d)
type Sp3d struct {
	Z            int64        `xml:"z,attr,omitempty"`
	ExtrusionH   int64        `xml:"extrusionH,attr,omitempty"`
	ContourW     int64        `xml:"contourW,attr,omitempty"`
	PrstMaterial string       `xml:"prstMaterial,attr,omitempty"`
	BevelT       *Bevel3d     `xml:"http://schemas.openxmlformats.org/drawingml/2006/main bevelT,omitempty"`
	BevelB       *Bevel3d     `xml:"http://schemas.openxmlformats.org/drawingml/2006/main bevelB,omitempty"`
	ExtrusionClr *ColorChoice `xml:"http://schemas.openxmlformats.org/drawingml/2006/main extrusionClr,omitempty"`
	ContourClr   *ColorChoice `xml:"http://schemas.openxmlformats.org/drawingml/2006/main contourClr,omitempty"`
}

// Bevel3d represents CT_Bevel (a:bevelT, a:bevelB)
type Bevel3d struct {
	W             int64           `xml:"w,attr,omitempty"`
	H             int64           `xml:"h,attr,omitempty"`
	Prst          string          `xml:"prst,attr,omitempty"`
	CapturedAttrs []xmlb.RootAttr `xml:"-"` // verbatim source attrs; see common/xml.CaptureAttrs
}

// UnmarshalXML captures the element's verbatim attribute list (source
// attribute order and any unmodeled attributes) before decoding through the
// struct tags; the reflection marshaler replays it.
func (b3 *Bevel3d) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	b3.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias Bevel3d
	return d.DecodeElement((*alias)(b3), &start)
}
