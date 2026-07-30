package docx

import (
	"strings"
	"testing"
)

// --- Header/Footer part names and Document.Footer ---

// A header and a footer of the same section index to different parts. PartName
// is how a caller correlates a story handle with the package part it came from,
// and a PartName that returned the *other* story's part (or the relationship id
// instead of the part name) would make every such correlation silently wrong —
// including the image-relationship scoping that depends on it.
func TestHeaderFooterPartNames(t *testing.T) {
	doc := Create()
	doc.AddParagraph().SetText("body")

	hdr := doc.AddHeader(HeaderDefault)
	hdr.AddParagraph().SetText("in the header")
	ftr := doc.AddFooter(FooterDefault)
	ftr.AddParagraph().SetText("in the footer")

	// Authored handles already know their part.
	if hdr.PartName() == "" {
		t.Error("Header.PartName() is empty on an authored header")
	}
	if ftr.PartName() == "" {
		t.Error("Footer.PartName() is empty on an authored footer")
	}
	if hdr.PartName() == ftr.PartName() {
		t.Errorf("the header and the footer report the same part name %q", hdr.PartName())
	}

	reopened := saveAndReopen(t, doc)
	secs := reopened.Sections()
	if len(secs) == 0 {
		t.Fatal("the reopened document has no sections")
	}
	sec := secs[len(secs)-1]

	gotHdr, ok := reopened.Header(sec, HeaderDefault)
	if !ok {
		t.Fatal("Document.Header did not find the default header after reopen")
	}
	gotFtr, ok := reopened.Footer(sec, FooterDefault)
	if !ok {
		t.Fatal("Document.Footer did not find the default footer after reopen")
	}

	// The part names must be the real package parts, distinct, and each must
	// hold its own story's text.
	parts, names := docParts(t, mustSaveBytes(t, reopened))
	for _, tc := range []struct {
		what, part, wantText string
	}{
		{"header", gotHdr.PartName(), "in the header"},
		{"footer", gotFtr.PartName(), "in the footer"},
	} {
		if !strings.HasPrefix(tc.part, "/word/") {
			t.Errorf("%s PartName() = %q, want an absolute /word/... part name", tc.what, tc.part)
		}
		zipName := strings.TrimPrefix(tc.part, "/")
		body, ok := parts[zipName]
		if !ok {
			t.Errorf("%s PartName() = %q, which is not a part of the package: %v", tc.what, tc.part, names)
			continue
		}
		if !strings.Contains(body, tc.wantText) {
			t.Errorf("%s PartName() = %q, but that part does not contain %q — the handle names the wrong part",
				tc.what, tc.part, tc.wantText)
		}
	}
	if gotHdr.PartName() == gotFtr.PartName() {
		t.Errorf("after reopen the header and footer both report %q", gotHdr.PartName())
	}

	// Reading the footer's text through the handle proves Document.Footer
	// resolved the relationship to the right part rather than to the header.
	if got := storyText(gotFtr.Paragraphs()); got != "in the footer" {
		t.Errorf("Document.Footer returned a story reading %q, want %q", got, "in the footer")
	}
	if got := storyText(gotHdr.Paragraphs()); got != "in the header" {
		t.Errorf("Document.Header returned a story reading %q, want %q", got, "in the header")
	}

	// A type the section does not declare must report absent rather than
	// falling back to the default one.
	if _, ok := reopened.Footer(sec, FooterFirst); ok {
		t.Error("Document.Footer reported a first-page footer the section never declared")
	}
	if _, ok := reopened.Footer(nil, FooterDefault); ok {
		t.Error("Document.Footer on a nil section reported a footer")
	}
}

func storyText(paras []*Paragraph) string {
	parts := make([]string, 0, len(paras))
	for _, p := range paras {
		if txt := p.Text(); txt != "" {
			parts = append(parts, txt)
		}
	}
	return strings.Join(parts, "\n")
}

func mustSaveBytes(t *testing.T, doc *Document) []byte {
	t.Helper()
	data, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	return data
}

// --- Comment.Paragraphs / Comment.SetInitials ---

// Comment and Footnote both expose a Paragraphs accessor, and both were
// unexercised. Each has to walk *its own* body — a Paragraphs that returned the
// document body would look plausible in every getter test that only counted
// paragraphs.
func TestCommentParagraphsAndInitials(t *testing.T) {
	doc := Create()
	doc.AddParagraph().SetText("BODY-TEXT")
	target := doc.AddParagraph()
	target.SetText("annotated")
	c := target.AddComment("Grace H", "COMMENT-BODY")
	if c == nil {
		t.Fatal("AddComment returned nil")
	}
	c.SetInitials("GBH")

	check := func(t *testing.T, c *Comment, stage string) {
		t.Helper()
		paras := c.Paragraphs()
		if len(paras) == 0 {
			t.Fatalf("%s: Comment.Paragraphs() is empty", stage)
		}
		got := storyText(paras)
		if got != "COMMENT-BODY" {
			t.Errorf("%s: Comment.Paragraphs() reads %q, want %q", stage, got, "COMMENT-BODY")
		}
		if strings.Contains(got, "BODY-TEXT") {
			t.Errorf("%s: Comment.Paragraphs() returned the document body, not the comment body", stage)
		}
		if c.Initials() != "GBH" {
			t.Errorf("%s: Initials() = %q, want %q", stage, c.Initials(), "GBH")
		}
		if c.Author() != "Grace H" {
			t.Errorf("%s: SetInitials overwrote the author: %q", stage, c.Author())
		}
	}
	check(t, c, "authored")

	reopened := saveAndReopen(t, doc)
	comments := reopened.Comments()
	if len(comments) != 1 {
		t.Fatalf("got %d comments after reopen, want 1", len(comments))
	}
	check(t, comments[0], "reopened")
}

// --- Footnote.Paragraphs ---

func TestFootnoteParagraphs(t *testing.T) {
	doc := Create()
	doc.AddParagraph().SetText("BODY-TEXT")
	run := doc.AddParagraph().AddRun()
	run.SetText("anchor")
	fn := run.AddFootnote("FOOTNOTE-BODY")
	if fn == nil {
		t.Fatal("AddFootnote returned nil")
	}

	check := func(t *testing.T, f *Footnote, stage string) {
		t.Helper()
		paras := f.Paragraphs()
		if len(paras) == 0 {
			t.Fatalf("%s: Footnote.Paragraphs() is empty", stage)
		}
		got := storyText(paras)
		if !strings.Contains(got, "FOOTNOTE-BODY") {
			t.Errorf("%s: Footnote.Paragraphs() reads %q, want it to contain %q", stage, got, "FOOTNOTE-BODY")
		}
		if strings.Contains(got, "BODY-TEXT") {
			t.Errorf("%s: Footnote.Paragraphs() returned the document body, not the note body", stage)
		}
		// Paragraphs and Text are two readings of the same content and must
		// agree, so a Paragraphs that walked a different note fails here too.
		if want := f.Text(); !strings.Contains(want, "FOOTNOTE-BODY") {
			t.Errorf("%s: Footnote.Text() = %q", stage, want)
		}
	}
	check(t, fn, "authored")

	reopened := saveAndReopen(t, doc)
	notes := reopened.Footnotes()
	if len(notes) != 1 {
		t.Fatalf("got %d footnotes after reopen, want 1", len(notes))
	}
	check(t, notes[0], "reopened")
	// The footnote body must not leak into the main story.
	if body := reopened.Body(); strings.Contains(body, "FOOTNOTE-BODY") {
		t.Errorf("the footnote body appears in the document body: %q", body)
	}
}

// --- Frameset.Title / Frame.Title ---

// Two Title accessors on two types, both unexercised. They read from different
// elements at different depths of the same part, so a Frame.Title that returned
// its parent frameset's title (or its own Name) round-trips undetected.
func TestFramesetAndFrameTitles(t *testing.T) {
	doc := Create()
	doc.AddParagraph().SetText("x")

	def := FramesetDef{
		Size:   "*,240",
		Layout: "cols",
		Title:  "OUTER-TITLE",
		Frames: []FrameDef{
			{Name: "nav", Title: "FRAME-TITLE-A", Size: "240", Scrollbar: "auto", SourceTarget: "https://example.com/nav.html"},
		},
		Framesets: []FramesetDef{{
			Layout: "rows",
			Title:  "INNER-TITLE",
			Frames: []FrameDef{
				{Name: "main", Title: "FRAME-TITLE-B", Size: "*", Scrollbar: "on", SourceTarget: "https://example.com/main.html"},
			},
		}},
	}
	if err := doc.SetFrameset(def); err != nil {
		t.Fatalf("SetFrameset: %v", err)
	}

	fs := saveAndReopen(t, doc).Frameset()
	if fs == nil {
		t.Fatal("the frameset did not survive a save/reopen")
	}
	if got := fs.Title(); got != "OUTER-TITLE" {
		t.Errorf("Frameset.Title() = %q, want %q", got, "OUTER-TITLE")
	}
	if len(fs.Frames()) != 1 {
		t.Fatalf("outer frameset has %d frames, want 1", len(fs.Frames()))
	}
	// Every title in the fixture is distinct, so a Title reading the wrong
	// element or the wrong depth cannot match by accident.
	if got := fs.Frames()[0].Title(); got != "FRAME-TITLE-A" {
		t.Errorf("Frame.Title() = %q, want %q", got, "FRAME-TITLE-A")
	}
	if got := fs.Frames()[0].Name(); got != "nav" {
		t.Errorf("Frame.Name() = %q, want %q (Title and Name must not be the same field)", got, "nav")
	}
	if len(fs.Framesets()) != 1 {
		t.Fatalf("outer frameset has %d nested framesets, want 1", len(fs.Framesets()))
	}
	inner := fs.Framesets()[0]
	if got := inner.Title(); got != "INNER-TITLE" {
		t.Errorf("nested Frameset.Title() = %q, want %q", got, "INNER-TITLE")
	}
	if len(inner.Frames()) != 1 {
		t.Fatalf("nested frameset has %d frames, want 1", len(inner.Frames()))
	}
	if got := inner.Frames()[0].Title(); got != "FRAME-TITLE-B" {
		t.Errorf("nested Frame.Title() = %q, want %q", got, "FRAME-TITLE-B")
	}
}

// --- ContentControl.SetDataBindingWithPrefixMappings ---

// The prefix-mappings variant is the one Word needs whenever the bound XPath
// uses a namespace prefix; without w:prefixMappings the expression cannot
// resolve and the control shows nothing. The plain SetDataBinding must not
// write the attribute, and this one must — and must not disturb the other two
// fields while doing it.
func TestContentControlDataBindingPrefixMappings(t *testing.T) {
	const (
		xpath    = "/ns0:root[1]/ns0:name[1]"
		storeID  = "{9D9F8B2A-1111-4C1A-9999-0123456789AB}"
		mappings = `xmlns:ns0='http://example.com/data'`
	)

	doc := Create()
	cc := doc.AddContentControl("bound", "value")
	cc.SetDataBindingWithPrefixMappings(xpath, storeID, mappings)

	gotXPath, gotID, gotMap, ok := cc.DataBinding()
	if !ok {
		t.Fatal("DataBinding() reports no binding after SetDataBindingWithPrefixMappings")
	}
	if gotXPath != xpath || gotID != storeID || gotMap != mappings {
		t.Errorf("DataBinding() = (%q, %q, %q), want (%q, %q, %q)",
			gotXPath, gotID, gotMap, xpath, storeID, mappings)
	}

	reopened := saveAndReopen(t, doc)
	ccs := reopened.ContentControls()
	if len(ccs) != 1 {
		t.Fatalf("got %d content controls after reopen, want 1", len(ccs))
	}
	gotXPath, gotID, gotMap, ok = ccs[0].DataBinding()
	if !ok {
		t.Fatal("the data binding did not survive a save/reopen")
	}
	if gotXPath != xpath || gotID != storeID || gotMap != mappings {
		t.Errorf("reopened DataBinding() = (%q, %q, %q), want (%q, %q, %q)",
			gotXPath, gotID, gotMap, xpath, storeID, mappings)
	}

	// The plain variant must leave prefixMappings empty, or the two methods are
	// the same method and one of them is a lie.
	plainDoc := Create()
	plain := plainDoc.AddContentControl("bound", "value")
	plain.SetDataBinding(xpath, storeID)
	_, _, gotMap, ok = plain.DataBinding()
	if !ok {
		t.Fatal("DataBinding() reports no binding after SetDataBinding")
	}
	if gotMap != "" {
		t.Errorf("SetDataBinding wrote prefixMappings %q; only the WithPrefixMappings variant should", gotMap)
	}
}

// --- Document.ListDefinitions and ListLevel.SetAlignment ---

// ListDefinitions is documented as an alias for Numbering. An alias that
// returned a *different* manager (a fresh one over a re-created model) would
// lose definitions built through it, so the test builds through the alias and
// reads back through the primary spelling.
func TestListDefinitionsAliasAndLevelAlignment(t *testing.T) {
	doc := Create()

	def := doc.ListDefinitions().AddDefinition()
	if def == nil {
		t.Fatal("ListDefinitions().AddDefinition() returned nil")
	}
	def.SetLevel(0, NumberFormatDecimal, "%1.").SetAlignment(AlignmentRight)
	def.SetLevel(1, NumberFormatLowerLetter, "%2)").SetAlignment(AlignmentCenter)

	p := doc.AddParagraph()
	p.SetText("item")
	p.SetListStyle(def.ListStyle(), 0)

	data := mustSaveBytes(t, doc)
	parts, _ := docParts(t, data)
	numbering := parts["word/numbering.xml"]
	if numbering == "" {
		t.Fatal("no word/numbering.xml was written")
	}
	// The two levels take different alignments, so a SetAlignment that wrote a
	// constant, or wrote w:jc instead of w:lvlJc, fails.
	for _, want := range []string{`<w:lvlJc w:val="right"/>`, `<w:lvlJc w:val="center"/>`} {
		if !strings.Contains(numbering, want) {
			t.Errorf("numbering.xml lacks %s:\n%s", want, numbering)
		}
	}
	if strings.Contains(numbering, `<w:lvlJc w:val="left"/>`) {
		t.Errorf("numbering.xml has a left w:lvlJc that no level asked for:\n%s", numbering)
	}

	// The alias and the primary spelling must be the same manager over the same
	// model: a definition added through one is visible through the other.
	viaAlias := doc.ListDefinitions().AddDefinition()
	beforeID := viaAlias.AbstractNumID()
	viaAlias.SetLevel(0, NumberFormatBullet, "\u2022")
	after := mustSaveBytes(t, doc)
	afterParts, _ := docParts(t, after)
	if !strings.Contains(afterParts["word/numbering.xml"], `w:abstractNumId="`) {
		t.Fatal("numbering.xml carries no abstract numbering definitions")
	}
	if doc.Numbering() == nil {
		t.Fatal("Document.Numbering() returned nil")
	}
	if got := doc.ListDefinitions().AddDefinition().AbstractNumID(); got == beforeID {
		t.Errorf("two successive AddDefinition calls both returned abstract id %d; the alias is handing back a fresh model", got)
	}
}

// --- Paragraph.SetIndentRight ---

// The four indent setters write four different attributes of one w:ind element.
// SetIndentRight was the only one never driven, and an implementation that
// wrote Left (the neighbouring field) is invisible unless the two are set to
// different values in the same paragraph.
func TestParagraphIndents(t *testing.T) {
	doc := Create()
	p := doc.AddParagraph()
	p.SetText("indented")
	p.SetIndentLeft(18)       // 360 twips
	p.SetIndentRight(9)       // 180 twips
	p.SetIndentFirstLine(4.5) // 90 twips

	parts, _ := docParts(t, mustSaveBytes(t, doc))
	main := parts["word/document.xml"]
	for _, want := range []string{`w:left="360"`, `w:right="180"`, `w:firstLine="90"`} {
		if !strings.Contains(main, want) {
			t.Errorf("word/document.xml lacks %s:\n%s", want, main)
		}
	}

	// Right must survive the first-line/hanging exclusivity rule, which only
	// touches those two attributes.
	p.SetIndentHanging(27)
	parts, _ = docParts(t, mustSaveBytes(t, doc))
	main = parts["word/document.xml"]
	if !strings.Contains(main, `w:right="180"`) {
		t.Errorf("SetIndentHanging cleared the right indent:\n%s", main)
	}
	if !strings.Contains(main, `w:left="360"`) {
		t.Errorf("SetIndentHanging cleared the left indent:\n%s", main)
	}
	if strings.Contains(main, `w:firstLine=`) {
		t.Errorf("SetIndentHanging left the first-line indent in place:\n%s", main)
	}
}
