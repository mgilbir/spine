// This file provides PresentationML comment types from pml.xsd.
// These types implement the p: namespace comment elements.

package oxml

import (
	"encoding/xml"

	"github.com/mgilbir/spine/common/dml"
	xmlb "github.com/mgilbir/spine/common/xml"
)

// --- Comments ---

// CommentList represents CT_CommentList (p:cmLst)
type CommentList struct {
	Cm []*Comment `xml:"http://schemas.openxmlformats.org/presentationml/2006/main cm,omitempty"`
}

// Comment represents CT_Comment (p:cm)
type Comment struct {
	AuthorId uint32         `xml:"authorId,attr"`
	Dt       string         `xml:"dt,attr,omitempty"` // datetime
	Idx      uint32         `xml:"idx,attr"`          // comment index
	Pos      *Point2D       `xml:"http://schemas.openxmlformats.org/presentationml/2006/main pos,omitempty"`
	Text     string         `xml:"http://schemas.openxmlformats.org/presentationml/2006/main text,omitempty"`
	ExtLst   *ExtensionList `xml:"http://schemas.openxmlformats.org/presentationml/2006/main extLst,omitempty"`
	// CapturedAttrs preserves the verbatim source attribute list (attribute
	// order and any unmodeled attribute) across the comments part's
	// regeneration; see common/xml.CaptureAttrs.
	CapturedAttrs []xmlb.RootAttr `xml:"-"`
}

// UnmarshalXML captures the element's verbatim attribute list before decoding
// through the struct tags; the reflection marshaler replays it.
func (cm *Comment) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	cm.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias Comment
	return d.DecodeElement((*alias)(cm), &start)
}

// Point2D represents CT_Point2D for comment position (x,y in EMUs)
type Point2D struct {
	X int64 `xml:"x,attr"`
	Y int64 `xml:"y,attr"`
}

// CommentAuthorList represents CT_CommentAuthorList (p:cmAuthorLst)
type CommentAuthorList struct {
	CmAuthor []*CommentAuthor `xml:"http://schemas.openxmlformats.org/presentationml/2006/main cmAuthor,omitempty"`
}

// CommentAuthor represents CT_CommentAuthor (p:cmAuthor). lastIdx and clrIdx
// are XSD default-0, so an explicit clrIdx="0" (PowerPoint's first author
// colour, very common) would be deleted by omitempty when the authors part is
// regenerated — CapturedAttrs keeps it (C420).
type CommentAuthor struct {
	Id            uint32          `xml:"id,attr"`
	Name          string          `xml:"name,attr"`
	Initials      string          `xml:"initials,attr,omitempty"`
	LastIdx       uint32          `xml:"lastIdx,attr,omitempty"`
	ClrIdx        uint32          `xml:"clrIdx,attr,omitempty"`
	ExtLst        *ExtensionList  `xml:"http://schemas.openxmlformats.org/presentationml/2006/main extLst,omitempty"`
	CapturedAttrs []xmlb.RootAttr `xml:"-"` // verbatim source attrs; see common/xml.CaptureAttrs
}

// UnmarshalXML captures the element's verbatim attribute list before decoding
// through the struct tags; the reflection marshaler replays it.
func (ca *CommentAuthor) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	ca.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias CommentAuthor
	return d.DecodeElement((*alias)(ca), &start)
}

// --- Notes ---

// NotesSlide represents CT_NotesSlide (p:notes). showMasterSp and
// showMasterPhAnim default to true in the schema, so they are modeled as *bool:
// an explicit showMasterSp="0" must survive a re-marshal rather than be dropped
// (which readers re-interpret as the default true), mirroring the Slide type.
type NotesSlide struct {
	CSld             *CommonSlideData `xml:"http://schemas.openxmlformats.org/presentationml/2006/main cSld,omitempty"`
	ClrMapOvr        *dml.ClrMapOvr   `xml:"http://schemas.openxmlformats.org/presentationml/2006/main clrMapOvr,omitempty"`
	ExtLst           *ExtensionList   `xml:"http://schemas.openxmlformats.org/presentationml/2006/main extLst,omitempty"`
	ShowMasterSp     *bool            `xml:"showMasterSp,attr,omitempty"`
	ShowMasterPhAnim *bool            `xml:"showMasterPhAnim,attr,omitempty"`
	// OriginalRootAttrs preserves the p:notes root's verbatim attribute list;
	// see Slide.OriginalRootAttrs. Without it a regenerated notes part carried
	// only the a/r/p declarations, so an mc:AlternateContent in the notes shape
	// tree — the normal PowerPoint form for ink and other guarded content — was
	// re-emitted with no xmlns:mc anywhere in the part, and every extra root
	// declaration and mc:Ignorable was dropped (C421).
	OriginalRootAttrs []xmlb.RootAttr `xml:"-"`
}

// UnmarshalXML captures the root element's verbatim attribute list before
// decoding through the struct tags.
func (ns *NotesSlide) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	ns.OriginalRootAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias NotesSlide
	return d.DecodeElement((*alias)(ns), &start)
}

// NotesMaster represents CT_NotesMaster (p:notesMaster)
type NotesMaster struct {
	CSld       *CommonSlideData `xml:"http://schemas.openxmlformats.org/presentationml/2006/main cSld,omitempty"`
	ClrMap     *dml.ClrMap      `xml:"http://schemas.openxmlformats.org/presentationml/2006/main clrMap,omitempty"`
	Hf         *HeaderFooter    `xml:"http://schemas.openxmlformats.org/presentationml/2006/main hf,omitempty"`
	NotesStyle *dml.LstStyle    `xml:"http://schemas.openxmlformats.org/presentationml/2006/main notesStyle,omitempty"`
	ExtLst     *ExtensionList   `xml:"http://schemas.openxmlformats.org/presentationml/2006/main extLst,omitempty"`
}

// --- Handout Master ---

// HandoutMaster represents CT_HandoutMaster (p:handoutMaster)
type HandoutMaster struct {
	CSld   *CommonSlideData `xml:"http://schemas.openxmlformats.org/presentationml/2006/main cSld,omitempty"`
	ClrMap *dml.ClrMap      `xml:"http://schemas.openxmlformats.org/presentationml/2006/main clrMap,omitempty"`
	Hf     *HeaderFooter    `xml:"http://schemas.openxmlformats.org/presentationml/2006/main hf,omitempty"`
	ExtLst *ExtensionList   `xml:"http://schemas.openxmlformats.org/presentationml/2006/main extLst,omitempty"`
}

// HeaderFooter represents CT_HeaderFooter (p:hf)
type HeaderFooter struct {
	SldNum        *bool           `xml:"sldNum,attr,omitempty"`
	Hdr           *bool           `xml:"hdr,attr,omitempty"`
	Ftr           *bool           `xml:"ftr,attr,omitempty"`
	Dt            *bool           `xml:"dt,attr,omitempty"`
	CapturedAttrs []xmlb.RootAttr `xml:"-"` // verbatim source attrs; see common/xml.CaptureAttrs
}

// UnmarshalXML captures the element's verbatim attribute list (source
// attribute order and any unmodeled attributes) before decoding through the
// struct tags; the reflection marshaler replays it.
func (hf *HeaderFooter) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	hf.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	type alias HeaderFooter
	return d.DecodeElement((*alias)(hf), &start)
}

// --- Tags ---

// TagList represents CT_TagList (p:tagLst)
type TagList struct {
	Tag []*StringTag `xml:"http://schemas.openxmlformats.org/presentationml/2006/main tag,omitempty"`
}

// StringTag represents CT_StringTag (p:tag)
type StringTag struct {
	Name string `xml:"name,attr"`
	Val  string `xml:"val,attr"`
}

// Note: CustomerDataList and CustomerData are defined in presentation.go

// RelationshipRef represents CT_Rel (p:tags, etc.)
type RelationshipRef struct {
	Id string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr"`
}
