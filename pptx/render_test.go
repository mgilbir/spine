package pptx

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"reflect"
	"testing"

	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/render"
)

func renderTestSlide(t *testing.T) (*Presentation, *Slide, *AutoShape) {
	t.Helper()
	p := CreateWithOptions(CreateOptions{Options: Options{SlideSize: SlideSizeCustom}, IncludeDefaultLayouts: true, Width: dml.Pixels(8), Height: dml.Pixels(6)})
	s := p.AddSlideWithLayout(p.GetLayoutByType(LayoutBlank))
	sh := NewAutoShape(PresetRect)
	sh.SetPosition(dml.Pixels(1), dml.Pixels(1))
	sh.SetSize(dml.Pixels(3), dml.Pixels(2))
	sh.SetFill(dml.NewSolidFill(dml.ColorRed))
	sh.SetNoLine()
	if e := s.AddShape(sh); e != nil {
		t.Fatal(e)
	}
	return p, s, sh
}
func TestRenderSlideSnapshotAndSourcePreservation(t *testing.T) {
	p, s, sh := renderTestSlide(t)
	before, e := p.SaveBytes()
	if e != nil {
		t.Fatal(e)
	}
	page, e := s.PrepareRender(context.Background(), render.Options{})
	if e != nil {
		t.Fatal(e)
	}
	after, e := p.SaveBytes()
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(zipParts(t, before), zipParts(t, after)) {
		t.Fatal("render changed saved parts")
	}
	sh.SetFill(dml.NewSolidFill(dml.ColorBlue))
	sh.SetPosition(dml.Pixels(5), dml.Pixels(1))
	var out bytes.Buffer
	if e = page.WritePNG(context.Background(), &out, 96); e != nil {
		t.Fatal(e)
	}
	img, e := png.Decode(&out)
	if e != nil {
		t.Fatal(e)
	}
	for _, tt := range []struct {
		x, y int
		want color.NRGBA
	}{{0, 0, color.NRGBA{R: 255, G: 255, B: 255, A: 255}}, {2, 2, color.NRGBA{R: 255, A: 255}}, {5, 2, color.NRGBA{R: 255, G: 255, B: 255, A: 255}}} {
		if got := color.NRGBAModel.Convert(img.At(tt.x, tt.y)).(color.NRGBA); got != tt.want {
			t.Fatalf("pixel %d,%d: %+v", tt.x, tt.y, got)
		}
	}
	newer, e := s.PrepareRender(context.Background(), render.Options{})
	if e != nil {
		t.Fatal(e)
	}
	out.Reset()
	if e = newer.WritePNG(context.Background(), &out, 96); e != nil {
		t.Fatal(e)
	}
	img, e = png.Decode(&out)
	if e != nil {
		t.Fatal(e)
	}
	if got := color.NRGBAModel.Convert(img.At(6, 2)).(color.NRGBA); got != (color.NRGBA{B: 255, A: 255}) {
		t.Fatalf("unsaved edit: %+v", got)
	}
	opened, e := OpenReader(bytes.NewReader(after), int64(len(after)))
	if e != nil {
		t.Fatal(e)
	}
	defer func() {
		if err := opened.Close(); err != nil {
			t.Error(err)
		}
	}()
	savedBefore, e := opened.SaveBytes()
	if e != nil {
		t.Fatal(e)
	}
	if _, e = opened.Slides()[0].PrepareRender(context.Background(), render.Options{}); e != nil {
		t.Fatal(e)
	}
	savedAfter, e := opened.SaveBytes()
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(zipParts(t, savedBefore), zipParts(t, savedAfter)) {
		t.Fatal("render changed opened parts")
	}
}
func TestRenderSlideImageAndExplicitErrors(t *testing.T) {
	p, s, sh := renderTestSlide(t)
	src := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	src.SetNRGBA(0, 0, color.NRGBA{G: 255, A: 255})
	var encoded bytes.Buffer
	_ = png.Encode(&encoded, src)
	pic := NewPicture()
	pic.SetImageData(encoded.Bytes(), "image/png")
	pic.SetPosition(dml.Pixels(5), dml.Pixels(1))
	pic.SetSize(dml.Pixels(2), dml.Pixels(2))
	if e := s.AddShape(pic); e != nil {
		t.Fatal(e)
	}
	page, e := s.PrepareRender(context.Background(), render.Options{})
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = page.WritePNG(context.Background(), &b, 96); e != nil {
		t.Fatal(e)
	}
	img, e := png.Decode(&b)
	if e != nil {
		t.Fatal(e)
	}
	if got := color.NRGBAModel.Convert(img.At(5, 1)).(color.NRGBA); got != (color.NRGBA{G: 255, A: 255}) {
		t.Fatal(got)
	}
	if _, e = s.PrepareRender(context.Background(), render.Options{Limits: render.Limits{MaxImagePixels: 1}, MaxLayoutNodes: 1}); !errors.Is(e, render.ErrLimit) {
		t.Fatal(e)
	}
	sh.SetShadow(dml.Shadow{Color: dml.ColorBlack, BlurRad: 2})
	if _, e = s.PrepareRender(context.Background(), render.Options{}); !errors.Is(e, render.ErrUnsupported) {
		t.Fatal(e)
	}
	p2, s2, _ := renderTestSlide(t)
	_ = p2
	tb := NewTextBox()
	tb.SetText("unsupported for this profile")
	if e = s2.AddShape(tb); e != nil {
		t.Fatal(e)
	}
	if _, e = s2.PrepareRender(context.Background(), render.Options{}); !errors.Is(e, render.ErrUnsupported) {
		t.Fatal(e)
	}
	_ = p
}

func TestRenderRejectsOriginalUnknownXMLBeforeProjection(t *testing.T) {
	p, _, _ := renderTestSlide(t)
	data, e := p.SaveBytes()
	if e != nil {
		t.Fatal(e)
	}
	for _, tt := range []struct {
		part    string
		rewrite func([]byte) []byte
	}{
		{"ppt/slides/slide1.xml", func(b []byte) []byte {
			return bytes.Replace(b, []byte("<a:xfrm>"), []byte("<a:xfrm unexpected=\"yes\">"), 1)
		}},
		{"ppt/slides/slide1.xml", func(b []byte) []byte {
			return bytes.Replace(b, []byte("</p:spTree>"), []byte("<p:contentPart/></p:spTree>"), 1)
		}},
		{"ppt/slideMasters/slideMaster1.xml", func(b []byte) []byte {
			return bytes.Replace(b, []byte("</p:cSld>"), []byte("<p:unknownVisible/></p:cSld>"), 1)
		}},
	} {
		modified := rewriteZipPart(t, data, tt.part, tt.rewrite)
		if bytes.Equal(zipParts(t, modified)["/"+tt.part], zipParts(t, data)["/"+tt.part]) {
			t.Fatalf("fixture replacement did not apply to %s: %s", tt.part, zipParts(t, data)["/"+tt.part])
		}
		opened, e := OpenReader(bytes.NewReader(modified), int64(len(modified)))
		if e != nil {
			t.Fatal(e)
		}
		_, e = opened.Slides()[0].PrepareRender(context.Background(), render.Options{})
		_ = opened.Close()
		if !errors.Is(e, render.ErrUnsupported) {
			t.Fatalf("%s: %v", tt.part, e)
		}
	}
}
func TestRenderNoLineOverlaysParsedStrokeWithoutSourceMutation(t *testing.T) {
	p, s, sh := renderTestSlide(t)
	sh.SetLine(dml.Line{Width: 2, Color: dml.ColorBlack})
	data, e := p.SaveBytes()
	if e != nil {
		t.Fatal(e)
	}
	opened, e := OpenReader(bytes.NewReader(data), int64(len(data)))
	if e != nil {
		t.Fatal(e)
	}
	defer func() {
		if err := opened.Close(); err != nil {
			t.Error(err)
		}
	}()
	slide := opened.Slides()[0]
	shape := slide.Shapes()[0].(*AutoShape)
	shape.SetNoLine()
	before := slide.sx().CSld.SpTree.Sp[0].SpPr.Ln
	if before.NoFill != nil || before.SolidFill == nil {
		t.Fatal("source was changed by pending overlay")
	}
	if _, e = slide.PrepareRender(context.Background(), render.Options{}); e != nil {
		t.Fatal(e)
	}
	if before.NoFill != nil || before.SolidFill == nil {
		t.Fatal("render mutated source stroke")
	}
	out, e := opened.SaveBytes()
	if e != nil {
		t.Fatal(e)
	}
	part := zipParts(t, out)["/ppt/slides/slide1.xml"]
	if !bytes.Contains(part, []byte("<a:noFill/>")) {
		t.Fatal("no-line edit not saved")
	}
	_ = s
}

func TestRenderBackgroundInheritanceAndEllipse(t *testing.T) {
	p, s, _ := renderTestSlide(t)
	s.layout.master.SetBackgroundFill(dml.NewSolidFill(dml.ColorBlue))
	s.layout.SetBackgroundFill(dml.NewSolidFill(dml.ColorGreen))
	page, e := s.PrepareRender(context.Background(), render.Options{})
	if e != nil {
		t.Fatal(e)
	}
	var b bytes.Buffer
	if e = page.WritePNG(context.Background(), &b, 96); e != nil {
		t.Fatal(e)
	}
	img, e := png.Decode(&b)
	if e != nil {
		t.Fatal(e)
	}
	if got := color.NRGBAModel.Convert(img.At(0, 0)).(color.NRGBA); got != (color.NRGBA{G: 255, A: 255}) {
		t.Fatal(got)
	}
	s.SetBackgroundFill(dml.NewSolidFill(dml.ColorWhite))
	ellipse := NewAutoShape("ellipse")
	ellipse.SetPosition(dml.Pixels(4), dml.Pixels(1))
	ellipse.SetSize(dml.Pixels(4), dml.Pixels(4))
	ellipse.SetFill(dml.NewSolidFill(dml.ColorBlue))
	ellipse.SetNoLine()
	if e = s.AddShape(ellipse); e != nil {
		t.Fatal(e)
	}
	page, e = s.PrepareRender(context.Background(), render.Options{})
	if e != nil {
		t.Fatal(e)
	}
	b.Reset()
	if e = page.WritePNG(context.Background(), &b, 96); e != nil {
		t.Fatal(e)
	}
	img, e = png.Decode(&b)
	if e != nil {
		t.Fatal(e)
	}
	if got := color.NRGBAModel.Convert(img.At(6, 3)).(color.NRGBA); got != (color.NRGBA{B: 255, A: 255}) {
		t.Fatal(got)
	}
	if _, e = s.PrepareRender(context.Background(), render.Options{MaxSourceBytes: -1}); !errors.Is(e, render.ErrInvalid) {
		t.Fatal(e)
	}
	data, e := p.SaveBytes()
	if e != nil {
		t.Fatal(e)
	}
	opened, e := OpenReader(bytes.NewReader(data), int64(len(data)))
	if e != nil {
		t.Fatal(e)
	}
	defer func() {
		if err := opened.Close(); err != nil {
			t.Error(err)
		}
	}()
	if _, e = opened.Slides()[0].PrepareRender(context.Background(), render.Options{MaxSourceBytes: 3}); !errors.Is(e, render.ErrLimit) {
		t.Fatal(e)
	}
}

func BenchmarkRenderStaticSlide(b *testing.B) {
	p := CreateWithOptions(CreateOptions{Options: Options{SlideSize: SlideSizeCustom}, Width: dml.Pixels(320), Height: dml.Pixels(180)})
	s := p.AddSlide()
	sh := NewAutoShape(PresetRect)
	sh.SetSize(dml.Pixels(100), dml.Pixels(50))
	sh.SetFill(dml.NewSolidFill(dml.ColorRed))
	sh.SetNoLine()
	if e := s.AddShape(sh); e != nil {
		b.Fatal(e)
	}
	ctx := context.Background()
	b.ReportAllocs()
	for b.Loop() {
		page, e := s.PrepareRender(ctx, render.Options{})
		if e != nil {
			b.Fatal(e)
		}
		var out bytes.Buffer
		if e = page.WritePNG(ctx, &out, 96); e != nil {
			b.Fatal(e)
		}
	}
}
