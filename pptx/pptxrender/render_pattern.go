package pptxrender

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"math"

	"github.com/mgilbir/forme/style"
	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/render"
)

// renderPatterns holds the 8 by 8 bitmaps of the preset patterns
// (ST_PresetPatternVal), measured from PowerPoint's own rendering of each
// preset: one byte per row, the top row first, the 0x80 bit the leftmost
// pixel, set where the foreground color shows. The standard names the
// patterns without giving their pixels.
var renderPatterns = map[string][8]uint8{
	"pct5":       {0x80, 0x00, 0x00, 0x00, 0x08, 0x00, 0x00, 0x00},
	"pct10":      {0x80, 0x00, 0x08, 0x00, 0x80, 0x00, 0x08, 0x00},
	"pct20":      {0x88, 0x00, 0x22, 0x00, 0x88, 0x00, 0x22, 0x00},
	"pct25":      {0x88, 0x22, 0x88, 0x22, 0x88, 0x22, 0x88, 0x22},
	"pct30":      {0xAA, 0x44, 0xAA, 0x11, 0xAA, 0x44, 0xAA, 0x11},
	"pct40":      {0xAA, 0x55, 0xAA, 0x51, 0xAA, 0x55, 0xAA, 0x15},
	"pct50":      {0xAA, 0x55, 0xAA, 0x55, 0xAA, 0x55, 0xAA, 0x55},
	"pct60":      {0xEE, 0x55, 0xBB, 0x55, 0xEE, 0x55, 0xBB, 0x55},
	"pct70":      {0x77, 0xDD, 0x77, 0xDD, 0x77, 0xDD, 0x77, 0xDD},
	"pct75":      {0x77, 0xFF, 0xDD, 0xFF, 0x77, 0xFF, 0xDD, 0xFF},
	"pct80":      {0xEF, 0xFF, 0xFE, 0xFF, 0xEF, 0xFF, 0xFE, 0xFF},
	"pct90":      {0xFF, 0xFF, 0xFF, 0xF7, 0xFF, 0xFF, 0xFF, 0x7F},
	"horz":       {0xFF, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
	"vert":       {0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80},
	"ltHorz":     {0xFF, 0x00, 0x00, 0x00, 0xFF, 0x00, 0x00, 0x00},
	"ltVert":     {0x88, 0x88, 0x88, 0x88, 0x88, 0x88, 0x88, 0x88},
	"dkHorz":     {0xFF, 0xFF, 0x00, 0x00, 0xFF, 0xFF, 0x00, 0x00},
	"dkVert":     {0xCC, 0xCC, 0xCC, 0xCC, 0xCC, 0xCC, 0xCC, 0xCC},
	"narHorz":    {0xFF, 0x00, 0xFF, 0x00, 0xFF, 0x00, 0xFF, 0x00},
	"narVert":    {0x55, 0x55, 0x55, 0x55, 0x55, 0x55, 0x55, 0x55},
	"dashHorz":   {0xF0, 0x00, 0x00, 0x00, 0x0F, 0x00, 0x00, 0x00},
	"dashVert":   {0x80, 0x80, 0x80, 0x80, 0x08, 0x08, 0x08, 0x08},
	"cross":      {0xFF, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80},
	"dnDiag":     {0x80, 0x40, 0x20, 0x10, 0x08, 0x04, 0x02, 0x01},
	"upDiag":     {0x01, 0x02, 0x04, 0x08, 0x10, 0x20, 0x40, 0x80},
	"ltDnDiag":   {0x88, 0x44, 0x22, 0x11, 0x88, 0x44, 0x22, 0x11},
	"ltUpDiag":   {0x11, 0x22, 0x44, 0x88, 0x11, 0x22, 0x44, 0x88},
	"dkDnDiag":   {0xCC, 0x66, 0x33, 0x99, 0xCC, 0x66, 0x33, 0x99},
	"dkUpDiag":   {0x33, 0x66, 0xCC, 0x99, 0x33, 0x66, 0xCC, 0x99},
	"wdDnDiag":   {0xC1, 0xE0, 0x70, 0x38, 0x1C, 0x0E, 0x07, 0x83},
	"wdUpDiag":   {0x83, 0x07, 0x0E, 0x1C, 0x38, 0x70, 0xE0, 0xC1},
	"dashDnDiag": {0x00, 0x00, 0x88, 0x44, 0x22, 0x11, 0x00, 0x00},
	"dashUpDiag": {0x00, 0x00, 0x11, 0x22, 0x44, 0x88, 0x00, 0x00},
	"diagCross":  {0x81, 0x42, 0x24, 0x18, 0x18, 0x24, 0x42, 0x81},
	"smCheck":    {0x99, 0x66, 0x66, 0x99, 0x99, 0x66, 0x66, 0x99},
	"lgCheck":    {0xF0, 0xF0, 0xF0, 0xF0, 0x0F, 0x0F, 0x0F, 0x0F},
	"smGrid":     {0xFF, 0x88, 0x88, 0x88, 0xFF, 0x88, 0x88, 0x88},
	"lgGrid":     {0xFF, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80, 0x80},
	"dotGrid":    {0xAA, 0x00, 0x80, 0x00, 0x80, 0x00, 0x80, 0x00},
	"smConfetti": {0x80, 0x08, 0x40, 0x02, 0x10, 0x01, 0x20, 0x04},
	"lgConfetti": {0xB1, 0x30, 0x03, 0x1B, 0xD8, 0xC0, 0x0C, 0x8D},
	"horzBrick":  {0xFF, 0x80, 0x80, 0x80, 0xFF, 0x08, 0x08, 0x08},
	"diagBrick":  {0x01, 0x02, 0x04, 0x08, 0x18, 0x24, 0x42, 0x81},
	"solidDmnd":  {0x10, 0x38, 0x7C, 0xFE, 0x7C, 0x38, 0x10, 0x00},
	"openDmnd":   {0x82, 0x44, 0x28, 0x10, 0x28, 0x44, 0x82, 0x01},
	"dotDmnd":    {0x80, 0x00, 0x22, 0x00, 0x08, 0x00, 0x22, 0x00},
	"plaid":      {0xAA, 0x55, 0xAA, 0x55, 0xF0, 0xF0, 0xF0, 0xF0},
	"sphere":     {0x77, 0x89, 0x8F, 0x8F, 0x77, 0x98, 0xF8, 0xF8},
	"weave":      {0x88, 0x54, 0x22, 0x45, 0x88, 0x14, 0x22, 0x51},
	"divot":      {0x00, 0x10, 0x08, 0x10, 0x00, 0x80, 0x01, 0x80},
	"shingle":    {0x03, 0x84, 0x48, 0x30, 0x0C, 0x02, 0x01, 0x01},
	"wave":       {0x00, 0x18, 0x25, 0xC0, 0x00, 0x18, 0x25, 0xC0},
	"trellis":    {0xFF, 0x66, 0xFF, 0x99, 0xFF, 0x66, 0xFF, 0x99},
	"zigZag":     {0x81, 0x42, 0x24, 0x18, 0x81, 0x42, 0x24, 0x18},
}

// renderPatternPoint is how many CSS pixels a pattern pixel spans: PowerPoint
// draws one per point, a 72nd of an inch.
const renderPatternPoint = 96.0 / 72

// renderPatternScales are the image pixels per CSS pixel a pattern fill is
// composed at, densest first. A pattern pixel is 4/3 CSS pixels, so at 3
// image pixels per CSS pixel it is 4 whole image pixels, and at 1.5 it is 2.
// Output at 288 DPI (3 device pixels per CSS pixel) shows the 3 one to one
// and the 1.5 doubled, and at 144 DPI (1.5) the 1.5 one to one and the 3
// halved, so pattern edges land on output pixels there. At 96 DPI a pattern
// pixel is 1.33 device pixels, so its edges cannot all be crisp whatever the
// scale. The densest scale that holds the fill's pixel budget is used,
// falling to 1 for a fill as large as a slide, whose pattern pixels are then
// sampled once per CSS pixel.
var renderPatternScales = [...]float64{3, 1.5, 1}

// patternPaint resolves a pattern fill for a box w by h CSS pixels at (x, y)
// on the slide to a composed image of its foreground and background.
// PowerPoint anchors the tiling to the slide's origin, so the box shows the
// part of the pattern under it; absent colors are black on white. turned is
// set for a box that is rotated or flipped, by itself or by a group: how
// PowerPoint tiles those is not known, so the pattern is composed from the
// unturned box's phase, as for an upright shape, and left to the shape's
// turn, approximately.
func (c *renderColors) patternPaint(p *dml.PattFill, placeholder *style.RGBA, x, y, w, h float64, turned bool) (renderPaint, error) {
	prst := p.Prst
	if prst == "" {
		return renderPaint{}, fmt.Errorf("%w: pattern without a preset", render.ErrUnsupported)
	}
	bits, ok := renderPatterns[prst]
	if !ok {
		return renderPaint{}, fmt.Errorf("%w: pattern %q", render.ErrInvalid, p.Prst)
	}
	for _, v := range []float64{w, h} {
		if !(v > 0) || math.IsInf(v, 0) || v > 1<<24 {
			return renderPaint{}, fmt.Errorf("%w: pattern extent", render.ErrInvalid)
		}
	}
	for _, v := range []float64{x, y} {
		if math.IsNaN(v) || math.Abs(v) > 1<<30 {
			return renderPaint{}, fmt.Errorf("%w: pattern position", render.ErrInvalid)
		}
	}
	if turned {
		if err := c.approximate(fmt.Errorf("%w: pattern %s in a rotated or flipped shape drawn as if upright", render.ErrUnsupported, prst)); err != nil {
			return renderPaint{}, err
		}
	}
	fg, bg := style.RGBA{A: 1}, renderWhite
	var err error
	if p.FgClr != nil {
		if fg, err = c.color(renderChoiceColor(p.FgClr), placeholder); err != nil {
			return renderPaint{}, err
		}
	}
	if p.BgClr != nil {
		if bg, err = c.color(renderChoiceColor(p.BgClr), placeholder); err != nil {
			return renderPaint{}, err
		}
	}
	nrgba := func(c style.RGBA) color.NRGBA {
		return color.NRGBA{R: uint8(math.Round(c.R)), G: uint8(math.Round(c.G)), B: uint8(math.Round(c.B)), A: uint8(math.Round(c.A * 255))}
	}
	on, off := nrgba(fg), nrgba(bg)
	scale := renderPatternScales[len(renderPatternScales)-1]
	for _, s := range renderPatternScales {
		if math.Ceil(w*s)*math.Ceil(h*s) <= renderMaxTilePixels {
			scale = s
			break
		}
	}
	// The image starts at the point grid line at or before the box, so that
	// its pixels, and the pattern's, sit on output pixels wherever the box
	// itself does not, and runs to cover the box.
	x0, y0 := math.Floor(x/renderPatternPoint)*renderPatternPoint, math.Floor(y/renderPatternPoint)*renderPatternPoint
	ow, oh := max(1, int(math.Ceil((x+w-x0)*scale))), max(1, int(math.Ceil((y+h-y0)*scale)))
	n := int64(ow) * int64(oh)
	if n > renderMaxTilePixels*2 || n > 4*renderMaxTilePixels-c.tilePixels {
		return renderPaint{}, fmt.Errorf("%w: tiled fill pixels", render.ErrLimit)
	}
	c.tilePixels += n
	ctx := c.ctx
	if ctx == nil {
		ctx = context.Background()
	}
	// along gives the pattern pixel, 0 to 7, under each image pixel's
	// centre on one axis, the image starting at origin: the point it lies
	// at from the slide's origin, floored.
	along := func(origin float64, count int) []int {
		out := make([]int, count)
		for i := range out {
			pt := math.Floor((origin + (float64(i)+0.5)/scale) / renderPatternPoint)
			out[i] = int(math.Mod(math.Mod(pt, 8)+8, 8))
		}
		return out
	}
	columns, rows := along(x0, ow), along(y0, oh)
	img := image.NewNRGBA(image.Rect(0, 0, ow, oh))
	for j, r := range rows {
		if j%64 == 0 {
			if err := ctx.Err(); err != nil {
				return renderPaint{}, err
			}
		}
		dst := img.Pix[j*img.Stride:]
		for i, col := range columns {
			px := off
			if bits[r]&(0x80>>col) != 0 {
				px = on
			}
			dst[4*i], dst[4*i+1], dst[4*i+2], dst[4*i+3] = px.R, px.G, px.B, px.A
		}
	}
	// The image runs past the box on each side, clipped to it.
	ew, eh := float64(ow)/scale, float64(oh)/scale
	l, t := (x0-x)/w, (y0-y)/h
	return renderPaint{image: img, fill: [4]float64{l, t, 1 - l - ew/w, 1 - t - eh/h}}, nil
}
