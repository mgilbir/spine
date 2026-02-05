package pptx

import (
	"github.com/mgilbir/spine/common/dml"
)

// Options contains configuration options for creating presentations.
type Options struct {
	// SlideSize specifies the slide dimensions.
	SlideSize SlideSize

	// DefaultFont specifies the default font for text.
	DefaultFont string

	// DefaultFontSize specifies the default font size in points.
	DefaultFontSize float64

	// Locale specifies the locale for the presentation (e.g., "en-US").
	Locale string
}

// SlideSize specifies predefined slide dimensions.
type SlideSize int

const (
	// SlideSizeStandard is 4:3 aspect ratio (10" x 7.5").
	SlideSizeStandard SlideSize = iota

	// SlideSizeWidescreen is 16:9 aspect ratio (13.33" x 7.5").
	SlideSizeWidescreen

	// SlideSizeA4Portrait is A4 paper in portrait orientation.
	SlideSizeA4Portrait

	// SlideSizeA4Landscape is A4 paper in landscape orientation.
	SlideSizeA4Landscape

	// SlideSizeLetterPortrait is US Letter in portrait orientation.
	SlideSizeLetterPortrait

	// SlideSizeLetterLandscape is US Letter in landscape orientation.
	SlideSizeLetterLandscape

	// SlideSizeCustom indicates custom dimensions.
	SlideSizeCustom
)

// Dimensions returns the width and height for the slide size.
func (ss SlideSize) Dimensions() (width, height dml.EMU) {
	switch ss {
	case SlideSizeStandard:
		return dml.Inches(10), dml.Inches(7.5)
	case SlideSizeWidescreen:
		return dml.Inches(13.333), dml.Inches(7.5)
	case SlideSizeA4Portrait:
		return dml.Millimeters(210), dml.Millimeters(297)
	case SlideSizeA4Landscape:
		return dml.Millimeters(297), dml.Millimeters(210)
	case SlideSizeLetterPortrait:
		return dml.Inches(8.5), dml.Inches(11)
	case SlideSizeLetterLandscape:
		return dml.Inches(11), dml.Inches(8.5)
	default:
		return dml.Inches(10), dml.Inches(7.5)
	}
}

// String returns the string representation of the slide size.
func (ss SlideSize) String() string {
	switch ss {
	case SlideSizeStandard:
		return "Standard (4:3)"
	case SlideSizeWidescreen:
		return "Widescreen (16:9)"
	case SlideSizeA4Portrait:
		return "A4 Portrait"
	case SlideSizeA4Landscape:
		return "A4 Landscape"
	case SlideSizeLetterPortrait:
		return "Letter Portrait"
	case SlideSizeLetterLandscape:
		return "Letter Landscape"
	case SlideSizeCustom:
		return "Custom"
	default:
		return "Unknown"
	}
}

// DefaultOptions returns the default options for creating a presentation.
func DefaultOptions() Options {
	return Options{
		SlideSize:       SlideSizeWidescreen,
		DefaultFont:     "Calibri",
		DefaultFontSize: 18,
		Locale:          "en-US",
	}
}

// CreateOptions are options for creating a new presentation.
type CreateOptions struct {
	Options

	// IncludeDefaultLayouts includes standard slide layouts.
	IncludeDefaultLayouts bool

	// IncludeDefaultTheme includes the default Office theme.
	IncludeDefaultTheme bool
}

// DefaultCreateOptions returns the default options for creating a presentation.
func DefaultCreateOptions() CreateOptions {
	return CreateOptions{
		Options:               DefaultOptions(),
		IncludeDefaultLayouts: true,
		IncludeDefaultTheme:   true,
	}
}

// OpenOptions are options for opening an existing presentation.
type OpenOptions struct {
	// ReadOnly opens the presentation in read-only mode.
	ReadOnly bool

	// Password is the password for encrypted presentations.
	Password string
}

// SaveOptions are options for saving a presentation.
type SaveOptions struct {
	// Compression specifies the compression level (0-9).
	// 0 = no compression, 9 = maximum compression.
	Compression int

	// Password encrypts the presentation with the specified password.
	Password string

	// RemovePersonalInfo removes personal information from properties.
	RemovePersonalInfo bool
}

// DefaultSaveOptions returns the default options for saving a presentation.
func DefaultSaveOptions() SaveOptions {
	return SaveOptions{
		Compression: 6, // Default deflate compression
	}
}

// ExportOptions are options for exporting a presentation.
type ExportOptions struct {
	// Format specifies the export format.
	Format ExportFormat

	// Quality specifies the image quality for raster formats (1-100).
	Quality int

	// DPI specifies the resolution for raster formats.
	DPI int
}

// ExportFormat specifies the export format.
type ExportFormat int

const (
	ExportFormatPDF ExportFormat = iota
	ExportFormatPNG
	ExportFormatJPEG
	ExportFormatSVG
)

// String returns the string representation of the export format.
func (f ExportFormat) String() string {
	switch f {
	case ExportFormatPDF:
		return "PDF"
	case ExportFormatPNG:
		return "PNG"
	case ExportFormatJPEG:
		return "JPEG"
	case ExportFormatSVG:
		return "SVG"
	default:
		return "Unknown"
	}
}

// Extension returns the file extension for the export format.
func (f ExportFormat) Extension() string {
	switch f {
	case ExportFormatPDF:
		return ".pdf"
	case ExportFormatPNG:
		return ".png"
	case ExportFormatJPEG:
		return ".jpg"
	case ExportFormatSVG:
		return ".svg"
	default:
		return ""
	}
}
