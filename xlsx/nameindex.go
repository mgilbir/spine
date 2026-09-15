package xlsx

import (
	"strconv"
	"strings"
	"unicode"

	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// foldKey returns a canonical form under which two strings compare equal
// exactly when strings.EqualFold says they do.
//
// The collision checks below all compared case-insensitively with EqualFold,
// and replacing a scan with a map means keying it. strings.ToLower is the
// obvious key and is subtly not the same relation: EqualFold("ſ", "s") is true
// (simple case folding maps the long s onto s) while ToLower leaves "ſ" alone,
// so a ToLower key would stop seeing a collision the scan used to report.
// Mapping each rune to the smallest member of its fold orbit reproduces
// EqualFold's relation exactly, and preserves rune count, which EqualFold also
// requires.
func foldKey(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		min := r
		for f := unicode.SimpleFold(r); f != r; f = unicode.SimpleFold(f) {
			if f < min {
				min = f
			}
		}
		b.WriteRune(min)
	}
	return b.String()
}

// sheetNameSet indexes existing sheet names for the case-insensitive collision
// check AddSheet and UniqueSheetName perform.
//
// Both ran a scan of every sheet per call, so adding sheets cost O(sheets^2):
// 500 sheets took 1.5ms and 2000 took 14.7ms. UniqueSheetName is worse than it
// looks — its suffix loop asks again for every candidate it tries.
//
// The set records how many sheets it was built from and rebuilds when that
// moves, so an add or a delete that does not maintain it costs a rebuild rather
// than a wrong answer. A rename leaves the count alone, so Sheet.SetName drops
// the set explicitly.
type sheetNameSet struct {
	sheets int
	names  map[string]bool
}

func (w *Workbook) sheetNameSetFor() *sheetNameSet {
	if w.sheetNames != nil && w.sheetNames.sheets == len(w.sheets) {
		return w.sheetNames
	}
	w.nameSetRebuilds++
	names := make(map[string]bool, len(w.sheets))
	for _, sh := range w.sheets {
		names[foldKey(sh.name)] = true
	}
	w.sheetNames = &sheetNameSet{sheets: len(w.sheets), names: names}
	return w.sheetNames
}

// recordSheetName adds a name to the cached set and accounts for the sheet that
// now carries it, so a run of AddSheet calls does not rebuild the set per add.
// Take the set before appending the sheet: it has to be in sync with w.sheets
// at the moment it is fetched, or fetching it rebuilds.
func (w *Workbook) recordSheetName(set *sheetNameSet, name string) {
	set.names[foldKey(name)] = true
	set.sheets = len(w.sheets)
}

// invalidateSheetNames drops the cached sheet-name set. Call it after a rename,
// which changes a name without changing how many sheets there are.
func (w *Workbook) invalidateSheetNames() {
	w.sheetNames = nil
}

// definedNameSet indexes defined names by scope for checkDefinedNameCollision,
// which scanned every existing name per call: 1000 names took 4.5ms and 4000
// took 63.4ms.
//
// Defined names are never renamed in place — every mutation appends to or
// replaces the slice — so tracking its length is enough to notice any change.
type definedNameSet struct {
	names int
	keys  map[string]bool
}

// definedNameKey combines the scope with the folded name. A sheet-scoped name
// only collides with another name in the same scope, which is what the scan
// compared.
func definedNameKey(name string, sheetIndex int) string {
	return strconv.Itoa(sheetIndex) + "\x00" + foldKey(name)
}

func (w *Workbook) definedNameSetFor() *definedNameSet {
	n := 0
	if w.workbook != nil && w.workbook.DefinedNames != nil {
		n = len(w.workbook.DefinedNames.DefinedName)
	}
	if w.definedNames != nil && w.definedNames.names == n {
		return w.definedNames
	}
	w.nameSetRebuilds++
	keys := make(map[string]bool, n)
	if n > 0 {
		for _, dn := range w.workbook.DefinedNames.DefinedName {
			scope := -1
			if dn.LocalSheetId != nil {
				scope = int(*dn.LocalSheetId)
			}
			keys[definedNameKey(dn.Name, scope)] = true
		}
	}
	w.definedNames = &definedNameSet{names: n, keys: keys}
	return w.definedNames
}

// appendDefinedName appends a defined name and keeps the cached set in step, so
// a run of adds does not rebuild it once per add. The set is fetched before the
// append, while it still describes the slice.
func (w *Workbook) appendDefinedName(dn oxml.CT_DefinedName, scope int) {
	set := w.definedNameSetFor()
	w.workbook.DefinedNames.DefinedName = append(w.workbook.DefinedNames.DefinedName, dn)
	set.keys[definedNameKey(dn.Name, scope)] = true
	set.names = len(w.workbook.DefinedNames.DefinedName)
}
