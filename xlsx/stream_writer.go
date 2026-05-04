package xlsx

import (
	"bytes"
	"fmt"
	"io"
	"time"

	xmlb "github.com/mgilbir/spine/common/xml"
	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

var streamWriterBufferLimit = 8 << 20

// StreamWriter incrementally writes rows for a single sheet.
type StreamWriter struct {
	sheet     *Sheet
	fragments *streamFragmentStore
	lastRow   int
	minRow    int
	minCol    int
	maxRowNum int
	maxCol    int
	flushed   bool
}

// SetRow writes a row starting at the given cell reference.
func (sw *StreamWriter) SetRow(startCell string, values []any) error {
	if sw.flushed {
		return ErrStreamWriterClosed
	}

	row, col, err := ParseCellRef(startCell)
	if err != nil {
		return err
	}
	if row <= sw.lastRow {
		return ErrStreamWriterOrder
	}

	rowModel := oxml.CT_Row{}
	r := uint32(row)
	rowModel.R = &r

	for i, value := range values {
		ref, err := CellRef(row, col+i)
		if err != nil {
			return err
		}

		cellModel := oxml.CT_Cell{R: ref}
		cell := &Cell{sheet: sw.sheet, cell: &cellModel}
		cell.SetValue(normalizeStreamValue(value))
		if !cell.IsEmpty() {
			rowModel.C = append(rowModel.C, cellModel)
		}
	}

	b := xmlb.NewSpreadsheetMLBuilder()
	rowModel.MarshalToBuilder(b, nsSML, "row")
	if err := sw.fragments.Write(b.Bytes()); err != nil {
		return err
	}

	sw.lastRow = row
	if sw.minRow == 0 || row < sw.minRow {
		sw.minRow = row
	}
	if sw.minCol == 0 || col < sw.minCol {
		sw.minCol = col
	}
	if row > sw.maxRowNum {
		sw.maxRowNum = row
	}
	if endCol := col + len(values) - 1; endCol > sw.maxCol {
		sw.maxCol = endCol
	}
	return nil
}

// Flush marks the stream writer complete.
func (sw *StreamWriter) Flush() error {
	if sw.flushed {
		return nil
	}
	if err := sw.fragments.Flush(); err != nil {
		return err
	}
	sw.flushed = true
	return nil
}

func (sw *StreamWriter) maxRow() int {
	return sw.maxRowNum
}

func normalizeStreamValue(value any) any {
	switch v := value.(type) {
	case time.Time:
		return v
	case fmt.Stringer:
		return v.String()
	case []byte:
		return string(v)
	default:
		return value
	}
}

type streamFragmentStore struct {
	buf    bytes.Buffer
	closed bool
}

func newStreamFragmentStore() *streamFragmentStore {
	return &streamFragmentStore{}
}

func (s *streamFragmentStore) Write(p []byte) error {
	if s.closed {
		return ErrStreamWriterClosed
	}
	if s.buf.Len()+len(p) > streamWriterBufferLimit {
		return ErrStreamWriterMemoryLimit
	}
	_, err := s.buf.Write(p)
	return err
}

func (s *streamFragmentStore) Flush() error {
	return nil
}

func (s *streamFragmentStore) Reader() (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(s.buf.Bytes())), nil
}

func (s *streamFragmentStore) Close() error {
	if s.closed {
		return nil
	}
	s.closed = true
	return nil
}
