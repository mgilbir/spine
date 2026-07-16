package oxml

import (
	"testing"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// Wild styles.xml files carry ST_OnOff boolean attributes with spellings
// strconv.ParseBool rejects (on/off) or values outside the schema ("N"). A
// single such value must not fail the whole Open with a bare ParseBool error.
func TestStylesLenientBooleans(t *testing.T) {
	const ns = "http://schemas.openxmlformats.org/spreadsheetml/2006/main"

	t.Run("xf on/off", func(t *testing.T) {
		var xf CT_Xf
		src := `<xf xmlns="` + ns + `" applyFont="on" applyFill="off" quotePrefix="1"/>`
		if err := xmlb.UnmarshalWithSource([]byte(src), &xf); err != nil {
			t.Fatalf("Open must not fail: %v", err)
		}
		if xf.ApplyFont == nil || !*xf.ApplyFont {
			t.Errorf("applyFont: got %v, want true", xf.ApplyFont)
		}
		if xf.ApplyFill == nil || *xf.ApplyFill {
			t.Errorf("applyFill: got %v, want false", xf.ApplyFill)
		}
		if xf.QuotePrefix == nil || !*xf.QuotePrefix {
			t.Errorf("quotePrefix: got %v, want true", xf.QuotePrefix)
		}
	})

	t.Run("protection out-of-schema value", func(t *testing.T) {
		var p CT_CellProtection
		src := `<protection xmlns="` + ns + `" locked="N" hidden="true"/>`
		if err := xmlb.UnmarshalWithSource([]byte(src), &p); err != nil {
			t.Fatalf("Open must not fail on out-of-schema bool: %v", err)
		}
		if p.Locked == nil || *p.Locked {
			t.Errorf("locked=N should degrade to false, got %v", p.Locked)
		}
		if p.Hidden == nil || !*p.Hidden {
			t.Errorf("hidden=true, got %v", p.Hidden)
		}
	})

	t.Run("font boolean property val=on", func(t *testing.T) {
		var b CT_BooleanProperty
		if err := xmlb.UnmarshalWithSource([]byte(`<b xmlns="`+ns+`" val="on"/>`), &b); err != nil {
			t.Fatalf("Open must not fail: %v", err)
		}
		if b.Val == nil || !*b.Val {
			t.Errorf("val=on should parse true, got %v", b.Val)
		}
	})

	// Valid boolean spellings are untouched (coerceBoolAttrs is a no-op when
	// ParseBool accepts): the byte-identity corpus is unaffected.
	t.Run("standard spellings unchanged", func(t *testing.T) {
		var xf CT_Xf
		src := `<xf xmlns="` + ns + `" applyFont="1" applyFill="0" applyBorder="true" applyAlignment="false"/>`
		if err := xmlb.UnmarshalWithSource([]byte(src), &xf); err != nil {
			t.Fatal(err)
		}
		if !*xf.ApplyFont || *xf.ApplyFill || !*xf.ApplyBorder || *xf.ApplyAlignment {
			t.Errorf("standard bool spellings misparsed: %v %v %v %v",
				*xf.ApplyFont, *xf.ApplyFill, *xf.ApplyBorder, *xf.ApplyAlignment)
		}
	})
}
