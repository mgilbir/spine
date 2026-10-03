package pptx

import (
	"bytes"
	"context"
	"errors"
	"image/png"
	"strings"
	"testing"

	"github.com/mgilbir/forme/fonts/notosans"
	"github.com/mgilbir/forme/shape"
	"github.com/mgilbir/spine/render"
)

// renderInkBounds returns the extent of dark pixels: left, top, right and
// bottom, or -1s when there are none.
func renderInkBounds(t *testing.T, data []byte) (int, int, int, int) {
	t.Helper()
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	l, tp, r, b := -1, -1, -1, -1
	bounds := img.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			cr, cg, cb, _ := img.At(x, y).RGBA()
			if cr>>8 < 128 && cg>>8 < 128 && cb>>8 < 128 {
				if l < 0 || x < l {
					l = x
				}
				if tp < 0 {
					tp = y
				}
				r, b = max(r, x), y
			}
		}
	}
	return l, tp, r, b
}

func TestRenderBreaksAndFields(t *testing.T) {
	data, opts := renderInheritedText(t)
	draw := func(bodyPr, paragraphs string) []byte {
		t.Helper()
		rewrite := renderBody(`<a:lstStyle/>`, paragraphs)
		return renderSlidePNG(t, data, opts, map[string]func(string) string{"ppt/slides/slide1.xml": func(s string) string {
			s = rewrite(s)
			if bodyPr != "" {
				s = strings.Replace(s, `wrap="square"`, bodyPr, 1)
			}
			return s
		}})
	}
	_, top, _, oneBottom := renderInkBounds(t, draw("", `<a:p>`+renderPlainRun+`</a:p>`))
	_, _, _, twoBottom := renderInkBounds(t, draw("", `<a:p>`+renderPlainRun+`<a:br/>`+renderPlainRun+`</a:p>`))
	if top < 0 || twoBottom-oneBottom < 15 {
		t.Fatalf("break: one line ends at %d, two at %d", oneBottom, twoBottom)
	}
	// A field shows its saved text like a run.
	field := `<a:p><a:fld id="{B6F15528-21DE-4FAA-801E-634DDDAF4B2B}" type="slidenum"><a:rPr lang="en-US"/><a:t>A</a:t></a:fld></a:p>`
	if !bytes.Equal(draw("", field), draw("", `<a:p>`+renderPlainRun+`</a:p>`)) {
		t.Fatal("field differs from a run with its text")
	}
	// Without wrapping a long line stays one line.
	small := `<a:r><a:rPr lang="en-US" sz="900"/><a:t>A </a:t></a:r>`
	_, _, _, short := renderInkBounds(t, draw("", `<a:p>`+small+`</a:p>`))
	long := `<a:p>` + strings.Repeat(small, 6) + `</a:p>`
	_, _, _, wrapped := renderInkBounds(t, draw("", long))
	_, _, right, unwrapped := renderInkBounds(t, draw(`wrap="none"`, long))
	if unwrapped != short || wrapped <= short || right < 52 {
		t.Fatalf("no wrap: one line ends at %d, wrapped at %d, unwrapped at %d, right %d", short, wrapped, unwrapped, right)
	}
	// Vertical text fails strictly and is drawn horizontally in best effort.
	vert := renderBody(`<a:lstStyle/>`, `<a:p>`+renderPlainRun+`</a:p>`)
	vertical := map[string]func(string) string{"ppt/slides/slide1.xml": func(s string) string {
		return strings.Replace(vert(s), `wrap="square"`, `wrap="square" vert="vert"`, 1)
	}}
	if _, err := renderRewrittenPNG(t, data, opts, vertical); !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("strict vertical text: %v", err)
	}
	var warnings []string
	opts.Warn = func(err error) { warnings = append(warnings, err.Error()) }
	renderSlidePNG(t, data, opts, vertical)
	if len(warnings) != 1 || !strings.Contains(warnings[0], "drawn horizontally") {
		t.Fatalf("vertical text warnings: %q", warnings)
	}
}

func TestRenderTextScalingAndSpacing(t *testing.T) {
	data, opts := renderInheritedText(t)
	draw := func(bodyPr, paragraphs string) []byte {
		t.Helper()
		rewrite := renderBody(`<a:lstStyle/>`, paragraphs)
		return renderSlidePNG(t, data, opts, map[string]func(string) string{"ppt/slides/slide1.xml": func(s string) string {
			s = rewrite(s)
			if bodyPr != "" {
				s = strings.Replace(s, `<a:noAutofit/>`, bodyPr, 1)
			}
			return s
		}})
	}
	plain := `<a:p>` + renderPlainRun + `</a:p>`
	_, top, _, bottom := renderInkBounds(t, draw("", plain))
	_, sTop, _, sBottom := renderInkBounds(t, draw(`<a:normAutofit fontScale="50000"/>`, plain))
	if full, half := bottom-top, sBottom-sTop; half*2 > full+2 || half*2 < full-2 {
		t.Fatalf("font scale: glyph %dpx tall, scaled %dpx", full, half)
	}
	// An exact 40pt line, drawn approximately in best effort, is taller than
	// the 18pt text's natural line, so its glyphs sit lower.
	opts.Warn = func(error) {}
	_, normal, _, _ := renderInkBounds(t, draw("", plain))
	_, exact, _, _ := renderInkBounds(t, draw("", `<a:p><a:pPr><a:lnSpc><a:spcPts val="4000"/></a:lnSpc></a:pPr>`+renderPlainRun+`</a:p>`))
	if exact <= normal+10 {
		t.Fatalf("exact spacing: glyph top at %d, normally %d", exact, normal)
	}
}

func TestRenderTabs(t *testing.T) {
	data, opts := renderInheritedText(t)
	noto, err := notosans.Face()
	if err != nil {
		t.Fatal(err)
	}
	opts.Fonts = func(context.Context, render.FontRequest) (*shape.Face, error) { return noto, nil }
	body := func(pPr string) map[string]func(string) string {
		return map[string]func(string) string{"ppt/slides/slide1.xml": renderBody(`<a:lstStyle/>`, `<a:p>`+pPr+`<a:r><a:rPr lang="en-US" sz="1200"/><a:t>A	A</a:t></a:r></a:p>`)}
	}
	dark := func(r, g, b uint8) bool { return r < 128 && g < 128 && b < 128 }
	// The text starts at x 4; a 0.25" (24px) stop puts the second A at 28.
	got := renderSlidePNG(t, data, opts, body(`<a:pPr defTabSz="228600"/>`))
	if !renderInk(t, got, 28, 4, 38, 24, dark) || renderInk(t, got, 16, 4, 27, 24, dark) {
		t.Fatal("tab stop")
	}
	// Explicit stops are placed at the default spacing, in best effort.
	explicit := body(`<a:pPr defTabSz="228600"><a:tabLst><a:tab pos="457200" algn="l"/></a:tabLst></a:pPr>`)
	if _, err := renderRewrittenPNG(t, data, opts, explicit); !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("strict explicit stops: %v", err)
	}
}
