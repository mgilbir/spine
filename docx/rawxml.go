package docx

import (
	"bytes"
	"strconv"
)

// Minimal, allocation-conscious helpers for reading and patching attributes on
// elements that live in the model as raw bytes (w:drawing, w:pict, w:object,
// mc:AlternateContent).
//
// They exist because those fragments must survive edits *without* being
// regenerated: rebuilding a parsed drawing from the narrow InlineImage model
// drops its docPr id, anchor offsets, wrap mode, rotation and every spPr extra
// (C372). Patching the one attribute a setter owns leaves the rest verbatim.
//
// Everything here matches on the element's LOCAL name, so a producer that binds
// the wordprocessingDrawing namespace to a prefix other than wp: is handled
// (C491), and attribute lookup is a real attribute scan rather than a search
// for `<wp:docPr id="`, so attribute order does not matter either.

// isTagNameByteBoundary reports whether b terminates an element or attribute
// name inside a start tag.
func isTagNameByteBoundary(b byte) bool {
	switch b {
	case ' ', '\t', '\r', '\n', '/', '>', '=':
		return true
	}
	return false
}

// isXMLSpaceByte reports whether b is XML whitespace.
func isXMLSpaceByte(b byte) bool {
	return b == ' ' || b == '\t' || b == '\r' || b == '\n'
}

// localName returns the part of an element or attribute qname after the prefix.
func localName(qname []byte) []byte {
	if i := bytes.IndexByte(qname, ':'); i >= 0 {
		return qname[i+1:]
	}
	return qname
}

// rawTagEnd returns the index just past the '>' that closes the start tag
// beginning at s, skipping quoted attribute values so a '>' inside a value does
// not terminate the tag. It returns -1 for an unterminated tag.
func rawTagEnd(raw []byte, s int) int {
	var quote byte
	for i := s; i < len(raw); i++ {
		c := raw[i]
		if quote != 0 {
			if c == quote {
				quote = 0
			}
			continue
		}
		switch c {
		case '"', '\'':
			quote = c
		case '>':
			return i + 1
		}
	}
	return -1
}

// rawTagRange returns the byte range [start,end) of the first start tag whose
// local name is local, searching from index from. end is one past the closing
// '>'. It returns (-1,-1) when no such tag is present. Comments are skipped so
// a commented-out element cannot be matched or patched.
func rawTagRange(raw []byte, local string, from int) (int, int) {
	want := []byte(local)
	for i := from; i < len(raw); {
		j := bytes.IndexByte(raw[i:], '<')
		if j < 0 {
			return -1, -1
		}
		s := i + j
		k := s + 1
		if k >= len(raw) {
			return -1, -1
		}
		switch raw[k] {
		case '/', '?':
			i = k + 1
			continue
		case '!':
			if bytes.HasPrefix(raw[s:], []byte("<!--")) {
				e := bytes.Index(raw[s:], []byte("-->"))
				if e < 0 {
					return -1, -1
				}
				i = s + e + 3
				continue
			}
			i = k + 1
			continue
		}
		n := k
		for n < len(raw) && !isTagNameByteBoundary(raw[n]) {
			n++
		}
		if bytes.Equal(localName(raw[k:n]), want) {
			e := rawTagEnd(raw, s)
			if e < 0 {
				return -1, -1
			}
			return s, e
		}
		i = n
	}
	return -1, -1
}

// rawTagAttrValueRange returns the byte range of the *value* of the attribute
// whose local name is attr inside the start tag raw[s:e], or (-1,-1) when the
// tag does not carry it.
func rawTagAttrValueRange(raw []byte, s, e int, attr string) (int, int) {
	want := []byte(attr)
	i := s + 1
	for i < e && !isTagNameByteBoundary(raw[i]) {
		i++
	}
	for i < e {
		for i < e && isXMLSpaceByte(raw[i]) {
			i++
		}
		if i >= e || raw[i] == '/' || raw[i] == '>' {
			return -1, -1
		}
		ns := i
		for i < e && !isTagNameByteBoundary(raw[i]) {
			i++
		}
		name := raw[ns:i]
		for i < e && isXMLSpaceByte(raw[i]) {
			i++
		}
		if i >= e || raw[i] != '=' {
			// Not a well-formed name="value" pair; give up rather than guess.
			return -1, -1
		}
		i++
		for i < e && isXMLSpaceByte(raw[i]) {
			i++
		}
		if i >= e {
			return -1, -1
		}
		q := raw[i]
		if q != '"' && q != '\'' {
			return -1, -1
		}
		i++
		vs := i
		for i < e && raw[i] != q {
			i++
		}
		if i >= e {
			return -1, -1
		}
		ve := i
		i++
		if bytes.Equal(localName(name), want) {
			return vs, ve
		}
	}
	return -1, -1
}

// rawTagAttr returns the value of attribute attr on the first element with
// local name local at or after from, and whether both were found.
func rawTagAttr(raw []byte, local, attr string, from int) (string, bool) {
	s, e := rawTagRange(raw, local, from)
	if s < 0 {
		return "", false
	}
	vs, ve := rawTagAttrValueRange(raw, s, e, attr)
	if vs < 0 {
		return "", false
	}
	return string(raw[vs:ve]), true
}

// setRawTagAttr sets attribute attr to value (attribute-escaped) on the first
// element with local name local at or after from, returning the new bytes. When
// the element carries the attribute its value is replaced in place, preserving
// the rest of the tag verbatim; otherwise the attribute is inserted before the
// tag's closing '/>' or '>'. raw is returned unchanged when the element is
// absent.
func setRawTagAttr(raw []byte, local, attr, value string, from int) []byte {
	s, e := rawTagRange(raw, local, from)
	if s < 0 {
		return raw
	}
	esc := xmlEscapeAttr(value)
	if vs, ve := rawTagAttrValueRange(raw, s, e, attr); vs >= 0 {
		out := make([]byte, 0, len(raw)-(ve-vs)+len(esc))
		out = append(out, raw[:vs]...)
		out = append(out, esc...)
		out = append(out, raw[ve:]...)
		return out
	}
	ins := e - 1 // the '>'
	if ins > s && raw[ins-1] == '/' {
		ins--
	}
	add := make([]byte, 0, len(attr)+len(esc)+4)
	add = append(add, ' ')
	add = append(add, attr...)
	add = append(add, '=', '"')
	add = append(add, esc...)
	add = append(add, '"')
	out := make([]byte, 0, len(raw)+len(add))
	out = append(out, raw[:ins]...)
	out = append(out, add...)
	out = append(out, raw[ins:]...)
	return out
}

// rawTagIntAttrs returns the value of the named integer attribute of every
// element with the given local name in raw, in document order. Unparseable
// values are skipped.
func rawTagIntAttrs(raw []byte, local, attr string) []int {
	var out []int
	for from := 0; ; {
		s, e := rawTagRange(raw, local, from)
		if s < 0 {
			return out
		}
		if vs, ve := rawTagAttrValueRange(raw, s, e, attr); vs >= 0 {
			if n, err := strconv.Atoi(string(bytes.TrimSpace(raw[vs:ve]))); err == nil {
				out = append(out, n)
			}
		}
		from = e
	}
}
