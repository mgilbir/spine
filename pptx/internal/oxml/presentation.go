// Package oxml contains the XML schema types for PPTX documents.
package oxml

import (
	"encoding/xml"
	"fmt"
)

// Presentation is the root element of presentation.xml.
type Presentation struct {
	XMLName         xml.Name         `xml:"http://schemas.openxmlformats.org/presentationml/2006/main presentation"`
	XmlnsA          string           `xml:"xmlns:a,attr,omitempty"`
	XmlnsR          string           `xml:"xmlns:r,attr,omitempty"`
	XmlnsP          string           `xml:"xmlns:p,attr,omitempty"`
	SaveSubsetFonts bool             `xml:"saveSubsetFonts,attr,omitempty"`
	SlideMasterIDs  *SlideMasterIDs  `xml:"sldMasterIdLst,omitempty"`
	SlideIDs        *SlideIDs        `xml:"sldIdLst,omitempty"`
	SlideSize       *SlideSize       `xml:"sldSz,omitempty"`
	NotesSize       *SlideSize       `xml:"notesSz,omitempty"`
	DefaultTextStyle *TextListStyle  `xml:"defaultTextStyle,omitempty"`
}

// PPTX namespace constants
const (
	NsPresentationML = "http://schemas.openxmlformats.org/presentationml/2006/main"
	NsDrawingML      = "http://schemas.openxmlformats.org/drawingml/2006/main"
	NsRelationships  = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
)

// SlideMasterIDs contains a list of slide master ID references.
type SlideMasterIDs struct {
	SlideMasterID []SlideMasterID `xml:"sldMasterId"`
}

// SlideMasterID references a slide master.
type SlideMasterID struct {
	ID  uint32 `xml:"id,attr,omitempty"`
	RID string `xml:"-"`
}

// MarshalXML implements custom XML marshaling for SlideMasterID.
func (s SlideMasterID) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	if s.ID > 0 {
		start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "id"}, Value: fmt.Sprintf("%d", s.ID)})
	}
	start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Space: NsRelationships, Local: "id"}, Value: s.RID})
	return e.EncodeElement(struct{}{}, start)
}

// UnmarshalXML implements custom XML unmarshaling for SlideMasterID.
func (s *SlideMasterID) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		if attr.Name.Local == "id" {
			if attr.Name.Space == "" || attr.Name.Space == NsPresentationML {
				var id uint32
				fmt.Sscanf(attr.Value, "%d", &id)
				s.ID = id
			} else if attr.Name.Space == NsRelationships {
				s.RID = attr.Value
			}
		}
	}
	return d.Skip()
}

// SlideIDs contains a list of slide ID references.
type SlideIDs struct {
	SlideID []SlideID `xml:"sldId"`
}

// SlideID references a slide.
type SlideID struct {
	ID  uint32 `xml:"-"`
	RID string `xml:"-"`
}

// MarshalXML implements custom XML marshaling for SlideID.
func (s SlideID) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "id"}, Value: fmt.Sprintf("%d", s.ID)})
	start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Space: NsRelationships, Local: "id"}, Value: s.RID})
	return e.EncodeElement(struct{}{}, start)
}

// UnmarshalXML implements custom XML unmarshaling for SlideID.
func (s *SlideID) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		if attr.Name.Local == "id" {
			if attr.Name.Space == "" || attr.Name.Space == NsPresentationML {
				// This is the numeric ID
				var id uint32
				fmt.Sscanf(attr.Value, "%d", &id)
				s.ID = id
			} else if attr.Name.Space == NsRelationships {
				// This is the relationship ID
				s.RID = attr.Value
			}
		}
	}
	// Consume the element
	return d.Skip()
}

// SlideSize specifies the size of slides.
type SlideSize struct {
	Cx   int64  `xml:"cx,attr"`
	Cy   int64  `xml:"cy,attr"`
	Type string `xml:"type,attr,omitempty"`
}

// TextListStyle contains text styling for different paragraph levels.
type TextListStyle struct {
	DefPPr *TextParagraphProperties `xml:"defPPr,omitempty"`
	Lvl1PPr *TextParagraphProperties `xml:"lvl1pPr,omitempty"`
	Lvl2PPr *TextParagraphProperties `xml:"lvl2pPr,omitempty"`
	Lvl3PPr *TextParagraphProperties `xml:"lvl3pPr,omitempty"`
	Lvl4PPr *TextParagraphProperties `xml:"lvl4pPr,omitempty"`
	Lvl5PPr *TextParagraphProperties `xml:"lvl5pPr,omitempty"`
	Lvl6PPr *TextParagraphProperties `xml:"lvl6pPr,omitempty"`
	Lvl7PPr *TextParagraphProperties `xml:"lvl7pPr,omitempty"`
	Lvl8PPr *TextParagraphProperties `xml:"lvl8pPr,omitempty"`
	Lvl9PPr *TextParagraphProperties `xml:"lvl9pPr,omitempty"`
}

// TextParagraphProperties contains paragraph-level text properties.
type TextParagraphProperties struct {
	MarL         *int64  `xml:"marL,attr,omitempty"`
	MarR         *int64  `xml:"marR,attr,omitempty"`
	Indent       *int64  `xml:"indent,attr,omitempty"`
	Algn         string  `xml:"algn,attr,omitempty"`
	DefTabSz     *int64  `xml:"defTabSz,attr,omitempty"`
	Rtl          *bool   `xml:"rtl,attr,omitempty"`
	FontAlgn     string  `xml:"fontAlgn,attr,omitempty"`
	LatinLnBrk   *bool   `xml:"latinLnBrk,attr,omitempty"`
	HangingPunct *bool   `xml:"hangingPunct,attr,omitempty"`
	DefRPr       *TextCharacterProperties `xml:"defRPr,omitempty"`
}

// TextCharacterProperties contains character-level text properties.
type TextCharacterProperties struct {
	Kumimoji   *bool   `xml:"kumimoji,attr,omitempty"`
	Lang       string  `xml:"lang,attr,omitempty"`
	AltLang    string  `xml:"altLang,attr,omitempty"`
	Sz         *int32  `xml:"sz,attr,omitempty"`
	B          *bool   `xml:"b,attr,omitempty"`
	I          *bool   `xml:"i,attr,omitempty"`
	U          string  `xml:"u,attr,omitempty"`
	Strike     string  `xml:"strike,attr,omitempty"`
	Kern       *int32  `xml:"kern,attr,omitempty"`
	Cap        string  `xml:"cap,attr,omitempty"`
	Spc        *int32  `xml:"spc,attr,omitempty"`
	Baseline   *int32  `xml:"baseline,attr,omitempty"`
	SolidFill  *SolidFill `xml:"solidFill,omitempty"`
	Latin      *TextFont  `xml:"latin,omitempty"`
	Ea         *TextFont  `xml:"ea,omitempty"`
	Cs         *TextFont  `xml:"cs,omitempty"`
}

// SolidFill specifies a solid color fill.
type SolidFill struct {
	SrgbClr   *SrgbColor   `xml:"srgbClr,omitempty"`
	SchemeClr *SchemeColor `xml:"schemeClr,omitempty"`
}

// SrgbColor specifies an RGB color.
type SrgbColor struct {
	Val string `xml:"val,attr"`
}

// SchemeColor specifies a theme/scheme color.
type SchemeColor struct {
	Val   string  `xml:"val,attr"`
	Tint  *int32  `xml:"tint>val,omitempty"`
	Shade *int32  `xml:"shade>val,omitempty"`
}

// TextFont specifies a font.
type TextFont struct {
	Typeface string `xml:"typeface,attr"`
	Panose   string `xml:"panose,attr,omitempty"`
	PitchFamily int8 `xml:"pitchFamily,attr,omitempty"`
	Charset  int8   `xml:"charset,attr,omitempty"`
}

// DefaultSlideSize returns the default slide size (10" x 7.5" at 96 DPI).
func DefaultSlideSize() *SlideSize {
	return &SlideSize{
		Cx:   9144000, // 10 inches in EMUs
		Cy:   6858000, // 7.5 inches in EMUs
		Type: "screen4x3",
	}
}

// WidescreenSlideSize returns the widescreen slide size (13.33" x 7.5").
func WidescreenSlideSize() *SlideSize {
	return &SlideSize{
		Cx:   12192000, // 13.33 inches in EMUs
		Cy:   6858000,  // 7.5 inches in EMUs
		Type: "screen16x9",
	}
}
