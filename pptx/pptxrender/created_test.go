package pptxrender

import (
	"bytes"
	"context"
	"testing"

	"github.com/mgilbir/forme/fonts/notosans"
	"github.com/mgilbir/forme/shape"
	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/pptx"
	"github.com/mgilbir/spine/render"
)

// TestRenderUnsavedCreatedSlide is the C610 regression: a slide added with
// AddSlide has no layout until save, which binds it to the deck's first
// layout. The renderer took the missing layout for a missing master, so every
// text body on a never-saved Create deck failed with "missing master color
// map" (strict) or was left out (best effort), although PrepareSlide promises
// to include unsaved edits. Rendering the unsaved slide must give exactly what
// rendering it after a save and reopen gives.
func TestRenderUnsavedCreatedSlide(t *testing.T) {
	p := pptx.CreateWidescreen()
	box := p.AddSlide().AddTextBox()
	box.SetPosition(dml.Inches(1), dml.Inches(1))
	box.SetSize(dml.Inches(8), dml.Inches(1))
	box.SetText("Quarterly review")

	noto, err := notosans.Face()
	if err != nil {
		t.Fatal(err)
	}
	opts := render.Options{Fonts: func(context.Context, render.FontRequest) (*shape.Face, error) { return noto, nil }}
	png := func(s *pptx.Slide) []byte {
		t.Helper()
		page, err := PrepareSlide(context.Background(), s, opts) // strict
		if err != nil {
			t.Fatal(err)
		}
		var b bytes.Buffer
		if err := page.WritePNG(context.Background(), &b, 48); err != nil {
			t.Fatal(err)
		}
		return b.Bytes()
	}

	unsaved := png(p.Slides()[0])
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := pptx.OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(unsaved, png(reopened.Slides()[0])) {
		t.Error("the unsaved slide renders differently from the same slide saved and reopened")
	}
}

// TestRenderCreatedDeckPlaceholdersStrictly is the C611 regression: the
// master Create writes gave its title, body and other text styles no kerning
// while the presentation's default text style sets kern="1200". Whether a
// placeholder inherits those defaults is not settled, so the renderer reads
// the style both ways and requires them to agree; they did not, and every
// title or body placeholder on a created deck failed strict rendering with
// "ambiguous inherited kerning". PowerPoint's own master sets kern="1200" on
// those styles.
func TestRenderCreatedDeckPlaceholdersStrictly(t *testing.T) {
	noto, err := notosans.Face()
	if err != nil {
		t.Fatal(err)
	}
	opts := render.Options{Fonts: func(context.Context, render.FontRequest) (*shape.Face, error) { return noto, nil }}
	for _, lt := range []pptx.SlideLayoutType{pptx.LayoutTitle, pptx.LayoutTitleAndContent, pptx.LayoutSectionHeader, pptx.LayoutTwoContent, pptx.LayoutTitleOnly} {
		p := pptx.CreateWidescreen()
		layout, err := p.LayoutByType(lt)
		if err != nil {
			t.Fatal(err)
		}
		slide := p.AddSlideFromLayout(layout)
		for _, ph := range slide.Placeholders() {
			ph.SetText("Some text")
		}
		if _, err := PrepareSlide(context.Background(), slide, opts); err != nil {
			t.Errorf("%s: strict: %v", lt, err)
		}
	}
}
