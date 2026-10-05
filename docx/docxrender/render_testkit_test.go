package docxrender

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"sort"
	"strings"
	"testing"

	"github.com/mgilbir/forme/fonttest"
	"github.com/mgilbir/forme/layout"
	"github.com/mgilbir/forme/shape"
	"github.com/mgilbir/spine/docx"
	"github.com/mgilbir/spine/render"
)

// The test kit builds documents from WordprocessingML text and renders them
// with a fixture face whose metrics make geometry exact: 2000 units per em,
// ascent 1600, descent 400 and no line gap, so a line is exactly one font size
// tall; every ASCII glyph is half an em wide.

const wordTestNS = `xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006" xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math" xmlns:w14="http://schemas.microsoft.com/office/word/2010/wordml"`

// wordTestSect is a section property block. Lengths are twips (15 per pixel).
func wordTestSect(w, h, top, right, bottom, left int, extra string) string {
	return fmt.Sprintf(`<w:sectPr>%s<w:pgSz w:w="%d" w:h="%d"/><w:pgMar w:top="%d" w:right="%d" w:bottom="%d" w:left="%d" w:header="0" w:footer="0" w:gutter="0"/></w:sectPr>`, extra, w, h, top, right, bottom, left)
}

// wordTestPage is a 300 by 200 pixel page with 20 pixel margins, leaving a
// 260 by 160 pixel text area: 20 lines of 8 pixels, or 10 lines of 16.
var wordTestPage = wordTestSect(4500, 3000, 300, 300, 300, 300, "")

type wordTestParts struct {
	styles   string
	settings string
	theme    string
	// extra are further parts, keyed by part name ("word/header1.xml"), with
	// their markup in full. The document refers to one by wordTestRID.
	extra map[string]wordTestExtra
}

// wordTestExtra is an extra part's markup, content type and relationship type.
type wordTestExtra struct{ xml, contentType, relType string }

// wordTestRID is the relationship id the test kit gives an added part.
func wordTestRID(part string) string {
	return "rIdT" + strings.NewReplacer("/", "_", ".", "_").Replace(strings.TrimPrefix(part, "word/"))
}

const (
	wordTestRelHeader    = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/header"
	wordTestRelFooter    = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/footer"
	wordTestRelFootnotes = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/footnotes"
	wordTestRelEndnotes  = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/endnotes"
	wordTestCTHeader     = "application/vnd.openxmlformats-officedocument.wordprocessingml.header+xml"
	wordTestCTFooter     = "application/vnd.openxmlformats-officedocument.wordprocessingml.footer+xml"
	wordTestCTFootnotes  = "application/vnd.openxmlformats-officedocument.wordprocessingml.footnotes+xml"
	wordTestCTEndnotes   = "application/vnd.openxmlformats-officedocument.wordprocessingml.endnotes+xml"
)

// wordTestHeader and wordTestFooter are header and footer parts with the given
// paragraphs; they are added with wordTestParts.extra.
func wordTestHeader(paras ...string) wordTestExtra {
	return wordTestExtra{`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><w:hdr ` + wordTestNS + `>` + strings.Join(paras, "") + `</w:hdr>`, wordTestCTHeader, wordTestRelHeader}
}

func wordTestFooter(paras ...string) wordTestExtra {
	return wordTestExtra{`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><w:ftr ` + wordTestNS + `>` + strings.Join(paras, "") + `</w:ftr>`, wordTestCTFooter, wordTestRelFooter}
}

// wordTestDoc builds a document whose body is the given WordprocessingML.
func wordTestDoc(t testing.TB, body string, parts ...wordTestParts) *docx.Document {
	t.Helper()
	seed := docx.Create()
	seed.AddParagraph().AddRun().SetText("x")
	data, err := seed.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	var p wordTestParts
	if len(parts) > 0 {
		p = parts[0]
	}
	replace := map[string]string{
		"word/document.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><w:document ` + wordTestNS + `><w:body>` + body + `</w:body></w:document>`,
	}
	if p.styles != "" {
		replace["word/styles.xml"] = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><w:styles ` + wordTestNS + `>` + p.styles + `</w:styles>`
	}
	add := map[string][3]string{}
	if p.settings != "" {
		add["word/settings.xml"] = [3]string{`<?xml version="1.0" encoding="UTF-8" standalone="yes"?><w:settings ` + wordTestNS + `>` + p.settings + `</w:settings>`,
			"application/vnd.openxmlformats-officedocument.wordprocessingml.settings+xml", "http://schemas.openxmlformats.org/officeDocument/2006/relationships/settings"}
	}
	if p.theme != "" {
		add["word/theme/theme1.xml"] = [3]string{p.theme, "application/vnd.openxmlformats-officedocument.theme+xml", "http://schemas.openxmlformats.org/officeDocument/2006/relationships/theme"}
	}
	for name, e := range p.extra {
		add[name] = [3]string{e.xml, e.contentType, e.relType}
	}
	out := wordTestRewrite(t, data, replace, add)
	doc, err := docx.OpenReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = doc.Close() })
	return doc
}

// wordTestRewrite replaces parts of a package and adds new ones with their
// content type and a relationship from the main part.
func wordTestRewrite(t testing.TB, data []byte, replace map[string]string, add map[string][3]string) []byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	names := make([]string, 0, len(add))
	for n := range add {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		if r, ok := replace[f.Name]; ok {
			b = []byte(r)
		}
		switch f.Name {
		case "[Content_Types].xml":
			s := string(b)
			for _, n := range names {
				s = strings.Replace(s, "</Types>", `<Override PartName="/`+n+`" ContentType="`+add[n][1]+`"/></Types>`, 1)
			}
			b = []byte(s)
		case "word/_rels/document.xml.rels":
			s := string(b)
			for _, n := range names {
				target := strings.TrimPrefix(n, "word/")
				s = strings.Replace(s, "</Relationships>", fmt.Sprintf(`<Relationship Id="%s" Type="%s" Target="%s"/></Relationships>`, wordTestRID(n), add[n][2], target), 1)
			}
			b = []byte(s)
		}
		w, err := zw.Create(f.Name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = w.Write(b); err != nil {
			t.Fatal(err)
		}
	}
	for _, n := range names {
		w, err := zw.Create(n)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = w.Write([]byte(add[n][0])); err != nil {
			t.Fatal(err)
		}
	}
	if err = zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// wordTestFace is the fixture face.
func wordTestFace(t testing.TB) *shape.Face {
	t.Helper()
	var glyphs []fonttest.Glyph
	for c := rune(0x21); c <= 0x7E; c++ {
		glyphs = append(glyphs, fonttest.Glyph{Rune: c, Advance: 1000, HasShape: true, Ink: [4]int{100, 0, 900, 1400}})
	}
	glyphs = append(glyphs, fonttest.Glyph{Rune: ' ', Advance: 1000}, fonttest.Glyph{Rune: 0xA0, Advance: 1000},
		fonttest.Glyph{Rune: 0xAD, Advance: 1000, HasShape: true, Ink: [4]int{100, 500, 900, 700}},
		fonttest.Glyph{Rune: 0x2011, Advance: 1000, HasShape: true, Ink: [4]int{100, 500, 900, 700}},
		fonttest.Glyph{Rune: 0xE9, Advance: 1000, HasShape: true, Ink: [4]int{100, 0, 900, 1500}},
		fonttest.Glyph{Rune: 0x65E5, Advance: 2000, HasShape: true, Ink: [4]int{100, 0, 1900, 1600}})
	f, err := shape.Load(fonttest.SFNT(fonttest.SFNTOptions{UnitsPerEm: 2000, Ascent: 1600, Descent: -400, Glyphs: glyphs}))
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// wordTestOpts is best-effort or strict options over the fixture face,
// recording each font request and warning.
type wordTestOpts struct {
	render.Options
	requests []render.FontRequest
	warnings []error
}

func newWordTestOpts(t testing.TB, lenient bool) *wordTestOpts {
	t.Helper()
	face := wordTestFace(t)
	o := &wordTestOpts{}
	o.Fonts = func(_ context.Context, r render.FontRequest) (*shape.Face, error) {
		o.requests = append(o.requests, r)
		return face, nil
	}
	if lenient {
		o.Warn = func(err error) { o.warnings = append(o.warnings, err) }
	}
	return o
}

// wordTestPages prepares a document.
func wordTestPages(t testing.TB, doc *docx.Document, o *wordTestOpts) *Pages {
	t.Helper()
	p, err := Prepare(context.Background(), doc, o.Options)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// wordTestText is one drawn text op in page pixels.
type wordTestText struct {
	text string
	x, y float64 // x of the start, y of the baseline
	size float64
}

// texts returns a page's text ops in paint order.
func (p *Pages) texts(page int) []wordTestText {
	var out []wordTestText
	for _, op := range p.pages[page-1].ops {
		if v, ok := op.(layout.DrawText); ok {
			out = append(out, wordTestText{text: v.Text, x: v.At.X.Px(), y: v.At.Y.Px(), size: v.Size.Px()})
		}
	}
	return out
}

// lines groups a page's text by baseline, returning for each baseline the
// concatenated visible text, in top to bottom order.
func (p *Pages) lines(page int) []wordTestLine {
	by := map[float64]*wordTestLine{}
	var order []float64
	for _, tx := range p.texts(page) {
		l := by[tx.y]
		if l == nil {
			l = &wordTestLine{y: tx.y, x: tx.x}
			by[tx.y] = l
			order = append(order, tx.y)
		}
		l.text += tx.text
		if tx.x < l.x {
			l.x = tx.x
		}
	}
	sort.Float64s(order)
	out := make([]wordTestLine, 0, len(order))
	for _, y := range order {
		out = append(out, *by[y])
	}
	return out
}

type wordTestLine struct {
	text string
	x, y float64
}

// wordTestPara is a paragraph of plain runs. Run text is placed inside w:t with
// space preservation.
func wordTestPara(ppr string, runs ...string) string {
	var sb strings.Builder
	sb.WriteString("<w:p>")
	if ppr != "" {
		sb.WriteString("<w:pPr>" + ppr + "</w:pPr>")
	}
	for _, r := range runs {
		sb.WriteString(r)
	}
	sb.WriteString("</w:p>")
	return sb.String()
}

// wordTestRun is a run with optional run properties.
func wordTestRun(rpr, text string) string {
	r := "<w:r>"
	if rpr != "" {
		r += "<w:rPr>" + rpr + "</w:rPr>"
	}
	return r + `<w:t xml:space="preserve">` + text + `</w:t></w:r>`
}

// wordTestStyles is the styles part used by most tests: 12 pixel (sz 18) text in
// one fixture family, no spacing.
const wordTestStyles = `<w:docDefaults><w:rPrDefault><w:rPr><w:rFonts w:ascii="Fix" w:hAnsi="Fix" w:eastAsia="Fix" w:cs="Fix"/><w:sz w:val="18"/><w:szCs w:val="18"/></w:rPr></w:rPrDefault><w:pPrDefault><w:pPr><w:spacing w:before="0" w:after="0" w:line="240" w:lineRule="auto"/></w:pPr></w:pPrDefault></w:docDefaults><w:style w:type="paragraph" w:default="1" w:styleId="Normal"><w:name w:val="Normal"/></w:style><w:style w:type="character" w:default="1" w:styleId="DefaultParagraphFont"><w:name w:val="Default Paragraph Font"/></w:style>`
