package pptxrender

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"strings"
	"testing"

	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/pptx"
	"github.com/mgilbir/spine/render"
)

// renderPictureDeck saves the text fixture with a 10px picture of a green
// image at (60,40) px, returning the deck, options and media part.
func renderPictureDeck(t *testing.T, encoded []byte, mime string) ([]byte, render.Options, string) {
	t.Helper()
	p, s, _, opts := renderTextSlide(t)
	pic := pptx.NewPicture()
	pic.SetImageData(encoded, mime)
	pic.SetPosition(dml.Pixels(60), dml.Pixels(40))
	pic.SetSize(dml.Pixels(10), dml.Pixels(10))
	if err := s.AddShape(pic); err != nil {
		t.Fatal(err)
	}
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	var media string
	for name := range zipParts(t, data) {
		if strings.HasPrefix(name, "/ppt/media/") {
			media = strings.TrimPrefix(name, "/ppt/")
		}
	}
	return data, opts, media
}

func renderGreenPNG(t *testing.T) []byte {
	t.Helper()
	src := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	src.SetNRGBA(0, 0, color.NRGBA{G: 255, A: 255})
	var b bytes.Buffer
	if err := png.Encode(&b, src); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func TestRenderPictureFeatures(t *testing.T) {
	green, white := color.NRGBA{G: 255, A: 255}, color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	data, opts, media := renderPictureDeck(t, renderGreenPNG(t), "image/png")
	pic := func(f func(string) string) map[string]func(string) string {
		return map[string]func(string) string{"ppt/slides/slide1.xml": func(s string) string {
			i := strings.Index(s, "<p:pic>")
			return s[:i] + f(s[i:])
		}}
	}
	// Half opacity over white.
	faded := renderSlidePNG(t, data, opts, pic(func(s string) string {
		return strings.Replace(strings.Replace(s, `/></p:blipFill>`, `><a:alphaModFix amt="50000"/></a:blip></p:blipFill>`, 1), `"/><a:stretch>`, `"><a:alphaModFix amt="50000"/></a:blip><a:stretch>`, 1)
	}))
	if px := renderPixel(t, faded, 65, 45); px.G != 255 || px.R < 120 || px.R > 135 {
		t.Fatalf("faded picture: %+v", px)
	}
	// Hidden pictures are not drawn.
	hidden := renderSlidePNG(t, data, opts, pic(func(s string) string {
		return strings.Replace(s, `<p:cNvPr `, `<p:cNvPr hidden="1" `, 1)
	}))
	if px := renderPixel(t, hidden, 65, 45); px != white {
		t.Fatalf("hidden picture: %+v", px)
	}
	// A picture background stretches over the slide.
	bg := renderSlidePNG(t, data, opts, map[string]func(string) string{
		"ppt/slides/slide1.xml": func(s string) string {
			return strings.Replace(s, `<p:cSld><p:spTree>`, `<p:cSld><p:bg><p:bgPr><a:blipFill><a:blip r:embed="rId99"/><a:stretch><a:fillRect/></a:stretch></a:blipFill><a:effectLst/></p:bgPr></p:bg><p:spTree>`, 1)
		},
		"ppt/slides/_rels/slide1.xml.rels": func(s string) string {
			return strings.Replace(s, `</Relationships>`, `<Relationship Id="rId99" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="../`+media+`"/></Relationships>`, 1)
		},
	})
	if px := renderPixel(t, bg, 1, 58); px != green {
		t.Fatalf("picture background: %+v", px)
	}
	// GIF pictures draw their first frame.
	frame := image.NewPaletted(image.Rect(0, 0, 1, 1), color.Palette{color.NRGBA{G: 255, A: 255}})
	var encoded bytes.Buffer
	if err := gif.Encode(&encoded, frame, nil); err != nil {
		t.Fatal(err)
	}
	data, opts, _ = renderPictureDeck(t, encoded.Bytes(), "image/gif")
	if px := renderPixel(t, renderSlidePNG(t, data, opts, nil), 65, 45); px != green {
		t.Fatalf("GIF picture: %+v", px)
	}
}
