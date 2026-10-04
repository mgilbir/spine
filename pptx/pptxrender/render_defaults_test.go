package pptxrender

import (
	"bytes"
	"context"
	"errors"
	"image/color"
	"strings"
	"testing"

	"github.com/mgilbir/forme/shape"
	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/pptx"
	"github.com/mgilbir/spine/render"
)

const renderExplicitBody = `<a:bodyPr wrap="square" lIns="0" tIns="0" rIns="0" bIns="0" anchor="t"><a:noAutofit/></a:bodyPr>`

func TestRenderAppliesDrawingMLDefaults(t *testing.T) {
	p, _, _, opts := renderTextSlide(t)
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	want, err := renderRewrittenPNG(t, data, opts, nil)
	if err != nil {
		t.Fatal(err)
	}
	body := func(bodyPr string) func(string) string {
		return func(s string) string { return strings.Replace(s, renderExplicitBody, bodyPr, 1) }
	}
	for name, rewrite := range map[string]func(string) string{
		"absent anchor, wrap and autofit": body(`<a:bodyPr lIns="0" tIns="0" rIns="0" bIns="0"/>`),
		"absent outline and fill": func(s string) string {
			return strings.Replace(s, `<a:noFill/><a:ln><a:noFill/></a:ln></p:spPr>`, `</p:spPr>`, 1)
		},
		// An outline without a fill takes the style's, and there is none.
		"outline without fill": func(s string) string {
			return strings.Replace(s, `<a:ln><a:noFill/></a:ln>`, `<a:ln w="12700"/>`, 1)
		},
		// PowerPoint stores the extent it fitted the text to.
		"shape autofit":   body(`<a:bodyPr wrap="square" lIns="0" tIns="0" rIns="0" bIns="0" anchor="t"><a:spAutoFit/></a:bodyPr>`),
		"unscaled normal": body(`<a:bodyPr wrap="square" lIns="0" tIns="0" rIns="0" bIns="0" anchor="t"><a:normAutofit/></a:bodyPr>`),
	} {
		got, err := renderRewrittenPNG(t, data, opts, map[string]func(string) string{"ppt/slides/slide1.xml": rewrite})
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("%s changed painted output", name)
		}
	}
	// Default insets need a wider frame for the 32px word.
	wide := func(bodyPr string) func(string) string {
		return func(s string) string {
			return strings.Replace(body(bodyPr)(s), `<a:ext cx="457200" cy="457200"/>`, `<a:ext cx="685800" cy="457200"/>`, 1)
		}
	}
	defaultInsets, err := renderRewrittenPNG(t, data, opts, map[string]func(string) string{"ppt/slides/slide1.xml": wide(`<a:bodyPr/>`)})
	if err != nil {
		t.Fatal(err)
	}
	explicitInsets, err := renderRewrittenPNG(t, data, opts, map[string]func(string) string{"ppt/slides/slide1.xml": wide(`<a:bodyPr lIns="91440" tIns="45720" rIns="91440" bIns="45720"/>`)})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(defaultInsets, explicitInsets) || bytes.Equal(defaultInsets, want) {
		t.Fatal("absent insets do not take the DrawingML defaults")
	}
}

func TestRenderTextFrameOverflowAndUnsupportedDefaults(t *testing.T) {
	p, _, _, opts := renderTextSlide(t)
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	// One 12pt line is 16px tall; a 0.1" (9.6px) frame cannot hold it.
	short := func(s string) string {
		return strings.Replace(s, `<a:ext cx="457200" cy="457200"/>`, `<a:ext cx="457200" cy="91440"/>`, 1)
	}
	grown := func(s string) string {
		return strings.Replace(short(s), `<a:noAutofit/>`, `<a:spAutoFit/>`, 1)
	}
	if _, err := renderRewrittenPNG(t, data, opts, map[string]func(string) string{"ppt/slides/slide1.xml": grown}); err != nil {
		t.Fatalf("autofit frame: %v", err)
	}
	for name, rewrite := range map[string]func(string) string{
		"fixed frame overflow": short,
		"two autofits":         func(s string) string { return strings.Replace(s, `<a:noAutofit/>`, `<a:noAutofit/><a:spAutoFit/>`, 1) },
		"visible outline": func(s string) string {
			return strings.Replace(s, `<a:ln><a:noFill/></a:ln>`, `<a:ln w="12700"><a:solidFill><a:srgbClr val="000000"/></a:solidFill></a:ln>`, 1)
		},
		"justified anchor": func(s string) string { return strings.Replace(s, `anchor="t"`, `anchor="just"`, 1) },
	} {
		if _, err := renderRewrittenPNG(t, data, opts, map[string]func(string) string{"ppt/slides/slide1.xml": rewrite}); !errors.Is(err, render.ErrUnsupported) && !errors.Is(err, render.ErrInvalid) {
			t.Fatalf("%s: %v", name, err)
		}
	}
}

// TestRenderEditedTextMatchesSave checks that a preview of pending text edits
// paints what saving and reopening paints.
func TestRenderEditedTextMatchesSave(t *testing.T) {
	p, _, _, opts := renderTextSlide(t)
	// Replaced text inherits the theme font; serve every family the fixture.
	face, err := opts.Fonts(context.Background(), render.FontRequest{Family: "Fixture"})
	if err != nil {
		t.Fatal(err)
	}
	opts.Fonts = func(context.Context, render.FontRequest) (*shape.Face, error) { return face, nil }
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	data = rewriteZipPart(t, data, "ppt/slides/slide1.xml", func(b []byte) []byte {
		return bytes.Replace(b, []byte(renderExplicitBody), []byte(`<a:bodyPr/>`), 1)
	})
	for name, edit := range map[string]func(*pptx.AutoShape){
		"margins":       func(sh *pptx.AutoShape) { sh.TextFrame().SetMargins(pptx.TextMargins{Left: dml.Pixels(8)}) },
		"run text":      func(sh *pptx.AutoShape) { sh.TextFrame().Paragraphs()[0].Runs()[0].SetText("A A") },
		"replaced text": func(sh *pptx.AutoShape) { sh.TextFrame().SetText("AA") },
	} {
		opened, err := pptx.OpenReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			t.Fatal(err)
		}
		slide := opened.Slides()[0]
		edit(slide.Shapes()[0].(*pptx.AutoShape))
		page, prepErr := PrepareSlide(context.Background(), slide, opts)
		saved, err := opened.SaveBytes()
		if err != nil {
			t.Fatal(err)
		}
		_ = opened.Close()
		want, wantErr := renderRewrittenPNG(t, saved, opts, nil)
		if (prepErr == nil) != (wantErr == nil) {
			t.Fatalf("%s: preview %v, saved %v", name, prepErr, wantErr)
		}
		if prepErr != nil {
			continue
		}
		var got bytes.Buffer
		if err = page.WritePNG(context.Background(), &got, 96); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got.Bytes(), want) {
			t.Fatalf("%s: preview differs from saved file", name)
		}
	}
}

func TestRenderAnchorsTextVertically(t *testing.T) {
	data, opts := renderInheritedText(t)
	// One 12pt line is 16px tall in the 48px frame at y 4: top 4-20, middle
	// 20-36, bottom 36-52.
	for anchor, ink := range map[string]int{"t": 12, "ctr": 28, "b": 44} {
		rewrite := func(s string) string {
			s = renderBody(`<a:lstStyle/>`, `<a:p><a:r><a:rPr sz="1200"/><a:t>A</a:t></a:r></a:p>`)(s)
			return strings.Replace(s, `anchor="t"`, `anchor="`+anchor+`"`, 1)
		}
		got := renderSlidePNG(t, data, opts, map[string]func(string) string{"ppt/slides/slide1.xml": rewrite})
		for _, y := range []int{12, 28, 44} {
			if dark := renderPixel(t, got, 10, y).R < 128; dark != (y == ink) {
				t.Fatalf("anchor %s, row %d: dark %v", anchor, y, dark)
			}
		}
	}
	// Text taller than a fixed frame fails whichever way it is anchored.
	tall := func(s string) string {
		s = renderBody(`<a:lstStyle/>`, `<a:p><a:r><a:rPr sz="4800"/><a:t>A</a:t></a:r></a:p>`)(s)
		return strings.Replace(s, `anchor="t"`, `anchor="ctr"`, 1)
	}
	if _, err := renderRewrittenPNG(t, data, opts, map[string]func(string) string{"ppt/slides/slide1.xml": tall}); !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("tall centered text: %v", err)
	}
}

func TestRenderEmptyEffectListsPaintNothing(t *testing.T) {
	p, _, _, opts := renderTextSlide(t)
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	background := `<p:bg><p:bgPr><a:solidFill><a:srgbClr val="FFFFFF"/></a:solidFill></p:bgPr></p:bg>`
	want, err := renderRewrittenPNG(t, data, opts, map[string]func(string) string{"ppt/slides/slide1.xml": func(s string) string {
		return strings.Replace(s, `<p:cSld><p:spTree>`, `<p:cSld>`+background+`<p:spTree>`, 1)
	}})
	if err != nil {
		t.Fatal(err)
	}
	got, err := renderRewrittenPNG(t, data, opts, map[string]func(string) string{"ppt/slides/slide1.xml": func(s string) string {
		// The schema requires effect properties in a background.
		s = strings.Replace(s, `<p:cSld><p:spTree>`, `<p:cSld>`+strings.Replace(background, `</p:bgPr>`, `<a:effectLst/></p:bgPr>`, 1)+`<p:spTree>`, 1)
		s = strings.Replace(s, `<a:ln><a:noFill/></a:ln></p:spPr>`, `<a:ln><a:noFill/></a:ln><a:effectLst/></p:spPr>`, 1)
		return strings.Replace(s, `<a:latin typeface="Fixture"/></a:rPr>`, `<a:effectLst/><a:latin typeface="Fixture"/></a:rPr>`, 1)
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("empty effect lists changed painted output")
	}
	shadow := func(s string) string {
		return strings.Replace(s, `<a:ln><a:noFill/></a:ln></p:spPr>`, `<a:ln><a:noFill/></a:ln><a:effectLst><a:outerShdw blurRad="38100" dist="38100"><a:srgbClr val="000000"/></a:outerShdw></a:effectLst></p:spPr>`, 1)
	}
	if _, err = renderRewrittenPNG(t, data, opts, map[string]func(string) string{"ppt/slides/slide1.xml": shadow}); !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("shadow: %v", err)
	}
}

func TestRenderEmptyTextPaintsNothing(t *testing.T) {
	p, _, _, opts := renderTextSlide(t)
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	// The fixture's resolver accepts only "Fixture", so laying out this empty
	// paragraph would fail; a body without characters is not laid out.
	empty := renderBody(`<a:lstStyle/>`, `<a:p><a:endParaRPr sz="9600"><a:latin typeface="Unavailable"/></a:endParaRPr></a:p>`)
	got, err := renderRewrittenPNG(t, data, opts, map[string]func(string) string{"ppt/slides/slide1.xml": empty})
	if err != nil {
		t.Fatal(err)
	}
	if px := renderPixel(t, got, 10, 10); px != (color.NRGBA{R: 255, G: 255, B: 255, A: 255}) {
		t.Fatalf("empty body painted %+v", px)
	}
	run := `<a:r><a:rPr sz="1200" b="0" i="0"><a:solidFill><a:srgbClr val="000000"/></a:solidFill><a:latin typeface="Fixture"/></a:rPr><a:t>A</a:t></a:r>`
	// A trailing empty 72pt paragraph overflows the 48px frame unseen.
	tail := renderBody(`<a:lstStyle/>`, `<a:p>`+run+`</a:p><a:p><a:endParaRPr sz="7200" b="0" i="0"><a:latin typeface="Fixture"/></a:endParaRPr></a:p>`)
	if _, err = renderRewrittenPNG(t, data, opts, map[string]func(string) string{"ppt/slides/slide1.xml": tail}); err != nil {
		t.Fatalf("empty overflowing line: %v", err)
	}
	// A line break alone makes two empty lines.
	if _, err = renderRewrittenPNG(t, data, opts, map[string]func(string) string{"ppt/slides/slide1.xml": renderBody(`<a:lstStyle/>`, `<a:p><a:br><a:rPr sz="1200" b="0" i="0"><a:latin typeface="Fixture"/></a:rPr></a:br><a:endParaRPr sz="1200" b="0" i="0"><a:latin typeface="Fixture"/></a:endParaRPr></a:p>`)}); err != nil {
		t.Fatalf("line break only: %v", err)
	}
	for name, body := range map[string]string{
		"visible overflow": `<a:p>` + strings.Replace(run, `sz="1200"`, `sz="7200"`, 1) + `</a:p>`,
	} {
		if _, err = renderRewrittenPNG(t, data, opts, map[string]func(string) string{"ppt/slides/slide1.xml": renderBody(`<a:lstStyle/>`, body)}); !errors.Is(err, render.ErrUnsupported) {
			t.Fatalf("%s: %v", name, err)
		}
	}
}
