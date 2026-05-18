package pptx

import (
	"strings"

	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/pptx/internal/oxml"
)

// ReplaceText performs template-style text replacement across all slides in the presentation.
// Keys in the replacements map should NOT include delimiters — they will be wrapped with
// {{ and }} automatically. For example, to replace "{{name}}" with "John", pass
// map[string]string{"name": "John"}.
//
// This modifies the underlying XML directly to preserve formatting fidelity.
// It also updates the materialized Go-level shapes so that Shapes()/Placeholders()
// reflect the changes.
func (p *Presentation) ReplaceText(replacements map[string]string) {
	if len(replacements) == 0 {
		return
	}

	// Build the delimited replacement map: "{{key}}" -> "value"
	delimited := make(map[string]string, len(replacements))
	for k, v := range replacements {
		delimited["{{"+k+"}}"] = v
	}

	for _, slide := range p.slides {
		slide.replaceTextInXML(delimited)
	}
}

// ReplaceText performs template-style text replacement on this slide.
// Keys in the replacements map should NOT include delimiters.
// See Presentation.ReplaceText for details.
func (s *Slide) ReplaceText(replacements map[string]string) {
	if len(replacements) == 0 {
		return
	}

	delimited := make(map[string]string, len(replacements))
	for k, v := range replacements {
		delimited["{{"+k+"}}"] = v
	}

	s.replaceTextInXML(delimited)
}

// ReplaceTextRaw performs text replacement on this slide using exact match strings.
// Unlike ReplaceText, the keys should include any delimiters you want to match.
// For example: map[string]string{"{{name}}": "John", "Hello": "Hi"}.
func (s *Slide) ReplaceTextRaw(replacements map[string]string) {
	if len(replacements) == 0 {
		return
	}
	s.replaceTextInXML(replacements)
}

// ReplaceTextInShape performs template-style text replacement on the named shape.
// Keys in the replacements map should NOT include delimiters.
func (s *Slide) ReplaceTextInShape(shapeName string, replacements map[string]string) {
	if shapeName == "" || len(replacements) == 0 {
		return
	}

	delimited := make(map[string]string, len(replacements))
	for k, v := range replacements {
		delimited["{{"+k+"}}"] = v
	}

	s.replaceTextInNamedShapeXML(shapeName, delimited)
}

// ReplaceTextRawInShape performs exact text replacement on the named shape.
// Unlike ReplaceTextInShape, the keys should include any delimiters you want to match.
func (s *Slide) ReplaceTextRawInShape(shapeName string, replacements map[string]string) {
	if shapeName == "" || len(replacements) == 0 {
		return
	}
	s.replaceTextInNamedShapeXML(shapeName, replacements)
}

// replaceTextInXML performs replacement directly on the slide's XML shape tree
// and re-materializes the Go-level shapes to keep them in sync.
// The replacements map contains exact strings to find (already delimited if needed).
func (s *Slide) replaceTextInXML(replacements map[string]string) {
	if s.slideXML == nil || s.slideXML.CSld == nil || s.slideXML.CSld.SpTree == nil {
		return
	}

	changed := false

	// Replace in all sp (shape) elements
	for _, sp := range s.slideXML.CSld.SpTree.Sp {
		if sp.TxBody != nil {
			if replaceTextInTxBody(sp.TxBody, replacements) {
				changed = true
			}
		}
	}

	// Replace in all graphic frames (tables)
	for _, gf := range s.slideXML.CSld.SpTree.GraphicFrame {
		if gf.Graphic != nil && gf.Graphic.GraphicData != nil && gf.Graphic.GraphicData.Table != nil {
			for _, tr := range gf.Graphic.GraphicData.Table.Tr {
				for _, tc := range tr.Tc {
					if tc.TxBody != nil {
						if replaceTextInTxBody(tc.TxBody, replacements) {
							changed = true
						}
					}
				}
			}
		}
	}

	// Replace in group shapes (recursive)
	for _, grpSp := range s.slideXML.CSld.SpTree.GrpSp {
		if replaceTextInGroupShape(grpSp, replacements) {
			changed = true
		}
	}

	// If anything changed, re-materialize the Go-level shapes
	if changed {
		s.shapes = nil
		s.materializeShapes()
	}
}

func (s *Slide) replaceTextInNamedShapeXML(shapeName string, replacements map[string]string) {
	if s.slideXML == nil || s.slideXML.CSld == nil || s.slideXML.CSld.SpTree == nil {
		return
	}

	changed := false
	spTree := s.slideXML.CSld.SpTree

	for _, sp := range spTree.Sp {
		if shapeNameOfShape(sp) != shapeName || sp.TxBody == nil {
			continue
		}
		if replaceTextInTxBody(sp.TxBody, replacements) {
			changed = true
		}
	}

	for _, gf := range spTree.GraphicFrame {
		if shapeNameOfGraphicFrame(gf) != shapeName || gf.Graphic == nil || gf.Graphic.GraphicData == nil || gf.Graphic.GraphicData.Table == nil {
			continue
		}
		for _, tr := range gf.Graphic.GraphicData.Table.Tr {
			for _, tc := range tr.Tc {
				if tc.TxBody != nil && replaceTextInTxBody(tc.TxBody, replacements) {
					changed = true
				}
			}
		}
	}

	for _, grpSp := range spTree.GrpSp {
		if replaceTextInNamedGroupShape(grpSp, shapeName, replacements) {
			changed = true
		}
	}

	if changed {
		s.shapes = nil
		s.materializeShapes()
	}
}

// replaceTextInGroupShape recursively replaces text in a group shape's children.
func replaceTextInGroupShape(gs *oxml.GroupShape, replacements map[string]string) bool {
	if gs == nil {
		return false
	}

	changed := false

	for _, sp := range gs.Shapes {
		if sp.TxBody != nil {
			if replaceTextInTxBody(sp.TxBody, replacements) {
				changed = true
			}
		}
	}

	for _, sub := range gs.GroupShapes {
		if replaceTextInGroupShape(sub, replacements) {
			changed = true
		}
	}

	return changed
}

func replaceTextInNamedGroupShape(gs *oxml.GroupShape, shapeName string, replacements map[string]string) bool {
	if gs == nil {
		return false
	}

	changed := false
	applyAllChildren := shapeNameOfGroupShape(gs) == shapeName

	for _, sp := range gs.Shapes {
		if (applyAllChildren || shapeNameOfShape(sp) == shapeName) && sp.TxBody != nil {
			if replaceTextInTxBody(sp.TxBody, replacements) {
				changed = true
			}
		}
	}

	for _, gf := range gs.GraphicFrames {
		if !applyAllChildren && shapeNameOfGraphicFrame(gf) != shapeName {
			continue
		}
		if gf.Graphic == nil || gf.Graphic.GraphicData == nil || gf.Graphic.GraphicData.Table == nil {
			continue
		}
		for _, tr := range gf.Graphic.GraphicData.Table.Tr {
			for _, tc := range tr.Tc {
				if tc.TxBody != nil && replaceTextInTxBody(tc.TxBody, replacements) {
					changed = true
				}
			}
		}
	}

	for _, sub := range gs.GroupShapes {
		if applyAllChildren {
			if replaceTextInGroupShape(sub, replacements) {
				changed = true
			}
			continue
		}
		if replaceTextInNamedGroupShape(sub, shapeName, replacements) {
			changed = true
		}
	}

	return changed
}

func shapeNameOfShape(sp *oxml.Shape) string {
	if sp == nil || sp.NvSpPr == nil || sp.NvSpPr.CNvPr == nil {
		return ""
	}
	return sp.NvSpPr.CNvPr.Name
}

func shapeNameOfGraphicFrame(gf *oxml.GraphicFrame) string {
	if gf == nil || gf.NvGraphicFramePr == nil || gf.NvGraphicFramePr.CNvPr == nil {
		return ""
	}
	return gf.NvGraphicFramePr.CNvPr.Name
}

func shapeNameOfGroupShape(gs *oxml.GroupShape) string {
	if gs == nil || gs.NvGrpSpPr == nil || gs.NvGrpSpPr.CNvPr == nil {
		return ""
	}
	return gs.NvGrpSpPr.CNvPr.Name
}

// replaceTextInTxBody performs template replacement in a TxBody, handling the case
// where a template key like {{name}} spans multiple runs within a paragraph.
//
// Algorithm:
//  1. For each paragraph, concatenate all run texts to form a "full text" string.
//  2. Check if any replacement key exists in the full text.
//  3. If matches are found, perform replacements on the full text, then redistribute
//     the result back into runs — the replacement text inherits the formatting of
//     the first run that contained part of the matched key.
func replaceTextInTxBody(txBody *dml.TxBody, replacements map[string]string) bool {
	if txBody == nil {
		return false
	}

	changed := false

	for _, p := range txBody.P {
		if replaceTextInParagraph(p, replacements) {
			changed = true
		}
	}

	return changed
}

// replaceTextInParagraph handles cross-run template replacement within a single paragraph.
func replaceTextInParagraph(p *dml.P, replacements map[string]string) bool {
	if p == nil || len(p.R) == 0 {
		return false
	}

	origRunCount := len(p.R)
	origRuns := append([]*dml.R(nil), p.R...)

	// Build the concatenated paragraph text and track run boundaries.
	var sb strings.Builder
	runBoundaries := make([]int, len(p.R)+1) // start position of each run in concatenated text
	for i, r := range p.R {
		runBoundaries[i] = sb.Len()
		sb.WriteString(r.T)
	}
	runBoundaries[len(p.R)] = sb.Len()

	fullText := sb.String()

	// Check if any replacement key exists in the full text.
	newText := fullText
	hasMatch := false
	for old, repl := range replacements {
		if strings.Contains(newText, old) {
			newText = strings.ReplaceAll(newText, old, repl)
			hasMatch = true
		}
	}

	if !hasMatch {
		return false
	}

	// The text has changed. We need to redistribute the new text back into runs.
	// Strategy: if only a single run, just update its text.
	if len(p.R) == 1 {
		p.R[0].T = newText
		if len(p.Br) == 0 && len(p.Fld) == 0 {
			p.ResetRunOrder()
		}
		return true
	}

	// Multi-run case: We need to figure out which runs were affected and rebuild them.
	// The simplest correct approach for template replacement:
	// Collapse all runs into one run (preserving the first run's formatting),
	// then split on the original non-template text boundaries.
	//
	// However, this would lose formatting on non-template parts. A better approach:
	// Process each replacement key individually, finding which runs contain parts of it.
	redistributeText(p, fullText, newText, runBoundaries)
	if len(p.Br) == 0 && len(p.Fld) == 0 {
		p.ResetRunOrder()
	} else if len(p.R) != origRunCount {
		// When paragraphs contain interleaved breaks or fields, changing the run count
		// would invalidate the preserved child ordering. Keep the original ordering and
		// run segmentation in those cases.
		p.R = origRuns
		return false
	}

	return true
}

// redistributeText redistributes replacement text across runs in a paragraph.
// It attempts to preserve formatting of runs that were not affected by the replacement,
// while consolidating runs that contained parts of a template key.
func redistributeText(p *dml.P, oldFullText, newFullText string, runBoundaries []int) {
	// For simple cases where the replacement doesn't change the structure significantly,
	// we try to map characters from the new text back to the original runs.
	// However, since replacements can change text length, we use a different approach:
	//
	// Walk through each replacement, find where in the original text the key appeared,
	// determine which runs it spanned, and consolidate those runs into one with the
	// replacement value, keeping the first run's formatting.

	// Since we may have multiple overlapping replacements already applied to newFullText,
	// we take a pragmatic approach: try to preserve as much original formatting as possible
	// by doing a character-level diff between old and new text.

	// Simpler approach that works well for non-overlapping template keys:
	// Rebuild runs based on which original characters survived vs. were replaced.
	// If the old full text was "Hello {{name}}, welcome!" and the new is "Hello John, welcome!",
	// we can map the unchanged prefix "Hello " to its original runs, then "John" gets the
	// formatting of the run that contained "{{", then ", welcome!" maps to its original runs.

	// For robustness and simplicity, we use the "first run formatting" approach for the
	// changed segments and preserve formatting for unchanged prefix/suffix.
	oldRuns := p.R
	if len(oldRuns) == 0 {
		return
	}

	// Find common prefix and suffix between old and new text
	prefixLen := commonPrefixLen(oldFullText, newFullText)
	suffixLen := commonSuffixLen(oldFullText, newFullText)

	// Ensure prefix and suffix don't overlap
	oldMiddleStart := prefixLen
	oldMiddleEnd := len(oldFullText) - suffixLen
	newMiddleStart := prefixLen
	newMiddleEnd := len(newFullText) - suffixLen

	if oldMiddleStart > oldMiddleEnd {
		oldMiddleEnd = oldMiddleStart
	}
	if newMiddleStart > newMiddleEnd {
		newMiddleEnd = newMiddleStart
	}

	// Rebuild runs:
	// 1. Prefix runs (unchanged text from the beginning)
	// 2. Middle run(s) (the replacement text, with formatting from the first affected run)
	// 3. Suffix runs (unchanged text from the end)
	var newRuns []*dml.R

	// Add prefix runs - runs that are entirely within the prefix
	for i, r := range oldRuns {
		runEnd := runBoundaries[i+1]
		if runEnd <= prefixLen {
			// Entire run is in the prefix — keep as-is
			newRuns = append(newRuns, r)
		} else if runBoundaries[i] < prefixLen {
			// Run partially overlaps the prefix — split it
			splitPoint := prefixLen - runBoundaries[i]
			prefixRun := cloneRunWithText(r, r.T[:splitPoint])
			newRuns = append(newRuns, prefixRun)
			break
		} else {
			break
		}
	}

	// Add the middle (replacement) run
	middleText := newFullText[newMiddleStart:newMiddleEnd]
	if len(middleText) > 0 {
		// Find the formatting run — the first run that intersects the replaced region
		fmtRun := findRunAtPosition(oldRuns, runBoundaries, oldMiddleStart)
		middleRun := cloneRunWithText(fmtRun, middleText)
		newRuns = append(newRuns, middleRun)
	}

	// Add suffix runs — runs that are entirely within the suffix
	suffixStart := len(oldFullText) - suffixLen
	for i, r := range oldRuns {
		runStart := runBoundaries[i]
		runEnd := runBoundaries[i+1]
		if runStart >= suffixStart {
			// Entire run is in the suffix — keep as-is
			newRuns = append(newRuns, r)
		} else if runEnd > suffixStart && runStart < suffixStart {
			// Run partially overlaps the suffix — split it
			splitPoint := suffixStart - runStart
			suffixRun := cloneRunWithText(r, r.T[splitPoint:])
			newRuns = append(newRuns, suffixRun)
		}
	}

	// If we ended up with no runs, create one with the full new text
	if len(newRuns) == 0 {
		newRuns = []*dml.R{cloneRunWithText(oldRuns[0], newFullText)}
	}

	p.R = newRuns
}

// commonPrefixLen returns the length of the common prefix between two strings.
func commonPrefixLen(a, b string) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	return n
}

// commonSuffixLen returns the length of the common suffix between two strings.
func commonSuffixLen(a, b string) int {
	n := len(a)
	m := len(b)
	maxLen := n
	if m < maxLen {
		maxLen = m
	}
	for i := 0; i < maxLen; i++ {
		if a[n-1-i] != b[m-1-i] {
			return i
		}
	}
	return maxLen
}

// findRunAtPosition finds the run that contains the character at the given position
// in the concatenated paragraph text.
func findRunAtPosition(runs []*dml.R, boundaries []int, pos int) *dml.R {
	for i, r := range runs {
		if pos >= boundaries[i] && pos < boundaries[i+1] {
			return r
		}
	}
	// If pos is at the very end, return the last run
	if len(runs) > 0 {
		return runs[len(runs)-1]
	}
	return nil
}

// cloneRunWithText creates a copy of a run with different text,
// preserving all formatting properties.
func cloneRunWithText(r *dml.R, text string) *dml.R {
	if r == nil {
		return &dml.R{T: text}
	}
	return &dml.R{
		RPr: r.RPr, // Share the RPr pointer — run properties are not modified
		T:   text,
	}
}
