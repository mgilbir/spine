package dml

import (
	"encoding/xml"
	"math"
	"testing"
)

// TestPercentageDegradesInsteadOfFailing pins C484: a wild percentage value
// must not abort the enclosing DecodeElement (and with it the whole part).
// Coordinates already degrade through roundInt64; percentages contradicted
// that policy by returning an error from UnmarshalXMLAttr.
func TestPercentageDegradesInsteadOfFailing(t *testing.T) {
	tests := []struct {
		in   string
		want int32
	}{
		{"99999999999", math.MaxInt32},  // beyond int32, integer form
		{"-99999999999", math.MinInt32}, // beyond int32, negative
		{"9e9%", math.MaxInt32},         // beyond int32 after ×1000
		{"-9e9%", math.MinInt32},        // ditto, negative
		{"2147483647", math.MaxInt32},   // exactly the bound, still fine
		{"1e5", 100000},                 // non-integer lexical form
		{"not-a-number", 0},             // outright garbage
		{"%", 0},                        // degenerate percent form
		{"340282366920938463463374607431768211456", math.MaxInt32}, // > int64 too
	}
	for _, tc := range tests {
		var p Percentage
		if err := p.UnmarshalXMLAttr(xml.Attr{Name: xml.Name{Local: "val"}, Value: tc.in}); err != nil {
			t.Errorf("UnmarshalXMLAttr(%q) returned an error (C484: must degrade, not fail): %v", tc.in, err)
			continue
		}
		if got := p.Int32(); got != tc.want {
			t.Errorf("UnmarshalXMLAttr(%q) = %d, want %d", tc.in, got, tc.want)
		}
		// A degraded value keeps its source spelling, so a save cannot silently
		// rewrite the producer's attribute to a clamped number.
		if got := p.AttrValue(); got != tc.in {
			t.Errorf("AttrValue after %q = %q, want the verbatim source", tc.in, got)
		}
	}
}

// TestPercentageWildValueDoesNotFailPart is the whole-part consequence: one
// out-of-range percentage used to take down DecodeElement for the element that
// contains it, which on the production path means Open fails.
func TestPercentageWildValueDoesNotFailPart(t *testing.T) {
	src := `<a:gs xmlns:a="` + NsDrawingML + `" pos="99999999999"><a:srgbClr val="FF0000"/></a:gs>`
	var gs Gs
	if err := xml.Unmarshal([]byte(src), &gs); err != nil {
		t.Fatalf("a gradient stop with a wild pos failed the whole element: %v", err)
	}
	if gs.SrgbClr == nil {
		t.Error("srgbClr child lost")
	}
	if got := gs.Pos.AttrValue(); got != "99999999999" {
		t.Errorf("pos re-emits as %q, want the verbatim source", got)
	}
}
