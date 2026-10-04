package pptx

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"testing"
)

// renderPhoto is a decoded 4000 by 3000 JPEG, as cameras produce.
func renderPhoto(b testing.TB) image.Image {
	b.Helper()
	src := image.NewNRGBA(image.Rect(0, 0, 4000, 3000))
	for y := 0; y < 3000; y++ {
		for x := 0; x < 4000; x++ {
			src.SetNRGBA(x, y, color.NRGBA{R: uint8(x), G: uint8(y), B: uint8(x + y), A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, src, nil); err != nil {
		b.Fatal(err)
	}
	img, err := jpeg.Decode(&buf)
	if err != nil {
		b.Fatal(err)
	}
	return img
}

func BenchmarkRenderDownscalePhoto(b *testing.B) {
	img := renderPhoto(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		renderDownscale(img, 400, 300)
	}
}

func BenchmarkRenderFadePhoto(b *testing.B) {
	img := renderPhoto(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		renderFade(img, 0.5)
	}
}

// renderDownscaleReference is the generic downscale the fast paths match.
func renderDownscaleReference(img image.Image, tw, th int) *image.NRGBA {
	b := img.Bounds()
	out := image.NewNRGBA(image.Rect(0, 0, tw, th))
	for y := 0; y < th; y++ {
		y0, y1 := y*b.Dy()/th, max((y+1)*b.Dy()/th, y*b.Dy()/th+1)
		for x := 0; x < tw; x++ {
			x0, x1 := x*b.Dx()/tw, max((x+1)*b.Dx()/tw, x*b.Dx()/tw+1)
			var r, g, bl, a, n uint64
			for sy := y0; sy < y1; sy++ {
				for sx := x0; sx < x1; sx++ {
					cr, cg, cb, ca := img.At(b.Min.X+sx, b.Min.Y+sy).RGBA()
					r, g, bl, a, n = r+uint64(cr), g+uint64(cg), bl+uint64(cb), a+uint64(ca), n+1
				}
			}
			out.Set(x, y, color.RGBA64{R: uint16(r / n), G: uint16(g / n), B: uint16(bl / n), A: uint16(a / n)})
		}
	}
	return out
}

func TestRenderImageFastPathsMatchGeneric(t *testing.T) {
	const w, h = 37, 23
	pixel := func(x, y int) color.NRGBA {
		return color.NRGBA{R: uint8(x * 7), G: uint8(y * 11), B: uint8(x*y + 3), A: uint8(255 - (x+y)%4*60)}
	}
	nrgba := image.NewNRGBA(image.Rect(0, 0, w, h))
	rgba := image.NewRGBA(image.Rect(0, 0, w, h))
	gray := image.NewGray(image.Rect(0, 0, w, h))
	cmyk := image.NewCMYK(image.Rect(0, 0, w, h))
	pal := image.NewPaletted(image.Rect(0, 0, w, h), color.Palette{color.NRGBA{255, 0, 0, 255}, color.NRGBA{0, 0, 255, 128}, color.Transparent})
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := pixel(x, y)
			nrgba.SetNRGBA(x, y, c)
			rgba.Set(x, y, c)
			gray.Set(x, y, c)
			cmyk.Set(x, y, c)
			pal.SetColorIndex(x, y, uint8((x+y)%3))
		}
	}
	images := map[string]image.Image{"nrgba": nrgba, "rgba": rgba, "gray": gray, "cmyk": cmyk, "paletted": pal,
		"nrgba sub": nrgba.SubImage(image.Rect(3, 2, 30, 20)), "rgba sub": rgba.SubImage(image.Rect(1, 5, 36, 22))}
	for _, ratio := range []image.YCbCrSubsampleRatio{image.YCbCrSubsampleRatio444, image.YCbCrSubsampleRatio422, image.YCbCrSubsampleRatio420} {
		ycc := image.NewYCbCr(image.Rect(0, 0, w, h), ratio)
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				ycc.Y[ycc.YOffset(x, y)] = uint8(x * 6)
				ycc.Cb[ycc.COffset(x, y)] = uint8(y * 9)
				ycc.Cr[ycc.COffset(x, y)] = uint8(x + y*3)
			}
		}
		images[ratio.String()] = ycc
		images[ratio.String()+" sub"] = ycc.SubImage(image.Rect(5, 3, 33, 21))
	}
	for name, img := range images {
		b := img.Bounds()
		ref := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
		for y := 0; y < b.Dy(); y++ {
			for x := 0; x < b.Dx(); x++ {
				ref.Set(x, y, img.At(b.Min.X+x, b.Min.Y+y))
			}
		}
		if got := renderNRGBA(img); !bytes.Equal(got.Pix, ref.Pix) {
			t.Errorf("%s: conversion differs", name)
		}
		for _, size := range [][2]int{{10, 7}, {b.Dx() - 1, b.Dy() / 2}, {1, 1}} {
			got := renderDownscale(img, float64(size[0]), float64(size[1])).(*image.NRGBA)
			if want := renderDownscaleReference(img, size[0], size[1]); !bytes.Equal(got.Pix, want.Pix) {
				t.Errorf("%s at %v: downscale differs", name, size)
			}
		}
	}
}
