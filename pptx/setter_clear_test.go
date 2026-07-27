package pptx

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/common/enum"
)

// openBox returns the first text box of the first slide of a reopened deck.
func openBox(t *testing.T, deck []byte) (*Presentation, *TextBox) {
	t.Helper()
	p, err := OpenReader(bytes.NewReader(deck), int64(len(deck)))
	if err != nil {
		t.Fatal(err)
	}
	for _, sh := range p.Slides()[0].Shapes() {
		if box, ok := sh.(*TextBox); ok {
			return p, box
		}
	}
	t.Fatal("no text box in the reopened deck")
	return nil, nil
}

// C521: updateShapeNode wrote the name only when non-empty, so SetName("") was
// silently ignored on the flush path.
func TestSetName_EmptyStringClearsTheName(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	box := s.AddTextBox()
	box.TextFrame().SetText("x")
	box.SetName("Original Name")

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(zipPart(t, data, "ppt/slides/slide1.xml")), `name="Original Name"`) {
		t.Fatal("the name was not written in the first place")
	}

	p2, box2 := openBox(t, data)
	box2.SetName("")

	saved, err := p2.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	out := string(zipPart(t, saved, "ppt/slides/slide1.xml"))
	if strings.Contains(out, `name="Original Name"`) {
		t.Errorf(`SetName("") was ignored:\n%s`, out)
	}
}

// C521: a shape dirtied for an unrelated reason must keep its parsed name — the
// explicit-intent flag must not turn every edit into a name rewrite.
func TestUnrelatedEdit_KeepsParsedShapeName(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	box := s.AddTextBox()
	box.TextFrame().SetText("x")
	box.SetName("Keep Me")

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	p2, box2 := openBox(t, data)
	box2.SetPosition(dml.Inches(1), dml.Inches(1))

	saved, err := p2.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	out := string(zipPart(t, saved, "ppt/slides/slide1.xml"))
	if !strings.Contains(out, `name="Keep Me"`) {
		t.Errorf("an unrelated edit dropped the parsed shape name:\n%s", out)
	}
}

// C521: no API could remove a shape-level hyperlink; the flush only ever wrote
// one when non-nil.
func TestRemoveHyperlink_Shape(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	box := s.AddTextBox()
	box.TextFrame().SetText("x")
	box.SetHyperlink("https://example.com/")

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(zipPart(t, data, "ppt/slides/slide1.xml")), "hlinkClick") {
		t.Fatal("the hyperlink was not written in the first place")
	}

	p2, box2 := openBox(t, data)
	if box2.Hyperlink() == nil {
		t.Fatal("the hyperlink did not survive the round trip")
	}
	box2.RemoveHyperlink()

	saved, err := p2.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	out := string(zipPart(t, saved, "ppt/slides/slide1.xml"))
	if strings.Contains(out, "hlinkClick") {
		t.Errorf("RemoveHyperlink left the a:hlinkClick in place:\n%s", out)
	}
}

// C521: the same for a run-level hyperlink.
func TestRemoveHyperlink_Run(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	box := s.AddTextBox()
	tf := box.TextFrame()
	tf.SetText("link")
	tf.Paragraphs()[0].Runs()[0].SetHyperlink("https://example.com/")

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(zipPart(t, data, "ppt/slides/slide1.xml")), "hlinkClick") {
		t.Fatal("the run hyperlink was not written in the first place")
	}

	p2, box2 := openBox(t, data)
	box2.TextFrame().Paragraphs()[0].Runs()[0].RemoveHyperlink()

	saved, err := p2.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	out := string(zipPart(t, saved, "ppt/slides/slide1.xml"))
	if strings.Contains(out, "hlinkClick") {
		t.Errorf("Run.RemoveHyperlink left the a:hlinkClick in place:\n%s", out)
	}
}

// C521: applyCellProps could not clear a parsed anchor.
func TestTableCell_ClearingAnchor(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	tbl := s.AddTable(1, 1)
	tbl.Cell(0, 0).SetVerticalAlign(enum.VerticalAlignMiddle)

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(zipPart(t, data, "ppt/slides/slide1.xml")), `anchor="ctr"`) {
		t.Fatal("the anchor was not written in the first place")
	}

	p2, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	firstTable(t, p2).Cell(0, 0).SetVerticalAlign("")

	saved, err := p2.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	out := string(zipPart(t, saved, "ppt/slides/slide1.xml"))
	if strings.Contains(out, `anchor="ctr"`) {
		t.Errorf(`SetVerticalAlign("") did not clear the parsed anchor:\n%s`, out)
	}
}

// C521: SetBorderLeft(nil) marked the cell dirty but applyCellProps skipped nil
// edges, so the parsed border stayed and there was no way to remove one.
func TestTableCell_RemovingABorder(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	tbl := s.AddTable(1, 1)
	tbl.Cell(0, 0).SetBorderLeft(&TableBorder{Width: dml.Points(2), Color: dml.ColorRed, Style: BorderStyleSingle})

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(zipPart(t, data, "ppt/slides/slide1.xml")), "<a:lnL") {
		t.Fatal("the border was not written in the first place")
	}

	p2, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	firstTable(t, p2).Cell(0, 0).SetBorderLeft(nil)

	saved, err := p2.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	out := string(zipPart(t, saved, "ppt/slides/slide1.xml"))
	if strings.Contains(out, "<a:lnL") {
		t.Errorf("SetBorderLeft(nil) left the parsed border in place:\n%s", out)
	}
}

// C521: a cell dirtied for an unrelated reason must keep its parsed borders —
// borders are not in the domain model, so nil must not mean "remove".
func TestTableCell_UnrelatedEdit_KeepsParsedBorders(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	tbl := s.AddTable(1, 1)
	tbl.Cell(0, 0).SetBorderLeft(&TableBorder{Width: dml.Points(2), Color: dml.ColorRed, Style: BorderStyleSingle})

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	p2, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	firstTable(t, p2).Cell(0, 0).SetVerticalAlign(enum.VerticalAlignBottom)

	saved, err := p2.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	out := string(zipPart(t, saved, "ppt/slides/slide1.xml"))
	if !strings.Contains(out, "<a:lnL") {
		t.Errorf("an unrelated cell edit removed the parsed border:\n%s", out)
	}
}
