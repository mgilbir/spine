package pptx

import (
	"bytes"
	"context"
	"errors"
	"image/color"
	"regexp"
	"strings"
	"testing"

	"github.com/mgilbir/forme/shape"
	"github.com/mgilbir/spine/render"
)

// renderInheritedText returns a saved deck whose only shape is a 48px square
// text box at (4,4) without insets, and options serving the fixture font for
// every family. Its default text styles are the ones a deck built in code is
// saved with: 18pt theme minor font in tx1.
func renderInheritedText(t *testing.T) ([]byte, render.Options) {
	t.Helper()
	p, _, _, opts := renderTextSlide(t)
	face, err := opts.Fonts(context.Background(), render.FontRequest{Family: "Fixture"})
	if err != nil {
		t.Fatal(err)
	}
	opts.Fonts = func(_ context.Context, r render.FontRequest) (*shape.Face, error) {
		if r.Bold || r.Italic {
			t.Fatalf("request: %+v", r)
		}
		return face, nil
	}
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	return data, opts
}

var renderTxBody = regexp.MustCompile(`<p:txBody>.*</p:txBody>`)

// renderBody replaces the shape's paragraphs, keeping a zero-inset frame.
func renderBody(lstStyle, paragraphs string) func(string) string {
	return func(s string) string {
		return renderTxBody.ReplaceAllLiteralString(s, `<p:txBody><a:bodyPr wrap="square" lIns="0" tIns="0" rIns="0" bIns="0" anchor="t"><a:noAutofit/></a:bodyPr>`+lstStyle+paragraphs+`</p:txBody>`)
	}
}

func renderSlidePNG(t *testing.T, data []byte, opts render.Options, rewrites map[string]func(string) string) []byte {
	t.Helper()
	png, err := renderRewrittenPNG(t, data, opts, rewrites)
	if err != nil {
		t.Fatal(err)
	}
	return png
}

const renderPlainRun = `<a:r><a:rPr lang="en-US" dirty="0"/><a:t>A</a:t></a:r>`

func TestRenderInheritsTextStyles(t *testing.T) {
	data, opts := renderInheritedText(t)
	slide := func(lstStyle, paragraphs string) map[string]func(string) string {
		return map[string]func(string) string{"ppt/slides/slide1.xml": renderBody(lstStyle, paragraphs)}
	}
	inherited := renderSlidePNG(t, data, opts, slide(`<a:lstStyle/>`, `<a:p>`+renderPlainRun+`</a:p>`))
	explicit := renderSlidePNG(t, data, opts, slide(`<a:lstStyle/>`, `<a:p><a:pPr algn="l"><a:lnSpc><a:spcPct val="100000"/></a:lnSpc><a:buNone/></a:pPr><a:r><a:rPr sz="1800" b="0" i="0" u="none" strike="noStrike" kern="1200"><a:solidFill><a:srgbClr val="000000"/></a:solidFill><a:latin typeface="Aptos"/></a:rPr><a:t>A</a:t></a:r></a:p>`))
	if !bytes.Equal(inherited, explicit) {
		t.Fatal("inherited style differs from its explicit equivalent")
	}
	// 18pt: the 24px glyph box spans y 4 to 28.
	if px := renderPixel(t, inherited, 10, 10); px != (color.NRGBA{A: 255}) {
		t.Fatalf("glyph: %+v", px)
	}
	if px := renderPixel(t, inherited, 10, 30); px != (color.NRGBA{R: 255, G: 255, B: 255, A: 255}) {
		t.Fatalf("below glyph: %+v", px)
	}
	for name, lstStyle := range map[string]string{
		"list style":                 `<a:lstStyle><a:lvl1pPr><a:defRPr sz="1200"/></a:lvl1pPr></a:lstStyle>`,
		"list style in another case": `<a:lstStyle><a:lvl1pPr algn="l"><a:defRPr sz="1200"><a:solidFill><a:schemeClr val="tx1"/></a:solidFill></a:defRPr></a:lvl1pPr></a:lstStyle>`,
	} {
		got := renderSlidePNG(t, data, opts, slide(lstStyle, `<a:p>`+renderPlainRun+`</a:p>`))
		want := renderSlidePNG(t, data, opts, slide(`<a:lstStyle/>`, `<a:p><a:r><a:rPr sz="1200"/><a:t>A</a:t></a:r></a:p>`))
		if !bytes.Equal(got, want) {
			t.Fatalf("%s not applied", name)
		}
	}
	// Level 2 of the default style indents by 0.5" (48px), so widen the box.
	// A deck built in code disagrees on level 2 sizes, so set the size.
	sized := `<a:r><a:rPr sz="1200"/><a:t>A</a:t></a:r>`
	wide := func(paragraphs string) map[string]func(string) string {
		return map[string]func(string) string{"ppt/slides/slide1.xml": func(s string) string {
			return strings.Replace(renderBody(`<a:lstStyle/>`, paragraphs)(s), `<a:ext cx="457200" cy="457200"/>`, `<a:ext cx="685800" cy="457200"/>`, 1)
		}}
	}
	levelled := renderSlidePNG(t, data, opts, wide(`<a:p><a:pPr lvl="1"/>`+sized+`</a:p>`))
	margin := renderSlidePNG(t, data, opts, wide(`<a:p><a:pPr marL="457200"/>`+sized+`</a:p>`))
	if !bytes.Equal(levelled, margin) || bytes.Equal(levelled, renderSlidePNG(t, data, opts, wide(`<a:p>`+sized+`</a:p>`))) {
		t.Fatal("paragraph level does not select its list level")
	}
}

func TestRenderParagraphSpacingAndEmptyParagraphs(t *testing.T) {
	data, opts := renderInheritedText(t)
	run := `<a:r><a:rPr sz="1200"/><a:t>A</a:t></a:r>`
	body := renderBody(`<a:lstStyle/>`,
		// 12pt lines are 16px: the first line spans y 4 to 20, an empty 12pt
		// paragraph 20 to 36, 6pt (8px) of space 36 to 44, the third line 44
		// to 60.
		`<a:p>`+run+`</a:p><a:p><a:endParaRPr sz="1200"/></a:p><a:p><a:pPr><a:spcBef><a:spcPts val="600"/></a:spcBef></a:pPr>`+run+`</a:p>`)
	got := renderSlidePNG(t, data, opts, map[string]func(string) string{"ppt/slides/slide1.xml": func(s string) string {
		return strings.Replace(body(s), `<a:ext cx="457200" cy="457200"/>`, `<a:ext cx="457200" cy="609600"/>`, 1)
	}})
	for _, tc := range []struct {
		y    int
		dark bool
	}{{8, true}, {28, false}, {40, false}, {50, true}} {
		px := renderPixel(t, got, 10, tc.y)
		if dark := px.R < 128; dark != tc.dark {
			t.Fatalf("row %d: %+v", tc.y, px)
		}
	}
}

func TestRenderRejectsAmbiguousOrUnsupportedInheritedText(t *testing.T) {
	data, opts := renderInheritedText(t)
	slide := func(lstStyle, paragraphs string) func(string) string { return renderBody(lstStyle, paragraphs) }
	otherSize := func(s string) string {
		return strings.Replace(s, `<p:otherStyle><a:lvl1pPr marL="0" algn="l"><a:defRPr sz="1800"/>`, `<p:otherStyle><a:lvl1pPr marL="0" algn="l"><a:defRPr sz="2400"/>`, 1)
	}
	for name, rewrites := range map[string]map[string]func(string) string{
		// The master's other-text style and the default text style disagree.
		"defaults disagree": {"ppt/slides/slide1.xml": slide(`<a:lstStyle/>`, `<a:p>`+renderPlainRun+`</a:p>`), "ppt/slideMasters/slideMaster1.xml": otherSize},
		// PowerPoint may ignore a paragraph's own default run properties.
		"paragraph default run": {"ppt/slides/slide1.xml": slide(`<a:lstStyle/>`, `<a:p><a:pPr><a:defRPr sz="2400"/></a:pPr>`+renderPlainRun+`</a:p>`)},
		"inherited highlight": {"ppt/slides/slide1.xml": slide(`<a:lstStyle/>`, `<a:p>`+renderPlainRun+`</a:p>`), "ppt/slideMasters/slideMaster1.xml": func(s string) string {
			return strings.Replace(s, `<p:otherStyle><a:lvl1pPr marL="0" algn="l"><a:defRPr sz="1800"/>`, `<p:otherStyle><a:lvl1pPr marL="0" algn="l"><a:defRPr sz="1800"><a:highlight><a:srgbClr val="FFFF00"/></a:highlight></a:defRPr>`, 1)
		}},
		"unknown default style content": {"ppt/slides/slide1.xml": slide(`<a:lstStyle/>`, `<a:p>`+renderPlainRun+`</a:p>`), "ppt/presentation.xml": func(s string) string {
			return strings.Replace(s, `<a:defPPr>`, `<a:defPPr futureAttr="1">`, 1)
		}},
		"space before first paragraph": {"ppt/slides/slide1.xml": slide(`<a:lstStyle/>`, `<a:p><a:pPr><a:spcBef><a:spcPts val="600"/></a:spcBef></a:pPr>`+renderPlainRun+`</a:p>`)},
		"indent":                       {"ppt/slides/slide1.xml": slide(`<a:lstStyle/>`, `<a:p><a:pPr indent="-228600"/>`+renderPlainRun+`</a:p>`)},
		"inherited bullet":             {"ppt/slides/slide1.xml": slide(`<a:lstStyle><a:lvl1pPr><a:buChar char="x"/></a:lvl1pPr></a:lstStyle>`, `<a:p>`+renderPlainRun+`</a:p>`)},
		"small capitals":               {"ppt/slides/slide1.xml": slide(`<a:lstStyle/>`, `<a:p><a:r><a:rPr cap="small"/><a:t>A</a:t></a:r></a:p>`)},
		"invalid level":                {"ppt/slides/slide1.xml": slide(`<a:lstStyle/>`, `<a:p><a:pPr lvl="9"/>`+renderPlainRun+`</a:p>`)},
	} {
		if _, err := renderRewrittenPNG(t, data, opts, rewrites); !errors.Is(err, render.ErrUnsupported) && !errors.Is(err, render.ErrInvalid) {
			t.Fatalf("%s: %v", name, err)
		}
	}
	// Explicit run properties settle what the defaults disagree on.
	if _, err := renderRewrittenPNG(t, data, opts, map[string]func(string) string{
		"ppt/slides/slide1.xml":             slide(`<a:lstStyle/>`, `<a:p><a:r><a:rPr sz="1200"/><a:t>A</a:t></a:r></a:p>`),
		"ppt/slideMasters/slideMaster1.xml": otherSize,
	}); err != nil {
		t.Fatalf("explicit size: %v", err)
	}
	// Style sources are only checked for slides with text.
	p, _, _ := renderTestSlide(t)
	plain, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := renderRewrittenPNG(t, plain, render.Options{}, map[string]func(string) string{"ppt/slideMasters/slideMaster1.xml": func(s string) string {
		return strings.Replace(s, `<p:otherStyle><a:lvl1pPr marL="0" algn="l">`, `<p:otherStyle><a:lvl1pPr marL="0" algn="l" futureAttr="1">`, 1)
	}}); err != nil {
		t.Fatalf("text-free slide: %v", err)
	}
	// A bullet set through the API reaches the resolver, not the source check.
	_, created, _, explicit := renderTextSlide(t)
	created.Shapes()[0].(*AutoShape).TextFrame().Paragraphs()[0].SetBulletChar("x")
	if _, err := created.PrepareRender(context.Background(), explicit); !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("API bullet: %v", err)
	}
}
