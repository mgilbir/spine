package oxml

import (
	"encoding/xml"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// CT_CommentsEx is the root of word/commentsExtended.xml (w15:commentsEx). It
// carries the threading and resolved state that comments.xml does not: each
// entry links a comment (by the w14:paraId of its last body paragraph) to its
// parent and records whether the thread is marked done.
type CT_CommentsEx struct {
	CommentEx         []*CT_CommentEx `xml:"-"`
	OriginalNSDecls   []xmlb.NSDecl   `xml:"-"`
	OriginalRootAttrs []xmlb.RootAttr `xml:"-"`
}

// CT_CommentEx is a single w15:commentEx entry.
type CT_CommentEx struct {
	ParaId       string `xml:"-"`
	ParaIdParent string `xml:"-"`
	Done         string `xml:"-"`
	// CapturedAttrs preserves the verbatim source attribute list (order and any
	// unmodeled attributes); replayed on marshal.
	CapturedAttrs []xmlb.RootAttr `xml:"-"`
}

// UnmarshalXML captures the root namespace declarations and decodes the
// commentEx children.
func (c *CT_CommentsEx) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	c.OriginalRootAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	c.OriginalNSDecls = captureNSDecls(start.Attr)
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "commentEx" {
				ce := &CT_CommentEx{}
				ce.CapturedAttrs = xmlb.CaptureAttrsSource(d, t.Attr)
				for _, attr := range t.Attr {
					switch attr.Name.Local {
					case "paraId":
						ce.ParaId = attr.Value
					case "paraIdParent":
						ce.ParaIdParent = attr.Value
					case "done":
						ce.Done = attr.Value
					}
				}
				if err := d.Skip(); err != nil {
					return err
				}
				c.CommentEx = append(c.CommentEx, ce)
			} else if err := d.Skip(); err != nil {
				return err
			}
		case xml.EndElement:
			return nil
		}
	}
}

// Find returns the commentEx entry for the given paraId, or nil.
func (c *CT_CommentsEx) Find(paraID string) *CT_CommentEx {
	for _, ce := range c.CommentEx {
		if ce.ParaId == paraID {
			return ce
		}
	}
	return nil
}

// IsDone reports whether the done attribute is a truthy on/off value ("1" or
// "true").
func (ce *CT_CommentEx) IsDone() bool {
	return ce.Done == "1" || ce.Done == "true" || ce.Done == "on"
}

// CT_People is the root of word/people.xml (w15:people), the registry of
// comment authors.
type CT_People struct {
	Person            []*CT_Person    `xml:"-"`
	OriginalNSDecls   []xmlb.NSDecl   `xml:"-"`
	OriginalRootAttrs []xmlb.RootAttr `xml:"-"`
}

// CT_Person is a single w15:person entry.
type CT_Person struct {
	Author        string          `xml:"-"`
	CapturedAttrs []xmlb.RootAttr `xml:"-"`
	// PresenceInfo captures the optional w15:presenceInfo child verbatim; nil
	// when absent.
	PresenceInfo *CT_PresenceInfo `xml:"-"`
}

// CT_PresenceInfo captures the w15:presenceInfo child attributes.
type CT_PresenceInfo struct {
	ProviderId    string          `xml:"-"`
	UserId        string          `xml:"-"`
	CapturedAttrs []xmlb.RootAttr `xml:"-"`
}

// UnmarshalXML captures the root namespaces and decodes the person children.
func (p *CT_People) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	p.OriginalRootAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	p.OriginalNSDecls = captureNSDecls(start.Attr)
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "person" {
				person := &CT_Person{}
				if err := person.decode(d, t); err != nil {
					return err
				}
				p.Person = append(p.Person, person)
			} else if err := d.Skip(); err != nil {
				return err
			}
		case xml.EndElement:
			return nil
		}
	}
}

func (person *CT_Person) decode(d *xml.Decoder, start xml.StartElement) error {
	person.CapturedAttrs = xmlb.CaptureAttrsSource(d, start.Attr)
	for _, attr := range start.Attr {
		if attr.Name.Local == "author" {
			person.Author = attr.Value
		}
	}
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "presenceInfo" {
				pi := &CT_PresenceInfo{CapturedAttrs: xmlb.CaptureAttrsSource(d, t.Attr)}
				for _, attr := range t.Attr {
					switch attr.Name.Local {
					case "providerId":
						pi.ProviderId = attr.Value
					case "userId":
						pi.UserId = attr.Value
					}
				}
				if err := d.Skip(); err != nil {
					return err
				}
				person.PresenceInfo = pi
			} else if err := d.Skip(); err != nil {
				return err
			}
		case xml.EndElement:
			return nil
		}
	}
}

// Has reports whether a person with the given author name is registered.
func (p *CT_People) Has(author string) bool {
	for _, person := range p.Person {
		if person.Author == author {
			return true
		}
	}
	return false
}

// captureNSDecls extracts the xmlns declarations from a start element's
// attribute list (both prefixed and default forms).
func captureNSDecls(attrs []xml.Attr) []xmlb.NSDecl {
	var decls []xmlb.NSDecl
	for _, attr := range attrs {
		switch {
		case attr.Name.Space == "xmlns":
			decls = append(decls, xmlb.NSDecl{Prefix: attr.Name.Local, URI: attr.Value})
		case attr.Name.Space == "" && attr.Name.Local == "xmlns":
			decls = append(decls, xmlb.NSDecl{Prefix: "", URI: attr.Value})
		}
	}
	return decls
}
