package pptx

import (
	"bytes"
	"testing"
	"time"
)

// openTestDeck opens testdata/test.pptx and reports its stored
// dcterms:modified, so a test can assert against the value on disk rather than
// against one an earlier step in the same test produced.
func openTestDeck(t *testing.T) (*Presentation, time.Time) {
	t.Helper()
	p, err := Open("testdata/test.pptx")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { _ = p.Close() })
	return p, p.Properties.Modified
}

// reopenModified saves data, reopens it, and returns the stored
// dcterms:modified.
func reopenModified(t *testing.T, data []byte) time.Time {
	t.Helper()
	rt, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer func() { _ = rt.Close() }()
	return rt.Properties.Modified
}

// TestReadOnlyAccessDoesNotStampModified is the trap the stamp rule has to
// avoid. Reading a slide materializes its model, and the save path regenerates
// any materialized slide — so a stamp keyed off "will this part be regenerated"
// would bump dcterms:modified for a caller who only looked at the deck. Reading
// is not editing: the timestamp must not move, and the package must still come
// back byte-for-byte.
func TestReadOnlyAccessDoesNotStampModified(t *testing.T) {
	// Baseline: save the deck without looking at it at all.
	untouched, err := Open("testdata/test.pptx")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	before := untouched.Properties.Modified
	baseline, err := untouched.SaveBytes()
	if err != nil {
		t.Fatalf("baseline SaveBytes: %v", err)
	}
	_ = untouched.Close()

	p, _ := openTestDeck(t)

	// Touch everything a reader would: this is what sets sxModel on each slide.
	for _, slide := range p.Slides() {
		_ = slide.Name()
		_ = slide.Notes()
		_ = slide.Transition()
		_ = slide.HasBackground()
		for _, shape := range slide.Shapes() {
			_ = shape.Name()
		}
		_ = slide.Text()
	}
	for _, master := range p.SlideMasters() {
		_ = master.Name()
		_ = master.EditablePlaceholders()
		_ = master.TitleStyle().Level(0)
	}
	for _, layout := range p.SlideLayouts() {
		_ = layout.Name()
		_ = layout.EditablePlaceholders()
	}
	_ = p.CustomProperties()
	_ = p.Sections()
	_ = p.CustomShows()
	_ = p.EmbeddedFonts()

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	if got := reopenModified(t, data); !got.Equal(before) {
		t.Errorf("read-only access bumped dcterms:modified: %v -> %v", before, got)
	}
	if !bytes.Equal(baseline, data) {
		t.Error("reading the deck changed the bytes it saves")
	}
}

// TestEditStampsModified is the other half: an edit that really changes the
// deck records when it was written.
func TestEditStampsModified(t *testing.T) {
	p, before := openTestDeck(t)

	slides := p.Slides()
	if len(slides) == 0 {
		t.Fatal("testdata/test.pptx has no slides")
	}
	tb := slides[0].AddTextBox()
	tb.SetText("edited")

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	got := reopenModified(t, data)
	if !got.After(before) {
		t.Errorf("edit did not stamp dcterms:modified: was %v, still %v", before, got)
	}
}

// TestEditedDeckSaveIsStillIdempotent pins the interaction between the two:
// the edit stamps once, and re-saving the same deck without touching it again
// must reproduce the same bytes — including the same stamp — rather than
// stamping afresh.
func TestEditedDeckSaveIsStillIdempotent(t *testing.T) {
	p, _ := openTestDeck(t)
	tb := p.Slides()[0].AddTextBox()
	tb.SetText("edited")

	first, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("first SaveBytes: %v", err)
	}
	// Cross a second boundary: a per-save stamp cannot pass this.
	time.Sleep(1100 * time.Millisecond)
	second, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("second SaveBytes: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Error("re-saving an already-saved edit produced different bytes")
	}
}

// TestSecondEditStampsAgain: after a save has stamped, a further edit must
// stamp again. The "was this Modified set by the caller or by us" test has to
// tell those apart, since after the first save Modified differs from the value
// the deck was opened with either way.
func TestSecondEditStampsAgain(t *testing.T) {
	p, _ := openTestDeck(t)
	p.Slides()[0].AddTextBox().SetText("first")
	first, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("first SaveBytes: %v", err)
	}
	afterFirst := reopenModified(t, first)

	time.Sleep(1100 * time.Millisecond)
	p.Slides()[0].AddTextBox().SetText("second")
	secondData, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("second SaveBytes: %v", err)
	}
	if got := reopenModified(t, secondData); !got.After(afterFirst) {
		t.Errorf("second edit did not re-stamp: %v then %v", afterFirst, got)
	}
}

// TestExplicitModifiedIsRespected: assigning Properties.Modified is itself a
// property edit, so the save must write the caller's value rather than
// overwriting it with the save time — even when the deck was also edited.
func TestExplicitModifiedIsRespected(t *testing.T) {
	p, _ := openTestDeck(t)
	want := time.Date(2001, 2, 3, 4, 5, 6, 0, time.UTC)
	p.Properties.Modified = want
	p.Slides()[0].AddTextBox().SetText("edited")

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	if got := reopenModified(t, data); !got.Equal(want) {
		t.Errorf("explicit Properties.Modified overwritten: want %v, got %v", want, got)
	}
}

// TestCreatedDeckStampsOnContentThenHoldsStill covers the created-deck side:
// Create stamps Created/Modified, adding content and saving records the write
// time, and saving again with nothing further changed reproduces those bytes.
func TestCreatedDeckStampsOnContentThenHoldsStill(t *testing.T) {
	p := Create()
	created := p.Properties.Modified
	// Create's own stamp has second granularity, so without this the "stamped"
	// assertion below could pass or fail on where the clock happens to be.
	time.Sleep(1100 * time.Millisecond)

	p.AddSlide().AddTextBox().SetText("hello")

	first, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("first SaveBytes: %v", err)
	}
	stamped := reopenModified(t, first)
	if !stamped.After(created) {
		t.Errorf("adding content to a created deck did not stamp: %v then %v", created, stamped)
	}

	time.Sleep(1100 * time.Millisecond)
	second, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("second SaveBytes: %v", err)
	}
	if !bytes.Equal(first, second) {
		t.Error("re-saving an unchanged created deck produced different bytes")
	}
}

// TestNoCorePropsDeckGainsNoCorePart: a package that stores no
// /docProps/core.xml must not grow one just because a slide was edited. Several
// producers keep core properties elsewhere (or omit them), and the stamp is not
// a reason to change the package shape.
func TestNoCorePropsDeckGainsNoCorePart(t *testing.T) {
	p, err := Open("testdata/no-core-props.pptx")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer func() { _ = p.Close() }()
	if p.hasCorePart {
		t.Skip("fixture now carries a core part; the case under test no longer applies")
	}
	before := p.Properties.Modified

	if len(p.Slides()) > 0 {
		p.Slides()[0].AddTextBox().SetText("edited")
	} else {
		p.AddSlide()
	}
	if _, err := p.SaveBytes(); err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	if !p.Properties.Modified.Equal(before) {
		t.Errorf("stamped a deck that has no core properties part: %v -> %v", before, p.Properties.Modified)
	}
}
