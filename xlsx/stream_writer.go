package xlsx

import (
	"bytes"
	"fmt"
	"time"

	xmlb "github.com/mgilbir/spine/common/xml"
	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// StreamWriter incrementally writes rows for a single sheet.
type StreamWriter struct {
	sheet   *Sheet
	buf     bytes.Buffer
	lastRow int
	maxCol  int
	flushed bool
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
	sw.buf.Write(b.Bytes())

	sw.lastRow = row
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
	sw.flushed = true
	return nil
}

func (sw *StreamWriter) maxRow() int {
	return sw.lastRow
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
