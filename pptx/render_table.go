package pptx

import (
	"context"
	"fmt"
	"reflect"

	"github.com/mgilbir/forme/layout"
	"github.com/mgilbir/forme/style"
	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/common/enum"
	core "github.com/mgilbir/spine/internal/render"
	"github.com/mgilbir/spine/pptx/internal/oxml"
	"github.com/mgilbir/spine/render"
)

// renderSourceFrame returns the parsed graphic frame behind a shape, or nil.
func (s *Slide) renderSourceFrame(index int) *oxml.GraphicFrame {
	if index >= len(s.shapeRefs) {
		return nil
	}
	ref := s.shapeRefs[index]
	if ref.Kind != oxml.ChildGraphicFrame || ref.Index < 0 || ref.Index >= len(s.sxModel.CSld.SpTree.GraphicFrame) {
		return nil
	}
	return s.sxModel.CSld.SpTree.GraphicFrame[ref.Index]
}

// renderBorder is a resolved cell edge; the zero value draws nothing.
type renderBorder struct {
	width dml.EMU
	color style.RGBA
}

// renderTable paints an unstyled table: cell fills, then borders, then cell
// text. A table with a style fails: the style may be a built-in Office style
// the file does not define, and whether PowerPoint applies the presentation's
// default table style to a table without a style id is undocumented, so such
// a table is drawn without one. Explore both before supporting styles.
//
// Only a parsed table without pending edits is drawn: the save path rewrites
// the domain model's cells, which preparation must not do.
func (s *Slide) renderTable(ctx context.Context, index int, t *Table, colors *renderColors, breaker *core.TextLayout, fonts *slideRenderFonts, styles *renderTextStyles) ([]layout.Op, error) {
	gf := s.renderSourceFrame(index)
	if gf == nil || t.isDirty() {
		return nil, fmt.Errorf("%w: new or edited table; save and reopen to preview it", render.ErrUnsupported)
	}
	if gf.ExtLst != nil || gf.Xfrm == nil || gf.Xfrm.Off == nil || gf.Graphic == nil || gf.Graphic.GraphicData == nil || gf.Graphic.GraphicData.URI != oxml.TableGraphicDataURI || gf.Graphic.GraphicData.Table == nil {
		return nil, fmt.Errorf("%w: graphic frame", render.ErrUnsupported)
	}
	if gf.Xfrm.Rot != 0 || gf.Xfrm.FlipH || gf.Xfrm.FlipV {
		return nil, fmt.Errorf("%w: table transformation", render.ErrUnsupported)
	}
	tbl := gf.Graphic.GraphicData.Table
	if pr := tbl.TblPr; pr != nil {
		if pr.TableStyle != nil || pr.TableStyleId != "" {
			return nil, fmt.Errorf("%w: table style", render.ErrUnsupported)
		}
		if pr.Rtl || pr.NoFill != nil || pr.SolidFill != nil || pr.GradFill != nil || pr.BlipFill != nil || pr.PattFill != nil || pr.GrpFill != nil || renderEffects(pr.EffectLst) || pr.EffectDag != nil || pr.ExtLst != nil {
			return nil, fmt.Errorf("%w: table properties", render.ErrUnsupported)
		}
		// Heading and banding flags select parts of a table style; without
		// one they have no effect.
	}
	if tbl.TblGrid == nil || len(tbl.TblGrid.GridCol) == 0 || len(tbl.Tr) == 0 {
		return nil, fmt.Errorf("%w: table grid", render.ErrInvalid)
	}
	cols := len(tbl.TblGrid.GridCol)
	rows := len(tbl.Tr)
	if cols > fonts.nodes || rows > fonts.nodes/cols {
		return nil, fmt.Errorf("%w: table cells", render.ErrLimit)
	}
	fonts.nodes -= rows * cols
	// Grid lines in EMU: xs[c] is the left of column c, ys[r] the top of row r.
	xs := make([]dml.EMU, cols+1)
	xs[0] = dml.EMU(gf.Xfrm.Off.X)
	for c, gc := range tbl.TblGrid.GridCol {
		if gc == nil || gc.W <= 0 {
			return nil, fmt.Errorf("%w: column width", render.ErrInvalid)
		}
		if gc.ExtLst != nil && !renderMetadataExt(gc.ExtLst, "a:gridCol") {
			return nil, fmt.Errorf("%w: column extension", render.ErrUnsupported)
		}
		xs[c+1] = xs[c] + dml.EMU(gc.W)
		// Bound each edge before the next sum so hostile widths cannot
		// overflow it.
		if gc.W > 1<<40 || xs[c+1] > 1<<40 {
			return nil, fmt.Errorf("%w: table width", render.ErrLimit)
		}
	}
	type cell struct {
		tc     *oxml.ATc
		blocks []renderBlock
		height float64 // text height in pixels
		margin [4]dml.EMU
		anchor enum.TextAnchor
	}
	cells := make([][]cell, rows)
	ys := make([]float64, rows+1) // in pixels; rows grow to fit their text
	ys[0] = float64(gf.Xfrm.Off.Y) / float64(dml.EMUsPerPixel)
	if err := styles.load(); err != nil {
		return nil, err
	}
	for r, tr := range tbl.Tr {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if tr == nil || len(tr.Tc) != cols {
			return nil, fmt.Errorf("%w: table row", render.ErrInvalid)
		}
		if tr.ExtLst != nil && !renderMetadataExt(tr.ExtLst, "a:tr") {
			return nil, fmt.Errorf("%w: row extension", render.ErrUnsupported)
		}
		height := 0.0
		if tr.H != nil {
			if *tr.H < 0 {
				return nil, fmt.Errorf("%w: row height", render.ErrInvalid)
			}
			height = float64(*tr.H) / float64(dml.EMUsPerPixel)
		}
		cells[r] = make([]cell, cols)
		for c, tc := range tr.Tc {
			if tc == nil {
				return nil, fmt.Errorf("%w: table cell", render.ErrInvalid)
			}
			if tc.RowSpan > 1 || tc.GridSpan > 1 || tc.HMerge || tc.VMerge || tc.ExtLst != nil {
				return nil, fmt.Errorf("%w: merged or extended table cell", render.ErrUnsupported)
			}
			cl := cell{tc: tc, margin: [4]dml.EMU{91440, 45720, 91440, 45720}, anchor: enum.TextAnchorTop}
			if pr := tc.TcPr; pr != nil {
				if (pr.Vert != "" && pr.Vert != "horz") || renderTrue(pr.AnchorCtr) || pr.HorzOverflow != "" || pr.LnTlToBr != nil || pr.LnBlToTr != nil || pr.Cell3D != nil || pr.GradFill != nil || pr.BlipFill != nil || pr.PattFill != nil || pr.GrpFill != nil || pr.ExtLst != nil {
					return nil, fmt.Errorf("%w: table cell properties", render.ErrUnsupported)
				}
				for i, m := range []*int64{pr.MarL, pr.MarT, pr.MarR, pr.MarB} {
					if m != nil {
						if *m < 0 {
							return nil, fmt.Errorf("%w: cell margin", render.ErrInvalid)
						}
						cl.margin[i] = dml.EMU(*m)
					}
				}
				switch enum.TextAnchor(pr.Anchor) {
				case "", enum.TextAnchorTop:
				case enum.TextAnchorMiddle, enum.TextAnchorBottom:
					cl.anchor = enum.TextAnchor(pr.Anchor)
				default:
					return nil, fmt.Errorf("%w: cell anchor", render.ErrUnsupported)
				}
			}
			w := xs[c+1] - xs[c]
			if cl.margin[0] > w || cl.margin[2] >= w-cl.margin[0] {
				return nil, fmt.Errorf("%w: cell margins", render.ErrUnsupported)
			}
			if body := tc.TxBody; body != nil && renderHasText(body) {
				if !renderCellBodyPr(body.BodyPr, cl.margin, cl.anchor) {
					return nil, fmt.Errorf("%w: cell body properties", render.ErrUnsupported)
				}
				if len(body.P) > fonts.nodes {
					return nil, fmt.Errorf("%w: text paragraphs", render.ErrLimit)
				}
				fonts.nodes -= len(body.P)
				blocks, h, err := renderLayoutParagraphs(ctx, body, xs[c]+cl.margin[0], w-cl.margin[0]-cl.margin[2], false, breaker, fonts, styles, styles.shapeChain())
				if err != nil {
					return nil, err
				}
				cl.blocks, cl.height = blocks, h
			}
			height = max(height, cl.height+float64(cl.margin[1]+cl.margin[3])/float64(dml.EMUsPerPixel))
			cells[r][c] = cl
		}
		ys[r+1] = ys[r] + height
	}
	px := func(v dml.EMU) float64 { return float64(v) / float64(dml.EMUsPerPixel) }
	unit := func(v float64) (style.Unit, error) {
		u, ok := style.FromPx(v)
		if !ok {
			return 0, fmt.Errorf("%w: table coordinate", render.ErrLimit)
		}
		return u, nil
	}
	var ops []layout.Op
	// Fills.
	for r := range cells {
		for c, cl := range cells[r] {
			pr := cl.tc.TcPr
			if pr == nil || pr.SolidFill == nil {
				continue
			}
			if pr.NoFill != nil {
				return nil, fmt.Errorf("%w: ambiguous cell fill", render.ErrInvalid)
			}
			color, err := colors.solid(pr.SolidFill, nil)
			if err != nil {
				return nil, err
			}
			x, err1 := unit(px(xs[c]))
			y, err2 := unit(ys[r])
			w, err3 := unit(px(xs[c+1]) - px(xs[c]))
			h, err4 := unit(ys[r+1] - ys[r])
			if err := firstErr(err1, err2, err3, err4); err != nil {
				return nil, err
			}
			ops = append(ops, layout.FillRect{Rect: layout.Rect{X: x, Y: y, W: w, H: h}, Color: color})
		}
	}
	// Borders: every grid edge segment takes the border its cells give it.
	edge := func(ln *dml.Ln) (renderBorder, bool, error) {
		if ln == nil {
			return renderBorder{}, false, nil
		}
		if ln.SolidFill == nil {
			if ln.GradFill != nil || ln.PattFill != nil {
				return renderBorder{}, false, fmt.Errorf("%w: patterned cell border", render.ErrUnsupported)
			}
			// No fill, or none specified without a style: no border.
			return renderBorder{}, true, nil
		}
		if ln.NoFill != nil {
			return renderBorder{}, false, fmt.Errorf("%w: ambiguous cell border", render.ErrInvalid)
		}
		if ln.W == nil || *ln.W <= 0 || (ln.Cmpd != "" && ln.Cmpd != "sng") || (ln.Algn != "" && ln.Algn != "ctr") || (ln.PrstDash != nil && ln.PrstDash.Val != "" && ln.PrstDash.Val != "solid") || ln.CustDash != nil || ln.ExtLst != nil {
			return renderBorder{}, false, fmt.Errorf("%w: cell border width, compound, alignment or dash", render.ErrUnsupported)
		}
		c, err := colors.solid(ln.SolidFill, nil)
		return renderBorder{width: dml.EMU(*ln.W), color: c}, true, err
	}
	pick := func(lns ...*dml.Ln) (renderBorder, error) {
		var (
			out  renderBorder
			have bool
		)
		for _, ln := range lns {
			b, set, err := edge(ln)
			if err != nil {
				return renderBorder{}, err
			}
			if !set {
				continue
			}
			// Which of two disagreeing cells wins a shared edge is
			// undocumented.
			if have && b != out {
				return renderBorder{}, fmt.Errorf("%w: conflicting cell borders", render.ErrUnsupported)
			}
			out, have = b, true
		}
		return out, nil
	}
	lnOf := func(r, c int, side int) *dml.Ln {
		if r < 0 || r >= rows || c < 0 || c >= cols || cells[r][c].tc.TcPr == nil {
			return nil
		}
		pr := cells[r][c].tc.TcPr
		return [...]*dml.Ln{pr.LnL, pr.LnT, pr.LnR, pr.LnB}[side]
	}
	// horizontal[r][c] lies on grid line r under column c; vertical[r][c] on
	// grid line c beside row r.
	horizontal := make([][]renderBorder, rows+1)
	vertical := make([][]renderBorder, rows)
	for r := 0; r <= rows; r++ {
		horizontal[r] = make([]renderBorder, cols)
		for c := 0; c < cols; c++ {
			b, err := pick(lnOf(r-1, c, 3), lnOf(r, c, 1))
			if err != nil {
				return nil, err
			}
			horizontal[r][c] = b
		}
	}
	for r := 0; r < rows; r++ {
		vertical[r] = make([]renderBorder, cols+1)
		for c := 0; c <= cols; c++ {
			b, err := pick(lnOf(r, c-1, 2), lnOf(r, c, 0))
			if err != nil {
				return nil, err
			}
			vertical[r][c] = b
		}
	}
	// Paint order decides which border shows where two cross; require the
	// borders meeting at a grid point to agree, and close their corners by
	// extending a segment half its width where another border meets it.
	at := func(r, c int) ([]renderBorder, error) {
		var met []renderBorder
		for _, b := range []renderBorder{
			cellBorder(horizontal, r, c-1), cellBorder(horizontal, r, c),
			cellBorder(vertical, r-1, c), cellBorder(vertical, r, c),
		} {
			if b.width > 0 {
				met = append(met, b)
			}
		}
		for _, b := range met {
			if b != met[0] {
				return nil, fmt.Errorf("%w: differing borders meet", render.ErrUnsupported)
			}
		}
		return met, nil
	}
	for r := 0; r <= rows; r++ {
		for c := 0; c <= cols; c++ {
			if _, err := at(r, c); err != nil {
				return nil, err
			}
		}
	}
	joined := func(r, c int) bool {
		met, _ := at(r, c)
		return len(met) > 1
	}
	segment := func(b renderBorder, x0, y0, x1, y1 float64) error {
		x, err1 := unit(x0)
		y, err2 := unit(y0)
		w, err3 := unit(x1 - x0)
		h, err4 := unit(y1 - y0)
		if err := firstErr(err1, err2, err3, err4); err != nil {
			return err
		}
		ops = append(ops, layout.FillRect{Rect: layout.Rect{X: x, Y: y, W: w, H: h}, Color: b.color})
		return nil
	}
	for r := 0; r <= rows; r++ {
		for c := 0; c < cols; c++ {
			b := horizontal[r][c]
			if b.width == 0 {
				continue
			}
			half := px(b.width) / 2
			x0, x1 := px(xs[c]), px(xs[c+1])
			if joined(r, c) {
				x0 -= half
			}
			if joined(r, c+1) {
				x1 += half
			}
			if err := segment(b, x0, ys[r]-half, x1, ys[r]+half); err != nil {
				return nil, err
			}
		}
	}
	for r := 0; r < rows; r++ {
		for c := 0; c <= cols; c++ {
			b := vertical[r][c]
			if b.width == 0 {
				continue
			}
			half := px(b.width) / 2
			y0, y1 := ys[r], ys[r+1]
			if joined(r, c) {
				y0 -= half
			}
			if joined(r+1, c) {
				y1 += half
			}
			if err := segment(b, px(xs[c])-half, y0, px(xs[c])+half, y1); err != nil {
				return nil, err
			}
		}
	}
	// Text, anchored in each cell; rows were sized to hold it.
	for r := range cells {
		for _, cl := range cells[r] {
			if len(cl.blocks) == 0 {
				continue
			}
			top := ys[r] + px(cl.margin[1])
			bottom := ys[r+1] - px(cl.margin[3])
			text, err := renderPlaceParagraphs(cl.blocks, cl.height, top, bottom, cl.anchor, true, fonts, styles.colors)
			if err != nil {
				return nil, err
			}
			ops = append(ops, text...)
		}
	}
	return ops, nil
}

func cellBorder(lines [][]renderBorder, r, c int) renderBorder {
	if r < 0 || r >= len(lines) || c < 0 || c >= len(lines[r]) {
		return renderBorder{}
	}
	return lines[r][c]
}

func firstErr(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// renderCellBodyPr reports whether a cell's body properties agree with the
// cell: a table cell takes its margins and anchoring from its cell
// properties, and PowerPoint writes an empty a:bodyPr, so any value set here
// must match what the cell applies.
func renderCellBodyPr(bp *dml.BodyPr, margin [4]dml.EMU, anchor enum.TextAnchor) bool {
	if bp == nil {
		return true
	}
	rest := *bp
	rest.CapturedAttrs = nil
	for i, inset := range []**int64{&rest.LIns, &rest.TIns, &rest.RIns, &rest.BIns} {
		if *inset != nil && dml.EMU(**inset) != margin[i] {
			return false
		}
		*inset = nil
	}
	if (rest.Wrap != "" && rest.Wrap != "square") || (rest.Anchor != "" && enum.TextAnchor(rest.Anchor) != anchor) || rest.NormAutofit != nil || rest.SpAutoFit != nil {
		return false
	}
	rest.Wrap, rest.Anchor, rest.NoAutofit = "", "", nil
	return reflect.DeepEqual(rest, dml.BodyPr{})
}

// renderMetadataExt reports whether a DrawingML extension list holds only
// extensions the source profile admits for its owner.
func renderMetadataExt(l *dml.ExtLst, owner string) bool {
	for _, e := range l.Ext {
		if e == nil || !renderExtensionAllowed(renderExtensions[owner], e.URI) {
			return false
		}
	}
	return true
}
