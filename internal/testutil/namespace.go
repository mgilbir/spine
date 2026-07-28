package testutil

import (
	"encoding/xml"
	"io"
	"strings"
)

// NamespaceWellFormed reports the first element or attribute in s written with
// a prefix that no in-scope declaration binds.
//
// Go's encoding/xml does not reject such input: when nothing binds a prefix it
// simply reports the bare prefix as the name's Space instead of a URI, so both
// plain well-formedness and Builder.Finish accept output that no conforming
// consumer will. That is exactly how C375 shipped — an mc:AlternateContent
// re-emitted under a prefix the document never declared — so tests covering the
// verbatim-replay paths assert through this rather than through a round trip.
//
// Checking Space against the set of declared URIs is sufficient: the decoder
// resolves a prefix to its URI only while a declaration for it is in scope, so
// anything still carrying a bare prefix here is unbound at that point.
func NamespaceWellFormed(s string) error {
	d := xml.NewDecoder(strings.NewReader(s))
	declared := map[string]bool{"http://www.w3.org/XML/1998/namespace": true}
	for {
		tok, err := d.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		for _, a := range start.Attr {
			if a.Name.Space == "xmlns" || (a.Name.Space == "" && a.Name.Local == "xmlns") {
				declared[a.Value] = true
			}
		}
		if start.Name.Space != "" && !declared[start.Name.Space] {
			return &UnboundPrefixError{Prefix: start.Name.Space, Name: start.Name.Local}
		}
		for _, a := range start.Attr {
			if a.Name.Space == "" || a.Name.Space == "xmlns" {
				continue
			}
			if !declared[a.Name.Space] {
				return &UnboundPrefixError{Prefix: a.Name.Space, Name: a.Name.Local, Attr: true}
			}
		}
	}
}

// UnboundPrefixError names the element or attribute NamespaceWellFormed
// rejected.
type UnboundPrefixError struct {
	Prefix string
	Name   string
	Attr   bool
}

func (e *UnboundPrefixError) Error() string {
	kind := "element"
	if e.Attr {
		kind = "attribute"
	}
	return "unbound namespace prefix " + e.Prefix + " on " + kind + " " + e.Prefix + ":" + e.Name
}
