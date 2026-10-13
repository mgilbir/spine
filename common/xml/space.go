package xml

import "bytes"

// IsSpace reports whether text is XML white space: only #x20, #x9, #xD and
// #xA (XML 1.0 §2.3, production S). That is narrower than Unicode's notion,
// which strings.TrimSpace and unicode.IsSpace follow: U+00A0, U+0085, U+2028,
// U+3000 and the rest are content to XML, and classing them as white space
// dropped text on the floor or let it stand outside the root element.
func IsSpace(text []byte) bool {
	for _, c := range text {
		if !isXMLSpace(c) {
			return false
		}
	}
	return true
}

// IsBlank reports whether a part holds no XML at all: nothing but white space
// after an optional byte order mark, in any encoding the library reads. A
// producer that writes such a part means "absent", and a reader tolerating an
// empty part must tolerate it whatever the encoding: "\xEF\xBB\xBF" is as
// empty as "".
func IsBlank(part []byte) bool {
	text, err := UTF8(part)
	if err != nil {
		return false
	}
	return IsSpace(bytes.TrimPrefix(text, utf8BOM))
}
