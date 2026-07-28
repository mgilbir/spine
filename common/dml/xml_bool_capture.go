// This file closes the xsd:boolean lexical-form class for DrawingML (C587).
//
// XSD gives xsd:boolean four lexical forms — "true", "false", "1", "0" — that
// are all semantically equal. A Go bool (or *bool) records only the value, so a
// producer that wrote rotWithShape="true" came back as rotWithShape="1": the
// document was unchanged, yet the part could not be reproduced byte-for-byte,
// and the raw-token comparator that lets pretty-printed parts fall back to
// their source bytes (xmlb.EqualIgnoringIndent) refuses to equate the two —
// correctly, since a token comparator must not embed schema knowledge about
// which attributes are booleans. The fix therefore belongs in the model:
// capture the source spelling at parse and replay it while the value is
// unmodified.
//
// The mechanism is the package-wide CapturedAttrs convention rather than a
// BoolLex-style scalar (xlsx/internal/oxml/lexical.go). Both preserve the
// lexeme, but a per-field type only fixes the one field it is applied to and
// changes the public field type of a shared model, while CapturedAttrs is
// already understood by the reflection marshaler and fixes every attribute on
// the element at once — booleans, integer spellings, unmodeled attributes, and
// the producer's attribute order. Replay is exactly
// xmlb.ReplayCapturedAttrsClearing:
//
//   - unmodified model, differing lexeme → boolLexEquivalent holds, so the
//     captured spelling is emitted ("true" stays "true");
//   - caller stored the other boolean → the lexemes are not equivalent, so the
//     model's "1"/"0" wins;
//   - caller reset a value-typed bool to false → the field is omitempty-cleared
//     and replay drops the captured attribute when it denoted true (C586);
//   - a nil *bool is never reported as cleared, which is what keeps the
//     XSD default-TRUE attributes modeled as pointers (grow, useA,
//     preferRelativeResize) safe: "we hold no value" must not delete a
//     source attribute, because absent means TRUE for those and deleting one
//     would invert its meaning;
//   - no capture (programmatically built values) → canonical "1"/"0" emission,
//     exactly as before.
//
// The types instrumented here are every struct in this package that carries a
// bool or *bool attribute and did not already have a capture. Sibling packages
// common/dml/chart and common/dml/diagram are deliberately excluded; see
// boolCaptureExemptTypes in capture_coverage_test.go for why.

package dml

import (
	"encoding/xml"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// UnmarshalXML captures the element's verbatim attribute list (boolean lexical
// forms like rotWithShape="true") before decoding through the struct tags; the
// reflection marshaler replays it.
func (v *BlipFillXML) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	v.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias BlipFillXML
	return d.DecodeElement((*alias)(v), &start)
}

// UnmarshalXML captures the element's verbatim attribute list; see
// BlipFillXML.UnmarshalXML.
func (v *BlipFill) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	v.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias BlipFill
	return d.DecodeElement((*alias)(v), &start)
}

// UnmarshalXML captures the element's verbatim attribute list; see
// BlipFillXML.UnmarshalXML.
func (v *GradFill) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	v.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias GradFill
	return d.DecodeElement((*alias)(v), &start)
}

// UnmarshalXML captures the element's verbatim attribute list; see
// BlipFillXML.UnmarshalXML.
func (v *Lin) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	v.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias Lin
	return d.DecodeElement((*alias)(v), &start)
}

// UnmarshalXML captures the element's verbatim attribute list; see
// BlipFillXML.UnmarshalXML.
func (v *BlurXML) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	v.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias BlurXML
	return d.DecodeElement((*alias)(v), &start)
}

// UnmarshalXML captures the element's verbatim attribute list; see
// BlipFillXML.UnmarshalXML.
func (v *ClrChange) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	v.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias ClrChange
	return d.DecodeElement((*alias)(v), &start)
}

// UnmarshalXML captures the element's verbatim attribute list; see
// BlipFillXML.UnmarshalXML.
func (v *CNvPicPr) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	v.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias CNvPicPr
	return d.DecodeElement((*alias)(v), &start)
}

// UnmarshalXML captures the element's verbatim attribute list; see
// BlipFillXML.UnmarshalXML.
func (v *NvAudioPr) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	v.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias NvAudioPr
	return d.DecodeElement((*alias)(v), &start)
}

// UnmarshalXML captures the element's verbatim attribute list; see
// BlipFillXML.UnmarshalXML.
func (v *CNvAudioPr) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	v.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias CNvAudioPr
	return d.DecodeElement((*alias)(v), &start)
}

// UnmarshalXML captures the element's verbatim attribute list; see
// BlipFillXML.UnmarshalXML.
func (v *OleObject) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	v.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias OleObject
	return d.DecodeElement((*alias)(v), &start)
}

// UnmarshalXML captures the element's verbatim attribute list; see
// BlipFillXML.UnmarshalXML.
func (v *PMLNvPr) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	v.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias PMLNvPr
	return d.DecodeElement((*alias)(v), &start)
}

// UnmarshalXML captures the element's verbatim attribute list; see
// BlipFillXML.UnmarshalXML.
func (v *TextRtl) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	v.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias TextRtl
	return d.DecodeElement((*alias)(v), &start)
}

// UnmarshalXML captures the element's verbatim attribute list; see
// BlipFillXML.UnmarshalXML.
func (v *Wsp) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	v.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias Wsp
	return d.DecodeElement((*alias)(v), &start)
}

// UnmarshalXML captures the element's verbatim attribute list; see
// BlipFillXML.UnmarshalXML.
func (v *WPAnchor) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	v.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias WPAnchor
	return d.DecodeElement((*alias)(v), &start)
}

// UnmarshalXML captures the element's verbatim attribute list; see
// BlipFillXML.UnmarshalXML.
func (v *WPWrapPolygon) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	v.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias WPWrapPolygon
	return d.DecodeElement((*alias)(v), &start)
}

// UnmarshalXML captures the element's verbatim attribute list; see
// BlipFillXML.UnmarshalXML.
func (v *XDRSp) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	v.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias XDRSp
	return d.DecodeElement((*alias)(v), &start)
}

// UnmarshalXML captures the element's verbatim attribute list; see
// BlipFillXML.UnmarshalXML.
func (v *XDRPic) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	v.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias XDRPic
	return d.DecodeElement((*alias)(v), &start)
}

// UnmarshalXML captures the element's verbatim attribute list; see
// BlipFillXML.UnmarshalXML.
func (v *XDRCxnSp) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	v.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias XDRCxnSp
	return d.DecodeElement((*alias)(v), &start)
}

// UnmarshalXML captures the element's verbatim attribute list; see
// BlipFillXML.UnmarshalXML.
func (v *XDRClientData) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	v.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias XDRClientData
	return d.DecodeElement((*alias)(v), &start)
}

// UnmarshalXML captures the element's verbatim attribute list; see
// BlipFillXML.UnmarshalXML.
func (v *CDRSp) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	v.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias CDRSp
	return d.DecodeElement((*alias)(v), &start)
}

// UnmarshalXML captures the element's verbatim attribute list; see
// BlipFillXML.UnmarshalXML.
func (v *CDRPic) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	v.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias CDRPic
	return d.DecodeElement((*alias)(v), &start)
}

// UnmarshalXML captures the element's verbatim attribute list; see
// BlipFillXML.UnmarshalXML.
func (v *CDRCxnSp) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	v.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias CDRCxnSp
	return d.DecodeElement((*alias)(v), &start)
}
