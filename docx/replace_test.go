package docx

import (
	"bytes"
	"strings"
	"testing"
)

// buildThreeRunParagraph adds a paragraph whose text "{{name}}" is split across
// three runs ("{{", "name", "}}"), with the first run bold, so a replacement
// spanning all three must consolidate them and carry the first run's formatting.
func buildThreeRunParagraph(p *Paragraph) {
	r1 := p.AddRun()
	r1.SetText("{{")
	r1.SetBold(true)
	p.AddRun().SetText("name")
	p.AddRun().SetText("}}")
}

func TestReplaceText_SplitAcrossThreeRuns(t *testing.T) {
	doc := Create()
	p := doc.AddParagraph()
	buildThreeRunParagraph(p)

	doc.ReplaceText(map[string]string{"{{name}}": "John Smith"})

	if got := p.Text(); got != "John Smith" {
		t.Fatalf("paragraph text = %q, want %q", got, "John Smith")
	}
	runs := p.Runs()
	// The three source runs consolidate into the single replacement run.
	if len(runs) != 1 {
		t.Fatalf("run count = %d, want 1", len(runs))
	}
	if runs[0].Text() != "John Smith" {
		t.Errorf("run text = %q, want %q", runs[0].Text(), "John Smith")
	}
	if !runs[0].Bold() {
		t.Error("replacement run did not inherit the first run's bold formatting")
	}
}

func TestReplaceText_PreservesSurroundingRunFormatting(t *testing.T) {
	doc := Create()
	p := doc.AddParagraph()
	// "Hello " (italic) + "{{" + "x" + "}}" (match) + "!" (bold)
	pre := p.AddRun()
	pre.SetText("Hello ")
	pre.SetItalic(true)
	p.AddRun().SetText("{{")
	p.AddRun().SetText("x")
	p.AddRun().SetText("}}")
	post := p.AddRun()
	post.SetText("!")
	post.SetBold(true)

	doc.ReplaceText(map[string]string{"{{x}}": "World"})

	if got := p.Text(); got != "Hello World!" {
		t.Fatalf("paragraph text = %q, want %q", got, "Hello World!")
	}
	runs := p.Runs()
	if len(runs) != 3 {
		t.Fatalf("run count = %d, want 3 (prefix, middle, suffix)", len(runs))
	}
	if runs[0].Text() != "Hello " || !runs[0].Italic() {
		t.Errorf("prefix run = %q italic=%v, want %q italic=true", runs[0].Text(), runs[0].Italic(), "Hello ")
	}
	if runs[2].Text() != "!" || !runs[2].Bold() {
		t.Errorf("suffix run = %q bold=%v, want %q bold=true", runs[2].Text(), runs[2].Bold(), "!")
	}
}

func TestReplaceText_TableCell(t *testing.T) {
	doc := Create()
	tbl := doc.AddTable(1, 1)
	cell := tbl.Rows()[0].Cells()[0]
	// Reuse the default empty cell paragraph.
	p := cell.Paragraphs()[0]
	buildThreeRunParagraph(p)

	doc.ReplaceText(map[string]string{"{{name}}": "Jane"})

	if got := cell.Text(); got != "Jane" {
		t.Errorf("table cell text = %q, want %q", got, "Jane")
	}
}

func TestReplaceText_Header(t *testing.T) {
	// Build a document with a header containing a split template key, save, and
	// reopen so the header lives as a preserved raw part — this exercises the
	// modified-part flagging that makes the edit survive the save.
	doc := Create()
	doc.AddParagraphWithText("body")
	h := doc.AddHeader(HeaderDefault)
	hp := h.AddParagraph()
	buildThreeRunParagraph(hp)

	reopened := saveAndReopen(t, doc)
	reopened.ReplaceText(map[string]string{"{{name}}": "ACME"})
	final := saveAndReopen(t, reopened)

	if got := headerText(final); !strings.Contains(got, "ACME") {
		t.Errorf("header text = %q, want it to contain %q", got, "ACME")
	}
	if strings.Contains(headerText(final), "{{name}}") {
		t.Error("header still contains the unreplaced key")
	}
}

// headerText concatenates the text of every header paragraph.
func headerText(d *Document) string {
	var sb strings.Builder
	for _, hp := range d.headers {
		if hp == nil || hp.hdr == nil {
			continue
		}
		for _, p := range hp.hdr.AllParagraphs() {
			sb.WriteString(p.Text())
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

func TestReplaceText_NoOpWhenKeyAbsent(t *testing.T) {
	doc := Create()
	p := doc.AddParagraph()
	buildThreeRunParagraph(p)

	doc.ReplaceText(map[string]string{"{{absent}}": "X"})

	if got := p.Text(); got != "{{name}}" {
		t.Errorf("text changed on absent key: %q", got)
	}
	// The runs must not have been consolidated.
	if len(p.Runs()) != 3 {
		t.Errorf("run count = %d, want 3 (unchanged)", len(p.Runs()))
	}
}

func TestReplaceText_EmptyInputs(t *testing.T) {
	doc := Create()
	p := doc.AddParagraphWithText("{{name}}")

	// Empty map is a no-op.
	doc.ReplaceText(map[string]string{})
	// Empty key is ignored.
	doc.ReplaceText(map[string]string{"": "X"})

	if got := p.Text(); got != "{{name}}" {
		t.Errorf("text changed unexpectedly: %q", got)
	}
}

// TestReplaceText_ByteIdenticalWhenNoMatch is the fidelity guarantee: a
// ReplaceText that matches nothing must leave the saved bytes untouched.
func TestReplaceText_ByteIdenticalWhenNoMatch(t *testing.T) {
	orig, err := Open("testdata/minimal.docx")
	if err != nil {
		t.Fatal(err)
	}
	baseline, err := orig.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}

	mutated, err := Open("testdata/minimal.docx")
	if err != nil {
		t.Fatal(err)
	}
	mutated.ReplaceText(map[string]string{"__does_not_exist__": "X"})
	after, err := mutated.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(baseline, after) {
		t.Errorf("no-match ReplaceText changed the saved bytes (%d vs %d)", len(baseline), len(after))
	}
}
