package pptx

import (
	"context"
	"fmt"

	"github.com/mgilbir/forme/layout"
	"github.com/mgilbir/spine/common/dml"
	core "github.com/mgilbir/spine/internal/render"
	"github.com/mgilbir/spine/pptx/internal/oxml"
	"github.com/mgilbir/spine/render"
)

// renderPlaceholder is what a slide placeholder inherits: its layout and
// master placeholders and the master text style for its type.
type renderPlaceholder struct {
	// sources are the slide, layout and master placeholders, nearest first;
	// layout or master may be nil.
	sources [3]*oxml.Shape
	style   *dml.LstStyle
}

// renderPlaceholderType returns a placeholder's type; absent is obj.
func renderPlaceholderType(ph *oxml.Placeholder) string {
	if ph.Type == "" {
		return "obj"
	}
	return ph.Type
}

// renderBaseType is the master placeholder type a placeholder inherits from.
func renderBaseType(typ string) string {
	switch typ {
	case "title", "ctrTitle":
		return "title"
	case "dt", "ftr", "sldNum", "hdr":
		return typ
	}
	return "body"
}

// renderMatch finds the one placeholder in a tree that ph inherits from: by
// index on a layout, by type on a master. More than one candidate fails.
func renderMatch(t *oxml.ShapeTree, ph *oxml.Placeholder, byIndex bool) (int, error) {
	if t == nil {
		return -1, nil
	}
	typ := renderPlaceholderType(ph)
	find := func(keep func(*oxml.Placeholder) bool) []int {
		var out []int
		for i, sp := range t.Sp {
			if sp != nil && sp.NvSpPr != nil && sp.NvSpPr.NvPr != nil && sp.NvSpPr.NvPr.Ph != nil && keep(sp.NvSpPr.NvPr.Ph) {
				out = append(out, i)
			}
		}
		return out
	}
	one := func(found []int) (int, error) {
		switch len(found) {
		case 0:
			return -1, nil
		case 1:
			return found[0], nil
		}
		return -1, fmt.Errorf("%w: ambiguous inherited placeholder", render.ErrUnsupported)
	}
	if byIndex {
		same := find(func(o *oxml.Placeholder) bool { return o.Idx == ph.Idx })
		if len(same) > 1 {
			same = find(func(o *oxml.Placeholder) bool { return o.Idx == ph.Idx && renderPlaceholderType(o) == typ })
		}
		if len(same) > 0 {
			return one(same)
		}
		return one(find(func(o *oxml.Placeholder) bool { return renderPlaceholderType(o) == typ }))
	}
	base := renderBaseType(typ)
	return one(find(func(o *oxml.Placeholder) bool { return renderPlaceholderType(o) == base }))
}

// renderPlaceholderBase resolves a slide placeholder's inheritance. The
// layout and master placeholders it uses must be free of unsupported content
// outside their prompt paragraphs.
func (s *Slide) renderPlaceholderBase(sp *oxml.Shape, ph *oxml.Placeholder, layoutErrs, masterErrs map[renderShapeKey]renderShapeErrs, styleErrs map[string]error) (*renderPlaceholder, error) {
	r := &renderPlaceholder{sources: [3]*oxml.Shape{sp}}
	inheritFrom := ph
	if l := s.layout; l != nil && l.layoutXML != nil && l.layoutXML.CSld != nil {
		i, err := renderMatch(l.layoutXML.CSld.SpTree, ph, true)
		if err != nil {
			return nil, err
		}
		if i >= 0 {
			if err = layoutErrs[renderShapeKey{name: "sp", occurrence: i + 1}].inherited; err != nil {
				return nil, fmt.Errorf("pptx: layout placeholder: %w", err)
			}
			r.sources[1] = l.layoutXML.CSld.SpTree.Sp[i]
			inheritFrom = r.sources[1].NvSpPr.NvPr.Ph
		}
	}
	if l := s.layout; l != nil && l.master != nil && l.master.masterXML != nil && l.master.masterXML.CSld != nil {
		m := l.master.masterXML
		i, err := renderMatch(m.CSld.SpTree, inheritFrom, false)
		if err != nil {
			return nil, err
		}
		if i >= 0 {
			if err = masterErrs[renderShapeKey{name: "sp", occurrence: i + 1}].inherited; err != nil {
				return nil, fmt.Errorf("pptx: master placeholder: %w", err)
			}
			r.sources[2] = m.CSld.SpTree.Sp[i]
		}
		name := "bodyStyle"
		switch renderBaseType(renderPlaceholderType(ph)) {
		case "title":
			name = "titleStyle"
		case "dt", "ftr", "sldNum", "hdr":
			name = "otherStyle"
		}
		if err = styleErrs[name]; err != nil {
			return nil, fmt.Errorf("pptx: master %s: %w", name, err)
		}
		if m.TxStyles != nil {
			r.style = map[string]*dml.LstStyle{"titleStyle": m.TxStyles.TitleStyle, "bodyStyle": m.TxStyles.BodyStyle, "otherStyle": m.TxStyles.OtherStyle}[name]
		}
	}
	for _, src := range r.sources {
		if src != nil && src.ExtLst != nil {
			return nil, fmt.Errorf("%w: placeholder extension", render.ErrUnsupported)
		}
	}
	return r, nil
}

// styleRef returns the nearest style reference.
func (r *renderPlaceholder) styleRef() *dml.Style {
	for _, src := range r.sources {
		if src != nil && src.Style != nil {
			return src.Style
		}
	}
	return nil
}

// spPr merges shape properties, nearest first, property by property.
func (r *renderPlaceholder) spPr() *dml.SpPr {
	var out dml.SpPr
	for i := len(r.sources) - 1; i >= 0; i-- {
		src := r.sources[i]
		if src == nil || src.SpPr == nil {
			continue
		}
		p := src.SpPr
		if p.Xfrm != nil && p.Xfrm.Off != nil && p.Xfrm.Ext != nil {
			out.Xfrm = p.Xfrm
		}
		if p.PrstGeom != nil || p.CustGeom != nil {
			out.PrstGeom, out.CustGeom = p.PrstGeom, p.CustGeom
		}
		if p.NoFill != nil || p.SolidFill != nil || p.GradFill != nil || p.BlipFill != nil || p.PattFill != nil || p.GrpFill != nil {
			out.NoFill, out.SolidFill, out.GradFill, out.BlipFill, out.PattFill, out.GrpFill = p.NoFill, p.SolidFill, p.GradFill, p.BlipFill, p.PattFill, p.GrpFill
		}
		if p.Ln != nil {
			out.Ln = p.Ln
		}
		if p.EffectLst != nil || p.EffectDag != nil {
			out.EffectLst, out.EffectDag = p.EffectLst, p.EffectDag
		}
		if p.Scene3d != nil {
			out.Scene3d = p.Scene3d
		}
		if p.Sp3d != nil {
			out.Sp3d = p.Sp3d
		}
		if p.BwMode != "" {
			out.BwMode = p.BwMode
		}
		if p.ExtLst != nil {
			out.ExtLst = p.ExtLst
		}
	}
	return &out
}

// bodyPr merges body properties, nearest first, attribute by attribute; own
// is the slide body as a save writes it.
func (r *renderPlaceholder) bodyPr(own *dml.BodyPr) *dml.BodyPr {
	var out dml.BodyPr
	layers := []*dml.BodyPr{nil, nil, own}
	for i := 2; i >= 1; i-- {
		if src := r.sources[i]; src != nil && src.TxBody != nil {
			layers[2-i] = src.TxBody.BodyPr
		}
	}
	for _, b := range layers {
		if b == nil {
			continue
		}
		for _, p := range []struct{ to, from **int64 }{{&out.LIns, &b.LIns}, {&out.TIns, &b.TIns}, {&out.RIns, &b.RIns}, {&out.BIns, &b.BIns}} {
			if *p.from != nil {
				*p.to = *p.from
			}
		}
		for _, p := range []struct{ to, from *string }{{&out.Anchor, &b.Anchor}, {&out.Wrap, &b.Wrap}, {&out.Vert, &b.Vert}, {&out.VertOverflow, &b.VertOverflow}, {&out.HorzOverflow, &b.HorzOverflow}} {
			if *p.from != "" {
				*p.to = *p.from
			}
		}
		for _, p := range []struct{ to, from **bool }{{&out.SpcFirstLastPara, &b.SpcFirstLastPara}, {&out.RtlCol, &b.RtlCol}, {&out.FromWordArt, &b.FromWordArt}, {&out.AnchorCtr, &b.AnchorCtr}, {&out.ForceAA, &b.ForceAA}, {&out.UpRight, &b.UpRight}, {&out.CompatLnSpc, &b.CompatLnSpc}} {
			if *p.from != nil {
				*p.to = *p.from
			}
		}
		if b.Rot != nil {
			out.Rot = b.Rot
		}
		if b.NumCol != 0 {
			out.NumCol = b.NumCol
		}
		if b.SpcCol != nil {
			out.SpcCol = b.SpcCol
		}
		if b.NoAutofit != nil || b.NormAutofit != nil || b.SpAutoFit != nil {
			out.NoAutofit, out.NormAutofit, out.SpAutoFit = b.NoAutofit, b.NormAutofit, b.SpAutoFit
		}
		if b.PrstTxWarp != nil {
			out.PrstTxWarp = b.PrstTxWarp
		}
		if b.Scene3d != nil {
			out.Scene3d = b.Scene3d
		}
		if b.Sp3d != nil {
			out.Sp3d = b.Sp3d
		}
		if b.FlatTx != nil {
			out.FlatTx = b.FlatTx
		}
		if b.ExtLst != nil {
			out.ExtLst = b.ExtLst
		}
	}
	return &out
}

// chain is the list styles placeholder text inherits through: the layout and
// master placeholders' list styles, then the master text style for its type.
// Whether the presentation default text style follows is unspecified, so the
// result must agree with and without it.
func (r *renderPlaceholder) chain(t *renderTextStyles) renderListChain {
	var inherited []*dml.LstStyle
	for _, src := range r.sources[1:] {
		if src != nil && src.TxBody != nil {
			inherited = append(inherited, src.TxBody.LstStyle)
		}
	}
	inherited = append(inherited, r.style)
	return renderListChain{inherited: inherited, variants: [][]*dml.LstStyle{nil, {t.defaults}}}
}

// renderPlaceholderShape paints a slide placeholder from its own and its
// inherited properties. The slide placeholder is taken as a save writes it.
func (s *Slide) renderPlaceholderShape(ctx context.Context, v *PlaceholderShape, sp *oxml.Shape, colors *renderColors, limits render.Limits, breaker *core.TextLayout, fonts *slideRenderFonts, styles *renderTextStyles, layoutErrs, masterErrs map[renderShapeKey]renderShapeErrs, styleErrs map[string]error, soft func(error) error) ([]layout.Op, error) {
	if sp == nil {
		// A placeholder added through the API is written from the model.
		sp = placeholderToOxml(v, 0)
	} else if v.dirty {
		// A save flushes edited geometry and placeholder attributes.
		copied := *sp
		props := dml.SpPr{}
		if sp.SpPr != nil {
			props = *sp.SpPr
		}
		updateXfrm(&props, &v.BaseShape)
		copied.SpPr = &props
		nv := *sp.NvSpPr
		pr := *nv.NvPr
		pr.Ph = &oxml.Placeholder{Type: string(v.phType), Orient: string(v.orientation), Sz: string(v.size), Idx: v.idx}
		nv.NvPr = &pr
		copied.NvSpPr = &nv
		sp = &copied
	}
	if v.fieldType != "" {
		return nil, fmt.Errorf("%w: field placeholder", render.ErrUnsupported)
	}
	ph, err := s.renderPlaceholderBase(sp, sp.NvSpPr.NvPr.Ph, layoutErrs, masterErrs, styleErrs)
	if err != nil {
		return nil, err
	}
	props := ph.spPr()
	if props.Xfrm == nil {
		return nil, fmt.Errorf("%w: placeholder without geometry", render.ErrInvalid)
	}
	preset := "rect"
	if props.PrstGeom != nil {
		preset = props.PrstGeom.Prst
	}
	box := &AutoShape{BaseShape: BaseShape{x: dml.EMU(props.Xfrm.Off.X), y: dml.EMU(props.Xfrm.Off.Y), width: dml.EMU(props.Xfrm.Ext.Cx), height: dml.EMU(props.Xfrm.Ext.Cy)}, presetGeometry: preset, textFrame: v.textFrame}
	drawn, geometry, err := renderAutoShape(box, props, ph.styleRef(), colors, limits)
	if err != nil {
		return nil, err
	}
	text, err := renderShapeText(ctx, sp, box, geometry, breaker, fonts, styles, ph)
	if err == nil && geometry.turned && len(text) > 0 {
		err = colors.approximate(fmt.Errorf("%w: text of a rotated or flipped shape drawn upright", render.ErrUnsupported))
	}
	if err = soft(renderTextLeftOut(v, err)); err != nil {
		return nil, err
	}
	return append(drawn, text...), nil
}
