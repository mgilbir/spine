package pptx_test

import (
	"bytes"
	"fmt"

	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/pptx"
)

// ExampleCreate mirrors the README quick start: build a deck in memory, add a
// slide with a text box, serialize with SaveBytes, then reopen the bytes with
// OpenReader and read the text back — no files touched.
func ExampleCreate() {
	p := pptx.Create()
	p.Properties.Title = "My Presentation"

	slide := p.AddSlide()
	slide.SetName("Introduction")

	textBox := slide.AddTextBox()
	textBox.SetPosition(dml.Inches(1), dml.Inches(1))
	textBox.SetSize(dml.Inches(8), dml.Inches(1))
	textBox.SetText("Hello, World!")

	data, err := p.SaveBytes()
	if err != nil {
		panic(err)
	}

	reopened, err := pptx.OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		panic(err)
	}
	defer func() { _ = reopened.Close() }()

	first, _ := reopened.Slide(0)
	fmt.Printf("slides=%d text=%q\n", reopened.SlideCount(), first.Text())
	// Output: slides=1 text="Hello, World!"
}

// Example_openAndModify shows the open-and-modify flavor entirely in memory:
// create a one-slide deck, serialize it, reopen it via OpenReader, add another
// slide, and count the result.
func Example_openAndModify() {
	p := pptx.Create()
	p.AddSlide()

	data, err := p.SaveBytes()
	if err != nil {
		panic(err)
	}

	reopened, err := pptx.OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		panic(err)
	}
	defer func() { _ = reopened.Close() }()

	reopened.AddSlide()
	fmt.Println("slides:", reopened.SlideCount())
	// Output: slides: 2
}

// ExamplePresentation_Validate builds a clean deck and runs the structural
// validator, which every Save runs first. A freshly created deck produces no
// findings.
func ExamplePresentation_Validate() {
	p := pptx.Create()
	p.AddSlide()

	report := p.Validate()
	fmt.Printf("findings=%d hasErrors=%v\n", len(report), report.HasErrors())
	// Output: findings=0 hasErrors=false
}
