package xlsx

import (
	"fmt"

	"github.com/mgilbir/spine/common/crypto"
	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// SheetProtection is a read-only view of a sheet's <sheetProtection> element.
// Each operation accessor reports whether that operation is LOCKED (disallowed)
// while protection is enabled.
//
// Note on defaults: in OOXML the format/insert/delete/sort/autoFilter/pivotTables
// operations default to locked when protection is on (their attribute is omitted
// when locked and written as "0" only to unlock), whereas objects and scenarios
// default to unlocked, and cell selection defaults to allowed. These accessors
// return the effective state after applying those defaults.
//
// Excel's sheet protection is a UI convenience, not encryption: the content is
// not protected cryptographically and any tool can clear it. HasPassword only
// reports that a (weak, legacy or hashed) password guard is present; the
// password itself is never exposed or recovered.
type SheetProtection struct {
	enabled           bool
	passwordProtected bool

	formatCells       bool
	formatColumns     bool
	formatRows        bool
	insertColumns     bool
	insertRows        bool
	insertHyperlinks  bool
	deleteColumns     bool
	deleteRows        bool
	sort              bool
	autoFilter        bool
	pivotTables       bool
	objects           bool
	scenarios         bool
	selectLockedCells bool
	selectUnlocked    bool
}

// Enabled reports whether sheet protection is turned on (<sheetProtection
// sheet="1">).
func (p *SheetProtection) Enabled() bool { return p.enabled }

// HasPassword reports whether a password guard is present (either the legacy
// 16-bit hash or a modern hashValue/saltValue). The password is never exposed.
func (p *SheetProtection) HasPassword() bool { return p.passwordProtected }

// FormatCells reports whether formatting cells is locked.
func (p *SheetProtection) FormatCells() bool { return p.formatCells }

// FormatColumns reports whether formatting columns is locked.
func (p *SheetProtection) FormatColumns() bool { return p.formatColumns }

// FormatRows reports whether formatting rows is locked.
func (p *SheetProtection) FormatRows() bool { return p.formatRows }

// InsertColumns reports whether inserting columns is locked.
func (p *SheetProtection) InsertColumns() bool { return p.insertColumns }

// InsertRows reports whether inserting rows is locked.
func (p *SheetProtection) InsertRows() bool { return p.insertRows }

// InsertHyperlinks reports whether inserting hyperlinks is locked.
func (p *SheetProtection) InsertHyperlinks() bool { return p.insertHyperlinks }

// DeleteColumns reports whether deleting columns is locked.
func (p *SheetProtection) DeleteColumns() bool { return p.deleteColumns }

// DeleteRows reports whether deleting rows is locked.
func (p *SheetProtection) DeleteRows() bool { return p.deleteRows }

// Sort reports whether sorting is locked.
func (p *SheetProtection) Sort() bool { return p.sort }

// AutoFilter reports whether using AutoFilter is locked.
func (p *SheetProtection) AutoFilter() bool { return p.autoFilter }

// PivotTables reports whether using PivotTables is locked.
func (p *SheetProtection) PivotTables() bool { return p.pivotTables }

// Objects reports whether editing objects is locked.
func (p *SheetProtection) Objects() bool { return p.objects }

// Scenarios reports whether editing scenarios is locked.
func (p *SheetProtection) Scenarios() bool { return p.scenarios }

// SelectLockedCells reports whether selecting locked cells is disallowed.
func (p *SheetProtection) SelectLockedCells() bool { return p.selectLockedCells }

// SelectUnlockedCells reports whether selecting unlocked cells is disallowed.
func (p *SheetProtection) SelectUnlockedCells() bool { return p.selectUnlocked }

// Protection returns the sheet's protection state, or nil when the sheet has no
// <sheetProtection> element. It is the read counterpart of Protect/Unprotect.
func (s *Sheet) Protection() *SheetProtection {
	if s.ws() == nil || s.ws().SheetProtection == nil {
		return nil
	}
	sp := s.ws().SheetProtection
	// Defaults: the format/insert/delete/sort/autoFilter/pivotTables operations
	// default to locked; objects/scenarios and selection default to unlocked.
	lockedDefaultTrue := func(b *bool) bool { return b == nil || *b }
	lockedDefaultFalse := func(b *bool) bool { return b != nil && *b }
	return &SheetProtection{
		enabled:           sp.Sheet != nil && *sp.Sheet,
		passwordProtected: sp.Password != "" || sp.HashValue != "",
		formatCells:       lockedDefaultTrue(sp.FormatCells),
		formatColumns:     lockedDefaultTrue(sp.FormatColumns),
		formatRows:        lockedDefaultTrue(sp.FormatRows),
		insertColumns:     lockedDefaultTrue(sp.InsertColumns),
		insertRows:        lockedDefaultTrue(sp.InsertRows),
		insertHyperlinks:  lockedDefaultTrue(sp.InsertHyperlinks),
		deleteColumns:     lockedDefaultTrue(sp.DeleteColumns),
		deleteRows:        lockedDefaultTrue(sp.DeleteRows),
		sort:              lockedDefaultTrue(sp.Sort),
		autoFilter:        lockedDefaultTrue(sp.AutoFilter),
		pivotTables:       lockedDefaultTrue(sp.PivotTables),
		objects:           lockedDefaultFalse(sp.Objects),
		scenarios:         lockedDefaultFalse(sp.Scenarios),
		selectLockedCells: lockedDefaultFalse(sp.SelectLockedCells),
		selectUnlocked:    lockedDefaultFalse(sp.SelectUnlockedCells),
	}
}

// SheetProtectionOptions configures Sheet.Protect. The zero value reproduces
// Excel's default "Protect Sheet" behavior: protection is on, every editing
// operation is locked, and only cell selection is allowed. Set an Allow* field
// to unlock that operation, or a Disable* field to further restrict selection.
type SheetProtectionOptions struct {
	// Password, when non-empty, is guarded with Excel's legacy 16-bit password
	// hash. This is obfuscation, not security — it is trivially removed and must
	// not be relied on to protect confidential data.
	Password string

	AllowFormatCells      bool
	AllowFormatColumns    bool
	AllowFormatRows       bool
	AllowInsertColumns    bool
	AllowInsertRows       bool
	AllowInsertHyperlinks bool
	AllowDeleteColumns    bool
	AllowDeleteRows       bool
	AllowSort             bool
	AllowAutoFilter       bool
	AllowPivotTables      bool
	AllowEditObjects      bool
	AllowEditScenarios    bool

	// DisableSelectLockedCells and DisableSelectUnlockedCells remove the
	// selection that protection allows by default.
	DisableSelectLockedCells   bool
	DisableSelectUnlockedCells bool
}

// Protect turns on sheet protection with the given options, replacing any
// existing <sheetProtection> element. It works on both created and opened
// workbooks; a save regenerates the worksheet with the new protection.
//
// Excel sheet protection is a UI guard, not encryption. Even with a password it
// is trivially removed; do not use it to protect confidential data.
func (s *Sheet) Protect(opts SheetProtectionOptions) {
	s.markDirty()
	s.ensureWorksheet()

	yes := true
	sp := &oxml.CT_SheetProtection{Sheet: &yes}

	// Operations that default to locked: emit "0" only to unlock them.
	unlock := func(allow bool) *bool {
		if allow {
			f := false
			return &f
		}
		return nil
	}
	sp.FormatCells = unlock(opts.AllowFormatCells)
	sp.FormatColumns = unlock(opts.AllowFormatColumns)
	sp.FormatRows = unlock(opts.AllowFormatRows)
	sp.InsertColumns = unlock(opts.AllowInsertColumns)
	sp.InsertRows = unlock(opts.AllowInsertRows)
	sp.InsertHyperlinks = unlock(opts.AllowInsertHyperlinks)
	sp.DeleteColumns = unlock(opts.AllowDeleteColumns)
	sp.DeleteRows = unlock(opts.AllowDeleteRows)
	sp.Sort = unlock(opts.AllowSort)
	sp.AutoFilter = unlock(opts.AllowAutoFilter)
	sp.PivotTables = unlock(opts.AllowPivotTables)

	// objects/scenarios default to unlocked: emit "1" to lock, matching Excel's
	// default protection output.
	if !opts.AllowEditObjects {
		sp.Objects = &yes
	}
	if !opts.AllowEditScenarios {
		sp.Scenarios = &yes
	}

	// Selection defaults to allowed: emit "1" to disallow.
	if opts.DisableSelectLockedCells {
		sp.SelectLockedCells = &yes
	}
	if opts.DisableSelectUnlockedCells {
		sp.SelectUnlockedCells = &yes
	}

	if opts.Password != "" {
		sp.Password = fmt.Sprintf("%04X", legacyPasswordHash(opts.Password))
	}

	s.ws().SheetProtection = sp
	s.ws().EnsureChildOrder("sheetProtection")
}

// Unprotect removes sheet protection, if any.
func (s *Sheet) Unprotect() {
	if s.ws() == nil || s.ws().SheetProtection == nil {
		return
	}
	s.markDirty()
	s.ws().SheetProtection = nil
}

// WorkbookProtection is a read-only view of a workbook's <workbookProtection>
// element. It reports whether the workbook's structure (adding, deleting,
// hiding or reordering sheets) and window layout are locked.
//
// Like sheet protection, this is a UI guard, not encryption: the workbook is
// not protected cryptographically and any tool can clear it. HasPassword only
// reports that a (weak legacy or hashed) password guard is present; the
// password itself is never exposed or recovered.
type WorkbookProtection struct {
	lockStructure     bool
	lockWindows       bool
	passwordProtected bool
}

// LockStructure reports whether the workbook structure is locked (sheets cannot
// be added, deleted, hidden, shown or reordered).
func (p *WorkbookProtection) LockStructure() bool { return p.lockStructure }

// LockWindows reports whether the workbook window layout is locked.
func (p *WorkbookProtection) LockWindows() bool { return p.lockWindows }

// HasPassword reports whether a password guard is present (either the legacy
// 16-bit hash or a modern hashValue/saltValue). The password is never exposed.
func (p *WorkbookProtection) HasPassword() bool { return p.passwordProtected }

// Protection returns the workbook's structure-protection state, or nil when the
// workbook has no <workbookProtection> element. It is the read counterpart of
// Protect/Unprotect.
func (w *Workbook) Protection() *WorkbookProtection {
	if w.workbook == nil || w.workbook.WorkbookProtection == nil {
		return nil
	}
	wp := w.workbook.WorkbookProtection
	return &WorkbookProtection{
		lockStructure: wp.LockStructure != nil && *wp.LockStructure,
		lockWindows:   wp.LockWindows != nil && *wp.LockWindows,
		passwordProtected: wp.WorkbookPassword != "" || wp.WorkbookHashValue != "" ||
			wp.RevisionsPassword != "" || wp.RevisionsHashValue != "",
	}
}

// WorkbookProtectionOptions configures Workbook.Protect. The zero value locks
// the workbook structure (the common case: preventing sheets from being added,
// deleted, hidden or reordered). Setting LockWindows alone locks only the
// window layout; set both fields to lock both.
type WorkbookProtectionOptions struct {
	// Password, when non-empty, is guarded with Excel's legacy 16-bit password
	// hash. This is obfuscation, not security — it is trivially removed and must
	// not be relied on to protect confidential data.
	Password string

	// LockStructure locks the workbook structure. When neither LockStructure nor
	// LockWindows is set, Protect locks the structure so the element guards
	// something rather than being an inert <workbookProtection/>.
	LockStructure bool

	// LockWindows locks the workbook window layout.
	LockWindows bool
}

// Protect turns on workbook-structure protection with the given options,
// replacing any existing <workbookProtection> element. A save regenerates
// workbook.xml with the new protection.
//
// Excel workbook protection is a UI guard, not encryption. Even with a password
// it is trivially removed; do not use it to protect confidential data.
func (w *Workbook) Protect(opts WorkbookProtectionOptions) {
	yes := true
	wp := &oxml.CT_WorkbookProtection{}
	if opts.LockStructure || !opts.LockWindows {
		wp.LockStructure = &yes
	}
	if opts.LockWindows {
		wp.LockWindows = &yes
	}
	if opts.Password != "" {
		wp.WorkbookPassword = fmt.Sprintf("%04X", legacyPasswordHash(opts.Password))
	}
	w.workbook.WorkbookProtection = wp
	w.workbook.EnsureChildOrder("workbookProtection")
}

// Unprotect removes workbook-structure protection, if any.
func (w *Workbook) Unprotect() {
	if w.workbook == nil {
		return
	}
	w.workbook.WorkbookProtection = nil
}

// legacyPasswordHash computes Excel's legacy 16-bit worksheet-protection
// password hash. It delegates to the shared common/crypto implementation, which
// docx reuses; see crypto.LegacyPasswordHash for the algorithm and its
// (non-cryptographic) security properties.
func legacyPasswordHash(password string) uint16 {
	return crypto.LegacyPasswordHash(password)
}
