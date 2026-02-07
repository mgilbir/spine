package oxml

import (
	"encoding/xml"
	"testing"
)

func TestTiming_RoundTrip(t *testing.T) {
	xmlStr := `<timing xmlns="http://schemas.openxmlformats.org/presentationml/2006/main">
  <tnLst>
    <par>
      <cTn id="1" dur="indefinite" nodeType="tmRoot"/>
    </par>
  </tnLst>
</timing>`

	var timing Timing
	if err := xml.Unmarshal([]byte(xmlStr), &timing); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	out, err := xml.Marshal(&timing)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var timing2 Timing
	if err := xml.Unmarshal(out, &timing2); err != nil {
		t.Fatalf("Re-unmarshal failed: %v", err)
	}
}

func TestTimeNodeList_RoundTrip(t *testing.T) {
	tests := []struct {
		name string
		xml  string
	}{
		{
			name: "parallel nodes",
			xml: `<tnLst xmlns="http://schemas.openxmlformats.org/presentationml/2006/main">
  <par><cTn id="1"/></par>
  <par><cTn id="2"/></par>
</tnLst>`,
		},
		{
			name: "sequence nodes",
			xml: `<tnLst xmlns="http://schemas.openxmlformats.org/presentationml/2006/main">
  <seq concurrent="true" prevAc="skipTimed" nextAc="seek">
    <cTn id="1"/>
  </seq>
</tnLst>`,
		},
		{
			name: "animate",
			xml: `<tnLst xmlns="http://schemas.openxmlformats.org/presentationml/2006/main">
  <anim from="0" to="100" calcmode="lin" valueType="num"/>
</tnLst>`,
		},
		{
			name: "set",
			xml: `<tnLst xmlns="http://schemas.openxmlformats.org/presentationml/2006/main">
  <set/>
</tnLst>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var tnl TimeNodeList
			if err := xml.Unmarshal([]byte(tt.xml), &tnl); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			out, err := xml.Marshal(&tnl)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var tnl2 TimeNodeList
			if err := xml.Unmarshal(out, &tnl2); err != nil {
				t.Fatalf("Re-unmarshal failed: %v", err)
			}
		})
	}
}

func TestParallelTimeNode_RoundTrip(t *testing.T) {
	xmlStr := `<par xmlns="http://schemas.openxmlformats.org/presentationml/2006/main">
  <cTn id="1" presetID="10" presetClass="entr" dur="500" fill="hold"/>
</par>`

	var ptn ParallelTimeNode
	if err := xml.Unmarshal([]byte(xmlStr), &ptn); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	out, err := xml.Marshal(&ptn)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var ptn2 ParallelTimeNode
	if err := xml.Unmarshal(out, &ptn2); err != nil {
		t.Fatalf("Re-unmarshal failed: %v", err)
	}
}

func TestSequenceTimeNode_RoundTrip(t *testing.T) {
	tests := []struct {
		name       string
		concurrent bool
		prevAc     string
		nextAc     string
	}{
		{"basic", false, "", ""},
		{"concurrent", true, "", ""},
		{"with actions", false, "skipTimed", "seek"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stn := &SequenceTimeNode{
				Concurrent: tt.concurrent,
				PrevAc:     tt.prevAc,
				NextAc:     tt.nextAc,
			}
			out, err := xml.Marshal(stn)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var stn2 SequenceTimeNode
			if err := xml.Unmarshal(out, &stn2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if stn2.Concurrent != tt.concurrent {
				t.Errorf("Concurrent = %v, want %v", stn2.Concurrent, tt.concurrent)
			}
		})
	}
}

func TestCommonTimeNode_RoundTrip(t *testing.T) {
	tests := []struct {
		name string
		xml  string
	}{
		{
			name: "basic time node",
			xml:  `<cTn xmlns="http://schemas.openxmlformats.org/presentationml/2006/main" id="1" dur="indefinite" nodeType="tmRoot"/>`,
		},
		{
			name: "entrance preset",
			xml:  `<cTn xmlns="http://schemas.openxmlformats.org/presentationml/2006/main" id="2" presetID="10" presetClass="entr" presetSubtype="0" dur="500" fill="hold"/>`,
		},
		{
			name: "with timing attributes",
			xml:  `<cTn xmlns="http://schemas.openxmlformats.org/presentationml/2006/main" id="3" dur="1000" repeatCount="3" spd="50000" accel="50000" decel="50000"/>`,
		},
		{
			name: "exit animation",
			xml:  `<cTn xmlns="http://schemas.openxmlformats.org/presentationml/2006/main" id="4" presetID="1" presetClass="exit" restart="whenNotActive"/>`,
		},
		{
			name: "emphasis",
			xml:  `<cTn xmlns="http://schemas.openxmlformats.org/presentationml/2006/main" id="5" presetClass="emph" autoRev="true"/>`,
		},
		{
			name: "motion path",
			xml:  `<cTn xmlns="http://schemas.openxmlformats.org/presentationml/2006/main" id="6" presetClass="path"/>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ctn CommonTimeNode
			if err := xml.Unmarshal([]byte(tt.xml), &ctn); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			out, err := xml.Marshal(&ctn)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var ctn2 CommonTimeNode
			if err := xml.Unmarshal(out, &ctn2); err != nil {
				t.Fatalf("Re-unmarshal failed: %v", err)
			}
		})
	}
}

func TestConditionList_RoundTrip(t *testing.T) {
	xmlStr := `<stCondLst xmlns="http://schemas.openxmlformats.org/presentationml/2006/main">
  <cond evt="onBegin" delay="0"/>
  <cond evt="onClick" delay="indefinite"/>
</stCondLst>`

	var cl ConditionList
	if err := xml.Unmarshal([]byte(xmlStr), &cl); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if len(cl.Cond) != 2 {
		t.Errorf("Expected 2 conditions, got %d", len(cl.Cond))
	}

	out, err := xml.Marshal(&cl)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var cl2 ConditionList
	if err := xml.Unmarshal(out, &cl2); err != nil {
		t.Fatalf("Re-unmarshal failed: %v", err)
	}
}

func TestCondition_RoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		evt   string
		delay string
	}{
		{"on begin", "onBegin", "0"},
		{"on click", "onClick", "indefinite"},
		{"on end", "onEnd", "500"},
		{"on double click", "onDblClick", "0"},
		{"on mouse over", "onMouseOver", "0"},
		{"on mouse out", "onMouseOut", "0"},
		{"on next", "onNext", "0"},
		{"on prev", "onPrev", "0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cond := &Condition{Evt: tt.evt, Delay: tt.delay}
			out, err := xml.Marshal(cond)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var cond2 Condition
			if err := xml.Unmarshal(out, &cond2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if cond2.Evt != tt.evt {
				t.Errorf("Evt = %q, want %q", cond2.Evt, tt.evt)
			}
			if cond2.Delay != tt.delay {
				t.Errorf("Delay = %q, want %q", cond2.Delay, tt.delay)
			}
		})
	}
}

func TestTargetElement_RoundTrip(t *testing.T) {
	tests := []struct {
		name string
		xml  string
	}{
		{
			name: "slide target",
			xml:  `<tgtEl xmlns="http://schemas.openxmlformats.org/presentationml/2006/main"><sldTgt/></tgtEl>`,
		},
		{
			name: "shape target",
			xml:  `<tgtEl xmlns="http://schemas.openxmlformats.org/presentationml/2006/main"><spTgt spid="5"/></tgtEl>`,
		},
		{
			name: "ink target",
			xml:  `<tgtEl xmlns="http://schemas.openxmlformats.org/presentationml/2006/main"><inkTgt spid="3"/></tgtEl>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var te TargetElement
			if err := xml.Unmarshal([]byte(tt.xml), &te); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			out, err := xml.Marshal(&te)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var te2 TargetElement
			if err := xml.Unmarshal(out, &te2); err != nil {
				t.Fatalf("Re-unmarshal failed: %v", err)
			}
		})
	}
}

func TestShapeTarget_RoundTrip(t *testing.T) {
	tests := []struct {
		name string
		spId string
	}{
		{"shape 1", "1"},
		{"shape 5", "5"},
		{"shape 100", "100"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			st := &ShapeTarget{SpId: tt.spId}
			out, err := xml.Marshal(st)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var st2 ShapeTarget
			if err := xml.Unmarshal(out, &st2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if st2.SpId != tt.spId {
				t.Errorf("SpId = %q, want %q", st2.SpId, tt.spId)
			}
		})
	}
}

func TestTextElement_RoundTrip(t *testing.T) {
	xmlStr := `<txEl xmlns="http://schemas.openxmlformats.org/presentationml/2006/main">
  <charRg st="0" end="10"/>
  <pRg st="0" end="2"/>
</txEl>`

	var te TextElement
	if err := xml.Unmarshal([]byte(xmlStr), &te); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	out, err := xml.Marshal(&te)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var te2 TextElement
	if err := xml.Unmarshal(out, &te2); err != nil {
		t.Fatalf("Re-unmarshal failed: %v", err)
	}
}

func TestTimeNode_RoundTrip(t *testing.T) {
	vals := []uint32{1, 5, 100, 0}
	for _, val := range vals {
		t.Run("val", func(t *testing.T) {
			tn := &TimeNode{Val: val}
			out, err := xml.Marshal(tn)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var tn2 TimeNode
			if err := xml.Unmarshal(out, &tn2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if tn2.Val != val {
				t.Errorf("Val = %d, want %d", tn2.Val, val)
			}
		})
	}
}

func TestRuntimeNode_RoundTrip(t *testing.T) {
	vals := []string{"first", "last", "all"}
	for _, val := range vals {
		t.Run(val, func(t *testing.T) {
			rn := &RuntimeNode{Val: val}
			out, err := xml.Marshal(rn)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var rn2 RuntimeNode
			if err := xml.Unmarshal(out, &rn2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if rn2.Val != val {
				t.Errorf("Val = %q, want %q", rn2.Val, val)
			}
		})
	}
}

func TestIterate_RoundTrip(t *testing.T) {
	tests := []struct {
		name      string
		iterType  string
		backwards bool
	}{
		{"element forward", "el", false},
		{"element backward", "el", true},
		{"word", "wd", false},
		{"letter", "lt", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			iter := &Iterate{Type: tt.iterType, Backwards: tt.backwards}
			out, err := xml.Marshal(iter)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var iter2 Iterate
			if err := xml.Unmarshal(out, &iter2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if iter2.Type != tt.iterType {
				t.Errorf("Type = %q, want %q", iter2.Type, tt.iterType)
			}
			if iter2.Backwards != tt.backwards {
				t.Errorf("Backwards = %v, want %v", iter2.Backwards, tt.backwards)
			}
		})
	}
}

func TestAnimate_RoundTrip(t *testing.T) {
	tests := []struct {
		name      string
		from      string
		to        string
		calcMode  string
		valueType string
	}{
		{"opacity", "0", "1", "lin", "num"},
		{"position", "(0,0)", "(100,100)", "discrete", "str"},
		{"formula", "0", "100", "fmla", "num"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			anim := &Animate{
				From:      tt.from,
				To:        tt.to,
				CalcMode:  tt.calcMode,
				ValueType: tt.valueType,
			}
			out, err := xml.Marshal(anim)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var anim2 Animate
			if err := xml.Unmarshal(out, &anim2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if anim2.From != tt.from {
				t.Errorf("From = %q, want %q", anim2.From, tt.from)
			}
			if anim2.To != tt.to {
				t.Errorf("To = %q, want %q", anim2.To, tt.to)
			}
		})
	}
}

func TestAnimateColor_RoundTrip(t *testing.T) {
	tests := []struct {
		name   string
		clrSpc string
		dir    string
	}{
		{"rgb clockwise", "rgb", "cw"},
		{"rgb counter-clockwise", "rgb", "ccw"},
		{"hsl clockwise", "hsl", "cw"},
		{"hsl counter-clockwise", "hsl", "ccw"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ac := &AnimateColor{ClrSpc: tt.clrSpc, Dir: tt.dir}
			out, err := xml.Marshal(ac)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var ac2 AnimateColor
			if err := xml.Unmarshal(out, &ac2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if ac2.ClrSpc != tt.clrSpc {
				t.Errorf("ClrSpc = %q, want %q", ac2.ClrSpc, tt.clrSpc)
			}
			if ac2.Dir != tt.dir {
				t.Errorf("Dir = %q, want %q", ac2.Dir, tt.dir)
			}
		})
	}
}

func TestByRgbColor_RoundTrip(t *testing.T) {
	tests := []struct {
		r, g, b int32
	}{
		{0, 0, 0},
		{100000, 0, 0},
		{0, 100000, 0},
		{0, 0, 100000},
		{-50000, 50000, 0},
	}

	for _, tt := range tests {
		t.Run("rgb", func(t *testing.T) {
			rgb := &ByRgbColor{R: tt.r, G: tt.g, B: tt.b}
			out, err := xml.Marshal(rgb)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var rgb2 ByRgbColor
			if err := xml.Unmarshal(out, &rgb2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if rgb2.R != tt.r {
				t.Errorf("R = %d, want %d", rgb2.R, tt.r)
			}
			if rgb2.G != tt.g {
				t.Errorf("G = %d, want %d", rgb2.G, tt.g)
			}
			if rgb2.B != tt.b {
				t.Errorf("B = %d, want %d", rgb2.B, tt.b)
			}
		})
	}
}

func TestByHslColor_RoundTrip(t *testing.T) {
	tests := []struct {
		h, s, l int32
	}{
		{0, 0, 0},
		{21600000, 0, 0},
		{0, 100000, 0},
		{0, 0, 100000},
		{10800000, -50000, 50000},
	}

	for _, tt := range tests {
		t.Run("hsl", func(t *testing.T) {
			hsl := &ByHslColor{H: tt.h, S: tt.s, L: tt.l}
			out, err := xml.Marshal(hsl)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var hsl2 ByHslColor
			if err := xml.Unmarshal(out, &hsl2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if hsl2.H != tt.h {
				t.Errorf("H = %d, want %d", hsl2.H, tt.h)
			}
			if hsl2.S != tt.s {
				t.Errorf("S = %d, want %d", hsl2.S, tt.s)
			}
			if hsl2.L != tt.l {
				t.Errorf("L = %d, want %d", hsl2.L, tt.l)
			}
		})
	}
}

func TestAnimateEffect_RoundTrip(t *testing.T) {
	tests := []struct {
		name       string
		transition string
		filter     string
	}{
		{"in transition", "in", "fade"},
		{"out transition", "out", "blinds(horizontal)"},
		{"none", "none", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ae := &AnimateEffect{Transition: tt.transition, Filter: tt.filter}
			out, err := xml.Marshal(ae)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var ae2 AnimateEffect
			if err := xml.Unmarshal(out, &ae2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if ae2.Transition != tt.transition {
				t.Errorf("Transition = %q, want %q", ae2.Transition, tt.transition)
			}
		})
	}
}

func TestAnimateMotion_RoundTrip(t *testing.T) {
	tests := []struct {
		name         string
		origin       string
		path         string
		pathEditMode string
	}{
		{"parent origin", "parent", "M 0 0 L 100 100", "relative"},
		{"layout origin", "layout", "M 50 50 C 0 0 100 100 50 50", "fixed"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			am := &AnimateMotion{
				Origin:       tt.origin,
				Path:         tt.path,
				PathEditMode: tt.pathEditMode,
			}
			out, err := xml.Marshal(am)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var am2 AnimateMotion
			if err := xml.Unmarshal(out, &am2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if am2.Origin != tt.origin {
				t.Errorf("Origin = %q, want %q", am2.Origin, tt.origin)
			}
			if am2.Path != tt.path {
				t.Errorf("Path = %q, want %q", am2.Path, tt.path)
			}
		})
	}
}

func TestPoint_RoundTrip(t *testing.T) {
	tests := []struct {
		x, y string
	}{
		{"0", "0"},
		{"100000", "100000"},
		{"-50000", "50000"},
		{"50000", "-50000"},
	}

	for _, tt := range tests {
		t.Run("point", func(t *testing.T) {
			pt := &Point{X: tt.x, Y: tt.y}
			out, err := xml.Marshal(pt)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var pt2 Point
			if err := xml.Unmarshal(out, &pt2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if pt2.X != tt.x {
				t.Errorf("X = %q, want %q", pt2.X, tt.x)
			}
			if pt2.Y != tt.y {
				t.Errorf("Y = %q, want %q", pt2.Y, tt.y)
			}
		})
	}
}

func TestAnimateRotation_RoundTrip(t *testing.T) {
	tests := []struct {
		by, from, to int32
	}{
		{0, 0, 21600000},
		{5400000, 0, 0},
		{0, 10800000, 21600000},
	}

	for _, tt := range tests {
		t.Run("rotation", func(t *testing.T) {
			ar := &AnimateRotation{By: tt.by, From: tt.from, To: tt.to}
			out, err := xml.Marshal(ar)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var ar2 AnimateRotation
			if err := xml.Unmarshal(out, &ar2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if ar2.By != tt.by {
				t.Errorf("By = %d, want %d", ar2.By, tt.by)
			}
		})
	}
}

func TestAnimateScale_RoundTrip(t *testing.T) {
	tests := []struct {
		name         string
		zoomContents bool
	}{
		{"zoom contents", true},
		{"no zoom contents", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			as := &AnimateScale{ZoomContents: tt.zoomContents}
			out, err := xml.Marshal(as)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var as2 AnimateScale
			if err := xml.Unmarshal(out, &as2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if as2.ZoomContents != tt.zoomContents {
				t.Errorf("ZoomContents = %v, want %v", as2.ZoomContents, tt.zoomContents)
			}
		})
	}
}

func TestCommand_RoundTrip(t *testing.T) {
	tests := []struct {
		name    string
		cmdType string
		cmd     string
	}{
		{"event", "evt", "onstopaudio"},
		{"call", "call", "pause"},
		{"verb", "verb", "1"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Command{Type: tt.cmdType, Cmd: tt.cmd}
			out, err := xml.Marshal(c)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var c2 Command
			if err := xml.Unmarshal(out, &c2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if c2.Type != tt.cmdType {
				t.Errorf("Type = %q, want %q", c2.Type, tt.cmdType)
			}
			if c2.Cmd != tt.cmd {
				t.Errorf("Cmd = %q, want %q", c2.Cmd, tt.cmd)
			}
		})
	}
}

func TestAudio_RoundTrip(t *testing.T) {
	tests := []struct {
		name        string
		isNarration bool
	}{
		{"narration", true},
		{"not narration", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			audio := &Audio{IsNarration: tt.isNarration}
			out, err := xml.Marshal(audio)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var audio2 Audio
			if err := xml.Unmarshal(out, &audio2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if audio2.IsNarration != tt.isNarration {
				t.Errorf("IsNarration = %v, want %v", audio2.IsNarration, tt.isNarration)
			}
		})
	}
}

func TestVideo_RoundTrip(t *testing.T) {
	tests := []struct {
		name     string
		fullScrn bool
	}{
		{"fullscreen", true},
		{"not fullscreen", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			video := &Video{FullScrn: tt.fullScrn}
			out, err := xml.Marshal(video)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var video2 Video
			if err := xml.Unmarshal(out, &video2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if video2.FullScrn != tt.fullScrn {
				t.Errorf("FullScrn = %v, want %v", video2.FullScrn, tt.fullScrn)
			}
		})
	}
}

func TestCommonMediaNode_RoundTrip(t *testing.T) {
	tests := []struct {
		name            string
		vol             string
		mute            bool
		numSld          uint32
		showWhenStopped bool
	}{
		{"full volume", "100", false, 1, true},
		{"muted", "50", true, 0, false},
		{"silent", "0", false, 5, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmn := &CommonMediaNode{
				Vol:             tt.vol,
				Mute:            tt.mute,
				NumSld:          tt.numSld,
				ShowWhenStopped: tt.showWhenStopped,
			}
			out, err := xml.Marshal(cmn)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var cmn2 CommonMediaNode
			if err := xml.Unmarshal(out, &cmn2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if cmn2.Vol != tt.vol {
				t.Errorf("Vol = %q, want %q", cmn2.Vol, tt.vol)
			}
			if cmn2.Mute != tt.mute {
				t.Errorf("Mute = %v, want %v", cmn2.Mute, tt.mute)
			}
		})
	}
}

func TestCommonBehavior_RoundTrip(t *testing.T) {
	tests := []struct {
		name       string
		additive   string
		accumulate string
		override   string
	}{
		{"base additive", "base", "none", "normal"},
		{"sum additive", "sum", "always", "childStyle"},
		{"replace", "repl", "none", "normal"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cb := &CommonBehavior{
				Additive:   tt.additive,
				Accumulate: tt.accumulate,
				Override:   tt.override,
			}
			out, err := xml.Marshal(cb)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var cb2 CommonBehavior
			if err := xml.Unmarshal(out, &cb2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if cb2.Additive != tt.additive {
				t.Errorf("Additive = %q, want %q", cb2.Additive, tt.additive)
			}
		})
	}
}

func TestTimeAnimateValueList_RoundTrip(t *testing.T) {
	xmlStr := `<tavLst xmlns="http://schemas.openxmlformats.org/presentationml/2006/main">
  <tav tm="0"/>
  <tav tm="50000"/>
  <tav tm="100000"/>
</tavLst>`

	var tavl TimeAnimateValueList
	if err := xml.Unmarshal([]byte(xmlStr), &tavl); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if len(tavl.Tav) != 3 {
		t.Errorf("Expected 3 values, got %d", len(tavl.Tav))
	}

	out, err := xml.Marshal(&tavl)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var tavl2 TimeAnimateValueList
	if err := xml.Unmarshal(out, &tavl2); err != nil {
		t.Fatalf("Re-unmarshal failed: %v", err)
	}
}

func TestTimeAnimateValue_RoundTrip(t *testing.T) {
	tests := []struct {
		name string
		tm   string
		fmla string
	}{
		{"start", "0", ""},
		{"middle", "50000", ""},
		{"end", "100000", ""},
		{"with formula", "50000", "#ppt_x"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tav := &TimeAnimateValue{Tm: tt.tm, Fmla: tt.fmla}
			out, err := xml.Marshal(tav)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var tav2 TimeAnimateValue
			if err := xml.Unmarshal(out, &tav2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if tav2.Tm != tt.tm {
				t.Errorf("Tm = %q, want %q", tav2.Tm, tt.tm)
			}
		})
	}
}

func TestAnimVariant_RoundTrip(t *testing.T) {
	tests := []struct {
		name string
		xml  string
	}{
		{
			name: "bool value",
			xml:  `<val xmlns="http://schemas.openxmlformats.org/presentationml/2006/main"><boolVal val="true"/></val>`,
		},
		{
			name: "int value",
			xml:  `<val xmlns="http://schemas.openxmlformats.org/presentationml/2006/main"><intVal val="100"/></val>`,
		},
		{
			name: "float value",
			xml:  `<val xmlns="http://schemas.openxmlformats.org/presentationml/2006/main"><fltVal val="0.5"/></val>`,
		},
		{
			name: "string value",
			xml:  `<val xmlns="http://schemas.openxmlformats.org/presentationml/2006/main"><strVal val="visible"/></val>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var av AnimVariant
			if err := xml.Unmarshal([]byte(tt.xml), &av); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			out, err := xml.Marshal(&av)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var av2 AnimVariant
			if err := xml.Unmarshal(out, &av2); err != nil {
				t.Fatalf("Re-unmarshal failed: %v", err)
			}
		})
	}
}

func TestBuildList_RoundTrip(t *testing.T) {
	xmlStr := `<bldLst xmlns="http://schemas.openxmlformats.org/presentationml/2006/main">
  <bldP spid="2" grpId="0" build="p" animBg="true"/>
  <bldDgm spid="3" bld="one"/>
</bldLst>`

	var bl BuildList
	if err := xml.Unmarshal([]byte(xmlStr), &bl); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	out, err := xml.Marshal(&bl)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var bl2 BuildList
	if err := xml.Unmarshal(out, &bl2); err != nil {
		t.Fatalf("Re-unmarshal failed: %v", err)
	}
}

func TestBuildParagraph_RoundTrip(t *testing.T) {
	tests := []struct {
		name   string
		spId   string
		build  string
		animBg bool
	}{
		{"all at once", "1", "allAtOnce", false},
		{"by paragraph", "2", "p", true},
		{"custom", "3", "cust", false},
		{"whole", "4", "whole", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bp := &BuildParagraph{SpId: tt.spId, Build: tt.build, AnimBg: &tt.animBg}
			out, err := xml.Marshal(bp)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var bp2 BuildParagraph
			if err := xml.Unmarshal(out, &bp2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if bp2.SpId != tt.spId {
				t.Errorf("SpId = %q, want %q", bp2.SpId, tt.spId)
			}
			if bp2.Build != tt.build {
				t.Errorf("Build = %q, want %q", bp2.Build, tt.build)
			}
		})
	}
}

func TestBuildDiagram_RoundTrip(t *testing.T) {
	tests := []struct {
		name string
		spId string
		bld  string
		rev  bool
	}{
		{"all at once", "1", "allAtOnce", false},
		{"one by one", "2", "one", false},
		{"level one", "3", "lvlOne", true},
		{"level at once", "4", "lvlAtOnce", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bd := &BuildDiagram{SpId: tt.spId, Bld: tt.bld, Rev: tt.rev}
			out, err := xml.Marshal(bd)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var bd2 BuildDiagram
			if err := xml.Unmarshal(out, &bd2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if bd2.SpId != tt.spId {
				t.Errorf("SpId = %q, want %q", bd2.SpId, tt.spId)
			}
			if bd2.Bld != tt.bld {
				t.Errorf("Bld = %q, want %q", bd2.Bld, tt.bld)
			}
		})
	}
}

func TestBuildOleChart_RoundTrip(t *testing.T) {
	tests := []struct {
		name   string
		spId   string
		bld    string
		animBg bool
	}{
		{"all at once", "1", "allAtOnce", false},
		{"by series", "2", "series", true},
		{"by category", "3", "category", false},
		{"series elements", "4", "seriesEl", true},
		{"category elements", "5", "categoryEl", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			boc := &BuildOleChart{SpId: tt.spId, Bld: tt.bld, AnimBg: tt.animBg}
			out, err := xml.Marshal(boc)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var boc2 BuildOleChart
			if err := xml.Unmarshal(out, &boc2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if boc2.SpId != tt.spId {
				t.Errorf("SpId = %q, want %q", boc2.SpId, tt.spId)
			}
			if boc2.Bld != tt.bld {
				t.Errorf("Bld = %q, want %q", boc2.Bld, tt.bld)
			}
		})
	}
}

func TestBuildGraphic_RoundTrip(t *testing.T) {
	tests := []struct {
		name     string
		spId     string
		uiExpand bool
	}{
		{"basic", "1", false},
		{"expanded", "2", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bg := &BuildGraphic{SpId: tt.spId, UiExpand: tt.uiExpand}
			out, err := xml.Marshal(bg)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var bg2 BuildGraphic
			if err := xml.Unmarshal(out, &bg2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if bg2.SpId != tt.spId {
				t.Errorf("SpId = %q, want %q", bg2.SpId, tt.spId)
			}
			if bg2.UiExpand != tt.uiExpand {
				t.Errorf("UiExpand = %v, want %v", bg2.UiExpand, tt.uiExpand)
			}
		})
	}
}

func TestTemplate_RoundTrip(t *testing.T) {
	levels := []int32{1, 2, 5, 9}
	for _, lvl := range levels {
		t.Run("level", func(t *testing.T) {
			tmpl := &Template{Lvl: lvl}
			out, err := xml.Marshal(tmpl)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var tmpl2 Template
			if err := xml.Unmarshal(out, &tmpl2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if tmpl2.Lvl != lvl {
				t.Errorf("Lvl = %d, want %d", tmpl2.Lvl, lvl)
			}
		})
	}
}
