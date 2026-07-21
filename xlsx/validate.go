package xlsx

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/mgilbir/spine/common/validate"
	"github.com/mgilbir/spine/opc"
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
	codeCommentPersonOrphan = "comment-person-orphan" // threaded comment personId with no matching person
	codeCommentRefInvalid   = "comment-ref-invalid"   // comment anchored to an unparseable cell ref
	codeHyperlinkRelMissing = "hyperlink-rel-missing" // <hyperlink r:id=N> with no matching sheet relationship
	codeDataValidationRange = "data-validation-range" // dataValidation sqref with a malformed range
	codeChartTargetMissing  = "chart-target-missing"  // drawing chart relationship whose target part is absent
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
	w.validateComments(c)
	w.validateHyperlinks(c)
	w.validateDataValidations(c)
	w.validateCharts(c)
	if w.reader != nil {
		w.validatePackage(c)
	}
	return c.Report()
}

// validateComments reports comment defects that Open tolerates but that Excel
// would treat as data loss: a threaded comment whose personId resolves to no
// person in the workbook person list (the author would show as blank), and a
// comment anchored to an unparseable cell reference. Both are warning severity
// — the file still opens and saves — matching the sound-checks policy. The
// checks read only already-parsed model state, so they cost a lazy parse of the
// comment parts at most.
func (w *Workbook) validateComments(c *validate.Collector) {
	for _, sheet := range w.sheets {
		if sheet == nil {
			continue
		}
		sheet.loadComments()
		sc := sheet.comments
		if sc == nil {
			continue
		}
		if sc.legacy != nil {
			for i := range sc.legacy.Comments {
				ref := sc.legacy.Comments[i].Ref
				if _, _, err := ParseCellRef(ref); err != nil {
					c.Warnf(codeCommentRefInvalid, sc.commentsPart,
						fmt.Sprintf("comment anchored to unparseable cell ref %q", ref))
				}
			}
		}
		if sc.threaded == nil {
			continue
		}
		for i := range sc.threaded.Comments {
			tc := &sc.threaded.Comments[i]
			if _, _, err := ParseCellRef(tc.Ref); err != nil {
				c.Warnf(codeCommentRefInvalid, sc.threadedPart,
					fmt.Sprintf("threaded comment anchored to unparseable cell ref %q", tc.Ref))
			}
			if w.persons == nil || w.persons.FindByID(tc.PersonID) == nil {
				c.Warnf(codeCommentPersonOrphan, sc.threadedPart,
					fmt.Sprintf("threaded comment %s references personId %s with no matching person", tc.ID, tc.PersonID))
			}
		}
	}
}

// validateCharts reports a chart relationship on a sheet's drawing whose target
// chart part is absent from the package (Excel would show an empty frame).
// Warning severity — the file still opens. Checked only for opened workbooks:
// a created workbook wires chart parts and their relationships together at save
// time, so there is nothing to cross-check.
func (w *Workbook) validateCharts(c *validate.Collector) {
	if !w.opened {
		return
	}
	for _, sheet := range w.sheets {
		if sheet == nil || sheet.ws() == nil || sheet.ws().Drawing == nil {
			continue
		}
		drawingPart := sheet.resolveRelTarget(sheet.partName, sheet.ws().Drawing.RID)
		if drawingPart == "" {
			continue
		}
		for _, rel := range w.relationships[drawingPart] {
			if rel == nil || rel.Type != opc.RelTypeChart || rel.TargetMode == opc.TargetModeExternal {
				continue
			}
			target := opc.ResolvePartName(drawingPart, rel.Target)
			if !w.partExists(target) {
				c.Warnf(codeChartTargetMissing, drawingPart,
					fmt.Sprintf("chart relationship %q targets %q with no matching part", rel.ID, rel.Target))
			}
		}
	}
}

// validateHyperlinks reports worksheet hyperlinks that carry an r:id with no
// matching relationship in the sheet's .rels (nor in the pending set the save
// would add). Such a link resolves to nothing in Excel. Warning severity — the
// file still opens and saves.
func (w *Workbook) validateHyperlinks(c *validate.Collector) {
	for _, sheet := range w.sheets {
		if sheet == nil || sheet.ws() == nil || sheet.ws().Hyperlinks == nil {
			continue
		}
		known := relIDSet(w.relationships[sheet.partName])
		for _, rel := range sheet.pendingHyperlinkRels {
			if rel != nil {
				known[rel.ID] = struct{}{}
			}
		}
		for _, hl := range sheet.ws().Hyperlinks.Hyperlink {
			if hl.RID == "" {
				continue
			}
			if _, ok := known[hl.RID]; !ok {
				c.Warnf(codeHyperlinkRelMissing, sheet.partName,
					fmt.Sprintf("hyperlink on %s references relationship %q with no matching relationship", hl.Ref, hl.RID))
			}
		}
	}
}

// validateDataValidations reports data-validation rules whose sqref contains a
// malformed range reference (Excel would drop the rule). Warning severity.
func (w *Workbook) validateDataValidations(c *validate.Collector) {
	for _, sheet := range w.sheets {
		if sheet == nil || sheet.ws() == nil || sheet.ws().DataValidations == nil {
			continue
		}
		for _, dv := range sheet.ws().DataValidations.DataValidation {
			if strings.TrimSpace(dv.Sqref) == "" {
				c.Warnf(codeDataValidationRange, sheet.partName,
					"data validation has an empty sqref")
				continue
			}
			for _, ref := range strings.Fields(dv.Sqref) {
				if _, err := parseCellRangeRef(ref); err != nil {
					c.Warnf(codeDataValidationRange, sheet.partName,
						fmt.Sprintf("data validation sqref %q contains malformed range %q", dv.Sqref, ref))
				}
			}
		}
	}
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
	for i, s := range w.workbook.Sheets.Sheet {
		// A sheet added this session but not yet persisted (no part name) has
		// its worksheet relationship synthesized at save time, so its
		// provisional r:id is legitimately absent from the current set — e.g.
		// the hidden data sheet AddChart adds to an opened workbook. Don't flag
		// it. w.sheets and w.workbook.Sheets.Sheet are kept index-parallel.
		if i < len(w.sheets) && w.sheets[i] != nil && w.sheets[i].partName == "" {
			continue
		}
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
		if sheet == nil || sheet.ws() == nil {
			continue
		}
		masters := make(map[uint32]bool)
		type follower struct {
			ref string
			si  uint32
		}
		var followers []follower
		for i := range sheet.ws().SheetData.Row {
			for _, cell := range sheet.ws().SheetData.Row[i].C {
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
		if sheet == nil || sheet.ws() == nil || sheet.ws().MergeCells == nil {
			continue
		}
		merges := sheet.ws().MergeCells.MergeCell
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
