package xlsx

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"
)

func sheet1XML(t *testing.T, data []byte) string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("zip: %v", err)
	}
	for _, f := range zr.File {
		if strings.Contains(f.Name, "worksheets/sheet1") && strings.HasSuffix(f.Name, ".xml") {
			rc, _ := f.Open()
			b, _ := io.ReadAll(rc)
			_ = rc.Close()
			return string(b)
		}
	}
	t.Fatal("sheet1.xml not found")
	return ""
}

func TestSetRichTextRoundTrip(t *testing.T) {
	wb := Create()
	sh := wb.AddSheet("S")
	c, _ := sh.Cell("A1")
	c.SetRichText([]TextRun{
		{Text: "Total: ", Font: &FontStyle{Bold: true}},
		{Text: "1,234", Font: &FontStyle{Italic: true, Color: "FF0000"}},
	})

	data, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	xml := sheet1XML(t, data)

	// Inline rich text: an <is> with two runs, bold on the first and
	// xml:space preserved on the trailing-space run.
	if !strings.Contains(xml, `t="inlineStr"`) {
		t.Errorf("cell is not inlineStr:\n%s", xml)
	}
	if !strings.Contains(xml, `<rPr><b/></rPr><t xml:space="preserve">Total: </t>`) {
		t.Errorf("first run wrong (bold + preserved space):\n%s", xml)
	}
	if !strings.Contains(xml, "<i/>") || !strings.Contains(xml, `rgb="FFFF0000"`) {
		t.Errorf("second run formatting (italic/color) missing:\n%s", xml)
	}

	// Reopen and read the runs back.
	wb2, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	c2, _ := wb2.Sheets()[0].Cell("A1")

	if got := c2.String(); got != "Total: 1,234" {
		t.Errorf("String() = %q, want %q", got, "Total: 1,234")
	}
	runs := c2.RichText()
	if len(runs) != 2 {
		t.Fatalf("RichText len = %d, want 2", len(runs))
	}
	if runs[0].Text != "Total: " || runs[0].Font == nil || !runs[0].Font.Bold {
		t.Errorf("run 0 = %+v (font %+v)", runs[0], runs[0].Font)
	}
	if runs[1].Text != "1,234" || runs[1].Font == nil || !runs[1].Font.Italic {
		t.Errorf("run 1 = %+v (font %+v)", runs[1], runs[1].Font)
	}
	if runs[1].Font.Color != "FFFF0000" {
		t.Errorf("run 1 color = %q, want FFFF0000", runs[1].Font.Color)
	}
}

func TestSetRichTextNilFont(t *testing.T) {
	wb := Create()
	sh := wb.AddSheet("S")
	c, _ := sh.Cell("A1")
	c.SetRichText([]TextRun{
		{Text: "plain"},
		{Text: "bold", Font: &FontStyle{Bold: true}},
	})
	data, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	xml := sheet1XML(t, data)
	// The first run has no rPr; the second does.
	if !strings.Contains(xml, "<r><t>plain</t></r>") {
		t.Errorf("nil-font run should have no rPr:\n%s", xml)
	}
	runs, err := reopenRuns(t, data)
	if err != nil {
		t.Fatal(err)
	}
	if runs[0].Font != nil {
		t.Errorf("run 0 font should be nil, got %+v", runs[0].Font)
	}
	if runs[1].Font == nil || !runs[1].Font.Bold {
		t.Errorf("run 1 should be bold")
	}
}

func reopenRuns(t *testing.T, data []byte) ([]TextRun, error) {
	t.Helper()
	wb, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}
	c, err := wb.Sheets()[0].Cell("A1")
	if err != nil {
		return nil, err
	}
	return c.RichText(), nil
}

// TestRichTextReplacesValue: SetRichText clears any prior numeric/string value.
func TestRichTextReplacesValue(t *testing.T) {
	wb := Create()
	sh := wb.AddSheet("S")
	c, _ := sh.Cell("A1")
	c.SetFloat(42)
	c.SetRichText([]TextRun{{Text: "now text"}})
	if c.cell.V != nil {
		t.Error("SetRichText should clear the numeric value")
	}
	if c.String() != "now text" {
		t.Errorf("String() = %q", c.String())
	}
}

// TestRichTextOnPlainString: RichText() of a plain string cell returns one run.
func TestRichTextOnPlainString(t *testing.T) {
	wb := Create()
	sh := wb.AddSheet("S")
	c, _ := sh.Cell("A1")
	c.SetString("hello")
	runs := c.RichText()
	if len(runs) != 1 || runs[0].Text != "hello" || runs[0].Font != nil {
		t.Errorf("plain string RichText = %+v", runs)
	}
}

// TestRichTextEmpty: empty runs produce an empty inline string.
func TestRichTextEmpty(t *testing.T) {
	wb := Create()
	sh := wb.AddSheet("S")
	c, _ := sh.Cell("A1")
	c.SetRichText(nil)
	data, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	if _, err := OpenReader(bytes.NewReader(data), int64(len(data))); err != nil {
		t.Fatalf("reopen: %v", err)
	}
}
