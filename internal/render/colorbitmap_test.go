package render

import (
	"fmt"
	"image"
	"image/color"
	"testing"
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
