package render

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"strings"
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
	// The strike is 100 pixels to the em: drawn at any other size its pixels
	// are resampled, which strict preparation refuses and best effort reports.
	for _, sbix := range []bool{true, false} {
		face := colorFont(t, colorFace{sbix: sbix, bitmaps: []*image.NRGBA{quadrants(50)}})
		for _, size := range []float64{50, 100.5, 200} {
			ops := colorOps(face, black, firstBase)
			run := ops[0].(layout.DrawGlyphs)
			run.Size = unit(size)
			ops[0] = run
			prepare := func(approximate func(error)) error {
				_, err := PrepareBestEffort(context.Background(), dml.Pixels(colorPage), dml.Pixels(colorPage), ops, Limits{}, approximate)
				return err
			}
			if err := prepare(nil); !errors.Is(err, ErrUnsupported) {
				t.Fatalf("sbix %v size %v, strict: %v", sbix, size, err)
			}
			var reports []string
			if err := prepare(collect(&reports)); err != nil || len(reports) != 1 || !strings.Contains(reports[0], "color bitmap glyph scaled from its strike") {
				t.Fatalf("sbix %v size %v: %v, reports %q", sbix, size, err, reports)
			}
		}
	}
}
