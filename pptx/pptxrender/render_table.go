package pptxrender

import (
	"context"
	"fmt"
	"reflect"
	"sort"

	"github.com/mgilbir/spine/pptx"

	"github.com/mgilbir/forme/layout"
	"github.com/mgilbir/forme/style"
	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/common/enum"
	core "github.com/mgilbir/spine/internal/render"
	"github.com/mgilbir/spine/pptx/internal/oxml"
	"github.com/mgilbir/spine/render"
)

// renderSourceFrame returns the parsed graphic frame behind a shape, or nil.
func (s *renderSlide) renderSourceFrame(index int) *oxml.GraphicFrame {
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
	// double is a compound line of two strokes with a gap, each a third of
	// the width.
	double bool
	// rank orders where borders cross: a cell's own border outranks every
	// style's, and a style's parts rank by precedence.
	rank int
}

// renderExplicitBorder is the rank of a border a cell sets itself.
const renderExplicitBorder = 1000

// renderTable paints a table: its style's background, cell fills, then
// borders, then cell text. The style is one of PowerPoint's built-in table
// styles, which a file does not define; a table with another style is drawn
// without one, as is a table without a style id, since whether PowerPoint
// applies the presentation's default table style to it is undocumented.
//
// Only a parsed table without pending edits is drawn: the save path rewrites
// the domain model's cells, which preparation must not do.
func (s *renderSlide) renderTable(ctx context.Context, index int, t *pptx.Table, colors *renderColors, breaker *core.TextLayout, fonts *slideRenderFonts, styles *renderTextStyles) ([]layout.Op, error) {
	gf := s.renderSourceFrame(index)
	if gf == nil || shapeState(t).Dirty {
		return nil, fmt.Errorf("%w: new or edited table; save and reopen to preview it", render.ErrUnsupported)
	}
	return renderTableFrame(ctx, gf, colors, breaker, fonts, styles)
}

// renderTableFrame paints a parsed table's frame, on a slide, layout or
// master, or in a group with its geometry mapped; see renderTable.
func renderTableFrame(ctx context.Context, gf *oxml.GraphicFrame, colors *renderColors, breaker *core.TextLayout, fonts *slideRenderFonts, styles *renderTextStyles) ([]layout.Op, error) {
	if gf.ExtLst != nil || gf.Xfrm == nil || gf.Xfrm.Off == nil || gf.Graphic == nil || gf.Graphic.GraphicData == nil || gf.Graphic.GraphicData.URI != oxml.TableGraphicDataURI || gf.Graphic.GraphicData.Table == nil {
		return nil, fmt.Errorf("%w: graphic frame", render.ErrUnsupported)
	}
	if gf.Xfrm.Rot != 0 || gf.Xfrm.FlipH || gf.Xfrm.FlipV {
		return nil, fmt.Errorf("%w: table transformation", render.ErrUnsupported)
	}
	tbl := gf.Graphic.GraphicData.Table
	var builtin *dml.TableStyle
	if pr := tbl.TblPr; pr != nil {
		look, err := colors.resolveTableStyle(pr)
		if err != nil {
			return nil, err
		}
		if look.leftOut != nil {
			// Best effort draws a styled table unstyled: with only its cells'
			// own fills, borders and text.
			if err := colors.approximate(look.leftOut); err != nil {
				return nil, err
			}
		}
		if builtin = look.style; builtin != nil && pr.BandCol {
			// Only banded rows were compared with PowerPoint.
			if err := colors.approximate(fmt.Errorf("%w: banded columns of a built-in table style", render.ErrUnsupported)); err != nil {
				return nil, err
			}
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
	var styler *tableStyler
	if builtin != nil {
		styler = newTableStyler(builtin, tbl.TblPr, rows, cols)
	}
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
		// origin is the row and column of the cell a merge covers this one
		// with, itself for a cell no merge covers; rs and cs are an origin
		// cell's row and column spans.
		origin [2]int
		rs, cs int
	}
	cells := make([][]cell, rows)
	// A merge's first cell spans rows and columns; the cells it covers are
	// marked merged and drawn as part of it.
	origin := make([][][2]int, rows)
	for r := range origin {
		origin[r] = make([][2]int, cols)
		for c := range origin[r] {
			origin[r][c] = [2]int{-1, -1}
		}
	}
	for r, tr := range tbl.Tr {
		if tr == nil || len(tr.Tc) != cols {
			return nil, fmt.Errorf("%w: table row", render.ErrInvalid)
		}
		for c, tc := range tr.Tc {
			if tc == nil {
				return nil, fmt.Errorf("%w: table cell", render.ErrInvalid)
			}
			if origin[r][c][0] >= 0 {
				if !tc.HMerge && !tc.VMerge {
					return nil, fmt.Errorf("%w: merged cell not marked merged", render.ErrInvalid)
				}
				continue
			}
			if tc.HMerge || tc.VMerge {
				return nil, fmt.Errorf("%w: merged cell outside a merge", render.ErrInvalid)
			}
			rs, cs := max(int(tc.RowSpan), 1), max(int(tc.GridSpan), 1)
			if r+rs > rows || c+cs > cols {
				return nil, fmt.Errorf("%w: cell span", render.ErrInvalid)
			}
			for dr := range rs {
				for dc := range cs {
					if origin[r+dr][c+dc][0] >= 0 {
						return nil, fmt.Errorf("%w: overlapping cell spans", render.ErrInvalid)
					}
					origin[r+dr][c+dc] = [2]int{r, c}
				}
			}
		}
	}
	// Text of cells spanning rows lays out with its own row; rows then grow
	// so each spanning cell's last row holds what its others do not.
	type tall struct {
		r, rs  int
		height float64
	}
	var talls []tall
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
			if tc.ExtLst != nil {
				return nil, fmt.Errorf("%w: extended table cell", render.ErrUnsupported)
			}
			cl := cell{tc: tc, margin: [4]dml.EMU{91440, 45720, 91440, 45720}, anchor: enum.TextAnchorTop, origin: origin[r][c]}
			if cl.origin != [2]int{r, c} {
				// Drawn as part of its merge.
				cells[r][c] = cl
				continue
			}
			cl.rs, cl.cs = max(int(tc.RowSpan), 1), max(int(tc.GridSpan), 1)
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
			w := xs[c+cl.cs] - xs[c]
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
				chain := styles.shapeChain()
				if styler != nil {
					if ts := styler.text(r, c); ts.bold != nil || ts.color != nil {
						chain.inherited = []*dml.LstStyle{renderTableTextList(ts)}
					}
				}
				blocks, h, err := renderLayoutParagraphs(ctx, body, xs[c]+cl.margin[0], w-cl.margin[0]-cl.margin[2], false, breaker, fonts, styles, chain)
				if err != nil {
					return nil, err
				}
				cl.blocks, cl.height = blocks, h
			}
			need := cl.height + float64(cl.margin[1]+cl.margin[3])/float64(dml.EMUsPerPixel)
			if cl.rs > 1 {
				talls = append(talls, tall{r: r, rs: cl.rs, height: need})
			} else {
				height = max(height, need)
			}
			cells[r][c] = cl
		}
		ys[r+1] = ys[r] + height
		// A spanning cell ending on this row grows it to fit.
		for _, t := range talls {
			if t.r+t.rs-1 == r && ys[r+1]-ys[t.r] < t.height {
				ys[r+1] = ys[t.r] + t.height
			}
		}
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
	// The style's background lies under the cells.
	if builtin != nil && builtin.TblBg != nil {
		x, w := px(xs[0]), px(xs[cols])-px(xs[0])
		bg, err := renderTableBackground(builtin.TblBg, x, ys[0], w, ys[rows]-ys[0], colors)
		if err != nil {
			return nil, err
		}
		ops = append(ops, bg...)
	}
	// Fills. Opaque fills run under the next cell's by half a pixel, so that
	// anti-aliasing does not leave a seam where cells meet at a fraction of a
	// pixel.
	fills := make([][]*style.RGBA, rows)
	for r := range cells {
		fills[r] = make([]*style.RGBA, cols)
		for c, cl := range cells[r] {
			pr := cl.tc.TcPr
			if cl.origin != [2]int{r, c} {
				continue
			}
			if pr != nil && pr.SolidFill != nil && pr.NoFill != nil {
				return nil, fmt.Errorf("%w: ambiguous cell fill", render.ErrInvalid)
			}
			var fill *dml.SolidFill
			switch {
			case pr != nil && pr.SolidFill != nil:
				fill = pr.SolidFill
			case pr != nil && pr.NoFill != nil:
			case styler != nil:
				fill, _ = styler.fill(r, c)
			}
			if fill == nil {
				continue
			}
			color, err := colors.solid(fill, nil)
			if err != nil {
				return nil, err
			}
			fills[r][c] = &color
		}
	}
	opaque := func(r, c int) bool {
		return r < rows && c < cols && fills[r][c] != nil && fills[r][c].A >= 1
	}
	const seam = 0.5
	for r := range cells {
		for c, cl := range cells[r] {
			color := fills[r][c]
			if color == nil {
				continue
			}
			right, bottom := 0.0, 0.0
			if color.A >= 1 {
				if opaque(r, c+cl.cs) {
					right = seam
				}
				if opaque(r+cl.rs, c) {
					bottom = seam
				}
			}
			x, err1 := unit(px(xs[c]))
			y, err2 := unit(ys[r])
			w, err3 := unit(px(xs[c+cl.cs]) - px(xs[c]) + right)
			h, err4 := unit(ys[r+cl.rs] - ys[r] + bottom)
			if err := firstErr(err1, err2, err3, err4); err != nil {
				return nil, err
			}
			ops = append(ops, layout.FillRect{Rect: layout.Rect{X: x, Y: y, W: w, H: h}, Color: *color})
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
			if ln.NoFill == nil && styler != nil {
				// A line with no fill of its own may take the style's; best
				// effort draws no border.
				if err := colors.approximate(fmt.Errorf("%w: cell border without a fill in a styled table", render.ErrUnsupported)); err != nil {
					return renderBorder{}, false, err
				}
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
		return renderBorder{width: dml.EMU(*ln.W), color: c, rank: renderExplicitBorder}, true, err
	}
	// pick resolves the borders two cells set on their shared edge; have is
	// false where neither sets one.
	pick := func(lns ...*dml.Ln) (out renderBorder, have bool, err error) {
		for _, ln := range lns {
			b, set, err := edge(ln)
			if err != nil {
				return renderBorder{}, false, err
			}
			if !set {
				continue
			}
			// Which of two disagreeing cells wins a shared edge is
			// undocumented.
			if have && b != out {
				return renderBorder{}, false, fmt.Errorf("%w: conflicting cell borders", render.ErrUnsupported)
			}
			out, have = b, true
		}
		return out, have, nil
	}
	// styled resolves the style's border on a grid cell's side.
	styled := func(r, c, side int) (renderBorder, bool, error) {
		if styler == nil || r < 0 || r >= rows || c < 0 || c >= cols {
			return renderBorder{}, false, nil
		}
		ln, rank := styler.edge(r, c, side)
		if ln == nil {
			return renderBorder{}, false, nil
		}
		if ln.SolidFill == nil {
			// An absent border, which hides the border of a part below.
			return renderBorder{rank: rank}, true, nil
		}
		if ln.W == nil || *ln.W <= 0 {
			return renderBorder{}, false, fmt.Errorf("%w: table style border width", render.ErrInvalid)
		}
		c0, err := colors.solid(ln.SolidFill, nil)
		return renderBorder{width: dml.EMU(*ln.W), color: c0, double: ln.Cmpd == "dbl", rank: rank}, true, err
	}
	// shared resolves a grid edge: a border a cell sets itself, else the
	// style's border of the cell whose part ranks higher, the first cell's
	// where they rank alike.
	shared := func(r0, c0, side0, r1, c1, side1 int, lns ...*dml.Ln) (renderBorder, error) {
		b, have, err := pick(lns...)
		if err != nil || have {
			return b, err
		}
		first, set0, err := styled(r0, c0, side0)
		if err != nil {
			return renderBorder{}, err
		}
		second, set1, err := styled(r1, c1, side1)
		if err != nil {
			return renderBorder{}, err
		}
		switch {
		case set0 && set1 && second.rank > first.rank:
			return second, nil
		case set0:
			return first, nil
		}
		return second, nil
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
			if r > 0 && r < rows && origin[r-1][c] == origin[r][c] {
				continue // inside a merge
			}
			b, err := shared(r-1, c, sideBottom, r, c, sideTop, lnOf(r-1, c, 3), lnOf(r, c, 1))
			if err != nil {
				return nil, err
			}
			horizontal[r][c] = b
		}
	}
	for r := 0; r < rows; r++ {
		vertical[r] = make([]renderBorder, cols+1)
		for c := 0; c <= cols; c++ {
			if c > 0 && c < cols && origin[r][c-1] == origin[r][c] {
				continue // inside a merge
			}
			b, err := shared(r, c-1, sideRight, r, c, sideLeft, lnOf(r, c-1, 2), lnOf(r, c, 0))
			if err != nil {
				return nil, err
			}
			vertical[r][c] = b
		}
	}
	// Borders meeting at a grid point must agree where a cell sets one of
	// them: which of two differing borders shows is undocumented. A style's
	// borders may differ, and are painted from the lowest precedence up so the
	// highest shows where they cross. A segment is extended half its width
	// where another border meets it, to close the corner.
	mixed := false
	meets := make([][]int, rows+1)
	for r := 0; r <= rows; r++ {
		meets[r] = make([]int, cols+1)
		for c := 0; c <= cols; c++ {
			var met []renderBorder
			for _, b := range []renderBorder{
				cellBorder(horizontal, r, c-1), cellBorder(horizontal, r, c),
				cellBorder(vertical, r-1, c), cellBorder(vertical, r, c),
			} {
				if b.width > 0 {
					met = append(met, b)
				}
			}
			meets[r][c] = len(met)
			explicit := 0
			for _, b := range met {
				if b.rank >= renderExplicitBorder {
					explicit++
				}
			}
			for _, b := range met {
				if b.sameLine(met[0]) {
					continue
				}
				if explicit == len(met) {
					return nil, fmt.Errorf("%w: differing borders meet", render.ErrUnsupported)
				}
				mixed = mixed || explicit > 0
			}
		}
	}
	if mixed {
		if err := colors.approximate(fmt.Errorf("%w: a cell's own border meets a table style's", render.ErrUnsupported)); err != nil {
			return nil, err
		}
	}
	joined := func(r, c int) bool { return meets[r][c] > 1 }
	type stroke struct {
		b              renderBorder
		x0, y0, x1, y1 float64
	}
	var strokes []stroke
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
			strokes = append(strokes, stroke{b, x0, ys[r] - half, x1, ys[r] + half})
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
			strokes = append(strokes, stroke{b, px(xs[c]) - half, y0, px(xs[c]) + half, y1})
		}
	}
	sort.SliceStable(strokes, func(i, j int) bool { return strokes[i].b.rank < strokes[j].b.rank })
	rect := func(b renderBorder, x0, y0, x1, y1 float64) error {
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
	for _, s := range strokes {
		if !s.b.double {
			if err := rect(s.b, s.x0, s.y0, s.x1, s.y1); err != nil {
				return nil, err
			}
			continue
		}
		// A compound line is two strokes of a third of its width, a third
		// apart.
		if s.x1-s.x0 > s.y1-s.y0 {
			t := (s.y1 - s.y0) / 3
			err := firstErr(rect(s.b, s.x0, s.y0, s.x1, s.y0+t), rect(s.b, s.x0, s.y1-t, s.x1, s.y1))
			if err != nil {
				return nil, err
			}
			continue
		}
		t := (s.x1 - s.x0) / 3
		if err := firstErr(rect(s.b, s.x0, s.y0, s.x0+t, s.y1), rect(s.b, s.x1-t, s.y0, s.x1, s.y1)); err != nil {
			return nil, err
		}
	}
	// Text, anchored in each cell; rows were sized to hold it.
	for r := range cells {
		for _, cl := range cells[r] {
			if len(cl.blocks) == 0 {
				continue
			}
			top := ys[r] + px(cl.margin[1])
			bottom := ys[r+cl.rs] - px(cl.margin[3])
			text, err := renderPlaceParagraphs(cl.blocks, cl.height, top, bottom, cl.anchor, true, renderColumns{}, fonts, styles.colors)
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

// sameLine reports whether two borders look alike: rank only orders them.
func (b renderBorder) sameLine(o renderBorder) bool {
	return b.width == o.width && b.color == o.color && b.double == o.double
}

// renderTableTextList is a list style whose every level carries a table
// style's text properties, to sit below a cell's own text properties.
func renderTableTextList(ts textStyle) *dml.LstStyle {
	lvl := func() *dml.PPr {
		return &dml.PPr{DefRPr: &dml.RPr{B: ts.bold, SolidFill: ts.color}}
	}
	return &dml.LstStyle{
		Lvl1pPr: lvl(), Lvl2pPr: lvl(), Lvl3pPr: lvl(), Lvl4pPr: lvl(), Lvl5pPr: lvl(),
		Lvl6pPr: lvl(), Lvl7pPr: lvl(), Lvl8pPr: lvl(), Lvl9pPr: lvl(),
	}
}

// renderTableBackground draws a table style's background over the box the
// table fills, in pixels: its fill from the theme, and the effects the theme
// gives it, such as a shadow.
func renderTableBackground(bg *dml.TableBgStyle, x, y, w, h float64, colors *renderColors) ([]layout.Op, error) {
	var ops []layout.Op
	fill, grad, placeholder, err := renderStyleFill(bg.FillRef, colors)
	if err != nil {
		return nil, err
	}
	if fill != nil || grad != nil {
		paint, err := colors.fillPaint(fill, grad, placeholder, w, h)
		if err != nil {
			return nil, err
		}
		if ops, err = paint.fillOps(x, y, w, h, nil); err != nil {
			return nil, err
		}
	}
	if bg.EffectRef == nil || len(ops) == 0 {
		return ops, nil
	}
	return renderShapeEffects(ops, nil, false, false, &dml.Style{EffectRef: bg.EffectRef}, colors, colors.limits.MaxOperations)
}
