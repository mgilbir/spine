package docxrender

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mgilbir/forme/layout"
	"github.com/mgilbir/forme/style"
	"github.com/mgilbir/spine/render"
)

// wordLaidSection is a section laid out once on an unbounded sheet, with its
// blocks located in the result.
type wordLaidSection struct {
	props  wordSectProps
	blocks []*wordLaidBlock
	// ops is the whole section painted, in section coordinates.
	ops []layout.Op
	// chunks are the pieces of the section placed on pages, in layout order.
	chunks []*wordChunk
}

// wordLaidBlock is a block with its geometry in section pixels.
type wordLaidBlock struct {
	*wordBlock
	top, bottom float64
	units       []wordUnit
}

// wordResolveSpacing applies contextual spacing and automatic-spacing collapse
// between neighbouring paragraph blocks.
func wordResolveSpacing(blocks []*wordBlock) {
	for i, b := range blocks {
		if b.kind != "p" {
			continue
		}
		if i > 0 && blocks[i-1].kind == "p" {
			prev := blocks[i-1]
			if b.contextual && prev.styleID == b.styleID && !b.continuation {
				b.before = 0
			}
			if prev.afterAuto && b.beforeAuto {
				b.before = 0
			}
		}
		if b.contextual && i+1 < len(blocks) && blocks[i+1].kind == "p" && blocks[i+1].styleID == b.styleID && !blocks[i+1].continuation {
			b.after = 0
		}
	}
}

// layoutSection builds, lays out and paints one section.
func (r *wordRenderer) layoutSection(sec *wordSection) (*wordLaidSection, error) {
	if err := r.ctx.Err(); err != nil {
		return nil, err
	}
	wordResolveSpacing(sec.blocks)
	// The body's own font is irrelevant to the lines (it is one pixel with no
	// line height) but layout wants a family it can resolve.
	strut := r.fonts.first
	if strut == nil {
		lv, err := r.styles.paragraph("")
		if err != nil {
			return nil, err
		}
		if strut, err = r.fonts.get(wordFamily(&lv.rpr, wordSlotASCII), false, false); err != nil {
			return nil, err
		}
	}
	var sb strings.Builder
	sb.WriteString(`<!DOCTYPE html><html><body style="margin:0;font-family:`)
	sb.WriteString(strut.css)
	sb.WriteString(`;font-size:1px;line-height:0;font-kerning:none;font-variant-ligatures:none">`)
	for i, b := range sec.blocks {
		var c wordCSS
		c.px("padding-top", b.before)
		c.px("padding-bottom", b.after)
		sb.WriteString(`<div id="b` + strconv.Itoa(i) + `" style="` + c.String() + `">`)
		sb.WriteString(b.inner)
		sb.WriteString("</div>")
	}
	sb.WriteString("</body></html>")
	if err := r.ctx.Err(); err != nil {
		return nil, err
	}
	p := sec.props
	built := layout.BuildFor(layout.Input{HTML: sb.String(), Fonts: r.fonts}, layout.PageSizePt(p.w*0.75, p.h*0.75))
	if err := r.findings(built.Findings, built.Failed); err != nil {
		return nil, err
	}
	if built.Root == nil {
		return nil, fmt.Errorf("%w: layout produced no boxes", render.ErrUnsupported)
	}
	if err := r.ctx.Err(); err != nil {
		return nil, err
	}
	width, ok := style.FromPx(p.contentW())
	if !ok {
		return nil, render.ErrLimit
	}
	rec := layout.NewRecorder(nil)
	frag := layout.Layout(built.Root, layout.Size{W: width}, built.Fonts, rec)
	if frag == nil {
		return nil, fmt.Errorf("%w: layout produced no fragments", render.ErrUnsupported)
	}
	if err := r.ctx.Err(); err != nil {
		return nil, err
	}
	ops := layout.PaintReporting(frag, rec)
	if err := r.findings(rec.Findings(), rec.Failed()); err != nil {
		return nil, err
	}
	if len(ops) > 64*r.limits.MaxOperations {
		return nil, render.ErrLimit
	}
	laid := &wordLaidSection{props: p, ops: ops}
	frags := make([]*layout.Fragment, len(sec.blocks))
	wordFindBlocks(frag, frags)
	for i, b := range sec.blocks {
		f := frags[i]
		if f == nil {
			return nil, fmt.Errorf("%w: layout dropped a block", render.ErrUnsupported)
		}
		lb := &wordLaidBlock{wordBlock: b, top: f.BorderRect.Y.Px(), bottom: f.BorderRect.Bottom().Px()}
		if b.units != nil {
			lb.units = b.units(f)
		} else {
			lb.units = wordParagraphUnits(f)
		}
		if len(lb.units) == 0 {
			lb.units = []wordUnit{{lb.top, lb.bottom}}
		}
		if lb.bottom >= style.MaxUnit.Px()/2 {
			return nil, render.ErrLimit
		}
		laid.blocks = append(laid.blocks, lb)
	}
	return laid, nil
}

// wordFindBlocks locates the wrapper fragments of a section's blocks by the
// generated id attribute.
func wordFindBlocks(root *layout.Fragment, out []*layout.Fragment) {
	stack := []*layout.Fragment{root}
	for len(stack) > 0 {
		f := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if f.Box != nil && f.Box.Element != nil {
			if id, ok := f.Box.Element.Attr("id"); ok && strings.HasPrefix(id, "b") {
				if k, err := strconv.Atoi(id[1:]); err == nil && k >= 0 && k < len(out) && out[k] == nil {
					out[k] = f
					continue
				}
			}
		}
		for i := len(f.Children) - 1; i >= 0; i-- {
			stack = append(stack, f.Children[i])
		}
	}
}

// wordParagraphUnits returns the line boxes of a paragraph block: the lines of
// the first fragment under the wrapper that has any.
func wordParagraphUnits(wrapper *layout.Fragment) []wordUnit {
	stack := []*layout.Fragment{wrapper}
	for len(stack) > 0 {
		f := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if len(f.Lines) > 0 {
			content := f.ContentRect()
			units := make([]wordUnit, 0, len(f.Lines))
			for _, l := range f.Lines {
				top := content.Y.Add(l.Rect.Y).Px()
				units = append(units, wordUnit{top, top + l.Rect.H.Px()})
			}
			return units
		}
		for i := len(f.Children) - 1; i >= 0; i-- {
			stack = append(stack, f.Children[i])
		}
	}
	return nil
}

// findings maps layout's findings to the renderer's policy. The generated
// markup is controlled, so any finding means something was not drawn as
// requested: strict mode fails, best-effort mode reports it.
func (r *wordRenderer) findings(list []layout.Finding, failed bool) error {
	for _, f := range list {
		msg := f.Error()
		if !r.lenient {
			return fmt.Errorf("%w: docx: layout: %s", render.ErrUnsupported, msg)
		}
		if err := r.approximate("layout: " + msg); err != nil {
			return err
		}
	}
	if failed && len(list) == 0 {
		return fmt.Errorf("%w: docx: layout failed", render.ErrUnsupported)
	}
	return nil
}
