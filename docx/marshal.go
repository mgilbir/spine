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
	if err := b.Finish(); err != nil {
		return nil, fmt.Errorf("docx: marshal document.xml: %w", err)
	}
	return b.Bytes(), nil
}

// marshalStylesXML marshals styles to XML.
func marshalStylesXML(styles *oxml.CT_Styles) ([]byte, error) {
	b := xmlb.NewWordprocessingMLBuilder()
	b.WriteHeader()
	b.MarshalRoot(nsW, "styles", styles, xmlb.WordprocessingMLNamespaces())
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
