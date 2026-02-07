package oxml

import (
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/mgilbir/spine/spec/spectest"
)

var pmlTypeMap = map[string]reflect.Type{
	// Root types
	"presentation": reflect.TypeOf(Presentation{}),
	"sld":          reflect.TypeOf(Slide{}),
	"sldLayout":    reflect.TypeOf(SlideLayout{}),
	"sldMaster":    reflect.TypeOf(SlideMaster{}),

	// Slide structure
	"cSld":       reflect.TypeOf(CommonSlideData{}),
	"spTree":     reflect.TypeOf(ShapeTree{}),
	"bg":         reflect.TypeOf(Background{}),
	"clrMap":     reflect.TypeOf(ColorMap{}),
	"clrMapOvr":  reflect.TypeOf(ColorMapOverride{}),
	"txStyles":   reflect.TypeOf(TxStyles{}),

	// Shapes
	"sp":           reflect.TypeOf(Shape{}),
	"pic":          reflect.TypeOf(Picture{}),
	"grpSp":        reflect.TypeOf(GroupShape{}),
	"graphicFrame": reflect.TypeOf(GraphicFrame{}),

	// Presentation structure
	"embeddedFont":    reflect.TypeOf(EmbeddedFont{}),
	"embeddedFontLst": reflect.TypeOf(EmbeddedFontList{}),
	"custShowLst":     reflect.TypeOf(CustomShowList{}),
	"custShow":        reflect.TypeOf(CustomShow{}),
	"modifyVerifier":  reflect.TypeOf(ModifyVerifier{}),
	"notes":           reflect.TypeOf(NotesSlide{}),

	// Transition types
	"transition": reflect.TypeOf(Transition{}),

	// Animation/timing types
	"timing":     reflect.TypeOf(Timing{}),
	"bldLst":     reflect.TypeOf(BuildList{}),
	"par":        reflect.TypeOf(ParallelTimeNode{}),
	"seq":        reflect.TypeOf(SequenceTimeNode{}),
	"cTn":        reflect.TypeOf(CommonTimeNode{}),
	"childTnLst": reflect.TypeOf(TimeNodeList{}),
	"subTnLst":   reflect.TypeOf(TimeNodeList{}),
	"tgtEl":      reflect.TypeOf(TargetElement{}),
	"pos":        reflect.TypeOf(Point2D{}),
	"anim":       reflect.TypeOf(Animate{}),
	"animClr":    reflect.TypeOf(AnimateColor{}),
	"animEffect": reflect.TypeOf(AnimateEffect{}),
	"animMotion": reflect.TypeOf(AnimateMotion{}),
	"animRot":    reflect.TypeOf(AnimateRotation{}),
	"animScale":  reflect.TypeOf(AnimateScale{}),
	"set":        reflect.TypeOf(Set{}),
	"cmd":        reflect.TypeOf(Command{}),
	"audio":      reflect.TypeOf(Audio{}),

	// Comments
	"cmLst":      reflect.TypeOf(CommentList{}),
	"cmAuthorLst": reflect.TypeOf(CommentAuthorList{}),
	"cmAuthor":   reflect.TypeOf(CommentAuthor{}),
	"cm":         reflect.TypeOf(Comment{}),

	// Presentation properties
	"htmlPubPr": reflect.TypeOf(HtmlPublishProperties{}),

	// View properties
	"style": reflect.TypeOf(SlideProperties{}),
}

func pmlTestdataPath() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "spec", "testdata", "pml_examples.json")
}

func TestPML_SpecExamples_WellFormed(t *testing.T) {
	examples := spectest.LoadExamples(t, pmlTestdataPath())
	spectest.TestWellFormedExamples(t, examples, spectest.WrapPML)
}

func TestPML_SpecExamples_Unmarshal(t *testing.T) {
	examples := spectest.LoadExamples(t, pmlTestdataPath())
	spectest.TestUnmarshalExamples(t, examples, pmlTypeMap, spectest.WrapPML)
}

func TestPML_SpecExamples_RoundTrip(t *testing.T) {
	examples := spectest.LoadExamples(t, pmlTestdataPath())
	spectest.TestRoundTripExamples(t, examples, pmlTypeMap, spectest.WrapPML)
}
