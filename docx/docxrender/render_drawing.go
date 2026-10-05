package docxrender

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/mgilbir/spine/common/dml"
	xmlb "github.com/mgilbir/spine/common/xml"
	"github.com/mgilbir/spine/render"
)

// Pictures and drawings.
//
// A w:drawing run child holds one wp:inline or wp:anchor with an a:graphic. A
// picture (pic:pic) is drawn; every other kind of graphic (charts, SmartArt,
// shapes and text boxes, groups) is left out, keeping an inline drawing's
// space. This file reads the drawing into a wordDrawing; render_picture.go
// turns the picture into pixels, render_float.go places it.

const (
	nsWP  = "http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing"
	nsPic = "http://schemas.openxmlformats.org/drawingml/2006/picture"
	// wordRelImage is the relationship type of an embedded image.
	wordRelImage = "http://schemas.openxmlformats.org/officeDocument/2006/relationships/image"
	// wordEMUPerPixel is the EMU in a CSS pixel.
	wordEMUPerPixel = 9525.0
)

// wordDrawing is one wp:inline or wp:anchor.
type wordDrawing struct {
	// w and h are the picture's frame, wp:extent, in pixels.
	w, h float64
	// ext is wp:effectExtent: the room the drawing takes around its frame on
	// the left, top, right and bottom.
	ext [4]float64
	// anchor is nil for an inline drawing.
	anchor *wordAnchor
	// pic is nil for a graphic that is not a picture, which is left out.
	pic *wordPicSpec
}

// wordAnchor is the placement of a wp:anchor.
type wordAnchor struct {
	simplePos        bool
	simpleX, simpleY float64
	hFrom, hAlign    string
	hOff             float64
	vFrom, vAlign    string
	vOff             float64
	// wrap is none, square, tight, through or topBottom; wrapText is
	// bothSides, left, right or largest.
	wrap, wrapText string
	// dist is the wrap distance: top, bottom, left, right.
	dist   [4]float64
	behind bool
	// z is relativeHeight, the stacking order.
	z int64
}

// wordPicSpec is a pic:pic.
type wordPicSpec struct {
	embed, link string
	// effects are the blip's colour effects and effectsKey their serialized
	// source, which keys the pixels they produced.
	effects    []*dml.BlipEffect
	effectsKey string
	// colorTransforms is set when an effect names a colour with transforms.
	colorTransforms bool
	// src is a:srcRect and fill is a:stretch/a:fillRect, as fractions
	// (left, top, right, bottom). Either may be negative.
	src, fill [4]float64
	flipH     bool
	flipV     bool
	// rot is the rotation in degrees, clockwise.
	rot     float64
	outline *wordOutline
}

// wordOutline is a picture's line.
type wordOutline struct {
	width float64
	color wordRGB
}

// rawAttr returns the attribute named local in the namespace space.
func (n *wordNode) rawAttr(space, local string) (string, bool) {
	for _, a := range n.attrs {
		if a.space == space && a.name == local {
			return a.value, true
		}
	}
	return "", false
}

// wordEMU parses an EMU count into pixels, clamped to the profile's length
// bound.
func wordEMU(s, what string) (float64, error) {
	v, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: %s", render.ErrInvalid, what)
	}
	return math.Max(-wordMaxLength, math.Min(wordMaxLength, float64(v)/wordEMUPerPixel)), nil
}

func wordEMUAttr(n *wordNode, name string, what string) (float64, bool, error) {
	v, ok := n.rawAttr("", name)
	if !ok {
		return 0, false, nil
	}
	px, err := wordEMU(v, what)
	return px, true, err
}

// wordTrue reads an xsd:boolean attribute.
func wordTrue(n *wordNode, name string) (bool, error) {
	v, ok := n.rawAttr("", name)
	if !ok {
		return false, nil
	}
	switch v {
	case "1", "true":
		return true, nil
	case "0", "false":
		return false, nil
	}
	return false, fmt.Errorf("%w: %s", render.ErrInvalid, name)
}

// wordGraphicKinds names the graphic kinds that are not pictures, by the URI
// of their a:graphicData.
var wordGraphicKinds = map[string]string{
	"http://schemas.openxmlformats.org/drawingml/2006/chart":             "charts",
	"http://schemas.openxmlformats.org/drawingml/2006/diagram":           "SmartArt diagrams",
	"http://schemas.microsoft.com/office/word/2010/wordprocessingShape":  "shapes and text boxes",
	"http://schemas.microsoft.com/office/word/2010/wordprocessingGroup":  "drawing groups",
	"http://schemas.microsoft.com/office/word/2010/wordprocessingCanvas": "drawing canvases",
	"http://schemas.microsoft.com/office/word/2010/wordprocessingInk":    "ink",
	"http://schemas.openxmlformats.org/drawingml/2006/lockedCanvas":      "drawing canvases",
}

var (
	wordHFrom = map[string]bool{"margin": true, "page": true, "column": true, "character": true, "leftMargin": true, "rightMargin": true, "insideMargin": true, "outsideMargin": true}
	wordVFrom = map[string]bool{"margin": true, "page": true, "paragraph": true, "line": true, "topMargin": true, "bottomMargin": true, "insideMargin": true, "outsideMargin": true}
	wordHAln  = map[string]bool{"left": true, "right": true, "center": true, "inside": true, "outside": true}
	wordVAln  = map[string]bool{"top": true, "bottom": true, "center": true, "inside": true, "outside": true}
)

// parseDrawing reads a w:drawing. It returns nil when the drawing draws
// nothing: hidden, empty, or left out (and reported).
func (r *wordRenderer) parseDrawing(n *wordNode) (*wordDrawing, error) {
	var host *wordNode
	for _, c := range n.children {
		if c.space == nsWP && (c.name == "inline" || c.name == "anchor") {
			host = c
			break
		}
	}
	if host == nil {
		return nil, r.leaveOut("drawing without a position")
	}
	d := &wordDrawing{}
	var graphic *wordNode
	var err error
	hasExtent := false
	if host.name == "anchor" {
		d.anchor = &wordAnchor{wrap: "none", hFrom: "column", vFrom: "paragraph"}
		if err = r.parseAnchorAttrs(host, d.anchor); err != nil {
			return nil, err
		}
	}
	for _, c := range host.children {
		if err = r.ctx.Err(); err != nil {
			return nil, err
		}
		switch {
		case c.space == nsWP:
			switch c.name {
			case "extent":
				hasExtent = true
				if d.w, d.h, err = wordExtent(c); err != nil {
					return nil, err
				}
			case "effectExtent":
				for i, name := range [4]string{"l", "t", "r", "b"} {
					v, _, e := wordEMUAttr(c, name, "wp:effectExtent")
					if e != nil {
						return nil, e
					}
					d.ext[i] = v
				}
			case "docPr":
				if hidden, e := wordTrue(c, "hidden"); e != nil {
					return nil, e
				} else if hidden {
					return nil, nil
				}
			case "simplePos", "positionH", "positionV", "wrapNone", "wrapSquare", "wrapTight", "wrapThrough", "wrapTopAndBottom":
				if d.anchor == nil {
					return nil, fmt.Errorf("%w: wp:%s in an inline drawing", render.ErrInvalid, c.name)
				}
				if err = r.parseAnchorChild(c, d.anchor); err != nil {
					return nil, err
				}
			}
		case c.space == nsA && c.name == "graphic":
			graphic = c
		}
	}
	if !hasExtent || graphic == nil {
		return nil, fmt.Errorf("%w: drawing without extent or graphic", render.ErrInvalid)
	}
	data := graphic.childIn(nsA, "graphicData")
	uri := ""
	if data != nil {
		uri, _ = data.rawAttr("", "uri")
	}
	if uri != nsPic {
		what, known := wordGraphicKinds[uri]
		if !known {
			what = "drawings of an unknown kind"
		}
		if err = r.leaveOut(what); err != nil {
			return nil, err
		}
		if d.anchor != nil {
			return nil, nil
		}
		// An inline drawing keeps its space so the line is as tall as it
		// would be.
		return d, nil
	}
	pic := data.childIn(nsPic, "pic")
	if pic == nil {
		return nil, fmt.Errorf("%w: picture without pic:pic", render.ErrInvalid)
	}
	if d.pic, err = r.parsePic(pic); err != nil {
		return nil, err
	}
	return d, nil
}

// wordExtent reads wp:extent.
func wordExtent(c *wordNode) (w, h float64, err error) {
	cx, ok1 := c.rawAttr("", "cx")
	cy, ok2 := c.rawAttr("", "cy")
	if !ok1 || !ok2 {
		return 0, 0, fmt.Errorf("%w: wp:extent", render.ErrInvalid)
	}
	for i, s := range [2]string{cx, cy} {
		v, e := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
		if e != nil || v < 0 {
			return 0, 0, fmt.Errorf("%w: wp:extent", render.ErrInvalid)
		}
		px := float64(v) / wordEMUPerPixel
		if px > wordMaxLength {
			return 0, 0, fmt.Errorf("%w: picture size", render.ErrLimit)
		}
		if i == 0 {
			w = px
		} else {
			h = px
		}
	}
	return w, h, nil
}

func (r *wordRenderer) parseAnchorAttrs(host *wordNode, a *wordAnchor) error {
	var err error
	for i, name := range [4]string{"distT", "distB", "distL", "distR"} {
		v, ok := host.rawAttr("", name)
		if !ok {
			continue
		}
		n, e := strconv.ParseUint(strings.TrimSpace(v), 10, 32)
		if e != nil {
			return fmt.Errorf("%w: wp:anchor %s", render.ErrInvalid, name)
		}
		a.dist[i] = float64(n) / wordEMUPerPixel
	}
	if a.behind, err = wordTrue(host, "behindDoc"); err != nil {
		return err
	}
	if a.simplePos, err = wordTrue(host, "simplePos"); err != nil {
		return err
	}
	if v, ok := host.rawAttr("", "relativeHeight"); ok {
		n, e := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		if e != nil {
			return fmt.Errorf("%w: wp:anchor relativeHeight", render.ErrInvalid)
		}
		a.z = n
	}
	return nil
}

func (r *wordRenderer) parseAnchorChild(c *wordNode, a *wordAnchor) error {
	switch c.name {
	case "simplePos":
		var err error
		if a.simpleX, _, err = wordEMUAttr(c, "x", "wp:simplePos"); err != nil {
			return err
		}
		a.simpleY, _, err = wordEMUAttr(c, "y", "wp:simplePos")
		return err
	case "positionH", "positionV":
		horizontal := c.name == "positionH"
		from, _ := c.rawAttr("", "relativeFrom")
		froms, alns := wordVFrom, wordVAln
		if horizontal {
			froms, alns = wordHFrom, wordHAln
		}
		if !froms[from] {
			return fmt.Errorf("%w: wp:%s relativeFrom", render.ErrInvalid, c.name)
		}
		align, off := "", 0.0
		for _, k := range c.children {
			if k.space != nsWP {
				continue
			}
			switch k.name {
			case "align":
				align = strings.TrimSpace(k.text)
				if !alns[align] {
					return fmt.Errorf("%w: wp:align", render.ErrInvalid)
				}
			case "posOffset":
				v, err := wordEMU(k.text, "wp:posOffset")
				if err != nil {
					return err
				}
				off = v
			}
		}
		if horizontal {
			a.hFrom, a.hAlign, a.hOff = from, align, off
		} else {
			a.vFrom, a.vAlign, a.vOff = from, align, off
		}
	case "wrapNone":
		a.wrap = "none"
	case "wrapTopAndBottom":
		a.wrap = "topBottom"
	default:
		a.wrap = map[string]string{"wrapSquare": "square", "wrapTight": "tight", "wrapThrough": "through"}[c.name]
		if a.wrapText = c.attrOrEmptyNS("wrapText"); a.wrapText == "" {
			a.wrapText = "bothSides"
		}
		switch a.wrapText {
		case "bothSides", "left", "right", "largest":
		default:
			return fmt.Errorf("%w: wrapText", render.ErrInvalid)
		}
	}
	return nil
}

// attrOrEmptyNS is the unqualified attribute's value, or "".
func (n *wordNode) attrOrEmptyNS(local string) string {
	v, _ := n.rawAttr("", local)
	return v
}

// parsePic reads a pic:pic.
func (r *wordRenderer) parsePic(pic *wordNode) (*wordPicSpec, error) {
	spec := &wordPicSpec{}
	fill := pic.childIn(nsPic, "blipFill")
	if fill == nil {
		return nil, fmt.Errorf("%w: picture without a fill", render.ErrInvalid)
	}
	blip := fill.childIn(nsA, "blip")
	if blip == nil {
		return nil, fmt.Errorf("%w: picture without a blip", render.ErrInvalid)
	}
	spec.embed, _ = blip.rawAttr(nsRelat, "embed")
	spec.link, _ = blip.rawAttr(nsRelat, "link")
	var err error
	if err = r.parseBlip(blip, spec); err != nil {
		return nil, err
	}
	for _, c := range fill.children {
		if c.space != nsA {
			continue
		}
		switch c.name {
		case "srcRect":
			if spec.src, err = wordRelRect(c); err != nil {
				return nil, err
			}
		case "stretch":
			if f := c.childIn(nsA, "fillRect"); f != nil {
				if spec.fill, err = wordRelRect(f); err != nil {
					return nil, err
				}
			}
		case "tile":
			if err = r.approximate("tiled picture fill drawn stretched"); err != nil {
				return nil, err
			}
		}
	}
	sp := pic.childIn(nsPic, "spPr")
	if sp != nil {
		if err = r.parseShapeProps(sp, spec); err != nil {
			return nil, err
		}
	}
	return spec, nil
}

// wordRelRect reads a:srcRect or a:fillRect as fractions.
func wordRelRect(n *wordNode) ([4]float64, error) {
	var out [4]float64
	for i, name := range [4]string{"l", "t", "r", "b"} {
		v, ok := n.rawAttr("", name)
		if !ok {
			continue
		}
		f, ok := wordPercent(v)
		if !ok || math.Abs(f) > 1000 {
			return out, fmt.Errorf("%w: a:%s %s", render.ErrInvalid, n.name, name)
		}
		out[i] = f
	}
	return out, nil
}

// wordPercent reads an ST_Percentage as a fraction: thousandths of a percent
// ("50000") or a percent string ("50%").
func wordPercent(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if num, ok := strings.CutSuffix(s, "%"); ok {
		f, err := strconv.ParseFloat(num, 64)
		return f / 100, err == nil && !math.IsNaN(f) && !math.IsInf(f, 0)
	}
	n, err := strconv.ParseInt(s, 10, 64)
	return float64(n) / 100000, err == nil
}

var wordColorElements = map[string]bool{"srgbClr": true, "schemeClr": true, "sysClr": true, "prstClr": true, "hslClr": true, "scrgbClr": true}

// parseBlip reads a:blip's colour effects and extensions.
func (r *wordRenderer) parseBlip(blip *wordNode, spec *wordPicSpec) error {
	var effectKids []*wordNode
	for _, c := range blip.children {
		if c.space == nsA && c.name == "extLst" {
			for _, e := range c.children {
				uri, _ := e.rawAttr("", "uri")
				switch uri {
				case "{28A0092B-C50C-407E-A947-70E740481C1C}", "{96DAC541-7B7A-43D3-8B79-37D633B846F1}":
					// Local DPI use and the SVG variant, whose PNG fallback
					// is what the embed refers to.
				default:
					if err := r.approximate("picture artistic effects and extensions not applied"); err != nil {
						return err
					}
				}
			}
			continue
		}
		effectKids = append(effectKids, c)
	}
	if len(effectKids) == 0 {
		return nil
	}
	// Re-read the blip through the dml model: it keeps the effects as a typed,
	// ordered list.
	shell := &wordNode{space: blip.space, name: blip.name, children: effectKids}
	text, err := wordSerialize(shell)
	if err != nil {
		return err
	}
	var b dml.BlipXML
	if err = xmlb.Unmarshal([]byte(text), &b); err != nil {
		return fmt.Errorf("%w: a:blip: %w", render.ErrInvalid, err)
	}
	spec.effects, spec.effectsKey = b.Effects, text
	var walk func(n *wordNode)
	walk = func(n *wordNode) {
		if n.space == nsA && wordColorElements[n.name] && len(n.children) > 0 {
			spec.colorTransforms = true
		}
		for _, c := range n.children {
			walk(c)
		}
	}
	for _, k := range effectKids {
		walk(k)
	}
	return nil
}

// parseShapeProps reads the picture's pic:spPr.
func (r *wordRenderer) parseShapeProps(sp *wordNode, spec *wordPicSpec) error {
	for _, c := range sp.children {
		if c.space != nsA {
			continue
		}
		var err error
		switch c.name {
		case "xfrm":
			if v, ok := c.rawAttr("", "rot"); ok {
				n, e := strconv.ParseInt(strings.TrimSpace(v), 10, 32)
				if e != nil {
					return fmt.Errorf("%w: a:xfrm rot", render.ErrInvalid)
				}
				spec.rot = math.Mod(float64(n)/60000, 360)
			}
			if spec.flipH, err = wordTrue(c, "flipH"); err != nil {
				return err
			}
			if spec.flipV, err = wordTrue(c, "flipV"); err != nil {
				return err
			}
		case "prstGeom":
			if prst, _ := c.rawAttr("", "prst"); prst != "rect" {
				err = r.approximate("picture shape, drawn as a rectangle")
			} else if av := c.childIn(nsA, "avLst"); av != nil && len(av.children) > 0 {
				err = r.approximate("picture shape, drawn as a rectangle")
			}
		case "custGeom":
			err = r.approximate("picture shape, drawn as a rectangle")
		case "noFill":
		case "solidFill", "gradFill", "pattFill", "blipFill", "grpFill":
			err = r.approximate("fill behind a picture not drawn")
		case "ln":
			spec.outline, err = r.parseOutline(c)
		case "effectLst", "effectDag", "scene3d", "sp3d":
			if len(c.children) > 0 {
				err = r.approximate("picture shadow, glow and 3-D effects not drawn")
			}
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// parseOutline reads a picture's a:ln. A solid line is drawn exactly; its dash
// and join styles are approximated by a solid line.
func (r *wordRenderer) parseOutline(ln *wordNode) (*wordOutline, error) {
	width, _, err := wordEMUAttr(ln, "w", "a:ln w")
	if err != nil {
		return nil, err
	}
	var out *wordOutline
	for _, c := range ln.children {
		if c.space != nsA {
			continue
		}
		switch c.name {
		case "solidFill":
			col, ok, e := r.fillColor(c)
			if e != nil {
				return nil, e
			}
			if !ok {
				return nil, r.approximate("picture outline colour not drawn")
			}
			out = &wordOutline{width: width, color: col}
		case "gradFill", "pattFill":
			return nil, r.approximate("picture outline fill not drawn")
		case "prstDash":
			if v, _ := c.rawAttr("", "val"); v != "solid" && v != "" {
				if err = r.approximate("dashed picture outline drawn solid"); err != nil {
					return nil, err
				}
			}
		case "custDash":
			if err = r.approximate("dashed picture outline drawn solid"); err != nil {
				return nil, err
			}
		}
	}
	if out != nil && out.width <= 0 {
		return nil, nil
	}
	return out, nil
}

// fillColor resolves the colour of an a:solidFill: a colour with no
// transforms in sRGB, the theme or the system. ok is false for any other.
func (r *wordRenderer) fillColor(fill *wordNode) (wordRGB, bool, error) {
	if len(fill.children) != 1 {
		return wordRGB{}, false, fmt.Errorf("%w: a:solidFill", render.ErrInvalid)
	}
	c := fill.children[0]
	if c.space != nsA || len(c.children) > 0 {
		return wordRGB{}, false, nil
	}
	col, ok := r.drawingColor(c.name, c.attrOrEmptyNS("val"), c.attrOrEmptyNS("lastClr"))
	return col, ok, nil
}

// drawingColor resolves a DrawingML colour element without transforms.
func (r *wordRenderer) drawingColor(kind, val, last string) (wordRGB, bool) {
	switch kind {
	case "srgbClr":
		return wordParseHex(val)
	case "sysClr":
		return wordParseHex(last)
	case "schemeClr":
		if r.theme == nil {
			return wordRGB{}, false
		}
		target := val
		if slot, ok := wordDefaultClrMap[val]; ok {
			target = slot
			if m := r.clrMap(); m != nil {
				if mapped, ok := m[val]; ok {
					target = mapped
				}
			}
		}
		c, ok := r.theme.colors[target]
		return c, ok
	}
	return wordRGB{}, false
}

// wordSerialize writes a node tree as XML text with its namespaces declared on
// the root, for the dml model to read.
func wordSerialize(n *wordNode) (string, error) {
	b := xmlb.NewBuilder()
	var decls []xmlb.NSDecl
	declared := map[string]bool{}
	var collect func(n *wordNode)
	collect = func(n *wordNode) {
		spaces := make([]string, 0, 1+len(n.attrs))
		spaces = append(spaces, n.space)
		for _, a := range n.attrs {
			spaces = append(spaces, a.space)
		}
		for _, s := range spaces {
			if s == "" || declared[s] {
				continue
			}
			declared[s] = true
			prefix := "n" + strconv.Itoa(len(decls))
			decls = append(decls, xmlb.NSDecl{Prefix: prefix, URI: s})
			b.RegisterNamespace(s, prefix)
		}
		for _, c := range n.children {
			collect(c)
		}
	}
	collect(n)
	attrs := func(n *wordNode) []xmlb.Attr {
		out := make([]xmlb.Attr, 0, len(n.attrs))
		for _, a := range n.attrs {
			out = append(out, xmlb.Attr{Namespace: a.space, Name: a.name, Value: a.value})
		}
		return out
	}
	var write func(n *wordNode)
	write = func(n *wordNode) {
		b.StartElement(n.space, n.name, attrs(n)...)
		for _, c := range n.children {
			write(c)
		}
		b.EndElement(n.space, n.name)
	}
	b.StartElementWithNS(n.space, n.name, decls, attrs(n)...)
	for _, c := range n.children {
		write(c)
	}
	b.EndElement(n.space, n.name)
	if err := b.Finish(); err != nil {
		return "", fmt.Errorf("%w: a:blip: %w", render.ErrInvalid, err)
	}
	return b.String(), nil
}
