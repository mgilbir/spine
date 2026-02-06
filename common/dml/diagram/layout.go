package diagram

import "github.com/mgilbir/spine/common/dml"

// --- Layout Definition (dgm:layoutDef) ---

// LayoutDef represents CT_DiagramDefinition (dgm:layoutDef) - root layout definition
type LayoutDef struct {
	UniqueId string       `xml:"uniqueId,attr,omitempty"`
	MinVer   string       `xml:"minVer,attr,omitempty"`
	DefStyle string       `xml:"defStyle,attr,omitempty"`
	Title    []*DiagTitle `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram title,omitempty"`
	Desc     []*DiagDesc  `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram desc,omitempty"`
	CatLst   *CatLst      `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram catLst,omitempty"`
	SampData *SampleData  `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram sampData,omitempty"`
	StyleData *StyleData  `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram styleData,omitempty"`
	ClrData  *CategoryData `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram clrData,omitempty"`
	LayoutNode *LayoutNode `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram layoutNode,omitempty"`
	ExtLst   *dml.ExtLst  `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram extLst,omitempty"`
}

// DiagTitle represents CT_Name (dgm:title) - localized title
type DiagTitle struct {
	Lang string `xml:"lang,attr,omitempty"`
	Val  string `xml:"val,attr"`
}

// DiagDesc represents CT_Description (dgm:desc) - localized description
type DiagDesc struct {
	Lang string `xml:"lang,attr,omitempty"`
	Val  string `xml:"val,attr"`
}

// CatLst represents CT_Categories (dgm:catLst) - list of categories
type CatLst struct {
	Cat []*Cat `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram cat,omitempty"`
}

// Cat represents CT_Category (dgm:cat)
type Cat struct {
	Type string `xml:"type,attr"`
	Pri  uint32 `xml:"pri,attr"`
}

// LayoutNode represents CT_LayoutNode (dgm:layoutNode) - layout node in tree
type LayoutNode struct {
	Name      string         `xml:"name,attr,omitempty"`
	StyleLbl  string         `xml:"styleLbl,attr,omitempty"`
	ChOrder   string         `xml:"chOrder,attr,omitempty"` // ST_ChildOrderType: b, t
	MoveWith  string         `xml:"moveWith,attr,omitempty"`
	Alg       *Algorithm     `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram alg,omitempty"`
	Shape     *LayoutShape   `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram shape,omitempty"`
	PresOf    *PresOf        `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram presOf,omitempty"`
	ConstrLst *ConstrLst     `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram constrLst,omitempty"`
	RuleLst   *RuleLst       `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram ruleLst,omitempty"`
	VarLst    *VarLst        `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram varLst,omitempty"`
	ForEach   []*ForEach     `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram forEach,omitempty"`
	LayoutNode []*LayoutNode `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram layoutNode,omitempty"`
	Choose    []*Choose      `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram choose,omitempty"`
	ExtLst    *dml.ExtLst    `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram extLst,omitempty"`
}

// Algorithm represents CT_Algorithm (dgm:alg) - layout algorithm
type Algorithm struct {
	Type   string    `xml:"type,attr"` // ST_AlgorithmType: composite, conn, cycle, hierChild, hierRoot, pyra, lin, sp, tx, snake
	Rev    uint32    `xml:"rev,attr,omitempty"`
	Param  []*Param  `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram param,omitempty"`
	ExtLst *dml.ExtLst `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram extLst,omitempty"`
}

// Param represents CT_Parameter (dgm:param) - algorithm parameter
type Param struct {
	Type string `xml:"type,attr"` // ST_ParameterID
	Val  string `xml:"val,attr"`
}

// LayoutShape represents CT_Shape (dgm:shape) - shape in layout
type LayoutShape struct {
	Rot     float64      `xml:"rot,attr,omitempty"`
	Type    string       `xml:"type,attr,omitempty"` // shape type or "conn"
	Blip    string       `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships blip,attr,omitempty"`
	ZOrderOff int32      `xml:"zOrderOff,attr,omitempty"`
	HideGeom  bool       `xml:"hideGeom,attr,omitempty"`
	LkTxEntry bool       `xml:"lkTxEntry,attr,omitempty"`
	BlipPhldr bool       `xml:"blipPhldr,attr,omitempty"`
	AdjLst    *AdjLst    `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram adjLst,omitempty"`
	ExtLst    *dml.ExtLst `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram extLst,omitempty"`
}

// AdjLst represents CT_AdjLst (dgm:adjLst) - list of shape adjustments
type AdjLst struct {
	Adj []*Adj `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram adj,omitempty"`
}

// Adj represents CT_Adj (dgm:adj) - shape adjustment value
type Adj struct {
	Idx uint32  `xml:"idx,attr"`
	Val float64 `xml:"val,attr"`
}

// PresOf represents CT_PresentationOf (dgm:presOf) - presentation association
type PresOf struct {
	Axis   string `xml:"axis,attr,omitempty"`  // ST_AxisTypes
	PtType string `xml:"ptType,attr,omitempty"` // ST_ElementTypes
	HideLastTrans string `xml:"hideLastTrans,attr,omitempty"`
	St     uint32 `xml:"st,attr,omitempty"`
	Cnt    uint32 `xml:"cnt,attr,omitempty"`
	Step   int32  `xml:"step,attr,omitempty"`
}

// ConstrLst represents CT_Constraints (dgm:constrLst) - list of constraints
type ConstrLst struct {
	Constr []*Constr `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram constr,omitempty"`
}

// Constr represents CT_Constraint (dgm:constr) - layout constraint
type Constr struct {
	Type      string  `xml:"type,attr"`      // ST_ConstraintType
	For       string  `xml:"for,attr,omitempty"` // ST_ConstraintRelationship
	ForName   string  `xml:"forName,attr,omitempty"`
	PtType    string  `xml:"ptType,attr,omitempty"` // ST_ElementType
	RefType   string  `xml:"refType,attr,omitempty"` // ST_ConstraintType
	RefFor    string  `xml:"refFor,attr,omitempty"`
	RefForName string `xml:"refForName,attr,omitempty"`
	RefPtType string  `xml:"refPtType,attr,omitempty"`
	Op        string  `xml:"op,attr,omitempty"` // ST_BoolOperator: none, equ, gte, lte
	Val       float64 `xml:"val,attr,omitempty"`
	Fact      float64 `xml:"fact,attr,omitempty"`
	ExtLst    *dml.ExtLst `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram extLst,omitempty"`
}

// RuleLst represents CT_Rules (dgm:ruleLst) - list of rules
type RuleLst struct {
	Rule []*Rule `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram rule,omitempty"`
}

// Rule represents CT_NumericRule (dgm:rule) - numeric rule
type Rule struct {
	Type    string  `xml:"type,attr"`    // ST_ConstraintType
	For     string  `xml:"for,attr,omitempty"` // ST_ConstraintRelationship
	ForName string  `xml:"forName,attr,omitempty"`
	PtType  string  `xml:"ptType,attr,omitempty"`
	Val     float64 `xml:"val,attr,omitempty"`
	Fact    float64 `xml:"fact,attr,omitempty"`
	Max     float64 `xml:"max,attr,omitempty"`
	ExtLst  *dml.ExtLst `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram extLst,omitempty"`
}

// VarLst represents CT_LayoutVariablePropertySet (dgm:varLst) - variable list
type VarLst struct {
	OrgChart   *BoolVal `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram orgChart,omitempty"`
	ChMax      *IntVal  `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram chMax,omitempty"`
	ChPref     *IntVal  `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram chPref,omitempty"`
	BulletEnabled *BoolVal `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram bulletEnabled,omitempty"`
	Dir        *DirVal  `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram dir,omitempty"`
	AnimOne    *AnimOneVal `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram animOne,omitempty"`
	AnimLvl    *AnimLvlVal `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram animLvl,omitempty"`
	ResizeHandles *ResizeHandlesVal `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram resizeHandles,omitempty"`
}

// ForEach represents CT_ForEach (dgm:forEach) - iteration over data points
type ForEach struct {
	Name      string         `xml:"name,attr,omitempty"`
	Ref       string         `xml:"ref,attr,omitempty"`
	Axis      string         `xml:"axis,attr,omitempty"`  // ST_AxisTypes
	PtType    string         `xml:"ptType,attr,omitempty"` // ST_ElementTypes
	HideLastTrans string    `xml:"hideLastTrans,attr,omitempty"`
	St        uint32         `xml:"st,attr,omitempty"`
	Cnt       uint32         `xml:"cnt,attr,omitempty"`
	Step      int32          `xml:"step,attr,omitempty"`
	Alg       *Algorithm     `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram alg,omitempty"`
	Shape     *LayoutShape   `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram shape,omitempty"`
	PresOf    *PresOf        `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram presOf,omitempty"`
	ConstrLst *ConstrLst     `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram constrLst,omitempty"`
	RuleLst   *RuleLst       `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram ruleLst,omitempty"`
	ForEach   []*ForEach     `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram forEach,omitempty"`
	LayoutNode []*LayoutNode `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram layoutNode,omitempty"`
	Choose    []*Choose      `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram choose,omitempty"`
	ExtLst    *dml.ExtLst    `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram extLst,omitempty"`
}

// Choose represents CT_Choose (dgm:choose) - conditional branching
type Choose struct {
	Name string `xml:"name,attr,omitempty"`
	If   []*If  `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram if,omitempty"`
	Else *Else  `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram else,omitempty"`
}

// If represents CT_When (dgm:if) - conditional branch
type If struct {
	Name      string         `xml:"name,attr,omitempty"`
	Axis      string         `xml:"axis,attr,omitempty"`
	PtType    string         `xml:"ptType,attr,omitempty"`
	Func      string         `xml:"func,attr"` // ST_FunctionType: cnt, pos, revPos, posEven, posOdd, var, depth, maxDepth
	Arg       string         `xml:"arg,attr,omitempty"` // ST_FunctionArgument
	Op        string         `xml:"op,attr"`  // ST_FunctionOperator: equ, neq, gt, lt, gte, lte
	Val       string         `xml:"val,attr"`
	HideLastTrans string    `xml:"hideLastTrans,attr,omitempty"`
	St        uint32         `xml:"st,attr,omitempty"`
	Cnt       uint32         `xml:"cnt,attr,omitempty"`
	Step      int32          `xml:"step,attr,omitempty"`
	Alg       *Algorithm     `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram alg,omitempty"`
	Shape     *LayoutShape   `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram shape,omitempty"`
	PresOf    *PresOf        `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram presOf,omitempty"`
	ConstrLst *ConstrLst     `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram constrLst,omitempty"`
	RuleLst   *RuleLst       `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram ruleLst,omitempty"`
	ForEach   []*ForEach     `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram forEach,omitempty"`
	LayoutNode []*LayoutNode `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram layoutNode,omitempty"`
	Choose    []*Choose      `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram choose,omitempty"`
	ExtLst    *dml.ExtLst    `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram extLst,omitempty"`
}

// Else represents CT_Otherwise (dgm:else) - default branch
type Else struct {
	Name       string         `xml:"name,attr,omitempty"`
	Alg        *Algorithm     `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram alg,omitempty"`
	Shape      *LayoutShape   `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram shape,omitempty"`
	PresOf     *PresOf        `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram presOf,omitempty"`
	ConstrLst  *ConstrLst     `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram constrLst,omitempty"`
	RuleLst    *RuleLst       `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram ruleLst,omitempty"`
	ForEach    []*ForEach     `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram forEach,omitempty"`
	LayoutNode []*LayoutNode  `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram layoutNode,omitempty"`
	Choose     []*Choose      `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram choose,omitempty"`
	ExtLst     *dml.ExtLst    `xml:"http://schemas.openxmlformats.org/drawingml/2006/diagram extLst,omitempty"`
}
