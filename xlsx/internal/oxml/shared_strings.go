// Package oxml contains the XML schema types for XLSX documents.
package oxml

import (
	"encoding/xml"
	"fmt"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// CT_Sst is the root element of sharedStrings.xml.
type CT_Sst struct {
	XMLName     xml.Name `xml:"http://schemas.openxmlformats.org/spreadsheetml/2006/main sst"`
	Count       *uint32  `xml:"count,attr,omitempty"`
	UniqueCount *uint32  `xml:"uniqueCount,attr,omitempty"`
	Si          []CT_Rst `xml:"si"`
	// OriginalNSDecls preserves the namespace declarations from the original XML
	// for byte-identical round-trip of sharedStrings.xml.
	OriginalNSDecls []xmlb.NSDecl `xml:"-"`
}

// UnmarshalXML implements custom unmarshaling for CT_Sst.
func (sst *CT_Sst) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	sst.XMLName = start.Name
	for _, attr := range start.Attr {
		switch {
		case attr.Name.Local == "count":
			var v uint32
			if _, err := parseUint32(attr.Value, &v); err != nil {
				return err
			}
			sst.Count = &v
		case attr.Name.Local == "uniqueCount":
			var v uint32
			if _, err := parseUint32(attr.Value, &v); err != nil {
				return err
			}
			sst.UniqueCount = &v
		}
		// Capture namespace declarations for round-trip preservation
		if attr.Name.Space == "xmlns" {
			sst.OriginalNSDecls = append(sst.OriginalNSDecls, xmlb.NSDecl{
				Prefix: attr.Name.Local,
				URI:    attr.Value,
			})
		} else if attr.Name.Space == "" && attr.Name.Local == "xmlns" {
			// Default namespace: xmlns="URI"
			sst.OriginalNSDecls = append([]xmlb.NSDecl{{Prefix: "", URI: attr.Value}}, sst.OriginalNSDecls...)
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
				d.Skip()
			}
		case xml.EndElement:
			return nil
		}
	}
}

// parseUint32 parses a uint32 from a string using fmt.Sscanf.
func parseUint32(s string, v *uint32) (int, error) {
	return fmt.Sscanf(s, "%d", v)
}

// CT_Rst represents a rich text string item (si element child, also used for
// inline strings in worksheet cells).
type CT_Rst struct {
	T          *string        `xml:"t,omitempty"`
	R          []CT_RElt      `xml:"r,omitempty"`
	PhoneticPr *CT_PhoneticPr `xml:"rPh,omitempty"`
}

// CT_RElt represents a rich text run element (r).
type CT_RElt struct {
	RPr *CT_RPrElt `xml:"rPr,omitempty"`
	T   string     `xml:"t"`
}

// CT_RPrElt represents run properties for rich text.
type CT_RPrElt struct {
	B         *CT_BooleanProperty            `xml:"b,omitempty"`
	I         *CT_BooleanProperty            `xml:"i,omitempty"`
	Strike    *CT_BooleanProperty            `xml:"strike,omitempty"`
	Condense  *CT_BooleanProperty            `xml:"condense,omitempty"`
	Extend    *CT_BooleanProperty            `xml:"extend,omitempty"`
	Outline   *CT_BooleanProperty            `xml:"outline,omitempty"`
	Shadow    *CT_BooleanProperty            `xml:"shadow,omitempty"`
	U         *CT_UnderlineProperty          `xml:"u,omitempty"`
	VertAlign *CT_VerticalAlignFontProperty  `xml:"vertAlign,omitempty"`
	Sz        *CT_FontSize                   `xml:"sz,omitempty"`
	Color     *CT_Color                      `xml:"color,omitempty"`
	RFont     *CT_FontName                   `xml:"rFont,omitempty"`
	Family    *CT_IntProperty                `xml:"family,omitempty"`
	Charset   *CT_IntProperty                `xml:"charset,omitempty"`
	Scheme    *CT_FontScheme                 `xml:"scheme,omitempty"`
}

// CT_BooleanProperty represents a boolean property element.
// When the element is present with no val attribute, the value defaults to true.
type CT_BooleanProperty struct {
	Val *bool `xml:"val,attr,omitempty"`
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
