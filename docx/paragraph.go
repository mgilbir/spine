package docx

// Paragraph represents a paragraph in a Word document.
type Paragraph struct {
	document *Document
	runs     []*Run
	style    string
}

// Text returns the text content of the paragraph.
func (p *Paragraph) Text() string {
	text := ""
	for _, run := range p.runs {
		text += run.text
	}
	return text
}

// SetText sets the text content, replacing all runs.
func (p *Paragraph) SetText(text string) {
	p.runs = p.runs[:0]
	p.AddRun().SetText(text)
}

// Runs returns all runs in the paragraph.
func (p *Paragraph) Runs() []*Run {
	return p.runs
}

// AddRun adds a new run to the paragraph.
func (p *Paragraph) AddRun() *Run {
	r := &Run{
		paragraph: p,
	}
	p.runs = append(p.runs, r)
	return r
}

// Style returns the paragraph style name.
func (p *Paragraph) Style() string {
	return p.style
}

// SetStyle sets the paragraph style.
func (p *Paragraph) SetStyle(style string) {
	p.style = style
}

// Alignment returns the paragraph alignment.
func (p *Paragraph) Alignment() Alignment {
	// Placeholder implementation
	return AlignmentLeft
}

// SetAlignment sets the paragraph alignment.
func (p *Paragraph) SetAlignment(align Alignment) {
	// Placeholder implementation
}

// Clear removes all runs from the paragraph.
func (p *Paragraph) Clear() {
	p.runs = p.runs[:0]
}

// InsertBefore inserts a new paragraph before this one.
func (p *Paragraph) InsertBefore() *Paragraph {
	// Find position in document
	for i, para := range p.document.paragraphs {
		if para == p {
			newPara := &Paragraph{
				document: p.document,
				runs:     make([]*Run, 0),
			}
			// Insert at position i
			p.document.paragraphs = append(
				p.document.paragraphs[:i],
				append([]*Paragraph{newPara}, p.document.paragraphs[i:]...)...,
			)
			return newPara
		}
	}
	return nil
}

// InsertAfter inserts a new paragraph after this one.
func (p *Paragraph) InsertAfter() *Paragraph {
	// Find position in document
	for i, para := range p.document.paragraphs {
		if para == p {
			newPara := &Paragraph{
				document: p.document,
				runs:     make([]*Run, 0),
			}
			// Insert at position i+1
			p.document.paragraphs = append(
				p.document.paragraphs[:i+1],
				append([]*Paragraph{newPara}, p.document.paragraphs[i+1:]...)...,
			)
			return newPara
		}
	}
	return nil
}

// Delete removes this paragraph from the document.
func (p *Paragraph) Delete() {
	for i, para := range p.document.paragraphs {
		if para == p {
			p.document.paragraphs = append(
				p.document.paragraphs[:i],
				p.document.paragraphs[i+1:]...,
			)
			return
		}
	}
}

// Alignment represents paragraph alignment.
type Alignment int

const (
	AlignmentLeft Alignment = iota
	AlignmentCenter
	AlignmentRight
	AlignmentJustify
)
