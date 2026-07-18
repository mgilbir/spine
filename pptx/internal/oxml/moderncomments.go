// This file models the Microsoft 2018 threaded ("modern") comment parts:
// the shared author list (ppt/authors.xml, p188:authorLst) and the per-thread
// comment parts (ppt/comments/modernComment*.xml, p188:cmLst). Only the fields
// the public API needs are modeled; every other child of a comment (anchor
// marker lists, position, extLst carrying task details or reactions) is
// preserved verbatim as a raw byte blob so a modified thread keeps the data
// this library does not interpret.

package oxml

import (
	"bytes"
	"encoding/xml"

	xmlb "github.com/mgilbir/spine/common/xml"
)

const (
	nsP188 = xmlb.NSPowerPointComment2018
	nsAdml = xmlb.NSDrawingML
	nsRel  = xmlb.NSOfficeDocumentRels
)

// ModernAuthorList models CT_AuthorList (p188:authorLst) in ppt/authors.xml.
type ModernAuthorList struct {
	Authors []*ModernAuthor
}

// ModernAuthor models CT_Author (p188:author).
type ModernAuthor struct {
	ID         string // GUID, e.g. "{7E013C82-...}"
	Name       string
	Initials   string
	UserID     string
	ProviderID string
	// ExtraAttrs holds author attributes this library does not model, so they
	// survive a rewrite.
	ExtraAttrs []xmlb.Attr
	// Raw preserves the verbatim inner content (e.g. an extLst) of an author
	// element read from disk, so a regenerated author list keeps it.
	RawInner []byte
}

// ModernComment models CT_Comment (p188:cm), the single top-level comment of a
// modernComment part.
type ModernComment struct {
	ID       string
	AuthorID string
	Created  string
	Status   string // "" (active) or "resolved"

	// ExtraAttrs holds cm attributes this library does not model (startDate,
	// dueDate, assignedTo, complete, title, ...) so they survive a rewrite.
	ExtraAttrs []xmlb.Attr

	// PreChildren are the raw children emitted before the reply list and the
	// comment body (anchor marker list, p188:pos, ...), in source order.
	PreChildren [][]byte
	// Replies are the threaded replies (p188:replyLst/p188:reply).
	Replies []*ModernReply
	// TxBody is the raw <p188:txBody>...</p188:txBody> of the top-level comment
	// (nil when the source had none). Library-created comments set BodyText
	// instead and TxBody is synthesized on marshal.
	TxBody   []byte
	BodyText string // used only for library-created comments (TxBody == nil)
	// PostChildren are raw children emitted after the comment body (extLst).
	PostChildren [][]byte
}

// ModernReply models CT_Reply (p188:reply).
type ModernReply struct {
	ID         string
	AuthorID   string
	Created    string
	ExtraAttrs []xmlb.Attr
	TxBody     []byte
	BodyText   string // used only for library-created replies (TxBody == nil)
	// PreChildren / PostChildren preserve raw children around the body.
	PreChildren  [][]byte
	PostChildren [][]byte
}

// ModernCommentPart models one ppt/comments/modernComment*.xml part: a
// p188:cmLst containing exactly one p188:cm (a comment thread).
type ModernCommentPart struct {
	Comment *ModernComment
}

// ParseModernCommentPart decodes a modernComment part.
func ParseModernCommentPart(data []byte) (*ModernCommentPart, error) {
	var lst struct {
		XMLName xml.Name
		CM      *ModernComment `xml:"http://schemas.microsoft.com/office/powerpoint/2018/8/main cm"`
	}
	if err := xml.Unmarshal(data, &lst); err != nil {
		return nil, err
	}
	return &ModernCommentPart{Comment: lst.CM}, nil
}

// ParseModernAuthorList decodes ppt/authors.xml.
func ParseModernAuthorList(data []byte) (*ModernAuthorList, error) {
	var lst ModernAuthorList
	if err := xml.Unmarshal(data, &lst); err != nil {
		return nil, err
	}
	return &lst, nil
}

// UnmarshalXML decodes p188:authorLst, capturing every author's known and
// unknown attributes plus any inner content.
func (l *ModernAuthorList) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "author" {
				a := &ModernAuthor{}
				if err := a.decode(d, t); err != nil {
					return err
				}
				l.Authors = append(l.Authors, a)
			} else {
				if err := d.Skip(); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if t.Name == start.Name {
				return nil
			}
		}
	}
}

func (a *ModernAuthor) decode(d *xml.Decoder, start xml.StartElement) error {
	for _, at := range start.Attr {
		if at.Name.Space == "xmlns" || (at.Name.Space == "" && at.Name.Local == "xmlns") {
			continue
		}
		switch at.Name.Local {
		case "id":
			a.ID = at.Value
		case "name":
			a.Name = at.Value
		case "initials":
			a.Initials = at.Value
		case "userId":
			a.UserID = at.Value
		case "providerId":
			a.ProviderID = at.Value
		default:
			a.ExtraAttrs = append(a.ExtraAttrs, xmlb.StrAttr(at.Name.Local, at.Value))
		}
	}
	var inner struct {
		Content []byte `xml:",innerxml"`
	}
	if err := d.DecodeElement(&inner, &start); err != nil {
		return err
	}
	if len(bytes.TrimSpace(inner.Content)) != 0 {
		a.RawInner = inner.Content
	}
	return nil
}

// UnmarshalXML decodes p188:cm, modeling id/authorId/created/status and the
// reply list, preserving every other child verbatim.
func (c *ModernComment) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, at := range start.Attr {
		if at.Name.Space == "xmlns" || (at.Name.Space == "" && at.Name.Local == "xmlns") {
			continue
		}
		switch at.Name.Local {
		case "id":
			c.ID = at.Value
		case "authorId":
			c.AuthorID = at.Value
		case "created":
			c.Created = at.Value
		case "status":
			c.Status = at.Value
		default:
			c.ExtraAttrs = append(c.ExtraAttrs, xmlb.StrAttr(at.Name.Local, at.Value))
		}
	}
	seenBody := false
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch {
			case t.Name.Space == nsP188 && t.Name.Local == "replyLst":
				if err := c.decodeReplyLst(d, t); err != nil {
					return err
				}
			case t.Name.Space == nsP188 && t.Name.Local == "txBody":
				raw, err := captureRaw(d, t)
				if err != nil {
					return err
				}
				c.TxBody = raw
				seenBody = true
			default:
				raw, err := captureRaw(d, t)
				if err != nil {
					return err
				}
				if seenBody {
					c.PostChildren = append(c.PostChildren, raw)
				} else {
					c.PreChildren = append(c.PreChildren, raw)
				}
			}
		case xml.EndElement:
			if t.Name == start.Name {
				return nil
			}
		}
	}
}

func (c *ModernComment) decodeReplyLst(d *xml.Decoder, start xml.StartElement) error {
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Space == nsP188 && t.Name.Local == "reply" {
				r := &ModernReply{}
				if err := r.decode(d, t); err != nil {
					return err
				}
				c.Replies = append(c.Replies, r)
			} else if err := d.Skip(); err != nil {
				return err
			}
		case xml.EndElement:
			if t.Name == start.Name {
				return nil
			}
		}
	}
}

func (r *ModernReply) decode(d *xml.Decoder, start xml.StartElement) error {
	for _, at := range start.Attr {
		if at.Name.Space == "xmlns" || (at.Name.Space == "" && at.Name.Local == "xmlns") {
			continue
		}
		switch at.Name.Local {
		case "id":
			r.ID = at.Value
		case "authorId":
			r.AuthorID = at.Value
		case "created":
			r.Created = at.Value
		default:
			r.ExtraAttrs = append(r.ExtraAttrs, xmlb.StrAttr(at.Name.Local, at.Value))
		}
	}
	seenBody := false
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			raw, err := captureRaw(d, t)
			if err != nil {
				return err
			}
			switch {
			case t.Name.Space == nsP188 && t.Name.Local == "txBody":
				r.TxBody = raw
				seenBody = true
			case seenBody:
				r.PostChildren = append(r.PostChildren, raw)
			default:
				r.PreChildren = append(r.PreChildren, raw)
			}
		case xml.EndElement:
			if t.Name == start.Name {
				return nil
			}
		}
	}
}

// captureRaw reconstructs the verbatim XML of a child element, preserving its
// inline namespace declarations and inner content.
func captureRaw(d *xml.Decoder, start xml.StartElement) ([]byte, error) {
	var inner struct {
		Content []byte `xml:",innerxml"`
	}
	if err := d.DecodeElement(&inner, &start); err != nil {
		return nil, err
	}
	return encodeRawChild(start, inner.Content), nil
}

// ModernCommentText returns the plain text of a p188 comment body: the a:t
// runs joined per paragraph, paragraphs joined by newlines.
func ModernCommentText(txBody []byte) string {
	return joinDrawingText(txBody)
}
