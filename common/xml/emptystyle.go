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
	// EmptyTagSelfCloseSpaced is <name /> even when the part-level style
	// writes "/>".
	EmptyTagSelfCloseSpaced
)

// IsSelfClose reports whether the style is any self-closing form.
func (s EmptyTagStyle) IsSelfClose() bool {
	return s == EmptyTagSelfClose || s == EmptyTagSelfCloseTight || s == EmptyTagSelfCloseSpaced
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
	decoderSources.Store(d, data)
	defer decoderSources.Delete(d)
	return d.Decode(v)
}

// CaptureEmptyTagStyle reports how the start tag the decoder just consumed
// was written. Call it at the top of an UnmarshalXML implementation (or right
// after receiving a StartElement token): the decoder's input offset then
// points just past the tag's '>', and the preceding byte distinguishes
// <name/> from <name>. Returns EmptyTagUnknown when the decoder has no
// registered source (e.g. plain xml.Unmarshal).
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
		if off >= 3 && (data[off-3] == ' ' || data[off-3] == '\t') {
			return EmptyTagSelfCloseSpaced
		}
		return EmptyTagSelfCloseTight
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
// just consumed, given the input offset taken right before Token(). Returns
// nil without a registered source or on inconsistent offsets.
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
	return data[pre:post]
}

// EmptyElementStyled writes a childless element honoring a captured source
// style: expanded (<name></name>) when the capture says so, self-closing
// otherwise (matching the emission the callers used before capture existed).
func (b *Builder) EmptyElementStyled(style EmptyTagStyle, namespace, localName string, attrs ...Attr) {
	switch style {
	case EmptyTagExpanded:
		b.StartElement(namespace, localName, attrs...)
		// Complete the deferred '>' so the pair cannot collapse.
		b.flushOpenTag()
		b.EndElement(namespace, localName)
	case EmptyTagSelfCloseTight, EmptyTagSelfCloseSpaced:
		// Per-instance spacing wins over the part-level flag: producers mix
		// "/>" and " />" within one part.
		saved := b.selfClosingSpace
		b.selfClosingSpace = style == EmptyTagSelfCloseSpaced
		b.EmptyElement(namespace, localName, attrs...)
		b.selfClosingSpace = saved
	default:
		b.EmptyElement(namespace, localName, attrs...)
	}
}
