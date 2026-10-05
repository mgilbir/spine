package pptxrender

import (
	"bytes"
	"context"
	"errors"
	"image/color"
	"image/png"
	"math"
	"reflect"
	"strings"
	"testing"

	"github.com/mgilbir/forme/fonttest"
	"github.com/mgilbir/forme/shape"
	"github.com/mgilbir/forme/style"
	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/common/enum"
	core "github.com/mgilbir/spine/internal/render"
	"github.com/mgilbir/spine/pptx"
	"github.com/mgilbir/spine/render"
)

func renderTextSlide(t testing.TB) (*pptx.Presentation, *pptx.Slide, *pptx.Run, render.Options) {
	t.Helper()
	p := pptx.CreateWithOptions(pptx.CreateOptions{Options: pptx.Options{SlideSize: pptx.SlideSizeCustom}, IncludeDefaultLayouts: true, Width: dml.Pixels(80), Height: dml.Pixels(60)})
	s := blankSlide(t, p)
	sh := pptx.NewAutoShape(pptx.PresetRect)
	sh.SetPosition(dml.Pixels(4), dml.Pixels(4))
	sh.SetSize(dml.Pixels(48), dml.Pixels(48))
	sh.SetFill(dml.NewNoFill())
	sh.SetNoLine()
	tf := sh.TextFrame()
	tf.SetMargins(pptx.TextMargins{})
	tf.SetAutofit(pptx.AutofitNone)
	p0 := tf.AddParagraph()
	p0.SetAlignment(enum.TextAlignLeft)
	p0.SetBullet(pptx.BulletNone)
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
	page, err := PrepareSlide(context.Background(), s, opts)
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
	opened, err := pptx.OpenReader(bytes.NewReader(before), int64(len(before)))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if e := opened.Close(); e != nil {
			t.Error(e)
		}
	}()
	if _, err = PrepareSlide(context.Background(), opened.Slides()[0], opts); err != nil {
		t.Fatal(err)
	}
	newPage, err := PrepareSlide(context.Background(), s, opts)
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
	if page, err := PrepareSlide(context.Background(), s, render.Options{}); page != nil || !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("unresolved: %v %v", page, err)
	}
	r.SetUnderline(enum.UnderlineSingle)
	if page, err := PrepareSlide(context.Background(), s, opts); page != nil || !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("underline: %v %v", page, err)
	}
	r.SetUnderline(enum.UnderlineNone)
	opts.Limits.MaxShapeWork = 1
	if page, err := PrepareSlide(context.Background(), s, opts); page != nil || !errors.Is(err, render.ErrLimit) {
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
	opened, err := pptx.OpenReader(bytes.NewReader(modified), int64(len(modified)))
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
	if page, err := PrepareSlide(context.Background(), opened.Slides()[0], opts); page != nil || !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("hidden effect: %v %v", page, err)
	}
}

func TestRenderExplicitTextBoxAndSave(t *testing.T) {
	p, s, r, opts := renderTextSlide(t)
	box := pptx.NewTextBox()
	box.SetPosition(dml.Pixels(4), dml.Pixels(4))
	box.SetSize(dml.Pixels(48), dml.Pixels(48))
	box.SetFill(dml.NewNoFill())
	box.SetNoLine()
	tf := box.TextFrame()
	tf.SetMargins(pptx.TextMargins{})
	tf.SetAutofit(pptx.AutofitNone)
	para := tf.AddParagraph()
	para.SetAlignment(enum.TextAlignLeft)
	para.SetBullet(pptx.BulletNone)
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
	if _, err := PrepareSlide(context.Background(), s, opts); err != nil {
		t.Fatal(err)
	}
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	opened, err := pptx.OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if e := opened.Close(); e != nil {
			t.Error(e)
		}
	}()
	if _, err = PrepareSlide(context.Background(), opened.Slides()[0], opts); err != nil {
		t.Fatal(err)
	}
}

func TestRenderJustifiedText(t *testing.T) {
	p, _, _, opts := renderTextSlide(t)
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	black, white := color.NRGBA{A: 255}, color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	// "A A AA" in the 48px box breaks after "A A "; each A is 16px, a space
	// 8px and a line 16px high, starting at (4,4).
	for _, tc := range []struct {
		align string
		want  map[[2]int]color.NRGBA
	}{
		// The first line's one space widens to put its second A flush
		// right; the last line stays left.
		{"just", map[[2]int]color.NRGBA{{28, 10}: white, {45, 10}: black, {45, 26}: white, {10, 26}: black}},
		{"justLow", map[[2]int]color.NRGBA{{28, 10}: white, {45, 10}: black}},
		// Distributed text spreads every line's character gaps: the first
		// line's two 4px wider, the last line's one 16px wider.
		{"dist", map[[2]int]color.NRGBA{{22, 10}: white, {30, 10}: white, {40, 10}: black, {28, 26}: white, {45, 26}: black}},
	} {
		var warnings []string
		opts.Warn = func(err error) { warnings = append(warnings, err.Error()) }
		got := renderSlidePNG(t, data, opts, map[string]func(string) string{"ppt/slides/slide1.xml": func(s string) string {
			if !strings.Contains(s, `algn="l"`) || !strings.Contains(s, "AA AA") {
				t.Fatalf("slide: %s", s)
			}
			return strings.Replace(strings.Replace(s, `algn="l"`, `algn="`+tc.align+`"`, 1), "AA AA", "A A AA", 1)
		}})
		if len(warnings) != 0 {
			t.Fatalf("%s: %q", tc.align, warnings)
		}
		for at, want := range tc.want {
			if px := renderPixel(t, got, at[0], at[1]); px != want {
				t.Fatalf("%s at %v: %+v, want %+v", tc.align, at, px, want)
			}
		}
	}
}

func TestRenderJustifyGaps(t *testing.T) {
	px := func(v float64) style.Unit { u, _ := style.FromPx(v); return u }
	// Glyphs of 10px at size 10px, one per byte.
	seg := func(text string, x float64) core.RichSegment {
		var glyphs []shape.Glyph
		if text != "\t" {
			for i := range text {
				glyphs = append(glyphs, shape.Glyph{GID: 1, Cluster: i, XAdvance: 1000})
			}
		}
		return core.RichSegment{Text: text, Glyphs: glyphs, Size: px(10), X: px(x), Width: px(10 * float64(len(glyphs)))}
	}
	starts := func(segments []core.RichSegment) []float64 {
		var out []float64
		for _, sg := range segments {
			pen := sg.X.Px()
			for _, g := range sg.Glyphs {
				out = append(out, math.Round(pen))
				pen += g.XAdvance * sg.Size.Px() / 1000
			}
		}
		return out
	}
	for _, tc := range []struct {
		name       string
		segments   []core.RichSegment
		distribute bool
		want       []float64
	}{
		// Two spaces share 30px; the trailing space hangs.
		{"spaces", []core.RichSegment{seg("a b", 0), seg(" c ", 30)}, false, []float64{0, 10, 35, 45, 70, 80}},
		// Only spaces after the last tab widen.
		{"tab", []core.RichSegment{seg("a b", 0), seg("\t", 30), seg("c d", 40)}, false, []float64{0, 10, 20, 40, 50, 70}},
		{"ends in a tab", []core.RichSegment{seg("a b", 0), seg("\t", 30)}, false, []float64{0, 10, 20}},
		{"one word", []core.RichSegment{seg("abc", 0)}, false, []float64{0, 10, 20}},
		{"distributed", []core.RichSegment{seg("abc", 0)}, true, []float64{0, 35, 70}},
	} {
		got := starts(renderJustify(tc.segments, 80, tc.distribute))
		if !reflect.DeepEqual(got, tc.want) {
			t.Errorf("%s: %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestRenderBreaksOverlongWords(t *testing.T) {
	p, _, _, opts := renderTextSlide(t)
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	black, white := color.NRGBA{A: 255}, color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	// Each A is 16px; the 48px box holds three a line, so "AA AAAAA"
	// breaks at the space and then inside the long word: "AA ", "AAA",
	// "AA", each line 16px high from y 4. Strict mode draws it.
	got := renderSlidePNG(t, data, opts, map[string]func(string) string{"ppt/slides/slide1.xml": func(s string) string {
		if !strings.Contains(s, "AA AA") {
			t.Fatalf("slide: %s", s)
		}
		return strings.Replace(s, "AA AA", "AA AAAAA", 1)
	}})
	for at, want := range map[[2]int]color.NRGBA{{30, 10}: black, {45, 10}: white, {45, 26}: black, {30, 42}: black, {45, 42}: white} {
		if px := renderPixel(t, got, at[0], at[1]); px != want {
			t.Fatalf("at %v: %+v, want %+v", at, px, want)
		}
	}
}

func TestRenderTextColumns(t *testing.T) {
	p, _, _, opts := renderTextSlide(t)
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	black, white := color.NRGBA{A: 255}, color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	// The 48px square box at (4,4) holds three 16px lines a column. "A A A
	// A A" in two 24px columns sets one A a line: three down the first
	// column and two down the second, which starts at x 28.
	columns := func(attrs string) map[string]func(string) string {
		return map[string]func(string) string{"ppt/slides/slide1.xml": func(s string) string {
			s = strings.Replace(s, "AA AA", "A A A A A", 1)
			return strings.Replace(s, `<a:bodyPr`, `<a:bodyPr numCol="2"`+attrs, 1)
		}}
	}
	for _, tc := range []struct {
		name, attrs string
		want        map[[2]int]color.NRGBA
	}{
		{"left to right", "", map[[2]int]color.NRGBA{{10, 10}: black, {10, 42}: black, {34, 10}: black, {34, 26}: black, {34, 42}: white}},
		{"right to left", ` rtlCol="1"`, map[[2]int]color.NRGBA{{34, 10}: black, {34, 42}: black, {10, 10}: black, {10, 26}: black, {10, 42}: white}},
	} {
		var warnings []string
		o := opts
		o.Warn = func(err error) { warnings = append(warnings, err.Error()) }
		got := renderSlidePNG(t, data, o, columns(tc.attrs))
		if len(warnings) != 0 {
			t.Fatalf("%s: %q", tc.name, warnings)
		}
		for at, want := range tc.want {
			if px := renderPixel(t, got, at[0], at[1]); px != want {
				t.Fatalf("%s at %v: %+v, want %+v", tc.name, at, px, want)
			}
		}
	}
	// Space between columns wider than the box draws one column, in best
	// effort.
	if _, err := renderRewrittenPNG(t, data, opts, columns(` spcCol="914400"`)); !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("strict wide spacing: %v", err)
	}
	// Text over several columns is anchored at the top.
	centred := columns("")
	inner := centred["ppt/slides/slide1.xml"]
	centred["ppt/slides/slide1.xml"] = func(s string) string {
		s = inner(s)
		if !strings.Contains(s, `anchor="t"`) {
			t.Fatalf("no anchor in %s", s)
		}
		return strings.Replace(s, `anchor="t"`, `anchor="ctr"`, 1)
	}
	if _, err := renderRewrittenPNG(t, data, opts, centred); !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("strict anchored columns: %v", err)
	}
}
