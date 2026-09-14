package xlsx

import (
	"fmt"
	"io"
	"strings"

	xmlb "github.com/mgilbir/spine/common/xml"
	"github.com/mgilbir/spine/opc"
	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// streamFlushRows is how many rows are built before the accumulated XML is
// drained into the package. It trades a little buffering for far fewer writes;
// at ten columns a block of this size is on the order of a hundred kilobytes.
const streamFlushRows = 512

// StreamWriter writes a workbook a row at a time, emitting each row into the
// package as it goes instead of holding the whole workbook in memory.
//
// The DOM path (Create/AddSheet/SetCellValue/SaveTo) keeps every cell resident:
// even after CT_Cell was cut from 112 bytes to 48, a workbook costs roughly
// 25x its file size in live heap, which puts a hard ceiling on how much can be
// written in one pass. A StreamWriter's memory is bounded by one block of rows
// regardless of how many rows are written.
//
// What it gives up, and why, is the whole of its cost:
//
//   - It only creates. There is no opening an existing workbook and streaming
//     into it: a part is one zip entry, and the bytes already in it come first.
//   - Rows are written in the order given and cannot be revisited. Nothing can
//     go back and change a row that has already left.
//   - No <dimension> is emitted. It precedes <sheetData> in schema order and
//     its value is not known until the last row has been written. The element
//     is optional and Excel recalculates it.
//   - Column widths must be set before the first row of their sheet, because
//     <cols> also precedes <sheetData>.
//   - There is no Validate pass. Validation reads a finished model, and there
//     is no model here.
//   - There is no encryption. Encrypting a package needs all of its bytes.
//
// Sheets are written one after another: adding a sheet finishes the previous
// one. That is not a style choice — the package writer is sequential, and a
// part stops being writable as soon as the next entry is created.
//
// Cells are emitted through the same row marshaller the DOM path uses, so a
// streamed workbook and a built one produce the same bytes for the same values.
// There is deliberately no second serializer to drift.
type StreamWriter struct {
	pkg    *opc.Writer
	sheets []streamSheetRef
	cur    *StreamSheet
	closed bool
}

// streamSheetRef is what the workbook part needs to know about a sheet once its
// own part has been written and forgotten.
type streamSheetRef struct {
	name     string
	partName string
	relID    string
	sheetID  uint32
}

// NewStreamWriter returns a writer that emits a workbook to dst.
//
// Nothing is written until the first sheet is added, and the package is only
// complete once Close returns without error. A StreamWriter that is abandoned
// without Close leaves dst holding a truncated archive.
func NewStreamWriter(dst io.Writer) *StreamWriter {
	return &StreamWriter{pkg: opc.NewWriter(dst)}
}

// AddSheet starts a new worksheet and finishes the previous one. The name must
// be a legal Excel sheet name and must not repeat one already added.
func (w *StreamWriter) AddSheet(name string) (*StreamSheet, error) {
	if w.closed {
		return nil, fmt.Errorf("xlsx: AddSheet after Close")
	}
	if err := ValidateSheetName(name); err != nil {
		return nil, err
	}
	for _, s := range w.sheets {
		if strings.EqualFold(s.name, name) {
			return nil, fmt.Errorf("%w: %q", ErrDuplicateSheetName, name)
		}
	}
	if err := w.finishCurrent(); err != nil {
		return nil, err
	}

	idx := len(w.sheets) + 1
	ref := streamSheetRef{
		name:     name,
		partName: fmt.Sprintf("/xl/worksheets/sheet%d.xml", idx),
		relID:    fmt.Sprintf("rId%d", idx),
		sheetID:  uint32(idx),
	}
	part, err := w.pkg.CreatePart(ref.partName, opc.ContentTypeWorksheet, opc.CompressionDeflate)
	if err != nil {
		return nil, err
	}

	b := xmlb.NewSpreadsheetMLBuilder()
	b.WriteHeader()
	b.StartElementWithNS(nsSML, "worksheet", xmlb.SpreadsheetMLNamespaces())

	w.sheets = append(w.sheets, ref)
	w.cur = &StreamSheet{w: w, b: b, part: part, partName: ref.partName, flushEvery: streamFlushRows}
	return w.cur, nil
}

// Close finishes the open sheet, writes the workbook part and its
// relationships, and completes the package. It must be called for the output to
// be a readable workbook.
func (w *StreamWriter) Close() error {
	if w.closed {
		return opc.ErrPackageClosed
	}
	if err := w.finishCurrent(); err != nil {
		w.closed = true
		_ = w.pkg.Abort()
		return err
	}
	w.closed = true
	if len(w.sheets) == 0 {
		_ = w.pkg.Abort()
		return ErrNoSheets
	}

	wb := &oxml.CT_Workbook{}
	var rels []*opc.Relationship
	for _, s := range w.sheets {
		wb.Sheets.Sheet = append(wb.Sheets.Sheet, oxml.CT_Sheet{
			Name: s.name, SheetId: s.sheetID, RID: s.relID,
		})
		rels = append(rels, &opc.Relationship{
			ID:     s.relID,
			Type:   opc.RelTypeWorksheet,
			Target: strings.TrimPrefix(s.partName, "/xl/"),
		})
	}
	data, err := marshalWorkbookXML(wb)
	if err != nil {
		_ = w.pkg.Abort()
		return err
	}
	const mainPart = defaultMainPartName
	if err := w.pkg.WritePart(mainPart, opc.ContentTypeWorkbook, data); err != nil {
		_ = w.pkg.Abort()
		return err
	}
	if err := w.pkg.WritePartRelationships(mainPart, rels); err != nil {
		_ = w.pkg.Abort()
		return err
	}
	if _, err := w.pkg.AddRelationship(opc.RelTypeOfficeDocument, strings.TrimPrefix(mainPart, "/"), opc.TargetModeInternal); err != nil {
		_ = w.pkg.Abort()
		return err
	}
	return w.pkg.Close()
}

// finishCurrent closes the open sheet's XML and drains it into the package.
func (w *StreamWriter) finishCurrent() error {
	if w.cur == nil {
		return nil
	}
	s := w.cur
	w.cur = nil
	return s.finish()
}

// StreamSheet is one worksheet of a StreamWriter, written a row at a time.
type StreamSheet struct {
	w        *StreamWriter
	b        *xmlb.Builder
	part     io.Writer
	partName string
	cols     []oxml.CT_Col
	// flushEvery is streamFlushRows, overridden in tests. The block size is a
	// buffering choice and must not change a byte of the output; a test writes
	// the same sheet at several sizes and compares.
	flushEvery int
	lastRow    int
	pending    int
	started    bool // <sheetData> has been opened, so no more <cols>
	done       bool
}

// SetColWidth sets a column's width, in characters. It must be called before
// the sheet's first row: <cols> precedes <sheetData> in schema order, and by
// the time a row has been written the point where it belongs has gone past.
func (s *StreamSheet) SetColWidth(col int, width float64) error {
	if s.done {
		return fmt.Errorf("xlsx: SetColWidth on a finished sheet")
	}
	if s.started {
		return fmt.Errorf("xlsx: SetColWidth after the first row; column widths precede the rows in the file")
	}
	if col < 1 || col > MaxCol {
		return ErrInvalidCell
	}
	c := uint32(col)
	w := width
	customWidth := true
	s.cols = append(s.cols, oxml.CT_Col{Min: c, Max: c, Width: &w, CustomWidth: &customWidth})
	return nil
}

// AppendRow writes one row below the last, and returns its 1-based number. The
// values are converted exactly as Cell.SetValue converts them.
//
// A nil value leaves that cell empty; trailing nils cost nothing, so a short
// row and a padded one produce the same bytes.
func (s *StreamSheet) AppendRow(values ...any) (int, error) {
	return s.WriteRowAt(s.lastRow+1, values...)
}

// WriteRowAt writes a row at an explicit number, which must be greater than the
// last row written: rows are emitted in order and cannot be revisited. Skipping
// numbers is allowed and leaves the gap empty, as a sparse sheet does.
func (s *StreamSheet) WriteRowAt(row int, values ...any) (int, error) {
	if s.done {
		return 0, fmt.Errorf("xlsx: row written to a finished sheet")
	}
	if row < 1 || row > MaxRow {
		return 0, ErrInvalidCell
	}
	if row <= s.lastRow {
		return 0, fmt.Errorf("xlsx: row %d is not past the last row written (%d); a stream cannot revisit a row", row, s.lastRow)
	}
	if len(values) > MaxCol {
		return 0, fmt.Errorf("xlsx: %d values exceeds the %d columns of a worksheet", len(values), MaxCol)
	}
	if err := s.beginData(); err != nil {
		return 0, err
	}

	rn := uint32(row)
	r := oxml.CT_Row{R: &rn}
	for i, v := range values {
		if v == nil {
			continue
		}
		cell := oxml.NewCell("")
		cell.SetPosition(row, i+1)
		// Reuse the public setter so a streamed value is encoded exactly as a
		// value written through the DOM: one conversion, not two.
		(&Cell{cell: cell}).SetValue(v)
		r.C = append(r.C, cell)
	}
	r.MarshalToBuilder(s.b, nsSML, "row")

	s.lastRow = row
	s.pending++
	if s.pending >= s.flushEvery {
		if err := s.drain(); err != nil {
			return 0, err
		}
	}
	return row, nil
}

// Rows returns the number of the last row written, or 0 before the first.
func (s *StreamSheet) Rows() int { return s.lastRow }

// beginData emits the column widths and opens <sheetData>, once.
func (s *StreamSheet) beginData() error {
	if s.started {
		return nil
	}
	s.started = true
	if len(s.cols) > 0 {
		group := oxml.CT_Cols{Col: s.cols}
		s.b.MarshalElement(nsSML, "cols", &group)
		s.cols = nil
	}
	s.b.StartElement(nsSML, "sheetData")
	return s.drain()
}

// drain moves the built bytes into the package part.
func (s *StreamSheet) drain() error {
	s.pending = 0
	_, err := s.b.DrainTo(s.part)
	return err
}

// finish closes the sheet's elements and drains the remainder.
func (s *StreamSheet) finish() error {
	if s.done {
		return nil
	}
	s.done = true
	// A sheet with no rows still needs a sheetData element; an empty worksheet
	// without one is schema-invalid.
	if err := s.beginData(); err != nil {
		return err
	}
	s.b.EndElement(nsSML, "sheetData")
	s.b.EndElement(nsSML, "worksheet")
	if err := s.b.Finish(); err != nil {
		return err
	}
	return s.drain()
}
