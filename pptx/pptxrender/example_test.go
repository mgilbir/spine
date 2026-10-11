package pptxrender_test

import (
	"bytes"
	"context"
	"fmt"

	"github.com/mgilbir/forme/fonts/notosans"
	"github.com/mgilbir/forme/shape"
	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/pptx"
	"github.com/mgilbir/spine/pptx/pptxrender"
	"github.com/mgilbir/spine/render"
)

// ExamplePrepareSlide renders a slide to PNG. Fonts is required: spine looks
// up no host fonts, so every family the slide names is asked of the resolver,
// which here answers with the bundled Noto Sans. Setting Warn selects best
// effort; leave it nil to fail on anything that cannot be drawn exactly.
func ExamplePrepareSlide() {
	p := pptx.CreateWidescreen()
	box := p.AddSlide().AddTextBox()
	box.SetPosition(dml.Inches(1), dml.Inches(1))
	box.SetSize(dml.Inches(8), dml.Inches(1))
	box.SetText("Quarterly review")

	noto, err := notosans.Face()
	if err != nil {
		panic(err)
	}
	ctx := context.Background()
	page, err := pptxrender.PrepareSlide(ctx, p.Slides()[0], render.Options{
		Fonts: func(context.Context, render.FontRequest) (*shape.Face, error) { return noto, nil },
		Warn:  func(err error) { fmt.Println("warning:", err) },
	})
	if err != nil {
		panic(err)
	}

	var png bytes.Buffer
	if err := page.WritePNG(ctx, &png, 96); err != nil {
		panic(err)
	}
	w, h, err := page.Size(96)
	if err != nil {
		panic(err)
	}
	fmt.Printf("%dx%d pixels, PNG: %v\n", w, h, bytes.HasPrefix(png.Bytes(), []byte("\x89PNG")))
	// Output:
	// 1280x720 pixels, PNG: true
}
