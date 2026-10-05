package docxrender

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"strings"
	"testing"

	"github.com/mgilbir/forme/layout"
	"github.com/mgilbir/spine/render"
)

// Pictures are built from raw WordprocessingML around small generated PNGs. A
// picture is related under the test kit's id for its part.

var wordTestEmbed = wordTestRID("word/media/image1.png")

var (
	wordTestRed   = color.NRGBA{R: 255, A: 255}
	wordTestBlue  = color.NRGBA{B: 255, A: 255}
	wordTestGreen = color.NRGBA{G: 255, A: 255}
)

// wordTestImage encodes a w by h PNG whose pixel colours f gives.
func wordTestImage(w, h int, f func(x, y int) color.NRGBA) []byte {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetNRGBA(x, y, f(x, y))
		}
	}
	var b bytes.Buffer
	if err := png.Encode(&b, img); err != nil {
		panic(err)
	}
	return b.Bytes()
}

func wordTestSolid(w, h int, c color.NRGBA) []byte {
	return wordTestImage(w, h, func(int, int) color.NRGBA { return c })
}

// wordTestHalves is red on the left half and blue on the right.
func wordTestHalves(w, h int) []byte {
	return wordTestImage(w, h, func(x, _ int) color.NRGBA {
		if x < w/2 {
			return wordTestRed
		}
		return wordTestBlue
	})
}

func wordTestMedia(data []byte) wordTestParts {
	return wordTestParts{styles: wordTestStyles, media: map[string][]byte{"word/media/image1.png": data}}
}

// wordTestPic describes a drawing.
type wordTestPic struct {
	// w and h are the frame in pixels.
	w, h float64
	// embed is the relationship, wordTestEmbed when empty.
	embed string
	// ext is wp:effectExtent in pixels (left, top, right, bottom).
	ext [4]float64
	// blip holds a:blip's children, src the attributes of a:srcRect, fill those
	// of a:fillRect, xfrm those of a:xfrm, sp more children of pic:spPr, and
	// graphic replaces the picture's a:graphicData content.
	blip, src, fill, xfrm, sp string
	graphicURI                string
	docPr                     string
}

func emu(px float64) int64 { return int64(math.Round(px * 9525)) }

const wordTestDrawingNS = `xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture"`

func (p wordTestPic) graphic() string {
	embed := p.embed
	if embed == "" {
		embed = wordTestEmbed
	}
	uri := p.graphicURI
	if uri == "" {
		uri = "http://schemas.openxmlformats.org/drawingml/2006/picture"
	}
	var sb strings.Builder
	sb.WriteString(`<a:graphic><a:graphicData uri="` + uri + `">`)
	if uri == "http://schemas.openxmlformats.org/drawingml/2006/picture" {
		sb.WriteString(`<pic:pic><pic:nvPicPr><pic:cNvPr id="0" name="p"/><pic:cNvPicPr/></pic:nvPicPr><pic:blipFill>`)
		sb.WriteString(`<a:blip r:embed="` + embed + `">` + p.blip + `</a:blip>`)
		if p.src != "" {
			sb.WriteString(`<a:srcRect ` + p.src + `/>`)
		}
		if p.fill != "" {
			sb.WriteString(`<a:stretch><a:fillRect ` + p.fill + `/></a:stretch>`)
		} else {
			sb.WriteString(`<a:stretch><a:fillRect/></a:stretch>`)
		}
		sb.WriteString(`</pic:blipFill><pic:spPr><a:xfrm ` + p.xfrm + `><a:off x="0" y="0"/><a:ext cx="` + fmt.Sprint(emu(p.w)) + `" cy="` + fmt.Sprint(emu(p.h)) + `"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom>` + p.sp + `</pic:spPr></pic:pic>`)
	}
	sb.WriteString(`</a:graphicData></a:graphic>`)
	return sb.String()
}

func (p wordTestPic) common() string {
	return fmt.Sprintf(`<wp:extent cx="%d" cy="%d"/><wp:effectExtent l="%d" t="%d" r="%d" b="%d"/><wp:docPr id="1" name="p" %s/>`,
		emu(p.w), emu(p.h), emu(p.ext[0]), emu(p.ext[1]), emu(p.ext[2]), emu(p.ext[3]), p.docPr)
}

// inline is a run holding the picture inline.
func (p wordTestPic) inline() string {
	return `<w:r><w:drawing ` + wordTestDrawingNS + `><wp:inline distT="0" distB="0" distL="0" distR="0">` + p.common() + p.graphic() + `</wp:inline></w:drawing></w:r>`
}

// wordTestAnchor describes where an anchored picture goes.
type wordTestAnchor struct {
	// attrs are more attributes of wp:anchor (behindDoc, simplePos...), dist
	// the wrap distances in pixels (top, bottom, left, right).
	attrs string
	dist  [4]float64
	// hFrom, hVal, vFrom and vVal are the positions: a value in pixels, or an
	// alignment word.
	hFrom, h string
	vFrom, v string
	wrap     string
}

func wordTestPos(tag, from, v string) string {
	if from == "" {
		from = map[string]string{"positionH": "column", "positionV": "paragraph"}[tag]
	}
	inner := ""
	if _, err := fmt.Sscanf(v, "%d", new(int)); err == nil {
		var px float64
		_, _ = fmt.Sscanf(v, "%f", &px)
		inner = fmt.Sprintf(`<wp:posOffset>%d</wp:posOffset>`, emu(px))
	} else if v != "" {
		inner = `<wp:align>` + v + `</wp:align>`
	} else {
		inner = `<wp:posOffset>0</wp:posOffset>`
	}
	return `<wp:` + tag + ` relativeFrom="` + from + `">` + inner + `</wp:` + tag + `>`
}

func (p wordTestPic) anchored(a wordTestAnchor) string {
	wrap := a.wrap
	if wrap == "" {
		wrap = `<wp:wrapNone/>`
	}
	attrs := fmt.Sprintf(`distT="%d" distB="%d" distL="%d" distR="%d" `, emu(a.dist[0]), emu(a.dist[1]), emu(a.dist[2]), emu(a.dist[3]))
	if !strings.Contains(a.attrs, "behindDoc") {
		attrs += `behindDoc="0" `
	}
	if !strings.Contains(a.attrs, "simplePos") {
		attrs += `simplePos="0" `
	}
	return `<w:r><w:drawing ` + wordTestDrawingNS + `><wp:anchor ` + attrs + `relativeHeight="1" locked="0" layoutInCell="1" allowOverlap="1" ` + a.attrs + `>` +
		`<wp:simplePos x="0" y="0"/>` + wordTestPos("positionH", a.hFrom, a.h) + wordTestPos("positionV", a.vFrom, a.v) +
		p.common() + wrap + p.graphic() + `</wp:anchor></w:drawing></w:r>`
}

// wordTestPicOps are a page's picture operations in paint order.
func (p *Pages) pictures(page int) []layout.DrawImage {
	var out []layout.DrawImage
	for _, op := range p.pages[page-1].ops {
		if v, ok := op.(layout.DrawImage); ok {
			out = append(out, v)
		}
	}
	return out
}

func (p *Pages) pixels(t testing.TB, page int) image.Image {
	t.Helper()
	pg, err := p.Page(context.Background(), page)
	if err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err = pg.WritePNG(context.Background(), &out, 96); err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(&out)
	if err != nil {
		t.Fatal(err)
	}
	return img
}

func nrgba(img image.Image, x, y int) color.NRGBA {
	return color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
}

func rectPx(v layout.DrawImage) [4]float64 {
	return [4]float64{v.Rect.X.Px(), v.Rect.Y.Px(), v.Rect.W.Px(), v.Rect.H.Px()}
}

func sameRect(a, b [4]float64) bool {
	for i := range a {
		if math.Abs(a[i]-b[i]) > 0.05 {
			return false
		}
	}
	return true
}

func near3(c color.NRGBA, want color.NRGBA, tol int) bool {
	d := func(a, b uint8) bool { return int(a)-int(b) <= tol && int(b)-int(a) <= tol }
	return d(c.R, want.R) && d(c.G, want.G) && d(c.B, want.B) && c.A == want.A
}

func TestInlinePictureSitsOnTheBaselineAndGrowsTheLine(t *testing.T) {
	pic := wordTestPic{w: 40, h: 30}
	body := wordTestBody(wordTestPara("", wordTestRun("", "ab"), pic.inline(), wordTestRun("", "cd")), wordTestPara("", wordTestRun("", "next")))
	p, _ := wordTestRender(t, body, wordTestMedia(wordTestSolid(8, 6, wordTestRed)))
	ims := p.pictures(1)
	if len(ims) != 1 {
		t.Fatalf("%d pictures", len(ims))
	}
	// "ab" is 12 wide; the line is the picture's 30 above the baseline plus
	// the 2.4 the face descends.
	if got := rectPx(ims[0]); !sameRect(got, [4]float64{wordTestLeft + 12, wordTestTop, 40, 30}) {
		t.Errorf("rect %v", got)
	}
	lines := p.lines(1)
	if len(lines) != 2 {
		t.Fatalf("%+v", lines)
	}
	if !near(lines[0].y, wordTestTop+30) || !near(lines[1].y, wordTestTop+32.4+wordTestAscent12) {
		t.Errorf("baselines %v and %v", lines[0].y, lines[1].y)
	}
	// The text after the picture follows it on the line.
	if !strings.Contains(lines[0].text, "ab") || !strings.Contains(lines[0].text, "cd") {
		t.Errorf("line %q", lines[0].text)
	}
	img := p.pixels(t, 1)
	if got := nrgba(img, 50, 35); got != wordTestRed {
		t.Errorf("inside the picture %+v", got)
	}
	if got := nrgba(img, 80, 35); got != (color.NRGBA{255, 255, 255, 255}) {
		t.Errorf("beside the picture %+v", got)
	}
}

func TestPictureOnlyParagraph(t *testing.T) {
	pic := wordTestPic{w: 40, h: 30}
	p, _ := wordTestRender(t, wordTestBody(wordTestPara("", pic.inline()), wordTestPara("", wordTestRun("", "next"))), wordTestMedia(wordTestSolid(4, 4, wordTestGreen)))
	ims := p.pictures(1)
	if len(ims) != 1 || !sameRect(rectPx(ims[0]), [4]float64{20, 20, 40, 30}) {
		t.Fatalf("%v", ims)
	}
	// The line is the picture and the paragraph mark's descent.
	if y := p.lines(1)[0].y; !near(y, 20+30+2.4+wordTestAscent12) {
		t.Errorf("next baseline %v", y)
	}
}

func TestEffectExtentIsMargin(t *testing.T) {
	pic := wordTestPic{w: 40, h: 30, ext: [4]float64{3, 5, 7, 11}}
	p, _ := wordTestRender(t, wordTestBody(wordTestPara("", pic.inline(), wordTestRun("", "x"))), wordTestMedia(wordTestSolid(4, 4, wordTestGreen)))
	ims := p.pictures(1)
	if got := rectPx(ims[0]); !sameRect(got, [4]float64{23, 25, 40, 30}) {
		t.Errorf("rect %v", got)
	}
	// 5 above the frame, 11 below, then the descent.
	if y := p.lines(1)[0].y; !near(y, 20+5+30+11) {
		t.Errorf("baseline %v", y)
	}
	if x := p.lines(1)[0].x; !near(x, 20+3+40+7) {
		t.Errorf("text %v", x)
	}
}

func TestPictureCropAndFlipAndRotation(t *testing.T) {
	// The source is 20 by 10, red on the left, blue on the right.
	data := wordTestHalves(20, 10)
	cases := []struct {
		name string
		pic  wordTestPic
		// samples are page pixels and the colours they must have.
		samples map[[2]int]color.NRGBA
	}{
		{"uncropped", wordTestPic{w: 40, h: 20}, map[[2]int]color.NRGBA{{30, 30}: wordTestRed, {50, 30}: wordTestBlue}},
		{"crop left half away", wordTestPic{w: 40, h: 20, src: `l="50000"`}, map[[2]int]color.NRGBA{{30, 30}: wordTestBlue, {50, 30}: wordTestBlue}},
		{"crop right quarter", wordTestPic{w: 40, h: 20, src: `r="25000"`}, map[[2]int]color.NRGBA{{30, 30}: wordTestRed, {40, 30}: wordTestRed, {55, 30}: wordTestBlue}},
		{"flip horizontally", wordTestPic{w: 40, h: 20, xfrm: `flipH="1"`}, map[[2]int]color.NRGBA{{30, 30}: wordTestBlue, {50, 30}: wordTestRed}},
		{"flip vertically keeps halves", wordTestPic{w: 40, h: 20, xfrm: `flipV="1"`}, map[[2]int]color.NRGBA{{30, 30}: wordTestRed, {50, 30}: wordTestBlue}},
		// A quarter turn clockwise puts the left (red) side on top: the box is
		// 20 wide and 40 tall about the same centre (40, 30).
		{"quarter turn", wordTestPic{w: 40, h: 20, xfrm: `rot="5400000"`}, map[[2]int]color.NRGBA{{40, 15}: wordTestRed, {40, 45}: wordTestBlue}},
		{"half turn", wordTestPic{w: 40, h: 20, xfrm: `rot="10800000"`}, map[[2]int]color.NRGBA{{30, 30}: wordTestBlue, {50, 30}: wordTestRed}},
		// Negative crop pads with transparent: the left half of the box is
		// empty and the picture is squeezed into the right.
		{"extended crop", wordTestPic{w: 40, h: 20, src: `l="-100000"`}, map[[2]int]color.NRGBA{{30, 30}: {255, 255, 255, 255}, {45, 30}: wordTestRed, {55, 30}: wordTestBlue}},
		{"fill rect insets", wordTestPic{w: 40, h: 20, fill: `l="50000"`}, map[[2]int]color.NRGBA{{30, 30}: {255, 255, 255, 255}, {45, 30}: wordTestRed, {55, 30}: wordTestBlue}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// A picture-only paragraph in the text area's corner: frame at
			// (20, 20), 40 by 20; but the left of the quarter turn is at x=30.
			body := wordTestBody(wordTestPara("", tc.pic.inline()))
			p, _ := wordTestRender(t, body, wordTestMedia(data))
			img := p.pixels(t, 1)
			for at, want := range tc.samples {
				if got := nrgba(img, at[0], at[1]); !near3(got, want, 3) {
					t.Errorf("pixel %v: %+v, want %+v", at, got, want)
				}
			}
		})
	}
}

func TestRotatedPictureBoundingBox(t *testing.T) {
	// 40 by 20 turned by 30 degrees: its bounds are (40cos+20sin) by (40sin+20cos).
	pic := wordTestPic{w: 40, h: 20, xfrm: `rot="1800000"`}
	body := wordTestBody(wordTestPara("", pic.inline()))
	p, _ := wordTestRender(t, body, wordTestMedia(wordTestSolid(40, 20, wordTestGreen)))
	ims := p.pictures(1)
	bw := 40*math.Cos(math.Pi/6) + 20*math.Sin(math.Pi/6)
	bh := 40*math.Sin(math.Pi/6) + 20*math.Cos(math.Pi/6)
	cx, cy := 20+20.0, 20+10.0
	if got := rectPx(ims[0]); !sameRect(got, [4]float64{cx - bw/2, cy - bh/2, bw, bh}) {
		t.Errorf("rect %v", got)
	}
	img := p.pixels(t, 1)
	if got := nrgba(img, 40, 30); got != wordTestGreen {
		t.Errorf("centre %+v", got)
	}
	// The corner of the bounding box lies outside the turned picture.
	if got := nrgba(img, int(cx-bw/2)+1, int(cy-bh/2)+1); got.A == 255 && got != (color.NRGBA{255, 255, 255, 255}) {
		t.Errorf("corner %+v", got)
	}
}

func TestPictureColourEffects(t *testing.T) {
	data := wordTestSolid(8, 8, color.NRGBA{R: 200, G: 100, B: 50, A: 255})
	gray := wordTestPic{w: 20, h: 20, blip: `<a:grayscl/>`}
	p, _ := wordTestRender(t, wordTestBody(wordTestPara("", gray.inline())), wordTestMedia(data))
	got := nrgba(p.pixels(t, 1), 30, 30)
	if got.R != got.G || got.G != got.B || got.R < 100 || got.R > 140 {
		t.Errorf("grayscale %+v", got)
	}
	duo := wordTestPic{w: 20, h: 20, blip: `<a:duotone><a:srgbClr val="000000"/><a:srgbClr val="FFFFFF"/></a:duotone>`}
	p, _ = wordTestRender(t, wordTestBody(wordTestPara("", duo.inline())), wordTestMedia(data))
	if got = nrgba(p.pixels(t, 1), 30, 30); got.R != got.G || got.G != got.B {
		t.Errorf("duotone %+v", got)
	}
	// Effects change the pixels, so pictures with different effects do not
	// share them.
	two := wordTestPara("", gray.inline(), duo.inline())
	p, _ = wordTestRender(t, wordTestBody(two), wordTestMedia(data))
	ims := p.pictures(1)
	if len(ims) != 2 || ims[0].Image == ims[1].Image {
		t.Errorf("pictures share pixels: %d", len(ims))
	}
	// Brightness is drawn as LibreOffice does: reported, refused when strict.
	lum := wordTestPic{w: 20, h: 20, blip: `<a:lum bright="20000"/>`}
	body := wordTestBody(wordTestPara("", lum.inline()))
	doc := wordTestDoc(t, body, wordTestMedia(data))
	_, err := Prepare(context.Background(), doc, newWordTestOpts(t, false).Options)
	if !errors.Is(err, render.ErrUnsupported) {
		t.Errorf("strict: %v", err)
	}
	o := newWordTestOpts(t, true)
	if _, err = Prepare(context.Background(), wordTestDoc(t, body, wordTestMedia(data)), o.Options); err != nil {
		t.Fatal(err)
	}
	if !warned(o, "brightness", render.ErrApproximated) {
		t.Errorf("warnings %v", o.warnings)
	}
	// A colour with transforms is not resolved: effects are skipped, reported.
	tr := wordTestPic{w: 20, h: 20, blip: `<a:duotone><a:srgbClr val="000000"/><a:srgbClr val="FFFFFF"><a:tint val="50000"/></a:srgbClr></a:duotone>`}
	o = newWordTestOpts(t, true)
	if _, err = Prepare(context.Background(), wordTestDoc(t, wordTestBody(wordTestPara("", tr.inline())), wordTestMedia(data)), o.Options); err != nil {
		t.Fatal(err)
	}
	if !warned(o, "colour transforms", render.ErrApproximated) {
		t.Errorf("warnings %v", o.warnings)
	}
}

// warned reports whether a warning contains text and wraps target.
func warned(o *wordTestOpts, text string, target error) bool {
	for _, w := range o.warnings {
		if strings.Contains(w.Error(), text) && errors.Is(w, target) {
			return true
		}
	}
	return false
}

func TestPictureOutline(t *testing.T) {
	pic := wordTestPic{w: 40, h: 20, sp: `<a:ln w="38100"><a:solidFill><a:srgbClr val="00FF00"/></a:solidFill></a:ln>`}
	p, _ := wordTestRender(t, wordTestBody(wordTestPara("", pic.inline())), wordTestMedia(wordTestSolid(4, 4, wordTestRed)))
	img := p.pixels(t, 1)
	// A 4 pixel line centred on the frame's edge: x from 18 to 22 on the left.
	if got := nrgba(img, 19, 30); got != wordTestGreen {
		t.Errorf("outline %+v", got)
	}
	if got := nrgba(img, 30, 30); got != wordTestRed {
		t.Errorf("inside %+v", got)
	}
	if got := nrgba(img, 15, 30); got != (color.NRGBA{255, 255, 255, 255}) {
		t.Errorf("outside %+v", got)
	}
}

func TestPictureDrawnOncePerImagePart(t *testing.T) {
	pic := wordTestPic{w: 20, h: 20}
	var runs []string
	for i := 0; i < 5; i++ {
		runs = append(runs, pic.inline())
	}
	p, _ := wordTestRender(t, wordTestBody(wordTestPara("", runs...)), wordTestMedia(wordTestSolid(4, 4, wordTestBlue)))
	ims := p.pictures(1)
	if len(ims) != 5 {
		t.Fatalf("%d pictures", len(ims))
	}
	for _, im := range ims[1:] {
		if im.Image != ims[0].Image {
			t.Fatal("equal pictures do not share their pixels")
		}
	}
}

func TestPictureTooTallMovesWithItsLine(t *testing.T) {
	// The text area is 160 tall. Five lines of 12 leave 100; a 120 picture
	// starts the next page and takes its line with it.
	var paras []string
	paras = append(paras, wordTestPara("", wordTestRun("", strings.Repeat("a ", 5)+"\n")))
	for i := 0; i < 4; i++ {
		paras = append(paras, wordTestPara("", wordTestRun("", "x")))
	}
	pic := wordTestPic{w: 40, h: 120}
	paras = append(paras, wordTestPara("", pic.inline()), wordTestPara("", wordTestRun("", "after")))
	p, _ := wordTestRender(t, wordTestBody(paras...), wordTestMedia(wordTestSolid(4, 4, wordTestGreen)))
	if p.Count() != 2 {
		t.Fatalf("%d pages", p.Count())
	}
	if len(p.pictures(1)) != 0 || len(p.pictures(2)) != 1 {
		t.Fatalf("pictures per page: %d, %d", len(p.pictures(1)), len(p.pictures(2)))
	}
	if got := rectPx(p.pictures(2)[0]); !sameRect(got, [4]float64{20, 20, 40, 120}) {
		t.Errorf("rect %v", got)
	}
	// A picture taller than the page is placed anyway and clipped.
	huge := wordTestPic{w: 40, h: 400}
	p, _ = wordTestRender(t, wordTestBody(wordTestPara("", huge.inline()), wordTestPara("", wordTestRun("", "after"))), wordTestMedia(wordTestSolid(4, 4, wordTestGreen)))
	if p.Count() != 2 || len(p.pictures(1)) != 1 {
		t.Fatalf("%d pages, %d pictures", p.Count(), len(p.pictures(1)))
	}
	p.pixels(t, 1)
}
