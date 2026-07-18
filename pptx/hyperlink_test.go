package pptx

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"strings"
	"testing"
)

// unzipAll returns every part in a zip archive keyed by name.
func unzipAll(t *testing.T, data []byte) map[string][]byte {
	t.Helper()
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	out := make(map[string][]byte, len(r.File))
	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatal(err)
		}
		out[f.Name] = b
	}
	return out
}

// saveReopen saves p to bytes and reopens it, failing the test on error.
func saveReopen(t *testing.T, p *Presentation) *Presentation {
	t.Helper()
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	rp, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	return rp
}

// firstRunHyperlink returns the hyperlink of the first run of the first text box
// on the slide, or nil.
func firstRunHyperlink(s *Slide) *Hyperlink {
	for _, shape := range s.Shapes() {
		tb, ok := shape.(*TextBox)
		if !ok {
			continue
		}
		for _, p := range tb.TextFrame().Paragraphs() {
			for _, r := range p.Runs() {
				if h := r.Hyperlink(); h != nil {
					return h
				}
			}
		}
	}
	return nil
}

// TestRunHyperlink_CreateRoundTrip sets an external hyperlink on a run created
// from scratch, then confirms it round-trips through SaveBytes/OpenReader — the
// Create path, where a write that only persisted on the opened path would fail.
func TestRunHyperlink_CreateRoundTrip(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	run := s.AddTextBox().TextFrame().AddParagraph().AddRun()
	run.SetText("link")
	run.SetHyperlink("https://example.com/x").SetTooltip("tip")

	rp := saveReopen(t, p)
	rs, _ := rp.Slide(0)
	h := firstRunHyperlink(rs)
	if h == nil {
		t.Fatal("run hyperlink not read back on the create path")
	}
	if h.URL() != "https://example.com/x" {
		t.Errorf("URL = %q, want https://example.com/x", h.URL())
	}
	if h.Tooltip() != "tip" {
		t.Errorf("Tooltip = %q, want tip", h.Tooltip())
	}
	if h.Anchor() != "" {
		t.Errorf("Anchor = %q, want empty for external link", h.Anchor())
	}
}

// TestShapeHyperlink_CreateRoundTrip sets a shape-level hyperlink on an auto
// shape and confirms it round-trips on the create path.
func TestShapeHyperlink_CreateRoundTrip(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	as := NewAutoShape(PresetRect)
	as.SetSize(914400, 914400)
	if err := s.AddShape(as); err != nil {
		t.Fatal(err)
	}
	as.SetHyperlink("https://example.com/shape").SetTooltip("shape tip")

	rp := saveReopen(t, p)
	rs, _ := rp.Slide(0)
	var found *Hyperlink
	for _, shape := range rs.Shapes() {
		if h := shape.(interface{ Hyperlink() *Hyperlink }).Hyperlink(); h != nil {
			found = h
		}
	}
	if found == nil {
		t.Fatal("shape hyperlink not read back on the create path")
	}
	if found.URL() != "https://example.com/shape" || found.Tooltip() != "shape tip" {
		t.Errorf("got url=%q tip=%q", found.URL(), found.Tooltip())
	}
}

// TestSlideJumpHyperlink_CreateRoundTrip sets an internal slide-jump on a run and
// confirms the destination slide number round-trips.
func TestSlideJumpHyperlink_CreateRoundTrip(t *testing.T) {
	p := Create()
	s0 := p.AddSlide()
	p.AddSlide() // jump target, index 1
	run := s0.AddTextBox().TextFrame().AddParagraph().AddRun()
	run.SetText("jump")
	run.SetHyperlinkToSlide(1)

	rp := saveReopen(t, p)
	rs, _ := rp.Slide(0)
	h := firstRunHyperlink(rs)
	if h == nil {
		t.Fatal("slide-jump hyperlink not read back")
	}
	if h.URL() != "" {
		t.Errorf("URL = %q, want empty for internal jump", h.URL())
	}
	if h.Anchor() != "2" {
		t.Errorf("Anchor = %q, want \"2\" (1-based destination slide)", h.Anchor())
	}
}

// TestActionHyperlink_CreateRoundTrip sets a ppaction:// verb on a run (no
// relationship) and confirms it round-trips as the anchor.
func TestActionHyperlink_CreateRoundTrip(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	run := s.AddTextBox().TextFrame().AddParagraph().AddRun()
	run.SetText("next")
	run.SetActionHyperlink(ActionNextSlide)

	rp := saveReopen(t, p)
	rs, _ := rp.Slide(0)
	h := firstRunHyperlink(rs)
	if h == nil {
		t.Fatal("action hyperlink not read back")
	}
	if h.Anchor() != ActionNextSlide {
		t.Errorf("Anchor = %q, want %q", h.Anchor(), ActionNextSlide)
	}
}

// TestHyperlinkAllocatesExternalRel confirms writing a run hyperlink allocates an
// External relationship in the slide's rels part.
func TestHyperlinkAllocatesExternalRel(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	run := s.AddTextBox().TextFrame().AddParagraph().AddRun()
	run.SetText("link")
	run.SetHyperlink("https://example.com/rel")

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	rels := string(zipPart(t, data, "ppt/slides/_rels/slide1.xml.rels"))
	if !strings.Contains(rels, "https://example.com/rel") {
		t.Errorf("slide rels missing external hyperlink target:\n%s", rels)
	}
	if !strings.Contains(rels, `TargetMode="External"`) {
		t.Errorf("hyperlink relationship not marked External:\n%s", rels)
	}
	if !strings.Contains(rels, "relationships/hyperlink") {
		t.Errorf("hyperlink relationship type missing:\n%s", rels)
	}
}

// TestSetHyperlinkOnOpenedSlide_Surgical opens a fixture, sets a hyperlink on one
// run, and confirms the edit patches surgically: the modified slide gains the
// hyperlink while every other slide's XML stays byte-identical (no clobbering).
func TestSetHyperlinkOnOpenedSlide_Surgical(t *testing.T) {
	const path = "testdata/external/big_data.pptx"
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skip("fixture not present:", path)
	}
	orig, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	origParts := unzipAll(t, orig)

	p, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	// Find a slide with a text run and set a hyperlink on it.
	var target *Slide
	var targetRun *Run
	for _, s := range p.Slides() {
		for _, shape := range s.Shapes() {
			tf := textFrameOf(shape)
			if tf == nil {
				continue
			}
			for _, para := range tf.Paragraphs() {
				for _, r := range para.Runs() {
					if r.Text() != "" && targetRun == nil {
						target, targetRun = s, r
					}
				}
			}
		}
		if targetRun != nil {
			break
		}
	}
	if targetRun == nil {
		t.Skip("no text run found in fixture")
	}
	targetRun.SetHyperlink("https://example.com/injected")

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	newParts := unzipAll(t, data)

	// Every slide part other than the edited one must be byte-identical.
	for name, ob := range origParts {
		if !strings.HasPrefix(name, "ppt/slides/slide") || strings.Contains(name, "_rels") {
			continue
		}
		if name == "ppt/slides/"+baseName(target.partName) {
			continue // the edited slide is expected to change
		}
		nb, ok := newParts[name]
		if !ok {
			t.Errorf("slide part vanished: %s", name)
			continue
		}
		if !bytes.Equal(ob, nb) {
			t.Errorf("unedited slide part changed (clobbered): %s", name)
		}
	}

	// The injected hyperlink must be readable back.
	rp, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, h := range rp.Hyperlinks() {
		if h.URL() == "https://example.com/injected" {
			found = true
		}
	}
	if !found {
		t.Error("injected hyperlink not read back after surgical edit")
	}
}

// TestReadExternalHyperlinks reads hyperlinks from a real fixture and confirms
// external URLs resolve.
func TestReadExternalHyperlinks(t *testing.T) {
	const path = "testdata/external/big_data.pptx"
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skip("fixture not present:", path)
	}
	p, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	hls := p.Hyperlinks()
	if len(hls) == 0 {
		t.Fatal("no hyperlinks read from a fixture known to contain them")
	}
	var external int
	for _, h := range hls {
		if strings.HasPrefix(h.URL(), "http") {
			external++
		}
	}
	if external == 0 {
		t.Error("no external URL hyperlinks resolved")
	}
}

func baseName(part string) string {
	if i := strings.LastIndexByte(part, '/'); i >= 0 {
		return part[i+1:]
	}
	return part
}
