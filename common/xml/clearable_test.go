package xml

import "testing"

// --- C586 / tension T-D: a modeled attribute cleared to its zero value ---
//
// collectStructAttrs used to drop omitempty-suppressed attributes before
// calling ReplayCapturedAttrs, so replay saw no modeled entry for them and fell
// back to "no modeled match: emit the captured value verbatim". That rule is
// what makes an unmodeled attribute and an explicitly-written zero survive, and
// it simultaneously made *clearing* a parsed value impossible: the zero the
// setter just wrote was suppressed and the source's value came back.
//
// Suppressed attributes are now passed through as a separate "cleared" list.
// Replay drops a captured entry only when the captured value provably denotes a
// different value than the zero the model now holds — which can only be true if
// the model changed after parse, since the capture is a snapshot of what the
// parse read.

// A cleared string attribute (the C584 shape: CommonSlideData.Name) no longer
// resurrects the parsed value.
func TestReplay_C584_ClearedStringAttributeDropsCapture(t *testing.T) {
	type node struct {
		Name string `xml:"name,attr,omitempty"`

		CapturedAttrs []RootAttr `xml:"-"`
	}
	n := node{
		Name: "", // cleared by the caller
		CapturedAttrs: []RootAttr{
			{LocalName: "name", Value: "Title 1", Raw: ` name="Title 1"`},
		},
	}
	b := NewBuilder()
	b.RegisterNamespace("urn:x", "x")
	b.MarshalElement("urn:x", "n", &n)
	if err := b.Finish(); err != nil {
		t.Fatalf("builder: %v", err)
	}
	want := `<x:n/>`
	if got := b.String(); got != want {
		t.Errorf("cleared name attribute was resurrected from the capture\ngot  %s\nwant %s", got, want)
	}
}

// A cleared boolean attribute (the C583 shape: a:tblPr@firstRow and siblings).
func TestReplay_C583_ClearedBoolAttributeDropsCapture(t *testing.T) {
	type node struct {
		FirstRow bool `xml:"firstRow,attr,omitempty"`
		BandRow  bool `xml:"bandRow,attr,omitempty"`

		CapturedAttrs []RootAttr `xml:"-"`
	}
	n := node{
		FirstRow: false, // cleared
		BandRow:  true,  // untouched
		CapturedAttrs: []RootAttr{
			{LocalName: "firstRow", Value: "1", Raw: ` firstRow="1"`},
			{LocalName: "rtl", Value: "1", Raw: ` rtl="1"`}, // unmodeled: must survive
			{LocalName: "bandRow", Value: "1", Raw: ` bandRow="1"`},
		},
	}
	b := NewBuilder()
	b.RegisterNamespace("urn:x", "x")
	b.MarshalElement("urn:x", "n", &n)
	if err := b.Finish(); err != nil {
		t.Fatalf("builder: %v", err)
	}
	want := `<x:n rtl="1" bandRow="1"/>`
	if got := b.String(); got != want {
		t.Errorf("clearing one flag must drop only that flag\ngot  %s\nwant %s", got, want)
	}
}

// A cleared numeric attribute (the C585 shape: p:ph@idx).
func TestReplay_C585_ClearedUintAttributeDropsCapture(t *testing.T) {
	type node struct {
		Idx uint32 `xml:"idx,attr,omitempty"`

		CapturedAttrs []RootAttr `xml:"-"`
	}
	n := node{Idx: 0, CapturedAttrs: []RootAttr{
		{LocalName: "type", Value: "body", Raw: ` type="body"`},
		{LocalName: "idx", Value: "3", Raw: ` idx="3"`},
	}}
	b := NewBuilder()
	b.RegisterNamespace("urn:x", "x")
	b.MarshalElement("urn:x", "n", &n)
	if err := b.Finish(); err != nil {
		t.Fatalf("builder: %v", err)
	}
	want := `<x:n type="body"/>`
	if got := b.String(); got != want {
		t.Errorf("cleared idx was resurrected from the capture\ngot  %s\nwant %s", got, want)
	}
}

// The counterpart the capture kit exists for: a producer that wrote the zero
// *explicitly*. The model agrees with the capture, so nothing was cleared and
// the captured rendering — including its lexical form — must survive verbatim.
// Dropping these would be a byte drift always, and a semantic inversion for
// every XSD default-TRUE or default-nonzero attribute (showComments="0",
// spokes="0").
func TestReplay_ExplicitZeroSurvivesUntouched(t *testing.T) {
	type node struct {
		FirstRow bool   `xml:"firstRow,attr,omitempty"`
		ShowCmt  bool   `xml:"showComments,attr,omitempty"`
		Spokes   int    `xml:"spokes,attr,omitempty"`
		Name     string `xml:"name,attr,omitempty"`

		CapturedAttrs []RootAttr `xml:"-"`
	}
	n := node{
		CapturedAttrs: []RootAttr{
			{LocalName: "firstRow", Value: "0", Raw: ` firstRow="0"`},
			{LocalName: "showComments", Value: "false", Raw: ` showComments="false"`},
			{LocalName: "spokes", Value: "0", Raw: ` spokes="0"`},
			{LocalName: "name", Value: "", Raw: ` name=""`},
		},
	}
	b := NewBuilder()
	b.RegisterNamespace("urn:x", "x")
	b.MarshalElement("urn:x", "n", &n)
	if err := b.Finish(); err != nil {
		t.Fatalf("builder: %v", err)
	}
	want := `<x:n firstRow="0" showComments="false" spokes="0" name=""/>`
	if got := b.String(); got != want {
		t.Errorf("an explicitly-written zero must round-trip verbatim\ngot  %s\nwant %s", got, want)
	}
}

// A captured value the model's lexical domain cannot interpret means the parse
// cannot have produced the field's current zero from it — so "the caller
// cleared it" is not a sound conclusion and the capture is kept.
func TestReplay_UninterpretableCapturedValueIsKept(t *testing.T) {
	type node struct {
		Idx uint32 `xml:"idx,attr,omitempty"`

		CapturedAttrs []RootAttr `xml:"-"`
	}
	n := node{Idx: 0, CapturedAttrs: []RootAttr{
		{LocalName: "idx", Value: "not-a-number", Raw: ` idx="not-a-number"`},
	}}
	b := NewBuilder()
	b.RegisterNamespace("urn:x", "x")
	b.MarshalElement("urn:x", "n", &n)
	if err := b.Finish(); err != nil {
		t.Fatalf("builder: %v", err)
	}
	want := `<x:n idx="not-a-number"/>`
	if got := b.String(); got != want {
		t.Errorf("an uninterpretable captured value must be kept\ngot  %s\nwant %s", got, want)
	}
}

// A nil pointer is "we hold no value", not "the caller cleared it": the parse
// populates a pointer field whenever the source carried the attribute, so a nil
// pointer alongside a captured attribute means the model never read it. Keeping
// the capture is the safe reading, and it is what preserves the XSD
// default-TRUE attributes modeled as *bool (C526).
func TestReplay_NilPointerDoesNotClearCapture(t *testing.T) {
	type node struct {
		ShowCmt *bool `xml:"showComments,attr,omitempty"`

		CapturedAttrs []RootAttr `xml:"-"`
	}
	n := node{CapturedAttrs: []RootAttr{
		{LocalName: "showComments", Value: "0", Raw: ` showComments="0"`},
	}}
	b := NewBuilder()
	b.RegisterNamespace("urn:x", "x")
	b.MarshalElement("urn:x", "n", &n)
	if err := b.Finish(); err != nil {
		t.Fatalf("builder: %v", err)
	}
	want := `<x:n showComments="0"/>`
	if got := b.String(); got != want {
		t.Errorf("a nil pointer must not clear a captured attribute\ngot  %s\nwant %s", got, want)
	}
}

// A modeled attribute set to a *non-zero* value still wins over the capture,
// and a cleared attribute in one element does not disturb another's.
func TestReplay_ModeledNonZeroStillWins(t *testing.T) {
	type node struct {
		Idx  uint32 `xml:"idx,attr,omitempty"`
		Name string `xml:"name,attr,omitempty"`

		CapturedAttrs []RootAttr `xml:"-"`
	}
	n := node{Idx: 9, Name: "", CapturedAttrs: []RootAttr{
		{LocalName: "idx", Value: "3", Raw: ` idx="3"`},
		{LocalName: "name", Value: "old", Raw: ` name="old"`},
	}}
	b := NewBuilder()
	b.RegisterNamespace("urn:x", "x")
	b.MarshalElement("urn:x", "n", &n)
	if err := b.Finish(); err != nil {
		t.Fatalf("builder: %v", err)
	}
	want := `<x:n idx="9"/>`
	if got := b.String(); got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

// A cleared attribute is matched by namespace + local name too, so a capture
// whose prefix could not be resolved still pairs with its modeled field.
func TestReplay_ClearedNamespacedAttributeDropsCapture(t *testing.T) {
	type node struct {
		Embed string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships embed,attr,omitempty"`

		CapturedAttrs []RootAttr `xml:"-"`
	}
	n := node{Embed: "", CapturedAttrs: []RootAttr{
		{LocalName: "embed", Prefix: "r", Space: NSOfficeDocumentRels, Value: "rId3", Raw: ` r:embed="rId3"`},
	}}
	b := NewBuilder()
	b.RegisterNamespace("urn:x", "x")
	b.RegisterNamespace(NSOfficeDocumentRels, "r")
	b.MarshalElement("urn:x", "n", &n)
	if err := b.Finish(); err != nil {
		t.Fatalf("builder: %v", err)
	}
	want := `<x:n/>`
	if got := b.String(); got != want {
		t.Errorf("cleared namespaced attribute was resurrected\ngot  %s\nwant %s", got, want)
	}
}

// The docx model tags its attributes with the full WordprocessingML URI, and
// the stdlib decoder only fills such a field from an attribute in that same
// namespace. A producer that writes the attribute *unprefixed* therefore leaves
// the modeled field zero while the capture still records it — the model never
// read that attribute, so it cannot have been cleared.
//
// Matching a cleared attribute to a captured one is namespace-aware for exactly
// this reason: an unqualified capture never pairs with a namespaced modeled
// field, so it is kept rather than deleted. Every docx type whose fields carry
// a URI-qualified tag depends on this.
func TestReplay_UnprefixedCaptureDoesNotPairWithNamespacedField(t *testing.T) {
	const nsW = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"
	type node struct {
		Color string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main color,attr,omitempty"`

		CapturedAttrs []RootAttr `xml:"-"`
	}
	n := node{Color: "", CapturedAttrs: []RootAttr{
		// Unprefixed in the source: Space is empty, so the stdlib never filled
		// Color and the field's zero means "not read", not "cleared".
		{LocalName: "color", Value: "FFFFFF", Raw: ` color="FFFFFF"`},
	}}
	b := NewBuilder()
	b.RegisterNamespace(nsW, "w")
	b.MarshalElement(nsW, "background", &n)
	if err := b.Finish(); err != nil {
		t.Fatalf("builder: %v", err)
	}
	want := `<w:background color="FFFFFF"/>`
	if got := b.String(); got != want {
		t.Errorf("an unqualified captured attribute must not be cleared by a namespaced field\ngot  %s\nwant %s", got, want)
	}
}

// The DrawingML readers degrade a malformed integer attribute to zero on
// purpose (roundInt64 / coerceIntAttrs: a wild file writes rot="0.4" for an
// angle the schema types as an integer) and rely on the capture to carry the
// original lexeme through the save. The field's zero there is an artifact of
// that coercion, not a clear — and a fractional lexeme is not a member of an
// integer field's lexical space, so it is replayed verbatim.
func TestReplay_LenientlyCoercedIntegerIsNotAClear(t *testing.T) {
	type node struct {
		Rot int32 `xml:"rot,attr,omitempty"`

		CapturedAttrs []RootAttr `xml:"-"`
	}
	for _, tc := range []struct{ name, captured string }{
		{"fractional", "0.4"},
		{"unparseable", "abc"},
		{"empty", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			n := node{Rot: 0, CapturedAttrs: []RootAttr{
				{LocalName: "rot", Value: tc.captured, Raw: ` rot="` + tc.captured + `"`},
			}}
			b := NewBuilder()
			b.RegisterNamespace("urn:x", "x")
			b.MarshalElement("urn:x", "xfrm", &n)
			if err := b.Finish(); err != nil {
				t.Fatalf("builder: %v", err)
			}
			want := `<x:xfrm rot="` + tc.captured + `"/>`
			if got := b.String(); got != want {
				t.Errorf("a leniently-coerced value must survive\ngot  %s\nwant %s", got, want)
			}
		})
	}
}

// An unsigned field can hold values past MaxInt64, so the integer reading must
// not fall back to "unreadable" (and silently keep a stale attribute) there.
func TestReplay_ClearedLargeUintDropsCapture(t *testing.T) {
	type node struct {
		ID uint64 `xml:"id,attr,omitempty"`

		CapturedAttrs []RootAttr `xml:"-"`
	}
	n := node{ID: 0, CapturedAttrs: []RootAttr{
		{LocalName: "id", Value: "18446744073709551615", Raw: ` id="18446744073709551615"`},
	}}
	b := NewBuilder()
	b.RegisterNamespace("urn:x", "x")
	b.MarshalElement("urn:x", "n", &n)
	if err := b.Finish(); err != nil {
		t.Fatalf("builder: %v", err)
	}
	if got, want := b.String(), `<x:n/>`; got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

// --- formatValue's %g: lexical drift on an unmodified numeric attribute ---
//
// A float attribute re-renders through %g, so a producer's
// tint="-4.9989318521683403E-2" came back as a decimal expansion and val="1.0"
// as val="1" — a byte drift on a document nobody edited. Replay already keeps
// the producer's form when a modeled boolean differs only lexically from the
// capture ("false" vs "0"); numeric attributes now get the same treatment.
func TestReplay_C531_NumericLexicalFormPreserved(t *testing.T) {
	type node struct {
		Val  float64 `xml:"val,attr,omitempty"`
		Tint float64 `xml:"tint,attr,omitempty"`
		Idx  int     `xml:"idx,attr,omitempty"`

		CapturedAttrs []RootAttr `xml:"-"`
	}
	n := node{
		Val:  1.0,
		Tint: -4.9989318521683403e-2,
		Idx:  7,
		CapturedAttrs: []RootAttr{
			{LocalName: "val", Value: "1.0", Raw: ` val="1.0"`},
			{LocalName: "tint", Value: "-4.9989318521683403E-2", Raw: ` tint="-4.9989318521683403E-2"`},
			{LocalName: "idx", Value: "007", Raw: ` idx="007"`},
		},
	}
	b := NewBuilder()
	b.RegisterNamespace("urn:x", "x")
	b.MarshalElement("urn:x", "n", &n)
	if err := b.Finish(); err != nil {
		t.Fatalf("builder: %v", err)
	}
	want := `<x:n val="1.0" tint="-4.9989318521683403E-2" idx="007"/>`
	if got := b.String(); got != want {
		t.Errorf("an unmodified numeric attribute must keep the producer's lexical form\ngot  %s\nwant %s", got, want)
	}
}

// A numeric attribute the caller actually changed re-renders; the stale source
// form is not kept.
func TestReplay_NumericChangeStillRerenders(t *testing.T) {
	type node struct {
		Val float64 `xml:"val,attr,omitempty"`

		CapturedAttrs []RootAttr `xml:"-"`
	}
	n := node{Val: 2.5, CapturedAttrs: []RootAttr{
		{LocalName: "val", Value: "1.0", Raw: ` val="1.0"`},
	}}
	b := NewBuilder()
	b.RegisterNamespace("urn:x", "x")
	b.MarshalElement("urn:x", "n", &n)
	if err := b.Finish(); err != nil {
		t.Fatalf("builder: %v", err)
	}
	want := `<x:n val="2.5"/>`
	if got := b.String(); got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

// Numeric equivalence is keyed off the *field's* kind, not the value's shape: a
// string-typed attribute whose values merely look numeric keeps the caller's
// exact spelling.
func TestReplay_StringAttributeIsNotNumericallyCompared(t *testing.T) {
	type node struct {
		Sz string `xml:"sz,attr,omitempty"`

		CapturedAttrs []RootAttr `xml:"-"`
	}
	n := node{Sz: "0.50", CapturedAttrs: []RootAttr{
		{LocalName: "sz", Value: "0.5", Raw: ` sz="0.5"`},
	}}
	b := NewBuilder()
	b.RegisterNamespace("urn:x", "x")
	b.MarshalElement("urn:x", "n", &n)
	if err := b.Finish(); err != nil {
		t.Fatalf("builder: %v", err)
	}
	want := `<x:n sz="0.50"/>`
	if got := b.String(); got != want {
		t.Errorf("a string attribute must not be compared numerically\ngot  %s\nwant %s", got, want)
	}
}
