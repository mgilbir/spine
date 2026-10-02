package pptx

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"strconv"
	"strings"
	"testing"

	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/render"
)

// renderSquare is a 10px square at (x,y) px with a solid fill.
func renderSquare(x, y int, rgb string) string {
	return `<p:sp><p:nvSpPr><p:cNvPr id="90" name="Square"/><p:cNvSpPr/><p:nvPr/></p:nvSpPr><p:spPr><a:xfrm><a:off x="` +
		strconv.Itoa(x*9525) + `" y="` + strconv.Itoa(y*9525) + `"/><a:ext cx="95250" cy="95250"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom><a:solidFill><a:srgbClr val="` + rgb + `"/></a:solidFill></p:spPr></p:sp>`
}

const (
	renderMasterPart = "ppt/slideMasters/slideMaster1.xml"
	renderLayoutPart = "ppt/slideLayouts/slideLayout6.xml"
)

func renderAddToTree(xml string) func(string) string {
	return func(s string) string { return strings.Replace(s, `</p:spTree>`, xml+`</p:spTree>`, 1) }
}

func TestRenderInheritedShapes(t *testing.T) {
	data, opts := renderInheritedText(t)
	red, blue, white := color.NRGBA{R: 255, A: 255}, color.NRGBA{B: 255, A: 255}, color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	noText := func(s string) string { return renderAnyTxBody.ReplaceAllLiteralString(s, "") }
	// The master's red square lies at (60,4), the layout's blue square at
	// (64,8), over it.
	layers := map[string]func(string) string{
		"ppt/slides/slide1.xml": noText,
		renderMasterPart:        renderAddToTree(renderSquare(60, 4, "FF0000")),
		renderLayoutPart:        renderAddToTree(renderSquare(64, 8, "0000FF")),
	}
	got := renderSlidePNG(t, data, opts, layers)
	for at, want := range map[[2]int]color.NRGBA{{61, 5}: red, {66, 10}: blue, {72, 16}: blue, {61, 16}: white} {
		if px := renderPixel(t, got, at[0], at[1]); px != want {
			t.Fatalf("%v: %+v, want %+v", at, px, want)
		}
	}
	hide := func(root string) func(string) string {
		return func(s string) string { return strings.Replace(s, "<"+root+" ", "<"+root+` showMasterSp="0" `, 1) }
	}
	for name, tc := range map[string]struct {
		part, root   string
		master, mine bool
	}{
		"slide hides both":    {"ppt/slides/slide1.xml", "p:sld", false, false},
		"layout hides master": {renderLayoutPart, "p:sldLayout", false, true},
	} {
		rewrites := map[string]func(string) string{}
		for k, v := range layers {
			rewrites[k] = v
		}
		prev := rewrites[tc.part]
		rewrites[tc.part] = func(s string) string { return hide(tc.root)(prev(s)) }
		got := renderSlidePNG(t, data, opts, rewrites)
		if shown := renderPixel(t, got, 61, 5) == red; shown != tc.master {
			t.Fatalf("%s: master shown %v", name, shown)
		}
		if shown := renderPixel(t, got, 72, 16) == blue; shown != tc.mine {
			t.Fatalf("%s: layout shown %v", name, shown)
		}
	}
	// Unsupported content fails only in a drawn shape; placeholders are never
	// drawn.
	shadow := strings.Replace(renderSquare(60, 4, "FF0000"), `</p:spPr>`, `<a:effectLst><a:outerShdw blurRad="1"><a:srgbClr val="000000"/></a:outerShdw></a:effectLst></p:spPr>`, 1)
	if _, err := renderRewrittenPNG(t, data, opts, map[string]func(string) string{renderMasterPart: renderAddToTree(shadow)}); !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("drawn master shadow: %v", err)
	}
	if _, err := renderRewrittenPNG(t, data, opts, map[string]func(string) string{renderMasterPart: renderAddToTree(shadow), "ppt/slides/slide1.xml": hide("p:sld")}); err != nil {
		t.Fatalf("hidden master shadow: %v", err)
	}
	placeholder := strings.Replace(strings.Replace(shadow, `<p:nvPr/>`, `<p:nvPr><p:ph type="pic" idx="9"/></p:nvPr>`, 1), `<p:cNvSpPr/>`, `<p:cNvSpPr><a:spLocks noGrp="1"/></p:cNvSpPr>`, 1)
	if _, err := renderRewrittenPNG(t, data, opts, map[string]func(string) string{renderMasterPart: renderAddToTree(placeholder)}); err != nil {
		t.Fatalf("master placeholder: %v", err)
	}
	group := `<p:grpSp><p:nvGrpSpPr><p:cNvPr id="95" name="Group"/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr/></p:grpSp>`
	if _, err := renderRewrittenPNG(t, data, opts, map[string]func(string) string{renderLayoutPart: renderAddToTree(group)}); err != nil {
		t.Fatalf("drawn layout group: %v", err)
	}
	if _, err := renderRewrittenPNG(t, data, opts, map[string]func(string) string{renderLayoutPart: renderAddToTree(group), "ppt/slides/slide1.xml": hide("p:sld")}); err != nil {
		t.Fatalf("hidden layout group: %v", err)
	}
}

func TestRenderInheritedPicture(t *testing.T) {
	p, s, _, opts := renderTextSlide(t)
	src := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	src.SetNRGBA(0, 0, color.NRGBA{G: 255, A: 255})
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, src); err != nil {
		t.Fatal(err)
	}
	pic := NewPicture()
	pic.SetImageData(encoded.Bytes(), "image/png")
	pic.SetPosition(dml.Pixels(78), dml.Pixels(58))
	pic.SetSize(dml.Pixels(1), dml.Pixels(1))
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
	if media == "" {
		t.Fatal("no media part")
	}
	masterPic := `<p:pic><p:nvPicPr><p:cNvPr id="91" name="Logo"/><p:cNvPicPr/><p:nvPr/></p:nvPicPr><p:blipFill><a:blip r:embed="rId99"/><a:stretch><a:fillRect/></a:stretch></p:blipFill><p:spPr><a:xfrm><a:off x="571500" y="381000"/><a:ext cx="95250" cy="95250"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr></p:pic>`
	got := renderSlidePNG(t, data, opts, map[string]func(string) string{
		renderMasterPart: renderAddToTree(masterPic),
		"ppt/slideMasters/_rels/slideMaster1.xml.rels": func(s string) string {
			return strings.Replace(s, `</Relationships>`, `<Relationship Id="rId99" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="../`+media+`"/></Relationships>`, 1)
		},
	})
	if px := renderPixel(t, got, 64, 44); px != (color.NRGBA{G: 255, A: 255}) {
		t.Fatalf("master picture: %+v", px)
	}
}

func TestRenderGroups(t *testing.T) {
	data, opts := renderInheritedText(t)
	red, white := color.NRGBA{R: 255, A: 255}, color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	// A group at (60,4) px, 20px square, whose child space is 10px square:
	// its 10px child square at (0,0) doubles to fill the group.
	group := func(xfrm, child string) string {
		return `<p:grpSp><p:nvGrpSpPr><p:cNvPr id="95" name="Group"/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr><a:xfrm` + xfrm + `><a:off x="571500" y="38100"/><a:ext cx="190500" cy="190500"/><a:chOff x="0" y="0"/><a:chExt cx="95250" cy="95250"/></a:xfrm></p:grpSpPr>` + child + `</p:grpSp>`
	}
	slide := func(xml string) map[string]func(string) string {
		return map[string]func(string) string{"ppt/slides/slide1.xml": func(s string) string {
			return renderAddToTree(xml)(renderAnyTxBody.ReplaceAllLiteralString(s, ""))
		}}
	}
	got := renderSlidePNG(t, data, opts, slide(group("", renderSquare(0, 0, "FF0000"))))
	for at, want := range map[[2]int]color.NRGBA{{61, 5}: red, {78, 22}: red, {58, 22}: white} {
		if px := renderPixel(t, got, at[0], at[1]); px != want {
			t.Fatalf("%v: %+v, want %+v", at, px, want)
		}
	}
	// Nested groups compose.
	inner := `<p:grpSp><p:nvGrpSpPr><p:cNvPr id="96" name="Inner"/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="95250" cy="95250"/><a:chOff x="0" y="0"/><a:chExt cx="95250" cy="95250"/></a:xfrm></p:grpSpPr>` + renderSquare(0, 0, "FF0000") + `</p:grpSp>`
	nested := renderSlidePNG(t, data, opts, slide(group("", inner)))
	if px := renderPixel(t, nested, 61, 5); px != red {
		t.Fatalf("nested group: %+v", px)
	}
	if _, err := renderRewrittenPNG(t, data, opts, slide(group(` rot="5400000"`, renderSquare(0, 0, "FF0000")))); !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("rotated group: %v", err)
	}
}
