package docxrender

import (
	"bytes"
	"context"
	"testing"

	"github.com/mgilbir/forme/shape"
	"github.com/mgilbir/spine/docx"
	"github.com/mgilbir/spine/internal/fuzzseed"
	"github.com/mgilbir/spine/render"
)

// wordFuzzDrawings are paragraphs with drawings that exercise the picture
// paths: they seed the fuzzer and are checked to draw pictures below.
func wordFuzzDrawings() []string {
	inline := wordTestPic{w: 30, h: 20, ext: [4]float64{1, 2, 3, 4}, src: `l="10000" t="-5000" r="20%"`, xfrm: `rot="2700000" flipH="1"`,
		blip:  `<a:duotone><a:srgbClr val="000000"/><a:srgbClr val="FFFFFF"/></a:duotone><a:lum bright="10000" contrast="-20000"/>`,
		sp:    `<a:ln w="12700"><a:solidFill><a:srgbClr val="FF0000"/></a:solidFill></a:ln>`,
		fill:  `l="10000" r="-10000"`,
		embed: wordTestEmbed}
	plain := wordTestPic{w: 40, h: 30}
	square := wordTestAnchor{dist: [4]float64{2, 2, 6, 6}, hFrom: "margin", h: "right", vFrom: "paragraph", v: "0", wrap: `<wp:wrapSquare wrapText="bothSides"/>`}
	left := wordTestAnchor{hFrom: "column", h: "50", vFrom: "line", v: "3", wrap: `<wp:wrapSquare wrapText="left"/>`}
	topBottom := wordTestAnchor{hFrom: "page", h: "center", vFrom: "paragraph", v: "0", wrap: `<wp:wrapTopAndBottom/>`}
	overlay := wordTestAnchor{attrs: `behindDoc="1"`, hFrom: "page", h: "-9", vFrom: "bottomMargin", v: "center"}
	simple := wordTestAnchor{attrs: `simplePos="1"`}
	chart := wordTestPic{w: 20, h: 20, graphicURI: "http://schemas.openxmlformats.org/drawingml/2006/chart"}
	return []string{
		wordTestPara("", wordTestRun("", "a"), inline.inline(), wordTestRun("", "b")),
		wordTestPara("", plain.anchored(square), wordTestRun("", "text text text text text text text text text text text text")),
		wordTestPara(tabStops(tabStop("right", 100, "dot")), plain.anchored(left), wordTestRun("", "x"), plain.inline(), `<w:r><w:tab/></w:r>`, wordTestRun("", "y")),
		wordTestPara("", plain.anchored(topBottom), plain.anchored(overlay), plain.anchored(simple), chart.inline(), chart.anchored(wordTestAnchor{})),
		wordTestPara(`<w:spacing w:line="200" w:lineRule="exact"/>`, plain.inline()) + wordTestPara(`<w:pageBreakBefore/>`, plain.anchored(square)),
	}
}

func TestFuzzDrawingSeedsDrawPictures(t *testing.T) {
	seed := wordTestDoc(t, wordTestPara("", wordTestRun("", "AA"))+wordTestPage, wordTestMedia(wordTestHalves(8, 4)))
	valid, err := seed.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	face := wordTestFace(t)
	for i, body := range wordFuzzDrawings() {
		data := []byte(`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><w:body>` + body + wordTestPage + `</w:body></w:document>`)
		wrapped := fuzzseed.ReplaceZipEntry(valid, "word/document.xml", data)
		doc, err := docx.OpenReader(bytes.NewReader(wrapped), int64(len(wrapped)))
		if err != nil {
			t.Fatal(err)
		}
		opts := render.Options{Fonts: func(context.Context, render.FontRequest) (*shape.Face, error) { return face, nil }, Warn: func(error) {}}
		pages, err := Prepare(context.Background(), doc, opts)
		if err != nil {
			t.Fatalf("seed %d: %v", i, err)
		}
		n := 0
		for p := 1; p <= pages.Count(); p++ {
			n += len(pages.pictures(p))
		}
		if n == 0 {
			t.Errorf("seed %d draws no picture", i)
		}
		_ = doc.Close()
	}
}
