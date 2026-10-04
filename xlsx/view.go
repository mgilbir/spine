package xlsx

import (
	"github.com/mgilbir/spine/xlsx/internal/oxml"
	"github.com/mgilbir/spine/xlsx/internal/view"
)

// The range renderer, xlsx/xlsxrender, reads the model through
// xlsx/internal/view; see that package.
func init() {
	view.SheetOf = func(v any) *view.Sheet { return v.(*Sheet).view() }
	view.DefaultStylesheet = defaultStylesheet
	view.ParseRange = func(ref string) (int, int, int, int, error) {
		r, err := parseCellRangeRef(ref)
		return r.minRow, r.minCol, r.maxRow, r.maxCol, err
	}
}

// view snapshots the sheet for the renderer.
func (s *Sheet) view() *view.Sheet {
	w := s.workbook
	if w == nil {
		return nil
	}
	return &view.Sheet{Opaque: s.opaque, PartName: s.partName, WorkbookPart: w.mainPartName, Relationships: w.relationships[w.mainPartName],
		PreservedParts: w.preservedParts, Stylesheet: w.stylesheet, SharedStrings: w.sharedStrings, Load: func() *oxml.CT_Worksheet { return s.ws() }}
}
