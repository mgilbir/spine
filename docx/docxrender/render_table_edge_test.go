package docxrender

import (
	"strings"
	"testing"
)

func TestTableRowOfOnlyMergedContinuationsKeepsItsHeight(t *testing.T) {
	// The second row holds nothing but the continuation of the cell above; its
	// height rule still applies and the row after it follows.
	body := wordTestBody(wordTestTable(wordTestFixed, []int{1500},
		wordTestRow("", wordTestCell(`<w:vMerge w:val="restart"/>`, "a")),
		wordTestRow(`<w:trHeight w:val="600"/>`, wordTestCell(`<w:vMerge/>`, wordTestPara(""))),
		wordTestRow("", wordTestCell("", "b"))))
	p, _ := wordTestRender(t, body)
	wordTestAt(t, p, 1, "a", 20, base(20))
	// The merged cell's content fits the 40 pixels of the row it continues in.
	wordTestAt(t, p, 1, "b", 20, base(20+40))
}

func TestTableRowsJoinedByAMergedCellAreCutOnlyBetweenItsLines(t *testing.T) {
	// Three 8 pixel rows hold one merged cell of two 12 pixel lines starting at
	// y = 164: the row boundaries at 172 and 180 cross a line, so the cut is the
	// gap between the lines and the second line starts the next page.
	body := wordTestBody(strings.Repeat(wordTestPara("", wordTestRun("", "p")), 12),
		wordTestTable(wordTestFixed, []int{600},
			wordTestRow(`<w:trHeight w:val="120" w:hRule="exact"/>`, wordTestCell(`<w:vMerge w:val="restart"/>`, "aaaa bbbb")),
			wordTestRow(`<w:trHeight w:val="120" w:hRule="exact"/>`, wordTestCell(`<w:vMerge/>`, wordTestPara(""))),
			wordTestRow(`<w:trHeight w:val="120" w:hRule="exact"/>`, wordTestCell(`<w:vMerge/>`, wordTestPara("")))))
	p, _ := wordTestRender(t, body)
	if p.Count() != 2 {
		t.Fatalf("%d pages", p.Count())
	}
	wordTestAt(t, p, 1, "aaaa", 20, base(164))
	wordTestAt(t, p, 2, "bbbb", 20, base(20))
}

func TestTableRowWithoutCellsIsNotARow(t *testing.T) {
	body := wordTestBody(wordTestTable(wordTestFixed, []int{1500},
		wordTestRow("", wordTestCell("", "a")),
		`<w:tr/>`,
		wordTestRow("", wordTestCell("", "b"))))
	p, _ := wordTestRender(t, body)
	wordTestAt(t, p, 1, "b", 20, base(32))
}

func TestTableWithoutRowsDrawsNothing(t *testing.T) {
	p, _ := wordTestRender(t, wordTestBody(wordTestTable("", []int{1500}), wordTestPara("", wordTestRun("", "x"))))
	wordTestAt(t, p, 1, "x", 20, base(20))
}

func TestTableEmptyCellsKeepTheirParagraphHeight(t *testing.T) {
	body := wordTestBody(wordTestTable(wordTestFixed, []int{1500, 1500},
		wordTestRow("", wordTestCell("", wordTestPara("")), wordTestCell("", "x")),
		wordTestRow("", wordTestCell("", "y"), wordTestCell("", wordTestPara("")))))
	p, _ := wordTestRender(t, body)
	wordTestAt(t, p, 1, "x", 120, base(20))
	wordTestAt(t, p, 1, "y", 20, base(32))
}

func TestTableRowsLongerThanOneCellContent(t *testing.T) {
	// The row is as tall as its tallest cell; the others keep their top.
	tall := wordTestPara("", wordTestRun("", "aaaa bbbb cccc"))
	body := wordTestBody(wordTestTable(wordTestFixed, []int{600, 600},
		wordTestRow("", wordTestCell("", tall), wordTestCell("", "x")),
		wordTestRow("", wordTestCell("", "y"), wordTestCell("", "z"))))
	p, _ := wordTestRender(t, body)
	wordTestAt(t, p, 1, "x", 60, base(20))
	wordTestAt(t, p, 1, "y", 20, base(56))
}
