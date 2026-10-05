package pptxrender

import "math"

// renderAxisHints are the explicit scaling a value axis carries; nil leaves
// that part to the automatic scale.
type renderAxisHints struct {
	min, max, major *float64
}

// renderAutoAxis is a value axis' resolved scale: its ends and the values
// of its major steps.
type renderAutoAxis struct {
	min, max, step float64
	ticks          []float64
}

// Spacing the automatic major unit keeps between steps, in multiples of the
// axis' text size: a vertical axis needs the height of a label, a horizontal
// one room for its widest.
const (
	renderVertGap  = 1.2
	renderHorizGap = 6.0
)

// renderMaxAutoIntervals is the most intervals an automatic major unit
// divides an axis into, as Office's automatic scaling does: data up to 4.5
// gets 0 to 5 by 0.5, data up to 5 gets 0 to 6 by 1.
const renderMaxAutoIntervals = 10

// renderMaxAxisTicks bounds the steps of an axis, which an explicit tiny
// major unit could otherwise make enormous.
const renderMaxAxisTicks = 200

// renderMajorUnit is the automatic major unit of an axis of the given span
// drawn length pixels long: the least 1, 2 or 5 times a power of ten that
// keeps its steps at least gap pixels apart.
func renderMajorUnit(span, length, gap float64) float64 {
	if !(span > 0) || math.IsInf(span, 0) {
		return 1
	}
	intervals := math.Max(1, length/gap)
	raw := span / intervals
	mag := math.Pow(10, math.Floor(math.Log10(raw)))
	for _, m := range []float64{1, 2, 5, 10, 20, 50} {
		if m*mag >= raw*(1-1e-9) {
			return m * mag
		}
	}
	return 100 * mag
}

// renderAutoScale resolves a value axis the way Office's automatic scaling
// does. Over data from lo to hi it
//
//   - starts at zero when the data's range is more than a sixth of its
//     largest magnitude, and otherwise half a range below the data;
//   - ends above the data by a twentieth of its range;
//   - steps by renderMajorUnit of the span, over an axis length pixels long
//     whose steps keep gap pixels apart and that has at most
//     renderMaxAutoIntervals steps, and rounds both ends out to a multiple
//     of the step.
//
// A percent-stacked axis is 0 to 1 with no padding. Explicit minimum,
// maximum and major unit win.
func renderAutoScale(lo, hi float64, h renderAxisHints, length, gap float64, percent bool) renderAutoAxis {
	if math.IsNaN(lo) || math.IsNaN(hi) || math.IsInf(lo, 0) || math.IsInf(hi, 0) {
		lo, hi = 0, 1
	}
	if lo > hi {
		lo, hi = hi, lo
	}
	var amin, amax float64
	switch {
	case percent:
		amin, amax = 0, 1
	case lo == 0 && hi == 0:
		amax = 1
	case lo >= 0:
		span := hi - lo
		if span/hi > 1.0/6 {
			amin = 0
		} else {
			amin = lo - span/2
		}
		amax = hi + 0.05*span
		if span == 0 {
			amax = hi + 0.05*hi
		}
	case hi <= 0:
		span := hi - lo
		if span/-lo > 1.0/6 {
			amax = 0
		} else {
			amax = hi + span/2
		}
		amin = lo - 0.05*span
		if span == 0 {
			amin = lo + 0.05*lo
		}
	default:
		pad := 0.05 * (hi - lo)
		amin, amax = lo-pad, hi+pad
	}
	if h.min != nil {
		amin = *h.min
	}
	if h.max != nil {
		amax = *h.max
	}
	if amax <= amin {
		amax = amin + 1
	}
	step := 0.0
	if h.major != nil && *h.major > 0 && (amax-amin) / *h.major <= renderMaxAxisTicks {
		step = *h.major
	} else {
		step = renderMajorUnit(amax-amin, length, gap)
		// Rounding the ends out widens the span, which can crowd the steps.
		for range 4 {
			lo2, hi2 := amin, amax
			if h.min == nil {
				lo2 = math.Floor(amin/step+1e-9) * step
			}
			if h.max == nil {
				hi2 = math.Ceil(amax/step-1e-9) * step
			}
			spaced := length <= 0 || length*step/(hi2-lo2) >= gap
			if spaced && (hi2-lo2)/step <= renderMaxAutoIntervals+1e-9 {
				break
			}
			step = renderNextUnit(step)
		}
	}
	if h.min == nil {
		amin = math.Floor(amin/step+1e-9) * step
	}
	if h.max == nil {
		amax = math.Ceil(amax/step-1e-9) * step
	}
	if amax <= amin {
		amax = amin + step
	}
	out := renderAutoAxis{min: renderTidy(amin), max: renderTidy(amax), step: step}
	for i := 0; i <= renderMaxAxisTicks; i++ {
		v := amin + float64(i)*step
		if v > amax+step*1e-9 {
			break
		}
		out.ticks = append(out.ticks, renderTidy(v))
	}
	return out
}

// renderTidy drops the floating-point noise of a computed axis value.
func renderTidy(v float64) float64 {
	return math.Round(v*1e9) / 1e9
}

// renderNextUnit is the 1, 2 or 5 times a power of ten above step.
func renderNextUnit(step float64) float64 {
	mag := math.Pow(10, math.Floor(math.Log10(step*(1+1e-9))))
	switch m := step / mag; {
	case m < 1.5:
		return 2 * mag
	case m < 3.5:
		return 5 * mag
	}
	return 10 * mag
}
