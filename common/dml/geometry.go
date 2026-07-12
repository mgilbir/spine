// This file provides DrawingML primitive types used across OOXML formats.

package dml

import "math"

// EMU represents English Metric Units, the base unit in OOXML.
// 1 inch = 914400 EMUs
// 1 point = 12700 EMUs
// 1 cm = 360000 EMUs
type EMU int64

// Conversion constants for EMU
const (
	EMUsPerInch       EMU = 914400
	EMUsPerPoint      EMU = 12700
	EMUsPerCentimeter EMU = 360000
	EMUsPerMillimeter EMU = 36000
	EMUsPerPixel      EMU = 9525  // at 96 DPI
)

// Inches converts inches to EMU. The result is rounded to the nearest EMU so
// that exact fractional inches (e.g. 0.57") do not truncate off by one.
func Inches(inches float64) EMU {
	return EMU(math.Round(inches * float64(EMUsPerInch)))
}

// Points converts points to EMU.
func Points(points float64) EMU {
	return EMU(math.Round(points * float64(EMUsPerPoint)))
}

// Centimeters converts centimeters to EMU.
func Centimeters(cm float64) EMU {
	return EMU(math.Round(cm * float64(EMUsPerCentimeter)))
}

// Millimeters converts millimeters to EMU.
func Millimeters(mm float64) EMU {
	return EMU(math.Round(mm * float64(EMUsPerMillimeter)))
}

// Pixels converts pixels to EMU (assuming 96 DPI).
func Pixels(px int) EMU {
	return EMU(px) * EMUsPerPixel
}

// ToInches converts EMU to inches.
func (e EMU) ToInches() float64 {
	return float64(e) / float64(EMUsPerInch)
}

// ToPoints converts EMU to points.
func (e EMU) ToPoints() float64 {
	return float64(e) / float64(EMUsPerPoint)
}

// ToCentimeters converts EMU to centimeters.
func (e EMU) ToCentimeters() float64 {
	return float64(e) / float64(EMUsPerCentimeter)
}

// ToMillimeters converts EMU to millimeters.
func (e EMU) ToMillimeters() float64 {
	return float64(e) / float64(EMUsPerMillimeter)
}

// ToPixels converts EMU to pixels (assuming 96 DPI), rounding to the nearest
// pixel rather than truncating.
func (e EMU) ToPixels() int {
	return int(math.Round(float64(e) / float64(EMUsPerPixel)))
}

// Point represents a 2D point in EMU coordinates.
type Point struct {
	X EMU
	Y EMU
}

// NewPoint creates a new Point with the specified coordinates.
func NewPoint(x, y EMU) Point {
	return Point{X: x, Y: y}
}

// Rect represents a rectangle with position and size in EMU.
type Rect struct {
	X      EMU // Left position
	Y      EMU // Top position
	Width  EMU
	Height EMU
}

// NewRect creates a new Rect with the specified dimensions.
func NewRect(x, y, width, height EMU) Rect {
	return Rect{X: x, Y: y, Width: width, Height: height}
}

// Right returns the right edge position (X + Width).
func (r Rect) Right() EMU {
	return r.X + r.Width
}

// Bottom returns the bottom edge position (Y + Height).
func (r Rect) Bottom() EMU {
	return r.Y + r.Height
}

// Center returns the center point of the rectangle.
func (r Rect) Center() Point {
	return Point{
		X: r.X + r.Width/2,
		Y: r.Y + r.Height/2,
	}
}

// Contains returns true if the point is inside the rectangle.
func (r Rect) Contains(p Point) bool {
	return p.X >= r.X && p.X <= r.Right() && p.Y >= r.Y && p.Y <= r.Bottom()
}

// Size represents dimensions in EMU.
type Size struct {
	Width  EMU
	Height EMU
}

// NewSize creates a new Size with the specified dimensions.
func NewSize(width, height EMU) Size {
	return Size{Width: width, Height: height}
}

// Offset represents a 2D offset in EMU.
type Offset struct {
	X EMU
	Y EMU
}

// NewOffset creates a new Offset with the specified values.
func NewOffset(x, y EMU) Offset {
	return Offset{X: x, Y: y}
}

// Extents represents 2D extents (size) in EMU.
// This matches the a:ext element in OOXML.
type Extents struct {
	Cx EMU // Width
	Cy EMU // Height
}

// NewExtents creates a new Extents with the specified dimensions.
func NewExtents(cx, cy EMU) Extents {
	return Extents{Cx: cx, Cy: cy}
}
