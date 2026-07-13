package xml

import (
	"encoding/xml"
	"testing"
)

type styledLeaf struct {
	Val string `xml:"val,attr,omitempty"`

	CapturedEmptyTag EmptyTagStyle `xml:"-"`
}

func (l *styledLeaf) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	l.CapturedEmptyTag = CaptureEmptyTagStyle(d)
	type alias styledLeaf
	return d.DecodeElement((*alias)(l), &start)
}

// CaptureEmptyTagStyle distinguishes <leaf/> from <leaf></leaf> via the
// registered decoder source, and the reflection marshaler replays the
// captured form through the conventional CapturedEmptyTag field.
func TestEmptyTagStyleCaptureAndReplay(t *testing.T) {
	cases := []struct {
		src  string
		want EmptyTagStyle
		out  string
	}{
		{`<leaf val="1"/>`, EmptyTagSelfClose, `<leaf val="1"/>`},
		{`<leaf val="1"></leaf>`, EmptyTagExpanded, `<leaf val="1"></leaf>`},
	}
	for _, tc := range cases {
		var l styledLeaf
		if err := UnmarshalWithSource([]byte(tc.src), &l); err != nil {
			t.Fatalf("unmarshal %q: %v", tc.src, err)
		}
		if l.CapturedEmptyTag != tc.want {
			t.Errorf("%q: style = %d, want %d", tc.src, l.CapturedEmptyTag, tc.want)
		}
		b := NewBuilder()
		b.RegisterNamespace("", "")
		b.MarshalElement("", "leaf", &l)
		if err := b.Finish(); err != nil {
			t.Fatalf("builder: %v", err)
		}
		if got := b.String(); got != tc.out {
			t.Errorf("%q: marshaled %q, want %q", tc.src, got, tc.out)
		}
	}
}

// Without a registered source (plain xml.Unmarshal), the capture stays
// unknown and the marshaler keeps the previous self-closing emission.
func TestEmptyTagStyleUnknownWithoutSource(t *testing.T) {
	var l styledLeaf
	if err := xml.Unmarshal([]byte(`<leaf val="1"></leaf>`), &l); err != nil {
		t.Fatal(err)
	}
	if l.CapturedEmptyTag != EmptyTagUnknown {
		t.Errorf("style = %d, want EmptyTagUnknown", l.CapturedEmptyTag)
	}
	b := NewBuilder()
	b.MarshalElement("", "leaf", &l)
	if got := b.String(); got != `<leaf val="1"/>` {
		t.Errorf("marshaled %q, want self-closing default", got)
	}
}
