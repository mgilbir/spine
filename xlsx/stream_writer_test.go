package xlsx

import (
	"bytes"
	"errors"
	"testing"
	"time"
)

func TestStreamWriterCreateAndReopen(t *testing.T) {
	wb := Create()
	sheet := wb.AddSheet("Streamed")
	sw, err := sheet.NewStreamWriter()
	if err != nil {
		t.Fatalf("NewStreamWriter error: %v", err)
	}
	if err := sw.SetRow("A1", []any{"name", "count"}); err != nil {
		t.Fatalf("SetRow A1 error: %v", err)
	}
	if err := sw.SetRow("A2", []any{"apples", 3}); err != nil {
		t.Fatalf("SetRow A2 error: %v", err)
	}

	buf, err := wb.WriteToBuffer()
	if err != nil {
		t.Fatalf("WriteToBuffer error: %v", err)
	}

	reopened, err := OpenReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("OpenReader error: %v", err)
	}
	defer func() { _ = reopened.Close() }()

	gotSheet, err := reopened.SheetByName("Streamed")
	if err != nil {
		t.Fatalf("SheetByName error: %v", err)
	}

	val, err := gotSheet.GetCellValue("A2")
	if err != nil {
		t.Fatalf("GetCellValue A2 error: %v", err)
	}
	if val != "apples" {
		t.Fatalf("A2 = %q, want %q", val, "apples")
	}
	val, err = gotSheet.GetCellValue("B2")
	if err != nil {
		t.Fatalf("GetCellValue B2 error: %v", err)
	}
	if val != "3" {
		t.Fatalf("B2 = %q, want %q", val, "3")
	}
}

func TestStreamWriterPreservesDimensionFromStartCell(t *testing.T) {
	wb := Create()
	sheet := wb.AddSheet("Offset")
	sw, err := sheet.NewStreamWriter()
	if err != nil {
		t.Fatalf("NewStreamWriter error: %v", err)
	}
	if err := sw.SetRow("C5", []any{"name", 7}); err != nil {
		t.Fatalf("SetRow C5 error: %v", err)
	}

	buf, err := wb.WriteToBuffer()
	if err != nil {
		t.Fatalf("WriteToBuffer error: %v", err)
	}

	reopened, err := OpenReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("OpenReader error: %v", err)
	}
	defer func() { _ = reopened.Close() }()

	gotSheet, err := reopened.SheetByName("Offset")
	if err != nil {
		t.Fatalf("SheetByName error: %v", err)
	}
	if gotSheet.worksheet == nil || gotSheet.worksheet.Dimension == nil {
		t.Fatalf("expected worksheet dimension to be set")
	}
	if gotSheet.worksheet.Dimension.Ref != "C5:D5" {
		t.Fatalf("dimension = %q, want %q", gotSheet.worksheet.Dimension.Ref, "C5:D5")
	}
}

func TestStreamWriterPreservesTimeValues(t *testing.T) {
	wb := Create()
	sheet := wb.AddSheet("Times")
	sw, err := sheet.NewStreamWriter()
	if err != nil {
		t.Fatalf("NewStreamWriter error: %v", err)
	}
	wantTime := time.Date(2026, time.May, 4, 15, 30, 0, 0, time.UTC)
	if err := sw.SetRow("A1", []any{wantTime}); err != nil {
		t.Fatalf("SetRow A1 error: %v", err)
	}

	buf, err := wb.WriteToBuffer()
	if err != nil {
		t.Fatalf("WriteToBuffer error: %v", err)
	}

	reopened, err := OpenReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("OpenReader error: %v", err)
	}
	defer func() { _ = reopened.Close() }()

	gotSheet, err := reopened.SheetByName("Times")
	if err != nil {
		t.Fatalf("SheetByName error: %v", err)
	}
	cell, err := gotSheet.Cell("A1")
	if err != nil {
		t.Fatalf("Cell A1 error: %v", err)
	}
	if cell.Type() != CellTypeNumber {
		t.Fatalf("A1 type = %v, want %v", cell.Type(), CellTypeNumber)
	}
	gotTime := cell.Time().UTC()
	delta := gotTime.Sub(wantTime)
	if delta < 0 {
		delta = -delta
	}
	if delta > time.Microsecond {
		t.Fatalf("A1 time = %v, want %v (delta %v)", gotTime, wantTime, delta)
	}
	value, err := gotSheet.GetCellValue("A1")
	if err != nil {
		t.Fatalf("GetCellValue A1 error: %v", err)
	}
	if value == wantTime.String() {
		t.Fatalf("A1 value was serialized as a string: %q", value)
	}
}

func TestStreamWriterRejectsOutOfOrderRows(t *testing.T) {
	wb := Create()
	sheet := wb.AddSheet("Streamed")
	sw, err := sheet.NewStreamWriter()
	if err != nil {
		t.Fatalf("NewStreamWriter error: %v", err)
	}
	if err := sw.SetRow("A2", []any{"later"}); err != nil {
		t.Fatalf("SetRow A2 error: %v", err)
	}
	if err := sw.SetRow("A2", []any{"again"}); !errors.Is(err, ErrStreamWriterOrder) {
		t.Fatalf("SetRow duplicate row error = %v, want %v", err, ErrStreamWriterOrder)
	}
}

func TestStreamWriterRejectsMixedModeWrites(t *testing.T) {
	wb := Create()
	sheet := wb.AddSheet("Streamed")
	_, err := sheet.NewStreamWriter()
	if err != nil {
		t.Fatalf("NewStreamWriter error: %v", err)
	}
	if _, err := sheet.Cell("A1"); !errors.Is(err, ErrStreamWriterMixedMode) {
		t.Fatalf("Cell error = %v, want %v", err, ErrStreamWriterMixedMode)
	}
	if err := sheet.SetCellValue("A1", "x"); !errors.Is(err, ErrStreamWriterMixedMode) {
		t.Fatalf("SetCellValue error = %v, want %v", err, ErrStreamWriterMixedMode)
	}
}

func TestWorkbookWithMixedNormalAndStreamedSheets(t *testing.T) {
	wb := Create()
	normal := wb.AddSheet("Normal")
	if err := normal.SetCellValue("A1", "left"); err != nil {
		t.Fatalf("normal SetCellValue error: %v", err)
	}
	streamed := wb.AddSheet("Streamed")
	sw, err := streamed.NewStreamWriter()
	if err != nil {
		t.Fatalf("NewStreamWriter error: %v", err)
	}
	if err := sw.SetRow("A1", []any{"right"}); err != nil {
		t.Fatalf("SetRow error: %v", err)
	}

	buf, err := wb.WriteToBuffer()
	if err != nil {
		t.Fatalf("WriteToBuffer error: %v", err)
	}

	reopened, err := OpenReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("OpenReader error: %v", err)
	}
	defer func() { _ = reopened.Close() }()

	left, err := reopened.SheetByName("Normal")
	if err != nil {
		t.Fatalf("SheetByName Normal error: %v", err)
	}
	leftVal, err := left.GetCellValue("A1")
	if err != nil {
		t.Fatalf("Normal GetCellValue error: %v", err)
	}
	if leftVal != "left" {
		t.Fatalf("Normal A1 = %q, want %q", leftVal, "left")
	}

	right, err := reopened.SheetByName("Streamed")
	if err != nil {
		t.Fatalf("SheetByName Streamed error: %v", err)
	}
	rightVal, err := right.GetCellValue("A1")
	if err != nil {
		t.Fatalf("Streamed GetCellValue error: %v", err)
	}
	if rightVal != "right" {
		t.Fatalf("Streamed A1 = %q, want %q", rightVal, "right")
	}
}

func TestStreamWriterFlushIdempotence(t *testing.T) {
	wb := Create()
	sheet := wb.AddSheet("Streamed")
	sw, err := sheet.NewStreamWriter()
	if err != nil {
		t.Fatalf("NewStreamWriter error: %v", err)
	}
	if err := sw.Flush(); err != nil {
		t.Fatalf("first Flush error: %v", err)
	}
	if err := sw.Flush(); err != nil {
		t.Fatalf("second Flush error: %v", err)
	}
	if err := sw.SetRow("A1", []any{"x"}); !errors.Is(err, ErrStreamWriterClosed) {
		t.Fatalf("SetRow after Flush error = %v, want %v", err, ErrStreamWriterClosed)
	}
}

func TestStreamWriterErrorsWhenBufferLimitExceeded(t *testing.T) {
	originalLimit := streamWriterBufferLimit
	streamWriterBufferLimit = 32
	defer func() { streamWriterBufferLimit = originalLimit }()

	wb := Create()
	sheet := wb.AddSheet("Streamed")
	sw, err := sheet.NewStreamWriter()
	if err != nil {
		t.Fatalf("NewStreamWriter error: %v", err)
	}
	err = sw.SetRow("A1", []any{"this is a long value that exceeds the in-memory buffer"})
	if !errors.Is(err, ErrStreamWriterMemoryLimit) {
		t.Fatalf("SetRow error = %v, want %v", err, ErrStreamWriterMemoryLimit)
	}
	if sw.fragments.buf.Len() != 0 {
		t.Fatalf("expected failed write to leave buffer empty, got %d bytes", sw.fragments.buf.Len())
	}
	if err := wb.Close(); err != nil {
		t.Fatalf("Workbook Close error: %v", err)
	}
}

func TestWorkbookCloseAfterBufferLimitExceeded(t *testing.T) {
	originalLimit := streamWriterBufferLimit
	streamWriterBufferLimit = 32
	defer func() { streamWriterBufferLimit = originalLimit }()

	wb := Create()
	sheet := wb.AddSheet("Streamed")
	sw, err := sheet.NewStreamWriter()
	if err != nil {
		t.Fatalf("NewStreamWriter error: %v", err)
	}
	err = sw.SetRow("A1", []any{"this is a long value that exceeds the in-memory buffer"})
	if !errors.Is(err, ErrStreamWriterMemoryLimit) {
		t.Fatalf("SetRow error = %v, want %v", err, ErrStreamWriterMemoryLimit)
	}

	if err := wb.Close(); err != nil {
		t.Fatalf("Workbook Close error: %v", err)
	}
}
