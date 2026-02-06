package omml

import (
	"encoding/xml"
	"testing"
)

func TestRun_RoundTrip(t *testing.T) {
	tests := []struct {
		name string
		text string
	}{
		{"simple text", "x"},
		{"equation", "x + y"},
		{"symbols", "α + β"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Run{T: tt.text}
			out, err := xml.Marshal(r)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var r2 Run
			if err := xml.Unmarshal(out, &r2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if r2.T != tt.text {
				t.Errorf("T = %q, want %q", r2.T, tt.text)
			}
		})
	}
}

func TestRunPr_RoundTrip(t *testing.T) {
	tests := []struct {
		name string
		xml  string
	}{
		{
			name: "literal",
			xml:  `<rPr xmlns="http://schemas.openxmlformats.org/officeDocument/2006/math"><lit m:val="on" xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math"/></rPr>`,
		},
		{
			name: "script style",
			xml:  `<rPr xmlns="http://schemas.openxmlformats.org/officeDocument/2006/math"><scr m:val="script" xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math"/></rPr>`,
		},
		{
			name: "bold italic",
			xml:  `<rPr xmlns="http://schemas.openxmlformats.org/officeDocument/2006/math"><sty m:val="bi" xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math"/></rPr>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var rpr RunPr
			if err := xml.Unmarshal([]byte(tt.xml), &rpr); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			out, err := xml.Marshal(&rpr)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var rpr2 RunPr
			if err := xml.Unmarshal(out, &rpr2); err != nil {
				t.Fatalf("Re-unmarshal failed: %v", err)
			}
		})
	}
}

func TestFraction_RoundTrip(t *testing.T) {
	tests := []struct {
		name     string
		fracType string
	}{
		{"bar fraction", "bar"},
		{"skewed fraction", "skw"},
		{"linear fraction", "lin"},
		{"no bar", "noBar"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := &Fraction{
				FPr: &FractionPr{
					Type: &FType{Val: tt.fracType},
				},
			}
			out, err := xml.Marshal(f)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var f2 Fraction
			if err := xml.Unmarshal(out, &f2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if f2.FPr != nil && f2.FPr.Type != nil && f2.FPr.Type.Val != tt.fracType {
				t.Errorf("Type = %q, want %q", f2.FPr.Type.Val, tt.fracType)
			}
		})
	}
}

func TestRadical_RoundTrip(t *testing.T) {
	tests := []struct {
		name    string
		degHide bool
	}{
		{"with degree", false},
		{"square root", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Radical{
				RadPr: &RadPr{
					DegHide: &OnOff{Val: func() string {
						if tt.degHide {
							return "on"
						}
						return "off"
					}()},
				},
			}
			out, err := xml.Marshal(r)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var r2 Radical
			if err := xml.Unmarshal(out, &r2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}
		})
	}
}

func TestSubscript_RoundTrip(t *testing.T) {
	s := &Subscript{}
	out, err := xml.Marshal(s)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var s2 Subscript
	if err := xml.Unmarshal(out, &s2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
}

func TestSuperscript_RoundTrip(t *testing.T) {
	s := &Superscript{}
	out, err := xml.Marshal(s)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var s2 Superscript
	if err := xml.Unmarshal(out, &s2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
}

func TestSubSuperscript_RoundTrip(t *testing.T) {
	s := &SubSuperscript{
		SSubSupPr: &SSubSupPr{
			AlnScr: &OnOff{Val: "on"},
		},
	}
	out, err := xml.Marshal(s)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var s2 SubSuperscript
	if err := xml.Unmarshal(out, &s2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
}

func TestNAry_RoundTrip(t *testing.T) {
	tests := []struct {
		name   string
		chr    string
		limLoc string
	}{
		{"sum", "∑", "undOvr"},
		{"product", "∏", "subSup"},
		{"integral", "∫", "subSup"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n := &NAry{
				NaryPr: &NaryPr{
					Chr:    &Char{Val: tt.chr},
					LimLoc: &LimLoc{Val: tt.limLoc},
				},
			}
			out, err := xml.Marshal(n)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var n2 NAry
			if err := xml.Unmarshal(out, &n2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}
		})
	}
}

func TestMatrix_RoundTrip(t *testing.T) {
	m := &Matrix{
		MPr: &MatrixPr{
			BaseJc: &YAlign{Val: "center"},
		},
		MR: []*MatrixRow{
			{},
			{},
		},
	}
	out, err := xml.Marshal(m)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var m2 Matrix
	if err := xml.Unmarshal(out, &m2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if len(m2.MR) != 2 {
		t.Errorf("Expected 2 rows, got %d", len(m2.MR))
	}
}

func TestDelimiter_RoundTrip(t *testing.T) {
	tests := []struct {
		name   string
		begChr string
		endChr string
	}{
		{"parentheses", "(", ")"},
		{"brackets", "[", "]"},
		{"braces", "{", "}"},
		{"pipes", "|", "|"},
		{"double pipes", "‖", "‖"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := &Delimiter{
				DPr: &DelimiterPr{
					BegChr: &Char{Val: tt.begChr},
					EndChr: &Char{Val: tt.endChr},
				},
			}
			out, err := xml.Marshal(d)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var d2 Delimiter
			if err := xml.Unmarshal(out, &d2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}
		})
	}
}

func TestAccent_RoundTrip(t *testing.T) {
	tests := []struct {
		name string
		chr  string
	}{
		{"hat", "̂"},
		{"tilde", "̃"},
		{"dot", "̇"},
		{"double dot", "̈"},
		{"bar", "̄"},
		{"arrow", "→"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &Accent{
				AccPr: &AccentPr{
					Chr: &Char{Val: tt.chr},
				},
			}
			out, err := xml.Marshal(a)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var a2 Accent
			if err := xml.Unmarshal(out, &a2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}
		})
	}
}

func TestBar_RoundTrip(t *testing.T) {
	tests := []struct {
		name string
		pos  string
	}{
		{"overbar", "top"},
		{"underbar", "bot"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := &Bar{
				BarPr: &BarPr{
					Pos: &TopBot{Val: tt.pos},
				},
			}
			out, err := xml.Marshal(b)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var b2 Bar
			if err := xml.Unmarshal(out, &b2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}
		})
	}
}

func TestGroupChar_RoundTrip(t *testing.T) {
	tests := []struct {
		name string
		chr  string
		pos  string
	}{
		{"underbrace", "⏟", "bot"},
		{"overbrace", "⏞", "top"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gc := &GroupChar{
				GroupChrPr: &GroupChrPr{
					Chr: &Char{Val: tt.chr},
					Pos: &TopBot{Val: tt.pos},
				},
			}
			out, err := xml.Marshal(gc)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var gc2 GroupChar
			if err := xml.Unmarshal(out, &gc2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}
		})
	}
}

func TestLimitLow_RoundTrip(t *testing.T) {
	ll := &LimitLow{}
	out, err := xml.Marshal(ll)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var ll2 LimitLow
	if err := xml.Unmarshal(out, &ll2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
}

func TestLimitUpper_RoundTrip(t *testing.T) {
	lu := &LimitUpper{}
	out, err := xml.Marshal(lu)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var lu2 LimitUpper
	if err := xml.Unmarshal(out, &lu2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
}

func TestFunction_RoundTrip(t *testing.T) {
	f := &Function{}
	out, err := xml.Marshal(f)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var f2 Function
	if err := xml.Unmarshal(out, &f2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
}

func TestEquationArray_RoundTrip(t *testing.T) {
	ea := &EquationArray{
		EqArrPr: &EqArrPr{
			BaseJc: &YAlign{Val: "center"},
		},
	}
	out, err := xml.Marshal(ea)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var ea2 EquationArray
	if err := xml.Unmarshal(out, &ea2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
}

func TestBox_RoundTrip(t *testing.T) {
	b := &Box{
		BoxPr: &BoxPr{
			OpEmu:   &OnOff{Val: "on"},
			NoBreak: &OnOff{Val: "on"},
		},
	}
	out, err := xml.Marshal(b)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var b2 Box
	if err := xml.Unmarshal(out, &b2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
}

func TestBorderBox_RoundTrip(t *testing.T) {
	bb := &BorderBox{
		BorderBoxPr: &BorderBoxPr{
			HideTop:  &OnOff{Val: "off"},
			HideBot:  &OnOff{Val: "off"},
			HideLeft: &OnOff{Val: "off"},
			HideRight: &OnOff{Val: "off"},
		},
	}
	out, err := xml.Marshal(bb)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var bb2 BorderBox
	if err := xml.Unmarshal(out, &bb2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
}

func TestPhantom_RoundTrip(t *testing.T) {
	p := &Phantom{
		PhantPr: &PhantPr{
			Show:    &OnOff{Val: "off"},
			ZeroWid: &OnOff{Val: "on"},
		},
	}
	out, err := xml.Marshal(p)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var p2 Phantom
	if err := xml.Unmarshal(out, &p2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
}

func TestOnOff_RoundTrip(t *testing.T) {
	tests := []struct {
		val string
	}{
		{"on"},
		{"off"},
		{"1"},
		{"0"},
		{"true"},
		{"false"},
	}

	for _, tt := range tests {
		t.Run(tt.val, func(t *testing.T) {
			oo := &OnOff{Val: tt.val}
			out, err := xml.Marshal(oo)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var oo2 OnOff
			if err := xml.Unmarshal(out, &oo2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if oo2.Val != tt.val {
				t.Errorf("Val = %q, want %q", oo2.Val, tt.val)
			}
		})
	}
}

func TestScript_RoundTrip(t *testing.T) {
	scripts := []string{"roman", "script", "fraktur", "double-struck", "sans-serif", "monospace"}
	for _, scr := range scripts {
		t.Run(scr, func(t *testing.T) {
			s := &Script{Val: scr}
			out, err := xml.Marshal(s)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var s2 Script
			if err := xml.Unmarshal(out, &s2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if s2.Val != scr {
				t.Errorf("Val = %q, want %q", s2.Val, scr)
			}
		})
	}
}

func TestStyle_RoundTrip(t *testing.T) {
	styles := []string{"p", "b", "i", "bi"}
	for _, sty := range styles {
		t.Run(sty, func(t *testing.T) {
			s := &Style{Val: sty}
			out, err := xml.Marshal(s)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var s2 Style
			if err := xml.Unmarshal(out, &s2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if s2.Val != sty {
				t.Errorf("Val = %q, want %q", s2.Val, sty)
			}
		})
	}
}

func TestMathPr_RoundTrip(t *testing.T) {
	mp := &MathPr{
		MathFont: &MathFont{Val: "Cambria Math"},
		BrkBin:   &BreakBin{Val: "before"},
		SmallFrac: &OnOff{Val: "off"},
		DefJc:    &MathJc{Val: "centerGroup"},
	}
	out, err := xml.Marshal(mp)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var mp2 MathPr
	if err := xml.Unmarshal(out, &mp2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
}

func TestOMathPara_RoundTrip(t *testing.T) {
	omp := &OMathPara{
		OMathParaPr: &OMathParaPr{
			Jc: &MathJc{Val: "center"},
		},
	}
	out, err := xml.Marshal(omp)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var omp2 OMathPara
	if err := xml.Unmarshal(out, &omp2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
}
