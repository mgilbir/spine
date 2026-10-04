package render

import (
	"math"
	"testing"

	"github.com/mgilbir/forme/shape"
)

func TestTabAdvanceAlignments(t *testing.T) {
	// Glyphs of 10px at size 10px, one per byte.
	seg := func(text string) RichSegment {
		var glyphs []shape.Glyph
		if text != "\t" {
			for i := range text {
				glyphs = append(glyphs, shape.Glyph{GID: 1, Cluster: i, XAdvance: 1000})
			}
		}
		return RichSegment{Text: text, Glyphs: glyphs, Size: unit(10)}
	}
	after := []RichSegment{seg("ab"), seg("c.d"), seg("\t"), seg("ignored")}
	for _, tc := range []struct {
		name  string
		stops []TabStop
		pen   float64
		want  float64
	}{
		{"left", []TabStop{{At: unit(100)}}, 20, 80},
		{"right", []TabStop{{At: unit(100), Align: TabRight}}, 20, 30},
		{"center", []TabStop{{At: unit(100), Align: TabCenter}}, 20, 55},
		{"decimal", []TabStop{{At: unit(100), Align: TabDecimal}}, 20, 50},
		{"past every stop", []TabStop{{At: unit(10)}}, 20, 7},
		{"next stop", []TabStop{{At: unit(20)}, {At: unit(60)}}, 20, 40},
		{"no room", []TabStop{{At: unit(30), Align: TabRight}}, 20, 0},
	} {
		if got := tabAdvance(after, tc.stops, tc.pen, 7); math.Abs(got-tc.want) > 1e-6 {
			t.Errorf("%s: %v, want %v", tc.name, got, tc.want)
		}
	}
	// Without a full stop, decimal alignment ends the text at the stop.
	if got := tabAdvance([]RichSegment{seg("ab")}, []TabStop{{At: unit(100), Align: TabDecimal}}, 0, 7); got != 80 {
		t.Errorf("decimal without a point: %v", got)
	}
}
