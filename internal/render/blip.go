package render

import (
	"context"
	"fmt"
	"image"
	"math"

	"github.com/mgilbir/forme/style"
	"github.com/mgilbir/spine/common/dml"
)

// BlipColors resolves the colors a picture's effects name. An error from it
// stops the effects.
type BlipColors interface {
	// Color resolves a color choice.
	Color(dml.ColorChoice) (style.RGBA, error)
	// Solid resolves a solid fill to its color.
	Solid(*dml.SolidFill) (style.RGBA, error)
	// Gradient resolves a gradient fill to one representative color.
	Gradient(*dml.GradFill) (style.RGBA, error)
	// Approximate reports that an effect is drawn approximately or left out:
	// it returns err in strict mode and nil in best-effort mode.
	Approximate(err error) error
}

// BlipEffects applies a picture's color effects, in document order, to a
// copy of its image: grayscale, bi-level, duotone, color replacement and
// change, HSL shifts, the alpha effects and solid fill overlays as the
// standard defines them. Luminance is Rec. 601's. Brightness and contrast,
// which the standard does not define, follow LibreOffice, and tint and
// gradient overlays are drawn approximately, as is blur, whose spread past
// the picture's box is left out; alpha masks and unknown effects are left
// out. Each is reported through colors, so strict mode refuses them.
//
// A blur spreads each pixel over its radius, emuPerPixel the EMU a picture
// pixel is drawn across; it stays within the picture's box.
func BlipEffects(ctx context.Context, colors BlipColors, img image.Image, effects []*dml.BlipEffect, emuPerPixel float64) (image.Image, error) {
	if len(effects) == 0 {
		return img, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}
	out := NRGBA(img)
	color := func(choice dml.ColorChoice) (style.RGBA, error) {
		return colors.Color(choice)
	}
	for _, e := range effects {
		if e == nil {
			continue
		}
		if err := ctx.Err(); err != nil {
			return nil, err
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
			f = func(p []uint8) { p[3] = Byte(float64(p[3]) * amt) }
		case e.AlphaRepl != nil:
			a := Byte(float64(e.AlphaRepl.A.Int32()) / 100000 * 255)
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
					p[0], p[1], p[2] = Byte(to.R), Byte(to.G), Byte(to.B)
				}
				p[3] = 255 - p[3]
			}
		case e.Grayscl != nil:
			f = func(p []uint8) {
				y := Byte(Luma(p))
				p[0], p[1], p[2] = y, y, y
			}
		case e.BiLevel != nil:
			t := float64(e.BiLevel.Thresh.Int32()) / 100000 * 255
			f = func(p []uint8) {
				v := uint8(0)
				if Luma(p) >= t {
					v = 255
				}
				p[0], p[1], p[2] = v, v, v
			}
		case e.Duotone != nil:
			cs := e.Duotone.Colors()
			if len(cs) != 2 {
				return nil, fmt.Errorf("%w: duotone colors", ErrInvalid)
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
				t := Luma(p) / 255
				p[0], p[1], p[2] = Byte(dark.R+(light.R-dark.R)*t), Byte(dark.G+(light.G-dark.G)*t), Byte(dark.B+(light.B-dark.B)*t)
			}
		case e.ClrRepl != nil:
			v := e.ClrRepl
			to, err := color(dml.ColorChoice{SrgbClr: v.SrgbClr, SchemeClr: v.SchemeClr, SysClr: v.SysClr, PrstClr: v.PrstClr, HslClr: v.HslClr, ScrgbClr: v.ScRgbClr})
			if err != nil {
				return nil, err
			}
			f = func(p []uint8) {
				p[0], p[1], p[2] = Byte(to.R), Byte(to.G), Byte(to.B)
				p[3] = Byte(float64(p[3]) * to.A)
			}
		case e.ClrChange != nil:
			v := e.ClrChange
			if v.ClrFrom == nil || v.ClrTo == nil {
				return nil, fmt.Errorf("%w: color change", ErrInvalid)
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
			fr, fg, fb := Byte(from.R), Byte(from.G), Byte(from.B)
			f = func(p []uint8) {
				if p[0] != fr || p[1] != fg || p[2] != fb {
					return
				}
				p[0], p[1], p[2] = Byte(to.R), Byte(to.G), Byte(to.B)
				if useA {
					p[3] = Byte(to.A * 255)
				}
			}
		case e.Hsl != nil:
			dh := float64(e.Hsl.Hue) / 60000 / 360
			ds, dl := float64(e.Hsl.Sat.Int32())/100000, float64(e.Hsl.Lum.Int32())/100000
			f = func(p []uint8) {
				h, s, l := toHSL(p)
				h = math.Mod(h+dh+1, 1)
				fromHSL(p, h, math.Min(1, math.Max(0, s+ds)), math.Min(1, math.Max(0, l+dl)))
			}
		case e.Lum != nil:
			if err := colors.Approximate(fmt.Errorf("%w: picture brightness and contrast drawn as LibreOffice does", ErrUnsupported)); err != nil {
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
					p[k] = Byte(float64(p[k])*m + off)
				}
			}
		case e.Tint != nil:
			if err := colors.Approximate(fmt.Errorf("%w: picture tint drawn approximately", ErrUnsupported)); err != nil {
				return nil, err
			}
			// Hue moves the given share of the way to the tint's.
			target := math.Mod(float64(e.Tint.Hue)/60000/360, 1)
			amt := math.Max(-1, math.Min(1, float64(e.Tint.Amt.Int32())/100000))
			f = func(p []uint8) {
				h, s, l := toHSL(p)
				d := math.Mod(target-h+1.5, 1) - 0.5
				fromHSL(p, math.Mod(h+d*amt+1, 1), s, l)
			}
		case e.FillOverlay != nil:
			v := e.FillOverlay
			var fill style.RGBA
			switch {
			case v.SolidFill != nil:
				fill, err = colors.Solid(v.SolidFill)
			case v.GradFill != nil:
				fill, err = colors.Gradient(v.GradFill)
			default:
				return nil, fmt.Errorf("%w: fill overlay without a fill", ErrUnsupported)
			}
			if err != nil {
				return nil, err
			}
			mix, err := blendMode(v.Blend)
			if err != nil {
				return nil, err
			}
			fc := [3]float64{fill.R, fill.G, fill.B}
			f = func(p []uint8) {
				for k := 0; k < 3; k++ {
					s := float64(p[k])
					p[k] = Byte(s + (mix(s, fc[k])-s)*fill.A)
				}
			}
		case e.Blur != nil:
			if err := colors.Approximate(fmt.Errorf("%w: picture blur drawn approximately", ErrUnsupported)); err != nil {
				return nil, err
			}
			if e.Blur.Rad > 0 && emuPerPixel > 0 {
				if err := blurNRGBA(ctx, out, float64(e.Blur.Rad)/emuPerPixel/2); err != nil {
					return nil, err
				}
			}
			continue
		case e.AlphaMod != nil:
			if err := colors.Approximate(fmt.Errorf("%w: picture alpha mask left out", ErrUnsupported)); err != nil {
				return nil, err
			}
			continue
		default:
			if err := colors.Approximate(fmt.Errorf("%w: unknown picture effect left out", ErrUnsupported)); err != nil {
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

// blendMode is a fill overlay's blend of a channel s with the fill's f.
func blendMode(mode string) (func(s, f float64) float64, error) {
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
	return nil, fmt.Errorf("%w: fill overlay blend %q", ErrInvalid, mode)
}

func Byte(v float64) uint8 {
	return uint8(math.Round(math.Min(255, math.Max(0, v))))
}

// Luma is a pixel's Rec. 601 luminance, 0 to 255.
func Luma(p []uint8) float64 {
	return 0.299*float64(p[0]) + 0.587*float64(p[1]) + 0.114*float64(p[2])
}

// toHSL reads a pixel as hue, saturation and lightness, each 0 to 1.
func toHSL(p []uint8) (h, s, l float64) {
	return HSL(style.RGBA{R: float64(p[0]), G: float64(p[1]), B: float64(p[2])})
}

// fromHSL writes hue, saturation and lightness to a pixel's color.
func fromHSL(p []uint8, h, s, l float64) {
	c := FromHSL(h, s, l)
	p[0], p[1], p[2] = Byte(c[0]*255), Byte(c[1]*255), Byte(c[2]*255)
}

// renderBlurNRGBA blurs an image in place with an approximate Gaussian of
// deviation sigma pixels, in premultiplied alpha.
func blurNRGBA(ctx context.Context, img *image.NRGBA, sigma float64) error {
	if ctx == nil {
		ctx = context.Background()
	}
	w, h := img.Rect.Dx(), img.Rect.Dy()
	planes := make([][]float32, 4)
	for k := range planes {
		planes[k] = make([]float32, w*h)
	}
	for i := 0; i < w*h; i++ {
		p := img.Pix[4*i : 4*i+4]
		a := float32(p[3]) / 255
		planes[0][i], planes[1][i], planes[2][i], planes[3][i] = float32(p[0])*a, float32(p[1])*a, float32(p[2])*a, a
	}
	for _, plane := range planes {
		if err := Blur(ctx, plane, w, h, sigma); err != nil {
			return err
		}
	}
	for i := 0; i < w*h; i++ {
		p := img.Pix[4*i : 4*i+4]
		a := planes[3][i]
		if a <= 0 {
			p[0], p[1], p[2], p[3] = 0, 0, 0, 0
			continue
		}
		p[0], p[1], p[2], p[3] = Byte(float64(planes[0][i]/a)), Byte(float64(planes[1][i]/a)), Byte(float64(planes[2][i]/a)), Byte(float64(a)*255)
	}
	return nil
}
