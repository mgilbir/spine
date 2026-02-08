package pptx

import (
	"github.com/mgilbir/spine/common/dml"
)

// Shape is the interface implemented by all shape types.
type Shape interface {
	// ShapeType returns the type of the shape.
	ShapeType() ShapeType

	// Name returns the name of the shape.
	Name() string

	// SetName sets the name of the shape.
	SetName(name string)

	// Position returns the position of the shape in EMUs.
	Position() (x, y dml.EMU)

	// SetPosition sets the position of the shape in EMUs.
	SetPosition(x, y dml.EMU)

	// Size returns the size of the shape in EMUs.
	Size() (width, height dml.EMU)

	// SetSize sets the size of the shape in EMUs.
	SetSize(width, height dml.EMU)
}

// ShapeType represents the type of a shape.
type ShapeType int

const (
	ShapeTypeUnknown ShapeType = iota
	ShapeTypeTextBox
	ShapeTypePicture
	ShapeTypeTable
	ShapeTypeChart
	ShapeTypeGroup
	ShapeTypePlaceholder
	ShapeTypeConnector
	ShapeTypeAutoShape
)

// String returns the string representation of the shape type.
func (st ShapeType) String() string {
	switch st {
	case ShapeTypeTextBox:
		return "textbox"
	case ShapeTypePicture:
		return "picture"
	case ShapeTypeTable:
		return "table"
	case ShapeTypeChart:
		return "chart"
	case ShapeTypeGroup:
		return "group"
	case ShapeTypePlaceholder:
		return "placeholder"
	case ShapeTypeConnector:
		return "connector"
	case ShapeTypeAutoShape:
		return "autoshape"
	default:
		return "unknown"
	}
}

// BaseShape provides common shape functionality.
type BaseShape struct {
	name   string
	x, y   dml.EMU
	width  dml.EMU
	height dml.EMU
}

// Name returns the name of the shape.
func (s *BaseShape) Name() string {
	return s.name
}

// SetName sets the name of the shape.
func (s *BaseShape) SetName(name string) {
	s.name = name
}

// Position returns the position of the shape in EMUs.
func (s *BaseShape) Position() (x, y dml.EMU) {
	return s.x, s.y
}

// SetPosition sets the position of the shape in EMUs.
func (s *BaseShape) SetPosition(x, y dml.EMU) {
	s.x = x
	s.y = y
}

// Size returns the size of the shape in EMUs.
func (s *BaseShape) Size() (width, height dml.EMU) {
	return s.width, s.height
}

// SetSize sets the size of the shape in EMUs.
func (s *BaseShape) SetSize(width, height dml.EMU) {
	s.width = width
	s.height = height
}

// Bounds returns the bounding rectangle of the shape.
func (s *BaseShape) Bounds() dml.Rect {
	return dml.NewRect(s.x, s.y, s.width, s.height)
}

// SetBounds sets the position and size of the shape from a rectangle.
func (s *BaseShape) SetBounds(r dml.Rect) {
	s.x = r.X
	s.y = r.Y
	s.width = r.Width
	s.height = r.Height
}

// TextBox represents a text box shape.
type TextBox struct {
	BaseShape
	textFrame *TextFrame
	spPr      dml.SpPr
}

// NewTextBox creates a new text box.
func NewTextBox() *TextBox {
	return &TextBox{
		textFrame: NewTextFrame(),
	}
}

// ShapeType returns ShapeTypeTextBox.
func (t *TextBox) ShapeType() ShapeType {
	return ShapeTypeTextBox
}

// TextFrame returns the text frame for the text box.
func (t *TextBox) TextFrame() *TextFrame {
	return t.textFrame
}

// SetText sets the text content of the text box.
func (t *TextBox) SetText(text string) {
	if t.textFrame == nil {
		t.textFrame = NewTextFrame()
	}
	t.textFrame.SetText(text)
}

// Text returns the text content of the text box.
func (t *TextBox) Text() string {
	if t.textFrame == nil {
		return ""
	}
	return t.textFrame.Text()
}

// GroupShape represents a group of shapes.
type GroupShape struct {
	BaseShape
	children []Shape
}

// NewGroupShape creates a new group shape.
func NewGroupShape() *GroupShape {
	return &GroupShape{
		children: make([]Shape, 0),
	}
}

// ShapeType returns ShapeTypeGroup.
func (g *GroupShape) ShapeType() ShapeType {
	return ShapeTypeGroup
}

// Children returns the shapes in the group.
func (g *GroupShape) Children() []Shape {
	return g.children
}

// AddChild adds a shape to the group.
func (g *GroupShape) AddChild(shape Shape) {
	g.children = append(g.children, shape)
}

// RemoveChild removes a shape from the group.
func (g *GroupShape) RemoveChild(shape Shape) {
	for i, child := range g.children {
		if child == shape {
			g.children = append(g.children[:i], g.children[i+1:]...)
			return
		}
	}
}

// AutoShape represents a preset geometric shape.
type AutoShape struct {
	BaseShape
	presetGeometry string
	textFrame      *TextFrame
	spPr           dml.SpPr
}

// NewAutoShape creates a new auto shape with the specified preset geometry.
func NewAutoShape(preset string) *AutoShape {
	return &AutoShape{
		presetGeometry: preset,
	}
}

// ShapeType returns ShapeTypeAutoShape.
func (a *AutoShape) ShapeType() ShapeType {
	return ShapeTypeAutoShape
}

// PresetGeometry returns the preset geometry name.
func (a *AutoShape) PresetGeometry() string {
	return a.presetGeometry
}

// TextFrame returns the text frame, creating one if needed.
func (a *AutoShape) TextFrame() *TextFrame {
	if a.textFrame == nil {
		a.textFrame = NewTextFrame()
	}
	return a.textFrame
}

// --- Fill, Line, Shadow for AutoShape ---

// SetFill sets the fill of the auto shape.
func (a *AutoShape) SetFill(fill dml.Fill) {
	fill.ApplyToSpPr(&a.spPr)
}

// SetLine sets the line (outline) of the auto shape.
func (a *AutoShape) SetLine(line dml.Line) {
	line.ApplyToSpPr(&a.spPr)
}

// SetShadow sets the shadow effect on the auto shape.
func (a *AutoShape) SetShadow(shadow dml.Shadow) {
	shadow.ApplyToSpPr(&a.spPr)
}

// --- Fill, Line, Shadow for TextBox ---

// SetFill sets the fill of the text box.
func (t *TextBox) SetFill(fill dml.Fill) {
	fill.ApplyToSpPr(&t.spPr)
}

// SetLine sets the line (outline) of the text box.
func (t *TextBox) SetLine(line dml.Line) {
	line.ApplyToSpPr(&t.spPr)
}

// SetShadow sets the shadow effect on the text box.
func (t *TextBox) SetShadow(shadow dml.Shadow) {
	shadow.ApplyToSpPr(&t.spPr)
}

// Common preset geometry names
const (
	PresetRect           = "rect"
	PresetRoundRect      = "roundRect"
	PresetEllipse        = "ellipse"
	PresetTriangle       = "triangle"
	PresetRightTriangle  = "rtTriangle"
	PresetParallelogram  = "parallelogram"
	PresetTrapezoid      = "trapezoid"
	PresetDiamond        = "diamond"
	PresetPentagon       = "pentagon"
	PresetHexagon        = "hexagon"
	PresetHeptagon       = "heptagon"
	PresetOctagon        = "octagon"
	PresetStar4          = "star4"
	PresetStar5          = "star5"
	PresetStar6          = "star6"
	PresetArrowRight     = "rightArrow"
	PresetArrowLeft      = "leftArrow"
	PresetArrowUp        = "upArrow"
	PresetArrowDown      = "downArrow"
	PresetCloud          = "cloud"
	PresetHeart          = "heart"
	PresetLightningBolt  = "lightningBolt"
	PresetSun            = "sun"
	PresetMoon           = "moon"
	PresetSmileyFace     = "smileyFace"
	PresetCallout1       = "wedgeRectCallout"
	PresetCallout2       = "wedgeRoundRectCallout"
	PresetCallout3       = "wedgeEllipseCallout"
)
