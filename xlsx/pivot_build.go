package xlsx

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// pivotBuild holds the parts and layout metrics produced from a source scan.
type pivotBuild struct {
	def     *oxml.CT_PivotTableDefinition
	cache   *oxml.CT_PivotCacheDefinition
	records []oxml.PivotRecord

	nRowFields  int
	nColFields  int
	nPageFields int
	nValues     int
	bodyRows    int
	bodyCols    int
	colHeaders  int
}

// cellDatum is one scanned source cell.
type cellDatum struct {
	empty  bool
	num    float64
	isNum  bool
	isDate bool
	text   string
}

// buildPivot scans the source range and assembles the pivot cache definition,
// cache records and pivot table definition (minus the anchored location, which
// placePivotLayout fills in).
func buildPivot(src *Sheet, rng cellRange, opts PivotOptions) (*pivotBuild, error) {
	nCols := rng.maxCol - rng.minCol + 1

	// Header names.
	headers := make([]string, nCols)
	for j := 0; j < nCols; j++ {
		ref := FormatCellRef(rng.minRow, rng.minCol+j)
		v, _ := src.GetCellValue(ref)
		v = strings.TrimSpace(v)
		if v == "" {
			v = fmt.Sprintf("Column%d", j+1)
		}
		headers[j] = v
	}

	fieldIndex := func(name string) (int, bool) {
		for j, h := range headers {
			if strings.EqualFold(h, name) {
				return j, true
			}
		}
		return 0, false
	}

	// Resolve axis assignments to column indices.
	rowIdx, err := resolveFields("row", opts.RowFields, fieldIndex)
	if err != nil {
		return nil, err
	}
	colIdx, err := resolveFields("column", opts.ColumnFields, fieldIndex)
	if err != nil {
		return nil, err
	}
	pageIdx, err := resolveFields("filter", opts.Filters, fieldIndex)
	if err != nil {
		return nil, err
	}

	// Resolve group specs (numeric range, date and discrete item groupings) to
	// base column indices. A group's base field is enumerated as a discrete cache
	// field (numeric, date or string) so its buckets can be derived.
	type groupSpec struct {
		kind        oxml.GroupKind
		base        int
		start       float64 // numeric
		end         float64
		interval    float64
		groupBy     PivotDateGroupBy  // date
		namedGroups []PivotNamedGroup // discrete
		onColumn    bool
	}
	var groupSpecs []groupSpec
	groupBase := make(map[int]bool)     // any group base
	numGroupBase := make(map[int]bool)  // numeric-range base
	dateGroupBase := make(map[int]bool) // date base
	itemGroupBase := make(map[int]bool) // discrete-item base
	claimBase := func(field string) (int, error) {
		j, ok := fieldIndex(field)
		if !ok {
			return 0, fmt.Errorf("group field %q is not a source column", field)
		}
		if groupBase[j] {
			return 0, fmt.Errorf("group field %q is listed more than once", field)
		}
		groupBase[j] = true
		return j, nil
	}
	for _, g := range opts.NumericGroups {
		j, err := claimBase(g.Field)
		if err != nil {
			return nil, err
		}
		if g.Interval <= 0 {
			return nil, fmt.Errorf("group field %q: interval must be positive", g.Field)
		}
		if g.End <= g.Start {
			return nil, fmt.Errorf("group field %q: end must exceed start", g.Field)
		}
		numGroupBase[j] = true
		groupSpecs = append(groupSpecs, groupSpec{kind: oxml.GroupNumeric, base: j, start: g.Start, end: g.End, interval: g.Interval, onColumn: g.OnColumn})
	}
	for _, g := range opts.DateGroups {
		j, err := claimBase(g.Field)
		if err != nil {
			return nil, err
		}
		by := g.By
		if by == "" {
			by = PivotByMonth
		}
		if !validDateGroupBy(by) {
			return nil, fmt.Errorf("date group field %q: unsupported unit %q", g.Field, by)
		}
		dateGroupBase[j] = true
		groupSpecs = append(groupSpecs, groupSpec{kind: oxml.GroupDate, base: j, groupBy: by, onColumn: g.OnColumn})
	}
	for _, g := range opts.ItemGroups {
		j, err := claimBase(g.Field)
		if err != nil {
			return nil, err
		}
		if len(g.Groups) == 0 {
			return nil, fmt.Errorf("item group field %q: at least one named group is required", g.Field)
		}
		itemGroupBase[j] = true
		groupSpecs = append(groupSpecs, groupSpec{kind: oxml.GroupDiscrete, base: j, namedGroups: g.Groups, onColumn: g.OnColumn})
	}

	isDim := make(map[int]bool)
	for _, j := range rowIdx {
		isDim[j] = true
	}
	for _, j := range colIdx {
		isDim[j] = true
	}
	for _, j := range pageIdx {
		isDim[j] = true
	}

	type valueSpec struct {
		field int
		agg   PivotAggregation
		name  string
	}
	values := make([]valueSpec, 0, len(opts.ValueFields))
	for _, vf := range opts.ValueFields {
		j, ok := fieldIndex(vf.Field)
		if !ok {
			return nil, fmt.Errorf("value field %q is not a source column", vf.Field)
		}
		if isDim[j] {
			return nil, fmt.Errorf("field %q cannot be both an axis field and a value field", vf.Field)
		}
		agg := vf.Aggregation
		if agg == "" {
			agg = PivotSum
		}
		name := vf.Name
		if name == "" {
			name = defaultValueName(agg, headers[j])
		}
		values = append(values, valueSpec{field: j, agg: agg, name: name})
	}

	// Scan the data rows into per-column data.
	nData := rng.maxRow - rng.minRow
	columns := make([][]cellDatum, nCols)
	for j := range columns {
		columns[j] = make([]cellDatum, 0, nData)
	}
	for r := rng.minRow + 1; r <= rng.maxRow; r++ {
		for j := 0; j < nCols; j++ {
			ref := FormatCellRef(r, rng.minCol+j)
			columns[j] = append(columns[j], scanCell(src, ref))
		}
	}

	// Classify each column and build its cache field. A numeric group's base
	// column is enumerated as a discrete numeric field so buckets can be derived.
	cacheFields := make([]oxml.CT_CacheField, nCols)
	sharedIndexByRow := make([][]int, nCols) // per column, per data row: shared/discrete index
	for j := 0; j < nCols; j++ {
		cf := oxml.CT_CacheField{Name: headers[j]}
		switch {
		case numGroupBase[j]:
			if !columnIsNumeric(columns[j]) {
				return nil, fmt.Errorf("group field %q is not numeric", headers[j])
			}
			cf.Kind = oxml.CacheFieldNumberDiscrete
			cf.NumericItems, sharedIndexByRow[j], cf.MinValue, cf.MaxValue, cf.ContainsInteger, cf.ContainsBlank = buildDiscreteNumericItems(columns[j])
		case dateGroupBase[j]:
			if !columnIsDate(columns[j]) {
				return nil, fmt.Errorf("date group field %q is not a date column", headers[j])
			}
			cf.Kind = oxml.CacheFieldDateDiscrete
			cf.DateItems, sharedIndexByRow[j], cf.MinDate, cf.MaxDate, cf.ContainsBlank = buildDiscreteDateItems(columns[j])
		case itemGroupBase[j]:
			cf.Kind = oxml.CacheFieldString
			cf.SharedItems, sharedIndexByRow[j], cf.ContainsBlank = buildSharedItems(columns[j])
		case isDim[j] || !columnIsNumeric(columns[j]):
			cf.Kind = oxml.CacheFieldString
			cf.SharedItems, sharedIndexByRow[j], cf.ContainsBlank = buildSharedItems(columns[j])
		default:
			cf.Kind = oxml.CacheFieldNumber
			cf.MinValue, cf.MaxValue, cf.ContainsInteger, cf.ContainsBlank = numericStats(columns[j])
		}
		cacheFields[j] = cf
	}

	// Validate value-field aggregations against the resolved field kinds.
	for _, v := range values {
		if numericAggregations[v.agg] && cacheFields[v.field].Kind != oxml.CacheFieldNumber {
			return nil, fmt.Errorf("value field %q is not numeric, so aggregation %q is not supported (use %q)",
				headers[v.field], v.agg, PivotCount)
		}
	}

	// Calculated fields become extra cache fields (databaseField="0", carrying a
	// formula) and value fields. They are appended after the database fields.
	nameTaken := func(name string) bool {
		for _, h := range headers {
			if strings.EqualFold(h, name) {
				return true
			}
		}
		return false
	}
	for _, cfDef := range opts.CalculatedFields {
		name := strings.TrimSpace(cfDef.Name)
		if name == "" {
			return nil, fmt.Errorf("calculated field: name is required")
		}
		if strings.TrimSpace(cfDef.Formula) == "" {
			return nil, fmt.Errorf("calculated field %q: formula is required", name)
		}
		if nameTaken(name) {
			return nil, fmt.Errorf("calculated field %q collides with a source column", name)
		}
		cacheFields = append(cacheFields, oxml.CT_CacheField{
			Name:    name,
			Kind:    oxml.CacheFieldCalculated,
			Formula: cfDef.Formula,
		})
		values = append(values, valueSpec{field: len(cacheFields) - 1, agg: PivotSum, name: defaultValueName(PivotSum, name)})
	}

	// Derived group fields: one per spec, placed on an axis. Each group field
	// carries no cache records of its own; its per-record bucket (index into the
	// group's <groupItems>) is derived from the base column and tracked in
	// groupIndexByRow for axis-item generation.
	groupIndexByRow := make(map[int][]int, len(groupSpecs)) // group cache-field index -> per-row bucket
	for _, gs := range groupSpecs {
		gIdx := len(cacheFields)
		info := &oxml.CacheFieldGroupInfo{Kind: gs.kind, Base: gs.base}
		var buckets []int
		switch gs.kind {
		case oxml.GroupDate:
			info.GroupBy = string(gs.groupBy)
			info.StartDate, info.EndDate, info.Items, buckets = buildDateGroup(columns[gs.base], gs.groupBy)
		case oxml.GroupDiscrete:
			var err error
			info.Items, info.DiscreteMap, buckets, err = buildDiscreteGroup(cacheFields[gs.base].SharedItems, sharedIndexByRow[gs.base], gs.namedGroups)
			if err != nil {
				return nil, fmt.Errorf("item group field %q: %w", headers[gs.base], err)
			}
		default: // GroupNumeric
			info.Start, info.End, info.Interval = gs.start, gs.end, gs.interval
			info.Items = numericGroupItems(gs.start, gs.end, gs.interval)
			buckets = make([]int, nData)
			for i, d := range columns[gs.base] {
				if d.empty || !d.isNum {
					buckets[i] = -1
				} else {
					buckets[i] = numericGroupBucket(d.num, gs.start, gs.end, gs.interval)
				}
			}
		}
		cacheFields = append(cacheFields, oxml.CT_CacheField{
			Name:  headers[gs.base] + " (grouped)",
			Kind:  oxml.CacheFieldGroup,
			Group: info,
		})
		groupIndexByRow[gIdx] = buckets
		if gs.onColumn {
			colIdx = append(colIdx, gIdx)
		} else {
			rowIdx = append(rowIdx, gIdx)
		}
	}

	// Build cache records: one value per database field.
	records := make([]oxml.PivotRecord, nData)
	for i := 0; i < nData; i++ {
		rec := oxml.PivotRecord{Values: make([]oxml.PivotRecordValue, nCols)}
		for j := 0; j < nCols; j++ {
			d := columns[j][i]
			switch cacheFields[j].Kind {
			case oxml.CacheFieldNumber:
				if d.empty || !d.isNum {
					rec.Values[j] = oxml.PivotRecordValue{IsMissing: true}
				} else {
					rec.Values[j] = oxml.PivotRecordValue{IsNumber: true, Number: d.num}
				}
			default: // string, discrete-numeric or discrete-date: reference shared item by index
				if d.empty || sharedIndexByRow[j][i] < 0 {
					rec.Values[j] = oxml.PivotRecordValue{IsMissing: true}
				} else {
					rec.Values[j] = oxml.PivotRecordValue{SharedIndex: sharedIndexByRow[j][i]}
				}
			}
		}
		records[i] = rec
	}

	cache := &oxml.CT_PivotCacheDefinition{
		RefreshOnLoad: true,
		CacheFields:   cacheFields,
		RecordCount:   uint32(nData),
	}

	// Build the pivot table definition.
	def := &oxml.CT_PivotTableDefinition{}
	def.PivotFields = make([]oxml.CT_PivotField, len(cacheFields))
	valueFields := make(map[int]bool)
	for _, v := range values {
		valueFields[v.field] = true
	}
	for j := range cacheFields {
		pf := oxml.CT_PivotField{}
		switch {
		case contains(rowIdx, j):
			pf.Axis = oxml.PivotAxisRow
			pf.ItemCount = pivotFieldItemCount(&cacheFields[j])
		case contains(colIdx, j):
			pf.Axis = oxml.PivotAxisCol
			pf.ItemCount = pivotFieldItemCount(&cacheFields[j])
		case contains(pageIdx, j):
			pf.Axis = oxml.PivotAxisPage
			pf.ItemCount = pivotFieldItemCount(&cacheFields[j])
		}
		if valueFields[j] {
			pf.DataField = true
		}
		def.PivotFields[j] = pf
	}

	valuesOnCols := len(values) > 1

	def.RowFields = append(def.RowFields, rowIdx...)
	def.ColFields = append(def.ColFields, colIdx...)
	if valuesOnCols {
		def.ColFields = append(def.ColFields, pivotDataPlaceholder)
	}

	for _, v := range values {
		def.DataFields = append(def.DataFields, oxml.CT_DataField{
			Name:     v.name,
			Fld:      v.field,
			Subtotal: string(v.agg),
		})
	}
	for _, j := range pageIdx {
		def.PageFields = append(def.PageFields, oxml.CT_PageField{Fld: j})
	}

	// Row and column item bodies. Group fields resolve their per-record bucket
	// through groupIndexByRow rather than the cache records.
	memberFor := func(field, row int) (int, bool) {
		if buckets, ok := groupIndexByRow[field]; ok {
			if buckets[row] < 0 {
				return 0, false
			}
			return buckets[row], true
		}
		v := records[row].Values[field]
		if v.IsMissing {
			return 0, false
		}
		return v.SharedIndex, true
	}
	def.RowItems = buildRowItems(nData, rowIdx, memberFor)
	def.ColItems = buildColItems(nData, colIdx, len(values), memberFor)

	b := &pivotBuild{
		def:         def,
		cache:       cache,
		records:     records,
		nRowFields:  len(rowIdx),
		nColFields:  len(colIdx),
		nPageFields: len(pageIdx),
		nValues:     len(values),
	}
	b.bodyRows = len(def.RowItems)
	if b.bodyRows == 0 {
		b.bodyRows = maxInt(1, b.nValues)
	}
	b.bodyCols = len(def.ColItems)
	if b.bodyCols == 0 {
		b.bodyCols = 1
	}
	b.colHeaders = maxInt(1, b.nColFields)
	if valuesOnCols {
		b.colHeaders++
	}
	return b, nil
}

// pivotDataPlaceholder marks the data-values position in rowFields/colFields.
const pivotDataPlaceholder = -2

// placePivotLayout positions the pivot's location box at the anchor cell and
// fills the location offsets.
func placePivotLayout(b *pivotBuild, anchorRow, anchorCol int) {
	filtersHeight := 0
	if b.nPageFields > 0 {
		filtersHeight = b.nPageFields + 1
	}
	firstDataCol := maxInt(1, b.nRowFields)

	totalRows := filtersHeight + b.colHeaders + b.bodyRows
	totalCols := firstDataCol + b.bodyCols

	endRow := anchorRow + totalRows - 1
	endCol := anchorCol + totalCols - 1
	start := FormatCellRef(anchorRow, anchorCol)
	end := FormatCellRef(endRow, endCol)
	if start == "" || end == "" {
		// Degenerate/overflowing box: fall back to a single-cell ref so the part
		// stays valid; Excel recomputes the extent on refresh.
		b.def.LocationRef = FormatCellRef(anchorRow, anchorCol)
		if b.def.LocationRef == "" {
			b.def.LocationRef = "A1"
		}
	} else {
		b.def.LocationRef = start + ":" + end
	}

	b.def.FirstHeaderRow = uint32(filtersHeight + 1)
	b.def.FirstDataRow = uint32(filtersHeight + b.colHeaders + 1)
	b.def.FirstDataCol = uint32(firstDataCol)
	if b.nPageFields > 0 {
		b.def.RowPageCount = uint32(b.nPageFields)
		b.def.ColPageCount = 1
	}
}

// --- source scanning helpers ---

func scanCell(s *Sheet, ref string) cellDatum {
	cell, err := s.Cell(ref)
	if err != nil || cell == nil || cell.IsEmpty() {
		return cellDatum{empty: true}
	}
	switch cell.Type() {
	case CellTypeDate:
		return cellDatum{num: cell.Float(), isNum: true, isDate: true, text: cell.String()}
	case CellTypeNumber:
		return cellDatum{num: cell.Float(), isNum: true, text: cell.String()}
	default:
		text := cell.String()
		if strings.TrimSpace(text) == "" {
			return cellDatum{empty: true}
		}
		return cellDatum{text: text}
	}
}

// columnIsNumeric reports whether a column has at least one numeric value and
// no non-empty string values.
func columnIsNumeric(col []cellDatum) bool {
	seenNum := false
	for _, d := range col {
		if d.empty {
			continue
		}
		if d.isNum {
			seenNum = true
			continue
		}
		return false
	}
	return seenNum
}

// buildSharedItems returns the distinct string values (first-seen order), a
// per-row index into them, and whether any cell was blank.
func buildSharedItems(col []cellDatum) (items []string, indexByRow []int, containsBlank bool) {
	pos := make(map[string]int)
	indexByRow = make([]int, len(col))
	for i, d := range col {
		if d.empty {
			containsBlank = true
			indexByRow[i] = -1
			continue
		}
		text := d.text
		if idx, ok := pos[text]; ok {
			indexByRow[i] = idx
			continue
		}
		idx := len(items)
		pos[text] = idx
		items = append(items, text)
		indexByRow[i] = idx
	}
	return items, indexByRow, containsBlank
}

// numericStats returns the min, max, whether all values are whole numbers, and
// whether any cell was blank.
func numericStats(col []cellDatum) (minV, maxV float64, integer, containsBlank bool) {
	integer = true
	seen := false
	for _, d := range col {
		if d.empty || !d.isNum {
			if d.empty {
				containsBlank = true
			}
			continue
		}
		if !seen {
			minV, maxV = d.num, d.num
			seen = true
		} else {
			if d.num < minV {
				minV = d.num
			}
			if d.num > maxV {
				maxV = d.num
			}
		}
		if d.num != float64(int64(d.num)) {
			integer = false
		}
	}
	return minV, maxV, integer, containsBlank
}

// buildDiscreteNumericItems enumerates a numeric column's distinct values in
// ascending order, returning them, a per-row index into them (-1 for a blank or
// non-numeric cell), the min/max, whether every value is a whole number, and
// whether any cell was blank. Used for a numeric group's base field.
func buildDiscreteNumericItems(col []cellDatum) (items []float64, indexByRow []int, minV, maxV float64, integer, containsBlank bool) {
	indexByRow = make([]int, len(col))
	distinct := make(map[float64]struct{})
	integer = true
	seen := false
	for i, d := range col {
		if d.empty || !d.isNum {
			if d.empty {
				containsBlank = true
			}
			indexByRow[i] = -1
			continue
		}
		if !seen {
			minV, maxV = d.num, d.num
			seen = true
		} else {
			if d.num < minV {
				minV = d.num
			}
			if d.num > maxV {
				maxV = d.num
			}
		}
		if d.num != float64(int64(d.num)) {
			integer = false
		}
		if _, ok := distinct[d.num]; !ok {
			distinct[d.num] = struct{}{}
			items = append(items, d.num)
		}
	}
	sort.Float64s(items)
	pos := make(map[float64]int, len(items))
	for idx, v := range items {
		pos[v] = idx
	}
	for i, d := range col {
		if indexByRow[i] < 0 {
			continue
		}
		indexByRow[i] = pos[d.num]
	}
	return items, indexByRow, minV, maxV, integer, containsBlank
}

// numericGroupItems returns the group bucket labels for a numeric range group:
// a leading "<start" item, one "lo-hi" item per interval up to end, and a
// trailing ">end" item, mirroring Excel's grouping labels.
func numericGroupItems(start, end, interval float64) []string {
	k := int(math.Ceil((end - start) / interval))
	if k < 0 {
		k = 0
	}
	items := make([]string, 0, k+2)
	items = append(items, "<"+trimFloat(start))
	for i := 0; i < k; i++ {
		lo := start + float64(i)*interval
		hi := lo + interval
		items = append(items, trimFloat(lo)+"-"+trimFloat(hi))
	}
	items = append(items, ">"+trimFloat(end))
	return items
}

// numericGroupBucket returns the index (into numericGroupItems) of the bucket a
// value falls into.
func numericGroupBucket(v, start, end, interval float64) int {
	k := int(math.Ceil((end - start) / interval))
	if k < 0 {
		k = 0
	}
	if v < start {
		return 0
	}
	if v >= end {
		return k + 1
	}
	return 1 + int(math.Floor((v-start)/interval))
}

// --- date grouping helpers ---

// monthNames are the abbreviated month labels Excel uses for date group buckets.
var monthNames = [12]string{"Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}

// validDateGroupBy reports whether by is a supported date grouping unit.
func validDateGroupBy(by PivotDateGroupBy) bool {
	switch by {
	case PivotByYear, PivotByQuarter, PivotByMonth, PivotByDay:
		return true
	}
	return false
}

// columnIsDate reports whether a column has at least one date value and no
// non-empty, non-date values.
func columnIsDate(col []cellDatum) bool {
	seen := false
	for _, d := range col {
		if d.empty {
			continue
		}
		if d.isDate {
			seen = true
			continue
		}
		return false
	}
	return seen
}

// isoDate formats a time as the ISO 8601 form Excel writes for cached dates.
func isoDate(t time.Time) string {
	return t.Format("2006-01-02T15:04:05")
}

// labelDate formats a date as the M/D/YYYY label Excel uses for a date group's
// leading "<start" and trailing ">end" bound items.
func labelDate(t time.Time) string {
	return fmt.Sprintf("%d/%d/%d", int(t.Month()), t.Day(), t.Year())
}

// buildDiscreteDateItems enumerates a date column's distinct values in ascending
// order as ISO 8601 strings, returning them, a per-row index into them (-1 for a
// blank or non-date cell), the min/max dates, and whether any cell was blank.
// Used for the base field of a date grouping.
func buildDiscreteDateItems(col []cellDatum) (items []string, indexByRow []int, minDate, maxDate string, containsBlank bool) {
	indexByRow = make([]int, len(col))
	distinct := make(map[float64]struct{})
	var serials []float64
	var minS, maxS float64
	seen := false
	for i, d := range col {
		if d.empty || !d.isDate {
			if d.empty {
				containsBlank = true
			}
			indexByRow[i] = -1
			continue
		}
		if !seen {
			minS, maxS = d.num, d.num
			seen = true
		} else {
			if d.num < minS {
				minS = d.num
			}
			if d.num > maxS {
				maxS = d.num
			}
		}
		if _, ok := distinct[d.num]; !ok {
			distinct[d.num] = struct{}{}
			serials = append(serials, d.num)
		}
	}
	sort.Float64s(serials)
	items = make([]string, len(serials))
	pos := make(map[float64]int, len(serials))
	for idx, s := range serials {
		items[idx] = isoDate(excelDateToTime(s))
		pos[s] = idx
	}
	for i, d := range col {
		if indexByRow[i] < 0 {
			continue
		}
		indexByRow[i] = pos[d.num]
	}
	if seen {
		minDate = isoDate(excelDateToTime(minS))
		maxDate = isoDate(excelDateToTime(maxS))
	}
	return items, indexByRow, minDate, maxDate, containsBlank
}

// dayOfYearItems returns the fixed 366-day bucket labels ("1-Jan" ... "31-Dec")
// and a lookup from (month, day) to their index, for day grouping.
func dayOfYearItems() ([]string, map[[2]int]int) {
	daysIn := [12]int{31, 29, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}
	var items []string
	idx := make(map[[2]int]int, 366)
	for m := 1; m <= 12; m++ {
		for day := 1; day <= daysIn[m-1]; day++ {
			idx[[2]int{m, day}] = len(items)
			items = append(items, strconv.Itoa(day)+"-"+monthNames[m-1])
		}
	}
	return items, idx
}

// buildDateGroup derives a date grouping's rangePr bounds (ISO), its groupItems
// labels, and the per-row bucket index (into the labels; -1 for a blank or
// non-date cell). Buckets fold each date into the whole calendar unit given by
// by (e.g. every January together for PivotByMonth). The leading "<start" and
// trailing ">end" bound items bracket the data range and stay empty.
func buildDateGroup(col []cellDatum, by PivotDateGroupBy) (startISO, endISO string, items []string, buckets []int) {
	buckets = make([]int, len(col))
	var minT, maxT time.Time
	seen := false
	for _, d := range col {
		if d.empty || !d.isDate {
			continue
		}
		t := excelDateToTime(d.num)
		if !seen {
			minT, maxT = t, t
			seen = true
		} else {
			if t.Before(minT) {
				minT = t
			}
			if t.After(maxT) {
				maxT = t
			}
		}
	}
	if !seen {
		minT, maxT = excelEpoch, excelEpoch
	}
	startDay := time.Date(minT.Year(), minT.Month(), minT.Day(), 0, 0, 0, 0, time.UTC)
	endDay := time.Date(maxT.Year(), maxT.Month(), maxT.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, 1)
	startISO = isoDate(startDay)
	endISO = isoDate(endDay)
	startLabel := "<" + labelDate(startDay)
	endLabel := ">" + labelDate(endDay)

	switch by {
	case PivotByYear:
		yearSet := make(map[int]struct{})
		for _, d := range col {
			if d.empty || !d.isDate {
				continue
			}
			yearSet[excelDateToTime(d.num).Year()] = struct{}{}
		}
		years := make([]int, 0, len(yearSet))
		for y := range yearSet {
			years = append(years, y)
		}
		sort.Ints(years)
		yearIdx := make(map[int]int, len(years))
		items = append(items, startLabel)
		for k, y := range years {
			yearIdx[y] = k + 1
			items = append(items, strconv.Itoa(y))
		}
		items = append(items, endLabel)
		for i, d := range col {
			if d.empty || !d.isDate {
				buckets[i] = -1
				continue
			}
			buckets[i] = yearIdx[excelDateToTime(d.num).Year()]
		}
	case PivotByQuarter:
		items = append(items, startLabel, "Qtr1", "Qtr2", "Qtr3", "Qtr4", endLabel)
		for i, d := range col {
			if d.empty || !d.isDate {
				buckets[i] = -1
				continue
			}
			m := int(excelDateToTime(d.num).Month())
			buckets[i] = (m-1)/3 + 1
		}
	case PivotByDay:
		dayItems, dayIdx := dayOfYearItems()
		items = append(items, startLabel)
		items = append(items, dayItems...)
		items = append(items, endLabel)
		for i, d := range col {
			if d.empty || !d.isDate {
				buckets[i] = -1
				continue
			}
			t := excelDateToTime(d.num)
			buckets[i] = dayIdx[[2]int{int(t.Month()), t.Day()}] + 1
		}
	default: // PivotByMonth
		items = append(items, startLabel)
		items = append(items, monthNames[:]...)
		items = append(items, endLabel)
		for i, d := range col {
			if d.empty || !d.isDate {
				buckets[i] = -1
				continue
			}
			buckets[i] = int(excelDateToTime(d.num).Month())
		}
	}
	return startISO, endISO, items, buckets
}

// buildDiscreteGroup folds a base string field's items into named parent groups.
// It returns the group's <groupItems> labels (named groups first, then each
// ungrouped base item), the discretePr map (one group-item index per base item,
// in base-item order), and the per-row bucket index (-1 for a blank base cell).
func buildDiscreteGroup(baseItems []string, indexByRow []int, groups []PivotNamedGroup) (items []string, discreteMap, buckets []int, err error) {
	itemPos := make(map[string]int, len(baseItems))
	for i, v := range baseItems {
		itemPos[v] = i
	}
	assigned := make([]int, len(baseItems))
	for i := range assigned {
		assigned[i] = -1
	}
	nameSet := make(map[string]struct{})
	for _, g := range groups {
		name := strings.TrimSpace(g.Name)
		if name == "" {
			return nil, nil, nil, fmt.Errorf("group name is required")
		}
		if _, dup := nameSet[name]; dup {
			return nil, nil, nil, fmt.Errorf("group name %q is listed more than once", name)
		}
		nameSet[name] = struct{}{}
		gi := len(items)
		items = append(items, name)
		for _, v := range g.Items {
			bi, ok := itemPos[v]
			if !ok {
				return nil, nil, nil, fmt.Errorf("item %q is not a value of the field", v)
			}
			if assigned[bi] >= 0 {
				return nil, nil, nil, fmt.Errorf("item %q is grouped more than once", v)
			}
			assigned[bi] = gi
		}
	}
	for bi, v := range baseItems {
		if assigned[bi] >= 0 {
			continue
		}
		if _, clash := nameSet[v]; clash {
			return nil, nil, nil, fmt.Errorf("ungrouped item %q collides with a group name", v)
		}
		assigned[bi] = len(items)
		items = append(items, v)
	}
	discreteMap = assigned
	buckets = make([]int, len(indexByRow))
	for i, bi := range indexByRow {
		if bi < 0 {
			buckets[i] = -1
			continue
		}
		buckets[i] = assigned[bi]
	}
	return items, discreteMap, buckets, nil
}

// pivotFieldItemCount is the number of items a row/column pivotField exposes:
// the group bucket labels for a group field, otherwise the shared-item count.
func pivotFieldItemCount(cf *oxml.CT_CacheField) int {
	if cf.Kind == oxml.CacheFieldGroup && cf.Group != nil {
		return len(cf.Group.Items)
	}
	if cf.Kind == oxml.CacheFieldNumberDiscrete {
		return len(cf.NumericItems)
	}
	return len(cf.SharedItems)
}

// trimFloat formats a float without a trailing ".0" for whole numbers.
func trimFloat(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// --- axis item builders ---

type axisEntry struct {
	members []int
	i       int // data field index
}

// memberResolver returns the item index a record contributes for an axis field,
// and whether the record contributes at all (false for a missing member).
type memberResolver func(field, row int) (int, bool)

// buildRowItems builds the rowItems body: one entry per distinct row-field
// tuple (sorted) plus a grand-total row. Returns nil when there are no row
// fields.
func buildRowItems(nData int, rowIdx []int, member memberResolver) []oxml.PivotAxisItem {
	if len(rowIdx) == 0 {
		return nil
	}
	tuples := distinctTuples(nData, rowIdx, member)
	entries := make([]axisEntry, len(tuples))
	for i, t := range tuples {
		entries[i] = axisEntry{members: t}
	}
	items := computeAxisItems(entries)
	items = append(items, oxml.PivotAxisItem{Type: "grand", X: []int{0}})
	return items
}

// buildColItems builds the colItems body from the column-field tuples crossed
// with the data-values dimension.
func buildColItems(nData int, colIdx []int, nValues int, member memberResolver) []oxml.PivotAxisItem {
	valuesOnCols := nValues > 1
	if len(colIdx) == 0 && !valuesOnCols {
		// Single value field, no column fields: a single data column.
		return []oxml.PivotAxisItem{{X: []int{0}}}
	}

	var base [][]int
	if len(colIdx) == 0 {
		base = [][]int{{}}
	} else {
		base = distinctTuples(nData, colIdx, member)
	}

	var entries []axisEntry
	for _, ct := range base {
		if valuesOnCols {
			for d := 0; d < nValues; d++ {
				members := append(append([]int(nil), ct...), d)
				entries = append(entries, axisEntry{members: members, i: d})
			}
		} else {
			entries = append(entries, axisEntry{members: ct})
		}
	}
	items := computeAxisItems(entries)
	if len(colIdx) > 0 {
		items = append(items, oxml.PivotAxisItem{Type: "grand", X: []int{0}})
	}
	return items
}

// distinctTuples returns the distinct tuples of item indices for the given
// fields across all rows, sorted ascending. Rows with a missing member in any
// of the fields are skipped.
func distinctTuples(nData int, fields []int, member memberResolver) [][]int {
	seen := make(map[string]struct{})
	var tuples [][]int
	for i := 0; i < nData; i++ {
		t := make([]int, len(fields))
		skip := false
		for k, f := range fields {
			m, ok := member(f, i)
			if !ok {
				skip = true
				break
			}
			t[k] = m
		}
		if skip {
			continue
		}
		key := tupleKey(t)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		tuples = append(tuples, t)
	}
	sort.Slice(tuples, func(a, b int) bool { return tupleLess(tuples[a], tuples[b]) })
	return tuples
}

// computeAxisItems renders axis entries into pivot items, compressing repeated
// leading members via the @r attribute.
func computeAxisItems(entries []axisEntry) []oxml.PivotAxisItem {
	items := make([]oxml.PivotAxisItem, 0, len(entries))
	var prev []int
	for _, e := range entries {
		r := commonPrefixLen(prev, e.members)
		items = append(items, oxml.PivotAxisItem{
			I: e.i,
			R: r,
			X: append([]int(nil), e.members[r:]...),
		})
		prev = e.members
	}
	return items
}

// --- small helpers ---

func resolveFields(axis string, names []string, index func(string) (int, bool)) ([]int, error) {
	out := make([]int, 0, len(names))
	seen := make(map[int]struct{})
	for _, n := range names {
		j, ok := index(n)
		if !ok {
			return nil, fmt.Errorf("%s field %q is not a source column", axis, n)
		}
		if _, dup := seen[j]; dup {
			return nil, fmt.Errorf("%s field %q is listed more than once", axis, n)
		}
		seen[j] = struct{}{}
		out = append(out, j)
	}
	return out, nil
}

func defaultValueName(agg PivotAggregation, field string) string {
	label := map[PivotAggregation]string{
		PivotSum:      "Sum",
		PivotCount:    "Count",
		PivotCountNum: "Count",
		PivotAverage:  "Average",
		PivotMax:      "Max",
		PivotMin:      "Min",
		PivotProduct:  "Product",
	}[agg]
	if label == "" {
		label = "Sum"
	}
	return label + " of " + field
}

func contains(s []int, v int) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func commonPrefixLen(a, b []int) int {
	n := 0
	for n < len(a) && n < len(b) && a[n] == b[n] {
		n++
	}
	return n
}

func tupleKey(t []int) string {
	var sb strings.Builder
	for _, v := range t {
		fmt.Fprintf(&sb, "%d,", v)
	}
	return sb.String()
}

func tupleLess(a, b []int) bool {
	for i := 0; i < len(a) && i < len(b); i++ {
		if a[i] != b[i] {
			return a[i] < b[i]
		}
	}
	return len(a) < len(b)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
