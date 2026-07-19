package docx

import (
	"bytes"
	"testing"

	"github.com/mgilbir/spine/common/enum"
)

// reopenDoc saves the document to bytes and reopens it, so a test can assert a
// property survives the save/reopen round trip.
func reopenDoc(t *testing.T, doc *Document) *Document {
	t.Helper()
	data, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes error: %v", err)
	}
	reopened, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("OpenReader error: %v", err)
	}
	t.Cleanup(func() { _ = reopened.Close() })
	return reopened
}

func firstRun(t *testing.T, doc *Document) *Run {
	t.Helper()
	paras := doc.Paragraphs()
	if len(paras) == 0 {
		t.Fatal("document has no paragraphs")
	}
	runs := paras[0].Runs()
	if len(runs) == 0 {
		t.Fatal("first paragraph has no runs")
	}
	return runs[0]
}

func TestRun_HighlightRoundTrip(t *testing.T) {
	doc := Create()
	run := doc.AddParagraph().AddRun()
	run.SetText("highlighted")
	run.SetHighlight("yellow")
	if got := run.Highlight(); got != "yellow" {
		t.Fatalf("Highlight() = %q, want yellow", got)
	}

	run2 := firstRun(t, reopenDoc(t, doc))
	if got := run2.Highlight(); got != "yellow" {
		t.Fatalf("after round trip Highlight() = %q, want yellow", got)
	}

	// Clearing removes the element.
	run2.SetHighlight("")
	if got := run2.Highlight(); got != "" {
		t.Fatalf("after clear Highlight() = %q, want empty", got)
	}
}

func TestRun_VerticalAlignRoundTrip(t *testing.T) {
	for _, tc := range []struct {
		name  string
		align enum.VerticalAlignRun
	}{
		{"superscript", enum.VerticalAlignRunSuperscript},
		{"subscript", enum.VerticalAlignRunSubscript},
		{"baseline", enum.VerticalAlignRunBaseline},
	} {
		t.Run(tc.name, func(t *testing.T) {
			doc := Create()
			run := doc.AddParagraph().AddRun()
			run.SetText("x")
			run.SetVerticalAlign(tc.align)
			if got := run.VerticalAlign(); got != tc.align {
				t.Fatalf("VerticalAlign() = %q, want %q", got, tc.align)
			}

			run2 := firstRun(t, reopenDoc(t, doc))
			if got := run2.VerticalAlign(); got != tc.align {
				t.Fatalf("after round trip VerticalAlign() = %q, want %q", got, tc.align)
			}
		})
	}
}

func TestRun_SuperSubscriptHelpers(t *testing.T) {
	doc := Create()
	run := doc.AddParagraph().AddRun()
	run.SetSuperscript(true)
	if !run.Superscript() || run.Subscript() {
		t.Fatal("SetSuperscript(true) should make Superscript true and Subscript false")
	}
	run.SetSubscript(true)
	if !run.Subscript() || run.Superscript() {
		t.Fatal("SetSubscript(true) should make Subscript true and Superscript false")
	}
	run.SetSuperscript(false)
	if run.Superscript() || run.Subscript() {
		t.Fatal("SetSuperscript(false) should reset to baseline")
	}
	if got := run.VerticalAlign(); got != enum.VerticalAlignRunBaseline {
		t.Fatalf("VerticalAlign() = %q, want baseline", got)
	}
}

func TestRun_CapsRoundTrip(t *testing.T) {
	doc := Create()
	run := doc.AddParagraph().AddRun()
	run.SetText("caps")
	run.SetCaps(true)
	run.SetSmallCaps(false)
	if !run.Caps() {
		t.Fatal("Caps() should be true")
	}

	run2 := firstRun(t, reopenDoc(t, doc))
	if !run2.Caps() {
		t.Fatal("after round trip Caps() should be true")
	}
	if run2.SmallCaps() {
		t.Fatal("SmallCaps() should be false (explicit off)")
	}
	run2.ClearCaps()
	if run2.Caps() {
		t.Fatal("ClearCaps should make Caps() false")
	}

	// smallCaps on.
	doc2 := Create()
	r := doc2.AddParagraph().AddRun()
	r.SetSmallCaps(true)
	if got := firstRun(t, reopenDoc(t, doc2)); !got.SmallCaps() {
		t.Fatal("after round trip SmallCaps() should be true")
	}
}

func TestRun_UnderlineStyleRoundTrip(t *testing.T) {
	doc := Create()
	run := doc.AddParagraph().AddRun()
	run.SetText("u")
	run.SetUnderlineStyle(UnderlineDouble)
	run.SetUnderlineColor("FF0000")
	if got := run.UnderlineStyle(); got != UnderlineDouble {
		t.Fatalf("UnderlineStyle() = %q, want double", got)
	}
	if got := run.UnderlineColor(); got != "FF0000" {
		t.Fatalf("UnderlineColor() = %q, want FF0000", got)
	}
	// The boolean getter still reflects the underline.
	if !run.Underline() {
		t.Fatal("Underline() should be true for a double underline")
	}

	run2 := firstRun(t, reopenDoc(t, doc))
	if got := run2.UnderlineStyle(); got != UnderlineDouble {
		t.Fatalf("after round trip UnderlineStyle() = %q, want double", got)
	}
	if got := run2.UnderlineColor(); got != "FF0000" {
		t.Fatalf("after round trip UnderlineColor() = %q, want FF0000", got)
	}

	// The legacy bool setter keeps working.
	run2.SetUnderline(true)
	if got := run2.UnderlineStyle(); got != UnderlineSingle {
		t.Fatalf("SetUnderline(true) UnderlineStyle() = %q, want single", got)
	}
}

func TestRun_CharacterSpacingRoundTrip(t *testing.T) {
	doc := Create()
	run := doc.AddParagraph().AddRun()
	run.SetText("spaced")
	run.SetCharacterSpacing(2) // 2 points -> 40 twips
	run.SetPosition(3)         // 3 points -> 6 half-points
	run.SetKerning(8)          // 8 points -> 16 half-points
	if got := run.CharacterSpacing(); got != 2 {
		t.Fatalf("CharacterSpacing() = %v, want 2", got)
	}
	if got := run.Position(); got != 3 {
		t.Fatalf("Position() = %v, want 3", got)
	}
	if got := run.Kerning(); got != 8 {
		t.Fatalf("Kerning() = %v, want 8", got)
	}

	run2 := firstRun(t, reopenDoc(t, doc))
	if got := run2.CharacterSpacing(); got != 2 {
		t.Fatalf("after round trip CharacterSpacing() = %v, want 2", got)
	}
	if got := run2.Position(); got != 3 {
		t.Fatalf("after round trip Position() = %v, want 3", got)
	}
	if got := run2.Kerning(); got != 8 {
		t.Fatalf("after round trip Kerning() = %v, want 8", got)
	}

	// Negative (condensed) spacing.
	doc3 := Create()
	r := doc3.AddParagraph().AddRun()
	r.SetCharacterSpacing(-1.5)
	if got := firstRun(t, reopenDoc(t, doc3)).CharacterSpacing(); got != -1.5 {
		t.Fatalf("after round trip negative CharacterSpacing() = %v, want -1.5", got)
	}
}

func TestParagraph_TabStopsRoundTrip(t *testing.T) {
	doc := Create()
	p := doc.AddParagraph()
	p.AddRun().SetText("tabbed")
	p.AddTabStop(TabStop{Position: 72, Alignment: TabAlignCenter, Leader: TabLeaderDot})
	p.AddTabStop(TabStop{Position: 144, Alignment: TabAlignRight})

	stops := p.Tabs()
	if len(stops) != 2 {
		t.Fatalf("Tabs() len = %d, want 2", len(stops))
	}
	if stops[0].Position != 72 || stops[0].Alignment != TabAlignCenter || stops[0].Leader != TabLeaderDot {
		t.Fatalf("tab 0 = %+v", stops[0])
	}
	if stops[1].Position != 144 || stops[1].Alignment != TabAlignRight || stops[1].Leader != "" {
		t.Fatalf("tab 1 = %+v", stops[1])
	}

	reopened := reopenDoc(t, doc)
	rp := reopened.Paragraphs()
	if len(rp) == 0 {
		t.Fatal("no paragraphs after reopen")
	}
	got := rp[0].Tabs()
	if len(got) != 2 {
		t.Fatalf("after round trip Tabs() len = %d, want 2", len(got))
	}
	if got[0].Position != 72 || got[0].Alignment != TabAlignCenter || got[0].Leader != TabLeaderDot {
		t.Fatalf("after round trip tab 0 = %+v", got[0])
	}
	if got[1].Position != 144 || got[1].Alignment != TabAlignRight {
		t.Fatalf("after round trip tab 1 = %+v", got[1])
	}

	rp[0].ClearTabStops()
	if len(rp[0].Tabs()) != 0 {
		t.Fatal("ClearTabStops should remove all tab stops")
	}
}
