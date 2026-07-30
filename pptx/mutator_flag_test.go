package pptx

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mgilbir/spine/common/dml"
)

// The tests here pin the three mutators the flag sweep found writing state that
// the save path then ignored — the "mutation-not-flagged" class. Each one
// changes something on a deck read back from a file, saves, and asserts the
// change is in the output.

// TestRunHyperlinkTooltipEditSurvives: editing a hyperlink read back from a file
// bound markDirty to the run's dirty flag, but patchRunInPlace writes
// rpr.HlinkClick only for a run that set the hyperlink property. The run was
// flushed and the tooltip dropped on the floor.
func TestRunHyperlinkTooltipEditSurvives(t *testing.T) {
	p := Create()
	tb := p.AddSlide().AddTextBox()
	run := tb.TextFrame().AddParagraph().AddRun()
	run.SetText("link")
	run.SetHyperlink("https://example.com").SetTooltip("first")

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	if !strings.Contains(string(zipPart(t, data, "ppt/slides/slide1.xml")), `tooltip="first"`) {
		t.Fatal("the tooltip was not written in the first place")
	}

	p2, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	h := firstParsedRunHyperlink(t, p2)
	h.SetTooltip("second")

	saved, err := p2.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes 2: %v", err)
	}
	out := string(zipPart(t, saved, "ppt/slides/slide1.xml"))
	if !strings.Contains(out, `tooltip="second"`) {
		t.Errorf("SetTooltip on a hyperlink read from the file was dropped:\n%s", out)
	}
}

// firstParsedRunHyperlink returns the hyperlink of the first run that carries
// one, on the first slide.
func firstParsedRunHyperlink(t *testing.T, p *Presentation) *Hyperlink {
	t.Helper()
	for _, shape := range p.Slides()[0].Shapes() {
		tb, ok := shape.(*TextBox)
		if !ok {
			continue
		}
		for _, para := range tb.TextFrame().Paragraphs() {
			for _, run := range para.Runs() {
				if h := run.Hyperlink(); h != nil {
					return h
				}
			}
		}
	}
	t.Fatal("no run hyperlink found")
	return nil
}

// TestTableCellSetBordersNilRemovesThem: SetBorders(nil) set the four border
// fields and the dirty flag but skipped markBorder, so bordersCleared stayed
// empty and the flush — which removes an edge only when the caller explicitly
// cleared it — left every parsed a:ln* alone. SetBorderLeft(nil) on the same
// cell did remove one, so the two spellings disagreed.
func TestTableCellSetBordersNilRemovesThem(t *testing.T) {
	p := Create()
	tbl := p.AddSlide().AddTable(1, 1)
	tbl.Cell(0, 0).SetBorders(&TableBorder{Width: dml.Points(2), Color: dml.ColorRed, Style: BorderStyleSingle})

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	if !strings.Contains(string(zipPart(t, data, "ppt/slides/slide1.xml")), "<a:lnL") {
		t.Fatal("the borders were not written in the first place")
	}

	p2, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	firstTable(t, p2).Cell(0, 0).SetBorders(nil)

	saved, err := p2.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes 2: %v", err)
	}
	out := string(zipPart(t, saved, "ppt/slides/slide1.xml"))
	for _, edge := range []string{"<a:lnL", "<a:lnR", "<a:lnT", "<a:lnB"} {
		if strings.Contains(out, edge) {
			t.Errorf("SetBorders(nil) left %s in place:\n%s", edge, out)
		}
	}
}

// TestSetImagePathDoesNotClaimToSwapTheImage pins the documented contract of
// the third finding. Picture.SetImagePath writes a field no serialization path
// reads, so it cannot change the saved deck; the fix was to say so and point at
// SetImage rather than to leave a setter that looks like it swaps the picture.
// If a future change makes it actually embed the file, this test fails and the
// doc comment has to be revisited with it.
func TestSetImagePathDoesNotClaimToSwapTheImage(t *testing.T) {
	p := Create()
	pic, err := p.AddSlide().AddPictureFromBytes(minimalTransparentPNG, "image/png")
	if err != nil {
		t.Fatalf("AddPictureFromBytes: %v", err)
	}
	before, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	pic.SetImagePath("some/other/image.png")
	if got := pic.ImagePath(); got != "some/other/image.png" {
		t.Errorf("ImagePath() = %q, want the recorded label back", got)
	}
	after, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes 2: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Error("SetImagePath changed the saved deck; its doc comment says it cannot")
	}
}
