package docx

import (
	"strconv"
	"strings"
	"testing"
)

// The run's font, size, colour and strike setters were reachable but never
// driven end to end. A getter that reads the wrong field of w:rPr, or a setter
// that writes the value in the wrong unit, produces a document that is silently
// wrong: everything still round-trips through the struct, so only the written
// part and a reopen can tell.
//
// Every value below is deliberately distinct in *kind* as well as in content —
// a font name, a hex colour, a fractional point size — so a getter that
// returned a neighbouring field of w:rPr cannot accidentally match.

const (
	wantFontName  = "Cambria Math"
	wantFontColor = "C00FFE"
	// Fractional on purpose: w:sz stores half-points, so 13.5pt must be
	// written as "27". A setter that stored points would write "13" or "14"
	// and a getter that skipped the halving would report 27.
	wantFontSizePt   = 13.5
	wantFontSizeHalf = "27"
)

// runFormatProp is one get/set pair on Run, plus the markup the setter must
// produce.
type runFormatProp struct {
	name string
	set  func(r *Run)
	// got renders the getter's value as a string so one table can hold
	// properties of different types.
	got func(r *Run) string
	// want is the getter's expected rendering, after authoring and again after
	// a save/reopen.
	want string
	// wantXML are substrings that must all appear in word/document.xml.
	wantXML []string
}

func runFormatProps() []runFormatProp {
	return []runFormatProp{
		{
			name: "Font",
			set:  func(r *Run) { r.SetFont(wantFontName) },
			got:  func(r *Run) string { return r.Font() },
			want: wantFontName,
			// Word applies w:ascii to Latin text and w:hAnsi to the high-ANSI
			// range; setting only one leaves half the document on the theme
			// font, which is the bug SetFont writing a single attribute would
			// produce.
			wantXML: []string{`w:ascii="Cambria Math"`, `w:hAnsi="Cambria Math"`},
		},
		{
			name:    "Color",
			set:     func(r *Run) { r.SetColor(wantFontColor) },
			got:     func(r *Run) string { return r.Color() },
			want:    wantFontColor,
			wantXML: []string{`<w:color w:val="C00FFE"/>`},
		},
		{
			name:    "FontSize",
			set:     func(r *Run) { r.SetFontSize(wantFontSizePt) },
			got:     func(r *Run) string { return formatPt(r.FontSize()) },
			want:    formatPt(wantFontSizePt),
			wantXML: []string{`<w:sz w:val="` + wantFontSizeHalf + `"/>`},
		},
		{
			name:    "StrikeOn",
			set:     func(r *Run) { r.SetStrike(true) },
			got:     func(r *Run) string { return boolStr(r.Strike()) },
			want:    "true",
			wantXML: []string{`<w:strike/>`},
		},
	}
}

func formatPt(v float64) string {
	return strconv.FormatFloat(v, 'g', -1, 64)
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// TestRunFormatting_RoundTrip applies every run-format setter to one run with
// distinct values, checks the getter, checks the written markup, then reopens
// the saved file and checks the getters again. The reopen is what proves the
// value reached the part rather than only the struct.
func TestRunFormatting_RoundTrip(t *testing.T) {
	props := runFormatProps()

	doc := Create()
	run := doc.AddParagraph().AddRun()
	run.SetText("formatted")
	for _, p := range props {
		p.set(run)
	}

	for _, p := range props {
		if got := p.got(run); got != p.want {
			t.Errorf("authored: %s = %q, want %q", p.name, got, p.want)
		}
	}

	data, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	parts, _ := docParts(t, data)
	main := parts["word/document.xml"]
	for _, p := range props {
		for _, frag := range p.wantXML {
			if !strings.Contains(main, frag) {
				t.Errorf("%s: word/document.xml does not contain %s\ngot: %s", p.name, frag, main)
			}
		}
	}

	reopened := saveAndReopen(t, doc)
	rr := firstRun(t, reopened)
	if got := rr.Text(); got != "formatted" {
		t.Fatalf("reopened run text = %q, want %q", got, "formatted")
	}
	for _, p := range props {
		if got := p.got(rr); got != p.want {
			t.Errorf("reopened: %s = %q, want %q", p.name, got, p.want)
		}
	}
}

// TestRunFormatting_Unset pins the zero values, so a getter that returns a
// wrong-but-nonzero default (or panics on a run with no w:rPr) fails.
func TestRunFormatting_Unset(t *testing.T) {
	doc := Create()
	run := doc.AddParagraph().AddRun()
	run.SetText("plain")
	if got := run.Font(); got != "" {
		t.Errorf("Font() on an unformatted run = %q, want \"\"", got)
	}
	if got := run.Color(); got != "" {
		t.Errorf("Color() on an unformatted run = %q, want \"\"", got)
	}
	if got := run.FontSize(); got != 0 {
		t.Errorf("FontSize() on an unformatted run = %v, want 0", got)
	}
	if run.Strike() {
		t.Error("Strike() on an unformatted run = true, want false")
	}

	// A run with a w:rPr but none of these properties must still report the
	// zero values — the second nil check in each getter.
	run.SetBold(true)
	if got := run.Font(); got != "" {
		t.Errorf("Font() with an unrelated rPr = %q, want \"\"", got)
	}
	if got := run.Color(); got != "" {
		t.Errorf("Color() with an unrelated rPr = %q, want \"\"", got)
	}
	if got := run.FontSize(); got != 0 {
		t.Errorf("FontSize() with an unrelated rPr = %v, want 0", got)
	}
	if run.Strike() {
		t.Error("Strike() with an unrelated rPr = true, want false")
	}
}

// TestRunFontSize_HalfPoints drives the half-point conversion across sizes,
// including one that only halves cleanly and one whose half-point value is odd.
// A setter that wrote points, or a getter that forgot to halve, disagrees on
// every row.
func TestRunFontSize_HalfPoints(t *testing.T) {
	cases := []struct {
		points float64
		val    string
	}{
		{8, "16"},
		{10.5, "21"},
		{11, "22"},
		{13.5, "27"},
		{72, "144"},
	}
	for _, c := range cases {
		doc := Create()
		run := doc.AddParagraph().AddRun()
		run.SetText("x")
		run.SetFontSize(c.points)

		if got := run.FontSize(); got != c.points {
			t.Errorf("SetFontSize(%v) then FontSize() = %v, want %v", c.points, got, c.points)
		}
		data, err := doc.SaveBytes()
		if err != nil {
			t.Fatalf("SaveBytes: %v", err)
		}
		parts, _ := docParts(t, data)
		want := `<w:sz w:val="` + c.val + `"/>`
		if !strings.Contains(parts["word/document.xml"], want) {
			t.Errorf("SetFontSize(%v) wrote no %s (w:sz is in half-points)", c.points, want)
		}
		if got := firstRun(t, saveAndReopen(t, doc)).FontSize(); got != c.points {
			t.Errorf("SetFontSize(%v) reopened as %v", c.points, got)
		}
	}
}

// TestRunStrike_TriState mirrors the bold tri-state contract: an explicit off
// has to survive as an element with w:val="false", because an absent w:strike
// would let a struck-through character style through.
func TestRunStrike_TriState(t *testing.T) {
	doc := Create()
	run := doc.AddParagraph().AddRun()
	run.SetText("x")

	run.SetStrike(false)
	if run.Strike() {
		t.Error("Strike() after SetStrike(false) = true")
	}
	data, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	parts, _ := docParts(t, data)
	if !strings.Contains(parts["word/document.xml"], `<w:strike w:val="false"/>`) {
		t.Errorf("SetStrike(false) must write an explicit off so an inherited strike is cancelled; got: %s",
			parts["word/document.xml"])
	}
	if firstRun(t, saveAndReopen(t, doc)).Strike() {
		t.Error("an explicit off reopened as Strike() = true")
	}

	run.SetStrike(true)
	if !run.Strike() {
		t.Error("Strike() after SetStrike(true) = false")
	}
	if !firstRun(t, saveAndReopen(t, doc)).Strike() {
		t.Error("SetStrike(true) did not survive a save/reopen")
	}
}
