package pptx

import (
	"bytes"
	"testing"
)

// breakRoundTrip makes a part serialize to bytes that do not reparse, by
// injecting an unterminated comment into the captured prolog separator (written
// verbatim ahead of the root element). It is the cheapest deterministic way to
// exercise the "source part cannot be round-tripped" branch of the importers.
const brokenPrologSep = "<!-- unterminated"

// openedSourceDeck returns an opened one-slide source deck.
func openedSourceDeck(t *testing.T) *Presentation {
	t.Helper()
	seed := buildDeck(t, []string{"Seed"})
	sb, err := seed.SaveBytes()
	if err != nil {
		t.Fatalf("seed SaveBytes: %v", err)
	}
	return openBytes(t, sb)
}

// C513: importMaster swallowed marshal/unmarshal failures, leaving masterXML
// nil — which SlideMaster.marshal silently replaced with the default
// newMasterXML(). The merge reported success while every slide under the source
// master had quietly acquired generic default furniture.
func TestImportMasterRoundTripFailureIsReported(t *testing.T) {
	src := openedSourceDeck(t)
	if len(src.slideMasters) == 0 || src.slideMasters[0].masterXML == nil {
		t.Fatal("source deck has no parsed slide master")
	}
	src.slideMasters[0].masterXML.Prolog.Captured = true
	src.slideMasters[0].masterXML.Prolog.Sep = brokenPrologSep

	dst := CreateWidescreen()
	dst.AddSlide().AddTextBox().TextFrame().SetText("Dst")
	before := len(dst.slideMasters)

	err := dst.AppendSlidesFrom(src)
	if err == nil {
		t.Fatal("AppendSlidesFrom succeeded with a slide master that does not round-trip")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("import slide master")) {
		t.Errorf("error does not name the failing import: %v", err)
	}
	if got := len(dst.slideMasters); got > before+1 {
		t.Errorf("destination gained %d masters from a failed import", got-before)
	}
}

// C513, layout half: addImportedLayout swallowed the same failure and returned
// nil, so importSlide silently fell back to matchLayout and the slide was
// re-pointed at a destination layout with different placeholders.
func TestImportLayoutRoundTripFailureIsReported(t *testing.T) {
	src := openedSourceDeck(t)
	if len(src.slideMasters) == 0 {
		t.Fatal("source deck has no parsed slide master")
	}
	layouts := src.slideMasters[0].layouts
	if len(layouts) == 0 || layouts[0].layoutXML == nil {
		t.Fatal("source master has no parsed layouts")
	}
	layouts[0].layoutXML.Prolog.Captured = true
	layouts[0].layoutXML.Prolog.Sep = brokenPrologSep

	dst := CreateWidescreen()
	dst.AddSlide().AddTextBox().TextFrame().SetText("Dst")

	err := dst.AppendSlidesFrom(src)
	if err == nil {
		t.Fatal("AppendSlidesFrom succeeded with a slide layout that does not round-trip")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("import slide layout")) {
		t.Errorf("error does not name the failing import: %v", err)
	}
}
