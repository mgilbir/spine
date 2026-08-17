package oxml

import (
	"encoding/xml"
	"strconv"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// Namespaces for the SpreadsheetML comment mechanisms.
const (
	// NSSpreadsheetML is the SpreadsheetML main namespace (legacy comments live
	// here, as CT_Comments is part of the base schema).
	nsSpreadsheetML = "http://schemas.openxmlformats.org/spreadsheetml/2006/main"
	// NSThreadedComments is the Microsoft 2018 namespace shared by the threaded
	// comments part and the person-list part.
	NSThreadedComments = "http://schemas.microsoft.com/office/spreadsheetml/2018/threadedcomments"
)

// ---------------------------------------------------------------------------
// Legacy comments (a.k.a. notes) — CT_Comments, section 18.7.
// ---------------------------------------------------------------------------

// CT_Comments models xl/commentsN.xml: an author list plus a list of comments,
// each anchored to a cell and carrying rich text (reusing CT_Rst, the same
// rich-string type used by shared strings and inline cell text).
type CT_Comments struct {
	Authors  []string
	Comments []CT_Comment
}

// CT_Comment is a single legacy comment anchored to a cell reference.
type CT_Comment struct {
	Ref      string
	AuthorID int
	ShapeID  string // optional; links to the VML shape. "" omits the attribute.
	Text     CT_Rst
}

type xmlComments struct {
	XMLName xml.Name `xml:"comments"`
	Authors struct {
		Author []string `xml:"author"`
	} `xml:"authors"`
	CommentList struct {
		Comment []xmlComment `xml:"comment"`
	} `xml:"commentList"`
}

type xmlComment struct {
	Ref      string `xml:"ref,attr"`
	AuthorID int    `xml:"authorId,attr"`
	ShapeID  string `xml:"shapeId,attr"`
	Text     CT_Rst `xml:"text"`
}

// ParseComments unmarshals a legacy comments part.
func ParseComments(data []byte) (*CT_Comments, error) {
	var x xmlComments
	if err := xmlb.Unmarshal(data, &x); err != nil {
		return nil, err
	}
	c := &CT_Comments{Authors: x.Authors.Author}
	for _, xc := range x.CommentList.Comment {
		c.Comments = append(c.Comments, CT_Comment(xc))
	}
	return c, nil
}

// AuthorIndex returns the index of author in the list, adding it if absent.
func (c *CT_Comments) AuthorIndex(author string) int {
	for i, a := range c.Authors {
		if a == author {
			return i
		}
	}
	c.Authors = append(c.Authors, author)
	return len(c.Authors) - 1
}

// MarshalComments serializes a legacy comments part.
func MarshalComments(c *CT_Comments) ([]byte, error) {
	b := xmlb.NewBuilder()
	b.RegisterNamespace(nsSpreadsheetML, "")
	b.WriteHeader()
	b.StartElementWithNS(nsSpreadsheetML, "comments",
		[]xmlb.NSDecl{{Prefix: "", URI: nsSpreadsheetML}})

	b.StartElement(nsSpreadsheetML, "authors")
	for _, a := range c.Authors {
		b.WriteElement(nsSpreadsheetML, "author", a)
	}
	b.EndElement(nsSpreadsheetML, "authors")

	b.StartElement(nsSpreadsheetML, "commentList")
	for i := range c.Comments {
		cm := &c.Comments[i]
		attrs := []xmlb.Attr{
			xmlb.StrAttr("ref", cm.Ref),
			xmlb.StrAttr("authorId", strconv.Itoa(cm.AuthorID)),
		}
		if cm.ShapeID != "" {
			attrs = append(attrs, xmlb.StrAttr("shapeId", cm.ShapeID))
		}
		b.StartElement(nsSpreadsheetML, "comment", attrs...)
		cm.Text.MarshalToBuilder(b, nsSpreadsheetML, "text")
		b.EndElement(nsSpreadsheetML, "comment")
	}
	b.EndElement(nsSpreadsheetML, "commentList")

	b.EndElement(nsSpreadsheetML, "comments")
	if err := b.Finish(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// PlainText flattens the rich text of a legacy comment to a string.
func (c *CT_Comment) PlainText() string {
	return rstPlainText(&c.Text)
}

func rstPlainText(rst *CT_Rst) string {
	if rst == nil {
		return ""
	}
	var s string
	if rst.T != nil {
		s = *rst.T
	}
	for i := range rst.R {
		s += rst.R[i].T
	}
	return s
}

// NewCommentText builds a CT_Rst holding a single unformatted run of text.
func NewCommentText(text string) CT_Rst {
	t := text
	return CT_Rst{T: &t}
}

// ---------------------------------------------------------------------------
// Threaded comments (modern) — CT_ThreadedComments.
// ---------------------------------------------------------------------------

// CT_ThreadedComments models xl/threadedComments/threadedCommentN.xml.
type CT_ThreadedComments struct {
	Comments []CT_ThreadedComment
}

// CT_ThreadedComment is one entry in a threaded-comment part. A root comment has
// an empty ParentID; a reply's ParentID is the root comment's ID. Done marks the
// thread resolved (set on the root comment).
type CT_ThreadedComment struct {
	Ref      string
	DT       string // ISO-8601 timestamp, e.g. 2026-01-15T10:00:00Z
	PersonID string
	ID       string
	ParentID string
	Done     bool
	Text     string
}

type xmlThreadedComments struct {
	XMLName  xml.Name             `xml:"ThreadedComments"`
	Comments []xmlThreadedComment `xml:"threadedComment"`
}

type xmlThreadedComment struct {
	Ref      string `xml:"ref,attr"`
	DT       string `xml:"dT,attr"`
	PersonID string `xml:"personId,attr"`
	ID       string `xml:"id,attr"`
	ParentID string `xml:"parentId,attr"`
	Done     string `xml:"done,attr"`
	Text     string `xml:"text"`
}

// ParseThreadedComments unmarshals a threaded-comments part.
func ParseThreadedComments(data []byte) (*CT_ThreadedComments, error) {
	var x xmlThreadedComments
	if err := xmlb.Unmarshal(data, &x); err != nil {
		return nil, err
	}
	tc := &CT_ThreadedComments{}
	for _, xc := range x.Comments {
		tc.Comments = append(tc.Comments, CT_ThreadedComment{
			Ref:      xc.Ref,
			DT:       xc.DT,
			PersonID: xc.PersonID,
			ID:       xc.ID,
			ParentID: xc.ParentID,
			Done:     xc.Done == "1" || xc.Done == "true",
			Text:     xc.Text,
		})
	}
	return tc, nil
}

// MarshalThreadedComments serializes a threaded-comments part.
func MarshalThreadedComments(tc *CT_ThreadedComments) ([]byte, error) {
	b := xmlb.NewBuilder()
	b.RegisterNamespace(NSThreadedComments, "")
	b.WriteHeader()
	b.StartElementWithNS(NSThreadedComments, "ThreadedComments",
		[]xmlb.NSDecl{{Prefix: "", URI: NSThreadedComments}})
	for i := range tc.Comments {
		c := &tc.Comments[i]
		attrs := []xmlb.Attr{
			xmlb.StrAttr("ref", c.Ref),
			xmlb.StrAttr("dT", c.DT),
			xmlb.StrAttr("personId", c.PersonID),
			xmlb.StrAttr("id", c.ID),
		}
		if c.ParentID != "" {
			attrs = append(attrs, xmlb.StrAttr("parentId", c.ParentID))
		}
		if c.Done {
			attrs = append(attrs, xmlb.StrAttr("done", "1"))
		}
		b.StartElement(NSThreadedComments, "threadedComment", attrs...)
		b.WriteElement(NSThreadedComments, "text", c.Text)
		b.EndElement(NSThreadedComments, "threadedComment")
	}
	b.EndElement(NSThreadedComments, "ThreadedComments")
	if err := b.Finish(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// ---------------------------------------------------------------------------
// Persons — CT_PersonList.
// ---------------------------------------------------------------------------

// CT_PersonList models xl/persons/personN.xml: the authors referenced by
// threaded comments via personId.
type CT_PersonList struct {
	Persons []CT_Person
}

// CT_Person is a single author identity.
type CT_Person struct {
	DisplayName string
	ID          string
	UserID      string
	ProviderID  string
}

type xmlPersonList struct {
	XMLName xml.Name    `xml:"personList"`
	Persons []xmlPerson `xml:"person"`
}

type xmlPerson struct {
	DisplayName string `xml:"displayName,attr"`
	ID          string `xml:"id,attr"`
	UserID      string `xml:"userId,attr"`
	ProviderID  string `xml:"providerId,attr"`
}

// ParsePersonList unmarshals a person-list part.
func ParsePersonList(data []byte) (*CT_PersonList, error) {
	var x xmlPersonList
	if err := xmlb.Unmarshal(data, &x); err != nil {
		return nil, err
	}
	pl := &CT_PersonList{}
	for _, xp := range x.Persons {
		pl.Persons = append(pl.Persons, CT_Person(xp))
	}
	return pl, nil
}

// MarshalPersonList serializes a person-list part.
func MarshalPersonList(pl *CT_PersonList) ([]byte, error) {
	b := xmlb.NewBuilder()
	b.RegisterNamespace(NSThreadedComments, "")
	b.RegisterNamespace(nsSpreadsheetML, "x")
	b.WriteHeader()
	b.StartElementWithNS(NSThreadedComments, "personList",
		[]xmlb.NSDecl{{Prefix: "", URI: NSThreadedComments}, {Prefix: "x", URI: nsSpreadsheetML}})
	for _, p := range pl.Persons {
		attrs := []xmlb.Attr{
			xmlb.StrAttr("displayName", p.DisplayName),
			xmlb.StrAttr("id", p.ID),
		}
		if p.UserID != "" {
			attrs = append(attrs, xmlb.StrAttr("userId", p.UserID))
		}
		if p.ProviderID != "" {
			attrs = append(attrs, xmlb.StrAttr("providerId", p.ProviderID))
		}
		b.EmptyElement(NSThreadedComments, "person", attrs...)
	}
	b.EndElement(NSThreadedComments, "personList")
	if err := b.Finish(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// Find returns the person with the given display name, or nil.
func (pl *CT_PersonList) Find(displayName string) *CT_Person {
	for i := range pl.Persons {
		if pl.Persons[i].DisplayName == displayName {
			return &pl.Persons[i]
		}
	}
	return nil
}

// FindByID returns the person with the given id, or nil.
func (pl *CT_PersonList) FindByID(id string) *CT_Person {
	for i := range pl.Persons {
		if pl.Persons[i].ID == id {
			return &pl.Persons[i]
		}
	}
	return nil
}
