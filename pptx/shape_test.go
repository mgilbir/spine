package pptx

import (
	"testing"

	"github.com/mgilbir/spine/common/dml"
)

func TestShapeType_String(t *testing.T) {
	tests := []struct {
		shapeType ShapeType
		want      string
	}{
		{ShapeTypeUnknown, "unknown"},
		{ShapeTypeTextBox, "textbox"},
		{ShapeTypePicture, "picture"},
		{ShapeTypeTable, "table"},
		{ShapeTypeChart, "chart"},
		{ShapeTypeGroup, "group"},
		{ShapeTypePlaceholder, "placeholder"},
		{ShapeTypeConnector, "connector"},
		{ShapeTypeAutoShape, "autoshape"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.shapeType.String()
			if got != tt.want {
				t.Errorf("ShapeType.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestBaseShape_Name(t *testing.T) {
	shape := &BaseShape{}

	if shape.Name() != "" {
		t.Errorf("Initial Name() = %q, want empty", shape.Name())
	}

	shape.SetName("Test Shape")
	if shape.Name() != "Test Shape" {
		t.Errorf("After SetName, Name() = %q, want %q", shape.Name(), "Test Shape")
	}
}

func TestBaseShape_Position(t *testing.T) {
	shape := &BaseShape{}

	x, y := shape.Position()
	if x != 0 || y != 0 {
		t.Errorf("Initial Position() = (%d, %d), want (0, 0)", x, y)
	}

	shape.SetPosition(dml.Inches(1), dml.Inches(2))
	x, y = shape.Position()
	if x != dml.Inches(1) || y != dml.Inches(2) {
		t.Errorf("After SetPosition, Position() = (%d, %d), want (%d, %d)",
			x, y, dml.Inches(1), dml.Inches(2))
	}
}

func TestBaseShape_Size(t *testing.T) {
	shape := &BaseShape{}

	w, h := shape.Size()
	if w != 0 || h != 0 {
		t.Errorf("Initial Size() = (%d, %d), want (0, 0)", w, h)
	}

	shape.SetSize(dml.Inches(5), dml.Inches(3))
	w, h = shape.Size()
	if w != dml.Inches(5) || h != dml.Inches(3) {
		t.Errorf("After SetSize, Size() = (%d, %d), want (%d, %d)",
			w, h, dml.Inches(5), dml.Inches(3))
	}
}

func TestBaseShape_Bounds(t *testing.T) {
	shape := &BaseShape{}
	shape.SetPosition(dml.Inches(1), dml.Inches(2))
	shape.SetSize(dml.Inches(5), dml.Inches(3))

	bounds := shape.Bounds()
	if bounds.X != dml.Inches(1) {
		t.Errorf("Bounds().X = %d, want %d", bounds.X, dml.Inches(1))
	}
	if bounds.Y != dml.Inches(2) {
		t.Errorf("Bounds().Y = %d, want %d", bounds.Y, dml.Inches(2))
	}
	if bounds.Width != dml.Inches(5) {
		t.Errorf("Bounds().Width = %d, want %d", bounds.Width, dml.Inches(5))
	}
	if bounds.Height != dml.Inches(3) {
		t.Errorf("Bounds().Height = %d, want %d", bounds.Height, dml.Inches(3))
	}
}

func TestBaseShape_SetBounds(t *testing.T) {
	shape := &BaseShape{}
	rect := dml.NewRect(dml.Inches(1), dml.Inches(2), dml.Inches(5), dml.Inches(3))
	shape.SetBounds(rect)

	x, y := shape.Position()
	w, h := shape.Size()

	if x != dml.Inches(1) || y != dml.Inches(2) {
		t.Errorf("After SetBounds, Position() = (%d, %d), want (%d, %d)",
			x, y, dml.Inches(1), dml.Inches(2))
	}
	if w != dml.Inches(5) || h != dml.Inches(3) {
		t.Errorf("After SetBounds, Size() = (%d, %d), want (%d, %d)",
			w, h, dml.Inches(5), dml.Inches(3))
	}
}

func TestNewTextBox(t *testing.T) {
	tb := NewTextBox()
	if tb == nil {
		t.Fatal("NewTextBox() returned nil")
	}

	if tb.ShapeType() != ShapeTypeTextBox {
		t.Errorf("TextBox.ShapeType() = %v, want ShapeTypeTextBox", tb.ShapeType())
	}

	if tb.TextFrame() == nil {
		t.Error("TextBox.TextFrame() is nil")
	}
}

func TestTextBox_SetText(t *testing.T) {
	tb := NewTextBox()
	tb.SetText("Hello World")

	if tb.Text() != "Hello World" {
		t.Errorf("TextBox.Text() = %q, want %q", tb.Text(), "Hello World")
	}
}

func TestTextBox_TextFrame(t *testing.T) {
	tb := NewTextBox()
	tf := tb.TextFrame()

	if tf == nil {
		t.Fatal("TextFrame() returned nil")
	}

	// Verify TextFrame is functional
	tf.SetText("Test")
	if tb.Text() != "Test" {
		t.Errorf("After TextFrame.SetText, Text() = %q, want %q", tb.Text(), "Test")
	}
}

func TestNewGroupShape(t *testing.T) {
	group := NewGroupShape()
	if group == nil {
		t.Fatal("NewGroupShape() returned nil")
	}

	if group.ShapeType() != ShapeTypeGroup {
		t.Errorf("GroupShape.ShapeType() = %v, want ShapeTypeGroup", group.ShapeType())
	}

	if len(group.Children()) != 0 {
		t.Errorf("Initial Children() has %d shapes, want 0", len(group.Children()))
	}
}

func TestGroupShape_AddRemoveChild(t *testing.T) {
	group := NewGroupShape()
	tb1 := NewTextBox()
	tb2 := NewTextBox()

	group.AddChild(tb1)
	group.AddChild(tb2)

	if len(group.Children()) != 2 {
		t.Errorf("After AddChild, Children() has %d shapes, want 2", len(group.Children()))
	}

	group.RemoveChild(tb1)
	if len(group.Children()) != 1 {
		t.Errorf("After RemoveChild, Children() has %d shapes, want 1", len(group.Children()))
	}

	if group.Children()[0] != tb2 {
		t.Error("Wrong child removed")
	}
}

func TestNewAutoShape(t *testing.T) {
	shape := NewAutoShape(PresetRect)
	if shape == nil {
		t.Fatal("NewAutoShape() returned nil")
	}

	if shape.ShapeType() != ShapeTypeAutoShape {
		t.Errorf("AutoShape.ShapeType() = %v, want ShapeTypeAutoShape", shape.ShapeType())
	}

	if shape.PresetGeometry() != PresetRect {
		t.Errorf("PresetGeometry() = %q, want %q", shape.PresetGeometry(), PresetRect)
	}
}

func TestAutoShape_TextFrame(t *testing.T) {
	shape := NewAutoShape(PresetEllipse)
	tf := shape.TextFrame()

	if tf == nil {
		t.Fatal("TextFrame() returned nil")
	}

	// TextFrame should be created on demand
	tf.SetText("Test")
	if shape.TextFrame().Text() != "Test" {
		t.Error("TextFrame not properly initialized")
	}
}

func TestPresetGeometryConstants(t *testing.T) {
	// Verify preset geometry constants are valid
	presets := []string{
		PresetRect,
		PresetRoundRect,
		PresetEllipse,
		PresetTriangle,
		PresetRightTriangle,
		PresetParallelogram,
		PresetTrapezoid,
		PresetDiamond,
		PresetPentagon,
		PresetHexagon,
		PresetHeptagon,
		PresetOctagon,
		PresetStar4,
		PresetStar5,
		PresetStar6,
		PresetArrowRight,
		PresetArrowLeft,
		PresetArrowUp,
		PresetArrowDown,
		PresetCloud,
		PresetHeart,
		PresetLightningBolt,
		PresetSun,
		PresetMoon,
		PresetSmileyFace,
		PresetCallout1,
		PresetCallout2,
		PresetCallout3,
	}

	for _, preset := range presets {
		if preset == "" {
			t.Error("Preset geometry constant is empty")
		}
	}
}
