package oxml

import (
	"encoding/xml"
	"strings"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// modernNamespaces are the declarations every regenerated modern-comment part
// carries on its root, matching what PowerPoint writes (a, r, p188). Raw child
// blobs (anchor marker lists, body runs, extLst) rely on a and p188 being bound
// here; every extension prefix they use declares itself inline.
func modernNamespaces() []xmlb.NSDecl {
	return []xmlb.NSDecl{
		{Prefix: xmlb.PrefixDrawingML, URI: nsAdml},
		{Prefix: xmlb.PrefixRelationships, URI: nsRel},
		{Prefix: xmlb.PrefixPowerPointComment, URI: nsP188},
	}
}

func newModernBuilder() *xmlb.Builder {
	b := xmlb.NewBuilder()
	b.RegisterNamespace(nsAdml, xmlb.PrefixDrawingML)
	b.RegisterNamespace(nsRel, xmlb.PrefixRelationships)
	b.RegisterNamespace(nsP188, xmlb.PrefixPowerPointComment)
	b.WriteRaw([]byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`))
	return b
}

// Marshal serializes a modernComment part (p188:cmLst with one p188:cm).
func (p *ModernCommentPart) Marshal() []byte {
	b := newModernBuilder()
	b.StartElementWithNS(nsP188, "cmLst", modernNamespaces())
	if p.Comment != nil {
		p.Comment.marshal(b)
	}
	b.EndElement(nsP188, "cmLst")
	_ = b.Finish()
	return b.Bytes()
}

func (c *ModernComment) marshal(b *xmlb.Builder) {
	attrs := []xmlb.Attr{
		xmlb.StrAttr("id", c.ID),
		xmlb.StrAttr("authorId", c.AuthorID),
	}
	if c.Status != "" {
		attrs = append(attrs, xmlb.StrAttr("status", c.Status))
	}
	if c.Created != "" {
		attrs = append(attrs, xmlb.StrAttr("created", c.Created))
	}
	attrs = append(attrs, c.ExtraAttrs...)
	b.StartElement(nsP188, "cm", attrs...)
	for _, raw := range c.PreChildren {
		b.WriteRaw(raw)
	}
	if len(c.Replies) > 0 {
		b.StartElement(nsP188, "replyLst")
		for _, r := range c.Replies {
			r.marshal(b)
		}
		b.EndElement(nsP188, "replyLst")
	}
	marshalBody(b, c.TxBody, c.BodyText)
	for _, raw := range c.PostChildren {
		b.WriteRaw(raw)
	}
	b.EndElement(nsP188, "cm")
}

func (r *ModernReply) marshal(b *xmlb.Builder) {
	attrs := []xmlb.Attr{
		xmlb.StrAttr("id", r.ID),
		xmlb.StrAttr("authorId", r.AuthorID),
	}
	if r.Created != "" {
		attrs = append(attrs, xmlb.StrAttr("created", r.Created))
	}
	attrs = append(attrs, r.ExtraAttrs...)
	b.StartElement(nsP188, "reply", attrs...)
	for _, raw := range r.PreChildren {
		b.WriteRaw(raw)
	}
	marshalBody(b, r.TxBody, r.BodyText)
	for _, raw := range r.PostChildren {
		b.WriteRaw(raw)
	}
	b.EndElement(nsP188, "reply")
}

// marshalBody writes the p188:txBody: the preserved raw body when present,
// otherwise a minimal DrawingML body synthesized from plain text.
func marshalBody(b *xmlb.Builder, raw []byte, text string) {
	if raw != nil {
		b.WriteRaw(raw)
		return
	}
	b.StartElement(nsP188, "txBody")
	b.EmptyElement(nsAdml, "bodyPr")
	b.EmptyElement(nsAdml, "lstStyle")
	paras := strings.Split(text, "\n")
	for _, para := range paras {
		b.StartElement(nsAdml, "p")
		b.StartElement(nsAdml, "r")
		b.EmptyElement(nsAdml, "rPr", xmlb.StrAttr("lang", "en-US"))
		b.WriteElement(nsAdml, "t", para)
		b.EndElement(nsAdml, "r")
		b.EndElement(nsAdml, "p")
	}
	b.EndElement(nsP188, "txBody")
}

// Marshal serializes ppt/authors.xml (p188:authorLst).
func (l *ModernAuthorList) Marshal() []byte {
	b := newModernBuilder()
	b.StartElementWithNS(nsP188, "authorLst", modernNamespaces())
	for _, a := range l.Authors {
		a.marshal(b)
	}
	b.EndElement(nsP188, "authorLst")
	_ = b.Finish()
	return b.Bytes()
}

func (a *ModernAuthor) marshal(b *xmlb.Builder) {
	attrs := []xmlb.Attr{
		xmlb.StrAttr("id", a.ID),
		xmlb.StrAttr("name", a.Name),
		xmlb.StrAttr("initials", a.Initials),
		xmlb.StrAttr("userId", a.UserID),
		xmlb.StrAttr("providerId", a.ProviderID),
	}
	attrs = append(attrs, a.ExtraAttrs...)
	if len(a.RawInner) == 0 {
		b.EmptyElement(nsP188, "author", attrs...)
		return
	}
	b.StartElement(nsP188, "author", attrs...)
	b.WriteRaw(a.RawInner)
	b.EndElement(nsP188, "author")
}

// joinDrawingText extracts the plain text of a DrawingML text body: every a:t
// run concatenated within a paragraph, paragraphs joined by newlines.
func joinDrawingText(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	dec := xml.NewDecoder(strings.NewReader(string(raw)))
	var paras []string
	var cur strings.Builder
	started := false
	inT := false
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "p":
				if started {
					paras = append(paras, cur.String())
					cur.Reset()
				}
				started = true
			case "t":
				inT = true
			case "br":
				cur.WriteByte('\n')
			}
		case xml.EndElement:
			if t.Name.Local == "t" {
				inT = false
			}
		case xml.CharData:
			if inT {
				cur.Write(t)
			}
		}
	}
	if started {
		paras = append(paras, cur.String())
	}
	return strings.Join(paras, "\n")
}
