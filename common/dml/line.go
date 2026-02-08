package dml

import "math"

// DashStyle represents a preset line dash pattern.
type DashStyle string

const (
	DashSolid    DashStyle = "solid"
	DashDot      DashStyle = "dot"
	DashDash     DashStyle = "dash"
	DashDashDot  DashStyle = "dashDot"
	DashLongDash DashStyle = "lgDash"
)

// Line represents line/stroke properties for shapes.
type Line struct {
	Width float64   // points
	Color Color
	Dash  DashStyle
}

// ApplyToSpPr sets the line on a SpPr.
func (l Line) ApplyToSpPr(spPr *SpPr) {
	ln := &Ln{}

	// Width in EMUs (1 point = 12700 EMUs)
	if l.Width > 0 {
		w := int64(math.Round(l.Width * 12700))
		ln.W = &w
	}

	// Color as solid fill on the line
	ln.SolidFill = colorToSolidFill(l.Color)

	// Dash style
	if l.Dash != "" && l.Dash != DashSolid {
		ln.PrstDash = &PrstDash{Val: string(l.Dash)}
	}

	spPr.Ln = ln
}
