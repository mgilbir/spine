package docx

import (
	"strings"
	"testing"
)

// styleXML returns word/styles.xml from a saved document, and the fragment of
// it belonging to the style with the given id, so an assertion cannot pass
// because some *other* style in the part happens to carry the markup. Word's
// default set (Normal, Heading1-9) is always present, and several of those
// carry w:uiPriority and w:link of their own.
func styleXML(t *testing.T, doc *Document, id string) string {
	t.Helper()
	data, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	parts, _ := docParts(t, data)
	all := parts["word/styles.xml"]
	if all == "" {
		t.Fatal("no word/styles.xml in the saved package")
	}
	marker := `w:styleId="` + id + `"`
	i := strings.Index(all, marker)
	if i < 0 {
		t.Fatalf("styles.xml has no style with id %q:\n%s", id, all)
	}
	// Walk back to the opening <w:style and forward to its </w:style>.
	start := strings.LastIndex(all[:i], "<w:style ")
	if start < 0 {
		t.Fatalf("malformed styles.xml around %q", id)
	}
	end := strings.Index(all[start:], "</w:style>")
	if end < 0 {
		t.Fatalf("style %q is not closed in styles.xml", id)
	}
	return all[start : start+end]
}

// styleSetterCase is one Style builder setter and the markup it must produce
// inside its own w:style element.
type styleSetterCase struct {
	name string
	// apply configures the style; it must be the only mutation the case makes.
	apply func(s *Style)
	// wantXML must all appear inside the style's own element.
	wantXML []string
	// notXML must not appear inside it (used for the mutually-exclusive
	// indent pair).
	notXML []string
}

func styleSetterCases() []styleSetterCase {
	return []styleSetterCase{
		{
			name:    "SetName",
			apply:   func(s *Style) { s.SetName("Renamed Emphasis") },
			wantXML: []string{`<w:name w:val="Renamed Emphasis"/>`},
		},
		{
			name:    "SetType",
			apply:   func(s *Style) { s.SetType(StyleTypeTable) },
			wantXML: []string{`w:type="table"`},
		},
		{
			name:    "SetLink",
			apply:   func(s *Style) { s.SetLink("BodyTextChar") },
			wantXML: []string{`<w:link w:val="BodyTextChar"/>`},
		},
		{
			name:    "SetUIPriority",
			apply:   func(s *Style) { s.SetUIPriority(37) },
			wantXML: []string{`<w:uiPriority w:val="37"/>`},
		},
		{
			name:    "SetAlignment/center",
			apply:   func(s *Style) { s.SetAlignment(AlignmentCenter) },
			wantXML: []string{`<w:jc w:val="center"/>`},
		},
		{
			name:  "SetAlignment/justify",
			apply: func(s *Style) { s.SetAlignment(AlignmentJustify) },
			// WordprocessingML spells "justified" as "both", not "justify".
			wantXML: []string{`<w:jc w:val="both"/>`},
			notXML:  []string{`w:val="justify"`},
		},
		{
			name:  "SetLineSpacing",
			apply: func(s *Style) { s.SetLineSpacing(1.5) },
			// 240ths of a line under lineRule="auto": 1.5 * 240 = 360.
			wantXML: []string{`w:line="360"`, `w:lineRule="auto"`},
		},
		{
			name:  "SetIndentFirstLine",
			apply: func(s *Style) { s.SetIndentFirstLine(18) },
			// 18pt = 360 twips.
			wantXML: []string{`w:firstLine="360"`},
			notXML:  []string{`w:hanging=`},
		},
		{
			name:    "SetIndentHanging",
			apply:   func(s *Style) { s.SetIndentHanging(27) },
			wantXML: []string{`w:hanging="540"`},
			notXML:  []string{`w:firstLine=`},
		},
	}
}

// TestStyleSetters_WriteTheirMarkup drives each Style setter on a style of its
// own and asserts on that style's element in the written part. Asserting on
// the part rather than the builder is what distinguishes a setter that reached
// styles.xml from one that only reached the struct — styles.xml is written only
// when the session flagged it, so a setter that forgot to flag would produce a
// document with none of these edits in it.
func TestStyleSetters_WriteTheirMarkup(t *testing.T) {
	for _, c := range styleSetterCases() {
		t.Run(c.name, func(t *testing.T) {
			doc := Create()
			// A character style, so AddCharacterStyle is exercised on every row
			// and the default w:type is not the one the case sets.
			st := doc.Styles().AddCharacterStyle("SubjectStyle", "Subject Style")
			if st == nil {
				t.Fatal("AddCharacterStyle returned nil")
			}
			c.apply(st)

			frag := styleXML(t, doc, "SubjectStyle")
			for _, want := range c.wantXML {
				if !strings.Contains(frag, want) {
					t.Errorf("style element lacks %s:\n%s", want, frag)
				}
			}
			for _, not := range c.notXML {
				if strings.Contains(frag, not) {
					t.Errorf("style element unexpectedly contains %s:\n%s", not, frag)
				}
			}
		})
	}
}

// TestAddCharacterStyle pins the type it creates and its idempotence contract.
// AddCharacterStyle delegating to the paragraph variant would leave every
// character style in the document as a paragraph style, which Word applies to
// the whole paragraph rather than the selected run.
func TestAddCharacterStyle(t *testing.T) {
	doc := Create()
	m := doc.Styles()

	cs := m.AddCharacterStyle("Emph2", "Emphasis Two")
	if got := cs.Type(); got != StyleTypeCharacter {
		t.Errorf("AddCharacterStyle produced a %q style, want %q", got, StyleTypeCharacter)
	}
	if got := cs.Name(); got != "Emphasis Two" {
		t.Errorf("Name() = %q, want %q", got, "Emphasis Two")
	}

	// A paragraph style and a character style must be distinguishable in the
	// written part.
	ps := m.AddParagraphStyle("Para2", "Paragraph Two")
	if got := ps.Type(); got != StyleTypeParagraph {
		t.Errorf("AddParagraphStyle produced a %q style, want %q", got, StyleTypeParagraph)
	}
	if !strings.Contains(styleXML(t, doc, "Emph2"), `w:type="character"`) {
		t.Error(`Emph2 is not written with w:type="character"`)
	}
	if !strings.Contains(styleXML(t, doc, "Para2"), `w:type="paragraph"`) {
		t.Error(`Para2 is not written with w:type="paragraph"`)
	}

	// Idempotent on the id: the existing definition comes back untouched.
	again := m.AddCharacterStyle("Emph2", "A Different Name")
	if got := again.Name(); got != "Emphasis Two" {
		t.Errorf("re-adding an existing id renamed it to %q; AddStyle documents that it returns the existing style as-is", got)
	}
	if got := again.Type(); got != StyleTypeCharacter {
		t.Errorf("re-adding an existing id changed its type to %q", got)
	}
	// And an id already taken by a paragraph style hands back that paragraph
	// style rather than converting it.
	clash := m.AddCharacterStyle("Para2", "Ignored")
	if got := clash.Type(); got != StyleTypeParagraph {
		t.Errorf("AddCharacterStyle on an id owned by a paragraph style returned a %q style; it must not convert the definition", got)
	}
}

// TestStyleSetters_SurviveReopen proves the values reached styles.xml by
// reading them back out of a reopened package through the same API.
func TestStyleSetters_SurviveReopen(t *testing.T) {
	doc := Create()
	doc.Styles().
		AddCharacterStyle("RoundTrip", "Round Trip").
		SetName("Round Trip Renamed").
		SetType(StyleTypeParagraph).
		SetLink("RoundTripChar").
		SetUIPriority(11).
		SetAlignment(AlignmentRight).
		SetLineSpacing(2).
		SetIndentHanging(9)

	reopened := saveAndReopen(t, doc)
	st := reopened.Styles().Style("RoundTrip")
	if st == nil {
		t.Fatal("the style did not survive the save/reopen")
	}
	if got := st.Name(); got != "Round Trip Renamed" {
		t.Errorf("reopened Name() = %q, want %q", got, "Round Trip Renamed")
	}
	if got := st.Type(); got != StyleTypeParagraph {
		t.Errorf("reopened Type() = %q, want %q", got, StyleTypeParagraph)
	}

	frag := styleXML(t, reopened, "RoundTrip")
	for _, want := range []string{
		`<w:link w:val="RoundTripChar"/>`,
		`<w:uiPriority w:val="11"/>`,
		`<w:jc w:val="right"/>`,
		`w:line="480"`,
		`w:lineRule="auto"`,
		`w:hanging="180"`,
	} {
		if !strings.Contains(frag, want) {
			t.Errorf("reopened style element lacks %s:\n%s", want, frag)
		}
	}
}

// TestStyleSetters_PersistThroughAnOpenedDocument edits a style in a document
// that was opened from disk, not created in memory. That is the case where
// styles.xml is written back as preserved bytes unless the session flagged the
// part, so a setter that mutates the model without raising the flag succeeds in
// memory and is discarded at save. TestMutationsFlagTheirPart proves the flag
// call is present in the source; this proves it works.
// Each setter is applied to a style of its own, one per subtest: chaining them
// on a single style would let one setter's flag carry another setter that
// forgot to raise it, which is precisely the omission being tested for.
func TestStyleSetters_PersistThroughAnOpenedDocument(t *testing.T) {
	for _, c := range styleSetterCases() {
		t.Run(c.name, func(t *testing.T) {
			seed := Create()
			seed.Styles().AddCharacterStyle("Persisted", "Persisted")
			opened := saveAndReopen(t, seed)

			st := opened.Styles().Style("Persisted")
			if st == nil {
				t.Fatal("the seeded style is missing from the reopened document")
			}
			c.apply(st)

			frag := styleXML(t, saveAndReopen(t, opened), "Persisted")
			for _, want := range c.wantXML {
				if !strings.Contains(frag, want) {
					t.Errorf("an edit made through an opened document was dropped at save: styles.xml lacks %s\n%s", want, frag)
				}
			}
		})
	}
}

// TestStyleIndentsAreMutuallyExclusive: w:ind cannot carry both a first-line
// and a hanging indent, so each setter has to clear the other. A setter that
// only wrote its own attribute would leave the pair fighting, and Word
// resolves that by ignoring one of them.
func TestStyleIndentsAreMutuallyExclusive(t *testing.T) {
	t.Run("hanging clears firstLine", func(t *testing.T) {
		doc := Create()
		doc.Styles().AddParagraphStyle("Ind1", "Ind One").SetIndentFirstLine(18).SetIndentHanging(27)
		frag := styleXML(t, doc, "Ind1")
		if !strings.Contains(frag, `w:hanging="540"`) {
			t.Errorf("hanging indent missing:\n%s", frag)
		}
		if strings.Contains(frag, `w:firstLine=`) {
			t.Errorf("SetIndentHanging left the first-line indent in place:\n%s", frag)
		}
	})
	t.Run("firstLine clears hanging", func(t *testing.T) {
		doc := Create()
		doc.Styles().AddParagraphStyle("Ind2", "Ind Two").SetIndentHanging(27).SetIndentFirstLine(18)
		frag := styleXML(t, doc, "Ind2")
		if !strings.Contains(frag, `w:firstLine="360"`) {
			t.Errorf("first-line indent missing:\n%s", frag)
		}
		if strings.Contains(frag, `w:hanging=`) {
			t.Errorf("SetIndentFirstLine left the hanging indent in place:\n%s", frag)
		}
	})
}

// TestLineSpacingAuto pins the 240ths-of-a-line conversion, including a
// multiplier whose product is not an integer so truncation and rounding
// disagree.
func TestLineSpacingAuto(t *testing.T) {
	cases := []struct {
		multiplier float64
		want       string
	}{
		{1, "240"},
		{1.5, "360"},
		{2, "480"},
		{0.9, "216"},
		// 1.333 * 240 = 319.92: rounds to 320, truncates to 319.
		{1.333, "320"},
	}
	for _, c := range cases {
		if got := lineSpacingAuto(c.multiplier); got != c.want {
			t.Errorf("lineSpacingAuto(%v) = %q, want %q", c.multiplier, got, c.want)
		}
	}
}

// TestStyleSetters_MatchParagraphSpelling: the Style builder and the Paragraph
// setters write the same properties, and a value spelled one way through the
// style and another way through the paragraph is a defect that neither side's
// own test would see. Both are checked against the same expected markup.
func TestStyleSetters_MatchParagraphSpelling(t *testing.T) {
	styleDoc := Create()
	styleDoc.Styles().AddParagraphStyle("Spelling", "Spelling").
		SetAlignment(AlignmentJustify).
		SetLineSpacing(1.5).
		SetIndentHanging(27)
	styleFrag := styleXML(t, styleDoc, "Spelling")

	paraDoc := Create()
	p := paraDoc.AddParagraph()
	p.SetText("x")
	p.SetAlignment(AlignmentJustify)
	p.SetLineSpacing(1.5)
	p.SetIndentHanging(27)
	data, err := paraDoc.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	parts, _ := docParts(t, data)
	paraFrag := parts["word/document.xml"]

	for _, want := range []string{`<w:jc w:val="both"/>`, `w:line="360"`, `w:lineRule="auto"`, `w:hanging="540"`} {
		if !strings.Contains(styleFrag, want) {
			t.Errorf("the style spelling lacks %s:\n%s", want, styleFrag)
		}
		if !strings.Contains(paraFrag, want) {
			t.Errorf("the paragraph spelling lacks %s — style and paragraph must agree:\n%s", want, paraFrag)
		}
	}
}
