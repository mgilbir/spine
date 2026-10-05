package docxrender

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/mgilbir/spine/render"
)

// Tables in the test pages: a 260 pixel wide text area from x = 20, 12 pixel
// lines in the fixture face (6 pixels a character). Lengths in the XML are
// twips, 15 to the pixel: 1500 is 100 pixels.

func wordTestTable(props string, grid []int, rows ...string) string {
	var sb strings.Builder
	sb.WriteString("<w:tbl><w:tblPr>" + props + "</w:tblPr><w:tblGrid>")
	for _, g := range grid {
		fmt.Fprintf(&sb, `<w:gridCol w:w="%d"/>`, g)
	}
	sb.WriteString("</w:tblGrid>" + strings.Join(rows, "") + "</w:tbl>")
	return sb.String()
}

func wordTestRow(trpr string, cells ...string) string {
	r := "<w:tr>"
	if trpr != "" {
		r += "<w:trPr>" + trpr + "</w:trPr>"
	}
	return r + strings.Join(cells, "") + "</w:tr>"
}

// wordTestCell is a cell holding the paragraphs, or one paragraph of text.
func wordTestCell(tcpr string, content string) string {
	if !strings.HasPrefix(content, "<") {
		content = wordTestPara("", wordTestRun("", content))
	}
	c := "<w:tc>"
	if tcpr != "" {
		c += "<w:tcPr>" + tcpr + "</w:tcPr>"
	}
	return c + content + "</w:tc>"
}

const wordTestFixed = `<w:tblLayout w:type="fixed"/>`

// wordTestNormalTable is the table style Word defines: 5.4 pt side margins.
const wordTestNormalTable = `<w:style w:type="table" w:default="1" w:styleId="TableNormal"><w:name w:val="Normal Table"/><w:tblPr><w:tblInd w:w="0" w:type="dxa"/><w:tblCellMar><w:top w:w="0" w:type="dxa"/><w:left w:w="108" w:type="dxa"/><w:bottom w:w="0" w:type="dxa"/><w:right w:w="108" w:type="dxa"/></w:tblCellMar></w:tblPr></w:style>`

const wordTestCompat15 = `<w:compat><w:compatSetting w:name="compatibilityMode" w:uri="http://schemas.microsoft.com/office/word" w:val="15"/></w:compat>`

func wordTestBorders(sz int, color string) string {
	var sb strings.Builder
	sb.WriteString("<w:tblBorders>")
	for _, s := range []string{"top", "left", "bottom", "right", "insideH", "insideV"} {
		fmt.Fprintf(&sb, `<w:%s w:val="single" w:sz="%d" w:space="0" w:color="%s"/>`, s, sz, color)
	}
	return sb.String() + "</w:tblBorders>"
}

// wordTestAt asserts the drawn position of a text.
func wordTestAt(t *testing.T, p *Pages, page int, text string, x, y float64) {
	t.Helper()
	tx, ok := p.textOf(page, text)
	if !ok {
		t.Fatalf("%q not drawn on page %d", text, page)
	}
	if !near(tx.At.X.Px(), x) || !near(tx.At.Y.Px(), y) {
		t.Errorf("%q at (%v, %v), want (%v, %v)", text, tx.At.X.Px(), tx.At.Y.Px(), x, y)
	}
}

// base is the baseline of a text whose line starts at top.
func base(top float64) float64 { return top + wordTestAscent12 }

func hasRect(p *Pages, page int, x, y, w, h float64, color string) bool {
	for _, r := range p.rects(page) {
		if near(r.Rect.X.Px(), x) && near(r.Rect.Y.Px(), y) && near(r.Rect.W.Px(), w) && near(r.Rect.H.Px(), h) && rgbOf(r.Color) == color {
			return true
		}
	}
	return false
}

func TestTableFixedGridPositionsCells(t *testing.T) {
	p, _ := wordTestRender(t, wordTestBody(wordTestTable(wordTestFixed, []int{1500, 3000},
		wordTestRow("", wordTestCell("", "aa"), wordTestCell("", "bb")),
		wordTestRow("", wordTestCell("", "cc"), wordTestCell("", "dd")))))
	wordTestAt(t, p, 1, "aa", 20, base(20))
	wordTestAt(t, p, 1, "bb", 120, base(20))
	wordTestAt(t, p, 1, "cc", 20, base(32))
	wordTestAt(t, p, 1, "dd", 120, base(32))
}

func TestTableCellMarginsAndBorders(t *testing.T) {
	// Word measures a cell margin from the grid line, borders centred on it.
	props := wordTestFixed + wordTestBorders(12, "FF0000") + `<w:tblCellMar><w:top w:w="75" w:type="dxa"/><w:left w:w="150" w:type="dxa"/></w:tblCellMar><w:tblInd w:w="150" w:type="dxa"/>`
	body := wordTestBody(wordTestTable(props, []int{1500, 1500},
		wordTestRow("", wordTestCell("", "aa"), wordTestCell("", "bb"))))
	p, _ := wordTestRender(t, body, wordTestParts{styles: wordTestStyles, settings: wordTestCompat15})
	// The grid line is at the margin plus the indent: x = 30, 2 pixels wide.
	wordTestAt(t, p, 1, "aa", 30+10, base(20+1+5))
	wordTestAt(t, p, 1, "bb", 130+10, base(20+1+5))
	var top, left bool
	for _, r := range p.rects(1) {
		if rgbOf(r.Color) != "ff0000" {
			continue
		}
		top = top || (near(r.Rect.Y.Px(), 20) && near(r.Rect.H.Px(), 2))
		left = left || (near(r.Rect.X.Px(), 29) && near(r.Rect.W.Px(), 2))
	}
	if !top || !left {
		t.Errorf("borders not centred on the grid lines: %v", p.rects(1))
	}
}

func TestTableBeforeWord2013TextLinesUpWithMargin(t *testing.T) {
	// Without compatibilityMode 15 the table moves left by the first cell's
	// margin, so the text lines up with the paragraphs around it.
	styles := wordTestStyles + wordTestNormalTable
	body := wordTestBody(wordTestTable(wordTestFixed, []int{1500},
		wordTestRow("", wordTestCell("", "aa"))))
	p, _ := wordTestRender(t, body, wordTestParts{styles: styles})
	wordTestAt(t, p, 1, "aa", 20, base(20))
	p, _ = wordTestRender(t, body, wordTestParts{styles: styles, settings: wordTestCompat15})
	wordTestAt(t, p, 1, "aa", 20+7.2, base(20))
}

func TestTableAlignment(t *testing.T) {
	for _, tc := range []struct {
		jc string
		x  float64
	}{{"left", 20}, {"center", 20 + 30}, {"right", 20 + 60}} {
		t.Run(tc.jc, func(t *testing.T) {
			p, _ := wordTestRender(t, wordTestBody(wordTestTable(wordTestFixed+`<w:jc w:val="`+tc.jc+`"/>`, []int{3000},
				wordTestRow("", wordTestCell("", "aa")))))
			wordTestAt(t, p, 1, "aa", tc.x, base(20))
		})
	}
}

func TestTablePercentWidth(t *testing.T) {
	// 50 percent of the 260 pixel text area, centred: the grid agrees.
	p, _ := wordTestRender(t, wordTestBody(wordTestTable(wordTestFixed+`<w:tblW w:w="2500" w:type="pct"/><w:jc w:val="center"/>`, []int{1950},
		wordTestRow("", wordTestCell("", "aa")))))
	wordTestAt(t, p, 1, "aa", 20+65, base(20))
}

func TestTableMergedCells(t *testing.T) {
	body := wordTestBody(wordTestTable(wordTestFixed, []int{1500, 1500, 1500},
		wordTestRow("", wordTestCell(`<w:gridSpan w:val="2"/>`, "wide"), wordTestCell("", "r1")),
		wordTestRow("", wordTestCell(`<w:vMerge w:val="restart"/><w:vAlign w:val="center"/>`, "tall"), wordTestCell("", "b2"), wordTestCell("", "c2")),
		wordTestRow("", wordTestCell(`<w:vMerge/>`, wordTestPara("")), wordTestCell("", "b3"), wordTestCell("", "c3"))))
	p, _ := wordTestRender(t, body)
	wordTestAt(t, p, 1, "wide", 20, base(20))
	wordTestAt(t, p, 1, "r1", 20+200, base(20))
	// The merged cell spans rows two and three (24 pixels) and centres its line.
	wordTestAt(t, p, 1, "tall", 20, base(32+6))
	wordTestAt(t, p, 1, "b3", 120, base(44))
	wordTestAt(t, p, 1, "c3", 220, base(44))
}

func TestTableRowHeights(t *testing.T) {
	body := wordTestBody(wordTestTable(wordTestFixed, []int{1500},
		wordTestRow(`<w:trHeight w:val="600"/>`, wordTestCell("", "a")),
		wordTestRow(`<w:trHeight w:val="300" w:hRule="exact"/>`, wordTestCell(`<w:vAlign w:val="bottom"/>`, "b")),
		wordTestRow(`<w:trHeight w:val="120" w:hRule="atLeast"/>`, wordTestCell("", "c")),
		wordTestRow("", wordTestCell("", "d"))))
	p, _ := wordTestRender(t, body)
	wordTestAt(t, p, 1, "a", 20, base(20))
	// An exact 20 pixel row; the text sits at its bottom.
	wordTestAt(t, p, 1, "b", 20, base(60+8))
	wordTestAt(t, p, 1, "c", 20, base(80))
	wordTestAt(t, p, 1, "d", 20, base(92))
}

func TestTableExactRowHeightTextTallerIsApproximated(t *testing.T) {
	body := wordTestBody(wordTestTable(wordTestFixed, []int{1500},
		wordTestRow(`<w:trHeight w:val="120" w:hRule="exact"/>`, wordTestCell("", "a"))))
	_, err := Prepare(context.Background(), wordTestDoc(t, body, wordTestParts{styles: wordTestStyles}), newWordTestOpts(t, false).Options)
	if !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("strict: %v", err)
	}
	o := newWordTestOpts(t, true)
	wordTestPages(t, wordTestDoc(t, body, wordTestParts{styles: wordTestStyles}), o)
	if len(o.warnings) != 1 || !errors.Is(o.warnings[0], render.ErrApproximated) || !strings.Contains(o.warnings[0].Error(), "exact row height") {
		t.Errorf("warnings %v", o.warnings)
	}
}

func TestTableShadingAndStyleBanding(t *testing.T) {
	style := `<w:style w:type="table" w:styleId="Banded"><w:name w:val="Banded"/><w:basedOn w:val="TableNormal"/>` +
		`<w:tblPr><w:tblStyleRowBandSize w:val="1"/></w:tblPr>` +
		`<w:tblStylePr w:type="firstRow"><w:rPr><w:b/></w:rPr><w:tcPr><w:shd w:val="clear" w:color="auto" w:fill="112233"/></w:tcPr></w:tblStylePr>` +
		`<w:tblStylePr w:type="band1Horz"><w:tcPr><w:shd w:val="clear" w:color="auto" w:fill="AABBCC"/></w:tcPr></w:tblStylePr>` +
		`</w:style>`
	look := `<w:tblLook w:val="0420" w:firstRow="1" w:lastRow="0" w:firstColumn="0" w:lastColumn="0" w:noHBand="0" w:noVBand="1"/>`
	rows := []string{wordTestRow("", wordTestCell("", "h"))}
	for i := range 4 {
		rows = append(rows, wordTestRow("", wordTestCell("", fmt.Sprint("r", i))))
	}
	body := wordTestBody(wordTestTable(wordTestFixed+`<w:tblStyle w:val="Banded"/>`+look, []int{1500}, rows...))
	p, o := wordTestRender(t, body, wordTestParts{styles: wordTestStyles + wordTestNormalTable + style})
	// The header row is shaded and bold; the band alternates after it.
	for _, want := range []struct {
		y     float64
		color string
	}{{20, "112233"}, {32, "aabbcc"}, {56, "aabbcc"}} {
		if !hasRect(p, 1, 12.8, want.y, 100, 12, want.color) {
			t.Errorf("no %s fill at y=%v in %v", want.color, want.y, p.rects(1))
		}
	}
	for _, r := range p.rects(1) {
		if near(r.Rect.Y.Px(), 44) && rgbOf(r.Color) == "aabbcc" {
			t.Errorf("second band is shaded: %v", r.Rect)
		}
	}
	bold := false
	for _, rq := range o.requests {
		bold = bold || rq.Bold
	}
	if !bold {
		t.Errorf("header row not bold: %v", o.requests)
	}
}

func TestTableStyleParagraphFormattingUnderParagraphStyle(t *testing.T) {
	// The table style sets spacing after; direct formatting overrides it.
	style := `<w:style w:type="table" w:styleId="Spaced"><w:name w:val="Spaced"/><w:pPr><w:spacing w:after="150"/></w:pPr></w:style>`
	body := wordTestBody(wordTestTable(wordTestFixed+`<w:tblStyle w:val="Spaced"/>`, []int{1500},
		wordTestRow("", wordTestCell("", wordTestPara("", wordTestRun("", "a"))+wordTestPara(`<w:spacing w:after="0"/>`, wordTestRun("", "b"))+wordTestPara("", wordTestRun("", "c"))))))
	p, _ := wordTestRender(t, body, wordTestParts{styles: wordTestStyles + style})
	wordTestAt(t, p, 1, "a", 20, base(20))
	wordTestAt(t, p, 1, "b", 20, base(20+12+10))
	wordTestAt(t, p, 1, "c", 20, base(20+12+10+12))
}

func TestTableBorderConflictsFollowWord(t *testing.T) {
	// The heavier border wins on a shared edge, whichever cell states it.
	left := `<w:tcBorders><w:right w:val="single" w:sz="12" w:color="FF0000"/></w:tcBorders>`
	right := `<w:tcBorders><w:left w:val="single" w:sz="24" w:color="0000FF"/></w:tcBorders>`
	body := wordTestBody(wordTestTable(wordTestFixed, []int{1500, 1500},
		wordTestRow("", wordTestCell(left, "a"), wordTestCell(right, "b"))))
	p, _ := wordTestRender(t, body)
	var red, blue int
	for _, r := range p.rects(1) {
		switch rgbOf(r.Color) {
		case "ff0000":
			red++
		case "0000ff":
			blue++
		}
	}
	if red != 0 || blue == 0 {
		t.Errorf("red %d blue %d", red, blue)
	}
	// At equal weight the darker colour wins.
	left = `<w:tcBorders><w:right w:val="single" w:sz="12" w:color="FFFF00"/></w:tcBorders>`
	right = `<w:tcBorders><w:left w:val="single" w:sz="12" w:color="000080"/></w:tcBorders>`
	body = wordTestBody(wordTestTable(wordTestFixed, []int{1500, 1500},
		wordTestRow("", wordTestCell(left, "a"), wordTestCell(right, "b"))))
	p, _ = wordTestRender(t, body)
	for _, r := range p.rects(1) {
		if rgbOf(r.Color) == "ffff00" {
			t.Errorf("the lighter border is drawn: %v", r.Rect)
		}
	}
}

func TestTableCellBordersOverrideTable(t *testing.T) {
	// A cell's own border replaces the table's inside border on its side.
	body := wordTestBody(wordTestTable(wordTestFixed+wordTestBorders(8, "000000"), []int{1500, 1500},
		wordTestRow("", wordTestCell(`<w:tcBorders><w:right w:val="nil"/></w:tcBorders>`, "a"), wordTestCell(`<w:tcBorders><w:left w:val="nil"/></w:tcBorders>`, "b"))))
	p, _ := wordTestRender(t, body)
	for _, r := range p.rects(1) {
		if near(r.Rect.X.Px(), 119) || near(r.Rect.X.Px(), 120) {
			if r.Rect.H.Px() > 3 && r.Rect.W.Px() < 3 {
				t.Errorf("vertical border drawn between the cells: %v", r.Rect)
			}
		}
	}
}

func TestTableNestedTable(t *testing.T) {
	inner := wordTestTable(wordTestFixed, []int{600, 600},
		wordTestRow("", wordTestCell("", "i1"), wordTestCell("", "i2")))
	body := wordTestBody(wordTestTable(wordTestFixed, []int{1500, 3000},
		wordTestRow("", wordTestCell("", "a"), wordTestCell("", wordTestPara("", wordTestRun("", "top"))+inner+wordTestPara("", wordTestRun("", "end"))))))
	p, _ := wordTestRender(t, body)
	wordTestAt(t, p, 1, "top", 120, base(20))
	wordTestAt(t, p, 1, "i1", 120, base(32))
	wordTestAt(t, p, 1, "i2", 160, base(32))
	wordTestAt(t, p, 1, "end", 120, base(44))
}

func TestTablePercentWidthInNestedTable(t *testing.T) {
	// A percentage is of the cell the table is in.
	inner := wordTestTable(wordTestFixed+`<w:tblW w:w="1250" w:type="pct"/><w:jc w:val="right"/>`, []int{750},
		wordTestRow("", wordTestCell("", "i")))
	body := wordTestBody(wordTestTable(wordTestFixed, []int{3000},
		wordTestRow("", wordTestCell("", inner+wordTestPara("", wordTestRun("", "e"))))))
	p, _ := wordTestRender(t, body)
	// The cell is 200 pixels; 25 percent is 50, flush right.
	wordTestAt(t, p, 1, "i", 20+150, base(20))
}

func TestTableTabsInCell(t *testing.T) {
	para := wordTestPara(`<w:tabs><w:tab w:val="right" w:pos="1500"/></w:tabs>`, `<w:r><w:t>ab</w:t><w:tab/><w:t>cd</w:t></w:r>`)
	body := wordTestBody(wordTestTable(wordTestFixed, []int{3000, 3000},
		wordTestRow("", wordTestCell("", "x"), wordTestCell("", para))))
	p, _ := wordTestRender(t, body)
	wordTestAt(t, p, 1, "ab", 220, base(20))
	// Right aligned at 100 pixels from the cell's left edge.
	wordTestAt(t, p, 1, "cd", 320-12, base(20))
}

func TestTableSpacingAroundTables(t *testing.T) {
	sp := `<w:spacing w:before="150" w:after="300"/>`
	body := wordTestBody(
		wordTestPara(sp, wordTestRun("", "before")),
		wordTestTable(wordTestFixed, []int{1500}, wordTestRow("", wordTestCell("", "cell"))),
		wordTestPara(sp, wordTestRun("", "after")))
	p, _ := wordTestRender(t, body)
	// before: 10 + 12 + 20 after; the table follows, then 10 before.
	wordTestAt(t, p, 1, "before", 20, base(30))
	wordTestAt(t, p, 1, "cell", 20, base(62))
	wordTestAt(t, p, 1, "after", 20, base(74+10))
}
