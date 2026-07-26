package pptx

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"

	"github.com/mgilbir/spine/chart"
	"github.com/mgilbir/spine/common/validate"
	"github.com/mgilbir/spine/opc"
)

// TestMergeSlideJumpHyperlink verifies that merging slides carrying an internal
// slide-jump hyperlink does not leave a dangling relationship id: the jump is
// remapped to the imported target slide, and the merged deck validates clean of
// the dangling-r:id class (C268).
func TestMergeSlideJumpHyperlink(t *testing.T) {
	src := Create()
	s0 := src.AddSlide()
	src.AddSlide() // jump target, source index 1
	run := s0.AddTextBox().TextFrame().AddParagraph().AddRun()
	run.SetText("go to slide 2")
	run.SetHyperlinkToSlide(1)

	dst := Create()
	dst.AddSlide().AddTextBox().TextFrame().SetText("Dst")
	if err := dst.AppendSlidesFrom(src); err != nil {
		t.Fatalf("AppendSlidesFrom: %v", err)
	}

	data, err := dst.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	rp, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}

	// dst slides after the merge: [Dst(0), imported-s0(1), imported-s1(2)]. The
	// jump on imported-s0 must resolve to imported-s1 (dst index 2 -> "3").
	rs, err := rp.Slide(1)
	if err != nil {
		t.Fatalf("Slide(1): %v", err)
	}
	h := firstRunHyperlink(rs)
	if h == nil {
		t.Fatal("merged slide-jump hyperlink not read back")
	}
	if h.URL() != "" {
		t.Errorf("URL = %q, want empty for internal jump", h.URL())
	}
	if h.Anchor() != "3" {
		t.Errorf("Anchor = %q, want \"3\" (imported target slide, 1-based)", h.Anchor())
	}

	// Force every slide to materialize so validateHyperlinks actually inspects
	// the merged jump (it skips slides never accessed), then assert the deck is
	// clean of the dangling-r:id class.
	for _, s := range rp.Slides() {
		_ = s.Hyperlinks()
	}
	if r := rp.Validate(); hasCode(r, codeHyperlinkNoRel, validate.SeverityWarning) {
		t.Fatalf("merged deck has a dangling slide-jump r:id: %v", r)
	}
}

func assertZipHasPrefix(t *testing.T, data []byte, prefix string) {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("zip.NewReader: %v", err)
	}
	for _, f := range zr.File {
		if strings.HasPrefix(f.Name, prefix) {
			return
		}
	}
	t.Fatalf("no zip entry with prefix %q", prefix)
}

// buildDeck returns a small created deck whose slides carry a text box, a
// picture, and a chart, for the merge tests.
func buildDeck(t *testing.T, titles []string) *Presentation {
	t.Helper()
	p := Create()
	for i, title := range titles {
		s := p.AddSlide()
		s.AddTextBox().TextFrame().SetText(title)
		if i == 0 {
			if _, err := s.AddPictureFromBytes(createMinimalPNG(), opc.ContentTypePNG); err != nil {
				t.Fatalf("AddPictureFromBytes: %v", err)
			}
		}
		if i == 1 {
			c := newCategoryChart(chart.NewColumn, title)
			if err := s.AddChart(c, 0, 0, 6096000, 4572000); err != nil {
				t.Fatalf("AddChart: %v", err)
			}
		}
	}
	return p
}

func mergeSlideTexts(t *testing.T, data []byte) []string {
	t.Helper()
	p, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	var texts []string
	for _, s := range p.Slides() {
		var sb bytes.Buffer
		for _, shape := range s.Shapes() {
			if tb, ok := shape.(*TextBox); ok {
				sb.WriteString(tb.TextFrame().Text())
			}
		}
		texts = append(texts, sb.String())
	}
	return texts
}

func TestAppendSlidesFrom(t *testing.T) {
	// dst carries only text, so the media/chart/embedding assertions below prove
	// the appended slides' parts were actually carried over.
	dst := Create()
	dst.AddSlide().AddTextBox().TextFrame().SetText("Dst-A")
	dst.AddSlide().AddTextBox().TextFrame().SetText("Dst-B")
	src := buildDeck(t, []string{"Src-A", "Src-B"})

	if err := dst.AppendSlidesFrom(src); err != nil {
		t.Fatalf("AppendSlidesFrom: %v", err)
	}
	if got := dst.SlideCount(); got != 4 {
		t.Fatalf("SlideCount = %d, want 4", got)
	}

	report := dst.Validate()
	if report.HasErrors() {
		t.Fatalf("Validate after append: %v", report)
	}

	data, err := dst.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	// Reopen and check content and a clean package.
	re, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	if got := re.SlideCount(); got != 4 {
		t.Fatalf("reopened SlideCount = %d, want 4", got)
	}
	if r := re.Validate(); r.HasErrors() {
		t.Fatalf("Validate reopened: %v", r)
	}

	texts := mergeSlideTexts(t, data)
	want := []string{"Dst-A", "Dst-B", "Src-A", "Src-B"}
	if len(texts) != len(want) {
		t.Fatalf("texts = %v, want %v", texts, want)
	}
	for i := range want {
		if texts[i] != want[i] {
			t.Errorf("slide %d text = %q, want %q", i, texts[i], want[i])
		}
	}

	// The appended chart's embedded workbook and the picture media must both be
	// present in the merged package (no dropped parts).
	assertZipHasPrefix(t, data, "ppt/media/")
	assertZipHasPrefix(t, data, "ppt/charts/")
	assertZipHasPrefix(t, data, "ppt/embeddings/")
}

func TestExtractSlides(t *testing.T) {
	src := buildDeck(t, []string{"One", "Two", "Three"})

	out, err := src.ExtractSlides([]int{2, 0})
	if err != nil {
		t.Fatalf("ExtractSlides: %v", err)
	}
	if got := out.SlideCount(); got != 2 {
		t.Fatalf("SlideCount = %d, want 2", got)
	}
	if r := out.Validate(); r.HasErrors() {
		t.Fatalf("Validate extracted: %v", r)
	}

	data, err := out.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	texts := mergeSlideTexts(t, data)
	want := []string{"Three", "One"}
	if len(texts) != len(want) {
		t.Fatalf("texts = %v, want %v", texts, want)
	}
	for i := range want {
		if texts[i] != want[i] {
			t.Errorf("extracted slide %d text = %q, want %q", i, texts[i], want[i])
		}
	}

	// The extracted "One" slide carried its picture media.
	assertZipHasPrefix(t, data, "ppt/media/")
}

func TestAppendSlidesCarriesNotes(t *testing.T) {
	dst := Create()
	dst.AddSlide().AddTextBox().TextFrame().SetText("Dst")

	src := Create()
	s := src.AddSlide()
	s.AddTextBox().TextFrame().SetText("Src")
	s.SetNotes("Speaker note carried across")

	if err := dst.AppendSlidesFrom(src); err != nil {
		t.Fatalf("AppendSlidesFrom: %v", err)
	}
	if r := dst.Validate(); r.HasErrors() {
		t.Fatalf("Validate after append: %v", r)
	}

	data, err := dst.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	assertZipHasPrefix(t, data, "ppt/notesSlides/")

	re, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	if r := re.Validate(); r.HasErrors() {
		t.Fatalf("Validate reopened: %v", r)
	}
	slides := re.Slides()
	if len(slides) != 2 {
		t.Fatalf("reopened SlideCount = %d, want 2", len(slides))
	}
	if got := slides[1].Notes(); got != "Speaker note carried across" {
		t.Errorf("carried notes = %q, want %q", got, "Speaker note carried across")
	}
}

func TestAppendSlidesFromOpenedImportsMasterTheme(t *testing.T) {
	// Build a source, save, and reopen so it carries a real theme part in
	// themeData (a created deck hardcodes its theme only at save). Appending its
	// slide into a fresh deck must import the source master, layout, and theme
	// with no dangling references.
	seed := buildDeck(t, []string{"Seed-A"})
	seedBytes, err := seed.SaveBytes()
	if err != nil {
		t.Fatalf("seed SaveBytes: %v", err)
	}
	src, err := OpenReader(bytes.NewReader(seedBytes), int64(len(seedBytes)))
	if err != nil {
		t.Fatalf("open source: %v", err)
	}

	dst := Create()
	dst.AddSlide().AddTextBox().TextFrame().SetText("Dst")
	beforeMasters := len(dst.slideMasters)

	if err := dst.AppendSlidesFrom(src); err != nil {
		t.Fatalf("AppendSlidesFrom: %v", err)
	}
	if len(dst.slideMasters) <= beforeMasters {
		t.Fatalf("expected an imported master, masters=%d (was %d)", len(dst.slideMasters), beforeMasters)
	}
	if r := dst.Validate(); r.HasErrors() {
		t.Fatalf("Validate after append: %v", r)
	}

	data, err := dst.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	re, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	if r := re.Validate(); r.HasErrors() {
		t.Fatalf("Validate reopened: %v", r)
	}
	// At least two masters and two theme parts (default + imported) present.
	assertZipEntryCountAtLeast(t, data, "ppt/slideMasters/slideMaster", 2)
	assertZipEntryCountAtLeast(t, data, "ppt/theme/theme", 2)
	if got := re.SlideCount(); got != 2 {
		t.Fatalf("reopened SlideCount = %d, want 2", got)
	}
}

func TestAppendOntoOpenedDestImportsMaster(t *testing.T) {
	// Opened source (real theme in themeData) appended onto an opened
	// destination (the round-trip save path, which needs the presentation->master
	// relationship registered for the imported master).
	seed := buildDeck(t, []string{"Seed-A"})
	sb, err := seed.SaveBytes()
	if err != nil {
		t.Fatalf("seed SaveBytes: %v", err)
	}
	src, err := OpenReader(bytes.NewReader(sb), int64(len(sb)))
	if err != nil {
		t.Fatalf("open source: %v", err)
	}

	// A widescreen destination: its master differs from the 4:3 source master, so
	// the source master is genuinely imported (not deduplicated).
	dseed := CreateWidescreen()
	dseed.AddSlide().AddTextBox().TextFrame().SetText("Dst")
	db, err := dseed.SaveBytes()
	if err != nil {
		t.Fatalf("dst seed SaveBytes: %v", err)
	}
	dst, err := OpenReader(bytes.NewReader(db), int64(len(db)))
	if err != nil {
		t.Fatalf("open dst: %v", err)
	}

	if err := dst.AppendSlidesFrom(src); err != nil {
		t.Fatalf("AppendSlidesFrom: %v", err)
	}
	if r := dst.Validate(); r.HasErrors() {
		t.Fatalf("Validate after append: %v", r)
	}
	out, err := dst.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	re, err := OpenReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	if r := re.Validate(); r.HasErrors() {
		t.Fatalf("Validate reopened: %v", r)
	}
	if got := re.SlideCount(); got != 2 {
		t.Fatalf("reopened SlideCount = %d, want 2", got)
	}
	assertZipEntryCountAtLeast(t, out, "ppt/slideMasters/slideMaster", 2)
}

func assertZipEntryCountAtLeast(t *testing.T, data []byte, prefix string, want int) {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("zip.NewReader: %v", err)
	}
	n := 0
	for _, f := range zr.File {
		if strings.HasPrefix(f.Name, prefix) && strings.HasSuffix(f.Name, ".xml") {
			n++
		}
	}
	if n < want {
		t.Errorf("found %d entries with prefix %q, want at least %d", n, prefix, want)
	}
}

func TestExtractSlidesInvalidIndex(t *testing.T) {
	src := buildDeck(t, []string{"Only"})
	if _, err := src.ExtractSlides([]int{5}); err != ErrSlideIndex {
		t.Fatalf("err = %v, want ErrSlideIndex", err)
	}
}

func TestAppendSlidesFromNil(t *testing.T) {
	dst := buildDeck(t, []string{"A"})
	if err := dst.AppendSlidesFrom(nil); err != ErrNilPresentation {
		t.Fatalf("err = %v, want ErrNilPresentation", err)
	}
}
