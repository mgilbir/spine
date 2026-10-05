package docxrender

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mgilbir/forme/layout"
	"github.com/mgilbir/forme/style"
	"github.com/mgilbir/spine/render"
)

func (p *Pages) rects(page int) []layout.FillRect {
	var out []layout.FillRect
	for _, op := range p.pages[page-1].ops[1:] { // the first op is the page background
		if v, ok := op.(layout.FillRect); ok {
			out = append(out, v)
		}
	}
	return out
}

func (p *Pages) textOf(page int, text string) (layout.DrawText, bool) {
	for _, op := range p.pages[page-1].ops {
		if v, ok := op.(layout.DrawText); ok && v.Text == text {
			return v, true
		}
	}
	return layout.DrawText{}, false
}

func rgbOf(c style.RGBA) string { return fmt.Sprintf("%02x%02x%02x", int(c.R), int(c.G), int(c.B)) }

func TestRunFontsSizeAndSlope(t *testing.T) {
	body := wordTestBody(wordTestPara("",
		wordTestRun(`<w:b/>`, "bold"),
		wordTestRun(`<w:i/>`, "ital"),
		wordTestRun(`<w:rFonts w:ascii="Other"/><w:sz w:val="36"/>`, "big"),
	))
	p, o := wordTestRender(t, body)
	want := map[render.FontRequest]bool{
		{Family: "Fix", Bold: true}: true, {Family: "Fix", Italic: true}: true, {Family: "Other"}: true, {Family: "Fix"}: true,
	}
	for _, r := range o.requests {
		if !want[r] {
			t.Errorf("unexpected request %+v", r)
		}
	}
	// The paragraph mark is not drawn and, beside text, sizes nothing, so its
	// own font is not asked for.
	if len(o.requests) != 3 {
		t.Errorf("requests %+v", o.requests)
	}
	big, ok := p.textOf(1, "big")
	if !ok || big.Size.Px() != 24 {
		t.Errorf("big run %+v", big.Size)
	}
}

func TestRunColourAndThemeColour(t *testing.T) {
	theme := `<a:theme xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><a:themeElements><a:clrScheme name="x"><a:dk1><a:sysClr val="windowText" lastClr="000000"/></a:dk1><a:lt1><a:sysClr val="window" lastClr="FFFFFF"/></a:lt1><a:dk2><a:srgbClr val="44546A"/></a:dk2><a:lt2><a:srgbClr val="E7E6E6"/></a:lt2><a:accent1><a:srgbClr val="4472C4"/></a:accent1></a:clrScheme><a:fontScheme name="f"><a:majorFont><a:latin typeface="MajorLatin"/><a:ea typeface=""/><a:cs typeface=""/></a:majorFont><a:minorFont><a:latin typeface="MinorLatin"/><a:ea typeface="MinorEA"/><a:cs typeface="MinorCS"/></a:minorFont></a:fontScheme></a:themeElements></a:theme>`
	body := wordTestBody(wordTestPara("",
		wordTestRun(`<w:color w:val="FF0000"/>`, "red"),
		wordTestRun(`<w:color w:val="000000" w:themeColor="accent1" w:themeShade="BF"/>`, "shade"),
		wordTestRun(`<w:color w:val="000000" w:themeColor="accent1"/>`, "plain"),
		wordTestRun(`<w:color w:val="auto"/>`, "auto"),
		wordTestRun(`<w:rFonts w:asciiTheme="majorHAnsi"/>`, "major"),
	))
	p, o := wordTestRender(t, body, wordTestParts{styles: wordTestStyles, theme: theme})
	for text, want := range map[string]string{"red": "ff0000", "shade": "2f5496", "plain": "4472c4", "auto": "000000"} {
		v, ok := p.textOf(1, text)
		if !ok || rgbOf(v.Color) != want {
			t.Errorf("%s: %v %v", text, rgbOf(v.Color), want)
		}
	}
	found := false
	for _, r := range o.requests {
		found = found || r.Family == "MajorLatin"
	}
	if !found {
		t.Errorf("theme font not requested: %+v", o.requests)
	}
}

func TestRunDecorations(t *testing.T) {
	body := wordTestBody(wordTestPara("",
		wordTestRun(`<w:u w:val="single"/>`, "under"),
		wordTestRun(`<w:strike/>`, "strike"),
		wordTestRun(`<w:highlight w:val="yellow"/>`, "mark"),
		wordTestRun(`<w:shd w:val="clear" w:color="auto" w:fill="00FF00"/>`, "shade"),
		wordTestRun(`<w:u w:val="double" w:color="0000FF"/><w:strike/>`, "both"),
	))
	p, _ := wordTestRender(t, body)
	var yellow, green, blue int
	for _, r := range p.rects(1) {
		switch rgbOf(r.Color) {
		case "ffff00":
			yellow++
		case "00ff00":
			green++
		case "0000ff":
			blue++
		}
	}
	if yellow != 1 || green != 1 || blue == 0 {
		t.Errorf("fills: yellow %d green %d blue %d (%d rects)", yellow, green, blue, len(p.rects(1)))
	}
	// Under, strike (two lines: one under a run with both) and the double underline.
	if len(p.rects(1)) < 5 {
		t.Errorf("only %d rects", len(p.rects(1)))
	}
}

func TestRunCapsAndScripts(t *testing.T) {
	body := wordTestBody(wordTestPara("",
		wordTestRun(`<w:caps/>`, "caps"),
		wordTestRun(`<w:vertAlign w:val="superscript"/>`, "sup"),
		wordTestRun(`<w:vertAlign w:val="subscript"/>`, "sub"),
		wordTestRun(`<w:position w:val="6"/>`, "raised"),
		wordTestRun("", "base"),
	))
	p, _ := wordTestRender(t, body)
	if _, ok := p.textOf(1, "CAPS"); !ok {
		t.Error("caps not uppercased")
	}
	base, _ := p.textOf(1, "base")
	sup, _ := p.textOf(1, "sup")
	sub, _ := p.textOf(1, "sub")
	raised, _ := p.textOf(1, "raised")
	if sup.At.Y >= base.At.Y || sub.At.Y <= base.At.Y {
		t.Errorf("baselines sup %v sub %v base %v", sup.At.Y.Px(), sub.At.Y.Px(), base.At.Y.Px())
	}
	if sup.Size.Px() >= 12 || sub.Size.Px() >= 12 {
		t.Errorf("script sizes %v %v", sup.Size.Px(), sub.Size.Px())
	}
	if d := base.At.Y.Px() - raised.At.Y.Px(); !near(d, 4) {
		t.Errorf("raised by %v, want 4", d)
	}
}

func TestHiddenRunsAreNotDrawn(t *testing.T) {
	p, _ := wordTestRender(t, wordTestBody(wordTestPara("", wordTestRun("", "shown "), wordTestRun(`<w:vanish/>`, "hidden"), wordTestRun("", "also"))))
	for _, tx := range p.texts(1) {
		if strings.Contains(tx.text, "hidden") {
			t.Fatal("hidden text drawn")
		}
	}
}

func TestSpacePreservationAndTrim(t *testing.T) {
	body := wordTestBody(wordTestPara("",
		`<w:r><w:t> trimmed </w:t></w:r>`,
		`<w:r><w:t xml:space="preserve">  kept  </w:t></w:r>`,
	))
	p, _ := wordTestRender(t, body)
	got := ""
	for _, tx := range p.texts(1) {
		got += tx.text
	}
	if got != "trimmed  kept  " {
		t.Errorf("text %q", got)
	}
}

func TestNonASCIIAndEastAsianSlot(t *testing.T) {
	body := wordTestBody(wordTestPara("",
		wordTestRun(`<w:rFonts w:ascii="Latin" w:hAnsi="HAnsi" w:eastAsia="East"/>`, "aé日"),
	))
	p, o := wordTestRender(t, body)
	families := map[string]bool{}
	for _, r := range o.requests {
		families[r.Family] = true
	}
	for _, f := range []string{"Latin", "HAnsi", "East"} {
		if !families[f] {
			t.Errorf("family %s not requested: %v", f, families)
		}
	}
	if _, ok := p.textOf(1, "日"); !ok {
		t.Error("CJK character not drawn")
	}
}

func TestFieldsDrawCachedResults(t *testing.T) {
	body := wordTestBody(
		wordTestPara("",
			wordTestRun("", "page "),
			`<w:r><w:fldChar w:fldCharType="begin"/></w:r>`,
			`<w:r><w:instrText xml:space="preserve"> PAGE </w:instrText></w:r>`,
			`<w:r><w:fldChar w:fldCharType="separate"/></w:r>`,
			wordTestRun("", "7"),
			`<w:r><w:fldChar w:fldCharType="end"/></w:r>`,
			`<w:fldSimple w:instr=" NUMPAGES "><w:r><w:t>9</w:t></w:r></w:fldSimple>`,
			`<w:hyperlink r:id="rId9"><w:r><w:t>link</w:t></w:r></w:hyperlink>`,
		),
	)
	p, _ := wordTestRender(t, body)
	got := ""
	for _, tx := range p.texts(1) {
		got += tx.text
	}
	if got != "page 79link" || strings.Contains(got, "PAGE") {
		t.Errorf("text %q", got)
	}
}

func TestFieldAcrossParagraphs(t *testing.T) {
	body := wordTestBody(
		wordTestPara("", `<w:r><w:fldChar w:fldCharType="begin"/></w:r><w:r><w:instrText>TOC</w:instrText></w:r><w:r><w:fldChar w:fldCharType="separate"/></w:r>`, wordTestRun("", "entry one")),
		wordTestPara("", wordTestRun("", "entry two"), `<w:r><w:fldChar w:fldCharType="end"/></w:r>`),
		wordTestPara("", wordTestRun("", "after")),
	)
	p, _ := wordTestRender(t, body)
	if n := len(p.lines(1)); n != 3 {
		t.Errorf("%d lines", n)
	}
}

func TestStyleCascade(t *testing.T) {
	styles := wordTestStyles + `
<w:style w:type="paragraph" w:styleId="Base"><w:name w:val="Base"/><w:basedOn w:val="Normal"/><w:pPr><w:ind w:left="300"/><w:spacing w:after="150"/></w:pPr><w:rPr><w:sz w:val="24"/><w:b/></w:rPr></w:style>
<w:style w:type="paragraph" w:styleId="Derived"><w:name w:val="Derived"/><w:basedOn w:val="Base"/><w:pPr><w:spacing w:before="150"/></w:pPr><w:rPr><w:i/></w:rPr></w:style>
<w:style w:type="character" w:styleId="Strong"><w:name w:val="Strong"/><w:rPr><w:b/></w:rPr></w:style>
<w:style w:type="character" w:styleId="Big"><w:name w:val="Big"/><w:rPr><w:sz w:val="48"/></w:rPr></w:style>`
	body := wordTestBody(
		wordTestPara(`<w:pStyle w:val="Derived"/>`, wordTestRun("", "derived"), wordTestRun(`<w:rStyle w:val="Strong"/>`, "strong"), wordTestRun(`<w:rStyle w:val="Big"/>`, "big"), wordTestRun(`<w:sz w:val="18"/>`, "direct")),
	)
	p, o := wordTestRender(t, body, wordTestParts{styles: styles})
	d, _ := p.textOf(1, "derived")
	if d.Size.Px() != 16 || !near(d.At.X.Px(), 20+20) {
		t.Errorf("derived size %v x %v", d.Size.Px(), d.At.X.Px())
	}
	// The paragraph style is bold italic; "strong" is bold in the character style
	// as well, and the toggle cancels it.
	var sawBoldItalic, sawItalicOnly bool
	for _, r := range o.requests {
		sawBoldItalic = sawBoldItalic || (r.Bold && r.Italic)
		sawItalicOnly = sawItalicOnly || (!r.Bold && r.Italic)
	}
	if !sawItalicOnly || !sawBoldItalic {
		t.Errorf("requests %+v", o.requests)
	}
	if big, _ := p.textOf(1, "big"); big.Size.Px() != 32 {
		t.Errorf("char style size %v", big.Size.Px())
	}
	if direct, _ := p.textOf(1, "direct"); direct.Size.Px() != 12 {
		t.Errorf("direct size %v", direct.Size.Px())
	}
}

func TestDefaultParagraphStyleAndDocDefaults(t *testing.T) {
	styles := `<w:docDefaults><w:rPrDefault><w:rPr><w:rFonts w:ascii="Dflt"/><w:sz w:val="24"/></w:rPr></w:rPrDefault><w:pPrDefault><w:pPr><w:spacing w:after="300" w:line="480" w:lineRule="auto"/></w:pPr></w:pPrDefault></w:docDefaults>
<w:style w:type="paragraph" w:default="1" w:styleId="Normal"><w:name w:val="Normal"/><w:rPr><w:color w:val="00AA00"/></w:rPr></w:style>`
	p, o := wordTestRender(t, wordTestBody(wordTestPara("", wordTestRun("", "one")), wordTestPara("", wordTestRun("", "two"))), wordTestParts{styles: styles})
	l := p.lines(1)
	// 16 px text, double spaced: 32 px lines, 20 px after.
	if !near(l[1].y-l[0].y, 32+20) {
		t.Errorf("gap %v", l[1].y-l[0].y)
	}
	one, _ := p.textOf(1, "one")
	if rgbOf(one.Color) != "00aa00" || one.Size.Px() != 16 {
		t.Errorf("run %v %v", rgbOf(one.Color), one.Size.Px())
	}
	if o.requests[0].Family != "Dflt" {
		t.Errorf("font %+v", o.requests)
	}
}

func TestStyleCycleIsInvalid(t *testing.T) {
	styles := wordTestStyles + `<w:style w:type="paragraph" w:styleId="A"><w:name w:val="A"/><w:basedOn w:val="B"/></w:style><w:style w:type="paragraph" w:styleId="B"><w:name w:val="B"/><w:basedOn w:val="A"/></w:style>`
	o := newWordTestOpts(t, false)
	doc := wordTestDoc(t, wordTestBody(wordTestPara(`<w:pStyle w:val="A"/>`, wordTestRun("", "x"))), wordTestParts{styles: styles})
	if _, err := Prepare(t.Context(), doc, o.Options); err == nil {
		t.Fatal("expected an error")
	}
	// An unreferenced cycle is ignored.
	doc = wordTestDoc(t, wordTestBody(wordTestPara("", wordTestRun("", "x"))), wordTestParts{styles: styles})
	if _, err := Prepare(t.Context(), doc, o.Options); err != nil {
		t.Fatal(err)
	}
}
