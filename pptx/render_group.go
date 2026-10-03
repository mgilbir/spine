package pptx

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/mgilbir/forme/layout"
	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/opc"
	"github.com/mgilbir/spine/pptx/internal/oxml"
	"github.com/mgilbir/spine/render"
)

// renderDraw paints one shape; see drawShape in PrepareRender.
type renderDraw func(Shape, *oxml.Shape, *dml.SpPr, int, func(*Picture) ([]byte, renderImageKey)) ([]layout.Op, error)

// renderPartPicture resolves a picture's image through a part's relationships.
func (s *Slide) renderPartPicture(part string) func(*Picture) ([]byte, renderImageKey) {
	return func(v *Picture) ([]byte, renderImageKey) {
		for _, rel := range s.presentation.relationships[part] {
			if rel != nil && rel.ID == v.relID && rel.TargetMode != opc.TargetModeExternal {
				name := opc.ResolvePartName(part, rel.Target)
				return s.presentation.rawPartData(name), renderImageKey{part: name}
			}
		}
		return nil, renderImageKey{}
	}
}

// renderMap places child coordinates: x maps to ox + (x - cx) * sx, in EMU.
type renderMap struct {
	ox, oy, cx, cy, sx, sy float64
}

var renderIdentity = renderMap{sx: 1, sy: 1}

func (m renderMap) point(x, y float64) (float64, float64) {
	return m.ox + (x-m.cx)*m.sx, m.oy + (y-m.cy)*m.sy
}

// then composes a nested group's map inside m.
func (m renderMap) then(n renderMap) renderMap {
	ox, oy := m.point(n.ox, n.oy)
	return renderMap{ox: ox, oy: oy, cx: n.cx, cy: n.cy, sx: m.sx * n.sx, sy: m.sy * n.sy}
}

// xfrm maps a shape's offset and extent, rounding to whole EMU.
func (m renderMap) xfrm(x *dml.Xfrm) (*dml.Xfrm, error) {
	if x == nil || x.Off == nil || x.Ext == nil {
		return nil, fmt.Errorf("%w: grouped shape without geometry", render.ErrUnsupported)
	}
	x0, y0 := m.point(float64(x.Off.X), float64(x.Off.Y))
	w, h := float64(x.Ext.Cx)*m.sx, float64(x.Ext.Cy)*m.sy
	for _, v := range []float64{x0, y0, w, h} {
		if math.IsNaN(v) || math.Abs(v) > 1<<50 {
			return nil, fmt.Errorf("%w: grouped shape coordinate", render.ErrLimit)
		}
	}
	out := *x
	out.Off = &dml.OffXML{X: int64(math.Round(x0)), Y: int64(math.Round(y0))}
	out.Ext = &dml.ExtXML{Cx: int64(math.Round(w)), Cy: int64(math.Round(h))}
	return &out, nil
}

// renderGroup paints a group's children in document order, their geometry
// mapped from the group's child space to its frame. Text sizes and line
// widths do not scale with a group, as PowerPoint draws them. Rotated or
// group fills and effects, placeholders and tables inside groups fail.
// With warn set, a child that cannot be drawn is reported and left out.
//
// A rotated or flipped group turns its drawn children about its centre:
// shapes exactly, and text and pictures, approximately, moved upright.
func renderGroup(g *oxml.GroupShape, parent renderMap, draw renderDraw, connect renderConnect, picture func(*Picture) ([]byte, renderImageKey), depth int, warn func(error), colors *renderColors, maxSegments int) ([]layout.Op, error) {
	if depth > 32 {
		return nil, fmt.Errorf("%w: group depth", render.ErrLimit)
	}
	local := renderIdentity
	var turn renderShapeTransform
	if p := g.GrpSpPr; p != nil {
		if p.SolidFill != nil || p.GradFill != nil || p.BlipFill != nil || p.PattFill != nil || p.GrpFill != nil || renderEffects(p.EffectLst) || p.EffectDag != nil || p.Scene3d != nil || p.ExtLst != nil {
			return nil, fmt.Errorf("%w: group fill or effect", render.ErrUnsupported)
		}
		if x := p.Xfrm; x != nil {
			if x.Off == nil || x.Ext == nil || x.ChOff == nil || x.ChExt == nil {
				return nil, fmt.Errorf("%w: partial group transform", render.ErrUnsupported)
			}
			cx, cy := parent.point(float64(x.Off.X)+float64(x.Ext.Cx)/2, float64(x.Off.Y)+float64(x.Ext.Cy)/2)
			px := float64(dml.EMUsPerPixel)
			turn = renderShapeTransform{flipH: x.FlipH, flipV: x.FlipV, rot: float64(x.Rot) / 60000, cx: cx / px, cy: cy / px}
			scale := func(ext, ch int64) (float64, error) {
				switch {
				case ch > 0:
					return float64(ext) / float64(ch), nil
				case ext == 0:
					return 1, nil
				}
				return 0, fmt.Errorf("%w: group child extent", render.ErrInvalid)
			}
			sx, err := scale(x.Ext.Cx, x.ChExt.Cx)
			if err != nil {
				return nil, err
			}
			sy, err := scale(x.Ext.Cy, x.ChExt.Cy)
			if err != nil {
				return nil, err
			}
			local = renderMap{ox: float64(x.Off.X), oy: float64(x.Off.Y), cx: float64(x.ChOff.X), cy: float64(x.ChOff.Y), sx: sx, sy: sy}
		}
	}
	m := parent.then(local)
	order := g.ChildOrder()
	if len(order) == 0 && len(g.Shapes)+len(g.Pictures)+len(g.GroupShapes)+len(g.GraphicFrames)+len(g.ConnectionShapes)+len(g.AltContent)+len(g.RawXML) > 0 {
		return nil, fmt.Errorf("%w: group without a recorded order", render.ErrUnsupported)
	}
	var ops []layout.Op
	for _, ref := range order {
		var (
			drawn []layout.Op
			err   error
		)
		if renderHiddenChild(g.Shapes, g.Pictures, g.GroupShapes, g.ConnectionShapes, g.GraphicFrames, ref) {
			continue
		}
		switch ref.Kind {
		case oxml.ChildSp:
			if ref.Index >= len(g.Shapes) || g.Shapes[ref.Index] == nil || g.Shapes[ref.Index].SpPr == nil {
				return nil, fmt.Errorf("%w: grouped shape", render.ErrInvalid)
			}
			sp := *g.Shapes[ref.Index]
			if sp.NvSpPr != nil && sp.NvSpPr.NvPr != nil && sp.NvSpPr.NvPr.Ph != nil {
				return nil, fmt.Errorf("%w: grouped placeholder", render.ErrUnsupported)
			}
			props := *sp.SpPr
			if props.Xfrm, err = m.xfrm(props.Xfrm); err != nil {
				return nil, err
			}
			sp.SpPr = &props
			drawn, err = draw(oxmlShapeToGoShape(&sp), &sp, nil, -1, picture)
		case oxml.ChildPic:
			if ref.Index >= len(g.Pictures) || g.Pictures[ref.Index] == nil || g.Pictures[ref.Index].SpPr == nil {
				return nil, fmt.Errorf("%w: grouped picture", render.ErrInvalid)
			}
			pic := *g.Pictures[ref.Index]
			props := *pic.SpPr
			if props.Xfrm, err = m.xfrm(props.Xfrm); err != nil {
				return nil, err
			}
			pic.SpPr = &props
			drawn, err = draw(oxmlPictureToGoPicture(&pic), nil, pic.SpPr, -1, picture)
		case oxml.ChildGrpSp:
			if ref.Index >= len(g.GroupShapes) || g.GroupShapes[ref.Index] == nil {
				return nil, fmt.Errorf("%w: nested group", render.ErrInvalid)
			}
			drawn, err = renderGroup(g.GroupShapes[ref.Index], m, draw, connect, picture, depth+1, warn, colors, maxSegments)
		case oxml.ChildCxnSp:
			if ref.Index >= len(g.ConnectionShapes) || g.ConnectionShapes[ref.Index] == nil || g.ConnectionShapes[ref.Index].SpPr == nil {
				return nil, fmt.Errorf("%w: grouped connector", render.ErrInvalid)
			}
			cxn := *g.ConnectionShapes[ref.Index]
			props := *cxn.SpPr
			if props.Xfrm, err = m.xfrm(props.Xfrm); err != nil {
				return nil, err
			}
			cxn.SpPr = &props
			drawn, err = connect(&cxn)
		default:
			err = fmt.Errorf("%w: table or other content in a group", render.ErrUnsupported)
		}
		if err != nil {
			if warn == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
				return nil, err
			}
			warn(fmt.Errorf("pptx: grouped shape: %w", err))
			continue
		}
		ops = append(ops, drawn...)
	}
	return turn.ops(ops, colors, maxSegments)
}

// renderHiddenChild reports whether a shape tree child is hidden.
func renderHiddenChild(sp []*oxml.Shape, pic []*oxml.Picture, grp []*oxml.GroupShape, cxn []*oxml.ConnectionShape, frames []*oxml.GraphicFrame, ref oxml.ChildRef) bool {
	i := ref.Index
	if i < 0 {
		return false
	}
	switch {
	case ref.Kind == oxml.ChildSp && i < len(sp):
		return renderHidden(sp[i])
	case ref.Kind == oxml.ChildPic && i < len(pic):
		return renderHidden(pic[i])
	case ref.Kind == oxml.ChildGrpSp && i < len(grp):
		return renderHidden(grp[i])
	case ref.Kind == oxml.ChildCxnSp && i < len(cxn):
		return renderHidden(cxn[i])
	case ref.Kind == oxml.ChildGraphicFrame && i < len(frames):
		return renderHidden(frames[i])
	}
	return false
}

// renderSourceGroup returns the parsed p:grpSp behind a shape, or nil.
func (s *Slide) renderSourceGroup(index int) *oxml.GroupShape {
	if index < 0 || index >= len(s.shapeRefs) {
		return nil
	}
	ref := s.shapeRefs[index]
	if ref.Kind != oxml.ChildGrpSp || ref.Index < 0 || ref.Index >= len(s.sxModel.CSld.SpTree.GrpSp) {
		return nil
	}
	return s.sxModel.CSld.SpTree.GrpSp[ref.Index]
}
