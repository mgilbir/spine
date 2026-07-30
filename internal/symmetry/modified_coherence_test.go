package symmetry_test

import (
	"bytes"
	"testing"
	"testing/synctest"
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
// exactly one of the cases below: docx declined to count a custom-property edit,
// citing pptx as precedent, while pptx and xlsx both counted it. The divergence
// was invisible to every per-format test, because each format's suite only knows
// its own answer. A property that is supposed to hold across formats has to be
// asserted across formats.
//
// Each case drives all three packages through the same scenario and requires the
// same answer. What that answer *is* matters less than that it is one answer;
// where the formats legitimately differ, that belongs in the capability table in
// crossformat_test.go, not here.
//
// # Why these run under testing/synctest
//
// dcterms:modified is a wall-clock value serialized at RFC3339 (one-second)
// resolution, so a stamp written in the same second as the baseline is
// indistinguishable from no stamp at all. Proving a save did *not* stamp
// therefore means letting a second elapse first — which used to be a real 1.1s
// sleep per format per case.
//
// Inside a synctest bubble the clock is fake: it starts at 2000-01-01T00:00:00Z
// and jumps instantly when every goroutine in the bubble is durably blocked, so
// the sleeps cost nothing. The library reaches the same fake clock, because a
// bubble captures every goroutine inside it rather than a package.
//
// The bigger win is the assertion, not the speed. A deterministic clock means
// the exact instant a save will stamp is knowable, so these tests assert
// Modified *equals* it rather than merely that it moved forward. That also
// sidesteps a trap the old shape would have walked into here: the bubble epoch
// is the year 2000, and real fixtures carry newer stored timestamps
// (pptx/testdata/test.pptx is 2012, docx/testdata/chart.docx is 2022), so
// "Modified.After(before)" would fail for a correctly stamping save. Equality
// against a known instant is both stronger and immune to that.
//
// Sleeps must be whole seconds: a sub-second instant would not survive the
// RFC3339 round trip. The bubble clock only advances by these sleeps, since the
// library sets no timers of its own.

// stampCase is one scenario, expressed once per format. Each function seeds a
// document of its format, performs the scenario, saves, and reports whether the
// save stamped.
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
			// The bubble goes inside t.Run, not around the loop: a subtest runs
			// on its own goroutine, and a bubble owns the goroutines within it.
			synctest.Test(t, func(t *testing.T) {
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
		})
	}
}

// classify turns the observed dcterms:modified into the yes/no answer the table
// compares, and fails on anything that is neither.
//
// The third outcome is the reason this is a function rather than a comparison
// inlined at each call site: a value that is neither the untouched baseline nor
// the instant of the save means the stamp fired at the wrong time — a real
// defect that a plain "did it move forward" check would silently report as a
// successful stamp.
func classify(t *testing.T, format string, got, untouched, atSave time.Time) bool {
	t.Helper()
	switch {
	case got.Equal(atSave):
		return true
	case got.Equal(untouched):
		return false
	default:
		t.Fatalf("%s: Modified is %v, which is neither the untouched value (%v) nor the save instant (%v)",
			format, got.UTC(), untouched.UTC(), atSave.UTC())
		return false
	}
}

// second is the gap opened between the baseline and the save. It is a whole
// second so the instant survives the RFC3339 round trip, and it is free: under
// the bubble's fake clock the sleep returns immediately.
const second = time.Second

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
	untouched := d.Properties.Modified

	time.Sleep(second)
	edit(d)
	atSave := time.Now()
	out, err := d.SaveBytes()
	if err != nil {
		t.Fatalf("docx save: %v", err)
	}
	got, err := docx.OpenReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatalf("docx reopen: %v", err)
	}
	return classify(t, "docx", got.Properties.Modified, untouched, atSave)
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
	untouched := w.Properties.Modified

	time.Sleep(second)
	edit(w)
	atSave := time.Now()
	out, err := w.SaveBytes()
	if err != nil {
		t.Fatalf("xlsx save: %v", err)
	}
	got, err := xlsx.OpenReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatalf("xlsx reopen: %v", err)
	}
	return classify(t, "xlsx", got.Properties.Modified, untouched, atSave)
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
	untouched := p.Properties.Modified

	time.Sleep(second)
	edit(p)
	atSave := time.Now()
	out, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("pptx save: %v", err)
	}
	got, err := pptx.OpenReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatalf("pptx reopen: %v", err)
	}
	return classify(t, "pptx", got.Properties.Modified, untouched, atSave)
}

// TestUnchangedSaveIsByteIdenticalAcrossFormats pins the other half of the rule
// — the half that makes "no option to turn it off" defensible. If an untouched
// save were not reproducible, callers would need a switch to choose between an
// accurate timestamp and deterministic output.
//
// The elapsed second between the two saves is the whole point: it is exactly
// what a per-save stamp needs in order to show itself.
func TestUnchangedSaveIsByteIdenticalAcrossFormats(t *testing.T) {
	t.Run("docx", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
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
			assertRepeatSaveIsIdentical(t, d.SaveBytes)
		})
	})

	t.Run("xlsx", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
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
			assertRepeatSaveIsIdentical(t, w.SaveBytes)
		})
	})

	t.Run("pptx", func(t *testing.T) {
		synctest.Test(t, func(t *testing.T) {
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
			assertRepeatSaveIsIdentical(t, p.SaveBytes)
		})
	})
}

// assertRepeatSaveIsIdentical saves twice with a second in between and requires
// the same bytes both times.
func assertRepeatSaveIsIdentical(t *testing.T, save func() ([]byte, error)) {
	t.Helper()
	first, err := save()
	if err != nil {
		t.Fatalf("first save: %v", err)
	}
	time.Sleep(second)
	next, err := save()
	if err != nil {
		t.Fatalf("second save: %v", err)
	}
	if !bytes.Equal(first, next) {
		t.Errorf("two untouched saves differ (%d vs %d bytes) across a second boundary",
			len(first), len(next))
	}
}
