// Package omml provides Office Math Markup Language types from shared-math.xsd.
// These types implement the m: namespace elements for mathematical equations.
package omml

// OMath represents CT_OMath (m:oMath) - a math zone/equation
type OMath struct {
	Elements []OMathElement `xml:",any"`
}

// OMathElement represents any element that can appear in a math zone
type OMathElement struct {
	// Math structures
	Acc    *Accent       `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math acc,omitempty"`
	Bar    *Bar          `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math bar,omitempty"`
	Box    *Box          `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math box,omitempty"`
	BorderBox *BorderBox `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math borderBox,omitempty"`
	D      *Delimiter    `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math d,omitempty"`
	EqArr  *EquationArray `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math eqArr,omitempty"`
	F      *Fraction     `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math f,omitempty"`
	Func   *Function     `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math func,omitempty"`
	GroupChr *GroupChar  `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math groupChr,omitempty"`
	LimLow *LimitLow     `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math limLow,omitempty"`
	LimUpp *LimitUpper   `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math limUpp,omitempty"`
	M      *Matrix       `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math m,omitempty"`
	Nary   *NAry         `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math nary,omitempty"`
	Phant  *Phantom      `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math phant,omitempty"`
	Rad    *Radical      `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math rad,omitempty"`
	SPre   *SubSuperscriptPre `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math sPre,omitempty"`
	SSub   *Subscript    `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math sSub,omitempty"`
	SSubSup *SubSuperscript `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math sSubSup,omitempty"`
	SSup   *Superscript  `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math sSup,omitempty"`
	R      *Run          `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math r,omitempty"`
}

// OMathPara represents CT_OMathPara (m:oMathPara) - a math paragraph
type OMathPara struct {
	OMathParaPr *OMathParaPr `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math oMathParaPr,omitempty"`
	OMath       []*OMath     `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math oMath,omitempty"`
}

// OMathParaPr represents CT_OMathParaPr (m:oMathParaPr) - math paragraph properties
type OMathParaPr struct {
	Jc *MathJc `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math jc,omitempty"`
}

// MathJc represents CT_OMathJc - math paragraph justification
type MathJc struct {
	Val string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math val,attr,omitempty"` // left, right, center, centerGroup
}

// Run represents CT_R (m:r) - a math run containing text
type Run struct {
	RPr *RunPr `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math rPr,omitempty"`
	T   string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math t,omitempty"`
}

// RunPr represents CT_RPR (m:rPr) - math run properties
type RunPr struct {
	Lit   *OnOff `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math lit,omitempty"`   // literal (non-italic)
	Nor   *OnOff `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math nor,omitempty"`   // normal text
	Scr   *Script `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math scr,omitempty"`  // script style
	Sty   *Style `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math sty,omitempty"`   // style (bold/italic)
	Brk   *Break `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math brk,omitempty"`   // break
	Aln   *OnOff `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math aln,omitempty"`   // alignment
}

// OnOff represents CT_OnOff - boolean on/off value
type OnOff struct {
	Val string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math val,attr,omitempty"` // on, off, 1, 0, true, false
}

// Script represents CT_Script - script type
type Script struct {
	Val string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math val,attr,omitempty"` // roman, script, fraktur, double-struck, sans-serif, monospace
}

// Style represents CT_Style - text style (bold/italic)
type Style struct {
	Val string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math val,attr,omitempty"` // p, b, i, bi
}

// Break represents CT_ManualBreak - manual break
type Break struct {
	AlnAt int32 `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math alnAt,attr,omitempty"`
}

// Accent represents CT_Acc (m:acc) - accent structure
type Accent struct {
	AccPr *AccentPr `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math accPr,omitempty"`
	E     *Element  `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math e,omitempty"`
}

// AccentPr represents CT_AccPr - accent properties
type AccentPr struct {
	Chr   *Char      `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math chr,omitempty"`
	CtrlPr *CtrlPr   `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math ctrlPr,omitempty"`
}

// Bar represents CT_Bar (m:bar) - bar structure (overbar/underbar)
type Bar struct {
	BarPr *BarPr   `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math barPr,omitempty"`
	E     *Element `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math e,omitempty"`
}

// BarPr represents CT_BarPr - bar properties
type BarPr struct {
	Pos    *TopBot  `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math pos,omitempty"`
	CtrlPr *CtrlPr  `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math ctrlPr,omitempty"`
}

// Box represents CT_Box (m:box) - box structure
type Box struct {
	BoxPr *BoxPr   `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math boxPr,omitempty"`
	E     *Element `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math e,omitempty"`
}

// BoxPr represents CT_BoxPr - box properties
type BoxPr struct {
	OpEmu  *OnOff   `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math opEmu,omitempty"`
	NoBreak *OnOff  `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math noBreak,omitempty"`
	Diff   *OnOff   `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math diff,omitempty"`
	Brk    *Break   `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math brk,omitempty"`
	Aln    *OnOff   `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math aln,omitempty"`
	CtrlPr *CtrlPr  `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math ctrlPr,omitempty"`
}

// BorderBox represents CT_BorderBox (m:borderBox)
type BorderBox struct {
	BorderBoxPr *BorderBoxPr `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math borderBoxPr,omitempty"`
	E           *Element     `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math e,omitempty"`
}

// BorderBoxPr represents CT_BorderBoxPr
type BorderBoxPr struct {
	HideTop    *OnOff  `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math hideTop,omitempty"`
	HideBot    *OnOff  `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math hideBot,omitempty"`
	HideLeft   *OnOff  `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math hideLeft,omitempty"`
	HideRight  *OnOff  `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math hideRight,omitempty"`
	StrikeH    *OnOff  `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math strikeH,omitempty"`
	StrikeV    *OnOff  `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math strikeV,omitempty"`
	StrikeBLTR *OnOff  `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math strikeBLTR,omitempty"`
	StrikeTLBR *OnOff  `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math strikeTLBR,omitempty"`
	CtrlPr     *CtrlPr `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math ctrlPr,omitempty"`
}

// Delimiter represents CT_D (m:d) - delimiter/parentheses structure
type Delimiter struct {
	DPr *DelimiterPr `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math dPr,omitempty"`
	E   []*Element   `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math e,omitempty"`
}

// DelimiterPr represents CT_DPr - delimiter properties
type DelimiterPr struct {
	BegChr *Char    `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math begChr,omitempty"`
	SepChr *Char    `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math sepChr,omitempty"`
	EndChr *Char    `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math endChr,omitempty"`
	Grow   *OnOff   `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math grow,omitempty"`
	Shp    *Shape   `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math shp,omitempty"`
	CtrlPr *CtrlPr  `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math ctrlPr,omitempty"`
}

// EquationArray represents CT_EqArr (m:eqArr) - equation array
type EquationArray struct {
	EqArrPr *EqArrPr   `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math eqArrPr,omitempty"`
	E       []*Element `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math e,omitempty"`
}

// EqArrPr represents CT_EqArrPr - equation array properties
type EqArrPr struct {
	BaseJc  *YAlign  `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math baseJc,omitempty"`
	MaxDist *OnOff   `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math maxDist,omitempty"`
	ObjDist *OnOff   `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math objDist,omitempty"`
	RSpRule *SpacingRule `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math rSpRule,omitempty"`
	RSp     *UnSignedInteger `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math rSp,omitempty"`
	CtrlPr  *CtrlPr  `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math ctrlPr,omitempty"`
}

// Fraction represents CT_F (m:f) - fraction
type Fraction struct {
	FPr  *FractionPr `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math fPr,omitempty"`
	Num  *Element    `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math num,omitempty"`
	Den  *Element    `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math den,omitempty"`
}

// FractionPr represents CT_FPr - fraction properties
type FractionPr struct {
	Type   *FType   `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math type,omitempty"`
	CtrlPr *CtrlPr  `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math ctrlPr,omitempty"`
}

// Function represents CT_Func (m:func) - function structure
type Function struct {
	FuncPr *FuncPr  `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math funcPr,omitempty"`
	FName  *Element `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math fName,omitempty"`
	E      *Element `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math e,omitempty"`
}

// FuncPr represents CT_FuncPr - function properties
type FuncPr struct {
	CtrlPr *CtrlPr `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math ctrlPr,omitempty"`
}

// GroupChar represents CT_GroupChr (m:groupChr) - group character (brace above/below)
type GroupChar struct {
	GroupChrPr *GroupChrPr `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math groupChrPr,omitempty"`
	E          *Element    `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math e,omitempty"`
}

// GroupChrPr represents CT_GroupChrPr
type GroupChrPr struct {
	Chr    *Char    `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math chr,omitempty"`
	Pos    *TopBot  `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math pos,omitempty"`
	VertJc *TopBot  `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math vertJc,omitempty"`
	CtrlPr *CtrlPr  `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math ctrlPr,omitempty"`
}

// LimitLow represents CT_LimLow (m:limLow) - lower limit
type LimitLow struct {
	LimLowPr *LimPr   `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math limLowPr,omitempty"`
	E        *Element `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math e,omitempty"`
	Lim      *Element `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math lim,omitempty"`
}

// LimitUpper represents CT_LimUpp (m:limUpp) - upper limit
type LimitUpper struct {
	LimUppPr *LimPr   `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math limUppPr,omitempty"`
	E        *Element `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math e,omitempty"`
	Lim      *Element `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math lim,omitempty"`
}

// LimPr represents CT_LimPr - limit properties
type LimPr struct {
	CtrlPr *CtrlPr `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math ctrlPr,omitempty"`
}

// Matrix represents CT_M (m:m) - matrix
type Matrix struct {
	MPr *MatrixPr    `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math mPr,omitempty"`
	MR  []*MatrixRow `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math mr,omitempty"`
}

// MatrixPr represents CT_MPr - matrix properties
type MatrixPr struct {
	BaseJc  *YAlign      `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math baseJc,omitempty"`
	PlcHide *OnOff       `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math plcHide,omitempty"`
	RSpRule *SpacingRule `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math rSpRule,omitempty"`
	CGpRule *SpacingRule `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math cGpRule,omitempty"`
	RSp     *UnSignedInteger `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math rSp,omitempty"`
	CGp     *UnSignedInteger `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math cGp,omitempty"`
	MCS     *MatrixColumns   `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math mcs,omitempty"`
	CtrlPr  *CtrlPr      `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math ctrlPr,omitempty"`
}

// MatrixRow represents CT_MR (m:mr) - matrix row
type MatrixRow struct {
	E []*Element `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math e,omitempty"`
}

// MatrixColumns represents CT_MCS - matrix column properties
type MatrixColumns struct {
	MC []*MatrixColumn `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math mc,omitempty"`
}

// MatrixColumn represents CT_MC - matrix column
type MatrixColumn struct {
	MCPr *MatrixColumnPr `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math mcPr,omitempty"`
}

// MatrixColumnPr represents CT_MCPr - matrix column properties
type MatrixColumnPr struct {
	Count *Integer `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math count,omitempty"`
	McJc  *XAlign  `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math mcJc,omitempty"`
}

// NAry represents CT_Nary (m:nary) - n-ary operator (sum, product, integral)
type NAry struct {
	NaryPr *NaryPr  `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math naryPr,omitempty"`
	Sub    *Element `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math sub,omitempty"`
	Sup    *Element `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math sup,omitempty"`
	E      *Element `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math e,omitempty"`
}

// NaryPr represents CT_NaryPr - n-ary properties
type NaryPr struct {
	Chr    *Char    `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math chr,omitempty"`
	LimLoc *LimLoc  `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math limLoc,omitempty"`
	Grow   *OnOff   `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math grow,omitempty"`
	SubHide *OnOff  `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math subHide,omitempty"`
	SupHide *OnOff  `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math supHide,omitempty"`
	CtrlPr *CtrlPr  `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math ctrlPr,omitempty"`
}

// Phantom represents CT_Phant (m:phant) - phantom (invisible with spacing)
type Phantom struct {
	PhantPr *PhantPr `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math phantPr,omitempty"`
	E       *Element `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math e,omitempty"`
}

// PhantPr represents CT_PhantPr - phantom properties
type PhantPr struct {
	Show     *OnOff  `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math show,omitempty"`
	ZeroWid  *OnOff  `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math zeroWid,omitempty"`
	ZeroAsc  *OnOff  `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math zeroAsc,omitempty"`
	ZeroDesc *OnOff  `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math zeroDesc,omitempty"`
	Transp   *OnOff  `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math transp,omitempty"`
	CtrlPr   *CtrlPr `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math ctrlPr,omitempty"`
}

// Radical represents CT_Rad (m:rad) - radical/root
type Radical struct {
	RadPr *RadPr   `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math radPr,omitempty"`
	Deg   *Element `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math deg,omitempty"`
	E     *Element `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math e,omitempty"`
}

// RadPr represents CT_RadPr - radical properties
type RadPr struct {
	DegHide *OnOff  `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math degHide,omitempty"`
	CtrlPr  *CtrlPr `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math ctrlPr,omitempty"`
}

// SubSuperscriptPre represents CT_SPre (m:sPre) - pre-sub/superscript
type SubSuperscriptPre struct {
	SPrePr *SPrePr  `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math sPrePr,omitempty"`
	Sub    *Element `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math sub,omitempty"`
	Sup    *Element `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math sup,omitempty"`
	E      *Element `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math e,omitempty"`
}

// SPrePr represents CT_SPrePr
type SPrePr struct {
	CtrlPr *CtrlPr `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math ctrlPr,omitempty"`
}

// Subscript represents CT_SSub (m:sSub)
type Subscript struct {
	SSubPr *SSubPr  `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math sSubPr,omitempty"`
	E      *Element `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math e,omitempty"`
	Sub    *Element `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math sub,omitempty"`
}

// SSubPr represents CT_SSubPr
type SSubPr struct {
	CtrlPr *CtrlPr `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math ctrlPr,omitempty"`
}

// SubSuperscript represents CT_SSubSup (m:sSubSup)
type SubSuperscript struct {
	SSubSupPr *SSubSupPr `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math sSubSupPr,omitempty"`
	E         *Element   `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math e,omitempty"`
	Sub       *Element   `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math sub,omitempty"`
	Sup       *Element   `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math sup,omitempty"`
}

// SSubSupPr represents CT_SSubSupPr
type SSubSupPr struct {
	AlnScr *OnOff  `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math alnScr,omitempty"`
	CtrlPr *CtrlPr `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math ctrlPr,omitempty"`
}

// Superscript represents CT_SSup (m:sSup)
type Superscript struct {
	SSupPr *SSupPr  `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math sSupPr,omitempty"`
	E      *Element `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math e,omitempty"`
	Sup    *Element `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math sup,omitempty"`
}

// SSupPr represents CT_SSupPr
type SSupPr struct {
	CtrlPr *CtrlPr `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math ctrlPr,omitempty"`
}

// Element represents CT_OMathArg (m:e) - a math argument/element
type Element struct {
	ArgPr *ArgPr         `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math argPr,omitempty"`
	Content []OMathElement `xml:",any"`
}

// ArgPr represents CT_OMathArgPr - argument properties
type ArgPr struct {
	ArgSz *Integer `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math argSz,omitempty"`
}

// CtrlPr represents CT_CtrlPr - control properties.
// Per XSD: contains w:EG_RPrMath (choice of rPr, ins, del).
type CtrlPr struct {
	RPr *WmlRPr `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main rPr,omitempty"`
	Ins *WmlMathCtrlIns `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main ins,omitempty"`
	Del *WmlMathCtrlDel `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main del,omitempty"`
}

// WmlRPr represents CT_RPr (w:rPr) - WordprocessingML run properties.
// Per XSD: EG_RPrContent = EG_RPrBase* + rPrChange?
type WmlRPr struct {
	RStyle  *WmlString   `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main rStyle,omitempty"`
	RFonts  *WmlFonts    `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main rFonts,omitempty"`
	B       *WmlOnOff    `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main b,omitempty"`
	BCs     *WmlOnOff    `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main bCs,omitempty"`
	I       *WmlOnOff    `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main i,omitempty"`
	ICs     *WmlOnOff    `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main iCs,omitempty"`
	Caps    *WmlOnOff    `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main caps,omitempty"`
	SmallCaps *WmlOnOff  `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main smallCaps,omitempty"`
	Strike  *WmlOnOff    `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main strike,omitempty"`
	Dstrike *WmlOnOff    `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main dstrike,omitempty"`
	Outline *WmlOnOff    `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main outline,omitempty"`
	Shadow  *WmlOnOff    `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main shadow,omitempty"`
	Emboss  *WmlOnOff    `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main emboss,omitempty"`
	Imprint *WmlOnOff    `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main imprint,omitempty"`
	NoProof *WmlOnOff    `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main noProof,omitempty"`
	SnapToGrid *WmlOnOff `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main snapToGrid,omitempty"`
	Vanish  *WmlOnOff    `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main vanish,omitempty"`
	WebHidden *WmlOnOff  `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main webHidden,omitempty"`
	Color   *WmlColor    `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main color,omitempty"`
	Spacing *WmlSignedMeasure `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main spacing,omitempty"`
	W       *WmlMeasure  `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main w,omitempty"`
	Kern    *WmlMeasure  `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main kern,omitempty"`
	Position *WmlSignedMeasure `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main position,omitempty"`
	Sz      *WmlMeasure  `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main sz,omitempty"`
	SzCs    *WmlMeasure  `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main szCs,omitempty"`
	Highlight *WmlString `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main highlight,omitempty"`
	U       *WmlUnderline `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main u,omitempty"`
	Effect  *WmlString   `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main effect,omitempty"`
	Bdr     *WmlBorder   `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main bdr,omitempty"`
	Shd     *WmlShd      `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main shd,omitempty"`
	FitText *WmlMeasure  `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main fitText,omitempty"`
	VertAlign *WmlString `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main vertAlign,omitempty"`
	Rtl     *WmlOnOff    `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main rtl,omitempty"`
	Cs      *WmlOnOff    `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main cs,omitempty"`
	Em      *WmlString   `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main em,omitempty"`
	Lang    *WmlLang     `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main lang,omitempty"`
	EastAsianLayout *WmlEastAsianLayout `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main eastAsianLayout,omitempty"`
	SpecVanish *WmlOnOff `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main specVanish,omitempty"`
	OMath   *WmlOnOff    `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main oMath,omitempty"`
}

// WmlOnOff represents CT_OnOff (w:b, w:i, etc.)
type WmlOnOff struct {
	Val string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr,omitempty"`
}

// WmlString represents CT_String (w:rStyle, etc.)
type WmlString struct {
	Val string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr,omitempty"`
}

// WmlFonts represents CT_Fonts (w:rFonts)
type WmlFonts struct {
	Hint          string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main hint,attr,omitempty"`
	Ascii         string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main ascii,attr,omitempty"`
	HAnsi         string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main hAnsi,attr,omitempty"`
	EastAsia      string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main eastAsia,attr,omitempty"`
	Cs            string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main cs,attr,omitempty"`
	AsciiTheme    string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main asciiTheme,attr,omitempty"`
	HAnsiTheme    string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main hAnsiTheme,attr,omitempty"`
	EastAsiaTheme string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main eastAsiaTheme,attr,omitempty"`
	CsTheme       string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main cstheme,attr,omitempty"`
}

// WmlColor represents CT_Color (w:color)
type WmlColor struct {
	Val       string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr,omitempty"`
	ThemeColor string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main themeColor,attr,omitempty"`
	ThemeTint  string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main themeTint,attr,omitempty"`
	ThemeShade string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main themeShade,attr,omitempty"`
}

// WmlMeasure represents CT_HpsMeasure / CT_TextScale / CT_FitText (val attr)
type WmlMeasure struct {
	Val string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr,omitempty"`
}

// WmlSignedMeasure represents CT_SignedTwipsMeasure / CT_SignedHpsMeasure (val attr)
type WmlSignedMeasure struct {
	Val string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr,omitempty"`
}

// WmlUnderline represents CT_Underline (w:u)
type WmlUnderline struct {
	Val       string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr,omitempty"`
	Color     string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main color,attr,omitempty"`
	ThemeColor string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main themeColor,attr,omitempty"`
}

// WmlBorder represents CT_Border (w:bdr)
type WmlBorder struct {
	Val       string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr,omitempty"`
	Color     string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main color,attr,omitempty"`
	Sz        string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main sz,attr,omitempty"`
	Space     string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main space,attr,omitempty"`
	ThemeColor string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main themeColor,attr,omitempty"`
}

// WmlShd represents CT_Shd (w:shd)
type WmlShd struct {
	Val       string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr,omitempty"`
	Color     string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main color,attr,omitempty"`
	Fill      string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main fill,attr,omitempty"`
	ThemeColor string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main themeColor,attr,omitempty"`
	ThemeFill  string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main themeFill,attr,omitempty"`
}

// WmlLang represents CT_Language (w:lang)
type WmlLang struct {
	Val      string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main val,attr,omitempty"`
	EastAsia string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main eastAsia,attr,omitempty"`
	Bidi     string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main bidi,attr,omitempty"`
}

// WmlEastAsianLayout represents CT_EastAsianLayout
type WmlEastAsianLayout struct {
	Id       string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main id,attr,omitempty"`
	Combine  string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main combine,attr,omitempty"`
	CombineBrackets string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main combineBrackets,attr,omitempty"`
	Vert     string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main vert,attr,omitempty"`
	VertCompress string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main vertCompress,attr,omitempty"`
}

// WmlMathCtrlIns represents CT_MathCtrlIns (w:ins in math context)
type WmlMathCtrlIns struct {
	Author string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main author,attr,omitempty"`
	Date   string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main date,attr,omitempty"`
	Id     string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main id,attr,omitempty"`
	Del    *WmlRPr `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main del,omitempty"`
	RPr    *WmlRPr `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main rPr,omitempty"`
}

// WmlMathCtrlDel represents CT_MathCtrlDel (w:del in math context)
type WmlMathCtrlDel struct {
	Author string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main author,attr,omitempty"`
	Date   string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main date,attr,omitempty"`
	Id     string `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main id,attr,omitempty"`
	RPr    *WmlRPr `xml:"http://schemas.openxmlformats.org/wordprocessingml/2006/main rPr,omitempty"`
}

// --- Simple value types ---

// Char represents CT_Char
type Char struct {
	Val string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math val,attr,omitempty"`
}

// TopBot represents CT_TopBot - top/bottom position
type TopBot struct {
	Val string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math val,attr,omitempty"` // top, bot
}

// Shape represents CT_Shp - delimiter shape
type Shape struct {
	Val string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math val,attr,omitempty"` // centered, match
}

// YAlign represents CT_YAlign - vertical alignment
type YAlign struct {
	Val string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math val,attr,omitempty"` // top, center, bot
}

// XAlign represents CT_XAlign - horizontal alignment
type XAlign struct {
	Val string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math val,attr,omitempty"` // left, center, right
}

// FType represents CT_FType - fraction type
type FType struct {
	Val string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math val,attr,omitempty"` // bar, skw, lin, noBar
}

// LimLoc represents CT_LimLoc - limit location
type LimLoc struct {
	Val string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math val,attr,omitempty"` // undOvr, subSup
}

// SpacingRule represents CT_SpacingRule
type SpacingRule struct {
	Val int32 `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math val,attr,omitempty"` // 0-4
}

// UnSignedInteger represents CT_UnSignedInteger
type UnSignedInteger struct {
	Val uint32 `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math val,attr,omitempty"`
}

// Integer represents CT_Integer2
type Integer struct {
	Val int32 `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math val,attr,omitempty"`
}

// --- Math Settings ---

// MathPr represents CT_MathPr - document-level math properties
type MathPr struct {
	MathFont         *MathFont  `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math mathFont,omitempty"`
	BrkBin           *BreakBin  `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math brkBin,omitempty"`
	BrkBinSub        *BreakBinSub `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math brkBinSub,omitempty"`
	SmallFrac        *OnOff     `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math smallFrac,omitempty"`
	DispDef          *OnOff     `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math dispDef,omitempty"`
	LMargin          *TwipsMeasure `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math lMargin,omitempty"`
	RMargin          *TwipsMeasure `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math rMargin,omitempty"`
	DefJc            *MathJc    `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math defJc,omitempty"`
	PreSp            *TwipsMeasure `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math preSp,omitempty"`
	PostSp           *TwipsMeasure `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math postSp,omitempty"`
	InterSp          *TwipsMeasure `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math interSp,omitempty"`
	IntraSp          *TwipsMeasure `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math intraSp,omitempty"`
	WrapIndent       *TwipsMeasure `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math wrapIndent,omitempty"`
	WrapRight        *OnOff     `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math wrapRight,omitempty"`
	IntLim           *LimLoc    `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math intLim,omitempty"`
	NaryLim          *LimLoc    `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math naryLim,omitempty"`
}

// MathFont represents CT_MathFont
type MathFont struct {
	Val string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math val,attr,omitempty"`
}

// BreakBin represents CT_BreakBin - break on binary operator
type BreakBin struct {
	Val string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math val,attr,omitempty"` // before, after, repeat
}

// BreakBinSub represents CT_BreakBinSub - break on binary subtraction
type BreakBinSub struct {
	Val string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math val,attr,omitempty"` // --, +-, -+
}

// TwipsMeasure represents CT_TwipsMeasure - measurement in twips
type TwipsMeasure struct {
	Val uint32 `xml:"http://schemas.openxmlformats.org/officeDocument/2006/math val,attr,omitempty"`
}
