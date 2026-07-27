package dml

import (
	"encoding/xml"
	"strings"
	"testing"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// Shared fragment helpers for the dml round-trip tests: parse a single element
// with the DrawingML prefixes bound on an enclosing wrapper, and re-marshal it
// through the production Builder path.

// pctWrapperNS declares the prefixes the test fragments use, on a wrapper root
// rather than on the element under test: a real part declares them once at the
// part root, and putting them on the element itself would both feed a spurious
// xmlns into CapturedAttrs and (for a:alphaRepl) collide with its "a" attribute
// under encoding/xml's namespace-insensitive attribute matching.
var pctWrapperNS = `<a:wrap xmlns:a="` + NsDrawingML + `" xmlns:r="` + xmlb.NSOfficeDocumentRels + `">`

// parseElement decodes a single element fragment into v, with the DrawingML
// and relationship prefixes bound on an enclosing wrapper.
func parseElement(t *testing.T, elem string, v interface{}) error {
	t.Helper()
	d := xml.NewDecoder(strings.NewReader(pctWrapperNS + elem + `</a:wrap>`))
	depth := 0
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		if se, ok := tok.(xml.StartElement); ok {
			depth++
			if depth == 2 {
				return d.DecodeElement(v, &se)
			}
		}
	}
}

// marshalElement marshals v through the production Builder path and returns
// the element source, with the wrapper's declarations stripped back off.
func marshalElement(t *testing.T, localName string, v interface{}) string {
	t.Helper()
	b := xmlb.NewBuilder()
	b.RegisterNamespace(NsDrawingML, xmlb.PrefixDrawingML)
	b.RegisterNamespace(xmlb.NSOfficeDocumentRels, xmlb.PrefixRelationships)
	b.SetCollapseEmptyElements(true)
	b.StartElementWithNS(NsDrawingML, "wrap", []xmlb.NSDecl{
		{Prefix: xmlb.PrefixDrawingML, URI: NsDrawingML},
		{Prefix: xmlb.PrefixRelationships, URI: xmlb.NSOfficeDocumentRels},
	})
	b.MarshalElement(NsDrawingML, localName, v)
	b.EndElement(NsDrawingML, "wrap")
	if err := b.Finish(); err != nil {
		t.Fatalf("builder: %v", err)
	}
	out := string(b.Bytes())
	out = strings.TrimPrefix(out, pctWrapperNS)
	return strings.TrimSuffix(out, `</a:wrap>`)
}
