package pptxrender

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mgilbir/spine/pptx"
	"github.com/mgilbir/spine/render"
)

// renderRewrittenPNG reopens data after applying part rewrites and renders its
// first slide to PNG.
func renderRewrittenPNG(t *testing.T, data []byte, opts render.Options, rewrites map[string]func(string) string) ([]byte, error) {
	t.Helper()
	data = renderApply(t, data, rewrites)
	opened, err := pptx.OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if e := opened.Close(); e != nil {
			t.Error(e)
		}
	}()
	page, err := PrepareSlide(context.Background(), opened.Slides()[0], opts)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	if err = page.WritePNG(context.Background(), &out, 96); err != nil {
		t.Fatal(err)
	}
	return out.Bytes(), nil
}

// renderApply rewrites package parts, failing when a rewrite changes nothing.
func renderApply(t *testing.T, data []byte, rewrites map[string]func(string) string) []byte {
	t.Helper()
	for part, rewrite := range rewrites {
		before := zipParts(t, data)["/"+part]
		data = rewriteZipPart(t, data, part, func(b []byte) []byte { return []byte(rewrite(string(b))) })
		if bytes.Equal(before, zipParts(t, data)["/"+part]) {
			t.Fatalf("fixture rewrite did not apply to %s", part)
		}
	}
	return data
}

const (
	renderTestCreationID = `<a:extLst><a:ext uri="{FF2B5EF4-FFF2-40B4-BE49-F238E27FC236}"><a16:creationId xmlns:a16="http://schemas.microsoft.com/office/drawing/2014/main" id="{00000000-0000-0000-0000-000000000001}"/></a:ext></a:extLst>`
	renderTestDecorative = `<a:extLst><a:ext uri="{C183D7F6-B498-43B3-948B-1728B52AA6E4}"><adec:decorative xmlns:adec="http://schemas.microsoft.com/office/drawing/2017/decorative" val="1"/></a:ext></a:extLst>`
	renderTestSlideID    = `<p:extLst><p:ext uri="{BB962C8B-B14F-4D97-AF65-F5344CB8AC3E}"><p14:creationId xmlns:p14="http://schemas.microsoft.com/office/powerpoint/2010/main" val="1"/></p:ext></p:extLst>`
)

func TestRenderIgnoresNonVisualMetadata(t *testing.T) {
	p, _, _, opts := renderTextSlide(t)
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	want, err := renderRewrittenPNG(t, data, opts, nil)
	if err != nil {
		t.Fatal(err)
	}
	for name, rewrites := range map[string]map[string]func(string) string{
		"creation ids and run metadata": {"ppt/slides/slide1.xml": func(s string) string {
			s = strings.Replace(s, `<p:cNvPr id="2" name="Shape"/>`, `<p:cNvPr id="2" name="Shape">`+renderTestCreationID+`</p:cNvPr>`, 1)
			s = strings.Replace(s, `</p:spTree></p:cSld>`, `</p:spTree>`+renderTestSlideID+`</p:cSld>`, 1)
			s = strings.Replace(s, `<a:bodyPr wrap="square"`, `<a:bodyPr rtlCol="0" wrap="square"`, 1)
			return strings.Replace(s, `<a:rPr sz="1200"`, `<a:rPr lang="nl-NL" altLang="en-US" dirty="0" err="1" noProof="1" smtClean="0" smtId="4" bmk="b" sz="1200"`, 1)
		}},
		"decorative flag": {"ppt/slides/slide1.xml": func(s string) string {
			return strings.Replace(s, `<p:cNvPr id="2" name="Shape"/>`, `<p:cNvPr id="2" name="Shape">`+renderTestDecorative+`</p:cNvPr>`, 1)
		}},
		"master footer flags and ids": {"ppt/slideMasters/slideMaster1.xml": func(s string) string {
			s = strings.Replace(s, `</p:spTree></p:cSld>`, `</p:spTree>`+strings.ReplaceAll(renderTestSlideID, `val="1"`, `val="2"`)+`</p:cSld>`, 1)
			return strings.Replace(s, `<p:txStyles>`, `<p:hf sldNum="0" hdr="0" ftr="0" dt="0"/><p:txStyles>`, 1)
		}},
	} {
		got, err := renderRewrittenPNG(t, data, opts, rewrites)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("%s changed painted output", name)
		}
	}
}

func TestRenderRejectsVisualOrMisplacedExtensions(t *testing.T) {
	p, _, _, opts := renderTextSlide(t)
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	cNvPr := func(content string) func(string) string {
		return func(s string) string {
			return strings.Replace(s, `<p:cNvPr id="2" name="Shape"/>`, `<p:cNvPr id="2" name="Shape">`+content+`</p:cNvPr>`, 1)
		}
	}
	for name, rewrite := range map[string]func(string) string{
		"unknown uri": cNvPr(`<a:extLst><a:ext uri="{00000000-0000-0000-0000-00000000FFFF}"><x:y xmlns:x="urn:x"/></a:ext></a:extLst>`),
		"uri for another owner": func(s string) string {
			return strings.Replace(s, `</p:spTree></p:cSld>`, `</p:spTree><p:extLst><p:ext uri="{FF2B5EF4-FFF2-40B4-BE49-F238E27FC236}"/></p:extLst></p:cSld>`, 1)
		},
		"wrong list namespace": cNvPr(strings.ReplaceAll(renderTestCreationID, "a:ext", "p:ext")),
		"repeated list":        cNvPr(renderTestCreationID + renderTestCreationID),
		"list attribute":       cNvPr(strings.Replace(renderTestCreationID, `<a:extLst>`, `<a:extLst x="1">`, 1)),
		"extension attribute":  cNvPr(strings.Replace(renderTestCreationID, `<a:ext uri=`, `<a:ext x="1" uri=`, 1)),
		// Skipping an allowed extension must end with its subtree.
		"content after skipped extension": func(s string) string {
			s = cNvPr(renderTestCreationID)(s)
			return strings.Replace(s, `<a:prstGeom prst="rect">`, `<a:prstGeom prst="rect" unexpected="1">`, 1)
		},
		"sibling after skipped extension": cNvPr(strings.Replace(renderTestCreationID, `</a:extLst>`, `</a:extLst><a:hlinkClick r:id=""/>`, 1)),
		"slide footer flags": func(s string) string {
			return strings.Replace(s, `<p:clrMapOvr>`, `<p:hf sldNum="0"/><p:clrMapOvr>`, 1)
		},
		"rtl paragraph": func(s string) string {
			return strings.Replace(s, `<a:pPr algn="l">`, `<a:pPr algn="l" rtl="1">`, 1)
		},
	} {
		if _, err := renderRewrittenPNG(t, data, opts, map[string]func(string) string{"ppt/slides/slide1.xml": rewrite}); !errors.Is(err, render.ErrUnsupported) && !errors.Is(err, render.ErrInvalid) {
			t.Fatalf("%s: %v", name, err)
		}
	}
}
