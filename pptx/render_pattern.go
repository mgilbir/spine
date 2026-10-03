package pptx

import (
	"fmt"
	"image"
	"image/color"
	"math"

	"github.com/mgilbir/forme/style"
	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/render"
)

// renderPatternCell is how many image pixels a pattern pixel spans, so that
// the composed fill keeps its pattern's edges sharp when drawn larger.
const renderPatternCell = 4

// renderBayer is the 8 by 8 ordered-dither matrix, ranking each cell 0 to 63.
var renderBayer = func() (m [8][8]int) {
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			v, xc, yc := 0, x^y, y
			for bit := 0; bit < 3; bit++ {
				// The lowest coordinate bits rank highest, which spreads
				// each threshold's cells evenly.
				v = v<<2 | (xc>>bit&1)<<1 | (yc >> bit & 1)
			}
			m[y][x] = v
		}
	}
	return m
}()

// renderPatternBits draws a preset pattern as 8 by 8 pixels, set where the
// foreground shows. The standard names the patterns and pictures them
// without giving their pixels, so each is drawn from its description:
// percentages as ordered dithers of that density, and lines, grids, checks
// and figures as their names say, light ones thin, dark ones thick, narrow
// ones close and wide ones far apart.
func renderPatternBits(prst string) (bits [8][8]bool, ok bool) {
	set := func(f func(x, y int) bool) [8][8]bool {
		var b [8][8]bool
		for y := 0; y < 8; y++ {
			for x := 0; x < 8; x++ {
				b[y][x] = f(x, y)
			}
		}
		return b
	}
	mod := func(v, m int) int { return ((v % m) + m) % m }
	percents := map[string]int{"pct5": 5, "pct10": 10, "pct20": 20, "pct25": 25, "pct30": 30, "pct40": 40, "pct50": 50, "pct60": 60, "pct70": 70, "pct75": 75, "pct80": 80, "pct90": 90}
	if p, found := percents[prst]; found {
		n := int(math.Round(float64(p) * 64 / 100))
		return set(func(x, y int) bool { return renderBayer[y][x] < n }), true
	}
	// Diagonals: down runs from top left to bottom right.
	down := func(period, width int) func(x, y int) bool {
		return func(x, y int) bool { return mod(x-y, period) < width }
	}
	up := func(period, width int) func(x, y int) bool {
		return func(x, y int) bool { return mod(x+y, period) < width }
	}
	f := map[string]func(x, y int) bool{
		"horz":       func(_, y int) bool { return y == 0 },
		"ltHorz":     func(_, y int) bool { return y%4 == 0 },
		"narHorz":    func(_, y int) bool { return y%2 == 0 },
		"dkHorz":     func(_, y int) bool { return y%4 < 2 },
		"dashHorz":   func(x, y int) bool { return (y == 0 && x < 4) || (y == 4 && x >= 4) },
		"vert":       func(x, _ int) bool { return x == 0 },
		"ltVert":     func(x, _ int) bool { return x%4 == 0 },
		"narVert":    func(x, _ int) bool { return x%2 == 0 },
		"dkVert":     func(x, _ int) bool { return x%4 < 2 },
		"dashVert":   func(x, y int) bool { return (x == 0 && y < 4) || (x == 4 && y >= 4) },
		"cross":      func(x, y int) bool { return x == 0 || y == 0 },
		"lgGrid":     func(x, y int) bool { return x == 0 || y == 0 },
		"smGrid":     func(x, y int) bool { return x%4 == 0 || y%4 == 0 },
		"dotGrid":    func(x, y int) bool { return (y == 0 && x%2 == 0) || (x == 0 && y%2 == 0) },
		"dnDiag":     down(8, 1),
		"ltDnDiag":   down(4, 1),
		"dkDnDiag":   down(4, 2),
		"wdDnDiag":   down(8, 3),
		"dashDnDiag": func(x, y int) bool { return mod(x-y, 4) == 0 && y%4 < 2 },
		"upDiag":     up(8, 1),
		"ltUpDiag":   up(4, 1),
		"dkUpDiag":   up(4, 2),
		"wdUpDiag":   up(8, 3),
		"dashUpDiag": func(x, y int) bool { return mod(x+y, 4) == 0 && y%4 < 2 },
		"diagCross":  func(x, y int) bool { return mod(x-y, 8) == 0 || mod(x+y, 8) == 0 },
		"smCheck":    func(x, y int) bool { return (x/2+y/2)%2 == 0 },
		"lgCheck":    func(x, y int) bool { return (x/4+y/4)%2 == 0 },
		"horzBrick":  func(x, y int) bool { return y == 0 || y == 4 || (y < 4 && x == 0) || (y > 4 && x == 4) },
		"diagBrick":  func(x, y int) bool { return mod(x+y, 8) == 0 || (mod(x-y, 8) == 4 && y < 4) },
		"solidDmnd":  func(x, y int) bool { return math.Abs(float64(x)-3.5)+math.Abs(float64(y)-3.5) <= 3.5 },
		"openDmnd":   func(x, y int) bool { return mod(x+y, 8) == 4 || mod(x-y, 8) == 0 },
		"dotDmnd":    func(x, y int) bool { return (x == 0 && y == 0) || (x == 4 && y == 4) },
		"plaid":      func(x, y int) bool { return (y < 2 && x%2 == 0) || (x < 2 && y%2 == 0) || (x < 2 && y < 2) },
		"sphere": func(x, y int) bool {
			d := math.Hypot(float64(x)-3.5, float64(y)-3.5)
			return d <= 3.6 && (x != 2 || y != 2)
		},
		"weave":      func(x, y int) bool { return (mod(x-y, 8) < 2 && x%8 < 4) || (mod(x+y, 8) < 2 && x%8 >= 4) },
		"divot":      func(x, y int) bool { return (y == 1 && x >= 2 && x <= 3) || (y == 2 && x == 4) || (y == 5 && x >= 6) || (y == 6 && x == 0) },
		"shingle":    func(x, y int) bool { return mod(x+y, 8) == 0 || (y == 4 && x >= 4) },
		"wave":       func(x, y int) bool { return y == int(math.Round(1.5-1.5*math.Cos(float64(x)*math.Pi/4))) },
		"trellis":    func(x, y int) bool { return mod(x-y, 4) < 2 || mod(x+y, 4) < 2 },
		"zigZag":     func(x, y int) bool { d := x % 4; return y%4 == min(d, 4-d) },
		"smConfetti": func(x, y int) bool { return renderBayer[mod(3*y+1, 8)][mod(5*x+2, 8)] < 8 },
		"lgConfetti": func(x, y int) bool { return renderBayer[mod(3*y+1, 8)][mod(5*x+2, 8)] < 16 },
	}[prst]
	if f == nil {
		return bits, false
	}
	return set(f), true
}

// patternPaint resolves a pattern fill to a tiled image of its foreground
// and background, drawn approximately: see renderPatternBits. A pattern
// pixel is a CSS pixel, as PowerPoint draws them at 100% zoom; absent
// colors are black on white.
func (c *renderColors) patternPaint(p *dml.PattFill, placeholder *style.RGBA, w, h float64) (renderPaint, error) {
	prst := p.Prst
	if prst == "" {
		return renderPaint{}, fmt.Errorf("%w: pattern without a preset", render.ErrUnsupported)
	}
	bits, ok := renderPatternBits(prst)
	if !ok {
		return renderPaint{}, fmt.Errorf("%w: pattern %q", render.ErrInvalid, p.Prst)
	}
	if err := c.approximate(fmt.Errorf("%w: pattern %s drawn from its description", render.ErrUnsupported, prst)); err != nil {
		return renderPaint{}, err
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
	cell := image.NewNRGBA(image.Rect(0, 0, 8*renderPatternCell, 8*renderPatternCell))
	for y := 0; y < 8*renderPatternCell; y++ {
		for x := 0; x < 8*renderPatternCell; x++ {
			if bits[y/renderPatternCell][x/renderPatternCell] {
				cell.SetNRGBA(x, y, on)
			} else {
				cell.SetNRGBA(x, y, off)
			}
		}
	}
	img, err := renderTileImage(c, cell, w, h, 8, 8, &dml.TileXML{})
	return renderPaint{image: img}, err
}
