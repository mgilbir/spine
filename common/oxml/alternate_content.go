// Package oxml provides shared Office XML types used across document formats.
package oxml

import (
	"encoding/xml"
	"strings"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// AlternateContentChoice is a single mc:Choice branch of an AlternateContent.
type AlternateContentChoice struct {
	// Requires is the mc:Choice/@Requires attribute. It may list more than one
	// namespace prefix, e.g. "p14 p15".
	Requires string
	// Content is the inner XML of the choice (xsd:any from external schemas).
	Content []byte
}

// AlternateContent represents mc:AlternateContent from ECMA-376 Part 3.
// The mc structure is typed, but the inner content of mc:Choice and mc:Fallback
// is xsd:any from external schemas (p14, p15, etc.) and stored as raw bytes.
type AlternateContent struct {
	// Choices holds every mc:Choice in document order. MCE permits more than one.
	Choices []AlternateContentChoice
	// HasFallback records whether an mc:Fallback element was present, so an
	// empty <mc:Fallback/> round-trips (its absence is semantically different).
	HasFallback bool
	// Fallback is the inner XML of mc:Fallback (xsd:any).
	Fallback []byte
}

// UnmarshalXML implements custom XML unmarshaling for AlternateContent.
// Parses every mc:Choice and the mc:Fallback child, capturing inner content as
// raw bytes.
func (ac *AlternateContent) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			// Choice/Fallback are only meaningful in the mc namespace; an
			// element with the same local name from any other namespace is
			// foreign content and must not be captured as a branch.
			if t.Name.Space != xmlb.NSMarkupCompatibility {
				if err := d.Skip(); err != nil {
					return err
				}
				continue
			}
			switch t.Name.Local {
			case "Choice":
				choice := AlternateContentChoice{}
				for _, attr := range t.Attr {
					if attr.Name.Local == "Requires" {
						choice.Requires = attr.Value
					}
				}
				var inner struct {
					Content []byte `xml:",innerxml"`
				}
				if err := d.DecodeElement(&inner, &t); err != nil {
					return err
				}
				choice.Content = inner.Content
				ac.Choices = append(ac.Choices, choice)
			case "Fallback":
				ac.HasFallback = true
				var inner struct {
					Content []byte `xml:",innerxml"`
				}
				if err := d.DecodeElement(&inner, &t); err != nil {
					return err
				}
				ac.Fallback = inner.Content
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

	// Declare an xmlns for every extension prefix referenced by any choice's
	// Requires (which may list several prefixes). Only declare a prefix that is
	// not already in scope, and register all of them so raw child content using
	// those prefixes resolves correctly. Namespaces that were already declared
	// (e.g. at the root) are left untouched: this element declares nothing for
	// them, so it must not reset their state afterwards either.
	var nsAttrs []xmlb.Attr
	seen := make(map[string]bool)
	for _, choice := range ac.Choices {
		for _, prefix := range strings.Fields(choice.Requires) {
			if seen[prefix] {
				continue
			}
			seen[prefix] = true
			extNS, ok := xmlb.ExtensionPrefixToNS[prefix]
			if !ok {
				continue
			}
			if !b.IsNamespaceDeclared(extNS) {
				nsAttrs = append(nsAttrs, xmlb.Attr{Name: "xmlns:" + prefix, Value: extNS})
			}
			b.RegisterNamespace(extNS, prefix)
		}
	}

	b.StartElementInlineNS(nsMC, prefixMC, "AlternateContent", nsAttrs...)

	for _, choice := range ac.Choices {
		b.StartElement(nsMC, "Choice", xmlb.StrAttr("Requires", choice.Requires))
		b.WriteRaw(choice.Content)
		b.EndElement(nsMC, "Choice")
	}

	// mc:Fallback with xmlns="" (Office convention to reset default NS). Emitted
	// whenever the original had a fallback, even if it was empty.
	if ac.HasFallback {
		b.StartElement(nsMC, "Fallback", xmlb.Attr{Name: "xmlns", Value: ""})
		b.WriteRaw(ac.Fallback)
		b.EndElement(nsMC, "Fallback")
	}

	// The mc declaration is scoped to this element by the Builder and its
	// previous declared-state is restored here. The extension declarations
	// were emitted as plain xmlns attributes (never recorded in the Builder's
	// declared set), so their scope ends with the element as well.
	b.EndElementInlineNS(prefixMC, "AlternateContent")
}
