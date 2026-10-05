package xml

import (
	"encoding/xml"
	"strings"
	"testing"
)

// benchOrderedProps is a property bag with typed, duplicated and unknown
// children, the mix UnmarshalOrderedChildren walks on a real part.
var benchOrderedProps = func() []byte {
	var sb strings.Builder
	sb.WriteString(`<w:root xmlns:w="http://example.com/w" xmlns:x="http://example.com/x"><w:props>`)
	for i := 0; i < 50; i++ {
		sb.WriteString(`<w:sz w:val="28"/><w:b/><w:b/><x:unknown x:a="1"><x:in>t</x:in></x:unknown><w:i/>`)
	}
	sb.WriteString(`</w:props></w:root>`)
	return []byte(sb.String())
}()

func BenchmarkUnmarshalOrderedChildrenSource(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var doc orderedDoc
		if err := UnmarshalWithSource(benchOrderedProps, &doc); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkUnmarshalOrderedChildrenNoSource(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		var doc orderedDoc
		if err := xml.Unmarshal(benchOrderedProps, &doc); err != nil {
			b.Fatal(err)
		}
	}
}
