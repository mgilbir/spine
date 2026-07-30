package omml

import (
	"encoding/xml"
	"io"
	"strings"
	"testing"
	"time"
	"unicode"

	xmlb "github.com/mgilbir/spine/common/xml"
	"github.com/mgilbir/spine/internal/fuzzbound"
)

// mathBudget bounds one parse of an OMML fragment.
//
// The amplifier this guards is the raw capture: an element the model does not
// type keeps its inner XML verbatim, so a change that made that capture recurse
// into nested unknown elements would cost O(n^2) on a chain of them — exactly
// the input a fuzzer reaches first. Measured on the corpus' largest equations
// (12 KB of m:oMathPara) the parse allocates around 20x the fragment size; a
// 1 MiB floor covers the decoder's fixed buffers, which dominate on the tiny
// fragments a fuzz run is mostly made of, and a 64x rate stays far below
// quadratic while leaving room for a legitimately capture-heavy equation.
var mathBudget = fuzzbound.Budget{
	What:              "omml parse",
	Bytes:             1 << 20,
	BytesPerInputByte: 64,
	Time:              10 * time.Second,
	TimePerMiB:        5 * time.Second,
}

// mathDecls are the bindings a w:document root carries for math content. They
// are injected onto the fragment's own start tag rather than a wrapper, as
// docx.unmarshalMath does, because only a decode whose root *is* the value
// being parsed registers the source bytes the lexical captures (empty-tag
// style) read back.
const mathDecls = ` xmlns:m="` + NS + `" xmlns:w="` + xmlb.NSWordprocessingML + `"`

// withMathDecls injects mathDecls onto a fragment rooted at tag.
func withMathDecls(src, tag string) (string, bool) {
	if !strings.HasPrefix(src, tag) {
		return "", false
	}
	rest := src[len(tag):]
	if rest == "" || !strings.ContainsAny(rest[:1], " \t\r\n/>") {
		return "", false // <m:oMathPara…> is not <m:oMath…>
	}
	return tag + mathDecls + rest, true
}

// cycleMath parses a fragment rooted at m:<localName> and marshals it back
// through the production Builder, returning the marshaled fragment. The
// declarations are injected here rather than by the caller so the second cycle
// reads the first one's output in the same live prefix scope a document gives
// it. ok is false when a step failed.
func cycleMath[T any, PT interface {
	*T
	xmlb.BuilderMarshaler
}](src, localName string) (out string, v PT, ok bool) {
	full, ok := withMathDecls(src, "<m:"+localName)
	if !ok {
		return "", nil, false
	}
	v = PT(new(T))
	if err := xmlb.UnmarshalWithSource([]byte(full), v); err != nil {
		return "", nil, false
	}
	b := xmlb.NewBuilder()
	b.RegisterNamespace(NS, xmlb.PrefixMath)
	b.RegisterNamespace(xmlb.NSWordprocessingML, xmlb.PrefixWordprocessingML)
	v.MarshalToBuilder(b, NS, localName)
	if err := b.Finish(); err != nil {
		return "", nil, false
	}
	return b.String(), v, true
}

// conformant reports whether a fragment is input a producer could actually
// have written: every prefix it uses is bound to a non-empty URI, every
// element is in a namespace, every element and attribute name is an NCName,
// and it holds no directive or processing instruction in element content.
//
// The gate exists because Go's decoder is markedly more permissive than XML,
// and each thing it lets through drives the model somewhere this package has
// never promised to go:
//
//   - `<m::/>` is reported as an element whose local name is ":", and
//     `<A A:0=""/>` as an attribute named "0" in an undeclared namespace "A".
//     The verbatim-capture path replays those names, producing output that is
//     not namespace-well-formed or — for the first — not well-formed at all.
//   - `xmlns:m14=""` is forbidden outright by XML Namespaces 1.0 (only the
//     *default* declaration may be undeclared with an empty URI). Go accepts it
//     and reports the element as being in no namespace.
//   - An element in no namespace inside a math zone hits raw.go's documented
//     normalization: a capture with an empty Space is re-emitted in the math
//     namespace. That is a deliberate one-time re-homing, and it legitimately
//     changes what Text() sees, because the re-homed element is then recognized
//     as math. Asserting Text() stability across it would be asserting that a
//     documented normalization does not happen.
//
// All of these are "reject malformed input" promises that this package has
// never made and no OOXML producer can trigger. The promise under test is
// fidelity, so the strong assertions are made over input a producer could
// write, and outside it the case simply ends.
func conformant(src string) bool {
	if namespaceWellFormed(src) != nil {
		return false
	}
	d := xml.NewDecoder(strings.NewReader(src))
	for {
		tok, err := d.Token()
		if err == io.EOF {
			return true
		}
		if err != nil {
			return false
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Space == "" || !isNCName(t.Name.Local) {
				return false
			}
			for _, a := range t.Attr {
				if !isNCName(a.Name.Local) {
					return false
				}
				if a.Name.Space == "xmlns" && a.Value == "" {
					return false
				}
			}
		case xml.EndElement:
			if !isNCName(t.Name.Local) {
				return false
			}
		case xml.Directive, xml.ProcInst:
			return false
		}
	}
}

// isNCName reports whether s is an XML Namespaces NCName — a name with no
// colon, as every element and attribute local name must be.
func isNCName(s string) bool {
	if s == "" {
		return false
	}
	for i, r := range s {
		switch {
		case unicode.IsLetter(r), r == '_':
		case i > 0 && (unicode.IsDigit(r) || r == '-' || r == '.' || r == '·'):
		default:
			return false
		}
	}
	return true
}

// FuzzOMathRoundTrip drives m:oMath, the math zone docx reaches through a
// paragraph's math runs. Its content is an ordered heterogeneous sequence —
// runs, fractions, radicals, n-ary operators, matrices — interleaved with
// WordprocessingML this package cannot type and keeps verbatim, so the order
// and the captures are the whole contract.
//
// Four properties are asserted, all stronger than "did not panic":
//
//   - The output of a successful cycle parses and marshals again. Held to only
//     for conformant input; see conformant for what Go's decoder lets through
//     otherwise.
//   - Fixed point. Parsing what the first marshal produced and marshaling it
//     again must reproduce it byte for byte. This package aims higher than a
//     fixed point — every fixture in math_test.go is byte-identical to its
//     source, and all 115 distinct m:oMath elements in the corpus round-trip
//     byte for byte — but byte identity is not assertable for arbitrary input
//     (attribute order, entity spelling and inter-element whitespace all
//     normalize), whereas the fixed point is.
//   - Namespace well-formedness of the output. Go's decoder reports an unbound
//     prefix as the name's namespace, so a closed Unmarshal/Marshal loop
//     validated with Go's own decoder cannot see a marshal that emits a prefix
//     nothing declares — which is exactly how earlier omml marshal defects
//     stayed green (see namespaceWellFormed). The raw-capture path replays a
//     producer's own xmlns declarations, and a mutation that breaks that
//     replay shows up here.
//   - Text extraction is stable across the cycle. Text() walks the same items
//     the marshal does, so a cycle that relocated or dropped an m:t changes it.
func FuzzOMathRoundTrip(f *testing.F) {
	f.Add(quadraticFormula)
	// Word writes w:rPr run formatting inside every math run; it is the most
	// common raw capture in the corpus by an order of magnitude.
	f.Add(`<m:oMath><m:r><w:rPr><w:rFonts w:ascii="Cambria Math" w:hAnsi="Cambria Math"/><w:sz w:val="24"/></w:rPr>` +
		`<m:t xml:space="preserve">ϕ=0,75 </m:t></m:r></m:oMath>`)
	f.Add(`<m:oMath><m:sSub><m:sSubPr><m:ctrlPr><w:rPr><w:i/></w:rPr></m:ctrlPr></m:sSubPr>` +
		`<m:e><m:r><m:t>α</m:t></m:r></m:e><m:sub><m:r><m:rPr><m:sty m:val="p"/></m:rPr><m:t>c</m:t></m:r></m:sub></m:sSub></m:oMath>`)
	f.Add(`<m:oMath><m:nary><m:naryPr><m:chr m:val="∑"/><m:limLoc m:val="undOvr"/></m:naryPr>` +
		`<m:sub><m:r><m:t>i=1</m:t></m:r></m:sub><m:sup><m:r><m:t>n</m:t></m:r></m:sup>` +
		`<m:e><m:r><m:t>i</m:t></m:r></m:e></m:nary></m:oMath>`)
	f.Add(`<m:oMath><m:d><m:dPr><m:begChr m:val="{"/><m:endChr m:val=""/></m:dPr>` +
		`<m:e><m:eqArr><m:e><m:r><m:t>a</m:t></m:r></m:e><m:e><m:r><m:t>b</m:t></m:r></m:e></m:eqArr></m:e></m:d></m:oMath>`)
	// An unknown element carrying its own declaration is the raw-capture path
	// that has to replay a prefix the Builder's registry knows nothing about.
	f.Add(`<m:oMath><m14:future xmlns:m14="urn:x-future" a="1"><m:r><m:t>z</m:t></m:r></m14:future></m:oMath>`)
	f.Add(`<m:oMath/>`)
	f.Add(`<m:oMath></m:oMath>`)

	f.Fuzz(func(t *testing.T, src string) {
		if !strings.HasPrefix(src, "<m:oMath") || fuzzbound.Tripped() {
			return
		}
		var first string
		var m1 *OMath
		ok := false
		mathBudget.Check(t, len(src), func() {
			first, m1, ok = cycleMath[OMath](src, "oMath")
		})
		if !ok {
			return
		}
		assertMathFixedPoint(t, mathStrict(src, "oMath"), "oMath", first, m1.Text(), func(s string) (string, string, bool) {
			out, m2, ok := cycleMath[OMath](s, "oMath")
			if !ok {
				return "", "", false
			}
			return out, m2.Text(), true
		})
	})
}

// FuzzMathParaRoundTrip drives m:oMathPara, the display-equation container.
// It is not reachable from FuzzOMathRoundTrip's root — a math paragraph holds
// math zones, not the other way round — and it exercises the fixed-sequence
// machinery (unmarshalSeq / marshalSeq), which anchors an unknown child to the
// schema child that preceded it, rather than OMath's flat ordered item list.
//
// Same oracles as FuzzOMathRoundTrip.
func FuzzMathParaRoundTrip(f *testing.F) {
	f.Add(`<m:oMathPara><m:oMathParaPr><m:jc m:val="left"/></m:oMathParaPr>` +
		`<m:oMath><m:r><m:t>x</m:t></m:r></m:oMath></m:oMathPara>`)
	f.Add(`<m:oMathPara><m:oMath><m:r><m:t>a</m:t></m:r></m:oMath>` +
		`<m:oMath><m:r><m:t>b</m:t></m:r></m:oMath></m:oMathPara>`)
	f.Add(`<m:oMathPara><m:oMathParaPr><m:jc m:val="centerGroup"/></m:oMathParaPr>` +
		`<m:unknown x="1"/><m:oMath/></m:oMathPara>`)
	f.Add(`<m:oMathPara/>`)

	f.Fuzz(func(t *testing.T, src string) {
		if !strings.HasPrefix(src, "<m:oMathPara") || fuzzbound.Tripped() {
			return
		}
		var first string
		var p1 *OMathPara
		ok := false
		mathBudget.Check(t, len(src), func() {
			first, p1, ok = cycleMath[OMathPara](src, "oMathPara")
		})
		if !ok {
			return
		}
		assertMathFixedPoint(t, mathStrict(src, "oMathPara"), "oMathPara", first, p1.Text(), func(s string) (string, string, bool) {
			out, p2, ok := cycleMath[OMathPara](s, "oMathPara")
			if !ok {
				return "", "", false
			}
			return out, p2.Text(), true
		})
	})
}

// mathStrict reports whether src, read in the prefix scope a document gives a
// fragment rooted at m:<localName>, is input a producer could have written.
// The gate is on the *input*: what the model emits is a replay of what it read,
// so a name or prefix the source made up is the source's, not the model's.
func mathStrict(src, localName string) bool {
	full, ok := withMathDecls(src, "<m:"+localName)
	return ok && conformant(full)
}

// assertMathFixedPoint checks the properties documented on FuzzOMathRoundTrip
// against first, the fragment the package produced. strict says whether the
// input was conformant, and so whether the promises that only hold over
// producible input apply.
//
// The fixed point is asserted from the *second* marshal on for input outside
// that domain, because there the first pass carries a real normalization: an
// element in no namespace is a raw capture, and raw.go re-emits it in the math
// namespace, where the next parse recognizes it as the modeled child it names.
// `<endChr A=""/>` therefore becomes `<m:endChr A=""/>` and then `<m:endChr/>`,
// the unmodeled attribute surviving only as long as the element was a verbatim
// capture. That is the documented re-homing, not drift; drift that is not that
// persists past the second pass and still fails here.
func assertMathFixedPoint(t *testing.T, strict bool, localName, first, firstText string, cycle func(string) (string, string, bool)) {
	t.Helper()
	second, secondText, ok := cycle(first)
	if !ok {
		if strict {
			t.Fatalf("the output of a successful parse+marshal must itself parse and marshal:\n%s", first)
		}
		return
	}
	if strict {
		full, _ := withMathDecls(first, "<m:"+localName)
		if err := namespaceWellFormed(full); err != nil {
			t.Fatalf("emitted XML is not namespace-well-formed: %v\n%s", err, first)
		}
		if first != second {
			t.Fatalf("marshal is not a fixed point:\nfirst:  %s\nsecond: %s", first, second)
		}
		if firstText != secondText {
			t.Fatalf("Text() changed across a marshal/unmarshal cycle: %q -> %q\n%s", firstText, secondText, first)
		}
	}
	third, thirdText, ok := cycle(second)
	if !ok {
		t.Fatalf("the third cycle must succeed once the second did:\n%s", second)
	}
	if second != third {
		t.Fatalf("marshal is not a fixed point after normalization:\nsecond: %s\nthird:  %s", second, third)
	}
	if secondText != thirdText {
		t.Fatalf("Text() changed after normalization: %q -> %q\n%s", secondText, thirdText, second)
	}
}
