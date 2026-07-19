package xlsx

import (
	"bytes"
	"testing"

	"github.com/mgilbir/spine/common/enum"
)

func TestFontStyle_StrikeVertAlignUnderline(t *testing.T) {
	wb := Create()
	sm := wb.Styles()

	idx, err := sm.NewCellStyle(CellStyle{
		Font: &FontStyle{
			Name:           "Calibri",
			Strike:         true,
			VertAlign:      enum.VerticalAlignRunSuperscript,
			UnderlineStyle: UnderlineDouble,
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	cs, err := sm.GetCellStyle(idx)
	if err != nil {
		t.Fatal(err)
	}
	if cs.Font == nil {
		t.Fatal("expected font")
	}
	if !cs.Font.Strike {
		t.Error("expected strike")
	}
	if cs.Font.VertAlign != enum.VerticalAlignRunSuperscript {
		t.Errorf("VertAlign = %q, want superscript", cs.Font.VertAlign)
	}
	if cs.Font.UnderlineStyle != UnderlineDouble {
		t.Errorf("UnderlineStyle = %q, want double", cs.Font.UnderlineStyle)
	}
	if !cs.Font.Underline {
		t.Error("Underline bool should be true for a double underline")
	}
}

// TestFontStyle_VertAlignDedup verifies fonts differing only in vertical
// alignment are not merged (fontEqual must compare vertAlign).
func TestFontStyle_VertAlignDedup(t *testing.T) {
	wb := Create()
	sm := wb.Styles()

	sup, err := sm.NewCellStyle(CellStyle{Font: &FontStyle{VertAlign: enum.VerticalAlignRunSuperscript}})
	if err != nil {
		t.Fatal(err)
	}
	sub, err := sm.NewCellStyle(CellStyle{Font: &FontStyle{VertAlign: enum.VerticalAlignRunSubscript}})
	if err != nil {
		t.Fatal(err)
	}
	if sup == sub {
		t.Fatal("superscript and subscript fonts must not dedupe to the same style")
	}
}

func TestFontStyle_CellRoundTrip(t *testing.T) {
	wb := Create()
	sh := wb.AddSheet("S")
	c, _ := sh.Cell("A1")
	c.SetString("x")
	if err := c.SetStyle(CellStyle{
		Font: &FontStyle{Strike: true, VertAlign: enum.VerticalAlignRunSubscript, UnderlineStyle: UnderlineSingleAccounting},
	}); err != nil {
		t.Fatal(err)
	}

	data, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	wb2, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	c2, _ := wb2.Sheets()[0].Cell("A1")
	si := c2.StyleIndex()
	if si == nil {
		t.Fatal("reopened cell lost its style index")
	}
	cs, err := wb2.Styles().GetCellStyle(*si)
	if err != nil {
		t.Fatal(err)
	}
	if cs.Font == nil {
		t.Fatal("reopened font is nil")
	}
	if !cs.Font.Strike {
		t.Error("strike lost on round trip")
	}
	if cs.Font.VertAlign != enum.VerticalAlignRunSubscript {
		t.Errorf("VertAlign = %q, want subscript", cs.Font.VertAlign)
	}
	if cs.Font.UnderlineStyle != UnderlineSingleAccounting {
		t.Errorf("UnderlineStyle = %q, want singleAccounting", cs.Font.UnderlineStyle)
	}
}

func TestRichText_StrikeVertAlignUnderlineRoundTrip(t *testing.T) {
	wb := Create()
	sh := wb.AddSheet("S")
	c, _ := sh.Cell("A1")
	c.SetRichText([]TextRun{
		{Text: "H", Font: &FontStyle{VertAlign: enum.VerticalAlignRunSuperscript}},
		{Text: "2", Font: &FontStyle{VertAlign: enum.VerticalAlignRunSubscript}},
		{Text: "O", Font: &FontStyle{Strike: true, UnderlineStyle: UnderlineDouble}},
	})

	data, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	wb2, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	c2, _ := wb2.Sheets()[0].Cell("A1")
	runs := c2.RichText()
	if len(runs) != 3 {
		t.Fatalf("RichText len = %d, want 3", len(runs))
	}
	if runs[0].Font == nil || runs[0].Font.VertAlign != enum.VerticalAlignRunSuperscript {
		t.Errorf("run 0 vertAlign = %+v, want superscript", runs[0].Font)
	}
	if runs[1].Font == nil || runs[1].Font.VertAlign != enum.VerticalAlignRunSubscript {
		t.Errorf("run 1 vertAlign = %+v, want subscript", runs[1].Font)
	}
	if runs[2].Font == nil || !runs[2].Font.Strike || runs[2].Font.UnderlineStyle != UnderlineDouble {
		t.Errorf("run 2 = %+v, want strike + double underline", runs[2].Font)
	}
}
