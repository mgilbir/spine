package xlsx

import (
	"sort"
	"strings"
	"unicode/utf8"
)

// ReplaceText performs text replacement across every sheet in the workbook.
// Keys in the replacements map are matched exactly as provided — to replace
// "{{name}}" with "John", pass map[string]string{"{{name}}": "John"}.
//
// Replacement applies to string cells (both shared-string and inline-string
// cells) and to the individual runs of rich (multi-run) text cells, where a run
// spanning a match inherits the first affected run's font. Formula cells are
// NOT touched: their string type is a cached formula result, not literal text.
// Numeric, boolean, date, and error cells are left unchanged.
//
// This mirrors pptx.Presentation.ReplaceText and docx.Document.ReplaceText.
// Empty keys are ignored, and a workbook with no matching text round-trips
// byte-for-byte.
func (w *Workbook) ReplaceText(replacements map[string]string) {
	if w == nil || len(replacements) == 0 {
		return
	}
	for _, s := range w.sheets {
		s.replaceText(replacements)
	}
}

// ReplaceText performs text replacement on this sheet only. See
// Workbook.ReplaceText for the matching rules.
func (s *Sheet) ReplaceText(replacements map[string]string) {
	if s == nil || len(replacements) == 0 {
		return
	}
	s.replaceText(replacements)
}

// replaceText walks every cell of the sheet applying the replacements.
func (s *Sheet) replaceText(replacements map[string]string) {
	if s == nil || s.worksheet == nil {
		return
	}
	for i := range s.worksheet.SheetData.Row {
		row := &s.worksheet.SheetData.Row[i]
		for _, c := range row.C {
			if c == nil {
				continue
			}
			cell := &Cell{sheet: s, cell: c}
			cell.replaceText(replacements)
		}
	}
}

// replaceText applies the replacements to a single cell's text. A shared-string
// cell is converted to an inline string so the shared table (and every other
// cell that references the same entry) is left untouched. Rich cells keep their
// per-run formatting. Formula cells are skipped.
func (c *Cell) replaceText(replacements map[string]string) {
	if c.cell.F != nil {
		return
	}
	switch c.cell.T {
	case "s":
		if si := c.sharedStringItem(); si != nil && len(si.R) > 0 {
			if runs, ok := replaceInTextRuns(reltRunsToTextRuns(si.R), replacements); ok {
				c.SetRichText(runs)
			}
			return
		}
		if newText, ok := applyReplacements(c.String(), replacements); ok {
			c.SetString(newText)
		}
	case "inlineStr":
		if c.cell.Is == nil {
			return
		}
		if len(c.cell.Is.R) > 0 {
			if runs, ok := replaceInTextRuns(reltRunsToTextRuns(c.cell.Is.R), replacements); ok {
				c.SetRichText(runs)
			}
			return
		}
		if c.cell.Is.T != nil {
			if newText, ok := applyReplacements(*c.cell.Is.T, replacements); ok {
				c.SetString(newText)
			}
		}
	}
}

// replaceInTextRuns applies the replacements across a sequence of rich-text
// runs, returning the rebuilt runs and whether anything matched. The replaced
// middle inherits the font of the first run it intersected; unchanged
// prefix/suffix runs keep their own font.
func replaceInTextRuns(runs []TextRun, replacements map[string]string) ([]TextRun, bool) {
	if len(runs) == 0 {
		return runs, false
	}

	var sb strings.Builder
	boundaries := make([]int, len(runs)+1)
	for i := range runs {
		boundaries[i] = sb.Len()
		sb.WriteString(runs[i].Text)
	}
	boundaries[len(runs)] = sb.Len()
	fullText := sb.String()

	newText, ok := applyReplacements(fullText, replacements)
	if !ok {
		return runs, false
	}

	if len(runs) == 1 {
		runs[0].Text = newText
		return runs, true
	}

	return redistributeTextRuns(runs, fullText, newText, boundaries), true
}

// redistributeTextRuns splices newFullText back into a rich-text run sequence,
// preserving the font of runs untouched by the replacement and giving the
// replaced middle the font of the first run it intersected.
func redistributeTextRuns(oldRuns []TextRun, oldFullText, newFullText string, boundaries []int) []TextRun {
	if len(oldRuns) == 0 {
		return oldRuns
	}

	// Common prefix/suffix advance by whole runes so every split point lands on
	// a rune boundary (see the docx counterpart for the full rationale).
	prefixLen := strCommonPrefixLen(oldFullText, newFullText)
	suffixLen := strCommonSuffixLen(oldFullText, newFullText)

	if maxSuffix := min(len(oldFullText), len(newFullText)) - prefixLen; suffixLen > maxSuffix {
		if maxSuffix < 0 {
			maxSuffix = 0
		}
		suffixLen = maxSuffix
	}
	for suffixLen > 0 && (!utf8.RuneStart(oldFullText[len(oldFullText)-suffixLen]) ||
		!utf8.RuneStart(newFullText[len(newFullText)-suffixLen])) {
		suffixLen--
	}

	newMiddleEnd := len(newFullText) - suffixLen

	var newRuns []TextRun

	// Prefix runs — kept as-is, with the straddling run split.
	for i := range oldRuns {
		runEnd := boundaries[i+1]
		if runEnd <= prefixLen {
			newRuns = append(newRuns, oldRuns[i])
		} else if boundaries[i] < prefixLen {
			splitPoint := prefixLen - boundaries[i]
			newRuns = append(newRuns, cloneTextRun(oldRuns[i], oldRuns[i].Text[:splitPoint]))
			break
		} else {
			break
		}
	}

	// Middle (replacement) run, inheriting the first affected run's font.
	if middleText := newFullText[prefixLen:newMiddleEnd]; len(middleText) > 0 {
		newRuns = append(newRuns, cloneTextRun(findTextRunAtPosition(oldRuns, boundaries, prefixLen), middleText))
	}

	// Suffix runs — kept as-is, with the straddling run split.
	suffixStart := len(oldFullText) - suffixLen
	for i := range oldRuns {
		runStart := boundaries[i]
		runEnd := boundaries[i+1]
		if runStart >= suffixStart {
			newRuns = append(newRuns, oldRuns[i])
		} else if runEnd > suffixStart && runStart < suffixStart {
			splitPoint := suffixStart - runStart
			newRuns = append(newRuns, cloneTextRun(oldRuns[i], oldRuns[i].Text[splitPoint:]))
		}
	}

	if len(newRuns) == 0 {
		newRuns = []TextRun{cloneTextRun(oldRuns[0], newFullText)}
	}

	return newRuns
}

// cloneTextRun returns a run with the given text and the source run's font.
func cloneTextRun(r TextRun, text string) TextRun {
	return TextRun{Text: text, Font: r.Font}
}

// findTextRunAtPosition finds the run containing the character at pos in the
// concatenated cell text.
func findTextRunAtPosition(runs []TextRun, boundaries []int, pos int) TextRun {
	for i := range runs {
		if pos >= boundaries[i] && pos < boundaries[i+1] {
			return runs[i]
		}
	}
	return runs[len(runs)-1]
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

// strCommonPrefixLen returns the byte length of the common prefix between two
// strings, counting whole runes only, so the result is always a rune boundary
// in both.
func strCommonPrefixLen(a, b string) int {
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

// strCommonSuffixLen returns the byte length of the common suffix between two
// strings, counting whole runes only, so the result is always a rune boundary
// in both.
func strCommonSuffixLen(a, b string) int {
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
