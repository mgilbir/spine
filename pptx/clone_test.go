package pptx

import (
	"strings"
	"testing"

	"github.com/mgilbir/spine/common/dml"
)

// TestCloneShapeDeepCopiesHyperlink confirms a cloned shape's external hyperlink
// shares no state with the original (C269): editing the clone's tooltip must not
// mutate the original, and placing the two shapes on different slides must leave
// both slides with their own resolvable hyperlink relationship (not one filled
// rel and a second dangling r:id).
func TestCloneShapeDeepCopiesHyperlink(t *testing.T) {
	p := Create()
	s1 := p.AddSlide()
	s2 := p.AddSlide()

	orig := s1.AddTextBox()
	orig.TextFrame().SetText("link")
	orig.SetHyperlink("https://example.com/orig").SetTooltip("orig tip")

	clone, ok := CloneShape(orig).(*TextBox)
	if !ok {
		t.Fatal("CloneShape did not return a *TextBox")
	}
	if err := s2.AddShape(clone); err != nil {
		t.Fatalf("AddShape: %v", err)
	}

	// Mutating the clone's hyperlink must not touch the original's.
	clone.Hyperlink().SetTooltip("clone tip")
	if got := orig.Hyperlink().Tooltip(); got != "orig tip" {
		t.Errorf("original tooltip = %q, want \"orig tip\" (clone shares the pointer)", got)
	}

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	// Both slides must carry their own external hyperlink relationship: before
	// the fix the shared *Hyperlink allocated a rel on the first slide only,
	// leaving slide 2's r:id dangling.
	for i, part := range []string{"ppt/slides/_rels/slide1.xml.rels", "ppt/slides/_rels/slide2.xml.rels"} {
		rels := string(zipPart(t, data, part))
		if !strings.Contains(rels, "https://example.com/orig") {
			t.Errorf("slide %d rels missing hyperlink relationship:\n%s", i+1, rels)
		}
	}
}

func TestCloneRowPreservesStyling(t *testing.T) {
	pres := Create()
	slide := pres.AddSlide()
	table := slide.AddTable(2, 2)
	proto := table.Row(1)
	proto.Cell(0).SetText("proto")
	proto.Cell(0).SetFill(dml.ColorBlue)
	proto.SetHeight(1234)

	clone := table.CloneRow(1, 2)
	if clone == nil {
		t.Fatal("CloneRow returned nil")
	}
	if table.RowCount() != 3 {
		t.Fatalf("expected 3 rows, got %d", table.RowCount())
	}
	if clone.Height() != 1234 {
		t.Fatalf("height not cloned: %d", clone.Height())
	}
	if clone.Cell(0).Fill() == nil {
		t.Fatal("fill not cloned")
	}
	if clone.Cell(0).Text() != "proto" {
		t.Fatalf("text not cloned: %q", clone.Cell(0).Text())
	}
	// Deep copy: mutating the clone must not touch the prototype.
	clone.Cell(0).SetText("changed")
	if proto.Cell(0).Text() != "proto" {
		t.Fatal("clone shares state with prototype")
	}
	if table.CloneRow(9, 0) != nil {
		t.Fatal("out-of-range src must return nil")
	}
}

func TestCloneColumnPreservesStyling(t *testing.T) {
	pres := Create()
	slide := pres.AddSlide()
	table := slide.AddTable(2, 2)
	table.SetColWidth(1, 4321)
	table.Row(0).Cell(1).SetText("HDR")
	table.Row(1).Cell(1).SetText("proto")
	table.Row(1).Cell(1).SetFill(dml.ColorBlue)

	if !table.CloneColumn(1, 2) {
		t.Fatal("CloneColumn returned false")
	}
	if table.ColCount() != 3 {
		t.Fatalf("expected 3 columns, got %d", table.ColCount())
	}
	if table.ColWidth(2) != 4321 {
		t.Fatalf("width not cloned: %d", table.ColWidth(2))
	}
	if table.Row(0).Cell(2).Text() != "HDR" || table.Row(1).Cell(2).Text() != "proto" {
		t.Fatal("cell text not cloned")
	}
	if table.Row(1).Cell(2).Fill() == nil {
		t.Fatal("fill not cloned")
	}
	table.Row(1).Cell(2).SetText("changed")
	if table.Row(1).Cell(1).Text() != "proto" {
		t.Fatal("clone shares state with prototype")
	}
	if table.CloneColumn(9, 0) {
		t.Fatal("out-of-range src must return false")
	}
}

func TestCloneShape(t *testing.T) {
	swatch := NewAutoShape(PresetRect)
	swatch.SetName("swatch")
	swatch.SetPosition(100, 200)
	swatch.SetSize(300, 300)
	swatch.SetFill(dml.NewSolidFill(dml.NewRGB(0xAA, 0xBB, 0xCC).ToColor()))
	swatch.SetLine(dml.Line{Width: 1.5, Color: dml.NewRGB(0, 0, 0).ToColor(), Dash: dml.DashDash})
	swatch.TextFrame().SetText("x")

	clone, ok := CloneShape(swatch).(*AutoShape)
	if !ok || clone == nil {
		t.Fatal("CloneShape returned no AutoShape")
	}
	x, y := clone.Position()
	if x != 100 || y != 200 {
		t.Fatalf("position not cloned: %d,%d", x, y)
	}
	clone.SetPosition(999, 999)
	if px, _ := swatch.Position(); px != 100 {
		t.Fatal("clone shares BaseShape with original")
	}
	clone.TextFrame().SetText("changed")
	if swatch.TextFrame().Text() != "x" {
		t.Fatal("clone shares text frame with original")
	}
	clone.SetFill(dml.NewSolidFill(dml.NewRGB(1, 2, 3).ToColor()))

	label := NewTextBox()
	label.SetName("label")
	label.TextFrame().SetText("hello")
	lc, ok := CloneShape(label).(*TextBox)
	if !ok || lc.TextFrame().Text() != "hello" {
		t.Fatal("TextBox clone failed")
	}
	if CloneShape(nil) != nil {
		t.Fatal("nil shape must clone to nil")
	}
}
