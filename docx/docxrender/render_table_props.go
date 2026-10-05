package docxrender

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/mgilbir/spine/render"
)

// Table properties, as one cascade level (a table style layer, the table
// itself, a row exception or a cell) states them. Unset fields inherit.

// Border sides of a table, in the order of wordTblPr.borders.
const (
	wordTop = iota
	wordLeft
	wordBottom
	wordRight
	wordInsideH
	wordInsideV
	wordBorderSides
)

// wordBorder is one border line: a table, row or cell border at one side.
type wordBorder struct {
	none bool
	// style is the CSS border-style that draws it.
	style string
	// width is the width of the line in pixels, which is the CSS border width
	// (a double line is three times the width of each of its lines).
	width float64
	color wordRGB
	// lines weighs the border in conflicts: 2 for a double line.
	lines int
}

// wordBorderWeight orders borders in a conflict: the wider and the more lined.
func (b wordBorder) weight() float64 {
	if b.none {
		return 0
	}
	return b.width * float64(max(b.lines, 1))
}

// wordTblW is a width: pixels (dxa), a fraction of the available width (pct)
// or automatic.
type wordTblW struct {
	typ string // dxa, pct, auto
	v   float64
}

// wordTblLook says which conditional formats of a table style apply.
type wordTblLook struct {
	firstRow, lastRow, firstCol, lastCol, noHBand, noVBand bool
}

// wordTblPr is a w:tblPr (or w:tblPrEx) level.
type wordTblPr struct {
	style string
	width wordOpt[wordTblW]
	jc    wordOpt[string]
	ind   wordOpt[float64]
	// borders holds the six table borders (wordTop ... wordInsideV).
	borders [wordBorderSides]wordOpt[wordBorder]
	shd     wordOpt[wordShading]
	layout  wordOpt[string]
	// mar is the default cell margin, top, left, bottom, right in pixels.
	mar              [4]wordOpt[float64]
	look             wordOpt[wordTblLook]
	rowBand, colBand wordOpt[int]
	spacing          wordOpt[float64]
	floating, bidi   bool

	issues []wordIssue
}

// over returns t with every unset field taken from base.
func (t wordTblPr) over(base wordTblPr) wordTblPr {
	out := t
	if out.style == "" {
		out.style = base.style
	}
	out.width, out.jc, out.ind = t.width.or(base.width), t.jc.or(base.jc), t.ind.or(base.ind)
	for i := range out.borders {
		out.borders[i] = t.borders[i].or(base.borders[i])
	}
	for i := range out.mar {
		out.mar[i] = t.mar[i].or(base.mar[i])
	}
	out.shd, out.layout, out.look = t.shd.or(base.shd), t.layout.or(base.layout), t.look.or(base.look)
	out.rowBand, out.colBand, out.spacing = t.rowBand.or(base.rowBand), t.colBand.or(base.colBand), t.spacing.or(base.spacing)
	out.floating, out.bidi = t.floating || base.floating, t.bidi || base.bidi
	out.issues = nil
	for _, i := range base.issues {
		out.issues = wordAddIssue(out.issues, i)
	}
	for _, i := range t.issues {
		out.issues = wordAddIssue(out.issues, i)
	}
	return out
}

// wordTrPr is a w:trPr.
type wordTrPr struct {
	cantSplit, header, hidden bool
	height                    float64
	// rule is the height rule: atLeast, exact or auto (no height).
	rule                  string
	gridBefore, gridAfter int
	issues                []wordIssue
}

// wordTcPr is a w:tcPr level.
type wordTcPr struct {
	width     wordOpt[wordTblW]
	span      int
	hMerge    string // "", restart or continue
	vMerge    string // "", restart or continue
	borders   [wordBorderSides]wordOpt[wordBorder]
	shd       wordOpt[wordShading]
	mar       [4]wordOpt[float64]
	vAlign    wordOpt[string]
	noWrap    bool
	issues    []wordIssue
	hasBorder bool
}

// over returns c with every unset field taken from base. Merge and span are the
// cell's own.
func (c wordTcPr) over(base wordTcPr) wordTcPr {
	out := c
	out.width = c.width.or(base.width)
	for i := range out.borders {
		out.borders[i] = c.borders[i].or(base.borders[i])
	}
	for i := range out.mar {
		out.mar[i] = c.mar[i].or(base.mar[i])
	}
	out.shd, out.vAlign = c.shd.or(base.shd), c.vAlign.or(base.vAlign)
	out.noWrap = c.noWrap || base.noWrap
	out.issues = nil
	for _, i := range base.issues {
		out.issues = wordAddIssue(out.issues, i)
	}
	for _, i := range c.issues {
		out.issues = wordAddIssue(out.issues, i)
	}
	return out
}

// wordBorderStyles maps the w:val of a border to the CSS style that draws it;
// exact says Word's line is drawn as it is.
var wordBorderStyles = map[string]struct {
	css   string
	lines int
	exact bool
}{
	"single": {"solid", 1, true},
	"double": {"double", 2, true},
	"dotted": {"dotted", 1, true},
	"dashed": {"dashed", 1, true},
	"thick":  {"solid", 1, false},
}

// parseBorder reads a w:top, w:left, ... border element.
func (r *wordRenderer) parseBorder(n *wordNode, issue func(wordIssue)) (wordBorder, error) {
	val := n.val()
	if val == "" {
		return wordBorder{}, fmt.Errorf("%w: w:%s", render.ErrInvalid, n.name)
	}
	if val == "nil" || val == "none" {
		return wordBorder{none: true}, nil
	}
	b := wordBorder{style: "solid", lines: 1}
	if st, ok := wordBorderStyles[val]; ok {
		b.style, b.lines = st.css, st.lines
		if !st.exact {
			issue(wordIssue{wordApproximate, "table border style " + val})
		}
	} else {
		issue(wordIssue{wordApproximate, "table border style " + val})
	}
	sz := 4
	if v, ok := n.attr("sz"); ok {
		var valid bool
		if sz, valid = wordRenderInt(v); !valid || sz < 0 {
			return b, fmt.Errorf("%w: w:%s sz", render.ErrInvalid, n.name)
		}
	}
	// Eighths of a point, from a quarter point to six points.
	sz = max(2, min(sz, 96))
	b.width = float64(sz) / 6
	if b.style == "double" {
		b.width *= 3
	}
	if _, ok := n.attr("color"); ok || n.attrOr("themeColor") != "" {
		col, err := r.parseColor(n, "color")
		if err != nil {
			return b, err
		}
		if !col.auto {
			b.color = col.rgb
		}
	}
	if v, ok := n.attr("shadow"); ok {
		if on, e := wordBoolAttr(v); e != nil {
			return b, e
		} else if on {
			issue(wordIssue{wordApproximate, "table border shadow"})
		}
	}
	return b, nil
}

// parseBorders reads the border children of w:tblBorders or w:tcBorders.
func (r *wordRenderer) parseBorders(n *wordNode, dst *[wordBorderSides]wordOpt[wordBorder], issue func(wordIssue)) error {
	for _, c := range n.children {
		if c.space != nsW {
			issue(wordIssue{wordLeaveOut, "table border " + c.name})
			continue
		}
		var side int
		switch c.name {
		case "top":
			side = wordTop
		case "left", "start":
			side = wordLeft
		case "bottom":
			side = wordBottom
		case "right", "end":
			side = wordRight
		case "insideH":
			side = wordInsideH
		case "insideV":
			side = wordInsideV
		case "tl2br", "tr2bl":
			b, err := r.parseBorder(c, issue)
			if err != nil {
				return err
			}
			if !b.none {
				issue(wordIssue{wordLeaveOut, "diagonal cell borders"})
			}
			continue
		default:
			issue(wordIssue{wordLeaveOut, "table border w:" + c.name})
			continue
		}
		b, err := r.parseBorder(c, issue)
		if err != nil {
			return err
		}
		dst[side] = wordSome(b)
	}
	return nil
}

// parseTblW reads a w:tblW or w:tcW.
func wordParseTblW(n *wordNode) (wordTblW, error) {
	typ := n.attrOr("type")
	if typ == "" {
		typ = "dxa"
	}
	w, has := n.attr("w")
	switch typ {
	case "nil", "auto":
		return wordTblW{typ: "auto"}, nil
	case "dxa":
		if !has {
			return wordTblW{typ: "auto"}, nil
		}
		px, ok := wordRenderTwips(w)
		if !ok || math.Abs(px) > wordMaxLength {
			return wordTblW{}, fmt.Errorf("%w: w:%s", render.ErrInvalid, n.name)
		}
		return wordTblW{typ: "dxa", v: px}, nil
	case "pct":
		if !has {
			return wordTblW{typ: "auto"}, nil
		}
		// Fiftieths of a percent, or a number with a percent sign.
		s := strings.TrimSpace(w)
		div := 5000.0
		if t, found := strings.CutSuffix(s, "%"); found {
			s, div = t, 100
		}
		f, err := strconv.ParseFloat(s, 64)
		if err != nil || math.IsNaN(f) || math.IsInf(f, 0) || f < 0 || f > 1e6 {
			return wordTblW{}, fmt.Errorf("%w: w:%s", render.ErrInvalid, n.name)
		}
		return wordTblW{typ: "pct", v: f / div}, nil
	}
	return wordTblW{}, fmt.Errorf("%w: w:%s type", render.ErrInvalid, n.name)
}

// parseMargins reads w:tblCellMar or w:tcMar into top, left, bottom, right.
func wordParseMargins(n *wordNode, dst *[4]wordOpt[float64]) error {
	for _, c := range n.children {
		if c.space != nsW {
			continue
		}
		side := -1
		switch c.name {
		case "top":
			side = 0
		case "left", "start":
			side = 1
		case "bottom":
			side = 2
		case "right", "end":
			side = 3
		}
		if side < 0 {
			continue
		}
		w := c.attrOr("w")
		if t := c.attrOr("type"); t != "" && t != "dxa" {
			if t == "nil" {
				dst[side] = wordSome(0.0)
				continue
			}
			return fmt.Errorf("%w: cell margin type", render.ErrInvalid)
		}
		px, err := wordLength(w, "cell margin")
		if err != nil {
			return err
		}
		dst[side] = wordSome(math.Max(px, 0))
	}
	return nil
}

// parseTblPr reads a w:tblPr, w:tblPrEx or the table properties of a style. A
// nil node is the empty level.
func (r *wordRenderer) parseTblPr(n *wordNode) (wordTblPr, error) {
	var t wordTblPr
	if n == nil {
		return t, nil
	}
	issue := func(i wordIssue) { t.issues = wordAddIssue(t.issues, i) }
	for _, c := range n.children {
		if c.space != nsW {
			issue(wordIssue{wordLeaveOut, "table property " + c.name})
			continue
		}
		var err error
		switch c.name {
		case "tblStyle":
			t.style = c.val()
		case "tblpPr":
			t.floating = true
		case "bidiVisual":
			var on bool
			if on, err = wordOnOff(c, "w:bidiVisual"); on {
				t.bidi = true
			}
		case "tblW":
			var w wordTblW
			if w, err = wordParseTblW(c); err == nil {
				t.width = wordSome(w)
			}
		case "jc":
			switch v := c.val(); v {
			case "left", "start", "center", "right", "end":
				t.jc = wordSome(v)
			default:
				err = fmt.Errorf("%w: table w:jc", render.ErrInvalid)
			}
		case "tblInd":
			var w wordTblW
			if w, err = wordParseTblW(c); err == nil {
				switch w.typ {
				case "dxa":
					t.ind = wordSome(w.v)
				case "pct":
					issue(wordIssue{wordApproximate, "table indent in percent"})
				}
			}
		case "tblBorders":
			err = r.parseBorders(c, &t.borders, issue)
		case "shd":
			s, ok, e := r.parseShading(c)
			if e != nil {
				return t, e
			}
			if !ok {
				issue(wordIssue{wordApproximate, "table shading pattern"})
			} else {
				t.shd = wordSome(s)
			}
		case "tblLayout":
			switch v := c.attrOr("type"); v {
			case "fixed", "autofit":
				t.layout = wordSome(v)
			case "":
				t.layout = wordSome("autofit")
			default:
				err = fmt.Errorf("%w: w:tblLayout", render.ErrInvalid)
			}
		case "tblCellMar":
			err = wordParseMargins(c, &t.mar)
		case "tblCellSpacing":
			var w wordTblW
			if w, err = wordParseTblW(c); err == nil && w.typ == "dxa" && w.v > 0 {
				t.spacing = wordSome(w.v)
				issue(wordIssue{wordApproximate, "table cell spacing (drawn without)"})
			}
		case "tblLook":
			t.look = wordSome(wordParseLook(c))
		case "tblStyleRowBandSize", "tblStyleColBandSize":
			v, ok := wordRenderInt(c.val())
			if !ok || v < 1 || v > 1<<16 {
				return t, fmt.Errorf("%w: w:%s", render.ErrInvalid, c.name)
			}
			if c.name == "tblStyleRowBandSize" {
				t.rowBand = wordSome(v)
			} else {
				t.colBand = wordSome(v)
			}
		case "tblOverlap", "tblCaption", "tblDescription":
			// No effect on a drawn page.
		case "tblPrChange":
			issue(wordIssue{wordApproximate, "tracked formatting change"})
		default:
			issue(wordIssue{wordLeaveOut, "table property w:" + c.name})
		}
		if err != nil {
			return t, err
		}
	}
	return t, nil
}

// wordParseLook reads w:tblLook, from its attributes or its bit mask.
func wordParseLook(n *wordNode) wordTblLook {
	var l wordTblLook
	if v, ok := n.attr("val"); ok {
		if m, err := strconv.ParseUint(strings.TrimSpace(v), 16, 32); err == nil {
			l = wordTblLook{firstRow: m&0x20 != 0, lastRow: m&0x40 != 0, firstCol: m&0x80 != 0,
				lastCol: m&0x100 != 0, noHBand: m&0x200 != 0, noVBand: m&0x400 != 0}
		}
	}
	for _, a := range []struct {
		name string
		dst  *bool
	}{{"firstRow", &l.firstRow}, {"lastRow", &l.lastRow}, {"firstColumn", &l.firstCol}, {"lastColumn", &l.lastCol},
		{"noHBand", &l.noHBand}, {"noVBand", &l.noVBand}} {
		if v, ok := n.attr(a.name); ok {
			if on, err := wordBoolAttr(v); err == nil {
				*a.dst = on
			}
		}
	}
	return l
}

// parseTrPr reads a w:trPr.
func (r *wordRenderer) parseTrPr(n *wordNode) (wordTrPr, error) {
	tr := wordTrPr{rule: "auto"}
	if n == nil {
		return tr, nil
	}
	issue := func(i wordIssue) { tr.issues = wordAddIssue(tr.issues, i) }
	for _, c := range n.children {
		if c.space != nsW {
			issue(wordIssue{wordLeaveOut, "row property " + c.name})
			continue
		}
		var err error
		switch c.name {
		case "cantSplit":
			tr.cantSplit, err = wordOnOff(c, "w:cantSplit")
		case "tblHeader":
			tr.header, err = wordOnOff(c, "w:tblHeader")
		case "hidden":
			tr.hidden, err = wordOnOff(c, "w:hidden")
		case "trHeight":
			tr.height, err = wordLength(c.val(), "w:trHeight")
			if err == nil {
				tr.height = math.Max(tr.height, 0)
				switch rule := c.attrOr("hRule"); rule {
				case "", "atLeast":
					tr.rule = "atLeast"
				case "exact", "auto":
					tr.rule = rule
				default:
					err = fmt.Errorf("%w: w:trHeight hRule", render.ErrInvalid)
				}
			}
		case "gridBefore", "gridAfter":
			v, ok := wordRenderInt(c.val())
			if !ok || v < 0 || v > wordMaxGridCols {
				return tr, fmt.Errorf("%w: w:%s", render.ErrInvalid, c.name)
			}
			if c.name == "gridBefore" {
				tr.gridBefore = v
			} else {
				tr.gridAfter = v
			}
		case "wBefore", "wAfter", "cnfStyle", "divId":
			// The grid says the same; cnfStyle is computed from the position.
		case "tblCellSpacing":
			if w, e := wordParseTblW(c); e != nil {
				return tr, e
			} else if w.typ == "dxa" && w.v > 0 {
				issue(wordIssue{wordApproximate, "table cell spacing"})
			}
		case "jc":
			issue(wordIssue{wordApproximate, "row alignment"})
		case "ins", "del", "trPrChange":
			issue(wordIssue{wordApproximate, "tracked row change"})
		default:
			issue(wordIssue{wordLeaveOut, "row property w:" + c.name})
		}
		if err != nil {
			return tr, err
		}
	}
	return tr, nil
}

// parseTcPr reads a w:tcPr, or the cell properties of a table style layer.
func (r *wordRenderer) parseTcPr(n *wordNode) (wordTcPr, error) {
	tc := wordTcPr{span: 1}
	if n == nil {
		return tc, nil
	}
	issue := func(i wordIssue) { tc.issues = wordAddIssue(tc.issues, i) }
	for _, c := range n.children {
		if c.space != nsW {
			issue(wordIssue{wordLeaveOut, "cell property " + c.name})
			continue
		}
		var err error
		switch c.name {
		case "tcW":
			var w wordTblW
			if w, err = wordParseTblW(c); err == nil {
				tc.width = wordSome(w)
			}
		case "gridSpan":
			v, ok := wordRenderInt(c.val())
			if !ok || v < 1 || v > wordMaxGridCols {
				return tc, fmt.Errorf("%w: w:gridSpan", render.ErrInvalid)
			}
			tc.span = v
		case "hMerge", "vMerge":
			v := c.val()
			if v == "" {
				v = "continue"
			}
			if v != "restart" && v != "continue" {
				return tc, fmt.Errorf("%w: w:%s", render.ErrInvalid, c.name)
			}
			if c.name == "hMerge" {
				tc.hMerge = v
			} else {
				tc.vMerge = v
			}
		case "tcBorders":
			tc.hasBorder = true
			err = r.parseBorders(c, &tc.borders, issue)
		case "shd":
			s, ok, e := r.parseShading(c)
			if e != nil {
				return tc, e
			}
			if !ok {
				issue(wordIssue{wordApproximate, "cell shading pattern"})
			} else {
				tc.shd = wordSome(s)
			}
		case "tcMar":
			err = wordParseMargins(c, &tc.mar)
		case "vAlign":
			switch v := c.val(); v {
			case "top", "center", "bottom":
				tc.vAlign = wordSome(v)
			case "both":
				issue(wordIssue{wordApproximate, "justified vertical cell alignment"})
				tc.vAlign = wordSome("center")
			default:
				err = fmt.Errorf("%w: cell w:vAlign", render.ErrInvalid)
			}
		case "noWrap":
			tc.noWrap, err = wordOnOff(c, "w:noWrap")
		case "textDirection":
			if v := c.val(); v != "lrTb" && v != "lr" {
				issue(wordIssue{wordApproximate, "cell text direction"})
			}
		case "tcFitText":
			if on, e := wordOnOff(c, "w:tcFitText"); e != nil {
				return tc, e
			} else if on {
				issue(wordIssue{wordApproximate, "cell text fitting"})
			}
		case "hideMark", "cnfStyle", "headers":
			// No effect on a drawn page.
		case "cellIns", "cellDel", "cellMerge", "tcPrChange":
			issue(wordIssue{wordApproximate, "tracked cell change"})
		default:
			issue(wordIssue{wordLeaveOut, "cell property w:" + c.name})
		}
		if err != nil {
			return tc, err
		}
	}
	return tc, nil
}
