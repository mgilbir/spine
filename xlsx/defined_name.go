package xlsx

import (
	"fmt"
	"strings"
	"unicode"

	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// maxDefinedNameLen is Excel's limit on the length of a defined name.
const maxDefinedNameLen = 255

// ValidateDefinedName reports whether name is a legal Excel defined name.
// Excel refuses names that collide with an A1- or R1C1-style cell reference,
// names containing spaces or other characters outside letters, digits, ".",
// "_" and "\", names that do not begin with a letter, "_" or "\", and names
// longer than 255 characters. Such a name is accepted by the file format but
// rejected by Excel when the workbook is opened (C426).
func ValidateDefinedName(name string) error {
	if name == "" {
		return fmt.Errorf("xlsx: defined name must not be empty")
	}
	if len([]rune(name)) > maxDefinedNameLen {
		return fmt.Errorf("xlsx: defined name %q is longer than %d characters", name, maxDefinedNameLen)
	}
	for i, r := range name {
		if i == 0 {
			if !unicode.IsLetter(r) && r != '_' && r != '\\' {
				return fmt.Errorf("xlsx: defined name %q must start with a letter, underscore or backslash", name)
			}
			continue
		}
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_' && r != '.' && r != '\\' {
			return fmt.Errorf("xlsx: defined name %q contains the illegal character %q", name, r)
		}
	}
	if looksLikeCellReference(name) {
		return fmt.Errorf("xlsx: defined name %q collides with a cell reference", name)
	}
	return nil
}

// looksLikeCellReference reports whether name would be read as a cell
// reference — an A1-style reference inside the worksheet grid, or an
// R1C1-style one (including the bare "R"/"C" shorthands).
func looksLikeCellReference(name string) bool {
	if _, _, err := ParseCellRef(name); err == nil {
		return true
	}
	upper := strings.ToUpper(name)
	if upper == "R" || upper == "C" {
		return true
	}
	// R1C1 forms: R<digits>C<digits>, with either index optionally absent
	// (R1C, RC1, RC are all relative references Excel understands).
	if len(upper) >= 2 && upper[0] == 'R' {
		rest := upper[1:]
		digits := 0
		for digits < len(rest) && rest[digits] >= '0' && rest[digits] <= '9' {
			digits++
		}
		rest = rest[digits:]
		if rest != "" && rest[0] == 'C' {
			rest = rest[1:]
			for rest != "" && rest[0] >= '0' && rest[0] <= '9' {
				rest = rest[1:]
			}
			return rest == ""
		}
	}
	return false
}

// checkDefinedNameCollision reports an error when a defined name with the same
// name (Excel compares names case-insensitively) already exists in the same
// scope; sheetIndex -1 is workbook scope. Excel refuses to open a workbook
// carrying duplicate name/scope pairs (C426).
func (w *Workbook) checkDefinedNameCollision(name string, sheetIndex int) error {
	if w.workbook == nil || w.workbook.DefinedNames == nil {
		return nil
	}
	for _, dn := range w.workbook.DefinedNames.DefinedName {
		scope := -1
		if dn.LocalSheetId != nil {
			scope = int(*dn.LocalSheetId)
		}
		if scope == sheetIndex && strings.EqualFold(dn.Name, name) {
			if sheetIndex < 0 {
				return fmt.Errorf("xlsx: workbook-scoped defined name %q already exists", name)
			}
			return fmt.Errorf("xlsx: defined name %q already exists on sheet %d", name, sheetIndex)
		}
	}
	return nil
}

// AddDefinedNameFull adds a defined name carrying the full set of attributes
// (scope plus the hidden flag, comment and description). SheetIndex -1 makes
// the name workbook-scoped; a valid sheet index makes it sheet-scoped. It is
// the richer counterpart to AddDefinedName / AddDefinedNameScoped.
//
// The name is validated (see ValidateDefinedName) and must not duplicate an
// existing name in the same scope.
func (w *Workbook) AddDefinedNameFull(dn DefinedName) error {
	if dn.SheetIndex >= 0 && dn.SheetIndex >= len(w.sheets) {
		return ErrSheetIndex
	}
	if err := ValidateDefinedName(dn.Name); err != nil {
		return err
	}
	scope := dn.SheetIndex
	if scope < 0 {
		scope = -1
	}
	if err := w.checkDefinedNameCollision(dn.Name, scope); err != nil {
		return err
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
