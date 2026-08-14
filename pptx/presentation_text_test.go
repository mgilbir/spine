package pptx

import (
	"strings"
	"testing"
)

func TestSlideAndPresentationText(t *testing.T) {
	pres := Create()

	s1 := pres.AddSlide()
	tb := s1.AddTextBox()
	tb.SetText("Title box")

	tbl := s1.AddTable(2, 2)
	tbl.Cell(0, 0).SetText("A1")
	tbl.Cell(0, 1).SetText("B1")
	tbl.Cell(1, 0).SetText("A2")
	tbl.Cell(1, 1).SetText("B2")

	if err := s1.SetNotes("Speaker notes here"); err != nil {
		t.Fatalf("SetNotes: %v", err)
	}
	if _, err := s1.AddComment("Reviewer", "Nice slide"); err != nil {
		t.Fatalf("AddComment: %v", err)
	}

	s2 := pres.AddSlide()
	box2 := s2.AddTextBox()
	box2.SetText("Second slide")

	want1 := "Title box\nA1\tB1\nA2\tB2\nSpeaker notes here\nNice slide"
	if got := s1.Text(); got != want1 {
		t.Fatalf("slide 1 Text() =\n%q\nwant\n%q", got, want1)
	}

	if got := s2.Text(); got != "Second slide" {
		t.Fatalf("slide 2 Text() = %q, want %q", got, "Second slide")
	}

	want := want1 + "\n\n" + "Second slide"
	if got := pres.Text(); got != want {
		t.Fatalf("Presentation.Text() =\n%q\nwant\n%q", got, want)
	}

	texts := pres.SlideTexts()
	if len(texts) != 2 || texts[0] != want1 || texts[1] != "Second slide" {
		t.Fatalf("SlideTexts() = %#v", texts)
	}
}

func TestSlideTextGroupRecursion(t *testing.T) {
	pres := Create()
	s := pres.AddSlide()

	grp := NewGroupShape()
	child := NewTextBox()
	child.SetText("Inside group")
	_ = grp.AddChild(child)
	if err := s.AddShape(grp); err != nil {
		t.Fatalf("AddShape(group): %v", err)
	}

	if got := s.Text(); !strings.Contains(got, "Inside group") {
		t.Fatalf("Text() missing grouped shape text; got %q", got)
	}
}

func TestPresentationTextEmpty(t *testing.T) {
	pres := Create()
	if got := pres.Text(); got != "" {
		t.Errorf("empty presentation Text() = %q, want \"\"", got)
	}
}
