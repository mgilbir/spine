package xlsx

import (
	"fmt"
	"strings"

	"github.com/mgilbir/spine/opc"
	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// Table is a worksheet table (a.k.a. ListObject): a named, structured range
// with a header row, optional totals row, per-column metadata and a built-in
// table style. A Table returned by Sheet.Tables reflects the table as stored in
// the workbook; the accessors are read-only.
type Table struct {
	model *oxml.CT_Table
}

// TableColumn describes one column of a table.
type TableColumn struct {
	// ID is the column's stable identifier within the table.
	ID uint32
	// Name is the column header text.
	Name string
	// TotalsRowFunction is the built-in totals-row aggregation for the column
	// (e.g. "sum", "count", "average", "min", "max", "countNums", "stdDev",
	// "var", "custom"), or "" when the column has no totals function.
	TotalsRowFunction string
	// TotalsRowLabel is the literal label shown in the column's totals cell
	// (used instead of a function, e.g. "Total").
	TotalsRowLabel string
	// CalculatedColumnFormula is the calculated-column formula filled down the
	// column, or "" when the column holds plain values.
	CalculatedColumnFormula string
}

// TableStyle selects a built-in table style and its banding options.
type TableStyle struct {
	// Name is a built-in table style name such as "TableStyleMedium2". When
	// empty (and every banding flag is false) no style is applied.
	Name string
	// ShowRowStripes bands alternate rows.
	ShowRowStripes bool
	// ShowColumnStripes bands alternate columns.
	ShowColumnStripes bool
	// ShowFirstColumn emphasizes the first column.
	ShowFirstColumn bool
	// ShowLastColumn emphasizes the last column.
	ShowLastColumn bool
}

// TotalsColumn configures a single column's totals-row cell. Consulted only
// when TableOptions.TotalsRow is true.
type TotalsColumn struct {
	// Function is a built-in totals aggregation (e.g. "sum", "count",
	// "average", "min", "max"), or "" for no function.
	Function string
	// Label is a literal label for the totals cell (e.g. "Total"), typically
	// set on the first column instead of a function.
	Label string
}

// TableOptions configures a table created via Sheet.AddTable.
type TableOptions struct {
	// Name is the table's name and displayName. It must be unique within the
	// workbook (case-insensitively) and a valid Excel table name (a defined
	// name: it cannot look like a cell reference and cannot contain spaces).
	// When empty, a unique name ("Table1", "Table2", ...) is generated.
	Name string
	// Columns overrides the column header names. When nil, names are taken from
	// the header row (the first row of the range); blank or duplicate headers
	// are replaced with "ColumnN". When non-nil, its length must equal the
	// number of columns spanned by the range. The header cells are written with
	// the resolved names so the sheet and the table agree.
	Columns []string
	// Style selects the built-in table style and banding. The zero value emits
	// no tableStyleInfo; use e.g. TableStyle{Name: "TableStyleMedium2",
	// ShowRowStripes: true} for Excel's default look.
	Style TableStyle
	// TotalsRow adds a totals row. When true, the last row of the range is the
	// totals row (so the range must include it) and ColumnTotals configures the
	// per-column cells.
	TotalsRow bool
	// ColumnTotals maps a column name to its totals-row function and label.
	// Consulted only when TotalsRow is true.
	ColumnTotals map[string]TotalsColumn
}

// Name returns the table's name.
func (t *Table) Name() string { return t.model.Name }

// DisplayName returns the table's display name.
func (t *Table) DisplayName() string { return t.model.DisplayName }

// Range returns the table's cell range (e.g. "A1:D10"), covering the header
// row, the data rows and the totals row (when present).
func (t *Table) Range() string { return t.model.Ref }

// HeaderRow reports whether the table shows a header row.
func (t *Table) HeaderRow() bool { return t.model.HeaderRowShown() }

// TotalsRow reports whether the table shows a totals row.
func (t *Table) TotalsRow() bool { return t.model.TotalsRowVisible() }

// Columns returns the table's columns in order.
func (t *Table) Columns() []TableColumn {
	out := make([]TableColumn, 0, len(t.model.Columns))
	for i := range t.model.Columns {
		c := &t.model.Columns[i]
		out = append(out, TableColumn{
			ID:                      c.ID,
			Name:                    c.Name,
			TotalsRowFunction:       c.TotalsRowFunction,
			TotalsRowLabel:          c.TotalsRowLabel,
			CalculatedColumnFormula: c.CalculatedColumnFormula,
		})
	}
	return out
}

// Style returns the table's style and banding. The second result is false when
// the table has no tableStyleInfo.
func (t *Table) Style() (TableStyle, bool) {
	if t.model.StyleInfo == nil {
		return TableStyle{}, false
	}
	si := t.model.StyleInfo
	return TableStyle{
		Name:              si.Name,
		ShowRowStripes:    si.ShowRowStripes,
		ShowColumnStripes: si.ShowColumnStripes,
		ShowFirstColumn:   si.ShowFirstColumn,
		ShowLastColumn:    si.ShowLastColumn,
	}, true
}

// Tables returns every table on the sheet: those parsed from the opened file
// and any added this session via AddTable, in that order. The slice is nil when
// the sheet has no tables.
func (s *Sheet) Tables() []*Table {
	var out []*Table
	out = append(out, s.openedTables()...)
	out = append(out, s.newTables...)
	if len(out) == 0 {
		return nil
	}
	return out
}

// Tables returns every table across all of the workbook's sheets, in sheet
// order. The slice is nil when the workbook has no tables.
func (w *Workbook) Tables() []*Table {
	var out []*Table
	for _, sheet := range w.sheets {
		if sheet == nil {
			continue
		}
		out = append(out, sheet.Tables()...)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// openedTables resolves the worksheet's tableParts references to their table
// parts and parses each. Parsing is read-only; the original part bytes still
// round-trip verbatim among the preserved parts.
func (s *Sheet) openedTables() []*Table {
	if s.workbook == nil || s.ws() == nil || s.ws().TableParts == nil {
		return nil
	}
	var out []*Table
	for i := range s.ws().TableParts.TablePart {
		rid := s.ws().TableParts.TablePart[i].RID
		if rid == "" {
			continue
		}
		partName := s.resolveRelTarget(s.partName, rid)
		if partName == "" {
			continue
		}
		part, ok := s.workbook.preservedParts[partName]
		if !ok {
			continue
		}
		model, err := oxml.ParseTable(part.Data)
		if err != nil {
			continue
		}
		out = append(out, &Table{model: model})
	}
	return out
}

// AddTable creates a table over cellRange (e.g. "A1:D10") and returns it. The
// range must be a rectangular reference of at least one column; its first row
// is the header row. Column names are taken from the header cells unless
// opts.Columns overrides them, and the resolved names are written back into the
// header cells so the sheet and the table agree.
//
// AddTable works on both created (Create) and opened (Open/OpenReader)
// workbooks; the table part, its worksheet relationship, the worksheet
// <tableParts> entry and the [Content_Types].xml override are added on the next
// save. Existing tables in an opened workbook are left untouched.
func (s *Sheet) AddTable(cellRange string, opts TableOptions) (*Table, error) {
	if s.opaque {
		return nil, ErrNotWorksheet
	}
	if s.workbook == nil {
		return nil, fmt.Errorf("xlsx: AddTable: sheet is not attached to a workbook")
	}

	rng, err := parseTableRange(cellRange)
	if err != nil {
		return nil, fmt.Errorf("xlsx: AddTable: %w", err)
	}
	ref := rng.ref()

	// A table implies a header row over the top of its range. Deriving the
	// column count from the range keeps the header, columns and ref consistent.
	nCols := rng.maxCol - rng.minCol + 1

	// A totals row (when requested) occupies the last row of the range and is
	// excluded from both the data body and the autoFilter range.
	totalsRows := 0
	if opts.TotalsRow {
		totalsRows = 1
		// The range must hold a header row, at least one data row and the
		// totals row. "< 1" allowed a 2-row range, producing a table with a
		// header and a totals row and no data at all, which Excel never
		// creates (C541).
		if rng.maxRow-rng.minRow < 2 {
			return nil, fmt.Errorf("xlsx: AddTable: a totals row requires a range with a header row, at least one data row and the totals row (at least 3 rows)")
		}
	}

	// One cursor over the header row serves both the name resolution and the
	// write-back below. Resolving the row's cells once keeps AddTable linear in
	// the column count; going through Sheet.Cell/Sheet.CellValue per column
	// re-scanned the (growing) row every time, so a full-width table cost
	// O(cols^2) reference comparisons.
	header := s.newRowCells(rng.minRow)

	names, err := resolveTableColumnNames(header, rng.minCol, nCols, opts.Columns)
	if err != nil {
		return nil, fmt.Errorf("xlsx: AddTable: %w", err)
	}

	name := opts.Name
	if name == "" {
		name = s.workbook.nextTableName()
	} else if err := validateTableName(name); err != nil {
		return nil, fmt.Errorf("xlsx: AddTable: %w", err)
	}
	if s.workbook.tableNameExists(name) {
		return nil, fmt.Errorf("xlsx: AddTable: table name %q already exists", name)
	}

	model := &oxml.CT_Table{
		ID:          s.workbook.nextTableID(),
		Name:        name,
		DisplayName: name,
		Ref:         ref,
	}
	// autoFilter spans the header and data rows (never the totals row).
	model.AutoFilterRef = FormatCellRef(rng.minRow, rng.minCol) + ":" +
		FormatCellRef(rng.maxRow-totalsRows, rng.maxCol)

	if totalsRows > 0 {
		one := uint32(1)
		model.TotalsRowCount = &one
	}

	for i, colName := range names {
		col := oxml.CT_TableColumn{ID: uint32(i + 1), Name: colName}
		if opts.TotalsRow {
			if tc, ok := opts.ColumnTotals[colName]; ok {
				col.TotalsRowFunction = tc.Function
				col.TotalsRowLabel = tc.Label
			}
		}
		model.Columns = append(model.Columns, col)
	}

	if si, ok := tableStyleInfoFrom(opts.Style); ok {
		model.StyleInfo = si
	}

	// Write the resolved header names back into the header cells so the sheet
	// data matches the table definition. Only touch a cell whose text differs
	// from the resolved name, so an unchanged header (e.g. a shared string) is
	// not needlessly rewritten as an inline string.
	for i, colName := range names {
		col := rng.minCol + i
		if header.value(col) == colName {
			continue
		}
		cell, err := header.cell(col)
		if err != nil {
			return nil, fmt.Errorf("xlsx: AddTable: %w", err)
		}
		cell.SetString(colName)
	}

	// Write the totals row's cells. tableN.xml records which function each
	// column totals, but Excel renders the totals row from the sheet cells, so
	// without these the row is blank until the user toggles the totals row off
	// and on again (C541).
	if opts.TotalsRow {
		if err := s.writeTableTotalsRow(model, rng, names); err != nil {
			return nil, fmt.Errorf("xlsx: AddTable: %w", err)
		}
	}

	tbl := &Table{model: model}
	s.newTables = append(s.newTables, tbl)
	s.markDirty()
	return tbl, nil
}

// subtotalFunctionCodes maps a table totalsRowFunction to the SUBTOTAL function
// number Excel writes in the totals cell. The 1xx codes are the "ignore
// manually hidden rows" variants Excel uses for tables. "custom" has no code:
// the caller supplies the formula through the column's own definition, and
// "none" totals nothing.
var subtotalFunctionCodes = map[string]int{
	"average":   101,
	"countNums": 102,
	"count":     103,
	"max":       104,
	"min":       105,
	"stdDev":    107,
	"sum":       109,
	"var":       110,
}

// writeTableTotalsRow fills the last row of a table's range: a literal label
// where the column defines one, otherwise a SUBTOTAL over the column's data
// body using a structured reference, which is what Excel itself writes.
func (s *Sheet) writeTableTotalsRow(model *oxml.CT_Table, rng cellRange, names []string) error {
	// As with the header row: index the totals row once, then write each column
	// in O(1). Per-column Sheet.Cell calls made this loop quadratic too.
	totals := s.newRowCells(rng.maxRow)
	for i, colName := range names {
		col := &model.Columns[i]
		cell, err := totals.cell(rng.minCol + i)
		if err != nil {
			return err
		}
		switch {
		case col.TotalsRowLabel != "":
			cell.SetString(col.TotalsRowLabel)
		case col.TotalsRowFunction == "" || col.TotalsRowFunction == "none":
			// No total for this column; leave the cell alone.
		case col.TotalsRowFunction == "custom":
			// A custom total is carried by the column definition, not by a
			// SUBTOTAL this package can synthesize.
		default:
			code, ok := subtotalFunctionCodes[col.TotalsRowFunction]
			if !ok {
				return fmt.Errorf("column %q: unknown totals function %q", colName, col.TotalsRowFunction)
			}
			cell.SetFormula(fmt.Sprintf("SUBTOTAL(%d,%s[%s])", code, model.Name, colName))
		}
	}
	return nil
}

// resolveTableColumnNames returns the table's column names, either from the
// override or derived from the header row, ensuring they are non-empty and
// unique (as Excel requires).
// The header cells are read through the caller's cursor over the header row, so
// resolving n columns costs one row scan rather than n — which is also why this
// no longer needs the sheet.
func resolveTableColumnNames(header *rowCells, minCol, nCols int, override []string) ([]string, error) {
	if override != nil && len(override) != nCols {
		return nil, fmt.Errorf("Columns has %d entries but the range spans %d columns", len(override), nCols)
	}
	names := make([]string, nCols)
	seen := make(map[string]struct{}, nCols)
	// nextSuffix remembers where the suffix search for a given base name left
	// off. Restarting it at 2 for every repeat made a range of identically
	// named headers quadratic in its own right: the k-th "dup" header walked
	// dup2..dupK before finding a free name. Once a suffix is taken it stays
	// taken, so resuming from the last one yields exactly the same names.
	nextSuffix := make(map[string]int)
	for i := 0; i < nCols; i++ {
		var name string
		if override != nil {
			name = strings.TrimSpace(override[i])
		} else {
			name = strings.TrimSpace(header.value(minCol + i))
		}
		if name == "" {
			name = fmt.Sprintf("Column%d", i+1)
		}
		// De-duplicate case-insensitively, as Excel does.
		base := name
		lowerBase := strings.ToLower(base)
		if _, dup := seen[lowerBase]; dup {
			n := nextSuffix[lowerBase]
			if n < 2 {
				n = 2
			}
			for ; ; n++ {
				name = fmt.Sprintf("%s%d", base, n)
				if _, dup := seen[strings.ToLower(name)]; !dup {
					break
				}
			}
			nextSuffix[lowerBase] = n
		}
		seen[strings.ToLower(name)] = struct{}{}
		names[i] = name
	}
	return names, nil
}

// tableStyleInfoFrom converts a public TableStyle into the internal style model,
// or reports ok=false when the style is the zero value (no style info emitted).
func tableStyleInfoFrom(st TableStyle) (*oxml.CT_TableStyleInfo, bool) {
	if st.Name == "" && !st.ShowRowStripes && !st.ShowColumnStripes && !st.ShowFirstColumn && !st.ShowLastColumn {
		return nil, false
	}
	return &oxml.CT_TableStyleInfo{
		Name:              st.Name,
		ShowFirstColumn:   st.ShowFirstColumn,
		ShowLastColumn:    st.ShowLastColumn,
		ShowRowStripes:    st.ShowRowStripes,
		ShowColumnStripes: st.ShowColumnStripes,
	}, true
}

// parseTableRange parses an "A1:D10" range for a table. A single-cell reference
// is rejected: a table needs a header row and at least one column.
func parseTableRange(ref string) (cellRange, error) {
	ref = strings.TrimSpace(ref)
	start, end, ok := strings.Cut(ref, ":")
	if !ok {
		return cellRange{}, fmt.Errorf("range %q must be a rectangular reference like \"A1:D10\"", ref)
	}
	rng, err := normalizeCellRange(start, end)
	if err != nil {
		return cellRange{}, ErrInvalidRange
	}
	return rng, nil
}

// sheetsHaveTables reports whether any sheet carries tables added this session.
func (w *Workbook) sheetsHaveTables() bool {
	for _, sheet := range w.sheets {
		if len(sheet.newTables) > 0 {
			return true
		}
	}
	return false
}

// nextTableName returns a table name not already used in the workbook.
func (w *Workbook) nextTableName() string {
	for i := 1; ; i++ {
		name := fmt.Sprintf("Table%d", i)
		if !w.tableNameExists(name) {
			return name
		}
	}
}

// tableNameExists reports whether name is already taken (case-insensitively) by
// a table — opened or session-added — or by a defined name. Excel keeps tables
// and defined names in one namespace, so a table called "Sales" collides with a
// defined name "Sales" and the workbook fails to open; checking only other
// tables let that pair through (C535).
func (w *Workbook) tableNameExists(name string) bool {
	for _, t := range w.Tables() {
		if strings.EqualFold(t.model.Name, name) {
			return true
		}
	}
	for _, dn := range w.DefinedNames() {
		if strings.EqualFold(dn.Name, name) {
			return true
		}
	}
	return false
}

// nextTableID returns a table id greater than every table id in the workbook.
func (w *Workbook) nextTableID() uint32 {
	var maxID uint32
	for _, t := range w.Tables() {
		if t.model.ID > maxID {
			maxID = t.model.ID
		}
	}
	return maxID + 1
}

// validateTableName rejects names Excel would refuse for a table. A table name
// is a defined name, so the rules are exactly ValidateDefinedName's: no empty
// name, no name over 255 characters, only letters, digits, ".", "_" and "\",
// a first character that is a letter, "_" or "\", and no collision with an A1-
// or R1C1-style cell reference.
//
// This used to check only empty/whitespace/A1-lookalike/first-character, so
// names Excel rejects on open — "Sales Q1)" and other punctuation, a 300-
// character name, "R1C1", a bare "R" — were written into tableN.xml unchallenged
// (C535).
func validateTableName(name string) error {
	if err := ValidateDefinedName(name); err != nil {
		// Restate against the table vocabulary; the defined-name text would
		// misdescribe what the caller passed.
		return fmt.Errorf("invalid table name: %w", err)
	}
	return nil
}

// writeSheetTables writes each of a sheet's session-added table parts
// (xl/tables/tableN.xml) with collision-safe names, appends the worksheet
// <tableParts> entries and the worksheet→table relationships, and returns the
// updated relationship slice. Table relationship ids continue after those
// already in relUsed.
func (w *Workbook) writeSheetTables(writer *opc.Writer, sheet *Sheet, sheetRels []*opc.Relationship, relUsed, used map[string]struct{}, tableSeq *int) ([]*opc.Relationship, error) {
	if sheet.ws().TableParts == nil {
		sheet.ws().TableParts = &oxml.CT_TableParts{}
	}
	ensureTablePartsInChildOrder(sheet.ws())

	// Rebuild the session-added <tableParts> from scratch each save so a repeated
	// save stays a projection: truncate back to the entries that predated this
	// session's AddTable calls, then re-append. Without this the durable model
	// grew every pass, so a second SaveBytes emitted duplicate <tablePart>
	// entries with duplicate r:ids (C257).
	if !sheet.tablePartsBaselineSet {
		sheet.tablePartsBaseline = len(sheet.ws().TableParts.TablePart)
		sheet.tablePartsBaselineSet = true
	}
	sheet.ws().TableParts.TablePart = sheet.ws().TableParts.TablePart[:sheet.tablePartsBaseline]

	for _, tbl := range sheet.newTables {
		tablePart, tableFile := allocTableName(used, tableSeq)
		if err := writer.WritePart(tablePart, opc.ContentTypeTable, oxml.MarshalTable(tbl.model)); err != nil {
			return nil, err
		}
		rid := fmt.Sprintf("rId%d", nextRelationshipID(relUsed))
		relUsed[rid] = struct{}{}
		sheetRels = append(sheetRels, &opc.Relationship{
			ID:     rid,
			Type:   opc.RelTypeTable,
			Target: fmt.Sprintf("../tables/%s", tableFile),
		})
		sheet.ws().TableParts.TablePart = append(sheet.ws().TableParts.TablePart, oxml.CT_TablePart{RID: rid})
	}
	count := uint32(len(sheet.ws().TableParts.TablePart))
	sheet.ws().TableParts.Count = &count
	return sheetRels, nil
}

// allocTableName finds a free /xl/tables/tableN.xml part, marking it used.
func allocTableName(used map[string]struct{}, seq *int) (partName, fileName string) {
	for {
		fileName = fmt.Sprintf("table%d.xml", *seq)
		partName = "/xl/tables/" + fileName
		*seq++
		if _, ok := used[partName]; !ok {
			used[partName] = struct{}{}
			return partName, fileName
		}
	}
}

// ensureTablePartsInChildOrder inserts "tableParts" into a parsed worksheet's
// captured child order at its schema position (after drawing/legacyDrawing,
// before extLst) so the ChildOrder-gated marshal emits it. It is a no-op when
// the order is empty (a from-scratch sheet marshals in schema order) or already
// contains "tableParts".
func ensureTablePartsInChildOrder(ws *oxml.CT_Worksheet) {
	if len(ws.ChildOrder) == 0 {
		return
	}
	ws.EnsureChildOrder("tableParts")
}
