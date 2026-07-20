package xlsx

import (
	"bytes"
	"testing"

	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

func strptr(s string) *string { return &s }

func TestReplaceText_InlineString(t *testing.T) {
	wb := Create()
	sh := wb.AddSheet("S")
	c, _ := sh.Cell("A1")
	c.SetString("Dear {{name}}, welcome")

	wb.ReplaceText(map[string]string{"{{name}}": "Alice"})

	if got := c.String(); got != "Dear Alice, welcome" {
		t.Errorf("cell = %q, want %q", got, "Dear Alice, welcome")
	}
}

func TestReplaceText_SheetScoped(t *testing.T) {
	wb := Create()
	s1 := wb.AddSheet("S1")
	s2 := wb.AddSheet("S2")
	c1, _ := s1.Cell("A1")
	c1.SetString("{{k}}")
	c2, _ := s2.Cell("A1")
	c2.SetString("{{k}}")

	// Sheet.ReplaceText touches only its own sheet.
	s1.ReplaceText(map[string]string{"{{k}}": "V"})

	if got := c1.String(); got != "V" {
		t.Errorf("s1 A1 = %q, want %q", got, "V")
	}
	if got := c2.String(); got != "{{k}}" {
		t.Errorf("s2 A1 = %q, want it unchanged", got)
	}
}

// TestReplaceText_RichRunsAcrossThree: a phrase split across three inline rich
// runs is replaced by a single run carrying the first run's font.
func TestReplaceText_RichRunsAcrossThree(t *testing.T) {
	wb := Create()
	sh := wb.AddSheet("S")
	c, _ := sh.Cell("A1")
	c.SetRichText([]TextRun{
		{Text: "{{", Font: &FontStyle{Bold: true}},
		{Text: "who"},
		{Text: "}}"},
	})

	wb.ReplaceText(map[string]string{"{{who}}": "World"})

	if got := c.String(); got != "World" {
		t.Fatalf("cell = %q, want %q", got, "World")
	}
	runs := c.RichText()
	if len(runs) != 1 {
		t.Fatalf("run count = %d, want 1", len(runs))
	}
	if runs[0].Text != "World" {
		t.Errorf("run text = %q, want %q", runs[0].Text, "World")
	}
	if runs[0].Font == nil || !runs[0].Font.Bold {
		t.Errorf("replacement run did not inherit the first run's bold font: %+v", runs[0].Font)
	}
}

// TestReplaceText_SharedString: a shared-string cell is replaced by converting
// it to an inline string; the shared table itself is left untouched so other
// cells referencing the same entry are not affected.
func TestReplaceText_SharedString(t *testing.T) {
	wb := Create()
	sh := wb.AddSheet("S")
	c, _ := sh.Cell("A1")

	wb.sharedStrings = &oxml.CT_Sst{Si: []oxml.CT_Rst{{T: strptr("Hi {{who}}")}}}
	wb.buildStringTable()
	c.cell.T = "s"
	c.cell.V = strptr("0")

	wb.ReplaceText(map[string]string{"{{who}}": "Bob"})

	if got := c.String(); got != "Hi Bob" {
		t.Errorf("cell = %q, want %q", got, "Hi Bob")
	}
	if c.cell.T != "inlineStr" {
		t.Errorf("cell type = %q, want inlineStr (converted from shared)", c.cell.T)
	}
	// The shared entry must be unchanged (not mutated in place).
	if got := *wb.sharedStrings.Si[0].T; got != "Hi {{who}}" {
		t.Errorf("shared string entry mutated: %q", got)
	}
}

func TestReplaceText_SkipsFormula(t *testing.T) {
	wb := Create()
	sh := wb.AddSheet("S")
	c, _ := sh.Cell("A1")
	c.SetFormula(`"{{k}}"`)

	wb.ReplaceText(map[string]string{"{{k}}": "V"})

	if got := c.Formula(); got != `"{{k}}"` {
		t.Errorf("formula changed: %q", got)
	}
}

func TestReplaceText_NoOpAndEmpty(t *testing.T) {
	wb := Create()
	sh := wb.AddSheet("S")
	c, _ := sh.Cell("A1")
	c.SetString("{{name}}")

	wb.ReplaceText(map[string]string{"{{absent}}": "X"})
	wb.ReplaceText(map[string]string{})
	wb.ReplaceText(map[string]string{"": "X"})

	if got := c.String(); got != "{{name}}" {
		t.Errorf("cell changed unexpectedly: %q", got)
	}
}

// TestReplaceText_ByteIdenticalWhenNoMatch is the fidelity guarantee for xlsx.
func TestReplaceText_ByteIdenticalWhenNoMatch(t *testing.T) {
	load := func() *Workbook {
		wb, err := Open("testdata/minimal.xlsx")
		if err != nil {
			t.Fatal(err)
		}
		return wb
	}

	baseline, err := load().SaveBytes()
	if err != nil {
		t.Fatal(err)
	}

	mutated := load()
	mutated.ReplaceText(map[string]string{"__does_not_exist__": "X"})
	after, err := mutated.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(baseline, after) {
		t.Errorf("no-match ReplaceText changed the saved bytes (%d vs %d)", len(baseline), len(after))
	}
}
