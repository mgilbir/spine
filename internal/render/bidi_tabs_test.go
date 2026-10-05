package render

import (
	"context"
	"testing"
)

// TestRichLinesRightToLeftTabs checks tabs in a right-to-left paragraph as
// PowerPoint places them: measured from the line's start, at its right end,
// a left stop putting the left edge of the text after the tab at the stop and
// a right stop its right edge.
func TestRichLinesRightToLeftTabs(t *testing.T) {
	face := bidiFixtureFace(t)
	lines := func(stops []TabStop, text string) []RichLine {
		t.Helper()
		layout, _ := NewTextLayout(Limits{})
		got, err := layout.RichLinesWith(context.Background(), []Span{{Face: face, Size: unit(10), Text: text, TabStop: unit(20), Tabs: stops}}, unit(500), RepertoireBidi, RichOptions{RTL: true})
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	// Reading order: aleph (10), tab, bet (20), tab, gimel (30).
	got := lines([]TabStop{{At: unit(40)}, {At: unit(100), Align: TabRight}}, "א\tב\tג")
	if len(got) != 1 || got[0].Width != unit(130) || got[0].TabsApprox || !got[0].TabsMoved {
		t.Fatalf("line: %+v", got)
	}
	// Drawn left to right: gimel, tab, bet, tab, aleph.
	want := []struct {
		text string
		x, w float64
	}{{"ג", 0, 30}, {"\t", 30, 60}, {"ב", 90, 20}, {"\t", 110, 10}, {"א", 120, 10}}
	if len(got[0].Segments) != len(want) {
		t.Fatalf("segments: %+v", got[0].Segments)
	}
	for i, w := range want {
		sg := got[0].Segments[i]
		if sg.Text != w.text || sg.X != unit(w.x) || sg.Width != unit(w.w) {
			t.Errorf("segment %d: %q at %v, %v wide; want %q at %v, %v wide", i, sg.Text, sg.X, sg.Width, w.text, w.x, w.w)
		}
	}
	// A centred stop, a stop that text has passed and no stop at all are
	// placed by rules that were not measured.
	if got = lines([]TabStop{{At: unit(40), Align: TabCenter}}, "א\tב"); !got[0].TabsApprox {
		t.Fatalf("centred stop: %+v", got[0])
	}
	if got = lines(nil, "א\tב"); !got[0].TabsApprox || got[0].Segments[1].Width != unit(10) {
		t.Fatalf("even stop: %+v", got[0])
	}
}
