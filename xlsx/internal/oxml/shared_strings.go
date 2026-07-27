// This file contains the XML schema types for XLSX documents.

package oxml

import (
	"encoding/xml"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// CT_Sst is the root element of sharedStrings.xml. The part has no writer:
// new strings are written inline on the cell and an opened sharedStrings.xml
// round-trips through its preserved raw bytes, so this type is read-only and
// carries no round-trip capture (C554).
type CT_Sst struct {
	XMLName     xml.Name `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main sst"`
	Count       *uint32  `xml:"count,attr,omitempty"`
	UniqueCount *uint32  `xml:"uniqueCount,attr,omitempty"`
	Si          []CT_Rst `xml:"si"`
}

// UnmarshalXML implements custom unmarshaling for CT_Sst. Unparsable counts
// leave the field unset rather than failing the whole Open: count/uniqueCount
// are advisory hints Excel recomputes, and a single malformed one must not
// make the workbook unopenable (C552).
func (sst *CT_Sst) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	sst.XMLName = start.Name
	for _, attr := range start.Attr {
		if attr.Name.Space != "" {
			continue
		}
		switch attr.Name.Local {
		case "count":
			sst.Count = parseUintPtr(attr.Value)
		case "uniqueCount":
			sst.UniqueCount = parseUintPtr(attr.Value)
		}
	}

	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "si":
				var si CT_Rst
				if err := d.DecodeElement(&si, &t); err != nil {
					return err
				}
				sst.Si = append(sst.Si, si)
			default:
				if err := d.Skip(); err != nil {
					return err
				}
			}
		case xml.EndElement:
			return nil
		}
	}
}

// CT_Rst represents a rich text string item (si element child, also used for
// inline strings in worksheet cells).
type CT_Rst struct {
	T *string   `xml:"t,omitempty"`
	R []CT_RElt `xml:"r,omitempty"`
	// RPh holds the phonetic runs (furigana) of a Japanese string. They sit
	// between the runs and phoneticPr in schema order; captured so an inline
	// string or shared string carrying phonetic guides round-trips them on a
	// dirty save rather than losing them on regeneration.
	RPh        []CT_PhoneticRun `xml:"rPh,omitempty"`
	PhoneticPr *CT_PhoneticPr   `xml:"phoneticPr,omitempty"`
}

// CT_PhoneticRun represents an rPh element: a phonetic-guide run spanning the
// base-text character range [sb, eb) with its reading in the t child.
type CT_PhoneticRun struct {
	Sb uint32 `xml:"sb,attr"`
	Eb uint32 `xml:"eb,attr"`
	T  string `xml:"t"`
}

// MarshalToBuilder implements xmlb.BuilderMarshaler for CT_Rst. The plain
// text element carries xml:space="preserve" when the value has leading or
// trailing whitespace, which XML processors would otherwise strip.
func (rst *CT_Rst) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	if rst.T == nil && len(rst.R) == 0 && len(rst.RPh) == 0 && rst.PhoneticPr == nil {
		b.EmptyElement(ns, localName)
		return
	}
	b.StartElement(ns, localName)
	if rst.T != nil {
		var attrs []xmlb.Attr
		if needsSpacePreserve(*rst.T) {
			attrs = append(attrs, xmlb.Attr{Name: "xml:space", Value: "preserve"})
		}
		b.WriteElement(ns, "t", *rst.T, attrs...)
	}
	for i := range rst.R {
		b.MarshalElement(ns, "r", &rst.R[i])
	}
	for i := range rst.RPh {
		b.MarshalElement(ns, "rPh", &rst.RPh[i])
	}
	if rst.PhoneticPr != nil {
		b.MarshalElement(ns, "phoneticPr", rst.PhoneticPr)
	}
	b.EndElement(ns, localName)
}

// needsSpacePreserve reports whether s would lose whitespace without
// xml:space="preserve".
func needsSpacePreserve(s string) bool {
	if s == "" {
		return false
	}
	isWS := func(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\r' }
	return isWS(s[0]) || isWS(s[len(s)-1])
}

// CT_RElt represents a rich text run element (r).
type CT_RElt struct {
	RPr *CT_RPrElt `xml:"rPr,omitempty"`
	T   string     `xml:"t"`
}

// MarshalToBuilder writes a rich-text run, emitting xml:space="preserve" on the
// text element when the run has leading or trailing whitespace so a strict
// reader (Excel) does not trim it — the "Total: " label case.
func (r *CT_RElt) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	b.StartElement(ns, localName)
	if r.RPr != nil {
		b.MarshalElement(ns, "rPr", r.RPr)
	}
	var attrs []xmlb.Attr
	if needsSpacePreserve(r.T) {
		attrs = append(attrs, xmlb.Attr{Name: "xml:space", Value: "preserve"})
	}
	b.WriteElement(ns, "t", r.T, attrs...)
	b.EndElement(ns, localName)
}

// CT_RPrElt represents run properties for rich text.
type CT_RPrElt struct {
	B         *CT_BooleanProperty           `xml:"b,omitempty"`
	I         *CT_BooleanProperty           `xml:"i,omitempty"`
	Strike    *CT_BooleanProperty           `xml:"strike,omitempty"`
	Condense  *CT_BooleanProperty           `xml:"condense,omitempty"`
	Extend    *CT_BooleanProperty           `xml:"extend,omitempty"`
	Outline   *CT_BooleanProperty           `xml:"outline,omitempty"`
	Shadow    *CT_BooleanProperty           `xml:"shadow,omitempty"`
	U         *CT_UnderlineProperty         `xml:"u,omitempty"`
	VertAlign *CT_VerticalAlignFontProperty `xml:"vertAlign,omitempty"`
	Sz        *CT_FontSize                  `xml:"sz,omitempty"`
	Color     *CT_Color                     `xml:"color,omitempty"`
	RFont     *CT_FontName                  `xml:"rFont,omitempty"`
	Family    *CT_IntProperty               `xml:"family,omitempty"`
	Charset   *CT_IntProperty               `xml:"charset,omitempty"`
	Scheme    *CT_FontScheme                `xml:"scheme,omitempty"`
}

// CT_BooleanProperty represents a boolean property element.
// When the element is present with no val attribute, the value defaults to true.
type CT_BooleanProperty struct {
	Val *bool `xml:"val,attr,omitempty"`
}

// UnmarshalXML coerces the val ST_OnOff boolean before decoding, so a wild
// font property such as <b val="on"/> does not fail the whole Open. The
// original bytes round-trip on a zero-mod save (styles/sharedStrings are
// re-emitted raw unless edited).
func (bp *CT_BooleanProperty) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	start.Attr = coerceBoolAttrs(start.Attr, "val")
	type alias CT_BooleanProperty
	return d.DecodeElement((*alias)(bp), &start)
}

// CT_FontSize represents a font size property element.
type CT_FontSize struct {
	Val float64 `xml:"val,attr"`
}

// CT_FontName represents a font name property element.
type CT_FontName struct {
	Val string `xml:"val,attr"`
}

// CT_IntProperty represents an integer property element.
type CT_IntProperty struct {
	Val int32 `xml:"val,attr"`
}

// CT_FontScheme represents a font scheme property element.
type CT_FontScheme struct {
	Val string `xml:"val,attr"`
}

// CT_UnderlineProperty represents an underline property element.
// When the element is present with no val attribute, the value defaults to "single".
type CT_UnderlineProperty struct {
	Val string `xml:"val,attr,omitempty"`
}

// CT_VerticalAlignFontProperty represents a vertical alignment font property element.
type CT_VerticalAlignFontProperty struct {
	Val string `xml:"val,attr"`
}
