package docxrender

import (
	"fmt"
	"strings"
	"testing"
)

// The test page's text area is 160 pixels tall: 13 rows of 12 pixels.

func wordTestRows(n int, prefix string) []string {
	var rows []string
	for i := range n {
		rows = append(rows, wordTestRow("", wordTestCell("", fmt.Sprintf("%s%02d", prefix, i))))
	}
	return rows
}

func TestTableHeaderRowsRepeatOnEachPage(t *testing.T) {
	rows := append([]string{wordTestRow(`<w:tblHeader/>`, wordTestCell("", "H"))}, wordTestRows(30, "r")...)
	p, _ := wordTestRender(t, wordTestBody(wordTestTable(wordTestFixed, []int{1500}, rows...)))
	if p.Count() != 3 {
		t.Fatalf("%d pages", p.Count())
	}
	wordTestAt(t, p, 1, "H", 20, base(20))
	wordTestAt(t, p, 1, "r11", 20, base(20+12*12))
	if _, ok := p.textOf(1, "r12"); ok {
		t.Errorf("a 14th row on the first page")
	}
	// Each later page starts with the header row, then the next body row.
	wordTestAt(t, p, 2, "H", 20, base(20))
	wordTestAt(t, p, 2, "r12", 20, base(32))
	wordTestAt(t, p, 2, "r23", 20, base(20+12*12))
	wordTestAt(t, p, 3, "H", 20, base(20))
	wordTestAt(t, p, 3, "r24", 20, base(32))
	wordTestAt(t, p, 3, "r29", 20, base(32+5*12))
}

func TestTableWithoutHeaderRowsDoesNotRepeat(t *testing.T) {
	p, _ := wordTestRender(t, wordTestBody(wordTestTable(wordTestFixed, []int{1500}, wordTestRows(30, "r")...)))
	if p.Count() != 3 {
		t.Fatalf("%d pages", p.Count())
	}
	wordTestAt(t, p, 2, "r13", 20, base(20))
}

func TestTableHeaderRowsRepeatWithTheirShadingAndBorders(t *testing.T) {
	head := wordTestRow(`<w:tblHeader/>`, wordTestCell(`<w:shd w:val="clear" w:fill="FFCC00"/>`, "H"))
	p, _ := wordTestRender(t, wordTestBody(wordTestTable(wordTestFixed+wordTestBorders(8, "000000"), []int{1500}, append([]string{head}, wordTestRows(30, "r")...)...)))
	for page := 1; page <= 2; page++ {
		found := false
		for _, r := range p.rects(page) {
			if rgbOf(r.Color) == "ffcc00" && near(r.Rect.Y.Px(), 20+2.0/3) {
				found = true
			}
		}
		if !found {
			t.Errorf("page %d: no header fill at the top: %v", page, p.rects(page))
		}
	}
}

func TestTableHeaderRowsAreNotLeftAloneAtThePageFoot(t *testing.T) {
	// Twelve lines leave room for the header row only.
	body := wordTestBody(strings.Repeat(wordTestPara("", wordTestRun("", "p")), 12),
		wordTestTable(wordTestFixed, []int{1500}, append([]string{wordTestRow(`<w:tblHeader/>`, wordTestCell("", "H"))}, wordTestRows(3, "r")...)...))
	p, _ := wordTestRender(t, body)
	if _, ok := p.textOf(1, "H"); ok {
		t.Errorf("the header row is alone at the foot of page 1")
	}
	wordTestAt(t, p, 2, "H", 20, base(20))
}

func TestTableRowsSplitBetweenLines(t *testing.T) {
	// A row of three lines starting at the foot of the page splits between
	// lines; the text of each line goes to one page.
	tall := wordTestPara("", wordTestRun("", "aaaa bbbb cccc"))
	body := wordTestBody(strings.Repeat(wordTestPara("", wordTestRun("", "p")), 12),
		wordTestTable(wordTestFixed, []int{600}, wordTestRow("", wordTestCell("", tall))))
	p, _ := wordTestRender(t, body)
	if p.Count() != 2 {
		t.Fatalf("%d pages", p.Count())
	}
	wordTestAt(t, p, 1, "aaaa", 20, base(20+144))
	wordTestAt(t, p, 2, "bbbb", 20, base(20))
	wordTestAt(t, p, 2, "cccc", 20, base(32))
}

func TestTableCantSplitRowMovesWhole(t *testing.T) {
	tall := wordTestPara("", wordTestRun("", "aaaa bbbb cccc"))
	body := wordTestBody(strings.Repeat(wordTestPara("", wordTestRun("", "p")), 12),
		wordTestTable(wordTestFixed, []int{600}, wordTestRow(`<w:cantSplit/>`, wordTestCell("", tall))))
	p, _ := wordTestRender(t, body)
	if p.Count() != 2 {
		t.Fatalf("%d pages", p.Count())
	}
	wordTestAt(t, p, 2, "aaaa", 20, base(20))
	wordTestAt(t, p, 2, "cccc", 20, base(44))
}

func TestTableCantSplitRowTallerThanAPageSplitsAnyway(t *testing.T) {
	words := strings.TrimSpace(strings.Repeat("wwww ", 20))
	body := wordTestBody(wordTestTable(wordTestFixed, []int{600},
		wordTestRow(`<w:cantSplit/>`, wordTestCell("", wordTestPara("", wordTestRun("", words))))))
	p, _ := wordTestRender(t, body)
	if p.Count() != 2 {
		t.Fatalf("%d pages", p.Count())
	}
	if n := len(p.lines(1)); n != 13 {
		t.Errorf("%d lines on page 1", n)
	}
	if n := len(p.lines(2)); n != 7 {
		t.Errorf("%d lines on page 2", n)
	}
}

func TestTableSplitRowClipsFillsAndDrawsBordersOnBothPages(t *testing.T) {
	tall := wordTestPara("", wordTestRun("", "aaaa bbbb cccc"))
	body := wordTestBody(strings.Repeat(wordTestPara("", wordTestRun("", "p")), 12),
		wordTestTable(wordTestFixed+wordTestBorders(24, "FF0000"), []int{600},
			wordTestRow("", wordTestCell(`<w:shd w:val="clear" w:fill="00FF00"/>`, tall))))
	p, _ := wordTestRender(t, body)
	var fill1, fill2 float64
	for _, r := range p.rects(1) {
		if rgbOf(r.Color) == "00ff00" {
			fill1 = r.Rect.Y.Px() + r.Rect.H.Px()
		}
	}
	for _, r := range p.rects(2) {
		if rgbOf(r.Color) == "00ff00" {
			fill2 = r.Rect.Y.Px()
		}
	}
	// The fill stops at the bottom of the text area and starts at the top.
	if fill1 > 180+0.01 || fill1 < 160 {
		t.Errorf("fill on page 1 ends at %v", fill1)
	}
	if fill2 > 20+0.01 {
		t.Errorf("fill on page 2 starts at %v", fill2)
	}
	// The border is a vertical line on both pages and a top line on the first.
	for page := 1; page <= 2; page++ {
		vertical := false
		for _, r := range p.rects(page) {
			if rgbOf(r.Color) == "ff0000" && r.Rect.H.Px() > r.Rect.W.Px()*2 {
				vertical = true
			}
		}
		if !vertical {
			t.Errorf("page %d has no side border", page)
		}
	}
}

func TestTableBordersAtARowBoundaryCutAreDrawnWholeOnBothPages(t *testing.T) {
	rows := wordTestRows(14, "r")
	p, _ := wordTestRender(t, wordTestBody(wordTestTable(wordTestFixed+wordTestBorders(24, "FF0000"), []int{1500}, rows...)))
	if p.Count() != 2 {
		t.Fatalf("%d pages", p.Count())
	}
	// The line below the last row of page 1 and the line above the first row of
	// page 2 are each 4 pixels thick, whole.
	count := func(page int) (n int) {
		for _, r := range p.rects(page) {
			if rgbOf(r.Color) == "ff0000" && near(r.Rect.H.Px(), 4) && r.Rect.W.Px() > 50 {
				n++
			}
		}
		return n
	}
	if count(2) < 2 {
		t.Errorf("page 2 has %d full horizontal lines, want its top and bottom", count(2))
	}
}

func TestTableVerticalMergeIsNotCutBetweenRows(t *testing.T) {
	// A vertically merged cell's text crosses the boundary below row 13: the
	// rows it joins stay together on the page they start on.
	var rows []string
	rows = append(rows, wordTestRows(12, "r")...)
	tall := wordTestPara("", wordTestRun("", "aaaa bbbb cccc dddd"))
	rows = append(rows,
		wordTestRow("", wordTestCell(`<w:vMerge w:val="restart"/>`, tall), wordTestCell("", "x")),
		wordTestRow("", wordTestCell(`<w:vMerge/>`, wordTestPara("")), wordTestCell("", "y")))
	rows[0] = wordTestRow("", wordTestCell("", "r00"), wordTestCell("", "q"))
	for i := 1; i < 12; i++ {
		rows[i] = wordTestRow("", wordTestCell("", fmt.Sprintf("r%02d", i)), wordTestCell("", "q"))
	}
	p, _ := wordTestRender(t, wordTestBody(wordTestTable(wordTestFixed, []int{600, 600}, rows...)))
	// The merged cell has four lines of text and the page room for one, so it is
	// cut between lines, and both pages carry the cell's lines.
	if p.Count() != 2 {
		t.Fatalf("%d pages", p.Count())
	}
	if _, ok := p.textOf(1, "aaaa"); !ok {
		t.Errorf("first line not on page 1")
	}
	if _, ok := p.textOf(2, "dddd"); !ok {
		t.Errorf("last line not on page 2")
	}
	// Row "y" is on the page where its text lies, after the merged cell's lines.
	if _, ok := p.textOf(2, "y"); !ok {
		t.Errorf("row y not on page 2")
	}
}

func TestTableAfterTextKeepsFlowing(t *testing.T) {
	body := wordTestBody(wordTestPara("", wordTestRun("", "intro")),
		wordTestTable(wordTestFixed, []int{1500}, wordTestRows(14, "r")...),
		wordTestPara("", wordTestRun("", "outro")))
	p, _ := wordTestRender(t, body)
	if p.Count() != 2 {
		t.Fatalf("%d pages", p.Count())
	}
	wordTestAt(t, p, 1, "intro", 20, base(20))
	wordTestAt(t, p, 1, "r11", 20, base(20+12*12))
	wordTestAt(t, p, 2, "r12", 20, base(20))
	wordTestAt(t, p, 2, "r13", 20, base(32))
	wordTestAt(t, p, 2, "outro", 20, base(44))
}

func TestTableKeepNextRowsStayTogether(t *testing.T) {
	keep := func(s string) string {
		return wordTestRow("", wordTestCell("", wordTestPara(`<w:keepNext/>`, wordTestRun("", s))))
	}
	// Eleven lines then a table whose first two rows keep with the third: the
	// first fits on page 1 by itself, the three together do not, so all move.
	body := wordTestBody(strings.Repeat(wordTestPara("", wordTestRun("", "p")), 11),
		wordTestTable(wordTestFixed, []int{1500}, keep("ka"), keep("kb"), wordTestRow("", wordTestCell("", "kc"))))
	p, _ := wordTestRender(t, body)
	if _, ok := p.textOf(1, "ka"); ok {
		t.Errorf("a kept row stayed on page 1 without the row it keeps with")
	}
	wordTestAt(t, p, 2, "ka", 20, base(20))
	// A chain longer than a page cannot be kept and breaks.
	var rows []string
	for i := range 20 {
		rows = append(rows, keep(fmt.Sprintf("k%02d", i)))
	}
	p, _ = wordTestRender(t, wordTestBody(wordTestTable(wordTestFixed, []int{1500}, rows...)))
	if p.Count() != 2 {
		t.Errorf("%d pages", p.Count())
	}
}

func TestTablePrecededByKeepNextParagraphKeepsWithItsFirstRow(t *testing.T) {
	body := wordTestBody(strings.Repeat(wordTestPara("", wordTestRun("", "p")), 12),
		wordTestPara(`<w:keepNext/>`, wordTestRun("", "head")),
		wordTestTable(wordTestFixed, []int{1500}, wordTestRows(2, "r")...))
	p, _ := wordTestRender(t, body)
	// "head" fits at the foot of page 1 (13th line) but its row does not.
	if _, ok := p.textOf(1, "head"); ok {
		t.Errorf("the paragraph stayed behind its table")
	}
	wordTestAt(t, p, 2, "head", 20, base(20))
	wordTestAt(t, p, 2, "r00", 20, base(32))
}

func TestTablePaginationIsBounded(t *testing.T) {
	// Many rows paginate without work beyond the page limit: with a header row
	// repeated on every page.
	rows := append([]string{wordTestRow(`<w:tblHeader/>`, wordTestCell("", "H"))}, wordTestRows(400, "r")...)
	p, _ := wordTestRender(t, wordTestBody(wordTestTable(wordTestFixed, []int{1500}, rows...)))
	if p.Count() < 30 {
		t.Errorf("%d pages", p.Count())
	}
	for pg := 1; pg <= p.Count(); pg++ {
		if tx, ok := p.textOf(pg, "H"); !ok || !near(tx.At.Y.Px(), base(20)) {
			t.Errorf("page %d: header %v %v", pg, tx, ok)
		}
	}
}
