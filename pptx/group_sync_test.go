package pptx

import (
	"bytes"
	"strings"
	"testing"
)

// groupXML is a p:grpSp with two text boxes plus a nested group holding a
// third, as PowerPoint authors them (ids 10-14).
const groupXML = `<p:grpSp><p:nvGrpSpPr><p:cNvPr id="10" name="Group 1"/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="2743200" cy="914400"/><a:chOff x="0" y="0"/><a:chExt cx="2743200" cy="914400"/></a:xfrm></p:grpSpPr><p:sp><p:nvSpPr><p:cNvPr id="11" name="Child A"/><p:cNvSpPr txBox="1"/><p:nvPr/></p:nvSpPr><p:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="914400" cy="914400"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr><p:txBody><a:bodyPr/><a:p><a:r><a:t>alpha</a:t></a:r></a:p></p:txBody></p:sp><p:sp><p:nvSpPr><p:cNvPr id="12" name="Child B"/><p:cNvSpPr txBox="1"/><p:nvPr/></p:nvSpPr><p:spPr><a:xfrm><a:off x="914400" y="0"/><a:ext cx="914400" cy="914400"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr><p:txBody><a:bodyPr/><a:p><a:r><a:t>beta</a:t></a:r></a:p></p:txBody></p:sp><p:grpSp><p:nvGrpSpPr><p:cNvPr id="13" name="Inner Group"/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr><a:xfrm><a:off x="1828800" y="0"/><a:ext cx="914400" cy="914400"/><a:chOff x="1828800" y="0"/><a:chExt cx="914400" cy="914400"/></a:xfrm></p:grpSpPr><p:sp><p:nvSpPr><p:cNvPr id="14" name="Child N"/><p:cNvSpPr txBox="1"/><p:nvPr/></p:nvSpPr><p:spPr><a:xfrm><a:off x="1828800" y="0"/><a:ext cx="914400" cy="914400"/></a:xfrm><a:prstGeom prst="rect"><a:avLst/></a:prstGeom></p:spPr><p:txBody><a:bodyPr/><a:p><a:r><a:t>nested</a:t></a:r></a:p></p:txBody></p:sp></p:grpSp></p:grpSp>`

// groupDeck returns a saved deck whose first slide carries the group above.
func groupDeck(t *testing.T) []byte {
	t.Helper()
	return rewriteZipPart(t, savedDeck(t), "ppt/slides/slide1.xml", func(xml []byte) []byte {
		return bytes.Replace(xml, []byte("</p:spTree>"), []byte(groupXML+"</p:spTree>"), 1)
	})
}

// C175(a): a SetText on a shape handed out by ShapeByName from inside a group
// must flush to the parsed grpSp; siblings and the group frame stay intact.
func TestGroupChildEditFlushes(t *testing.T) {
	p := openBytes(t, groupDeck(t))
	tb, ok := p.Slides()[0].ShapeByName("Child A").(*TextBox)
	if !ok {
		t.Fatal("Child A not found inside the group")
	}
	tb.SetText("edited-alpha")

	out, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	slideXML := zipPart(t, out, "ppt/slides/slide1.xml")
	if !bytes.Contains(slideXML, []byte("<a:t>edited-alpha</a:t>")) {
		t.Error("group-child edit was dropped")
	}
	if !bytes.Contains(slideXML, []byte("<a:t>beta</a:t>")) || !bytes.Contains(slideXML, []byte("<a:t>nested</a:t>")) {
		t.Error("sibling / nested content lost")
	}
	if got := strings.Count(string(slideXML), "<p:grpSp>"); got != 2 {
		t.Errorf("grpSp count = %d, want 2", got)
	}
	if !bytes.Contains(slideXML, []byte(`id="11" name="Child A"`)) {
		t.Error("edited child's id changed")
	}
}

// C175(a): edits inside nested groups flush recursively.
func TestGroupNestedChildEditFlushes(t *testing.T) {
	p := openBytes(t, groupDeck(t))
	tb, ok := p.Slides()[0].ShapeByName("Child N").(*TextBox)
	if !ok {
		t.Fatal("Child N not found inside the nested group")
	}
	tb.SetText("edited-nested")

	out, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	slideXML := zipPart(t, out, "ppt/slides/slide1.xml")
	if !bytes.Contains(slideXML, []byte("<a:t>edited-nested</a:t>")) {
		t.Error("nested group-child edit was dropped")
	}
	for _, keep := range []string{"<a:t>alpha</a:t>", "<a:t>beta</a:t>"} {
		if !bytes.Contains(slideXML, []byte(keep)) {
			t.Errorf("sibling content %q lost", keep)
		}
	}
}

// C175(a): edit -> save -> edit -> save, same session and through a reopen.
func TestGroupChildEditMultiCycle(t *testing.T) {
	p := openBytes(t, groupDeck(t))
	p.Slides()[0].ShapeByName("Child A").(*TextBox).SetText("first")
	out1, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	p.Slides()[0].ShapeByName("Child B").(*TextBox).SetText("second")
	out2, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	slideXML := zipPart(t, out2, "ppt/slides/slide1.xml")
	for _, want := range []string{"<a:t>first</a:t>", "<a:t>second</a:t>", "<a:t>nested</a:t>"} {
		if !bytes.Contains(slideXML, []byte(want)) {
			t.Errorf("%q missing after same-session second cycle", want)
		}
	}

	p2 := openBytes(t, out1)
	p2.Slides()[0].ShapeByName("Child A").(*TextBox).SetText("third")
	out3, err := p2.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	slideXML = zipPart(t, out3, "ppt/slides/slide1.xml")
	if !bytes.Contains(slideXML, []byte("<a:t>third</a:t>")) {
		t.Error("edit dropped after reopen cycle")
	}
}

// C175(b): RemoveChild deletes the child's node from the parsed grpSp,
// leaving the group and its other children intact.
func TestGroupRemoveChild(t *testing.T) {
	p := openBytes(t, groupDeck(t))
	grp, ok := p.Slides()[0].ShapeByName("Group 1").(*GroupShape)
	if !ok {
		t.Fatal("Group 1 not found")
	}
	grp.RemoveChild(p.Slides()[0].ShapeByName("Child B"))

	out, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	slideXML := zipPart(t, out, "ppt/slides/slide1.xml")
	if bytes.Contains(slideXML, []byte("<a:t>beta</a:t>")) {
		t.Error("removed child still present")
	}
	for _, keep := range []string{"<a:t>alpha</a:t>", "<a:t>nested</a:t>", `name="Group 1"`} {
		if !bytes.Contains(slideXML, []byte(keep)) {
			t.Errorf("%q lost by RemoveChild", keep)
		}
	}
	if got := strings.Count(string(slideXML), "<p:grpSp>"); got != 2 {
		t.Errorf("grpSp count = %d, want 2", got)
	}

	// Second cycle: remove another child from the already-compacted group.
	grp.RemoveChild(p.Slides()[0].ShapeByName("Child A"))
	out2, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	slideXML = zipPart(t, out2, "ppt/slides/slide1.xml")
	if bytes.Contains(slideXML, []byte("<a:t>alpha</a:t>")) {
		t.Error("second removed child still present")
	}
	if !bytes.Contains(slideXML, []byte("<a:t>nested</a:t>")) {
		t.Error("nested group lost in second removal cycle")
	}
}

// C175(b): AddChild appends the child inside the grpSp with a fresh
// slide-wide unique id, and the child stays editable afterwards.
func TestGroupAddChild(t *testing.T) {
	p := openBytes(t, groupDeck(t))
	grp, ok := p.Slides()[0].ShapeByName("Group 1").(*GroupShape)
	if !ok {
		t.Fatal("Group 1 not found")
	}
	tb := NewTextBox()
	tb.SetName("Child C")
	tb.SetPosition(0, 0)
	tb.SetSize(914400, 914400)
	tb.SetText("gamma")
	grp.AddChild(tb)

	out, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	slideXML := zipPart(t, out, "ppt/slides/slide1.xml")
	if !bytes.Contains(slideXML, []byte("<a:t>gamma</a:t>")) {
		t.Fatal("added child missing from slide XML")
	}
	// The new node must live inside the group, not at the top level.
	grpStart := bytes.Index(slideXML, []byte("<p:grpSp>"))
	grpEnd := bytes.LastIndex(slideXML, []byte("</p:grpSp>"))
	gamma := bytes.Index(slideXML, []byte("<a:t>gamma</a:t>"))
	if gamma < grpStart || gamma > grpEnd {
		t.Error("added child not inside the grpSp")
	}
	// Max id in the fixture tree is 14 (deck textbox ids are lower).
	if !bytes.Contains(slideXML, []byte(`id="15" name="Child C"`)) {
		t.Errorf("added child did not get the next slide-wide id:\n%s", slideXML)
	}

	// The deck must reopen cleanly and the added child must stay editable.
	p2 := openBytes(t, out)
	tb2, ok := p2.Slides()[0].ShapeByName("Child C").(*TextBox)
	if !ok {
		t.Fatal("added child not found after reopen")
	}
	tb2.SetText("gamma-2")
	out2, err := p2.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	slideXML = zipPart(t, out2, "ppt/slides/slide1.xml")
	if !bytes.Contains(slideXML, []byte("<a:t>gamma-2</a:t>")) {
		t.Error("edit to reopened added child was dropped")
	}
}

// C175: same-session AddChild then edit-through-held-pointer next cycle.
func TestGroupAddChildThenEditSameSession(t *testing.T) {
	p := openBytes(t, groupDeck(t))
	grp := p.Slides()[0].ShapeByName("Group 1").(*GroupShape)
	tb := NewTextBox()
	tb.SetName("Child C")
	tb.SetText("gamma")
	grp.AddChild(tb)
	if _, err := p.SaveBytes(); err != nil {
		t.Fatal(err)
	}

	tb.SetText("gamma-edited")
	out, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	slideXML := zipPart(t, out, "ppt/slides/slide1.xml")
	if !bytes.Contains(slideXML, []byte("<a:t>gamma-edited</a:t>")) {
		t.Error("post-append edit through held pointer was dropped")
	}
	if c := strings.Count(string(slideXML), `name="Child C"`); c != 1 {
		t.Errorf("added child appears %d times, want 1", c)
	}
}
