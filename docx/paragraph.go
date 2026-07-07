package docx

import (
	"math"
	"strconv"

	"github.com/mgilbir/spine/docx/internal/oxml"
)

// Paragraph represents a paragraph in a Word document.
type Paragraph struct {
	document *Document
	p        *oxml.CT_P
}

// Text returns the text content of the paragraph, including text nested in
// hyperlinks, simple fields, tracked insertions, and structured document tags.
func (p *Paragraph) Text() string {
	return p.p.Text()
}

// SetText sets the text content, replacing all runs.
func (p *Paragraph) SetText(text string) {
	p.p.R = []*oxml.CT_R{{
		T: []*oxml.CT_Text{{Space: "preserve", Text: text}},
	}}
}

// Runs returns all runs in the paragraph.
func (p *Paragraph) Runs() []*Run {
	runs := make([]*Run, len(p.p.R))
	for i, r := range p.p.R {
		runs[i] = &Run{paragraph: p, r: r}
	}
	return runs
}

// AddRun adds a new run to the paragraph.
func (p *Paragraph) AddRun() *Run {
	r := &oxml.CT_R{}
	p.p.R = append(p.p.R, r)
	return &Run{paragraph: p, r: r}
}

// Style returns the paragraph style name.
func (p *Paragraph) Style() string {
	if p.p.PPr != nil && p.p.PPr.PStyle != nil {
		return p.p.PPr.PStyle.Val
	}
	return ""
}

// SetStyle sets the paragraph style.
func (p *Paragraph) SetStyle(style string) {
	if p.p.PPr == nil {
		p.p.PPr = &oxml.CT_PPr{}
	}
	p.p.PPr.PStyle = &oxml.CT_String{Val: style}
}

// Alignment returns the paragraph alignment.
func (p *Paragraph) Alignment() Alignment {
	if p.p.PPr != nil && p.p.PPr.Jc != nil {
		switch p.p.PPr.Jc.Val {
		case "center":
			return AlignmentCenter
		case "right":
			return AlignmentRight
		case "both":
			return AlignmentJustify
		}
	}
	return AlignmentLeft
}

// SetAlignment sets the paragraph alignment.
func (p *Paragraph) SetAlignment(align Alignment) {
	if p.p.PPr == nil {
		p.p.PPr = &oxml.CT_PPr{}
	}
	val := "left"
	switch align {
	case AlignmentCenter:
		val = "center"
	case AlignmentRight:
		val = "right"
	case AlignmentJustify:
		val = "both"
	}
	p.p.PPr.Jc = &oxml.CT_Jc{Val: val}
}

// Clear removes all runs from the paragraph.
func (p *Paragraph) Clear() {
	p.p.R = nil
}

// --- Spacing ---

// SpaceBefore returns the spacing before the paragraph in points.
func (p *Paragraph) SpaceBefore() float64 {
	if p.p.PPr != nil && p.p.PPr.Spacing != nil && p.p.PPr.Spacing.Before != "" {
		return twipsToPoints(p.p.PPr.Spacing.Before)
	}
	return 0
}

// SetSpaceBefore sets the spacing before the paragraph in points.
func (p *Paragraph) SetSpaceBefore(points float64) {
	p.ensureSpacing().Before = pointsToTwips(points)
}

// SpaceAfter returns the spacing after the paragraph in points.
func (p *Paragraph) SpaceAfter() float64 {
	if p.p.PPr != nil && p.p.PPr.Spacing != nil && p.p.PPr.Spacing.After != "" {
		return twipsToPoints(p.p.PPr.Spacing.After)
	}
	return 0
}

// SetSpaceAfter sets the spacing after the paragraph in points.
func (p *Paragraph) SetSpaceAfter(points float64) {
	p.ensureSpacing().After = pointsToTwips(points)
}

// SetLineSpacing sets proportional line spacing. 1.0 = single, 1.5, 2.0, etc.
// Internally this uses lineRule="auto" with the value in 240ths of a line.
func (p *Paragraph) SetLineSpacing(multiplier float64) {
	sp := p.ensureSpacing()
	// Word stores auto line spacing as 240 * multiplier
	sp.Line = strconv.Itoa(int(math.Round(multiplier * 240)))
	sp.LineRule = "auto"
}

// SetLineSpacingExact sets exact line spacing in points.
func (p *Paragraph) SetLineSpacingExact(points float64) {
	sp := p.ensureSpacing()
	sp.Line = pointsToTwips(points)
	sp.LineRule = "exact"
}

// --- Indentation ---

// SetIndentLeft sets the left indentation in points.
func (p *Paragraph) SetIndentLeft(points float64) {
	p.ensureInd().Left = pointsToTwips(points)
}

// SetIndentRight sets the right indentation in points.
func (p *Paragraph) SetIndentRight(points float64) {
	p.ensureInd().Right = pointsToTwips(points)
}

// SetIndentFirstLine sets the first-line indent in points.
func (p *Paragraph) SetIndentFirstLine(points float64) {
	ind := p.ensureInd()
	ind.FirstLine = pointsToTwips(points)
	ind.Hanging = "" // mutually exclusive with hanging
}

// SetIndentHanging sets the hanging indent in points.
func (p *Paragraph) SetIndentHanging(points float64) {
	ind := p.ensureInd()
	ind.Hanging = pointsToTwips(points)
	ind.FirstLine = "" // mutually exclusive with first-line
}

// --- Flow Control ---

// SetKeepWithNext sets whether the paragraph should be kept with the next paragraph.
func (p *Paragraph) SetKeepWithNext(keep bool) {
	p.ensurePPr()
	if keep {
		p.p.PPr.KeepNext = &oxml.CT_OnOff{}
	} else {
		p.p.PPr.KeepNext = nil
	}
}

// SetKeepTogether sets whether the paragraph lines should be kept together on one page.
func (p *Paragraph) SetKeepTogether(keep bool) {
	p.ensurePPr()
	if keep {
		p.p.PPr.KeepLines = &oxml.CT_OnOff{}
	} else {
		p.p.PPr.KeepLines = nil
	}
}

// SetPageBreakBefore sets whether a page break should occur before the paragraph.
func (p *Paragraph) SetPageBreakBefore(brk bool) {
	p.ensurePPr()
	if brk {
		p.p.PPr.PageBreakBefore = &oxml.CT_OnOff{}
	} else {
		p.p.PPr.PageBreakBefore = nil
	}
}

// --- helpers ---

func (p *Paragraph) ensurePPr() {
	if p.p.PPr == nil {
		p.p.PPr = &oxml.CT_PPr{}
	}
}

func (p *Paragraph) ensureSpacing() *oxml.CT_Spacing {
	p.ensurePPr()
	if p.p.PPr.Spacing == nil {
		p.p.PPr.Spacing = &oxml.CT_Spacing{}
	}
	return p.p.PPr.Spacing
}

func (p *Paragraph) ensureInd() *oxml.CT_Ind {
	p.ensurePPr()
	if p.p.PPr.Ind == nil {
		p.p.PPr.Ind = &oxml.CT_Ind{}
	}
	return p.p.PPr.Ind
}

// pointsToTwips converts points to twips (1 point = 20 twips).
func pointsToTwips(points float64) string {
	return strconv.Itoa(int(math.Round(points * 20)))
}

// twipsToPoints converts a twips string to points.
func twipsToPoints(twips string) float64 {
	v, err := strconv.Atoi(twips)
	if err != nil {
		return 0
	}
	return float64(v) / 20.0
}

// Alignment represents paragraph alignment.
type Alignment int

const (
	AlignmentLeft Alignment = iota
	AlignmentCenter
	AlignmentRight
	AlignmentJustify
)
