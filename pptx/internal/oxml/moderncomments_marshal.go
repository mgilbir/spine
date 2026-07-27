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

// replayP188Attrs renders a parsed element's verbatim attribute list with the
// modeled fields substituted: source order, unmodeled attributes and the exact
// set of attributes the producer wrote all survive, so re-marshaling does not
// invent initials=""/userId=""/providerId="" or reorder what it did not touch
// (C525).
//
// A modeled field that is empty normally means "not set" and leaves the
// captured value alone. Names listed in clearable invert that: an empty modeled
// value removes the attribute, which is what makes a clearing setter
// (SetResolved(false) dropping @status) actually take effect rather than being
// shadowed by the captured original.
func replayP188Attrs(captured []xmlb.RootAttr, model map[string]string, clearable ...string) []xmlb.Attr {
	override := make(map[string]string, len(model))
	for name, v := range model {
		if v != "" {
			override[name] = v
		}
	}
	attrs := xmlb.RawAttrListOverride(captured, override)
	if len(clearable) == 0 {
		return attrs
	}
	out := attrs[:0]
	for _, a := range attrs {
		drop := false
		for _, name := range clearable {
			if a.Name == name && model[name] == "" {
				drop = true
				break
			}
		}
		if !drop {
			out = append(out, a)
		}
	}
	return out
}

func endP188(b *xmlb.Builder, prefix, localName string) {
	if prefix == "" {
		b.EndElement(nsP188, localName)
		return
	}
	b.EndElementLiteral(prefix, localName)
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
	prefix := ""
	if c.CapturedAttrs == nil {
		attrs = append(attrs, c.ExtraAttrs...)
		b.StartElement(nsP188, "cm", attrs...)
	} else {
		prefix = xmlb.RawAttrPrefix(c.CapturedAttrs, nsP188, xmlb.PrefixPowerPointComment)
		b.StartElementLiteral(prefix, "cm", nil, replayP188Attrs(c.CapturedAttrs, map[string]string{
			"id": c.ID, "authorId": c.AuthorID, "status": c.Status, "created": c.Created,
		}, "status")...)
	}
	for _, raw := range c.PreChildren {
		b.WriteRaw(raw)
	}
	// An empty <p188:replyLst/> in the source is kept: dropping it changed bytes
	// that did not need touching (C525).
	if len(c.Replies) > 0 || c.HasReplyLst {
		if len(c.Replies) == 0 {
			b.EmptyElement(nsP188, "replyLst")
		} else {
			b.StartElement(nsP188, "replyLst")
			for _, r := range c.Replies {
				r.marshal(b)
			}
			b.EndElement(nsP188, "replyLst")
		}
	}
	marshalBody(b, c.TxBody, c.BodyText)
	for _, raw := range c.PostChildren {
		b.WriteRaw(raw)
	}
	endP188(b, prefix, "cm")
}

func (r *ModernReply) marshal(b *xmlb.Builder) {
	attrs := []xmlb.Attr{
		xmlb.StrAttr("id", r.ID),
		xmlb.StrAttr("authorId", r.AuthorID),
	}
	if r.Created != "" {
		attrs = append(attrs, xmlb.StrAttr("created", r.Created))
	}
	prefix := ""
	if r.CapturedAttrs == nil {
		attrs = append(attrs, r.ExtraAttrs...)
		b.StartElement(nsP188, "reply", attrs...)
	} else {
		prefix = xmlb.RawAttrPrefix(r.CapturedAttrs, nsP188, xmlb.PrefixPowerPointComment)
		b.StartElementLiteral(prefix, "reply", nil, replayP188Attrs(r.CapturedAttrs, map[string]string{
			"id": r.ID, "authorId": r.AuthorID, "created": r.Created,
		})...)
	}
	for _, raw := range r.PreChildren {
		b.WriteRaw(raw)
	}
	marshalBody(b, r.TxBody, r.BodyText)
	for _, raw := range r.PostChildren {
		b.WriteRaw(raw)
	}
	endP188(b, prefix, "reply")
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
	if a.CapturedAttrs != nil {
		// Verbatim replay: an author that carried no initials/userId/providerId
		// does not gain initials=""/userId=""/providerId="", and the producer's
		// attribute order survives (C525).
		prefix := xmlb.RawAttrPrefix(a.CapturedAttrs, nsP188, xmlb.PrefixPowerPointComment)
		attrs := replayP188Attrs(a.CapturedAttrs, map[string]string{
			"id": a.ID, "name": a.Name, "initials": a.Initials,
			"userId": a.UserID, "providerId": a.ProviderID,
		})
		if len(a.RawInner) == 0 {
			b.EmptyElementLiteral(prefix, "author", attrs...)
			return
		}
		b.StartElementLiteral(prefix, "author", nil, attrs...)
		b.WriteRaw(a.RawInner)
		b.EndElementLiteral(prefix, "author")
		return
	}
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
