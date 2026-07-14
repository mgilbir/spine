// Package oxml provides internal XML types for WordprocessingML (WML) documents.
package oxml

import (
	"encoding/xml"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// CT_Empty represents a WML empty element (e.g., <w:noProof/>).
type CT_Empty struct{}

// CT_OnOff represents an on/off toggle element (e.g., <w:b/>, <w:b w:val="0"/>).
// When the element is present without a val attribute, the value is true.
type CT_OnOff struct {
	Val *string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr,omitempty"`
	// CapturedEmptyTag records the source's per-instance "/>" vs " />" form.
	CapturedEmptyTag xmlb.EmptyTagStyle `xml:"-"`
	// CapturedAttrs preserves the verbatim attribute rendering.
	CapturedAttrs []xmlb.RootAttr `xml:"-"`
}

// UnmarshalXML captures the element's empty-tag style and verbatim attribute
// list before decoding through the struct tags.
func (o *CT_OnOff) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	o.CapturedEmptyTag = xmlb.CaptureEmptyTagStyle(d)
	o.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias CT_OnOff
	return d.DecodeElement((*alias)(o), &start)
}

// IsOn returns whether the toggle is on. Present without val means true,
// val="" means true, val="1"/"true"/"on" means true.
func (o *CT_OnOff) IsOn() bool {
	if o == nil {
		return false
	}
	if o.Val == nil {
		return true // present without val = on
	}
	v := *o.Val
	return v == "" || v == "1" || v == "true" || v == "on"
}

// CT_String represents an element with a w:val string attribute.
type CT_String struct {
	// CapturedAttrs preserves the verbatim source attribute list (explicit
	// empty w:val, unmodeled attributes); replayed on marshal.
	CapturedAttrs []xmlb.RootAttr `xml:"-"`
	// CapturedEmptyTag records the source's per-instance "/>" vs " />" form.
	CapturedEmptyTag xmlb.EmptyTagStyle `xml:"-"`
	// omitempty: some CT_String-bound elements carry an optional w:val
	// (e.g. w:vMerge, where a bare <w:vMerge/> means "continue"); emitting
	// w:val="" for those would both drift bytes and change semantics.
	Val string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr,omitempty"`
}

// UnmarshalXML captures the element's verbatim attribute list before decoding
// through the struct tags; the reflection marshaler replays it.
func (cs *CT_String) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	cs.CapturedEmptyTag = xmlb.CaptureEmptyTagStyle(d)
	cs.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias CT_String
	return d.DecodeElement((*alias)(cs), &start)
}

// CT_DecimalNumber represents an element with a w:val integer attribute.
type CT_DecimalNumber struct {
	Val int `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr"`
	// CapturedEmptyTag records the source's empty-element form (per-instance
	// "/>" vs " />" spacing survives a spaced part-level style).
	CapturedEmptyTag xmlb.EmptyTagStyle `xml:"-"`
	// CapturedAttrs preserves the verbatim attribute rendering (quote style,
	// unmodeled attributes).
	CapturedAttrs []xmlb.RootAttr `xml:"-"`
}

// UnmarshalXML captures the element's empty-tag style and verbatim attribute
// list before decoding through the struct tags.
func (dn *CT_DecimalNumber) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	dn.CapturedEmptyTag = xmlb.CaptureEmptyTagStyle(d)
	dn.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias CT_DecimalNumber
	return d.DecodeElement((*alias)(dn), &start)
}

// CT_UnsignedDecimalNumber represents an element with a w:val unsigned integer attribute.
type CT_UnsignedDecimalNumber struct {
	Val uint64 `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr"`
	// CapturedEmptyTag records the source's per-instance "/>" vs " />" form.
	CapturedEmptyTag xmlb.EmptyTagStyle `xml:"-"`
}

// UnmarshalXML captures the element's empty-tag style before decoding
// through the struct tags.
func (v *CT_UnsignedDecimalNumber) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	v.CapturedEmptyTag = xmlb.CaptureEmptyTagStyle(d)
	type alias CT_UnsignedDecimalNumber
	return d.DecodeElement((*alias)(v), &start)
}

// CT_TwipsMeasure represents a measurement in twips (1/20 of a point).
type CT_TwipsMeasure struct {
	Val string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr"`
}

// CT_HpsMeasure represents a measurement in half-points.
type CT_HpsMeasure struct {
	Val string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr"`
	// CapturedEmptyTag records the source's per-instance "/>" vs " />" form.
	CapturedEmptyTag xmlb.EmptyTagStyle `xml:"-"`
}

// UnmarshalXML captures the element's empty-tag style before decoding
// through the struct tags.
func (v *CT_HpsMeasure) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	v.CapturedEmptyTag = xmlb.CaptureEmptyTagStyle(d)
	type alias CT_HpsMeasure
	return d.DecodeElement((*alias)(v), &start)
}

// CT_SignedTwipsMeasure represents a signed measurement in twips.
type CT_SignedTwipsMeasure struct {
	Val string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr"`
	// CapturedEmptyTag records the source's per-instance "/>" vs " />" form.
	CapturedEmptyTag xmlb.EmptyTagStyle `xml:"-"`
}

// UnmarshalXML captures the element's empty-tag style before decoding
// through the struct tags.
func (v *CT_SignedTwipsMeasure) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	v.CapturedEmptyTag = xmlb.CaptureEmptyTagStyle(d)
	type alias CT_SignedTwipsMeasure
	return d.DecodeElement((*alias)(v), &start)
}

// CT_SignedHpsMeasure represents a signed half-point measurement.
type CT_SignedHpsMeasure struct {
	Val string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr"`
	// CapturedEmptyTag records the source's per-instance "/>" vs " />" form.
	CapturedEmptyTag xmlb.EmptyTagStyle `xml:"-"`
}

// UnmarshalXML captures the element's empty-tag style before decoding
// through the struct tags.
func (v *CT_SignedHpsMeasure) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	v.CapturedEmptyTag = xmlb.CaptureEmptyTagStyle(d)
	type alias CT_SignedHpsMeasure
	return d.DecodeElement((*alias)(v), &start)
}

// CT_LongHexNumber represents a 4-byte hex number (e.g., rsidR).
type CT_LongHexNumber struct {
	Val string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr"`
}

// CT_TextScale represents text scaling percentage.
type CT_TextScale struct {
	Val string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr"`
	// CapturedEmptyTag records the source's per-instance "/>" vs " />" form.
	CapturedEmptyTag xmlb.EmptyTagStyle `xml:"-"`
}

// UnmarshalXML captures the element's empty-tag style before decoding
// through the struct tags.
func (v *CT_TextScale) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	v.CapturedEmptyTag = xmlb.CaptureEmptyTagStyle(d)
	type alias CT_TextScale
	return d.DecodeElement((*alias)(v), &start)
}

// CT_Highlight represents a text highlight color.
type CT_Highlight struct {
	Val string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr"`
	// CapturedEmptyTag records the source's per-instance "/>" vs " />" form.
	CapturedEmptyTag xmlb.EmptyTagStyle `xml:"-"`
}

// UnmarshalXML captures the element's empty-tag style before decoding
// through the struct tags.
func (v *CT_Highlight) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	v.CapturedEmptyTag = xmlb.CaptureEmptyTagStyle(d)
	type alias CT_Highlight
	return d.DecodeElement((*alias)(v), &start)
}

// CT_Color represents a color value with optional theme color.
type CT_Color struct {
	Val           string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr"`
	ThemeColor    string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main themeColor,attr,omitempty"`
	ThemeTint     string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main themeTint,attr,omitempty"`
	ThemeShade    string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main themeShade,attr,omitempty"`
	CapturedAttrs []xmlb.RootAttr `xml:"-"` // verbatim source attrs; see common/xml.CaptureAttrs
	// CapturedEmptyTag records the source's per-instance "/>" vs " />" form.
	CapturedEmptyTag xmlb.EmptyTagStyle `xml:"-"`
}

// UnmarshalXML captures the element's verbatim attribute list (source
// attribute order and any unmodeled attributes) before decoding through the
// struct tags; the reflection marshaler replays it.
func (clr *CT_Color) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	clr.CapturedEmptyTag = xmlb.CaptureEmptyTagStyle(d)
	clr.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias CT_Color
	return d.DecodeElement((*alias)(clr), &start)
}

// CT_Border represents a border definition. Field order follows Word's
// attribute emission order (val, sz, space, color, theme*), not the XSD.
type CT_Border struct {
	Val           string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr"`
	Sz            string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main sz,attr,omitempty"`
	Space         string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main space,attr,omitempty"`
	Color         string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main color,attr,omitempty"`
	ThemeColor    string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main themeColor,attr,omitempty"`
	ThemeTint     string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main themeTint,attr,omitempty"`
	ThemeShade    string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main themeShade,attr,omitempty"`
	Shadow        string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main shadow,attr,omitempty"`
	Frame         string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main frame,attr,omitempty"`
	CapturedAttrs []xmlb.RootAttr `xml:"-"` // verbatim source attrs; see common/xml.CaptureAttrs
	// CapturedEmptyTag records the source's per-instance "/>" vs " />" form.
	CapturedEmptyTag xmlb.EmptyTagStyle `xml:"-"`
}

// UnmarshalXML captures the element's verbatim attribute list (source
// attribute order and any unmodeled attributes) before decoding through the
// struct tags; the reflection marshaler replays it.
func (bdr *CT_Border) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	bdr.CapturedEmptyTag = xmlb.CaptureEmptyTagStyle(d)
	bdr.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias CT_Border
	return d.DecodeElement((*alias)(bdr), &start)
}

// CT_Shd represents shading properties.
type CT_Shd struct {
	Val            string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr"`
	Color          string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main color,attr,omitempty"`
	Fill           string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main fill,attr,omitempty"`
	ThemeFill      string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main themeFill,attr,omitempty"`
	ThemeFillTint  string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main themeFillTint,attr,omitempty"`
	ThemeFillShade string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main themeFillShade,attr,omitempty"`
	ThemeColor     string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main themeColor,attr,omitempty"`
	ThemeTint      string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main themeTint,attr,omitempty"`
	ThemeShade     string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main themeShade,attr,omitempty"`
	CapturedAttrs  []xmlb.RootAttr `xml:"-"` // verbatim source attrs; see common/xml.CaptureAttrs
	// CapturedEmptyTag records the source's per-instance "/>" vs " />" form.
	CapturedEmptyTag xmlb.EmptyTagStyle `xml:"-"`
}

// UnmarshalXML captures the element's verbatim attribute list (source
// attribute order and any unmodeled attributes) before decoding through the
// struct tags; the reflection marshaler replays it.
func (sh *CT_Shd) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	sh.CapturedEmptyTag = xmlb.CaptureEmptyTagStyle(d)
	sh.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias CT_Shd
	return d.DecodeElement((*alias)(sh), &start)
}

// CT_Underline represents underline formatting.
type CT_Underline struct {
	Val           string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr,omitempty"`
	Color         string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main color,attr,omitempty"`
	ThemeColor    string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main themeColor,attr,omitempty"`
	CapturedAttrs []xmlb.RootAttr `xml:"-"` // verbatim source attrs; see common/xml.CaptureAttrs
	// CapturedEmptyTag records the source's per-instance "/>" vs " />" form.
	CapturedEmptyTag xmlb.EmptyTagStyle `xml:"-"`
}

// UnmarshalXML captures the element's verbatim attribute list (source
// attribute order and any unmodeled attributes) before decoding through the
// struct tags; the reflection marshaler replays it.
func (u *CT_Underline) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	u.CapturedEmptyTag = xmlb.CaptureEmptyTagStyle(d)
	u.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias CT_Underline
	return d.DecodeElement((*alias)(u), &start)
}

// CT_Lang represents language identification.
type CT_Lang struct {
	Val           string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr,omitempty"`
	EastAsia      string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main eastAsia,attr,omitempty"`
	Bidi          string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main bidi,attr,omitempty"`
	CapturedAttrs []xmlb.RootAttr `xml:"-"` // verbatim source attrs; see common/xml.CaptureAttrs
	// CapturedEmptyTag records the source's per-instance "/>" vs " />" form.
	CapturedEmptyTag xmlb.EmptyTagStyle `xml:"-"`
}

// UnmarshalXML captures the element's verbatim attribute list (source
// attribute order and any unmodeled attributes) before decoding through the
// struct tags; the reflection marshaler replays it.
func (lg *CT_Lang) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	lg.CapturedEmptyTag = xmlb.CaptureEmptyTagStyle(d)
	lg.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias CT_Lang
	return d.DecodeElement((*alias)(lg), &start)
}

// CT_Fonts represents font specifications. Field order follows Word's
// attribute emission order: each script slot (ascii, eastAsia, hAnsi, cs)
// with its theme twin adjacent, and hint last.
type CT_Fonts struct {
	Ascii         string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main ascii,attr,omitempty"`
	AsciiTheme    string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main asciiTheme,attr,omitempty"`
	EastAsia      string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main eastAsia,attr,omitempty"`
	EastAsiaTheme string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main eastAsiaTheme,attr,omitempty"`
	HAnsi         string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main hAnsi,attr,omitempty"`
	HAnsiTheme    string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main hAnsiTheme,attr,omitempty"`
	Cs            string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main cs,attr,omitempty"`
	CsTheme       string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main cstheme,attr,omitempty"`
	Hint          string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main hint,attr,omitempty"`
	CapturedAttrs []xmlb.RootAttr `xml:"-"` // verbatim source attrs; see common/xml.CaptureAttrs
	// CapturedEmptyTag records the source's per-instance "/>" vs " />" form.
	CapturedEmptyTag xmlb.EmptyTagStyle `xml:"-"`
}

// UnmarshalXML captures the element's verbatim attribute list (source
// attribute order and any unmodeled attributes) before decoding through the
// struct tags; the reflection marshaler replays it.
func (f *CT_Fonts) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	f.CapturedEmptyTag = xmlb.CaptureEmptyTagStyle(d)
	f.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias CT_Fonts
	return d.DecodeElement((*alias)(f), &start)
}

// CT_FitText represents text fitting properties.
type CT_FitText struct {
	Val string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr"`
	ID  string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main id,attr,omitempty"`
	// CapturedEmptyTag records the source's per-instance "/>" vs " />" form.
	CapturedEmptyTag xmlb.EmptyTagStyle `xml:"-"`
}

// UnmarshalXML captures the element's empty-tag style before decoding
// through the struct tags.
func (v *CT_FitText) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	v.CapturedEmptyTag = xmlb.CaptureEmptyTagStyle(d)
	type alias CT_FitText
	return d.DecodeElement((*alias)(v), &start)
}

// CT_Em represents emphasis mark type.
type CT_Em struct {
	Val string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr"`
	// CapturedEmptyTag records the source's per-instance "/>" vs " />" form.
	CapturedEmptyTag xmlb.EmptyTagStyle `xml:"-"`
}

// UnmarshalXML captures the element's empty-tag style before decoding
// through the struct tags.
func (v *CT_Em) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	v.CapturedEmptyTag = xmlb.CaptureEmptyTagStyle(d)
	type alias CT_Em
	return d.DecodeElement((*alias)(v), &start)
}

// CT_VerticalAlignRun represents vertical character alignment.
type CT_VerticalAlignRun struct {
	Val string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr"`
	// CapturedEmptyTag records the source's per-instance "/>" vs " />" form.
	CapturedEmptyTag xmlb.EmptyTagStyle `xml:"-"`
}

// UnmarshalXML captures the element's empty-tag style before decoding
// through the struct tags.
func (v *CT_VerticalAlignRun) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	v.CapturedEmptyTag = xmlb.CaptureEmptyTagStyle(d)
	type alias CT_VerticalAlignRun
	return d.DecodeElement((*alias)(v), &start)
}

// CT_TblWidth represents table width/measurement.
type CT_TblWidth struct {
	W             string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main w,attr,omitempty"`
	Type          string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main type,attr,omitempty"`
	CapturedAttrs []xmlb.RootAttr `xml:"-"` // verbatim source attrs; see common/xml.CaptureAttrs
	// CapturedEmptyTag records the source's per-instance "/>" vs " />" form.
	CapturedEmptyTag xmlb.EmptyTagStyle `xml:"-"`
}

// UnmarshalXML captures the element's verbatim attribute list (source
// attribute order and any unmodeled attributes) and empty-tag form before
// decoding through the struct tags; the reflection marshaler replays them.
func (tw *CT_TblWidth) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	tw.CapturedEmptyTag = xmlb.CaptureEmptyTagStyle(d)
	tw.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias CT_TblWidth
	return d.DecodeElement((*alias)(tw), &start)
}

// CT_PBdr represents paragraph borders.
type CT_PBdr struct {
	Top     *CT_Border `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main top,omitempty"`
	Left    *CT_Border `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main left,omitempty"`
	Bottom  *CT_Border `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main bottom,omitempty"`
	Right   *CT_Border `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main right,omitempty"`
	Between *CT_Border `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main between,omitempty"`
	Bar     *CT_Border `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main bar,omitempty"`
	// CapturedEmptyTag records how an empty w:pBdr was written in the source.
	CapturedEmptyTag xmlb.EmptyTagStyle `xml:"-"`
}

// UnmarshalXML captures the element's empty-tag style before decoding
// through the struct tags.
func (v *CT_PBdr) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	v.CapturedEmptyTag = xmlb.CaptureEmptyTagStyle(d)
	type alias CT_PBdr
	return d.DecodeElement((*alias)(v), &start)
}

// CT_TblBorders represents table borders.
type CT_TblBorders struct {
	Top     *CT_Border `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main top,omitempty"`
	Left    *CT_Border `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main left,omitempty"`
	Bottom  *CT_Border `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main bottom,omitempty"`
	Right   *CT_Border `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main right,omitempty"`
	InsideH *CT_Border `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main insideH,omitempty"`
	InsideV *CT_Border `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main insideV,omitempty"`
	// CapturedEmptyTag records how an empty w:tblBorders was written in the source.
	CapturedEmptyTag xmlb.EmptyTagStyle `xml:"-"`
}

// UnmarshalXML captures the element's empty-tag style before decoding
// through the struct tags.
func (v *CT_TblBorders) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	v.CapturedEmptyTag = xmlb.CaptureEmptyTagStyle(d)
	type alias CT_TblBorders
	return d.DecodeElement((*alias)(v), &start)
}

// CT_TcBorders represents table cell borders.
type CT_TcBorders struct {
	Top     *CT_Border `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main top,omitempty"`
	Left    *CT_Border `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main left,omitempty"`
	Bottom  *CT_Border `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main bottom,omitempty"`
	Right   *CT_Border `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main right,omitempty"`
	InsideH *CT_Border `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main insideH,omitempty"`
	InsideV *CT_Border `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main insideV,omitempty"`
	Tl2Br   *CT_Border `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main tl2br,omitempty"`
	Tr2Bl   *CT_Border `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main tr2bl,omitempty"`
	// CapturedEmptyTag records how an empty w:tcBorders was written in the source.
	CapturedEmptyTag xmlb.EmptyTagStyle `xml:"-"`
}

// UnmarshalXML captures the element's empty-tag style before decoding
// through the struct tags.
func (v *CT_TcBorders) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	v.CapturedEmptyTag = xmlb.CaptureEmptyTagStyle(d)
	type alias CT_TcBorders
	return d.DecodeElement((*alias)(v), &start)
}

// CT_Spacing represents paragraph spacing. Field order follows Word's
// attribute emission order: the *Lines variant precedes each twip value
// (w:beforeLines before w:before), matching the CT_Ind Chars-first pattern.
type CT_Spacing struct {
	BeforeLines       string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main beforeLines,attr,omitempty"`
	Before            string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main before,attr,omitempty"`
	BeforeAutospacing string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main beforeAutospacing,attr,omitempty"`
	AfterLines        string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main afterLines,attr,omitempty"`
	After             string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main after,attr,omitempty"`
	AfterAutospacing  string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main afterAutospacing,attr,omitempty"`
	Line              string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main line,attr,omitempty"`
	LineRule          string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main lineRule,attr,omitempty"`
	CapturedAttrs     []xmlb.RootAttr `xml:"-"` // verbatim source attrs; see common/xml.CaptureAttrs
	// CapturedEmptyTag records the source's per-instance "/>" vs " />" form.
	CapturedEmptyTag xmlb.EmptyTagStyle `xml:"-"`
}

// UnmarshalXML captures the element's verbatim attribute list (source
// attribute order and any unmodeled attributes) before decoding through the
// struct tags; the reflection marshaler replays it.
func (sp *CT_Spacing) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	sp.CapturedEmptyTag = xmlb.CaptureEmptyTagStyle(d)
	sp.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias CT_Spacing
	return d.DecodeElement((*alias)(sp), &start)
}

// CT_Ind represents paragraph indentation. Field order follows Word's
// attribute emission order: the Chars variant precedes each twip value.
type CT_Ind struct {
	LeftChars      string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main leftChars,attr,omitempty"`
	Left           string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main left,attr,omitempty"`
	RightChars     string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main rightChars,attr,omitempty"`
	Right          string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main right,attr,omitempty"`
	HangingChars   string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main hangingChars,attr,omitempty"`
	Hanging        string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main hanging,attr,omitempty"`
	FirstLineChars string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main firstLineChars,attr,omitempty"`
	FirstLine      string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main firstLine,attr,omitempty"`
	CapturedAttrs  []xmlb.RootAttr `xml:"-"` // verbatim source attrs; see common/xml.CaptureAttrs
	// CapturedEmptyTag records the source's per-instance "/>" vs " />" form.
	CapturedEmptyTag xmlb.EmptyTagStyle `xml:"-"`
}

// UnmarshalXML captures the element's verbatim attribute list (source
// attribute order and any unmodeled attributes) before decoding through the
// struct tags; the reflection marshaler replays it.
func (ind *CT_Ind) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	ind.CapturedEmptyTag = xmlb.CaptureEmptyTagStyle(d)
	ind.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias CT_Ind
	return d.DecodeElement((*alias)(ind), &start)
}

// CT_Jc represents paragraph justification.
type CT_Jc struct {
	Val string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr"`
	// CapturedEmptyTag records the source's per-instance "/>" vs " />" form.
	CapturedEmptyTag xmlb.EmptyTagStyle `xml:"-"`
}

// UnmarshalXML captures the element's empty-tag style before decoding
// through the struct tags.
func (v *CT_Jc) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	v.CapturedEmptyTag = xmlb.CaptureEmptyTagStyle(d)
	type alias CT_Jc
	return d.DecodeElement((*alias)(v), &start)
}

// CT_NumPr represents numbering properties.
type CT_NumPr struct {
	Ilvl  *CT_DecimalNumber `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main ilvl,omitempty"`
	NumId *CT_DecimalNumber `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main numId,omitempty"`
	// CapturedEmptyTag records the source's per-instance "/>" vs " />" form.
	CapturedEmptyTag xmlb.EmptyTagStyle `xml:"-"`
}

// UnmarshalXML captures the element's empty-tag style before decoding
// through the struct tags.
func (v *CT_NumPr) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	v.CapturedEmptyTag = xmlb.CaptureEmptyTagStyle(d)
	type alias CT_NumPr
	return d.DecodeElement((*alias)(v), &start)
}

// CT_Tabs represents a set of tab stops.
type CT_Tabs struct {
	Tab []CT_TabStop `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main tab"`
	// CapturedEmptyTag records the source's per-instance "/>" vs " />" form.
	CapturedEmptyTag xmlb.EmptyTagStyle `xml:"-"`
}

// UnmarshalXML captures the element's empty-tag style before decoding
// through the struct tags.
func (v *CT_Tabs) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	v.CapturedEmptyTag = xmlb.CaptureEmptyTagStyle(d)
	type alias CT_Tabs
	return d.DecodeElement((*alias)(v), &start)
}

// CT_TabStop represents a single tab stop. Field order follows Word's
// attribute emission order (val, leader, pos).
type CT_TabStop struct {
	Val           string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr"`
	Leader        string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main leader,attr,omitempty"`
	Pos           string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main pos,attr"`
	CapturedAttrs []xmlb.RootAttr `xml:"-"` // verbatim source attrs; see common/xml.CaptureAttrs
	// CapturedEmptyTag records the source's per-instance "/>" vs " />" form.
	CapturedEmptyTag xmlb.EmptyTagStyle `xml:"-"`
}

// UnmarshalXML captures the element's verbatim attribute list (source
// attribute order and any unmodeled attributes) before decoding through the
// struct tags; the reflection marshaler replays it.
func (ts *CT_TabStop) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	ts.CapturedEmptyTag = xmlb.CaptureEmptyTagStyle(d)
	ts.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias CT_TabStop
	return d.DecodeElement((*alias)(ts), &start)
}

// CT_FramePr represents frame properties.
type CT_FramePr struct {
	DropCap       string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main dropCap,attr,omitempty"`
	Lines         string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main lines,attr,omitempty"`
	W             string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main w,attr,omitempty"`
	H             string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main h,attr,omitempty"`
	VSpace        string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main vSpace,attr,omitempty"`
	HSpace        string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main hSpace,attr,omitempty"`
	Wrap          string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main wrap,attr,omitempty"`
	HAnchor       string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main hAnchor,attr,omitempty"`
	VAnchor       string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main vAnchor,attr,omitempty"`
	X             string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main x,attr,omitempty"`
	XAlign        string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main xAlign,attr,omitempty"`
	Y             string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main y,attr,omitempty"`
	YAlign        string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main yAlign,attr,omitempty"`
	HRule         string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main hRule,attr,omitempty"`
	AnchorLock    string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main anchorLock,attr,omitempty"`
	CapturedAttrs []xmlb.RootAttr `xml:"-"` // verbatim source attrs; see common/xml.CaptureAttrs
	// CapturedEmptyTag records the source's per-instance "/>" vs " />" form.
	CapturedEmptyTag xmlb.EmptyTagStyle `xml:"-"`
}

// UnmarshalXML captures the element's verbatim attribute list (source
// attribute order and any unmodeled attributes) before decoding through the
// struct tags; the reflection marshaler replays it.
func (fp *CT_FramePr) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	fp.CapturedEmptyTag = xmlb.CaptureEmptyTagStyle(d)
	fp.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias CT_FramePr
	return d.DecodeElement((*alias)(fp), &start)
}

// CT_DocGrid represents the document grid.
type CT_DocGrid struct {
	Type      string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main type,attr,omitempty"`
	LinePitch string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main linePitch,attr,omitempty"`
	CharSpace string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main charSpace,attr,omitempty"`
	// CapturedEmptyTag records the source's per-instance "/>" vs " />" form.
	CapturedEmptyTag xmlb.EmptyTagStyle `xml:"-"`
}

// UnmarshalXML captures the element's empty-tag style before decoding
// through the struct tags.
func (v *CT_DocGrid) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	v.CapturedEmptyTag = xmlb.CaptureEmptyTagStyle(d)
	type alias CT_DocGrid
	return d.DecodeElement((*alias)(v), &start)
}

// CT_Columns represents column definitions. Field order follows Word's
// attribute emission order (num, space, equalWidth).
type CT_Columns struct {
	Num           string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main num,attr,omitempty"`
	Space         string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main space,attr,omitempty"`
	EqualWidth    string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main equalWidth,attr,omitempty"`
	Sep           string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main sep,attr,omitempty"`
	Col           []CT_Column     `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main col,omitempty"`
	CapturedAttrs []xmlb.RootAttr `xml:"-"` // verbatim source attrs; see common/xml.CaptureAttrs
	// CapturedEmptyTag records the source's per-instance "/>" vs " />" form.
	CapturedEmptyTag xmlb.EmptyTagStyle `xml:"-"`
}

// UnmarshalXML captures the element's verbatim attribute list (source
// attribute order and any unmodeled attributes) before decoding through the
// struct tags; the reflection marshaler replays it.
func (c *CT_Columns) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	c.CapturedEmptyTag = xmlb.CaptureEmptyTagStyle(d)
	c.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias CT_Columns
	return d.DecodeElement((*alias)(c), &start)
}

// CT_Column represents a single column definition.
type CT_Column struct {
	W             string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main w,attr,omitempty"`
	Space         string          `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main space,attr,omitempty"`
	CapturedAttrs []xmlb.RootAttr `xml:"-"` // verbatim source attrs; see common/xml.CaptureAttrs
	// CapturedEmptyTag records the source's per-instance "/>" vs " />" form.
	CapturedEmptyTag xmlb.EmptyTagStyle `xml:"-"`
}

// UnmarshalXML captures the element's verbatim attribute list (source
// attribute order and any unmodeled attributes) before decoding through the
// struct tags; the reflection marshaler replays it.
func (c *CT_Column) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	c.CapturedEmptyTag = xmlb.CaptureEmptyTagStyle(d)
	c.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias CT_Column
	return d.DecodeElement((*alias)(c), &start)
}

// NsWml is the WML namespace constant used throughout this package.
const NsWml = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"

// NsRelationships is the relationship namespace.
const NsRelationships = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"

// UnmarshalOnOff handles the common WML pattern of decoding an on/off element.
// Returns a pointer to CT_OnOff or nil if the element is not found at the current position.
func UnmarshalOnOff(d *xml.Decoder, start *xml.StartElement) *CT_OnOff {
	o := &CT_OnOff{}
	o.CapturedEmptyTag = xmlb.CaptureEmptyTagStyle(d)
	o.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	for _, attr := range start.Attr {
		if attr.Name.Local == "val" {
			val := attr.Value
			o.Val = &val
		}
	}
	_ = d.Skip()
	return o
}
