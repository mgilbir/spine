package xml

import (
	"bytes"
	"encoding/xml"
	"sync"
)

// This file provides per-instance empty-element style capture. Go's decoder
// reports <t/> and <t></t> as identical token streams, so distinguishing them
// requires the raw source bytes: parse entry points register the source with
// their decoder (UnmarshalWithSource), and unmarshal hooks call
// CaptureEmptyTagStyle right after their start tag was consumed to record
// which form the producer wrote. Producers mix both forms within one part
// (WPS writes <a:t></a:t> beside self-closed siblings), so the existing
// part-level collapse flag cannot express them.

// EmptyTagStyle records how a source wrote a childless element.
type EmptyTagStyle uint8

const (
	// EmptyTagUnknown means no capture: the part-level style applies.
	EmptyTagUnknown EmptyTagStyle = iota
	// EmptyTagSelfClose is a self-closing tag with the builder's default
	// spacing (values built programmatically).
	EmptyTagSelfClose
	// EmptyTagExpanded is <name></name>.
	EmptyTagExpanded
	// EmptyTagSelfCloseTight is <name/> — no space before the slash — even
	// when the part-level style writes " />".
	EmptyTagSelfCloseTight
	// EmptyTagSelfCloseSpaced is <name /> — exactly one space before the
	// slash — even when the part-level style writes "/>".
	EmptyTagSelfCloseSpaced
	// emptyTagWSBase is the first value of the encoded whitespace-run block.
	// A style at or above it is a self-closing tag whose whitespace run
	// before the slash is something other than a single space (a tab, a line
	// break, or a short mixed run); the run itself is packed into the value,
	// see selfCloseStyleFor / selfCloseWS. Keeping the type a uint8 matters:
	// it is embedded per instance in ~60 model structs (one per run, cell,
	// property bag), so a string-bearing struct would add ~24 bytes to every
	// captured element of a document.
	emptyTagWSBase
)

// selfCloseWSChars are the whitespace bytes a self-closing run is built from,
// indexed by their 2-bit code. XML whitespace is exactly these four.
var selfCloseWSChars = [4]byte{' ', '\t', '\n', '\r'}

// selfCloseWSCode returns c's 2-bit code, or ok=false when c is not XML
// whitespace.
func selfCloseWSCode(c byte) (uint8, bool) {
	switch c {
	case ' ':
		return 0, true
	case '\t':
		return 1, true
	case '\n':
		return 2, true
	case '\r':
		return 3, true
	}
	return 0, false
}

// Block sizes of the packed whitespace-run encoding: runs of one, two and
// three whitespace bytes occupy 4, 16 and 64 consecutive style values, so the
// largest style is emptyTagWSBase+83 — still a uint8.
const (
	selfCloseWS1 = 4
	selfCloseWS2 = selfCloseWS1 * 4
	selfCloseWS3 = selfCloseWS2 * 4
)

// selfCloseStyleFor classifies the whitespace run a source wrote between a
// self-closing tag's content and its slash. An empty run is tight, a single
// space is the canonical spaced form, and any other run of up to three
// whitespace bytes is packed into the style so replay reproduces it verbatim.
// Runs longer than three bytes are not representable and degrade to the spaced
// form (a documented, byte-level-only drift; no OOXML producer writes them).
func selfCloseStyleFor(run []byte) EmptyTagStyle {
	switch len(run) {
	case 0:
		return EmptyTagSelfCloseTight
	case 1:
		if run[0] == ' ' {
			return EmptyTagSelfCloseSpaced
		}
		c, ok := selfCloseWSCode(run[0])
		if !ok {
			return EmptyTagSelfCloseSpaced
		}
		return emptyTagWSBase + EmptyTagStyle(c)
	case 2, 3:
		var n uint8
		for _, c := range run {
			code, ok := selfCloseWSCode(c)
			if !ok {
				return EmptyTagSelfCloseSpaced
			}
			n = n*4 + code
		}
		if len(run) == 2 {
			return emptyTagWSBase + EmptyTagStyle(selfCloseWS1+n)
		}
		return emptyTagWSBase + EmptyTagStyle(selfCloseWS1+selfCloseWS2+n)
	}
	return EmptyTagSelfCloseSpaced
}

// selfCloseWS unpacks the whitespace run encoded in a style at or above
// emptyTagWSBase. It is only reached for the exotic runs selfCloseStyleFor
// packs (never for the tight or single-space forms), so the string allocation
// is off every normal path.
func (s EmptyTagStyle) selfCloseWS() string {
	n := int(s - emptyTagWSBase)
	switch {
	case n < selfCloseWS1:
		return string(selfCloseWSChars[n : n+1])
	case n < selfCloseWS1+selfCloseWS2:
		n -= selfCloseWS1
		return string([]byte{selfCloseWSChars[n/4], selfCloseWSChars[n%4]})
	case n < selfCloseWS1+selfCloseWS2+selfCloseWS3:
		n -= selfCloseWS1 + selfCloseWS2
		return string([]byte{selfCloseWSChars[n/16], selfCloseWSChars[(n/4)%4], selfCloseWSChars[n%4]})
	}
	return " "
}

// IsSelfClose reports whether the style is any self-closing form.
func (s EmptyTagStyle) IsSelfClose() bool {
	return s == EmptyTagSelfClose || s >= EmptyTagSelfCloseTight
}

// decoderSources maps a live *xml.Decoder to the raw bytes it reads, letting
// unmarshal hooks inspect the exact source form of the token just consumed.
var decoderSources sync.Map

// UnmarshalWithSource decodes data into v like xml.Unmarshal, with the source
// bytes registered for the decoder so unmarshal hooks can capture lexical
// details the token stream hides (self-closing vs expanded empty elements).
func UnmarshalWithSource(data []byte, v interface{}) error {
	d := xml.NewDecoder(bytes.NewReader(data))
	d.CharsetReader = CharsetReader
	// A non-UTF-8 charset declaration makes CharsetReader transcode the stream,
	// so the decoder's InputOffset would index the transcoded UTF-8 bytes while
	// data holds the original source — every offset-based capture helper would
	// then slice the wrong (shifted) bytes and replay garbage. Skip registering
	// the source in that case so the helpers cleanly fall back to canonical
	// regeneration; such parts round-trip via preserved raw bytes, not offset
	// capture.
	if OffsetCaptureSafe(data) {
		decoderSources.Store(d, data)
		defer decoderSources.Delete(d)
	}
	if err := d.Decode(v); err != nil {
		return err
	}
	// This is the entry point every preserved part is read through, and a
	// preserved part is rewritten byte for byte. Accepting content after the
	// root element here is what let the library emit parts that do not parse.
	if err := CheckDocumentEnd(d); err != nil {
		return err
	}
	return CheckNamespacePrefixes(data)
}

// CaptureEmptyTagStyle reports how the start tag the decoder just consumed
// was written. Call it at the top of an UnmarshalXML implementation (or right
// after receiving a StartElement token): the decoder's input offset then
// points just past the tag's '>', and the preceding byte distinguishes
// <name/> from <name>. For a self-closing tag the whole whitespace run before
// the slash is captured, not merely its presence, so <t/>, <t />, <t\t/> and
// <leaf\n/> each replay verbatim. Returns EmptyTagUnknown when the decoder has
// no registered source (e.g. plain xml.Unmarshal).
func CaptureEmptyTagStyle(d *xml.Decoder) EmptyTagStyle {
	v, ok := decoderSources.Load(d)
	if !ok {
		return EmptyTagUnknown
	}
	data := v.([]byte)
	off := d.InputOffset()
	if off < 2 || off > int64(len(data)) || data[off-1] != '>' {
		return EmptyTagUnknown
	}
	if data[off-2] == '/' {
		// Walk back over the whitespace run between the tag's content and the
		// slash. The scan always terminates: a start tag begins with '<'.
		end := off - 2
		start := end
		for start > 0 && isXMLSpace(data[start-1]) {
			start--
		}
		return selfCloseStyleFor(data[start:end])
	}
	return EmptyTagExpanded
}

// InputOffsetOf returns the decoder's current input offset when it has a
// registered source (UnmarshalWithSource), for callers that capture raw
// source slices around a child decode. ok is false without a source.
func InputOffsetOf(d *xml.Decoder) (off int64, ok bool) {
	if _, has := decoderSources.Load(d); !has {
		return 0, false
	}
	return d.InputOffset(), true
}

// CaptureRawInner returns the verbatim inner bytes of the element the decoder
// just fully consumed: startOff is the offset taken right after its start tag
// (InputOffsetOf before decoding the content). The trailing end tag is
// stripped; a self-closed element yields nil. The returned slice is an
// independent copy, so callers may retain it without pinning the source.
func CaptureRawInner(d *xml.Decoder, startOff int64) []byte {
	v, ok := decoderSources.Load(d)
	if !ok {
		return nil
	}
	data := v.([]byte)
	end := d.InputOffset()
	if startOff < 0 || end > int64(len(data)) || startOff >= end {
		return nil
	}
	inner := data[startOff:end]
	// Strip the end tag (the last '<' begins it); text content cannot contain
	// a raw '<', so anything before it is the verbatim inner form.
	if i := bytes.LastIndexByte(inner, '<'); i >= 0 {
		inner = inner[:i]
	} else {
		return nil
	}
	if len(inner) == 0 {
		return nil
	}
	// Copy: inner sub-slices the (potentially large) registered source buffer.
	// A run keeps its rawText for the document's lifetime, so returning the
	// alias verbatim would pin the entire part in memory — one captured run of
	// a 50 MB document.xml would hold all 50 MB alive.
	return bytes.Clone(inner)
}

// ElementPrefix re-lexes the start tag the decoder just consumed and returns
// the element name's prefix ("" for unprefixed). It recovers the producer's
// prefix choice when several prefixes bind one URI (Word 2007 files alias the
// markup-compatibility namespace as both mc and ve). ok is false without a
// registered source.
func ElementPrefix(d *xml.Decoder) (string, bool) {
	v, has := decoderSources.Load(d)
	if !has {
		return "", false
	}
	data := v.([]byte)
	end := d.InputOffset()
	if end < 2 || end > int64(len(data)) || data[end-1] != '>' {
		return "", false
	}
	tagStart := -1
	for i := end - 2; i >= 0; i-- {
		if data[i] == '<' {
			tagStart = int(i)
			break
		}
	}
	if tagStart < 0 {
		return "", false
	}
	name := data[tagStart+1:]
	for i, c := range name {
		if c == ':' {
			return string(name[:i]), true
		}
		if c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == '>' || c == '/' {
			return "", true
		}
	}
	return "", false
}

// RawTokenBytes returns the verbatim source bytes of the token the decoder
// just consumed, given the input offset taken right before Token(). The
// returned slice is an independent copy, so callers may retain it in
// long-lived model state without pinning the source (see CaptureRawInner).
// Returns nil without a registered source or on inconsistent offsets.
func RawTokenBytes(d *xml.Decoder, pre int64) []byte {
	v, ok := decoderSources.Load(d)
	if !ok {
		return nil
	}
	data := v.([]byte)
	post := d.InputOffset()
	if pre < 0 || post > int64(len(data)) || pre >= post {
		return nil
	}
	// Copy: every production caller retains the result for the model's
	// lifetime (root comments and inter-child whitespace in docx's
	// RootExtras, duplicate color transforms, xlsx per-gap whitespace), and a
	// sub-slice of the registered source would hold the whole part alive —
	// exactly the pinning class C282 fixed elsewhere, and the reason docx
	// re-reads document.xml instead of keeping its bytes. The spans are
	// single tokens (a comment, a whitespace gap, one skipped element), so
	// the copy is cheap.
	return bytes.Clone(data[pre:post])
}

// EmptyElementStyled writes a childless element honoring a captured source
// style: expanded (<name></name>) when the capture says so, self-closing
// otherwise (matching the emission the callers used before capture existed).
func (b *Builder) EmptyElementStyled(style EmptyTagStyle, namespace, localName string, attrs ...Attr) {
	switch {
	case style == EmptyTagExpanded:
		b.StartElement(namespace, localName, attrs...)
		// Complete the deferred '>' so the pair cannot collapse.
		b.flushOpenTag()
		b.EndElement(namespace, localName)
	case style == EmptyTagSelfCloseTight || style == EmptyTagSelfCloseSpaced:
		// Per-instance spacing wins over the part-level flag: producers mix
		// "/>" and " />" within one part.
		saved := b.selfClosingSpace
		b.selfClosingSpace = style == EmptyTagSelfCloseSpaced
		b.EmptyElement(namespace, localName, attrs...)
		b.selfClosingSpace = saved
	case style >= emptyTagWSBase:
		// A captured whitespace run other than a single space.
		saved := b.selfCloseWS
		b.selfCloseWS = style.selfCloseWS()
		b.EmptyElement(namespace, localName, attrs...)
		b.selfCloseWS = saved
	default:
		b.EmptyElement(namespace, localName, attrs...)
	}
}
