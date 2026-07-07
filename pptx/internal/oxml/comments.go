// Package oxml provides PresentationML comment types from pml.xsd.
// These types implement the p: namespace comment elements.
package oxml

import "github.com/mgilbir/spine/common/dml"

// --- Comments ---

// CommentList represents CT_CommentList (p:cmLst)
type CommentList struct {
	Cm []*Comment `xml:"http://schemas.openxmlformats.org/presentationml/2006/main cm,omitempty"`
}

// Comment represents CT_Comment (p:cm)
type Comment struct {
	AuthorId uint32 `xml:"authorId,attr"`
	Dt       string `xml:"dt,attr,omitempty"` // datetime
	Idx      uint32 `xml:"idx,attr"`          // comment index
	Pos      *Point2D `xml:"http://schemas.openxmlformats.org/presentationml/2006/main pos,omitempty"`
	Text     string   `xml:"http://schemas.openxmlformats.org/presentationml/2006/main text,omitempty"`
	ExtLst   *dml.ExtLst `xml:"http://schemas.openxmlformats.org/presentationml/2006/main extLst,omitempty"`
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

// CommentAuthor represents CT_CommentAuthor (p:cmAuthor)
type CommentAuthor struct {
	Id       uint32 `xml:"id,attr"`
	Name     string `xml:"name,attr"`
	Initials string `xml:"initials,attr,omitempty"`
	LastIdx  uint32 `xml:"lastIdx,attr,omitempty"`
	ClrIdx   uint32 `xml:"clrIdx,attr,omitempty"`
	ExtLst   *dml.ExtLst `xml:"http://schemas.openxmlformats.org/presentationml/2006/main extLst,omitempty"`
}

// CommentText represents the text content of a comment (p:text).
type CommentText struct {
	Value string `xml:",chardata"`
}

// --- Notes ---

// NotesSlide represents CT_NotesSlide (p:notes)
type NotesSlide struct {
	CSld       *CommonSlideData `xml:"http://schemas.openxmlformats.org/presentationml/2006/main cSld,omitempty"`
	ClrMapOvr  *dml.ClrMapOvr   `xml:"http://schemas.openxmlformats.org/presentationml/2006/main clrMapOvr,omitempty"`
	ExtLst     *dml.ExtLst      `xml:"http://schemas.openxmlformats.org/presentationml/2006/main extLst,omitempty"`
	ShowMasterSp bool `xml:"showMasterSp,attr,omitempty"`
	ShowMasterPhAnim bool `xml:"showMasterPhAnim,attr,omitempty"`
}

// NotesMaster represents CT_NotesMaster (p:notesMaster)
type NotesMaster struct {
	CSld      *CommonSlideData `xml:"http://schemas.openxmlformats.org/presentationml/2006/main cSld,omitempty"`
	ClrMap    *dml.ClrMap      `xml:"http://schemas.openxmlformats.org/presentationml/2006/main clrMap,omitempty"`
	Hf        *HeaderFooter    `xml:"http://schemas.openxmlformats.org/presentationml/2006/main hf,omitempty"`
	NotesStyle *dml.LstStyle   `xml:"http://schemas.openxmlformats.org/presentationml/2006/main notesStyle,omitempty"`
	ExtLst    *dml.ExtLst      `xml:"http://schemas.openxmlformats.org/presentationml/2006/main extLst,omitempty"`
}

// --- Handout Master ---

// HandoutMaster represents CT_HandoutMaster (p:handoutMaster)
type HandoutMaster struct {
	CSld      *CommonSlideData `xml:"http://schemas.openxmlformats.org/presentationml/2006/main cSld,omitempty"`
	ClrMap    *dml.ClrMap      `xml:"http://schemas.openxmlformats.org/presentationml/2006/main clrMap,omitempty"`
	Hf        *HeaderFooter    `xml:"http://schemas.openxmlformats.org/presentationml/2006/main hf,omitempty"`
	ExtLst    *dml.ExtLst      `xml:"http://schemas.openxmlformats.org/presentationml/2006/main extLst,omitempty"`
}

// HeaderFooter represents CT_HeaderFooter (p:hf)
type HeaderFooter struct {
	SldNum *bool `xml:"sldNum,attr,omitempty"`
	Hdr    *bool `xml:"hdr,attr,omitempty"`
	Ftr    *bool `xml:"ftr,attr,omitempty"`
	Dt     *bool `xml:"dt,attr,omitempty"`
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
