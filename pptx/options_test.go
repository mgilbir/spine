package pptx

import (
	"strings"
	"testing"

	"github.com/mgilbir/spine/common/dml"
)

func TestSlideSize_Dimensions(t *testing.T) {
	tests := []struct {
		size       SlideSize
		wantWidth  dml.EMU
		wantHeight dml.EMU
	}{
		{SlideSizeStandard, 9144000, 6858000},
		{SlideSizeWidescreen, 12192000, 6858000},
	}

	for _, tt := range tests {
		t.Run(tt.size.String(), func(t *testing.T) {
			w, h := tt.size.Dimensions()
			if w != tt.wantWidth {
				t.Errorf("Width = %d, want %d", w, tt.wantWidth)
			}
			if h != tt.wantHeight {
				t.Errorf("Height = %d, want %d", h, tt.wantHeight)
			}
		})
	}
}

func TestSlideSize_String(t *testing.T) {
	tests := []struct {
		size SlideSize
		want string
	}{
		{SlideSizeStandard, "Standard (4:3)"},
		{SlideSizeWidescreen, "Widescreen (16:9)"},
		{SlideSizeA4Portrait, "A4 Portrait"},
		{SlideSizeA4Landscape, "A4 Landscape"},
		{SlideSizeLetterPortrait, "Letter Portrait"},
		{SlideSizeLetterLandscape, "Letter Landscape"},
		{SlideSizeCustom, "Custom"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.size.String()
			if got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()

	if opts.SlideSize != SlideSizeStandard {
		t.Errorf("SlideSize = %v, want SlideSizeStandard", opts.SlideSize)
	}
}

func TestDefaultCreateOptions(t *testing.T) {
	opts := DefaultCreateOptions()

	if !opts.IncludeDefaultLayouts {
		t.Error("IncludeDefaultLayouts should be true")
	}
}

// C83: CreateWithOptions must honor the requested slide size instead of
// hardcoding 4:3.
func TestCreateWithOptions_Widescreen(t *testing.T) {
	opts := DefaultCreateOptions()
	opts.SlideSize = SlideSizeWidescreen
	p := CreateWithOptions(opts)

	if got := p.SlideWidth(); got != 12192000 {
		t.Errorf("SlideWidth = %d, want 12192000", got)
	}
	if got := p.SlideHeight(); got != 6858000 {
		t.Errorf("SlideHeight = %d, want 6858000", got)
	}

	slide := p.AddSlide()
	slide.AddTextBox().SetText("wide")
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	presXML := string(zipPart(t, data, "ppt/presentation.xml"))
	if want := `<p:sldSz cx="12192000" cy="6858000" type="screen16x9"/>`; !strings.Contains(presXML, want) {
		t.Errorf("presentation.xml sldSz = missing %q:\n%s", want, presXML)
	}
}

// SlideSizeCustom with explicit Width/Height produces a deck of those exact
// dimensions, rather than silently defaulting to 4:3.
func TestCreateWithOptions_Custom(t *testing.T) {
	opts := DefaultCreateOptions()
	opts.SlideSize = SlideSizeCustom
	opts.Width = dml.Inches(12)
	opts.Height = dml.Inches(9)
	p := CreateWithOptions(opts)

	if got := p.SlideWidth(); got != int64(dml.Inches(12)) {
		t.Errorf("SlideWidth = %d, want %d", got, int64(dml.Inches(12)))
	}
	if got := p.SlideHeight(); got != int64(dml.Inches(9)) {
		t.Errorf("SlideHeight = %d, want %d", got, int64(dml.Inches(9)))
	}
}

// C83: the default Create keeps its 4:3 size.
func TestCreate_DefaultSlideSizeUnchanged(t *testing.T) {
	p := Create()
	if got := p.SlideWidth(); got != 9144000 {
		t.Errorf("SlideWidth = %d, want 9144000", got)
	}
	if got := p.SlideHeight(); got != 6858000 {
		t.Errorf("SlideHeight = %d, want 6858000", got)
	}
}

func TestSlideSizeA4_Dimensions(t *testing.T) {
	// A4 is 210mm x 297mm
	w, h := SlideSizeA4Portrait.Dimensions()

	wMM := w.ToMillimeters()
	hMM := h.ToMillimeters()

	if wMM < 209 || wMM > 211 {
		t.Errorf("A4 Portrait width = %.1fmm, want ~210mm", wMM)
	}
	if hMM < 296 || hMM > 298 {
		t.Errorf("A4 Portrait height = %.1fmm, want ~297mm", hMM)
	}
}

func TestSlideSizeLetter_Dimensions(t *testing.T) {
	// Letter is 8.5" x 11"
	w, h := SlideSizeLetterPortrait.Dimensions()

	wIn := w.ToInches()
	hIn := h.ToInches()

	if wIn < 8.4 || wIn > 8.6 {
		t.Errorf("Letter Portrait width = %.1fin, want 8.5in", wIn)
	}
	if hIn < 10.9 || hIn > 11.1 {
		t.Errorf("Letter Portrait height = %.1fin, want 11in", hIn)
	}
}
