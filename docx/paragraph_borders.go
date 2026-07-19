package docx

import (
	"github.com/mgilbir/spine/docx/internal/oxml"
)

// ParagraphBorders describes the borders drawn around a paragraph (w:pBdr).
// Each side is nil when that edge has no border. Between is the border drawn
// between consecutive paragraphs that share the same border settings, and Bar is
// the border to the left of the text.
type ParagraphBorders struct {
	Top, Left, Bottom, Right, Between, Bar *Border
}

// Borders returns the paragraph's borders (w:pBdr) and whether the element is
// present.
func (p *Paragraph) Borders() (ParagraphBorders, bool) {
	if p.p.PPr == nil || p.p.PPr.PBdr == nil {
		return ParagraphBorders{}, false
	}
	b := p.p.PPr.PBdr
	return ParagraphBorders{
		Top:     oxmlToBorder(b.Top),
		Left:    oxmlToBorder(b.Left),
		Bottom:  oxmlToBorder(b.Bottom),
		Right:   oxmlToBorder(b.Right),
		Between: oxmlToBorder(b.Between),
		Bar:     oxmlToBorder(b.Bar),
	}, true
}

// SetBorders sets the paragraph's borders (w:pBdr), replacing any existing
// element.
func (p *Paragraph) SetBorders(b ParagraphBorders) {
	p.ensurePPr()
	p.p.PPr.PBdr = &oxml.CT_PBdr{
		Top:     borderToOxml(b.Top),
		Left:    borderToOxml(b.Left),
		Bottom:  borderToOxml(b.Bottom),
		Right:   borderToOxml(b.Right),
		Between: borderToOxml(b.Between),
		Bar:     borderToOxml(b.Bar),
	}
}

// ClearBorders removes the paragraph's w:pBdr element.
func (p *Paragraph) ClearBorders() {
	if p.p.PPr != nil {
		p.p.PPr.PBdr = nil
	}
}

// Shading returns the paragraph's background fill color (w:pPr/w:shd@w:fill), or
// "" when unset.
func (p *Paragraph) Shading() string {
	if p.p.PPr == nil || p.p.PPr.Shd == nil {
		return ""
	}
	return p.p.PPr.Shd.Fill
}

// SetShading sets the paragraph's background fill to the given hex color
// (w:pPr/w:shd with w:val="clear"). Passing "" removes the shading.
func (p *Paragraph) SetShading(hexColor string) {
	if hexColor == "" {
		p.ClearShading()
		return
	}
	p.ensurePPr()
	p.p.PPr.Shd = &oxml.CT_Shd{Val: "clear", Fill: hexColor}
}

// ClearShading removes the paragraph's w:shd element.
func (p *Paragraph) ClearShading() {
	if p.p.PPr != nil {
		p.p.PPr.Shd = nil
	}
}
