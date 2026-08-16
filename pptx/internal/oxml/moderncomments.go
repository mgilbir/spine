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
	// OriginalRootAttrs is the verbatim root attribute list of a parsed part,
	// replayed on marshal so the prefixes in every author's preserved inner
	// content keep meaning what they meant in the source. nil for a list this
	// library built, which takes the canonical declarations instead.
	OriginalRootAttrs []xmlb.RootAttr
}

// ModernAuthor models CT_Author (p188:author).
type ModernAuthor struct {
	ID         string // GUID, e.g. "{7E013C82-...}"
	Name       string
	Initials   string
	UserID     string
	ProviderID string
	// ExtraAttrs holds author attributes this library does not model, so they
	// survive a rewrite. It is used only for authors built programmatically; a
	// parsed author replays CapturedAttrs, which already carries them.
	ExtraAttrs []xmlb.Attr
	// CapturedAttrs is the verbatim source attribute list of a parsed author.
	// Replaying it keeps the producer's attribute order and, crucially, does not
	// invent initials=""/userId=""/providerId="" for an author that carried
	// none (C525). nil for authors built programmatically.
	CapturedAttrs []xmlb.RootAttr
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
	// dueDate, assignedTo, complete, title, ...) so they survive a rewrite. It
	// is used only for comments built programmatically; a parsed comment
	// replays CapturedAttrs, which already carries them.
	ExtraAttrs []xmlb.Attr
	// CapturedAttrs is the verbatim source attribute list of a parsed comment,
	// replayed so the producer's attribute order survives (C525). nil for
	// comments built programmatically.
	CapturedAttrs []xmlb.RootAttr
	// HasReplyLst records that the source carried a p188:replyLst element even
	// when it held no reply, so an empty <p188:replyLst/> is not deleted (C525).
	HasReplyLst bool

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
	ID       string
	AuthorID string
	Created  string
	// ExtraAttrs holds reply attributes this library does not model; used only
	// for replies built programmatically (see ModernComment.ExtraAttrs).
	ExtraAttrs []xmlb.Attr
	// CapturedAttrs is the verbatim source attribute list of a parsed reply
	// (see ModernComment.CapturedAttrs).
	CapturedAttrs []xmlb.RootAttr
	TxBody        []byte
	BodyText   string // used only for library-created replies (TxBody == nil)
	// PreChildren / PostChildren preserve raw children around the body.
	PreChildren  [][]byte
	PostChildren [][]byte
}

// ModernCommentPart models one ppt/comments/modernComment*.xml part: a
// p188:cmLst containing exactly one p188:cm (a comment thread).
type ModernCommentPart struct {
	Comment *ModernComment
	// OriginalRootAttrs is the verbatim root attribute list of a parsed part.
	// The comment's anchor markers, body and extLst are all preserved as raw
	// bytes, and raw bytes carry a prefix rather than a namespace: replacing the
	// declarations above them would change what they mean without touching them.
	// nil for a part this library built.
	OriginalRootAttrs []xmlb.RootAttr
}

// ParseModernCommentPart decodes a modernComment part.
//
// It goes through xmlb.Unmarshal rather than xml.Unmarshal because this reads a
// whole part off the package: the part has to be well-formed to its end and has
// to bind every prefix it uses, and encoding/xml enforces neither. Parsing one
// that did not cost 303 comments — an attribute spelled cre0:0ated came back
// with its unknown prefix dropped, the emitted part stopped parsing at that
// byte, and a comment part that fails to parse is absent rather than an error
// (FuzzPptxModernCommentXML).
func ParseModernCommentPart(data []byte) (*ModernCommentPart, error) {
	var lst modernCommentRoot
	if err := xmlb.Unmarshal(data, &lst); err != nil {
		return nil, err
	}
	return &ModernCommentPart{Comment: lst.CM, OriginalRootAttrs: lst.RootAttrs}, nil
}

// modernCommentRoot decodes p188:cmLst, capturing the root's declarations
// alongside the thread so the marshaler can replay them.
type modernCommentRoot struct {
	CM        *ModernComment
	RootAttrs []xmlb.RootAttr
}

func (r *modernCommentRoot) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	// CaptureAttrs, not CaptureAttrsSource: the structured form re-renders each
	// declaration rather than replaying source bytes, which keeps the attribute
	// name guard on the writing path.
	r.RootAttrs = xmlb.CaptureAttrs(start.Attr)
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Space == nsP188 && t.Name.Local == "cm" {
				cm := &ModernComment{}
				if err := cm.UnmarshalXML(d, t); err != nil {
					return err
				}
				r.CM = cm
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

// ParseModernAuthorList decodes ppt/authors.xml.
func ParseModernAuthorList(data []byte) (*ModernAuthorList, error) {
	var lst ModernAuthorList
	if err := xmlb.Unmarshal(data, &lst); err != nil {
		return nil, err
	}
	return &lst, nil
}

// UnmarshalXML decodes p188:authorLst, capturing every author's known and
// unknown attributes plus any inner content.
func (l *ModernAuthorList) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	// See ModernAuthorList.OriginalRootAttrs: an author's RawInner is replayed
	// verbatim, so the declarations it resolves against have to come back too.
	l.OriginalRootAttrs = xmlb.CaptureAttrs(start.Attr)
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
	a.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
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
	c.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
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
				c.HasReplyLst = true
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
	r.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
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

// ModernCommentText returns the plain text of a p188 comment body: the a:t
// runs joined per paragraph, paragraphs joined by newlines.
func ModernCommentText(txBody []byte) string {
	return joinDrawingText(txBody)
}

// Text returns the comment's plain body text, from whichever of the two body
// representations this comment carries: the verbatim txBody of a parsed
// comment, or the BodyText of one this library created and has not serialized
// yet (marshal synthesizes the txBody from it).
//
// Readers have to ask for the text this way rather than reading TxBody. Reading
// TxBody alone answered "" for every comment added in the current session,
// which was invisible while every setter serialized on the spot and the reader
// re-parsed what it wrote.
func (c *ModernComment) Text() string {
	if c.TxBody != nil {
		return ModernCommentText(c.TxBody)
	}
	return c.BodyText
}

// Text returns the reply's plain body text (see ModernComment.Text).
func (r *ModernReply) Text() string {
	if r.TxBody != nil {
		return ModernCommentText(r.TxBody)
	}
	return r.BodyText
}
