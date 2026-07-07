package docx

import (
	"path/filepath"
	"strings"
	"testing"
)

// saveAndReopen saves the document to a temp file and opens the result.
func saveAndReopen(t *testing.T, doc *Document) *Document {
	t.Helper()
	tmp := filepath.Join(t.TempDir(), "out.docx")
	if err := doc.Save(tmp); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(tmp)
	if err != nil {
		t.Fatal(err)
	}
	return reopened
}

// C153: on a new document, SetText writes runs the child order does not track
// yet; the first AddRun must backfill them instead of flipping the paragraph
// to childOrder-only marshaling and dropping the SetText run.
func TestNewDocument_SetTextThenAddRun(t *testing.T) {
	doc := Create()
	p := doc.AddParagraph()
	p.SetText("FIRST-SETTEXT")
	p.AddRun().SetText("SECOND-ADDRUN")

	reopened := saveAndReopen(t, doc)
	paras := reopened.Paragraphs()
	if len(paras) != 1 {
		t.Fatalf("got %d paragraphs, want 1", len(paras))
	}
	if got, want := paras[0].Text(), "FIRST-SETTEXTSECOND-ADDRUN"; got != want {
		t.Errorf("paragraph text = %q, want %q", got, want)
	}

	// Multi-cycle: mutate the reopened document and save again.
	paras[0].AddRun().SetText("THIRD-ADDRUN")
	final := saveAndReopen(t, reopened)
	if got, want := final.Paragraphs()[0].Text(), "FIRST-SETTEXTSECOND-ADDRUNTHIRD-ADDRUN"; got != want {
		t.Errorf("after second cycle, paragraph text = %q, want %q", got, want)
	}
}

// C154: AddTable seeds each cell with an empty paragraph; text set on that
// seed paragraph must survive a later AddParagraph on the same cell.
func TestNewDocument_TableSeedParagraphSurvivesAddParagraph(t *testing.T) {
	doc := Create()
	tbl := doc.AddTable(1, 1)
	cell := tbl.Rows()[0].Cells()[0]
	cell.Paragraphs()[0].AddRun().SetText("VIA-INITIAL-PARA")
	cell.AddParagraph().AddRun().SetText("VIA-ADDPARA")

	reopened := saveAndReopen(t, doc)
	cellR := reopened.Tables()[0].Rows()[0].Cells()[0]
	if got, want := cellR.Text(), "VIA-INITIAL-PARA\nVIA-ADDPARA"; got != want {
		t.Errorf("cell text = %q, want %q", got, want)
	}

	// Multi-cycle: append another paragraph to the parsed cell and save again.
	cellR.AddParagraph().AddRun().SetText("SECOND-CYCLE")
	final := saveAndReopen(t, reopened)
	cellF := final.Tables()[0].Rows()[0].Cells()[0]
	if got, want := cellF.Text(), "VIA-INITIAL-PARA\nVIA-ADDPARA\nSECOND-CYCLE"; got != want {
		t.Errorf("after second cycle, cell text = %q, want %q", got, want)
	}
}

// C154 (AddCell path): a cell added to a parsed table row seeds its paragraph
// the same way and must not lose it on the first AddParagraph.
func TestNewDocument_AddCellSeedParagraphSurvives(t *testing.T) {
	doc := Create()
	tbl := doc.AddTable(1, 1)
	row := tbl.Rows()[0]
	cell := row.AddCell()
	cell.Paragraphs()[0].AddRun().SetText("CELL2-SEED")
	cell.AddParagraph().AddRun().SetText("CELL2-ADDED")

	reopened := saveAndReopen(t, doc)
	cells := reopened.Tables()[0].Rows()[0].Cells()
	if len(cells) != 2 {
		t.Fatalf("got %d cells, want 2", len(cells))
	}
	if got, want := cells[1].Text(), "CELL2-SEED\nCELL2-ADDED"; got != want {
		t.Errorf("cell text = %q, want %q", got, want)
	}
}

// C159: Clear on a parsed paragraph must drop the stale run references from
// the recorded child order, otherwise a following AddRun is serialized twice
// (once via the stale reference, once via the appended entry).
func TestOpenedDocument_ClearThenAddRun(t *testing.T) {
	doc, err := Open("testdata/minimal.docx")
	if err != nil {
		t.Fatal(err)
	}
	p := doc.Paragraphs()[0]
	p.Clear()
	p.AddRun().SetText("REPLACED")

	reopened := saveAndReopen(t, doc)
	pr := reopened.Paragraphs()[0]
	if got, want := pr.Text(), "REPLACED"; got != want {
		t.Errorf("paragraph text = %q, want %q (duplicated or dropped run)", got, want)
	}
	if got := len(pr.Runs()); got != 1 {
		t.Errorf("got %d runs, want 1", got)
	}

	// Multi-cycle: clear and replace again on the reopened document.
	pr.Clear()
	pr.AddRun().SetText("REPLACED-AGAIN")
	final := saveAndReopen(t, reopened)
	pf := final.Paragraphs()[0]
	if got, want := pf.Text(), "REPLACED-AGAIN"; got != want {
		t.Errorf("after second cycle, paragraph text = %q, want %q", got, want)
	}
	if got := len(pf.Runs()); got != 1 {
		t.Errorf("after second cycle, got %d runs, want 1", got)
	}
}

// SetText on a parsed paragraph replaces the runs in place; a following
// AddRun must not resolve a stale run reference and duplicate content
// (same mechanism as C159, via SetText instead of Clear).
func TestOpenedDocument_SetTextThenAddRun(t *testing.T) {
	doc, err := Open("testdata/minimal.docx")
	if err != nil {
		t.Fatal(err)
	}
	p := doc.Paragraphs()[0]
	p.SetText("BASE")
	p.AddRun().SetText("EXTRA")

	reopened := saveAndReopen(t, doc)
	if got, want := reopened.Paragraphs()[0].Text(), "BASEEXTRA"; got != want {
		t.Errorf("paragraph text = %q, want %q", got, want)
	}
}

// Run-level same-pattern sites (C153 analogue on CT_R): SetText writes text
// elements the child order does not track; AddTab/AddBreak must backfill them
// instead of flipping the run to childOrder-only marshaling and dropping them.
func TestNewDocument_RunSetTextThenTrackedAppends(t *testing.T) {
	doc := Create()
	r := doc.AddParagraph().AddRun()
	r.SetText("A")
	r.AddTab()
	r.AddBreak()

	data, err := doc.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	dx, ok := zipEntry(t, data, "word/document.xml")
	if !ok {
		t.Fatal("document.xml missing")
	}
	for _, want := range []string{">A</w:t>", "<w:tab/>", "<w:br/>"} {
		if !strings.Contains(string(dx), want) {
			t.Errorf("document.xml missing %q", want)
		}
	}

	// Multi-cycle: the reparsed run must keep its content through another save.
	reopened := saveAndReopen(t, doc)
	data2, err := reopened.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	dx2, ok := zipEntry(t, data2, "word/document.xml")
	if !ok {
		t.Fatal("document.xml missing after second cycle")
	}
	for _, want := range []string{">A</w:t>", "<w:tab/>", "<w:br/>"} {
		if !strings.Contains(string(dx2), want) {
			t.Errorf("after second cycle, document.xml missing %q", want)
		}
	}
}

// Run.Clear must reset the run's recorded child order so later appends do not
// duplicate content through stale references (C159 analogue on CT_R).
func TestOpenedDocument_RunClearThenSetText(t *testing.T) {
	doc, err := Open("testdata/minimal.docx")
	if err != nil {
		t.Fatal(err)
	}
	r := doc.Paragraphs()[0].Runs()[0]
	r.Clear()
	r.SetText("RUN-REPLACED")
	r.AddBreak()

	reopened := saveAndReopen(t, doc)
	if got, want := reopened.Paragraphs()[0].Text(), "RUN-REPLACED"; got != want {
		t.Errorf("paragraph text = %q, want %q", got, want)
	}
}
