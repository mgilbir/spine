package pptx

import (
	"fmt"
	"image"
	"image/draw"
	"math"

	"github.com/mgilbir/forme/style"
	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/render"
)

// blipEffects applies a picture's color effects, in document order, to a
// copy of its image: grayscale, bi-level, duotone, color replacement and
// change, HSL shifts, the alpha effects and solid fill overlays as the
// standard defines them. Luminance is Rec. 601's. Brightness and contrast,
// which the standard does not define, follow LibreOffice, and tint and
// gradient overlays are drawn approximately; blur, alpha masks and unknown
// effects are left out. Each is reported, so strict mode refuses them.
func (c *renderColors) blipEffects(img image.Image, effects []*dml.BlipEffect) (image.Image, error) {
	if len(effects) == 0 {
		return img, nil
	}
	b := img.Bounds()
	out := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(out, out.Rect, img, b.Min, draw.Src)
	color := func(choice dml.ColorChoice) (style.RGBA, error) {
		return c.color(renderChoiceColor(&choice), nil)
	}
	for _, e := range effects {
		if e == nil {
			continue
		}
		if c.ctx != nil && c.ctx.Err() != nil {
			return nil, c.ctx.Err()
		}
		var (
			f   func(p []uint8)
			err error
		)
		switch {
		case e.AlphaModFix != nil:
			amt := 1.0
			if e.AlphaModFix.Amt != nil {
				amt = math.Max(0, float64(e.AlphaModFix.Amt.Int32())/100000)
			}
			f = func(p []uint8) { p[3] = renderByte(float64(p[3]) * amt) }
		case e.AlphaRepl != nil:
			a := renderByte(float64(e.AlphaRepl.A.Int32()) / 100000 * 255)
			f = func(p []uint8) { p[3] = a }
		case e.AlphaBiLevel != nil:
			t := float64(e.AlphaBiLevel.Thresh.Int32()) / 100000 * 255
			f = func(p []uint8) {
				if float64(p[3]) >= t {
					p[3] = 255
				} else {
					p[3] = 0
				}
			}
		case e.AlphaCeiling != nil:
			f = func(p []uint8) {
				if p[3] > 0 {
					p[3] = 255
				}
			}
		case e.AlphaFloor != nil:
			f = func(p []uint8) {
				if p[3] < 255 {
					p[3] = 0
				}
			}
		case e.AlphaInv != nil:
			v := e.AlphaInv
			choice := dml.ColorChoice{SrgbClr: v.SrgbClr, SchemeClr: v.SchemeClr, SysClr: v.SysClr, PrstClr: v.PrstClr, HslClr: v.HslClr, ScrgbClr: v.ScRgbClr}
			set := choice != (dml.ColorChoice{})
			var to style.RGBA
			if set {
				if to, err = color(choice); err != nil {
					return nil, err
				}
			}
			f = func(p []uint8) {
				if set {
					p[0], p[1], p[2] = renderByte(to.R), renderByte(to.G), renderByte(to.B)
				}
				p[3] = 255 - p[3]
			}
		case e.Grayscl != nil:
			f = func(p []uint8) {
				y := renderByte(renderLuma(p))
				p[0], p[1], p[2] = y, y, y
			}
		case e.BiLevel != nil:
			t := float64(e.BiLevel.Thresh.Int32()) / 100000 * 255
			f = func(p []uint8) {
				v := uint8(0)
				if renderLuma(p) >= t {
					v = 255
				}
				p[0], p[1], p[2] = v, v, v
			}
		case e.Duotone != nil:
			cs := e.Duotone.Colors()
			if len(cs) != 2 {
				return nil, fmt.Errorf("%w: duotone colors", render.ErrInvalid)
			}
			dark, err := color(cs[0])
			if err != nil {
				return nil, err
			}
			light, err := color(cs[1])
			if err != nil {
				return nil, err
			}
			f = func(p []uint8) {
				t := renderLuma(p) / 255
				p[0], p[1], p[2] = renderByte(dark.R+(light.R-dark.R)*t), renderByte(dark.G+(light.G-dark.G)*t), renderByte(dark.B+(light.B-dark.B)*t)
			}
		case e.ClrRepl != nil:
			v := e.ClrRepl
			to, err := color(dml.ColorChoice{SrgbClr: v.SrgbClr, SchemeClr: v.SchemeClr, SysClr: v.SysClr, PrstClr: v.PrstClr, HslClr: v.HslClr, ScrgbClr: v.ScRgbClr})
			if err != nil {
				return nil, err
			}
			f = func(p []uint8) {
				p[0], p[1], p[2] = renderByte(to.R), renderByte(to.G), renderByte(to.B)
				p[3] = renderByte(float64(p[3]) * to.A)
			}
		case e.ClrChange != nil:
			v := e.ClrChange
			if v.ClrFrom == nil || v.ClrTo == nil {
				return nil, fmt.Errorf("%w: color change", render.ErrInvalid)
			}
			from, err := color(*v.ClrFrom)
			if err != nil {
				return nil, err
			}
			to, err := color(*v.ClrTo)
			if err != nil {
				return nil, err
			}
			useA := v.UseA == nil || *v.UseA
			fr, fg, fb := renderByte(from.R), renderByte(from.G), renderByte(from.B)
			f = func(p []uint8) {
				if p[0] != fr || p[1] != fg || p[2] != fb {
					return
				}
				p[0], p[1], p[2] = renderByte(to.R), renderByte(to.G), renderByte(to.B)
				if useA {
					p[3] = renderByte(to.A * 255)
				}
			}
		case e.Hsl != nil:
			dh := float64(e.Hsl.Hue) / 60000 / 360
			ds, dl := float64(e.Hsl.Sat.Int32())/100000, float64(e.Hsl.Lum.Int32())/100000
			f = func(p []uint8) {
				h, s, l := renderToHSL(p)
				h = math.Mod(h+dh+1, 1)
				renderFromHSL(p, h, math.Min(1, math.Max(0, s+ds)), math.Min(1, math.Max(0, l+dl)))
			}
		case e.Lum != nil:
			if err := c.approximate(fmt.Errorf("%w: picture brightness and contrast drawn as LibreOffice does", render.ErrUnsupported)); err != nil {
				return nil, err
			}
			// LibreOffice's Bitmap::Adjust, with both as percentages.
			bright, contrast := float64(e.Lum.Bright.Int32())/1000, float64(e.Lum.Contrast.Int32())/1000
			m := (128 + 1.27*contrast) / 128
			if contrast > 0 {
				m = 128 / (128 - 1.27*math.Min(contrast, 100))
			}
			off := bright*2.55 + 128 - m*128
			f = func(p []uint8) {
				for k := 0; k < 3; k++ {
					p[k] = renderByte(float64(p[k])*m + off)
				}
			}
		case e.Tint != nil:
			if err := c.approximate(fmt.Errorf("%w: picture tint drawn approximately", render.ErrUnsupported)); err != nil {
				return nil, err
			}
			// Hue moves the given share of the way to the tint's.
			target := math.Mod(float64(e.Tint.Hue)/60000/360, 1)
			amt := math.Max(-1, math.Min(1, float64(e.Tint.Amt.Int32())/100000))
			f = func(p []uint8) {
				h, s, l := renderToHSL(p)
				d := math.Mod(target-h+1.5, 1) - 0.5
				renderFromHSL(p, math.Mod(h+d*amt+1, 1), s, l)
			}
		case e.FillOverlay != nil:
			v := e.FillOverlay
			var fill style.RGBA
			switch {
			case v.SolidFill != nil:
				fill, err = c.solid(v.SolidFill, nil)
			case v.GradFill != nil:
				fill, err = c.representative(v.GradFill, nil, nil)
			default:
				return nil, fmt.Errorf("%w: fill overlay without a fill", render.ErrUnsupported)
			}
			if err != nil {
				return nil, err
			}
			blend, err := renderBlend(v.Blend)
			if err != nil {
				return nil, err
			}
			fc := [3]float64{fill.R, fill.G, fill.B}
			f = func(p []uint8) {
				for k := 0; k < 3; k++ {
					s := float64(p[k])
					p[k] = renderByte(s + (blend(s, fc[k])-s)*fill.A)
				}
			}
		case e.Blur != nil || e.AlphaMod != nil:
			if err := c.approximate(fmt.Errorf("%w: picture blur or alpha mask left out", render.ErrUnsupported)); err != nil {
				return nil, err
			}
			continue
		default:
			if err := c.approximate(fmt.Errorf("%w: unknown picture effect left out", render.ErrUnsupported)); err != nil {
				return nil, err
			}
			continue
		}
		for i := 0; i < len(out.Pix); i += 4 {
			f(out.Pix[i : i+4])
		}
	}
	return out, nil
}

// renderBlend is a fill overlay's blend of a channel s with the fill's f.
func renderBlend(mode string) (func(s, f float64) float64, error) {
	switch mode {
	case "over":
		return func(_, f float64) float64 { return f }, nil
	case "mult":
		return func(s, f float64) float64 { return s * f / 255 }, nil
	case "screen":
		return func(s, f float64) float64 { return 255 - (255-s)*(255-f)/255 }, nil
	case "darken":
		return math.Min, nil
	case "lighten":
		return math.Max, nil
	}
	return nil, fmt.Errorf("%w: fill overlay blend %q", render.ErrInvalid, mode)
}

func renderByte(v float64) uint8 {
	return uint8(math.Round(math.Min(255, math.Max(0, v))))
}

// renderLuma is a pixel's Rec. 601 luminance, 0 to 255.
func renderLuma(p []uint8) float64 {
	return 0.299*float64(p[0]) + 0.587*float64(p[1]) + 0.114*float64(p[2])
}

// renderToHSL reads a pixel as hue, saturation and lightness, each 0 to 1.
func renderToHSL(p []uint8) (h, s, l float64) {
	return renderHSL(style.RGBA{R: float64(p[0]), G: float64(p[1]), B: float64(p[2])})
}

// renderFromHSL writes hue, saturation and lightness to a pixel's color.
func renderFromHSL(p []uint8, h, s, l float64) {
	c := renderFromHSLExact(h, s, l)
	p[0], p[1], p[2] = renderByte(c[0]*255), renderByte(c[1]*255), renderByte(c[2]*255)
}
