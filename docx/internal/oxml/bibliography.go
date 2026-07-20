package oxml

import (
	"encoding/xml"
	"strings"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// CT_Sources is the root of the bibliography sources part
// (word/bibliography/sources.xml, b:Sources). It carries the list of
// b:Source entries referenced by CITATION fields.
//
// The whole part lives in a single namespace (the shared bibliography
// namespace), so it is modeled as a recursive element tree (BibElement) that
// round-trips any source verbatim while typed accessors read the common
// leaf fields (Tag, SourceType, Title, Year, Author).
type CT_Sources struct {
	// OriginalNSDecls preserves the root namespace declarations of a parsed
	// part so regeneration keeps them.
	OriginalNSDecls []xmlb.NSDecl `xml:"-"`
	// SelectedStyle, StyleName, and URI are the b:Sources root attributes Word
	// writes (the active bibliography style). Preserved verbatim.
	SelectedStyle string `xml:"-"`
	StyleName     string `xml:"-"`
	URI           string `xml:"-"`
	// Source holds the b:Source children in document order.
	Source []*BibElement `xml:"-"`
}

// BibElement is one element of the bibliography tree: either a leaf carrying
// text (e.g. b:Title) or a container carrying child elements (e.g. b:Author).
type BibElement struct {
	Local    string        // local name, e.g. "Source", "Title", "Author"
	Text     string        // character data for a leaf element
	Children []*BibElement // child elements for a container element
}

// UnmarshalXML implements custom unmarshaling for CT_Sources.
func (s *CT_Sources) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for _, attr := range start.Attr {
		switch {
		case attr.Name.Space == "xmlns":
			s.OriginalNSDecls = append(s.OriginalNSDecls, xmlb.NSDecl{Prefix: attr.Name.Local, URI: attr.Value})
		case attr.Name.Space == "" && attr.Name.Local == "xmlns":
			s.OriginalNSDecls = append(s.OriginalNSDecls, xmlb.NSDecl{Prefix: "", URI: attr.Value})
		case attr.Name.Local == "SelectedStyle":
			s.SelectedStyle = attr.Value
		case attr.Name.Local == "StyleName":
			s.StyleName = attr.Value
		case attr.Name.Local == "URI":
			s.URI = attr.Value
		}
	}
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			child, err := decodeBibElement(d, t)
			if err != nil {
				return err
			}
			s.Source = append(s.Source, child)
		case xml.EndElement:
			return nil
		}
	}
}

// decodeBibElement recursively reads a bibliography element and its subtree.
func decodeBibElement(d *xml.Decoder, start xml.StartElement) (*BibElement, error) {
	e := &BibElement{Local: start.Name.Local}
	var text strings.Builder
	for {
		tok, err := d.Token()
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			child, err := decodeBibElement(d, t)
			if err != nil {
				return nil, err
			}
			e.Children = append(e.Children, child)
		case xml.CharData:
			text.Write(t)
		case xml.EndElement:
			// Only a leaf (no child elements) keeps its character data; a
			// container's inter-element whitespace is not content.
			if len(e.Children) == 0 {
				e.Text = text.String()
			}
			return e, nil
		}
	}
}

// MarshalContent writes the b:Source children of the sources part.
func (s *CT_Sources) MarshalContent(b *xmlb.Builder, ns string) {
	for _, src := range s.Source {
		src.marshal(b, ns)
	}
}

// marshal writes a bibliography element and its subtree in the given namespace.
func (e *BibElement) marshal(b *xmlb.Builder, ns string) {
	if len(e.Children) == 0 {
		b.WriteElement(ns, e.Local, e.Text)
		return
	}
	b.StartElement(ns, e.Local)
	for _, c := range e.Children {
		c.marshal(b, ns)
	}
	b.EndElement(ns, e.Local)
}

// Leaf returns the text of the first direct child element with the given local
// name, or "" when absent. Used to read scalar source fields (Title, Year, ...).
func (e *BibElement) Leaf(local string) string {
	for _, c := range e.Children {
		if c.Local == local && len(c.Children) == 0 {
			return c.Text
		}
	}
	return ""
}

// Child returns the first direct child element with the given local name, or
// nil when absent. Used to descend into container fields (b:Author).
func (e *BibElement) Child(local string) *BibElement {
	for _, c := range e.Children {
		if c.Local == local {
			return c
		}
	}
	return nil
}

// Empty reports whether the sources part carries no b:Source entries.
func (s *CT_Sources) Empty() bool { return len(s.Source) == 0 }
