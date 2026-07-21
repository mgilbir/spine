package docx

import (
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/mgilbir/spine/docx/internal/oxml"
)

// ReplaceText performs text replacement across the whole document: every body
// paragraph (including those nested in tables and structured document tags) and
// every header and footer. Keys in the replacements map are matched exactly as
// provided — to replace "{{name}}" with "John", pass
// map[string]string{"{{name}}": "John"}.
//
// A match may span multiple w:r runs: Word often splits a single logical string
// across several runs (spell-check state, rsids), so the paragraph's run text is
// concatenated before matching and the replacement is spliced back in. The
// replacement inherits the formatting of the first run that contained part of
// the match; runs before and after the match keep their own formatting. A key
// is not matched across a line break, tab, field, drawing, or other non-text
// run, nor across a hyperlink or field boundary — those delimit distinct
// content.
//
// This mirrors pptx.Presentation.ReplaceText. Empty keys are ignored, and a
// document with no matching text round-trips byte-for-byte.
func (d *Document) ReplaceText(replacements map[string]string) {
	if d == nil || len(replacements) == 0 {
		return
	}

	// Body: the main document part is always regenerated on save, so edits to
	// its paragraphs persist without any modification bookkeeping.
	if d.doc() != nil && d.doc().Body != nil {
		for _, p := range d.doc().Body.AllParagraphs() {
			replaceTextInParagraph(p, replacements)
		}
	}

	// Headers and footers round-trip as preserved raw bytes unless their part
	// is flagged modified, so a replacement that changes one must mark it for
	// regeneration — otherwise the edit would be silently dropped on save.
	for name, hp := range d.headers {
		if hp == nil || hp.hdr == nil {
			continue
		}
		changed := false
		for _, p := range hp.hdr.AllParagraphs() {
			if replaceTextInParagraph(p, replacements) {
				changed = true
			}
		}
		if changed {
			d.markHdrFtrModified(name)
		}
	}
	for name, fp := range d.footers {
		if fp == nil || fp.ftr == nil {
			continue
		}
		changed := false
		for _, p := range fp.ftr.AllParagraphs() {
			if replaceTextInParagraph(p, replacements) {
				changed = true
			}
		}
		if changed {
			d.markHdrFtrModified(name)
		}
	}
}

// replaceTextInParagraph applies the replacements across the paragraph's
// text-only runs, handling keys that span multiple runs. It reports whether
// anything changed.
func replaceTextInParagraph(p *oxml.CT_P, replacements map[string]string) bool {
	if p == nil {
		return false
	}
	return p.ReplaceInTextRuns(func(runs []*oxml.CT_R) ([]*oxml.CT_R, bool) {
		return replaceTextInRuns(runs, replacements)
	})
}

// replaceTextInRuns applies the replacements to a sequence of consecutive
// text-only runs, returning the rebuilt run slice and whether anything matched.
// Formatting of unchanged prefix/suffix runs is preserved; the replaced middle
// inherits the formatting of the first run that contained part of the match.
func replaceTextInRuns(runs []*oxml.CT_R, replacements map[string]string) ([]*oxml.CT_R, bool) {
	if len(runs) == 0 {
		return runs, false
	}

	// Concatenate the run texts and record where each run starts.
	var sb strings.Builder
	boundaries := make([]int, len(runs)+1)
	for i, r := range runs {
		boundaries[i] = sb.Len()
		sb.WriteString(r.Text())
	}
	boundaries[len(runs)] = sb.Len()
	fullText := sb.String()

	newText, ok := applyReplacements(fullText, replacements)
	if !ok {
		return runs, false
	}

	// Single run: swap the text in place, keeping the formatting.
	if len(runs) == 1 {
		runs[0].SetTexts([]*oxml.CT_Text{{Space: "preserve", Text: newText}})
		return runs, true
	}

	return redistributeRuns(runs, fullText, newText, boundaries), true
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

// redistributeRuns splices newFullText back into a run sequence, preserving the
// formatting of runs untouched by the replacement and giving the replaced
// middle the formatting of the first run it intersected.
func redistributeRuns(oldRuns []*oxml.CT_R, oldFullText, newFullText string, boundaries []int) []*oxml.CT_R {
	if len(oldRuns) == 0 {
		return oldRuns
	}

	// Common prefix/suffix advance by whole runes, so every split point is a
	// rune boundary in the full text — and, since run boundaries are rune
	// boundaries too, in the run being split. Byte-level splits would cut
	// multi-byte UTF-8 sequences in half.
	prefixLen := commonPrefixLen(oldFullText, newFullText)
	suffixLen := commonSuffixLen(oldFullText, newFullText)

	// The prefix and suffix must not overlap in either string. A shrinking
	// replacement ("aa" -> "a") otherwise double-counts the shared region.
	if maxSuffix := min(len(oldFullText), len(newFullText)) - prefixLen; suffixLen > maxSuffix {
		if maxSuffix < 0 {
			maxSuffix = 0
		}
		suffixLen = maxSuffix
	}
	// Re-snap the clamped byte count to a rune boundary in both strings.
	for suffixLen > 0 && (!utf8.RuneStart(oldFullText[len(oldFullText)-suffixLen]) ||
		!utf8.RuneStart(newFullText[len(newFullText)-suffixLen])) {
		suffixLen--
	}

	oldMiddleStart := prefixLen
	newMiddleStart := prefixLen
	newMiddleEnd := len(newFullText) - suffixLen

	var newRuns []*oxml.CT_R

	// Prefix runs — runs entirely within the unchanged prefix are kept as-is;
	// the run straddling the prefix boundary is split.
	for i, r := range oldRuns {
		runEnd := boundaries[i+1]
		if runEnd <= prefixLen {
			newRuns = append(newRuns, r)
		} else if boundaries[i] < prefixLen {
			splitPoint := prefixLen - boundaries[i]
			newRuns = append(newRuns, r.CloneWithText(r.Text()[:splitPoint]))
			break
		} else {
			break
		}
	}

	// Middle (replacement) run, inheriting the first affected run's formatting.
	middleText := newFullText[newMiddleStart:newMiddleEnd]
	if len(middleText) > 0 {
		fmtRun := findRunAtPosition(oldRuns, boundaries, oldMiddleStart)
		newRuns = append(newRuns, fmtRun.CloneWithText(middleText))
	}

	// Suffix runs — runs entirely within the unchanged suffix are kept as-is;
	// the run straddling the suffix boundary is split.
	suffixStart := len(oldFullText) - suffixLen
	for i, r := range oldRuns {
		runStart := boundaries[i]
		runEnd := boundaries[i+1]
		if runStart >= suffixStart {
			newRuns = append(newRuns, r)
		} else if runEnd > suffixStart && runStart < suffixStart {
			splitPoint := suffixStart - runStart
			newRuns = append(newRuns, r.CloneWithText(r.Text()[splitPoint:]))
		}
	}

	if len(newRuns) == 0 {
		newRuns = []*oxml.CT_R{oldRuns[0].CloneWithText(newFullText)}
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

// findRunAtPosition finds the run that contains the character at the given
// position in the concatenated paragraph text.
func findRunAtPosition(runs []*oxml.CT_R, boundaries []int, pos int) *oxml.CT_R {
	for i, r := range runs {
		if pos >= boundaries[i] && pos < boundaries[i+1] {
			return r
		}
	}
	if len(runs) > 0 {
		return runs[len(runs)-1]
	}
	return nil
}
