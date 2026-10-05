package docxrender

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mgilbir/forme/layout"
	"github.com/mgilbir/spine/render"
)

func floatMedia() wordTestParts { return wordTestMedia(wordTestSolid(4, 4, wordTestGreen)) }

func TestUnwrappedPictureBelongsToThePageOfItsAnchor(t *testing.T) {
	pic := wordTestPic{w: 40, h: 30}
	cases := []struct {
		name string
		a    wordTestAnchor
		want [4]float64
	}{
		{"page offsets", wordTestAnchor{hFrom: "page", h: "100", vFrom: "page", v: "50"}, [4]float64{100, 50, 40, 30}},
		{"margin offsets", wordTestAnchor{hFrom: "margin", h: "10", vFrom: "margin", v: "7"}, [4]float64{30, 27, 40, 30}},
		{"column offset", wordTestAnchor{hFrom: "column", h: "15", vFrom: "page", v: "0"}, [4]float64{35, 0, 40, 30}},
		{"right aligned in margin", wordTestAnchor{hFrom: "margin", h: "right", vFrom: "page", v: "0"}, [4]float64{240, 0, 40, 30}},
		{"centred on the page", wordTestAnchor{hFrom: "page", h: "center", vFrom: "page", v: "center"}, [4]float64{130, 85, 40, 30}},
		{"bottom of the margin", wordTestAnchor{hFrom: "leftMargin", h: "0", vFrom: "margin", v: "bottom"}, [4]float64{0, 150, 40, 30}},
		{"paragraph offset", wordTestAnchor{hFrom: "page", h: "0", vFrom: "paragraph", v: "5"}, [4]float64{0, 25, 40, 30}},
		{"line offset", wordTestAnchor{hFrom: "page", h: "0", vFrom: "line", v: "3"}, [4]float64{0, 23, 40, 30}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// The anchor is in the second paragraph, which opens page 2.
			body := wordTestBody(wordTestPara("", wordTestRun("", "one")),
				wordTestPara(`<w:pageBreakBefore/>`, wordTestRun("", "two"), pic.anchored(tc.a)))
			p, _ := wordTestRender(t, body, floatMedia())
			if p.Count() != 2 || len(p.pictures(1)) != 0 || len(p.pictures(2)) != 1 {
				t.Fatalf("%d pages, pictures %d/%d", p.Count(), len(p.pictures(1)), len(p.pictures(2)))
			}
			if got := rectPx(p.pictures(2)[0]); !sameRect(got, tc.want) {
				t.Errorf("rect %v, want %v", got, tc.want)
			}
			// Text is not moved.
			if l := p.lines(2); len(l) != 1 || !near(l[0].x, 20) {
				t.Errorf("text %+v", l)
			}
		})
	}
}

func TestCharacterRelativePictureFollowsItsAnchor(t *testing.T) {
	pic := wordTestPic{w: 40, h: 30}
	a := wordTestAnchor{hFrom: "character", h: "4", vFrom: "line", v: "0"}
	p, _ := wordTestRender(t, wordTestBody(wordTestPara("", wordTestRun("", "abcde"), pic.anchored(a), wordTestRun("", "fg"))), floatMedia())
	// "abcde" is 30 wide; the picture is 4 right of it, at the line's top.
	if got := rectPx(p.pictures(1)[0]); !sameRect(got, [4]float64{54, 20, 40, 30}) {
		t.Errorf("rect %v", got)
	}
}

func TestBehindAndInFrontOfText(t *testing.T) {
	pic := wordTestPic{w: 40, h: 30}
	front := wordTestAnchor{hFrom: "page", h: "10", vFrom: "page", v: "10"}
	behind := wordTestAnchor{attrs: `behindDoc="1"`, hFrom: "page", h: "20", vFrom: "page", v: "20"}
	body := wordTestBody(wordTestPara("", wordTestRun("", "text"), pic.anchored(front), pic.anchored(behind)))
	p, _ := wordTestRender(t, body, floatMedia())
	var kinds []string
	for _, op := range p.pages[0].ops {
		switch v := op.(type) {
		case layout.DrawImage:
			kinds = append(kinds, map[bool]string{true: "behind", false: "front"}[v.Rect.X.Px() == 20])
		case layout.DrawText:
			kinds = append(kinds, "text")
		}
	}
	if strings.Join(kinds, ",") != "behind,text,front" {
		t.Errorf("order %v", kinds)
	}
}

func TestStackingOrderFollowsRelativeHeight(t *testing.T) {
	pic := wordTestPic{w: 40, h: 30}
	hi := strings.Replace(pic.anchored(wordTestAnchor{hFrom: "page", h: "10", vFrom: "page", v: "10"}), `relativeHeight="1"`, `relativeHeight="9"`, 1)
	lo := strings.Replace(pic.anchored(wordTestAnchor{hFrom: "page", h: "20", vFrom: "page", v: "20"}), `relativeHeight="1"`, `relativeHeight="3"`, 1)
	p, _ := wordTestRender(t, wordTestBody(wordTestPara("", hi, lo)), floatMedia())
	ims := p.pictures(1)
	if len(ims) != 2 || ims[0].Rect.X.Px() != 20 || ims[1].Rect.X.Px() != 10 {
		t.Errorf("order %v", ims)
	}
}

func TestSquareWrapFloatsTheTextAround(t *testing.T) {
	pic := wordTestPic{w: 40, h: 30}
	a := wordTestAnchor{dist: [4]float64{0, 0, 0, 8}, hFrom: "column", h: "0", vFrom: "paragraph", v: "0", wrap: `<wp:wrapSquare wrapText="bothSides"/>`}
	text := strings.Repeat("word ", 40)
	p, _ := wordTestRender(t, wordTestBody(wordTestPara("", pic.anchored(a), wordTestRun("", text))), floatMedia())
	if got := rectPx(p.pictures(1)[0]); !sameRect(got, [4]float64{20, 20, 40, 30}) {
		t.Errorf("rect %v", got)
	}
	lines := p.lines(1)
	// Lines at 12 pixels: the first three meet the picture (30 tall).
	for i, l := range lines[:4] {
		want := 20.0 + 48
		if i == 3 {
			want = 20
		}
		if !near(l.x, want) {
			t.Errorf("line %d starts at %v, want %v", i, l.x, want)
		}
	}
}

func TestSquareWrapOnTheRight(t *testing.T) {
	pic := wordTestPic{w: 40, h: 30}
	a := wordTestAnchor{dist: [4]float64{0, 0, 8, 0}, hFrom: "margin", h: "right", vFrom: "paragraph", v: "0", wrap: `<wp:wrapSquare wrapText="bothSides"/>`}
	text := strings.Repeat("word ", 40)
	p, _ := wordTestRender(t, wordTestBody(wordTestPara("", pic.anchored(a), wordTestRun("", text))), floatMedia())
	if got := rectPx(p.pictures(1)[0]); !sameRect(got, [4]float64{240, 20, 40, 30}) {
		t.Errorf("rect %v", got)
	}
	// A line beside the picture holds (260-48)/6 = 35 characters.
	first := p.lines(1)[0]
	if !near(first.x, 20) || len(strings.TrimRight(first.text, " ")) > 35 {
		t.Errorf("first line %q at %v is wider than the room beside the picture", first.text, first.x)
	}
}

func TestWrapTextSides(t *testing.T) {
	pic := wordTestPic{w: 40, h: 30}
	// Text on the left of a picture in the middle: the picture floats right
	// with the rest of the width as its margin.
	a := wordTestAnchor{hFrom: "column", h: "100", vFrom: "paragraph", v: "0", wrap: `<wp:wrapSquare wrapText="left"/>`}
	text := strings.Repeat("word ", 40)
	p, _ := wordTestRender(t, wordTestBody(wordTestPara("", pic.anchored(a), wordTestRun("", text))), floatMedia())
	if got := rectPx(p.pictures(1)[0]); !sameRect(got, [4]float64{120, 20, 40, 30}) {
		t.Errorf("rect %v", got)
	}
	// The first line ends before the picture: 100 pixels of room.
	if first := strings.TrimRight(p.lines(1)[0].text, " "); len(first) > 16 || !near(p.lines(1)[0].x, 20) {
		t.Errorf("first line %q at %v", first, p.lines(1)[0].x)
	}
	// Text on the right of it.
	a.wrap = `<wp:wrapSquare wrapText="right"/>`
	p, _ = wordTestRender(t, wordTestBody(wordTestPara("", pic.anchored(a), wordTestRun("", text))), floatMedia())
	if got := rectPx(p.pictures(1)[0]); !sameRect(got, [4]float64{120, 20, 40, 30}) {
		t.Errorf("rect %v", got)
	}
	if !near(p.lines(1)[0].x, 20+100+40) {
		t.Errorf("text starts at %v", p.lines(1)[0].x)
	}
}

func TestTopAndBottomWrapPushesTextBelow(t *testing.T) {
	pic := wordTestPic{w: 40, h: 30}
	a := wordTestAnchor{hFrom: "column", h: "100", vFrom: "paragraph", v: "0", wrap: `<wp:wrapTopAndBottom/>`}
	p, _ := wordTestRender(t, wordTestBody(wordTestPara("", pic.anchored(a), wordTestRun("", "text"))), floatMedia())
	if got := rectPx(p.pictures(1)[0]); !sameRect(got, [4]float64{120, 20, 40, 30}) {
		t.Errorf("rect %v", got)
	}
	if l := p.lines(1); len(l) != 1 || !near(l[0].y, 20+30+wordTestAscent12) || !near(l[0].x, 20) {
		t.Errorf("text %+v", l)
	}
}

func TestFloatMovesWithItsParagraphAcrossPages(t *testing.T) {
	pic := wordTestPic{w: 40, h: 30}
	a := wordTestAnchor{hFrom: "column", h: "0", vFrom: "paragraph", v: "0", wrap: `<wp:wrapSquare wrapText="bothSides"/>`}
	var paras []string
	for i := 0; i < 13; i++ {
		paras = append(paras, wordTestPara("", wordTestRun("", "filler")))
	}
	paras = append(paras, wordTestPara("", pic.anchored(a), wordTestRun("", "anchor")))
	p, _ := wordTestRender(t, wordTestBody(paras...), floatMedia())
	// Thirteen lines fill 156 of 160, so the anchor line opens page 2 and the
	// picture goes with it.
	if p.Count() != 2 || len(p.pictures(1)) != 0 || len(p.pictures(2)) != 1 {
		t.Fatalf("%d pages, pictures %d/%d", p.Count(), len(p.pictures(1)), len(p.pictures(2)))
	}
	if got := rectPx(p.pictures(2)[0]); !sameRect(got, [4]float64{20, 20, 40, 30}) {
		t.Errorf("rect %v", got)
	}
}

func TestApproximatedPlacementsAreReported(t *testing.T) {
	pic := wordTestPic{w: 40, h: 30}
	cases := []struct {
		name string
		a    wordTestAnchor
		warn string
	}{
		{"tight wrapping", wordTestAnchor{vFrom: "paragraph", v: "0", wrap: `<wp:wrapTight wrapText="bothSides"/>`}, "tight and through"},
		{"wrapped on the page", wordTestAnchor{hFrom: "page", h: "10", vFrom: "page", v: "10", wrap: `<wp:wrapSquare wrapText="bothSides"/>`}, "positioned on the page"},
		{"both sides in the middle", wordTestAnchor{hFrom: "column", h: "100", vFrom: "paragraph", v: "0", wrap: `<wp:wrapSquare wrapText="bothSides"/>`}, "both sides"},
		{"gap above", wordTestAnchor{hFrom: "column", h: "0", vFrom: "paragraph", v: "20", wrap: `<wp:wrapSquare wrapText="bothSides"/>`}, "space above"},
		{"inside alignment", wordTestAnchor{hFrom: "margin", h: "inside", vFrom: "page", v: "0"}, "inside and outside"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := wordTestBody(wordTestPara("", pic.anchored(tc.a), wordTestRun("", "text")))
			_, err := Prepare(context.Background(), wordTestDoc(t, body, floatMedia()), newWordTestOpts(t, false).Options)
			if !errors.Is(err, render.ErrUnsupported) {
				t.Errorf("strict: %v", err)
			}
			o := newWordTestOpts(t, true)
			p, err := Prepare(context.Background(), wordTestDoc(t, body, floatMedia()), o.Options)
			if err != nil {
				t.Fatal(err)
			}
			if !warned(o, tc.warn, render.ErrApproximated) {
				t.Errorf("warnings %v", o.warnings)
			}
			if len(p.pictures(1)) != 1 {
				t.Errorf("%d pictures", len(p.pictures(1)))
			}
		})
	}
}

func TestHiddenAndEmptyDrawings(t *testing.T) {
	pic := wordTestPic{w: 40, h: 30}
	hidden := wordTestPic{w: 40, h: 30, docPr: `hidden="1"`}
	zero := wordTestPic{w: 0, h: 30}
	body := wordTestBody(wordTestPara("", hidden.inline(), zero.inline(), strings.Replace(pic.inline(), "<w:r>", "<w:r><w:rPr><w:vanish/></w:rPr>", 1), wordTestRun("", "text")))
	p, _ := wordTestRender(t, body, floatMedia())
	if n := len(p.pictures(1)); n != 0 {
		t.Errorf("%d pictures", n)
	}
}

func TestSquareWrapKeepsThePictureWhenLinesMove(t *testing.T) {
	// At an exact 20 pixel line the text moves down to Word's baseline (0.8
	// of the line) but the floated picture stays at the paragraph's top.
	pic := wordTestPic{w: 40, h: 30}
	a := wordTestAnchor{dist: [4]float64{0, 0, 0, 8}, hFrom: "column", h: "0", vFrom: "paragraph", v: "0", wrap: `<wp:wrapSquare wrapText="bothSides"/>`}
	text := strings.Repeat("word ", 40)
	p, _ := wordTestRender(t, wordTestBody(wordTestPara(`<w:spacing w:line="300" w:lineRule="exact"/>`, pic.anchored(a), wordTestRun("", text))), floatMedia())
	if got := rectPx(p.pictures(1)[0]); !sameRect(got, [4]float64{20, 20, 40, 30}) {
		t.Errorf("rect %v", got)
	}
	lines := p.lines(1)
	if !near(lines[0].y, 20+16) || !near(lines[0].x, 20+48) {
		t.Errorf("first line at %v,%v", lines[0].x, lines[0].y)
	}
}
