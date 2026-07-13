package dml

import (
	"encoding/xml"
	"fmt"
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
func (p *Percentage) UnmarshalXMLAttr(attr xml.Attr) error {
	s := strings.TrimSpace(attr.Value)
	if strings.HasSuffix(s, "%") {
		// String form: "50%" → 50000, "20.000%" → 20000, "-20%" → -20000
		numStr := strings.TrimSuffix(s, "%")
		f, err := strconv.ParseFloat(numStr, 64)
		if err != nil {
			return fmt.Errorf("dml.Percentage: parsing %q: %w", attr.Value, err)
		}
		p.Val = int32(math.Round(f * 1000))
		p.orig = attr.Value
		return nil
	}
	// Integer form: "50000"
	n, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		return fmt.Errorf("dml.Percentage: parsing %q: %w", attr.Value, err)
	}
	p.Val = int32(n)
	p.orig = ""
	return nil
}

// MarshalXMLAttr implements xml.MarshalerAttr, re-emitting the original
// lexical form.
func (p Percentage) MarshalXMLAttr(name xml.Name) (xml.Attr, error) {
	return xml.Attr{Name: name, Value: p.AttrValue()}, nil
}
