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
	// EmptyTagSelfClose is <name/>.
	EmptyTagSelfClose
	// EmptyTagExpanded is <name></name>.
	EmptyTagExpanded
)

// decoderSources maps a live *xml.Decoder to the raw bytes it reads, letting
// unmarshal hooks inspect the exact source form of the token just consumed.
var decoderSources sync.Map

// UnmarshalWithSource decodes data into v like xml.Unmarshal, with the source
// bytes registered for the decoder so unmarshal hooks can capture lexical
// details the token stream hides (self-closing vs expanded empty elements).
func UnmarshalWithSource(data []byte, v interface{}) error {
	d := xml.NewDecoder(bytes.NewReader(data))
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
		return EmptyTagSelfClose
	}
	return EmptyTagExpanded
}

// EmptyElementStyled writes a childless element honoring a captured source
// style: expanded (<name></name>) when the capture says so, self-closing
// otherwise (matching the emission the callers used before capture existed).
func (b *Builder) EmptyElementStyled(style EmptyTagStyle, namespace, localName string, attrs ...Attr) {
	if style == EmptyTagExpanded {
		b.StartElement(namespace, localName, attrs...)
		// Complete the deferred '>' so the pair cannot collapse.
		b.flushOpenTag()
		b.EndElement(namespace, localName)
		return
	}
	b.EmptyElement(namespace, localName, attrs...)
}
