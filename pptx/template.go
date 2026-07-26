package pptx

import (
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/pptx/internal/oxml"
)

// ReplaceText substitutes text across every slide in the presentation. Keys in
// the replacements map are matched exactly — to replace "{{name}}" with "John",
// pass map[string]string{"{{name}}": "John"}. An empty map (or an empty key) is
// a no-op.
//
// A match may span several a:r runs: PowerPoint frequently splits one logical
// string across runs, so each paragraph's run text is concatenated before
// matching and the replacement is spliced back in. The replacement inherits the
// formatting of the first run that held part of the match; runs before and after
// the match keep their own formatting. A key is not matched across an a:br line
// break or an a:fld field boundary — those delimit distinct content.
//
// Scope: within each slide this reaches the text of shapes and of table cells at
// the shape-tree level, and the text of shapes inside grouped shapes (recursively
// through nested groups). It does NOT touch speaker notes, slide masters, or
// slide layouts — only the slides themselves. Slide.ReplaceText applies the same
// rules to one slide, and Slide.ReplaceTextInShape to a single named shape.
//
// The replacement edits the parsed slide model in place and refreshes the
// materialized Go-level shapes so Shapes()/Placeholders() reflect it. Unlike the
// docx and xlsx counterparts, scanning a slide materializes its model, so a
// subsequent Save regenerates every scanned slide from that model rather than
// copying its original bytes; the run text of content that matches no key is
// preserved exactly, but a slide scanned this way no longer takes the
// verbatim-bytes passthrough that a never-accessed slide gets.
func (p *Presentation) ReplaceText(replacements map[string]string) {
	if len(replacements) == 0 {
		return
	}

	for _, slide := range p.slides {
		slide.replaceTextInXML(replacements)
	}
}

// ReplaceText performs text replacement on this slide.
// Keys in the replacements map are matched exactly as provided.
// See Presentation.ReplaceText for details.
func (s *Slide) ReplaceText(replacements map[string]string) {
	if len(replacements) == 0 {
		return
	}
	s.replaceTextInXML(replacements)
}

// ReplaceTextInShape performs text replacement on the named shape.
// Keys in the replacements map are matched exactly as provided.
// Only the shape with the given name is affected; other shapes on the slide are untouched.
func (s *Slide) ReplaceTextInShape(shapeName string, replacements map[string]string) {
	if shapeName == "" || len(replacements) == 0 {
		return
	}
	s.replaceTextInNamedShapeXML(shapeName, replacements)
}

// replaceTextInXML performs replacement directly on the slide's XML shape tree
// and re-materializes the Go-level shapes to keep them in sync.
// The replacements map contains exact strings to find (already delimited if needed).
func (s *Slide) replaceTextInXML(replacements map[string]string) {
	if s.sx() == nil {
		return
	}
	// Flush pending domain-shape edits into the XML tree first. Decks built via
	// the API author their text in domain shapes that are only synced to XML at
	// marshal time, so without this the walk below sees an empty tree (a no-op
	// on created decks); loaded decks may also have API-added shapes that the
	// re-materialize at the end would otherwise drop. Dirty in-place edits are
	// flushed too, so the replacement operates on the caller's latest text.
	if s.shapesModified || s.hasDirtyShapes() {
		s.syncShapesToXML()
	}
	if s.sx().CSld == nil || s.sx().CSld.SpTree == nil {
		return
	}

	changed := false

	// Replace in all sp (shape) elements
	for _, sp := range s.sx().CSld.SpTree.Sp {
		if sp.TxBody != nil {
			if replaceTextInTxBody(sp.TxBody, replacements) {
				changed = true
			}
		}
	}

	// Replace in all graphic frames (tables)
	for _, gf := range s.sx().CSld.SpTree.GraphicFrame {
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
	for _, grpSp := range s.sx().CSld.SpTree.GrpSp {
		if replaceTextInGroupShape(grpSp, replacements) {
			changed = true
		}
	}

	// If anything changed, refresh the Go-level shapes from the XML, keeping
	// caller-held shape pointers attached (see rematerializeShapes).
	if changed {
		s.rematerializeShapes()
	}
}

func (s *Slide) replaceTextInNamedShapeXML(shapeName string, replacements map[string]string) {
	if s.sx() == nil {
		return
	}
	if s.shapesModified || s.hasDirtyShapes() {
		s.syncShapesToXML()
	}
	if s.sx().CSld == nil || s.sx().CSld.SpTree == nil {
		return
	}

	changed := false
	spTree := s.sx().CSld.SpTree

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
		s.rematerializeShapes()
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
//
// Paragraphs containing a:br or a:fld children are handled per run segment:
// each maximal sequence of consecutive a:r children between break/field
// boundaries is replaced independently, and the br/fld elements stay in place
// (previously such paragraphs silently reverted every replacement, C87). A
// key spanning a br or fld boundary is deliberately not matched: a break is a
// line boundary, so text on either side is distinct content.
func replaceTextInParagraph(p *dml.P, replacements map[string]string) bool {
	if p == nil || len(p.R) == 0 {
		return false
	}

	if len(p.Br) > 0 || len(p.Fld) > 0 {
		changed := false
		p.MapRunSegments(func(runs []*dml.R) []*dml.R {
			out, ok := replaceTextInRuns(runs, replacements)
			if !ok {
				return runs
			}
			changed = true
			return out
		})
		return changed
	}

	newRuns, ok := replaceTextInRuns(p.R, replacements)
	if !ok {
		return false
	}
	p.R = newRuns
	p.ResetRunOrder()
	return true
}

// replaceTextInRuns applies the replacements to a sequence of runs, returning
// the rebuilt run slice and whether anything matched. Formatting of unchanged
// prefix/suffix runs is preserved; the replaced middle inherits the formatting
// of the first run that contained part of the match.
func replaceTextInRuns(runs []*dml.R, replacements map[string]string) ([]*dml.R, bool) {
	if len(runs) == 0 {
		return runs, false
	}

	// Build the concatenated text and track run boundaries.
	var sb strings.Builder
	runBoundaries := make([]int, len(runs)+1) // start position of each run in concatenated text
	for i, r := range runs {
		runBoundaries[i] = sb.Len()
		sb.WriteString(r.T)
	}
	runBoundaries[len(runs)] = sb.Len()

	fullText := sb.String()

	// Apply all replacements in a single deterministic pass (see applyReplacements),
	// so the result does not depend on Go's map iteration order and a replacement
	// value that happens to contain another key is not re-replaced.
	newText, hasMatch := applyReplacements(fullText, replacements)
	if !hasMatch {
		return runs, false
	}

	// Single run: just swap the text, keeping the formatting.
	if len(runs) == 1 {
		runs[0].T = newText
		return runs, true
	}

	return redistributeRuns(runs, fullText, newText, runBoundaries), true
}

// applyReplacements applies every key->value replacement to text in a single
// left-to-right pass. At each position the longest matching key wins and the
// scan advances past the inserted value, so the result is independent of map
// iteration order and a value that contains another key is never re-replaced.
func applyReplacements(text string, replacements map[string]string) (string, bool) {
	keys := make([]string, 0, len(replacements))
	for k := range replacements {
		if k != "" {
			keys = append(keys, k)
		}
	}
	sort.Slice(keys, func(i, j int) bool {
		if len(keys[i]) != len(keys[j]) {
			return len(keys[i]) > len(keys[j])
		}
		return keys[i] < keys[j]
	})

	var sb strings.Builder
	changed := false
	for i := 0; i < len(text); {
		matched := false
		for _, k := range keys {
			if strings.HasPrefix(text[i:], k) {
				sb.WriteString(replacements[k])
				i += len(k)
				matched = true
				changed = true
				break
			}
		}
		if !matched {
			sb.WriteByte(text[i])
			i++
		}
	}
	if !changed {
		return text, false
	}
	return sb.String(), true
}

// redistributeRuns redistributes replacement text across a run sequence.
// It attempts to preserve formatting of runs that were not affected by the replacement,
// while consolidating runs that contained parts of a template key.
func redistributeRuns(oldRuns []*dml.R, oldFullText, newFullText string, runBoundaries []int) []*dml.R {
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
	if len(oldRuns) == 0 {
		return oldRuns
	}

	// Find common prefix and suffix between old and new text. Both advance by
	// whole runes, so every split point computed below is a rune boundary in
	// the full text — and, because run boundaries are rune boundaries too, in
	// the run being split. Byte-level splits cut multi-byte UTF-8 sequences in
	// half, emitting <a:t> content that is not valid XML.
	prefixLen := commonPrefixLen(oldFullText, newFullText)
	suffixLen := commonSuffixLen(oldFullText, newFullText)

	// The prefix and suffix must not overlap in either string. A shrinking
	// replacement (e.g. "aa" -> "a") otherwise double-counts the shared region
	// and emits "aa" instead of "a". Clamp the suffix so prefix+suffix fits the
	// shorter of the two strings...
	if maxSuffix := min(len(oldFullText), len(newFullText)) - prefixLen; suffixLen > maxSuffix {
		if maxSuffix < 0 {
			maxSuffix = 0
		}
		suffixLen = maxSuffix
	}
	// ...and re-snap the clamped byte count to a rune boundary in both
	// strings (the clamp can land mid-rune even though prefix and suffix
	// individually cannot).
	for suffixLen > 0 && (!utf8.RuneStart(oldFullText[len(oldFullText)-suffixLen]) ||
		!utf8.RuneStart(newFullText[len(newFullText)-suffixLen])) {
		suffixLen--
	}

	// After the clamp, prefixLen+suffixLen <= min(len(oldFullText),
	// len(newFullText)), so the middle bounds below cannot cross.
	oldMiddleStart := prefixLen
	newMiddleStart := prefixLen
	newMiddleEnd := len(newFullText) - suffixLen

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

	return newRuns
}

// commonPrefixLen returns the byte length of the common prefix between two
// strings, counting whole runes only, so the result is always a rune boundary
// in both.
func commonPrefixLen(a, b string) int {
	n := 0
	for n < len(a) && n < len(b) {
		_, size := utf8.DecodeRuneInString(a[n:])
		if n+size > len(b) || a[n:n+size] != b[n:n+size] {
			break
		}
		n += size
	}
	return n
}

// commonSuffixLen returns the byte length of the common suffix between two
// strings, counting whole runes only, so the result is always a rune boundary
// in both.
func commonSuffixLen(a, b string) int {
	n := 0
	for n < len(a) && n < len(b) {
		_, size := utf8.DecodeLastRuneInString(a[:len(a)-n])
		if size == 0 || len(b)-n-size < 0 ||
			a[len(a)-n-size:len(a)-n] != b[len(b)-n-size:len(b)-n] {
			break
		}
		n += size
	}
	return n
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
