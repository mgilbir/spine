package pptx

import (
	"reflect"
	"regexp"
	"strconv"
	"testing"

	"github.com/mgilbir/spine/pptx/internal/oxml"
)

var cTnIDRE = regexp.MustCompile(`<p:cTn id="(\d+)"`)

// cTnIDs returns every p:cTn id emitted in the slide XML, in document order.
func cTnIDs(slideXML string) []string {
	var out []string
	for _, m := range cTnIDRE.FindAllStringSubmatch(slideXML, -1) {
		out = append(out, m[1])
	}
	return out
}

// C378: timingMaxIDs walked par/seq/excl and the five behavior kinds it knew
// about, but not p:cmd, p:audio, p:video (nor p:animClr / p:animMotion). Every
// tree the library generates for auto-play media carries both a p:cmd behavior
// and a root p:video/p:audio media node, so seeding an animation id generator
// from it under-reported the maximum and AddAnimation handed out ids already in
// use. PowerPoint keys bookmarks and condition targets off these ids, so a
// duplicate is not cosmetic.
func TestAddAnimation_NoIDCollisionWithAutoplayMedia(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	tb := s.AddTextBox()
	tb.TextFrame().SetText("hello")
	v := s.AddVideo([]byte("vid"), "video/mp4")
	v.SetPlayMode(PlayAutomatically)

	// First save resolves shape ids and builds the auto-play timing tree.
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	slideXML := string(zipPart(t, data, "ppt/slides/slide1.xml"))
	tbID := shapeIDByName(t, slideXML, "TextBox")

	// Animate the text box; the effect ids are allocated above the tree's max.
	id, err := parseUint32(tbID)
	if err != nil {
		t.Fatal(err)
	}
	s.AddAnimation(id, EffectFadeIn, TriggerOnClick)

	data2, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	out := string(zipPart(t, data2, "ppt/slides/slide1.xml"))

	ids := cTnIDs(out)
	seen := make(map[string]int, len(ids))
	for _, id := range ids {
		seen[id]++
	}
	var dupes []string
	for id, n := range seen {
		if n > 1 {
			dupes = append(dupes, id)
		}
	}
	if len(dupes) > 0 {
		t.Errorf("duplicate p:cTn ids %v in one p:timing (ids=%v)\n%s", dupes, ids, out)
	}
}

// C378 (unit): the walker must reach every time-node kind CT_TimeNodeList can
// hold. A tree that parks its only cTn under each kind in turn must report that
// node's id, so a new kind cannot silently escape the max scan.
func TestWalkCTns_CoversEveryTimeNodeKind(t *testing.T) {
	bhvr := func(id uint32) *oxml.CommonBehavior {
		return &oxml.CommonBehavior{CTn: &oxml.CommonTimeNode{Id: id}}
	}
	const want = 4242

	cases := map[string]func(*oxml.TimeNodeList){
		"par":        func(l *oxml.TimeNodeList) { l.AppendPar(&oxml.ParallelTimeNode{CTn: &oxml.CommonTimeNode{Id: want}}) },
		"seq":        func(l *oxml.TimeNodeList) { l.AppendSeq(&oxml.SequenceTimeNode{CTn: &oxml.CommonTimeNode{Id: want}}) },
		"excl":       func(l *oxml.TimeNodeList) { l.AppendExcl(&oxml.ExclusiveTimeNode{CTn: &oxml.CommonTimeNode{Id: want}}) },
		"anim":       func(l *oxml.TimeNodeList) { l.AppendAnim(&oxml.Animate{CBhvr: bhvr(want)}) },
		"animClr":    func(l *oxml.TimeNodeList) { l.AppendAnimClr(&oxml.AnimateColor{CBhvr: bhvr(want)}) },
		"animEffect": func(l *oxml.TimeNodeList) { l.AppendAnimEffect(&oxml.AnimateEffect{CBhvr: bhvr(want)}) },
		"animMotion": func(l *oxml.TimeNodeList) { l.AppendAnimMotion(&oxml.AnimateMotion{CBhvr: bhvr(want)}) },
		"animRot":    func(l *oxml.TimeNodeList) { l.AppendAnimRot(&oxml.AnimateRotation{CBhvr: bhvr(want)}) },
		"animScale":  func(l *oxml.TimeNodeList) { l.AppendAnimScale(&oxml.AnimateScale{CBhvr: bhvr(want)}) },
		"cmd":        func(l *oxml.TimeNodeList) { l.AppendCmd(&oxml.Command{CBhvr: bhvr(want)}) },
		"set":        func(l *oxml.TimeNodeList) { l.AppendSet(&oxml.Set{CBhvr: bhvr(want)}) },
		"audio": func(l *oxml.TimeNodeList) {
			l.AppendAudio(&oxml.Audio{CMediaNode: &oxml.CommonMediaNode{CTn: &oxml.CommonTimeNode{Id: want}}})
		},
		"video": func(l *oxml.TimeNodeList) {
			l.AppendVideo(&oxml.Video{CMediaNode: &oxml.CommonMediaNode{CTn: &oxml.CommonTimeNode{Id: want}}})
		},
	}

	for name, add := range cases {
		t.Run(name, func(t *testing.T) {
			l := &oxml.TimeNodeList{}
			add(l)
			got, _ := timingMaxIDs(&oxml.Timing{TnLst: l})
			if got != want {
				t.Errorf("timingMaxIDs missed the cTn under p:%s: got %d, want %d", name, got, want)
			}
		})
	}
}

// C378 (guard): the case table above must enumerate every kind
// CT_TimeNodeList models. If a new child kind is added to the oxml type and not
// to the walker's coverage test, this fails — the walker is a max-scan
// allocator, and a kind it cannot see hands out ids that are already in use.
func TestWalkCTns_CoverageTableIsComplete(t *testing.T) {
	// Every exported slice field of TimeNodeList is a time-node kind.
	var modeled []string
	rt := reflect.TypeOf(oxml.TimeNodeList{})
	for i := 0; i < rt.NumField(); i++ {
		f := rt.Field(i)
		if !f.IsExported() || f.Type.Kind() != reflect.Slice {
			continue
		}
		modeled = append(modeled, f.Name)
	}

	// Field name -> the element name used by the coverage test above.
	covered := map[string]string{
		"Par": "par", "Seq": "seq", "Excl": "excl",
		"Anim": "anim", "AnimClr": "animClr", "AnimEffect": "animEffect",
		"AnimMotion": "animMotion", "AnimRot": "animRot", "AnimScale": "animScale",
		"Cmd": "cmd", "Set": "set", "Audio": "audio", "Video": "video",
	}
	for _, name := range modeled {
		if _, ok := covered[name]; !ok {
			t.Errorf("TimeNodeList.%s is a time-node kind with no walkCTns coverage case; "+
				"add it to walkCTns and to TestWalkCTns_CoversEveryTimeNodeKind", name)
		}
	}
	if len(covered) != len(modeled) {
		t.Errorf("coverage table has %d entries for %d modeled kinds: %v", len(covered), len(modeled), modeled)
	}
}

// parseUint32 is a test helper: shape ids come back from the XML as strings.
func parseUint32(s string) (uint32, error) {
	n, err := strconv.ParseUint(s, 10, 32)
	return uint32(n), err
}
