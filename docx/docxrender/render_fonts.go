package docxrender

import (
	"fmt"
	"strconv"

	"github.com/mgilbir/forme/paragraph"
	"github.com/mgilbir/forme/shape"
	"github.com/mgilbir/spine/render"
)

// wordFace is one resolved font request. Layout sees it under the synthetic
// family name css, so document-supplied family names never reach the CSS.
type wordFace struct {
	css  string
	face *shape.Face
	// natural is the face's own line height as a multiple of the font size,
	// computed as forme computes "line-height: normal".
	natural float64
}

// wordFonts resolves the document's font requests through render.Options.Fonts
// and serves them to layout as a layout.FontSet.
type wordFonts struct {
	r     *wordRenderer
	faces map[render.FontRequest]*wordFace
	byCSS map[string]*shape.Face
	// first is the first face resolved, which sets the (zero-sized) strut of
	// the document body.
	first *wordFace
}

func newWordFonts(r *wordRenderer) *wordFonts {
	return &wordFonts{r: r, faces: map[render.FontRequest]*wordFace{}, byCSS: map[string]*shape.Face{}}
}

// get returns the face for a family, weight and slope, asking the resolver at
// most once per distinct request.
func (f *wordFonts) get(family string, bold, italic bool) (*wordFace, error) {
	req := render.FontRequest{Family: family, Bold: bold, Italic: italic}
	if w := f.faces[req]; w != nil {
		return w, nil
	}
	if f.r.opts.Fonts == nil {
		return nil, fmt.Errorf("%w: explicit font resolver required", render.ErrUnsupported)
	}
	if len(f.faces) >= f.r.limits.MaxFonts {
		return nil, render.ErrLimit
	}
	if len(family) == 0 || len(family) > 1024 {
		return nil, fmt.Errorf("%w: font family", render.ErrInvalid)
	}
	face, err := f.r.opts.Fonts(f.r.ctx, req)
	if err != nil {
		return nil, err
	}
	if err = f.r.ctx.Err(); err != nil {
		return nil, err
	}
	if face == nil {
		return nil, fmt.Errorf("%w: unresolved font", render.ErrUnsupported)
	}
	w := &wordFace{css: "f" + strconv.Itoa(len(f.faces)), face: face, natural: 1.2}
	if top, bottom, upem, ok := paragraph.LineMetrics(face); ok {
		gap := 0.0
		if d := face.Descriptor(); d.Has(shape.MetricLineGap) {
			gap = float64(d.LineGap)
		}
		if h := (top - bottom + gap) / upem; h > 0 {
			w.natural = h
		}
	}
	f.faces[req] = w
	f.byCSS[w.css] = face
	if f.first == nil {
		f.first = w
	}
	return w, nil
}

// Face implements layout.FontSet. The family is a synthetic name from get; the
// weight and slope flags are ignored because each synthetic family is already
// one resolved face.
func (f *wordFonts) Face(family string, _, _ bool) (*shape.Face, bool) {
	face, ok := f.byCSS[family]
	return face, ok
}
