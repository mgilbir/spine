package docxrender

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/mgilbir/spine/render"
)

// tabbed is a paragraph of text pieces separated by tabs. The text area is 260
// pixels wide from x=20 and each glyph is 6 pixels wide; 15 twips are a pixel.
func tabbed(ppr string, pieces ...string) string {
	var sb strings.Builder
	sb.WriteString("<w:r>")
	for i, p := range pieces {
		if i > 0 {
			sb.WriteString("<w:tab/>")
		}
		sb.WriteString(`<w:t xml:space="preserve">` + p + `</w:t>`)
	}
	sb.WriteString("</w:r>")
	return wordTestPara(ppr, sb.String())
}

func tabStops(stops ...string) string {
	return "<w:tabs>" + strings.Join(stops, "") + "</w:tabs>"
}

func tabStop(val string, px int, leader string) string {
	s := `<w:tab w:val="` + val + `" w:pos="` + strconv.Itoa(px*15) + `"`
	if leader != "" {
		s += ` w:leader="` + leader + `"`
	}
	return s + "/>"
}

func (p *Pages) startOf(page int, text string, nth int) float64 {
	n := 0
	for _, tx := range p.texts(page) {
		if tx.text == text {
			if n == nth {
				return tx.x
			}
			n++
		}
	}
	return -1
}

func TestCustomTabStopsAlign(t *testing.T) {
	tests := []struct {
		name   string
		ppr    string
		pieces []string
		// want maps a text to the x its first occurrence starts at.
		want map[string]float64
	}{
		{"left", tabStops(tabStop("left", 100, "")), []string{"a", "b"}, map[string]float64{"b": 120}},
		{"right", tabStops(tabStop("right", 200, "")), []string{"a", "bb"}, map[string]float64{"bb": 20 + 200 - 12}},
		{"center", tabStops(tabStop("center", 200, "")), []string{"a", "bbbb"}, map[string]float64{"bbbb": 20 + 200 - 12}},
		{"two stops", tabStops(tabStop("left", 100, ""), tabStop("right", 200, "")), []string{"a", "b", "c"}, map[string]float64{"b": 120, "c": 20 + 200 - 6}},
		{"past the last stop uses the default grid", tabStops(tabStop("left", 40, "")), []string{"aaaaaaaaaa", "b"}, map[string]float64{"b": 20 + 96}},
		{"a stop at the pen moves on", tabStops(tabStop("left", 6, ""), tabStop("left", 100, "")), []string{"a", "b"}, map[string]float64{"b": 120}},
		{"too wide for a right stop", tabStops(tabStop("right", 20, "")), []string{"a", "bbbbbbbbbb"}, map[string]float64{"bbbbbbbbbb": 26}},
		{"indent not a multiple of the stop", `<w:ind w:left="360"/>`, []string{"a", "b"}, map[string]float64{"b": 20 + 48}},
		{"indent that is a multiple", `<w:ind w:left="720"/>`, []string{"a", "b"}, map[string]float64{"b": 20 + 96}},
		{"hanging indent stop then a custom stop", `<w:ind w:left="720" w:hanging="360"/>` + tabStops(tabStop("left", 150, "")), []string{"1.", "text", "x"}, map[string]float64{"text": 20 + 48, "x": 20 + 150}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			o := newWordTestOpts(t, false)
			doc := wordTestDoc(t, wordTestBody(tabbed(tc.ppr, tc.pieces...)), wordTestParts{styles: wordTestStyles})
			p, err := Prepare(context.Background(), doc, o.Options)
			if err != nil {
				t.Fatal(err)
			}
			for text, want := range tc.want {
				if got := p.startOf(1, text, 0); !near(got, want) {
					t.Errorf("%q starts at %v, want %v", text, got, want)
				}
			}
		})
	}
}

func TestClearedAndInheritedTabStops(t *testing.T) {
	styles := wordTestStyles + `<w:style w:type="paragraph" w:styleId="Tabbed"><w:name w:val="Tabbed"/><w:pPr>` + tabStops(tabStop("left", 100, ""), tabStop("left", 150, "")) + `</w:pPr></w:style>`
	body := wordTestBody(
		tabbed(`<w:pStyle w:val="Tabbed"/>`, "a", "b", "c"),
		// Clearing the stop at 100 leaves the one at 150.
		tabbed(`<w:pStyle w:val="Tabbed"/>`+tabStops(tabStop("clear", 100, "")), "a", "b"),
	)
	p, _ := wordTestRender(t, body, wordTestParts{styles: styles})
	if got := p.startOf(1, "b", 0); !near(got, 120) {
		t.Errorf("inherited stop: %v", got)
	}
	if got := p.startOf(1, "c", 0); !near(got, 170) {
		t.Errorf("second inherited stop: %v", got)
	}
	if got := p.startOf(1, "b", 1); !near(got, 170) {
		t.Errorf("cleared stop: %v", got)
	}
}

func TestManualBreakRestartsTheLineForTabs(t *testing.T) {
	body := wordTestBody(wordTestPara(tabStops(tabStop("left", 100, "")),
		`<w:r><w:t>a</w:t><w:tab/><w:t>b</w:t><w:br/><w:t>cc</w:t><w:tab/><w:t>d</w:t></w:r>`))
	p, _ := wordTestRender(t, body)
	if len(p.lines(1)) != 2 {
		t.Fatalf("lines %+v", p.lines(1))
	}
	if b, d := p.startOf(1, "b", 0), p.startOf(1, "d", 0); !near(b, 120) || !near(d, 120) {
		t.Errorf("b at %v, d at %v", b, d)
	}
}

func TestTabLeaders(t *testing.T) {
	for _, tc := range []struct {
		leader string
		char   string
	}{{"dot", "."}, {"hyphen", "-"}, {"underscore", "_"}} {
		t.Run(tc.leader, func(t *testing.T) {
			body := wordTestBody(tabbed(tabStops(tabStop("right", 200, tc.leader)), "Title", "12"))
			p, _ := wordTestRender(t, body)
			count := 0
			for _, tx := range p.texts(1) {
				if strings.Trim(tx.text, tc.char) == "" && tx.text != "" {
					count += len(tx.text)
				}
			}
			// From the end of "Title" (30 px) to the start of "12" (188): 158 px
			// of 6 px characters.
			if count != 26 {
				t.Errorf("%d leader characters, want 26", count)
			}
			if got := p.startOf(1, "12", 0); !near(got, 20+200-12) {
				t.Errorf("right aligned text starts at %v", got)
			}
		})
	}
}

func TestWrappedTabParagraphIsApproximated(t *testing.T) {
	long := strings.Repeat("word ", 60)
	body := wordTestBody(tabbed(tabStops(tabStop("left", 100, "")), long, "end"))
	o := newWordTestOpts(t, false)
	if _, err := Prepare(context.Background(), wordTestDoc(t, body, wordTestParts{styles: wordTestStyles}), o.Options); !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("strict: %v", err)
	}
	o = newWordTestOpts(t, true)
	if _, err := Prepare(context.Background(), wordTestDoc(t, body, wordTestParts{styles: wordTestStyles}), o.Options); err != nil {
		t.Fatal(err)
	}
	if len(o.warnings) != 1 || !errors.Is(o.warnings[0], render.ErrApproximated) || !strings.Contains(o.warnings[0].Error(), "wraps") {
		t.Errorf("warnings %v", o.warnings)
	}
}

func TestTabsAcrossManualPageBreaksAndFonts(t *testing.T) {
	body := wordTestBody(wordTestPara(tabStops(tabStop("right", 200, "")),
		`<w:r><w:rPr><w:sz w:val="36"/></w:rPr><w:t>big</w:t><w:tab/><w:t>x</w:t></w:r>`,
		`<w:r><w:br w:type="page"/><w:t>y</w:t><w:tab/><w:t>z</w:t></w:r>`))
	p, _ := wordTestRender(t, body)
	// 18 pt is 24 px: "x" is 12 px wide, right aligned to 200.
	if got := p.startOf(1, "x", 0); !near(got, 20+200-12) {
		t.Errorf("x at %v", got)
	}
	if got := p.startOf(2, "z", 0); !near(got, 20+200-6) {
		t.Errorf("z at %v", got)
	}
}

func TestTabsLeaveTheLineHeightAlone(t *testing.T) {
	plain, _ := wordTestRender(t, wordTestBody(wordTestPara("", wordTestRun("", "a b")), wordTestPara("", wordTestRun("", "next"))))
	tab, _ := wordTestRender(t, wordTestBody(tabbed(tabStops(tabStop("left", 100, "dot")), "a", "b"), wordTestPara("", wordTestRun("", "next"))))
	if a, b := plain.lines(1)[1].y, tab.lines(1)[1].y; !near(a, b) {
		t.Errorf("next paragraph at %v with a tab, %v without", b, a)
	}
}
