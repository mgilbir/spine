package spectest

import (
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/mgilbir/spine/common/omml"
	xmlb "github.com/mgilbir/spine/common/xml"
)

// OMMLTypeMap maps Office Math (m:) element local names to Go types from
// common/omml.
var OMMLTypeMap = map[string]reflect.Type{
	// Containers
	"oMath":       reflect.TypeOf(omml.OMath{}),
	"oMathPara":   reflect.TypeOf(omml.OMathPara{}),
	"oMathParaPr": reflect.TypeOf(omml.OMathParaPr{}),

	// Structures (EG_OMathMathElements)
	"acc":       reflect.TypeOf(omml.Accent{}),
	"bar":       reflect.TypeOf(omml.Bar{}),
	"box":       reflect.TypeOf(omml.Box{}),
	"borderBox": reflect.TypeOf(omml.BorderBox{}),
	"d":         reflect.TypeOf(omml.Delimiter{}),
	"eqArr":     reflect.TypeOf(omml.EquationArray{}),
	"f":         reflect.TypeOf(omml.Fraction{}),
	"func":      reflect.TypeOf(omml.Function{}),
	"groupChr":  reflect.TypeOf(omml.GroupChar{}),
	"limLow":    reflect.TypeOf(omml.LimitLow{}),
	"limUpp":    reflect.TypeOf(omml.LimitUpper{}),
	"m":         reflect.TypeOf(omml.Matrix{}),
	"nary":      reflect.TypeOf(omml.NAry{}),
	"phant":     reflect.TypeOf(omml.Phantom{}),
	"rad":       reflect.TypeOf(omml.Radical{}),
	"sPre":      reflect.TypeOf(omml.SubSuperscriptPre{}),
	"sSub":      reflect.TypeOf(omml.Subscript{}),
	"sSubSup":   reflect.TypeOf(omml.SubSuperscript{}),
	"sSup":      reflect.TypeOf(omml.Superscript{}),
	"r":         reflect.TypeOf(omml.Run{}),

	// Runs and arguments
	"t":     reflect.TypeOf(omml.Text{}),
	"e":     reflect.TypeOf(omml.Element{}),
	"argPr": reflect.TypeOf(omml.ArgPr{}),
	"rPr":   reflect.TypeOf(omml.RunPr{}),

	// Property containers
	"accPr":       reflect.TypeOf(omml.AccentPr{}),
	"barPr":       reflect.TypeOf(omml.BarPr{}),
	"boxPr":       reflect.TypeOf(omml.BoxPr{}),
	"borderBoxPr": reflect.TypeOf(omml.BorderBoxPr{}),
	"ctrlPr":      reflect.TypeOf(omml.CtrlPr{}),
	"dPr":         reflect.TypeOf(omml.DelimiterPr{}),
	"eqArrPr":     reflect.TypeOf(omml.EqArrPr{}),
	"fPr":         reflect.TypeOf(omml.FractionPr{}),
	"funcPr":      reflect.TypeOf(omml.FuncPr{}),
	"groupChrPr":  reflect.TypeOf(omml.GroupChrPr{}),
	"limLowPr":    reflect.TypeOf(omml.LimPr{}),
	"limUppPr":    reflect.TypeOf(omml.LimPr{}),
	"mPr":         reflect.TypeOf(omml.MatrixPr{}),
	"mr":          reflect.TypeOf(omml.MatrixRow{}),
	"mc":          reflect.TypeOf(omml.MatrixColumn{}),
	"mcs":         reflect.TypeOf(omml.MatrixColumns{}),
	"mcPr":        reflect.TypeOf(omml.MatrixColumnPr{}),
	"naryPr":      reflect.TypeOf(omml.NaryPr{}),
	"phantPr":     reflect.TypeOf(omml.PhantPr{}),
	"radPr":       reflect.TypeOf(omml.RadPr{}),
	"sPrePr":      reflect.TypeOf(omml.SPrePr{}),
	"sSubPr":      reflect.TypeOf(omml.SSubPr{}),
	"sSubSupPr":   reflect.TypeOf(omml.SSubSupPr{}),
	"sSupPr":      reflect.TypeOf(omml.SSupPr{}),

	// Document-level math settings
	"mathPr":     reflect.TypeOf(omml.MathPr{}),
	"mathFont":   reflect.TypeOf(omml.MathFont{}),
	"brkBin":     reflect.TypeOf(omml.BreakBin{}),
	"brkBinSub":  reflect.TypeOf(omml.BreakBinSub{}),
	"brk":        reflect.TypeOf(omml.Break{}),
	"chr":        reflect.TypeOf(omml.Char{}),
	"limLoc":     reflect.TypeOf(omml.LimLoc{}),
	"jc":         reflect.TypeOf(omml.MathJc{}),
	"defJc":      reflect.TypeOf(omml.MathJc{}),
	"scr":        reflect.TypeOf(omml.Script{}),
	"sty":        reflect.TypeOf(omml.Style{}),
	"type":       reflect.TypeOf(omml.FType{}),
	"pos":        reflect.TypeOf(omml.TopBot{}),
	"vertJc":     reflect.TypeOf(omml.TopBot{}),
	"baseJc":     reflect.TypeOf(omml.YAlign{}),
	"mcJc":       reflect.TypeOf(omml.XAlign{}),
	"shp":        reflect.TypeOf(omml.Shape{}),
	"argSz":      reflect.TypeOf(omml.Integer{}),
	"count":      reflect.TypeOf(omml.Integer255{}),
	"rSp":        reflect.TypeOf(omml.UnSignedInteger{}),
	"cSp":        reflect.TypeOf(omml.UnSignedInteger{}),
	"cGp":        reflect.TypeOf(omml.UnSignedInteger{}),
	"rSpRule":    reflect.TypeOf(omml.SpacingRule{}),
	"cGpRule":    reflect.TypeOf(omml.SpacingRule{}),
	"lMargin":    reflect.TypeOf(omml.TwipsMeasure{}),
	"rMargin":    reflect.TypeOf(omml.TwipsMeasure{}),
	"preSp":      reflect.TypeOf(omml.TwipsMeasure{}),
	"postSp":     reflect.TypeOf(omml.TwipsMeasure{}),
	"interSp":    reflect.TypeOf(omml.TwipsMeasure{}),
	"intraSp":    reflect.TypeOf(omml.TwipsMeasure{}),
	"wrapIndent": reflect.TypeOf(omml.TwipsMeasure{}),
	"degHide":    reflect.TypeOf(omml.OnOff{}),
	"grow":       reflect.TypeOf(omml.OnOff{}),
	"subHide":    reflect.TypeOf(omml.OnOff{}),
	"supHide":    reflect.TypeOf(omml.OnOff{}),
	"lit":        reflect.TypeOf(omml.OnOff{}),
	"nor":        reflect.TypeOf(omml.OnOff{}),
	"aln":        reflect.TypeOf(omml.OnOff{}),
	"alnScr":     reflect.TypeOf(omml.OnOff{}),
	"smallFrac":  reflect.TypeOf(omml.OnOff{}),
	"dispDef":    reflect.TypeOf(omml.OnOff{}),
	"wrapRight":  reflect.TypeOf(omml.OnOff{}),
	"intLim":     reflect.TypeOf(omml.LimLoc{}),
	"naryLim":    reflect.TypeOf(omml.LimLoc{}),
	"begChr":     reflect.TypeOf(omml.Char{}),
	"sepChr":     reflect.TypeOf(omml.Char{}),
	"endChr":     reflect.TypeOf(omml.Char{}),
	"opEmu":      reflect.TypeOf(omml.OnOff{}),
	"noBreak":    reflect.TypeOf(omml.OnOff{}),
	"diff":       reflect.TypeOf(omml.OnOff{}),
	"hideTop":    reflect.TypeOf(omml.OnOff{}),
	"hideBot":    reflect.TypeOf(omml.OnOff{}),
	"hideLeft":   reflect.TypeOf(omml.OnOff{}),
	"hideRight":  reflect.TypeOf(omml.OnOff{}),
	"strikeH":    reflect.TypeOf(omml.OnOff{}),
	"strikeV":    reflect.TypeOf(omml.OnOff{}),
	"strikeBLTR": reflect.TypeOf(omml.OnOff{}),
	"strikeTLBR": reflect.TypeOf(omml.OnOff{}),
	"show":       reflect.TypeOf(omml.OnOff{}),
	"zeroWid":    reflect.TypeOf(omml.OnOff{}),
	"zeroAsc":    reflect.TypeOf(omml.OnOff{}),
	"zeroDesc":   reflect.TypeOf(omml.OnOff{}),
	"transp":     reflect.TypeOf(omml.OnOff{}),
	"maxDist":    reflect.TypeOf(omml.OnOff{}),
	"objDist":    reflect.TypeOf(omml.OnOff{}),
	"plcHide":    reflect.TypeOf(omml.OnOff{}),
}

func ommlTestdataPath() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "testdata", "shared_examples.json")
}

// ommlExamples returns the Office Math (m: prefixed) examples from the shared
// spec test data. The shared file also carries OPC/Dublin Core examples whose
// element names would collide with math ones (r, f, ...), so the math subset
// is selected by namespace prefix.
func ommlExamples(t *testing.T) []Example {
	t.Helper()
	all := LoadExamples(t, ommlTestdataPath())
	var out []Example
	for _, ex := range all {
		if ex.NSPrefix == "m" {
			out = append(out, ex)
		}
	}
	return out
}

func TestOMML_SpecExamples_WellFormed(t *testing.T) {
	TestWellFormedExamples(t, ommlExamples(t), WrapWML)
}

func TestOMML_SpecExamples_Unmarshal(t *testing.T) {
	TestUnmarshalExamples(t, ommlExamples(t), OMMLTypeMap, WrapWML)
}

func TestOMML_SpecExamples_RoundTrip(t *testing.T) {
	marshalFn := func(v interface{}, rootElem string) ([]byte, error) {
		b := xmlb.NewWordprocessingMLBuilder()
		b.MarshalElement(xmlb.NSMath, rootElem, v)
		if err := b.Finish(); err != nil {
			return nil, err
		}
		return b.Bytes(), nil
	}
	TestRoundTripExamplesWithMarshal(t, ommlExamples(t), OMMLTypeMap, WrapWML, marshalFn)
}
