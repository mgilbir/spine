package docx

import (
	xmlb "github.com/mgilbir/spine/common/xml"
	"github.com/mgilbir/spine/docx/internal/oxml"
)

const nsW = xmlb.NSWordprocessingML

// marshalDocumentXML marshals a document to XML.
func marshalDocumentXML(doc *oxml.CT_Document) []byte {
	b := xmlb.NewWordprocessingMLBuilder()
	b.WriteHeader()

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

