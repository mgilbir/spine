package pptx

import (
	"regexp"
	"strings"
	"testing"

	"github.com/mgilbir/spine/common/dml"
)

// buildTestGroup returns a domain-built group with two text-box children.
func buildTestGroup() *GroupShape {
	g := NewGroupShape()
	g.SetName("MyGroup")
	g.SetPosition(dml.Inches(1), dml.Inches(1))
	g.SetSize(dml.Inches(4), dml.Inches(2))

	tb1 := NewTextBox()
	tb1.SetText("child one")
	tb1.SetPosition(dml.Inches(1), dml.Inches(1))
	tb1.SetSize(dml.Inches(2), dml.Inches(1))

	tb2 := NewTextBox()
	tb2.SetText("child two")
	tb2.SetPosition(dml.Inches(3), dml.Inches(1))
	tb2.SetSize(dml.Inches(2), dml.Inches(1))

	_ = g.AddChild(tb1)
	_ = g.AddChild(tb2)
	return g
}

var anyCNvPrIDRE = regexp.MustCompile(`<p:cNvPr id="(\d+)"`)

func assertUniqueShapeIDs(t *testing.T, slideXML string) {
	t.Helper()
	seen := map[string]bool{}
	for _, m := range anyCNvPrIDRE.FindAllStringSubmatch(slideXML, -1) {
		if seen[m[1]] {
			t.Errorf("duplicate cNvPr id %s in slide XML:\n%s", m[1], slideXML)
		}
		seen[m[1]] = true
	}
}

// C85: a domain-built GroupShape added via AddShape on a created deck is
// serialized as a real p:grpSp with its children (previously it was accepted
// and silently never written).
func TestAddShape_GroupShape_CreatedDeck(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	if err := s.AddShape(buildTestGroup()); err != nil {
		t.Fatal(err)
	}

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	xml := string(zipPart(t, data, "ppt/slides/slide1.xml"))
	if got := strings.Count(xml, "<p:grpSp>"); got != 1 {
		t.Fatalf("slide has %d p:grpSp elements, want 1:\n%s", got, xml)
	}
	grp := xml[strings.Index(xml, "<p:grpSp>"):strings.Index(xml, "</p:grpSp>")]
	if !strings.Contains(grp, "child one") || !strings.Contains(grp, "child two") {
		t.Errorf("group is missing its children:\n%s", grp)
	}
	if !strings.Contains(grp, `name="MyGroup"`) {
		t.Errorf("group name lost:\n%s", grp)
	}
	assertUniqueShapeIDs(t, xml)
}

// C85: the same group appended to a slide loaded from a file serializes via
// the append path, preserving the loaded content.
func TestAddShape_GroupShape_LoadedDeck(t *testing.T) {
	p := loadedDeck(t)
	s := p.Slides()[0]
	if err := s.AddShape(buildTestGroup()); err != nil {
		t.Fatal(err)
	}

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	xml := string(zipPart(t, data, "ppt/slides/slide1.xml"))
	if got := strings.Count(xml, "<p:grpSp>"); got != 1 {
		t.Fatalf("slide has %d p:grpSp elements, want 1:\n%s", got, xml)
	}
	if !strings.Contains(xml, "child one") || !strings.Contains(xml, "child two") {
		t.Errorf("group children missing:\n%s", xml)
	}
	if !strings.Contains(xml, "content") {
		t.Errorf("pre-existing slide content dropped by the group append:\n%s", xml)
	}
	assertUniqueShapeIDs(t, xml)

	// The appended group can be removed again surgically.
	shapes := s.Shapes()
	s.RemoveShape(shapes[len(shapes)-1])
	data, err = p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	xml = string(zipPart(t, data, "ppt/slides/slide1.xml"))
	if strings.Contains(xml, "<p:grpSp>") {
		t.Errorf("removed group still present:\n%s", xml)
	}
	if !strings.Contains(xml, "content") {
		t.Errorf("pre-existing content lost by group removal:\n%s", xml)
	}
}

// unsupportedShape implements Shape but is not a type the library can
// serialize.
type unsupportedShape struct{ BaseShape }

func (*unsupportedShape) ShapeType() ShapeType { return ShapeTypeUnknown }

// C85: AddShape rejects shape types it cannot serialize instead of silently
// dropping them on save.
func TestAddShape_UnsupportedType_Error(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	if err := s.AddShape(&unsupportedShape{}); err == nil {
		t.Fatal("AddShape accepted a shape type it cannot serialize")
	}
	if got := len(s.Shapes()); got != 0 {
		t.Errorf("rejected shape was still added: %d shapes", got)
	}
}
