package xml

import (
	"errors"
	"strings"
	"unicode/utf8"
)

// IsName reports whether s is an XML 1.0 Name (§2.3).
//
// It exists for the capture paths that keep a name taken from input and write
// it back out. Go's decoder is more permissive than the grammar — it accepted
// <A:0/> and reported the local name "0" — and its encoder writes whatever name
// it is handed, so a name that is not a Name goes in, comes out, and the result
// does not parse. FuzzGroupRoundTrip found it through a v:textbox child.
//
// A caller that models a name it cannot reproduce has nothing useful to do with
// it, so this is a parse-time gate rather than an emit-time one: the failure is
// reported against the document that carries the bad name, not against the
// library that faithfully echoed it.
func IsName(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		if r == utf8.RuneError {
			// A decoded string holding U+FFFD came from bytes that are not
			// valid UTF-8, which cannot be written back out either.
			if _, size := utf8.DecodeRuneInString(s[i:]); size == 1 {
				return false
			}
		}
		if i == 0 {
			if !isNameStartChar(r) {
				return false
			}
			continue
		}
		if !isNameChar(r) {
			return false
		}
	}
	return true
}

// isNameStartChar implements the NameStartChar production. The colon is in it:
// XML itself has no notion of prefixes, and a QName's single colon is the
// Namespaces spec's restriction, not this one's.
func isNameStartChar(r rune) bool {
	switch {
	case r == ':' || r == '_':
		return true
	case r >= 'A' && r <= 'Z', r >= 'a' && r <= 'z':
		return true
	case r >= 0xC0 && r <= 0xD6, r >= 0xD8 && r <= 0xF6, r >= 0xF8 && r <= 0x2FF:
		return true
	case r >= 0x370 && r <= 0x37D, r >= 0x37F && r <= 0x1FFF:
		return true
	case r >= 0x200C && r <= 0x200D, r >= 0x2070 && r <= 0x218F:
		return true
	case r >= 0x2C00 && r <= 0x2FEF, r >= 0x3001 && r <= 0xD7FF:
		return true
	case r >= 0xF900 && r <= 0xFDCF, r >= 0xFDF0 && r <= 0xFFFD:
		return true
	case r >= 0x10000 && r <= 0xEFFFF:
		return true
	}
	return false
}

// isNameChar implements the NameChar production: NameStartChar plus the
// characters a name may continue with but not begin with.
func isNameChar(r rune) bool {
	switch {
	case isNameStartChar(r):
		return true
	case r == '-' || r == '.' || r == 0xB7:
		return true
	case r >= '0' && r <= '9':
		return true
	case r >= 0x300 && r <= 0x36F, r >= 0x203F && r <= 0x2040:
		return true
	}
	return false
}

// IsNCName reports whether s is an XML Namespaces NCName (§3): a Name with no
// colon in it. Prefixes and the local parts of a QName are NCNames.
func IsNCName(s string) bool {
	return IsName(s) && strings.IndexByte(s, ':') < 0
}

// IsQName reports whether s is an XML Namespaces QName: an NCName, optionally
// preceded by a prefix and a colon.
//
// The distinction from IsName is the whole point. XML's own Name production
// allows a colon anywhere and any number of times — <:/> and <a:b:c/> are
// well-formed XML — so a name that passes IsName can still be one no
// namespace-aware consumer accepts, and Word is namespace-aware.
func IsQName(s string) bool {
	if i := strings.IndexByte(s, ':'); i >= 0 {
		return IsNCName(s[:i]) && IsNCName(s[i+1:])
	}
	return IsNCName(s)
}

// ErrUnwritableName is what every refusal to write a name wraps.
//
// The Builder declines to emit an element or attribute name that is not a
// QName, because the part it would produce is one no namespace-aware parser
// reads back — and a part that fails to parse is absent to the consumer rather
// than an error, so writing it loses content silently.
//
// It is a distinct sentinel because "this document contains a name that cannot
// be written" is a different answer from "the save failed". A caller handed a
// document from somewhere else can tell the two apart with errors.Is and decide
// whether to repair the document or report it, instead of matching on message
// text.
var ErrUnwritableName = errors.New("xml: unwritable name")
