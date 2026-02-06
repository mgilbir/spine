package oxml

import (
	"encoding/xml"
	"testing"
)

func TestTransition_RoundTrip(t *testing.T) {
	tests := []struct {
		name string
		xml  string
	}{
		{
			name: "basic transition with fade",
			xml: `<transition xmlns="http://schemas.openxmlformats.org/presentationml/2006/main" spd="slow" advClick="true" advTm="3000">
  <fade thruBlk="true"/>
</transition>`,
		},
		{
			name: "blinds transition",
			xml: `<transition xmlns="http://schemas.openxmlformats.org/presentationml/2006/main" spd="med">
  <blinds dir="vert"/>
</transition>`,
		},
		{
			name: "checker transition",
			xml: `<transition xmlns="http://schemas.openxmlformats.org/presentationml/2006/main" spd="fast">
  <checker dir="horz"/>
</transition>`,
		},
		{
			name: "circle transition",
			xml: `<transition xmlns="http://schemas.openxmlformats.org/presentationml/2006/main">
  <circle/>
</transition>`,
		},
		{
			name: "dissolve transition",
			xml: `<transition xmlns="http://schemas.openxmlformats.org/presentationml/2006/main">
  <dissolve/>
</transition>`,
		},
		{
			name: "comb transition",
			xml: `<transition xmlns="http://schemas.openxmlformats.org/presentationml/2006/main">
  <comb dir="vert"/>
</transition>`,
		},
		{
			name: "cover transition with direction",
			xml: `<transition xmlns="http://schemas.openxmlformats.org/presentationml/2006/main">
  <cover dir="lu"/>
</transition>`,
		},
		{
			name: "cut transition",
			xml: `<transition xmlns="http://schemas.openxmlformats.org/presentationml/2006/main">
  <cut thruBlk="false"/>
</transition>`,
		},
		{
			name: "diamond transition",
			xml: `<transition xmlns="http://schemas.openxmlformats.org/presentationml/2006/main">
  <diamond/>
</transition>`,
		},
		{
			name: "newsflash transition",
			xml: `<transition xmlns="http://schemas.openxmlformats.org/presentationml/2006/main">
  <newsflash/>
</transition>`,
		},
		{
			name: "plus transition",
			xml: `<transition xmlns="http://schemas.openxmlformats.org/presentationml/2006/main">
  <plus/>
</transition>`,
		},
		{
			name: "pull transition",
			xml: `<transition xmlns="http://schemas.openxmlformats.org/presentationml/2006/main">
  <pull dir="rd"/>
</transition>`,
		},
		{
			name: "push transition",
			xml: `<transition xmlns="http://schemas.openxmlformats.org/presentationml/2006/main">
  <push dir="r"/>
</transition>`,
		},
		{
			name: "random transition",
			xml: `<transition xmlns="http://schemas.openxmlformats.org/presentationml/2006/main">
  <random/>
</transition>`,
		},
		{
			name: "randomBar transition",
			xml: `<transition xmlns="http://schemas.openxmlformats.org/presentationml/2006/main">
  <randomBar dir="horz"/>
</transition>`,
		},
		{
			name: "split transition",
			xml: `<transition xmlns="http://schemas.openxmlformats.org/presentationml/2006/main">
  <split orient="vert" dir="in"/>
</transition>`,
		},
		{
			name: "strips transition",
			xml: `<transition xmlns="http://schemas.openxmlformats.org/presentationml/2006/main">
  <strips dir="ru"/>
</transition>`,
		},
		{
			name: "wedge transition",
			xml: `<transition xmlns="http://schemas.openxmlformats.org/presentationml/2006/main">
  <wedge/>
</transition>`,
		},
		{
			name: "wheel transition with spokes",
			xml: `<transition xmlns="http://schemas.openxmlformats.org/presentationml/2006/main">
  <wheel spokes="8"/>
</transition>`,
		},
		{
			name: "wipe transition",
			xml: `<transition xmlns="http://schemas.openxmlformats.org/presentationml/2006/main">
  <wipe dir="d"/>
</transition>`,
		},
		{
			name: "zoom transition",
			xml: `<transition xmlns="http://schemas.openxmlformats.org/presentationml/2006/main">
  <zoom dir="in"/>
</transition>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var trans Transition
			if err := xml.Unmarshal([]byte(tt.xml), &trans); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			out, err := xml.Marshal(&trans)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var trans2 Transition
			if err := xml.Unmarshal(out, &trans2); err != nil {
				t.Fatalf("Re-unmarshal failed: %v", err)
			}
		})
	}
}

func TestTransitionSoundAction_RoundTrip(t *testing.T) {
	tests := []struct {
		name string
		xml  string
	}{
		{
			name: "start sound",
			xml: `<sndAc xmlns="http://schemas.openxmlformats.org/presentationml/2006/main">
  <stSnd loop="true"/>
</sndAc>`,
		},
		{
			name: "end sound",
			xml: `<sndAc xmlns="http://schemas.openxmlformats.org/presentationml/2006/main">
  <endSnd/>
</sndAc>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var sndAc TransitionSoundAction
			if err := xml.Unmarshal([]byte(tt.xml), &sndAc); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			out, err := xml.Marshal(&sndAc)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var sndAc2 TransitionSoundAction
			if err := xml.Unmarshal(out, &sndAc2); err != nil {
				t.Fatalf("Re-unmarshal failed: %v", err)
			}
		})
	}
}

func TestOrientationTransition_RoundTrip(t *testing.T) {
	tests := []struct {
		name string
		dir  string
	}{
		{"horizontal", "horz"},
		{"vertical", "vert"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ot := &OrientationTransition{Dir: tt.dir}
			out, err := xml.Marshal(ot)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var ot2 OrientationTransition
			if err := xml.Unmarshal(out, &ot2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if ot2.Dir != tt.dir {
				t.Errorf("Dir = %q, want %q", ot2.Dir, tt.dir)
			}
		})
	}
}

func TestSideDirectionTransition_RoundTrip(t *testing.T) {
	directions := []string{"l", "u", "r", "d"}
	for _, dir := range directions {
		t.Run(dir, func(t *testing.T) {
			sdt := &SideDirectionTransition{Dir: dir}
			out, err := xml.Marshal(sdt)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var sdt2 SideDirectionTransition
			if err := xml.Unmarshal(out, &sdt2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if sdt2.Dir != dir {
				t.Errorf("Dir = %q, want %q", sdt2.Dir, dir)
			}
		})
	}
}

func TestCornerDirectionTransition_RoundTrip(t *testing.T) {
	directions := []string{"lu", "ru", "ld", "rd"}
	for _, dir := range directions {
		t.Run(dir, func(t *testing.T) {
			cdt := &CornerDirectionTransition{Dir: dir}
			out, err := xml.Marshal(cdt)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var cdt2 CornerDirectionTransition
			if err := xml.Unmarshal(out, &cdt2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if cdt2.Dir != dir {
				t.Errorf("Dir = %q, want %q", cdt2.Dir, dir)
			}
		})
	}
}

func TestEightDirectionTransition_RoundTrip(t *testing.T) {
	directions := []string{"l", "u", "r", "d", "lu", "ru", "ld", "rd"}
	for _, dir := range directions {
		t.Run(dir, func(t *testing.T) {
			edt := &EightDirectionTransition{Dir: dir}
			out, err := xml.Marshal(edt)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var edt2 EightDirectionTransition
			if err := xml.Unmarshal(out, &edt2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if edt2.Dir != dir {
				t.Errorf("Dir = %q, want %q", edt2.Dir, dir)
			}
		})
	}
}

func TestSplitTransition_RoundTrip(t *testing.T) {
	tests := []struct {
		orient string
		dir    string
	}{
		{"horz", "in"},
		{"horz", "out"},
		{"vert", "in"},
		{"vert", "out"},
	}

	for _, tt := range tests {
		t.Run(tt.orient+"_"+tt.dir, func(t *testing.T) {
			st := &SplitTransition{Orient: tt.orient, Dir: tt.dir}
			out, err := xml.Marshal(st)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var st2 SplitTransition
			if err := xml.Unmarshal(out, &st2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if st2.Orient != tt.orient {
				t.Errorf("Orient = %q, want %q", st2.Orient, tt.orient)
			}
			if st2.Dir != tt.dir {
				t.Errorf("Dir = %q, want %q", st2.Dir, tt.dir)
			}
		})
	}
}

func TestWheelTransition_RoundTrip(t *testing.T) {
	spokes := []uint32{1, 2, 3, 4, 8}
	for _, s := range spokes {
		t.Run("spokes_"+string(rune('0'+s)), func(t *testing.T) {
			wt := &WheelTransition{Spokes: s}
			out, err := xml.Marshal(wt)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var wt2 WheelTransition
			if err := xml.Unmarshal(out, &wt2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if wt2.Spokes != s {
				t.Errorf("Spokes = %d, want %d", wt2.Spokes, s)
			}
		})
	}
}

func TestInOutTransition_RoundTrip(t *testing.T) {
	directions := []string{"in", "out"}
	for _, dir := range directions {
		t.Run(dir, func(t *testing.T) {
			iot := &InOutTransition{Dir: dir}
			out, err := xml.Marshal(iot)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var iot2 InOutTransition
			if err := xml.Unmarshal(out, &iot2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if iot2.Dir != dir {
				t.Errorf("Dir = %q, want %q", iot2.Dir, dir)
			}
		})
	}
}

func TestOptionalBlackTransition_RoundTrip(t *testing.T) {
	tests := []struct {
		name    string
		thruBlk bool
	}{
		{"thru_black_true", true},
		{"thru_black_false", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			obt := &OptionalBlackTransition{ThruBlk: tt.thruBlk}
			out, err := xml.Marshal(obt)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var obt2 OptionalBlackTransition
			if err := xml.Unmarshal(out, &obt2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if obt2.ThruBlk != tt.thruBlk {
				t.Errorf("ThruBlk = %v, want %v", obt2.ThruBlk, tt.thruBlk)
			}
		})
	}
}

// Office 2010+ transition tests
func TestTransitionPrism_RoundTrip(t *testing.T) {
	tests := []struct {
		dir        string
		isContent  bool
		isInverted bool
	}{
		{"l", true, false},
		{"r", false, true},
		{"u", true, true},
		{"d", false, false},
	}

	for _, tt := range tests {
		t.Run(tt.dir, func(t *testing.T) {
			tp := &TransitionPrism{Dir: tt.dir, IsContent: tt.isContent, IsInverted: tt.isInverted}
			out, err := xml.Marshal(tp)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var tp2 TransitionPrism
			if err := xml.Unmarshal(out, &tp2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if tp2.Dir != tt.dir {
				t.Errorf("Dir = %q, want %q", tp2.Dir, tt.dir)
			}
		})
	}
}

func TestTransitionRipple_RoundTrip(t *testing.T) {
	directions := []string{"center", "l", "r", "u", "d"}
	for _, dir := range directions {
		t.Run(dir, func(t *testing.T) {
			tr := &TransitionRipple{Dir: dir}
			out, err := xml.Marshal(tr)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var tr2 TransitionRipple
			if err := xml.Unmarshal(out, &tr2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if tr2.Dir != dir {
				t.Errorf("Dir = %q, want %q", tr2.Dir, dir)
			}
		})
	}
}

func TestTransitionShred_RoundTrip(t *testing.T) {
	tests := []struct {
		pattern string
		dir     string
	}{
		{"strip", "in"},
		{"strip", "out"},
		{"rectangle", "in"},
		{"rectangle", "out"},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.dir, func(t *testing.T) {
			ts := &TransitionShred{Pattern: tt.pattern, Dir: tt.dir}
			out, err := xml.Marshal(ts)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var ts2 TransitionShred
			if err := xml.Unmarshal(out, &ts2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if ts2.Pattern != tt.pattern {
				t.Errorf("Pattern = %q, want %q", ts2.Pattern, tt.pattern)
			}
			if ts2.Dir != tt.dir {
				t.Errorf("Dir = %q, want %q", ts2.Dir, tt.dir)
			}
		})
	}
}

func TestTransitionGlitter_RoundTrip(t *testing.T) {
	tests := []struct {
		dir     string
		pattern string
	}{
		{"l", "diamond"},
		{"r", "hexagon"},
		{"u", "diamond"},
		{"d", "hexagon"},
	}

	for _, tt := range tests {
		t.Run(tt.dir+"_"+tt.pattern, func(t *testing.T) {
			tg := &TransitionGlitter{Dir: tt.dir, Pattern: tt.pattern}
			out, err := xml.Marshal(tg)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var tg2 TransitionGlitter
			if err := xml.Unmarshal(out, &tg2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if tg2.Dir != tt.dir {
				t.Errorf("Dir = %q, want %q", tg2.Dir, tt.dir)
			}
			if tg2.Pattern != tt.pattern {
				t.Errorf("Pattern = %q, want %q", tg2.Pattern, tt.pattern)
			}
		})
	}
}

func TestTransitionFlythrough_RoundTrip(t *testing.T) {
	tests := []struct {
		dir       string
		hasBounce bool
	}{
		{"in", true},
		{"in", false},
		{"out", true},
		{"out", false},
	}

	for _, tt := range tests {
		name := tt.dir
		if tt.hasBounce {
			name += "_bounce"
		}
		t.Run(name, func(t *testing.T) {
			tf := &TransitionFlythrough{Dir: tt.dir, HasBounce: tt.hasBounce}
			out, err := xml.Marshal(tf)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var tf2 TransitionFlythrough
			if err := xml.Unmarshal(out, &tf2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if tf2.Dir != tt.dir {
				t.Errorf("Dir = %q, want %q", tf2.Dir, tt.dir)
			}
			if tf2.HasBounce != tt.hasBounce {
				t.Errorf("HasBounce = %v, want %v", tf2.HasBounce, tt.hasBounce)
			}
		})
	}
}

func TestTransitionReveal_RoundTrip(t *testing.T) {
	tests := []struct {
		thruBlk bool
		dir     string
	}{
		{true, "l"},
		{false, "r"},
	}

	for _, tt := range tests {
		t.Run(tt.dir, func(t *testing.T) {
			tr := &TransitionReveal{ThruBlk: tt.thruBlk, Dir: tt.dir}
			out, err := xml.Marshal(tr)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var tr2 TransitionReveal
			if err := xml.Unmarshal(out, &tr2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if tr2.ThruBlk != tt.thruBlk {
				t.Errorf("ThruBlk = %v, want %v", tr2.ThruBlk, tt.thruBlk)
			}
			if tr2.Dir != tt.dir {
				t.Errorf("Dir = %q, want %q", tr2.Dir, tt.dir)
			}
		})
	}
}
