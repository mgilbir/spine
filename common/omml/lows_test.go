package omml

import (
	"encoding/xml"
	"io"
	"strings"
	"testing"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// namespaceWellFormed reports the first element or attribute written with a
// prefix that no in-scope declaration binds. Go's decoder accepts such input
// (it reports the bare prefix as the name's Space), so neither plain
// well-formedness nor Builder.Finish catches it; this does. A closed
// Unmarshal(Marshal(x)) loop validated with Go's own lenient decoder can never
// detect an unbound prefix, which is exactly how the vml/omml marshal defects
// stayed green.
func namespaceWellFormed(s string) error {
	d := xml.NewDecoder(strings.NewReader(s))
	declared := map[string]bool{xmlNamespace: true}
	for {
		tok, err := d.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		for _, a := range start.Attr {
			if a.Name.Space == "xmlns" || (a.Name.Space == "" && a.Name.Local == "xmlns") {
				declared[a.Value] = true
			}
		}
		if start.Name.Space != "" && !declared[start.Name.Space] {
			return errUnbound(start.Name.Space + ":" + start.Name.Local)
		}
		for _, a := range start.Attr {
			if a.Name.Space == "" || a.Name.Space == "xmlns" {
				continue
			}
			if !declared[a.Name.Space] {
				return errUnbound(a.Name.Space + ":" + a.Name.Local)
			}
		}
	}
}

type errUnbound string

func (e errUnbound) Error() string { return "unbound namespace prefix on " + string(e) }

// assertStrictWellFormed runs the marshaled fragment through the strict
// namespace check, wrapping it in a root that declares only the prefixes the
// production document root declares.
func assertStrictWellFormed(t *testing.T, fragment string) {
	t.Helper()
	doc := `<w:document xmlns:w="` + xmlb.NSWordprocessingML + `" xmlns:m="` + NS + `">` +
		fragment + `</w:document>`
	if err := namespaceWellFormed(doc); err != nil {
		t.Errorf("emitted XML is not namespace-well-formed: %v\nXML: %s", err, fragment)
	}
}

// --- C436: m:t drops every attribute but xml:space ------------------------

func TestTextKeepsUnmodeledAttributes(t *testing.T) {
	// A producer extension (or a future schema attribute) on the single most
	// common math element must survive; every other element in the package
	// falls through to a raw capture.
	fixture := `<m:r><m:t m:foo="1" xml:space="preserve">a </m:t></m:r>`
	r := &Run{}
	parseFixture(t, fixture, r)
	if got := marshalFixture(t, r, "r"); got != fixture {
		t.Errorf("byte round-trip mismatch:\n got: %s\nwant: %s", got, fixture)
	}
	txt, ok := r.Items[0].(*Text)
	if !ok {
		t.Fatalf("Items[0] = %T, want *Text", r.Items[0])
	}
	if txt.Space != "preserve" {
		t.Errorf("Space = %q, want preserve", txt.Space)
	}
	if txt.Value != "a " {
		t.Errorf("Value = %q, want %q", txt.Value, "a ")
	}
}

func TestTextUnqualifiedUnmodeledAttribute(t *testing.T) {
	fixture := `<m:r><m:t custom="x">a</m:t></m:r>`
	r := &Run{}
	parseFixture(t, fixture, r)
	if got := marshalFixture(t, r, "r"); got != fixture {
		t.Errorf("byte round-trip mismatch:\n got: %s\nwant: %s", got, fixture)
	}
}

func TestTextProgrammaticSpaceOnly(t *testing.T) {
	// A model built in code carries no capture; it must still emit xml:space.
	r := &Run{Items: []RunChild{&Text{Space: "preserve", Value: " x "}}}
	if got, want := marshalFixture(t, r, "r"), `<m:r><m:t xml:space="preserve"> x </m:t></m:r>`; got != want {
		t.Errorf("marshal = %s, want %s", got, want)
	}
}

// --- C469: parse/emit asymmetries that invent markup ----------------------

func TestValElementsDoNotInventAttributes(t *testing.T) {
	// emitVal(required=true) could not tell "attribute absent" from
	// "attribute present and zero/empty", so an element the source wrote bare
	// came back carrying a val the document never had — for m:count a value
	// outside CT_Integer255's 1..255 range, i.e. schema-invalid markup.
	tests := []struct {
		name    string
		fixture string
	}{
		{"rSp absent", `<m:oMath><m:m><m:mPr><m:rSp/></m:mPr></m:m></m:oMath>`},
		{"cSp absent", `<m:oMath><m:m><m:mPr><m:cSp/></m:mPr></m:m></m:oMath>`},
		{"rSpRule absent", `<m:oMath><m:m><m:mPr><m:rSpRule/></m:mPr></m:m></m:oMath>`},
		{"count absent", `<m:oMath><m:m><m:mPr><m:mcs><m:mc><m:mcPr><m:count/></m:mcPr></m:mc></m:mcs></m:mPr></m:m></m:oMath>`},
		{"chr absent", `<m:oMath><m:acc><m:accPr><m:chr/></m:accPr><m:e/></m:acc></m:oMath>`},
		{"type absent", `<m:oMath><m:f><m:fPr><m:type/></m:fPr><m:num/><m:den/></m:f></m:oMath>`},
		{"argSz absent", `<m:oMath><m:f><m:num><m:argPr><m:argSz/></m:argPr></m:num><m:den/></m:f></m:oMath>`},
		{"lMargin absent", `<m:mathPr><m:lMargin/></m:mathPr>`},
		{"begChr present empty", `<m:oMath><m:d><m:dPr><m:begChr m:val=""/></m:dPr><m:e/></m:d></m:oMath>`},
		{"count present zero", `<m:oMath><m:m><m:mPr><m:mcs><m:mc><m:mcPr><m:count m:val="0"/></m:mcPr></m:mc></m:mcs></m:mPr></m:m></m:oMath>`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			local := "oMath"
			var v xmlb.BuilderMarshaler
			if strings.HasPrefix(tc.fixture, "<m:mathPr") {
				local = "mathPr"
				v = &MathPr{}
			} else {
				v = &OMath{}
			}
			parseFixture(t, tc.fixture, v)
			if got := marshalFixture(t, v, local); got != tc.fixture {
				t.Errorf("byte round-trip mismatch:\n got: %s\nwant: %s", got, tc.fixture)
			}
		})
	}
}

func TestUnqualifiedValStaysUnqualified(t *testing.T) {
	// valAttr leniently accepts an unqualified val; re-qualifying it to m:val
	// on the way out rewrites markup the source did not have.
	fixture := `<m:oMath><m:f><m:fPr><m:type val="bar"/></m:fPr><m:num/><m:den/></m:f></m:oMath>`
	m := &OMath{}
	parseFixture(t, fixture, m)
	if got := marshalFixture(t, m, "oMath"); got != fixture {
		t.Errorf("byte round-trip mismatch:\n got: %s\nwant: %s", got, fixture)
	}
}

func TestEmptyTextElementKeepsItsForm(t *testing.T) {
	fixture := `<m:r><m:t></m:t></m:r>`
	r := &Run{}
	parseFixture(t, fixture, r)
	if got := marshalFixture(t, r, "r"); got != fixture {
		t.Errorf("byte round-trip mismatch:\n got: %s\nwant: %s", got, fixture)
	}

	// The self-closing form must stay self-closing.
	selfClosed := `<m:r><m:t/></m:r>`
	r2 := &Run{}
	parseFixture(t, selfClosed, r2)
	if got := marshalFixture(t, r2, "r"); got != selfClosed {
		t.Errorf("byte round-trip mismatch:\n got: %s\nwant: %s", got, selfClosed)
	}
}

func TestValMutationAfterAbsentParseStillEmits(t *testing.T) {
	// The presence capture must not become a T-D trap: setting a value on a
	// parsed element whose attribute was absent has to emit it.
	fixture := `<m:oMath><m:f><m:fPr><m:type/></m:fPr><m:num/><m:den/></m:f></m:oMath>`
	m := &OMath{}
	parseFixture(t, fixture, m)
	f := m.Items[0].(*Fraction)
	f.FPr.Type.Val = "lin"
	want := `<m:oMath><m:f><m:fPr><m:type m:val="lin"/></m:fPr><m:num/><m:den/></m:f></m:oMath>`
	if got := marshalFixture(t, m, "oMath"); got != want {
		t.Errorf("marshal = %s, want %s", got, want)
	}
}

func TestProgrammaticValElementsEmitCanonically(t *testing.T) {
	// Values built in code carry no capture and keep the canonical emission.
	m := &OMath{Items: []MathItem{
		&Matrix{MPr: &MatrixPr{RSp: &UnSignedInteger{Val: 0}, RSpRule: &SpacingRule{Val: 0}}},
	}}
	want := `<m:oMath><m:m><m:mPr><m:rSpRule m:val="0"/><m:rSp m:val="0"/></m:mPr></m:m></m:oMath>`
	if got := marshalFixture(t, m, "oMath"); got != want {
		t.Errorf("marshal = %s, want %s", got, want)
	}
}

// --- C470: one malformed numeric attribute aborted the whole math zone -----

func TestMalformedNumericValuesDoNotAbortTheZone(t *testing.T) {
	tests := []struct {
		name    string
		fixture string
	}{
		{"brk alnAt not a number", `<m:oMath><m:r><m:rPr><m:brk m:alnAt="x"/></m:rPr><m:t>a</m:t></m:r></m:oMath>`},
		{"rSp negative", `<m:oMath><m:m><m:mPr><m:rSp m:val="-1"/></m:mPr></m:m></m:oMath>`},
		{"count not a number", `<m:oMath><m:m><m:mPr><m:mcs><m:mc><m:mcPr><m:count m:val="lots"/></m:mcPr></m:mc></m:mcs></m:mPr></m:m></m:oMath>`},
		{"argSz overflow", `<m:oMath><m:f><m:num><m:argPr><m:argSz m:val="99999999999999999999"/></m:argPr></m:num><m:den/></m:f></m:oMath>`},
		{"rSpRule not a number", `<m:oMath><m:m><m:mPr><m:rSpRule m:val="?"/></m:mPr></m:m></m:oMath>`},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := &OMath{}
			parseFixture(t, tc.fixture, m)
			if got := marshalFixture(t, m, "oMath"); got != tc.fixture {
				t.Errorf("byte round-trip mismatch:\n got: %s\nwant: %s", got, tc.fixture)
			}
		})
	}
}

func TestMalformedNumericValueYieldsToAnExplicitSet(t *testing.T) {
	fixture := `<m:oMath><m:m><m:mPr><m:rSp m:val="-1"/></m:mPr></m:m></m:oMath>`
	m := &OMath{}
	parseFixture(t, fixture, m)
	m.Items[0].(*Matrix).MPr.RSp.Val = 7
	want := `<m:oMath><m:m><m:mPr><m:rSp m:val="7"/></m:mPr></m:m></m:oMath>`
	if got := marshalFixture(t, m, "oMath"); got != want {
		t.Errorf("marshal = %s, want %s", got, want)
	}
}

// --- C471: duplicate typed children last-wins, CharData dropped -----------

func TestDuplicateTypedChildIsRawCaptured(t *testing.T) {
	tests := []string{
		`<m:oMath><m:f><m:num><m:r><m:t>1</m:t></m:r></m:num><m:num><m:r><m:t>2</m:t></m:r></m:num><m:den><m:r><m:t>3</m:t></m:r></m:den></m:f></m:oMath>`,
		`<m:oMath><m:f><m:fPr><m:type m:val="bar"/></m:fPr><m:fPr><m:type m:val="lin"/></m:fPr><m:num/><m:den/></m:f></m:oMath>`,
		`<m:oMath><m:f><m:num><m:argPr><m:argSz m:val="1"/></m:argPr><m:argPr><m:argSz m:val="2"/></m:argPr></m:num><m:den/></m:f></m:oMath>`,
		`<m:oMath><m:r><m:rPr><m:sty m:val="p"/></m:rPr><m:rPr><m:sty m:val="b"/></m:rPr><m:t>a</m:t></m:r></m:oMath>`,
		`<m:oMath><m:f><m:num><m:ctrlPr><w:rPr/></m:ctrlPr><m:ctrlPr><w:rPr/></m:ctrlPr></m:num><m:den/></m:f></m:oMath>`,
	}
	for _, fixture := range tests {
		m := &OMath{}
		parseFixture(t, fixture, m)
		if got := marshalFixture(t, m, "oMath"); got != fixture {
			t.Errorf("byte round-trip mismatch:\n got: %s\nwant: %s", got, fixture)
		}
	}
}

func TestNonWhitespaceCharDataPreserved(t *testing.T) {
	fixture := `<m:oMath><m:r><m:t>a</m:t></m:r>stray<m:r><m:t>b</m:t></m:r></m:oMath>`
	m := &OMath{}
	parseFixture(t, fixture, m)
	if got := marshalFixture(t, m, "oMath"); got != fixture {
		t.Errorf("byte round-trip mismatch:\n got: %s\nwant: %s", got, fixture)
	}
	if got, want := m.Text(), "ab"; got != want {
		t.Errorf("Text() = %q, want %q", got, want)
	}
}

// --- §5 expectation gap: Text() ignored raw-captured math -----------------

func TestTextWalksUnknownMathStructures(t *testing.T) {
	// A future m: structure lands in Raw. Extraction must not silently drop
	// its text: the zone reads "a?b", not "ab".
	fixture := `<m:oMath><m:r><m:t>a</m:t></m:r>` +
		`<m:future><m:e><m:r><m:t>?</m:t></m:r></m:e></m:future>` +
		`<m:r><m:t>b</m:t></m:r></m:oMath>`
	m := &OMath{}
	parseFixture(t, fixture, m)
	if got, want := m.Text(), "a?b"; got != want {
		t.Errorf("Text() = %q, want %q", got, want)
	}
}

// --- strict validation of everything this package emits -------------------

func TestMarshaledMathIsNamespaceWellFormed(t *testing.T) {
	fixtures := []string{
		quadraticFormula,
		`<m:oMath><m:r><m:rPr><m:sty m:val="b"/></m:rPr><w:rPr><w:i/></w:rPr><m:t>a</m:t></m:r></m:oMath>`,
		`<m:oMath><m:r><m:t m:foo="1">a</m:t></m:r></m:oMath>`,
		`<m:oMath><m:future m:val="x"><m:mystery/></m:future></m:oMath>`,
	}
	for _, fixture := range fixtures {
		m := &OMath{}
		parseFixture(t, fixture, m)
		assertStrictWellFormed(t, marshalFixture(t, m, "oMath"))
	}
}

func TestRawChildDeclaringItsOwnNamespaceIsWellFormed(t *testing.T) {
	// A raw child whose namespace is declared on the element itself must be
	// re-emitted under the prefix the source bound, with the declaration —
	// the Builder's registry knows nothing about a vendor URI.
	fixture := `<m:oMath><m:r><foo:bar xmlns:foo="urn:x-foo" a="1"/><m:t>z</m:t></m:r></m:oMath>`
	m := &OMath{}
	parseFixture(t, fixture, m)
	got := marshalFixture(t, m, "oMath")
	if got != fixture {
		t.Errorf("byte round-trip mismatch:\n got: %s\nwant: %s", got, fixture)
	}
	assertStrictWellFormed(t, got)
}

func TestRawChildWithDefaultNamespaceDeclaration(t *testing.T) {
	fixture := `<m:oMath><m:r><bar xmlns="urn:x-foo"><baz/></bar><m:t>z</m:t></m:r></m:oMath>`
	m := &OMath{}
	parseFixture(t, fixture, m)
	got := marshalFixture(t, m, "oMath")
	if got != fixture {
		t.Errorf("byte round-trip mismatch:\n got: %s\nwant: %s", got, fixture)
	}
	assertStrictWellFormed(t, got)
}
