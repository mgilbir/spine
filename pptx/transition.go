package pptx

import (
	"bytes"
	"strconv"

	"github.com/mgilbir/spine/common/dml"
	coxml "github.com/mgilbir/spine/common/oxml"
	xmlb "github.com/mgilbir/spine/common/xml"
	"github.com/mgilbir/spine/pptx/internal/oxml"
)

// nsMorph is the PowerPoint 2015 morph-transition namespace (prefix p159). The
// morph transition is not part of the base PresentationML schema, so PowerPoint
// wraps the p:transition that carries <p159:morph> in an mc:AlternateContent,
// with a plain fade as the mc:Fallback for readers that do not understand it.
const nsMorph = "http://schemas.microsoft.com/office/powerpoint/2015/09/main"

// TransitionType represents the type of slide transition.
type TransitionType int

const (
	TransitionNone      TransitionType = iota
	TransitionFade                     // Fade
	TransitionPush                     // Push from a side
	TransitionWipe                     // Wipe from a side
	TransitionSplit                    // Split in/out
	TransitionCover                    // Cover from a direction
	TransitionDissolve                 // Dissolve
	TransitionBlind                    // Blinds
	TransitionChecker                  // Checkerboard
	TransitionWheel                    // Wheel (clock-like)
	TransitionRandom                   // Random transition
	TransitionCut                      // Cut
	TransitionDiamond                  // Diamond
	TransitionPlus                     // Plus sign
	TransitionCircle                   // Circle
	TransitionComb                     // Comb
	TransitionNewsflash                // Newsflash
	TransitionPull                     // Pull from a direction
	TransitionRandomBar                // Random bars
	TransitionStrips                   // Strips from a corner
	TransitionWedge                    // Wedge
	TransitionZoom                     // Zoom in/out
	TransitionMorph                    // Morph (p159 extension, PowerPoint 2016+)
)

// MorphOption selects what the Morph transition animates between the two
// slides: whole objects, words, or characters.
type MorphOption string

const (
	// MorphByObject morphs whole shapes/objects (PowerPoint's default).
	MorphByObject MorphOption = "byObject"
	// MorphByWord morphs at the word level within text.
	MorphByWord MorphOption = "byWord"
	// MorphByChar morphs at the character level within text.
	MorphByChar MorphOption = "byChar"
)

// TransitionDirection is the direction a directional transition moves in. Which
// values are valid depends on the transition type: side transitions (Push,
// Wipe) accept the four edge directions; Cover and Pull additionally accept the
// four corners; Strips accepts only corners; Zoom and Split accept In/Out.
type TransitionDirection string

const (
	TransitionDirLeft      TransitionDirection = "l"
	TransitionDirRight     TransitionDirection = "r"
	TransitionDirUp        TransitionDirection = "u"
	TransitionDirDown      TransitionDirection = "d"
	TransitionDirLeftUp    TransitionDirection = "lu"
	TransitionDirRightUp   TransitionDirection = "ru"
	TransitionDirLeftDown  TransitionDirection = "ld"
	TransitionDirRightDown TransitionDirection = "rd"
	TransitionDirIn        TransitionDirection = "in"  // Zoom, Split
	TransitionDirOut       TransitionDirection = "out" // Zoom, Split
)

// TransitionOrientation is the orientation of an oriented transition (Blinds,
// Checker, Comb, RandomBar, Split).
type TransitionOrientation string

const (
	TransitionHorizontal TransitionOrientation = "horz"
	TransitionVertical   TransitionOrientation = "vert"
)

// TransitionSound describes the sound action attached to a transition
// (p:sndAc): a start sound (p:stSnd) that plays when the transition runs and/or
// a stop-previous action (p:endSnd).
type TransitionSound struct {
	// StopPreviousSound emits <p:endSnd/>, silencing any sound still playing
	// from an earlier transition. It needs no embedded audio part, so it can be
	// set on any transition.
	StopPreviousSound bool
	// StartSoundName is the display name of the start sound (<p:stSnd>);
	// StartSoundLoop plays it until the next sound. On a parsed transition they
	// report the existing start sound; when authoring one (StartSoundData set)
	// they name and loop the embedded clip.
	StartSoundName string
	StartSoundLoop bool

	// StartSoundData, when non-empty, is an audio clip to embed as the start
	// sound. On the next Slide.SetTransition it is stored as a media part and
	// referenced by the emitted <p:stSnd>; embedded transition sounds are
	// typically WAV. StartSoundContentType is its MIME type (e.g. "audio/wav"),
	// inferred from the bytes when empty. Once embedded, the start sound (its
	// r:embed, name, and loop flag) round-trips like a parsed one.
	StartSoundData        []byte
	StartSoundContentType string

	// startSoundEmbed preserves the r:embed of a parsed or freshly embedded
	// start sound so it survives a round trip when the transition is otherwise
	// re-set (and so a repeated SetTransition does not embed the clip twice).
	startSoundEmbed string
}

// Transition represents slide transition settings.
type Transition struct {
	Type TransitionType
	// Duration is the transition speed in seconds. The base OOXML schema only
	// stores a coarse speed (fast/med/slow), so the value snaps to 0.5, 1.0, or
	// 2.0 on a round-trip; exact durations require the p14 extension, which is
	// not yet modeled.
	Duration       float64
	AdvanceOnClick bool
	AdvanceAfter   float64 // seconds, 0 = disabled

	// Direction applies to directional transitions (Push, Wipe, Cover, Pull,
	// Strips, Zoom, Split); it is ignored for others.
	Direction TransitionDirection
	// Orientation applies to oriented transitions (Blind, Checker, Comb,
	// RandomBar, Split); it is ignored for others.
	Orientation TransitionOrientation
	// Spokes is the number of spokes for the Wheel transition (1, 2, 3, 4, or
	// 8); 0 keeps PowerPoint's default of 4.
	Spokes int
	// ThroughBlack applies to the Cut and Fade transitions: fade/cut via a
	// black frame instead of directly.
	ThroughBlack bool
	// MorphOption applies to the Morph transition: what it animates between
	// (objects, words, or characters). Empty defaults to MorphByObject.
	MorphOption MorphOption
	// Sound is the transition's sound action, or nil when it has none.
	Sound *TransitionSound
}

// SetTransition sets the slide transition.
func (s *Slide) SetTransition(t Transition) {
	s.ensureModel()

	// A morph (or any other) transition replaces whatever transition the slide
	// had, including a previously authored/parsed morph AlternateContent, so drop
	// any existing morph wrapper first.
	s.sx().RemoveAlternateContent(isMorphAlternateContent)

	if t.Type == TransitionNone {
		s.sx().Transition = nil
		return
	}

	// Morph is not expressible in the base p:transition schema: it is a
	// <p159:morph> child wrapped in an mc:AlternateContent that stands in for the
	// transition element. Build that wrapper and clear the base transition.
	if t.Type == TransitionMorph {
		s.sx().Transition = nil
		s.sx().AppendAlternateContent(buildMorphAlternateContent(t), "transition")
		return
	}

	// Always set advClick explicitly so AdvanceOnClick=false is emitted
	// (advClick="0") rather than omitted and read back as the default true.
	advClick := t.AdvanceOnClick
	tr := &oxml.Transition{
		AdvClick: &advClick,
	}

	// Convert duration to speed attribute
	if t.Duration > 0 {
		switch {
		case t.Duration <= 0.5:
			tr.Spd = "fast"
		case t.Duration <= 1.0:
			tr.Spd = "med"
		default:
			tr.Spd = "slow"
		}
	}

	// Convert advance after to milliseconds
	if t.AdvanceAfter > 0 {
		advTm := uint32(t.AdvanceAfter * 1000)
		tr.AdvTm = &advTm
	}

	dir := string(t.Direction)
	orient := string(t.Orientation)

	// Set transition type
	switch t.Type {
	case TransitionFade:
		tr.Fade = &oxml.OptionalBlackTransition{ThruBlk: t.ThroughBlack}
	case TransitionCut:
		tr.Cut = &oxml.OptionalBlackTransition{ThruBlk: t.ThroughBlack}
	case TransitionPush:
		tr.Push = &oxml.SideDirectionTransition{Dir: dir}
	case TransitionWipe:
		tr.Wipe = &oxml.SideDirectionTransition{Dir: dir}
	case TransitionSplit:
		tr.Split = &oxml.SplitTransition{Orient: orient, Dir: dir}
	case TransitionCover:
		tr.Cover = &oxml.EightDirectionTransition{Dir: dir}
	case TransitionPull:
		tr.Pull = &oxml.EightDirectionTransition{Dir: dir}
	case TransitionStrips:
		tr.Strips = &oxml.CornerDirectionTransition{Dir: dir}
	case TransitionZoom:
		tr.Zoom = &oxml.InOutTransition{Dir: dir}
	case TransitionDissolve:
		tr.Dissolve = &oxml.EmptyTransition{}
	case TransitionBlind:
		tr.Blinds = &oxml.OrientationTransition{Dir: orient}
	case TransitionChecker:
		tr.Checker = &oxml.OrientationTransition{Dir: orient}
	case TransitionComb:
		tr.Comb = &oxml.OrientationTransition{Dir: orient}
	case TransitionRandomBar:
		tr.RandomBar = &oxml.OrientationTransition{Dir: orient}
	case TransitionWheel:
		spokes := uint32(t.Spokes)
		if spokes == 0 {
			spokes = 4
		}
		tr.Wheel = &oxml.WheelTransition{Spokes: spokes}
	case TransitionRandom:
		tr.Random = &oxml.EmptyTransition{}
	case TransitionDiamond:
		tr.Diamond = &oxml.EmptyTransition{}
	case TransitionPlus:
		tr.Plus = &oxml.EmptyTransition{}
	case TransitionCircle:
		tr.Circle = &oxml.EmptyTransition{}
	case TransitionNewsflash:
		tr.Newsflash = &oxml.EmptyTransition{}
	case TransitionWedge:
		tr.Wedge = &oxml.EmptyTransition{}
	}

	if snd := s.soundActionToOxml(t.Sound); snd != nil {
		tr.SndAc = snd
	}

	s.sx().Transition = tr
}

// soundActionToOxml builds a p:sndAc from the public sound settings, or nil,
// embedding a newly supplied start-sound clip as a media part first. A start
// sound whose content type cannot be resolved (not given and unrecognizable) is
// omitted; the stop-previous action, which needs no part, is still emitted.
func (s *Slide) soundActionToOxml(snd *TransitionSound) *oxml.TransitionSoundAction {
	if snd == nil {
		return nil
	}
	ac := &oxml.TransitionSoundAction{}

	if snd.startSoundEmbed == "" && len(snd.StartSoundData) > 0 &&
		s.presentation != nil && s.partName != "" {
		ct := snd.StartSoundContentType
		if ct == "" {
			ct = sniffMediaContentType(snd.StartSoundData)
		}
		if ct != "" {
			snd.startSoundEmbed = s.embedAudioPart(snd.StartSoundData, ct)
		}
	}

	if snd.startSoundEmbed != "" {
		ac.StSnd = &oxml.TransitionStartSoundAction{
			Loop: snd.StartSoundLoop,
			Snd: &dml.EmbeddedWAVXML{
				Embed: snd.startSoundEmbed,
				Name:  snd.StartSoundName,
			},
		}
	}
	if snd.StopPreviousSound {
		ac.EndSnd = &oxml.EmptyTransition{}
	}
	if ac.StSnd == nil && ac.EndSnd == nil {
		return nil
	}
	return ac
}

// Transition returns the current slide transition, or nil if none is set.
func (s *Slide) Transition() *Transition {
	if s.sx() == nil {
		return nil
	}
	// A morph transition lives in an mc:AlternateContent standing in for the base
	// transition element, so it is reported even though slideXML.Transition is nil.
	if m := morphFromAlternateContent(s.sx().AlternateContent); m != nil {
		return m
	}
	if s.sx().Transition == nil {
		return nil
	}

	tr := s.sx().Transition
	t := &Transition{
		// advClick defaults to true when the attribute is absent.
		AdvanceOnClick: tr.AdvClick == nil || *tr.AdvClick,
	}

	// Convert speed to approximate duration
	switch tr.Spd {
	case "fast":
		t.Duration = 0.5
	case "med":
		t.Duration = 1.0
	case "slow":
		t.Duration = 2.0
	default:
		t.Duration = 1.0 // default
	}

	// Convert advance time from ms to seconds
	if tr.AdvTm != nil && *tr.AdvTm > 0 {
		t.AdvanceAfter = float64(*tr.AdvTm) / 1000.0
	}

	// Detect type and its parameters
	switch {
	case tr.Fade != nil:
		t.Type, t.ThroughBlack = TransitionFade, tr.Fade.ThruBlk
	case tr.Cut != nil:
		t.Type, t.ThroughBlack = TransitionCut, tr.Cut.ThruBlk
	case tr.Push != nil:
		t.Type, t.Direction = TransitionPush, TransitionDirection(tr.Push.Dir)
	case tr.Wipe != nil:
		t.Type, t.Direction = TransitionWipe, TransitionDirection(tr.Wipe.Dir)
	case tr.Split != nil:
		t.Type = TransitionSplit
		t.Direction = TransitionDirection(tr.Split.Dir)
		t.Orientation = TransitionOrientation(tr.Split.Orient)
	case tr.Cover != nil:
		t.Type, t.Direction = TransitionCover, TransitionDirection(tr.Cover.Dir)
	case tr.Pull != nil:
		t.Type, t.Direction = TransitionPull, TransitionDirection(tr.Pull.Dir)
	case tr.Strips != nil:
		t.Type, t.Direction = TransitionStrips, TransitionDirection(tr.Strips.Dir)
	case tr.Zoom != nil:
		t.Type, t.Direction = TransitionZoom, TransitionDirection(tr.Zoom.Dir)
	case tr.Dissolve != nil:
		t.Type = TransitionDissolve
	case tr.Blinds != nil:
		t.Type, t.Orientation = TransitionBlind, TransitionOrientation(tr.Blinds.Dir)
	case tr.Checker != nil:
		t.Type, t.Orientation = TransitionChecker, TransitionOrientation(tr.Checker.Dir)
	case tr.Comb != nil:
		t.Type, t.Orientation = TransitionComb, TransitionOrientation(tr.Comb.Dir)
	case tr.RandomBar != nil:
		t.Type, t.Orientation = TransitionRandomBar, TransitionOrientation(tr.RandomBar.Dir)
	case tr.Wheel != nil:
		t.Type, t.Spokes = TransitionWheel, int(tr.Wheel.Spokes)
	case tr.Random != nil:
		t.Type = TransitionRandom
	case tr.Diamond != nil:
		t.Type = TransitionDiamond
	case tr.Plus != nil:
		t.Type = TransitionPlus
	case tr.Circle != nil:
		t.Type = TransitionCircle
	case tr.Newsflash != nil:
		t.Type = TransitionNewsflash
	case tr.Wedge != nil:
		t.Type = TransitionWedge
	}

	t.Sound = soundActionFromOxml(tr.SndAc)

	return t
}

// transitionSpeed maps a duration in seconds to the coarse p:transition/@spd
// value (the base schema stores only fast/med/slow).
func transitionSpeed(duration float64) string {
	switch {
	case duration <= 0:
		return ""
	case duration <= 0.5:
		return "fast"
	case duration <= 1.0:
		return "med"
	default:
		return "slow"
	}
}

// buildMorphAlternateContent builds the mc:AlternateContent that carries a morph
// transition: an mc:Choice (Requires="p159") wrapping a p:transition whose child
// is <p159:morph>, and an mc:Fallback with a plain fade so pre-2016 PowerPoint
// still animates. The choice content declares xmlns:p159 (and xmlns:p14 for the
// exact-duration attribute) inline, since the p159 prefix is not one the generic
// AlternateContent marshaler declares from Requires.
func buildMorphAlternateContent(t Transition) *coxml.AlternateContent {
	option := t.MorphOption
	if option == "" {
		option = MorphByObject
	}

	spd := transitionSpeed(t.Duration)
	baseAttrs := ""
	if spd != "" {
		baseAttrs += ` spd="` + spd + `"`
	}
	// advClick is emitted explicitly so AdvanceOnClick=false round-trips.
	if t.AdvanceOnClick {
		baseAttrs += ` advClick="1"`
	} else {
		baseAttrs += ` advClick="0"`
	}
	if t.AdvanceAfter > 0 {
		baseAttrs += ` advTm="` + strconv.FormatUint(uint64(t.AdvanceAfter*1000), 10) + `"`
	}

	// p14:dur carries the exact morph duration in ms (the coarse spd cannot).
	durAttr := ""
	if t.Duration > 0 {
		durAttr = ` xmlns:p14="` + xmlb.NSPowerPoint2010 + `" p14:dur="` +
			strconv.FormatUint(uint64(t.Duration*1000), 10) + `"`
	}

	choice := `<p:transition` + durAttr + baseAttrs + `>` +
		`<p159:morph xmlns:p159="` + nsMorph + `" option="` + string(option) + `"/>` +
		`</p:transition>`
	fallback := `<p:transition` + baseAttrs + `><p:fade/></p:transition>`

	return &coxml.AlternateContent{
		Choices:     []coxml.AlternateContentChoice{{Requires: "p159", Content: []byte(choice)}},
		HasFallback: true,
		Fallback:    []byte(fallback),
	}
}

// isMorphAlternateContent reports whether an mc:AlternateContent carries a morph
// transition (used to drop a prior morph before setting a new transition).
func isMorphAlternateContent(ac *coxml.AlternateContent) bool {
	if ac == nil {
		return false
	}
	for _, choice := range ac.Choices {
		if bytes.Contains(choice.Content, []byte(":morph")) {
			return true
		}
	}
	return false
}

// morphFromAlternateContent finds a morph transition among the slide's
// root-level mc:AlternateContent elements and reports it as a Transition, or nil
// when none is present.
func morphFromAlternateContent(acs []*coxml.AlternateContent) *Transition {
	for _, ac := range acs {
		if ac == nil {
			continue
		}
		for _, choice := range ac.Choices {
			if !bytes.Contains(choice.Content, []byte(":morph")) {
				continue
			}
			t := &Transition{Type: TransitionMorph, MorphOption: MorphByObject, Duration: 1.0, AdvanceOnClick: true}
			if opt := attrValueFromRaw(choice.Content, "option"); opt != "" {
				t.MorphOption = MorphOption(opt)
			}
			if dur := attrValueFromRaw(choice.Content, "p14:dur"); dur != "" {
				if ms, err := strconv.Atoi(dur); err == nil && ms > 0 {
					t.Duration = float64(ms) / 1000.0
				}
			}
			if attrValueFromRaw(choice.Content, "advClick") == "0" {
				t.AdvanceOnClick = false
			}
			if adv := attrValueFromRaw(choice.Content, "advTm"); adv != "" {
				if ms, err := strconv.Atoi(adv); err == nil && ms > 0 {
					t.AdvanceAfter = float64(ms) / 1000.0
				}
			}
			return t
		}
	}
	return nil
}

// attrValueFromRaw extracts the value of the named attribute (e.g. `option` or
// `p14:dur`) from a raw XML fragment. It is a small helper for reading back the
// handful of attributes on a synthesized morph transition; it matches the first
// `name="..."` occurrence.
func attrValueFromRaw(raw []byte, name string) string {
	needle := []byte(name + `="`)
	i := bytes.Index(raw, needle)
	if i < 0 {
		return ""
	}
	rest := raw[i+len(needle):]
	end := bytes.IndexByte(rest, '"')
	if end < 0 {
		return ""
	}
	return string(rest[:end])
}

// soundActionFromOxml reads a p:sndAc into the public sound settings, or nil.
func soundActionFromOxml(ac *oxml.TransitionSoundAction) *TransitionSound {
	if ac == nil {
		return nil
	}
	snd := &TransitionSound{StopPreviousSound: ac.EndSnd != nil}
	if ac.StSnd != nil {
		snd.StartSoundLoop = ac.StSnd.Loop
		if ac.StSnd.Snd != nil {
			snd.StartSoundName = ac.StSnd.Snd.Name
			snd.startSoundEmbed = ac.StSnd.Snd.Embed
		}
	}
	return snd
}
