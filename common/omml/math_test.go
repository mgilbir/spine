package omml

import (
	"reflect"
	"strings"
	"testing"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// parseFixture decodes a fixture fragment (rooted at any m: element) into v,
// binding the m: and w: prefixes the way a WML document root does. The
// declarations go on the fixture's own root element rather than a wrapper so
// the fixture is the decoded document: the production path
// (docx.unmarshalMath) does the same, and only a decode whose root is the
// value being parsed can register its source bytes for the lexical captures
// (empty-tag style).
func parseFixture(t *testing.T, fixture string, v interface{}) {
	t.Helper()
	decls := ` xmlns:m="` + NS + `" xmlns:w="` + xmlb.NSWordprocessingML + `"`
	cut := strings.IndexAny(fixture, " \t\r\n/>")
	if !strings.HasPrefix(fixture, "<") || cut < 0 {
		t.Fatalf("fixture does not start with an element: %s", fixture)
	}
	src := fixture[:cut] + decls + fixture[cut:]
	if err := xmlb.UnmarshalWithSource([]byte(src), v); err != nil {
		t.Fatalf("Unmarshal failed: %v\nXML: %s", err, fixture)
	}
}

// marshalFixture renders v through the production Builder with the m: and w:
// prefixes registered (as the WML document marshaler does).
func marshalFixture(t *testing.T, v xmlb.BuilderMarshaler, localName string) string {
	t.Helper()
	b := xmlb.NewBuilder()
	b.RegisterNamespace(NS, xmlb.PrefixMath)
	b.RegisterNamespace(xmlb.NSWordprocessingML, xmlb.PrefixWordprocessingML)
	v.MarshalToBuilder(b, NS, localName)
	if err := b.Finish(); err != nil {
		t.Fatalf("Builder error: %v", err)
	}
	return b.String()
}

// roundTripBytes asserts unmarshal→marshal reproduces the fixture exactly and
// returns the parsed value for structural assertions.
func roundTripOMath(t *testing.T, fixture string) *OMath {
	t.Helper()
	m := &OMath{}
	parseFixture(t, fixture, m)
	if got := marshalFixture(t, m, "oMath"); got != fixture {
		t.Errorf("byte round-trip mismatch:\n got: %s\nwant: %s", got, fixture)
	}
	return m
}

const quadraticFormula = `<m:oMath><m:r><m:t>x=</m:t></m:r>` +
	`<m:f><m:fPr><m:type m:val="bar"/></m:fPr>` +
	`<m:num><m:r><m:t>−b±</m:t></m:r>` +
	`<m:rad><m:radPr><m:degHide m:val="1"/></m:radPr><m:deg/>` +
	`<m:e><m:sSup><m:e><m:r><m:t>b</m:t></m:r></m:e><m:sup><m:r><m:t>2</m:t></m:r></m:sup></m:sSup>` +
	`<m:r><m:t>−4ac</m:t></m:r></m:e></m:rad></m:num>` +
	`<m:den><m:r><m:t>2a</m:t></m:r></m:den></m:f></m:oMath>`

func TestQuadraticFormulaRoundTrip(t *testing.T) {
	m := roundTripOMath(t, quadraticFormula)

	if len(m.Items) != 2 {
		t.Fatalf("Items = %d, want 2", len(m.Items))
	}
	f, ok := m.Items[1].(*Fraction)
	if !ok {
		t.Fatalf("Items[1] = %T, want *Fraction", m.Items[1])
	}
	if f.FPr == nil || f.FPr.Type == nil || f.FPr.Type.Val != "bar" {
		t.Error("fraction type not parsed")
	}
	if len(f.Num.Items) != 2 {
		t.Fatalf("numerator items = %d, want 2", len(f.Num.Items))
	}
	rad, ok := f.Num.Items[1].(*Radical)
	if !ok {
		t.Fatalf("numerator Items[1] = %T, want *Radical", f.Num.Items[1])
	}
	if rad.RadPr == nil || rad.RadPr.DegHide == nil || rad.RadPr.DegHide.Val != "1" {
		t.Error("radical degHide not parsed")
	}
	if rad.Deg == nil || len(rad.Deg.Items) != 0 {
		t.Error("empty degree argument not preserved")
	}
	if got, want := m.Text(), "x=−b±b2−4ac2a"; got != want {
		t.Errorf("Text() = %q, want %q", got, want)
	}
}

func TestCorpusRoundTrips(t *testing.T) {
	fixtures := []struct {
		name string
		xml  string
		text string // expected Text(); "-" to skip the check
	}{
		{
			name: "nary sum with limits",
			xml: `<m:oMath><m:nary><m:naryPr><m:chr m:val="∑"/><m:limLoc m:val="undOvr"/></m:naryPr>` +
				`<m:sub><m:r><m:t>i=1</m:t></m:r></m:sub><m:sup><m:r><m:t>n</m:t></m:r></m:sup>` +
				`<m:e><m:sSup><m:e><m:r><m:t>i</m:t></m:r></m:e><m:sup><m:r><m:t>2</m:t></m:r></m:sup></m:sSup></m:e>` +
				`</m:nary></m:oMath>`,
			text: "i=1ni2",
		},
		{
			name: "matrix in brackets",
			xml: `<m:oMath><m:d><m:dPr><m:begChr m:val="["/><m:endChr m:val="]"/></m:dPr>` +
				`<m:e><m:m><m:mPr><m:mcs><m:mc><m:mcPr><m:count m:val="2"/><m:mcJc m:val="center"/></m:mcPr></m:mc></m:mcs></m:mPr>` +
				`<m:mr><m:e><m:r><m:t>a</m:t></m:r></m:e><m:e><m:r><m:t>b</m:t></m:r></m:e></m:mr>` +
				`<m:mr><m:e><m:r><m:t>c</m:t></m:r></m:e><m:e><m:r><m:t>d</m:t></m:r></m:e></m:mr>` +
				`</m:m></m:e></m:d></m:oMath>`,
			text: "abcd",
		},
		{
			name: "delimiter with separator and two args",
			xml: `<m:oMath><m:d><m:dPr><m:begChr m:val="("/><m:sepChr m:val="|"/><m:endChr m:val=")"/>` +
				`<m:grow m:val="1"/><m:shp m:val="centered"/></m:dPr>` +
				`<m:e><m:r><m:t>a</m:t></m:r></m:e><m:e><m:r><m:t>b</m:t></m:r></m:e></m:d></m:oMath>`,
			text: "ab",
		},
		{
			name: "accent and bar",
			xml: `<m:oMath><m:acc><m:accPr><m:chr m:val="̂"/></m:accPr><m:e><m:r><m:t>a</m:t></m:r></m:e></m:acc>` +
				`<m:bar><m:barPr><m:pos m:val="top"/></m:barPr><m:e><m:r><m:t>x</m:t></m:r></m:e></m:bar></m:oMath>`,
			text: "ax",
		},
		{
			name: "runs with wml rPr styling",
			xml: `<m:oMath><m:r><m:rPr><m:sty m:val="b"/></m:rPr><w:rPr><w:b/><w:color w:val="FF0000"/></w:rPr><m:t>E</m:t></m:r>` +
				`<m:r><w:rPr><w:i/></w:rPr><m:t>=mc</m:t></m:r></m:oMath>`,
			text: "E=mc",
		},
		{
			name: "nested fractions",
			xml: `<m:oMath><m:f><m:num><m:f><m:num><m:r><m:t>1</m:t></m:r></m:num><m:den><m:r><m:t>x</m:t></m:r></m:den></m:f></m:num>` +
				`<m:den><m:r><m:t>1+x</m:t></m:r></m:den></m:f></m:oMath>`,
			text: "1x1+x",
		},
		{
			name: "equation array with spacing",
			xml: `<m:oMath><m:eqArr><m:eqArrPr><m:baseJc m:val="top"/><m:maxDist m:val="1"/>` +
				`<m:rSpRule m:val="3"/><m:rSp m:val="720"/></m:eqArrPr>` +
				`<m:e><m:r><m:t>a</m:t></m:r></m:e><m:e><m:r><m:t>b</m:t></m:r></m:e></m:eqArr></m:oMath>`,
			text: "ab",
		},
		{
			name: "function with ctrlPr formatting",
			xml: `<m:oMath><m:func><m:funcPr><m:ctrlPr><w:rPr><w:i/></w:rPr></m:ctrlPr></m:funcPr>` +
				`<m:fName><m:r><m:t>sin</m:t></m:r></m:fName><m:e><m:r><m:t>x</m:t></m:r></m:e></m:func></m:oMath>`,
			text: "sinx",
		},
		{
			name: "upper limit",
			xml: `<m:oMath><m:limUpp><m:e><m:r><m:t>x</m:t></m:r></m:e>` +
				`<m:lim><m:r><m:t>n→∞</m:t></m:r></m:lim></m:limUpp></m:oMath>`,
			text: "xn→∞",
		},
		{
			name: "lower limit with limLowPr",
			xml: `<m:oMath><m:limLow><m:limLowPr><m:ctrlPr/></m:limLowPr><m:e><m:r><m:t>lim</m:t></m:r></m:e>` +
				`<m:lim><m:r><m:t>x→0</m:t></m:r></m:lim></m:limLow></m:oMath>`,
			text: "limx→0",
		},
		{
			name: "group character underbrace",
			xml: `<m:oMath><m:groupChr><m:groupChrPr><m:chr m:val="⏟"/><m:pos m:val="bot"/><m:vertJc m:val="top"/></m:groupChrPr>` +
				`<m:e><m:r><m:t>abc</m:t></m:r></m:e></m:groupChr></m:oMath>`,
			text: "abc",
		},
		{
			name: "box with manual break",
			xml: `<m:oMath><m:box><m:boxPr><m:opEmu m:val="1"/><m:brk m:alnAt="2"/></m:boxPr>` +
				`<m:e><m:r><m:t>+</m:t></m:r></m:e></m:box></m:oMath>`,
			text: "+",
		},
		{
			name: "border box with strikes",
			xml: `<m:oMath><m:borderBox><m:borderBoxPr><m:hideTop m:val="1"/><m:strikeH m:val="1"/></m:borderBoxPr>` +
				`<m:e><m:r><m:t>y</m:t></m:r></m:e></m:borderBox></m:oMath>`,
			text: "y",
		},
		{
			name: "phantom",
			xml: `<m:oMath><m:phant><m:phantPr><m:show m:val="0"/><m:zeroWid m:val="1"/></m:phantPr>` +
				`<m:e><m:r><m:t>y</m:t></m:r></m:e></m:phant></m:oMath>`,
			text: "y",
		},
		{
			name: "pre-sub-superscript",
			xml: `<m:oMath><m:sPre><m:sub><m:r><m:t>n</m:t></m:r></m:sub><m:sup><m:r><m:t>p</m:t></m:r></m:sup>` +
				`<m:e><m:r><m:t>C</m:t></m:r></m:e></m:sPre></m:oMath>`,
			text: "npC",
		},
		{
			name: "subscript",
			xml: `<m:oMath><m:sSub><m:e><m:r><m:t>x</m:t></m:r></m:e>` +
				`<m:sub><m:r><m:t>1</m:t></m:r></m:sub></m:sSub></m:oMath>`,
			text: "x1",
		},
		{
			name: "sub-superscript with alnScr",
			xml: `<m:oMath><m:sSubSup><m:sSubSupPr><m:alnScr m:val="1"/></m:sSubSupPr>` +
				`<m:e><m:r><m:t>f</m:t></m:r></m:e><m:sub><m:r><m:t>2</m:t></m:r></m:sub>` +
				`<m:sup><m:r><m:t>3</m:t></m:r></m:sup></m:sSubSup></m:oMath>`,
			text: "f23",
		},
		{
			name: "argument properties",
			xml: `<m:oMath><m:box><m:boxPr><m:noBreak m:val="0"/><m:ctrlPr/></m:boxPr>` +
				`<m:e><m:argPr><m:argSz m:val="-1"/></m:argPr><m:r><m:t>abc</m:t></m:r></m:e></m:box></m:oMath>`,
			text: "abc",
		},
		{
			name: "run properties full set",
			xml: `<m:oMath><m:r><m:rPr><m:lit m:val="1"/><m:scr m:val="fraktur"/><m:sty m:val="p"/>` +
				`<m:brk m:alnAt="1"/><m:aln/></m:rPr><m:t>q</m:t></m:r></m:oMath>`,
			text: "q",
		},
		{
			name: "text with xml:space preserve",
			xml:  `<m:oMath><m:r><m:t xml:space="preserve"> + </m:t></m:r></m:oMath>`,
			text: " + ",
		},
		{
			name: "wml run content inside run",
			xml:  `<m:oMath><m:r><m:rPr><m:sty m:val="p"/></m:rPr><w:br/></m:r></m:oMath>`,
			text: "",
		},
	}

	for _, tt := range fixtures {
		t.Run(tt.name, func(t *testing.T) {
			m := roundTripOMath(t, tt.xml)
			if tt.text != "-" {
				if got := m.Text(); got != tt.text {
					t.Errorf("Text() = %q, want %q", got, tt.text)
				}
			}
		})
	}
}

func TestOrderPreservation(t *testing.T) {
	fixture := `<m:oMath><m:r><m:t>a</m:t></m:r>` +
		`<m:f><m:num><m:r><m:t>1</m:t></m:r></m:num><m:den><m:r><m:t>2</m:t></m:r></m:den></m:f>` +
		`<m:r><m:t>b</m:t></m:r>` +
		`<m:sSup><m:e><m:r><m:t>c</m:t></m:r></m:e><m:sup><m:r><m:t>2</m:t></m:r></m:sup></m:sSup></m:oMath>`
	m := roundTripOMath(t, fixture)

	wantKinds := []string{"*omml.Run", "*omml.Fraction", "*omml.Run", "*omml.Superscript"}
	if len(m.Items) != len(wantKinds) {
		t.Fatalf("Items = %d, want %d", len(m.Items), len(wantKinds))
	}
	for i, it := range m.Items {
		if got := reflect.TypeOf(it).String(); got != wantKinds[i] {
			t.Errorf("Items[%d] = %s, want %s", i, got, wantKinds[i])
		}
	}
	if got, want := m.Text(), "a12bc2"; got != want {
		t.Errorf("Text() = %q, want %q", got, want)
	}
}

func TestUnknownChildRoundTripsRaw(t *testing.T) {
	// Unknown element between two runs in a math zone: must be preserved
	// verbatim, in position.
	fixture := `<m:oMath><m:r><m:t>a</m:t></m:r>` +
		`<m:future m:val="x"><m:mystery/></m:future>` +
		`<m:r><m:t>b</m:t></m:r></m:oMath>`
	m := roundTripOMath(t, fixture)

	raw, ok := m.Items[1].(*Raw)
	if !ok {
		t.Fatalf("Items[1] = %T, want *Raw", m.Items[1])
	}
	if raw.Local != "future" || raw.Space != NS {
		t.Errorf("raw name = {%s %s}, want {future %s}", raw.Space, raw.Local, NS)
	}
	if string(raw.Content) != "<m:mystery/>" {
		t.Errorf("raw content = %q", raw.Content)
	}
}

func TestUnknownChildInPropertiesKeepsPosition(t *testing.T) {
	// Unknown child between the schema children of a fixed-sequence Pr
	// element: the anchored raw capture must re-emit it in position.
	fixture := `<m:oMath><m:f><m:fPr><m:type m:val="lin"/><m:future/><m:ctrlPr/></m:fPr>` +
		`<m:num><m:r><m:t>1</m:t></m:r></m:num><m:den><m:r><m:t>2</m:t></m:r></m:den></m:f></m:oMath>`
	roundTripOMath(t, fixture)

	// At the very start of a container, too.
	fixture = `<m:oMath><m:f><m:fPr><m:future/><m:type m:val="lin"/></m:fPr>` +
		`<m:num><m:r><m:t>1</m:t></m:r></m:num><m:den><m:r><m:t>2</m:t></m:r></m:den></m:f></m:oMath>`
	roundTripOMath(t, fixture)
}

func TestOMathParaRoundTrip(t *testing.T) {
	fixture := `<m:oMathPara><m:oMathParaPr><m:jc m:val="center"/></m:oMathParaPr>` +
		`<m:oMath><m:r><m:t>a</m:t></m:r></m:oMath>` +
		`<m:oMath><m:r><m:t>b</m:t></m:r></m:oMath></m:oMathPara>`
	p := &OMathPara{}
	parseFixture(t, fixture, p)
	if got := marshalFixture(t, p, "oMathPara"); got != fixture {
		t.Errorf("byte round-trip mismatch:\n got: %s\nwant: %s", got, fixture)
	}
	if p.OMathParaPr == nil || p.OMathParaPr.Jc == nil || p.OMathParaPr.Jc.Val != "center" {
		t.Error("oMathParaPr/jc not parsed")
	}
	if len(p.OMath) != 2 {
		t.Fatalf("OMath = %d, want 2", len(p.OMath))
	}
	if got, want := p.Text(), "ab"; got != want {
		t.Errorf("Text() = %q, want %q", got, want)
	}
}

func TestCtrlPrRawCapture(t *testing.T) {
	fixture := `<m:ctrlPr><w:rPr><w:rFonts w:ascii="Cambria Math"/><w:i/></w:rPr></m:ctrlPr>`
	c := &CtrlPr{}
	parseFixture(t, fixture, c)
	if got := marshalFixture(t, c, "ctrlPr"); got != fixture {
		t.Errorf("byte round-trip mismatch:\n got: %s\nwant: %s", got, fixture)
	}
	if len(c.Items) != 1 || c.Items[0].Local != "rPr" || c.Items[0].Space != xmlb.NSWordprocessingML {
		t.Fatalf("ctrlPr content not raw-captured: %+v", c.Items)
	}
}

func TestMathPrRoundTrip(t *testing.T) {
	fixture := `<m:mathPr><m:mathFont m:val="Cambria Math"/><m:brkBin m:val="before"/>` +
		`<m:brkBinSub m:val="--"/><m:smallFrac m:val="0"/><m:dispDef/>` +
		`<m:lMargin m:val="0"/><m:rMargin m:val="0"/><m:defJc m:val="centerGroup"/>` +
		`<m:wrapIndent m:val="1440"/><m:intLim m:val="subSup"/><m:naryLim m:val="undOvr"/></m:mathPr>`
	p := &MathPr{}
	parseFixture(t, fixture, p)
	if got := marshalFixture(t, p, "mathPr"); got != fixture {
		t.Errorf("byte round-trip mismatch:\n got: %s\nwant: %s", got, fixture)
	}
	if p.MathFont == nil || p.MathFont.Val != "Cambria Math" {
		t.Error("mathFont not parsed")
	}
}

func TestEmptyValAttributePreserved(t *testing.T) {
	// A cases construct: Word writes an empty required delimiter character as
	// m:val="" (attribute present, value empty). Required-val types must
	// re-emit it.
	fixture := `<m:oMath><m:d><m:dPr><m:begChr m:val="{"/><m:endChr m:val=""/></m:dPr>` +
		`<m:e><m:r><m:t>x</m:t></m:r></m:e></m:d></m:oMath>`
	roundTripOMath(t, fixture)
}

func TestProgrammaticBuildMarshals(t *testing.T) {
	// A model built in code (no parse, so no captured ordering) marshals in
	// schema order.
	m := &OMath{Items: []MathItem{
		&Run{Items: []RunChild{&Text{Value: "x="}}},
		&Fraction{
			Num: &Element{Items: []MathItem{&Run{Items: []RunChild{&Text{Value: "1"}}}}},
			Den: &Element{Items: []MathItem{&Run{Items: []RunChild{&Text{Value: "2"}}}}},
		},
	}}
	want := `<m:oMath><m:r><m:t>x=</m:t></m:r>` +
		`<m:f><m:num><m:r><m:t>1</m:t></m:r></m:num><m:den><m:r><m:t>2</m:t></m:r></m:den></m:f></m:oMath>`
	if got := marshalFixture(t, m, "oMath"); got != want {
		t.Errorf("marshal = %s, want %s", got, want)
	}
}

func TestMutatedParsedModelKeepsNewChildren(t *testing.T) {
	// Adding a schema child to a parsed container must marshal it at its
	// schema position even though it is absent from the captured layout.
	fixture := `<m:oMath><m:f><m:num><m:r><m:t>1</m:t></m:r></m:num><m:den><m:r><m:t>2</m:t></m:r></m:den></m:f></m:oMath>`
	m := roundTripOMath(t, fixture)
	f := m.Items[0].(*Fraction)
	f.FPr = &FractionPr{Type: &FType{Val: "lin"}}

	want := `<m:oMath><m:f><m:fPr><m:type m:val="lin"/></m:fPr>` +
		`<m:num><m:r><m:t>1</m:t></m:r></m:num><m:den><m:r><m:t>2</m:t></m:r></m:den></m:f></m:oMath>`
	if got := marshalFixture(t, m, "oMath"); got != want {
		t.Errorf("marshal = %s, want %s", got, want)
	}
}

func TestBuilderErrorOnUnboundRawNamespace(t *testing.T) {
	// A raw child whose namespace has no registered prefix must surface as a
	// Builder error, not ship a part with an unbound prefix.
	m := &OMath{Items: []MathItem{
		&Raw{Local: "foo", Space: "urn:not-registered"},
	}}
	b := xmlb.NewBuilder()
	b.RegisterNamespace(NS, xmlb.PrefixMath)
	m.MarshalToBuilder(b, NS, "oMath")
	if err := b.Finish(); err == nil {
		t.Error("expected Builder error for unregistered raw namespace")
	}
}

func TestStrayCharDataIgnored(t *testing.T) {
	// Whitespace / stray text between structural elements (pretty-printed
	// sources) is not content; it must parse cleanly and normalize away.
	fixture := "<m:oMath>\n  <m:r>\n    <m:t>a</m:t>\n  </m:r>\n</m:oMath>"
	m := &OMath{}
	parseFixture(t, fixture, m)
	want := `<m:oMath><m:r><m:t>a</m:t></m:r></m:oMath>`
	if got := marshalFixture(t, m, "oMath"); got != want {
		t.Errorf("marshal = %s, want %s", got, want)
	}
}
