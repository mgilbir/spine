package pptx

import (
	"testing"

	"github.com/mgilbir/spine/common/dml"
)

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
