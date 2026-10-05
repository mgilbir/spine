package docxrender

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"image/color"
	"image/png"
	"reflect"
	"strings"
	"testing"

	"github.com/mgilbir/forme/shape"
	"github.com/mgilbir/spine/docx"
	"github.com/mgilbir/spine/internal/fuzzseed"
	"github.com/mgilbir/spine/render"
)

func TestPreparePageMatchesPagesAndDoesNotChangeTheSource(t *testing.T) {
	d := wordTestDoc(t, wordTestBody(wordTestPara("", wordTestRun("", wordTestLines(20)))), wordTestParts{styles: wordTestStyles})
	before, err := d.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	o := newWordTestOpts(t, false)
	pages, err := Prepare(context.Background(), d, o.Options)
	if err != nil {
		t.Fatal(err)
	}
	if pages.Count() != 2 {
		t.Fatalf("%d pages", pages.Count())
	}
	for n := 1; n <= 2; n++ {
		one, err := PreparePage(context.Background(), d, n, o.Options)
		if err != nil {
			t.Fatal(err)
		}
		two, err := pages.Page(context.Background(), n)
		if err != nil {
			t.Fatal(err)
		}
		var a, b bytes.Buffer
		if err = one.WritePNG(context.Background(), &a, 96); err != nil {
			t.Fatal(err)
		}
		if err = two.WritePNG(context.Background(), &b, 96); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(a.Bytes(), b.Bytes()) {
			t.Errorf("page %d differs between PreparePage and Pages.Page", n)
		}
	}
	if _, err = PreparePage(context.Background(), d, 3, o.Options); !errors.Is(err, ErrPageOutOfRange) || !errors.Is(err, render.ErrInvalid) {
		t.Errorf("outside: %v", err)
	}
	after, err := d.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(readAllZipParts(t, before), readAllZipParts(t, after)) {
		t.Fatal("render changed source")
	}
}

func TestUnsavedEditsAreDrawn(t *testing.T) {
	d := docx.Create()
	d.DefaultSection().SetPageSize(69, 45)
	d.DefaultSection().SetMargins(docx.PageMargins{Top: 4.5, Bottom: 4.5, Left: 4.5, Right: 4.5})
	r := d.AddParagraph().AddRun()
	r.SetText("AA")
	o := newWordTestOpts(t, false)
	pages, err := Prepare(context.Background(), d, o.Options)
	if err != nil {
		t.Fatal(err)
	}
	if got := pages.texts(1); len(got) != 1 || got[0].text != "AA" {
		t.Fatalf("texts %+v", got)
	}
	r.SetText("BB")
	pages, err = Prepare(context.Background(), d, o.Options)
	if err != nil {
		t.Fatal(err)
	}
	if got := pages.texts(1); len(got) != 1 || got[0].text != "BB" {
		t.Fatalf("after edit %+v", got)
	}
}

func TestPNGPixels(t *testing.T) {
	d := wordTestDoc(t, wordTestPara("", wordTestRun(`<w:highlight w:val="red"/>`, "WWWW"))+wordTestPage, wordTestParts{styles: wordTestStyles})
	o := newWordTestOpts(t, false)
	page, err := PreparePage(context.Background(), d, 1, o.Options)
	if err != nil {
		t.Fatal(err)
	}
	w, h, err := page.Size(96)
	if err != nil || w != 300 || h != 200 {
		t.Fatalf("size %d,%d: %v", w, h, err)
	}
	var out bytes.Buffer
	if err = page.WritePNG(context.Background(), &out, 96); err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(&out)
	if err != nil {
		t.Fatal(err)
	}
	at := func(x, y int) color.NRGBA { return color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA) }
	// Glyph ink is 100..900 units of 2000 at 12 px: the box of the first glyph
	// starts at x=20 and the highlight is red where there is no ink.
	if got := at(0, 0); got != (color.NRGBA{255, 255, 255, 255}) {
		t.Errorf("page corner %+v", got)
	}
	if got := at(22, 31); got.R != 255 || got.G > 16 {
		t.Errorf("highlight %+v", got)
	}
	if got := at(150, 150); got != (color.NRGBA{255, 255, 255, 255}) {
		t.Errorf("below the text %+v", got)
	}
}

func TestMarkupInTextIsNeverInterpreted(t *testing.T) {
	hostile := `</span></div><script>alert(1)</script><img src="x.png"> &amp; &lt;b&gt; <style>div{display:none}</style> "quote" 'q'`
	var esc bytes.Buffer
	if err := xml.EscapeText(&esc, []byte(hostile)); err != nil {
		t.Fatal(err)
	}
	body := wordTestPara("", wordTestRun("", esc.String()), wordTestRun(`<w:rFonts w:ascii="a;}&lt;/style&gt;&lt;div style=&quot;x"/>`, "family"))
	d := wordTestDoc(t, body+wordTestPage, wordTestParts{styles: wordTestStyles})
	o := newWordTestOpts(t, false)
	pages, err := Prepare(context.Background(), d, o.Options)
	if err != nil {
		t.Fatal(err)
	}
	var got strings.Builder
	for pg := 1; pg <= pages.Count(); pg++ {
		for _, tx := range pages.texts(pg) {
			got.WriteString(tx.text)
		}
	}
	// Every printable character survives literally, in order.
	want := strings.Join(strings.Fields(hostile), "") + "family"
	if strings.Join(strings.Fields(got.String()), "") != want {
		t.Errorf("drawn %q", got.String())
	}
	var sawFamily bool
	for _, r := range o.requests {
		sawFamily = sawFamily || r.Family == `a;}</style><div style="x`
	}
	if !sawFamily {
		t.Errorf("the family is passed to the resolver verbatim, not into markup: %+v", o.requests)
	}
}

func TestControlCharactersAreNotDrawnAsMarkup(t *testing.T) {
	d := wordTestDoc(t, wordTestPara("", `<w:r><w:t>a&#x7F;b&#x85;c&#x2028;d</w:t></w:r>`)+wordTestPage, wordTestParts{styles: wordTestStyles})
	o := newWordTestOpts(t, false)
	if _, err := Prepare(context.Background(), d, o.Options); err != nil {
		t.Fatal(err)
	}
}

func TestTabsUseDefaultStopsFromTheTextColumn(t *testing.T) {
	// Default stops every 48 px (720 twips): "a" ends at 6, the tab runs to 48.
	body := wordTestBody(
		wordTestPara("", `<w:r><w:t>a</w:t><w:tab/><w:t>b</w:t></w:r>`),
		wordTestPara("", `<w:r><w:t>aaaaaaaaaaaa</w:t><w:tab/><w:t>b</w:t></w:r>`),
		// A hanging indent puts a stop at the left indent: the text after the
		// tab lines up with the wrapped lines.
		wordTestPara(`<w:ind w:left="720" w:hanging="360"/>`, `<w:r><w:t>1.</w:t><w:tab/><w:t>b</w:t></w:r>`),
	)
	p, _ := wordTestRender(t, body)
	b1, _ := p.textOf(1, "b")
	if !near(b1.At.X.Px(), 20+48) {
		t.Errorf("first tab ends at %v", b1.At.X.Px())
	}
	l := p.lines(1)
	if len(l) != 3 {
		t.Fatalf("lines %+v", l)
	}
	var bx []float64
	for _, tx := range p.texts(1) {
		if tx.text == "b" {
			bx = append(bx, tx.x)
		}
	}
	// 12 characters take 72 px: the next stop after 72 is 96.
	if len(bx) != 3 || !near(bx[1], 20+96) || !near(bx[2], 20+48) {
		t.Errorf("tab targets %v", bx)
	}
}

func TestDefaultTabStopSettingIsHonoured(t *testing.T) {
	p, _ := wordTestRender(t, wordTestBody(wordTestPara("", `<w:r><w:t>a</w:t><w:tab/><w:t>b</w:t></w:r>`)),
		wordTestParts{styles: wordTestStyles, settings: `<w:defaultTabStop w:val="1500"/>`})
	b, _ := p.textOf(1, "b")
	if !near(b.At.X.Px(), 20+100) {
		t.Errorf("tab ends at %v", b.At.X.Px())
	}
}

func TestLongWordsBreakAtTheMargin(t *testing.T) {
	p, _ := wordTestRender(t, wordTestBody(wordTestPara("", wordTestRun("", strings.Repeat("W", 100)))))
	if n := len(p.lines(1)); n != 3 {
		t.Errorf("%d lines", n)
	}
}

func TestLigaturesAndKerningAreOffUnlessAsked(t *testing.T) {
	body := wordTestBody(wordTestPara("", wordTestRun("", "fi"), wordTestRun(`<w:kern w:val="2"/>`, "AV")))
	p, _ := wordTestRender(t, body)
	fi, _ := p.textOf(1, "fi")
	av, _ := p.textOf(1, "AV")
	if !fi.Features.NoOptionalLigatures || !fi.Features.NoKerning || !fi.Features.NoContextualAlternates {
		t.Errorf("default features %+v", fi.Features)
	}
	if av.Features.NoKerning {
		t.Errorf("kerning requested but off: %+v", av.Features)
	}
}

func TestBudgets(t *testing.T) {
	d := wordTestDoc(t, wordTestBody(wordTestPara("", wordTestRun("", "one")), wordTestPara("", wordTestRun("", "two"))), wordTestParts{styles: wordTestStyles})
	for name, mutate := range map[string]func(*render.Options){
		"source bytes": func(o *render.Options) { o.MaxSourceBytes = 64 },
		"layout nodes": func(o *render.Options) { o.MaxLayoutNodes = 8 },
		"fonts":        func(o *render.Options) { o.Limits.MaxFonts = 1 },
	} {
		t.Run(name, func(t *testing.T) {
			o := newWordTestOpts(t, false)
			if name == "fonts" {
				d = wordTestDoc(t, wordTestBody(wordTestPara("", wordTestRun(`<w:b/>`, "one"), wordTestRun("", "two"))), wordTestParts{styles: wordTestStyles})
			}
			mutate(&o.Options)
			if p, err := Prepare(context.Background(), d, o.Options); p != nil || !errors.Is(err, render.ErrLimit) {
				t.Fatalf("%v %v", p, err)
			}
		})
	}
	t.Run("text bytes", func(t *testing.T) {
		o := newWordTestOpts(t, false)
		o.Limits.MaxTextBytes = 4
		big := wordTestDoc(t, wordTestBody(wordTestPara("", wordTestRun("", strings.Repeat("x", 200)))), wordTestParts{styles: wordTestStyles})
		if p, err := Prepare(context.Background(), big, o.Options); p != nil || !errors.Is(err, render.ErrLimit) {
			t.Fatalf("%v %v", p, err)
		}
	})
	t.Run("shaping work", func(t *testing.T) {
		o := newWordTestOpts(t, false)
		o.Limits.MaxShapeWork = 1
		pages, err := Prepare(context.Background(), d, o.Options)
		if err != nil {
			t.Fatal(err)
		}
		if p, err := pages.Page(context.Background(), 1); p != nil || !errors.Is(err, render.ErrLimit) {
			t.Fatalf("%v %v", p, err)
		}
	})
}

func TestPageGeometryLimits(t *testing.T) {
	for name, sect := range map[string]string{
		"huge":     wordTestSect(2147483647, 2147483647, 300, 300, 300, 300, ""),
		"negative": wordTestSect(4500, 3000, -300, 300, 300, 300, ""),
		"margins":  wordTestSect(4500, 3000, 300, 3000, 300, 3000, ""),
		"empty":    wordTestSect(0, 0, 0, 0, 0, 0, ""),
	} {
		t.Run(name, func(t *testing.T) {
			o := newWordTestOpts(t, false)
			doc := wordTestDoc(t, wordTestPara("", wordTestRun("", "x"))+sect, wordTestParts{styles: wordTestStyles})
			if p, err := Prepare(context.Background(), doc, o.Options); p != nil || err == nil {
				t.Fatalf("%v %v", p, err)
			}
		})
	}
}

func TestCancelledContext(t *testing.T) {
	d := wordTestDoc(t, wordTestBody(wordTestPara("", wordTestRun("", "x"))), wordTestParts{styles: wordTestStyles})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	o := newWordTestOpts(t, false)
	if _, err := Prepare(ctx, d, o.Options); !errors.Is(err, context.Canceled) {
		t.Fatalf("%v", err)
	}
	if _, err := PreparePage(nil, d, 1, o.Options); !errors.Is(err, render.ErrInvalid) { //nolint:staticcheck // nil context is the case under test
		t.Fatalf("%v", err)
	}
	if _, err := PreparePage(context.Background(), nil, 1, o.Options); !errors.Is(err, render.ErrInvalid) {
		t.Fatalf("%v", err)
	}
	if _, err := PreparePage(context.Background(), d, 0, o.Options); !errors.Is(err, render.ErrInvalid) {
		t.Fatalf("%v", err)
	}
}

func TestPagesAreSafeForConcurrentUse(t *testing.T) {
	d := wordTestDoc(t, wordTestBody(wordTestPara("", wordTestRun("", wordTestLines(30)))), wordTestParts{styles: wordTestStyles})
	o := newWordTestOpts(t, false)
	pages, err := Prepare(context.Background(), d, o.Options)
	if err != nil {
		t.Fatal(err)
	}
	errs := make(chan error, pages.Count())
	for n := 1; n <= pages.Count(); n++ {
		go func() {
			p, err := pages.Page(context.Background(), n)
			if err == nil {
				var b bytes.Buffer
				err = p.WritePNG(context.Background(), &b, 96)
			}
			errs <- err
		}()
	}
	for n := 1; n <= pages.Count(); n++ {
		if err := <-errs; err != nil {
			t.Error(err)
		}
	}
}

func FuzzDocxRender(f *testing.F) {
	seedNumbering := numAbstract("0", "", numDecimal(0, "%1.", ""), numLvl(1, `<w:start w:val="1"/><w:numFmt w:val="lowerRoman"/><w:lvlText w:val="%1.%2"/>`)) + numInstance("1", "0")
	hdr := wordTestHeader(wordTestPara("", wordTestRun("", "H"), hfField("PAGE", "1"), hfField("NUMPAGES", "1")))
	ftr := wordTestFooter(wordTestPara("", wordTestRun("", "F"), `<w:r><w:pgNum/></w:r>`))
	notes := fnPart("footnote", fnNote("footnote", "1", "note"), fnNote("footnote", "2", "a", "b"))
	ends := fnPart("endnote", fnNote("endnote", "1", "end"))
	seed := wordTestDoc(f, wordTestPara("", wordTestRun("", "AA"), fnRef("1"), enRef("1"))+numPara("1", 0, "AA")+numPara("1", 1, "AA")+hfSect(hfRef("header", "default", hfRIDHeader1)+hfRef("footer", "default", hfRIDFooter1), 0, 0),
		wordTestParts{styles: wordTestStyles, numbering: seedNumbering, extra: map[string]wordTestExtra{"word/header1.xml": hdr, "word/footer1.xml": ftr, "word/footnotes.xml": notes, "word/endnotes.xml": ends}})
	valid, err := seed.SaveBytes()
	if err != nil {
		f.Fatal(err)
	}
	face := wordTestFace(f)
	const part, stylesPart, hdrPart, notesPart, numberingPart = "word/document.xml", "word/styles.xml", "word/header1.xml", "word/footnotes.xml", "word/numbering.xml"
	styles := fuzzseed.ZipEntry(valid, stylesPart)
	header := fuzzseed.ZipEntry(valid, hdrPart)
	noteData := fuzzseed.ZipEntry(valid, notesPart)
	numbering := fuzzseed.ZipEntry(valid, numberingPart)
	// Every seed below carries the seed document's numbering part.
	add := func(data, styles, hdr, notes []byte) { f.Add(data, styles, hdr, notes, numbering) }
	add(fuzzseed.ZipEntry(valid, part), styles, header, noteData)
	add([]byte(`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>AA</w:t></w:r></w:p><w:sectPr><w:pgSz w:w="4500" w:h="3000"/><w:pgMar w:top="300" w:right="300" w:bottom="300" w:left="300"/></w:sectPr></w:body></w:document>`), styles, header, noteData)
	add([]byte(`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:sectPr><w:pgSz w:w="2147483647" w:h="2147483647"/></w:sectPr></w:body></w:document>`), styles, header, noteData)
	add([]byte(`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:pPr><w:pStyle w:val="A"/><w:keepNext/><w:ind w:left="9999999" w:hanging="1"/><w:sectPr><w:pgSz w:w="4500" w:h="3000"/><w:pgMar w:top="300" w:right="300" w:bottom="300" w:left="300"/></w:sectPr></w:pPr><w:r><w:rPr><w:u w:val="double"/><w:vertAlign w:val="superscript"/></w:rPr><w:t>A</w:t><w:br w:type="page"/><w:tab/><w:t>B</w:t></w:r></w:p><w:sectPr><w:pgSz w:w="4500" w:h="3000"/><w:pgMar w:top="300" w:right="300" w:bottom="300" w:left="300"/></w:sectPr></w:body></w:document>`),
		[]byte(`<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:style w:type="paragraph" w:styleId="A"><w:basedOn w:val="A"/></w:style></w:styles>`), header, noteData)
	add([]byte(`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:pPr><w:tabs><w:tab w:val="right" w:leader="dot" w:pos="3000"/><w:tab w:val="center" w:pos="1500"/><w:tab w:val="clear" w:pos="1500"/></w:tabs><w:ind w:left="361" w:hanging="200"/></w:pPr><w:r><w:t>A</w:t><w:tab/><w:t>B</w:t><w:br/><w:tab/><w:t>C</w:t><w:tab/></w:r></w:p><w:sectPr><w:pgSz w:w="4500" w:h="3000"/><w:pgMar w:top="300" w:right="300" w:bottom="300" w:left="300"/></w:sectPr></w:body></w:document>`), styles, header, noteData)
	// Headers, footers and page numbering.
	hdrDoc := func(extra string) []byte {
		return []byte(`<w:document ` + wordTestNS + `><w:body><w:p><w:r><w:t>A</w:t></w:r></w:p><w:p><w:pPr><w:pageBreakBefore/></w:pPr><w:r><w:t>B</w:t></w:r></w:p>` + hfSect(extra, 6, 6) + `</w:body></w:document>`)
	}
	add(hdrDoc(hfRef("header", "default", hfRIDHeader1)+hfRef("header", "first", hfRIDHeader1)+hfRef("header", "even", hfRIDHeader1)+hfRef("footer", "default", hfRIDFooter1)+`<w:titlePg/><w:pgNumType w:start="-3" w:fmt="upperRoman"/>`), styles, header, noteData)
	add(hdrDoc(hfRef("header", "default", hfRIDHeader1)+`<w:pgNumType w:fmt="lowerLetter" w:start="2147483647"/>`), styles, []byte(`<w:hdr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:p><w:fldSimple w:instr="PAGE \* ROMAN \# 0"><w:r><w:t>1</w:t></w:r></w:fldSimple><w:r><w:fldChar w:fldCharType="begin"/><w:instrText>NUMPAGES</w:instrText><w:fldChar w:fldCharType="end"/></w:r></w:p></w:hdr>`), noteData)
	add(hdrDoc(hfRef("header", "default", "rId999")+hfRef("footer", "default", hfRIDHeader1)), styles, header, noteData)
	// Footnotes and endnotes: references, numbering restarts and formats, a
	// note that continues over pages.
	noteDoc := func(sect, body string) []byte {
		return []byte(`<w:document ` + wordTestNS + `><w:body>` + body + hfSect(sect, 0, 0) + `</w:body></w:document>`)
	}
	add(noteDoc(`<w:footnotePr><w:numRestart w:val="eachPage"/><w:numFmt w:val="chicago"/><w:numStart w:val="-2"/></w:footnotePr><w:endnotePr><w:pos w:val="sectEnd"/></w:endnotePr>`,
		fnPara(14, fnRef("1"), fnRef("2"), enRef("1"))+fnPara(3, fnRef("2"), `<w:r><w:footnoteReference w:id="9" w:customMarkFollows="1"/><w:t>*</w:t></w:r>`)), styles, header, noteData)
	add(noteDoc(`<w:footnotePr><w:numRestart w:val="eachSect"/></w:footnotePr>`, fnPara(40, fnRef("2"), fnRef("1"), fnRef("1"))), styles, header,
		[]byte(`<w:footnotes xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:footnote w:type="separator" w:id="-1"><w:p><w:r><w:separator/></w:r></w:p></w:footnote><w:footnote w:id="1"><w:p><w:r><w:footnoteRef/><w:footnoteReference w:id="1"/></w:r></w:p></w:footnote></w:footnotes>`))
	// Numbering: a list through paragraph styles and a numbering style that
	// links to itself, levels with every suffix, alignment and format, and
	// overrides.
	const nsDecl = `xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"`
	listBody := `<w:document ` + nsDecl + `><w:body><w:p><w:pPr><w:pStyle w:val="L"/></w:pPr><w:r><w:t>A</w:t></w:r></w:p><w:p><w:pPr><w:numPr><w:ilvl w:val="1"/><w:numId w:val="2"/></w:numPr><w:ind w:left="40"/></w:pPr><w:r><w:t>B</w:t></w:r></w:p><w:p><w:pPr><w:numPr><w:ilvl w:val="8"/><w:numId w:val="1"/></w:numPr></w:pPr></w:p><w:sectPr><w:pgSz w:w="4500" w:h="3000"/><w:pgMar w:top="300" w:right="300" w:bottom="300" w:left="300"/></w:sectPr></w:body></w:document>`
	listStyles := `<w:styles ` + nsDecl + `><w:style w:type="paragraph" w:styleId="L"><w:pPr><w:numPr><w:numId w:val="1"/></w:numPr></w:pPr></w:style><w:style w:type="numbering" w:styleId="N"><w:pPr><w:numPr><w:numId w:val="3"/></w:numPr></w:pPr></w:style></w:styles>`
	listNumbering := `<w:numbering ` + nsDecl + `><w:abstractNum w:abstractNumId="0"><w:lvl w:ilvl="0"><w:start w:val="2"/><w:numFmt w:val="upperRoman"/><w:suff w:val="space"/><w:lvlText w:val="%1)"/><w:lvlJc w:val="right"/><w:pStyle w:val="L"/><w:pPr><w:ind w:left="720" w:hanging="360"/></w:pPr></w:lvl><w:lvl w:ilvl="1"><w:numFmt w:val="cardinalText"/><w:isLgl/><w:lvlRestart w:val="0"/><w:lvlText w:val="%1.%2.%9"/><w:lvlJc w:val="center"/></w:lvl><w:lvl w:ilvl="2"><w:numFmt w:val="bullet"/><w:lvlText w:val="&#xF0B7;"/><w:rPr><w:rFonts w:ascii="Symbol"/></w:rPr></w:lvl></w:abstractNum><w:abstractNum w:abstractNumId="1"><w:numStyleLink w:val="N"/></w:abstractNum><w:num w:numId="1"><w:abstractNumId w:val="0"/></w:num><w:num w:numId="2"><w:abstractNumId w:val="0"/><w:lvlOverride w:ilvl="1"><w:startOverride w:val="7"/></w:lvlOverride></w:num><w:num w:numId="3"><w:abstractNumId w:val="1"/></w:num></w:numbering>`
	f.Add([]byte(listBody), []byte(listStyles), header, noteData, []byte(listNumbering))
	f.Add([]byte(listBody), []byte(listStyles), header, noteData, []byte(`<w:numbering `+nsDecl+`><w:abstractNum w:abstractNumId="0"><w:lvl w:ilvl="0"><w:start w:val="32767"/><w:numFmt w:val="lowerLetter"/><w:lvlText w:val="%1%1%1"/></w:lvl></w:abstractNum><w:num w:numId="1"><w:abstractNumId w:val="0"/></w:num></w:numbering>`))
	tableDoc := func(body string) []byte {
		return []byte(`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>` + body + `<w:sectPr><w:pgSz w:w="4500" w:h="3000"/><w:pgMar w:top="300" w:right="300" w:bottom="300" w:left="300"/></w:sectPr></w:body></w:document>`)
	}
	tableStyles := []byte(`<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:style w:type="table" w:default="1" w:styleId="T"><w:tblPr><w:tblCellMar><w:left w:w="108" w:type="dxa"/></w:tblCellMar></w:tblPr><w:tblStylePr w:type="firstRow"><w:rPr><w:b/></w:rPr><w:tcPr><w:shd w:val="clear" w:fill="CCCCCC"/></w:tcPr></w:tblStylePr><w:tblStylePr w:type="band1Horz"><w:tcPr><w:tcBorders><w:bottom w:val="double" w:sz="4"/></w:tcBorders></w:tcPr></w:tblStylePr></w:style></w:styles>`)
	f.Add(tableDoc(`<w:tbl><w:tblPr><w:tblStyle w:val="T"/><w:tblW w:w="3000" w:type="dxa"/><w:jc w:val="center"/><w:tblBorders><w:top w:val="single" w:sz="8"/><w:insideH w:val="dashed" w:sz="4"/><w:insideV w:val="single" w:sz="4"/></w:tblBorders><w:tblLook w:val="04A0"/></w:tblPr><w:tblGrid><w:gridCol w:w="1000"/><w:gridCol w:w="1000"/><w:gridCol w:w="1000"/></w:tblGrid><w:tr><w:trPr><w:tblHeader/><w:cantSplit/></w:trPr><w:tc><w:tcPr><w:gridSpan w:val="2"/><w:shd w:val="clear" w:fill="FF0000"/></w:tcPr><w:p><w:r><w:t>A</w:t></w:r></w:p></w:tc><w:tc><w:p/></w:tc></w:tr><w:tr><w:trPr><w:trHeight w:val="600" w:hRule="exact"/></w:trPr><w:tc><w:tcPr><w:vMerge w:val="restart"/><w:vAlign w:val="center"/></w:tcPr><w:p><w:r><w:t>B</w:t></w:r></w:p></w:tc><w:tc><w:p><w:r><w:t>C</w:t><w:tab/><w:t>D</w:t></w:r></w:p></w:tc><w:tc><w:p/></w:tc></w:tr><w:tr><w:tc><w:tcPr><w:vMerge/></w:tcPr><w:p/></w:tc><w:tc><w:p/></w:tc><w:tc><w:p/></w:tc></w:tr></w:tbl>`), tableStyles, header, noteData, numbering)
	f.Add(tableDoc(`<w:tbl><w:tblPr><w:tblLayout w:type="fixed"/><w:tblCellSpacing w:w="20"/><w:tblpPr w:tblpX="10"/></w:tblPr><w:tblGrid><w:gridCol w:w="800"/></w:tblGrid><w:tr><w:trPr><w:gridBefore w:val="1"/><w:gridAfter w:val="2"/></w:trPr><w:tc><w:tcPr><w:tcW w:w="2000" w:type="dxa"/><w:tcBorders><w:tl2br w:val="single"/><w:left w:val="wave" w:sz="96"/></w:tcBorders><w:tcMar><w:left w:w="99999"/></w:tcMar></w:tcPr><w:tbl><w:tr><w:tc><w:p><w:r><w:t>N</w:t></w:r></w:p></w:tc></w:tr></w:tbl><w:p/></w:tc><w:tc><w:tcPr><w:hMerge w:val="continue"/></w:tcPr><w:p><w:r><w:t>H</w:t></w:r></w:p></w:tc></w:tr><w:tr><w:tc><w:tcPr><w:vMerge/><w:gridSpan w:val="9999"/></w:tcPr><w:p><w:pPr><w:sectPr/></w:pPr></w:p></w:tc></w:tr></w:tbl><w:p/>`), tableStyles, header, noteData, numbering)
	f.Add(tableDoc(`<w:tbl><w:tblGrid/><w:tr><w:tc><w:p/></w:tc></w:tr></w:tbl>`), []byte(`<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:style w:type="table" w:styleId="A"><w:basedOn w:val="A"/></w:style></w:styles>`), header, noteData, numbering)
	f.Fuzz(func(t *testing.T, data, stylesData, hdrData, notesData, numberingData []byte) {
		if len(data) > 65536 || len(stylesData) > 65536 || len(hdrData) > 65536 || len(notesData) > 65536 || len(numberingData) > 65536 {
			t.Skip()
		}
		wrapped := fuzzseed.ReplaceZipEntry(valid, part, data)
		if wrapped == nil {
			t.Fatal("seed")
		}
		wrapped = fuzzseed.ReplaceZipEntry(wrapped, stylesPart, stylesData)
		if wrapped == nil {
			t.Fatal("seed")
		}
		if wrapped = fuzzseed.ReplaceZipEntry(wrapped, hdrPart, hdrData); wrapped == nil {
			t.Fatal("seed")
		}
		if wrapped = fuzzseed.ReplaceZipEntry(wrapped, notesPart, notesData); wrapped == nil {
			t.Fatal("seed")
		}
		if wrapped = fuzzseed.ReplaceZipEntry(wrapped, numberingPart, numberingData); wrapped == nil {
			t.Fatal("seed")
		}
		doc, err := docx.OpenReader(bytes.NewReader(wrapped), int64(len(wrapped)))
		if err != nil {
			return
		}
		defer func() {
			if e := doc.Close(); e != nil {
				t.Error(e)
			}
		}()
		for _, lenient := range []bool{false, true} {
			opts := render.Options{Fonts: func(context.Context, render.FontRequest) (*shape.Face, error) { return face, nil }, MaxSourceBytes: 65536, MaxLayoutNodes: 512, Limits: render.Limits{MaxDimension: 256, MaxPixels: 65536, MaxOperations: 128, MaxGlyphs: 256, MaxShapeWork: 65536, MaxOutputBytes: 65536, MaxTextBytes: 4096, MaxRunBytes: 1024}}
			if lenient {
				opts.Warn = func(error) {}
			}
			pages, err := Prepare(context.Background(), doc, opts)
			if err != nil {
				if pages != nil {
					t.Fatal("partial result")
				}
				continue
			}
			for n := 1; n <= pages.Count() && n <= 3; n++ {
				page, err := pages.Page(context.Background(), n)
				if err != nil {
					continue
				}
				var out bytes.Buffer
				if err = page.WritePNG(context.Background(), &out, 96); err != nil && !errors.Is(err, render.ErrLimit) {
					t.Fatal(err)
				}
			}
		}
	})
}

func BenchmarkRenderPhysicalPages(b *testing.B) {
	var body strings.Builder
	for i := 0; i < 200; i++ {
		body.WriteString(wordTestPara(`<w:spacing w:before="60" w:after="60"/>`, wordTestRun(`<w:b/>`, "Heading "), wordTestRun("", strings.Repeat("word ", 40))))
	}
	body.WriteString(wordTestPage)
	d := wordTestDoc(b, body.String(), wordTestParts{styles: wordTestStyles})
	o := newWordTestOpts(b, false)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pages, err := Prepare(context.Background(), d, o.Options)
		if err != nil {
			b.Fatal(err)
		}
		page, err := pages.Page(context.Background(), 1)
		if err != nil {
			b.Fatal(err)
		}
		var out bytes.Buffer
		if err = page.WritePNG(context.Background(), &out, 96); err != nil {
			b.Fatal(err)
		}
	}
}
