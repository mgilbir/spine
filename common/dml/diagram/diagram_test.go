package diagram

import (
	"encoding/xml"
	"testing"

	"github.com/mgilbir/spine/common/dml"
)

// helper to round-trip marshal/unmarshal and verify
func roundTrip[T any](t *testing.T, name string, v T) T {
	t.Helper()
	data, err := xml.Marshal(v)
	if err != nil {
		t.Fatalf("%s: marshal error: %v", name, err)
	}
	var result T
	if err := xml.Unmarshal(data, &result); err != nil {
		t.Fatalf("%s: unmarshal error: %v\nXML: %s", name, err, string(data))
	}
	return result
}

// --- Data Model Tests ---

func TestDataModel(t *testing.T) {
	dm := DataModel{
		PtLst: &PtLst{
			Pt: []*Pt{
				{
					ModelId: "0",
					Type:    "doc",
					PrSet: &PrSet{
						PhldrT: "Document",
						Phldr:  true,
					},
					T: &dml.TxBody{
						BodyPr: &dml.BodyPr{},
						P: []*dml.P{
							{R: []*dml.R{{T: "Root"}}},
						},
					},
				},
				{
					ModelId: "1",
					Type:    "node",
					PrSet: &PrSet{
						CustAng: 90,
					},
				},
				{
					ModelId: "2",
					Type:    "sibTrans",
				},
			},
		},
		CxnLst: &CxnLst{
			Cxn: []*Cxn{
				{
					ModelId:    "10",
					Type:       "parOf",
					SrcId:      "0",
					DestId:     "1",
					SrcOrd:     0,
					DestOrd:    0,
					ParTransId: "2",
				},
			},
		},
	}

	result := roundTrip(t, "DataModel", dm)

	if len(result.PtLst.Pt) != 3 {
		t.Errorf("expected 3 points, got %d", len(result.PtLst.Pt))
	}
	if result.PtLst.Pt[0].ModelId != "0" {
		t.Errorf("expected modelId '0', got %q", result.PtLst.Pt[0].ModelId)
	}
	if result.PtLst.Pt[0].Type != "doc" {
		t.Errorf("expected type 'doc', got %q", result.PtLst.Pt[0].Type)
	}
	if result.PtLst.Pt[0].PrSet.PhldrT != "Document" {
		t.Errorf("expected phldrT 'Document', got %q", result.PtLst.Pt[0].PrSet.PhldrT)
	}
	if len(result.CxnLst.Cxn) != 1 {
		t.Errorf("expected 1 connection, got %d", len(result.CxnLst.Cxn))
	}
	if result.CxnLst.Cxn[0].SrcId != "0" {
		t.Errorf("expected srcId '0', got %q", result.CxnLst.Cxn[0].SrcId)
	}
	if result.CxnLst.Cxn[0].ParTransId != "2" {
		t.Errorf("expected parTransId '2', got %q", result.CxnLst.Cxn[0].ParTransId)
	}
}

func TestPt(t *testing.T) {
	pt := Pt{
		ModelId: "5",
		Type:    "asst",
		CxnId:   "3",
		PrSet: &PrSet{
			PresAssocID:  "assoc1",
			PresName:     "node1",
			PresStyleLbl: "vennNode1",
			PresStyleIdx: 0,
			PresStyleCnt: 4,
			CustFlipVert: true,
			CustFlipHor:  false,
			CustSzX:      100,
			CustSzY:      200,
			CustScaleX:   50,
			CustScaleY:   75,
		},
		SpPr: &dml.SpPr{
			PrstGeom: &dml.PrstGeom{Prst: "rect"},
		},
	}

	result := roundTrip(t, "Pt", pt)

	if result.ModelId != "5" {
		t.Errorf("expected modelId '5', got %q", result.ModelId)
	}
	if result.Type != "asst" {
		t.Errorf("expected type 'asst', got %q", result.Type)
	}
	if result.CxnId != "3" {
		t.Errorf("expected cxnId '3', got %q", result.CxnId)
	}
	if result.PrSet.PresStyleLbl != "vennNode1" {
		t.Errorf("expected presStyleLbl 'vennNode1', got %q", result.PrSet.PresStyleLbl)
	}
	if result.PrSet.CustFlipVert != true {
		t.Error("expected custFlipVert true")
	}
	if result.PrSet.CustSzX != 100 {
		t.Errorf("expected custSzX 100, got %d", result.PrSet.CustSzX)
	}
}

func TestCxn(t *testing.T) {
	cxn := Cxn{
		ModelId:    "20",
		Type:       "presOf",
		SrcId:      "0",
		DestId:     "1",
		SrcOrd:     1,
		DestOrd:    2,
		SibTransId: "5",
		PresId:     "pres1",
	}

	result := roundTrip(t, "Cxn", cxn)

	if result.ModelId != "20" {
		t.Errorf("expected modelId '20', got %q", result.ModelId)
	}
	if result.Type != "presOf" {
		t.Errorf("expected type 'presOf', got %q", result.Type)
	}
	if result.SrcOrd != 1 {
		t.Errorf("expected srcOrd 1, got %d", result.SrcOrd)
	}
	if result.DestOrd != 2 {
		t.Errorf("expected destOrd 2, got %d", result.DestOrd)
	}
	if result.SibTransId != "5" {
		t.Errorf("expected sibTransId '5', got %q", result.SibTransId)
	}
}

func TestSampleData(t *testing.T) {
	sd := SampleData{
		UseDef: true,
		DataModel: &DataModel{
			PtLst: &PtLst{
				Pt: []*Pt{{ModelId: "0", Type: "doc"}},
			},
		},
	}

	result := roundTrip(t, "SampleData", sd)

	if result.UseDef != true {
		t.Error("expected useDef true")
	}
	if len(result.DataModel.PtLst.Pt) != 1 {
		t.Errorf("expected 1 point, got %d", len(result.DataModel.PtLst.Pt))
	}
}

// --- Layout Definition Tests ---

func TestLayoutDef(t *testing.T) {
	ld := LayoutDef{
		UniqueId: "urn:microsoft.com/office/officeart/2005/8/layout/venn1",
		MinVer:   "http://schemas.openxmlformats.org/drawingml/2006/diagram",
		Title: []*DiagTitle{
			{Lang: "en-US", Val: "Basic Venn"},
		},
		Desc: []*DiagDesc{
			{Lang: "en-US", Val: "Overlapping circles to show relationships"},
		},
		CatLst: &CatLst{
			Cat: []*Cat{
				{Type: "relationship", Pri: 1000},
			},
		},
		LayoutNode: &LayoutNode{
			Name: "root",
			Alg: &Algorithm{
				Type: "composite",
			},
			ConstrLst: &ConstrLst{
				Constr: []*Constr{
					{Type: "w", For: "ch", Val: 100},
					{Type: "h", For: "ch", Val: 100},
				},
			},
		},
	}

	result := roundTrip(t, "LayoutDef", ld)

	if result.UniqueId != "urn:microsoft.com/office/officeart/2005/8/layout/venn1" {
		t.Errorf("unexpected uniqueId: %q", result.UniqueId)
	}
	if len(result.Title) != 1 || result.Title[0].Val != "Basic Venn" {
		t.Error("title mismatch")
	}
	if len(result.Desc) != 1 || result.Desc[0].Val != "Overlapping circles to show relationships" {
		t.Error("desc mismatch")
	}
	if result.CatLst.Cat[0].Type != "relationship" {
		t.Errorf("expected cat type 'relationship', got %q", result.CatLst.Cat[0].Type)
	}
	if result.LayoutNode.Alg.Type != "composite" {
		t.Errorf("expected alg type 'composite', got %q", result.LayoutNode.Alg.Type)
	}
}

func TestLayoutNode(t *testing.T) {
	ln := LayoutNode{
		Name:     "node1",
		StyleLbl: "node1",
		ChOrder:  "b",
		Alg: &Algorithm{
			Type: "tx",
			Rev:  1,
			Param: []*Param{
				{Type: "txAnchorVert", Val: "t"},
				{Type: "parTxLTRAlign", Val: "l"},
			},
		},
		Shape: &LayoutShape{
			Type:     "ellipse",
			HideGeom: false,
		},
		PresOf: &PresOf{
			Axis:   "self",
			PtType: "node",
		},
		ConstrLst: &ConstrLst{
			Constr: []*Constr{
				{Type: "primFontSz", Val: 65},
			},
		},
		VarLst: &VarLst{
			ChMax: &IntVal{Val: 0},
			Dir:   &DirVal{Val: "norm"},
		},
	}

	result := roundTrip(t, "LayoutNode", ln)

	if result.Name != "node1" {
		t.Errorf("expected name 'node1', got %q", result.Name)
	}
	if result.ChOrder != "b" {
		t.Errorf("expected chOrder 'b', got %q", result.ChOrder)
	}
	if result.Alg.Type != "tx" {
		t.Errorf("expected alg type 'tx', got %q", result.Alg.Type)
	}
	if len(result.Alg.Param) != 2 {
		t.Errorf("expected 2 params, got %d", len(result.Alg.Param))
	}
	if result.Shape.Type != "ellipse" {
		t.Errorf("expected shape type 'ellipse', got %q", result.Shape.Type)
	}
	if result.PresOf.Axis != "self" {
		t.Errorf("expected axis 'self', got %q", result.PresOf.Axis)
	}
	if result.VarLst.ChMax.Val != 0 {
		t.Errorf("expected chMax 0, got %d", result.VarLst.ChMax.Val)
	}
}

func TestAlgorithm(t *testing.T) {
	alg := Algorithm{
		Type: "snake",
		Rev:  0,
		Param: []*Param{
			{Type: "grDir", Val: "tL"},
			{Type: "flowDir", Val: "row"},
			{Type: "contDir", Val: "sameDir"},
			{Type: "off", Val: "ctr"},
		},
	}

	result := roundTrip(t, "Algorithm", alg)

	if result.Type != "snake" {
		t.Errorf("expected type 'snake', got %q", result.Type)
	}
	if len(result.Param) != 4 {
		t.Errorf("expected 4 params, got %d", len(result.Param))
	}
	if result.Param[0].Type != "grDir" || result.Param[0].Val != "tL" {
		t.Error("param[0] mismatch")
	}
}

func TestConstr(t *testing.T) {
	constr := Constr{
		Type:      "w",
		For:       "ch",
		ForName:   "node1",
		PtType:    "node",
		RefType:   "w",
		RefFor:    "ch",
		RefForName: "node2",
		Op:        "equ",
		Val:       0,
		Fact:      0.5,
	}

	result := roundTrip(t, "Constr", constr)

	if result.Type != "w" {
		t.Errorf("expected type 'w', got %q", result.Type)
	}
	if result.For != "ch" {
		t.Errorf("expected for 'ch', got %q", result.For)
	}
	if result.ForName != "node1" {
		t.Errorf("expected forName 'node1', got %q", result.ForName)
	}
	if result.RefType != "w" {
		t.Errorf("expected refType 'w', got %q", result.RefType)
	}
	if result.Fact != 0.5 {
		t.Errorf("expected fact 0.5, got %f", result.Fact)
	}
}

func TestRule(t *testing.T) {
	rule := Rule{
		Type:    "primFontSz",
		For:     "des",
		ForName: "text1",
		PtType:  "node",
		Val:     5,
		Fact:    1,
		Max:     100,
	}

	result := roundTrip(t, "Rule", rule)

	if result.Type != "primFontSz" {
		t.Errorf("expected type 'primFontSz', got %q", result.Type)
	}
	if result.Max != 100 {
		t.Errorf("expected max 100, got %f", result.Max)
	}
}

func TestForEach(t *testing.T) {
	fe := ForEach{
		Name:   "Name0",
		Axis:   "ch",
		PtType: "node",
		St:     1,
		Cnt:    0,
		Step:   1,
		LayoutNode: []*LayoutNode{
			{
				Name: "childNode",
				Alg:  &Algorithm{Type: "tx"},
				Shape: &LayoutShape{Type: "rect"},
			},
		},
	}

	result := roundTrip(t, "ForEach", fe)

	if result.Name != "Name0" {
		t.Errorf("expected name 'Name0', got %q", result.Name)
	}
	if result.Axis != "ch" {
		t.Errorf("expected axis 'ch', got %q", result.Axis)
	}
	if result.PtType != "node" {
		t.Errorf("expected ptType 'node', got %q", result.PtType)
	}
	if len(result.LayoutNode) != 1 {
		t.Errorf("expected 1 layout node, got %d", len(result.LayoutNode))
	}
	if result.LayoutNode[0].Name != "childNode" {
		t.Errorf("expected child name 'childNode', got %q", result.LayoutNode[0].Name)
	}
}

func TestChoose(t *testing.T) {
	choose := Choose{
		Name: "Name1",
		If: []*If{
			{
				Name:   "Name2",
				Func:   "cnt",
				Arg:    "ch",
				Op:     "equ",
				Val:    "1",
				Axis:   "ch",
				PtType: "node",
				LayoutNode: []*LayoutNode{
					{Name: "single", Alg: &Algorithm{Type: "tx"}},
				},
			},
		},
		Else: &Else{
			Name: "Name3",
			LayoutNode: []*LayoutNode{
				{Name: "multi", Alg: &Algorithm{Type: "lin"}},
			},
		},
	}

	result := roundTrip(t, "Choose", choose)

	if result.Name != "Name1" {
		t.Errorf("expected name 'Name1', got %q", result.Name)
	}
	if len(result.If) != 1 {
		t.Fatalf("expected 1 if, got %d", len(result.If))
	}
	if result.If[0].Func != "cnt" {
		t.Errorf("expected func 'cnt', got %q", result.If[0].Func)
	}
	if result.If[0].Op != "equ" {
		t.Errorf("expected op 'equ', got %q", result.If[0].Op)
	}
	if result.If[0].Val != "1" {
		t.Errorf("expected val '1', got %q", result.If[0].Val)
	}
	if result.Else == nil {
		t.Fatal("expected else branch")
	}
	if result.Else.Name != "Name3" {
		t.Errorf("expected else name 'Name3', got %q", result.Else.Name)
	}
	if len(result.Else.LayoutNode) != 1 {
		t.Errorf("expected 1 else layout node, got %d", len(result.Else.LayoutNode))
	}
}

func TestLayoutShape(t *testing.T) {
	ls := LayoutShape{
		Rot:       45.5,
		Type:      "roundRect",
		ZOrderOff: 2,
		HideGeom:  true,
		LkTxEntry: true,
		BlipPhldr: false,
		AdjLst: &AdjLst{
			Adj: []*Adj{
				{Idx: 1, Val: 0.25},
				{Idx: 2, Val: 0.75},
			},
		},
	}

	result := roundTrip(t, "LayoutShape", ls)

	if result.Rot != 45.5 {
		t.Errorf("expected rot 45.5, got %f", result.Rot)
	}
	if result.Type != "roundRect" {
		t.Errorf("expected type 'roundRect', got %q", result.Type)
	}
	if result.ZOrderOff != 2 {
		t.Errorf("expected zOrderOff 2, got %d", result.ZOrderOff)
	}
	if !result.HideGeom {
		t.Error("expected hideGeom true")
	}
	if len(result.AdjLst.Adj) != 2 {
		t.Errorf("expected 2 adjustments, got %d", len(result.AdjLst.Adj))
	}
	if result.AdjLst.Adj[0].Val != 0.25 {
		t.Errorf("expected adj[0] val 0.25, got %f", result.AdjLst.Adj[0].Val)
	}
}

func TestVarLst(t *testing.T) {
	vl := VarLst{
		OrgChart:      &BoolVal{Val: true},
		ChMax:         &IntVal{Val: 5},
		ChPref:        &IntVal{Val: 3},
		BulletEnabled: &BoolVal{Val: false},
		Dir:           &DirVal{Val: "rev"},
		AnimOne:       &AnimOneVal{Val: "branch"},
		AnimLvl:       &AnimLvlVal{Val: "lvl"},
		ResizeHandles: &ResizeHandlesVal{Val: "exact"},
	}

	result := roundTrip(t, "VarLst", vl)

	if !result.OrgChart.Val {
		t.Error("expected orgChart true")
	}
	if result.ChMax.Val != 5 {
		t.Errorf("expected chMax 5, got %d", result.ChMax.Val)
	}
	if result.ChPref.Val != 3 {
		t.Errorf("expected chPref 3, got %d", result.ChPref.Val)
	}
	if result.Dir.Val != "rev" {
		t.Errorf("expected dir 'rev', got %q", result.Dir.Val)
	}
	if result.AnimOne.Val != "branch" {
		t.Errorf("expected animOne 'branch', got %q", result.AnimOne.Val)
	}
	if result.AnimLvl.Val != "lvl" {
		t.Errorf("expected animLvl 'lvl', got %q", result.AnimLvl.Val)
	}
	if result.ResizeHandles.Val != "exact" {
		t.Errorf("expected resizeHandles 'exact', got %q", result.ResizeHandles.Val)
	}
}

// --- Color Transform Tests ---

func TestColorsDef(t *testing.T) {
	cd := ColorsDef{
		UniqueId: "urn:microsoft.com/office/officeart/2005/8/colors/accent1_2",
		MinVer:   "http://schemas.openxmlformats.org/drawingml/2006/diagram",
		Title: []*DiagTitle{
			{Lang: "en-US", Val: "Colorful - Accent Colors"},
		},
		Desc: []*DiagDesc{
			{Lang: "en-US", Val: "Colors 1 to 4"},
		},
		CatLst: &CatLst{
			Cat: []*Cat{
				{Type: "colorful", Pri: 10100},
			},
		},
		StyleLbl: []*CTStyleLabel{
			{
				Name: "node0",
				FillClrLst: &ColorList{
					Meth:   "repeat",
					HueDir: "cw",
					SchemeClr: []*dml.SchemeClrTransform{
						{Val: "accent1"},
						{Val: "accent2"},
					},
				},
				LinClrLst: &ColorList{
					Meth: "repeat",
					SchemeClr: []*dml.SchemeClrTransform{
						{Val: "lt1"},
					},
				},
				EffectClrLst: &ColorList{
					Meth: "repeat",
				},
				TxLinClrLst: &ColorList{
					Meth: "repeat",
				},
				TxFillClrLst: &ColorList{
					Meth: "repeat",
					SchemeClr: []*dml.SchemeClrTransform{
						{Val: "lt1"},
					},
				},
				TxEffectClrLst: &ColorList{
					Meth: "repeat",
				},
			},
		},
	}

	result := roundTrip(t, "ColorsDef", cd)

	if result.UniqueId != "urn:microsoft.com/office/officeart/2005/8/colors/accent1_2" {
		t.Errorf("unexpected uniqueId: %q", result.UniqueId)
	}
	if len(result.StyleLbl) != 1 {
		t.Fatalf("expected 1 style label, got %d", len(result.StyleLbl))
	}
	sl := result.StyleLbl[0]
	if sl.Name != "node0" {
		t.Errorf("expected name 'node0', got %q", sl.Name)
	}
	if sl.FillClrLst.Meth != "repeat" {
		t.Errorf("expected meth 'repeat', got %q", sl.FillClrLst.Meth)
	}
	if sl.FillClrLst.HueDir != "cw" {
		t.Errorf("expected hueDir 'cw', got %q", sl.FillClrLst.HueDir)
	}
	if len(sl.FillClrLst.SchemeClr) != 2 {
		t.Errorf("expected 2 scheme colors, got %d", len(sl.FillClrLst.SchemeClr))
	}
	if sl.FillClrLst.SchemeClr[0].Val != "accent1" {
		t.Errorf("expected scheme color 'accent1', got %q", sl.FillClrLst.SchemeClr[0].Val)
	}
}

func TestColorList(t *testing.T) {
	cl := ColorList{
		Meth:   "cycle",
		HueDir: "ccw",
		SrgbClr: []*dml.SrgbClr{
			{Val: "FF0000"},
			{Val: "00FF00"},
			{Val: "0000FF"},
		},
	}

	result := roundTrip(t, "ColorList", cl)

	if result.Meth != "cycle" {
		t.Errorf("expected meth 'cycle', got %q", result.Meth)
	}
	if result.HueDir != "ccw" {
		t.Errorf("expected hueDir 'ccw', got %q", result.HueDir)
	}
	if len(result.SrgbClr) != 3 {
		t.Errorf("expected 3 srgb colors, got %d", len(result.SrgbClr))
	}
	if result.SrgbClr[1].Val != "00FF00" {
		t.Errorf("expected '00FF00', got %q", result.SrgbClr[1].Val)
	}
}

func TestColorsDefHdr(t *testing.T) {
	hdr := ColorsDefHdr{
		UniqueId: "urn:test:colors1",
		MinVer:   "http://schemas.openxmlformats.org/drawingml/2006/diagram",
		ResId:    "res1",
		Title:    []*DiagTitle{{Lang: "en-US", Val: "Test Colors"}},
		Desc:     []*DiagDesc{{Lang: "en-US", Val: "Description"}},
	}

	result := roundTrip(t, "ColorsDefHdr", hdr)

	if result.UniqueId != "urn:test:colors1" {
		t.Errorf("expected uniqueId 'urn:test:colors1', got %q", result.UniqueId)
	}
	if result.ResId != "res1" {
		t.Errorf("expected resId 'res1', got %q", result.ResId)
	}
}

// --- Style Definition Tests ---

func TestStyleDef(t *testing.T) {
	sd := StyleDef{
		UniqueId: "urn:microsoft.com/office/officeart/2005/8/quickstyle/simple1",
		MinVer:   "http://schemas.openxmlformats.org/drawingml/2006/diagram",
		Title: []*DiagTitle{
			{Lang: "en-US", Val: "Simple Fill"},
		},
		Desc: []*DiagDesc{
			{Lang: "en-US", Val: "Simple fill with subtle line"},
		},
		CatLst: &CatLst{
			Cat: []*Cat{
				{Type: "simple", Pri: 10100},
			},
		},
		StyleLbl: []*StyleDefLabel{
			{
				Name: "node0",
				Style: &DiagStyle{
					LnRef: &StyleMatrixRef{
						Idx: 2,
						SchemeClr: &dml.SchemeClrTransform{Val: "accent1"},
					},
					FillRef: &StyleMatrixRef{
						Idx: 1,
						SchemeClr: &dml.SchemeClrTransform{Val: "accent1"},
					},
					EffectRef: &StyleMatrixRef{
						Idx: 0,
						SchemeClr: &dml.SchemeClrTransform{Val: "accent1"},
					},
					FontRef: &FontReference{
						Idx:       "minor",
						SchemeClr: &dml.SchemeClrTransform{Val: "lt1"},
					},
				},
			},
		},
	}

	result := roundTrip(t, "StyleDef", sd)

	if result.UniqueId != "urn:microsoft.com/office/officeart/2005/8/quickstyle/simple1" {
		t.Errorf("unexpected uniqueId: %q", result.UniqueId)
	}
	if len(result.StyleLbl) != 1 {
		t.Fatalf("expected 1 style label, got %d", len(result.StyleLbl))
	}
	sl := result.StyleLbl[0]
	if sl.Name != "node0" {
		t.Errorf("expected name 'node0', got %q", sl.Name)
	}
	if sl.Style.LnRef.Idx != 2 {
		t.Errorf("expected lnRef idx 2, got %d", sl.Style.LnRef.Idx)
	}
	if sl.Style.LnRef.SchemeClr.Val != "accent1" {
		t.Errorf("expected scheme color 'accent1', got %q", sl.Style.LnRef.SchemeClr.Val)
	}
	if sl.Style.FontRef.Idx != "minor" {
		t.Errorf("expected fontRef idx 'minor', got %q", sl.Style.FontRef.Idx)
	}
}

func TestStyleMatrixRef(t *testing.T) {
	ref := StyleMatrixRef{
		Idx:     3,
		SrgbClr: &dml.SrgbClr{Val: "4F81BD"},
	}

	result := roundTrip(t, "StyleMatrixRef", ref)

	if result.Idx != 3 {
		t.Errorf("expected idx 3, got %d", result.Idx)
	}
	if result.SrgbClr.Val != "4F81BD" {
		t.Errorf("expected srgb '4F81BD', got %q", result.SrgbClr.Val)
	}
}

func TestFontReference(t *testing.T) {
	ref := FontReference{
		Idx:       "major",
		SchemeClr: &dml.SchemeClrTransform{Val: "dk1"},
	}

	result := roundTrip(t, "FontReference", ref)

	if result.Idx != "major" {
		t.Errorf("expected idx 'major', got %q", result.Idx)
	}
	if result.SchemeClr.Val != "dk1" {
		t.Errorf("expected scheme color 'dk1', got %q", result.SchemeClr.Val)
	}
}

func TestStyleDefHdr(t *testing.T) {
	hdr := StyleDefHdr{
		UniqueId: "urn:test:style1",
		MinVer:   "http://schemas.openxmlformats.org/drawingml/2006/diagram",
		ResId:    "sRes1",
		Title:    []*DiagTitle{{Lang: "en-US", Val: "Test Style"}},
	}

	result := roundTrip(t, "StyleDefHdr", hdr)

	if result.UniqueId != "urn:test:style1" {
		t.Errorf("expected uniqueId 'urn:test:style1', got %q", result.UniqueId)
	}
	if result.ResId != "sRes1" {
		t.Errorf("expected resId 'sRes1', got %q", result.ResId)
	}
}

func TestStyleDefLabel(t *testing.T) {
	sl := StyleDefLabel{
		Name: "fgAcc1",
		TxPr: &DiagTxPr{
			Sp3d: &dml.Sp3d{Z: 100},
		},
		Style: &DiagStyle{
			LnRef: &StyleMatrixRef{Idx: 1, SchemeClr: &dml.SchemeClrTransform{Val: "accent2"}},
			FillRef: &StyleMatrixRef{Idx: 1, SchemeClr: &dml.SchemeClrTransform{Val: "accent2"}},
		},
	}

	result := roundTrip(t, "StyleDefLabel", sl)

	if result.Name != "fgAcc1" {
		t.Errorf("expected name 'fgAcc1', got %q", result.Name)
	}
	if result.TxPr.Sp3d.Z != 100 {
		t.Errorf("expected sp3d z 100, got %d", result.TxPr.Sp3d.Z)
	}
	if result.Style.LnRef.SchemeClr.Val != "accent2" {
		t.Errorf("expected lnRef scheme 'accent2', got %q", result.Style.LnRef.SchemeClr.Val)
	}
}

// --- Simple Value Type Tests ---

func TestBoolVal(t *testing.T) {
	bv := BoolVal{Val: true}
	result := roundTrip(t, "BoolVal", bv)
	if !result.Val {
		t.Error("expected true")
	}
}

func TestIntVal(t *testing.T) {
	iv := IntVal{Val: 42}
	result := roundTrip(t, "IntVal", iv)
	if result.Val != 42 {
		t.Errorf("expected 42, got %d", result.Val)
	}
}

func TestDirVal(t *testing.T) {
	dv := DirVal{Val: "norm"}
	result := roundTrip(t, "DirVal", dv)
	if result.Val != "norm" {
		t.Errorf("expected 'norm', got %q", result.Val)
	}
}

func TestAnimOneVal(t *testing.T) {
	av := AnimOneVal{Val: "one"}
	result := roundTrip(t, "AnimOneVal", av)
	if result.Val != "one" {
		t.Errorf("expected 'one', got %q", result.Val)
	}
}

func TestAnimLvlVal(t *testing.T) {
	av := AnimLvlVal{Val: "ctr"}
	result := roundTrip(t, "AnimLvlVal", av)
	if result.Val != "ctr" {
		t.Errorf("expected 'ctr', got %q", result.Val)
	}
}

func TestResizeHandlesVal(t *testing.T) {
	rv := ResizeHandlesVal{Val: "rel"}
	result := roundTrip(t, "ResizeHandlesVal", rv)
	if result.Val != "rel" {
		t.Errorf("expected 'rel', got %q", result.Val)
	}
}

// --- PrSet Tests ---

func TestPrSet(t *testing.T) {
	ps := PrSet{
		PresAssocID:  "assoc1",
		PresName:     "nodePres",
		PresStyleLbl: "style1",
		PresStyleIdx: 2,
		PresStyleCnt: 5,
		LoTypeId:     "urn:layout:basic",
		QsTypeId:     "urn:qs:simple",
		CsTypeId:     "urn:cs:accent1",
		Coherent3DOff: true,
		PhldrT:       "Placeholder",
		Phldr:        true,
		CustAng:      180,
		CustFlipVert: true,
		CustFlipHor:  true,
		CustSzX:      500,
		CustSzY:      300,
		CustScaleX:   120,
		CustScaleY:   80,
		CustLinFactX: 10,
		CustLinFactY: 20,
	}

	result := roundTrip(t, "PrSet", ps)

	if result.PresAssocID != "assoc1" {
		t.Errorf("expected presAssocID 'assoc1', got %q", result.PresAssocID)
	}
	if result.PresStyleIdx != 2 {
		t.Errorf("expected presStyleIdx 2, got %d", result.PresStyleIdx)
	}
	if result.PresStyleCnt != 5 {
		t.Errorf("expected presStyleCnt 5, got %d", result.PresStyleCnt)
	}
	if !result.Coherent3DOff {
		t.Error("expected coherent3DOff true")
	}
	if result.CustAng != 180 {
		t.Errorf("expected custAng 180, got %d", result.CustAng)
	}
	if result.CustSzX != 500 {
		t.Errorf("expected custSzX 500, got %d", result.CustSzX)
	}
}

// --- PresOf Tests ---

func TestPresOf(t *testing.T) {
	po := PresOf{
		Axis:          "ch",
		PtType:        "node",
		HideLastTrans: "1",
		St:            2,
		Cnt:           5,
		Step:          1,
	}

	result := roundTrip(t, "PresOf", po)

	if result.Axis != "ch" {
		t.Errorf("expected axis 'ch', got %q", result.Axis)
	}
	if result.PtType != "node" {
		t.Errorf("expected ptType 'node', got %q", result.PtType)
	}
	if result.St != 2 {
		t.Errorf("expected st 2, got %d", result.St)
	}
	if result.Cnt != 5 {
		t.Errorf("expected cnt 5, got %d", result.Cnt)
	}
}

// --- RelIds Tests ---

func TestRelIds(t *testing.T) {
	ri := RelIds{
		Dm: "rId1",
		Lo: "rId2",
		Qs: "rId3",
		Cs: "rId4",
	}

	result := roundTrip(t, "RelIds", ri)

	if result.Dm != "rId1" {
		t.Errorf("expected dm 'rId1', got %q", result.Dm)
	}
	if result.Lo != "rId2" {
		t.Errorf("expected lo 'rId2', got %q", result.Lo)
	}
	if result.Qs != "rId3" {
		t.Errorf("expected qs 'rId3', got %q", result.Qs)
	}
	if result.Cs != "rId4" {
		t.Errorf("expected cs 'rId4', got %q", result.Cs)
	}
}

// --- Integration test: Full Layout with nested ForEach and Choose ---

func TestNestedLayoutStructure(t *testing.T) {
	ld := LayoutDef{
		UniqueId: "urn:test:nested",
		LayoutNode: &LayoutNode{
			Name: "root",
			Alg:  &Algorithm{Type: "composite"},
			ConstrLst: &ConstrLst{
				Constr: []*Constr{
					{Type: "w", For: "ch", ForName: "composite", Val: 1},
					{Type: "h", For: "ch", ForName: "composite", Val: 1},
				},
			},
			LayoutNode: []*LayoutNode{
				{
					Name: "composite",
					Alg:  &Algorithm{Type: "composite"},
					Choose: []*Choose{
						{
							Name: "maxDepthCheck",
							If: []*If{
								{
									Name: "hasChildren",
									Func: "cnt",
									Arg:  "ch",
									Op:   "gt",
									Val:  "0",
									ForEach: []*ForEach{
										{
											Name:   "childIter",
											Axis:   "ch",
											PtType: "node",
											LayoutNode: []*LayoutNode{
												{
													Name:  "childNode",
													Alg:   &Algorithm{Type: "tx"},
													Shape: &LayoutShape{Type: "rect"},
													PresOf: &PresOf{Axis: "self", PtType: "node"},
												},
											},
										},
									},
								},
							},
							Else: &Else{
								Name: "noChildren",
								LayoutNode: []*LayoutNode{
									{Name: "emptyNode", Alg: &Algorithm{Type: "sp"}},
								},
							},
						},
					},
				},
			},
		},
	}

	result := roundTrip(t, "NestedLayout", ld)

	if result.LayoutNode.Name != "root" {
		t.Errorf("expected root name 'root', got %q", result.LayoutNode.Name)
	}
	if len(result.LayoutNode.LayoutNode) != 1 {
		t.Fatal("expected 1 child layout node")
	}
	composite := result.LayoutNode.LayoutNode[0]
	if composite.Name != "composite" {
		t.Errorf("expected 'composite', got %q", composite.Name)
	}
	if len(composite.Choose) != 1 {
		t.Fatal("expected 1 choose")
	}
	ch := composite.Choose[0]
	if len(ch.If) != 1 {
		t.Fatal("expected 1 if branch")
	}
	if ch.If[0].Func != "cnt" {
		t.Errorf("expected func 'cnt', got %q", ch.If[0].Func)
	}
	if len(ch.If[0].ForEach) != 1 {
		t.Fatal("expected 1 forEach in if")
	}
	if ch.If[0].ForEach[0].Name != "childIter" {
		t.Errorf("expected forEach name 'childIter', got %q", ch.If[0].ForEach[0].Name)
	}
	if ch.Else.Name != "noChildren" {
		t.Errorf("expected else name 'noChildren', got %q", ch.Else.Name)
	}
}
