// Package oxml provides shared Office XML types used across document formats.
package oxml

import (
	"encoding/xml"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// AlternateContent represents mc:AlternateContent from ECMA-376 Part 3.
// The mc structure is typed, but the inner content of mc:Choice and mc:Fallback
// is xsd:any from external schemas (p14, p15, etc.) and stored as raw bytes.
type AlternateContent struct {
	Requires        string // mc:Choice/@Requires (e.g., "p14")
	ChoiceContent   []byte // inner XML of mc:Choice (xsd:any from external schemas)
	FallbackContent []byte // inner XML of mc:Fallback (xsd:any)
}

// UnmarshalXML implements custom XML unmarshaling for AlternateContent.
// Parses mc:Choice and mc:Fallback children, capturing inner content as raw bytes.
func (ac *AlternateContent) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "Choice":
				// Extract Requires attribute
				for _, attr := range t.Attr {
					if attr.Name.Local == "Requires" {
						ac.Requires = attr.Value
					}
				}
				// Capture inner content as raw bytes
				var inner struct {
					Content []byte `xml:",innerxml"`
				}
				if err := d.DecodeElement(&inner, &t); err != nil {
					return err
				}
				ac.ChoiceContent = inner.Content
			case "Fallback":
				var inner struct {
					Content []byte `xml:",innerxml"`
				}
				if err := d.DecodeElement(&inner, &t); err != nil {
					return err
				}
				ac.FallbackContent = inner.Content
			default:
				if err := d.Skip(); err != nil {
					return err
				}
			}
		case xml.EndElement:
			return nil
		}
	}
}

// MarshalToBuilder implements xmlb.BuilderMarshaler for AlternateContent.
// Uses inline namespace declarations matching Office conventions.
func (ac *AlternateContent) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	nsMC := xmlb.NSMarkupCompatibility
	prefixMC := xmlb.PrefixMarkupCompatibility

	// Build inline namespace attrs for mc:AlternateContent element.
	// Always declare xmlns:mc. Also declare extension NS from Requires prefix,
	// but only if it hasn't already been declared at a higher level (e.g., root).
	var nsAttrs []xmlb.Attr
	if ac.Requires != "" {
		if extNS, ok := xmlb.ExtensionPrefixToNS[ac.Requires]; ok {
			if !b.IsNamespaceDeclared(extNS) {
				nsAttrs = append(nsAttrs, xmlb.Attr{Name: "xmlns:" + ac.Requires, Value: extNS})
			}
			// Register so child raw content with this prefix resolves correctly
			b.RegisterNamespace(extNS, ac.Requires)
		}
	}

	b.StartElementInlineNS(nsMC, prefixMC, "AlternateContent", nsAttrs...)

	// mc:Choice with Requires attribute
	b.StartElement(nsMC, "Choice", xmlb.StrAttr("Requires", ac.Requires))
	b.WriteRaw(ac.ChoiceContent)
	b.EndElement(nsMC, "Choice")

	// mc:Fallback with xmlns="" (Office convention to reset default NS)
	// Only emit if fallback content was present in the original.
	if len(ac.FallbackContent) > 0 {
		b.StartElement(nsMC, "Fallback", xmlb.Attr{Name: "xmlns", Value: ""})
		b.WriteRaw(ac.FallbackContent)
		b.EndElement(nsMC, "Fallback")
	}

	b.EndElementInlineNS(prefixMC, "AlternateContent")

	// Reset so next usage gets fresh inline declarations
	b.ResetNamespaceDeclaration(nsMC)
	if ac.Requires != "" {
		if extNS, ok := xmlb.ExtensionPrefixToNS[ac.Requires]; ok {
			b.ResetNamespaceDeclaration(extNS)
		}
	}
}
