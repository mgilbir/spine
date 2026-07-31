package xml

import "fmt"

// Nesting depth is a resource dimension of its own.
//
// Nothing else in the parse path bounds recursion. Both the decoder and the
// domain converters above it descend one frame per level, so nesting depth in
// the file translates directly into memory: a 244 KB slide holding 80,000
// nested p:grpSp elements cost 627 MB resident, split about evenly between the
// decode and the materialisation, and the per-level cost *grows* with depth
// (5.5 KB/level at 20,000, 7.8 KB/level at 80,000). At 200,000 levels — a file
// well under a megabyte — the process is killed. That is the same class as the
// unvalidated CFB sector count (C360) and the chart ptCount (879,239x
// amplification): a small input driving unbounded resource use. It differs only
// in that no field declares the size, so no clamp on a number could catch it.
//
// The limit itself is not decided here: this package is the XML substrate and
// has no opinion about packages. opc owns the knob (opc.MaxNestingDepth and
// ReaderOptions.MaxNestingDepth) and passes it in, which keeps it beside the
// decompression and entry-count limits a caller already tunes, and lets it be
// captured per Reader rather than read from a global mid-parse.

// ErrNestingTooDeep reports a part whose element nesting exceeds the limit it
// was checked against.
var ErrNestingTooDeep = fmt.Errorf("xml: element nesting exceeds the maximum depth")

// CheckNestingDepth reports an error when data nests elements deeper than max.
// A max of zero or less disables the check.
//
// It scans bytes rather than tokens on purpose. Wrapping the decoder was the
// obvious alternative and is not available: UnmarshalWithSource registers the
// *xml.Decoder pointer in decoderSources so the capture helpers can call
// InputOffset on it, and an xml.NewTokenDecoder wrapper is a different decoder
// whose offsets do not track the source — every offset-based capture would
// slice the wrong bytes and byte-identity round-trip would break. A separate
// pass costs one cheap traversal and leaves the decode untouched.
//
// The scan only has to be correct about where tags start and end. '<' cannot
// appear in an attribute value or in text (XML 1.0 requires &lt;), so the only
// constructs that can hide one are comments, CDATA sections and processing
// instructions, each of which is skipped whole. '>' *can* appear unescaped
// inside an attribute value, so quoted runs are tracked while looking for a
// tag's end — otherwise <a b=">"/> would be read as an unclosed start tag.
func CheckNestingDepth(data []byte, limit int) error {
	if limit <= 0 {
		return nil
	}
	depth, deepest := 0, 0
	for i := 0; i < len(data); {
		if data[i] != '<' {
			i++
			continue
		}
		switch {
		case hasPrefixAt(data, i, "<!--"):
			i = skipPast(data, i+4, "-->")
			continue
		case hasPrefixAt(data, i, "<![CDATA["):
			i = skipPast(data, i+9, "]]>")
			continue
		case hasPrefixAt(data, i, "<?"):
			i = skipPast(data, i+2, "?>")
			continue
		case hasPrefixAt(data, i, "<!"):
			// DOCTYPE and friends: no element nesting to count, and an internal
			// subset can contain '>' inside quotes, so reuse the tag scanner.
			i = endOfTag(data, i+2)
			continue
		}

		closing := i+1 < len(data) && data[i+1] == '/'
		end := endOfTag(data, i+1)
		selfClosing := end >= 2 && data[end-2] == '/'

		switch {
		case closing:
			if depth > 0 {
				depth--
			}
		case selfClosing:
			// Opens and closes in one tag; contributes a level but no descent.
			if depth+1 > deepest {
				deepest = depth + 1
			}
		default:
			depth++
			if depth > deepest {
				deepest = depth
			}
			if depth > limit {
				return fmt.Errorf("%w (%d)", ErrNestingTooDeep, limit)
			}
		}
		i = end
	}
	return nil
}

// MaxObservedNestingDepth reports the deepest element nesting in data. It exists
// so the limit can be calibrated against real documents rather than guessed.
func MaxObservedNestingDepth(data []byte) int {
	depth, max := 0, 0
	for i := 0; i < len(data); {
		if data[i] != '<' {
			i++
			continue
		}
		switch {
		case hasPrefixAt(data, i, "<!--"):
			i = skipPast(data, i+4, "-->")
			continue
		case hasPrefixAt(data, i, "<![CDATA["):
			i = skipPast(data, i+9, "]]>")
			continue
		case hasPrefixAt(data, i, "<?"):
			i = skipPast(data, i+2, "?>")
			continue
		case hasPrefixAt(data, i, "<!"):
			i = endOfTag(data, i+2)
			continue
		}
		closing := i+1 < len(data) && data[i+1] == '/'
		end := endOfTag(data, i+1)
		selfClosing := end >= 2 && data[end-2] == '/'
		switch {
		case closing:
			if depth > 0 {
				depth--
			}
		case selfClosing:
			if depth+1 > max {
				max = depth + 1
			}
		default:
			depth++
			if depth > max {
				max = depth
			}
		}
		i = end
	}
	return max
}

// endOfTag returns the index just past the '>' that ends the tag whose body
// starts at i, tracking quoted attribute values so an unescaped '>' inside one
// does not end the tag early. It returns len(data) for an unterminated tag,
// which leaves the caller's loop finished rather than spinning.
func endOfTag(data []byte, i int) int {
	var quote byte
	for ; i < len(data); i++ {
		c := data[i]
		switch {
		case quote != 0:
			if c == quote {
				quote = 0
			}
		case c == '"' || c == '\'':
			quote = c
		case c == '>':
			return i + 1
		}
	}
	return len(data)
}

// skipPast returns the index just past the first occurrence of sep at or after
// i, or len(data) when sep never appears.
func skipPast(data []byte, i int, sep string) int {
	for ; i+len(sep) <= len(data); i++ {
		if hasPrefixAt(data, i, sep) {
			return i + len(sep)
		}
	}
	return len(data)
}

// hasPrefixAt reports whether data has s at index i.
func hasPrefixAt(data []byte, i int, s string) bool {
	if i+len(s) > len(data) {
		return false
	}
	return string(data[i:i+len(s)]) == s
}
