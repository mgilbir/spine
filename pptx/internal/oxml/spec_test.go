package oxml

import (
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/mgilbir/spine/common/dml"
	xmlb "github.com/mgilbir/spine/common/xml"
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

	// Shape style: p:style is a:CT_ShapeStyle (lnRef/fillRef/effectRef/fontRef).
	// It used to be mapped to a placeholder "SlideProperties" struct that shared
	// no child with it, so the round trip compared two empty values and passed
	// vacuously (C527).
	"style": reflect.TypeOf(dml.Style{}),

	// Background reference (uses DML FillRef / CT_StyleMatrixReference)
	"bgRef": reflect.TypeOf(dml.FillRef{}),

	// p:text has no standalone Go type: it is CT_Comment's xsd:string child,
	// modeled as Comment.Text. It is covered in its real context by
	// TestComment_TextChildRoundTripsThroughBuilder (comments_test.go) rather
	// than through a struct that exists only for this map (C527).
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
	marshalFn := func(v interface{}, rootElem string) ([]byte, error) {
		b := xmlb.NewPresentationMLBuilder()
		b.MarshalElement(xmlb.NSPresentationML, rootElem, v)
		return b.Bytes(), nil
	}
	spectest.TestRoundTripExamplesWithMarshal(t, examples, pmlTypeMap, spectest.WrapPML, marshalFn)
}
