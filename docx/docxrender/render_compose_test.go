package docxrender

import "testing"

// Composition of tables with numbering.

func TestNumberedParagraphsInTableCellsContinueTheDocumentList(t *testing.T) {
	numbering := numAbstract("0", "", numDecimal(0, "%1.", "")) + numInstance("1", "0")
	cell := wordTestCell("", numPara("1", 0, "in")+numPara("1", 0, "cell"))
	body := numPara("1", 0, "before") + wordTestTable(wordTestFixed, []int{3000}, wordTestRow("", cell)) + numPara("1", 0, "after")
	p, o := numRender(t, body, numbering, false)
	equalLines(t, numLines(p, 1), []string{"1.before", "2.in", "3.cell", "4.after"})
	if len(o.warnings) != 0 {
		t.Errorf("warnings: %v", o.warnings)
	}
}
