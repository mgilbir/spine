package docxrender

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"image/color"
	"strings"
	"testing"

	"github.com/mgilbir/spine/docx"
	"github.com/mgilbir/spine/render"
)

// prepareBoth prepares a document strictly and in best effort mode, returning
// both results and the warnings.
func prepareBoth(t *testing.T, body string, parts wordTestParts, tweak func(*render.Options)) (strictErr error, lenient *Pages, o *wordTestOpts, lenientErr error) {
	t.Helper()
	so := newWordTestOpts(t, false)
	if tweak != nil {
		tweak(&so.Options)
	}
	_, strictErr = Prepare(context.Background(), wordTestDoc(t, body, parts), so.Options)
	o = newWordTestOpts(t, true)
	if tweak != nil {
		tweak(&o.Options)
	}
	lenient, lenientErr = Prepare(context.Background(), wordTestDoc(t, body, parts), o.Options)
	return strictErr, lenient, o, lenientErr
}

func TestUnreadablePicturesAreRefusedOrLeftOutKeepingTheirSpace(t *testing.T) {
	pic := wordTestPic{w: 40, h: 30}
	cases := []struct {
		name   string
		pic    wordTestPic
		parts  wordTestParts
		strict error
		warn   string
	}{
		{"missing relationship", wordTestPic{w: 40, h: 30, embed: "rIdNone"}, wordTestMedia(wordTestSolid(2, 2, wordTestRed)), render.ErrUnsupported, "image is missing"},
		{"not an image relationship", wordTestPic{w: 40, h: 30, embed: wordTestRID("word/theme/theme1.xml")}, wordTestParts{styles: wordTestStyles, theme: `<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"/>`}, render.ErrUnsupported, "do not embed an image"},
		{"no image at all", wordTestPic{w: 40, h: 30, embed: "NONE"}, wordTestMedia(wordTestSolid(2, 2, wordTestRed)), render.ErrUnsupported, "image is missing"},
		{"corrupt png", pic, wordTestMedia(append([]byte("\x89PNG\r\n\x1a\n"), 1, 2, 3, 4)), render.ErrInvalid, "corrupt image"},
		{"unsupported format", pic, wordTestMedia([]byte("\xd7\xcd\xc6\x9a wmf-ish bytes")), render.ErrUnsupported, "not PNG, JPEG or GIF"},
		{"declared far too large", pic, wordTestMedia(wordTestBombPNG(100000, 100000)), render.ErrLimit, "too large"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// The picture is inline between two paragraphs of text.
			p := tc.pic
			body := wordTestBody(wordTestPara("", p.inline()), wordTestPara("", wordTestRun("", "next")))
			if tc.name == "no image at all" {
				body = strings.Replace(body, `r:embed="NONE"`, `r:link="rIdLinked"`, 1)
			}
			strictErr, pages, o, err := prepareBoth(t, body, tc.parts, nil)
			if !errors.Is(strictErr, tc.strict) {
				t.Errorf("strict: %v, want %v", strictErr, tc.strict)
			}
			if err != nil {
				t.Fatal(err)
			}
			if !warned(o, tc.warn, render.ErrUnsupported) && (tc.name != "no image at all" || !warned(o, "linked", render.ErrUnsupported)) {
				t.Errorf("warnings %v", o.warnings)
			}
			if n := len(pages.pictures(1)); n != 0 {
				t.Errorf("%d pictures", n)
			}
			// The space is kept: the next line is below a 30 pixel line.
			if y := pages.lines(1)[0].y; !near(y, 20+30+2.4+wordTestAscent12) {
				t.Errorf("next baseline %v", y)
			}
		})
	}
}

// wordTestBombPNG is a PNG header declaring a size no decoder should be asked
// to allocate, with no pixels behind it.
func wordTestBombPNG(w, h uint32) []byte {
	var ihdr bytes.Buffer
	_ = binary.Write(&ihdr, binary.BigEndian, struct {
		W, H                  uint32
		Depth, Color, C, F, I uint8
	}{w, h, 8, 6, 0, 0, 0})
	var out bytes.Buffer
	out.WriteString("\x89PNG\r\n\x1a\n")
	chunk := func(kind string, data []byte) {
		_ = binary.Write(&out, binary.BigEndian, uint32(len(data)))
		out.WriteString(kind)
		out.Write(data)
		sum := crc32.NewIEEE()
		sum.Write([]byte(kind))
		sum.Write(data)
		_ = binary.Write(&out, binary.BigEndian, sum.Sum32())
	}
	chunk("IHDR", ihdr.Bytes())
	chunk("IEND", nil)
	return out.Bytes()
}

func TestLimitsOnPictures(t *testing.T) {
	pic := wordTestPic{w: 40, h: 30}
	solid := wordTestMedia(wordTestSolid(20, 20, wordTestRed))
	t.Run("image pixels", func(t *testing.T) {
		tweak := func(o *render.Options) { o.Limits.MaxImagePixels = 100 }
		body := wordTestBody(wordTestPara("", pic.inline(), wordTestRun("", "x")))
		strictErr, pages, o, err := prepareBoth(t, body, solid, tweak)
		if !errors.Is(strictErr, render.ErrLimit) {
			t.Errorf("strict: %v", strictErr)
		}
		if err != nil || len(pages.pictures(1)) != 0 || !warned(o, "too large", render.ErrUnsupported) {
			t.Errorf("lenient: %v %v", err, o.warnings)
		}
	})
	t.Run("image bytes", func(t *testing.T) {
		tweak := func(o *render.Options) { o.Limits.MaxImageBytes = 16 }
		body := wordTestBody(wordTestPara("", pic.inline(), wordTestRun("", "x")))
		strictErr, _, _, _ := prepareBoth(t, body, solid, tweak)
		if !errors.Is(strictErr, render.ErrLimit) {
			t.Errorf("strict: %v", strictErr)
		}
	})
	t.Run("extent", func(t *testing.T) {
		huge := wordTestPic{w: 40, h: 30}
		body := wordTestBody(wordTestPara("", huge.inline()))
		body = strings.Replace(body, `<wp:extent cx="381000"`, `<wp:extent cx="9000000000000000"`, 1)
		strictErr, _, _, err := prepareBoth(t, body, solid, nil)
		if !errors.Is(strictErr, render.ErrLimit) || !errors.Is(err, render.ErrLimit) {
			t.Errorf("%v / %v", strictErr, err)
		}
	})
	t.Run("negative extent", func(t *testing.T) {
		body := wordTestBody(wordTestPara("", pic.inline()))
		body = strings.Replace(body, `<wp:extent cx="381000"`, `<wp:extent cx="-5"`, 1)
		strictErr, _, _, err := prepareBoth(t, body, solid, nil)
		if !errors.Is(strictErr, render.ErrInvalid) || !errors.Is(err, render.ErrInvalid) {
			t.Errorf("%v / %v", strictErr, err)
		}
	})
	t.Run("giant sizes do not crash", func(t *testing.T) {
		for _, p := range []wordTestPic{
			{w: 900000, h: 900000},
			{w: 1000, h: 1, xfrm: `rot="2700000"`},
			{w: 40, h: 30, ext: [4]float64{1e6, 1e6, 1e6, 1e6}},
			{w: 40, h: 30, src: `l="-90000000" r="-90000000"`},
			{w: 40, h: 30, src: `l="60000" r="60000"`},
		} {
			body := wordTestBody(wordTestPara("", p.inline(), wordTestRun("", "x")))
			for _, lenient := range []bool{false, true} {
				o := newWordTestOpts(t, lenient)
				pages, err := Prepare(context.Background(), wordTestDoc(t, body, solid), o.Options)
				if err != nil {
					continue
				}
				for i := 1; i <= pages.Count(); i++ {
					if pg, e := pages.Page(context.Background(), i); e == nil {
						var out bytes.Buffer
						_ = pg.WritePNG(context.Background(), &out, 96)
					}
				}
			}
		}
	})
	t.Run("pictures per page", func(t *testing.T) {
		var runs []string
		for i := 0; i < 40; i++ {
			runs = append(runs, wordTestPic{w: 10, h: 5, src: fmt.Sprintf(`l="%d"`, i*1000)}.inline())
		}
		p, _ := wordTestRender(t, wordTestBody(wordTestPara("", runs...)), solid)
		if _, err := p.Page(context.Background(), 1); !errors.Is(err, render.ErrLimit) {
			t.Errorf("page: %v", err)
		}
	})
	t.Run("pictures in the document", func(t *testing.T) {
		var runs []string
		for i := 0; i < wordMaxPictures+1; i++ {
			runs = append(runs, wordTestPic{w: 1, h: 1}.inline())
		}
		o := newWordTestOpts(t, true)
		_, err := Prepare(context.Background(), wordTestDoc(t, wordTestBody(wordTestPara("", runs...)), solid), o.Options)
		if !errors.Is(err, render.ErrLimit) {
			t.Errorf("%v", err)
		}
	})
}

func TestOnlyTheDocumentsPicturesAreLoadable(t *testing.T) {
	r := &wordRenderer{pictures: []*wordPicture{{id: 0}, {id: 1}}}
	res := &wordImages{r: r, cache: map[int][]byte{}}
	for _, ref := range []string{"", "x", "i", "i2", "i-1", "i01", "i1.png", "I1", "i1 ", "../i1", "/etc/passwd", "file:///etc/passwd", "http://127.0.0.1/i0", "//host/i0", `\\host\i0`, "data:image/png;base64,AAAA", "i99999999999", "i0?x"} {
		if b, err := res.Resolve(ref); err == nil {
			t.Errorf("%q resolved to %d bytes", ref, len(b))
		}
	}
	for id := 0; id < 2; id++ {
		b, err := res.Resolve("i" + string(rune('0'+id)))
		if err != nil {
			t.Fatal(err)
		}
		if len(b) == 0 || len(b) > 200 {
			t.Errorf("placeholder of %d bytes", len(b))
		}
	}
}

func TestPictureReferencesNeverReachMarkup(t *testing.T) {
	hostile := `x" onerror="alert(1)"><script>alert(2)</script><img src="http://evil/x.png`
	pic := wordTestPic{w: 40, h: 30, embed: "NONE"}
	body := wordTestBody(wordTestPara("", pic.inline(), wordTestRun("", "text")))
	body = strings.Replace(body, `r:embed="NONE"`, `r:embed="`+strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;").Replace(hostile)+`"`, 1)
	body = strings.Replace(body, `name="p"`, `name="`+strings.NewReplacer("<", "&lt;", ">", "&gt;", `"`, "&quot;").Replace(hostile)+`" descr="`+strings.NewReplacer("<", "&lt;", ">", "&gt;", `"`, "&quot;").Replace(hostile)+`"`, 1)
	_, pages, o, err := prepareBoth(t, body, wordTestMedia(wordTestSolid(2, 2, wordTestRed)), nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range o.warnings {
		if strings.Contains(w.Error(), "evil") || strings.Contains(w.Error(), "script") {
			t.Errorf("warning repeats the document: %v", w)
		}
	}
	if got := pages.lines(1); len(got) != 1 || !strings.Contains(got[0].text, "text") {
		t.Errorf("text %+v", got)
	}
}

func TestOtherGraphicsAreLeftOut(t *testing.T) {
	for _, tc := range []struct{ uri, warn string }{
		{"http://schemas.openxmlformats.org/drawingml/2006/chart", "charts"},
		{"http://schemas.openxmlformats.org/drawingml/2006/diagram", "SmartArt"},
		{"http://schemas.microsoft.com/office/word/2010/wordprocessingShape", "shapes and text boxes"},
		{"urn:unknown", "unknown kind"},
	} {
		t.Run(tc.warn, func(t *testing.T) {
			p := wordTestPic{w: 40, h: 30, graphicURI: tc.uri}
			body := wordTestBody(wordTestPara("", p.inline()), wordTestPara("", wordTestRun("", "next")))
			strictErr, pages, o, err := prepareBoth(t, body, wordTestMedia(wordTestSolid(2, 2, wordTestRed)), nil)
			if !errors.Is(strictErr, render.ErrUnsupported) || err != nil {
				t.Fatalf("%v / %v", strictErr, err)
			}
			if !warned(o, tc.warn, render.ErrUnsupported) {
				t.Errorf("warnings %v", o.warnings)
			}
			// An inline drawing keeps its space.
			if y := pages.lines(1)[0].y; !near(y, 20+30+2.4+wordTestAscent12) {
				t.Errorf("next baseline %v", y)
			}
			// An anchored one leaves no trace.
			body = wordTestBody(wordTestPara("", p.anchored(wordTestAnchor{}), wordTestRun("", "next")))
			_, pages, _, err = prepareBoth(t, body, wordTestMedia(wordTestSolid(2, 2, wordTestRed)), nil)
			if err != nil {
				t.Fatal(err)
			}
			if len(pages.pictures(1)) != 0 || !near(pages.lines(1)[0].y, 20+wordTestAscent12) {
				t.Errorf("anchored: %+v", pages.lines(1))
			}
		})
	}
}

func TestPicturesBuiltWithTheDocumentAPI(t *testing.T) {
	data := wordTestSolid(8, 8, color.NRGBA{R: 10, G: 200, B: 30, A: 255})
	doc := docx.Create()
	p := doc.AddParagraph()
	run := p.AddRun()
	img, err := run.AddImageFromBytes(data, "image/png")
	if err != nil {
		t.Fatal(err)
	}
	img.SetSize(60, 30)
	fl, err := p.AddRun().AddFloatingImageFromBytes(data, "image/png", docx.Anchor{RelativeToPage: true, X: 36, Y: 72, BehindText: true})
	if err != nil {
		t.Fatal(err)
	}
	fl.SetSize(30, 30)
	// The new document has no page setup, which only best effort accepts.
	o := newWordTestOpts(t, true)
	pages, err := Prepare(context.Background(), doc, o.Options)
	if err != nil {
		t.Fatal(err)
	}
	for _, w := range o.warnings {
		if strings.Contains(w.Error(), "picture") {
			t.Errorf("warning %v", w)
		}
	}
	ims := pages.pictures(1)
	if len(ims) != 2 {
		t.Fatalf("%d pictures", len(ims))
	}
	// Behind the text first: 36pt and 72pt on the page, 30pt square.
	if !sameRect(rectPx(ims[0]), [4]float64{48, 96, 40, 40}) {
		t.Errorf("floating %v", rectPx(ims[0]))
	}
	if w, h := rectPx(ims[1])[2], rectPx(ims[1])[3]; !near(w, 80) || !near(h, 40) {
		t.Errorf("inline %v x %v", w, h)
	}
	if got := nrgba(pages.pixels(t, 1), 60, 110); !near3(got, color.NRGBA{R: 10, G: 200, B: 30, A: 255}, 2) {
		t.Errorf("pixel %+v", got)
	}
}
