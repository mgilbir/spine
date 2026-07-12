// Package oxml provides PresentationML animation types from pml.xsd.
// These types implement the p: namespace animation and timing elements.
package oxml

import (
	"encoding/xml"

	"github.com/mgilbir/spine/common/dml"
	xmlb "github.com/mgilbir/spine/common/xml"
)

// --- Slide Timing (p:timing) ---

// Timing represents CT_SlideTiming (p:timing)
type Timing struct {
	TnLst  *TimeNodeList `xml:"http://schemas.openxmlformats.org/presentationml/2006/main tnLst,omitempty"`
	BldLst *BuildList    `xml:"http://schemas.openxmlformats.org/presentationml/2006/main bldLst,omitempty"`
	ExtLst *ExtensionList   `xml:"http://schemas.openxmlformats.org/presentationml/2006/main extLst,omitempty"`
}

// TimeNodeList represents CT_TimeNodeList (p:tnLst, p:childTnLst, p:subTnLst).
// Uses custom UnmarshalXML/MarshalToBuilder to preserve xs:choice element ordering.
type TimeNodeList struct {
	Par        []*ParallelTimeNode  `xml:"-"`
	Seq        []*SequenceTimeNode  `xml:"-"`
	Excl       []*ExclusiveTimeNode `xml:"-"`
	Anim       []*Animate           `xml:"-"`
	AnimClr    []*AnimateColor      `xml:"-"`
	AnimEffect []*AnimateEffect     `xml:"-"`
	AnimMotion []*AnimateMotion     `xml:"-"`
	AnimRot    []*AnimateRotation   `xml:"-"`
	AnimScale  []*AnimateScale      `xml:"-"`
	Cmd        []*Command           `xml:"-"`
	Set        []*Set               `xml:"-"`
	Audio      []*Audio             `xml:"-"`
	Video      []*Video             `xml:"-"`
	childOrder []tnlChildRef
}

type tnlChildKind int

const (
	tnlPar tnlChildKind = iota
	tnlSeq
	tnlExcl
	tnlAnim
	tnlAnimClr
	tnlAnimEffect
	tnlAnimMotion
	tnlAnimRot
	tnlAnimScale
	tnlCmd
	tnlSet
	tnlAudio
	tnlVideo
)

type tnlChildRef struct {
	kind  tnlChildKind
	index int
}

var tnlNameMap = map[string]tnlChildKind{
	"par":        tnlPar,
	"seq":        tnlSeq,
	"excl":       tnlExcl,
	"anim":       tnlAnim,
	"animClr":    tnlAnimClr,
	"animEffect": tnlAnimEffect,
	"animMotion": tnlAnimMotion,
	"animRot":    tnlAnimRot,
	"animScale":  tnlAnimScale,
	"cmd":        tnlCmd,
	"set":        tnlSet,
	"audio":      tnlAudio,
	"video":      tnlVideo,
}

// AppendPar adds a parallel time node, maintaining child order so it is
// marshaled (the list serializes strictly by child order).
func (tnl *TimeNodeList) AppendPar(p *ParallelTimeNode) {
	tnl.childOrder = append(tnl.childOrder, tnlChildRef{tnlPar, len(tnl.Par)})
	tnl.Par = append(tnl.Par, p)
}

// AppendSeq adds a sequence time node, maintaining child order.
func (tnl *TimeNodeList) AppendSeq(s *SequenceTimeNode) {
	tnl.childOrder = append(tnl.childOrder, tnlChildRef{tnlSeq, len(tnl.Seq)})
	tnl.Seq = append(tnl.Seq, s)
}

// AppendCmd adds a command node, maintaining child order.
func (tnl *TimeNodeList) AppendCmd(c *Command) {
	tnl.childOrder = append(tnl.childOrder, tnlChildRef{tnlCmd, len(tnl.Cmd)})
	tnl.Cmd = append(tnl.Cmd, c)
}

// AppendVideo adds a video media node, maintaining child order.
func (tnl *TimeNodeList) AppendVideo(v *Video) {
	tnl.childOrder = append(tnl.childOrder, tnlChildRef{tnlVideo, len(tnl.Video)})
	tnl.Video = append(tnl.Video, v)
}

// AppendAudio adds an audio media node, maintaining child order.
func (tnl *TimeNodeList) AppendAudio(a *Audio) {
	tnl.childOrder = append(tnl.childOrder, tnlChildRef{tnlAudio, len(tnl.Audio)})
	tnl.Audio = append(tnl.Audio, a)
}

func (tnl *TimeNodeList) UnmarshalXML(d *xml.Decoder, start xml.StartElement) error {
	for {
		tok, err := d.Token()
		if err != nil {
			return err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			kind, ok := tnlNameMap[t.Name.Local]
			if !ok {
				if err := d.Skip(); err != nil {
					return err
				}
				continue
			}
			switch kind {
			case tnlPar:
				var v ParallelTimeNode
				if err := d.DecodeElement(&v, &t); err != nil {
					return err
				}
				tnl.childOrder = append(tnl.childOrder, tnlChildRef{tnlPar, len(tnl.Par)})
				tnl.Par = append(tnl.Par, &v)
			case tnlSeq:
				var v SequenceTimeNode
				if err := d.DecodeElement(&v, &t); err != nil {
					return err
				}
				tnl.childOrder = append(tnl.childOrder, tnlChildRef{tnlSeq, len(tnl.Seq)})
				tnl.Seq = append(tnl.Seq, &v)
			case tnlExcl:
				var v ExclusiveTimeNode
				if err := d.DecodeElement(&v, &t); err != nil {
					return err
				}
				tnl.childOrder = append(tnl.childOrder, tnlChildRef{tnlExcl, len(tnl.Excl)})
				tnl.Excl = append(tnl.Excl, &v)
			case tnlAnim:
				var v Animate
				if err := d.DecodeElement(&v, &t); err != nil {
					return err
				}
				tnl.childOrder = append(tnl.childOrder, tnlChildRef{tnlAnim, len(tnl.Anim)})
				tnl.Anim = append(tnl.Anim, &v)
			case tnlAnimClr:
				var v AnimateColor
				if err := d.DecodeElement(&v, &t); err != nil {
					return err
				}
				tnl.childOrder = append(tnl.childOrder, tnlChildRef{tnlAnimClr, len(tnl.AnimClr)})
				tnl.AnimClr = append(tnl.AnimClr, &v)
			case tnlAnimEffect:
				var v AnimateEffect
				if err := d.DecodeElement(&v, &t); err != nil {
					return err
				}
				tnl.childOrder = append(tnl.childOrder, tnlChildRef{tnlAnimEffect, len(tnl.AnimEffect)})
				tnl.AnimEffect = append(tnl.AnimEffect, &v)
			case tnlAnimMotion:
				var v AnimateMotion
				if err := d.DecodeElement(&v, &t); err != nil {
					return err
				}
				tnl.childOrder = append(tnl.childOrder, tnlChildRef{tnlAnimMotion, len(tnl.AnimMotion)})
				tnl.AnimMotion = append(tnl.AnimMotion, &v)
			case tnlAnimRot:
				var v AnimateRotation
				if err := d.DecodeElement(&v, &t); err != nil {
					return err
				}
				tnl.childOrder = append(tnl.childOrder, tnlChildRef{tnlAnimRot, len(tnl.AnimRot)})
				tnl.AnimRot = append(tnl.AnimRot, &v)
			case tnlAnimScale:
				var v AnimateScale
				if err := d.DecodeElement(&v, &t); err != nil {
					return err
				}
				tnl.childOrder = append(tnl.childOrder, tnlChildRef{tnlAnimScale, len(tnl.AnimScale)})
				tnl.AnimScale = append(tnl.AnimScale, &v)
			case tnlCmd:
				var v Command
				if err := d.DecodeElement(&v, &t); err != nil {
					return err
				}
				tnl.childOrder = append(tnl.childOrder, tnlChildRef{tnlCmd, len(tnl.Cmd)})
				tnl.Cmd = append(tnl.Cmd, &v)
			case tnlSet:
				var v Set
				if err := d.DecodeElement(&v, &t); err != nil {
					return err
				}
				tnl.childOrder = append(tnl.childOrder, tnlChildRef{tnlSet, len(tnl.Set)})
				tnl.Set = append(tnl.Set, &v)
			case tnlAudio:
				var v Audio
				if err := d.DecodeElement(&v, &t); err != nil {
					return err
				}
				tnl.childOrder = append(tnl.childOrder, tnlChildRef{tnlAudio, len(tnl.Audio)})
				tnl.Audio = append(tnl.Audio, &v)
			case tnlVideo:
				var v Video
				if err := d.DecodeElement(&v, &t); err != nil {
					return err
				}
				tnl.childOrder = append(tnl.childOrder, tnlChildRef{tnlVideo, len(tnl.Video)})
				tnl.Video = append(tnl.Video, &v)
			}
		case xml.EndElement:
			return nil
		}
	}
}

func (tnl *TimeNodeList) MarshalXML(e *xml.Encoder, start xml.StartElement) error {
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	nsP := xml.Name{Space: xmlb.NSPresentationML}
	for _, ref := range tnl.childOrder {
		switch ref.kind {
		case tnlPar:
			if err := e.EncodeElement(tnl.Par[ref.index], xml.StartElement{Name: xml.Name{Local: "par", Space: nsP.Space}}); err != nil {
				return err
			}
		case tnlSeq:
			if err := e.EncodeElement(tnl.Seq[ref.index], xml.StartElement{Name: xml.Name{Local: "seq", Space: nsP.Space}}); err != nil {
				return err
			}
		case tnlExcl:
			if err := e.EncodeElement(tnl.Excl[ref.index], xml.StartElement{Name: xml.Name{Local: "excl", Space: nsP.Space}}); err != nil {
				return err
			}
		case tnlAnim:
			if err := e.EncodeElement(tnl.Anim[ref.index], xml.StartElement{Name: xml.Name{Local: "anim", Space: nsP.Space}}); err != nil {
				return err
			}
		case tnlAnimClr:
			if err := e.EncodeElement(tnl.AnimClr[ref.index], xml.StartElement{Name: xml.Name{Local: "animClr", Space: nsP.Space}}); err != nil {
				return err
			}
		case tnlAnimEffect:
			if err := e.EncodeElement(tnl.AnimEffect[ref.index], xml.StartElement{Name: xml.Name{Local: "animEffect", Space: nsP.Space}}); err != nil {
				return err
			}
		case tnlAnimMotion:
			if err := e.EncodeElement(tnl.AnimMotion[ref.index], xml.StartElement{Name: xml.Name{Local: "animMotion", Space: nsP.Space}}); err != nil {
				return err
			}
		case tnlAnimRot:
			if err := e.EncodeElement(tnl.AnimRot[ref.index], xml.StartElement{Name: xml.Name{Local: "animRot", Space: nsP.Space}}); err != nil {
				return err
			}
		case tnlAnimScale:
			if err := e.EncodeElement(tnl.AnimScale[ref.index], xml.StartElement{Name: xml.Name{Local: "animScale", Space: nsP.Space}}); err != nil {
				return err
			}
		case tnlCmd:
			if err := e.EncodeElement(tnl.Cmd[ref.index], xml.StartElement{Name: xml.Name{Local: "cmd", Space: nsP.Space}}); err != nil {
				return err
			}
		case tnlSet:
			if err := e.EncodeElement(tnl.Set[ref.index], xml.StartElement{Name: xml.Name{Local: "set", Space: nsP.Space}}); err != nil {
				return err
			}
		case tnlAudio:
			if err := e.EncodeElement(tnl.Audio[ref.index], xml.StartElement{Name: xml.Name{Local: "audio", Space: nsP.Space}}); err != nil {
				return err
			}
		case tnlVideo:
			if err := e.EncodeElement(tnl.Video[ref.index], xml.StartElement{Name: xml.Name{Local: "video", Space: nsP.Space}}); err != nil {
				return err
			}
		}
	}
	return e.EncodeToken(start.End())
}

// MarshalToBuilder writes the TimeNodeList preserving child element order.
func (tnl *TimeNodeList) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	if len(tnl.childOrder) == 0 {
		b.EmptyElement(ns, localName)
		return
	}
	b.StartElement(ns, localName)
	for _, ref := range tnl.childOrder {
		switch ref.kind {
		case tnlPar:
			b.MarshalElement(ns, "par", tnl.Par[ref.index])
		case tnlSeq:
			b.MarshalElement(ns, "seq", tnl.Seq[ref.index])
		case tnlExcl:
			b.MarshalElement(ns, "excl", tnl.Excl[ref.index])
		case tnlAnim:
			b.MarshalElement(ns, "anim", tnl.Anim[ref.index])
		case tnlAnimClr:
			b.MarshalElement(ns, "animClr", tnl.AnimClr[ref.index])
		case tnlAnimEffect:
			b.MarshalElement(ns, "animEffect", tnl.AnimEffect[ref.index])
		case tnlAnimMotion:
			b.MarshalElement(ns, "animMotion", tnl.AnimMotion[ref.index])
		case tnlAnimRot:
			b.MarshalElement(ns, "animRot", tnl.AnimRot[ref.index])
		case tnlAnimScale:
			b.MarshalElement(ns, "animScale", tnl.AnimScale[ref.index])
		case tnlCmd:
			b.MarshalElement(ns, "cmd", tnl.Cmd[ref.index])
		case tnlSet:
			b.MarshalElement(ns, "set", tnl.Set[ref.index])
		case tnlAudio:
			b.MarshalElement(ns, "audio", tnl.Audio[ref.index])
		case tnlVideo:
			b.MarshalElement(ns, "video", tnl.Video[ref.index])
		}
	}
	b.EndElement(ns, localName)
}

// --- Time Node Types ---

// ParallelTimeNode represents CT_TLTimeNodeParallel (p:par)
type ParallelTimeNode struct {
	CTn *CommonTimeNode `xml:"http://schemas.openxmlformats.org/presentationml/2006/main cTn,omitempty"`
}

// SequenceTimeNode represents CT_TLTimeNodeSequence (p:seq)
type SequenceTimeNode struct {
	Concurrent bool   `xml:"concurrent,attr,omitempty"`
	PrevAc     string `xml:"prevAc,attr,omitempty"` // none, skipTimed
	NextAc     string `xml:"nextAc,attr,omitempty"` // none, seek
	CTn        *CommonTimeNode `xml:"http://schemas.openxmlformats.org/presentationml/2006/main cTn,omitempty"`
	PrevCondLst *ConditionList `xml:"http://schemas.openxmlformats.org/presentationml/2006/main prevCondLst,omitempty"`
	NextCondLst *ConditionList `xml:"http://schemas.openxmlformats.org/presentationml/2006/main nextCondLst,omitempty"`
}

// ExclusiveTimeNode represents CT_TLTimeNodeExclusive (p:excl)
type ExclusiveTimeNode struct {
	CTn *CommonTimeNode `xml:"http://schemas.openxmlformats.org/presentationml/2006/main cTn,omitempty"`
}

// CommonTimeNode represents CT_TLCommonTimeNodeData (p:cTn)
type CommonTimeNode struct {
	Id             uint32          `xml:"id,attr,omitempty"`
	Presetid       *int32          `xml:"presetID,attr,omitempty"`
	PresetClass    string          `xml:"presetClass,attr,omitempty"` // entr, exit, emph, path, verb, mediacall
	PresetSubtype  *int32          `xml:"presetSubtype,attr,omitempty"`
	Dur            string          `xml:"dur,attr,omitempty"` // indefinite, or time in ms
	RepeatCount    string          `xml:"repeatCount,attr,omitempty"`
	RepeatDur      string          `xml:"repeatDur,attr,omitempty"`
	Spd            string          `xml:"spd,attr,omitempty"` // percentage
	Accel          string          `xml:"accel,attr,omitempty"`
	Decel          string          `xml:"decel,attr,omitempty"`
	// AutoRev, Display, AfterEffect, and NodePh have no XSD default, so an
	// explicit "0" must round-trip on the always-remarshaled timing path
	// instead of being deleted (C224).
	AutoRev        *bool           `xml:"autoRev,attr,omitempty"`
	Restart        string          `xml:"restart,attr,omitempty"` // always, whenNotActive, never
	Fill           string          `xml:"fill,attr,omitempty"`    // remove, freeze, hold, transition
	SyncBehavior   string          `xml:"syncBehavior,attr,omitempty"`
	TmFilter       string          `xml:"tmFilter,attr,omitempty"`
	EvtFilter      string          `xml:"evtFilter,attr,omitempty"`
	Display        *bool           `xml:"display,attr,omitempty"`
	MasterRel      string          `xml:"masterRel,attr,omitempty"` // sameClick, lastClick, nextClick
	BldLvl         int32           `xml:"bldLvl,attr,omitempty"`
	GrpId          *uint32         `xml:"grpId,attr,omitempty"`
	AfterEffect    *bool           `xml:"afterEffect,attr,omitempty"`
	NodeType       string          `xml:"nodeType,attr,omitempty"` // clickEffect, withEffect, afterEffect, mainSeq, interactiveSeq, clickPar, withGroup, afterGroup, tmRoot
	NodePh         *bool           `xml:"nodePh,attr,omitempty"`
	StCondLst      *ConditionList  `xml:"http://schemas.openxmlformats.org/presentationml/2006/main stCondLst,omitempty"`
	EndCondLst     *ConditionList  `xml:"http://schemas.openxmlformats.org/presentationml/2006/main endCondLst,omitempty"`
	EndSync        *Condition      `xml:"http://schemas.openxmlformats.org/presentationml/2006/main endSync,omitempty"`
	Iterate        *Iterate        `xml:"http://schemas.openxmlformats.org/presentationml/2006/main iterate,omitempty"`
	ChildTnLst     *TimeNodeList   `xml:"http://schemas.openxmlformats.org/presentationml/2006/main childTnLst,omitempty"`
	SubTnLst       *TimeNodeList   `xml:"http://schemas.openxmlformats.org/presentationml/2006/main subTnLst,omitempty"`
}

// --- Conditions ---

// ConditionList represents CT_TLTimeConditionList
type ConditionList struct {
	Cond []*Condition `xml:"http://schemas.openxmlformats.org/presentationml/2006/main cond,omitempty"`
}

// Condition represents CT_TLTimeCondition (p:cond)
type Condition struct {
	Evt   string       `xml:"evt,attr,omitempty"`   // onBegin, onEnd, begin, end, onClick, onDblClick, onMouseOver, onMouseOut, onNext, onPrev, onStopAudio
	Delay string       `xml:"delay,attr,omitempty"` // indefinite or time in ms
	TgtEl *TargetElement `xml:"http://schemas.openxmlformats.org/presentationml/2006/main tgtEl,omitempty"`
	Tn    *TimeNode    `xml:"http://schemas.openxmlformats.org/presentationml/2006/main tn,omitempty"`
	Rtn   *RuntimeNode `xml:"http://schemas.openxmlformats.org/presentationml/2006/main rtn,omitempty"`
}

// TimeNode represents CT_TLTimeNodeId (p:tn)
type TimeNode struct {
	Val uint32 `xml:"val,attr"`
}

// RuntimeNode represents CT_TLTriggerRuntimeNode (p:rtn)
type RuntimeNode struct {
	Val string `xml:"val,attr"` // first, last, all
}

// TargetElement represents CT_TLTimeTargetElement (p:tgtEl)
type TargetElement struct {
	SldTgt   *SlideTarget   `xml:"http://schemas.openxmlformats.org/presentationml/2006/main sldTgt,omitempty"`
	SndTgt   *SoundTarget   `xml:"http://schemas.openxmlformats.org/presentationml/2006/main sndTgt,omitempty"`
	SpTgt    *ShapeTarget   `xml:"http://schemas.openxmlformats.org/presentationml/2006/main spTgt,omitempty"`
	InkTgt   *InkTarget     `xml:"http://schemas.openxmlformats.org/presentationml/2006/main inkTgt,omitempty"`
}

// SlideTarget represents CT_Empty (p:sldTgt)
type SlideTarget struct{}

// SoundTarget represents CT_TLSoundTarget (p:sndTgt) - reference to embedded WAV
type SoundTarget struct {
	Embed string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships embed,attr,omitempty"`
	Name  string `xml:"name,attr,omitempty"`
}

// ShapeTarget represents CT_TLShapeTargetElement (p:spTgt)
type ShapeTarget struct {
	SpId   string      `xml:"spid,attr"`
	Bg     *AnimTargetBg `xml:"http://schemas.openxmlformats.org/presentationml/2006/main bg,omitempty"`
	SubSp  *SubShape   `xml:"http://schemas.openxmlformats.org/presentationml/2006/main subSp,omitempty"`
	OleChartEl *OleChartElement `xml:"http://schemas.openxmlformats.org/presentationml/2006/main oleChartEl,omitempty"`
	TxEl   *TextElement `xml:"http://schemas.openxmlformats.org/presentationml/2006/main txEl,omitempty"`
	GraphicEl *GraphicElement `xml:"http://schemas.openxmlformats.org/presentationml/2006/main graphicEl,omitempty"`
}

// AnimTargetBg represents CT_Empty (p:bg) used as animation shape target background.
type AnimTargetBg struct{}

// SubShape represents CT_TLSubShapeId (p:subSp)
type SubShape struct {
	SpId string `xml:"spid,attr"`
}

// OleChartElement represents CT_TLOleChartTargetElement (p:oleChartEl)
type OleChartElement struct {
	Type  string `xml:"type,attr"` // embed, link
	Lvl   int32  `xml:"lvl,attr,omitempty"`
}

// TextElement represents CT_TLTextTargetElement (p:txEl)
type TextElement struct {
	CharRg *IndexRange `xml:"http://schemas.openxmlformats.org/presentationml/2006/main charRg,omitempty"`
	PRg    *IndexRange `xml:"http://schemas.openxmlformats.org/presentationml/2006/main pRg,omitempty"`
}

// GraphicElement represents a:CT_AnimationElementChoice (p:graphicEl).
// Its children live in the DrawingML namespace (a:dgm/a:chart), not the
// PresentationML one, and carry element-target types — not the build-property
// types this type previously (incorrectly) used (C34).
type GraphicElement struct {
	Dgm   *AnimationDgmElement   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main dgm,omitempty"`
	Chart *AnimationChartElement `xml:"http://schemas.openxmlformats.org/drawingml/2006/main chart,omitempty"`
}

// AnimationDgmElement represents a:CT_AnimationDgmElement (a:dgm)
type AnimationDgmElement struct {
	Id      string `xml:"id,attr,omitempty"`      // GUID, defaults to the zero GUID
	BldStep string `xml:"bldStep,attr,omitempty"` // sp, bg
}

// AnimationChartElement represents a:CT_AnimationChartElement (a:chart).
// seriesIdx/categoryIdx default to -1, so explicit zeros must round-trip.
type AnimationChartElement struct {
	SeriesIdx   *int32 `xml:"seriesIdx,attr,omitempty"`
	CategoryIdx *int32 `xml:"categoryIdx,attr,omitempty"`
	BldStep     string `xml:"bldStep,attr"` // category, categoryEl, series, seriesEl, allPts, gridLegend
}

// AnimationChartBuildProperties represents a:CT_AnimationChartBuildProperties
// (a:bldChart). animBg defaults to true, so an explicit false must be emitted.
type AnimationChartBuildProperties struct {
	Bld    string `xml:"bld,attr,omitempty"` // allAtOnce, series, category, seriesEl, categoryEl
	AnimBg *bool  `xml:"animBg,attr,omitempty"`
}

// AnimationDgmBuildProperties represents a:CT_AnimationDgmBuildProperties (a:bldDgm)
type AnimationDgmBuildProperties struct {
	Bld string `xml:"bld,attr,omitempty"` // allAtOnce, one, lvlOne, lvlAtOnce
	Rev bool   `xml:"rev,attr,omitempty"`
}

// InkTarget represents CT_TLInkTargetElement (p:inkTgt)
type InkTarget struct {
	SpId string `xml:"spid,attr"`
}

// IndexRange represents CT_IndexRange (p:charRg, p:pRg)
type IndexRange struct {
	St  uint32 `xml:"st,attr"`
	End uint32 `xml:"end,attr"`
}

// Iterate represents CT_TLIterateData (p:iterate)
type Iterate struct {
	Type     string `xml:"type,attr,omitempty"` // el, wd, lt
	Backwards bool  `xml:"backwards,attr,omitempty"`
	TmAbs    *TimeAbsolute `xml:"http://schemas.openxmlformats.org/presentationml/2006/main tmAbs,omitempty"`
	TmPct    *TimePercent  `xml:"http://schemas.openxmlformats.org/presentationml/2006/main tmPct,omitempty"`
}

// TimeAbsolute represents CT_TLIterateIntervalTime (p:tmAbs)
type TimeAbsolute struct {
	Val uint32 `xml:"val,attr"` // time in ms
}

// TimePercent represents CT_TLIterateIntervalPercentage (p:tmPct)
type TimePercent struct {
	Val string `xml:"val,attr"` // percentage in 1000ths or "N%"
}

// --- Animation Behaviors ---

// Animate represents CT_TLAnimateBehavior (p:anim)
type Animate struct {
	By         string `xml:"by,attr,omitempty"`
	From       string `xml:"from,attr,omitempty"`
	To         string `xml:"to,attr,omitempty"`
	CalcMode   string `xml:"calcmode,attr,omitempty"` // discrete, lin, fmla
	ValueType  string `xml:"valueType,attr,omitempty"` // str, num, clr
	CBhvr      *CommonBehavior `xml:"http://schemas.openxmlformats.org/presentationml/2006/main cBhvr,omitempty"`
	TavLst     *TimeAnimateValueList `xml:"http://schemas.openxmlformats.org/presentationml/2006/main tavLst,omitempty"`
}

// AnimateColor represents CT_TLAnimateColorBehavior (p:animClr)
type AnimateColor struct {
	ClrSpc   string `xml:"clrSpc,attr,omitempty"` // rgb, hsl
	Dir      string `xml:"dir,attr,omitempty"`    // cw, ccw
	CBhvr    *CommonBehavior `xml:"http://schemas.openxmlformats.org/presentationml/2006/main cBhvr,omitempty"`
	By       *ByAnimateColor `xml:"http://schemas.openxmlformats.org/presentationml/2006/main by,omitempty"`
	From     *dml.ColorChoice `xml:"http://schemas.openxmlformats.org/presentationml/2006/main from,omitempty"`
	To       *dml.ColorChoice `xml:"http://schemas.openxmlformats.org/presentationml/2006/main to,omitempty"`
}

// ByAnimateColor represents CT_TLByAnimateColorTransform (p:by)
type ByAnimateColor struct {
	Rgb *ByRgbColor `xml:"http://schemas.openxmlformats.org/presentationml/2006/main rgb,omitempty"`
	Hsl *ByHslColor `xml:"http://schemas.openxmlformats.org/presentationml/2006/main hsl,omitempty"`
}

// ByRgbColor represents CT_TLByRgbColorTransform (p:rgb)
type ByRgbColor struct {
	R int32 `xml:"r,attr"` // -100000 to 100000
	G int32 `xml:"g,attr"`
	B int32 `xml:"b,attr"`
}

// ByHslColor represents CT_TLByHslColorTransform (p:hsl)
type ByHslColor struct {
	H int32 `xml:"h,attr"` // angle in 60000ths of degree
	S int32 `xml:"s,attr"` // -100000 to 100000
	L int32 `xml:"l,attr"` // -100000 to 100000
}

// AnimateEffect represents CT_TLAnimateEffectBehavior (p:animEffect)
type AnimateEffect struct {
	Transition string `xml:"transition,attr,omitempty"` // in, out, none
	Filter     string `xml:"filter,attr,omitempty"`
	PrLst      string `xml:"prLst,attr,omitempty"`
	CBhvr      *CommonBehavior `xml:"http://schemas.openxmlformats.org/presentationml/2006/main cBhvr,omitempty"`
	// Progress is CT_TLAnimVariant per the XSD (boolVal/intVal/fltVal/strVal/
	// clrVal choice), not a tavLst container (C34).
	Progress *AnimVariant `xml:"http://schemas.openxmlformats.org/presentationml/2006/main progress,omitempty"`
}

// AnimateMotion represents CT_TLAnimateMotionBehavior (p:animMotion)
type AnimateMotion struct {
	Origin       string `xml:"origin,attr,omitempty"` // parent, layout
	Path         string `xml:"path,attr,omitempty"`
	PathEditMode string `xml:"pathEditMode,attr,omitempty"` // relative, fixed
	RAng         int32  `xml:"rAng,attr,omitempty"`
	PtsTypes     string `xml:"ptsTypes,attr,omitempty"`
	CBhvr        *CommonBehavior `xml:"http://schemas.openxmlformats.org/presentationml/2006/main cBhvr,omitempty"`
	By           *Point `xml:"http://schemas.openxmlformats.org/presentationml/2006/main by,omitempty"`
	From         *Point `xml:"http://schemas.openxmlformats.org/presentationml/2006/main from,omitempty"`
	To           *Point `xml:"http://schemas.openxmlformats.org/presentationml/2006/main to,omitempty"`
	RCtr         *Point `xml:"http://schemas.openxmlformats.org/presentationml/2006/main rCtr,omitempty"`
}

// Point represents CT_TLPoint (p:by, p:from, p:to, p:rCtr)
type Point struct {
	X string `xml:"x,attr"` // percentage in 1000ths or "N%"
	Y string `xml:"y,attr"`
}

// AnimateRotation represents CT_TLAnimateRotationBehavior (p:animRot)
type AnimateRotation struct {
	By    int32  `xml:"by,attr,omitempty"`   // angle in 60000ths
	From  int32  `xml:"from,attr,omitempty"`
	To    int32  `xml:"to,attr,omitempty"`
	CBhvr *CommonBehavior `xml:"http://schemas.openxmlformats.org/presentationml/2006/main cBhvr,omitempty"`
}

// AnimateScale represents CT_TLAnimateScaleBehavior (p:animScale)
type AnimateScale struct {
	ZoomContents bool `xml:"zoomContents,attr,omitempty"`
	CBhvr        *CommonBehavior `xml:"http://schemas.openxmlformats.org/presentationml/2006/main cBhvr,omitempty"`
	By           *Point `xml:"http://schemas.openxmlformats.org/presentationml/2006/main by,omitempty"`
	From         *Point `xml:"http://schemas.openxmlformats.org/presentationml/2006/main from,omitempty"`
	To           *Point `xml:"http://schemas.openxmlformats.org/presentationml/2006/main to,omitempty"`
}

// Set represents CT_TLSetBehavior (p:set)
type Set struct {
	CBhvr *CommonBehavior `xml:"http://schemas.openxmlformats.org/presentationml/2006/main cBhvr,omitempty"`
	To    *AnimVariant    `xml:"http://schemas.openxmlformats.org/presentationml/2006/main to,omitempty"`
}

// Command represents CT_TLCommandBehavior (p:cmd)
type Command struct {
	Type  string `xml:"type,attr,omitempty"` // evt, call, verb
	Cmd   string `xml:"cmd,attr,omitempty"`
	CBhvr *CommonBehavior `xml:"http://schemas.openxmlformats.org/presentationml/2006/main cBhvr,omitempty"`
}

// Audio represents CT_TLMediaNodeAudio (p:audio)
type Audio struct {
	IsNarration bool `xml:"isNarration,attr,omitempty"`
	CMediaNode  *CommonMediaNode `xml:"http://schemas.openxmlformats.org/presentationml/2006/main cMediaNode,omitempty"`
}

// Video represents CT_TLMediaNodeVideo (p:video)
type Video struct {
	FullScrn   bool `xml:"fullScrn,attr,omitempty"`
	CMediaNode *CommonMediaNode `xml:"http://schemas.openxmlformats.org/presentationml/2006/main cMediaNode,omitempty"`
}

// CommonMediaNode represents CT_TLCommonMediaNodeData (p:cMediaNode)
type CommonMediaNode struct {
	Vol      string `xml:"vol,attr,omitempty"`
	Mute     bool   `xml:"mute,attr,omitempty"`
	NumSld   uint32 `xml:"numSld,attr,omitempty"`
	// ShowWhenStopped defaults to true when absent, so it is a pointer: an
	// explicit false must be emitted rather than omitted (which readers treat as
	// true).
	ShowWhenStopped *bool `xml:"showWhenStopped,attr,omitempty"`
	CTn      *CommonTimeNode `xml:"http://schemas.openxmlformats.org/presentationml/2006/main cTn,omitempty"`
	TgtEl    *TargetElement  `xml:"http://schemas.openxmlformats.org/presentationml/2006/main tgtEl,omitempty"`
}

// CommonBehavior represents CT_TLCommonBehaviorData (p:cBhvr)
type CommonBehavior struct {
	Additive   string `xml:"additive,attr,omitempty"`  // base, sum, repl, mult, none
	Accumulate string `xml:"accumulate,attr,omitempty"` // none, always
	XfrmType   string `xml:"xfrmType,attr,omitempty"`  // pt, img
	From       string `xml:"from,attr,omitempty"`
	To         string `xml:"to,attr,omitempty"`
	By         string `xml:"by,attr,omitempty"`
	RuntimeContext string `xml:"rctx,attr,omitempty"`
	Override   string `xml:"override,attr,omitempty"` // normal, childStyle
	CTn        *CommonTimeNode `xml:"http://schemas.openxmlformats.org/presentationml/2006/main cTn,omitempty"`
	TgtEl      *TargetElement  `xml:"http://schemas.openxmlformats.org/presentationml/2006/main tgtEl,omitempty"`
	AttrNameLst *AttributeNameList `xml:"http://schemas.openxmlformats.org/presentationml/2006/main attrNameLst,omitempty"`
}

// AttributeNameList represents CT_TLBehaviorAttributeNameList (p:attrNameLst)
type AttributeNameList struct {
	AttrName []string `xml:"http://schemas.openxmlformats.org/presentationml/2006/main attrName,omitempty"`
}

// --- Animation Values ---

// TimeAnimateValueList represents CT_TLTimeAnimateValueList (p:tavLst)
type TimeAnimateValueList struct {
	Tav []*TimeAnimateValue `xml:"http://schemas.openxmlformats.org/presentationml/2006/main tav,omitempty"`
}

// TimeAnimateValue represents CT_TLTimeAnimateValue (p:tav)
type TimeAnimateValue struct {
	Tm  string `xml:"tm,attr,omitempty"` // percentage or "indefinite"
	Fmla string `xml:"fmla,attr,omitempty"`
	Val *AnimVariant `xml:"http://schemas.openxmlformats.org/presentationml/2006/main val,omitempty"`
}

// AnimVariant represents CT_TLAnimVariant (p:val, p:to)
type AnimVariant struct {
	BoolVal *AnimVariantBool   `xml:"http://schemas.openxmlformats.org/presentationml/2006/main boolVal,omitempty"`
	IntVal  *AnimVariantInt    `xml:"http://schemas.openxmlformats.org/presentationml/2006/main intVal,omitempty"`
	FltVal  *AnimVariantFloat  `xml:"http://schemas.openxmlformats.org/presentationml/2006/main fltVal,omitempty"`
	StrVal  *AnimVariantString `xml:"http://schemas.openxmlformats.org/presentationml/2006/main strVal,omitempty"`
	ClrVal  *dml.ColorChoice   `xml:"http://schemas.openxmlformats.org/presentationml/2006/main clrVal,omitempty"`
}

// AnimVariantBool represents CT_TLAnimVariantBooleanVal
type AnimVariantBool struct {
	Val bool `xml:"val,attr"`
}

// AnimVariantInt represents CT_TLAnimVariantIntegerVal
type AnimVariantInt struct {
	Val int32 `xml:"val,attr"`
}

// AnimVariantFloat represents CT_TLAnimVariantFloatVal
type AnimVariantFloat struct {
	Val float64 `xml:"val,attr"`
}

// AnimVariantString represents CT_TLAnimVariantStringVal
type AnimVariantString struct {
	Val string `xml:"val,attr"`
}

// --- Build List ---

// BuildList represents CT_BuildList (p:bldLst)
type BuildList struct {
	BldP       []*BuildParagraph    `xml:"http://schemas.openxmlformats.org/presentationml/2006/main bldP,omitempty"`
	BldDgm     []*BuildDiagram      `xml:"http://schemas.openxmlformats.org/presentationml/2006/main bldDgm,omitempty"`
	BldOleChart []*BuildOleChart    `xml:"http://schemas.openxmlformats.org/presentationml/2006/main bldOleChart,omitempty"`
	BldGraphic []*BuildGraphic      `xml:"http://schemas.openxmlformats.org/presentationml/2006/main bldGraphic,omitempty"`
}

// BuildParagraph represents CT_TLBuildParagraph (p:bldP)
type BuildParagraph struct {
	SpId             string  `xml:"spid,attr"`
	GrpId            *uint32 `xml:"grpId,attr,omitempty"`
	UiExpand         *bool   `xml:"uiExpand,attr,omitempty"`
	Build            string  `xml:"build,attr,omitempty"` // allAtOnce, p, cust, whole
	BldLvl           *int32  `xml:"bldLvl,attr,omitempty"`
	AnimBg           *bool   `xml:"animBg,attr,omitempty"`
	AutoUpdateAnimBg *bool   `xml:"autoUpdateAnimBg,attr,omitempty"`
	Rev              *bool   `xml:"rev,attr,omitempty"`
	AdvAuto        string `xml:"advAuto,attr,omitempty"` // time in ms or "indefinite"
	TmplLst        *TemplateList `xml:"http://schemas.openxmlformats.org/presentationml/2006/main tmplLst,omitempty"`
}

// BuildDiagram represents CT_TLBuildDiagram (p:bldDgm)
type BuildDiagram struct {
	SpId   string  `xml:"spid,attr"`
	GrpId  *uint32 `xml:"grpId,attr,omitempty"`
	UiExpand bool `xml:"uiExpand,attr,omitempty"`
	Bld    string `xml:"bld,attr,omitempty"` // allAtOnce, one, lvlOne, lvlAtOnce
	Rev    bool   `xml:"rev,attr,omitempty"`
}

// BuildOleChart represents CT_TLOleBuildChart (p:bldOleChart)
type BuildOleChart struct {
	SpId   string  `xml:"spid,attr"`
	GrpId  *uint32 `xml:"grpId,attr,omitempty"`
	UiExpand bool `xml:"uiExpand,attr,omitempty"`
	Bld    string `xml:"bld,attr,omitempty"` // allAtOnce, series, category, seriesEl, categoryEl
	AnimBg bool   `xml:"animBg,attr,omitempty"`
}

// BuildGraphic represents CT_TLGraphicalObjectBuild (p:bldGraphic)
type BuildGraphic struct {
	SpId   string  `xml:"spid,attr"`
	GrpId  *uint32 `xml:"grpId,attr,omitempty"`
	UiExpand bool `xml:"uiExpand,attr,omitempty"`
	BldAsOne *BuildAsOne `xml:"http://schemas.openxmlformats.org/presentationml/2006/main bldAsOne,omitempty"`
	BldSub  *BuildSub   `xml:"http://schemas.openxmlformats.org/presentationml/2006/main bldSub,omitempty"`
}

// BuildAsOne represents CT_Empty (p:bldAsOne)
type BuildAsOne struct{}

// BuildSub represents a:CT_AnimationGraphicalObjectBuildProperties (p:bldSub).
// Its children are a:bldDgm/a:bldChart in the DrawingML namespace — not
// dgm/chart in the PresentationML one (C34).
type BuildSub struct {
	BldDgm   *AnimationDgmBuildProperties   `xml:"http://schemas.openxmlformats.org/drawingml/2006/main bldDgm,omitempty"`
	BldChart *AnimationChartBuildProperties `xml:"http://schemas.openxmlformats.org/drawingml/2006/main bldChart,omitempty"`
}

// TemplateList represents CT_TLTemplateList (p:tmplLst)
type TemplateList struct {
	Tmpl []*Template `xml:"http://schemas.openxmlformats.org/presentationml/2006/main tmpl,omitempty"`
}

// Template represents CT_TLTemplate (p:tmpl)
type Template struct {
	Lvl   int32  `xml:"lvl,attr,omitempty"`
	TnLst *TimeNodeList `xml:"http://schemas.openxmlformats.org/presentationml/2006/main tnLst,omitempty"`
}
