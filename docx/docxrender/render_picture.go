package docxrender

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"strconv"
	"strings"

	"github.com/mgilbir/forme/style"
	"github.com/mgilbir/spine/common/dml"
	core "github.com/mgilbir/spine/internal/render"
	"github.com/mgilbir/spine/opc"
	"github.com/mgilbir/spine/render"
)

// Pictures.
//
// A picture's pixels are made here, once: the blip's image is read from the
// package (a relationship of the part being translated, never a path or URL
// the document names), decoded once under the image limits, cropped, given its
// colour effects, and resampled into the frame the picture is drawn in, with
// the picture's flips and rotation applied to the pixels. The result is what
// the display list draws.
//
// Layout does not see the pixels. Each picture is an <img> whose source is a
// synthetic reference ("i" and the picture's number); the resolver below serves
// a one-pixel placeholder that carries the number in its colour, so the draw
// operation layout emits identifies the picture and the real raster is put in
// its place when the page's operations are distributed. Nothing the document
// says is ever resolved: the resolver knows only the pictures this renderer
// made, and the placeholder is the same for every document.

// wordMaxPictures bounds the pictures of a document.
const wordMaxPictures = 1 << 14

// wordImageBudget is how many times the page's image limits a document may
// decode and keep in total.
const wordImageBudget = 4

// wordPicture is a drawing the document places.
type wordPicture struct {
	id int
	// w and h are the frame in pixels; ext the effect extent (left, top, right,
	// bottom).
	w, h float64
	ext  [4]float64
	// img holds the frame's pixels, or is nil for a drawing that is not drawn
	// but keeps its space.
	img image.Image
	// bw and bh are the size img covers: the frame's bounding box once
	// rotated.
	bw, bh  float64
	outline *wordOutline
	anchor  *wordAnchor
	// flow says layout places the picture as a float; otherwise an anchored
	// picture is drawn over the page at its computed position.
	flow bool
}

// wordSource is an image part decoded once.
type wordSource struct {
	img image.Image
	// failed is set when the image could not be drawn, after it was reported.
	failed bool
}

// wordRasterKey identifies the pixels of a picture so equal pictures share
// them.
type wordRasterKey struct {
	part    string
	effects string
	geom    [13]float64
}

type wordRaster struct {
	img    image.Image
	bw, bh float64
}

// wordPart is the part whose relationships resolve the pictures being
// translated.
type wordPart struct {
	name string
	rels []*opc.Relationship
}

// rel returns the current part's relationship.
func (r *wordRenderer) rel(id string) *opc.Relationship {
	for _, rel := range r.part.rels {
		if rel != nil && rel.ID == id {
			return rel
		}
	}
	return nil
}

// source loads the image a blip embeds. It returns nil, with the reason
// reported, when the picture cannot be drawn.
// A metafile is drawn at the size w by h CSS pixels of its frame, which sizes
// its raster; the first picture to use one sets it.
func (r *wordRenderer) source(embed, link string, w, h float64) (*wordSource, string, error) {
	if embed == "" {
		if link != "" {
			return nil, "", r.leaveOut("linked pictures")
		}
		return nil, "", r.leaveOut("pictures without an image")
	}
	rel := r.rel(embed)
	switch {
	case rel == nil:
		return nil, "", r.leaveOut("pictures whose image is missing")
	case rel.IsExternal():
		return nil, "", r.leaveOut("linked pictures")
	case rel.Type != wordRelImage:
		return nil, "", r.leaveOut("pictures that do not embed an image")
	}
	part := opc.ResolvePartName(r.part.name, rel.Target)
	if s := r.sources[part]; s != nil {
		if s.failed {
			return nil, part, nil
		}
		return s, part, nil
	}
	if r.doc == nil || r.doc.PartData == nil {
		return nil, part, r.leaveOut("pictures whose image is missing")
	}
	data := r.doc.PartData(part)
	s := &wordSource{}
	r.sources[part] = s
	if len(data) == 0 {
		s.failed = true
		return nil, part, r.leaveOut("pictures whose image is missing")
	}
	lim := r.limits
	bytesLeft := lim.MaxImageBytes*wordImageBudget - r.imgBytes
	pixelsLeft := lim.MaxImagePixels*wordImageBudget - r.imgPixels
	if int64(len(data)) > bytesLeft || pixelsLeft <= 0 {
		return nil, part, fmt.Errorf("%w: document image budget", render.ErrLimit)
	}
	budgetBound := pixelsLeft < lim.MaxImagePixels
	if budgetBound {
		lim.MaxImagePixels = pixelsLeft
	}
	img, err := r.decode(data, lim, w, h)
	if err != nil {
		if cerr := r.ctx.Err(); cerr != nil {
			return nil, part, cerr
		}
		s.failed = true
		if !r.lenient || (budgetBound && errors.Is(err, render.ErrLimit)) {
			return nil, part, err
		}
		what := "pictures with a corrupt image"
		switch {
		case errors.Is(err, render.ErrUnsupported):
			what = "pictures in an image format that is not PNG, JPEG, GIF, EMF or WMF"
		case errors.Is(err, render.ErrLimit):
			what = "pictures too large to decode"
		}
		return nil, part, r.leaveOut(what)
	}
	r.imgBytes += int64(len(data))
	r.imgPixels += int64(img.Bounds().Dx()) * int64(img.Bounds().Dy())
	s.img = img
	return s, part, nil
}

// buildPicture makes the picture of a drawing, or nil when nothing is placed.
func (r *wordRenderer) buildPicture(d *wordDrawing) (*wordPicture, error) {
	if d.w <= 0 || d.h <= 0 {
		return nil, nil
	}
	if len(r.pictures) >= wordMaxPictures {
		return nil, fmt.Errorf("%w: pictures", render.ErrLimit)
	}
	pic := &wordPicture{id: len(r.pictures), w: d.w, h: d.h, ext: d.ext, anchor: d.anchor, bw: d.w, bh: d.h}
	if d.pic != nil {
		var err error
		if err = r.drawPicture(pic, d.pic); err != nil {
			return nil, err
		}
	}
	r.pictures = append(r.pictures, pic)
	return pic, nil
}

// drawPicture fills pic.img from the spec.
func (r *wordRenderer) drawPicture(pic *wordPicture, spec *wordPicSpec) error {
	src, part, err := r.source(spec.embed, spec.link, pic.w, pic.h)
	if err != nil || src == nil {
		return err
	}
	key := wordRasterKey{part: part, effects: spec.effectsKey, geom: [13]float64{pic.w, pic.h,
		spec.src[0], spec.src[1], spec.src[2], spec.src[3], spec.fill[0], spec.fill[1], spec.fill[2], spec.fill[3],
		spec.rot, wordBoolFloat(spec.flipH), wordBoolFloat(spec.flipV)}}
	if got := r.rasters[key]; got != nil {
		pic.img, pic.bw, pic.bh = got.img, got.bw, got.bh
	} else {
		img, bw, bh, err := r.raster(src.img, spec, pic.w, pic.h)
		if err != nil {
			return err
		}
		if img != nil {
			r.rasters[key] = &wordRaster{img, bw, bh}
		}
		pic.img, pic.bw, pic.bh = img, bw, bh
	}
	if spec.outline != nil {
		if math.Mod(spec.rot, 360) != 0 {
			return r.approximate("rotated picture outline not drawn")
		}
		pic.outline = spec.outline
	}
	return nil
}

func wordBoolFloat(b bool) float64 {
	if b {
		return 1
	}
	return 0
}

// raster resamples the source into the picture's frame: cropped by srcRect,
// filled into fillRect, flipped, rotated, with the colour effects applied. It
// returns the pixels and the size of the box they cover, which is the frame's
// bounding box once rotated.
func (r *wordRenderer) raster(src image.Image, spec *wordPicSpec, w, h float64) (image.Image, float64, float64, error) {
	l, t, rr, b := spec.src[0], spec.src[1], spec.src[2], spec.src[3]
	fl, ft, fr, fb := spec.fill[0], spec.fill[1], spec.fill[2], spec.fill[3]
	sw, sh := 1-l-rr, 1-t-b
	fw, fh := 1-fl-fr, 1-ft-fb
	if sw <= 0 || sh <= 0 || fw <= 0 || fh <= 0 {
		return nil, 0, 0, fmt.Errorf("%w: empty picture crop", render.ErrInvalid)
	}
	// The part of the source that exists inside the crop.
	x0, x1 := math.Max(l, 0), math.Min(1-rr, 1)
	y0, y1 := math.Max(t, 0), math.Min(1-b, 1)
	if x1 <= x0 || y1 <= y0 {
		return nil, w, h, nil
	}
	const scale = core.MaxImageScale
	// The size that part is drawn at, in frame pixels.
	dw, dh := w*fw*(x1-x0)/sw, h*fh*(y1-y0)/sh
	img, err := core.Crop(src, x0, y0, 1-x1, 1-y1)
	if err != nil {
		return nil, 0, 0, err
	}
	img = core.Downscale(img, math.Max(dw*scale, 1), math.Max(dh*scale, 1))
	if err = r.ctx.Err(); err != nil {
		return nil, 0, 0, err
	}
	// The work from here on is in the pixels of this image.
	if err = r.chargeRaster(img.Bounds().Dx(), img.Bounds().Dy()); err != nil {
		return nil, 0, 0, err
	}
	if len(spec.effects) > 0 {
		if img, err = r.effects(img, spec, dw*wordEMUPerPixel/float64(img.Bounds().Dx())); err != nil {
			return nil, 0, 0, err
		}
	}
	rot := math.Mod(spec.rot, 360)
	if rot < 0 {
		rot += 360
	}
	if l >= 0 && t >= 0 && rr >= 0 && b >= 0 && fl == 0 && ft == 0 && fr == 0 && fb == 0 && rot == 0 && !spec.flipH && !spec.flipV {
		return img, w, h, nil
	}
	theta := rot * math.Pi / 180
	cos, sin := math.Cos(theta), math.Sin(theta)
	bw := math.Abs(w*cos) + math.Abs(h*sin)
	bh := math.Abs(w*sin) + math.Abs(h*cos)
	// Resolution: that of the source, between one and scale pixels per drawn
	// pixel, and within the image limits.
	cw, ch := float64(img.Bounds().Dx()), float64(img.Bounds().Dy())
	s := math.Min(scale, math.Max(1, math.Max(cw/math.Max(dw, 1e-9), ch/math.Max(dh, 1e-9))))
	if limit := float64(r.limits.MaxImagePixels); bw*bh*s*s > limit {
		s = math.Sqrt(limit / (bw * bh))
	}
	ow, oh := int(math.Ceil(bw*s)), int(math.Ceil(bh*s))
	if ow < 1 {
		ow = 1
	}
	if oh < 1 {
		oh = 1
	}
	if err = r.chargeRaster(ow, oh); err != nil {
		return nil, 0, 0, err
	}
	in := core.NRGBA(img)
	out := image.NewNRGBA(image.Rect(0, 0, ow, oh))
	// Edges that are not on pixel boundaries are antialiased by sampling each
	// output pixel at four points.
	sub := []float64{0}
	if rot != 0 && rot != 90 && rot != 180 && rot != 270 {
		sub = []float64{-0.25, 0.25}
	}
	// frac maps an output sample to the cropped source's pixel position.
	inside := func(px, py float64) (cx, cy float64, ok bool) {
		qx := px*cos + py*sin
		qy := -px*sin + py*cos
		fu, fv := qx/w+0.5, qy/h+0.5
		if spec.flipH {
			fu = 1 - fu
		}
		if spec.flipV {
			fv = 1 - fv
		}
		const eps = 1e-9
		ru, rv := (fu-fl)/fw, (fv-ft)/fh
		if ru < -eps || ru >= 1+eps || rv < -eps || rv >= 1+eps {
			return 0, 0, false
		}
		sx, sy := l+ru*sw, t+rv*sh
		if sx < x0-eps || sx >= x1+eps || sy < y0-eps || sy >= y1+eps {
			return 0, 0, false
		}
		return (sx - x0) / (x1 - x0) * cw, (sy - y0) / (y1 - y0) * ch, true
	}
	iw, ih := in.Rect.Dx(), in.Rect.Dy()
	for y := 0; y < oh; y++ {
		if y&63 == 0 {
			if err = r.ctx.Err(); err != nil {
				return nil, 0, 0, err
			}
		}
		for x := 0; x < ow; x++ {
			var acc [4]float64
			n := 0
			for _, dy := range sub {
				for _, dx := range sub {
					n++
					px := ((float64(x)+0.5+dx)/float64(ow) - 0.5) * bw
					py := ((float64(y)+0.5+dy)/float64(oh) - 0.5) * bh
					cx, cy, ok := inside(px, py)
					if !ok {
						continue
					}
					wordBilinear(in, iw, ih, cx-0.5, cy-0.5, &acc)
				}
			}
			p := out.Pix[y*out.Stride+4*x:]
			if a := acc[3] / float64(n); a > 0 {
				p[0] = core.Byte(acc[0] / float64(n) / a)
				p[1] = core.Byte(acc[1] / float64(n) / a)
				p[2] = core.Byte(acc[2] / float64(n) / a)
				p[3] = core.Byte(a * 255)
			}
		}
	}
	return out, bw, bh, nil
}

// wordBilinear adds the premultiplied bilinear sample of in at (x, y), pixel
// centres at integers, to acc as (r, g, b, a) with a in 0..1 and r, g, b
// scaled by a, in 0..255.
func wordBilinear(in *image.NRGBA, w, h int, x, y float64, acc *[4]float64) {
	x0, y0 := math.Floor(x), math.Floor(y)
	fx, fy := x-x0, y-y0
	ix, iy := int(x0), int(y0)
	for j := 0; j < 2; j++ {
		wy := fy
		if j == 0 {
			wy = 1 - fy
		}
		yy := min(max(iy+j, 0), h-1)
		for i := 0; i < 2; i++ {
			wx := fx
			if i == 0 {
				wx = 1 - fx
			}
			xx := min(max(ix+i, 0), w-1)
			p := in.Pix[yy*in.Stride+4*xx:]
			wt := wx * wy
			a := float64(p[3]) / 255
			acc[0] += wt * float64(p[0]) * a
			acc[1] += wt * float64(p[1]) * a
			acc[2] += wt * float64(p[2]) * a
			acc[3] += wt * a
		}
	}
}

// chargeRaster accounts the pixels of a picture's raster against the
// document's budget.
func (r *wordRenderer) chargeRaster(w, h int) error {
	px := int64(w) * int64(h)
	if px > r.limits.MaxImagePixels || px > r.limits.MaxImagePixels*wordImageBudget-r.rasterPixels {
		return fmt.Errorf("%w: picture pixels", render.ErrLimit)
	}
	r.rasterPixels += px
	return nil
}

// effects applies the picture's colour effects. Effects that cannot be applied
// are reported, and the picture is drawn without them.
func (r *wordRenderer) effects(img image.Image, spec *wordPicSpec, emuPerPixel float64) (image.Image, error) {
	if spec.colorTransforms {
		return img, r.approximate("picture colour effects with colour transforms not applied")
	}
	out, err := core.BlipEffects(r.ctx, wordBlipColors{r}, img, spec.effects, emuPerPixel)
	if err != nil {
		if cerr := r.ctx.Err(); cerr != nil {
			return nil, cerr
		}
		if r.lenient && errors.Is(err, render.ErrUnsupported) {
			return img, r.approximate("picture colour effects not applied")
		}
		return nil, err
	}
	return out, nil
}

// wordBlipColors resolves the colours of picture effects: sRGB, theme and
// system colours without transforms.
type wordBlipColors struct{ r *wordRenderer }

func (c wordBlipColors) Color(ch dml.ColorChoice) (style.RGBA, error) {
	var kind, val, last string
	switch {
	case ch.SrgbClr != nil:
		kind, val = "srgbClr", ch.SrgbClr.Val
	case ch.SchemeClr != nil:
		kind, val = "schemeClr", ch.SchemeClr.Val
	case ch.SysClr != nil:
		kind, last = "sysClr", ch.SysClr.LastClr
	}
	rgb, ok := c.r.drawingColor(kind, val, last)
	if !ok {
		return style.RGBA{}, fmt.Errorf("%w: picture effect colour", render.ErrUnsupported)
	}
	return style.RGBA{R: float64(rgb.r), G: float64(rgb.g), B: float64(rgb.b), A: 1}, nil
}

func (c wordBlipColors) Solid(f *dml.SolidFill) (style.RGBA, error) {
	return c.Color(dml.ColorChoice{SrgbClr: f.SrgbClr, SchemeClr: f.SchemeClr, SysClr: f.SysClr})
}

func (c wordBlipColors) Gradient(*dml.GradFill) (style.RGBA, error) {
	return style.RGBA{}, fmt.Errorf("%w: gradient picture overlay", render.ErrUnsupported)
}

func (c wordBlipColors) Approximate(err error) error {
	return c.r.approximate(strings.TrimPrefix(err.Error(), render.ErrUnsupported.Error()+": "))
}

// wordImages is the resource resolver layout is given. It serves only the
// pictures this renderer made, by the synthetic reference "i" and the picture's
// number, and a placeholder for each: one pixel whose colour is the number.
type wordImages struct {
	r     *wordRenderer
	cache map[int][]byte
}

// Resolve implements layout.ResourceResolver.
func (w *wordImages) Resolve(ref string) ([]byte, error) {
	id, ok := wordImageRef(ref)
	if !ok || id >= len(w.r.pictures) {
		return nil, errors.New("no such picture")
	}
	if b, ok := w.cache[id]; ok {
		return b, nil
	}
	img := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	img.SetNRGBA(0, 0, color.NRGBA{R: uint8(id >> 16), G: uint8(id >> 8), B: uint8(id), A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	w.cache[id] = buf.Bytes()
	return w.cache[id], nil
}

// wordImageRef parses a synthetic picture reference.
func wordImageRef(ref string) (int, bool) {
	rest, ok := strings.CutPrefix(ref, "i")
	if !ok || rest == "" || len(rest) > 8 {
		return 0, false
	}
	n, err := strconv.Atoi(rest)
	if err != nil || n < 0 || n >= 1<<24 || strconv.Itoa(n) != rest {
		return 0, false
	}
	return n, true
}

// wordImageID reads the picture number from a placeholder layout drew.
func wordImageID(img image.Image) (int, bool) {
	if img == nil {
		return 0, false
	}
	b := img.Bounds()
	if b.Dx() != 1 || b.Dy() != 1 {
		return 0, false
	}
	r, g, bl, a := img.At(b.Min.X, b.Min.Y).RGBA()
	if a != 0xffff {
		return 0, false
	}
	return int(r>>8)<<16 | int(g>>8)<<8 | int(bl>>8), true
}
