package pptx

import "strings"

// Text returns every piece of text in the presentation as a single plain
// string, with no markup, suitable for search, indexing, or LLM ingestion. It
// concatenates each slide's text (see Slide.Text) in slide order, separated by
// a blank line.
func (p *Presentation) Text() string {
	var sb strings.Builder
	for _, s := range p.Slides() {
		t := s.Text()
		if t == "" {
			continue
		}
		if sb.Len() > 0 {
			sb.WriteString("\n\n")
		}
		sb.WriteString(t)
	}
	return sb.String()
}

// SlideTexts returns the extracted text of each slide (see Slide.Text), one
// entry per slide in slide order.
func (p *Presentation) SlideTexts() []string {
	slides := p.Slides()
	out := make([]string, len(slides))
	for i, s := range slides {
		out[i] = s.Text()
	}
	return out
}

// Text returns the slide's text as a single plain string: the text of every
// shape (text boxes, placeholders, auto shapes, and tables, descending into
// groups) in shape order, followed by the speaker notes and then the comments
// (including threaded replies).
//
// Shape text keeps its paragraph newlines. Table cells are separated by a tab
// ("\t") and rows by "\n". Shapes, the notes block, and each comment are
// separated from one another by "\n".
func (s *Slide) Text() string {
	var sb strings.Builder

	for _, sh := range s.Shapes() {
		writeShapeText(&sb, sh)
	}

	if notes := s.Notes(); notes != "" {
		appendLine(&sb, notes)
	}

	for _, c := range s.Comments() {
		writeCommentText(&sb, c)
	}

	return sb.String()
}

// writeShapeText appends a shape's text, recursing into group children and
// laying tables out as tab-separated cells / newline-separated rows.
func writeShapeText(sb *strings.Builder, sh Shape) {
	switch v := sh.(type) {
	case *GroupShape:
		for _, child := range v.Children() {
			writeShapeText(sb, child)
		}
	case *Table:
		for _, row := range v.Rows() {
			cells := row.Cells()
			texts := make([]string, len(cells))
			for i, c := range cells {
				texts[i] = c.Text()
			}
			appendLine(sb, strings.Join(texts, "\t"))
		}
	case *TextBox:
		appendNonEmptyLine(sb, v.Text())
	case *PlaceholderShape:
		appendNonEmptyLine(sb, v.Text())
	case *AutoShape:
		if tf := v.TextFrame(); tf != nil {
			appendNonEmptyLine(sb, tf.Text())
		}
	}
}

// writeCommentText appends a comment's text and, recursively, its replies.
func writeCommentText(sb *strings.Builder, c *Comment) {
	appendNonEmptyLine(sb, c.Text())
	for _, reply := range c.Replies() {
		writeCommentText(sb, reply)
	}
}

// appendNonEmptyLine appends s as its own line only when it carries text.
func appendNonEmptyLine(sb *strings.Builder, s string) {
	if s == "" {
		return
	}
	appendLine(sb, s)
}

// appendLine appends s as its own line, separated from prior content by "\n".
func appendLine(sb *strings.Builder, s string) {
	if sb.Len() > 0 {
		sb.WriteByte('\n')
	}
	sb.WriteString(s)
}
