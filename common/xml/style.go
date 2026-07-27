package xml

import "bytes"

// DetectSelfClosingSpace reports whether a source part writes self-closing
// elements as " />" (space before the close) rather than "/>". It scans for
// the first self-closing tag after the XML declaration (producers are
// consistent within a part). That first "/>" may be the root element's own
// tag or a self-closing child; either reflects the part's style.
func DetectSelfClosingSpace(data []byte) bool {
	// Search from just after the XML declaration (or the start if none).
	start := bytes.Index(data, []byte("?>"))
	if start < 0 {
		start = 0
	}
	rootOpen := bytes.IndexByte(data[start:], '>')
	if rootOpen < 0 {
		return false
	}
	searchFrom := start + rootOpen + 1
	idx := bytes.Index(data[searchFrom:], []byte("/>"))
	if idx < 0 {
		return false
	}
	absIdx := searchFrom + idx
	return absIdx > 0 && data[absIdx-1] == ' '
}

// DetectCollapsedEmptyElements reports whether a source part writes its empty
// elements self-closing (<name/>), so a regenerated part should collapse
// empty open/close pairs via Builder.SetCollapseEmptyElements. True when the
// part contains at least one self-closing tag and no expanded empty element
// (<name attrs></name>); producers are consistent within a part, so a single
// expanded empty disables collapsing to preserve the source style.
func DetectCollapsedEmptyElements(data []byte) bool {
	if !bytes.Contains(data, []byte("/>")) {
		return false
	}
	return !hasExpandedEmptyElement(data)
}

// hasExpandedEmptyElement reports whether data contains an element written as
// an adjacent open/close pair with no content, e.g. <w:p w:rsidR="X"></w:p>.
func hasExpandedEmptyElement(data []byte) bool {
	i := 0
	for {
		j := bytes.Index(data[i:], []byte("></"))
		if j < 0 {
			return false
		}
		j += i
		i = j + 1

		// The '>' must terminate an open tag, not a close tag ("</a></b>"),
		// a self-closing tag ("/></a>"), or the XML declaration ("?><...").
		if j > 0 && (data[j-1] == '/' || data[j-1] == '?') {
			continue
		}
		k := bytes.LastIndexByte(data[:j], '<')
		if k < 0 {
			continue
		}
		seg := data[k+1 : j]
		if len(seg) == 0 || seg[0] == '/' || seg[0] == '?' || seg[0] == '!' {
			continue
		}

		// Element name runs to the first whitespace (or the whole segment).
		name := seg
		for n := 0; n < len(seg); n++ {
			if c := seg[n]; c == ' ' || c == '\t' || c == '\r' || c == '\n' {
				name = seg[:n]
				break
			}
		}

		// Expanded empty iff the close tag names the same element.
		close := data[j+2:]
		if len(close) >= len(name)+2 && close[0] == '/' &&
			bytes.Equal(close[1:1+len(name)], name) && close[1+len(name)] == '>' {
			return true
		}
	}
}
