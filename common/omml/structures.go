// This file provides the math structure types of EG_OMathMathElements —
// fractions, radicals, scripts, n-ary operators, delimiters, matrices, and
// the accent/bar/box family — together with their property (…Pr) types.
// Every structure is a fixed schema sequence; parsing and serialization go
// through the shared sequence machinery in raw.go, which raw-captures any
// unmodeled child in position.

package omml

import (
	"encoding/xml"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// Accent represents CT_Acc (m:acc): a base with a combining accent character.
type Accent struct {
	AccPr *AccentPr
	E     *Element

	extra []extraChild
}

var accentFields = seqFields(Accent{}, "accPr=AccPr", "e=E")

// UnmarshalXML implements xml.Unmarshaler.
func (v *Accent) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	return unmarshalSeq(d, v, accentFields, &v.extra)
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *Accent) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	marshalSeq(b, ns, localName, v, accentFields, v.extra)
}

func (v *Accent) emitMath(b *xmlb.Builder) { v.MarshalToBuilder(b, NS, "acc") }

// AccentPr represents CT_AccPr (m:accPr).
type AccentPr struct {
	Chr    *Char
	CtrlPr *CtrlPr

	extra []extraChild
}

var accentPrFields = seqFields(AccentPr{}, "chr=Chr", "ctrlPr=CtrlPr")

// UnmarshalXML implements xml.Unmarshaler.
func (v *AccentPr) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	return unmarshalSeq(d, v, accentPrFields, &v.extra)
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *AccentPr) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	marshalSeq(b, ns, localName, v, accentPrFields, v.extra)
}

// Bar represents CT_Bar (m:bar): a base with an overbar or underbar.
type Bar struct {
	BarPr *BarPr
	E     *Element

	extra []extraChild
}

var barFields = seqFields(Bar{}, "barPr=BarPr", "e=E")

// UnmarshalXML implements xml.Unmarshaler.
func (v *Bar) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	return unmarshalSeq(d, v, barFields, &v.extra)
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *Bar) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	marshalSeq(b, ns, localName, v, barFields, v.extra)
}

func (v *Bar) emitMath(b *xmlb.Builder) { v.MarshalToBuilder(b, NS, "bar") }

// BarPr represents CT_BarPr (m:barPr).
type BarPr struct {
	Pos    *TopBot
	CtrlPr *CtrlPr

	extra []extraChild
}

var barPrFields = seqFields(BarPr{}, "pos=Pos", "ctrlPr=CtrlPr")

// UnmarshalXML implements xml.Unmarshaler.
func (v *BarPr) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	return unmarshalSeq(d, v, barPrFields, &v.extra)
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *BarPr) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	marshalSeq(b, ns, localName, v, barPrFields, v.extra)
}

// Box represents CT_Box (m:box): groups an expression so it behaves as a
// single operator or operand.
type Box struct {
	BoxPr *BoxPr
	E     *Element

	extra []extraChild
}

var boxFields = seqFields(Box{}, "boxPr=BoxPr", "e=E")

// UnmarshalXML implements xml.Unmarshaler.
func (v *Box) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	return unmarshalSeq(d, v, boxFields, &v.extra)
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *Box) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	marshalSeq(b, ns, localName, v, boxFields, v.extra)
}

func (v *Box) emitMath(b *xmlb.Builder) { v.MarshalToBuilder(b, NS, "box") }

// BoxPr represents CT_BoxPr (m:boxPr).
type BoxPr struct {
	OpEmu   *OnOff
	NoBreak *OnOff
	Diff    *OnOff
	Brk     *Break
	Aln     *OnOff
	CtrlPr  *CtrlPr

	extra []extraChild
}

var boxPrFields = seqFields(BoxPr{},
	"opEmu=OpEmu", "noBreak=NoBreak", "diff=Diff", "brk=Brk", "aln=Aln", "ctrlPr=CtrlPr")

// UnmarshalXML implements xml.Unmarshaler.
func (v *BoxPr) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	return unmarshalSeq(d, v, boxPrFields, &v.extra)
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *BoxPr) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	marshalSeq(b, ns, localName, v, boxPrFields, v.extra)
}

// BorderBox represents CT_BorderBox (m:borderBox): a bordered (and
// optionally struck-through) expression.
type BorderBox struct {
	BorderBoxPr *BorderBoxPr
	E           *Element

	extra []extraChild
}

var borderBoxFields = seqFields(BorderBox{}, "borderBoxPr=BorderBoxPr", "e=E")

// UnmarshalXML implements xml.Unmarshaler.
func (v *BorderBox) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	return unmarshalSeq(d, v, borderBoxFields, &v.extra)
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *BorderBox) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	marshalSeq(b, ns, localName, v, borderBoxFields, v.extra)
}

func (v *BorderBox) emitMath(b *xmlb.Builder) { v.MarshalToBuilder(b, NS, "borderBox") }

// BorderBoxPr represents CT_BorderBoxPr (m:borderBoxPr).
type BorderBoxPr struct {
	HideTop    *OnOff
	HideBot    *OnOff
	HideLeft   *OnOff
	HideRight  *OnOff
	StrikeH    *OnOff
	StrikeV    *OnOff
	StrikeBLTR *OnOff
	StrikeTLBR *OnOff
	CtrlPr     *CtrlPr

	extra []extraChild
}

var borderBoxPrFields = seqFields(BorderBoxPr{},
	"hideTop=HideTop", "hideBot=HideBot", "hideLeft=HideLeft", "hideRight=HideRight",
	"strikeH=StrikeH", "strikeV=StrikeV", "strikeBLTR=StrikeBLTR", "strikeTLBR=StrikeTLBR",
	"ctrlPr=CtrlPr")

// UnmarshalXML implements xml.Unmarshaler.
func (v *BorderBoxPr) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	return unmarshalSeq(d, v, borderBoxPrFields, &v.extra)
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *BorderBoxPr) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	marshalSeq(b, ns, localName, v, borderBoxPrFields, v.extra)
}

// Delimiter represents CT_D (m:d): one or more arguments enclosed by
// delimiter characters and separated by a separator character.
type Delimiter struct {
	DPr *DelimiterPr
	E   []*Element

	extra []extraChild
}

var delimiterFields = seqFields(Delimiter{}, "dPr=DPr", "e=E")

// UnmarshalXML implements xml.Unmarshaler.
func (v *Delimiter) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	return unmarshalSeq(d, v, delimiterFields, &v.extra)
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *Delimiter) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	marshalSeq(b, ns, localName, v, delimiterFields, v.extra)
}

func (v *Delimiter) emitMath(b *xmlb.Builder) { v.MarshalToBuilder(b, NS, "d") }

// DelimiterPr represents CT_DPr (m:dPr).
type DelimiterPr struct {
	BegChr *Char
	SepChr *Char
	EndChr *Char
	Grow   *OnOff
	Shp    *Shape
	CtrlPr *CtrlPr

	extra []extraChild
}

var delimiterPrFields = seqFields(DelimiterPr{},
	"begChr=BegChr", "sepChr=SepChr", "endChr=EndChr", "grow=Grow", "shp=Shp", "ctrlPr=CtrlPr")

// UnmarshalXML implements xml.Unmarshaler.
func (v *DelimiterPr) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	return unmarshalSeq(d, v, delimiterPrFields, &v.extra)
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *DelimiterPr) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	marshalSeq(b, ns, localName, v, delimiterPrFields, v.extra)
}

// EquationArray represents CT_EqArr (m:eqArr): a vertical stack of aligned
// equations.
type EquationArray struct {
	EqArrPr *EqArrPr
	E       []*Element

	extra []extraChild
}

var equationArrayFields = seqFields(EquationArray{}, "eqArrPr=EqArrPr", "e=E")

// UnmarshalXML implements xml.Unmarshaler.
func (v *EquationArray) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	return unmarshalSeq(d, v, equationArrayFields, &v.extra)
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *EquationArray) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	marshalSeq(b, ns, localName, v, equationArrayFields, v.extra)
}

func (v *EquationArray) emitMath(b *xmlb.Builder) { v.MarshalToBuilder(b, NS, "eqArr") }

// EqArrPr represents CT_EqArrPr (m:eqArrPr).
type EqArrPr struct {
	BaseJc  *YAlign
	MaxDist *OnOff
	ObjDist *OnOff
	RSpRule *SpacingRule
	RSp     *UnSignedInteger
	CtrlPr  *CtrlPr

	extra []extraChild
}

var eqArrPrFields = seqFields(EqArrPr{},
	"baseJc=BaseJc", "maxDist=MaxDist", "objDist=ObjDist", "rSpRule=RSpRule", "rSp=RSp", "ctrlPr=CtrlPr")

// UnmarshalXML implements xml.Unmarshaler.
func (v *EqArrPr) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	return unmarshalSeq(d, v, eqArrPrFields, &v.extra)
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *EqArrPr) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	marshalSeq(b, ns, localName, v, eqArrPrFields, v.extra)
}

// Fraction represents CT_F (m:f): a numerator over (or beside) a
// denominator.
type Fraction struct {
	FPr *FractionPr
	Num *Element
	Den *Element

	extra []extraChild
}

var fractionFields = seqFields(Fraction{}, "fPr=FPr", "num=Num", "den=Den")

// UnmarshalXML implements xml.Unmarshaler.
func (v *Fraction) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	return unmarshalSeq(d, v, fractionFields, &v.extra)
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *Fraction) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	marshalSeq(b, ns, localName, v, fractionFields, v.extra)
}

func (v *Fraction) emitMath(b *xmlb.Builder) { v.MarshalToBuilder(b, NS, "f") }

// FractionPr represents CT_FPr (m:fPr).
type FractionPr struct {
	Type   *FType
	CtrlPr *CtrlPr

	extra []extraChild
}

var fractionPrFields = seqFields(FractionPr{}, "type=Type", "ctrlPr=CtrlPr")

// UnmarshalXML implements xml.Unmarshaler.
func (v *FractionPr) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	return unmarshalSeq(d, v, fractionPrFields, &v.extra)
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *FractionPr) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	marshalSeq(b, ns, localName, v, fractionPrFields, v.extra)
}

// Function represents CT_Func (m:func): a function name applied to an
// argument.
type Function struct {
	FuncPr *FuncPr
	FName  *Element
	E      *Element

	extra []extraChild
}

var functionFields = seqFields(Function{}, "funcPr=FuncPr", "fName=FName", "e=E")

// UnmarshalXML implements xml.Unmarshaler.
func (v *Function) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	return unmarshalSeq(d, v, functionFields, &v.extra)
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *Function) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	marshalSeq(b, ns, localName, v, functionFields, v.extra)
}

func (v *Function) emitMath(b *xmlb.Builder) { v.MarshalToBuilder(b, NS, "func") }

// FuncPr represents CT_FuncPr (m:funcPr).
type FuncPr struct {
	CtrlPr *CtrlPr

	extra []extraChild
}

var funcPrFields = seqFields(FuncPr{}, "ctrlPr=CtrlPr")

// UnmarshalXML implements xml.Unmarshaler.
func (v *FuncPr) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	return unmarshalSeq(d, v, funcPrFields, &v.extra)
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *FuncPr) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	marshalSeq(b, ns, localName, v, funcPrFields, v.extra)
}

// GroupChar represents CT_GroupChr (m:groupChr): an expression grouped by a
// character above or below it (over/underbrace).
type GroupChar struct {
	GroupChrPr *GroupChrPr
	E          *Element

	extra []extraChild
}

var groupCharFields = seqFields(GroupChar{}, "groupChrPr=GroupChrPr", "e=E")

// UnmarshalXML implements xml.Unmarshaler.
func (v *GroupChar) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	return unmarshalSeq(d, v, groupCharFields, &v.extra)
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *GroupChar) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	marshalSeq(b, ns, localName, v, groupCharFields, v.extra)
}

func (v *GroupChar) emitMath(b *xmlb.Builder) { v.MarshalToBuilder(b, NS, "groupChr") }

// GroupChrPr represents CT_GroupChrPr (m:groupChrPr).
type GroupChrPr struct {
	Chr    *Char
	Pos    *TopBot
	VertJc *TopBot
	CtrlPr *CtrlPr

	extra []extraChild
}

var groupChrPrFields = seqFields(GroupChrPr{},
	"chr=Chr", "pos=Pos", "vertJc=VertJc", "ctrlPr=CtrlPr")

// UnmarshalXML implements xml.Unmarshaler.
func (v *GroupChrPr) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	return unmarshalSeq(d, v, groupChrPrFields, &v.extra)
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *GroupChrPr) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	marshalSeq(b, ns, localName, v, groupChrPrFields, v.extra)
}

// LimitLow represents CT_LimLow (m:limLow): a base with a limit below it.
type LimitLow struct {
	LimLowPr *LimPr
	E        *Element
	Lim      *Element

	extra []extraChild
}

var limitLowFields = seqFields(LimitLow{}, "limLowPr=LimLowPr", "e=E", "lim=Lim")

// UnmarshalXML implements xml.Unmarshaler.
func (v *LimitLow) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	return unmarshalSeq(d, v, limitLowFields, &v.extra)
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *LimitLow) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	marshalSeq(b, ns, localName, v, limitLowFields, v.extra)
}

func (v *LimitLow) emitMath(b *xmlb.Builder) { v.MarshalToBuilder(b, NS, "limLow") }

// LimitUpper represents CT_LimUpp (m:limUpp): a base with a limit above it.
type LimitUpper struct {
	LimUppPr *LimPr
	E        *Element
	Lim      *Element

	extra []extraChild
}

var limitUpperFields = seqFields(LimitUpper{}, "limUppPr=LimUppPr", "e=E", "lim=Lim")

// UnmarshalXML implements xml.Unmarshaler.
func (v *LimitUpper) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	return unmarshalSeq(d, v, limitUpperFields, &v.extra)
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *LimitUpper) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	marshalSeq(b, ns, localName, v, limitUpperFields, v.extra)
}

func (v *LimitUpper) emitMath(b *xmlb.Builder) { v.MarshalToBuilder(b, NS, "limUpp") }

// LimPr represents CT_LimLowPr / CT_LimUppPr (m:limLowPr, m:limUppPr), which
// the schema defines identically.
type LimPr struct {
	CtrlPr *CtrlPr

	extra []extraChild
}

var limPrFields = seqFields(LimPr{}, "ctrlPr=CtrlPr")

// UnmarshalXML implements xml.Unmarshaler.
func (v *LimPr) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	return unmarshalSeq(d, v, limPrFields, &v.extra)
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *LimPr) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	marshalSeq(b, ns, localName, v, limPrFields, v.extra)
}

// Matrix represents CT_M (m:m): rows of aligned math arguments.
type Matrix struct {
	MPr *MatrixPr
	MR  []*MatrixRow

	extra []extraChild
}

var matrixFields = seqFields(Matrix{}, "mPr=MPr", "mr=MR")

// UnmarshalXML implements xml.Unmarshaler.
func (v *Matrix) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	return unmarshalSeq(d, v, matrixFields, &v.extra)
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *Matrix) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	marshalSeq(b, ns, localName, v, matrixFields, v.extra)
}

func (v *Matrix) emitMath(b *xmlb.Builder) { v.MarshalToBuilder(b, NS, "m") }

// MatrixPr represents CT_MPr (m:mPr).
type MatrixPr struct {
	BaseJc  *YAlign
	PlcHide *OnOff
	RSpRule *SpacingRule
	CGpRule *SpacingRule
	RSp     *UnSignedInteger
	CSp     *UnSignedInteger
	CGp     *UnSignedInteger
	MCS     *MatrixColumns
	CtrlPr  *CtrlPr

	extra []extraChild
}

var matrixPrFields = seqFields(MatrixPr{},
	"baseJc=BaseJc", "plcHide=PlcHide", "rSpRule=RSpRule", "cGpRule=CGpRule",
	"rSp=RSp", "cSp=CSp", "cGp=CGp", "mcs=MCS", "ctrlPr=CtrlPr")

// UnmarshalXML implements xml.Unmarshaler.
func (v *MatrixPr) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	return unmarshalSeq(d, v, matrixPrFields, &v.extra)
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *MatrixPr) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	marshalSeq(b, ns, localName, v, matrixPrFields, v.extra)
}

// MatrixRow represents CT_MR (m:mr): one matrix row.
type MatrixRow struct {
	E []*Element

	extra []extraChild
}

var matrixRowFields = seqFields(MatrixRow{}, "e=E")

// UnmarshalXML implements xml.Unmarshaler.
func (v *MatrixRow) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	return unmarshalSeq(d, v, matrixRowFields, &v.extra)
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *MatrixRow) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	marshalSeq(b, ns, localName, v, matrixRowFields, v.extra)
}

// MatrixColumns represents CT_MCS (m:mcs): the matrix column groups.
type MatrixColumns struct {
	MC []*MatrixColumn

	extra []extraChild
}

var matrixColumnsFields = seqFields(MatrixColumns{}, "mc=MC")

// UnmarshalXML implements xml.Unmarshaler.
func (v *MatrixColumns) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	return unmarshalSeq(d, v, matrixColumnsFields, &v.extra)
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *MatrixColumns) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	marshalSeq(b, ns, localName, v, matrixColumnsFields, v.extra)
}

// MatrixColumn represents CT_MC (m:mc): one matrix column group.
type MatrixColumn struct {
	MCPr *MatrixColumnPr

	extra []extraChild
}

var matrixColumnFields = seqFields(MatrixColumn{}, "mcPr=MCPr")

// UnmarshalXML implements xml.Unmarshaler.
func (v *MatrixColumn) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	return unmarshalSeq(d, v, matrixColumnFields, &v.extra)
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *MatrixColumn) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	marshalSeq(b, ns, localName, v, matrixColumnFields, v.extra)
}

// MatrixColumnPr represents CT_MCPr (m:mcPr).
type MatrixColumnPr struct {
	Count *Integer255
	McJc  *XAlign

	extra []extraChild
}

var matrixColumnPrFields = seqFields(MatrixColumnPr{}, "count=Count", "mcJc=McJc")

// UnmarshalXML implements xml.Unmarshaler.
func (v *MatrixColumnPr) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	return unmarshalSeq(d, v, matrixColumnPrFields, &v.extra)
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *MatrixColumnPr) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	marshalSeq(b, ns, localName, v, matrixColumnPrFields, v.extra)
}

// NAry represents CT_Nary (m:nary): an n-ary operator (sum, product,
// integral) with optional limits and a base argument.
type NAry struct {
	NaryPr *NaryPr
	Sub    *Element
	Sup    *Element
	E      *Element

	extra []extraChild
}

var nAryFields = seqFields(NAry{}, "naryPr=NaryPr", "sub=Sub", "sup=Sup", "e=E")

// UnmarshalXML implements xml.Unmarshaler.
func (v *NAry) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	return unmarshalSeq(d, v, nAryFields, &v.extra)
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *NAry) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	marshalSeq(b, ns, localName, v, nAryFields, v.extra)
}

func (v *NAry) emitMath(b *xmlb.Builder) { v.MarshalToBuilder(b, NS, "nary") }

// NaryPr represents CT_NaryPr (m:naryPr).
type NaryPr struct {
	Chr     *Char
	LimLoc  *LimLoc
	Grow    *OnOff
	SubHide *OnOff
	SupHide *OnOff
	CtrlPr  *CtrlPr

	extra []extraChild
}

var naryPrFields = seqFields(NaryPr{},
	"chr=Chr", "limLoc=LimLoc", "grow=Grow", "subHide=SubHide", "supHide=SupHide", "ctrlPr=CtrlPr")

// UnmarshalXML implements xml.Unmarshaler.
func (v *NaryPr) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	return unmarshalSeq(d, v, naryPrFields, &v.extra)
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *NaryPr) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	marshalSeq(b, ns, localName, v, naryPrFields, v.extra)
}

// Phantom represents CT_Phant (m:phant): an invisible expression that still
// occupies space.
type Phantom struct {
	PhantPr *PhantPr
	E       *Element

	extra []extraChild
}

var phantomFields = seqFields(Phantom{}, "phantPr=PhantPr", "e=E")

// UnmarshalXML implements xml.Unmarshaler.
func (v *Phantom) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	return unmarshalSeq(d, v, phantomFields, &v.extra)
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *Phantom) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	marshalSeq(b, ns, localName, v, phantomFields, v.extra)
}

func (v *Phantom) emitMath(b *xmlb.Builder) { v.MarshalToBuilder(b, NS, "phant") }

// PhantPr represents CT_PhantPr (m:phantPr).
type PhantPr struct {
	Show     *OnOff
	ZeroWid  *OnOff
	ZeroAsc  *OnOff
	ZeroDesc *OnOff
	Transp   *OnOff
	CtrlPr   *CtrlPr

	extra []extraChild
}

var phantPrFields = seqFields(PhantPr{},
	"show=Show", "zeroWid=ZeroWid", "zeroAsc=ZeroAsc", "zeroDesc=ZeroDesc",
	"transp=Transp", "ctrlPr=CtrlPr")

// UnmarshalXML implements xml.Unmarshaler.
func (v *PhantPr) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	return unmarshalSeq(d, v, phantPrFields, &v.extra)
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *PhantPr) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	marshalSeq(b, ns, localName, v, phantPrFields, v.extra)
}

// Radical represents CT_Rad (m:rad): a radical with an optional degree.
type Radical struct {
	RadPr *RadPr
	Deg   *Element
	E     *Element

	extra []extraChild
}

var radicalFields = seqFields(Radical{}, "radPr=RadPr", "deg=Deg", "e=E")

// UnmarshalXML implements xml.Unmarshaler.
func (v *Radical) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	return unmarshalSeq(d, v, radicalFields, &v.extra)
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *Radical) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	marshalSeq(b, ns, localName, v, radicalFields, v.extra)
}

func (v *Radical) emitMath(b *xmlb.Builder) { v.MarshalToBuilder(b, NS, "rad") }

// RadPr represents CT_RadPr (m:radPr).
type RadPr struct {
	DegHide *OnOff
	CtrlPr  *CtrlPr

	extra []extraChild
}

var radPrFields = seqFields(RadPr{}, "degHide=DegHide", "ctrlPr=CtrlPr")

// UnmarshalXML implements xml.Unmarshaler.
func (v *RadPr) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	return unmarshalSeq(d, v, radPrFields, &v.extra)
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *RadPr) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	marshalSeq(b, ns, localName, v, radPrFields, v.extra)
}

// SubSuperscriptPre represents CT_SPre (m:sPre): a base preceded by a
// subscript and superscript.
type SubSuperscriptPre struct {
	SPrePr *SPrePr
	Sub    *Element
	Sup    *Element
	E      *Element

	extra []extraChild
}

var sPreFields = seqFields(SubSuperscriptPre{}, "sPrePr=SPrePr", "sub=Sub", "sup=Sup", "e=E")

// UnmarshalXML implements xml.Unmarshaler.
func (v *SubSuperscriptPre) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	return unmarshalSeq(d, v, sPreFields, &v.extra)
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *SubSuperscriptPre) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	marshalSeq(b, ns, localName, v, sPreFields, v.extra)
}

func (v *SubSuperscriptPre) emitMath(b *xmlb.Builder) { v.MarshalToBuilder(b, NS, "sPre") }

// SPrePr represents CT_SPrePr (m:sPrePr).
type SPrePr struct {
	CtrlPr *CtrlPr

	extra []extraChild
}

var sPrePrFields = seqFields(SPrePr{}, "ctrlPr=CtrlPr")

// UnmarshalXML implements xml.Unmarshaler.
func (v *SPrePr) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	return unmarshalSeq(d, v, sPrePrFields, &v.extra)
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *SPrePr) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	marshalSeq(b, ns, localName, v, sPrePrFields, v.extra)
}

// Subscript represents CT_SSub (m:sSub): a base with a subscript.
type Subscript struct {
	SSubPr *SSubPr
	E      *Element
	Sub    *Element

	extra []extraChild
}

var subscriptFields = seqFields(Subscript{}, "sSubPr=SSubPr", "e=E", "sub=Sub")

// UnmarshalXML implements xml.Unmarshaler.
func (v *Subscript) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	return unmarshalSeq(d, v, subscriptFields, &v.extra)
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *Subscript) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	marshalSeq(b, ns, localName, v, subscriptFields, v.extra)
}

func (v *Subscript) emitMath(b *xmlb.Builder) { v.MarshalToBuilder(b, NS, "sSub") }

// SSubPr represents CT_SSubPr (m:sSubPr).
type SSubPr struct {
	CtrlPr *CtrlPr

	extra []extraChild
}

var sSubPrFields = seqFields(SSubPr{}, "ctrlPr=CtrlPr")

// UnmarshalXML implements xml.Unmarshaler.
func (v *SSubPr) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	return unmarshalSeq(d, v, sSubPrFields, &v.extra)
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *SSubPr) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	marshalSeq(b, ns, localName, v, sSubPrFields, v.extra)
}

// SubSuperscript represents CT_SSubSup (m:sSubSup): a base with both a
// subscript and a superscript.
type SubSuperscript struct {
	SSubSupPr *SSubSupPr
	E         *Element
	Sub       *Element
	Sup       *Element

	extra []extraChild
}

var subSuperscriptFields = seqFields(SubSuperscript{},
	"sSubSupPr=SSubSupPr", "e=E", "sub=Sub", "sup=Sup")

// UnmarshalXML implements xml.Unmarshaler.
func (v *SubSuperscript) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	return unmarshalSeq(d, v, subSuperscriptFields, &v.extra)
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *SubSuperscript) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	marshalSeq(b, ns, localName, v, subSuperscriptFields, v.extra)
}

func (v *SubSuperscript) emitMath(b *xmlb.Builder) { v.MarshalToBuilder(b, NS, "sSubSup") }

// SSubSupPr represents CT_SSubSupPr (m:sSubSupPr).
type SSubSupPr struct {
	AlnScr *OnOff
	CtrlPr *CtrlPr

	extra []extraChild
}

var sSubSupPrFields = seqFields(SSubSupPr{}, "alnScr=AlnScr", "ctrlPr=CtrlPr")

// UnmarshalXML implements xml.Unmarshaler.
func (v *SSubSupPr) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	return unmarshalSeq(d, v, sSubSupPrFields, &v.extra)
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *SSubSupPr) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	marshalSeq(b, ns, localName, v, sSubSupPrFields, v.extra)
}

// Superscript represents CT_SSup (m:sSup): a base with a superscript.
type Superscript struct {
	SSupPr *SSupPr
	E      *Element
	Sup    *Element

	extra []extraChild
}

var superscriptFields = seqFields(Superscript{}, "sSupPr=SSupPr", "e=E", "sup=Sup")

// UnmarshalXML implements xml.Unmarshaler.
func (v *Superscript) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	return unmarshalSeq(d, v, superscriptFields, &v.extra)
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *Superscript) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	marshalSeq(b, ns, localName, v, superscriptFields, v.extra)
}

func (v *Superscript) emitMath(b *xmlb.Builder) { v.MarshalToBuilder(b, NS, "sSup") }

// SSupPr represents CT_SSupPr (m:sSupPr).
type SSupPr struct {
	CtrlPr *CtrlPr

	extra []extraChild
}

var sSupPrFields = seqFields(SSupPr{}, "ctrlPr=CtrlPr")

// UnmarshalXML implements xml.Unmarshaler.
func (v *SSupPr) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	return unmarshalSeq(d, v, sSupPrFields, &v.extra)
}

// MarshalToBuilder implements xmlb.BuilderMarshaler.
func (v *SSupPr) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	marshalSeq(b, ns, localName, v, sSupPrFields, v.extra)
}
