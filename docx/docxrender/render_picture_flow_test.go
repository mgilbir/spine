package docxrender

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mgilbir/spine/render"
)

func TestPictureBeforeATabKeepsTheStop(t *testing.T) {
	pic := wordTestPic{w: 30, h: 10}
	para := wordTestPara(tabStops(tabStop("left", 100, "")), wordTestRun("", "a"), pic.inline(), `<w:r><w:tab/></w:r>`, wordTestRun("", "b"))
	p, _ := wordTestRender(t, wordTestBody(para), floatMedia())
	// "a" is 6 wide and the picture 30: the tab runs to the stop at 100.
	if x := p.startOf(1, "b", 0); !near(x, 20+100) {
		t.Errorf("b at %v", x)
	}
	if got := rectPx(p.pictures(1)[0]); !near(got[0], 26) {
		t.Errorf("picture %v", got)
	}
}

func TestAnchorOnlyParagraphIsOneLine(t *testing.T) {
	pic := wordTestPic{w: 40, h: 90}
	a := wordTestAnchor{hFrom: "page", h: "100", vFrom: "page", v: "100"}
	p, _ := wordTestRender(t, wordTestBody(wordTestPara("", pic.anchored(a)), wordTestPara("", wordTestRun("", "next"))), floatMedia())
	if l := p.lines(1); len(l) != 1 || !near(l[0].y, 20+12+wordTestAscent12) {
		t.Errorf("%+v", l)
	}
}

func TestManyFloatsAreBounded(t *testing.T) {
	pic := wordTestPic{w: 5, h: 5}
	a := wordTestAnchor{hFrom: "column", h: "0", vFrom: "paragraph", v: "0", wrap: `<wp:wrapSquare wrapText="bothSides"/>`}
	var runs []string
	for i := 0; i < 3000; i++ {
		runs = append(runs, pic.anchored(a))
	}
	runs = append(runs, wordTestRun("", strings.Repeat("word ", 100)))
	start := time.Now()
	o := newWordTestOpts(t, true)
	if _, err := Prepare(context.Background(), wordTestDoc(t, wordTestBody(wordTestPara("", runs...)), floatMedia()), o.Options); err != nil && !errors.Is(err, render.ErrLimit) {
		t.Fatal(err)
	}
	if d := time.Since(start); d > 20*time.Second {
		t.Errorf("took %v", d)
	}
	t.Logf("3000 floats took %v", time.Since(start))
}
