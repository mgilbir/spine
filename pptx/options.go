package pptx

import (
	"github.com/mgilbir/spine/common/dml"
)

// Options contains configuration options for creating presentations.
type Options struct {
	// SlideSize specifies the slide dimensions.
	SlideSize SlideSize
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
		// 13 1/3" x 7.5": the canonical widescreen 12192000 x 6858000 EMU.
		return dml.EMU(12192000), dml.Inches(7.5)
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
// The default slide size is 4:3, matching Create.
func DefaultOptions() Options {
	return Options{
		SlideSize: SlideSizeStandard,
	}
}

// CreateOptions are options for creating a new presentation.
type CreateOptions struct {
	Options

	// IncludeDefaultLayouts includes standard slide layouts.
	IncludeDefaultLayouts bool

	// Width and Height are the custom slide dimensions in EMU, consulted only
	// when SlideSize is SlideSizeCustom. When either is unset (0), a custom
	// deck falls back to the 4:3 default. The dml.Inches / dml.Millimeters
	// helpers convert convenient units to EMU.
	Width  dml.EMU
	Height dml.EMU
}

// DefaultCreateOptions returns the default options for creating a presentation.
func DefaultCreateOptions() CreateOptions {
	return CreateOptions{
		Options:               DefaultOptions(),
		IncludeDefaultLayouts: true,
	}
}
