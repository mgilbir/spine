package symmetry_test

import (
	"archive/zip"
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"testing"

	"github.com/mgilbir/spine/docx"
	"github.com/mgilbir/spine/internal/faultio"
	"github.com/mgilbir/spine/pptx"
	"github.com/mgilbir/spine/xlsx"
)

// A save that fails leaves the document able to save.
//
// The three formats are held to it together because it is one property with
// three implementations, and because the ways they can break it differ: a save
// stamps the modified timestamp, embeds pending media, allocates shape ids, and
// records how many table parts were written before this session. Each is state
// the *next* save reads. A save that fails after some of them and before the
// rest leaves a document whose next save is wrong — in the format whose save
// happens to mutate the most, not in the one anybody thought to test.
//
// Nothing exercised this: not one test in the module passed a failing writer to
// SaveTo. The property holds today for all three, which is worth pinning before
// the next change to a save path rather than after.

func testImage(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// saver is one format's build-and-save, reduced to what this property needs.
type saver struct {
	// build returns a fresh document with pending edits of the kind a save
	// flushes: media to embed, ids to allocate, a timestamp to stamp.
	build func(t *testing.T) func(w io.Writer) error
}

func TestSaveIntoAFailingWriterLeavesTheDocumentUsable(t *testing.T) {
	savers := map[string]saver{
		"docx": {build: func(t *testing.T) func(io.Writer) error {
			d := docx.Create()
			d.AddParagraph().SetText("hello")
			d.AddParagraph().SetText("world")
			if _, err := d.AddParagraph().AddRun().AddImageFromBytes(testImage(t), "image/png"); err != nil {
				t.Fatalf("AddImageFromBytes: %v", err)
			}
			d.AddTable(2, 2)
			return d.SaveTo
		}},
		"xlsx": {build: func(t *testing.T) func(io.Writer) error {
			w := xlsx.Create()
			s, err := w.AddSheet("Data")
			if err != nil {
				t.Fatal(err)
			}
			for _, ref := range []string{"A1", "A2", "B1", "B2"} {
				if err := s.SetCellValue(ref, ref); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := s.AddTable("A1:B2", xlsx.TableOptions{Name: "SalesTable"}); err != nil {
				t.Fatalf("AddTable: %v", err)
			}
			return w.SaveTo
		}},
		"pptx": {build: func(t *testing.T) func(io.Writer) error {
			p := pptx.Create()
			s := p.AddSlide()
			tb := pptx.NewTextBox()
			tb.SetText("hello")
			if err := s.AddShape(tb); err != nil {
				t.Fatal(err)
			}
			if _, err := s.AddPictureFromBytes(testImage(t), "image/png"); err != nil {
				t.Fatalf("AddPictureFromBytes: %v", err)
			}
			s.SetNotes("speaker notes")
			return p.SaveTo
		}},
	}

	for format, sv := range savers {
		t.Run(format, func(t *testing.T) {
			var clean bytes.Buffer
			if err := sv.build(t)(&clean); err != nil {
				t.Fatalf("a clean save failed: %v", err)
			}

			truncated := 0
			for _, limit := range faultio.Limits(clean.Len()) {
				// A fresh document per limit: the point is what one failure
				// does to one document, not what a series of them does.
				save := sv.build(t)

				w := faultio.FailAfter(limit)
				err := save(w)
				if err == nil {
					if w.Tripped() {
						t.Errorf("limit=%d: the writer refused a write and the save reported no error",
							limit)
					}
					// Otherwise this save simply fit inside the limit — the
					// baseline it was chosen from was a byte or two larger —
					// and there is nothing to recover from.
					continue
				}
				truncated++
				if !errors.Is(err, faultio.ErrFull) {
					// Wrapping is fine; losing the cause is not, because a
					// caller distinguishes "the disk is full" from "this
					// document cannot be written" by asking.
					t.Errorf("limit=%d: save error does not wrap the writer's: %v", limit, err)
				}

				var retry bytes.Buffer
				if err := save(&retry); err != nil {
					t.Errorf("limit=%d: the document could not be saved after a failed save: %v", limit, err)
					continue
				}
				if why := sameDocument(t, clean.Bytes(), retry.Bytes()); why != "" {
					t.Errorf("limit=%d: the failed save changed the document: %s", limit, why)
				}
			}
			if truncated == 0 {
				t.Error("no limit actually truncated a save, so nothing was recovered from")
			}
		})
	}
}

// volatileParts hold values that legitimately differ between two saves of the
// same document, so they are compared by presence rather than by content.
var volatileParts = map[string]bool{
	// dcterms:modified is stamped at save time. Two saves a second apart carry
	// different timestamps, which also perturbs the compressed size of the
	// package — comparing package lengths rather than parts reports that as a
	// difference, which is what an earlier version of this test did.
	"docProps/core.xml": true,
}

// sameDocument reports how two saved packages differ, ignoring the parts whose
// content is expected to move, or "" when they carry the same document.
func sameDocument(t *testing.T, clean, retry []byte) string {
	t.Helper()
	a, b := unzip(t, clean), unzip(t, retry)

	for name := range a {
		if _, ok := b[name]; !ok {
			return "the retry dropped " + name
		}
	}
	for name := range b {
		if _, ok := a[name]; !ok {
			return "the retry added " + name
		}
	}
	for name, want := range a {
		if volatileParts[name] {
			continue
		}
		got := b[name]
		if bytes.Equal(want, got) {
			continue
		}
		if len(want) != len(got) {
			return fmt.Sprintf("%s is %d bytes after the failed save and %d after a clean one",
				name, len(got), len(want))
		}
		for i := range want {
			if want[i] != got[i] {
				lo := max(0, i-40)
				return fmt.Sprintf("%s differs at byte %d:\n\tclean: ...%s...\n\tretry: ...%s...",
					name, i, want[lo:min(i+60, len(want))], got[lo:min(i+60, len(got))])
			}
		}
	}
	return ""
}

func unzip(t *testing.T, pkg []byte) map[string][]byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(pkg), int64(len(pkg)))
	if err != nil {
		t.Fatalf("saved package is not a zip: %v", err)
	}
	out := make(map[string][]byte, len(zr.File))
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("opening %s: %v", f.Name, err)
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("reading %s: %v", f.Name, err)
		}
		out[f.Name] = data
	}
	return out
}
