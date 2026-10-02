package pptx

import (
	"context"
	"encoding/xml"
	"fmt"
	"strings"

	"github.com/mgilbir/forme/layout"
	"github.com/mgilbir/forme/style"
	"github.com/mgilbir/spine/common/dml"
	xmlb "github.com/mgilbir/spine/common/xml"
	core "github.com/mgilbir/spine/internal/render"
	"github.com/mgilbir/spine/pptx/internal/oxml"
	"github.com/mgilbir/spine/render"
)

// PrepareRender prepares this static slide, including unsaved edits, for PNG or
// SVG output. The first supported profile is explicit RGB backgrounds, filled
// rectangles/ellipses with no stroke, uncropped embedded PNG/JPEG pictures, and
// explicitly styled plain horizontal ASCII text in rectangles with supplied fonts.
// Inherited visible master/layout objects, theme fills, transformations,
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
	textLayout, err := core.NewTextLayout(resolved)
	if err != nil {
		return nil, err
	}
	fonts := newSlideRenderFonts(opts)
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
			e = budget.CheckReader(ctx, stream, (&renderProfile{}).slide)
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
	if len(model.AlternateContent) > 0 || model.Timing != nil || model.Transition != nil {
		return nil, fmt.Errorf("%w: slide animation/alternate content", render.ErrUnsupported)
	}
	if err = renderModelExtensions(model.ExtLst, "p:sld"); err != nil {
		return nil, err
	}
	layers := []*oxml.CommonSlideData{}
	if s.layout != nil {
		if s.layout.master != nil && s.layout.master.masterXML != nil {
			m := s.layout.master.masterXML
			// Header/footer flags only select footer placeholders, which this
			// profile rejects on the master and the slide.
			if len(m.AlternateContent) > 0 {
				return nil, fmt.Errorf("%w: master alternate content", render.ErrUnsupported)
			}
			if err = renderModelExtensions(m.ExtLst, "p:sldMaster"); err != nil {
				return nil, err
			}
			layers = append(layers, m.CSld)
		}
		if s.layout.layoutXML != nil {
			m := s.layout.layoutXML
			if len(m.AlternateContent) > 0 {
				return nil, fmt.Errorf("%w: layout alternate content", render.ErrUnsupported)
			}
			if err = renderModelExtensions(m.ExtLst, "p:sldLayout"); err != nil {
				return nil, err
			}
			layers = append(layers, m.CSld)
		}
	}
	if s.layout != nil {
		if s.layout.layoutXML != nil && len(s.layout.layoutXML.SourceXML) > 0 {
			if err = budget.CheckXML(ctx, s.layout.layoutXML.SourceXML, (&renderProfile{}).inherited); err != nil {
				return nil, fmt.Errorf("pptx: layout: %w", err)
			}
		}
		if s.layout.master != nil && s.layout.master.masterXML != nil && len(s.layout.master.masterXML.SourceXML) > 0 {
			if err = budget.CheckXML(ctx, s.layout.master.masterXML.SourceXML, (&renderProfile{}).inherited); err != nil {
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
		case *TextBox:
			if v == nil {
				return nil, fmt.Errorf("%w: nil text box", render.ErrInvalid)
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
		case *TextBox:
			box := &AutoShape{BaseShape: v.BaseShape, textFrame: v.textFrame, spPr: v.spPr, presetGeometry: "rect"}
			drawn, err = renderAutoShape(box, s.renderShapeProps(i))
			if err == nil && box.textFrame != nil {
				var textOps []layout.Op
				textOps, err = s.renderShapeText(ctx, i, box, textLayout, fonts)
				drawn = append(drawn, textOps...)
			}
		case *AutoShape:
			base := s.renderShapeProps(i)
			drawn, err = renderAutoShape(v, base)
			if err == nil && v.textFrame != nil {
				var textOps []layout.Op
				textOps, err = s.renderShapeText(ctx, i, v, textLayout, fonts)
				drawn = append(drawn, textOps...)
			}
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
	if len(c.Controls) > 0 || len(c.CustDataLst) > 0 {
		return fmt.Errorf("%w: slide controls/customer data", render.ErrUnsupported)
	}
	if err := renderModelExtensions(c.ExtLst, "p:cSld"); err != nil {
		return err
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
		case "hf":
			attrs = "sldNum hdr ftr dt"
		case "cNvPicPr", "cNvGrpSpPr", "nvGrpSpPr", "grpSpPr", "spTree", "nvSpPr", "nvPicPr", "nvPr", "spPr", "blipFill", "pic", "sp", "bg", "bgPr", "clrMapOvr", "txBody":
		default:
			return fmt.Errorf("%w: XML %s", render.ErrUnsupported, el.Name.Local)
		}
	case nsA:
		switch el.Name.Local {
		case "bodyPr":
			// rtlCol orders columns; the single-column profile has one.
			attrs = "wrap anchor lIns tIns rIns bIns rtlCol"
		case "pPr":
			attrs = "algn"
		case "rPr":
			// Language, proofing, smart-tag and bookmark attributes do not
			// change the painting of horizontal ASCII text.
			attrs = "sz b i u strike lang altLang dirty err noProof smtClean smtId bmk"
		case "latin":
			attrs = "typeface"
		case "spcPct", "spcPts":
			attrs = "val"
		case "p", "r", "t", "lstStyle", "noAutofit", "buNone", "lnSpc", "spcBef", "spcAft":
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

// renderProfile checks original slide, layout and master XML. It skips the
// subtree of an extension whose URI renderExtensions lists for its owner; the
// streaming checker reports no end tags, so a skipped subtree ends at the next
// node that is not below it.
type renderProfile struct {
	skipDepth int
}

func (r *renderProfile) skipped(node core.XMLNode) bool {
	if r.skipDepth == 0 {
		return false
	}
	if len(node.Path) > r.skipDepth || (node.Text && len(node.Path) == r.skipDepth) {
		return true
	}
	r.skipDepth = 0
	return false
}

// extension checks an extension list or extension element and reports whether
// it consumed the node.
func (r *renderProfile) extension(node core.XMLNode) (bool, error) {
	if node.Text || len(node.Path) < 2 || node.Name.Space != nsP && node.Name.Space != nsA {
		return false, nil
	}
	parent := node.Path[len(node.Path)-2]
	switch {
	case node.Name.Local == "extLst":
		owner, ok := renderExtensions[renderXMLKey(parent)]
		if !ok || owner.list != node.Name.Space {
			return true, fmt.Errorf("%w: XML placement %s", render.ErrUnsupported, node.Name.Local)
		}
		if node.Occurrence > 1 {
			return true, fmt.Errorf("%w: repeated XML %s", render.ErrInvalid, node.Name.Local)
		}
		return true, renderExtensionAttrs(node.StartElement, "")
	case node.Name.Local == "ext" && parent.Local == "extLst" && parent.Space == node.Name.Space:
		if len(node.Path) < 3 {
			return true, fmt.Errorf("%w: extension owner", render.ErrInvalid)
		}
		owner := renderExtensions[renderXMLKey(node.Path[len(node.Path)-3])]
		var uri string
		for _, a := range node.Attr {
			if a.Name.Space == "" && a.Name.Local == "uri" {
				uri = a.Value
			}
		}
		if !renderExtensionAllowed(owner, uri) {
			return true, fmt.Errorf("%w: extension %s", render.ErrUnsupported, uri)
		}
		if err := renderExtensionAttrs(node.StartElement, "uri"); err != nil {
			return true, err
		}
		r.skipDepth = len(node.Path)
		return true, nil
	}
	return false, nil
}

func renderExtensionAttrs(el xml.StartElement, allowed string) error {
	for _, a := range el.Attr {
		if a.Name.Space == "xmlns" || (a.Name.Space == "" && a.Name.Local == "xmlns") {
			continue
		}
		if a.Name.Space != "" || a.Name.Local != allowed {
			return fmt.Errorf("%w: XML %s/@%s", render.ErrUnsupported, el.Name.Local, a.Name.Local)
		}
	}
	return nil
}

// renderExtensionOwner names the namespace of an owner's extension list and the
// extension URIs it may carry.
type renderExtensionOwner struct {
	list string
	uris []string
}

// renderExtensions lists extensions that hold only identity, editor-guide,
// accessibility or image-storage metadata. None can change painted output;
// every other extension fails explicitly.
var renderExtensions = map[string]renderExtensionOwner{
	"p:cNvPr":     {list: nsA, uris: []string{xmlb.ExtURICreationId, xmlb.ExtURIDecorative}},
	"a:blip":      {list: nsA, uris: []string{xmlb.ExtURIUseLocalDpi}},
	"p:cSld":      {list: nsP, uris: []string{xmlb.ExtURIPMLCreationId}},
	"p:sld":       {list: nsP, uris: []string{xmlb.ExtURISldGuideLst}},
	"p:sldLayout": {list: nsP, uris: []string{xmlb.ExtURISldGuideLstLayout}},
	"p:sldMaster": {list: nsP, uris: []string{xmlb.ExtURISldGuideLstMaster}},
}

func renderExtensionAllowed(owner renderExtensionOwner, uri string) bool {
	for _, allowed := range owner.uris {
		if strings.EqualFold(uri, allowed) {
			return true
		}
	}
	return false
}

// renderModelExtensions applies the source extension profile to a parsed list,
// which also covers lists created or edited through the API.
func renderModelExtensions(l *oxml.ExtensionList, owner string) error {
	if l == nil {
		return nil
	}
	if l.Mod != nil {
		return fmt.Errorf("%w: extension list modification", render.ErrUnsupported)
	}
	for _, e := range l.Ext {
		if !renderExtensionAllowed(renderExtensions[owner], e.URI) {
			return fmt.Errorf("%w: extension %s", render.ErrUnsupported, e.URI)
		}
	}
	return nil
}

func (r *renderProfile) slide(node core.XMLNode) error {
	if r.skipped(node) {
		return nil
	}
	if done, err := r.extension(node); done {
		return err
	}
	return slideRenderNode(node)
}

func slideRenderNode(node core.XMLNode) error {
	if node.Text {
		if len(node.Path) > 0 && node.Path[len(node.Path)-1] == (xml.Name{Space: nsA, Local: "t"}) {
			return nil
		}
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
	repeated := (node.Name.Space == nsP && (node.Name.Local == "sp" || node.Name.Local == "pic")) || (node.Name.Space == nsA && (node.Name.Local == "p" || node.Name.Local == "r"))
	if node.Occurrence > 1 && !repeated {
		return fmt.Errorf("%w: repeated XML %s", render.ErrInvalid, node.Name.Local)
	}
	return slideRenderXML(node.StartElement)
}
func (r *renderProfile) inherited(node core.XMLNode) error {
	if r.skipped(node) {
		return nil
	}
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
			if node.Name.Space == nsP && node.Name.Local == "sp" {
				return r.slide(node)
			}
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
	return r.slide(node)
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
	"p:txBody": "p:sp", "a:bodyPr": "p:txBody", "a:lstStyle": "p:txBody", "a:noAutofit": "a:bodyPr",
	"a:p": "p:txBody", "a:pPr": "a:p", "a:r": "a:p", "a:rPr": "a:r", "a:t": "a:r", "a:latin": "a:rPr",
	"a:buNone": "a:pPr", "a:lnSpc": "a:pPr", "a:spcBef": "a:pPr", "a:spcAft": "a:pPr", "a:spcPct": "a:lnSpc a:spcBef a:spcAft", "a:spcPts": "a:lnSpc a:spcBef a:spcAft",
	"p:cSld": "p:sld p:sldMaster p:sldLayout", "p:spTree": "p:cSld", "p:bg": "p:cSld", "p:bgPr": "p:bg",
	"p:clrMapOvr": "p:sld p:sldLayout", "a:masterClrMapping": "p:clrMapOvr", "p:hf": "p:sldMaster p:sldLayout",
	"p:nvGrpSpPr": "p:spTree", "p:grpSpPr": "p:spTree", "p:sp": "p:spTree", "p:pic": "p:spTree",
	"p:nvSpPr": "p:sp", "p:nvPicPr": "p:pic", "p:cNvPr": "p:nvSpPr p:nvPicPr p:nvGrpSpPr",
	"p:cNvSpPr": "p:nvSpPr", "p:cNvPicPr": "p:nvPicPr", "p:cNvGrpSpPr": "p:nvGrpSpPr",
	"p:nvPr": "p:nvSpPr p:nvPicPr p:nvGrpSpPr", "p:spPr": "p:sp p:pic", "p:blipFill": "p:pic",
	"a:xfrm": "p:spPr p:grpSpPr", "a:off": "a:xfrm", "a:ext": "a:xfrm", "a:chOff": "a:xfrm", "a:chExt": "a:xfrm",
	"a:prstGeom": "p:spPr", "a:avLst": "a:prstGeom", "a:noFill": "p:spPr p:bgPr a:ln",
	"a:solidFill": "p:spPr p:bgPr a:ln a:rPr", "a:srgbClr": "a:solidFill", "a:ln": "p:spPr",
	"a:picLocks": "p:cNvPicPr", "a:blip": "p:blipFill", "a:stretch": "p:blipFill", "a:fillRect": "a:stretch",
}
