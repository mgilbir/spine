package xml_test

import (
	"math"
	"strconv"
	"strings"
	"testing"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// TestFormatFloatNeverUsesExponent pins the property the whole policy exists
// for, across the magnitudes where 'g' would have switched notation. The
// boundary cases are the point: 'g' flips at 1e6 and below 1e-4, so a formatter
// that only tested single digits would look correct and ship E-notation the
// moment a chart axis reached a million.
func TestFormatFloatNeverUsesExponent(t *testing.T) {
	values := []float64{
		0, 1, -1, 0.5, 12.75, 3,
		999999, 1e6, -1e6, 1000001, 1e15, 1e20, 1e21,
		1e-3, 1e-4, 1e-5, -1e-5, 1e-7,
		2500000, 0.000005,
		math.MaxFloat64, math.SmallestNonzeroFloat64,
	}
	for _, v := range values {
		got := xmlb.FormatFloat(v)
		if strings.ContainsAny(got, "eE") {
			t.Errorf("FormatFloat(%v) = %q, which is exponent form", v, got)
		}
	}
}

// TestFormatFloatRoundTrips checks the other half: avoiding E-notation must not
// cost precision. Every value must parse back to exactly what went in.
func TestFormatFloatRoundTrips(t *testing.T) {
	values := []float64{
		0, 1, -1, 0.1, 0.5, 12.75, 1.0 / 3.0,
		999999, 1e6, 1e15, 1e20, 1e21, 2500000,
		1e-4, 1e-5, 0.000005, -4.9989318521683403e-2,
		math.MaxFloat64, math.SmallestNonzeroFloat64,
	}
	for _, v := range values {
		got := xmlb.FormatFloat(v)
		back, err := strconv.ParseFloat(got, 64)
		if err != nil {
			t.Errorf("FormatFloat(%v) = %q, which does not parse: %v", v, got, err)
			continue
		}
		if back != v {
			t.Errorf("FormatFloat(%v) = %q, which parses back as %v", v, got, back)
		}
	}
}

// TestFormatFloatSpellings pins the exact output for the values a reviewer is
// most likely to have an opinion about.
func TestFormatFloatSpellings(t *testing.T) {
	cases := map[float64]string{
		0:       "0",
		1:       "1",
		3:       "3",
		12.75:   "12.75",
		1e6:     "1000000",
		2500000: "2500000",
		1e-5:    "0.00001",
		-1e6:    "-1000000",
	}
	for in, want := range cases {
		if got := xmlb.FormatFloat(in); got != want {
			t.Errorf("FormatFloat(%v) = %q, want %q", in, got, want)
		}
	}
}

// TestFormatFloatExtremesAreBounded documents the cost of never using exponent
// form, so the tradeoff is a recorded decision rather than a surprise.
func TestFormatFloatExtremesAreBounded(t *testing.T) {
	for _, v := range []float64{math.MaxFloat64, math.SmallestNonzeroFloat64} {
		if got := len(xmlb.FormatFloat(v)); got > 1200 {
			t.Errorf("FormatFloat(%v) is %d characters, beyond the documented bound", v, got)
		}
	}
}
