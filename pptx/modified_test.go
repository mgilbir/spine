package pptx

import (
	"bytes"
	"testing"
	"testing/synctest"
	"time"
)

// The stamp these tests exercise is wall-clock, and dcterms:modified serializes
// at RFC3339's one-second resolution, so proving "a save stamped" needs a save
// that lands in a different second from the value it is compared against.
// The tests that need that run inside a synctest bubble: the bubble's clock is
// fake, starts at midnight UTC 2000-01-01, and advances only when every
// goroutine in it is durably blocked — so a time.Sleep costs no real time while
// still moving the clock by exactly the requested amount. The library spawns no
// goroutines on any save path, so the bubble's "all goroutines exit" rule is
// satisfied trivially, and the OPC writer stamps no zip metadata from the clock,
// so a fake clock cannot perturb archive bytes.
//
// Two rules follow from the fake clock, and both matter:
//
//   - Sleep in whole seconds only. A sub-second instant would not survive the
//     RFC3339 round trip, and since the library sets no timers the bubble clock
//     advances by exactly the sleeps these tests request — so whole seconds stay
//     exact and the stamped instant is knowable in advance.
//   - Assert the stamped value exactly, not that it moved forward. testdata's
//     stored dcterms:modified is 2012-12-17, which is *after* the bubble epoch,
//     so "got.After(fixture)" would fail a correctly stamping save inside a
//     bubble. Exact equality against time.Now() taken just before the save is
//     both correct there and a strictly stronger claim: it pins which instant
//     was recorded rather than merely that some later one was.
//
// Tests that never consult the clock — the ones comparing against the fixture's
// stored value or against a caller-assigned constant — are deliberately left
// outside the bubble.

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
// deck records when it was written — and records exactly the instant of the
// save, not merely something later than the value on disk.
func TestEditStampsModified(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p, before := openTestDeck(t)

		slides := p.Slides()
		if len(slides) == 0 {
			t.Fatal("testdata/test.pptx has no slides")
		}
		tb := slides[0].AddTextBox()
		tb.SetText("edited")

		// Move off the instant the deck was opened at, so a stamp cannot be
		// mistaken for "nothing happened", then pin the instant the save will
		// record.
		time.Sleep(time.Second)
		want := time.Now()

		data, err := p.SaveBytes()
		if err != nil {
			t.Fatalf("SaveBytes: %v", err)
		}

		got := reopenModified(t, data)
		if !got.Equal(want) {
			t.Errorf("edit stamped dcterms:modified %v, want the save time %v (deck was opened at %v)", got, want, before)
		}
	})
}

// TestEditedDeckSaveIsStillIdempotent pins the interaction between the two:
// the edit stamps once, and re-saving the same deck without touching it again
// must reproduce the same bytes — including the same stamp — rather than
// stamping afresh.
func TestEditedDeckSaveIsStillIdempotent(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p, _ := openTestDeck(t)
		tb := p.Slides()[0].AddTextBox()
		tb.SetText("edited")

		stamped := time.Now()
		first, err := p.SaveBytes()
		if err != nil {
			t.Fatalf("first SaveBytes: %v", err)
		}
		// Cross a second boundary: a per-save stamp cannot pass this, because a
		// second stamp would serialize to a different value rather than hiding
		// inside the same second.
		time.Sleep(time.Second)
		second, err := p.SaveBytes()
		if err != nil {
			t.Fatalf("second SaveBytes: %v", err)
		}
		if !bytes.Equal(first, second) {
			t.Error("re-saving an already-saved edit produced different bytes")
		}
		if got := reopenModified(t, second); !got.Equal(stamped) {
			t.Errorf("re-save moved dcterms:modified: %v, want the first save's %v", got, stamped)
		}
	})
}

// TestSecondEditStampsAgain: after a save has stamped, a further edit must
// stamp again. The "was this Modified set by the caller or by us" test has to
// tell those apart, since after the first save Modified differs from the value
// the deck was opened with either way.
func TestSecondEditStampsAgain(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		p, _ := openTestDeck(t)
		p.Slides()[0].AddTextBox().SetText("first")
		wantFirst := time.Now()
		first, err := p.SaveBytes()
		if err != nil {
			t.Fatalf("first SaveBytes: %v", err)
		}
		afterFirst := reopenModified(t, first)
		if !afterFirst.Equal(wantFirst) {
			t.Fatalf("first edit stamped %v, want the save time %v", afterFirst, wantFirst)
		}

		time.Sleep(time.Second)
		p.Slides()[0].AddTextBox().SetText("second")
		wantSecond := time.Now()
		secondData, err := p.SaveBytes()
		if err != nil {
			t.Fatalf("second SaveBytes: %v", err)
		}
		if got := reopenModified(t, secondData); !got.Equal(wantSecond) {
			t.Errorf("second edit stamped %v, want the second save time %v (first save stamped %v)", got, wantSecond, afterFirst)
		}
	})
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
	synctest.Test(t, func(t *testing.T) {
		p := Create()
		created := p.Properties.Modified
		// Create's own stamp has second granularity, so without this the
		// "stamped" assertion below could not tell a stamp from the constructor's
		// own value.
		time.Sleep(time.Second)

		p.AddSlide().AddTextBox().SetText("hello")

		want := time.Now()
		first, err := p.SaveBytes()
		if err != nil {
			t.Fatalf("first SaveBytes: %v", err)
		}
		stamped := reopenModified(t, first)
		if !stamped.Equal(want) {
			t.Errorf("adding content to a created deck stamped %v, want the save time %v (deck was created at %v)", stamped, want, created)
		}

		time.Sleep(time.Second)
		second, err := p.SaveBytes()
		if err != nil {
			t.Fatalf("second SaveBytes: %v", err)
		}
		if !bytes.Equal(first, second) {
			t.Error("re-saving an unchanged created deck produced different bytes")
		}
	})
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
