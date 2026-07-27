package dml

import (
	"encoding/xml"
	"math"
	"strconv"
	"strings"
)

// Percentage represents an OOXML percentage value (the ST_Percentage family).
// Internally normalized to thousandths of a percent (e.g., 50000 = 50%).
//
// Two lexical spaces exist for these attributes: ECMA-376 strict (and modern
// Office) writes thousandths integers ("50000"), while transitional /
// first-edition producers write percent strings ("50%", "-20%", "20.5%").
// Both parse; the original lexical form is kept and re-emitted verbatim so a
// transitional source round-trips byte-identically.
type Percentage struct {
	// Val is the normalized value in thousandths of a percent.
	Val int32

	// orig holds the original lexical form when the source used the
	// transitional "%" string form; empty for the integer form (which Val
	// reproduces exactly).
	orig string
}

// NewPercentage returns a Percentage holding the given thousandths-of-a-percent
// value, marshaled in the strict integer form.
func NewPercentage(thousandths int32) Percentage {
	return Percentage{Val: thousandths}
}

// Int32 returns the normalized value in thousandths of a percent.
func (p Percentage) Int32() int32 { return p.Val }

// AttrValue implements xmlb.AttrValuer: the original lexical form when the
// source used the transitional "%" string, otherwise the strict integer form.
func (p Percentage) AttrValue() string {
	if p.orig != "" {
		return p.orig
	}
	return strconv.FormatInt(int64(p.Val), 10)
}

// IsZeroAttr implements xmlb.AttrValuer for omitempty handling: a Percentage
// is zero only when it was never set (an explicit source "0%" is retained).
func (p Percentage) IsZeroAttr() bool {
	return p.Val == 0 && p.orig == ""
}

// UnmarshalXMLAttr implements xml.UnmarshalerAttr.
//
// Parsing degrades rather than failing: a value outside int32, a non-integer
// lexical form, or outright garbage saturates or falls back to zero and keeps
// the source spelling for verbatim re-emission. Returning an error here would
// abort DecodeElement and fail the whole part over one wild attribute, which is
// the opposite of the leniency roundInt64 gives coordinates — and a percentage
// is never load-bearing enough to justify refusing the document.
func (p *Percentage) UnmarshalXMLAttr(attr xml.Attr) error {
	s := strings.TrimSpace(attr.Value)
	if strings.HasSuffix(s, "%") {
		// String form: "50%" → 50000, "20.000%" → 20000, "-20%" → -20000.
		// An unparseable or overflowing number saturates instead of producing
		// an implementation-defined float→int32 conversion.
		numStr := strings.TrimSuffix(s, "%")
		f, err := strconv.ParseFloat(numStr, 64)
		if err != nil {
			p.Val = 0
		} else {
			p.Val = clampToInt32(math.Round(f * 1000))
		}
		p.orig = attr.Value
		return nil
	}
	// Integer form: "50000". Out-of-range and non-integer values saturate to
	// the int32 bound (or 0) and are re-emitted verbatim from orig.
	n, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		if wide, wideErr := strconv.ParseInt(s, 10, 64); wideErr == nil {
			p.Val = clampToInt32(float64(wide))
		} else if f, floatErr := strconv.ParseFloat(s, 64); floatErr == nil {
			p.Val = clampToInt32(math.Round(f))
		} else {
			p.Val = 0
		}
		p.orig = attr.Value
		return nil
	}
	p.Val = int32(n)
	// Keep the source form whenever it is not the canonical rendering of Val
	// (e.g. a zero-padded "050000" or a signed "+50000"): AttrValue would
	// otherwise re-emit the canonical "50000" and drift from the source. The
	// canonical case leaves orig empty so an explicit "0" still reports zero.
	if strconv.FormatInt(n, 10) != s {
		p.orig = attr.Value
	} else {
		p.orig = ""
	}
	return nil
}

// clampToInt32 saturates a float to the int32 range. NaN maps to 0; ±Inf and
// out-of-range magnitudes map to the corresponding bound, so no value reaches
// the implementation-defined float→int32 conversion.
func clampToInt32(f float64) int32 {
	switch {
	case math.IsNaN(f):
		return 0
	case f >= math.MaxInt32:
		return math.MaxInt32
	case f <= math.MinInt32:
		return math.MinInt32
	}
	return int32(f)
}

// MarshalXMLAttr implements xml.MarshalerAttr, re-emitting the original
// lexical form.
func (p Percentage) MarshalXMLAttr(name xml.Name) (xml.Attr, error) {
	return xml.Attr{Name: name, Value: p.AttrValue()}, nil
}
