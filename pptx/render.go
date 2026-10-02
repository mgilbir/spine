package pptx

import (
	"context"
	"encoding/xml"
	"fmt"
	"strings"

	"github.com/mgilbir/forme/layout"
	"github.com/mgilbir/forme/style"
	"github.com/mgilbir/spine/common/dml"
	core "github.com/mgilbir/spine/internal/render"
	"github.com/mgilbir/spine/pptx/internal/oxml"
	"github.com/mgilbir/spine/render"
)

// PrepareRender prepares this static slide, including unsaved edits, for PNG or
// SVG output. The first supported profile is explicit RGB backgrounds, filled
// rectangles/ellipses with no stroke, and uncropped embedded PNG/JPEG pictures.
// Text, inherited visible master/layout objects, theme fills, transformations,
// effects and other content fail explicitly. Hidden slides can be selected.
// Preparation does not synchronize or save source parts. Caller edits must not
// race with preparation; returned pages can be rendered concurrently.
// See docs/rendering.md for capability and resource contracts.
func (s *Slide) PrepareRender(ctx context.Context, opts render.Options) (*render.Page, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s == nil || s.presentation == nil {
		return nil, fmt.Errorf("%w: detached slide", render.ErrInvalid)
	}
	resolved, err := core.ResolveLimits(opts.Limits)
	if err != nil {
		return nil, err
	}
	opts.Limits = resolved
	budget, err := core.NewSourceBudget(opts.MaxSourceBytes, opts.MaxLayoutNodes)
	if err != nil {
		return nil, err
	}
	// Check original bytes before lazy parsing can discard unknown markup. The
	// check is deliberately conservative for source features subsequently removed.
	if s.presentation.reader != nil {
		if file := s.presentation.reader.GetFile(s.partName); file != nil {
			stream, e := file.Open()
			if e != nil {
				return nil, e
			}
			e = budget.CheckReader(ctx, stream, slideRenderProfile)
			closeErr := stream.Close()
			if e != nil {
				return nil, fmt.Errorf("pptx: %s: %w", s.partName, e)
			}
			if closeErr != nil {
				return nil, closeErr
			}
		}
	}
	model := s.sx()
	if model == nil || model.CSld == nil {
		return nil, fmt.Errorf("%w: missing slide data", render.ErrInvalid)
	}
	if len(model.AlternateContent) > 0 || model.Timing != nil || model.Transition != nil || model.ExtLst != nil {
		return nil, fmt.Errorf("%w: slide animation/extension", render.ErrUnsupported)
	}
	layers := []*oxml.CommonSlideData{}
	if s.layout != nil {
		if s.layout.master != nil && s.layout.master.masterXML != nil {
			m := s.layout.master.masterXML
			if len(m.AlternateContent) > 0 || m.ExtLst != nil || m.Hf != nil {
				return nil, fmt.Errorf("%w: master extension/footer", render.ErrUnsupported)
			}
			layers = append(layers, m.CSld)
		}
		if s.layout.layoutXML != nil {
			m := s.layout.layoutXML
			if len(m.AlternateContent) > 0 || m.ExtLst != nil || m.Hf != nil {
				return nil, fmt.Errorf("%w: layout extension/footer", render.ErrUnsupported)
			}
			layers = append(layers, m.CSld)
		}
	}
	if s.layout != nil {
		if s.layout.layoutXML != nil && len(s.layout.layoutXML.SourceXML) > 0 {
			if err = budget.CheckXML(ctx, s.layout.layoutXML.SourceXML, inheritedRenderProfile); err != nil {
				return nil, fmt.Errorf("pptx: layout: %w", err)
			}
		}
		if s.layout.master != nil && s.layout.master.masterXML != nil && len(s.layout.master.masterXML.SourceXML) > 0 {
			if err = budget.CheckXML(ctx, s.layout.master.masterXML.SourceXML, inheritedRenderProfile); err != nil {
				return nil, fmt.Errorf("pptx: master: %w", err)
			}
		}
	}
	for _, layer := range layers {
		if err = renderInheritance(layer, budget); err != nil {
			return nil, err
		}
	}
	layers = append(layers, model.CSld)
	w, h := s.presentation.slideDimensions()
	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("%w: slide dimensions", render.ErrInvalid)
	}
	for _, v := range []dml.EMU{w, h} {
		if _, ok := style.FromPx(float64(v) / float64(dml.EMUsPerPixel)); !ok {
			return nil, fmt.Errorf("%w: slide coordinate", render.ErrLimit)
		}
	}
	background := style.RGBA{R: 255, G: 255, B: 255, A: 1}
	for _, layer := range layers {
		if layer == nil || layer.Bg == nil {
			continue
		}
		bg := layer.Bg
		if bg.BgRef != nil || bg.BgPr == nil || bg.BwMode != "" {
			return nil, fmt.Errorf("%w: referenced slide background", render.ErrUnsupported)
		}
		v := bg.BgPr
		if v.GradFill != nil || v.BlipFill != nil || v.PattFill != nil || v.EffectLst != nil || v.ExtLst != nil {
			return nil, fmt.Errorf("%w: background fill/effect", render.ErrUnsupported)
		}
		if v.NoFill != nil && v.SolidFill == nil {
			background = style.RGBA{R: 255, G: 255, B: 255, A: 1}
			continue
		}
		background, err = renderSolid(v.SolidFill)
		if err != nil {
			return nil, err
		}
	}
	if err = renderTreeBase(model.CSld, budget); err != nil {
		return nil, err
	}
	shapes := s.shapeList()
	if len(shapes) > budget.Nodes || len(shapes) >= resolved.MaxOperations {
		return nil, fmt.Errorf("%w: slide shapes", render.ErrLimit)
	}
	budget.Nodes -= len(shapes)
	ops := []layout.Op{layout.FillRect{Rect: layout.Rect{W: renderUnit(w), H: renderUnit(h)}, Color: background}}
	imageCount := 0
	imagePixels, imageBytes := int64(0), int64(0)
	for i, sh := range shapes {
		switch v := sh.(type) {
		case *AutoShape:
			if v == nil {
				return nil, fmt.Errorf("%w: nil shape", render.ErrInvalid)
			}
		case *Picture:
			if v == nil {
				return nil, fmt.Errorf("%w: nil picture", render.ErrInvalid)
			}
		default:
			return nil, fmt.Errorf("%w: shape %d (%T)", render.ErrUnsupported, i, sh)
		}
		x, y := sh.Position()
		sw, shh := sh.Size()
		if sw < 0 || shh < 0 {
			return nil, fmt.Errorf("%w: shape extent", render.ErrInvalid)
		}
		for _, v := range []dml.EMU{x, y, sw, shh} {
			if _, ok := style.FromPx(float64(v) / float64(dml.EMUsPerPixel)); !ok {
				return nil, fmt.Errorf("%w: shape coordinate", render.ErrLimit)
			}
		}
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		var drawn []layout.Op
		switch v := sh.(type) {
		case *AutoShape:
			base := s.renderShapeProps(i)
			drawn, err = renderAutoShape(v, base)
		case *Picture:
			if props := s.renderPictureProps(i); props != nil {
				if e := renderPictureProperties(props); e != nil {
					err = e
					break
				}
			}
			if v.isMedia || len(v.svgData) > 0 || v.svgRelID != "" || v.cropLeft != 0 || v.cropRight != 0 || v.cropTop != 0 || v.cropBottom != 0 {
				err = fmt.Errorf("%w: media/SVG/cropped picture", render.ErrUnsupported)
				break
			}
			var imgData = v.Data()
			if len(imgData) == 0 {
				err = fmt.Errorf("%w: missing picture data", render.ErrInvalid)
				break
			}
			if imageCount >= resolved.MaxImages || int64(len(imgData)) > resolved.MaxImageBytes-imageBytes || imagePixels >= resolved.MaxImagePixels {
				err = fmt.Errorf("%w: slide image budget", render.ErrLimit)
				break
			}
			imageCount++
			imageBytes += int64(len(imgData))
			decodeLimits := resolved
			decodeLimits.MaxImagePixels = resolved.MaxImagePixels - imagePixels
			img, imgErr := core.DecodeImage(ctx, imgData, decodeLimits)
			if imgErr != nil {
				err = imgErr
				break
			}
			imagePixels += int64(img.Bounds().Dx()) * int64(img.Bounds().Dy())

			x, y := v.Position()
			width, height := v.Size()
			drawn = []layout.Op{layout.DrawImage{Rect: layout.Rect{X: renderUnit(x), Y: renderUnit(y), W: renderUnit(width), H: renderUnit(height)}, Image: img}}
		default:
			err = fmt.Errorf("%w: shape %T", render.ErrUnsupported, sh)
		}
		if err != nil {
			return nil, fmt.Errorf("pptx: slide %d shape %d: %w", s.index, i, err)
		}
		ops = append(ops, drawn...)
	}
	return render.Prepare(ctx, w, h, ops, opts.Limits)
}
func renderUnit(v dml.EMU) style.Unit {
	u, _ := style.FromPx(float64(v) / float64(dml.EMUsPerPixel))
	return u
}
func renderTreeBase(c *oxml.CommonSlideData, b *core.SourceBudget) error {
	if c == nil || c.SpTree == nil {
		return fmt.Errorf("%w: shape tree", render.ErrInvalid)
	}
	if len(c.Controls) > 0 || len(c.CustDataLst) > 0 || c.ExtLst != nil {
		return fmt.Errorf("%w: slide controls/extensions", render.ErrUnsupported)
	}
	t := c.SpTree
	if len(t.AltContent) > 0 || len(t.RawXML) > 0 {
		return fmt.Errorf("%w: raw/alternate drawing content", render.ErrUnsupported)
	}
	if t.GrpSpPr != nil {
		g := t.GrpSpPr
		if !renderRootTransform(g.Xfrm) || g.NoFill != nil || g.SolidFill != nil || g.GradFill != nil || g.BlipFill != nil || g.PattFill != nil || g.EffectLst != nil || g.EffectDag != nil || g.Scene3d != nil || g.ExtLst != nil || g.BwMode != "" {
			return fmt.Errorf("%w: root group properties", render.ErrUnsupported)
		}
	}
	if len(t.GraphicFrame) > 0 || len(t.GrpSp) > 0 || len(t.CxnSp) > 0 {
		return fmt.Errorf("%w: group/connector/graphic", render.ErrUnsupported)
	}
	return nil
}
func renderInheritance(c *oxml.CommonSlideData, b *core.SourceBudget) error {
	if c == nil {
		return nil
	}
	if err := renderTreeBase(c, b); err != nil {
		return err
	}
	if len(c.SpTree.Sp) > b.Nodes {
		return fmt.Errorf("%w: inherited shapes", render.ErrLimit)
	}
	b.Nodes -= len(c.SpTree.Sp)
	if len(c.SpTree.Pic) > 0 {
		return fmt.Errorf("%w: inherited pictures", render.ErrUnsupported)
	}
	for _, sp := range c.SpTree.Sp {
		if sp == nil || sp.NvSpPr == nil || sp.NvSpPr.NvPr == nil || sp.NvSpPr.NvPr.Ph == nil {
			return fmt.Errorf("%w: inherited visible shape", render.ErrUnsupported)
		}
		typ := sp.NvSpPr.NvPr.Ph.Type
		if typ != "title" && typ != "body" && typ != "ctrTitle" && typ != "subTitle" {
			return fmt.Errorf("%w: inherited placeholder %s", render.ErrUnsupported, typ)
		}
	}
	return nil
}
func renderAutoShape(v *AutoShape, source *dml.SpPr) ([]layout.Op, error) {
	if v.textFrame != nil {
		return nil, fmt.Errorf("%w: shape text", render.ErrUnsupported)
	}
	p := &v.spPr
	if source != nil {
		if source.Xfrm == nil || source.Xfrm.Off == nil || source.Xfrm.Ext == nil {
			return nil, fmt.Errorf("%w: inherited/missing shape geometry", render.ErrUnsupported)
		}
		if source.EffectLst != nil || source.EffectDag != nil || source.Scene3d != nil || source.Sp3d != nil || v.spPr.EffectLst != nil || v.spPr.EffectDag != nil || v.spPr.Scene3d != nil || v.spPr.Sp3d != nil {
			return nil, fmt.Errorf("%w: shape effect", render.ErrUnsupported)
		}
		copyProps := *source
		if source.Ln != nil {
			line := *source.Ln
			copyProps.Ln = &line
		}
		applyShapeStyle(&copyProps, &v.spPr)
		p = &copyProps
	}
	if p.BwMode != "" || p.CustGeom != nil || p.GradFill != nil || p.BlipFill != nil || p.PattFill != nil || p.GrpFill != nil || p.EffectLst != nil || p.EffectDag != nil || p.Scene3d != nil || p.Sp3d != nil || p.ExtLst != nil {
		return nil, fmt.Errorf("%w: shape fill/effect", render.ErrUnsupported)
	}
	if p.Xfrm != nil && (p.Xfrm.Rot != 0 || p.Xfrm.FlipH || p.Xfrm.FlipV) {
		return nil, fmt.Errorf("%w: shape transformation", render.ErrUnsupported)
	}
	if p.Ln == nil || p.Ln.NoFill == nil {
		return nil, fmt.Errorf("%w: implicit or visible shape stroke", render.ErrUnsupported)
	}
	if p.Ln.SolidFill != nil || p.Ln.GradFill != nil || p.Ln.PattFill != nil {
		return nil, fmt.Errorf("%w: ambiguous shape stroke", render.ErrUnsupported)
	}
	if p.PrstGeom != nil && p.PrstGeom.AvLst != nil && len(p.PrstGeom.AvLst.Gd) > 0 {
		return nil, fmt.Errorf("%w: shape adjustment", render.ErrUnsupported)
	}
	if v.presetGeometry != "rect" && v.presetGeometry != "ellipse" {
		return nil, fmt.Errorf("%w: preset %s", render.ErrUnsupported, v.presetGeometry)
	}
	if p.NoFill != nil && p.SolidFill != nil {
		return nil, fmt.Errorf("%w: ambiguous shape fill", render.ErrInvalid)
	}
	if p.NoFill != nil {
		return nil, nil
	}
	c, err := renderSolid(p.SolidFill)
	if err != nil {
		return nil, err
	}
	x, y := v.Position()
	w, h := v.Size()
	if w < 0 || h < 0 {
		return nil, fmt.Errorf("%w: shape extent", render.ErrInvalid)
	}
	rect := layout.Rect{X: renderUnit(x), Y: renderUnit(y), W: renderUnit(w), H: renderUnit(h)}
	if v.presetGeometry == "rect" {
		return []layout.Op{layout.FillRect{Rect: rect, Color: c}}, nil
	}
	cx, okX := style.FromPx(rect.X.Px() + rect.W.Px()/2)
	cy, okY := style.FromPx(rect.Y.Px() + rect.H.Px()/2)
	if !okX || !okY {
		return nil, fmt.Errorf("%w: ellipse center", render.ErrLimit)
	}
	path := layout.Path{{Op: layout.ArcTo, Center: layout.Point{X: cx, Y: cy}, RadiusX: rect.W / 2, RadiusY: rect.H / 2, SweepAngle: 360}, {Op: layout.ClosePath}}
	return []layout.Op{layout.FillPath{Path: path, Color: c}}, nil
}
func renderSolid(v *dml.SolidFill) (style.RGBA, error) {
	if v == nil || v.SrgbClr == nil || v.ScRgbClr != nil || v.HslClr != nil || v.SysClr != nil || v.SchemeClr != nil || v.PrstClr != nil {
		return style.RGBA{}, fmt.Errorf("%w: explicit RGB fill required", render.ErrUnsupported)
	}
	c := v.SrgbClr
	if c.Tint != nil || c.Shade != nil || c.SatMod != nil || c.LumMod != nil || c.LumOff != nil || c.Comp != nil || c.Inv != nil || c.Gray != nil || c.AlphaOff != nil || c.AlphaMod != nil || c.Hue != nil || c.HueOff != nil || c.HueMod != nil || c.Sat != nil || c.SatOff != nil || c.Lum != nil || c.Red != nil || c.RedOff != nil || c.RedMod != nil || c.Green != nil || c.GreenOff != nil || c.GreenMod != nil || c.Blue != nil || c.BlueOff != nil || c.BlueMod != nil || c.Gamma != nil || c.InvGamma != nil {
		return style.RGBA{}, fmt.Errorf("%w: color transform", render.ErrUnsupported)
	}
	rgb, err := dml.ParseRGB(c.Val)
	if err != nil {
		return style.RGBA{}, fmt.Errorf("%w: RGB color", render.ErrInvalid)
	}
	if c.Alpha != nil {
		return style.RGBA{}, fmt.Errorf("%w: alpha color transform", render.ErrUnsupported)
	}
	alpha := 1.0
	return style.RGBA{R: float64(rgb.R), G: float64(rgb.G), B: float64(rgb.B), A: alpha}, nil
}

func slideRenderXML(el xml.StartElement) error {
	var attrs string
	switch el.Name.Space {
	case nsP:
		switch el.Name.Local {
		case "sld":
			attrs = "show showMasterSp showMasterPhAnim"
		case "cSld":
			attrs = "name"
		case "cNvPr":
			attrs = "id name descr title"
		case "cNvSpPr":
			attrs = "txBox"
		case "cNvPicPr", "cNvGrpSpPr", "nvGrpSpPr", "grpSpPr", "spTree", "nvSpPr", "nvPicPr", "nvPr", "spPr", "blipFill", "pic", "sp", "bg", "bgPr", "clrMapOvr":
		default:
			return fmt.Errorf("%w: XML %s", render.ErrUnsupported, el.Name.Local)
		}
	case nsA:
		switch el.Name.Local {
		case "xfrm":
			attrs = "rot flipH flipV"
		case "off", "chOff":
			attrs = "x y"
		case "ext", "chExt":
			attrs = "cx cy"
		case "prstGeom":
			attrs = "prst"
		case "srgbClr":
			attrs = "val"
		case "picLocks":
			attrs = "noChangeAspect"
		case "blip":
			attrs = "cstate embed"
		case "fillRect":
			attrs = "l t r b"
		case "ln":
			attrs = "w cap cmpd algn"
		case "avLst", "noFill", "solidFill", "stretch", "masterClrMapping":
		default:
			return fmt.Errorf("%w: XML %s", render.ErrUnsupported, el.Name.Local)
		}
	default:
		return fmt.Errorf("%w: XML namespace %s", render.ErrUnsupported, el.Name.Space)
	}
	for _, a := range el.Attr {
		if el.Name.Local == "xfrm" && (a.Name.Local == "rot" || a.Name.Local == "flipH" || a.Name.Local == "flipV") && a.Value != "0" && a.Value != "false" {
			return fmt.Errorf("%w: picture/shape transform", render.ErrUnsupported)
		}
		if el.Name.Local == "fillRect" && a.Value != "0" {
			return fmt.Errorf("%w: picture fill rectangle", render.ErrUnsupported)
		}
		if a.Name.Space == "xmlns" || (a.Name.Space == "" && a.Name.Local == "xmlns") {
			continue
		}
		if a.Name.Space != "" && (el.Name.Local != "blip" || a.Name.Space != nsR || a.Name.Local != "embed") {
			return fmt.Errorf("%w: XML attribute namespace", render.ErrUnsupported)
		}
		if !strings.Contains(" "+attrs+" ", " "+a.Name.Local+" ") {
			return fmt.Errorf("%w: XML %s/@%s", render.ErrUnsupported, el.Name.Local, a.Name.Local)
		}
	}
	return nil
}

func renderRootTransform(x *dml.GrpXfrm) bool {
	if x == nil {
		return true
	}
	if x.Rot != 0 || x.FlipH || x.FlipV {
		return false
	}
	for _, v := range []*dml.OffXML{x.Off, x.ChOff} {
		if v != nil && (v.X != 0 || v.Y != 0) {
			return false
		}
	}
	for _, v := range []*dml.ExtXML{x.Ext, x.ChExt} {
		if v != nil && (v.Cx != 0 || v.Cy != 0) {
			return false
		}
	}
	return true
}

func (s *Slide) renderShapeProps(index int) *dml.SpPr {
	if index >= len(s.shapeRefs) {
		return nil
	}
	ref := s.shapeRefs[index]
	t := s.sxModel.CSld.SpTree
	if ref.Kind == oxml.ChildSp && ref.Index >= 0 && ref.Index < len(t.Sp) {
		return t.Sp[ref.Index].SpPr
	}
	return nil
}
func (s *Slide) renderPictureProps(index int) *dml.SpPr {
	if index >= len(s.shapeRefs) {
		return nil
	}
	ref := s.shapeRefs[index]
	t := s.sxModel.CSld.SpTree
	if ref.Kind == oxml.ChildPic && ref.Index >= 0 && ref.Index < len(t.Pic) {
		return t.Pic[ref.Index].SpPr
	}
	return nil
}
func renderPictureProperties(p *dml.SpPr) error {
	if p.BwMode != "" || p.CustGeom != nil || p.SolidFill != nil || p.GradFill != nil || p.BlipFill != nil || p.PattFill != nil || p.GrpFill != nil || p.EffectLst != nil || p.EffectDag != nil || p.Scene3d != nil || p.Sp3d != nil || p.ExtLst != nil {
		return fmt.Errorf("%w: picture shape properties", render.ErrUnsupported)
	}
	if p.Xfrm != nil && (p.Xfrm.Rot != 0 || p.Xfrm.FlipH || p.Xfrm.FlipV) {
		return fmt.Errorf("%w: picture transformation", render.ErrUnsupported)
	}
	if p.Ln != nil && (p.Ln.NoFill == nil || p.Ln.SolidFill != nil || p.Ln.GradFill != nil || p.Ln.PattFill != nil) {
		return fmt.Errorf("%w: picture outline", render.ErrUnsupported)
	}
	if p.PrstGeom != nil && (p.PrstGeom.Prst != "rect" || (p.PrstGeom.AvLst != nil && len(p.PrstGeom.AvLst.Gd) > 0)) {
		return fmt.Errorf("%w: picture geometry", render.ErrUnsupported)
	}
	return nil
}

func slideRenderProfile(node core.XMLNode) error {
	if node.Text {
		return fmt.Errorf("%w: unexpected XML text", render.ErrUnsupported)
	}
	if len(node.Path) == 1 && (node.Name.Space != nsP || node.Name.Local != "sld") {
		return fmt.Errorf("%w: slide XML root", render.ErrInvalid)
	}
	if len(node.Path) > 1 {
		parents := renderXMLParents[renderXMLKey(node.Name)]
		parent := renderXMLKey(node.Path[len(node.Path)-2])
		if !strings.Contains(" "+parents+" ", " "+parent+" ") {
			return fmt.Errorf("%w: XML placement %s", render.ErrUnsupported, node.Name.Local)
		}
	}
	if node.Occurrence > 1 && (node.Name.Space != nsP || (node.Name.Local != "sp" && node.Name.Local != "pic")) {
		return fmt.Errorf("%w: repeated XML %s", render.ErrInvalid, node.Name.Local)
	}
	return slideRenderXML(node.StartElement)
}
func inheritedRenderProfile(node core.XMLNode) error {
	if len(node.Path) == 1 {
		if node.Name.Space != nsP || (node.Name.Local != "sldMaster" && node.Name.Local != "sldLayout") {
			return fmt.Errorf("%w: inherited XML root", render.ErrUnsupported)
		}
		for _, a := range node.Attr {
			if a.Name.Space == "xmlns" || (a.Name.Space == "" && a.Name.Local == "xmlns") {
				continue
			}
			if a.Name.Space != "" || !strings.Contains(" preserve showMasterSp showMasterPhAnim type userDrawn matchingName ", " "+a.Name.Local+" ") {
				return fmt.Errorf("%w: inherited root attribute", render.ErrUnsupported)
			}
		}
		return nil
	}
	rootChild := node.Path[1]
	if rootChild.Space == nsP && (rootChild.Local == "txStyles" || rootChild.Local == "sldLayoutIdLst" || rootChild.Local == "clrMap" || rootChild.Local == "clrMapOvr") {
		if len(node.Path) == 2 && node.Occurrence > 1 {
			return fmt.Errorf("%w: repeated inherited metadata", render.ErrInvalid)
		}
		// These defined metadata subtrees cannot affect the explicitly styled,
		// non-text profile; none of their relationships are dereferenced.
		return nil
	}
	for _, ancestor := range node.Path {
		if ancestor.Space == nsP && ancestor.Local == "sp" {
			// renderInheritance permits only title/body placeholders, which are
			// definitions and are not painted independently on a blank slide.
			if node.Text {
				return nil
			}
			if node.Name.Local == "nvSpPr" || node.Name.Local == "nvPr" || node.Name.Local == "ph" {
				if node.Occurrence > 1 {
					return fmt.Errorf("%w: repeated placeholder property", render.ErrInvalid)
				}
				if node.Name.Space != nsP {
					return fmt.Errorf("%w: placeholder namespace", render.ErrUnsupported)
				}
			}
			return nil
		}
	}
	return slideRenderProfile(node)
}

func renderXMLKey(n xml.Name) string {
	if n.Space == nsP {
		return "p:" + n.Local
	}
	if n.Space == nsA {
		return "a:" + n.Local
	}
	return "?" + n.Local
}

var renderXMLParents = map[string]string{
	"p:cSld": "p:sld p:sldMaster p:sldLayout", "p:spTree": "p:cSld", "p:bg": "p:cSld", "p:bgPr": "p:bg",
	"p:clrMapOvr": "p:sld p:sldLayout", "a:masterClrMapping": "p:clrMapOvr",
	"p:nvGrpSpPr": "p:spTree", "p:grpSpPr": "p:spTree", "p:sp": "p:spTree", "p:pic": "p:spTree",
	"p:nvSpPr": "p:sp", "p:nvPicPr": "p:pic", "p:cNvPr": "p:nvSpPr p:nvPicPr p:nvGrpSpPr",
	"p:cNvSpPr": "p:nvSpPr", "p:cNvPicPr": "p:nvPicPr", "p:cNvGrpSpPr": "p:nvGrpSpPr",
	"p:nvPr": "p:nvSpPr p:nvPicPr p:nvGrpSpPr", "p:spPr": "p:sp p:pic", "p:blipFill": "p:pic",
	"a:xfrm": "p:spPr p:grpSpPr", "a:off": "a:xfrm", "a:ext": "a:xfrm", "a:chOff": "a:xfrm", "a:chExt": "a:xfrm",
	"a:prstGeom": "p:spPr", "a:avLst": "a:prstGeom", "a:noFill": "p:spPr p:bgPr a:ln",
	"a:solidFill": "p:spPr p:bgPr a:ln", "a:srgbClr": "a:solidFill", "a:ln": "p:spPr",
	"a:picLocks": "p:cNvPicPr", "a:blip": "p:blipFill", "a:stretch": "p:blipFill", "a:fillRect": "a:stretch",
}
