package docxrender

import (
	"strconv"
	"strings"

	"github.com/mgilbir/spine/render"
)

// Fields.
//
// A field's cached result is drawn as written, except for the page-dependent
// fields, whose value only pagination knows: PAGE, NUMPAGES, SECTIONPAGES and
// SECTION (and the pgNum run placeholder) are drawn with their real values in
// headers and footers, which are translated once for each distinct set of
// values (render_headers.go). In the body their value would depend on the
// pagination of the very text they sit in, so the saved result is drawn and
// reported as approximated.

// wordFieldState is one open complex field. It is in its instruction until its
// separator and in its result afterwards.
type wordFieldState struct {
	result bool
	// instr is the start of the instruction text.
	instr string
	// subst says the result is replaced by text, drawn once (at the first text
	// of the result, or at the field's end when it has none).
	subst bool
	done  bool
	text  string
	// check is set for a page number field in the body, whose saved result is
	// checked against the page it lands on; mark is its tag in the layout.
	check *wordFieldCheck
	mark  int
}

// wordFieldCheck is a page-dependent field in the body: its instruction and
// the result Word saved, which is drawn as it is. Once pages are known the
// result is compared with what the field shows there.
type wordFieldCheck struct {
	info   wordFieldInfo
	cached string
}

// checkField is the innermost open field with a result to check.
func (f *wordFlow) checkField() *wordFieldState {
	for i := len(f.fields) - 1; i >= 0; i-- {
		if f.fields[i].check != nil {
			return &f.fields[i]
		}
	}
	return nil
}

// wordMaxInstr bounds the instruction text kept: field names and switches are
// at its start.
const wordMaxInstr = 512

// substField is the innermost open field whose result is replaced.
func (f *wordFlow) substField() *wordFieldState {
	for i := len(f.fields) - 1; i >= 0; i-- {
		if f.fields[i].subst {
			return &f.fields[i]
		}
	}
	return nil
}

// wordFieldInfo is a parsed field instruction.
type wordFieldInfo struct {
	name string
	// format is the general format switch (\* roman, ...), without the
	// formatting-preserving ones.
	format string
	// other is set when the instruction has switches the profile does not draw
	// (numeric pictures, \h, ...).
	other bool
}

// wordParseField reads a field instruction: the upper-cased name and the
// switches that change a page-dependent field's value.
func wordParseField(instr string) wordFieldInfo {
	args := strings.Fields(instr)
	var info wordFieldInfo
	if len(args) == 0 {
		return info
	}
	info.name = strings.ToUpper(args[0])
	for i := 1; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, `\`) {
			// A bookmark or other argument.
			info.other = true
			continue
		}
		if a != `\*` {
			info.other = true
			continue
		}
		if i+1 >= len(args) {
			info.other = true
			break
		}
		i++
		switch sw := args[i]; strings.ToLower(sw) {
		case "mergeformat", "charformat":
		default:
			if info.format != "" {
				info.other = true
			}
			info.format = sw
		}
	}
	return info
}

// wordPageField reports whether a field's value is the page numbering's.
func wordPageField(name string) bool {
	switch name {
	case "PAGE", "NUMPAGES", "SECTIONPAGES", "SECTION":
		return true
	}
	return false
}

// wordPageVals are the values the page-dependent fields take on one page.
type wordPageVals struct {
	// page is the displayed page number, in the section's format pageFmt.
	page    int
	pageFmt string
	// total is the number of physical pages; secPages those of the section;
	// sec the section's 1-based number.
	total, secPages, sec int
}

// Bits of wordHFCtx.uses: which values a header or footer shows.
const (
	wordUsePage = 1 << iota
	wordUseTotal
	wordUseSecPages
	wordUseSec
)

// wordHFCtx is the state of translating a header or footer for one page.
type wordHFCtx struct {
	vals wordPageVals
	// uses collects the kinds of values the part drew.
	uses uint8
}

// pageText is the page number in the section's format.
func (h *wordHFCtx) pageText() (string, bool) {
	h.uses |= wordUsePage
	return wordNumberText(h.vals.page, h.vals.pageFmt)
}

// fieldValue is the value of a page-dependent field and whether it is exactly
// what Word draws.
func (h *wordHFCtx) fieldValue(info wordFieldInfo) (string, bool) {
	format, exact := "", !info.other
	n := 0
	switch info.name {
	case "PAGE":
		h.uses |= wordUsePage
		n, format = h.vals.page, h.vals.pageFmt
	case "NUMPAGES":
		h.uses |= wordUseTotal
		n = h.vals.total
	case "SECTIONPAGES":
		h.uses |= wordUseSecPages
		n = h.vals.secPages
	case "SECTION":
		h.uses |= wordUseSec
		n = h.vals.sec
	}
	switch info.format {
	case "":
	case "Arabic":
		format = "decimal"
	case "roman":
		format = "lowerRoman"
	case "ROMAN":
		format = "upperRoman"
	case "alphabetic":
		format = "lowerLetter"
	case "ALPHABETIC":
		format = "upperLetter"
	default:
		exact = false
	}
	s, ok := wordNumberText(n, format)
	return s, exact && ok
}

// key identifies the values a part shows, for caching its layout.
func (v wordPageVals) key(uses uint8) string {
	var sb strings.Builder
	if uses&wordUsePage != 0 {
		sb.WriteString("p" + strconv.Itoa(v.page) + v.pageFmt + ";")
	}
	if uses&wordUseTotal != 0 {
		sb.WriteString("n" + strconv.Itoa(v.total) + ";")
	}
	if uses&wordUseSecPages != 0 {
		sb.WriteString("s" + strconv.Itoa(v.secPages) + ";")
	}
	if uses&wordUseSec != 0 {
		sb.WriteString("S" + strconv.Itoa(v.sec) + ";")
	}
	return sb.String()
}

// startFieldResult is called when a field enters its result: it parses the
// instruction and arranges the replacement of a page-dependent field's value.
func (f *wordFlow) startFieldResult(s *wordFieldState) error {
	info := wordParseField(s.instr)
	if !wordPageField(info.name) {
		return nil
	}
	if f.hf == nil {
		if f.note != nil || !f.visible() {
			return nil
		}
		notes := f.r.notes
		if len(notes.marks) >= f.r.limits.MaxOperations {
			return render.ErrLimit
		}
		check := &wordFieldCheck{info: info}
		notes.marks = append(notes.marks, &wordMark{ref: &wordNoteRef{page: -1, field: check}})
		s.check, s.mark = check, len(notes.marks)
		return nil
	}
	text, exact := f.hf.fieldValue(info)
	s.subst, s.text = true, text
	if !exact && f.visible() {
		return f.r.approximate("page number field format")
	}
	return nil
}
