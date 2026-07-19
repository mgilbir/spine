package docx

import (
	"strconv"

	"github.com/mgilbir/spine/docx/internal/oxml"
)

// oxmlToBorder converts an internal border element to the public Border, or nil
// when the source is nil. Width is returned in points (the source stores eighths
// of a point).
func oxmlToBorder(b *oxml.CT_Border) *Border {
	if b == nil {
		return nil
	}
	width := 0.0
	if b.Sz != "" {
		if v, err := strconv.ParseFloat(b.Sz, 64); err == nil {
			width = v / 8
		}
	}
	return &Border{Style: b.Val, Width: width, Color: b.Color}
}

// --- Page borders (w:pgBorders) ---

// PageBorders describes a section's page borders (w:pgBorders). Each side is nil
// when that edge has no border.
type PageBorders struct {
	// OffsetFrom selects the reference edge for border offsets: "page" measures
	// from the page edge, "text" from the text. Empty leaves the attribute unset.
	OffsetFrom               string
	Top, Left, Bottom, Right *Border
}

// PageBorders returns the section's page borders and whether a w:pgBorders
// element is present.
func (s *Section) PageBorders() (PageBorders, bool) {
	pb := s.sectPr.PgBorders
	if pb == nil {
		return PageBorders{}, false
	}
	return PageBorders{
		OffsetFrom: pb.OffsetFrom,
		Top:        oxmlToBorder(pb.Top),
		Left:       oxmlToBorder(pb.Left),
		Bottom:     oxmlToBorder(pb.Bottom),
		Right:      oxmlToBorder(pb.Right),
	}, true
}

// SetPageBorders sets the section's page borders, replacing any existing
// w:pgBorders element.
func (s *Section) SetPageBorders(b PageBorders) {
	s.sectPr.PgBorders = &oxml.CT_PgBorders{
		OffsetFrom: b.OffsetFrom,
		Top:        borderToOxml(b.Top),
		Left:       borderToOxml(b.Left),
		Bottom:     borderToOxml(b.Bottom),
		Right:      borderToOxml(b.Right),
	}
}

// ClearPageBorders removes the section's w:pgBorders element.
func (s *Section) ClearPageBorders() {
	s.sectPr.PgBorders = nil
}

// --- Line numbering (w:lnNumType) ---

// LineNumbering describes a section's line-numbering settings (w:lnNumType).
type LineNumbering struct {
	// CountBy is the increment between numbered lines (every CountBy-th line is
	// labeled). Zero means unset (Word treats it as every line).
	CountBy int
	// Start is the first line number.
	Start int
	// Distance is the gap between the numbers and the text, in points.
	Distance float64
	// Restart selects when the count resets: "newPage", "newSection", or
	// "continuous". Empty leaves the attribute unset.
	Restart string
}

// LineNumbering returns the section's line-numbering settings and whether a
// w:lnNumType element is present.
func (s *Section) LineNumbering() (LineNumbering, bool) {
	ln := s.sectPr.LnNumType
	if ln == nil {
		return LineNumbering{}, false
	}
	return LineNumbering{
		CountBy:  atoiOr(ln.CountBy, 0),
		Start:    atoiOr(ln.Start, 0),
		Distance: twipsToPoints(ln.Distance),
		Restart:  ln.Restart,
	}, true
}

// SetLineNumbering sets the section's line-numbering settings, replacing any
// existing w:lnNumType element. Zero-valued CountBy/Start/Distance are omitted.
func (s *Section) SetLineNumbering(ln LineNumbering) {
	el := &oxml.CT_LnNumType{Restart: ln.Restart}
	if ln.CountBy != 0 {
		el.CountBy = strconv.Itoa(ln.CountBy)
	}
	if ln.Start != 0 {
		el.Start = strconv.Itoa(ln.Start)
	}
	if ln.Distance != 0 {
		el.Distance = pointsToTwips(ln.Distance)
	}
	s.sectPr.LnNumType = el
}

// ClearLineNumbering removes the section's w:lnNumType element.
func (s *Section) ClearLineNumbering() {
	s.sectPr.LnNumType = nil
}

// --- Vertical alignment (w:vAlign) ---

// VerticalAlignment returns the section's vertical text alignment (w:vAlign):
// one of "top", "center", "both" (justified), or "bottom", or "" when unset.
func (s *Section) VerticalAlignment() string {
	if s.sectPr.VAlign == nil {
		return ""
	}
	return s.sectPr.VAlign.Val
}

// SetVerticalAlignment sets the section's vertical text alignment (w:vAlign).
// Valid values are "top", "center", "both", and "bottom"; passing "" removes
// the element.
func (s *Section) SetVerticalAlignment(align string) {
	if align == "" {
		s.sectPr.VAlign = nil
		return
	}
	s.sectPr.VAlign = &oxml.CT_String{Val: align}
}

// --- Paper source (w:paperSrc) ---

// PaperSource returns the printer paper-source bin numbers for the first page
// and the other pages (w:paperSrc), and whether the element is present.
func (s *Section) PaperSource() (first, other int, ok bool) {
	ps := s.sectPr.PaperSrc
	if ps == nil {
		return 0, 0, false
	}
	return atoiOr(ps.First, 0), atoiOr(ps.Other, 0), true
}

// SetPaperSource sets the printer paper-source bin numbers for the first page
// and the other pages (w:paperSrc), replacing any existing element.
func (s *Section) SetPaperSource(first, other int) {
	s.sectPr.PaperSrc = &oxml.CT_PaperSrc{
		First: strconv.Itoa(first),
		Other: strconv.Itoa(other),
	}
}

// ClearPaperSource removes the section's w:paperSrc element.
func (s *Section) ClearPaperSource() {
	s.sectPr.PaperSrc = nil
}

// --- Document grid (w:docGrid) ---

// DocumentGrid describes a section's document grid (w:docGrid), which governs
// the character/line grid used for East Asian layout.
type DocumentGrid struct {
	// Type selects the grid mode: "default", "lines", "linesAndChars", or
	// "snapToChars". Empty leaves the attribute unset.
	Type string
	// LinePitch is the grid line pitch in twips; CharSpace is the character
	// pitch adjustment. Both are raw grid units, not points.
	LinePitch int
	CharSpace int
}

// DocumentGrid returns the section's document grid and whether a w:docGrid
// element is present.
func (s *Section) DocumentGrid() (DocumentGrid, bool) {
	dg := s.sectPr.DocGrid
	if dg == nil {
		return DocumentGrid{}, false
	}
	return DocumentGrid{
		Type:      dg.Type,
		LinePitch: atoiOr(dg.LinePitch, 0),
		CharSpace: atoiOr(dg.CharSpace, 0),
	}, true
}

// SetDocumentGrid sets the section's document grid, replacing any existing
// w:docGrid element. A zero LinePitch/CharSpace is omitted.
func (s *Section) SetDocumentGrid(dg DocumentGrid) {
	el := &oxml.CT_DocGrid{Type: dg.Type}
	if dg.LinePitch != 0 {
		el.LinePitch = strconv.Itoa(dg.LinePitch)
	}
	if dg.CharSpace != 0 {
		el.CharSpace = strconv.Itoa(dg.CharSpace)
	}
	s.sectPr.DocGrid = el
}

// ClearDocumentGrid removes the section's w:docGrid element.
func (s *Section) ClearDocumentGrid() {
	s.sectPr.DocGrid = nil
}

// --- Footnote / endnote properties (w:footnotePr / w:endnotePr) ---

// NoteProperties describes the numbering of footnotes or endnotes for a section
// (w:footnotePr / w:endnotePr).
type NoteProperties struct {
	// Position selects where the notes are placed. For footnotes: "pageBottom",
	// "beneathText", "sectEnd", or "docEnd". For endnotes: "sectEnd" or "docEnd".
	// Empty leaves the element unset.
	Position string
	// NumberFormat is the w:numFmt value (e.g. "decimal", "lowerRoman",
	// "chicago"). Empty leaves it unset.
	NumberFormat string
	// NumberStart is the first note number (w:numStart); nil leaves it unset.
	NumberStart *int
	// Restart selects when the count resets (w:numRestart): "continuous",
	// "eachSect", or "eachPage". Empty leaves it unset.
	Restart string
}

// noteFromProps converts an internal note-properties element to NoteProperties.
func noteFromProps(pos, restart *oxml.CT_String, numFmt *oxml.CT_NumFmt, numStart *oxml.CT_DecimalNumber) NoteProperties {
	np := NoteProperties{}
	if pos != nil {
		np.Position = pos.Val
	}
	if numFmt != nil {
		np.NumberFormat = numFmt.Val
	}
	if numStart != nil {
		v := numStart.Val
		np.NumberStart = &v
	}
	if restart != nil {
		np.Restart = restart.Val
	}
	return np
}

// applyNote fills the shared note-properties fields from np, returning the four
// child elements (nil when the corresponding field is unset).
func applyNote(np NoteProperties) (pos, restart *oxml.CT_String, numFmt *oxml.CT_NumFmt, numStart *oxml.CT_DecimalNumber) {
	if np.Position != "" {
		pos = &oxml.CT_String{Val: np.Position}
	}
	if np.NumberFormat != "" {
		numFmt = &oxml.CT_NumFmt{Val: np.NumberFormat}
	}
	if np.NumberStart != nil {
		numStart = &oxml.CT_DecimalNumber{Val: *np.NumberStart}
	}
	if np.Restart != "" {
		restart = &oxml.CT_String{Val: np.Restart}
	}
	return pos, restart, numFmt, numStart
}

// FootnoteProperties returns the section's footnote numbering properties
// (w:footnotePr) and whether the element is present.
func (s *Section) FootnoteProperties() (NoteProperties, bool) {
	fp := s.sectPr.FootnoteProperties
	if fp == nil {
		return NoteProperties{}, false
	}
	return noteFromProps(fp.Pos, fp.NumRestart, fp.NumFmt, fp.NumStart), true
}

// SetFootnoteProperties sets the section's footnote numbering properties
// (w:footnotePr), replacing any existing element.
func (s *Section) SetFootnoteProperties(np NoteProperties) {
	pos, restart, numFmt, numStart := applyNote(np)
	s.sectPr.FootnoteProperties = &oxml.CT_FtnProps{
		Pos:        pos,
		NumFmt:     numFmt,
		NumStart:   numStart,
		NumRestart: restart,
	}
}

// ClearFootnoteProperties removes the section's w:footnotePr element.
func (s *Section) ClearFootnoteProperties() {
	s.sectPr.FootnoteProperties = nil
}

// EndnoteProperties returns the section's endnote numbering properties
// (w:endnotePr) and whether the element is present.
func (s *Section) EndnoteProperties() (NoteProperties, bool) {
	ep := s.sectPr.EndnoteProperties
	if ep == nil {
		return NoteProperties{}, false
	}
	return noteFromProps(ep.Pos, ep.NumRestart, ep.NumFmt, ep.NumStart), true
}

// SetEndnoteProperties sets the section's endnote numbering properties
// (w:endnotePr), replacing any existing element.
func (s *Section) SetEndnoteProperties(np NoteProperties) {
	pos, restart, numFmt, numStart := applyNote(np)
	s.sectPr.EndnoteProperties = &oxml.CT_EdnProps{
		Pos:        pos,
		NumFmt:     numFmt,
		NumStart:   numStart,
		NumRestart: restart,
	}
}

// ClearEndnoteProperties removes the section's w:endnotePr element.
func (s *Section) ClearEndnoteProperties() {
	s.sectPr.EndnoteProperties = nil
}
