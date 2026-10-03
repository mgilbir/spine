package pptx

import (
	"context"
	"errors"
	"fmt"

	"github.com/mgilbir/forme/layout"
	core "github.com/mgilbir/spine/internal/render"
	"github.com/mgilbir/spine/pptx/internal/oxml"
	"github.com/mgilbir/spine/render"
)

// renderShapeKey names a shape on a master or layout by element and its
// 1-based occurrence among its spTree siblings of that element.
type renderShapeKey struct {
	name       string
	occurrence int
}

// renderShapeErrs are the first unsupported nodes of a master or layout
// shape: anywhere in it, and outside its paragraphs, which is all a
// placeholder passes on.
type renderShapeErrs struct {
	any, inherited error
}

// renderInherited is a visible master or layout shape tree.
type renderInherited struct {
	data      *oxml.CommonSlideData
	part      string
	shapeErrs map[renderShapeKey]renderShapeErrs
}

// renderLayer paints a master's or layout's own shapes beneath the slide's.
// Placeholders are prompts and are not drawn. Text in these shapes resolves
// like slide text, and colors through the slide's color map, as PowerPoint
// shows the slide.
// With warn set, a shape that cannot be drawn is reported and left out.
func (s *Slide) renderLayer(layer renderInherited, budget *core.SourceBudget, draw renderDraw, connect renderConnect, warn func(error)) ([]layout.Op, error) {
	if layer.data == nil || layer.data.SpTree == nil {
		return nil, nil
	}
	t := layer.data.SpTree
	order := t.ChildOrder()
	if len(order) == 0 {
		// A tree built in code records no paint order; its default
		// placeholders are not drawn, but other shapes cannot be ordered.
		for _, sp := range t.Sp {
			if sp == nil || sp.NvSpPr == nil || sp.NvSpPr.NvPr == nil || sp.NvSpPr.NvPr.Ph == nil {
				return nil, fmt.Errorf("%w: inherited shapes without a recorded order", render.ErrUnsupported)
			}
		}
		if len(t.Pic)+len(t.GraphicFrame)+len(t.GrpSp)+len(t.CxnSp)+len(t.AltContent)+len(t.RawXML) > 0 {
			return nil, fmt.Errorf("%w: inherited shapes without a recorded order", render.ErrUnsupported)
		}
		return nil, nil
	}
	if len(order) > budget.Nodes {
		return nil, fmt.Errorf("%w: inherited shapes", render.ErrLimit)
	}
	budget.Nodes -= len(order)
	picture := s.renderPartPicture(layer.part)
	var ops []layout.Op
	for _, ref := range order {
		var (
			drawn []layout.Op
			err   error
		)
		if renderHiddenChild(t.Sp, t.Pic, t.GrpSp, t.CxnSp, t.GraphicFrame, ref) {
			continue
		}
		switch ref.Kind {
		case oxml.ChildSp:
			if ref.Index >= len(t.Sp) || t.Sp[ref.Index] == nil {
				return nil, fmt.Errorf("%w: inherited shape", render.ErrInvalid)
			}
			sp := t.Sp[ref.Index]
			if sp.NvSpPr != nil && sp.NvSpPr.NvPr != nil && sp.NvSpPr.NvPr.Ph != nil {
				continue
			}
			if err = layer.shapeErrs[renderShapeKey{name: "sp", occurrence: ref.Index + 1}].any; err == nil {
				drawn, err = draw(oxmlShapeToGoShape(sp), sp, nil, -1, picture)
			}
		case oxml.ChildPic:
			if ref.Index >= len(t.Pic) || t.Pic[ref.Index] == nil {
				return nil, fmt.Errorf("%w: inherited picture", render.ErrInvalid)
			}
			pic := t.Pic[ref.Index]
			if pic.NvPicPr != nil && pic.NvPicPr.NvPr != nil && pic.NvPicPr.NvPr.Ph != nil {
				continue
			}
			if err = layer.shapeErrs[renderShapeKey{name: "pic", occurrence: ref.Index + 1}].any; err == nil {
				drawn, err = draw(oxmlPictureToGoPicture(pic), nil, pic.SpPr, -1, picture)
			}
		case oxml.ChildGrpSp:
			if ref.Index >= len(t.GrpSp) || t.GrpSp[ref.Index] == nil {
				return nil, fmt.Errorf("%w: inherited group", render.ErrInvalid)
			}
			if err = layer.shapeErrs[renderShapeKey{name: "grpSp", occurrence: ref.Index + 1}].any; err == nil {
				drawn, err = renderGroup(t.GrpSp[ref.Index], renderIdentity, draw, connect, picture, 0, warn)
			}
		case oxml.ChildCxnSp:
			if ref.Index >= len(t.CxnSp) || t.CxnSp[ref.Index] == nil {
				return nil, fmt.Errorf("%w: inherited connector", render.ErrInvalid)
			}
			if err = layer.shapeErrs[renderShapeKey{name: "cxnSp", occurrence: ref.Index + 1}].any; err == nil {
				drawn, err = connect(t.CxnSp[ref.Index])
			}
		default:
			err = fmt.Errorf("%w: inherited table or other content", render.ErrUnsupported)
		}
		if err != nil {
			err = fmt.Errorf("pptx: %s shape: %w", layer.part, err)
			if warn == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil, err
			}
			warn(err)
			continue
		}
		ops = append(ops, drawn...)
	}
	return ops, nil
}
