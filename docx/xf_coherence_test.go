package docx

import (
	"archive/zip"
	"bytes"
	"io"
	"testing"

	"github.com/mgilbir/spine/chart"
)

// zipEntryMap reads every entry of a saved package into a name->bytes map.
func zipEntryMap(t *testing.T, data []byte) map[string][]byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("reading saved package: %v", err)
	}
	out := make(map[string][]byte, len(zr.File))
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("opening %s: %v", f.Name, err)
		}
		b, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("reading %s: %v", f.Name, err)
		}
		out[f.Name] = b
	}
	return out
}

// C439 — a created document must serialize identically however many times it is
// saved.
//
// saveNew used to draw the furniture relationship ids (styles, numbering,
// settings) from the durable nextRelID counter on EVERY save, so a document
// with one chart yielded styles=rId2 on the first save, rId3 on the second and
// rId4 on the third. The output stayed valid — the chart's pre-allocated rId1
// kept pointing at the chart — but a reproducible-build or content-hash-dedup
// pipeline saw three different packages for identical content, and only for
// docx: pptx and xlsx double-saves were already byte-identical.
func TestRepeatedSaveIsByteIdentical(t *testing.T) {
	build := func() *Document {
		d := Create()
		d.AddHeading("Title", 1)
		d.AddParagraphWithText("body")
		// A chart pre-allocates a document relationship before save, which is
		// what made the furniture ids drift past it on each subsequent pass.
		c := chart.NewBar().SetTitle("Sales").SetCategories([]string{"a", "b"})
		c.AddSeries("Q1", []float64{1, 2})
		if err := d.AddChart(c, 3000000, 2000000); err != nil {
			t.Fatalf("AddChart: %v", err)
		}
		return d
	}

	d := build()
	var saves [][]byte
	for i := 0; i < 3; i++ {
		data, err := d.SaveBytes()
		if err != nil {
			t.Fatalf("save %d: %v", i+1, err)
		}
		saves = append(saves, data)
	}

	// Core properties carry a Modified timestamp, so compare the part that the
	// finding is about rather than the whole package.
	first := mustZipEntry(t, saves[0], "word/_rels/document.xml.rels")
	for i, data := range saves[1:] {
		got := mustZipEntry(t, data, "word/_rels/document.xml.rels")
		if got != first {
			t.Errorf("save %d produced different document relationships (C439).\nfirst:\n%s\ngot:\n%s",
				i+2, first, got)
		}
	}

	// And a fresh document built the same way lands on the same ids, so the
	// determinism is of the content and not merely of one object's history.
	fresh, err := build().SaveBytes()
	if err != nil {
		t.Fatalf("fresh save: %v", err)
	}
	if got := mustZipEntry(t, fresh, "word/_rels/document.xml.rels"); got != first {
		t.Errorf("an identically built document produced different relationships.\nfirst:\n%s\ngot:\n%s", first, got)
	}

	// The whole package differs only in the timestamped core properties: assert
	// every other part is byte-identical across saves.
	for i, data := range saves[1:] {
		a := zipEntryMap(t, saves[0])
		b := zipEntryMap(t, data)
		if len(a) != len(b) {
			t.Fatalf("save %d has %d parts, first save has %d", i+2, len(b), len(a))
		}
		for name, want := range a {
			if name == "docProps/core.xml" {
				continue // carries a save-time Modified timestamp
			}
			if !bytes.Equal(want, b[name]) {
				t.Errorf("save %d differs from save 1 in %s (C439)", i+2, name)
			}
		}
	}
}
