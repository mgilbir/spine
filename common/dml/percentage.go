package dml

import (
	"encoding/xml"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// Percentage represents an OOXML percentage value (ST_Percentage).
// Internally stored as thousandths of a percent (e.g., 50000 = 50%).
//
// Handles two forms during unmarshaling:
//   - String form from spec examples: "50%", "-20%", "20.000%"
//   - Integer form from real Office files: 50000, -20000, 20000
//
// Always marshals to integer form for round-trip fidelity with real documents.
type Percentage int32

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
		*p = Percentage(math.Round(f * 1000))
		return nil
	}
	// Integer form: "50000"
	n, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		return fmt.Errorf("dml.Percentage: parsing %q: %w", attr.Value, err)
	}
	*p = Percentage(n)
	return nil
}

// MarshalXMLAttr implements xml.MarshalerAttr.
func (p Percentage) MarshalXMLAttr(name xml.Name) (xml.Attr, error) {
	return xml.Attr{Name: name, Value: strconv.FormatInt(int64(p), 10)}, nil
}
