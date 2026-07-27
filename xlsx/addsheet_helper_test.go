package xlsx

// addSheetT is the test-suite shorthand for AddSheet with a name the test
// already knows is legal and unused.
//
// AddSheet returns (*Sheet, error) since C440 — it no longer silently rewrites
// an invalid or duplicate name into something else — and the several hundred
// call sites in this suite all pass a constant, valid, unique name. Threading
// an error check through every one of them would bury what those tests are
// actually about, so the impossible error panics here instead: if it ever
// fires, AddSheet has regressed and the panic names the sheet.
//
// Tests that are ABOUT the validation call AddSheet directly and assert on the
// error; see TestAddSheetRejectsInvalidAndDuplicateNames.
func addSheetT(w *Workbook, name string) *Sheet {
	s, err := w.AddSheet(name)
	if err != nil {
		panic("addSheetT(" + name + "): " + err.Error())
	}
	return s
}
