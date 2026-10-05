package docxrender

import (
	"fmt"

	"github.com/mgilbir/spine/render"
)

// Table styles.
//
// A table style is a whole-table layer and conditional layers (first row, last
// column, banded rows, corner cells, ...). Each layer holds paragraph and run
// formatting and the table and cell properties. A cell takes the layers that
// apply to its position, in the order ECMA-376 17.7.6 gives (whole table, banded
// columns, banded rows, first and last column, first and last row, corner
// cells), each over the one before, all beneath the table's own properties and
// the cell's: document defaults, table style, paragraph style, direct formatting.

// Layer names, in the order they are applied.
const (
	wordLayerWhole = iota
	wordLayerBand1Vert
	wordLayerBand2Vert
	wordLayerBand1Horz
	wordLayerBand2Horz
	wordLayerFirstCol
	wordLayerLastCol
	wordLayerFirstRow
	wordLayerLastRow
	wordLayerNW
	wordLayerNE
	wordLayerSW
	wordLayerSE
	wordLayers
)

var wordLayerTypes = map[string]int{
	"wholeTable": wordLayerWhole, "band1Vert": wordLayerBand1Vert, "band2Vert": wordLayerBand2Vert,
	"band1Horz": wordLayerBand1Horz, "band2Horz": wordLayerBand2Horz, "firstCol": wordLayerFirstCol,
	"lastCol": wordLayerLastCol, "firstRow": wordLayerFirstRow, "lastRow": wordLayerLastRow,
	"nwCell": wordLayerNW, "neCell": wordLayerNE, "swCell": wordLayerSW, "seCell": wordLayerSE,
}

// wordTblLayer is one layer of a table style.
type wordTblLayer struct {
	set bool
	ppr wordPPr
	rpr wordRPr
	tbl wordTblPr
	tc  wordTcPr
}

// over returns l with every unset property taken from base.
func (l wordTblLayer) over(base wordTblLayer) wordTblLayer {
	if !base.set {
		return l
	}
	if !l.set {
		return base
	}
	return wordTblLayer{set: true, ppr: l.ppr.over(base.ppr), rpr: l.rpr.over(base.rpr), tbl: l.tbl.over(base.tbl), tc: l.tc.over(base.tc)}
}

// wordTableStyle is a table style resolved along its basedOn chain.
type wordTableStyle struct {
	layers [wordLayers]wordTblLayer
}

// parseLayer reads the formatting children of a style or a w:tblStylePr.
func (r *wordRenderer) parseLayer(n *wordNode) (wordTblLayer, error) {
	l := wordTblLayer{set: true}
	var err error
	if l.ppr, err = r.parsePPr(n.child("pPr")); err != nil {
		return l, err
	}
	if l.rpr, err = r.parseRPr(n.child("rPr")); err != nil {
		return l, err
	}
	if l.tbl, err = r.parseTblPr(n.child("tblPr")); err != nil {
		return l, err
	}
	l.tc, err = r.parseTcPr(n.child("tcPr"))
	return l, err
}

// table resolves a table style, or the default table style when id is empty or
// unknown. Styles the document does not use are never read. The result is nil
// when the document has no table style at all.
func (s *wordStyles) table(id string) (*wordTableStyle, error) {
	if st := s.byID[id]; st == nil || st.typ != "table" {
		id = s.defTable
	}
	if id == "" {
		return nil, nil
	}
	if ts := s.tables[id]; ts != nil {
		return ts, nil
	}
	chain, err := s.chain(id)
	if err != nil {
		return nil, err
	}
	ts := &wordTableStyle{}
	for _, st := range chain {
		if st.typ != "table" {
			continue
		}
		whole, err := s.r.parseLayer(st.node)
		if err != nil {
			return nil, err
		}
		ts.layers[wordLayerWhole] = whole.over(ts.layers[wordLayerWhole])
		for _, c := range st.node.children {
			if !c.is("tblStylePr") {
				continue
			}
			idx, ok := wordLayerTypes[c.attrOr("type")]
			if !ok {
				return nil, fmt.Errorf("%w: w:tblStylePr type", render.ErrInvalid)
			}
			l, err := s.r.parseLayer(c)
			if err != nil {
				return nil, err
			}
			ts.layers[idx] = l.over(ts.layers[idx])
		}
	}
	s.tables[id] = ts
	return ts, nil
}
