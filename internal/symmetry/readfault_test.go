package symmetry_test

import (
	"bytes"
	"testing"

	"github.com/mgilbir/spine/docx"
	"github.com/mgilbir/spine/internal/faultio"
	"github.com/mgilbir/spine/opc"
	"github.com/mgilbir/spine/pptx"
	"github.com/mgilbir/spine/xlsx"
)

// A read that fails part way is reported, not absorbed.
//
// A package is opened through an io.ReaderAt the library does not control — a
// file on failing storage, a network-backed reader, a truncated download — and
// the failure mode on this side is worse than on the write side. A short write
// loses bytes the caller still has; a short read that is not reported produces
// a *document* that looks complete and is not: a sheet missing its rows, a
// slide missing its shapes, with no error anywhere to act on.
//
// Nothing exercised it: every reader test in the module reads from a
// bytes.Reader that cannot fail. The contract asserted here is the minimum a
// caller can act on — an error or a document, never both, never neither, and
// never a silent truncation.

func TestOpenFromAFailingReaderIsReported(t *testing.T) {
	// One package per format, saved cleanly, then re-opened through a reader
	// that fails at a series of offsets. The offsets walk the whole file so the
	// failure lands in the local headers, the part data, and the central
	// directory in turn — the three places an open reads.
	packages := map[string][]byte{
		"docx": func() []byte {
			d := docx.Create()
			d.AddParagraph().SetText("hello")
			d.AddTable(2, 2)
			return saveDocument(t, d)
		}(),
		"xlsx": func() []byte {
			w := xlsx.Create()
			s := mustSheet(t, w, "Data")
			for _, ref := range []string{"A1", "A2", "B1"} {
				if err := s.SetCellValue(ref, ref); err != nil {
					t.Fatal(err)
				}
			}
			return saveWorkbook(t, w)
		}(),
		"pptx": func() []byte {
			p := pptx.Create()
			addTextSlide(t, p, "hello")
			return savePresentation(t, p)
		}(),
	}

	opens := map[string]func(data []byte, r *readerAtCounter) error{
		"opc": func(data []byte, r *readerAtCounter) error {
			rc, err := opc.NewReader(r, int64(len(data)))
			if err != nil {
				if rc != nil {
					t.Error("opc.NewReader returned a Reader alongside an error")
				}
				return err
			}
			if rc == nil {
				t.Fatal("opc.NewReader returned neither a Reader nor an error")
			}
			// Reading every part is where a truncation would surface as bytes
			// rather than as an error.
			for _, f := range rc.Files {
				if _, err := f.ReadAll(); err != nil {
					return err
				}
			}
			return nil
		},
		"docx": func(data []byte, r *readerAtCounter) error {
			d, err := docx.OpenReader(r, int64(len(data)))
			if err == nil && d == nil {
				t.Fatal("docx.OpenReader returned neither a document nor an error")
			}
			if err != nil && d != nil {
				t.Error("docx.OpenReader returned a document alongside an error")
			}
			return err
		},
		"xlsx": func(data []byte, r *readerAtCounter) error {
			w, err := xlsx.OpenReader(r, int64(len(data)))
			if err == nil && w == nil {
				t.Fatal("xlsx.OpenReader returned neither a workbook nor an error")
			}
			if err != nil && w != nil {
				t.Error("xlsx.OpenReader returned a workbook alongside an error")
			}
			if w != nil {
				// Sheets parse lazily, so the read that can still fail happens
				// here rather than during the open.
				for _, s := range w.Sheets() {
					_, _ = s.CellValue("A1")
				}
				_ = w.Close()
			}
			return err
		},
		"pptx": func(data []byte, r *readerAtCounter) error {
			p, err := pptx.OpenReader(r, int64(len(data)))
			if err == nil && p == nil {
				t.Fatal("pptx.OpenReader returned neither a presentation nor an error")
			}
			if err != nil && p != nil {
				t.Error("pptx.OpenReader returned a presentation alongside an error")
			}
			if p != nil {
				for i := range p.Slides() {
					if s, err := p.Slide(i); err == nil {
						_ = s.Shapes()
					}
				}
			}
			return err
		},
	}

	for format, pkg := range packages {
		for opener, open := range opens {
			if opener != "opc" && opener != format {
				continue // a format opener only gets its own package
			}
			t.Run(format+"/"+opener, func(t *testing.T) {
				// A quarter of the offsets is enough to land in each region
				// without running the whole suite for every byte.
				step := len(pkg) / 16
				if step == 0 {
					step = 1
				}
				reported := 0
				for off := 0; off < len(pkg); off += step {
					r := &readerAtCounter{ReaderAt: faultio.ReaderAt(pkg, int64(off))}
					err := open(pkg, r)
					if err != nil {
						reported++
					}
					if r.reads == 0 {
						t.Fatalf("offset %d: the open never read anything, so nothing was tested", off)
					}
				}
				// Failing at offset zero means not a single byte is readable,
				// which no implementation can open — if even that is not
				// reported, errors are being swallowed wholesale.
				if reported == 0 {
					t.Error("no failing offset produced an error; read failures are being absorbed")
				}
			})
		}
	}
}

// readerAtCounter records that the reader was actually used, so a case that
// silently reads nothing cannot pass for a case that survived a fault.
type readerAtCounter struct {
	ReaderAt interface {
		ReadAt(p []byte, off int64) (int, error)
	}
	reads int
}

func (r *readerAtCounter) ReadAt(p []byte, off int64) (int, error) {
	r.reads++
	return r.ReaderAt.ReadAt(p, off)
}

// A part whose bytes cannot be read must not come back as a short part.
//
// This is the specific failure the contract above exists for: ReadAll returning
// what it managed to read, with a nil error, leaves a caller holding a
// truncated part it cannot distinguish from a small one.
//
// The fault is a window rather than a suffix. A fault that runs to the end of
// the file stops the package from opening at all — a zip is read from its
// central directory at the end — so the part-reading path is never reached and
// the test proves nothing. That is not hypothetical: the first version of this
// test was written that way, and a planted ReadAll that swallowed its error
// passed it.
func TestTruncatedPartIsNotReturnedAsShortData(t *testing.T) {
	w := xlsx.Create()
	s := mustSheet(t, w, "Data")
	for i := 1; i <= 200; i++ {
		if err := s.SetCellValue(cellRef(i), "value that takes up room"); err != nil {
			t.Fatal(err)
		}
	}
	pkg := saveWorkbook(t, w)

	full, err := opc.NewReader(bytes.NewReader(pkg), int64(len(pkg)))
	if err != nil {
		t.Fatalf("NewReader over intact bytes: %v", err)
	}
	sizes := map[string]int{}
	for _, f := range full.Files {
		data, err := f.ReadAll()
		if err != nil {
			t.Fatalf("ReadAll(%s) over intact bytes: %v", f.Name, err)
		}
		sizes[f.Name] = len(data)
	}

	var opened, faultsHit int
	for from := int64(64); from < int64(len(pkg))-64; from += int64(len(pkg)) / 12 {
		r, err := opc.NewReader(faultio.ReaderAtWindow(pkg, from, from+96), int64(len(pkg)))
		if err != nil {
			continue // refusing to open at all is a fine outcome
		}
		opened++
		for _, f := range r.Files {
			data, err := f.ReadAll()
			if err != nil {
				faultsHit++
				continue // reported, which is the contract
			}
			if want := sizes[f.Name]; len(data) != want {
				t.Errorf("window at %d: ReadAll(%s) returned %d bytes with no error, but the part is %d — "+
					"a truncated part is indistinguishable from a small one",
					from, f.Name, len(data), want)
			}
		}
	}
	// Without these the test passes on a build where nothing opens, or where
	// the fault never lands on a part anyone reads.
	if opened == 0 {
		t.Fatal("no windowed fault left the package openable, so no part read was exercised")
	}
	if faultsHit == 0 {
		t.Fatal("no part read ever hit the fault, so a swallowed read error would go unnoticed here")
	}
}

// cellRef returns A1, A2, ... for a row number.
func cellRef(row int) string {
	return "A" + itoa(row)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
