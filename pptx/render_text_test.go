package pptx

import (
	"bytes"
	"context"
	"errors"
	"image/color"
	"image/png"
	"reflect"
	"testing"

	"github.com/mgilbir/forme/fonttest"
	"github.com/mgilbir/forme/shape"
	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/common/enum"
	"github.com/mgilbir/spine/render"
)

func renderTextSlide(t *testing.T) (*Presentation, *Slide, *Run, render.Options) {
	t.Helper()
	p := CreateWithOptions(CreateOptions{Options: Options{SlideSize: SlideSizeCustom}, IncludeDefaultLayouts: true, Width: dml.Pixels(80), Height: dml.Pixels(60)})
	s := p.AddSlideWithLayout(p.GetLayoutByType(LayoutBlank))
	sh := NewAutoShape(PresetRect)
	sh.SetPosition(dml.Pixels(4), dml.Pixels(4))
	sh.SetSize(dml.Pixels(48), dml.Pixels(48))
	sh.SetFill(dml.NewNoFill())
	sh.SetNoLine()
	tf := sh.TextFrame()
	tf.SetMargins(TextMargins{})
	tf.SetAutofit(AutofitNone)
	p0 := tf.AddParagraph()
	p0.SetAlignment(enum.TextAlignLeft)
	p0.SetBullet(BulletNone)
	p0.SetLineSpacing(100000)
	p0.SetSpaceBefore(0)
	p0.SetSpaceAfter(0)
	r := p0.AddRun()
	r.SetText("AA AA")
	r.SetFont("Fixture")
	r.SetFontSize(12)
	r.SetBold(false)
	r.SetItalic(false)
	r.SetUnderline(enum.UnderlineNone)
	r.SetStrike(enum.StrikeNone)
	r.SetColor(dml.ColorBlack)
	if err := s.AddShape(sh); err != nil {
		t.Fatal(err)
	}
	// A non-1000-em font catches confusion between metrics and glyph advances.
	f, err := shape.Load(fonttest.SFNT(fonttest.SFNTOptions{UnitsPerEm: 2000, Ascent: 1600, Descent: -400, Glyphs: []fonttest.Glyph{{Rune: 'A', Advance: 2000, HasShape: true, Ink: [4]int{0, 0, 2000, 1600}}, {Rune: ' ', Advance: 1000}}}))
	if err != nil {
		t.Fatal(err)
	}
	opts := render.Options{Fonts: func(ctx context.Context, request render.FontRequest) (*shape.Face, error) {
		if request.Family != "Fixture" || request.Bold || request.Italic {
			t.Fatalf("request: %+v", request)
		}
		return f, nil
	}}
	return p, s, r, opts
}

func TestRenderSlideTextPixelsOwnershipAndRoundTrip(t *testing.T) {
	p, s, r, opts := renderTextSlide(t)
	before, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	page, err := s.PrepareRender(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	after, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(zipParts(t, before), zipParts(t, after)) {
		t.Fatal("preparation changed source")
	}
	r.SetText("AA")
	var out bytes.Buffer
	if err = page.WritePNG(context.Background(), &out, 96); err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(&out)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		x, y  int
		black bool
	}{{5, 5, true}, {25, 5, true}, {45, 5, false}, {5, 21, true}, {25, 21, true}, {5, 40, false}} {
		got := color.NRGBAModel.Convert(img.At(tc.x, tc.y)).(color.NRGBA)
		want := color.NRGBA{A: 255}
		if !tc.black {
			want.R = 255
			want.G = 255
			want.B = 255
		}
		if got != want {
			t.Fatalf("%d,%d: %+v != %+v", tc.x, tc.y, got, want)
		}
	}
	opened, err := OpenReader(bytes.NewReader(before), int64(len(before)))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if e := opened.Close(); e != nil {
			t.Error(e)
		}
	}()
	if _, err = opened.Slides()[0].PrepareRender(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
	newPage, err := s.PrepareRender(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	out.Reset()
	if err = newPage.WritePNG(context.Background(), &out, 96); err != nil {
		t.Fatal(err)
	}
	img, err = png.Decode(&out)
	if err != nil {
		t.Fatal(err)
	}
	if got := color.NRGBAModel.Convert(img.At(5, 21)).(color.NRGBA); got.R != 255 {
		t.Fatal("unsaved edit omitted")
	}
}

func TestRenderSlideTextRejectsUnresolvedAndUnsupportedStyles(t *testing.T) {
	_, s, r, opts := renderTextSlide(t)
	if page, err := s.PrepareRender(context.Background(), render.Options{}); page != nil || !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("unresolved: %v %v", page, err)
	}
	r.SetUnderline(enum.UnderlineSingle)
	if page, err := s.PrepareRender(context.Background(), opts); page != nil || !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("underline: %v %v", page, err)
	}
	r.SetUnderline(enum.UnderlineNone)
	opts.Limits.MaxShapeWork = 1
	if page, err := s.PrepareRender(context.Background(), opts); page != nil || !errors.Is(err, render.ErrLimit) {
		t.Fatalf("work: %v %v", page, err)
	}
}

func TestRenderSlideTextRejectsHiddenSourceContentBeforeFontResolution(t *testing.T) {
	p, _, _, opts := renderTextSlide(t)
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	modified := rewriteZipPart(t, data, "ppt/slides/slide1.xml", func(b []byte) []byte {
		return bytes.Replace(b, []byte("</a:rPr>"), []byte("<a:unknownTextEffect/></a:rPr>"), 1)
	})
	if bytes.Equal(zipParts(t, data)["/ppt/slides/slide1.xml"], zipParts(t, modified)["/ppt/slides/slide1.xml"]) {
		t.Fatal("fixture edit did not apply")
	}
	opened, err := OpenReader(bytes.NewReader(modified), int64(len(modified)))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if e := opened.Close(); e != nil {
			t.Error(e)
		}
	}()
	opts.Fonts = func(context.Context, render.FontRequest) (*shape.Face, error) {
		t.Fatal("font resolved before source check")
		return nil, nil
	}
	if page, err := opened.Slides()[0].PrepareRender(context.Background(), opts); page != nil || !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("hidden effect: %v %v", page, err)
	}
}

func TestRenderExplicitTextBoxAndSave(t *testing.T) {
	p, s, r, opts := renderTextSlide(t)
	box := NewTextBox()
	box.SetPosition(dml.Pixels(4), dml.Pixels(4))
	box.SetSize(dml.Pixels(48), dml.Pixels(48))
	box.SetFill(dml.NewNoFill())
	box.SetNoLine()
	tf := box.TextFrame()
	tf.SetMargins(TextMargins{})
	tf.SetAutofit(AutofitNone)
	para := tf.AddParagraph()
	para.SetAlignment(enum.TextAlignLeft)
	para.SetBullet(BulletNone)
	para.SetLineSpacing(100000)
	para.SetSpaceBefore(0)
	para.SetSpaceAfter(0)
	run := para.AddRun()
	run.SetText(r.Text())
	run.SetFont(r.Font())
	run.SetFontSize(12)
	run.SetBold(false)
	run.SetItalic(false)
	run.SetUnderline(enum.UnderlineNone)
	run.SetStrike(enum.StrikeNone)
	run.SetColor(dml.ColorBlack)
	if err := s.AddShape(box); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PrepareRender(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	opened, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if e := opened.Close(); e != nil {
			t.Error(e)
		}
	}()
	if _, err = opened.Slides()[0].PrepareRender(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
}
