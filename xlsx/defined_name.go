package xlsx

import "github.com/mgilbir/spine/xlsx/internal/oxml"

// AddDefinedNameFull adds a defined name carrying the full set of attributes
// (scope plus the hidden flag, comment and description). SheetIndex -1 makes
// the name workbook-scoped; a valid sheet index makes it sheet-scoped. It is
// the richer counterpart to AddDefinedName / AddDefinedNameScoped.
func (w *Workbook) AddDefinedNameFull(dn DefinedName) error {
	if dn.SheetIndex >= 0 && dn.SheetIndex >= len(w.sheets) {
		return ErrSheetIndex
	}
	if w.workbook.DefinedNames == nil {
		w.workbook.DefinedNames = &oxml.CT_DefinedNames{}
	}
	w.workbook.EnsureChildOrder("definedNames")

	out := oxml.CT_DefinedName{
		Name:        dn.Name,
		Value:       dn.Value,
		Comment:     dn.Comment,
		Description: dn.Description,
	}
	if dn.SheetIndex >= 0 {
		idx := uint32(dn.SheetIndex)
		out.LocalSheetId = &idx
	}
	if dn.Hidden {
		out.Hidden = oxml.NewBoolLex(true)
	}
	w.workbook.DefinedNames.DefinedName = append(w.workbook.DefinedNames.DefinedName, out)
	return nil
}

// RemoveDefinedName removes every workbook-scoped defined name with the given
// name and reports whether any were removed.
func (w *Workbook) RemoveDefinedName(name string) bool {
	return w.removeDefinedName(name, -1)
}

// RemoveDefinedNameScoped removes the sheet-scoped defined name with the given
// name on the given sheet and reports whether it was removed.
func (w *Workbook) RemoveDefinedNameScoped(name string, sheetIndex int) bool {
	return w.removeDefinedName(name, sheetIndex)
}

// removeDefinedName drops defined names matching name and scope (sheetIndex -1
// for workbook scope) and returns whether the list changed.
func (w *Workbook) removeDefinedName(name string, sheetIndex int) bool {
	if w.workbook.DefinedNames == nil {
		return false
	}
	names := w.workbook.DefinedNames.DefinedName
	kept := names[:0:0]
	removed := false
	for _, dn := range names {
		scope := -1
		if dn.LocalSheetId != nil {
			scope = int(*dn.LocalSheetId)
		}
		if dn.Name == name && scope == sheetIndex {
			removed = true
			continue
		}
		kept = append(kept, dn)
	}
	if !removed {
		return false
	}
	if len(kept) == 0 {
		w.workbook.DefinedNames = nil
	} else {
		w.workbook.DefinedNames.DefinedName = kept
	}
	return true
}
