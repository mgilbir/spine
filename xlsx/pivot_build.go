package xlsx

import (
	"fmt"
	"sort"
	"strings"

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
	empty bool
	num   float64
	isNum bool
	text  string
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

	// Classify each column and build its cache field.
	cacheFields := make([]oxml.CT_CacheField, nCols)
	sharedIndexByRow := make([][]int, nCols) // per column, per data row: shared item index (string fields)
	for j := 0; j < nCols; j++ {
		cf := oxml.CT_CacheField{Name: headers[j]}
		if isDim[j] || !columnIsNumeric(columns[j]) {
			cf.Kind = oxml.CacheFieldString
			cf.SharedItems, sharedIndexByRow[j], cf.ContainsBlank = buildSharedItems(columns[j])
		} else {
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

	// Build cache records.
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
			default:
				if d.empty {
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
	def.PivotFields = make([]oxml.CT_PivotField, nCols)
	valueFields := make(map[int]bool)
	for _, v := range values {
		valueFields[v.field] = true
	}
	for j := 0; j < nCols; j++ {
		pf := oxml.CT_PivotField{}
		switch {
		case contains(rowIdx, j):
			pf.Axis = oxml.PivotAxisRow
			pf.ItemCount = len(cacheFields[j].SharedItems)
		case contains(colIdx, j):
			pf.Axis = oxml.PivotAxisCol
			pf.ItemCount = len(cacheFields[j].SharedItems)
		case contains(pageIdx, j):
			pf.Axis = oxml.PivotAxisPage
			pf.ItemCount = len(cacheFields[j].SharedItems)
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

	// Row and column item bodies.
	def.RowItems = buildRowItems(records, rowIdx)
	def.ColItems = buildColItems(records, colIdx, len(values))

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
	case CellTypeNumber, CellTypeDate:
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

// --- axis item builders ---

type axisEntry struct {
	members []int
	i       int // data field index
}

// buildRowItems builds the rowItems body: one entry per distinct row-field
// tuple (sorted) plus a grand-total row. Returns nil when there are no row
// fields.
func buildRowItems(records []oxml.PivotRecord, rowIdx []int) []oxml.PivotAxisItem {
	if len(rowIdx) == 0 {
		return nil
	}
	tuples := distinctTuples(records, rowIdx)
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
func buildColItems(records []oxml.PivotRecord, colIdx []int, nValues int) []oxml.PivotAxisItem {
	valuesOnCols := nValues > 1
	if len(colIdx) == 0 && !valuesOnCols {
		// Single value field, no column fields: a single data column.
		return []oxml.PivotAxisItem{{X: []int{0}}}
	}

	var base [][]int
	if len(colIdx) == 0 {
		base = [][]int{{}}
	} else {
		base = distinctTuples(records, colIdx)
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

// distinctTuples returns the distinct tuples of shared-item indices for the
// given fields across all records, sorted ascending. Records with a missing
// (-1) member in any of the fields are skipped.
func distinctTuples(records []oxml.PivotRecord, fields []int) [][]int {
	seen := make(map[string]struct{})
	var tuples [][]int
	for i := range records {
		t := make([]int, len(fields))
		skip := false
		for k, f := range fields {
			v := records[i].Values[f]
			if v.IsMissing {
				skip = true
				break
			}
			t[k] = v.SharedIndex
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
