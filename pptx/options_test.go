package pptx

import (
	"testing"

	"github.com/mgilbir/spine/common/dml"
)

func TestSlideSize_Dimensions(t *testing.T) {
	tests := []struct {
		size        SlideSize
		wantWidth   dml.EMU
		wantHeight  dml.EMU
	}{
		{SlideSizeStandard, dml.Inches(10), dml.Inches(7.5)},
		{SlideSizeWidescreen, dml.Inches(13.333), dml.Inches(7.5)},
	}

	for _, tt := range tests {
		t.Run(tt.size.String(), func(t *testing.T) {
			w, h := tt.size.Dimensions()
			// Allow some tolerance for floating point
			if w < tt.wantWidth-1000 || w > tt.wantWidth+1000 {
				t.Errorf("Width = %d, want ~%d", w, tt.wantWidth)
			}
			if h < tt.wantHeight-1000 || h > tt.wantHeight+1000 {
				t.Errorf("Height = %d, want ~%d", h, tt.wantHeight)
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

	if opts.SlideSize != SlideSizeWidescreen {
		t.Errorf("SlideSize = %v, want SlideSizeWidescreen", opts.SlideSize)
	}
	if opts.DefaultFont != "Calibri" {
		t.Errorf("DefaultFont = %q, want %q", opts.DefaultFont, "Calibri")
	}
	if opts.DefaultFontSize != 18 {
		t.Errorf("DefaultFontSize = %f, want 18", opts.DefaultFontSize)
	}
	if opts.Locale != "en-US" {
		t.Errorf("Locale = %q, want %q", opts.Locale, "en-US")
	}
}

func TestDefaultCreateOptions(t *testing.T) {
	opts := DefaultCreateOptions()

	if !opts.IncludeDefaultLayouts {
		t.Error("IncludeDefaultLayouts should be true")
	}
	if !opts.IncludeDefaultTheme {
		t.Error("IncludeDefaultTheme should be true")
	}
}

func TestDefaultSaveOptions(t *testing.T) {
	opts := DefaultSaveOptions()

	if opts.Compression != 6 {
		t.Errorf("Compression = %d, want 6", opts.Compression)
	}
}

func TestExportFormat_String(t *testing.T) {
	tests := []struct {
		format ExportFormat
		want   string
	}{
		{ExportFormatPDF, "PDF"},
		{ExportFormatPNG, "PNG"},
		{ExportFormatJPEG, "JPEG"},
		{ExportFormatSVG, "SVG"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.format.String()
			if got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExportFormat_Extension(t *testing.T) {
	tests := []struct {
		format ExportFormat
		want   string
	}{
		{ExportFormatPDF, ".pdf"},
		{ExportFormatPNG, ".png"},
		{ExportFormatJPEG, ".jpg"},
		{ExportFormatSVG, ".svg"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := tt.format.Extension()
			if got != tt.want {
				t.Errorf("Extension() = %q, want %q", got, tt.want)
			}
		})
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
