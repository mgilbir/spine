package docxrender

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/mgilbir/forme/shape"
	"github.com/mgilbir/spine/render"
)

// policyCase is content the foundation profile does not draw exactly. In strict
// mode it fails with ErrUnsupported; in best-effort mode it is reported with
// the named kind and the surrounding paragraphs still draw.
type policyCase struct {
	name string
	// body is placed between a paragraph "before" and a paragraph "after".
	body string
	// sect replaces the section properties when set.
	sect string
	// parts replaces the styles/settings parts when set.
	parts *wordTestParts
	// warn is a substring of the best-effort warning, and approx says it wraps
	// ErrApproximated rather than ErrUnsupported.
	warn   string
	approx bool
	// text is what remains drawn on the page for the paragraph carrying the
	// feature, or "" when the content is left out entirely.
	text string
}

func policyCases() []policyCase {
	return []policyCase{
		{name: "unknown table property", body: `<w:tbl><w:tblPr><w:futureTableProp/></w:tblPr><w:tblGrid><w:gridCol w:w="1000"/></w:tblGrid><w:tr><w:tc><w:p><w:r><w:t>cell</w:t></w:r></w:p></w:tc></w:tr></w:tbl>`, warn: "futureTableProp", text: "cell"},
		{name: "number format outside the profile", body: numPara("1", 0, "item"), parts: &wordTestParts{styles: wordTestStyles, numbering: numAbstract("0", "", numLvl(0, `<w:numFmt w:val="japaneseCounting"/><w:start w:val="1"/><w:lvlText w:val="%1."/>`)) + numInstance("1", "0")}, warn: "number format japaneseCounting", approx: true, text: "item"},
		{name: "numbering level property", body: numPara("1", 0, "item"), parts: &wordTestParts{styles: wordTestStyles, numbering: numAbstract("0", "", numLvl(0, `<w:start w:val="1"/><w:lvlText w:val="%1."/><w:futureLevelProp/>`)) + numInstance("1", "0")}, warn: "futureLevelProp", text: "item"},
		{name: "drawing", body: wordTestPara("", `<w:r><w:drawing/></w:r>`, wordTestRun("", "next")), warn: "drawings and images", text: "next"},
		{name: "alternate content", body: wordTestPara("", `<mc:AlternateContent><mc:Choice Requires="wps"/></mc:AlternateContent>`, wordTestRun("", "next")), warn: "alternate content", text: "next"},
		{name: "footnote reference", body: wordTestPara("", wordTestRun("", "note"), `<w:r><w:footnoteReference w:id="2"/></w:r>`), warn: "footnote that the document does not have", text: "note"},
		{name: "comment", body: wordTestPara("", `<w:commentRangeStart w:id="0"/>`, wordTestRun("", "commented"), `<w:commentRangeEnd w:id="0"/>`, `<w:r><w:commentReference w:id="0"/></w:r>`), warn: "comments", text: "commented"},
		{name: "equation", body: wordTestPara("", `<m:oMath><m:r><m:t>x</m:t></m:r></m:oMath>`, wordTestRun("", "tail")), warn: "equations", text: "tail"},
		{name: "header part that is missing", sect: wordTestSect(4500, 3000, 300, 300, 300, 300, `<w:headerReference w:type="default" r:id="rId9"/>`), warn: "header or footer part"},
		{name: "page borders", sect: wordTestSect(4500, 3000, 300, 300, 300, 300, `<w:pgBorders/>`), warn: "page borders"},
		{name: "columns", sect: wordTestSect(4500, 3000, 300, 300, 300, 300, `<w:cols w:num="2"/>`), warn: "multiple text columns", approx: true},
		{name: "decimal tab stops", body: wordTestPara(`<w:tabs><w:tab w:val="decimal" w:pos="3000"/></w:tabs>`, `<w:r><w:t>a</w:t><w:tab/><w:t>b</w:t></w:r>`), warn: "decimal tab stops", approx: true, text: "b"},
		{name: "bar tab stops", body: wordTestPara(`<w:tabs><w:tab w:val="bar" w:pos="3000"/></w:tabs>`, `<w:r><w:t>a</w:t><w:tab/><w:t>b</w:t></w:r>`), warn: "bar tab stops", approx: true, text: "b"},
		{name: "heavy tab leader", body: wordTestPara(`<w:tabs><w:tab w:val="left" w:leader="heavy" w:pos="3000"/></w:tabs>`, `<w:r><w:t>a</w:t><w:tab/><w:t>b</w:t></w:r>`), warn: "tab leader heavy", approx: true, text: "b"},
		{name: "small caps", body: wordTestPara("", wordTestRun(`<w:smallCaps/>`, "small")), warn: "smallCaps", approx: true},
		{name: "character spacing", body: wordTestPara("", wordTestRun(`<w:spacing w:val="20"/>`, "spaced")), warn: "character spacing", approx: true, text: "spaced"},
		{name: "dotted underline", body: wordTestPara("", wordTestRun(`<w:u w:val="dotted"/>`, "dots")), warn: "underline style dotted", approx: true, text: "dots"},
		{name: "text effect", body: wordTestPara("", wordTestRun(`<w:effect w:val="shimmer"/>`, "glow")), warn: "w:effect", text: "glow"},
		{name: "run border", body: wordTestPara("", wordTestRun(`<w:bdr w:val="single"/>`, "boxed")), warn: "w:bdr", text: "boxed"},
		{name: "paragraph border", body: wordTestPara(`<w:pBdr><w:top w:val="single"/></w:pBdr>`, wordTestRun("", "ruled")), warn: "w:pBdr", text: "ruled"},
		{name: "paragraph shading", body: wordTestPara(`<w:shd w:val="clear" w:fill="FFFF00"/>`, wordTestRun("", "shaded")), warn: "w:shd", text: "shaded"},
		{name: "tracked insertion", body: wordTestPara("", `<w:ins w:id="1" w:author="a"><w:r><w:t>inserted</w:t></w:r></w:ins>`), warn: "tracked insertions", approx: true, text: "inserted"},
		{name: "tracked deletion", body: wordTestPara("", `<w:del w:id="1" w:author="a"><w:r><w:delText>gone</w:delText></w:r></w:del>`), warn: "tracked deletions", approx: true},
		{name: "right to left paragraph", body: wordTestPara(`<w:bidi/>`, wordTestRun("", "rtl")), warn: "w:bidi", approx: true, text: "rtl"},
		{name: "symbol", body: wordTestPara("", `<w:r><w:sym w:font="Symbol" w:char="F0B7"/></w:r>`, wordTestRun("", "after sym")), warn: "symbol", text: "after sym"},
		{name: "justified manual break", body: wordTestPara(`<w:jc w:val="both"/>`, `<w:r><w:t>one</w:t><w:br/><w:t>two</w:t></w:r>`), warn: "justified", approx: true, text: "onetwo"},
		{name: "mirrored margins", parts: &wordTestParts{styles: wordTestStyles, settings: `<w:mirrorMargins/>`}, warn: "mirrored margins", approx: true},
		{name: "unknown block element", body: `<w:futureBlock/>`, warn: "futureBlock"},
		{name: "unknown run element", body: wordTestPara("", `<w:r><w:futureThing/></w:r>`, wordTestRun("", "kept")), warn: "futureThing", text: "kept"},
		{name: "unknown paragraph element", body: wordTestPara("", `<w:futureInline/>`, wordTestRun("", "kept")), warn: "futureInline", text: "kept"},
		{name: "unknown paragraph property", body: wordTestPara(`<w:futureProp/>`, wordTestRun("", "kept")), warn: "futureProp", text: "kept"},
		{name: "unknown run property", body: wordTestPara("", wordTestRun(`<w:futureRunProp/>`, "kept")), warn: "futureRunProp", text: "kept"},
	}
}

func (c policyCase) doc(t *testing.T) (body string, parts wordTestParts) {
	sect := wordTestPage
	if c.sect != "" {
		sect = c.sect
	}
	body = wordTestPara("", wordTestRun("", "before")) + c.body + wordTestPara("", wordTestRun("", "after")) + sect
	parts = wordTestParts{styles: wordTestStyles}
	if c.parts != nil {
		parts = *c.parts
	}
	return body, parts
}

func TestStrictModeRefusesWhatItCannotDrawExactly(t *testing.T) {
	for _, c := range policyCases() {
		t.Run(c.name, func(t *testing.T) {
			body, parts := c.doc(t)
			o := newWordTestOpts(t, false)
			p, err := Prepare(context.Background(), wordTestDoc(t, body, parts), o.Options)
			if p != nil || !errors.Is(err, render.ErrUnsupported) {
				t.Fatalf("got %v, %v", p, err)
			}
		})
	}
}

func TestBestEffortReportsAndDrawsTheRest(t *testing.T) {
	for _, c := range policyCases() {
		t.Run(c.name, func(t *testing.T) {
			body, parts := c.doc(t)
			o := newWordTestOpts(t, true)
			p, err := Prepare(context.Background(), wordTestDoc(t, body, parts), o.Options)
			if err != nil {
				t.Fatal(err)
			}
			var found error
			for _, w := range o.warnings {
				if strings.Contains(w.Error(), c.warn) {
					found = w
				}
			}
			if found == nil {
				t.Fatalf("no warning containing %q in %v", c.warn, o.warnings)
			}
			if got := errors.Is(found, render.ErrApproximated); got != c.approx {
				t.Errorf("approximated = %v, want %v: %v", got, c.approx, found)
			}
			if !c.approx && !errors.Is(found, render.ErrUnsupported) {
				t.Errorf("left-out warning does not wrap ErrUnsupported: %v", found)
			}
			all := ""
			for pg := 1; pg <= p.Count(); pg++ {
				for _, tx := range p.texts(pg) {
					all += tx.text
				}
			}
			if !strings.HasPrefix(all, "before") || !strings.HasSuffix(all, "after") {
				t.Errorf("surrounding paragraphs drawn as %q", all)
			}
			if c.text != "" && !strings.Contains(all, c.text) {
				t.Errorf("%q not drawn in %q", c.text, all)
			}
		})
	}
}

func TestBestEffortReportsEachKindOnce(t *testing.T) {
	var body strings.Builder
	for i := 0; i < 5; i++ {
		body.WriteString(wordTestPara("", `<w:r><w:footnoteReference w:id="2"/></w:r>`, wordTestRun("", "x")))
	}
	body.WriteString(wordTestPage)
	o := newWordTestOpts(t, true)
	if _, err := Prepare(context.Background(), wordTestDoc(t, body.String(), wordTestParts{styles: wordTestStyles}), o.Options); err != nil {
		t.Fatal(err)
	}
	if len(o.warnings) != 1 {
		t.Fatalf("%d warnings: %v", len(o.warnings), o.warnings)
	}
}

func TestInstructionTextAndUnusedFeaturesAreNotReported(t *testing.T) {
	// A footnote reference inside a field instruction is not drawn, and a
	// style the document never uses is never read.
	styles := wordTestStyles + `<w:style w:type="paragraph" w:styleId="Unused"><w:name w:val="Unused"/><w:rPr><w:futureRunProp/></w:rPr></w:style>`
	body := wordTestPara("",
		`<w:r><w:fldChar w:fldCharType="begin"/></w:r>`,
		`<w:r><w:footnoteReference w:id="1"/></w:r>`,
		`<w:r><w:fldChar w:fldCharType="end"/></w:r>`,
		wordTestRun("", "text"),
	) + wordTestPage
	o := newWordTestOpts(t, false)
	if _, err := Prepare(context.Background(), wordTestDoc(t, body, wordTestParts{styles: styles}), o.Options); err != nil {
		t.Fatal(err)
	}
}

func TestMissingPageGeometryTakesTheDefaultPage(t *testing.T) {
	for _, tc := range []struct {
		name, sect string
		w, h       int
	}{
		// No section properties at all: Letter, 816 by 1056 pixels at 96 DPI.
		{"no section", "", 816, 1056},
		// A size but no margins keeps the size.
		{"no margins", `<w:sectPr><w:pgSz w:w="4500" w:h="3000"/></w:sectPr>`, 300, 200},
	} {
		doc := wordTestDoc(t, wordTestPara("", wordTestRun("", "x"))+tc.sect, wordTestParts{styles: wordTestStyles})
		// Strict mode refuses a page the document does not set.
		if _, err := Prepare(context.Background(), doc, newWordTestOpts(t, false).Options); !errors.Is(err, render.ErrUnsupported) {
			t.Errorf("%s strict: %v", tc.name, err)
		}
		// Best effort draws it on Letter with one-inch margins, and says so.
		o := newWordTestOpts(t, true)
		pages := wordTestPages(t, doc, o)
		page, err := pages.Page(context.Background(), 1)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if w, h, err := page.Size(96); err != nil || w != tc.w || h != tc.h {
			t.Errorf("%s: %dx%d, %v; want %dx%d", tc.name, w, h, err, tc.w, tc.h)
		}
		if len(o.warnings) != 1 || !errors.Is(o.warnings[0], render.ErrApproximated) || !strings.Contains(o.warnings[0].Error(), "Letter with one-inch margins") {
			t.Errorf("%s warnings: %v", tc.name, o.warnings)
		}
	}
}

func TestFontResolverIsRequiredAndErrorsPropagate(t *testing.T) {
	doc := wordTestDoc(t, wordTestPara("", wordTestRun("", "x"))+wordTestPage, wordTestParts{styles: wordTestStyles})
	if _, err := Prepare(context.Background(), doc, render.Options{}); !errors.Is(err, render.ErrUnsupported) {
		t.Errorf("no resolver: %v", err)
	}
	boom := errors.New("boom")
	opts := render.Options{Fonts: func(context.Context, render.FontRequest) (*shape.Face, error) { return nil, boom }}
	if _, err := Prepare(context.Background(), doc, opts); !errors.Is(err, boom) {
		t.Errorf("resolver error: %v", err)
	}
	opts.Fonts = func(context.Context, render.FontRequest) (*shape.Face, error) { return nil, nil }
	if _, err := Prepare(context.Background(), doc, opts); !errors.Is(err, render.ErrUnsupported) {
		t.Errorf("nil face: %v", err)
	}
}
