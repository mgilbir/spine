package symmetry_test

import (
	"bytes"
	"testing"
	"time"

	"github.com/mgilbir/spine/docx"
	"github.com/mgilbir/spine/pptx"
	"github.com/mgilbir/spine/xlsx"
)

// Cross-format guard for the "stamp dcterms:modified iff the content changed"
// rule, which all three formats gained in the same wave (pptx first, then docx
// and xlsx).
//
// It exists because the rule was implemented three times, once per format,
// against three different flag topologies — and the first cut diverged on
// exactly the case below: docx declined to count a custom-property edit, citing
// pptx as precedent, while pptx and xlsx both counted it. The divergence was
// invisible to every per-format test, because each format's suite only knows its
// own answer. A property that is supposed to hold across formats has to be
// asserted across formats.
//
// Each case drives all three packages through the same scenario and requires the
// same answer. What that answer *is* matters less than that it is one answer;
// where the formats legitimately differ, that belongs in the capability table in
// crossformat_test.go, not here.

// stampCase is one scenario, expressed once per format. Each function opens a
// freshly-saved document of its format, performs the scenario, saves, and
// reports whether Modified advanced.
type stampCase struct {
	name   string
	want   bool
	reason string
	docx   func(t *testing.T) bool
	xlsx   func(t *testing.T) bool
	pptx   func(t *testing.T) bool
}

func TestModifiedStampingAgreesAcrossFormats(t *testing.T) {
	cases := []stampCase{
		{
			name:   "untouched save",
			want:   false,
			reason: "an unchanged save must reproduce the package byte-for-byte",
			docx:   func(t *testing.T) bool { return docxStamps(t, func(*docx.Document) {}) },
			xlsx:   func(t *testing.T) bool { return xlsxStamps(t, func(*xlsx.Workbook) {}) },
			pptx:   func(t *testing.T) bool { return pptxStamps(t, func(*pptx.Presentation) {}) },
		},
		{
			name:   "custom property edit",
			want:   true,
			reason: "SetCustomProperty is a real mutator with a real setter to hook; the snapshot comparison that regenerates the part latches and cannot distinguish a second edit from the first",
			docx: func(t *testing.T) bool {
				return docxStamps(t, func(d *docx.Document) {
					if err := d.SetCustomProperty("Probe", "v"); err != nil {
						t.Fatalf("docx SetCustomProperty: %v", err)
					}
				})
			},
			xlsx: func(t *testing.T) bool {
				return xlsxStamps(t, func(w *xlsx.Workbook) {
					if err := w.SetCustomProperty("Probe", "v"); err != nil {
						t.Fatalf("xlsx SetCustomProperty: %v", err)
					}
				})
			},
			pptx: func(t *testing.T) bool {
				return pptxStamps(t, func(p *pptx.Presentation) {
					if err := p.SetCustomProperty("Probe", "v"); err != nil {
						t.Fatalf("pptx SetCustomProperty: %v", err)
					}
				})
			},
		},
		{
			name:   "core property assignment",
			want:   false,
			reason: "Properties is a plain struct field in all three; there is no setter to hook, and a caller authoring metadata is stating what it should say rather than asking for a write time on top",
			docx: func(t *testing.T) bool {
				return docxStamps(t, func(d *docx.Document) { d.Properties.Title = "probe" })
			},
			xlsx: func(t *testing.T) bool {
				return xlsxStamps(t, func(w *xlsx.Workbook) { w.Properties.Title = "probe" })
			},
			pptx: func(t *testing.T) bool {
				return pptxStamps(t, func(p *pptx.Presentation) { p.Properties.Title = "probe" })
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := map[string]bool{
				"docx": tc.docx(t),
				"xlsx": tc.xlsx(t),
				"pptx": tc.pptx(t),
			}
			for format, stamped := range got {
				if stamped != tc.want {
					t.Errorf("%s: %s stamped=%v, want %v for every format\n\treason: %s\n\tall three: %v",
						tc.name, format, stamped, tc.want, tc.reason, got)
				}
			}
		})
	}
}

// pastSecondBoundary sleeps far enough that a wall-clock stamp is observable.
// Without it a stamp written in the same second as the baseline is
// indistinguishable from no stamp at all, and every case would pass.
func pastSecondBoundary() { time.Sleep(1100 * time.Millisecond) }

func docxStamps(t *testing.T, edit func(*docx.Document)) bool {
	t.Helper()
	seed := docx.Create()
	seed.AddParagraph().AddRun().SetText("seed")
	b, err := seed.SaveBytes()
	if err != nil {
		t.Fatalf("docx seed save: %v", err)
	}
	d, err := docx.OpenReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		t.Fatalf("docx open: %v", err)
	}
	before := d.Properties.Modified
	pastSecondBoundary()
	edit(d)
	out, err := d.SaveBytes()
	if err != nil {
		t.Fatalf("docx save: %v", err)
	}
	got, err := docx.OpenReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatalf("docx reopen: %v", err)
	}
	return got.Properties.Modified.After(before)
}

func xlsxStamps(t *testing.T, edit func(*xlsx.Workbook)) bool {
	t.Helper()
	seed := xlsx.Create()
	sh, err := seed.AddSheet("Seed")
	if err != nil {
		t.Fatalf("xlsx seed sheet: %v", err)
	}
	c, err := sh.Cell("A1")
	if err != nil {
		t.Fatalf("xlsx seed cell: %v", err)
	}
	c.SetString("seed")
	b, err := seed.SaveBytes()
	if err != nil {
		t.Fatalf("xlsx seed save: %v", err)
	}
	w, err := xlsx.OpenReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		t.Fatalf("xlsx open: %v", err)
	}
	before := w.Properties.Modified
	pastSecondBoundary()
	edit(w)
	out, err := w.SaveBytes()
	if err != nil {
		t.Fatalf("xlsx save: %v", err)
	}
	got, err := xlsx.OpenReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatalf("xlsx reopen: %v", err)
	}
	return got.Properties.Modified.After(before)
}

func pptxStamps(t *testing.T, edit func(*pptx.Presentation)) bool {
	t.Helper()
	seed := pptx.Create()
	seed.AddSlide()
	b, err := seed.SaveBytes()
	if err != nil {
		t.Fatalf("pptx seed save: %v", err)
	}
	p, err := pptx.OpenReader(bytes.NewReader(b), int64(len(b)))
	if err != nil {
		t.Fatalf("pptx open: %v", err)
	}
	before := p.Properties.Modified
	pastSecondBoundary()
	edit(p)
	out, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("pptx save: %v", err)
	}
	got, err := pptx.OpenReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatalf("pptx reopen: %v", err)
	}
	return got.Properties.Modified.After(before)
}

// TestUnchangedSaveIsByteIdenticalAcrossFormats pins the other half of the rule
// — the half that makes "no option to turn it off" defensible. If an untouched
// save were not reproducible, callers would need a switch to choose between an
// accurate timestamp and deterministic output.
func TestUnchangedSaveIsByteIdenticalAcrossFormats(t *testing.T) {
	t.Run("docx", func(t *testing.T) {
		seed := docx.Create()
		seed.AddParagraph().AddRun().SetText("seed")
		b, err := seed.SaveBytes()
		if err != nil {
			t.Fatalf("seed save: %v", err)
		}
		d, err := docx.OpenReader(bytes.NewReader(b), int64(len(b)))
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		first, err := d.SaveBytes()
		if err != nil {
			t.Fatalf("first save: %v", err)
		}
		pastSecondBoundary()
		second, err := d.SaveBytes()
		if err != nil {
			t.Fatalf("second save: %v", err)
		}
		if !bytes.Equal(first, second) {
			t.Errorf("two untouched saves differ (%d vs %d bytes)", len(first), len(second))
		}
	})

	t.Run("xlsx", func(t *testing.T) {
		seed := xlsx.Create()
		sh, err := seed.AddSheet("Seed")
		if err != nil {
			t.Fatalf("seed sheet: %v", err)
		}
		c, err := sh.Cell("A1")
		if err != nil {
			t.Fatalf("seed cell: %v", err)
		}
		c.SetString("seed")
		b, err := seed.SaveBytes()
		if err != nil {
			t.Fatalf("seed save: %v", err)
		}
		w, err := xlsx.OpenReader(bytes.NewReader(b), int64(len(b)))
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		first, err := w.SaveBytes()
		if err != nil {
			t.Fatalf("first save: %v", err)
		}
		pastSecondBoundary()
		second, err := w.SaveBytes()
		if err != nil {
			t.Fatalf("second save: %v", err)
		}
		if !bytes.Equal(first, second) {
			t.Errorf("two untouched saves differ (%d vs %d bytes)", len(first), len(second))
		}
	})

	t.Run("pptx", func(t *testing.T) {
		seed := pptx.Create()
		seed.AddSlide()
		b, err := seed.SaveBytes()
		if err != nil {
			t.Fatalf("seed save: %v", err)
		}
		p, err := pptx.OpenReader(bytes.NewReader(b), int64(len(b)))
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		first, err := p.SaveBytes()
		if err != nil {
			t.Fatalf("first save: %v", err)
		}
		pastSecondBoundary()
		second, err := p.SaveBytes()
		if err != nil {
			t.Fatalf("second save: %v", err)
		}
		if !bytes.Equal(first, second) {
			t.Errorf("two untouched saves differ (%d vs %d bytes)", len(first), len(second))
		}
	})
}
