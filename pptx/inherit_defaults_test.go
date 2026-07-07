package pptx

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/common/enum"
)

// C162/C167: a plain API-authored paragraph must not emit explicit line
// spacing or underline/strike attributes — those would override the spacing
// and run style inherited from the placeholder/layout/master.
func TestPlainParagraphInheritsSpacingAndRunStyle(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	s.AddTextBox().TextFrame().SetText("plain")

	// A run with some other explicit property still must not pick up
	// underline/strike (the rPr is emitted for the bold flag alone).
	bold := s.AddTextBox().TextFrame().AddParagraph().AddRun()
	bold.SetText("bold")
	bold.SetBold(true)

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	slideXML := string(zipPart(t, data, "ppt/slides/slide1.xml"))

	if strings.Contains(slideXML, "lnSpc") {
		t.Errorf("plain paragraph emitted explicit line spacing:\n%s", slideXML)
	}
	if strings.Contains(slideXML, ` u="`) {
		t.Errorf("plain run emitted explicit underline:\n%s", slideXML)
	}
	if strings.Contains(slideXML, ` strike="`) {
		t.Errorf("plain run emitted explicit strike:\n%s", slideXML)
	}
}

// C162/C167: explicitly set spacing and underline/strike (including the
// "suppress inherited value" forms 100%, none, and noStrike) are still
// emitted and survive a save/reopen round trip.
func TestExplicitSpacingAndRunStyleRoundTrip(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	para := s.AddTextBox().TextFrame().AddParagraph()
	para.SetLineSpacing(100000) // explicit 100% is distinct from unset
	para.SetSpaceBefore(dml.EMU(600))
	para.SetSpaceAfter(dml.EMU(1200))
	run := para.AddRun()
	run.SetText("styled")
	run.SetUnderline(enum.UnderlineNone)
	run.SetStrike(enum.StrikeNone)

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	slideXML := string(zipPart(t, data, "ppt/slides/slide1.xml"))
	for _, want := range []string{`spcPct val="100000"`, "spcBef", "spcAft", `u="none"`, `strike="noStrike"`} {
		if !strings.Contains(slideXML, want) {
			t.Errorf("explicit setting %q not emitted:\n%s", want, slideXML)
		}
	}

	reopened, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	var box *TextBox
	for _, shape := range reopened.Slides()[0].Shapes() {
		if tb, ok := shape.(*TextBox); ok {
			box = tb
			break
		}
	}
	if box == nil {
		t.Fatal("no text box materialized from the reopened deck")
	}
	got := box.TextFrame().Paragraphs()[0]
	if got.LineSpacing() != 100000 {
		t.Errorf("LineSpacing() after reopen = %d, want 100000", got.LineSpacing())
	}
	if got.SpaceBefore() != dml.EMU(600) {
		t.Errorf("SpaceBefore() after reopen = %d, want 600", got.SpaceBefore())
	}
	if got.SpaceAfter() != dml.EMU(1200) {
		t.Errorf("SpaceAfter() after reopen = %d, want 1200", got.SpaceAfter())
	}
	gotRun := got.Runs()[0]
	if gotRun.Underline() != enum.UnderlineNone {
		t.Errorf("Underline() after reopen = %q, want %q", gotRun.Underline(), enum.UnderlineNone)
	}
	if gotRun.Strike() != enum.StrikeNone {
		t.Errorf("Strike() after reopen = %q, want %q", gotRun.Strike(), enum.StrikeNone)
	}
}

// C162: a paragraph parsed without explicit spacing reads back as unset, not
// as a fabricated 100% that a later re-sync would write into the file.
func TestParsedParagraphWithoutSpacingStaysUnset(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	s.AddTextBox().TextFrame().SetText("plain")
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}

	reopened, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	for _, shape := range reopened.Slides()[0].Shapes() {
		tb, ok := shape.(*TextBox)
		if !ok {
			continue
		}
		para := tb.TextFrame().Paragraphs()[0]
		if para.LineSpacing() != 0 {
			t.Errorf("parsed paragraph LineSpacing() = %d, want 0 (unset)", para.LineSpacing())
		}
		run := para.Runs()[0]
		if run.Underline() != "" {
			t.Errorf("parsed run Underline() = %q, want empty (unset)", run.Underline())
		}
		if run.Strike() != "" {
			t.Errorf("parsed run Strike() = %q, want empty (unset)", run.Strike())
		}
		return
	}
	t.Fatal("no text box materialized from the reopened deck")
}
