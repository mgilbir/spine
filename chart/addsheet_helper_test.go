package chart_test

import "github.com/mgilbir/spine/xlsx"

// addSheetT is the shorthand for xlsx.Workbook.AddSheet with a name the test
// already knows is legal and unused. AddSheet returns (*Sheet, error) since
// C440; these tests are about charts, not sheet naming, so the impossible error
// panics rather than obscuring every call site.
func addSheetT(w *xlsx.Workbook, name string) *xlsx.Sheet {
	s, err := w.AddSheet(name)
	if err != nil {
		panic("addSheetT(" + name + "): " + err.Error())
	}
	return s
}
