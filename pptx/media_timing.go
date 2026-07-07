package pptx

import (
	"strconv"

	"github.com/mgilbir/spine/pptx/internal/oxml"
)

// mediaTimingRef identifies an auto-play media shape by its shape id and kind.
type mediaTimingRef struct {
	spid uint32
	kind mediaKind
}

// applyMediaTiming installs an auto-play timing tree for media added to the
// slide. It is a no-op when nothing auto-plays, or when the slide already has a
// timing tree (building one would clobber existing animations), in which case
// the media still plays on click.
func (s *Slide) applyMediaTiming() {
	refs := s.autoplayMedia
	s.autoplayMedia = nil
	if len(refs) == 0 || s.slideXML == nil || s.slideXML.Timing != nil {
		return
	}
	s.slideXML.Timing = buildAutoplayTiming(refs)
}

func i32p(v int32) *int32 { return &v }

func spTgt(spid uint32) *oxml.TargetElement {
	return &oxml.TargetElement{SpTgt: &oxml.ShapeTarget{SpId: strconv.FormatUint(uint64(spid), 10)}}
}

func sldTgt() *oxml.TargetElement {
	return &oxml.TargetElement{SldTgt: &oxml.SlideTarget{}}
}

func condDelay(delay string) *oxml.ConditionList {
	return &oxml.ConditionList{Cond: []*oxml.Condition{{Delay: delay}}}
}

func slideEventCond(evt string) *oxml.ConditionList {
	return &oxml.ConditionList{Cond: []*oxml.Condition{{Evt: evt, Delay: "0", TgtEl: sldTgt()}}}
}

// buildAutoplayTiming builds a p:timing tree that auto-plays the given media
// when the slide appears. The structure mirrors what PowerPoint emits for media
// set to "Start: Automatically": a main sequence whose mediacall command issues
// playFrom(0.0) against each media shape, plus a p:video/p:audio media node per
// shape. Time-node ids are independent of shape ids.
func buildAutoplayTiming(refs []mediaTimingRef) *oxml.Timing {
	var next uint32
	id := func() uint32 { next++; return next }

	rootID := id()     // tmRoot
	mainSeqID := id()  // mainSeq
	clickParID := id() // click group (waits: delay indefinite)
	withParID := id()  // with-previous group (delay 0)

	// One mediacall effect per media, nested under the with-previous group.
	effChild := &oxml.TimeNodeList{}
	for i, ref := range refs {
		nodeType := "withEffect"
		if i == 0 {
			nodeType = "afterEffect"
		}
		effID := id()
		bhvrID := id()

		cmdChild := &oxml.TimeNodeList{}
		cmdChild.AppendCmd(&oxml.Command{
			Type: "call",
			Cmd:  "playFrom(0.0)",
			CBhvr: &oxml.CommonBehavior{
				CTn:   &oxml.CommonTimeNode{Id: bhvrID, Fill: "hold"},
				TgtEl: spTgt(ref.spid),
			},
		})
		effChild.AppendPar(&oxml.ParallelTimeNode{CTn: &oxml.CommonTimeNode{
			Id:            effID,
			PresetClass:   "mediacall",
			PresetSubtype: i32p(0),
			Fill:          "hold",
			NodeType:      nodeType,
			StCondLst:     condDelay("0"),
			ChildTnLst:    cmdChild,
		}})
	}

	withChild := &oxml.TimeNodeList{}
	withChild.AppendPar(&oxml.ParallelTimeNode{CTn: &oxml.CommonTimeNode{
		Id: withParID, Fill: "hold", StCondLst: condDelay("0"), ChildTnLst: effChild,
	}})

	clickChild := &oxml.TimeNodeList{}
	clickChild.AppendPar(&oxml.ParallelTimeNode{CTn: &oxml.CommonTimeNode{
		Id: clickParID, Fill: "hold", StCondLst: condDelay("indefinite"), ChildTnLst: withChild,
	}})

	seq := &oxml.SequenceTimeNode{
		Concurrent:  true,
		NextAc:      "seek",
		CTn:         &oxml.CommonTimeNode{Id: mainSeqID, Dur: "indefinite", NodeType: "mainSeq", ChildTnLst: clickChild},
		PrevCondLst: slideEventCond("onPrev"),
		NextCondLst: slideEventCond("onNext"),
	}

	// tmRoot children: the main sequence followed by a media node per shape.
	rootChild := &oxml.TimeNodeList{}
	rootChild.AppendSeq(seq)
	for _, ref := range refs {
		cmn := &oxml.CommonMediaNode{
			CTn:   &oxml.CommonTimeNode{Id: id(), Fill: "hold", StCondLst: condDelay("indefinite")},
			TgtEl: spTgt(ref.spid),
		}
		if ref.kind == mediaAudio {
			rootChild.AppendAudio(&oxml.Audio{CMediaNode: cmn})
		} else {
			rootChild.AppendVideo(&oxml.Video{CMediaNode: cmn})
		}
	}

	tnLst := &oxml.TimeNodeList{}
	tnLst.AppendPar(&oxml.ParallelTimeNode{CTn: &oxml.CommonTimeNode{
		Id: rootID, Dur: "indefinite", Restart: "never", NodeType: "tmRoot", ChildTnLst: rootChild,
	}})

	return &oxml.Timing{TnLst: tnLst}
}
