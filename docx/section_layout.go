package docx

import (
	"strconv"

	"github.com/mgilbir/spine/docx/internal/oxml"
)

// Section type values (w:type/@w:val) for Section.SetSectionType. An empty
// string leaves the section type unset, which Word treats as "nextPage".
const (
	SectionTypeNextPage   = "nextPage"
	SectionTypeNextColumn = "nextColumn"
	SectionTypeContinuous = "continuous"
	SectionTypeEvenPage   = "evenPage"
	SectionTypeOddPage    = "oddPage"
)

// Page number format values (w:pgNumType/@w:fmt) for PageNumbering.Format.
const (
	PageNumberDecimal     = "decimal"
	PageNumberUpperRoman  = "upperRoman"
	PageNumberLowerRoman  = "lowerRoman"
	PageNumberUpperLetter = "upperLetter"
	PageNumberLowerLetter = "lowerLetter"
)

// Sections returns every section in the document in document order: one for
// each section break (a paragraph carrying w:pPr/w:sectPr) followed by the
// final body-level section. Unlike DefaultSection, it does not fabricate a
// body section when none exists; a well-formed document always ends with one.
func (d *Document) Sections() []*Section {
	if d.doc() == nil || d.doc().Body == nil {
		return nil
	}
	var result []*Section
	for _, p := range d.doc().Body.Paragraphs() {
		if p.PPr != nil && p.PPr.SectPr != nil {
			result = append(result, &Section{sectPr: p.PPr.SectPr})
		}
	}
	if d.doc().Body.SectPr != nil {
		result = append(result, &Section{sectPr: d.doc().Body.SectPr})
	}
	return result
}

// SectionType returns the section's start type (w:type): one of the
// SectionType* values, or "" when unset (Word treats unset as nextPage).
func (s *Section) SectionType() string {
	if s.sectPr.Type == nil {
		return ""
	}
	return s.sectPr.Type.Val
}

// SetSectionType sets the section's start type. Passing "" removes the w:type
// element, restoring Word's default (nextPage).
func (s *Section) SetSectionType(typ string) {
	if typ == "" {
		s.sectPr.Type = nil
		return
	}
	s.sectPr.Type = &oxml.CT_String{Val: typ}
}

// TitlePage reports whether the section has a distinct first-page header/footer
// (w:titlePg).
func (s *Section) TitlePage() bool {
	return s.sectPr.TitlePg.IsOn()
}

// SetTitlePage enables or disables the distinct first-page header/footer. When
// disabled, the w:titlePg element is removed.
func (s *Section) SetTitlePage(on bool) {
	if on {
		s.sectPr.TitlePg = &oxml.CT_OnOff{}
		return
	}
	s.sectPr.TitlePg = nil
}

// PageNumbering describes the section's page-number format and starting value
// (w:pgNumType). Format is one of the PageNumber* values (or "" for unset), and
// Start is the first page number for the section (nil when unset).
type PageNumbering struct {
	Format string
	Start  *int
}

// PageNumbering returns the section's page-numbering settings and whether a
// w:pgNumType element is present.
func (s *Section) PageNumbering() (PageNumbering, bool) {
	if s.sectPr.PgNumType == nil {
		return PageNumbering{}, false
	}
	pn := PageNumbering{Format: s.sectPr.PgNumType.Fmt}
	if s.sectPr.PgNumType.Start != "" {
		if v, err := strconv.Atoi(s.sectPr.PgNumType.Start); err == nil {
			pn.Start = &v
		}
	}
	return pn, true
}

// SetPageNumbering sets the section's page-number format and starting value,
// creating the w:pgNumType element if absent. A zero PageNumbering (empty
// Format and nil Start) still emits an empty w:pgNumType; use ClearPageNumbering
// to remove it.
func (s *Section) SetPageNumbering(pn PageNumbering) {
	if s.sectPr.PgNumType == nil {
		s.sectPr.PgNumType = &oxml.CT_PgNumType{}
	}
	s.sectPr.PgNumType.Fmt = pn.Format
	if pn.Start != nil {
		s.sectPr.PgNumType.Start = strconv.Itoa(*pn.Start)
	} else {
		s.sectPr.PgNumType.Start = ""
	}
}

// ClearPageNumbering removes the section's w:pgNumType element.
func (s *Section) ClearPageNumbering() {
	s.sectPr.PgNumType = nil
}

// Column describes a single explicit text column (w:col). Width and Spacing are
// in points; Spacing is the gap following the column.
type Column struct {
	Width   float64
	Spacing float64
}

// Columns describes a section's multi-column layout (w:cols). When EqualWidth is
// true (the default), Count equal columns are laid out with Spacing between
// them and the Cols slice is ignored. When EqualWidth is false, the Cols slice
// gives each column's explicit width and trailing spacing.
type Columns struct {
	Count      int
	Spacing    float64
	Separator  bool
	EqualWidth bool
	Cols       []Column
}

// Columns returns the section's multi-column layout and whether a w:cols element
// is present. A section with no w:cols is a single-column section.
func (s *Section) Columns() (Columns, bool) {
	if s.sectPr.Cols == nil {
		return Columns{}, false
	}
	c := s.sectPr.Cols
	out := Columns{
		Count:      atoiOr(c.Num, 1),
		Spacing:    twipsToPoints(c.Space),
		Separator:  isOnOffTrue(c.Sep),
		EqualWidth: c.EqualWidth == "" || isOnOffTrue(c.EqualWidth),
	}
	for _, col := range c.Col {
		out.Cols = append(out.Cols, Column{
			Width:   twipsToPoints(col.W),
			Spacing: twipsToPoints(col.Space),
		})
	}
	return out, true
}

// SetColumns sets the section's multi-column layout, replacing any existing
// w:cols element. Count defaults to 1 when non-positive; equal-width columns
// carry Spacing between them, while explicit-width columns are emitted from the
// Cols slice with EqualWidth="0".
func (s *Section) SetColumns(cols Columns) {
	c := &oxml.CT_Columns{}
	count := cols.Count
	if count < 1 {
		count = 1
	}
	c.Num = strconv.Itoa(count)
	if cols.Spacing > 0 {
		c.Space = pointsToTwips(cols.Spacing)
	}
	if cols.Separator {
		c.Sep = "1"
	}
	if !cols.EqualWidth && len(cols.Cols) > 0 {
		c.EqualWidth = "0"
		for _, col := range cols.Cols {
			cc := oxml.CT_Column{}
			if col.Width > 0 {
				cc.W = pointsToTwips(col.Width)
			}
			if col.Spacing > 0 {
				cc.Space = pointsToTwips(col.Spacing)
			}
			c.Col = append(c.Col, cc)
		}
	}
	s.sectPr.Cols = c
}

// ClearColumns removes the section's w:cols element (reverting to a single
// column).
func (s *Section) ClearColumns() {
	s.sectPr.Cols = nil
}

// atoiOr parses s as an int, returning def when s is empty or unparseable.
func atoiOr(s string, def int) int {
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return v
}

// isOnOffTrue interprets a WML on/off attribute string as a boolean.
func isOnOffTrue(v string) bool {
	return v == "1" || v == "true" || v == "on"
}
