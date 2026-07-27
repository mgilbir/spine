package xlsx

import (
	"fmt"
	"strings"

	"github.com/mgilbir/spine/opc"
	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// PivotAggregation is the aggregation function a value (data) field applies to
// its source values.
type PivotAggregation string

// Supported value-field aggregations. The zero value ("") is treated as
// PivotSum.
const (
	PivotSum      PivotAggregation = "sum"
	PivotCount    PivotAggregation = "count"     // counts non-empty values (countA)
	PivotCountNum PivotAggregation = "countNums" // counts numeric values
	PivotAverage  PivotAggregation = "average"
	PivotMax      PivotAggregation = "max"
	PivotMin      PivotAggregation = "min"
	PivotProduct  PivotAggregation = "product"
)

// numericAggregations are the aggregations that require a numeric source field.
var numericAggregations = map[PivotAggregation]bool{
	PivotSum:     true,
	PivotAverage: true,
	PivotMax:     true,
	PivotMin:     true,
	PivotProduct: true,
}

// PivotValueField specifies a source field placed on the value (data) axis and
// how it is aggregated.
type PivotValueField struct {
	// Field is the source column header name.
	Field string
	// Aggregation is the aggregation function; the zero value is PivotSum.
	Aggregation PivotAggregation
	// Name is the value field's display name (e.g. "Sum of Sales"). When empty a
	// name is derived from the aggregation and field (e.g. "Sum of Sales").
	Name string
}

// PivotOptions configures a pivot table created via Sheet.AddPivotTable.
type PivotOptions struct {
	// Name is the pivot table's name; it must be unique in the workbook. When
	// empty a unique name ("PivotTable1", "PivotTable2", ...) is generated.
	Name string
	// RowFields are source column names placed on the row axis, in order.
	RowFields []string
	// ColumnFields are source column names placed on the column axis, in order.
	ColumnFields []string
	// ValueFields are the aggregated value fields. At least one value field or
	// calculated field is required.
	ValueFields []PivotValueField
	// Filters are source column names placed on the page (report filter) axis.
	Filters []string
	// CalculatedFields are formula-derived value fields (e.g. Profit =
	// "Sales-Cost"). Each is added to the cache as a calculated field and placed
	// on the value axis (summed). Formulas reference source column names.
	CalculatedFields []PivotCalculatedField
	// NumericGroups group a numeric source field into value ranges (e.g. bucket
	// Age into 10-year bands). Each grouped field is placed on the row axis (or
	// the column axis when OnColumn is set) in place of the raw field.
	NumericGroups []PivotNumericGroup
	// DateGroups group a date/time source field into calendar buckets (year,
	// quarter, month or day). Each grouped field is placed on the row axis (or
	// the column axis when OnColumn is set).
	DateGroups []PivotDateGroup
	// ItemGroups fold selected items of a source field into named parent groups
	// (e.g. group states into "West"/"East"). Each grouped field is placed on the
	// row axis (or the column axis when OnColumn is set).
	ItemGroups []PivotItemGroup
}

// PivotDateGroupBy is the calendar unit a date field is grouped by.
type PivotDateGroupBy string

// Supported date grouping units.
const (
	PivotByYear    PivotDateGroupBy = "years"
	PivotByQuarter PivotDateGroupBy = "quarters"
	PivotByMonth   PivotDateGroupBy = "months"
	PivotByDay     PivotDateGroupBy = "days"
)

// PivotDateGroup groups a date/time source field into calendar buckets placed
// on an axis. Values are bucketed by the whole calendar unit (e.g. By
// PivotByMonth buckets every January together regardless of year).
type PivotDateGroup struct {
	// Field is the date/time source column to group.
	Field string
	// By is the calendar unit: PivotByYear, PivotByQuarter, PivotByMonth or
	// PivotByDay. The zero value groups by month.
	By PivotDateGroupBy
	// OnColumn places the grouped field on the column axis instead of the row axis.
	OnColumn bool
}

// PivotItemGroup folds selected items of a source field into named parent
// groups placed on an axis. Items not named in any group remain as themselves.
type PivotItemGroup struct {
	// Field is the source column whose items are grouped.
	Field string
	// Groups are the named parent groups; each names the source item values it
	// collects. A value may appear in at most one group.
	Groups []PivotNamedGroup
	// OnColumn places the grouped field on the column axis instead of the row axis.
	OnColumn bool
}

// PivotNamedGroup is one named parent group of an item grouping: a display name
// and the source item values folded into it.
type PivotNamedGroup struct {
	// Name is the group's display label (e.g. "West"). It must be unique within
	// the item grouping and must not collide with an ungrouped source item.
	Name string
	// Items are the source item values collected into the group.
	Items []string
}

// PivotCalculatedField is a formula-derived value field. The formula references
// source column names, e.g. Formula: "Sales-Cost".
type PivotCalculatedField struct {
	// Name is the calculated field's name (and, prefixed with the aggregation,
	// its value-field display name). It must be unique among the source columns.
	Name string
	// Formula is the calculation, e.g. "Sales-Cost" or "Price*Quantity".
	Formula string
}

// PivotNumericGroup groups a numeric source field into equal-width value ranges
// placed on an axis. Values below Start collect into a leading "<Start" bucket,
// values at or above End into a trailing ">End" bucket, and the remainder into
// [Start, Start+Interval), [Start+Interval, Start+2*Interval), ... buckets.
type PivotNumericGroup struct {
	// Field is the numeric source column to group.
	Field string
	// Start, End and Interval define the buckets. Interval must be positive and
	// End must exceed Start.
	Start    float64
	End      float64
	Interval float64
	// OnColumn places the grouped field on the column axis instead of the row axis.
	OnColumn bool
}

// PivotValue describes a value field of an existing pivot table.
type PivotValue struct {
	// Name is the value field's display name (e.g. "Sum of Sales").
	Name string
	// Field is the source field name this value aggregates.
	Field string
	// Aggregation is the aggregation function.
	Aggregation PivotAggregation
}

// PivotTable is a pivot table: a cross-tabulation of a source range summarized
// on the row, column, value and page (filter) axes. A PivotTable returned by
// Sheet.PivotTables or Workbook.PivotTables reflects the table as stored in the
// workbook; its accessors are read-only.
type PivotTable struct {
	def   *oxml.CT_PivotTableDefinition
	cache *oxml.CT_PivotCacheDefinition

	// The following fields are set only for a pivot table created this session
	// via AddPivotTable; they drive the save.
	records []oxml.PivotRecord
	cacheID uint32
}

// Name returns the pivot table's name.
func (p *PivotTable) Name() string { return p.def.Name }

// Location returns the cell range the pivot table occupies on its sheet
// (e.g. "A3:C12").
func (p *PivotTable) Location() string { return p.def.LocationRef }

// CacheID returns the id of the pivot cache the table draws from.
func (p *PivotTable) CacheID() uint32 { return p.def.CacheId }

// SourceRange returns the source data range the pivot cache was built from
// (e.g. "A1:D100"), or "" when the cache could not be resolved.
func (p *PivotTable) SourceRange() string {
	if p.cache == nil {
		return ""
	}
	return p.cache.SourceRef
}

// SourceSheet returns the name of the sheet holding the source range, or ""
// when the cache could not be resolved.
func (p *PivotTable) SourceSheet() string {
	if p.cache == nil {
		return ""
	}
	return p.cache.SourceSheet
}

// cacheFieldName resolves a cache field index to its name, or "" when out of
// range or the cache is unresolved.
func (p *PivotTable) cacheFieldName(idx int) string {
	if p.cache == nil || idx < 0 || idx >= len(p.cache.CacheFields) {
		return ""
	}
	return p.cache.CacheFields[idx].Name
}

// RowFields returns the source field names on the row axis, in order.
func (p *PivotTable) RowFields() []string { return p.axisFieldNames(p.def.RowFields) }

// ColumnFields returns the source field names on the column axis, in order.
func (p *PivotTable) ColumnFields() []string { return p.axisFieldNames(p.def.ColFields) }

func (p *PivotTable) axisFieldNames(indices []int) []string {
	var out []string
	for _, idx := range indices {
		if idx < 0 {
			// -2 marks the position of the data-values dimension, which has no
			// source field.
			continue
		}
		if name := p.cacheFieldName(idx); name != "" {
			out = append(out, name)
		}
	}
	return out
}

// Filters returns the source field names on the page (report filter) axis.
func (p *PivotTable) Filters() []string {
	var out []string
	for _, pf := range p.def.PageFields {
		if name := p.cacheFieldName(pf.Fld); name != "" {
			out = append(out, name)
		}
	}
	return out
}

// ValueFields returns the pivot table's value (data) fields, each with its
// display name, source field and aggregation.
func (p *PivotTable) ValueFields() []PivotValue {
	var out []PivotValue
	for _, df := range p.def.DataFields {
		agg := PivotAggregation(df.Subtotal)
		if agg == "" {
			agg = PivotSum
		}
		out = append(out, PivotValue{
			Name:        df.Name,
			Field:       p.cacheFieldName(df.Fld),
			Aggregation: agg,
		})
	}
	return out
}

// PivotTables returns every pivot table anchored on the sheet: those parsed
// from the opened file and any added this session via AddPivotTable, in that
// order. The slice is nil when the sheet has no pivot tables.
func (s *Sheet) PivotTables() []*PivotTable {
	var out []*PivotTable
	out = append(out, s.openedPivotTables()...)
	out = append(out, s.newPivots...)
	if len(out) == 0 {
		return nil
	}
	return out
}

// PivotTables returns every pivot table across all of the workbook's sheets, in
// sheet order. The slice is nil when the workbook has no pivot tables.
func (w *Workbook) PivotTables() []*PivotTable {
	var out []*PivotTable
	for _, sheet := range w.sheets {
		if sheet == nil {
			continue
		}
		out = append(out, sheet.PivotTables()...)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// openedPivotTables resolves the sheet's pivotTable relationships to their
// parts and parses each, resolving each table's cache through the pivot table
// part's own relationships. Parsing is read-only; the original part bytes still
// round-trip verbatim among the preserved parts.
func (s *Sheet) openedPivotTables() []*PivotTable {
	if s.workbook == nil || s.partName == "" {
		return nil
	}
	var out []*PivotTable
	for _, rel := range s.workbook.relationships[s.partName] {
		if rel == nil || rel.Type != opc.RelTypePivotTable {
			continue
		}
		partName := opc.ResolvePartName(s.partName, rel.Target)
		part, ok := s.workbook.preservedParts[partName]
		if !ok {
			continue
		}
		def, err := oxml.ParsePivotTableDefinition(part.Data)
		if err != nil {
			continue
		}
		pt := &PivotTable{def: def}
		pt.cache = s.workbook.resolvePivotCache(partName)
		out = append(out, pt)
	}
	return out
}

// resolvePivotCache resolves the pivot cache definition a pivot table part
// references through its own relationships.
func (w *Workbook) resolvePivotCache(pivotTablePart string) *oxml.CT_PivotCacheDefinition {
	for _, rel := range w.relationships[pivotTablePart] {
		if rel == nil || rel.Type != opc.RelTypePivotCacheDef {
			continue
		}
		cachePart := opc.ResolvePartName(pivotTablePart, rel.Target)
		part, ok := w.preservedParts[cachePart]
		if !ok {
			continue
		}
		cache, err := oxml.ParsePivotCacheDefinition(part.Data)
		if err != nil {
			continue
		}
		return cache
	}
	return nil
}

// AddPivotTable creates a pivot table summarizing sourceRange and anchors it at
// anchor (the top-left cell of the pivot's output) on this sheet. The pivot
// table part, its cache (definition + records), the workbook <pivotCaches>
// entry, all relationships and the [Content_Types].xml overrides are written on
// the next save.
//
// sourceRange may be sheet-qualified ("Data!A1:D100") or a bare range
// ("A1:D100"); a bare range is resolved on this sheet. Its first row is the
// header row naming the source fields. opts places those fields on the row,
// column, value and filter axes.
//
// The pivot cache is written with refreshOnLoad set, so Excel rebuilds the
// cached values and the rendered layout when the workbook is opened.
//
// opts.CalculatedFields adds calculated (formula) fields as value fields;
// opts.NumericGroups groups a numeric source field into value ranges;
// opts.DateGroups groups a date/time field by year, quarter, month or day; and
// opts.ItemGroups folds selected items of a field into named parent groups.
// Each grouped field is placed on the row (or, with OnColumn, the column) axis.
// A workbook that already contains pivot caches is extended: the new cache is
// allocated a fresh id and parts without disturbing existing pivots.
//
// Limitations: multiple consolidation ranges and external-data caches are out
// of scope. Pivot slicers and timelines can be read (Sheet.Slicers,
// Workbook.Slicers, Sheet.Timelines, Workbook.Timelines) and round-trip
// byte-for-byte, but creating them is not yet supported (see the package
// documentation).
func (s *Sheet) AddPivotTable(sourceRange, anchor string, opts PivotOptions) (*PivotTable, error) {
	if s.opaque {
		return nil, ErrNotWorksheet
	}
	if s.workbook == nil {
		return nil, fmt.Errorf("xlsx: AddPivotTable: sheet is not attached to a workbook")
	}
	if len(opts.ValueFields) == 0 && len(opts.CalculatedFields) == 0 {
		return nil, fmt.Errorf("xlsx: AddPivotTable: at least one value field is required")
	}

	srcSheet, rng, err := s.resolvePivotSource(sourceRange)
	if err != nil {
		return nil, fmt.Errorf("xlsx: AddPivotTable: %w", err)
	}
	if rng.maxRow <= rng.minRow {
		return nil, fmt.Errorf("xlsx: AddPivotTable: source range %q must have a header row and at least one data row", sourceRange)
	}

	anchorRow, anchorCol, err := ParseCellRef(anchor)
	if err != nil {
		return nil, fmt.Errorf("xlsx: AddPivotTable: invalid anchor %q: %w", anchor, err)
	}

	name := opts.Name
	if name == "" {
		name = s.workbook.nextPivotTableName()
	} else {
		// Uniqueness was checked but syntax was not at all, so a pivot named
		// "My Pivot" or "A1" reached pivotTableN.xml and Excel refused the
		// workbook (C535). A pivot table name follows the defined-name rules.
		if err := ValidateDefinedName(name); err != nil {
			return nil, fmt.Errorf("xlsx: AddPivotTable: invalid pivot table name: %w", err)
		}
		if s.workbook.pivotTableNameExists(name) {
			return nil, fmt.Errorf("xlsx: AddPivotTable: pivot table name %q already exists", name)
		}
	}

	build, err := buildPivot(srcSheet, rng, opts)
	if err != nil {
		return nil, fmt.Errorf("xlsx: AddPivotTable: %w", err)
	}

	cacheID := s.workbook.nextPivotCacheID()
	build.def.Name = name
	build.def.CacheId = cacheID
	build.cache.SourceSheet = srcSheet.name
	build.cache.SourceRef = rng.ref()

	// Position the layout at the anchor.
	placePivotLayout(build, anchorRow, anchorCol)

	// A pivot may not be laid out over its own source data on the same sheet:
	// Excel's UI refuses the placement, and with refreshOnLoad set its rebuild
	// writes the pivot over the very cells the cache reads, destroying the
	// source (C543). The layout box is only known after placePivotLayout, so
	// the check belongs here rather than next to the anchor parse.
	if srcSheet == s {
		if box, ok := parsePivotLocation(build.def.LocationRef); ok && box.overlaps(rng) {
			return nil, fmt.Errorf(
				"xlsx: AddPivotTable: layout %s at anchor %q overlaps the source range %s on the same sheet; anchor the pivot clear of its source or place it on another sheet",
				build.def.LocationRef, anchor, rng.ref())
		}
	}

	pt := &PivotTable{
		def:     build.def,
		cache:   build.cache,
		records: build.records,
		cacheID: cacheID,
	}
	s.newPivots = append(s.newPivots, pt)
	s.markDirty()
	return pt, nil
}

// resolvePivotSource parses a possibly sheet-qualified source range and returns
// the sheet it lives on together with the normalized rectangle.
func (s *Sheet) resolvePivotSource(sourceRange string) (*Sheet, cellRange, error) {
	sheetName, ref, qualified := strings.Cut(sourceRange, "!")
	if !qualified {
		ref = sheetName
		sheetName = ""
	}
	srcSheet := s
	if sheetName != "" {
		name := strings.Trim(sheetName, "'")
		srcSheet = nil
		for _, cand := range s.workbook.sheets {
			if strings.EqualFold(cand.name, name) {
				srcSheet = cand
				break
			}
		}
		if srcSheet == nil {
			return nil, cellRange{}, fmt.Errorf("source sheet %q not found", name)
		}
	}
	rng, err := parseTableRange(ref)
	if err != nil {
		return nil, cellRange{}, fmt.Errorf("invalid source range %q", sourceRange)
	}
	return srcSheet, rng, nil
}

// nextPivotTableName returns a pivot table name not already used in the workbook.
func (w *Workbook) nextPivotTableName() string {
	for i := 1; ; i++ {
		name := fmt.Sprintf("PivotTable%d", i)
		if !w.pivotTableNameExists(name) {
			return name
		}
	}
}

// parsePivotLocation parses a pivot layout ref ("B2:D8", or the degenerate
// single-cell "B2" placePivotLayout falls back to) into a rectangle. It reports
// ok=false when the ref does not parse, so an unparseable layout skips the
// overlap check rather than blocking the call.
func parsePivotLocation(ref string) (cellRange, bool) {
	start, end, isRange := strings.Cut(ref, ":")
	if !isRange {
		end = start
	}
	rng, err := normalizeCellRange(start, end)
	if err != nil {
		return cellRange{}, false
	}
	return rng, true
}

// pivotTableNameExists reports whether any pivot table in the workbook already
// uses name (case-insensitively).
func (w *Workbook) pivotTableNameExists(name string) bool {
	for _, p := range w.PivotTables() {
		if strings.EqualFold(p.def.Name, name) {
			return true
		}
	}
	return false
}

// nextPivotCacheID returns a pivot cache id greater than every one already used
// by a pivot cache the workbook was opened with or created this session, so a
// new cache never collides with an existing pivot.
func (w *Workbook) nextPivotCacheID() uint32 {
	var maxID uint32
	if w.workbook != nil {
		for _, id := range w.workbook.ExistingPivotCacheIDs() {
			if id > maxID {
				maxID = id
			}
		}
	}
	for _, sheet := range w.sheets {
		for _, p := range sheet.newPivots {
			if p.cacheID > maxID {
				maxID = p.cacheID
			}
		}
	}
	return maxID + 1
}

// sheetsHavePivots reports whether any sheet carries pivot tables added this
// session.
func (w *Workbook) sheetsHavePivots() bool {
	for _, sheet := range w.sheets {
		if len(sheet.newPivots) > 0 {
			return true
		}
	}
	return false
}
