package docx

import (
	xmlb "github.com/mgilbir/spine/common/xml"
	"github.com/mgilbir/spine/docx/internal/oxml"
)

const (
	nsW = xmlb.NSWordprocessingML
	nsR = xmlb.NSPresentationRels
)

// marshalDocumentXML marshals a document to XML.
func marshalDocumentXML(doc *oxml.CT_Document) []byte {
	b := xmlb.NewWordprocessingMLBuilder()
	b.WriteHeader()

	var attrs []xmlb.Attr
	if doc.Conformance != "" {
		attrs = append(attrs, xmlb.StrAttr(xmlb.PrefixMarkupCompatibility+":Ignorable", doc.Conformance))
	}

	// Use original namespace declarations if available (for round-trip fidelity),
	// otherwise use the standard set.
	nsDecls := xmlb.WordprocessingMLNamespaces()
	if len(doc.OriginalNSDecls) > 0 {
		nsDecls = doc.OriginalNSDecls
	}

	b.StartElementWithNS(nsW, "document", nsDecls, attrs...)

	if doc.Background != nil {
		b.MarshalElement(nsW, "background", doc.Background)
	}
	if doc.Body != nil {
		doc.Body.MarshalToBuilder(b, nsW, "body")
	}

	b.EndElement(nsW, "document")
	return b.Bytes()
}

// marshalStylesXML marshals styles to XML.
func marshalStylesXML(styles *oxml.CT_Styles) []byte {
	b := xmlb.NewWordprocessingMLBuilder()
	b.WriteHeader()
	b.MarshalRoot(nsW, "styles", styles, xmlb.WordprocessingMLNamespaces())
	return b.Bytes()
}

// marshalNumberingXML marshals numbering definitions to XML.
func marshalNumberingXML(numbering *oxml.CT_Numbering) []byte {
	b := xmlb.NewWordprocessingMLBuilder()
	b.WriteHeader()
	b.MarshalRoot(nsW, "numbering", numbering, xmlb.WordprocessingMLNamespaces())
	return b.Bytes()
}

// marshalHeaderXML marshals a header/footer to XML.
func marshalHeaderXML(hf *oxml.CT_HdrFtr, localName string) []byte {
	b := xmlb.NewWordprocessingMLBuilder()
	b.WriteHeader()

	b.StartElementWithNS(nsW, localName, xmlb.WordprocessingMLNamespaces())
	marshalBodyContentHelper(b, hf)
	b.EndElement(nsW, localName)

	return b.Bytes()
}

// marshalBodyContentHelper writes body-level content for a header/footer.
func marshalBodyContentHelper(b *xmlb.Builder, hf *oxml.CT_HdrFtr) {
	hf.MarshalToBuilder(b, nsW, "")
}

// marshalFootnotesXML marshals footnotes to XML.
func marshalFootnotesXML(fn *oxml.CT_Footnotes) []byte {
	b := xmlb.NewWordprocessingMLBuilder()
	b.WriteHeader()
	b.StartElementWithNS(nsW, "footnotes", xmlb.WordprocessingMLNamespaces())
	for _, f := range fn.Footnote {
		f.MarshalToBuilder(b, nsW, "footnote")
	}
	b.EndElement(nsW, "footnotes")
	return b.Bytes()
}

// marshalEndnotesXML marshals endnotes to XML.
func marshalEndnotesXML(en *oxml.CT_Endnotes) []byte {
	b := xmlb.NewWordprocessingMLBuilder()
	b.WriteHeader()
	b.StartElementWithNS(nsW, "endnotes", xmlb.WordprocessingMLNamespaces())
	for _, e := range en.Endnote {
		e.MarshalToBuilder(b, nsW, "endnote")
	}
	b.EndElement(nsW, "endnotes")
	return b.Bytes()
}

// marshalCommentsXML marshals comments to XML.
func marshalCommentsXML(comments *oxml.CT_Comments) []byte {
	b := xmlb.NewWordprocessingMLBuilder()
	b.WriteHeader()
	b.StartElementWithNS(nsW, "comments", xmlb.WordprocessingMLNamespaces())
	for _, c := range comments.Comment {
		c.MarshalToBuilder(b, nsW, "comment")
	}
	b.EndElement(nsW, "comments")
	return b.Bytes()
}
