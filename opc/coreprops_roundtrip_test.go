package opc

import (
	"strings"
	"testing"
	"time"
)

// TestCoreProperties_PreservesReducedPrecisionDate verifies that a W3CDTF date
// with reduced precision (not valid RFC3339) is parsed rather than dropped, and
// re-emitted in its original lexical form (C47).
func TestCoreProperties_PreservesReducedPrecisionDate(t *testing.T) {
	for _, raw := range []string{"2024", "2024-01", "2024-01-15"} {
		src := []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
			`<cp:coreProperties xmlns:cp="` + nsCoreProperties + `" xmlns:dcterms="` + nsDcTerms +
			`" xmlns:xsi="` + nsXsi + `">` +
			`<dcterms:created xsi:type="dcterms:W3CDTF">` + raw + `</dcterms:created>` +
			`</cp:coreProperties>`)

		cp, err := UnmarshalCoreProperties(src)
		if err != nil {
			t.Fatalf("UnmarshalCoreProperties(%q) error = %v", raw, err)
		}
		if cp.Created.IsZero() {
			t.Errorf("date %q was dropped (Created is zero)", raw)
		}
		out, err := cp.Marshal()
		if err != nil {
			t.Fatalf("Marshal() error = %v", err)
		}
		if !strings.Contains(string(out), `>`+raw+`</dcterms:created>`) {
			t.Errorf("date %q not preserved verbatim, got: %s", raw, out)
		}
	}
}

// TestCoreProperties_ReassignedDateReformatted verifies that programmatically
// setting a date after load emits the new value (not the stale raw form).
func TestCoreProperties_ReassignedDateReformatted(t *testing.T) {
	src := []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
		`<cp:coreProperties xmlns:cp="` + nsCoreProperties + `" xmlns:dcterms="` + nsDcTerms +
		`" xmlns:xsi="` + nsXsi + `">` +
		`<dcterms:created xsi:type="dcterms:W3CDTF">2024</dcterms:created>` +
		`</cp:coreProperties>`)

	cp, err := UnmarshalCoreProperties(src)
	if err != nil {
		t.Fatalf("UnmarshalCoreProperties() error = %v", err)
	}
	cp.Created = time.Date(2030, 6, 1, 12, 0, 0, 0, time.UTC)
	out, err := cp.Marshal()
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if strings.Contains(string(out), ">2024<") {
		t.Errorf("stale raw date emitted after reassignment: %s", out)
	}
	if !strings.Contains(string(out), "2030-06-01T12:00:00Z") {
		t.Errorf("reassigned date not emitted: %s", out)
	}
}

// TestCoreProperties_NoApostropheOverEscaping verifies that apostrophes in text
// content survive as literal characters (C122).
func TestCoreProperties_NoApostropheOverEscaping(t *testing.T) {
	cp := &CoreProperties{Creator: "O'Brien", Title: `a "quote" & <tag>`}
	out, err := cp.Marshal()
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "<dc:creator>O'Brien</dc:creator>") {
		t.Errorf("apostrophe over-escaped: %s", s)
	}
	// Ampersand and angle brackets must still be escaped; quotes must not.
	if !strings.Contains(s, `a "quote" &amp; &lt;tag&gt;`) {
		t.Errorf("text content escaping wrong: %s", s)
	}
}
