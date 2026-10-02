package pptx

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mgilbir/forme/shape"
	"github.com/mgilbir/spine/common/dml"
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
		"scaled autofit": func(s string) string {
			return strings.Replace(s, `<a:noAutofit/>`, `<a:normAutofit fontScale="90000"/>`, 1)
		},
		"two autofits":         func(s string) string { return strings.Replace(s, `<a:noAutofit/>`, `<a:noAutofit/><a:spAutoFit/>`, 1) },
		"outline without fill": func(s string) string { return strings.Replace(s, `<a:ln><a:noFill/></a:ln>`, `<a:ln w="12700"/>`, 1) },
		"style reference": func(s string) string {
			return strings.Replace(s, `</p:spPr><p:txBody>`, `</p:spPr><p:style><a:lnRef idx="2"><a:schemeClr val="accent1"/></a:lnRef><a:fillRef idx="1"><a:schemeClr val="accent1"/></a:fillRef><a:effectRef idx="0"><a:schemeClr val="accent1"/></a:effectRef><a:fontRef idx="minor"><a:schemeClr val="lt1"/></a:fontRef></p:style><p:txBody>`, 1)
		},
		"bottom anchor": func(s string) string { return strings.Replace(s, `anchor="t"`, `anchor="b"`, 1) },
	} {
		if _, err := renderRewrittenPNG(t, data, opts, map[string]func(string) string{"ppt/slides/slide1.xml": rewrite}); !errors.Is(err, render.ErrUnsupported) && !errors.Is(err, render.ErrInvalid) {
			t.Fatalf("%s: %v", name, err)
		}
	}
	// A style held only in the model, which the source check cannot see.
	opened, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if e := opened.Close(); e != nil {
			t.Error(e)
		}
	}()
	slide := opened.Slides()[0]
	slide.sx().CSld.SpTree.Sp[0].Style = &dml.Style{}
	if _, err = slide.PrepareRender(context.Background(), opts); !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("model style: %v", err)
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
	for name, edit := range map[string]func(*AutoShape){
		"margins":       func(sh *AutoShape) { sh.TextFrame().SetMargins(TextMargins{Left: dml.Pixels(8)}) },
		"run text":      func(sh *AutoShape) { sh.TextFrame().Paragraphs()[0].Runs()[0].SetText("A A") },
		"replaced text": func(sh *AutoShape) { sh.TextFrame().SetText("AA") },
	} {
		opened, err := OpenReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			t.Fatal(err)
		}
		slide := opened.Slides()[0]
		edit(slide.Shapes()[0].(*AutoShape))
		page, prepErr := slide.PrepareRender(context.Background(), opts)
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
