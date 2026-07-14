// This file contains the XML schema types for PPTX documents.

package oxml

import (
	"encoding/xml"
	"fmt"

	"github.com/mgilbir/spine/common/dml"
	xmlb "github.com/mgilbir/spine/common/xml"
)

// Presentation is the root element of presentation.xml.
// Based on CT_Presentation from pml.xsd
type Presentation struct {
	XMLName xml.Name `xml:"http://schemas.openxmlformats.org/presentationml/2006/main presentation"`
	// Prolog preserves the source part's XML declaration and surrounding
	// whitespace for byte-faithful regeneration.
	Prolog xmlb.Prolog `xml:"-"`
	// SelfClosingSpace records whether the source writes " />" instead of "/>".
	SelfClosingSpace bool `xml:"-"`
	// CollapseEmpty records whether the source writes empty elements
	// self-closing, so empty open/close pairs collapse on regeneration.
	CollapseEmpty bool `xml:"-"`
	// OriginalRootAttrs preserves the root element's verbatim attribute list
	// (namespace declarations interleaved with attributes); nil for decks
	// built programmatically, which emit the standard a/r/p declarations and
	// the XSD attribute order.
	OriginalRootAttrs []xmlb.RootAttr `xml:"-"`
	XmlnsA            string          `xml:"xmlns:a,attr,omitempty"`
	XmlnsR            string          `xml:"xmlns:r,attr,omitempty"`
	XmlnsP            string          `xml:"xmlns:p,attr,omitempty"`

	// Attributes from CT_Presentation (pml.xsd lines 1057-1068)
	ServerZoom               string  `xml:"serverZoom,attr,omitempty"`
	FirstSlideNum            *int    `xml:"firstSlideNum,attr,omitempty"`
	ShowSpecialPlsOnTitleSld *bool   `xml:"showSpecialPlsOnTitleSld,attr,omitempty"`
	Rtl                      *bool   `xml:"rtl,attr,omitempty"`
	RemovePersonalInfoOnSave *bool   `xml:"removePersonalInfoOnSave,attr,omitempty"`
	CompatMode               *bool   `xml:"compatMode,attr,omitempty"`
	StrictFirstAndLastChars  *bool   `xml:"strictFirstAndLastChars,attr,omitempty"`
	EmbedTrueTypeFonts       *bool   `xml:"embedTrueTypeFonts,attr,omitempty"`
	SaveSubsetFonts          *bool   `xml:"saveSubsetFonts,attr,omitempty"`
	AutoCompressPictures     *bool   `xml:"autoCompressPictures,attr,omitempty"`
	BookmarkIdSeed           *uint32 `xml:"bookmarkIdSeed,attr,omitempty"`
	Conformance              string  `xml:"conformance,attr,omitempty"`

	// Elements from CT_Presentation (pml.xsd lines 1040-1055)
	SlideMasterIDs   *SlideMasterIDs   `xml:"sldMasterIdLst,omitempty"`
	NotesMasterIDs   *NotesMasterIDs   `xml:"notesMasterIdLst,omitempty"`
	HandoutMasterIDs *HandoutMasterIDs `xml:"handoutMasterIdLst,omitempty"`
	SlideIDs         *SlideIDs         `xml:"sldIdLst,omitempty"`
	SlideSize        *SlideSize        `xml:"sldSz,omitempty"`
	NotesSize        *SlideSize        `xml:"notesSz,omitempty"`
	SmartTags        *SmartTags        `xml:"smartTags,omitempty"`
	EmbeddedFontLst  *EmbeddedFontList `xml:"embeddedFontLst,omitempty"`
	CustShowLst      *CustomShowList   `xml:"custShowLst,omitempty"`
	PhotoAlbum       *PhotoAlbum       `xml:"photoAlbum,omitempty"`
	CustDataLst      *CustomerDataList `xml:"custDataLst,omitempty"`
	Kinsoku          *Kinsoku          `xml:"kinsoku,omitempty"`
	DefaultTextStyle *dml.LstStyle     `xml:"defaultTextStyle,omitempty"`
	ModifyVerifier   *ModifyVerifier   `xml:"modifyVerifier,omitempty"`
	ExtLst           *ExtensionList    `xml:"extLst,omitempty"`
}

// UnmarshalXML captures the root element's verbatim attribute list before
// decoding through the struct tags.
func (p *Presentation) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	p.OriginalRootAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias Presentation
	return d.DecodeElement((*alias)(p), &start)
}

// NotesMasterIDs contains a list of notes master ID references.
type NotesMasterIDs struct {
	NotesMasterID []NotesMasterID `xml:"notesMasterId"`
}

// NotesMasterID references a notes master.
type NotesMasterID struct {
	RID    string         `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
	ExtLst *ExtensionList `xml:"extLst,omitempty"`
}

// HandoutMasterIDs contains a list of handout master ID references.
type HandoutMasterIDs struct {
	HandoutMasterID []HandoutMasterID `xml:"handoutMasterId"`
}

// HandoutMasterID references a handout master.
type HandoutMasterID struct {
	RID    string         `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
	ExtLst *ExtensionList `xml:"extLst,omitempty"`
}

// SmartTags placeholder for smart tags.
type SmartTags struct {
	RID string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr,omitempty"`
}

// EmbeddedFontList contains embedded fonts.
type EmbeddedFontList struct {
	EmbeddedFont []EmbeddedFont `xml:"embeddedFont,omitempty"`
}

// EmbeddedFont represents an embedded font.
type EmbeddedFont struct {
	Font       *TextFont         `xml:"font,omitempty"`
	Regular    *EmbeddedFontData `xml:"regular,omitempty"`
	Bold       *EmbeddedFontData `xml:"bold,omitempty"`
	Italic     *EmbeddedFontData `xml:"italic,omitempty"`
	BoldItalic *EmbeddedFontData `xml:"boldItalic,omitempty"`
}

// EmbeddedFontData references embedded font data.
type EmbeddedFontData struct {
	RID string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
}

// CustomShowList contains custom slide shows.
type CustomShowList struct {
	CustShow []CustomShow `xml:"custShow,omitempty"`
}

// CustomShow represents CT_CustomShow (p:custShow). Its sldLst child is
// XSD-required; modeling it (C4) keeps parsed custom shows schema-valid
// instead of re-emitting an empty <p:custShow name id/>.
type CustomShow struct {
	Name   string                 `xml:"name,attr"`
	ID     uint32                 `xml:"id,attr"`
	SldLst *SlideRelationshipList `xml:"sldLst"`
	ExtLst *ExtensionList         `xml:"extLst,omitempty"`
}

// SlideRelationshipList represents CT_SlideRelationshipList (p:sldLst).
type SlideRelationshipList struct {
	Sld []RelationshipRef `xml:"sld,omitempty"`
}

// PhotoAlbum contains photo album settings.
type PhotoAlbum struct {
	Bw           *bool          `xml:"bw,attr,omitempty"`
	ShowCaptions *bool          `xml:"showCaptions,attr,omitempty"`
	Layout       string         `xml:"layout,attr,omitempty"`
	Frame        string         `xml:"frame,attr,omitempty"`
	ExtLst       *ExtensionList `xml:"extLst,omitempty"`
}

// CustomerDataList contains custom data.
type CustomerDataList struct {
	CustData []CustomerData `xml:"custData,omitempty"`
	Tags     *Tags          `xml:"tags,omitempty"`
}

// CustomerData references custom data.
type CustomerData struct {
	RID string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
}

// Tags references tags.
type Tags struct {
	RID string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
}

// Kinsoku contains kinsoku settings for East Asian text.
type Kinsoku struct {
	Lang          string `xml:"lang,attr,omitempty"`
	InvalStChars  string `xml:"invalStChars,attr,omitempty"`
	InvalEndChars string `xml:"invalEndChars,attr,omitempty"`
}

// ModifyVerifier contains modify verification settings.
type ModifyVerifier struct {
	CryptProviderType   string  `xml:"cryptProviderType,attr,omitempty"`
	CryptAlgorithmClass string  `xml:"cryptAlgorithmClass,attr,omitempty"`
	CryptAlgorithmType  string  `xml:"cryptAlgorithmType,attr,omitempty"`
	CryptAlgorithmSid   *uint32 `xml:"cryptAlgorithmSid,attr,omitempty"`
	SpinCount           *uint32 `xml:"spinCount,attr,omitempty"`
	SaltData            string  `xml:"saltData,attr,omitempty"`
	HashData            string  `xml:"hashData,attr,omitempty"`
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
	RID string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr,omitempty"`
	// IDOmitted records that the source entry had no id attribute (it is
	// optional in the schema); the regenerated entry then omits it too
	// instead of synthesizing one.
	IDOmitted bool           `xml:"-"`
	ExtLst    *ExtensionList `xml:"extLst,omitempty"`
}

// MarshalXML implements custom XML marshaling for SlideMasterID.
// Uses r:id attribute to match OOXML conventions (requires xmlns:r declaration in parent).
func (s SlideMasterID) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	if s.ID > 0 {
		start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "id"}, Value: fmt.Sprintf("%d", s.ID)})
	}
	// Use r:id directly - the r prefix is declared in the root presentation element
	start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "r:id"}, Value: s.RID})
	return e.EncodeElement(struct{}{}, start)
}

// MarshalToBuilder implements xmlb.BuilderMarshaler (see SlideLayoutID). The
// presentation.xml writer emits sldMasterId entries explicitly, so this is a
// safety net for any reflection path and to satisfy the Builder's C106 guard.
func (s SlideMasterID) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	var attrs []xmlb.Attr
	if !s.IDOmitted {
		attrs = append(attrs, xmlb.UintAttr("id", s.ID))
	}
	if s.RID != "" {
		attrs = append(attrs, xmlb.RelAttr("id", s.RID))
	}
	if s.ExtLst == nil {
		b.EmptyElement(ns, localName, attrs...)
		return
	}
	b.StartElement(ns, localName, attrs...)
	b.MarshalElement(ns, "extLst", s.ExtLst)
	b.EndElement(ns, localName)
}

// UnmarshalXML implements custom XML unmarshaling for SlideMasterID.
// Handles both namespaced (relationships:id) and prefixed (r:id) formats,
// and captures the optional extLst child (C225).
func (s *SlideMasterID) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	s.IDOmitted = true
	for _, attr := range start.Attr {
		switch {
		case attr.Name.Local == "id" && (attr.Name.Space == "" || attr.Name.Space == NsPresentationML):
			// Numeric ID
			var id uint32
			_, _ = fmt.Sscanf(attr.Value, "%d", &id)
			s.ID = id
			s.IDOmitted = false
		case attr.Name.Local == "id" && attr.Name.Space == NsRelationships:
			// Relationship ID with full namespace
			s.RID = attr.Value
		case attr.Name.Local == "r:id":
			// Relationship ID with r: prefix (our marshaled format)
			s.RID = attr.Value
		}
	}
	return unmarshalIDEntryChildren(d, &s.ExtLst)
}

// SlideIDs contains a list of slide ID references.
type SlideIDs struct {
	SlideID []SlideID `xml:"sldId"`
}

// SlideID references a slide.
type SlideID struct {
	ID     uint32         `xml:"id,attr"`
	RID    string         `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
	ExtLst *ExtensionList `xml:"extLst,omitempty"`
}

// unmarshalIDEntryChildren consumes the children of an sldId-family entry,
// decoding the optional extLst child into dst (previously the whole subtree
// was skipped, deleting parsed extensions on save — C225).
func unmarshalIDEntryChildren(d *xml.Decoder, dst **ExtensionList) error {
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "extLst" {
				el := &ExtensionList{}
				if err := d.DecodeElement(el, &t); err != nil {
					return err
				}
				*dst = el
				continue
			}
			if err := d.Skip(); err != nil {
				return err
			}
		case xml.EndElement:
			return nil
		}
	}
}

// MarshalXML implements custom XML marshaling for SlideID.
// Uses r:id attribute to match OOXML conventions (requires xmlns:r declaration in parent).
func (s SlideID) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "id"}, Value: fmt.Sprintf("%d", s.ID)})
	// Use r:id directly - the r prefix is declared in the root presentation element
	start.Attr = append(start.Attr, xml.Attr{Name: xml.Name{Local: "r:id"}, Value: s.RID})
	return e.EncodeElement(struct{}{}, start)
}

// MarshalToBuilder implements xmlb.BuilderMarshaler (see SlideLayoutID). The
// presentation.xml writer emits sldId entries explicitly, so this is a safety
// net for any reflection path and to satisfy the Builder's C106 guard.
func (s SlideID) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	attrs := []xmlb.Attr{xmlb.UintAttr("id", s.ID)}
	if s.RID != "" {
		attrs = append(attrs, xmlb.RelAttr("id", s.RID))
	}
	if s.ExtLst == nil {
		b.EmptyElement(ns, localName, attrs...)
		return
	}
	b.StartElement(ns, localName, attrs...)
	b.MarshalElement(ns, "extLst", s.ExtLst)
	b.EndElement(ns, localName)
}

// UnmarshalXML implements custom XML unmarshaling for SlideID.
// Handles both namespaced (relationships:id) and prefixed (r:id) formats.
func (s *SlideID) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		switch {
		case attr.Name.Local == "id" && (attr.Name.Space == "" || attr.Name.Space == NsPresentationML):
			// Numeric ID
			var id uint32
			_, _ = fmt.Sscanf(attr.Value, "%d", &id)
			s.ID = id
		case attr.Name.Local == "id" && attr.Name.Space == NsRelationships:
			// Relationship ID with full namespace
			s.RID = attr.Value
		case attr.Name.Local == "r:id":
			// Relationship ID with r: prefix (our marshaled format)
			s.RID = attr.Value
		}
	}
	return unmarshalIDEntryChildren(d, &s.ExtLst)
}

// SlideSize specifies the size of slides.
type SlideSize struct {
	Cx            int64           `xml:"cx,attr"`
	Cy            int64           `xml:"cy,attr"`
	Type          string          `xml:"type,attr,omitempty"`
	CapturedAttrs []xmlb.RootAttr `xml:"-"` // verbatim source attrs; see common/xml.CaptureAttrs
}

// UnmarshalXML captures the element's verbatim attribute list (source
// attribute order and any unmodeled attributes) before decoding through the
// struct tags; the reflection marshaler replays it.
func (ssz *SlideSize) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	ssz.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias SlideSize
	return d.DecodeElement((*alias)(ssz), &start)
}

// TextFont specifies a font.
type TextFont struct {
	Typeface      string          `xml:"typeface,attr"`
	Panose        string          `xml:"panose,attr,omitempty"`
	PitchFamily   *int8           `xml:"pitchFamily,attr,omitempty"`
	Charset       *int8           `xml:"charset,attr,omitempty"`
	CapturedAttrs []xmlb.RootAttr `xml:"-"` // verbatim source attrs; see common/xml.CaptureAttrs
}

// UnmarshalXML captures the element's verbatim attribute list (source
// attribute order and any unmodeled attributes) before decoding through the
// struct tags; the reflection marshaler replays it.
func (tft *TextFont) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	tft.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias TextFont
	return d.DecodeElement((*alias)(tft), &start)
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
