package docx

import (
	"fmt"

	xmlb "github.com/mgilbir/spine/common/xml"
	"github.com/mgilbir/spine/docx/internal/oxml"
)

const nsW = xmlb.NSWordprocessingML

// marshalDocumentXML marshals a document to XML.
func marshalDocumentXML(doc *oxml.CT_Document) ([]byte, error) {
	b := xmlb.NewWordprocessingMLBuilder()
	b.SetSelfClosingSpace(doc.SelfClosingSpace)
	b.SetCollapseEmptyElements(doc.CollapseEmpty)
	b.WriteProlog(doc.Prolog)

	var attrs []xmlb.Attr
	if doc.Conformance != "" {
		attrs = append(attrs, xmlb.StrAttr("w:conformance", doc.Conformance))
	}
	if doc.Ignorable != "" {
		attrs = append(attrs, xmlb.StrAttr(xmlb.PrefixMarkupCompatibility+":Ignorable", doc.Ignorable))
	}

	// Use original namespace declarations if available (for round-trip fidelity),
	// otherwise use the standard set.
	nsDecls := xmlb.WordprocessingMLNamespaces()
	if len(doc.OriginalNSDecls) > 0 {
		nsDecls = doc.OriginalNSDecls
	}

	// The captured m:oMath/m:oMathPara content is re-emitted prefixed, so the
	// math namespace must be bound. If the original declarations already bind
	// it, reuse their prefix (whatever it is); otherwise, when the document
	// contains math, add the declaration to the root. Documents without math
	// keep their declarations byte-identical.
	mathDeclared := false
	for _, decl := range nsDecls {
		if decl.URI == xmlb.NSMath {
			mathDeclared = true
			if decl.Prefix != "" {
				b.RegisterNamespace(xmlb.NSMath, decl.Prefix)
			}
			break
		}
	}
	if !mathDeclared && doc.ContainsMath() {
		nsDecls = append(append([]xmlb.NSDecl{}, nsDecls...),
			xmlb.NSDecl{Prefix: xmlb.PrefixMath, URI: xmlb.NSMath})
	}

	if doc.OriginalRootAttrs != nil {
		// Verbatim root replay: keeps xmlns="", xml:space, and the source's
		// declaration/attribute interleaving. A math declaration the source
		// lacked is still appended when captured math requires it.
		var extra []xmlb.Attr
		if !mathDeclared && doc.ContainsMath() {
			extra = append(extra, xmlb.Attr{Name: "xmlns:" + xmlb.PrefixMath, Value: xmlb.NSMath})
		}
		b.StartElementWithRootAttrs(nsW, "document", doc.OriginalRootAttrs, extra...)
	} else {
		b.StartElementWithNS(nsW, "document", nsDecls, attrs...)
	}

	for _, raw := range doc.RootExtras[0] {
		b.WriteRaw(raw)
	}
	if doc.Background != nil {
		b.MarshalElement(nsW, "background", doc.Background)
	}
	for _, raw := range doc.RootExtras[1] {
		b.WriteRaw(raw)
	}
	if doc.Body != nil {
		doc.Body.MarshalToBuilder(b, nsW, "body")
	}
	for _, raw := range doc.RootExtras[2] {
		b.WriteRaw(raw)
	}

	b.EndElement(nsW, "document")
	b.WriteTrailer(doc.Prolog)
	if err := b.Finish(); err != nil {
		return nil, fmt.Errorf("docx: marshal document.xml: %w", err)
	}
	return b.Bytes(), nil
}

// commentsNamespaces is the standard root declaration set for a comments part
// created from scratch: w plus the extension prefixes the comment bodies and
// their extensions reference (w14:paraId, w15 threading). mc: comes with
// WordprocessingMLNamespaces and must not be added again — a second xmlns:mc
// on the same element is a duplicate attribute, which is malformed XML that
// Go's decoder accepts and libxml2 does not.
func commentsNamespaces() []xmlb.NSDecl {
	return append(xmlb.WordprocessingMLNamespaces(),
		xmlb.NSDecl{Prefix: xmlb.PrefixWord2010, URI: xmlb.NSWord2010},
		xmlb.NSDecl{Prefix: xmlb.PrefixWord2012, URI: xmlb.NSWord2012},
	)
}

// commentsExtNamespaces is the standard root declaration set for the
// commentsExtended and people parts (both w15).
func commentsExtNamespaces() []xmlb.NSDecl {
	return []xmlb.NSDecl{
		{Prefix: xmlb.PrefixWordprocessingML, URI: xmlb.NSWordprocessingML},
		{Prefix: xmlb.PrefixMarkupCompatibility, URI: xmlb.NSMarkupCompatibility},
		{Prefix: xmlb.PrefixWord2010, URI: xmlb.NSWord2010},
		{Prefix: xmlb.PrefixWord2012, URI: xmlb.NSWord2012},
	}
}

// marshalCommentsXML marshals the comments part (word/comments.xml). A part
// parsed from an opened package replays its captured root attributes verbatim,
// with the w14/w15 declarations backfilled if the source lacked them (a newly
// added comment carries a w14:paraId that must resolve).
func marshalCommentsXML(comments *oxml.CT_Comments) ([]byte, error) {
	b := xmlb.NewWordprocessingMLBuilder()
	b.WriteHeader()
	if comments.OriginalRootAttrs != nil {
		var extra []xmlb.Attr
		if !nsDeclsHave(comments.OriginalNSDecls, xmlb.NSWord2010) {
			extra = append(extra, xmlb.Attr{Name: "xmlns:" + xmlb.PrefixWord2010, Value: xmlb.NSWord2010})
		}
		if !nsDeclsHave(comments.OriginalNSDecls, xmlb.NSWord2012) {
			extra = append(extra, xmlb.Attr{Name: "xmlns:" + xmlb.PrefixWord2012, Value: xmlb.NSWord2012})
		}
		b.StartElementWithRootAttrs(nsW, "comments", comments.OriginalRootAttrs, extra...)
	} else {
		// An authored part declares w14 and w15 and then writes w14:paraId on
		// the comment's paragraphs. Declaring a namespace is not what licenses
		// a consumer to skip markup in it — mc:Ignorable is — so without this
		// the part is one a strict consumer must reject, and the schema says so
		// (w14:paraId "is not allowed"). Word writes the same list for the same
		// reason. A parsed part replays whatever its producer wrote instead.
		b.StartElementWithNS(nsW, "comments", commentsNamespaces(),
			xmlb.StrAttr(xmlb.PrefixMarkupCompatibility+":Ignorable",
				xmlb.PrefixWord2010+" "+xmlb.PrefixWord2012))
	}
	for _, c := range comments.Comment {
		c.MarshalToBuilder(b, nsW, "comment")
	}
	b.EndElement(nsW, "comments")
	if err := b.Finish(); err != nil {
		return nil, fmt.Errorf("docx: marshal comments.xml: %w", err)
	}
	return b.Bytes(), nil
}

// marshalCommentsExtendedXML marshals word/commentsExtended.xml (w15).
func marshalCommentsExtendedXML(ext *oxml.CT_CommentsEx) ([]byte, error) {
	const nsW15 = xmlb.NSWord2012
	b := xmlb.NewWordprocessingMLBuilder()
	b.WriteHeader()
	if ext.OriginalRootAttrs != nil {
		b.StartElementWithRootAttrs(nsW15, "commentsEx", ext.OriginalRootAttrs)
	} else {
		b.StartElementWithNS(nsW15, "commentsEx", commentsExtNamespaces())
	}
	for _, ce := range ext.CommentEx {
		attrs := []xmlb.Attr{{Namespace: nsW15, Name: "paraId", Value: ce.ParaId}}
		if ce.ParaIdParent != "" {
			attrs = append(attrs, xmlb.Attr{Namespace: nsW15, Name: "paraIdParent", Value: ce.ParaIdParent})
		}
		if ce.Done != "" {
			attrs = append(attrs, xmlb.Attr{Namespace: nsW15, Name: "done", Value: ce.Done})
		}
		b.EmptyElement(nsW15, "commentEx", attrs...)
	}
	b.EndElement(nsW15, "commentsEx")
	if err := b.Finish(); err != nil {
		return nil, fmt.Errorf("docx: marshal commentsExtended.xml: %w", err)
	}
	return b.Bytes(), nil
}

// marshalPeopleXML marshals word/people.xml (w15), the comment-author registry.
func marshalPeopleXML(people *oxml.CT_People) ([]byte, error) {
	const nsW15 = xmlb.NSWord2012
	b := xmlb.NewWordprocessingMLBuilder()
	b.WriteHeader()
	if people.OriginalRootAttrs != nil {
		b.StartElementWithRootAttrs(nsW15, "people", people.OriginalRootAttrs)
	} else {
		b.StartElementWithNS(nsW15, "people", commentsExtNamespaces())
	}
	for _, person := range people.Person {
		// The captured attribute lists are replayed rather than discarded: the
		// model types only w15:author (and providerId/userId on the child), so
		// rebuilding the element from those alone dropped every other attribute
		// a producer wrote and re-ordered the ones it kept (C500). Modeled
		// values still win, so an edited author is authoritative.
		b.StartElement(nsW15, "person", b.ReplayCapturedAttrs(person.CapturedAttrs,
			[]xmlb.Attr{{Namespace: nsW15, Name: "author", Value: person.Author}})...)
		if pi := person.PresenceInfo; pi != nil {
			b.EmptyElement(nsW15, "presenceInfo", b.ReplayCapturedAttrs(pi.CapturedAttrs, []xmlb.Attr{
				{Namespace: nsW15, Name: "providerId", Value: pi.ProviderId},
				{Namespace: nsW15, Name: "userId", Value: pi.UserId},
			})...)
		}
		b.EndElement(nsW15, "person")
	}
	b.EndElement(nsW15, "people")
	if err := b.Finish(); err != nil {
		return nil, fmt.Errorf("docx: marshal people.xml: %w", err)
	}
	return b.Bytes(), nil
}

// marshalFootnotesXML marshals the footnotes part (word/footnotes.xml). A part
// parsed from an opened package replays its captured root attributes verbatim;
// one created from scratch gets the standard WordprocessingML declarations.
func marshalFootnotesXML(footnotes *oxml.CT_Footnotes) ([]byte, error) {
	b := xmlb.NewWordprocessingMLBuilder()
	b.WriteHeader()
	if footnotes.OriginalRootAttrs != nil {
		b.StartElementWithRootAttrs(nsW, "footnotes", footnotes.OriginalRootAttrs)
	} else {
		b.StartElementWithNS(nsW, "footnotes", xmlb.WordprocessingMLNamespaces())
	}
	for _, f := range footnotes.Footnote {
		f.MarshalToBuilder(b, nsW, "footnote")
	}
	b.EndElement(nsW, "footnotes")
	if err := b.Finish(); err != nil {
		return nil, fmt.Errorf("docx: marshal footnotes.xml: %w", err)
	}
	return b.Bytes(), nil
}

// marshalEndnotesXML marshals the endnotes part (word/endnotes.xml); see
// marshalFootnotesXML.
func marshalEndnotesXML(endnotes *oxml.CT_Endnotes) ([]byte, error) {
	b := xmlb.NewWordprocessingMLBuilder()
	b.WriteHeader()
	if endnotes.OriginalRootAttrs != nil {
		b.StartElementWithRootAttrs(nsW, "endnotes", endnotes.OriginalRootAttrs)
	} else {
		b.StartElementWithNS(nsW, "endnotes", xmlb.WordprocessingMLNamespaces())
	}
	for _, e := range endnotes.Endnote {
		e.MarshalToBuilder(b, nsW, "endnote")
	}
	b.EndElement(nsW, "endnotes")
	if err := b.Finish(); err != nil {
		return nil, fmt.Errorf("docx: marshal endnotes.xml: %w", err)
	}
	return b.Bytes(), nil
}

// nsDeclsHave reports whether decls declares the given namespace URI.
func nsDeclsHave(decls []xmlb.NSDecl, uri string) bool {
	for _, d := range decls {
		if d.URI == uri {
			return true
		}
	}
	return false
}

// marshalStylesXML marshals styles to XML. A part parsed from an opened package
// replays its captured root attributes verbatim; hardcoding the standard
// three-declaration set instead dropped mc:Ignorable and every extension
// declaration the part's captured children reference (C370). One created from
// scratch gets the standard set.
func marshalStylesXML(styles *oxml.CT_Styles) ([]byte, error) {
	b := xmlb.NewWordprocessingMLBuilder()
	b.WriteHeader()
	if styles.OriginalRootAttrs != nil {
		b.StartElementWithRootAttrs(nsW, "styles", styles.OriginalRootAttrs)
		b.MarshalChildren(nsW, styles)
		b.EndElement(nsW, "styles")
	} else {
		b.MarshalRoot(nsW, "styles", styles, xmlb.WordprocessingMLNamespaces())
	}
	if err := b.Finish(); err != nil {
		return nil, fmt.Errorf("docx: marshal styles.xml: %w", err)
	}
	return b.Bytes(), nil
}

// marshalNumberingXML marshals numbering definitions to XML. A part parsed
// from an opened package keeps its original root namespace declarations and
// mc:Ignorable so the raw-preserved children still resolve their prefixes.
func marshalNumberingXML(numbering *oxml.CT_Numbering) ([]byte, error) {
	b := xmlb.NewWordprocessingMLBuilder()
	b.WriteHeader()
	var attrs []xmlb.Attr
	if numbering.Ignorable != "" {
		attrs = append(attrs, xmlb.StrAttr(xmlb.PrefixMarkupCompatibility+":Ignorable", numbering.Ignorable))
	}
	nsDecls := xmlb.WordprocessingMLNamespaces()
	if len(numbering.OriginalNSDecls) > 0 {
		nsDecls = numbering.OriginalNSDecls
	}
	b.StartElementWithNS(nsW, "numbering", nsDecls, attrs...)
	numbering.MarshalContent(b, nsW)
	b.EndElement(nsW, "numbering")
	if err := b.Finish(); err != nil {
		return nil, fmt.Errorf("docx: marshal numbering.xml: %w", err)
	}
	return b.Bytes(), nil
}

// marshalSettingsXML marshals the settings part to XML (see marshalNumberingXML
// for the root-attribute handling).
func marshalSettingsXML(settings *oxml.CT_Settings) ([]byte, error) {
	b := xmlb.NewWordprocessingMLBuilder()
	b.WriteHeader()
	if settings.OriginalRootAttrs != nil {
		// Verbatim root replay preserves every source attribute (all namespace
		// declarations, mc:Ignorable, and any others like xml:space) in order.
		b.StartElementWithRootAttrs(nsW, "settings", settings.OriginalRootAttrs)
		settings.MarshalContent(b, nsW)
		b.EndElement(nsW, "settings")
		if err := b.Finish(); err != nil {
			return nil, fmt.Errorf("docx: marshal settings.xml: %w", err)
		}
		return b.Bytes(), nil
	}
	var attrs []xmlb.Attr
	if settings.Ignorable != "" {
		attrs = append(attrs, xmlb.StrAttr(xmlb.PrefixMarkupCompatibility+":Ignorable", settings.Ignorable))
	}
	nsDecls := xmlb.WordprocessingMLNamespaces()
	if len(settings.OriginalNSDecls) > 0 {
		nsDecls = settings.OriginalNSDecls
	}
	b.StartElementWithNS(nsW, "settings", nsDecls, attrs...)
	settings.MarshalContent(b, nsW)
	b.EndElement(nsW, "settings")
	if err := b.Finish(); err != nil {
		return nil, fmt.Errorf("docx: marshal settings.xml: %w", err)
	}
	return b.Bytes(), nil
}
