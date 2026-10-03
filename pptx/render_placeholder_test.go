package pptx

import (
	"bytes"
	"context"
	"errors"
	"image/color"
	"regexp"
	"strings"
	"testing"

	"github.com/mgilbir/forme/fonts/notosans"
	"github.com/mgilbir/forme/shape"
	"github.com/mgilbir/spine/render"
)

// renderPlaceholderSp is a placeholder shape with properties and paragraphs.
func renderPlaceholderSp(ph, spPr, body string) string {
	return `<p:sp><p:nvSpPr><p:cNvPr id="2" name="Placeholder"/><p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr><p:nvPr>` + ph + `</p:nvPr></p:nvSpPr><p:spPr>` + spPr + `</p:spPr><p:txBody><a:bodyPr/><a:lstStyle/>` + body + `</p:txBody></p:sp>`
}

var renderSpTreeShapes = regexp.MustCompile(`(<p:spTree>.*?</p:grpSpPr>).*(</p:spTree>)`)

// renderTitleDeck rewrites the text fixture so its slide holds a title
// placeholder inheriting its geometry from the layout, which places it at
// (4,4) px, 48px wide and 30px tall, and its text style from a PowerPoint-like
// master title style: 12pt red text in the theme's major font.
func renderTitleDeck(t *testing.T, slideTitle string) ([]byte, render.Options, map[string]func(string) string) {
	t.Helper()
	data, opts := renderInheritedText(t)
	title := `<p:ph type="title"/>`
	rewrites := map[string]func(string) string{
		"ppt/slides/slide1.xml": func(s string) string {
			return renderSpTreeShapes.ReplaceAllString(s, `${1}`+slideTitle+`${2}`)
		},
		renderLayoutPart: renderAddToTree(renderPlaceholderSp(title, `<a:xfrm><a:off x="38100" y="38100"/><a:ext cx="457200" cy="285750"/></a:xfrm>`,
			`<a:p><a:r><a:rPr lang="en-US"/><a:t>Click to edit Master title style</a:t></a:r></a:p>`)),
		renderMasterPart: func(s string) string {
			return regexp.MustCompile(`<p:titleStyle>.*?</p:titleStyle>`).ReplaceAllLiteralString(s, `<p:titleStyle><a:lvl1pPr algn="l" defTabSz="914400" rtl="0" eaLnBrk="1" latinLnBrk="0" hangingPunct="1"><a:lnSpc><a:spcPct val="100000"/></a:lnSpc><a:spcBef><a:spcPct val="0"/></a:spcBef><a:buNone/><a:defRPr sz="1200" kern="1200"><a:solidFill><a:srgbClr val="FF0000"/></a:solidFill><a:latin typeface="+mj-lt"/><a:ea typeface="+mj-ea"/><a:cs typeface="+mj-cs"/></a:defRPr></a:lvl1pPr></p:titleStyle>`)
		},
	}
	return data, opts, rewrites
}

func TestRenderPlaceholderText(t *testing.T) {
	slideTitle := renderPlaceholderSp(`<p:ph type="title"/>`, ``, `<a:p><a:r><a:rPr lang="en-US" dirty="0"/><a:t>A</a:t></a:r></a:p>`)
	data, opts, rewrites := renderTitleDeck(t, slideTitle)
	got, err := renderRewrittenPNG(t, data, opts, rewrites)
	if err != nil {
		t.Fatal(err)
	}
	// Default insets put the 16px glyph at x 13.6-29.6, y 8.8-21.6.
	red := color.NRGBA{R: 255, A: 255}
	for at, want := range map[[2]int]color.NRGBA{{20, 15}: red, {40, 15}: {R: 255, G: 255, B: 255, A: 255}} {
		if px := renderPixel(t, got, at[0], at[1]); px != want {
			t.Fatalf("%v: %+v, want %+v", at, px, want)
		}
	}
	// The slide's own geometry and body properties win.
	moved := renderPlaceholderSp(`<p:ph type="title"/>`, `<a:xfrm><a:off x="285750" y="285750"/><a:ext cx="381000" cy="285750"/></a:xfrm>`, `<a:p><a:r><a:rPr lang="en-US"/><a:t>A</a:t></a:r></a:p>`)
	moved = strings.Replace(moved, `<a:bodyPr/>`, `<a:bodyPr lIns="0" tIns="0"/>`, 1)
	data, opts, rewrites = renderTitleDeck(t, moved)
	got, err = renderRewrittenPNG(t, data, opts, rewrites)
	if err != nil {
		t.Fatal(err)
	}
	if px := renderPixel(t, got, 32, 35); px != red {
		t.Fatalf("moved title: %+v", px)
	}
	if px := renderPixel(t, got, 20, 15); px == red {
		t.Fatal("title drawn at the layout position")
	}
}

func TestRenderEditedPlaceholderMatchesSave(t *testing.T) {
	slideTitle := renderPlaceholderSp(`<p:ph type="title"/>`, ``, `<a:p><a:r><a:rPr lang="en-US"/><a:t>A</a:t></a:r></a:p>`)
	data, opts, rewrites := renderTitleDeck(t, slideTitle)
	data = renderApply(t, data, rewrites)
	opened, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	slide := opened.Slides()[0]
	slide.Shapes()[0].(*PlaceholderShape).TextFrame().Paragraphs()[0].Runs()[0].SetText(" A")
	page, err := slide.PrepareRender(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := opened.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	_ = opened.Close()
	want, err := renderRewrittenPNG(t, saved, opts, nil)
	if err != nil {
		t.Fatal(err)
	}
	var got bytes.Buffer
	if err = page.WritePNG(context.Background(), &got, 96); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got.Bytes(), want) {
		t.Fatal("edited placeholder preview differs from the saved file")
	}
}

func TestRenderRejectsUnsupportedPlaceholders(t *testing.T) {
	body := `<a:p><a:r><a:rPr lang="en-US"/><a:t>A</a:t></a:r></a:p>`
	for name, tc := range map[string]struct {
		slide  string
		extend func(map[string]func(string) string)
	}{
		"without geometry": {slide: renderPlaceholderSp(`<p:ph type="hdr" idx="7"/>`, ``, body)},
		"ambiguous layout match": {slide: renderPlaceholderSp(`<p:ph type="title"/>`, ``, body), extend: func(r map[string]func(string) string) {
			prev := r[renderLayoutPart]
			r[renderLayoutPart] = func(s string) string {
				return renderAddToTree(renderPlaceholderSp(`<p:ph type="title"/>`, ``, ``))(prev(s))
			}
		}},
		"inherited effect": {slide: renderPlaceholderSp(`<p:ph type="title"/>`, ``, body), extend: func(r map[string]func(string) string) {
			r[renderLayoutPart] = renderAddToTree(renderPlaceholderSp(`<p:ph type="title"/>`, `<a:xfrm><a:off x="38100" y="38100"/><a:ext cx="457200" cy="190500"/></a:xfrm><a:effectLst><a:glow rad="1"><a:srgbClr val="000000"/></a:glow></a:effectLst>`, ``))
		}},
	} {
		data, opts, rewrites := renderTitleDeck(t, tc.slide)
		if tc.extend != nil {
			tc.extend(rewrites)
		}
		if _, err := renderRewrittenPNG(t, data, opts, rewrites); !errors.Is(err, render.ErrUnsupported) && !errors.Is(err, render.ErrInvalid) {
			t.Fatalf("%s: %v", name, err)
		}
	}
}

func TestRenderBodyPlaceholderBullets(t *testing.T) {
	body := renderPlaceholderSp(`<p:ph idx="1"/>`, `<a:xfrm><a:off x="38100" y="38100"/><a:ext cx="457200" cy="457200"/></a:xfrm>`, `<a:p><a:r><a:rPr lang="en-US"/><a:t>A</a:t></a:r></a:p>`)
	body = strings.Replace(body, `<a:bodyPr/>`, `<a:bodyPr lIns="0" tIns="0" rIns="0" bIns="0"/>`, 1)
	data, _, rewrites := renderTitleDeck(t, body)
	prev := rewrites[renderMasterPart]
	rewrites[renderMasterPart] = func(s string) string {
		return regexp.MustCompile(`<p:bodyStyle>.*?</p:bodyStyle>`).ReplaceAllLiteralString(prev(s), `<p:bodyStyle><a:lvl1pPr marL="228600" indent="-228600" algn="l"><a:lnSpc><a:spcPct val="90000"/></a:lnSpc><a:spcBef><a:spcPts val="1000"/></a:spcBef><a:buFont typeface="Arial"/><a:buChar char="•"/><a:defRPr sz="1200" kern="1200"><a:solidFill><a:srgbClr val="000000"/></a:solidFill><a:latin typeface="+mn-lt"/></a:defRPr></a:lvl1pPr></p:bodyStyle>`)
	}
	noto, err := notosans.Face()
	if err != nil {
		t.Fatal(err)
	}
	opts := render.Options{Fonts: func(context.Context, render.FontRequest) (*shape.Face, error) { return noto, nil }}
	// Space before the first paragraph is undocumented; the slide sets none.
	slide := rewrites["ppt/slides/slide1.xml"]
	rewrites["ppt/slides/slide1.xml"] = func(s string) string {
		return strings.Replace(slide(s), `<a:p><a:r>`, `<a:p><a:pPr><a:spcBef><a:spcPts val="0"/></a:spcBef></a:pPr><a:r>`, 1)
	}
	got, err := renderRewrittenPNG(t, data, opts, rewrites)
	if err != nil {
		t.Fatal(err)
	}
	dark := func(r, g, b uint8) bool { return r < 128 && g < 128 && b < 128 }
	// The placeholder's text starts at x 4: the bullet hangs at 4-28.
	if !renderInk(t, got, 4, 4, 16, 24, dark) || !renderInk(t, got, 28, 4, 44, 24, dark) {
		t.Fatal("bulleted body text missing")
	}
}
