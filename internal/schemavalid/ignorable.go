package schemavalid

import (
	"bytes"
	"encoding/xml"
	"strings"
)

// StripIgnorable removes the elements and attributes that a consumer is
// licensed to ignore, which is what makes the remaining validation failures
// mean something.
//
// The license is the markup-compatibility mc:Ignorable attribute: a producer
// lists there the namespace prefixes a consumer may drop without losing
// meaning, and Word writes w14:paraId, w15: and wp14: exactly so older readers
// can. The ISO schemas describe none of them — they are Microsoft extensions —
// so a validator that has not implemented MCE reports every one of them as a
// violation. Doing what the producer said a consumer may do leaves the markup
// ISO actually defines.
//
// Only namespaces the producer declared ignorable are removed. Anything else in
// an unknown namespace stays and fails, deliberately: an extension that was
// *not* offered as ignorable, and not wrapped in an mc:AlternateContent with a
// fallback, is markup a down-level consumer cannot render and cannot skip.
func StripIgnorable(part []byte) []byte {
	ignorable := ignorableNamespaces(part)
	if len(ignorable) == 0 {
		return part
	}
	for i := 0; i < 64; i++ {
		out, changed := stripFirstIgnorableElement(part, ignorable)
		if !changed {
			part = out
			break
		}
		part = out
	}
	// The compatibility attributes are themselves markup a consumer processes
	// and drops: mc:Ignorable names what may be skipped, and no schema declares
	// it on the elements that carry it, so leaving it in trades one spurious
	// failure for another.
	return stripIgnorableAttrs(part, ignorable, true)
}

// ignorableNamespaces resolves the prefixes listed in the root's mc:Ignorable
// to the URIs they are bound to.
func ignorableNamespaces(part []byte) map[string]bool {
	dec := xml.NewDecoder(bytes.NewReader(part))
	for {
		tok, err := dec.Token()
		if err != nil {
			return nil
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		// The root element carries the declaration; anything deeper is a
		// scoped override this does not need to model.
		prefixes := map[string]bool{}
		for _, a := range start.Attr {
			if a.Name.Space == nsMarkupCompatibility && a.Name.Local == "Ignorable" {
				for _, p := range strings.Fields(a.Value) {
					prefixes[p] = true
				}
			}
		}
		if len(prefixes) == 0 {
			return nil
		}
		out := map[string]bool{}
		for _, a := range start.Attr {
			if a.Name.Space == "xmlns" && prefixes[a.Name.Local] {
				out[a.Value] = true
			}
		}
		return out
	}
}

// stripFirstIgnorableElement removes the first element in an ignorable
// namespace, subtree and all.
func stripFirstIgnorableElement(part []byte, ignorable map[string]bool) ([]byte, bool) {
	dec := xml.NewDecoder(bytes.NewReader(part))
	var (
		start        int64 = -1
		startDepth   int
		depth        int
		prevTokenEnd int64
	)
	for {
		tok, err := dec.Token()
		if err != nil {
			return part, false
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if start < 0 && ignorable[t.Name.Space] {
				start, startDepth = prevTokenEnd, depth
			}
			depth++
		case xml.EndElement:
			depth--
			if start >= 0 && depth == startDepth {
				end := dec.InputOffset()
				out := make([]byte, 0, len(part))
				out = append(out, part[:start]...)
				out = append(out, part[end:]...)
				return out, true
			}
		}
		prevTokenEnd = dec.InputOffset()
	}
}

// stripIgnorableAttrs removes attributes in ignorable namespaces from every
// start tag, leaving the tags otherwise byte-identical.
func stripIgnorableAttrs(part []byte, ignorable map[string]bool, alsoMC bool) []byte {
	// Resolve which prefixes are bound to ignorable namespaces, since attribute
	// removal works on the raw tag text.
	dec := xml.NewDecoder(bytes.NewReader(part))
	prefixes := map[string]bool{}
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		for _, a := range start.Attr {
			if a.Name.Space != "xmlns" {
				continue
			}
			if ignorable[a.Value] || (alsoMC && a.Value == nsMarkupCompatibility) {
				prefixes[a.Name.Local] = true
			}
		}
	}
	if len(prefixes) == 0 {
		return part
	}

	out := part
	for prefix := range prefixes {
		for {
			i := bytes.Index(out, []byte(" "+prefix+":"))
			if i < 0 {
				break
			}
			// Only an attribute assignment is removable; a prefixed element
			// name never follows a space inside a tag this way.
			eq := bytes.IndexByte(out[i:], '=')
			if eq < 0 {
				break
			}
			rest := out[i+eq+1:]
			if len(rest) == 0 {
				break
			}
			quote := rest[0]
			if quote != '"' && quote != '\'' {
				break
			}
			closing := bytes.IndexByte(rest[1:], quote)
			if closing < 0 {
				break
			}
			end := i + eq + 1 + 1 + closing + 1
			trimmed := make([]byte, 0, len(out))
			trimmed = append(trimmed, out[:i]...)
			trimmed = append(trimmed, out[end:]...)
			out = trimmed
		}
	}
	return out
}
