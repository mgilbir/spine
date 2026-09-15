package xlsx

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
)

// TestStreamedWorkbookMatchesTheBuiltOne is the property that makes the stream
// writer trustworthy: for the same values it must produce the same workbook the
// DOM path produces. Both go through one row marshaller, so a difference means
// the streaming path has grown a serializer of its own.
func TestStreamedWorkbookMatchesTheBuiltOne(t *testing.T) {
	const rows, cols = 40, 6
	value := func(r, c int) any {
		switch c % 3 {
		case 0:
			return float64(r*c) / 4
		case 1:
			return fmt.Sprintf("r%dc%d", r, c)
		default:
			return r%2 == 0
		}
	}

	var streamed bytes.Buffer
	sw := NewStreamWriter(&streamed)
	ss, err := sw.AddSheet("Data")
	if err != nil {
		t.Fatal(err)
	}
	if err := ss.SetColWidth(2, 18.5); err != nil {
		t.Fatal(err)
	}
	for r := 1; r <= rows; r++ {
		vals := make([]any, cols)
		for c := 1; c <= cols; c++ {
			vals[c-1] = value(r, c)
		}
		if _, err := ss.AppendRow(vals...); err != nil {
			t.Fatalf("row %d: %v", r, err)
		}
	}
	if err := sw.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}

	wb := Create()
	sh, err := wb.AddSheet("Data")
	if err != nil {
		t.Fatal(err)
	}
	if err := sh.SetColWidth(2, 18.5); err != nil {
		t.Fatal(err)
	}
	for r := 1; r <= rows; r++ {
		for c := 1; c <= cols; c++ {
			ref, err := CellRef(r, c)
			if err != nil {
				t.Fatal(err)
			}
			if err := sh.SetCellValue(ref, value(r, c)); err != nil {
				t.Fatal(err)
			}
		}
	}
	built, err := wb.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}

	gotXML, err := readPartFromZip(streamed.Bytes(), "xl/worksheets/sheet1.xml")
	if err != nil {
		t.Fatal(err)
	}
	wantXML, err := readPartFromZip(built, "xl/worksheets/sheet1.xml")
	if err != nil {
		t.Fatal(err)
	}
	// The DOM path records a used range; a stream cannot know one before the
	// last row, and the element is optional. That is the one difference.
	wantXML = stripDimension(wantXML)
	if gotXML != wantXML {
		t.Errorf("streamed worksheet differs from the built one\n got: %s\nwant: %s",
			firstDifference(gotXML, wantXML), firstDifference(wantXML, gotXML))
	}
}

func stripDimension(s string) string {
	i := strings.Index(s, "<dimension")
	if i < 0 {
		return s
	}
	j := strings.Index(s[i:], "/>")
	if j < 0 {
		return s
	}
	return s[:i] + s[i+j+2:]
}

// firstDifference returns a window of a around where it first differs from b.
func firstDifference(a, b string) string {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			lo := i - 60
			if lo < 0 {
				lo = 0
			}
			hi := i + 60
			if hi > len(a) {
				hi = len(a)
			}
			return fmt.Sprintf("...%s... (at %d)", a[lo:hi], i)
		}
	}
	if len(a) != len(b) {
		return fmt.Sprintf("length %d vs %d", len(a), len(b))
	}
	return "(identical)"
}

// TestStreamedWorkbookReopens checks the package is a workbook spine itself can
// read back, with the values intact.
func TestStreamedWorkbookReopens(t *testing.T) {
	var buf bytes.Buffer
	sw := NewStreamWriter(&buf)
	for _, name := range []string{"First", "Second"} {
		ss, err := sw.AddSheet(name)
		if err != nil {
			t.Fatal(err)
		}
		for r := 1; r <= 5; r++ {
			if _, err := ss.AppendRow(name, float64(r), r%2 == 0); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := sw.Close(); err != nil {
		t.Fatal(err)
	}

	wb, err := OpenReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if got := wb.SheetCount(); got != 2 {
		t.Fatalf("sheet count = %d, want 2", got)
	}
	for i, name := range []string{"First", "Second"} {
		sh, err := wb.Sheet(i)
		if err != nil {
			t.Fatal(err)
		}
		if sh.Name() != name {
			t.Errorf("sheet %d name = %q, want %q", i, sh.Name(), name)
		}
		if c := sh.FindCell("A3"); c == nil || c.String() != name {
			t.Errorf("%s!A3 = %v, want %q", name, c, name)
		}
		if c := sh.FindCell("B4"); c == nil || c.Float() != 4 {
			t.Errorf("%s!B4 = %v, want 4", name, c)
		}
	}
}

// TestStreamRejectsGoingBackwards pins the constraint that makes streaming
// possible at all: a row that has left cannot be revisited.
func TestStreamRejectsGoingBackwards(t *testing.T) {
	var buf bytes.Buffer
	sw := NewStreamWriter(&buf)
	ss, err := sw.AddSheet("S")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ss.WriteRowAt(10, "x"); err != nil {
		t.Fatal(err)
	}
	if _, err := ss.WriteRowAt(10, "y"); err == nil {
		t.Error("rewriting the last row was accepted")
	}
	if _, err := ss.WriteRowAt(3, "y"); err == nil {
		t.Error("writing an earlier row was accepted")
	}
	// A gap is fine: sheets are sparse.
	if _, err := ss.WriteRowAt(20, "z"); err != nil {
		t.Errorf("skipping rows was rejected: %v", err)
	}
	// Column widths belong before the rows.
	if err := ss.SetColWidth(1, 10); err == nil {
		t.Error("SetColWidth after a row was accepted")
	}
	if err := sw.Close(); err != nil {
		t.Fatal(err)
	}
}

// TestStreamFlushBlockSizeDoesNotChangeOutput pins that the block size is a
// buffering choice and nothing more. Drains happen at arbitrary points in the
// row stream, and if any of them perturbed the XML — a lost separator, a
// mis-set baseline — the bytes would depend on how often the buffer was
// emptied, which is exactly the class of bug streaming invites.
func TestStreamFlushBlockSizeDoesNotChangeOutput(t *testing.T) {
	build := func(every int) string {
		var buf bytes.Buffer
		sw := NewStreamWriter(&buf)
		ss, err := sw.AddSheet("Data")
		if err != nil {
			t.Fatal(err)
		}
		ss.flushEvery = every
		if err := ss.SetColWidth(1, 12); err != nil {
			t.Fatal(err)
		}
		for r := 1; r <= 37; r++ {
			if _, err := ss.AppendRow("a"+fmt.Sprint(r), float64(r), nil, r%2 == 0); err != nil {
				t.Fatal(err)
			}
		}
		if err := sw.Close(); err != nil {
			t.Fatal(err)
		}
		xml, err := readPartFromZip(buf.Bytes(), "xl/worksheets/sheet1.xml")
		if err != nil {
			t.Fatal(err)
		}
		return xml
	}

	want := build(1 << 30) // effectively never
	for _, every := range []int{1, 2, 3, 5, 13, 37, 38} {
		if got := build(every); got != want {
			t.Errorf("flushing every %d rows changed the output:\n%s", every, firstDifference(got, want))
		}
	}
}
