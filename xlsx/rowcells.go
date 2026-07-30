package xlsx

import (
	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// rowCells is a cursor over one row of a worksheet's cell grid. It resolves the
// row and indexes its cells by column number once, so code that walks a range
// column by column pays a single scan instead of one scan per column.
//
// Sheet.Cell and Sheet.findCell each re-scan SheetData.Row for the row and then
// every <c> in it, comparing references with strings.EqualFold. That is fine for
// a one-off lookup but turns any loop over a range into O(cells^2): a
// full-width table header (16384 columns) cost ~134 million string comparisons
// and ~850ms, because the loop creates cells as it goes and so the row grows
// underneath it. Every per-cell range loop in this package goes through a
// cursor instead.
//
// The cursor holds the row's index in SheetData.Row rather than its address, so
// it survives the slice being reallocated when another row is appended. It must
// not be held across a mutation that inserts or removes rows; nothing in this
// package does that (rows are only ever appended).
type rowCells struct {
	sheet *Sheet
	rowNo int
	// idx is the row's position in ws().SheetData.Row, or -1 when the row does
	// not exist yet.
	idx   int
	byCol map[int]*oxml.CT_Cell
	// prepared records that the worksheet model and its sheetData child-order
	// entry have been ensured for this cursor. Sheet.Cell does that on every
	// call; a cursor does it once, on the first write.
	prepared bool
}

// newRowCells returns a read-only cursor over the given 1-based row. Nothing is
// created: an absent row, an empty sheet or a non-worksheet sheet all yield a
// cursor whose lookups miss. Call cell to materialize a cell (and the row).
func (s *Sheet) newRowCells(row int) *rowCells {
	rc := &rowCells{sheet: s, rowNo: row, idx: -1}
	if s.opaque || row < 1 || row > MaxRow || s.ws() == nil {
		return rc
	}
	rc.locate()
	return rc
}

// locate finds the row in SheetData.Row and indexes its cells. It assumes the
// worksheet model exists.
func (rc *rowCells) locate() {
	sd := &rc.sheet.ws().SheetData
	want := uint32(rc.rowNo)
	for i := range sd.Row {
		if rn, ok := rowNumberOf(&sd.Row[i]); ok && rn == want {
			rc.idx = i
			rc.index()
			return
		}
	}
}

// row returns the resolved CT_Row. Only valid when idx >= 0.
func (rc *rowCells) row() *oxml.CT_Row {
	return &rc.sheet.ws().SheetData.Row[rc.idx]
}

// index builds the column -> cell map for the resolved row.
func (rc *rowCells) index() {
	cells := rc.row().C
	rc.byCol = make(map[int]*oxml.CT_Cell, len(cells))
	for _, c := range cells {
		if c == nil {
			continue
		}
		r, col, err := ParseCellRef(c.R)
		// Only cells that actually name this row are reachable through it, which
		// is what Sheet.Cell's full-reference match already implied: a malformed
		// <c> carrying another row's reference stays unaddressable, as before.
		if err != nil || r != rc.rowNo {
			continue
		}
		if _, dup := rc.byCol[col]; dup {
			// A duplicate reference keeps the first cell, matching the linear
			// scan's first-match-wins behaviour.
			continue
		}
		rc.byCol[col] = c
	}
}

// find returns the cell at the 1-based column, or nil when it does not exist.
// It never creates anything.
func (rc *rowCells) find(col int) *Cell {
	if c, ok := rc.byCol[col]; ok {
		return &Cell{sheet: rc.sheet, cell: c}
	}
	return nil
}

// value returns the stored value of the cell at the 1-based column, or "" when
// the cell does not exist. It is the cursor's equivalent of Sheet.CellValue.
func (rc *rowCells) value(col int) string {
	if c := rc.find(col); c != nil {
		return c.String()
	}
	return ""
}

// cell returns the cell at the 1-based column, creating it — and the row, when
// needed — exactly as Sheet.Cell does. New cells are appended at the end of the
// row; marshalling sorts a row's cells into ascending reference order, so the
// append order never reaches the file.
func (rc *rowCells) cell(col int) (*Cell, error) {
	if rc.sheet.opaque {
		return nil, ErrNotWorksheet
	}
	ref, err := CellRef(rc.rowNo, col)
	if err != nil {
		return nil, err
	}
	rc.prepare()
	if c := rc.find(col); c != nil {
		return c, nil
	}
	if err := rc.ensureRow(); err != nil {
		return nil, err
	}
	nc := &oxml.CT_Cell{R: ref}
	row := rc.row()
	row.C = append(row.C, nc)
	rc.byCol[col] = nc
	return &Cell{sheet: rc.sheet, cell: nc}, nil
}

// prepare ensures the worksheet model and its sheetData child-order entry
// exist, and re-resolves the row if the model only came into being now. It runs
// once per cursor, on the first write.
func (rc *rowCells) prepare() {
	if rc.prepared {
		return
	}
	rc.prepared = true
	hadWS := rc.sheet.ws() != nil
	rc.sheet.ensureWS()
	rc.sheet.ws().EnsureChildOrder("sheetData")
	if !hadWS && rc.rowNo >= 1 && rc.rowNo <= MaxRow {
		rc.locate()
	}
}

// ensureRow materializes the cursor's row if it is not present yet, mirroring
// the row half of Sheet.Cell.
func (rc *rowCells) ensureRow() error {
	if rc.idx >= 0 {
		return nil
	}
	if rc.rowNo < 1 || rc.rowNo > MaxRow {
		return ErrInvalidCell
	}
	rc.prepare()
	if rc.idx >= 0 {
		return nil
	}
	sd := &rc.sheet.ws().SheetData
	r := uint32(rc.rowNo)
	sd.Row = append(sd.Row, oxml.CT_Row{R: &r})
	rc.idx = len(sd.Row) - 1
	rc.byCol = make(map[int]*oxml.CT_Cell)
	return nil
}

// rowCursors hands out one rowCells per row number, so a loop that writes cells
// scattered over many rows still pays a single scan per row rather than one per
// cell. It is the multi-row form of rowCells and carries the same constraint:
// do not hold it across a mutation that inserts or removes rows.
type rowCursors struct {
	sheet *Sheet
	rows  map[int]*rowCells
}

func (s *Sheet) newRowCursors() *rowCursors {
	return &rowCursors{sheet: s, rows: make(map[int]*rowCells)}
}

// row returns the cursor for a 1-based row number, creating and indexing it on
// first use.
func (rcs *rowCursors) row(row int) *rowCells {
	if rc, ok := rcs.rows[row]; ok {
		return rc
	}
	rc := rcs.sheet.newRowCells(row)
	rcs.rows[row] = rc
	return rc
}

// cell returns the cell at a 1-based row and column, creating it if absent.
func (rcs *rowCursors) cell(row, col int) (*Cell, error) {
	if row < 1 || row > MaxRow || col < 1 || col > MaxCol {
		return nil, ErrInvalidCell
	}
	return rcs.row(row).cell(col)
}

// cellByRef returns the cell at a stored reference, creating it if absent. The
// reference is canonicalized exactly as Sheet.Cell canonicalizes it.
func (rcs *rowCursors) cellByRef(ref string) (*Cell, error) {
	row, col, err := ParseCellRef(ref)
	if err != nil {
		return nil, err
	}
	return rcs.cell(row, col)
}
