package pptxrender

import (
	"context"

	"github.com/mgilbir/spine/common/dml"
	coxml "github.com/mgilbir/spine/common/oxml"
	"github.com/mgilbir/spine/opc"
	"github.com/mgilbir/spine/pptx"
	"github.com/mgilbir/spine/pptx/internal/oxml"
	"github.com/mgilbir/spine/pptx/internal/view"
	"github.com/mgilbir/spine/render"
)

// PrepareSlide prepares an immutable render snapshot of a slide; see
// docs/rendering.md for what it draws. The slide and its presentation must
// not change while it runs; the page it returns does not depend on them.
func PrepareSlide(ctx context.Context, slide *pptx.Slide, opts render.Options) (*render.Page, error) {
	if slide == nil {
		return nil, render.ErrInvalid
	}
	return newRenderSlide(slide).PrepareRender(ctx, opts)
}

// renderSlide is a slide as the renderer reads it, snapshotted from the
// pptx package through pptx/internal/view.
type renderSlide struct {
	slide        *pptx.Slide
	load         func() (*oxml.Slide, []any, []oxml.ChildRef)
	loaded       bool
	sxModel      *oxml.Slide
	shapes       []pptx.Shape
	shapeRefs    []oxml.ChildRef
	partName     string
	index        int
	layout       *renderLayout
	presentation *renderPackage
}

type renderLayout struct {
	layoutXML *oxml.SlideLayout
	partName  string
	master    *renderMaster
}

type renderMaster struct {
	masterXML         *oxml.SlideMaster
	partName          string
	resolvedThemePart string
}

type renderPackage struct {
	reader        *opc.ReadCloser
	presentation  *oxml.Presentation
	relationships map[string][]*opc.Relationship
	otherParts    map[string]*coxml.RawPart
	themeData     map[string][]byte
	themeEditors  map[string]*dml.ThemeEditor
	partData      func(string) []byte
	width, height dml.EMU
}

func newRenderSlide(s *pptx.Slide) *renderSlide {
	v := view.SlideOf(s)
	out := &renderSlide{slide: s, load: v.Load, partName: v.PartName, index: v.Index}
	if p := v.Package; p != nil {
		out.presentation = &renderPackage{reader: p.Reader, presentation: p.Presentation, relationships: p.Relationships, otherParts: p.OtherParts,
			themeData: p.ThemeData, themeEditors: p.ThemeEditors, partData: p.PartData, width: p.Width, height: p.Height}
	} else {
		out.presentation = &renderPackage{}
	}
	if l := v.Layout; l != nil {
		out.layout = &renderLayout{layoutXML: l.XML, partName: l.PartName}
		if m := l.Master; m != nil {
			out.layout.master = &renderMaster{masterXML: m.XML, partName: m.PartName, resolvedThemePart: m.ThemePart}
		}
	}
	return out
}

// sx parses the slide part on first use; callers check its source bytes
// first.
func (s *renderSlide) sx() *oxml.Slide {
	if !s.loaded {
		s.loaded = true
		model, shapes, refs := s.load()
		s.sxModel, s.shapeRefs = model, refs
		for _, sh := range shapes {
			s.shapes = append(s.shapes, sh.(pptx.Shape))
		}
	}
	return s.sxModel
}

func (s *renderSlide) shapeList() []pptx.Shape {
	s.sx()
	return s.shapes
}

// relTargetPart resolves one of the slide's internal relationships.
func (s *renderSlide) relTargetPart(relID string) string {
	return s.renderPartTarget(s.partName, relID)
}

func (p *renderPackage) rawPartData(name string) []byte {
	if p.partData == nil {
		return nil
	}
	return p.partData(name)
}

func (p *renderPackage) slideDimensions() (w, h dml.EMU) { return p.width, p.height }
