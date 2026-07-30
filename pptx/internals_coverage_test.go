package pptx

import (
	"testing"

	"github.com/mgilbir/spine/common/dml"
	coxml "github.com/mgilbir/spine/common/oxml"
	"github.com/mgilbir/spine/opc"
	"github.com/mgilbir/spine/pptx/internal/oxml"
)

// TestShortRelType covers the diagnostic helper that trims a relationship type
// URI. The bug classes are taking the FIRST segment instead of the last, and
// returning "" for a URI that ends in a separator (which would produce an
// unreadable validation message).
func TestShortRelType(t *testing.T) {
	cases := []struct{ in, want string }{
		{"http://schemas.openxmlformats.org/officeDocument/2006/relationships/slideLayout", "slideLayout"},
		{"http://schemas.openxmlformats.org/officeDocument/2006/relationships/image", "image"},
		{"slideLayout", "slideLayout"},
		{"", ""},
		// A trailing separator leaves nothing to trim to, so the whole string
		// is kept rather than reported as "".
		{"http://example.com/rel/", "http://example.com/rel/"},
		{"/", "/"},
		{"a/b", "b"},
	}
	for _, tc := range cases {
		if got := shortRelType(tc.in); got != tc.want {
			t.Errorf("shortRelType(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// TestNextNotesShapeID covers the notes-slide id allocator. Handing back an id
// already in use produces duplicate p:cNvPr ids, which is a corrupt part; the
// cases below cover an unsorted id list (so "last wins" fails), an id carried
// by the shape tree's own group properties (so ignoring it fails), and the
// empty tree.
func TestNextNotesShapeID(t *testing.T) {
	shapeWithID := func(id uint32) *oxml.Shape {
		return &oxml.Shape{NvSpPr: &oxml.NvSpPr{CNvPr: &dml.CNvPr{Id: id}}}
	}

	cases := []struct {
		name string
		ns   *oxml.NotesSlide
		want uint32
	}{
		{"no cSld", &oxml.NotesSlide{}, 2},
		{"no shape tree", &oxml.NotesSlide{CSld: &oxml.CommonSlideData{}}, 2},
		{"empty tree", &oxml.NotesSlide{CSld: &oxml.CommonSlideData{SpTree: newShapeTree()}}, 2},
	}
	for _, tc := range cases {
		if got := nextNotesShapeID(tc.ns); got != tc.want {
			t.Errorf("%s: nextNotesShapeID = %d, want %d", tc.name, got, tc.want)
		}
	}

	// Descending ids: the maximum must win, not the last one seen.
	tree := newShapeTree()
	tree.Sp = []*oxml.Shape{shapeWithID(9), shapeWithID(2), shapeWithID(4)}
	ns := &oxml.NotesSlide{CSld: &oxml.CommonSlideData{SpTree: tree}}
	if got := nextNotesShapeID(ns); got != 10 {
		t.Errorf("nextNotesShapeID over ids 9,2,4 = %d, want 10", got)
	}

	// The shape tree's own p:nvGrpSpPr id counts too: it shares the id space.
	if gp := tree.NvGrpSpPr; gp != nil && gp.CNvPr != nil {
		gp.CNvPr.Id = 42
		if got := nextNotesShapeID(ns); got != 43 {
			t.Errorf("nextNotesShapeID with a group id of 42 = %d, want 43", got)
		}
		gp.CNvPr.Id = 0
	} else {
		t.Fatal("the shape tree has no nvGrpSpPr to exercise")
	}
}

// TestSetNotesBodyTextAddsPlaceholder covers newNotesBodyShape through the
// caller that needs it: a notes slide that has a shape tree but no body
// placeholder. The added placeholder must be a body placeholder, must carry the
// text, and must not reuse an id already present.
func TestSetNotesBodyTextAddsPlaceholder(t *testing.T) {
	tree := newShapeTree()
	// An existing, non-body shape occupying id 5.
	tree.Sp = []*oxml.Shape{{
		NvSpPr: &oxml.NvSpPr{
			CNvPr: &dml.CNvPr{Id: 5, Name: "Slide Image Placeholder"},
			NvPr:  &oxml.NvPr{Ph: &oxml.Placeholder{Type: "sldImg"}},
		},
	}}
	ns := &oxml.NotesSlide{CSld: &oxml.CommonSlideData{SpTree: tree}}

	if notesBodyPlaceholder(ns) != nil {
		t.Fatal("the fixture already has a body placeholder")
	}
	setNotesBodyText(ns, "speaker notes")

	body := notesBodyPlaceholder(ns)
	if body == nil {
		t.Fatal("setNotesBodyText did not add a body placeholder")
	}
	if body.NvSpPr == nil || body.NvSpPr.NvPr == nil || body.NvSpPr.NvPr.Ph == nil {
		t.Fatal("the added shape carries no p:ph")
	}
	if got := body.NvSpPr.NvPr.Ph.Type; got != "body" {
		t.Errorf("added placeholder type = %q, want body", got)
	}
	if body.TxBody == nil {
		t.Fatal("the added placeholder has no text body")
	}
	if got := notesBodyText(body.TxBody); got != "speaker notes" {
		t.Errorf("notes text = %q, want %q", got, "speaker notes")
	}

	// The id must not collide with the shape that was already there.
	ids := map[uint32]int{}
	for _, sp := range tree.Sp {
		if sp != nil && sp.NvSpPr != nil && sp.NvSpPr.CNvPr != nil {
			ids[sp.NvSpPr.CNvPr.Id]++
		}
	}
	for id, n := range ids {
		if n > 1 {
			t.Errorf("shape id %d is used %d times after setNotesBodyText", id, n)
		}
	}
	if body.NvSpPr.CNvPr.Id == 5 {
		t.Error("the added placeholder reused the existing shape's id")
	}

	// Called again, it must replace the text rather than add a second body.
	setNotesBodyText(ns, "revised notes")
	bodies := 0
	for _, sp := range tree.Sp {
		if sp != nil && sp.NvSpPr != nil && sp.NvSpPr.NvPr != nil &&
			sp.NvSpPr.NvPr.Ph != nil && sp.NvSpPr.NvPr.Ph.Type == "body" {
			bodies++
		}
	}
	if bodies != 1 {
		t.Errorf("notes slide has %d body placeholders after a second set, want 1", bodies)
	}
	if got := notesBodyText(notesBodyPlaceholder(ns).TxBody); got != "revised notes" {
		t.Errorf("notes text = %q, want %q", got, "revised notes")
	}
}

// TestRunHighlightRoundTrip drives oxmlColorChoiceToColor through the public
// path: a highlight set on a run, saved, and read back. Two runs carry
// different highlights so a reader that hoisted the first colour onto every run
// fails, and a third carries none so a reader that fabricated a zero colour
// fails.
func TestRunHighlightRoundTrip(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	tb := NewTextBox()
	tb.SetSize(dml.Inches(4), dml.Inches(1))
	tf := tb.TextFrame()
	para := tf.AddParagraph()
	r1 := para.AddRun()
	r1.SetText("yellow")
	r1.SetHighlight(dml.NewRGB(0xFF, 0xFF, 0x00).ToColor())
	r2 := para.AddRun()
	r2.SetText("cyan")
	r2.SetHighlight(dml.NewRGB(0x00, 0xFF, 0xFF).ToColor())
	para.AddRun().SetText("plain")
	if err := s.AddShape(tb); err != nil {
		t.Fatalf("AddShape: %v", err)
	}

	rp := saveReopen(t, p)
	rs, err := rp.Slide(0)
	if err != nil {
		t.Fatalf("Slide(0): %v", err)
	}
	var runs []*Run
	for _, sh := range rs.Shapes() {
		if got, ok := sh.(*TextBox); ok {
			for _, pp := range got.TextFrame().Paragraphs() {
				runs = append(runs, pp.Runs()...)
			}
		}
	}
	if len(runs) != 3 {
		t.Fatalf("reopened text box has %d runs, want 3", len(runs))
	}

	h1 := runs[0].Highlight()
	if h1 == nil {
		t.Fatal("run 1 lost its highlight")
	}
	if h1.RGB != dml.NewRGB(0xFF, 0xFF, 0x00) {
		t.Errorf("run 1 highlight = %+v, want FFFF00", h1.RGB)
	}
	h2 := runs[1].Highlight()
	if h2 == nil {
		t.Fatal("run 2 lost its highlight")
	}
	if h2.RGB != dml.NewRGB(0x00, 0xFF, 0xFF) {
		t.Errorf("run 2 highlight = %+v, want 00FFFF", h2.RGB)
	}
	if h3 := runs[2].Highlight(); h3 != nil {
		t.Errorf("run 3 reported a highlight of %+v, want nil", h3)
	}
}

// TestOxmlColorChoiceToColor covers the converter's three branches directly,
// including the failure branch: an unparseable srgb value must report absence
// rather than silently becoming black.
func TestOxmlColorChoiceToColor(t *testing.T) {
	if got := oxmlColorChoiceToColor(nil); got != nil {
		t.Errorf("nil choice = %+v, want nil", got)
	}
	if got := oxmlColorChoiceToColor(&dml.ColorChoice{}); got != nil {
		t.Errorf("empty choice = %+v, want nil", got)
	}

	got := oxmlColorChoiceToColor(&dml.ColorChoice{SrgbClr: &dml.SrgbClr{Val: "112233"}})
	if got == nil {
		t.Fatal("srgb choice returned nil")
	}
	if got.RGB != dml.NewRGB(0x11, 0x22, 0x33) {
		t.Errorf("srgb choice = %+v, want 112233", got.RGB)
	}
	if got.Type != dml.ColorTypeRGB {
		t.Errorf("srgb choice colour type = %v, want ColorTypeRGB", got.Type)
	}

	// An unparseable srgb value is absence, not black.
	if got := oxmlColorChoiceToColor(&dml.ColorChoice{SrgbClr: &dml.SrgbClr{Val: "not-a-colour"}}); got != nil {
		t.Errorf("unparseable srgb = %+v, want nil", got)
	}

	// The scheme branch maps the name to its theme colour, and two different
	// names must not collapse onto one.
	a1 := oxmlColorChoiceToColor(&dml.ColorChoice{SchemeClr: &dml.SchemeClrTransform{Val: "accent1"}})
	a2 := oxmlColorChoiceToColor(&dml.ColorChoice{SchemeClr: &dml.SchemeClrTransform{Val: "accent2"}})
	if a1 == nil || a2 == nil {
		t.Fatal("scheme choice returned nil")
	}
	if a1.Type != dml.ColorTypeTheme || a2.Type != dml.ColorTypeTheme {
		t.Errorf("scheme choices reported types %v / %v, want ColorTypeTheme", a1.Type, a2.Type)
	}
	if a1.Theme != dml.ThemeColorAccent1 {
		t.Errorf("accent1 mapped to %v, want ThemeColorAccent1", a1.Theme)
	}
	if a1.Theme == a2.Theme {
		t.Errorf("accent1 and accent2 both mapped to %v", a1.Theme)
	}
}

// TestMergeImportsLegacyCommentAuthors merges a slide carrying a LEGACY
// (p:cm/idx-based) comment into a deck that has no author list of its own. The
// author list hangs off presentation.xml rather than the slide, so an import
// that carried only the comment part would leave the comment attributed to
// nobody.
func TestMergeImportsLegacyCommentAuthors(t *testing.T) {
	// Build a source deck holding a legacy authors part and a legacy comment
	// on its first slide, then round-trip it so the merge reads a real package.
	seed, err := Open("testdata/test.pptx")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	ss := firstSlide(t, seed)
	seed.otherParts[legacyAuthorsPart] = &coxml.RawPart{
		ContentType: opc.ContentTypePresentationCommentAuthors,
		Data:        []byte(legacyAuthorsXML),
	}
	seed.relationships[presentationPartName] = append(seed.relationships[presentationPartName], &opc.Relationship{
		ID:         "rId900",
		Type:       relTypeCommentAuthors,
		Target:     "commentAuthors.xml",
		TargetMode: opc.TargetModeInternal,
	})
	injectSlidePart(seed, ss, "/ppt/comments/comment1.xml",
		opc.ContentTypePresentationComments, opc.RelTypeComments, legacyCommentXML)

	src := reopen(t, seed)
	if got := len(firstSlide(t, src).Comments()); got != 1 {
		t.Fatalf("the source deck has %d comments after reopen, want 1", got)
	}

	dst := Create()
	dst.AddSlide().AddTextBox().TextFrame().SetText("Dst")
	if _, ok := dst.otherParts[legacyAuthorsPart]; ok {
		t.Fatal("the destination already has a legacy author list")
	}
	if err := dst.AppendSlidesFrom(src); err != nil {
		t.Fatalf("AppendSlidesFrom: %v", err)
	}

	rp := reopen(t, dst)
	rs, err := rp.Slide(len(rp.Slides()) - 1)
	if err != nil {
		t.Fatalf("Slide: %v", err)
	}
	comments := rs.Comments()
	if len(comments) != 1 {
		t.Fatalf("got %d comments on the merged slide, want 1", len(comments))
	}
	// "Ada Lovelace" only resolves if the author list came across: authorId 0
	// is otherwise unresolvable and the author reads as "".
	if got := comments[0].Author(); got != "Ada Lovelace" {
		t.Errorf("author = %q, want Ada Lovelace (the legacy author list was not imported)", got)
	}
	if got := comments[0].Text(); got != "Looks great" {
		t.Errorf("text = %q, want %q", got, "Looks great")
	}
}
