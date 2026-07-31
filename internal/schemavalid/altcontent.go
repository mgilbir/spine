package schemavalid

import (
	"bytes"
	"encoding/xml"
)

// nsMarkupCompatibility is the namespace of the mc: elements and attributes.
const nsMarkupCompatibility = "http://schemas.openxmlformats.org/markup-compatibility/2006"

// ResolveAlternateContent rewrites every mc:AlternateContent element as the
// content of its mc:Fallback branch, and returns the part unchanged when there
// is none to rewrite.
//
// This is what makes schema validation usable on real Office markup rather than
// only on the subset that avoids extensions. mc:AlternateContent exists so a
// producer can offer newer markup with older markup beside it — a docx text box
// is written as a wps: shape with a VML w:pict fallback — and a consumer that
// does not understand the Choice is *required* to take the Fallback. The ISO
// schemas model neither: the Choice content sits in vendor namespaces that are
// not in the schema set, and the validator's wildcard is strict, so validating
// the raw part reports "no matching global element declaration" for markup Word
// itself writes. Resolving the way a conforming consumer must resolve leaves
// exactly the markup the schema does describe.
//
// The rewrite is a byte splice rather than a re-encode: everything outside the
// replaced span is passed through untouched, so a validator complaint always
// refers to bytes this library actually wrote. Nesting is handled by repeating
// the pass, since a Fallback may itself carry an AlternateContent.
func ResolveAlternateContent(part []byte) []byte {
	// The bound is a backstop against a pathological document, not a real
	// limit: Office nests these at most a couple of levels deep.
	for i := 0; i < 16; i++ {
		out, changed := resolveFirstAlternateContent(part)
		if !changed {
			return out
		}
		part = out
	}
	return part
}

// resolveFirstAlternateContent replaces the first mc:AlternateContent it finds,
// reporting whether it replaced anything.
func resolveFirstAlternateContent(part []byte) ([]byte, bool) {
	dec := xml.NewDecoder(bytes.NewReader(part))

	var (
		acStart         int64 = -1
		fbStart, fbEnd  int64 = -1, -1
		depth, acDepth  int
		inFallback      bool
		prevTokenEnd    int64
		haveFallbackEnd bool
	)

	for {
		tok, err := dec.Token()
		if err != nil {
			return part, false
		}

		switch t := tok.(type) {
		case xml.StartElement:
			switch {
			case acStart < 0 && isMC(t.Name, "AlternateContent"):
				acStart, acDepth = prevTokenEnd, depth
			case acStart >= 0 && !inFallback && !haveFallbackEnd &&
				depth == acDepth+1 && isMC(t.Name, "Fallback"):
				// Content starts after this start tag.
				inFallback, fbStart = true, dec.InputOffset()
			}
			depth++

		case xml.EndElement:
			depth--
			if inFallback && depth == acDepth+1 && isMC(t.Name, "Fallback") {
				// Content ends where this end tag begins.
				fbEnd, inFallback, haveFallbackEnd = prevTokenEnd, false, true
			}
			if acStart >= 0 && depth == acDepth && isMC(t.Name, "AlternateContent") {
				acEnd := dec.InputOffset()
				var fallback []byte
				if haveFallbackEnd && fbEnd >= fbStart {
					fallback = part[fbStart:fbEnd]
				}
				// No Fallback means the producer offered nothing a consumer
				// without the extension can render, so the element drops out
				// entirely — which is also what such a consumer displays.
				out := make([]byte, 0, len(part))
				out = append(out, part[:acStart]...)
				out = append(out, fallback...)
				out = append(out, part[acEnd:]...)
				return out, true
			}
		}
		prevTokenEnd = dec.InputOffset()
	}
}

func isMC(name xml.Name, local string) bool {
	return name.Space == nsMarkupCompatibility && name.Local == local
}
