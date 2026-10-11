package docxrender

import (
	"bytes"
	"context"
	"errors"
	"image/png"
	"sync"
	"testing"

	"github.com/mgilbir/forme/fonttest"
	"github.com/mgilbir/forme/shape"
	"github.com/mgilbir/spine/docx"
	"github.com/mgilbir/spine/render"
)

// colorGlyphFace is a 2000-unit font whose letter B is a COLRv1 sweep
// gradient, which the page painter cannot draw exactly, and whose letter A is
// a solid red box, which it can.
func colorGlyphFace(t *testing.T) *shape.Face {
	t.Helper()
	be16 := func(v int) []byte { return []byte{byte(v >> 8), byte(v)} }
	be32 := func(v int) []byte { return []byte{byte(v >> 24), byte(v >> 16), byte(v >> 8), byte(v)} }
	cat := func(parts ...[]byte) []byte {
		var out []byte
		for _, p := range parts {
			out = append(out, p...)
		}
		return out
	}
	solid := cat([]byte{10, 0, 0, 6}, be16(3), []byte{2}, be16(0), be16(0x4000))
	sweep := cat([]byte{10, 0, 0, 6}, be16(3),
		[]byte{8, 0, 0, 12}, be16(1000), be16(800), be16(0), be16(0x4000),
		[]byte{0}, be16(2), be16(0), be16(0), be16(0x4000), be16(0x4000), be16(1), be16(0x4000))
	const header = 34
	list := cat(be32(2), be16(1), be32(4+12), be16(2), be32(4+12+len(solid)), solid, sweep)
	colr := cat(be16(1), be16(0), be32(header), be32(header), be16(0), be32(header), be32(0), be32(0), be32(0), be32(0), list)
	cpal := cat(be16(0), be16(2), be16(1), be16(2), be32(14), be16(0), []byte{0, 0, 255, 255, 255, 0, 0, 255})
	f, err := shape.Load(fonttest.SFNT(fonttest.SFNTOptions{UnitsPerEm: 2000, Ascent: 1600, Descent: -400, Glyphs: []fonttest.Glyph{
		{Rune: 'A', Advance: 2000, HasShape: true, Ink: [4]int{900, 700, 1100, 900}},
		{Rune: 'B', Advance: 2000, HasShape: true, Ink: [4]int{900, 700, 1100, 900}},
		{Unmapped: true, Advance: 2000, HasShape: true, Ink: [4]int{0, 0, 2000, 1600}},
		{Rune: ' ', Advance: 1000},
	}, Extra: map[string][]byte{"COLR": colr, "CPAL": cpal}}))
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// TestPageApproximatesColorGlyphsInBestEffort: a Word page was always
// prepared strictly, so with Warn set a color glyph that can only be drawn
// approximately still failed the whole page with ErrUnsupported, where a
// slide draws it approximately and reports it. In best effort the page now
// draws it and reports ErrApproximated; strict still refuses it.
func TestPageApproximatesColorGlyphsInBestEffort(t *testing.T) {
	doc := docx.Create()
	section := doc.DefaultSection()
	section.SetPageSize(360, 240)
	section.SetMargins(docx.PageMargins{Top: 24, Bottom: 24, Left: 24, Right: 24})
	r := doc.AddParagraph().AddRun()
	r.SetText("B")
	r.SetFont("Color")
	r.SetFontSize(36)
	face := colorGlyphFace(t)
	fonts := func(context.Context, render.FontRequest) (*shape.Face, error) { return face, nil }
	ctx := context.Background()

	strict, err := Prepare(ctx, doc, render.Options{Fonts: fonts})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := strict.Page(ctx, 1); !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("strict: %v, want ErrUnsupported", err)
	}

	var mu sync.Mutex
	var warnings []error
	lenient, err := Prepare(ctx, doc, render.Options{Fonts: fonts, Warn: func(err error) {
		mu.Lock()
		defer mu.Unlock()
		warnings = append(warnings, err)
	}})
	if err != nil {
		t.Fatal(err)
	}
	page, err := lenient.Page(ctx, 1)
	if err != nil {
		t.Fatalf("best effort: %v", err)
	}
	approximated := false
	for _, w := range warnings {
		approximated = approximated || errors.Is(w, render.ErrApproximated)
	}
	if !approximated {
		t.Errorf("best effort reported %v, want an ErrApproximated warning", warnings)
	}

	// The glyph is drawn, as one color, rather than left blank.
	var buf bytes.Buffer
	if err := page.WritePNG(ctx, &buf, 96); err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(&buf)
	if err != nil {
		t.Fatal(err)
	}
	inked := false
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y && !inked; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if r, g, bl, a := img.At(x, y).RGBA(); a > 0 && (r>>8 < 200 || g>>8 < 200 || bl>>8 < 200) {
				inked = true
				break
			}
		}
	}
	if !inked {
		t.Error("best effort page has no ink: the color glyph was not drawn")
	}
}
