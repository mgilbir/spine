package render

import (
	"context"
	"testing"

	"github.com/mgilbir/forme/fonttest"
	"github.com/mgilbir/forme/shape"
)

// FuzzRichLinesBidirectional checks that lines of mixed-direction, East Asian
// and Latin text cover their text exactly once, in segments that sit end to
// end, whatever the width and direction.
func FuzzRichLinesBidirectional(f *testing.F) {
	for _, seed := range []string{"A אב B", "אבג 12 (א) ב", "بب Aب", "日本語、。A", "אב\u200f\u200cג", "(((א)))", "1.2 א", "", "א\tב\tג", "A\tאב\t1"} {
		f.Add(seed, uint8(3), true)
		f.Add(seed, uint8(40), false)
	}
	var glyphs []fonttest.Glyph
	for _, r := range "אבג AB12().日本語、。بﺐﺑﺒ\u200c\u200d\u200e\u200f" {
		glyphs = append(glyphs, fonttest.Glyph{Rune: r, Advance: 700, HasShape: r != ' '})
	}
	face, err := shape.Load(fonttest.SFNT(fonttest.SFNTOptions{Glyphs: glyphs}))
	if err != nil {
		f.Fatal(err)
	}
	f.Fuzz(func(t *testing.T, text string, width uint8, rtl bool) {
		if len(text) > 512 || width == 0 {
			t.Skip()
		}
		layout, err := NewTextLayout(Limits{})
		if err != nil {
			t.Fatal(err)
		}
		layout.AllowOverflow()
		spans := []Span{{Face: face, Size: unit(10), Text: text, BreakWord: true, TabStop: unit(20), Tabs: []TabStop{{At: unit(30)}, {At: unit(70), Align: TabRight}, {At: unit(90), Align: TabCenter}}}}
		lines, err := layout.RichLinesWith(context.Background(), spans, unit(float64(width)), RepertoireBidi, RichOptions{RTL: rtl, HangPunct: true})
		if err != nil {
			return
		}
		covered := 0
		for _, l := range lines {
			type part struct{ at, n int }
			var parts []part
			x := unit(0)
			for _, sg := range l.Segments {
				if sg.X != x || sg.Offset < 0 || sg.Offset+len(sg.Text) > len(text) || text[sg.Offset:sg.Offset+len(sg.Text)] != sg.Text {
					t.Fatalf("segment %+v in %q", sg, text)
				}
				for _, g := range sg.Glyphs {
					if g.Cluster < 0 || g.Cluster >= len(sg.Text) {
						t.Fatalf("cluster %d in %q", g.Cluster, sg.Text)
					}
				}
				x += sg.Width
				parts = append(parts, part{sg.Offset, len(sg.Text)})
			}
			for len(parts) > 0 {
				i := 0
				for k := range parts {
					if parts[k].at < parts[i].at {
						i = k
					}
				}
				if parts[i].at != covered {
					t.Fatalf("line text from %d, want %d, in %q", parts[i].at, covered, text)
				}
				covered += parts[i].n
				parts = append(parts[:i], parts[i+1:]...)
			}
		}
		if covered != len(text) {
			t.Fatalf("covered %d of %q", covered, text)
		}
	})
}
