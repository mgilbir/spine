package xlsx

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"time"

	xmlb "github.com/mgilbir/spine/common/xml"
	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

var streamWriterSpillThreshold = 8 << 20

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
	case fmt.Stringer:
		return v.String()
	case []byte:
		return string(v)
	case time.Time:
		return v
	default:
		return value
	}
}

type streamFragmentStore struct {
	buf      bytes.Buffer
	tempFile *os.File
	tempPath string
	closed   bool
}

func newStreamFragmentStore() *streamFragmentStore {
	return &streamFragmentStore{}
}

func (s *streamFragmentStore) Write(p []byte) error {
	if s.closed {
		return ErrStreamWriterClosed
	}
	if s.tempFile == nil && s.buf.Len()+len(p) > streamWriterSpillThreshold {
		if err := s.spillToDisk(); err != nil {
			return err
		}
	}
	if s.tempFile != nil {
		_, err := s.tempFile.Write(p)
		return err
	}
	_, err := s.buf.Write(p)
	return err
}

func (s *streamFragmentStore) Flush() error {
	if s.tempFile != nil {
		return s.tempFile.Sync()
	}
	return nil
}

func (s *streamFragmentStore) Reader() (io.ReadCloser, error) {
	if s.tempPath != "" {
		return os.Open(s.tempPath)
	}
	return io.NopCloser(bytes.NewReader(s.buf.Bytes())), nil
}

func (s *streamFragmentStore) Close() error {
	if s.closed {
		return nil
	}
	s.closed = true

	var err error
	if s.tempFile != nil {
		err = s.tempFile.Close()
		s.tempFile = nil
	}
	if s.tempPath != "" {
		removeErr := os.Remove(s.tempPath)
		s.tempPath = ""
		if err == nil {
			err = removeErr
		}
	}
	return err
}

func (s *streamFragmentStore) usingTempFile() bool {
	return s.tempPath != ""
}

func (s *streamFragmentStore) spillToDisk() error {
	tempFile, err := os.CreateTemp("", "spine-xlsx-stream-*.xml")
	if err != nil {
		return err
	}
	if _, err := tempFile.Write(s.buf.Bytes()); err != nil {
		_ = tempFile.Close()
		_ = os.Remove(tempFile.Name())
		return err
	}
	s.buf.Reset()
	s.tempFile = tempFile
	s.tempPath = tempFile.Name()
	return nil
}
