package pptx

import (
	"strconv"

	"github.com/mgilbir/spine/pptx/internal/oxml"
)

// AnimationEffect identifies a slide animation effect. The common entrance,
// emphasis, and exit effects are supported; the visual behaviour is driven by
// the p:set/p:anim/p:animEffect/p:animRot/p:animScale bodies built for each,
// while presetID/presetClass/presetSubtype match what PowerPoint records so its
// editor shows the effect under the right name.
type AnimationEffect int

const (
	// EffectUnknown is the zero value and is returned by Animations for a
	// timing node whose preset this package does not recognize.
	EffectUnknown AnimationEffect = iota
	// EffectAppear is an entrance effect that makes the shape appear instantly.
	EffectAppear
	// EffectFadeIn is an entrance effect that fades the shape in.
	EffectFadeIn
	// EffectFlyIn is an entrance effect that flies the shape in from the bottom.
	EffectFlyIn
	// EffectWipe is an entrance effect that wipes the shape in from the bottom.
	EffectWipe
	// EffectZoom is an entrance effect that zooms the shape in.
	EffectZoom
	// EffectPulse is an emphasis effect that briefly grows the shape and returns.
	EffectPulse
	// EffectSpin is an emphasis effect that spins the shape a full turn.
	EffectSpin
	// EffectGrowShrink is an emphasis effect that grows the shape to 150%.
	EffectGrowShrink
	// EffectDisappear is an exit effect that hides the shape instantly.
	EffectDisappear
	// EffectFadeOut is an exit effect that fades the shape out.
	EffectFadeOut
	// EffectFlyOut is an exit effect that flies the shape out to the bottom.
	EffectFlyOut
)

// String returns the effect's stable identifier.
func (e AnimationEffect) String() string {
	switch e {
	case EffectAppear:
		return "appear"
	case EffectFadeIn:
		return "fadeIn"
	case EffectFlyIn:
		return "flyIn"
	case EffectWipe:
		return "wipe"
	case EffectZoom:
		return "zoom"
	case EffectPulse:
		return "pulse"
	case EffectSpin:
		return "spin"
	case EffectGrowShrink:
		return "growShrink"
	case EffectDisappear:
		return "disappear"
	case EffectFadeOut:
		return "fadeOut"
	case EffectFlyOut:
		return "flyOut"
	default:
		return "unknown"
	}
}

// AnimationTrigger controls when an animation starts relative to the ones
// before it in the slide's main animation sequence.
type AnimationTrigger int

const (
	// TriggerOnClick starts the effect on the next slide-advance click. It is
	// the zero value.
	TriggerOnClick AnimationTrigger = iota
	// TriggerWithPrevious starts the effect together with the previous one.
	TriggerWithPrevious
	// TriggerAfterPrevious starts the effect when the previous one finishes.
	TriggerAfterPrevious
)

// String returns the trigger's stable identifier.
func (t AnimationTrigger) String() string {
	switch t {
	case TriggerWithPrevious:
		return "withPrevious"
	case TriggerAfterPrevious:
		return "afterPrevious"
	default:
		return "onClick"
	}
}

// Animation is a handle to a slide animation. AddAnimation returns one so the
// effect can be further configured (e.g. build-by-paragraph) before the deck is
// saved; Animations returns them for reading back existing timing.
type Animation struct {
	shapeID     uint32
	effect      AnimationEffect
	trigger     AnimationTrigger
	byParagraph bool
}

// ShapeID returns the cNvPr id of the shape the animation targets.
func (a *Animation) ShapeID() uint32 { return a.shapeID }

// Effect returns the animation effect.
func (a *Animation) Effect() AnimationEffect { return a.effect }

// Trigger returns the animation's start trigger.
func (a *Animation) Trigger() AnimationTrigger { return a.trigger }

// ByParagraph reports whether the animation plays paragraph-by-paragraph.
func (a *Animation) ByParagraph() bool { return a.byParagraph }

// SetByParagraph enables (or disables) building the target's text one paragraph
// at a time, each paragraph revealed on its own click. It applies only to a
// shape with a text body of more than one paragraph; otherwise the whole shape
// animates as one. Returns the animation for chaining.
func (a *Animation) SetByParagraph(v bool) *Animation {
	a.byParagraph = v
	return a
}

// AddAnimation adds an animation targeting the shape with the given cNvPr id
// (see Shape.ID). effect selects the entrance/emphasis/exit effect and trigger
// controls its start relative to earlier animations. The p:timing tree is built
// (or, when the slide already has a main sequence, appended to) when the deck is
// saved. A timing tree the caller never touches this way round-trips unchanged.
func (s *Slide) AddAnimation(shapeID uint32, effect AnimationEffect, trigger AnimationTrigger) *Animation {
	// Materialize the slide so the save regenerates the part (and flushes this
	// animation into the timing tree) instead of passing the original bytes
	// through and dropping it.
	s.ensureModel()
	a := &Animation{shapeID: shapeID, effect: effect, trigger: trigger}
	s.pendingAnims = append(s.pendingAnims, a)
	return a
}

// Animations returns the slide's animations: those read from an existing
// p:timing main sequence plus any added via AddAnimation that are not yet
// serialized. Effects whose presets this package does not recognize are
// reported with EffectUnknown.
func (s *Slide) Animations() []*Animation {
	var out []*Animation
	if s.sx() != nil && s.sx().Timing != nil {
		out = readAnimations(s.sx().Timing)
	}
	out = append(out, s.pendingAnims...)
	return out
}

// --- timing tree construction ---

func u32p(v uint32) *uint32 { return &v }

// animIDGen hands out sequential time-node ids. next is the last id handed out,
// so alloc pre-increments; seed it with the current maximum id in the tree when
// appending to an existing sequence.
type animIDGen struct{ next uint32 }

func (g *animIDGen) alloc() uint32 { g.next++; return g.next }

// spTgtPara targets a single paragraph (0-based) of a shape's text body.
func spTgtPara(spid, para uint32) *oxml.TargetElement {
	return &oxml.TargetElement{SpTgt: &oxml.ShapeTarget{
		SpId: strconv.FormatUint(uint64(spid), 10),
		TxEl: &oxml.TextElement{PRg: &oxml.IndexRange{St: para, End: para}},
	}}
}

// effectDef describes how one AnimationEffect maps to PowerPoint's preset
// hints and behavior bodies. build appends the effect's behaviors (each with a
// fresh target from mk) to the returned childTnLst of the effect time node.
type effectDef struct {
	presetClass string // entr, emph, exit
	presetID    int32
	presetSub   int32
	build       func(g *animIDGen, mk func() *oxml.TargetElement) *oxml.TimeNodeList
}

func strv(s string) *oxml.AnimVariant {
	return &oxml.AnimVariant{StrVal: &oxml.AnimVariantString{Val: s}}
}

// setVis builds a p:set that forces the target's visibility (used to reveal a
// shape before an entrance motion, or hide it after an exit).
func setVis(g *animIDGen, tgt *oxml.TargetElement, val, delay string) *oxml.Set {
	ctn := &oxml.CommonTimeNode{Id: g.alloc(), Dur: "1", Fill: "hold"}
	if delay != "" {
		ctn.StCondLst = condDelay(delay)
	}
	return &oxml.Set{
		CBhvr: &oxml.CommonBehavior{
			CTn:         ctn,
			TgtEl:       tgt,
			AttrNameLst: &oxml.AttributeNameList{AttrName: []string{"style.visibility"}},
		},
		To: strv(val),
	}
}

// animEff builds a p:animEffect (a filter-driven transition such as fade or wipe).
func animEff(g *animIDGen, tgt *oxml.TargetElement, transition, filter string) *oxml.AnimateEffect {
	return &oxml.AnimateEffect{
		Transition: transition,
		Filter:     filter,
		CBhvr: &oxml.CommonBehavior{
			CTn:   &oxml.CommonTimeNode{Id: g.alloc(), Dur: "500"},
			TgtEl: tgt,
		},
	}
}

// animMove builds a p:anim that interpolates a position property (ppt_x/ppt_y)
// from one formula value to another — the motion behind fly in/out.
func animMove(g *animIDGen, tgt *oxml.TargetElement, attr, from, to string) *oxml.Animate {
	return &oxml.Animate{
		CalcMode:  "lin",
		ValueType: "num",
		CBhvr: &oxml.CommonBehavior{
			Additive:    "base",
			CTn:         &oxml.CommonTimeNode{Id: g.alloc(), Dur: "500", Fill: "hold"},
			TgtEl:       tgt,
			AttrNameLst: &oxml.AttributeNameList{AttrName: []string{attr}},
		},
		TavLst: &oxml.TimeAnimateValueList{Tav: []*oxml.TimeAnimateValue{
			{Tm: "0", Val: strv(from)},
			{Tm: "100000", Val: strv(to)},
		}},
	}
}

// animRotBy builds a p:animRot that rotates the target by an angle in 60000ths
// of a degree (21600000 = one full turn).
func animRotBy(g *animIDGen, tgt *oxml.TargetElement, by int32) *oxml.AnimateRotation {
	return &oxml.AnimateRotation{
		By: by,
		CBhvr: &oxml.CommonBehavior{
			CTn:         &oxml.CommonTimeNode{Id: g.alloc(), Dur: "2000", Fill: "hold"},
			TgtEl:       tgt,
			AttrNameLst: &oxml.AttributeNameList{AttrName: []string{"r"}},
		},
	}
}

// animScaleBy builds a p:animScale that scales the target to the given
// percentage (in 1000ths: 150000 = 150%), optionally auto-reversing.
func animScaleBy(g *animIDGen, tgt *oxml.TargetElement, pct string, autoRev bool) *oxml.AnimateScale {
	ctn := &oxml.CommonTimeNode{Id: g.alloc(), Dur: "2000", Fill: "hold"}
	if autoRev {
		yes := true
		ctn.AutoRev = &yes
	}
	return &oxml.AnimateScale{
		CBhvr: &oxml.CommonBehavior{CTn: ctn, TgtEl: tgt},
		By:    &oxml.Point{X: pct, Y: pct},
	}
}

// animEffectDefs maps each supported effect to its preset hints and body.
var animEffectDefs = map[AnimationEffect]effectDef{
	EffectAppear: {"entr", 1, 0, func(g *animIDGen, mk func() *oxml.TargetElement) *oxml.TimeNodeList {
		l := &oxml.TimeNodeList{}
		l.AppendSet(setVis(g, mk(), "visible", ""))
		return l
	}},
	EffectFadeIn: {"entr", 10, 0, func(g *animIDGen, mk func() *oxml.TargetElement) *oxml.TimeNodeList {
		l := &oxml.TimeNodeList{}
		l.AppendAnimEffect(animEff(g, mk(), "in", "fade"))
		return l
	}},
	EffectFlyIn: {"entr", 2, 4, func(g *animIDGen, mk func() *oxml.TargetElement) *oxml.TimeNodeList {
		l := &oxml.TimeNodeList{}
		l.AppendSet(setVis(g, mk(), "visible", ""))
		l.AppendAnim(animMove(g, mk(), "ppt_y", "1+#ppt_h/2", "#ppt_y"))
		return l
	}},
	EffectWipe: {"entr", 22, 4, func(g *animIDGen, mk func() *oxml.TargetElement) *oxml.TimeNodeList {
		l := &oxml.TimeNodeList{}
		l.AppendAnimEffect(animEff(g, mk(), "in", "wipe(up)"))
		return l
	}},
	EffectZoom: {"entr", 23, 16, func(g *animIDGen, mk func() *oxml.TargetElement) *oxml.TimeNodeList {
		l := &oxml.TimeNodeList{}
		l.AppendAnimEffect(animEff(g, mk(), "in", "zoom"))
		return l
	}},
	EffectPulse: {"emph", 1, 0, func(g *animIDGen, mk func() *oxml.TargetElement) *oxml.TimeNodeList {
		l := &oxml.TimeNodeList{}
		l.AppendAnimScale(animScaleBy(g, mk(), "110000", true))
		return l
	}},
	EffectSpin: {"emph", 8, 0, func(g *animIDGen, mk func() *oxml.TargetElement) *oxml.TimeNodeList {
		l := &oxml.TimeNodeList{}
		l.AppendAnimRot(animRotBy(g, mk(), 21600000))
		return l
	}},
	EffectGrowShrink: {"emph", 6, 0, func(g *animIDGen, mk func() *oxml.TargetElement) *oxml.TimeNodeList {
		l := &oxml.TimeNodeList{}
		l.AppendAnimScale(animScaleBy(g, mk(), "150000", false))
		return l
	}},
	EffectDisappear: {"exit", 1, 0, func(g *animIDGen, mk func() *oxml.TargetElement) *oxml.TimeNodeList {
		l := &oxml.TimeNodeList{}
		l.AppendSet(setVis(g, mk(), "hidden", ""))
		return l
	}},
	EffectFadeOut: {"exit", 10, 0, func(g *animIDGen, mk func() *oxml.TargetElement) *oxml.TimeNodeList {
		l := &oxml.TimeNodeList{}
		l.AppendAnimEffect(animEff(g, mk(), "out", "fade"))
		return l
	}},
	EffectFlyOut: {"exit", 2, 4, func(g *animIDGen, mk func() *oxml.TargetElement) *oxml.TimeNodeList {
		l := &oxml.TimeNodeList{}
		l.AppendAnim(animMove(g, mk(), "ppt_y", "#ppt_y", "1+#ppt_h/2"))
		l.AppendSet(setVis(g, mk(), "hidden", "499"))
		return l
	}},
}

// effectByPreset reverses animEffectDefs for reading a timing tree back.
var effectByPreset = func() map[presetKey]AnimationEffect {
	m := make(map[presetKey]AnimationEffect, len(animEffectDefs))
	for eff, def := range animEffectDefs {
		m[presetKey{def.presetClass, def.presetID}] = eff
	}
	return m
}()

type presetKey struct {
	class string
	id    int32
}

func nodeTypeFor(t AnimationTrigger) string {
	switch t {
	case TriggerWithPrevious:
		return "withEffect"
	case TriggerAfterPrevious:
		return "afterEffect"
	default:
		return "clickEffect"
	}
}

// seqBuilder appends effect time nodes into a main sequence, grouping them under
// click nodes: an on-click effect opens a new click group, while with/after
// previous effects join the current group.
type seqBuilder struct {
	mainList *oxml.TimeNodeList // the mainSeq's childTnLst
	idgen    *animIDGen
	grpNext  uint32
	curGroup *oxml.TimeNodeList // current click group's childTnLst, or nil
}

func (sb *seqBuilder) nextGrp() uint32 {
	g := sb.grpNext
	sb.grpNext++
	return g
}

// newClickPar builds a click time node (waits for a click) wrapping a with-group
// node (starts immediately), returning the par and the group's childTnLst that
// effect nodes are appended to.
func newClickPar(g *animIDGen) (*oxml.ParallelTimeNode, *oxml.TimeNodeList) {
	clickID := g.alloc()
	groupID := g.alloc()
	group := &oxml.TimeNodeList{}
	inner := &oxml.TimeNodeList{}
	inner.AppendPar(&oxml.ParallelTimeNode{CTn: &oxml.CommonTimeNode{
		Id: groupID, Fill: "hold", StCondLst: condDelay("0"), ChildTnLst: group,
	}})
	par := &oxml.ParallelTimeNode{CTn: &oxml.CommonTimeNode{
		Id: clickID, Fill: "hold", StCondLst: condDelay("indefinite"), ChildTnLst: inner,
	}}
	return par, group
}

// buildEffectPar builds the effect time node (the cTn carrying the preset hints)
// with its behavior bodies.
func buildEffectPar(def effectDef, nodeType string, grpID uint32, g *animIDGen, mk func() *oxml.TargetElement) *oxml.ParallelTimeNode {
	effID := g.alloc()
	child := def.build(g, mk)
	return &oxml.ParallelTimeNode{CTn: &oxml.CommonTimeNode{
		Id:            effID,
		Presetid:      i32p(def.presetID),
		PresetClass:   def.presetClass,
		PresetSubtype: i32p(def.presetSub),
		Fill:          "hold",
		GrpId:         u32p(grpID),
		NodeType:      nodeType,
		StCondLst:     condDelay("0"),
		ChildTnLst:    child,
	}}
}

func (sb *seqBuilder) addEffect(def effectDef, trigger AnimationTrigger, grpID uint32, mk func() *oxml.TargetElement) {
	if trigger == TriggerOnClick || sb.curGroup == nil {
		par, group := newClickPar(sb.idgen)
		sb.mainList.AppendPar(par)
		sb.curGroup = group
	}
	sb.curGroup.AppendPar(buildEffectPar(def, nodeTypeFor(trigger), grpID, sb.idgen, mk))
}

// applyAnimations flushes pending animations into the slide's timing tree,
// building a fresh main sequence when the slide has none or appending into an
// existing one. It never runs when nothing was added via AddAnimation, so an
// untouched timing tree is left byte-for-byte intact.
func (s *Slide) applyAnimations() {
	if len(s.pendingAnims) == 0 {
		return
	}
	anims := s.pendingAnims
	s.pendingAnims = nil
	if s.sx() == nil {
		return
	}

	if s.sx().Timing == nil {
		s.sx().Timing = &oxml.Timing{}
	}
	t := s.sx().Timing
	maxID, nextGrp := timingMaxIDs(t)
	idgen := &animIDGen{next: maxID}
	mainList := mainSeqChildList(t, idgen)
	sb := &seqBuilder{mainList: mainList, idgen: idgen, grpNext: nextGrp}

	added := false
	for _, a := range anims {
		def, ok := animEffectDefs[a.effect]
		if !ok {
			continue
		}
		if a.byParagraph {
			if n := s.paragraphCount(a.shapeID); n > 1 {
				grpID := sb.nextGrp()
				s.ensureBldP(a.shapeID, grpID)
				for i := 0; i < n; i++ {
					trig := TriggerOnClick
					if i == 0 {
						trig = a.trigger
					}
					para := uint32(i)
					sb.addEffect(def, trig, grpID, func() *oxml.TargetElement {
						return spTgtPara(a.shapeID, para)
					})
				}
				added = true
				continue
			}
		}
		sid := a.shapeID
		sb.addEffect(def, a.trigger, sb.nextGrp(), func() *oxml.TargetElement {
			return spTgt(sid)
		})
		added = true
	}

	// Once authored (non-mediacall) effects are appended, freeze a
	// library-generated autoplay tree so a later autoplay rebuild
	// (buildAutoplayTiming, which emits only mediacall nodes — a second autoplay
	// medium, SetPlayMode, or a full rebuild) cannot replace the tree and drop
	// these animations. The added media still plays on click, exactly as media
	// added to a timing tree parsed from a file does.
	if added && s.timingAutoGenerated {
		s.timingAutoGenerated = false
	}
}

// mainSeqChildList returns the childTnLst of the timing tree's main sequence,
// creating the tmRoot and/or mainSeq scaffolding (with fresh ids from g) when
// they are absent. The returned list is where click groups are appended.
func mainSeqChildList(t *oxml.Timing, g *animIDGen) *oxml.TimeNodeList {
	if t.TnLst == nil {
		t.TnLst = &oxml.TimeNodeList{}
	}
	var root *oxml.CommonTimeNode
	for _, p := range t.TnLst.Par {
		if p != nil && p.CTn != nil && p.CTn.NodeType == "tmRoot" {
			root = p.CTn
			break
		}
	}
	if root == nil {
		root = &oxml.CommonTimeNode{
			Id: g.alloc(), Dur: "indefinite", Restart: "never", NodeType: "tmRoot",
			ChildTnLst: &oxml.TimeNodeList{},
		}
		t.TnLst.AppendPar(&oxml.ParallelTimeNode{CTn: root})
	}
	if root.ChildTnLst == nil {
		root.ChildTnLst = &oxml.TimeNodeList{}
	}
	for _, sq := range root.ChildTnLst.Seq {
		if sq != nil && sq.CTn != nil && sq.CTn.NodeType == "mainSeq" {
			if sq.CTn.ChildTnLst == nil {
				sq.CTn.ChildTnLst = &oxml.TimeNodeList{}
			}
			return sq.CTn.ChildTnLst
		}
	}
	mainChild := &oxml.TimeNodeList{}
	root.ChildTnLst.AppendSeq(&oxml.SequenceTimeNode{
		Concurrent:  true,
		NextAc:      "seek",
		CTn:         &oxml.CommonTimeNode{Id: g.alloc(), Dur: "indefinite", NodeType: "mainSeq", ChildTnLst: mainChild},
		PrevCondLst: slideEventCond("onPrev"),
		NextCondLst: slideEventCond("onNext"),
	})
	return mainChild
}

// paragraphCount returns the number of paragraphs in the text body of the shape
// with the given cNvPr id, or 0 when the shape has no text body.
func (s *Slide) paragraphCount(spid uint32) int {
	if s.sx() == nil || s.sx().CSld == nil || s.sx().CSld.SpTree == nil {
		return 0
	}
	for _, sp := range s.sx().CSld.SpTree.Sp {
		if sp.NvSpPr != nil && sp.NvSpPr.CNvPr != nil && sp.NvSpPr.CNvPr.Id == spid {
			if sp.TxBody != nil {
				return len(sp.TxBody.P)
			}
			return 0
		}
	}
	return 0
}

// ensureBldP records a build-by-paragraph entry for the shape in the timing's
// build list, so PowerPoint groups the per-paragraph effects as one text build.
func (s *Slide) ensureBldP(spid, grpID uint32) {
	t := s.sx().Timing
	if t.BldLst == nil {
		t.BldLst = &oxml.BuildList{}
	}
	sid := strconv.FormatUint(uint64(spid), 10)
	for _, b := range t.BldLst.BldP {
		if b != nil && b.SpId == sid {
			return
		}
	}
	t.BldLst.BldP = append(t.BldLst.BldP, &oxml.BuildParagraph{
		SpId: sid, GrpId: u32p(grpID), Build: "p",
	})
}

// --- timing tree traversal (max ids, reading) ---

// timingMaxIDs walks the tree for the largest time-node id and group id in use.
// The returned nextGrp is one past the largest group id, or 0 when none is set,
// so freshly built trees start their first click group at 0 like PowerPoint.
func timingMaxIDs(t *oxml.Timing) (maxID, nextGrp uint32) {
	hasGrp := false
	var maxGrp uint32
	walkCTns(t.TnLst, func(c *oxml.CommonTimeNode) {
		if c.Id > maxID {
			maxID = c.Id
		}
		if c.GrpId != nil {
			hasGrp = true
			if *c.GrpId > maxGrp {
				maxGrp = *c.GrpId
			}
		}
	})
	if hasGrp {
		return maxID, maxGrp + 1
	}
	return maxID, 0
}

// walkCTns visits every common time node reachable from a time-node list,
// descending through par/seq/excl nodes and behavior bodies.
func walkCTns(tnl *oxml.TimeNodeList, fn func(*oxml.CommonTimeNode)) {
	if tnl == nil {
		return
	}
	for _, p := range tnl.Par {
		if p != nil {
			walkCTnNode(p.CTn, fn)
		}
	}
	for _, sq := range tnl.Seq {
		if sq != nil {
			walkCTnNode(sq.CTn, fn)
		}
	}
	for _, ex := range tnl.Excl {
		if ex != nil {
			walkCTnNode(ex.CTn, fn)
		}
	}
	for _, b := range tnl.Set {
		if b != nil && b.CBhvr != nil {
			walkCTnNode(b.CBhvr.CTn, fn)
		}
	}
	for _, b := range tnl.Anim {
		if b != nil && b.CBhvr != nil {
			walkCTnNode(b.CBhvr.CTn, fn)
		}
	}
	for _, b := range tnl.AnimEffect {
		if b != nil && b.CBhvr != nil {
			walkCTnNode(b.CBhvr.CTn, fn)
		}
	}
	for _, b := range tnl.AnimRot {
		if b != nil && b.CBhvr != nil {
			walkCTnNode(b.CBhvr.CTn, fn)
		}
	}
	for _, b := range tnl.AnimScale {
		if b != nil && b.CBhvr != nil {
			walkCTnNode(b.CBhvr.CTn, fn)
		}
	}
}

func walkCTnNode(c *oxml.CommonTimeNode, fn func(*oxml.CommonTimeNode)) {
	if c == nil {
		return
	}
	fn(c)
	walkCTns(c.ChildTnLst, fn)
	walkCTns(c.SubTnLst, fn)
}

// readAnimations reconstructs Animation handles from a timing tree's main
// sequence: each effect time node (one carrying a presetClass) yields its
// effect, trigger, and target shape.
func readAnimations(t *oxml.Timing) []*Animation {
	root := findMainSeqChild(t)
	if root == nil {
		return nil
	}
	var out []*Animation
	collectEffects(root, &out)
	return out
}

// findMainSeqChild returns the mainSeq's childTnLst if the tree has one.
func findMainSeqChild(t *oxml.Timing) *oxml.TimeNodeList {
	if t == nil || t.TnLst == nil {
		return nil
	}
	for _, p := range t.TnLst.Par {
		if p == nil || p.CTn == nil || p.CTn.NodeType != "tmRoot" || p.CTn.ChildTnLst == nil {
			continue
		}
		for _, sq := range p.CTn.ChildTnLst.Seq {
			if sq != nil && sq.CTn != nil && sq.CTn.NodeType == "mainSeq" {
				return sq.CTn.ChildTnLst
			}
		}
	}
	return nil
}

// collectEffects walks the click groups of a main sequence, emitting one
// Animation per effect time node (a par whose cTn carries a known presetClass).
func collectEffects(tnl *oxml.TimeNodeList, out *[]*Animation) {
	if tnl == nil {
		return
	}
	for _, p := range tnl.Par {
		if p == nil || p.CTn == nil {
			continue
		}
		if a := effectFromCTn(p.CTn); a != nil {
			*out = append(*out, a)
			continue
		}
		collectEffects(p.CTn.ChildTnLst, out)
	}
}

// effectFromCTn returns the Animation described by an effect time node, or nil
// when the node is not a recognized entrance/emphasis/exit effect.
func effectFromCTn(c *oxml.CommonTimeNode) *Animation {
	if c.PresetClass == "" || c.Presetid == nil {
		return nil
	}
	eff, ok := effectByPreset[presetKey{c.PresetClass, *c.Presetid}]
	if !ok {
		eff = EffectUnknown
	}
	a := &Animation{effect: eff}
	switch c.NodeType {
	case "withEffect":
		a.trigger = TriggerWithPrevious
	case "afterEffect":
		a.trigger = TriggerAfterPrevious
	default:
		a.trigger = TriggerOnClick
	}
	if spid, para, ok := targetOfCTn(c.ChildTnLst); ok {
		a.shapeID = spid
		a.byParagraph = para
	}
	return a
}

// targetOfCTn finds the shape a node's behaviors animate, reporting whether the
// target addresses a text paragraph range (build-by-paragraph).
func targetOfCTn(tnl *oxml.TimeNodeList) (spid uint32, byPara bool, ok bool) {
	if tnl == nil {
		return 0, false, false
	}
	var found *oxml.ShapeTarget
	collect := func(cb *oxml.CommonBehavior) {
		if found == nil && cb != nil && cb.TgtEl != nil && cb.TgtEl.SpTgt != nil {
			found = cb.TgtEl.SpTgt
		}
	}
	for _, b := range tnl.Set {
		if b != nil {
			collect(b.CBhvr)
		}
	}
	for _, b := range tnl.Anim {
		if b != nil {
			collect(b.CBhvr)
		}
	}
	for _, b := range tnl.AnimEffect {
		if b != nil {
			collect(b.CBhvr)
		}
	}
	for _, b := range tnl.AnimRot {
		if b != nil {
			collect(b.CBhvr)
		}
	}
	for _, b := range tnl.AnimScale {
		if b != nil {
			collect(b.CBhvr)
		}
	}
	if found == nil {
		return 0, false, false
	}
	id, err := strconv.ParseUint(found.SpId, 10, 32)
	if err != nil {
		return 0, false, false
	}
	return uint32(id), found.TxEl != nil && found.TxEl.PRg != nil, true
}
