package xlsx

import (
	"strings"

	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// Page orientation values for PageSetup.Orientation. An empty string leaves the
// attribute unset, which Excel treats as the default (portrait).
const (
	OrientationDefault   = "default"
	OrientationPortrait  = "portrait"
	OrientationLandscape = "landscape"
)

// Reserved (built-in) defined names Excel uses to record a sheet's print area
// and print titles. They are always scoped to a single sheet via localSheetId.
const (
	printAreaName   = "_xlnm.Print_Area"
	printTitlesName = "_xlnm.Print_Titles"
)

// PageSetup is the modeled subset of a worksheet's <pageSetup> element exposed
// through the public API. Pointer fields are unset when nil; SetPageSetup only
// writes the fields present here and leaves any other attributes on an existing
// element (printer relationship, DPI, copies, ...) untouched.
type PageSetup struct {
	// Orientation is "portrait", "landscape", "default", or "" (unset).
	Orientation     string
	PaperSize       *uint32
	Scale           *uint32
	FitToWidth      *uint32
	FitToHeight     *uint32
	FirstPageNumber *uint32
	BlackAndWhite   *bool
	Draft           *bool
}

// PageSetup returns the sheet's page-setup settings and whether a <pageSetup>
// element is present. When absent, the zero PageSetup and false are returned.
func (s *Sheet) PageSetup() (PageSetup, bool) {
	if s.worksheet == nil || s.worksheet.PageSetup == nil {
		return PageSetup{}, false
	}
	ps := s.worksheet.PageSetup
	return PageSetup{
		Orientation:     ps.Orientation,
		PaperSize:       cloneUint32(ps.PaperSize),
		Scale:           cloneUint32(ps.Scale),
		FitToWidth:      cloneUint32(ps.FitToWidth),
		FitToHeight:     cloneUint32(ps.FitToHeight),
		FirstPageNumber: cloneUint32(ps.FirstPageNumber),
		BlackAndWhite:   cloneBool(ps.BlackAndWhite),
		Draft:           cloneBool(ps.Draft),
	}, true
}

// SetPageSetup applies the given page-setup settings, creating the <pageSetup>
// element if the sheet lacks one. Only the modeled fields are written; other
// attributes on a pre-existing element are preserved, so a getter/modify/setter
// round-trip does not drop a printer-settings relationship or DPI values.
func (s *Sheet) SetPageSetup(ps PageSetup) {
	s.markDirty()
	s.ensureWorksheet()
	if s.worksheet.PageSetup == nil {
		s.worksheet.PageSetup = &oxml.CT_PageSetup{}
	}
	s.worksheet.EnsureChildOrder("pageSetup")
	cur := s.worksheet.PageSetup
	cur.Orientation = ps.Orientation
	cur.PaperSize = cloneUint32(ps.PaperSize)
	cur.Scale = cloneUint32(ps.Scale)
	cur.FitToWidth = cloneUint32(ps.FitToWidth)
	cur.FitToHeight = cloneUint32(ps.FitToHeight)
	cur.FirstPageNumber = cloneUint32(ps.FirstPageNumber)
	cur.BlackAndWhite = cloneBool(ps.BlackAndWhite)
	cur.Draft = cloneBool(ps.Draft)
}

// PageMargins is the set of page margins (in inches) exposed through the public
// API, mirroring the worksheet <pageMargins> element.
type PageMargins struct {
	Left   float64
	Right  float64
	Top    float64
	Bottom float64
	Header float64
	Footer float64
}

// PageMargins returns the sheet's page margins and whether a <pageMargins>
// element is present.
func (s *Sheet) PageMargins() (PageMargins, bool) {
	if s.worksheet == nil || s.worksheet.PageMargins == nil {
		return PageMargins{}, false
	}
	m := s.worksheet.PageMargins
	return PageMargins{
		Left:   m.Left,
		Right:  m.Right,
		Top:    m.Top,
		Bottom: m.Bottom,
		Header: m.Header,
		Footer: m.Footer,
	}, true
}

// SetPageMargins sets the sheet's page margins (in inches), creating the
// <pageMargins> element if absent.
func (s *Sheet) SetPageMargins(m PageMargins) {
	s.markDirty()
	s.ensureWorksheet()
	s.worksheet.EnsureChildOrder("pageMargins")
	s.worksheet.PageMargins = &oxml.CT_PageMargins{
		Left:   m.Left,
		Right:  m.Right,
		Top:    m.Top,
		Bottom: m.Bottom,
		Header: m.Header,
		Footer: m.Footer,
	}
}

// HeaderFooter is the modeled content of the worksheet <headerFooter> element:
// the header and footer strings (each using Excel's &L/&C/&R section codes) and
// the flags controlling which of them apply.
type HeaderFooter struct {
	DifferentOddEven *bool
	DifferentFirst   *bool
	ScaleWithDoc     *bool
	AlignWithMargins *bool
	OddHeader        string
	OddFooter        string
	EvenHeader       string
	EvenFooter       string
	FirstHeader      string
	FirstFooter      string
}

// HeaderFooter returns the sheet's header/footer settings and whether a
// <headerFooter> element is present.
func (s *Sheet) HeaderFooter() (HeaderFooter, bool) {
	if s.worksheet == nil || s.worksheet.HeaderFooter == nil {
		return HeaderFooter{}, false
	}
	hf := s.worksheet.HeaderFooter
	return HeaderFooter{
		DifferentOddEven: cloneBool(hf.DifferentOddEven),
		DifferentFirst:   cloneBool(hf.DifferentFirst),
		ScaleWithDoc:     cloneBool(hf.ScaleWithDoc),
		AlignWithMargins: cloneBool(hf.AlignWithMargins),
		OddHeader:        deref(hf.OddHeader),
		OddFooter:        deref(hf.OddFooter),
		EvenHeader:       deref(hf.EvenHeader),
		EvenFooter:       deref(hf.EvenFooter),
		FirstHeader:      deref(hf.FirstHeader),
		FirstFooter:      deref(hf.FirstFooter),
	}, true
}

// SetHeaderFooter sets the sheet's header/footer settings, replacing any
// existing <headerFooter> element. An empty header/footer string is treated as
// absent (the child element is omitted); a non-empty string emits the element.
func (s *Sheet) SetHeaderFooter(hf HeaderFooter) {
	s.markDirty()
	s.ensureWorksheet()
	s.worksheet.EnsureChildOrder("headerFooter")
	s.worksheet.HeaderFooter = &oxml.CT_HeaderFooter{
		DifferentOddEven: cloneBool(hf.DifferentOddEven),
		DifferentFirst:   cloneBool(hf.DifferentFirst),
		ScaleWithDoc:     cloneBool(hf.ScaleWithDoc),
		AlignWithMargins: cloneBool(hf.AlignWithMargins),
		OddHeader:        optString(hf.OddHeader),
		OddFooter:        optString(hf.OddFooter),
		EvenHeader:       optString(hf.EvenHeader),
		EvenFooter:       optString(hf.EvenFooter),
		FirstHeader:      optString(hf.FirstHeader),
		FirstFooter:      optString(hf.FirstFooter),
	}
}

// PrintOptions is the modeled content of the worksheet <printOptions> element:
// whether gridlines and row/column headings print and how the sheet is centered
// on the page.
type PrintOptions struct {
	HorizontalCentered *bool
	VerticalCentered   *bool
	Headings           *bool
	GridLines          *bool
	GridLinesSet       *bool
}

// PrintOptions returns the sheet's print options and whether a <printOptions>
// element is present.
func (s *Sheet) PrintOptions() (PrintOptions, bool) {
	if s.worksheet == nil || s.worksheet.PrintOptions == nil {
		return PrintOptions{}, false
	}
	po := s.worksheet.PrintOptions
	return PrintOptions{
		HorizontalCentered: cloneBool(po.HorizontalCentered),
		VerticalCentered:   cloneBool(po.VerticalCentered),
		Headings:           cloneBool(po.Headings),
		GridLines:          cloneBool(po.GridLines),
		GridLinesSet:       cloneBool(po.GridLinesSet),
	}, true
}

// SetPrintOptions sets the sheet's print options, replacing any existing
// <printOptions> element.
func (s *Sheet) SetPrintOptions(po PrintOptions) {
	s.markDirty()
	s.ensureWorksheet()
	s.worksheet.EnsureChildOrder("printOptions")
	s.worksheet.PrintOptions = &oxml.CT_PrintOptions{
		HorizontalCentered: cloneBool(po.HorizontalCentered),
		VerticalCentered:   cloneBool(po.VerticalCentered),
		Headings:           cloneBool(po.Headings),
		GridLines:          cloneBool(po.GridLines),
		GridLinesSet:       cloneBool(po.GridLinesSet),
	}
}

// ---------------------------------------------------------------------------
// Print area & print titles (reserved defined names)
// ---------------------------------------------------------------------------

// SetPrintArea sets the sheet's print area from one or more A1-style ranges
// (e.g. "A1:D20"). Each range is made absolute and qualified with the sheet
// name, then stored in the reserved _xlnm.Print_Area defined name scoped to
// this sheet. Passing no ranges clears the print area.
func (s *Sheet) SetPrintArea(ranges ...string) error {
	if s.workbook == nil {
		return ErrNoWorkbook
	}
	if len(ranges) == 0 {
		s.workbook.clearReservedName(printAreaName, s.index)
		return nil
	}
	prefix := s.sheetRefPrefix()
	parts := make([]string, 0, len(ranges))
	for _, r := range ranges {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}
		parts = append(parts, prefix+absolutizeRange(r))
	}
	if len(parts) == 0 {
		s.workbook.clearReservedName(printAreaName, s.index)
		return nil
	}
	s.workbook.setReservedName(printAreaName, s.index, strings.Join(parts, ","))
	return nil
}

// PrintArea returns the raw value of the sheet's _xlnm.Print_Area defined name
// (e.g. "Sheet1!$A$1:$D$20"), or "" when no print area is set.
func (s *Sheet) PrintArea() string {
	if s.workbook == nil {
		return ""
	}
	v, _ := s.workbook.reservedName(printAreaName, s.index)
	return v
}

// ClearPrintArea removes the sheet's print area.
func (s *Sheet) ClearPrintArea() {
	if s.workbook != nil {
		s.workbook.clearReservedName(printAreaName, s.index)
	}
}

// SetPrintTitles sets the rows and/or columns that repeat on every printed page.
// rows is a row range such as "1:1" (repeat the first row) and cols is a column
// range such as "A:B"; either may be empty to leave that dimension unset. The
// value is stored in the reserved _xlnm.Print_Titles defined name scoped to this
// sheet. Passing both empty clears the print titles.
func (s *Sheet) SetPrintTitles(rows, cols string) error {
	if s.workbook == nil {
		return ErrNoWorkbook
	}
	rows = strings.TrimSpace(rows)
	cols = strings.TrimSpace(cols)
	if rows == "" && cols == "" {
		s.workbook.clearReservedName(printTitlesName, s.index)
		return nil
	}
	prefix := s.sheetRefPrefix()
	var parts []string
	// Excel writes the column titles before the row titles.
	if cols != "" {
		parts = append(parts, prefix+absolutizeRange(cols))
	}
	if rows != "" {
		parts = append(parts, prefix+absolutizeRange(rows))
	}
	s.workbook.setReservedName(printTitlesName, s.index, strings.Join(parts, ","))
	return nil
}

// PrintTitles returns the raw value of the sheet's _xlnm.Print_Titles defined
// name (e.g. "Sheet1!$A:$B,Sheet1!$1:$1"), or "" when no print titles are set.
func (s *Sheet) PrintTitles() string {
	if s.workbook == nil {
		return ""
	}
	v, _ := s.workbook.reservedName(printTitlesName, s.index)
	return v
}

// ClearPrintTitles removes the sheet's print titles.
func (s *Sheet) ClearPrintTitles() {
	if s.workbook != nil {
		s.workbook.clearReservedName(printTitlesName, s.index)
	}
}

// sheetRefPrefix returns the sheet-qualified prefix used in defined-name values
// (e.g. "Sheet1!" or "'My Sheet'!"), quoting the sheet name when required.
func (s *Sheet) sheetRefPrefix() string {
	return quoteSheetName(s.name) + "!"
}

// ---------------------------------------------------------------------------
// Reserved defined-name helpers on Workbook
// ---------------------------------------------------------------------------

// reservedName returns the value of a reserved defined name scoped to the given
// sheet index, and whether it exists.
func (w *Workbook) reservedName(name string, sheetIndex int) (string, bool) {
	if w.workbook.DefinedNames == nil {
		return "", false
	}
	for _, dn := range w.workbook.DefinedNames.DefinedName {
		if dn.Name == name && dn.LocalSheetId != nil && int(*dn.LocalSheetId) == sheetIndex {
			return dn.Value, true
		}
	}
	return "", false
}

// setReservedName sets (or replaces) a reserved defined name scoped to the given
// sheet index.
func (w *Workbook) setReservedName(name string, sheetIndex int, value string) {
	if w.workbook.DefinedNames == nil {
		w.workbook.DefinedNames = &oxml.CT_DefinedNames{}
	}
	// Workbook marshaling is ChildOrder-gated for opened files: a definedNames
	// element the original lacked must be inserted at its schema position (C12).
	w.workbook.EnsureChildOrder("definedNames")
	for i := range w.workbook.DefinedNames.DefinedName {
		dn := &w.workbook.DefinedNames.DefinedName[i]
		if dn.Name == name && dn.LocalSheetId != nil && int(*dn.LocalSheetId) == sheetIndex {
			dn.Value = value
			return
		}
	}
	idx := uint32(sheetIndex)
	w.workbook.DefinedNames.DefinedName = append(w.workbook.DefinedNames.DefinedName, oxml.CT_DefinedName{
		Name:         name,
		Value:        value,
		LocalSheetId: &idx,
	})
}

// clearReservedName removes a reserved defined name scoped to the given sheet
// index, if present.
func (w *Workbook) clearReservedName(name string, sheetIndex int) {
	if w.workbook.DefinedNames == nil {
		return
	}
	names := w.workbook.DefinedNames.DefinedName
	kept := names[:0]
	for _, dn := range names {
		if dn.Name == name && dn.LocalSheetId != nil && int(*dn.LocalSheetId) == sheetIndex {
			continue
		}
		kept = append(kept, dn)
	}
	w.workbook.DefinedNames.DefinedName = kept
	if len(kept) == 0 {
		w.workbook.DefinedNames = nil
	}
}

// ---------------------------------------------------------------------------
// Range/name formatting helpers
// ---------------------------------------------------------------------------

// quoteSheetName wraps a sheet name in single quotes when Excel requires it —
// any name that is not a simple identifier — doubling embedded single quotes.
func quoteSheetName(name string) string {
	simple := name != ""
	for i, r := range name {
		isLetter := r == '_' || (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z')
		isDigit := r >= '0' && r <= '9'
		if isLetter || (i > 0 && (isDigit || r == '.')) {
			continue
		}
		simple = false
		break
	}
	if simple {
		return name
	}
	return "'" + strings.ReplaceAll(name, "'", "''") + "'"
}

// absolutizeRange converts an A1-style range or single reference into its
// absolute form ("A1:D10" -> "$A$1:$D$10", "1:1" -> "$1:$1", "A:B" -> "$A:$B").
func absolutizeRange(rng string) string {
	start, end, ok := strings.Cut(rng, ":")
	if !ok {
		return absolutizeRef(rng)
	}
	return absolutizeRef(start) + ":" + absolutizeRef(end)
}

// absolutizeRef makes a single cell/row/column reference absolute, stripping any
// existing '$' signs first so already-absolute input is not double-prefixed.
func absolutizeRef(ref string) string {
	ref = strings.ReplaceAll(ref, "$", "")
	i := 0
	for i < len(ref) && ((ref[i] >= 'A' && ref[i] <= 'Z') || (ref[i] >= 'a' && ref[i] <= 'z')) {
		i++
	}
	letters := strings.ToUpper(ref[:i])
	digits := ref[i:]
	var b strings.Builder
	if letters != "" {
		b.WriteByte('$')
		b.WriteString(letters)
	}
	if digits != "" {
		b.WriteByte('$')
		b.WriteString(digits)
	}
	return b.String()
}

// ---------------------------------------------------------------------------
// small pointer helpers
// ---------------------------------------------------------------------------

func cloneUint32(v *uint32) *uint32 {
	if v == nil {
		return nil
	}
	c := *v
	return &c
}

func cloneBool(v *bool) *bool {
	if v == nil {
		return nil
	}
	c := *v
	return &c
}

func deref(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}

func optString(v string) *string {
	if v == "" {
		return nil
	}
	c := v
	return &c
}
