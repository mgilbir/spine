package chart

import "math"

// Blank returns the sentinel that marks a blank data point in a Series'
// Values, XValues, or Sizes: a NaN.
//
// A spreadsheet cell can be empty, and a chart plots that as a gap rather than
// as zero. Go's []float64 has no "no value here", so the package uses NaN as
// the blank sentinel throughout: Parse writes it for every cache position no
// c:pt lands on (Excel omits c:pt for blank cells), and every write path —
// the numeric caches in chart.xml, the embedded workbook, and the host
// worksheet an xlsx chart's references point at — turns it back into an
// omitted point or an empty cell. Use it when building a chart with gaps:
//
//	c.AddSeries("North", []float64{10, chart.Blank(), 30})
//
// Never compare against it with ==: NaN != NaN. Test with IsBlank.
func Blank() float64 { return math.NaN() }

// IsBlank reports whether v marks a blank data point rather than a number to
// plot. It is the predicate every consumer of Series.Values must use before
// treating a value as a number.
//
// Any non-finite value counts as blank: NaN is the sentinel Blank returns, and
// ±Inf has no SpreadsheetML representation either (writing it would produce a
// cache or cell Excel reports as damaged), so it is written as a blank too.
func IsBlank(v float64) bool { return math.IsNaN(v) || math.IsInf(v, 0) }
