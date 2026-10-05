package docxrender

import "testing"

func TestUnderlineAndStrikeGeometry(t *testing.T) {
	for _, tc := range []struct {
		name, rpr string
		bars      int
		colours   []string
	}{
		{"single", `<w:u w:val="single"/>`, 1, []string{"000000"}},
		{"double", `<w:u w:val="double"/>`, 2, []string{"000000", "000000"}},
		{"strike", `<w:strike/>`, 1, []string{"000000"}},
		{"coloured underline", `<w:u w:val="single" w:color="FF0000"/>`, 1, []string{"ff0000"}},
		{"underline and strike", `<w:u w:val="single" w:color="FF0000"/><w:strike/>`, 2, []string{"ff0000", "000000"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p, _ := wordTestRender(t, wordTestBody(wordTestPara("", wordTestRun(tc.rpr, "word"))))
			bars := p.rects(1)
			if len(bars) != tc.bars {
				t.Fatalf("%d bars", len(bars))
			}
			text, _ := p.textOf(1, "word")
			for i, b := range bars {
				if !near(b.Rect.X.Px(), text.At.X.Px()) || !near(b.Rect.W.Px(), 24) {
					t.Errorf("bar %d spans %v+%v", i, b.Rect.X.Px(), b.Rect.W.Px())
				}
				if rgbOf(b.Color) != tc.colours[i] {
					t.Errorf("bar %d colour %s", i, rgbOf(b.Color))
				}
			}
			if tc.name == "single" && !(bars[0].Rect.Y.Px() > text.At.Y.Px() && bars[0].Rect.Y.Px() < text.At.Y.Px()+2) {
				t.Errorf("underline at %v, baseline %v", bars[0].Rect.Y.Px(), text.At.Y.Px())
			}
			if tc.name == "double" && !(bars[1].Rect.Y.Px() > bars[0].Rect.Y.Px()+1) {
				t.Errorf("double lines at %v and %v", bars[0].Rect.Y.Px(), bars[1].Rect.Y.Px())
			}
			if tc.name == "strike" && !(bars[0].Rect.Y.Px() < text.At.Y.Px()) {
				t.Errorf("strike below baseline: %v", bars[0].Rect.Y.Px())
			}
		})
	}
}
