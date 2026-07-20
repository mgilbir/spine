package pptx

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"

	"github.com/mgilbir/spine/chart"
	"github.com/mgilbir/spine/opc"
)

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
