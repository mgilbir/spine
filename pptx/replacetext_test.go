package pptx

import (
	"bytes"
	"testing"
	"unicode/utf8"

	"github.com/mgilbir/spine/common/dml"
)

// C86: replacements apply in a single deterministic pass — a value that
// contains another key is not re-replaced, and the result is order-independent.
func TestApplyReplacements_NoCascadeDeterministic(t *testing.T) {
	repl := map[string]string{"A": "B", "B": "C"}
	got, changed := applyReplacements("AB", repl)
	if !changed || got != "BC" {
		t.Errorf("applyReplacements(AB) = %q (changed=%v), want BC — cascade or nondeterminism", got, changed)
	}
	// Longest key wins at a position.
	if got, _ := applyReplacements("ab", map[string]string{"ab": "X", "a": "Y"}); got != "X" {
		t.Errorf("longest-match failed: got %q, want X", got)
	}
	// No match leaves the text unchanged.
	if got, changed := applyReplacements("plain", repl); changed || got != "plain" {
		t.Errorf("unexpected change: %q %v", got, changed)
	}
}

// C20: a shrinking replacement spanning two runs with a shared prefix/suffix
// character produces the correct result (previously "aa"->"a" stayed "aa").
func TestReplaceTextInParagraph_ShrinkAcrossRuns(t *testing.T) {
	p := &dml.P{R: []*dml.R{{T: "a"}, {T: "a"}}}
	if !replaceTextInParagraph(p, map[string]string{"aa": "a"}) {
		t.Fatal("expected a replacement to occur")
	}
	got := ""
	for _, r := range p.R {
		got += r.T
	}
	if got != "a" {
		t.Errorf("cross-run shrink = %q, want %q", got, "a")
	}
}

// C19: ReplaceText works on a deck built via the API (text authored in domain
// shapes), not only on decks loaded from a file.
func TestReplaceText_CreatedDeck(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	tb := s.AddTextBox()
	tb.SetText("Hello {{name}}")

	p.ReplaceText(map[string]string{"{{name}}": "World"})

	got := shapeText(t, s)
	if got != "Hello World" {
		t.Errorf("created-deck ReplaceText = %q, want %q", got, "Hello World")
	}
}

// C5: ReplaceText on a loaded deck must not drop a shape added via the API
// before the replacement.
func TestReplaceText_PreservesAddedShapeOnLoadedDeck(t *testing.T) {
	// Build and save a one-textbox deck.
	p := Create()
	s := p.AddSlide()
	tb := s.AddTextBox()
	tb.SetText("Hello {{x}}")
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}

	loaded, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	ls := loaded.Slides()[0]
	before := len(ls.Shapes())

	added := ls.AddTextBox()
	added.SetText("added")

	loaded.ReplaceText(map[string]string{"{{x}}": "Y"})

	if got := len(ls.Shapes()); got != before+1 {
		t.Errorf("added shape dropped by ReplaceText: shape count %d, want %d", got, before+1)
	}
}

// shapeText returns the concatenated text of the first textbox shape on a slide.
func shapeText(t *testing.T, s *Slide) string {
	t.Helper()
	for _, sh := range s.Shapes() {
		if tb, ok := sh.(*TextBox); ok {
			return tb.Text()
		}
	}
	t.Fatal("no textbox shape found")
	return ""
}

// C143: cross-run replacement splices must operate on rune boundaries. The
// byte-level prefix/suffix split used to cut multi-byte UTF-8 sequences in
// half, emitting <a:t> content that is invalid UTF-8 (malformed slide XML).
func TestReplaceTextInParagraph_MultiByteBoundaries(t *testing.T) {
	cases := []struct {
		name string
		runs []string
		repl map[string]string
		want string
	}{
		{"prefix keeps alpha", []string{"αα", "x"}, map[string]string{"αx": "γ"}, "αγ"},
		{"suffix keeps alpha", []string{"x", "αα"}, map[string]string{"xα": "γ"}, "γα"},
		{"prefix and suffix multi-byte", []string{"α", "βγ", "δ"}, map[string]string{"βγ": "Q"}, "αQδ"},
		{"multi-byte shrink across runs", []string{"α", "α"}, map[string]string{"αα": "α"}, "α"},
		{"ascii shrink stays fixed", []string{"a", "a"}, map[string]string{"aa": "a"}, "a"},
		{"grow across runs", []string{"αβ", "γ"}, map[string]string{"βγ": "ββγγ"}, "αββγγ"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runs := make([]*dml.R, len(tc.runs))
			for i, s := range tc.runs {
				runs[i] = &dml.R{T: s}
			}
			p := &dml.P{R: runs}
			if !replaceTextInParagraph(p, tc.repl) {
				t.Fatal("expected a replacement to occur")
			}
			got := ""
			for _, r := range p.R {
				if !utf8.ValidString(r.T) {
					t.Errorf("run %q is not valid UTF-8", r.T)
				}
				got += r.T
			}
			if got != tc.want {
				t.Errorf("result = %q, want %q", got, tc.want)
			}
		})
	}
}

// C143 end to end: the exact probe scenario. The saved slide XML must be
// valid UTF-8 and re-parse cleanly.
func TestReplaceText_MultiByteMultiRunSavesValidXML(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	tb := s.AddTextBox()
	para := tb.TextFrame().AddParagraph()
	para.AddRun().SetText("αα")
	para.AddRun().SetText("x")

	p.ReplaceText(map[string]string{"αx": "γ"})

	for _, r := range para.Runs() {
		if !utf8.ValidString(r.Text()) {
			t.Errorf("run %q is not valid UTF-8 after ReplaceText", r.Text())
		}
	}

	out, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	slideXML := zipPart(t, out, "ppt/slides/slide1.xml")
	if !utf8.Valid(slideXML) {
		t.Fatal("saved slide1.xml is not valid UTF-8")
	}
	// The result may be split across runs ("α" prefix run + "γ" middle run);
	// each <a:t> must hold complete runes.
	for _, want := range []string{"<a:t>α</a:t>", "<a:t>γ</a:t>"} {
		if !bytes.Contains(slideXML, []byte(want)) {
			t.Errorf("%q missing from slide XML", want)
		}
	}

	// The deck must reopen without a parse error.
	p2 := openBytes(t, out)
	if got := shapeText(t, p2.Slides()[0]); got != "αγ" {
		t.Errorf("reopened text = %q, want %q", got, "αγ")
	}
}
