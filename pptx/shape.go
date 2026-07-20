package pptx

import (
	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/pptx/internal/oxml"
)

// Shape is the interface implemented by all shape types.
type Shape interface {
	// ShapeType returns the type of the shape.
	ShapeType() ShapeType

	// ID returns the shape's non-visual id (cNvPr id). For a shape loaded from
	// a file this is its stored id; for a shape created through this package the
	// id is assigned when the deck is saved, so ID reports 0 until then.
	ID() uint32

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
	ShapeTypeVideo
	ShapeTypeAudio
	ShapeTypeDiagram
	ShapeTypeOLEObject
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
	case ShapeTypeVideo:
		return "video"
	case ShapeTypeAudio:
		return "audio"
	case ShapeTypeDiagram:
		return "diagram"
	case ShapeTypeOLEObject:
		return "oleobject"
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

	// dirty is set by every mutator. During the shape sync, dirty shapes that
	// are already represented in a parsed slide tree get their node updated in
	// place (see syncDirtyShapes); without the flag those edits would be
	// silently dropped, since parsed trees are never rebuilt wholesale.
	dirty bool

	// sourceID is the cNvPr id of the node this shape was materialized from
	// (0 for API-created shapes until they are first written). cNvPr ids are
	// slide-wide unique and stable across save cycles, so they identify the
	// exact node to update — in particular inside group shapes, where slice
	// indices shift as siblings come and go.
	sourceID uint32

	// hyperlink is the shape-level hyperlink (a:hlinkClick on p:cNvPr), or nil.
	hyperlink *Hyperlink
}

// Hyperlink returns the shape-level hyperlink, or nil when the shape carries
// none.
func (s *BaseShape) Hyperlink() *Hyperlink {
	return s.hyperlink
}

// setHyperlinkURL attaches an external-URL hyperlink to the shape.
func (s *BaseShape) setHyperlinkURL(url string) *Hyperlink {
	s.hyperlink = newExternalHyperlink(url, func() { s.dirty = true })
	s.dirty = true
	return s.hyperlink
}

// setActionHyperlink attaches a ppaction:// action hyperlink to the shape.
func (s *BaseShape) setActionHyperlink(action string) *Hyperlink {
	s.hyperlink = newActionHyperlink(action, func() { s.dirty = true })
	s.dirty = true
	return s.hyperlink
}

// setSlideLink attaches an internal slide-jump hyperlink to the shape.
func (s *BaseShape) setSlideLink(index int) *Hyperlink {
	s.hyperlink = newSlideJumpHyperlink(index, func() { s.dirty = true })
	s.dirty = true
	return s.hyperlink
}

// baseShapeOf returns the embedded BaseShape of any concrete shape type.
func baseShapeOf(shape Shape) *BaseShape {
	switch sh := shape.(type) {
	case *TextBox:
		return &sh.BaseShape
	case *PlaceholderShape:
		return &sh.BaseShape
	case *AutoShape:
		return &sh.BaseShape
	case *Picture:
		return &sh.BaseShape
	case *Video:
		return &sh.BaseShape
	case *Audio:
		return &sh.BaseShape
	case *Table:
		return &sh.BaseShape
	case *GroupShape:
		return &sh.BaseShape
	case *ChartFrame:
		return &sh.BaseShape
	case *SmartArtFrame:
		return &sh.BaseShape
	case *OLEObjectFrame:
		return &sh.BaseShape
	case *Connector:
		return &sh.BaseShape
	}
	return nil
}

// Name returns the name of the shape.
func (s *BaseShape) Name() string {
	return s.name
}

// ID returns the shape's cNvPr id — the slide-wide unique identifier used to
// target the shape from a slide animation (see Slide.AddAnimation). For a shape
// loaded from a file this is its stored id; for a shape created through this
// package the id is assigned when the deck is saved (sequentially from 2 in the
// order shapes were added), so ID reports 0 until then.
func (s *BaseShape) ID() uint32 {
	return s.sourceID
}

// SetName sets the name of the shape.
func (s *BaseShape) SetName(name string) {
	s.name = name
	s.dirty = true
}

// Position returns the position of the shape in EMUs.
func (s *BaseShape) Position() (x, y dml.EMU) {
	return s.x, s.y
}

// SetPosition sets the position of the shape in EMUs.
func (s *BaseShape) SetPosition(x, y dml.EMU) {
	s.x = x
	s.y = y
	s.dirty = true
}

// Size returns the size of the shape in EMUs.
func (s *BaseShape) Size() (width, height dml.EMU) {
	return s.width, s.height
}

// SetSize sets the size of the shape in EMUs.
func (s *BaseShape) SetSize(width, height dml.EMU) {
	s.width = width
	s.height = height
	s.dirty = true
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
	s.dirty = true
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

	// sourceGrp is the parsed p:grpSp node this group was materialized from
	// (nil for API-created groups). Child edits, additions, and removals are
	// flushed into it at sync time.
	sourceGrp *oxml.GroupShape
	// syncedChildren counts the leading children already represented in
	// sourceGrp; children beyond it were added via AddChild and are appended
	// to the parsed node on the next sync.
	syncedChildren int
	// removedChildIDs collects the cNvPr ids of removed synced children,
	// applied surgically at sync time (id-matched: slice indices shift).
	removedChildIDs []uint32
	// childrenModified is set when children were added or removed via the
	// API, so the dirty scan flushes the group even when no child is dirty.
	childrenModified bool
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

// AddChild adds a shape to the group. On a group loaded from a file the
// child is appended to the parsed p:grpSp on save, with a fresh slide-wide
// unique id; its position and size are interpreted in the group's child
// coordinate space (chOff/chExt).
func (g *GroupShape) AddChild(shape Shape) {
	g.children = append(g.children, shape)
	g.childrenModified = true
}

// RemoveChild removes a shape from the group. On a group loaded from a file
// the child's parsed node is deleted surgically from the p:grpSp on save;
// other children (including kinds the domain model does not represent) are
// preserved.
func (g *GroupShape) RemoveChild(shape Shape) {
	for i, child := range g.children {
		if child != shape {
			continue
		}
		if i < g.syncedChildren {
			if base := baseShapeOf(child); base != nil && base.sourceID != 0 {
				g.removedChildIDs = append(g.removedChildIDs, base.sourceID)
			}
			g.syncedChildren--
		}
		g.children = append(g.children[:i], g.children[i+1:]...)
		g.childrenModified = true
		return
	}
}

// isDirty reports whether the group carries unflushed mutations: its own
// frame, structural child changes, or edits to any (transitively nested)
// child shape.
func (g *GroupShape) isDirty() bool {
	if g.dirty || g.childrenModified || len(g.removedChildIDs) > 0 {
		return true
	}
	for _, child := range g.children {
		if shapeDirty(child) {
			return true
		}
	}
	return false
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
	a.dirty = true
}

// SetLine sets the line (outline) of the auto shape.
func (a *AutoShape) SetLine(line dml.Line) {
	line.ApplyToSpPr(&a.spPr)
	a.dirty = true
}

// SetShadow sets the shadow effect on the auto shape.
func (a *AutoShape) SetShadow(shadow dml.Shadow) {
	shadow.ApplyToSpPr(&a.spPr)
	a.dirty = true
}

// SetHyperlink attaches an external-URL hyperlink to the auto shape (clicking the
// shape opens the URL). The External relationship is allocated on save.
func (a *AutoShape) SetHyperlink(url string) *Hyperlink { return a.setHyperlinkURL(url) }

// SetActionHyperlink attaches a slide-show action hyperlink (e.g. ActionNextSlide)
// to the auto shape.
func (a *AutoShape) SetActionHyperlink(action string) *Hyperlink { return a.setActionHyperlink(action) }

// SetHyperlinkToSlide attaches an internal jump to the slide at the given 0-based
// index; the RelTypeSlide relationship is allocated on save.
func (a *AutoShape) SetHyperlinkToSlide(index int) *Hyperlink { return a.setSlideLink(index) }

// --- Fill, Line, Shadow for TextBox ---

// SetFill sets the fill of the text box.
func (t *TextBox) SetFill(fill dml.Fill) {
	fill.ApplyToSpPr(&t.spPr)
	t.dirty = true
}

// SetLine sets the line (outline) of the text box.
func (t *TextBox) SetLine(line dml.Line) {
	line.ApplyToSpPr(&t.spPr)
	t.dirty = true
}

// SetShadow sets the shadow effect on the text box.
func (t *TextBox) SetShadow(shadow dml.Shadow) {
	shadow.ApplyToSpPr(&t.spPr)
	t.dirty = true
}

// SetHyperlink attaches an external-URL hyperlink to the text box. The External
// relationship is allocated on save.
func (t *TextBox) SetHyperlink(url string) *Hyperlink { return t.setHyperlinkURL(url) }

// SetActionHyperlink attaches a slide-show action hyperlink (e.g. ActionNextSlide)
// to the text box.
func (t *TextBox) SetActionHyperlink(action string) *Hyperlink { return t.setActionHyperlink(action) }

// SetHyperlinkToSlide attaches an internal jump to the slide at the given 0-based
// index; the RelTypeSlide relationship is allocated on save.
func (t *TextBox) SetHyperlinkToSlide(index int) *Hyperlink { return t.setSlideLink(index) }

// Common preset geometry names
const (
	PresetRect          = "rect"
	PresetRoundRect     = "roundRect"
	PresetEllipse       = "ellipse"
	PresetTriangle      = "triangle"
	PresetRightTriangle = "rtTriangle"
	PresetParallelogram = "parallelogram"
	PresetTrapezoid     = "trapezoid"
	PresetDiamond       = "diamond"
	PresetPentagon      = "pentagon"
	PresetHexagon       = "hexagon"
	PresetHeptagon      = "heptagon"
	PresetOctagon       = "octagon"
	PresetStar4         = "star4"
	PresetStar5         = "star5"
	PresetStar6         = "star6"
	PresetArrowRight    = "rightArrow"
	PresetArrowLeft     = "leftArrow"
	PresetArrowUp       = "upArrow"
	PresetArrowDown     = "downArrow"
	PresetCloud         = "cloud"
	PresetHeart         = "heart"
	PresetLightningBolt = "lightningBolt"
	PresetSun           = "sun"
	PresetMoon          = "moon"
	PresetSmileyFace    = "smileyFace"
	PresetCallout1      = "wedgeRectCallout"
	PresetCallout2      = "wedgeRoundRectCallout"
	PresetCallout3      = "wedgeEllipseCallout"
)
