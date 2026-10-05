package pptxrender

import (
	"context"
	"encoding/xml"
	"image"
	"image/color"
	"strings"
	"testing"

	"github.com/mgilbir/spine/common/dml"
)

// A repeated effect or one this library does not know is kept in the picture
// and reported, never drawn as if absent: strict mode refuses the slide and
// best effort draws the rest and warns.
func TestRenderPictureRawEffects(t *testing.T) {
	data, opts, _ := renderPictureDeck(t, renderGreenPNG(t), "image/png")
	effect := func(x string) map[string]func(string) string {
		return map[string]func(string) string{"ppt/slides/slide1.xml": func(s string) string {
			s = renderAnyTxBody.ReplaceAllLiteralString(s, "")
			i := strings.Index(s, "<p:pic>")
			rest := s[i:]
			j := strings.Index(rest, "<a:blip ")
			k := j + strings.Index(rest[j:], ">")
			if rest[k-1] == '/' {
				return s[:i] + rest[:k-1] + ">" + x + "</a:blip>" + rest[k+1:]
			}
			return s[:i] + rest[:k+1] + x + rest[k+1:]
		}}
	}
	for _, x := range []string{`<a:grayscl/><a:grayscl/>`, `<a:futureEffect/>`, `<a:grayscl/><a:futureEffect/>`} {
		if _, err := renderRewrittenPNG(t, data, opts, effect(x)); err == nil {
			t.Errorf("%s: strict mode drew it", x)
		}
		var warnings []string
		lenient := opts
		lenient.Warn = func(err error) { warnings = append(warnings, err.Error()) }
		if _, err := renderRewrittenPNG(t, data, lenient, effect(x)); err != nil {
			t.Errorf("%s: best effort: %v", x, err)
		}
		if len(warnings) == 0 {
			t.Errorf("%s: best effort did not report it", x)
		}
	}
}

// The model path alone reports a raw effect where it sits in the list.
func TestBlipEffectsReportRawEffect(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	img.SetNRGBA(0, 0, color.NRGBA{200, 100, 50, 255})
	var warnings []string
	colors := &renderColors{ctx: context.Background(), approx: func(err error) { warnings = append(warnings, err.Error()) }}
	effects := []*dml.BlipEffect{{Grayscl: &dml.GrayscaleXML{}}, {RawName: xml.Name{Local: "grayscl"}}}
	if _, err := colors.blipEffects(img, effects, 9525); err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 1 || !strings.Contains(warnings[0], "unknown picture effect left out") {
		t.Fatalf("warnings: %q", warnings)
	}
}
