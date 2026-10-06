package render

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"testing"

	"github.com/mgilbir/forme/layout"
	"github.com/mgilbir/spine/common/dml"
)

var yellow = color.RGBA{255, 255, 0, 255}

func TestColorGlyphBitmap(t *testing.T) {
	for _, sbix := range []bool{true, false} {
		face := colorFont(t, colorFace{sbix: sbix, bitmaps: []*image.NRGBA{quadrants(50)}})
		img := colorRaster(t, face, black, firstBase)
		for _, tc := range []struct {
			x, y float64
			want color.RGBA
		}{{125, 375, red}, {375, 375, green}, {125, 125, blue}, {375, 125, yellow}, {600, 250, nothing}, {250, 600, nothing}} {
			near(t, at(img, tc.x, tc.y), tc.want, 0, fmt.Sprintf("sbix %v at %v, %v", sbix, tc.x, tc.y))
		}
	}
}

func TestColorGlyphBitmapScaledFromItsStrike(t *testing.T) {
	// The strike is 100 pixels to the em. Color strikes are made to be scaled,
	// as an emoji font's one large strike is, so drawn at any other size they
	// are exact: strict preparation draws them and nothing is reported.
	for _, sbix := range []bool{true, false} {
		face := colorFont(t, colorFace{sbix: sbix, bitmaps: []*image.NRGBA{quadrants(50)}})
		for _, size := range []float64{50, 100.5, 200} {
			ops := colorOps(face, black, firstBase)
			run := ops[0].(layout.DrawGlyphs)
			run.Size = unit(size)
			ops[0] = run
			var reports []string
			for _, approximate := range []func(error){nil, collect(&reports)} {
				if _, err := PrepareBestEffort(context.Background(), dml.Pixels(colorPage), dml.Pixels(colorPage), ops, Limits{}, approximate); err != nil {
					t.Fatalf("sbix %v size %v: %v", sbix, size, err)
				}
			}
			if len(reports) != 0 {
				t.Fatalf("sbix %v size %v: reports %q", sbix, size, reports)
			}
		}
	}
}
