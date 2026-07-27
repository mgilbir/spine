package oxml

import (
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
)

// This file also holds the package's single attribute-parse policy for numeric
// SpreadsheetML attributes: parse-or-skip. A value the schema's type cannot
// represent leaves the field unset, so the marshaler omits the attribute and
// the verbatim capture (where the type has one) replays the producer's original
// text. The two rejected alternatives were both live bugs (C552): fabricating a
// zero silently rewrote data — an unparsable f/@si became si="0", merging
// distinct shared-formula groups — and returning the parse error failed the
// whole Open over one advisory hint. Every numeric attribute in this package
// goes through these helpers; none parse inline.

// parseUintPtr returns the parsed value of an xsd:unsignedInt attribute, or nil
// when the text does not parse.
func parseUintPtr(s string) *uint32 {
	n, err := strconv.ParseUint(strings.TrimSpace(s), 10, 32)
	if err != nil {
		return nil
	}
	v := uint32(n)
	return &v
}

// parseUint8Ptr returns the parsed value of an xsd:unsignedByte attribute, or
// nil when the text does not parse.
func parseUint8Ptr(s string) *uint8 {
	n, err := strconv.ParseUint(strings.TrimSpace(s), 10, 8)
	if err != nil {
		return nil
	}
	v := uint8(n)
	return &v
}

// parseIntPtr returns the parsed value of an xsd:int attribute, or nil when the
// text does not parse.
func parseIntPtr(s string) *int32 {
	n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 32)
	if err != nil {
		return nil
	}
	v := int32(n)
	return &v
}

// parseFloatPtr returns the parsed value of an xsd:double attribute, or nil
// when the text does not parse.
func parseFloatPtr(s string) *float64 {
	v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return nil
	}
	return &v
}

// parseOnOff parses ECMA-376 ST_OnOff leniently. The schema admits
// true/false/1/0/on/off, but strconv.ParseBool (which Go's reflection decoder
// uses for bool fields) rejects on/off and any other spelling, failing the
// whole Open on a single odd value. Wild files also carry values outside the
// schema entirely (e.g. "N"); those degrade to false rather than aborting.
func parseOnOff(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "on":
		return true
	default:
		// false/0/off/""/unknown → false (graceful default).
		return false
	}
}

// coerceBoolAttrs returns start.Attr with each listed boolean-typed attribute
// whose value strconv.ParseBool cannot handle rewritten to canonical "1"/"0"
// via lenient ST_OnOff parsing, so the reflection decoder does not fail the
// whole Open on one non-standard value. A copy is made only when a rewrite is
// needed. On a zero-mod save the original styles.xml bytes are preserved raw,
// so this lenience only affects the model, never round-trip fidelity.
func coerceBoolAttrs(attrs []xml.Attr, names ...string) []xml.Attr {
	out := attrs
	copied := false
	for i := range attrs {
		matched := false
		for _, n := range names {
			if attrs[i].Name.Local == n {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		if _, err := strconv.ParseBool(strings.TrimSpace(attrs[i].Value)); err == nil {
			continue
		}
		norm := "0"
		if parseOnOff(attrs[i].Value) {
			norm = "1"
		}
		if !copied {
			out = append([]xml.Attr(nil), attrs...)
			copied = true
		}
		out[i].Value = norm
	}
	return out
}

// BoolLex is a boolean attribute value that re-emits its source lexical form.
// SpreadsheetML booleans have two common lexical spaces: Excel writes "1"/"0"
// while Apache POI and LibreOffice write "true"/"false". Both parse; the
// original spelling is kept and replayed so either producer round-trips
// byte-identically. Values built programmatically marshal as "1"/"0".
type BoolLex struct {
	// Val is the parsed boolean value.
	Val bool

	// orig holds the source lexical form; empty for programmatic values.
	orig string
}

// NewBoolLex returns a BoolLex marshaling as "1"/"0".
func NewBoolLex(v bool) *BoolLex { return &BoolLex{Val: v} }

// UnmarshalXMLAttr implements xml.UnmarshalerAttr. It parses ST_OnOff
// leniently: 1/0/true/false are exact, on/off and any other spelling degrade
// through parseOnOff (unknown → false) rather than failing the whole Open on a
// single odd value (C322). The source spelling is kept in orig and replayed
// verbatim on marshal, so a value like date1904="on" round-trips unchanged.
func (v *BoolLex) UnmarshalXMLAttr(attr xml.Attr) error {
	switch attr.Value {
	case "1", "true":
		v.Val = true
	case "0", "false":
		v.Val = false
	default:
		v.Val = parseOnOff(attr.Value)
	}
	v.orig = attr.Value
	return nil
}

// AttrValue implements xmlb.AttrValuer: the source lexical form when
// captured, otherwise "1"/"0".
func (v BoolLex) AttrValue() string {
	if v.orig != "" {
		return v.orig
	}
	if v.Val {
		return "1"
	}
	return "0"
}

// IsZeroAttr implements xmlb.AttrValuer for omitempty handling: a BoolLex is
// zero only when it was never set (an explicit source "0" is retained).
func (v BoolLex) IsZeroAttr() bool { return !v.Val && v.orig == "" }

// parseBoolLex parses an ST_OnOff attribute into a BoolLex. Parsing is lenient
// (see BoolLex.UnmarshalXMLAttr), so this never fails; the helper exists so the
// hand-written unmarshalers do not each repeat the allocate-and-check dance.
func parseBoolLex(attr xml.Attr) *BoolLex {
	v := &BoolLex{}
	// UnmarshalXMLAttr degrades unknown spellings rather than erroring.
	_ = v.UnmarshalXMLAttr(attr)
	return v
}

// FloatLex is a floating-point attribute value that re-emits its source
// lexical form (e.g. Excel's iterateDelta="1E-4", which Go would otherwise
// reprint as "0.0001").
type FloatLex struct {
	// Val is the parsed value.
	Val float64

	// orig holds the source lexical form; empty for programmatic values.
	orig string
}

// UnmarshalXMLAttr implements xml.UnmarshalerAttr.
func (v *FloatLex) UnmarshalXMLAttr(attr xml.Attr) error {
	f, err := strconv.ParseFloat(attr.Value, 64)
	if err != nil {
		return fmt.Errorf("oxml.FloatLex: parsing %q: %w", attr.Value, err)
	}
	v.Val = f
	v.orig = attr.Value
	return nil
}

// AttrValue implements xmlb.AttrValuer.
func (v FloatLex) AttrValue() string {
	if v.orig != "" {
		return v.orig
	}
	return strconv.FormatFloat(v.Val, 'g', -1, 64)
}

// IsZeroAttr implements xmlb.AttrValuer for omitempty handling.
func (v FloatLex) IsZeroAttr() bool { return v.Val == 0 && v.orig == "" }
