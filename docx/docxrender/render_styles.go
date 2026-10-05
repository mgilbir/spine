package docxrender

import (
	"fmt"

	"github.com/mgilbir/spine/render"
)

// wordStyle is one w:style, parsed on first use so an unreferenced style the
// profile cannot read never fails a render.
type wordStyle struct {
	id, typ, basedOn string
	isDefault        bool
	node             *wordNode
	parsed           bool
	ppr              wordPPr
	rpr              wordRPr
}

// wordParaLevel is a paragraph style resolved over the document defaults.
type wordParaLevel struct {
	ppr wordPPr
	rpr wordRPr
	// own is the style chain's paragraph formatting without the document
	// defaults, which numbering levels slot in under (see listProps).
	own wordPPr
}

// wordStyles is the document's style sheet. Tables styles keep their nodes for
// the table translator (see wordStyle.node).
type wordStyles struct {
	r        *wordRenderer
	byID     map[string]*wordStyle
	defPara  string
	defChar  string
	docPPr   wordPPr
	docRPr   wordRPr
	paraMemo map[string]*wordParaLevel
	charMemo map[string]*wordRPr
}

// wordMaxStyleDepth bounds a basedOn chain.
const wordMaxStyleDepth = 64

// wordRenderStyles reads a styles part. A nil root is a document without styles.
func (r *wordRenderer) loadStyles(root *wordNode) error {
	s := &wordStyles{r: r, byID: map[string]*wordStyle{}, paraMemo: map[string]*wordParaLevel{}, charMemo: map[string]*wordRPr{}}
	r.styles = s
	if root == nil {
		return nil
	}
	for _, c := range root.children {
		if c.space != nsW {
			continue
		}
		switch c.name {
		case "docDefaults":
			if d := c.child("rPrDefault"); d != nil {
				rpr, err := r.parseRPr(d.child("rPr"))
				if err != nil {
					return err
				}
				s.docRPr = rpr
			}
			if d := c.child("pPrDefault"); d != nil {
				ppr, err := r.parsePPr(d.child("pPr"))
				if err != nil {
					return err
				}
				s.docPPr = ppr
			}
		case "style":
			id, ok := c.attr("styleId")
			if !ok || id == "" {
				return fmt.Errorf("%w: style without styleId", render.ErrInvalid)
			}
			if s.byID[id] != nil {
				return fmt.Errorf("%w: duplicate style %q", render.ErrInvalid, id)
			}
			st := &wordStyle{id: id, typ: attrOrEmpty(c, "type"), node: c}
			if b := c.child("basedOn"); b != nil {
				st.basedOn = b.val()
			}
			if d, ok := c.attr("default"); ok {
				on, err := wordBoolAttr(d)
				if err != nil {
					return err
				}
				st.isDefault = on
			}
			s.byID[id] = st
			if st.isDefault {
				switch st.typ {
				case "paragraph":
					if s.defPara == "" {
						s.defPara = id
					}
				case "character":
					if s.defChar == "" {
						s.defChar = id
					}
				}
			}
		}
	}
	return nil
}

func (s *wordStyles) parse(st *wordStyle) error {
	if st.parsed {
		return nil
	}
	ppr, err := s.r.parsePPr(st.node.child("pPr"))
	if err != nil {
		return err
	}
	rpr, err := s.r.parseRPr(st.node.child("rPr"))
	if err != nil {
		return err
	}
	st.ppr, st.rpr, st.parsed = ppr, rpr, true
	return nil
}

// chain returns the styles from the root of the basedOn chain to id.
func (s *wordStyles) chain(id string) ([]*wordStyle, error) {
	var rev []*wordStyle
	seen := map[string]bool{}
	for id != "" {
		st := s.byID[id]
		if st == nil {
			break
		}
		if seen[id] || len(rev) >= wordMaxStyleDepth {
			return nil, fmt.Errorf("%w: style inheritance", render.ErrInvalid)
		}
		seen[id] = true
		rev = append(rev, st)
		id = st.basedOn
	}
	out := make([]*wordStyle, len(rev))
	for i, st := range rev {
		out[len(rev)-1-i] = st
	}
	return out, nil
}

// paragraph resolves a paragraph style (or the default paragraph style when id
// is empty or unknown) over the document defaults.
func (s *wordStyles) paragraph(id string) (*wordParaLevel, error) {
	if s.byID[id] == nil || s.byID[id].typ != "paragraph" {
		id = s.defPara
	}
	if lv := s.paraMemo[id]; lv != nil {
		return lv, nil
	}
	lv := &wordParaLevel{ppr: s.docPPr, rpr: s.docRPr}
	chain, err := s.chain(id)
	if err != nil {
		return nil, err
	}
	for _, st := range chain {
		if err = s.parse(st); err != nil {
			return nil, err
		}
		lv.ppr = st.ppr.over(lv.ppr)
		lv.own = st.ppr.over(lv.own)
		lv.rpr = st.rpr.over(lv.rpr)
	}
	s.paraMemo[id] = lv
	return lv, nil
}

// character resolves a character style chain (without the defaults). An
// unknown id resolves to nothing.
func (s *wordStyles) character(id string) (wordRPr, error) {
	if id == "" || s.byID[id] == nil {
		return wordRPr{}, nil
	}
	if rp := s.charMemo[id]; rp != nil {
		return *rp, nil
	}
	chain, err := s.chain(id)
	if err != nil {
		return wordRPr{}, err
	}
	var rp wordRPr
	for _, st := range chain {
		if err = s.parse(st); err != nil {
			return wordRPr{}, err
		}
		rp = st.rpr.over(rp)
	}
	s.charMemo[id] = &rp
	return rp, nil
}

// runProps resolves the formatting of a run: the paragraph level (document
// defaults and paragraph style), then the run's character style (toggle
// properties combine by exclusive or), then direct formatting.
func (s *wordStyles) runProps(para wordRPr, direct wordRPr) (wordRPr, error) {
	char, err := s.character(direct.rStyle)
	if err != nil {
		return wordRPr{}, err
	}
	merged := char.over(para)
	merged.bold, merged.italic = wordToggle(para.bold, char.bold), wordToggle(para.italic, char.italic)
	merged.boldCs, merged.italicCs = wordToggle(para.boldCs, char.boldCs), wordToggle(para.italicCs, char.italicCs)
	merged.caps, merged.smallCaps = wordToggle(para.caps, char.caps), wordToggle(para.smallCaps, char.smallCaps)
	merged.strike, merged.dstrike = wordToggle(para.strike, char.strike), wordToggle(para.dstrike, char.dstrike)
	merged.vanish = wordToggle(para.vanish, char.vanish)
	return direct.over(merged), nil
}
