package docxrender

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/mgilbir/forme/layout"
	"github.com/mgilbir/forme/style"
	"github.com/mgilbir/spine/render"
)

// Placing pictures.
//
// An inline picture is an <img> on the line, sized by its extent, with the
// effect extent as its margins. Its line grows to hold it.
//
// An anchored picture (wp:anchor) is placed by where it is anchored:
//
//   - A picture with no wrapping (wrapNone) is drawn over, or behind, the page
//     at a position computed from the page and the anchor paragraph once the
//     layout is cut into pages. Text is unaffected, as in Word. It belongs to
//     the page of its anchor line.
//   - A picture that text wraps around (square, tight, through, top and
//     bottom) and that is positioned from its paragraph or line is a float in
//     the layout, so text wraps around it as it does in Word, and it moves with
//     its paragraph from page to page. Tight and through wrapping follow the
//     picture's box, not its outline.
//   - A wrapped picture positioned from the page or margin cannot be a float
//     (its position does not depend on where the paragraph falls in the flow,
//     which pagination decides after layout). It is drawn at its position and
//     the text is not made to avoid it, which is reported.
//
// Every anchored picture also leaves a zero-size marker in the text, whose
// position after layout tells the line it is anchored to.

func init() { wordRegisterRun("drawing", (*wordRun).drawing) }

// wordMarkerHTML is the marker of an anchored picture.
func wordMarkerHTML(id int) string {
	return `<span id="a` + strconv.Itoa(id) + `" style="display:inline-block;width:0;height:0"></span>`
}

// wordFloatToken stands for a float's markup in a block until its section's
// page is known.
func wordFloatToken(id int) string { return "\x02f" + strconv.Itoa(id) + "\x02" }

// wordPlacement is where layout put an anchored picture, in section pixels.
type wordPlacement struct {
	pic *wordPicture
	// markerX is the horizontal position of the anchor in the text area.
	markerX float64
	// lineTop and lineBottom are the anchor line; paraTop and paraBottom the
	// extent of the lines of the anchor paragraph's block.
	lineTop, lineBottom float64
	paraTop, paraBottom float64
}

// drawing translates a w:drawing.
func (rn *wordRun) drawing(n *wordNode) error {
	if rn.hidden() {
		return nil
	}
	p := rn.p
	r := p.r
	d, err := r.parseDrawing(n)
	if err != nil || d == nil {
		return err
	}
	pic, err := r.buildPicture(d)
	if err != nil || pic == nil {
		return err
	}
	if pic.anchor == nil {
		if ln := p.ppr.line; ln.set && ln.v.rule == "exact" && pic.h+pic.ext[1]+pic.ext[3] > ln.v.val {
			// Word clips a picture to a line of exact height; layout grows
			// the line.
			if err = r.approximate("picture in a line of exact height"); err != nil {
				return err
			}
		}
		return rn.putMarkup(pic.html(), 1)
	}
	return p.anchored(rn, pic)
}

// anchored places an anchored picture: its marker goes in the text, and a
// float's markup is deferred to layout.
func (p *wordPara) anchored(rn *wordRun, pic *wordPicture) error {
	r := p.r
	a := pic.anchor
	if err := rn.putMarkup(wordMarkerHTML(pic.id), 1); err != nil {
		return err
	}
	p.pics = append(p.pics, pic)
	if a.wrap == "none" {
		return nil
	}
	if a.wrap == "tight" || a.wrap == "through" {
		if err := r.approximate("tight and through text wrapping drawn around the picture's box"); err != nil {
			return err
		}
	}
	paragraphRel := a.vFrom == "paragraph" || a.vFrom == "line"
	if a.simplePos || !paragraphRel || (a.vAlign != "" && a.vAlign != "top") {
		return r.approximate("text wrapping around a picture positioned on the page not applied")
	}
	if p.tabMode {
		if err := r.approximate("tab stops in a paragraph beside a floating picture"); err != nil {
			return err
		}
	}
	pic.flow = true
	token := wordFloatToken(pic.id)
	if a.vFrom == "line" && !p.tabMode {
		// A line-relative float starts at the anchor's line.
		return rn.putMarkup(token, 0)
	}
	p.lead.WriteString(token)
	return nil
}

// html is the markup of an inline picture: an image on the line, with the
// effect extent as its margins. A drawing that is not drawn is an empty box of
// the same size.
func (pic *wordPicture) html() string {
	var c wordCSS
	c.px("width", pic.w)
	c.px("height", pic.h)
	c.add("margin", wordPx(pic.ext[1])+" "+wordPx(pic.ext[2])+" "+wordPx(pic.ext[3])+" "+wordPx(pic.ext[0]))
	if pic.img == nil {
		c.add("display", "inline-block")
		return `<span style="` + c.String() + `"></span>`
	}
	return `<img src="i` + strconv.Itoa(pic.id) + `" style="` + c.String() + `">`
}

// floatHTML is the markup of a floated picture in a section with the given
// text width.
func (r *wordRenderer) floatHTML(pic *wordPicture, props wordSectProps) (string, error) {
	a := pic.anchor
	W := props.contentW()
	w, h := pic.w, pic.h
	outerW := pic.ext[0] + w + pic.ext[2]
	// Horizontal reference: the text area, the page or a margin, relative to the
	// text area's left edge.
	refL, refR := 0.0, W
	switch a.hFrom {
	case "page":
		refL, refR = -props.left, props.w-props.left
	case "leftMargin", "insideMargin":
		refL, refR = -props.left, 0
	case "rightMargin", "outsideMargin":
		refL, refR = W, W+props.right
	}
	if a.hFrom == "insideMargin" || a.hFrom == "outsideMargin" || a.hAlign == "inside" || a.hAlign == "outside" {
		if err := r.approximate("inside and outside picture positions drawn as left and right"); err != nil {
			return "", err
		}
	}
	if a.hFrom == "character" {
		if err := r.approximate("picture positioned from its character drawn from the column"); err != nil {
			return "", err
		}
	}
	var outerLeft float64
	switch a.hAlign {
	case "left", "inside":
		outerLeft = refL
	case "right", "outside":
		outerLeft = refR - outerW
	case "center":
		outerLeft = (refL+refR)/2 - outerW/2
	default:
		outerLeft = refL + a.hOff
	}
	x0 := outerLeft + pic.ext[0]
	left := outerLeft - a.dist[2]
	right := W - (outerLeft + outerW + a.dist[3])
	var c wordCSS
	c.px("width", w)
	c.px("height", h)
	side := "left"
	marginL, marginR := x0, pic.ext[2]+a.dist[3]
	if a.wrap == "topBottom" {
		marginR = W - x0 - w
	} else {
		// The side text is on: the larger gap, or the one asked for.
		textLeft := false
		switch a.wrapText {
		case "left":
			textLeft = true
		case "right":
		case "largest":
			textLeft = left >= right
		default:
			switch {
			case left <= 1:
			case right <= 1:
				textLeft = true
			default:
				if err := r.approximate("text wrapping on both sides of a picture drawn on one side"); err != nil {
					return "", err
				}
				textLeft = left >= right
			}
		}
		if textLeft {
			side = "right"
			marginL, marginR = pic.ext[0]+a.dist[2], W-x0-w
		}
	}
	top := 0.0
	if a.vAlign == "" {
		top = a.vOff
	}
	if top < 0 {
		top = 0
		if err := r.approximate("picture positioned above its paragraph drawn at the paragraph"); err != nil {
			return "", err
		}
	}
	if top-a.dist[0] > 1 {
		if err := r.approximate("text beside the space above a picture does not wrap around it"); err != nil {
			return "", err
		}
	}
	c.add("float", side)
	c.add("margin", wordPx(top+pic.ext[1])+" "+wordPx(marginR)+" "+wordPx(pic.ext[3]+a.dist[1])+" "+wordPx(marginL))
	id := strconv.Itoa(pic.id)
	if pic.img == nil {
		c.add("display", "block")
		return `<span id="f` + id + `" style="` + c.String() + `"></span>`, nil
	}
	return `<img id="f` + id + `" src="i` + id + `" style="` + c.String() + `">`, nil
}

// expandFloats replaces the float tokens of a section's blocks, in one pass
// over each block.
func (r *wordRenderer) expandFloats(sec *wordSection) error {
	for _, b := range sec.blocks {
		var floats map[int]*wordPicture
		for _, pic := range b.pics {
			if pic.flow {
				if floats == nil {
					floats = map[int]*wordPicture{}
				}
				floats[pic.id] = pic
			}
		}
		if floats == nil {
			continue
		}
		var sb strings.Builder
		rest := b.inner
		for {
			i := strings.Index(rest, "\x02f")
			if i < 0 {
				break
			}
			j := strings.IndexByte(rest[i+2:], 2)
			if j < 0 {
				return fmt.Errorf("%w: float token", render.ErrInvalid)
			}
			id, err := strconv.Atoi(rest[i+2 : i+2+j])
			pic := floats[id]
			if err != nil || pic == nil {
				return fmt.Errorf("%w: float token", render.ErrInvalid)
			}
			html, err := r.floatHTML(pic, sec.props)
			if err != nil {
				return err
			}
			if err = r.charge(1); err != nil {
				return err
			}
			sb.WriteString(rest[:i])
			sb.WriteString(html)
			rest = rest[i+2+j+1:]
		}
		sb.WriteString(rest)
		b.inner = sb.String()
	}
	return nil
}

// wordFindElements maps the ids of the elements whose id starts with a letter
// in prefixes and continues with digits to their fragments.
func wordFindElements(root *layout.Fragment, prefixes string) map[string]*layout.Fragment {
	out := map[string]*layout.Fragment{}
	stack := []*layout.Fragment{root}
	for len(stack) > 0 {
		f := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if f.Box != nil && f.Box.Element != nil {
			if id, ok := f.Box.Element.Attr("id"); ok && len(id) > 1 && strings.IndexByte(prefixes, id[0]) >= 0 {
				if _, dup := out[id]; !dup {
					out[id] = f
				}
			}
		}
		for i := len(f.Children) - 1; i >= 0; i-- {
			stack = append(stack, f.Children[i])
		}
	}
	return out
}

// locatePictures finds where layout put the anchored pictures of a section's
// blocks.
func wordLocatePictures(laid *wordLaidSection, root *layout.Fragment) {
	anchored := false
	for _, b := range laid.blocks {
		for _, pic := range b.pics {
			if pic.anchor != nil {
				anchored = true
			}
		}
	}
	if !anchored {
		return
	}
	found := wordFindElements(root, "a")
	laid.places = map[int]*wordPlacement{}
	for _, b := range laid.blocks {
		for _, pic := range b.pics {
			if pic.anchor == nil {
				continue
			}
			id := strconv.Itoa(pic.id)
			pl := &wordPlacement{pic: pic, paraTop: b.units[0].top, paraBottom: b.units[len(b.units)-1].bottom}
			pl.lineTop, pl.lineBottom = pl.paraTop, b.units[0].bottom
			if m := found["a"+id]; m != nil {
				pl.markerX = m.BorderRect.X.Px()
				// The first line whose bottom is at or below the marker's baseline.
				y := m.BorderRect.Y.Px()
				if k := sort.Search(len(b.units), func(i int) bool { return b.units[i].bottom+wordEps >= y }); k < len(b.units) && y >= b.units[k].top-wordEps {
					pl.lineTop, pl.lineBottom = b.units[k].top, b.units[k].bottom
				}
			}
			laid.places[pic.id] = pl
		}
	}
}

// distributeImage places a picture drawn by layout: an inline picture or a
// float. The placeholder's pixel names the picture.
func (r *wordRenderer) distributeImage(sec *wordLaidSection, chunkAt func(float64) *wordChunk, v layout.DrawImage) error {
	id, ok := wordImageID(v.Image)
	if !ok || id >= len(r.pictures) {
		return fmt.Errorf("%w: image drawn by layout", render.ErrUnsupported)
	}
	if v.Clip.Active {
		return fmt.Errorf("%w: clipped image", render.ErrUnsupported)
	}
	pic := r.pictures[id]
	y := v.Rect.Y.Px() + v.Rect.H.Px()/2
	if pl := sec.places[id]; pl != nil {
		y = pl.lineTop + wordEps
	}
	c := chunkAt(y)
	t, err := c.shift()
	if err != nil {
		return err
	}
	if pl := sec.places[id]; pl != nil && pic.flow {
		if end := c.y0 + (c.page.bodyH() - c.page.notes.h - c.dest); v.Rect.Y.Px()+v.Rect.H.Px() > end+wordEps {
			if err = r.approximate("picture that crosses a page boundary"); err != nil {
				return err
			}
		}
	}
	return r.putPicture(c, pic, v.Rect.X.Px()+t.x.Px(), v.Rect.Y.Px()+t.y.Px(), pic.w, pic.h)
}

// putPicture adds a picture whose frame has its top left corner at (x, y) page
// pixels.
func (r *wordRenderer) putPicture(c *wordChunk, pic *wordPicture, x, y, w, h float64) error {
	var ops []layout.Op
	if pic.img != nil {
		cx, cy := x+w/2, y+h/2
		rect, ok := wordRect(cx-pic.bw/2, cy-pic.bh/2, pic.bw, pic.bh)
		if !ok {
			// Beyond what layout can place: not on any page.
			return nil
		}
		ops = append(ops, layout.DrawImage{Rect: rect, Image: pic.img})
	}
	if pic.outline != nil {
		ops = append(ops, wordOutlineOps(pic.outline, x, y, w, h)...)
	}
	if len(ops) == 0 {
		return nil
	}
	if pic.anchor != nil && pic.anchor.behind {
		c.page.behind = append(c.page.behind, wordOverlay{ops: ops, z: pic.anchor.z, seq: pic.id})
		return nil
	}
	if pic.anchor != nil && !pic.flow {
		c.page.front = append(c.page.front, wordOverlay{ops: ops, z: pic.anchor.z, seq: pic.id})
		return nil
	}
	c.page.ops = append(c.page.ops, ops...)
	return nil
}

// wordRect converts pixels to a layout rectangle.
func wordRect(x, y, w, h float64) (layout.Rect, bool) {
	ux, ok1 := style.FromPx(x)
	uy, ok2 := style.FromPx(y)
	uw, ok3 := style.FromPx(w)
	uh, ok4 := style.FromPx(h)
	return layout.Rect{X: ux, Y: uy, W: uw, H: uh}, ok1 && ok2 && ok3 && ok4
}

// wordOutlineOps draws a picture's line centred on the edge of its frame.
func wordOutlineOps(o *wordOutline, x, y, w, h float64) []layout.Op {
	half := o.width / 2
	col := style.RGBA{R: float64(o.color.r), G: float64(o.color.g), B: float64(o.color.b), A: 1}
	var ops []layout.Op
	for _, s := range [][4]float64{
		{x - half, y - half, w + o.width, o.width},
		{x - half, y + h - half, w + o.width, o.width},
		{x - half, y + half, o.width, h - o.width},
		{x + w - half, y + half, o.width, h - o.width},
	} {
		if s[2] <= 0 || s[3] <= 0 {
			continue
		}
		if rect, ok := wordRect(s[0], s[1], s[2], s[3]); ok {
			ops = append(ops, layout.FillRect{Rect: rect, Color: col})
		}
	}
	return ops
}

// wordOverlay is a picture drawn over or behind the page's text.
type wordOverlay struct {
	ops []layout.Op
	z   int64
	seq int
}

// overlays draws the anchored pictures that are not floats: each belongs to
// the page of its anchor line.
func (r *wordRenderer) overlays(sec *wordLaidSection, chunkAt func(float64) *wordChunk) error {
	ids := make([]int, 0, len(sec.places))
	for id := range sec.places {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	for _, id := range ids {
		pl := sec.places[id]
		pic := pl.pic
		if pic.flow {
			continue
		}
		if err := r.ctx.Err(); err != nil {
			return err
		}
		c := chunkAt(pl.lineTop + wordEps)
		t, err := c.shift()
		if err != nil {
			return err
		}
		x, y, err := r.anchorPosition(pic, pl, c, t)
		if err != nil {
			return err
		}
		if err = r.putPicture(c, pic, x, y, pic.w, pic.h); err != nil {
			return err
		}
	}
	return nil
}

// anchorPosition is the page position of the frame of a picture that is not a
// float. Offsets and alignments place the picture's outer box, the frame with
// its effect extent.
func (r *wordRenderer) anchorPosition(pic *wordPicture, pl *wordPlacement, c *wordChunk, t wordShift) (x, y float64, err error) {
	a := pic.anchor
	props := c.sec.props
	outerW := pic.ext[0] + pic.w + pic.ext[2]
	outerH := pic.ext[1] + pic.h + pic.ext[3]
	if a.simplePos {
		return a.simpleX + pic.ext[0], a.simpleY + pic.ext[1], nil
	}
	pageW, pageH := c.page.w, c.page.h
	refL, refR := props.left, pageW-props.right
	switch a.hFrom {
	case "page":
		refL, refR = 0, pageW
	case "leftMargin", "insideMargin":
		refL, refR = 0, props.left
	case "rightMargin", "outsideMargin":
		refL, refR = pageW-props.right, pageW
	}
	if a.hFrom == "insideMargin" || a.hFrom == "outsideMargin" || a.hAlign == "inside" || a.hAlign == "outside" {
		if err = r.approximate("inside and outside picture positions drawn as left and right"); err != nil {
			return 0, 0, err
		}
	}
	var left float64
	switch a.hAlign {
	case "left", "inside":
		left = refL
	case "right", "outside":
		left = refR - outerW
	case "center":
		left = (refL+refR)/2 - outerW/2
	default:
		if a.hFrom == "character" {
			refL = props.left + pl.markerX
		}
		left = refL + a.hOff
	}
	refT, refB := props.top, pageH-props.bottom
	switch a.vFrom {
	case "page":
		refT, refB = 0, pageH
	case "topMargin", "insideMargin":
		refT, refB = 0, props.top
	case "bottomMargin", "outsideMargin":
		refT, refB = pageH-props.bottom, pageH
	case "paragraph":
		refT, refB = t.y.Px()+pl.paraTop, t.y.Px()+pl.paraBottom
	case "line":
		refT, refB = t.y.Px()+pl.lineTop, t.y.Px()+pl.lineBottom
	}
	if a.vFrom == "insideMargin" || a.vFrom == "outsideMargin" || a.vAlign == "inside" || a.vAlign == "outside" {
		if err = r.approximate("inside and outside picture positions drawn as top and bottom"); err != nil {
			return 0, 0, err
		}
	}
	var top float64
	switch a.vAlign {
	case "top", "inside":
		top = refT
	case "bottom", "outside":
		top = refB - outerH
	case "center":
		top = (refT+refB)/2 - outerH/2
	default:
		top = refT + a.vOff
	}
	if math.IsNaN(left) || math.IsNaN(top) {
		return 0, 0, fmt.Errorf("%w: picture position", render.ErrInvalid)
	}
	return left + pic.ext[0], top + pic.ext[1], nil
}
