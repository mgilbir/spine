package pptx

import (
	"fmt"

	"github.com/mgilbir/forme/layout"
	"github.com/mgilbir/spine/common/dml"
	core "github.com/mgilbir/spine/internal/render"
	"github.com/mgilbir/spine/opc"
	"github.com/mgilbir/spine/pptx/internal/oxml"
	"github.com/mgilbir/spine/render"
)

// renderShapeKey names a shape on a master or layout by element and its
// 1-based occurrence among its spTree siblings of that element.
type renderShapeKey struct {
	name       string
	occurrence int
}

// renderInherited is a visible master or layout shape tree.
type renderInherited struct {
	data      *oxml.CommonSlideData
	part      string
	shapeErrs map[renderShapeKey]error
}

// renderLayer paints a master's or layout's own shapes beneath the slide's.
// Placeholders are prompts and are not drawn. Text in these shapes resolves
// like slide text, and colors through the slide's color map, as PowerPoint
// shows the slide.
func (s *Slide) renderLayer(layer renderInherited, budget *core.SourceBudget, draw func(Shape, *oxml.Shape, *dml.SpPr, int, func(*Picture) ([]byte, renderImageKey)) ([]layout.Op, error)) ([]layout.Op, error) {
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
	picture := func(v *Picture) ([]byte, renderImageKey) {
		for _, rel := range s.presentation.relationships[layer.part] {
			if rel != nil && rel.ID == v.relID && rel.TargetMode != opc.TargetModeExternal {
				name := opc.ResolvePartName(layer.part, rel.Target)
				return s.presentation.rawPartData(name), renderImageKey{part: name}
			}
		}
		return nil, renderImageKey{}
	}
	var ops []layout.Op
	for _, ref := range order {
		var (
			drawn []layout.Op
			err   error
		)
		switch ref.Kind {
		case oxml.ChildSp:
			if ref.Index >= len(t.Sp) || t.Sp[ref.Index] == nil {
				return nil, fmt.Errorf("%w: inherited shape", render.ErrInvalid)
			}
			sp := t.Sp[ref.Index]
			if sp.NvSpPr != nil && sp.NvSpPr.NvPr != nil && sp.NvSpPr.NvPr.Ph != nil {
				continue
			}
			if err = layer.shapeErrs[renderShapeKey{name: "sp", occurrence: ref.Index + 1}]; err == nil {
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
			if err = layer.shapeErrs[renderShapeKey{name: "pic", occurrence: ref.Index + 1}]; err == nil {
				drawn, err = draw(oxmlPictureToGoPicture(pic), nil, pic.SpPr, -1, picture)
			}
		default:
			err = fmt.Errorf("%w: inherited group, table, connector or other content", render.ErrUnsupported)
		}
		if err != nil {
			return nil, fmt.Errorf("pptx: %s shape: %w", layer.part, err)
		}
		ops = append(ops, drawn...)
	}
	return ops, nil
}
