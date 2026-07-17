package xlsx

import (
	"bytes"
	"fmt"

	"github.com/mgilbir/spine/common/validate"
)

// Validation codes specific to SpreadsheetML (see common/validate for the
// shared OPC-level codes).
const (
	codeSharedFormulaOrphan = "shared-formula-orphan" // <f t="shared" si=N/> with no master ref for si
	codeSheetIDDup          = "sheet-id-dup"          // duplicate sheetId across sheets
	codeDefinedNameScope    = "defined-name-scope"    // definedName localSheetId out of range
	codeMergeOverlap        = "merge-overlap"         // overlapping merged ranges
	codeMergeMalformed      = "merge-malformed"       // unparseable merged-range ref
	codeStylesEmpty         = "styles-empty"          // xl/styles.xml is present but 0-byte/whitespace-only
)

// Validate walks the in-memory workbook model and reports structural problems
// without saving or re-parsing. Save and SaveTo run it first and refuse to
// write when any error-severity finding is present; use SaveToUnvalidated to
// bypass the gate.
//
// The checks are sound (no false positives on Excel-accepted packages).
func (w *Workbook) Validate() validate.Report {
	c := validate.New()
	w.validateSheetIDs(c)
	w.validateSheetRels(c)
	w.validateSharedFormulas(c)
	w.validateDefinedNames(c)
	w.validateMergedRanges(c)
	w.validateStyles(c)
	if w.reader != nil {
		w.validatePackage(c)
	}
	return c.Report()
}

// validateStyles reports an xl/styles.xml part that is present in the package
// but empty (0-byte or whitespace-only). Open tolerates such a part as an empty
// stylesheet — matching Excel, and real Common Crawl files ship a 0-byte
// styles.xml — but surfaces it here so the tolerated defect is not swallowed
// silently. The finding is warning severity: the file opens and saves fine
// (the empty part is preserved raw), so it must not block a save. An entirely
// absent-but-referenced styles part is covered instead by the shared
// rel-target-missing check.
func (w *Workbook) validateStyles(c *validate.Collector) {
	part, ok := w.preservedParts["/xl/styles.xml"]
	if !ok || part == nil {
		return
	}
	if len(bytes.TrimSpace(part.Data)) == 0 {
		c.Warnf(codeStylesEmpty, "/xl/styles.xml",
			"styles part is present but empty; treated as an empty stylesheet (styles default to Excel's built-ins)")
	}
}

// validateSheetIDs reports duplicate sheetId values. sheetId must be unique
// across a workbook; Excel keys internal references off it.
func (w *Workbook) validateSheetIDs(c *validate.Collector) {
	if w.workbook == nil {
		return
	}
	seen := make(map[uint32]bool)
	reported := make(map[uint32]bool)
	for _, s := range w.workbook.Sheets.Sheet {
		if seen[s.SheetId] {
			if !reported[s.SheetId] {
				c.Errorf(codeSheetIDDup, w.mainPart(),
					fmt.Sprintf("sheetId %d is used by more than one sheet", s.SheetId))
				reported[s.SheetId] = true
			}
			continue
		}
		seen[s.SheetId] = true
	}
}

// validateSheetRels reports sheet entries whose r:id has no backing
// relationship — the dangling-relationship class. The relationship set checked
// against is what the save writes: the workbook part's existing rels plus one
// synthesized per sheet.
func (w *Workbook) validateSheetRels(c *validate.Collector) {
	// A freshly created workbook synthesizes its sheet relationships at save
	// time (the r:id on the model and the rel are assigned together), so there
	// is nothing to cross-check and the in-memory r:id may not be final yet.
	if w.workbook == nil || !w.opened {
		return
	}
	ids := relIDSet(w.relationships[w.mainPart()])
	for _, s := range w.sheets {
		if s != nil {
			ids[s.relID] = struct{}{}
		}
	}
	for _, s := range w.workbook.Sheets.Sheet {
		if _, ok := ids[s.RID]; s.RID != "" && !ok {
			c.Errorf(validate.CodeDanglingRel, w.mainPart(),
				fmt.Sprintf("sheet %q references relationship %q with no matching relationship", s.Name, s.RID))
		}
	}
}

// validateSharedFormulas reports shared-formula followers (<f t="shared" si=N/>
// with no ref) whose shared index has no master (a cell defining <f t="shared"
// ref="..." si=N>) on the same sheet. An orphaned follower yields no formula in
// Excel.
func (w *Workbook) validateSharedFormulas(c *validate.Collector) {
	for _, sheet := range w.sheets {
		if sheet == nil || sheet.worksheet == nil {
			continue
		}
		masters := make(map[uint32]bool)
		type follower struct {
			ref string
			si  uint32
		}
		var followers []follower
		for i := range sheet.worksheet.SheetData.Row {
			for _, cell := range sheet.worksheet.SheetData.Row[i].C {
				if cell == nil || cell.F == nil || cell.F.T != "shared" || cell.F.Si == nil {
					continue
				}
				if cell.F.Ref != "" {
					masters[*cell.F.Si] = true
				} else {
					followers = append(followers, follower{ref: cell.R, si: *cell.F.Si})
				}
			}
		}
		for _, f := range followers {
			if !masters[f.si] {
				c.Errorf(codeSharedFormulaOrphan, sheet.partName,
					fmt.Sprintf("cell %s is a shared-formula follower with si=%d but no master defines that shared group", f.ref, f.si))
			}
		}
	}
}

// validateDefinedNames reports definedName entries whose localSheetId is out of
// range (it is a 0-based index into the workbook's sheet list).
func (w *Workbook) validateDefinedNames(c *validate.Collector) {
	if w.workbook == nil || w.workbook.DefinedNames == nil {
		return
	}
	n := uint32(len(w.sheets))
	for _, dn := range w.workbook.DefinedNames.DefinedName {
		if dn.LocalSheetId != nil && *dn.LocalSheetId >= n {
			c.Errorf(codeDefinedNameScope, w.mainPart(),
				fmt.Sprintf("definedName %q has localSheetId %d but the workbook has %d sheet(s)", dn.Name, *dn.LocalSheetId, n))
		}
	}
}

// validateMergedRanges reports malformed merged-range references (warning) and
// overlapping merged ranges (error, the C128 class — Excel refuses overlapping
// merges).
func (w *Workbook) validateMergedRanges(c *validate.Collector) {
	for _, sheet := range w.sheets {
		if sheet == nil || sheet.worksheet == nil || sheet.worksheet.MergeCells == nil {
			continue
		}
		merges := sheet.worksheet.MergeCells.MergeCell
		ranges := make([]cellRange, 0, len(merges))
		for _, mc := range merges {
			rng, err := parseCellRangeRef(mc.Ref)
			if err != nil {
				c.Warnf(codeMergeMalformed, sheet.partName,
					fmt.Sprintf("merged range %q is not a valid cell range", mc.Ref))
				continue
			}
			ranges = append(ranges, rng)
		}
		// Overlap detection. Merged-range counts are small on real sheets, but
		// cap the pairwise scan so a pathological sheet cannot blow up the pass;
		// skipping the check on such a sheet only loses coverage, never
		// soundness.
		if len(ranges) > 4096 {
			continue
		}
		for i := 0; i < len(ranges); i++ {
			for j := i + 1; j < len(ranges); j++ {
				if ranges[i].overlaps(ranges[j]) {
					c.Errorf(codeMergeOverlap, sheet.partName,
						fmt.Sprintf("merged range %s overlaps %s", ranges[i].ref(), ranges[j].ref()))
				}
			}
		}
	}
}

// validatePackage runs the shared OPC-level checks against the source package.
func (w *Workbook) validatePackage(c *validate.Collector) {
	parts := w.knownPartNames()
	ct := func(name string) string {
		if w.reader != nil && w.reader.ContentTypes != nil {
			return w.reader.ContentTypes.GetContentType(name)
		}
		return ""
	}
	validate.CheckDuplicateParts(c, parts)
	validate.CheckContentTypes(c, parts, ct)
	validate.CheckRelationshipTargets(c, w.relationships, w.partExists)
}

func (w *Workbook) knownPartNames() []string {
	seen := make(map[string]bool)
	var out []string
	add := func(name string) {
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		out = append(out, name)
	}
	if w.reader != nil {
		for _, f := range w.reader.Files {
			add(f.Name)
		}
	}
	for _, s := range w.sheets {
		if s != nil {
			add(s.partName)
		}
	}
	for name := range w.preservedParts {
		add(name)
	}
	return out
}

// partExists reports whether a part name is present in the source package or a
// model collection. Deliberately over-inclusive so relationship-target checks
// never yield a false positive.
func (w *Workbook) partExists(name string) bool {
	if w.reader != nil && w.reader.GetFile(name) != nil {
		return true
	}
	if _, ok := w.preservedParts[name]; ok {
		return true
	}
	for _, s := range w.sheets {
		if s != nil && s.partName == name {
			return true
		}
	}
	switch name {
	case w.mainPart(), "/docProps/core.xml", "/docProps/app.xml":
		return true
	}
	return false
}
