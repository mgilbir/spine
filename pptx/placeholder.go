package pptx

import (
	"github.com/mgilbir/spine/common/dml"
)

// PlaceholderType represents the type of a placeholder.
type PlaceholderType string

const (
	PlaceholderTitle          PlaceholderType = "title"
	PlaceholderBody           PlaceholderType = "body"
	PlaceholderCenteredTitle  PlaceholderType = "ctrTitle"
	PlaceholderSubtitle       PlaceholderType = "subTitle"
	PlaceholderDateTime       PlaceholderType = "dt"
	PlaceholderSlideNumber    PlaceholderType = "sldNum"
	PlaceholderFooter         PlaceholderType = "ftr"
	PlaceholderHeader         PlaceholderType = "hdr"
	PlaceholderObject         PlaceholderType = "obj"
	PlaceholderChart          PlaceholderType = "chart"
	PlaceholderTable          PlaceholderType = "tbl"
	PlaceholderClipArt        PlaceholderType = "clipArt"
	PlaceholderDiagram        PlaceholderType = "dgm"
	PlaceholderMedia          PlaceholderType = "media"
	PlaceholderSlideImage     PlaceholderType = "sldImg"
	PlaceholderPicture        PlaceholderType = "pic"
)

// PlaceholderOrientation specifies the orientation of a placeholder.
type PlaceholderOrientation string

const (
	PlaceholderOrientationHorizontal PlaceholderOrientation = "horz"
	PlaceholderOrientationVertical   PlaceholderOrientation = "vert"
)

// PlaceholderSize specifies the size hint for a placeholder.
type PlaceholderSize string

const (
	PlaceholderSizeFull    PlaceholderSize = "full"
	PlaceholderSizeHalf    PlaceholderSize = "half"
	PlaceholderSizeQuarter PlaceholderSize = "quarter"
)

// PlaceholderShape represents a placeholder shape on a slide.
type PlaceholderShape struct {
	BaseShape
	phType      PlaceholderType
	orientation PlaceholderOrientation
	size        PlaceholderSize
	idx         uint32
	textFrame   *TextFrame
}

// NewPlaceholderShape creates a new placeholder shape.
func NewPlaceholderShape(phType PlaceholderType) *PlaceholderShape {
	return &PlaceholderShape{
		phType:    phType,
		textFrame: NewTextFrame(),
	}
}

// ShapeType returns ShapeTypePlaceholder.
func (p *PlaceholderShape) ShapeType() ShapeType {
	return ShapeTypePlaceholder
}

// PlaceholderType returns the type of the placeholder.
func (p *PlaceholderShape) PlaceholderType() PlaceholderType {
	return p.phType
}

// Orientation returns the orientation of the placeholder.
func (p *PlaceholderShape) Orientation() PlaceholderOrientation {
	return p.orientation
}

// SetOrientation sets the orientation of the placeholder.
func (p *PlaceholderShape) SetOrientation(orient PlaceholderOrientation) {
	p.orientation = orient
}

// Size returns the size hint of the placeholder.
func (p *PlaceholderShape) PlaceholderSize() PlaceholderSize {
	return p.size
}

// SetPlaceholderSize sets the size hint of the placeholder.
func (p *PlaceholderShape) SetPlaceholderSize(size PlaceholderSize) {
	p.size = size
}

// Index returns the placeholder index.
func (p *PlaceholderShape) Index() uint32 {
	return p.idx
}

// SetIndex sets the placeholder index.
func (p *PlaceholderShape) SetIndex(idx uint32) {
	p.idx = idx
}

// TextFrame returns the text frame for the placeholder.
func (p *PlaceholderShape) TextFrame() *TextFrame {
	return p.textFrame
}

// SetText sets the text content of the placeholder.
func (p *PlaceholderShape) SetText(text string) {
	if p.textFrame == nil {
		p.textFrame = NewTextFrame()
	}
	p.textFrame.SetText(text)
}

// Text returns the text content of the placeholder.
func (p *PlaceholderShape) Text() string {
	if p.textFrame == nil {
		return ""
	}
	return p.textFrame.Text()
}

// IsTitle returns true if this is a title placeholder.
func (p *PlaceholderShape) IsTitle() bool {
	return p.phType == PlaceholderTitle || p.phType == PlaceholderCenteredTitle
}

// IsBody returns true if this is a body/content placeholder.
func (p *PlaceholderShape) IsBody() bool {
	return p.phType == PlaceholderBody || p.phType == PlaceholderObject
}

// DefaultTitlePlaceholder creates a title placeholder with default settings.
func DefaultTitlePlaceholder() *PlaceholderShape {
	ph := NewPlaceholderShape(PlaceholderTitle)
	ph.SetPosition(dml.Inches(0.5), dml.Inches(0.3))
	ph.SetSize(dml.Inches(9), dml.Inches(1.2))
	return ph
}

// DefaultBodyPlaceholder creates a body placeholder with default settings.
func DefaultBodyPlaceholder() *PlaceholderShape {
	ph := NewPlaceholderShape(PlaceholderBody)
	ph.SetPosition(dml.Inches(0.5), dml.Inches(1.6))
	ph.SetSize(dml.Inches(9), dml.Inches(5.1))
	return ph
}

// DefaultCenteredTitlePlaceholder creates a centered title placeholder.
func DefaultCenteredTitlePlaceholder() *PlaceholderShape {
	ph := NewPlaceholderShape(PlaceholderCenteredTitle)
	ph.SetPosition(dml.Inches(0.5), dml.Inches(2.5))
	ph.SetSize(dml.Inches(9), dml.Inches(1.5))
	return ph
}

// DefaultSubtitlePlaceholder creates a subtitle placeholder.
func DefaultSubtitlePlaceholder() *PlaceholderShape {
	ph := NewPlaceholderShape(PlaceholderSubtitle)
	ph.SetPosition(dml.Inches(0.5), dml.Inches(4.0))
	ph.SetSize(dml.Inches(9), dml.Inches(1.0))
	return ph
}
