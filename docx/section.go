package docx

import (
	"github.com/mgilbir/spine/docx/internal/oxml"
)

// Section represents a document section with page layout properties.
type Section struct {
	sectPr *oxml.CT_SectPr
}

// Orientation represents page orientation.
type Orientation int

const (
	OrientationPortrait Orientation = iota
	OrientationLandscape
)

// PageMargins represents the page margins in points.
type PageMargins struct {
	Top, Bottom, Left, Right float64
	Header, Footer           float64
}

// PageSize returns the page width and height in points.
func (s *Section) PageSize() (width, height float64) {
	if s.sectPr.PgSz == nil {
		w, h := PageSizeLetter()
		return w, h
	}
	return twipsToPoints(s.sectPr.PgSz.W), twipsToPoints(s.sectPr.PgSz.H)
}

// SetPageSize sets the page width and height in points.
func (s *Section) SetPageSize(width, height float64) {
	if s.sectPr.PgSz == nil {
		s.sectPr.PgSz = &oxml.CT_PgSz{}
	}
	s.sectPr.PgSz.W = pointsToTwips(width)
	s.sectPr.PgSz.H = pointsToTwips(height)
}

// Orientation returns the page orientation.
func (s *Section) Orientation() Orientation {
	if s.sectPr.PgSz != nil && s.sectPr.PgSz.Orient == "landscape" {
		return OrientationLandscape
	}
	return OrientationPortrait
}

// SetOrientation sets the page orientation and swaps dimensions if needed.
func (s *Section) SetOrientation(orient Orientation) {
	if s.sectPr.PgSz == nil {
		s.sectPr.PgSz = &oxml.CT_PgSz{}
		w, h := PageSizeLetter()
		s.sectPr.PgSz.W = pointsToTwips(w)
		s.sectPr.PgSz.H = pointsToTwips(h)
	}

	current := s.Orientation()
	if orient == current {
		return
	}

	// Swap width and height
	w := s.sectPr.PgSz.W
	s.sectPr.PgSz.W = s.sectPr.PgSz.H
	s.sectPr.PgSz.H = w

	if orient == OrientationLandscape {
		s.sectPr.PgSz.Orient = "landscape"
	} else {
		s.sectPr.PgSz.Orient = ""
	}
}

// Margins returns the page margins in points.
func (s *Section) Margins() PageMargins {
	if s.sectPr.PgMar == nil {
		return PageMargins{}
	}
	m := s.sectPr.PgMar
	return PageMargins{
		Top:    twipsToPoints(m.Top),
		Bottom: twipsToPoints(m.Bottom),
		Left:   twipsToPoints(m.Left),
		Right:  twipsToPoints(m.Right),
		Header: twipsToPoints(m.Header),
		Footer: twipsToPoints(m.Footer),
	}
}

// SetMargins sets the page margins in points.
func (s *Section) SetMargins(m PageMargins) {
	if s.sectPr.PgMar == nil {
		s.sectPr.PgMar = &oxml.CT_PgMar{}
	}
	s.sectPr.PgMar.Top = pointsToTwips(m.Top)
	s.sectPr.PgMar.Bottom = pointsToTwips(m.Bottom)
	s.sectPr.PgMar.Left = pointsToTwips(m.Left)
	s.sectPr.PgMar.Right = pointsToTwips(m.Right)
	if m.Header > 0 {
		s.sectPr.PgMar.Header = pointsToTwips(m.Header)
	}
	if m.Footer > 0 {
		s.sectPr.PgMar.Footer = pointsToTwips(m.Footer)
	}
}

// Predefined page sizes in points.

// PageSizeLetter returns US Letter size (8.5 x 11 inches) in points.
func PageSizeLetter() (float64, float64) { return 612, 792 }

// PageSizeA4 returns A4 size (210 x 297 mm) in points.
func PageSizeA4() (float64, float64) { return 595.3, 841.9 }

// PageSizeLegal returns US Legal size (8.5 x 14 inches) in points.
func PageSizeLegal() (float64, float64) { return 612, 1008 }
