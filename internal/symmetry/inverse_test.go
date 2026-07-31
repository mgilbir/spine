package symmetry_test

import (
	"archive/zip"
	"bytes"
	"io"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/mgilbir/spine/docx"
	"github.com/mgilbir/spine/pptx"
	"github.com/mgilbir/spine/xlsx"
)

// Adding something and then removing it leaves the package as it was.
//
// This is the shape of defect the audits keep finding — a Clear that zeroes
// instead of removing, a delete that drops the model entry and leaves the part,
// the relationship or the content-type override behind — and it is invisible to
// every other kind of test. The getter agrees, because the model entry is gone.
// The round trip agrees, because it faithfully reproduces the leftovers. Only
// comparing against a package that never had the thing shows it.
//
// The comparison is deliberately not byte equality. An id counter that does not
// rewind is not a defect: ids need not be reused, and demanding it would pin an
// implementation detail. What is a defect is a part nobody references, a
// relationship pointing at nothing, a content-type override for a part that is
// not there — leftovers a consumer trips over. So the assertions are on the set
// of parts, the relationship graph, and the content types.

// packageShape is what an add-then-remove has to restore.
type packageShape struct {
	parts        []string
	contentTypes []string
	// rels maps a relationship source part to the targets it names, so a
	// relationship that outlives its target is visible as a target with no part.
	rels map[string][]string
}

func shapeOf(t *testing.T, pkg []byte) packageShape {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(pkg), int64(len(pkg)))
	if err != nil {
		t.Fatalf("saved package is not a zip: %v", err)
	}
	shape := packageShape{rels: map[string][]string{}}
	target := regexp.MustCompile(`Target="([^"]+)"`)
	override := regexp.MustCompile(`PartName="([^"]+)"`)

	for _, f := range zr.File {
		shape.parts = append(shape.parts, f.Name)
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("opening %s: %v", f.Name, err)
		}
		data, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("reading %s: %v", f.Name, err)
		}
		switch {
		case f.Name == "[Content_Types].xml":
			for _, m := range override.FindAllStringSubmatch(string(data), -1) {
				shape.contentTypes = append(shape.contentTypes, m[1])
			}
		case strings.HasSuffix(f.Name, ".rels"):
			var targets []string
			for _, m := range target.FindAllStringSubmatch(string(data), -1) {
				targets = append(targets, m[1])
			}
			sort.Strings(targets)
			shape.rels[f.Name] = targets
		}
	}
	sort.Strings(shape.parts)
	sort.Strings(shape.contentTypes)
	return shape
}

// diff describes how a shape departs from the baseline, or "" when it does not.
func (want packageShape) diff(got packageShape) string {
	var problems []string
	if d := listDiff(want.parts, got.parts); d != "" {
		problems = append(problems, "parts: "+d)
	}
	if d := listDiff(want.contentTypes, got.contentTypes); d != "" {
		problems = append(problems, "content-type overrides: "+d)
	}
	for name, wantTargets := range want.rels {
		gotTargets, ok := got.rels[name]
		if !ok {
			problems = append(problems, "relationship part missing: "+name)
			continue
		}
		if d := listDiff(wantTargets, gotTargets); d != "" {
			problems = append(problems, name+": "+d)
		}
	}
	for name := range got.rels {
		if _, ok := want.rels[name]; !ok {
			problems = append(problems, "extra relationship part: "+name)
		}
	}
	return strings.Join(problems, "; ")
}

func listDiff(want, got []string) string {
	inWant := map[string]bool{}
	for _, s := range want {
		inWant[s] = true
	}
	inGot := map[string]bool{}
	for _, s := range got {
		inGot[s] = true
	}
	var extra, missing []string
	for _, s := range got {
		if !inWant[s] {
			extra = append(extra, s)
		}
	}
	for _, s := range want {
		if !inGot[s] {
			missing = append(missing, s)
		}
	}
	var parts []string
	if len(extra) > 0 {
		parts = append(parts, "left behind "+strings.Join(extra, ", "))
	}
	if len(missing) > 0 {
		parts = append(parts, "removed too much: "+strings.Join(missing, ", "))
	}
	return strings.Join(parts, "; ")
}

// inverseCase is a document built two ways: without the operation, and with the
// operation followed by its inverse.
type inverseCase struct {
	baseline func(t *testing.T) []byte
	roundTip func(t *testing.T) []byte
}

func TestAddThenRemoveLeavesNoTrace(t *testing.T) {
	cases := map[string]inverseCase{
		"xlsx AddSheet then RemoveSheet": {
			baseline: func(t *testing.T) []byte {
				w := xlsx.Create()
				mustSheet(t, w, "Keep")
				return saveWorkbook(t, w)
			},
			roundTip: func(t *testing.T) []byte {
				w := xlsx.Create()
				mustSheet(t, w, "Keep")
				s := mustSheet(t, w, "Temp")
				if err := s.SetCellValue("A1", "scratch"); err != nil {
					t.Fatal(err)
				}
				idx := len(w.Sheets()) - 1
				if err := w.RemoveSheet(idx); err != nil {
					t.Fatalf("RemoveSheet: %v", err)
				}
				return saveWorkbook(t, w)
			},
		},
		"pptx AddSlide then RemoveSlide": {
			baseline: func(t *testing.T) []byte {
				p := pptx.Create()
				addTextSlide(t, p, "keep")
				return savePresentation(t, p)
			},
			roundTip: func(t *testing.T) []byte {
				p := pptx.Create()
				addTextSlide(t, p, "keep")
				addTextSlide(t, p, "temporary")
				if err := p.RemoveSlide(len(p.Slides()) - 1); err != nil {
					t.Fatalf("RemoveSlide: %v", err)
				}
				return savePresentation(t, p)
			},
		},
		// The same pair on a deck read from a file. It is a separate case
		// because it exercises a different writer: an opened package carries
		// preserved parts that the save copies through, so a removal that
		// forgets one leaves it in the output — where a created deck, whose
		// every part is regenerated from the model, cannot.
		"pptx AddSlide then RemoveSlide, on an opened deck": {
			baseline: func(t *testing.T) []byte {
				p := reopenPresentation(t, func() []byte {
					q := pptx.Create()
					addTextSlide(t, q, "keep")
					return savePresentation(t, q)
				}())
				return savePresentation(t, p)
			},
			roundTip: func(t *testing.T) []byte {
				p := reopenPresentation(t, func() []byte {
					q := pptx.Create()
					addTextSlide(t, q, "keep")
					return savePresentation(t, q)
				}())
				s := addTextSlide(t, p, "temporary")
				s.SetNotes("notes that go with the slide")
				if _, err := s.AddPictureFromBytes(testImage(t), "image/png"); err != nil {
					t.Fatalf("AddPictureFromBytes: %v", err)
				}
				if err := p.RemoveSlide(len(p.Slides()) - 1); err != nil {
					t.Fatalf("RemoveSlide: %v", err)
				}
				return savePresentation(t, p)
			},
		},
		"pptx AddSlide with notes then RemoveSlide": {
			baseline: func(t *testing.T) []byte {
				p := pptx.Create()
				addTextSlide(t, p, "keep")
				return savePresentation(t, p)
			},
			roundTip: func(t *testing.T) []byte {
				p := pptx.Create()
				addTextSlide(t, p, "keep")
				s := addTextSlide(t, p, "temporary")
				s.SetNotes("notes that go with the slide")
				if err := p.RemoveSlide(len(p.Slides()) - 1); err != nil {
					t.Fatalf("RemoveSlide: %v", err)
				}
				return savePresentation(t, p)
			},
		},
		"pptx AddSlide with a picture then RemoveSlide": {
			baseline: func(t *testing.T) []byte {
				p := pptx.Create()
				addTextSlide(t, p, "keep")
				return savePresentation(t, p)
			},
			roundTip: func(t *testing.T) []byte {
				p := pptx.Create()
				addTextSlide(t, p, "keep")
				s := addTextSlide(t, p, "temporary")
				// The media part is the leak most likely to outlive its only
				// reference: it is written from a separate collection, and
				// nothing but the removed slide points at it.
				if _, err := s.AddPictureFromBytes(testImage(t), "image/png"); err != nil {
					t.Fatalf("AddPictureFromBytes: %v", err)
				}
				if err := p.RemoveSlide(len(p.Slides()) - 1); err != nil {
					t.Fatalf("RemoveSlide: %v", err)
				}
				return savePresentation(t, p)
			},
		},
		"docx AddCustomProperty then RemoveCustomProperty": {
			baseline: func(t *testing.T) []byte {
				d := docx.Create()
				d.AddParagraph().SetText("body")
				return saveDocument(t, d)
			},
			roundTip: func(t *testing.T) []byte {
				d := docx.Create()
				d.AddParagraph().SetText("body")
				if err := d.SetCustomProperty("Scratch", "value"); err != nil {
					t.Fatalf("SetCustomProperty: %v", err)
				}
				if !d.RemoveCustomProperty("Scratch") {
					t.Fatal("RemoveCustomProperty reported nothing to remove")
				}
				return saveDocument(t, d)
			},
		},
	}

	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			after := c.roundTip(t)
			want := shapeOf(t, c.baseline(t))
			got := shapeOf(t, after)
			if d := want.diff(got); d != "" {
				t.Errorf("the operation and its inverse did not cancel: %s", d)
			}
			// Independent of the baseline: whatever the package ends up
			// containing has to be self-consistent. A relationship naming a
			// part that is not there, or a content-type override for one, is
			// what a half-completed removal leaves and what makes a consumer
			// report the file as damaged.
			for _, problem := range danglingReferences(t, after) {
				t.Errorf("after the operation and its inverse: %s", problem)
			}
		})
	}
}

func mustSheet(t *testing.T, w *xlsx.Workbook, name string) *xlsx.Sheet {
	t.Helper()
	s, err := w.AddSheet(name)
	if err != nil {
		t.Fatalf("AddSheet(%s): %v", name, err)
	}
	return s
}

func addTextSlide(t *testing.T, p *pptx.Presentation, text string) *pptx.Slide {
	t.Helper()
	s := p.AddSlide()
	tb := pptx.NewTextBox()
	tb.SetText(text)
	if err := s.AddShape(tb); err != nil {
		t.Fatalf("AddShape: %v", err)
	}
	return s
}

func saveWorkbook(t *testing.T, w *xlsx.Workbook) []byte {
	t.Helper()
	out, err := w.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	_ = w.Close()
	return out
}

func savePresentation(t *testing.T, p *pptx.Presentation) []byte {
	t.Helper()
	out, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	return out
}

func saveDocument(t *testing.T, d *docx.Document) []byte {
	t.Helper()
	out, err := d.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	return out
}

// danglingReferences reports relationship targets and content-type overrides
// that name a part the package does not contain.
func danglingReferences(t *testing.T, pkg []byte) []string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(pkg), int64(len(pkg)))
	if err != nil {
		t.Fatalf("saved package is not a zip: %v", err)
	}
	present := map[string]bool{}
	for _, f := range zr.File {
		present["/"+f.Name] = true
	}

	var problems []string
	target := regexp.MustCompile(`Target="([^"]+)"`)
	mode := regexp.MustCompile(`TargetMode="External"`)
	override := regexp.MustCompile(`PartName="([^"]+)"`)

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
		switch {
		case f.Name == "[Content_Types].xml":
			for _, m := range override.FindAllStringSubmatch(string(data), -1) {
				if !present[m[1]] {
					problems = append(problems, "[Content_Types].xml overrides "+m[1]+", which is not in the package")
				}
			}
		case strings.HasSuffix(f.Name, ".rels"):
			// An external target names something outside the package by
			// design; only internal ones have to resolve.
			for _, rel := range strings.Split(string(data), "<Relationship") {
				if mode.MatchString(rel) {
					continue
				}
				m := target.FindStringSubmatch(rel)
				if m == nil {
					continue
				}
				owner := "/" + strings.TrimSuffix(strings.Replace(f.Name, "_rels/", "", 1), ".rels")
				resolved := opcResolve(owner, m[1])
				if !present[resolved] {
					problems = append(problems, f.Name+" points at "+m[1]+" ("+resolved+"), which is not in the package")
				}
			}
		}
	}
	sort.Strings(problems)
	return problems
}

// opcResolve resolves a relationship target against the part that owns it,
// which is enough for the shapes these packages use.
func opcResolve(owner, target string) string {
	if strings.HasPrefix(target, "/") {
		return target
	}
	dir := owner[:strings.LastIndexByte(owner, '/')+1]
	for strings.HasPrefix(target, "../") {
		target = target[3:]
		trimmed := strings.TrimSuffix(dir, "/")
		dir = trimmed[:strings.LastIndexByte(trimmed, '/')+1]
	}
	return dir + target
}

func reopenPresentation(t *testing.T, pkg []byte) *pptx.Presentation {
	t.Helper()
	p, err := pptx.OpenReader(bytes.NewReader(pkg), int64(len(pkg)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	return p
}
