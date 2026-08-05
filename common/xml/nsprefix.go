package xml

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
)

// CheckNamespacePrefixes reports whether every prefix the document uses is
// bound by a declaration in scope.
//
// Namespaces in XML 1.0 §3 makes using an undeclared prefix a fatal error, and
// Word will not open a part that does one. Go's decoder is deliberately
// lenient: rather than failing, it resolves an unbound prefix to the prefix
// *itself*, so w:styles in a document that never declares w parses as the name
// "styles" in namespace "w". Nothing distinguishes it from a real namespace
// downstream.
//
// The consequence is not that the content is dropped, which would be harmless.
// It is that the content is silently re-homed on the way out. Nothing matches
// the model (the model's namespace is the real URI, not the string "w"), so the
// part is preserved as captured children; the writer then emits a root carrying
// the standard declarations, including xmlns:w. The replayed prefixes now bind
// to the real namespace, and elements that meant nothing on the way in mean
// WordprocessingML on the way out.
//
// FuzzDocxStylesXML found it through the one symptom that surfaces: a document
// whose xmlns:w was corrupted to jxmlns:w carried
// <w:uiPriority w:val="99999999999999999999"/>, which parsed as nothing on the
// way in, was written into the real namespace, and then would not reopen —
// CT_DecimalNumber.Val is an int. The reopen failure is the visible half. The
// invisible half is every other element in that part quietly changing meaning,
// which no oracle would have caught.
//
// The check costs one token pass. It tracks declared URIs rather than prefixes
// because that is what the decoder leaves visible: a bound prefix arrives as
// its URI, which some declaration in scope must therefore have introduced,
// while an unbound one arrives as itself and matches nothing.
func CheckNamespacePrefixes(data []byte) error {
	d := NewDecoder(bytes.NewReader(data))
	// scopes[i] holds the URIs declared by the i-th open element. A URI is in
	// scope while any frame holding it is open.
	declared := map[string]int{NSXML: 1}
	var scopes [][]string

	for {
		tok, err := d.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			// Malformed input is the caller's own decode to report; this pass
			// only answers the namespace question.
			return nil
		}
		switch t := tok.(type) {
		case xml.StartElement:
			var frame []string
			for _, a := range t.Attr {
				if uri, ok := nsDeclURI(a); ok && uri != "" {
					declared[uri]++
					frame = append(frame, uri)
				}
			}
			scopes = append(scopes, frame)
			if err := checkName(t.Name, declared, "element"); err != nil {
				return err
			}
			for _, a := range t.Attr {
				if _, ok := nsDeclURI(a); ok {
					continue // a declaration, not a use
				}
				if err := checkName(a.Name, declared, "attribute"); err != nil {
					return err
				}
			}
		case xml.EndElement:
			if n := len(scopes); n > 0 {
				for _, uri := range scopes[n-1] {
					if declared[uri]--; declared[uri] <= 0 {
						delete(declared, uri)
					}
				}
				scopes = scopes[:n-1]
			}
		}
	}
}

// nsDeclURI reports whether attr is an xmlns declaration and, if so, the URI it
// binds. Go spells the two forms differently: xmlns:p="U" arrives with Space
// "xmlns", and a default xmlns="U" as the bare local name "xmlns".
func nsDeclURI(attr xml.Attr) (string, bool) {
	switch {
	case attr.Name.Space == "xmlns":
		return attr.Value, true
	case attr.Name.Space == "" && attr.Name.Local == "xmlns":
		return attr.Value, true
	}
	return "", false
}

// checkName rejects a name whose namespace no declaration in scope introduced,
// which is the decoder's signature for an unbound prefix.
func checkName(name xml.Name, declared map[string]int, what string) error {
	if name.Space == "" || declared[name.Space] > 0 {
		return nil
	}
	return fmt.Errorf("%s %s:%s uses a namespace prefix that nothing declares",
		what, name.Space, name.Local)
}
