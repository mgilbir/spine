package xml

import (
	"encoding/xml"
	"strings"
	"testing"
)

// orderedProps is a stand-in for a properties bag like w:rPr: singleton
// pointer children in a fixed declaration order plus a slice child.
type orderedProps struct {
	B                *orderedOnOff `xml:"http://example.com/w b,omitempty"`
	I                *orderedOnOff `xml:"http://example.com/w i,omitempty"`
	Sz               *orderedVal   `xml:"http://example.com/w sz,omitempty"`
	Alt              []*orderedVal `xml:"http://example.com/w alt,omitempty"`
	CapturedChildren *ChildCapture `xml:"-"`
	CapturedEmptyTag EmptyTagStyle `xml:"-"`
}

type orderedOnOff struct {
	Val string `xml:"http://example.com/w val,attr,omitempty"`
}

type orderedVal struct {
	Val string `xml:"http://example.com/w val,attr,omitempty"`
}

func (p *orderedProps) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	p.CapturedEmptyTag = CaptureEmptyTagStyle(d)
	return UnmarshalOrderedChildren(d, p)
}

// orderedDoc wraps orderedProps under a root, mirroring how the docx model
// nests property bags.
type orderedDoc struct {
	XMLName xml.Name      `xml:"http://example.com/w root"`
	Props   *orderedProps `xml:"http://example.com/w props"`
}

func marshalOrderedDoc(t *testing.T, doc *orderedDoc) string {
	t.Helper()
	b := NewBuilder()
	b.RegisterNamespace("http://example.com/w", "w")
	b.MarshalElement("http://example.com/w", "props", doc.Props)
	return b.String()
}

func TestOrderedChildren_SourceOrderAndUnknownReplay(t *testing.T) {
	src := `<w:root xmlns:w="http://example.com/w" xmlns:w14="http://example.com/w14">` +
		`<w:props><w:sz w:val="28"/><w:b/><w14:glow w14:rad="101600"><w14:srgbClr/></w14:glow><w:i/></w:props>` +
		`</w:root>`
	var doc orderedDoc
	if err := UnmarshalWithSource([]byte(src), &doc); err != nil {
		t.Fatal(err)
	}
	got := marshalOrderedDoc(t, &doc)
	want := `<w:props><w:sz w:val="28"/><w:b/><w14:glow w14:rad="101600"><w14:srgbClr/></w14:glow><w:i/></w:props>`
	if got != want {
		t.Errorf("replay mismatch:\n got %s\nwant %s", got, want)
	}
}

func TestOrderedChildren_DuplicateSingletonPreserved(t *testing.T) {
	src := `<w:root xmlns:w="http://example.com/w">` +
		`<w:props><w:b/><w:b/><w:i w:val="false"/><w:i w:val="false"/><w:sz w:val="24"/></w:props>` +
		`</w:root>`
	var doc orderedDoc
	if err := UnmarshalWithSource([]byte(src), &doc); err != nil {
		t.Fatal(err)
	}
	got := marshalOrderedDoc(t, &doc)
	want := `<w:props><w:b/><w:b/><w:i w:val="false"/><w:i w:val="false"/><w:sz w:val="24"/></w:props>`
	if got != want {
		t.Errorf("duplicate replay mismatch:\n got %s\nwant %s", got, want)
	}
	// The typed model sees single fields (first occurrence wins).
	if doc.Props.B == nil || doc.Props.I == nil || doc.Props.I.Val != "false" {
		t.Errorf("typed fields not populated from first occurrences: %+v", doc.Props)
	}
}

func TestOrderedChildren_PostParseEditsWin(t *testing.T) {
	src := `<w:root xmlns:w="http://example.com/w">` +
		`<w:props><w:sz w:val="28"/><w:b/></w:props>` +
		`</w:root>`
	var doc orderedDoc
	if err := UnmarshalWithSource([]byte(src), &doc); err != nil {
		t.Fatal(err)
	}
	doc.Props.Sz.Val = "36"       // mutate: new value at captured position
	doc.Props.B = nil             // remove
	doc.Props.I = &orderedOnOff{} // add: spliced at its declaration rank
	got := marshalOrderedDoc(t, &doc)
	want := `<w:props><w:sz w:val="36"/><w:i/></w:props>`
	if got != want {
		t.Errorf("post-parse edit mismatch:\n got %s\nwant %s", got, want)
	}
}

// InsertTypedField places a field set after parse at its schema position rather
// than appending it after every captured child.
func TestInsertTypedField_SchemaPosition(t *testing.T) {
	// Source in declaration order but missing the middle field (i).
	src := `<w:root xmlns:w="http://example.com/w">` +
		`<w:props><w:b/><w:sz w:val="28"/></w:props>` +
		`</w:root>`
	var doc orderedDoc
	if err := UnmarshalWithSource([]byte(src), &doc); err != nil {
		t.Fatal(err)
	}
	doc.Props.I = &orderedOnOff{} // add the middle field (field index 1)
	doc.Props.CapturedChildren.InsertTypedField(1)
	got := marshalOrderedDoc(t, &doc)
	want := `<w:props><w:b/><w:i/><w:sz w:val="28"/></w:props>`
	if got != want {
		t.Errorf("schema-position insert mismatch:\n got %s\nwant %s", got, want)
	}
}

// InsertTypedField inserts before a trailing raw (unmodeled) child, mirroring a
// new lvlNpPr added ahead of a captured a:extLst.
func TestInsertTypedField_BeforeTrailingRaw(t *testing.T) {
	src := `<w:root xmlns:w="http://example.com/w" xmlns:w14="http://example.com/w14">` +
		`<w:props><w:b/><w14:glow w14:rad="1"/></w:props>` +
		`</w:root>`
	var doc orderedDoc
	if err := UnmarshalWithSource([]byte(src), &doc); err != nil {
		t.Fatal(err)
	}
	doc.Props.Sz = &orderedVal{Val: "24"} // add field index 2
	doc.Props.CapturedChildren.InsertTypedField(2)
	got := marshalOrderedDoc(t, &doc)
	want := `<w:props><w:b/><w:sz w:val="24"/><w14:glow w14:rad="1"/></w:props>`
	if got != want {
		t.Errorf("insert-before-raw mismatch:\n got %s\nwant %s", got, want)
	}
}

// C329: a field set after parse is spliced at its declaration rank without the
// caller having to say so — the append that used to happen here emitted the new
// child after every captured sibling, which for a type whose content model is
// an xsd:sequence is schema-invalid.
func TestOrderedChildren_AddedFieldSplicedAtSchemaPosition(t *testing.T) {
	src := `<w:root xmlns:w="http://example.com/w">` +
		`<w:props><w:b/><w:sz w:val="28"/></w:props>` +
		`</w:root>`
	var doc orderedDoc
	if err := UnmarshalWithSource([]byte(src), &doc); err != nil {
		t.Fatal(err)
	}
	doc.Props.I = &orderedOnOff{} // the middle field, no InsertTypedField call
	got := marshalOrderedDoc(t, &doc)
	want := `<w:props><w:b/><w:i/><w:sz w:val="28"/></w:props>`
	if got != want {
		t.Errorf("added field not spliced at its rank:\n got %s\nwant %s", got, want)
	}
}

// The splice lands before a trailing raw (unmodeled) child, which is where the
// schema puts a typed sibling relative to an extension list.
func TestOrderedChildren_AddedFieldPrecedesTrailingRaw(t *testing.T) {
	src := `<w:root xmlns:w="http://example.com/w" xmlns:w14="http://example.com/w14">` +
		`<w:props><w:b/><w14:glow w14:rad="1"/></w:props>` +
		`</w:root>`
	var doc orderedDoc
	if err := UnmarshalWithSource([]byte(src), &doc); err != nil {
		t.Fatal(err)
	}
	doc.Props.Sz = &orderedVal{Val: "24"}
	got := marshalOrderedDoc(t, &doc)
	want := `<w:props><w:b/><w:sz w:val="24"/><w14:glow w14:rad="1"/></w:props>`
	if got != want {
		t.Errorf("added field not placed before the trailing raw child:\n got %s\nwant %s", got, want)
	}
}

// Several fields added at once come out in declaration order among themselves.
func TestOrderedChildren_MultipleAddedFieldsStayOrdered(t *testing.T) {
	src := `<w:root xmlns:w="http://example.com/w">` +
		`<w:props><w:alt w:val="1"/></w:props>` +
		`</w:root>`
	var doc orderedDoc
	if err := UnmarshalWithSource([]byte(src), &doc); err != nil {
		t.Fatal(err)
	}
	doc.Props.Sz = &orderedVal{Val: "24"}
	doc.Props.B = &orderedOnOff{}
	doc.Props.I = &orderedOnOff{}
	got := marshalOrderedDoc(t, &doc)
	want := `<w:props><w:b/><w:i/><w:sz w:val="24"/><w:alt w:val="1"/></w:props>`
	if got != want {
		t.Errorf("multiple added fields mismatch:\n got %s\nwant %s", got, want)
	}
}

// A source whose children were already out of schema order replays exactly as
// it came in: the splice must never re-sort what was captured, or a
// zero-modification save of the many real documents shaped like this would stop
// being byte-identical.
func TestOrderedChildren_OutOfOrderSourceIsNotResorted(t *testing.T) {
	src := `<w:root xmlns:w="http://example.com/w">` +
		`<w:props><w:sz w:val="28"/><w:i/><w:b/></w:props>` +
		`</w:root>`
	var doc orderedDoc
	if err := UnmarshalWithSource([]byte(src), &doc); err != nil {
		t.Fatal(err)
	}
	want := `<w:props><w:sz w:val="28"/><w:i/><w:b/></w:props>`
	if got := marshalOrderedDoc(t, &doc); got != want {
		t.Errorf("captured order was re-sorted:\n got %s\nwant %s", got, want)
	}
	// Adding a field to such a bag places it after the last smaller-ranked
	// captured child; nothing already present moves.
	doc.Props.Alt = append(doc.Props.Alt, &orderedVal{Val: "9"})
	want = `<w:props><w:sz w:val="28"/><w:i/><w:b/><w:alt w:val="9"/></w:props>`
	if got := marshalOrderedDoc(t, &doc); got != want {
		t.Errorf("addition to an out-of-order bag mismatch:\n got %s\nwant %s", got, want)
	}
}

// The splice must not mutate the capture: marshaling twice yields the same
// bytes, and the recorded source order is untouched.
func TestOrderedChildren_SpliceLeavesCaptureIntact(t *testing.T) {
	src := `<w:root xmlns:w="http://example.com/w">` +
		`<w:props><w:sz w:val="28"/></w:props>` +
		`</w:root>`
	var doc orderedDoc
	if err := UnmarshalWithSource([]byte(src), &doc); err != nil {
		t.Fatal(err)
	}
	doc.Props.B = &orderedOnOff{}
	before := len(doc.Props.CapturedChildren.Order)
	first := marshalOrderedDoc(t, &doc)
	second := marshalOrderedDoc(t, &doc)
	if first != second {
		t.Errorf("marshal is not idempotent:\n first  %s\n second %s", first, second)
	}
	if got := len(doc.Props.CapturedChildren.Order); got != before {
		t.Errorf("marshal mutated the capture: %d entries, want %d", got, before)
	}
}

func TestOrderedChildren_SliceOrderInterleaved(t *testing.T) {
	src := `<w:root xmlns:w="http://example.com/w">` +
		`<w:props><w:alt w:val="1"/><w:b/><w:alt w:val="2"/></w:props>` +
		`</w:root>`
	var doc orderedDoc
	if err := UnmarshalWithSource([]byte(src), &doc); err != nil {
		t.Fatal(err)
	}
	got := marshalOrderedDoc(t, &doc)
	want := `<w:props><w:alt w:val="1"/><w:b/><w:alt w:val="2"/></w:props>`
	if got != want {
		t.Errorf("slice interleave mismatch:\n got %s\nwant %s", got, want)
	}
	// Appended slice entries follow at the end.
	doc.Props.Alt = append(doc.Props.Alt, &orderedVal{Val: "3"})
	got = marshalOrderedDoc(t, &doc)
	want = `<w:props><w:alt w:val="1"/><w:b/><w:alt w:val="2"/><w:alt w:val="3"/></w:props>`
	if got != want {
		t.Errorf("slice append mismatch:\n got %s\nwant %s", got, want)
	}
}

func TestOrderedChildren_RawOnlyElementStaysExpanded(t *testing.T) {
	src := `<w:root xmlns:w="http://example.com/w" xmlns:x="http://example.com/x">` +
		`<w:props><x:unknown/></w:props>` +
		`</w:root>`
	var doc orderedDoc
	if err := UnmarshalWithSource([]byte(src), &doc); err != nil {
		t.Fatal(err)
	}
	got := marshalOrderedDoc(t, &doc)
	want := `<w:props><x:unknown/></w:props>`
	if got != want {
		t.Errorf("raw-only replay mismatch:\n got %s\nwant %s", got, want)
	}
}

func TestOrderedChildren_NoSourceRegisteredDropsUnknown(t *testing.T) {
	// Plain xml.Unmarshal has no registered source: unknown children are
	// skipped (the pre-capture behavior), typed children still decode and
	// keep their order.
	src := `<w:root xmlns:w="http://example.com/w" xmlns:x="http://example.com/x">` +
		`<w:props><w:sz w:val="28"/><x:unknown/><w:b/></w:props>` +
		`</w:root>`
	var doc orderedDoc
	if err := xml.Unmarshal([]byte(src), &doc); err != nil {
		t.Fatal(err)
	}
	got := marshalOrderedDoc(t, &doc)
	want := `<w:props><w:sz w:val="28"/><w:b/></w:props>`
	if got != want {
		t.Errorf("no-source mismatch:\n got %s\nwant %s", got, want)
	}
}

func TestOrderedChildren_EmptyElementKeepsStyle(t *testing.T) {
	for _, tc := range []struct{ src, want string }{
		{`<w:props></w:props>`, `<w:props></w:props>`},
		{`<w:props/>`, `<w:props/>`},
	} {
		full := `<w:root xmlns:w="http://example.com/w">` + tc.src + `</w:root>`
		var doc orderedDoc
		if err := UnmarshalWithSource([]byte(full), &doc); err != nil {
			t.Fatal(err)
		}
		got := marshalOrderedDoc(t, &doc)
		if got != tc.want {
			t.Errorf("empty style mismatch:\n got %s\nwant %s", got, tc.want)
		}
	}
}

func TestOrderedChildren_MutatedDuplicateKeepsRawSecond(t *testing.T) {
	// Mutating the typed field changes the first occurrence only; the raw
	// duplicate replays with its source bytes.
	src := `<w:root xmlns:w="http://example.com/w">` +
		`<w:props><w:sz w:val="28"/><w:sz w:val="30"/></w:props>` +
		`</w:root>`
	var doc orderedDoc
	if err := UnmarshalWithSource([]byte(src), &doc); err != nil {
		t.Fatal(err)
	}
	doc.Props.Sz.Val = "44"
	got := marshalOrderedDoc(t, &doc)
	if !strings.Contains(got, `<w:sz w:val="44"/><w:sz w:val="30"/>`) {
		t.Errorf("mutated duplicate mismatch: %s", got)
	}
}
