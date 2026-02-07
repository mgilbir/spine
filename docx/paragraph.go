package docx

import (
	"github.com/mgilbir/spine/docx/internal/oxml"
)

// Paragraph represents a paragraph in a Word document.
type Paragraph struct {
	document *Document
	p        *oxml.CT_P
}

// Text returns the text content of the paragraph.
func (p *Paragraph) Text() string {
	text := ""
	for _, r := range p.p.R {
		for _, t := range r.T {
			text += t.Text
		}
	}
	return text
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

// Alignment represents paragraph alignment.
type Alignment int

const (
	AlignmentLeft Alignment = iota
	AlignmentCenter
	AlignmentRight
	AlignmentJustify
)
