// Package view gives the slide renderer, pptx/pptxrender, the parts of the
// pptx package's model it reads but the package does not export. The pptx
// package sets every function at initialization; the renderer only calls
// them. Values of pptx types travel as any, since this package cannot name
// them; the renderer converts them back. Nothing here imports a rendering
// dependency.
package view

import (
	"github.com/mgilbir/spine/common/dml"
	coxml "github.com/mgilbir/spine/common/oxml"
	xmlb "github.com/mgilbir/spine/common/xml"
	"github.com/mgilbir/spine/opc"
	"github.com/mgilbir/spine/pptx/internal/oxml"
)

// Conversions and writers the renderer shares with saving.
var (
	ChartRelID        func(gf *oxml.GraphicFrame) string
	ShapeFromOxml     func(sp *oxml.Shape) any
	PictureFromOxml   func(pic *oxml.Picture) any
	UpdateXfrm        func(spPr *dml.SpPr, base any)
	UpdateTxBody      func(dst **dml.TxBody, tf any)
	TextFrameToOxml   func(tf any) *dml.TxBody
	PlaceholderToOxml func(ph any, id uint32) *oxml.Shape
	DefaultTextStyle  func(b *xmlb.Builder)
	ApplyShapeStyle   func(dst, src *dml.SpPr)
)

// Slide is a slide's state as the renderer reads it. Load parses the slide
// part on first use and returns the model with its shapes; the renderer
// calls it only after checking the part's source bytes.
type Slide struct {
	Load     func() (model *oxml.Slide, shapes []any, refs []oxml.ChildRef)
	PartName string
	Index    int
	Layout   *Layout
	Package  *Package
}

// Layout is a slide layout's state; Master is nil without one.
type Layout struct {
	XML      *oxml.SlideLayout
	PartName string
	Master   *Master
}

// Master is a slide master's state.
type Master struct {
	XML       *oxml.SlideMaster
	PartName  string
	ThemePart string
}

// Package is a presentation's state. Reader is nil for a deck built in
// code.
type Package struct {
	Reader        *opc.ReadCloser
	Presentation  *oxml.Presentation
	Relationships map[string][]*opc.Relationship
	OtherParts    map[string]*coxml.RawPart
	ThemeData     map[string][]byte
	ThemeEditors  map[string]*dml.ThemeEditor
	PartData      func(name string) []byte
	Width, Height dml.EMU
}

// SlideOf snapshots a *pptx.Slide.
var SlideOf func(s any) *Slide

// Shape is a shape's unexported state the renderer reads: what applies to
// its kind is set, the rest left zero.
type Shape struct {
	SpPr      dml.SpPr
	TextFrame any // *pptx.TextFrame, or nil
	Preset    string
	Dirty     bool
	Base      any // *pptx.BaseShape, for UpdateXfrm

	// Placeholders.
	PhType, Orientation, Size, FieldType string
	Idx                                  uint32

	// Pictures.
	Crop         [4]float64 // left, top, right, bottom
	Effects      []*dml.BlipEffect
	Opacity      *float64
	Media        bool
	HasImageData bool
	RelID        string
	ImagePart    string // the part the picture's relationship names

	// Chart frames.
	Part string
}

// ShapeOf snapshots a pptx.Shape.
var ShapeOf func(sh any) Shape

// TextFrameDirty reports whether a *pptx.TextFrame has unsaved edits.
var TextFrameDirty func(tf any) bool
