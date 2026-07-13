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

// TestCoreProperties_UnknownChildRoundTrip verifies that a vendor extension
// element inside cp:coreProperties survives regeneration verbatim, at its
// original position (C48).
func TestCoreProperties_UnknownChildRoundTrip(t *testing.T) {
	src := []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
		`<cp:coreProperties xmlns:cp="` + nsCoreProperties + `" xmlns:dc="` + nsDublinCore + `">` +
		`<dc:title>T</dc:title>` +
		`<x:custom xmlns:x="urn:x" flag="1">v</x:custom>` +
		`<cp:keywords>k</cp:keywords>` +
		`</cp:coreProperties>`)

	cp, err := UnmarshalCoreProperties(src)
	if err != nil {
		t.Fatalf("UnmarshalCoreProperties() error = %v", err)
	}
	out, err := cp.Marshal()
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	want := `<dc:title>T</dc:title><x:custom xmlns:x="urn:x" flag="1">v</x:custom><cp:keywords>k</cp:keywords>`
	if !strings.Contains(string(out), want) {
		t.Errorf("unknown child not preserved verbatim in position; want %s in:\n%s", want, out)
	}
}

// TestCoreProperties_UnknownChildKnownNamespace verifies that an unknown local
// name in the standard cp namespace is preserved verbatim without a redundant
// namespace declaration (the regenerated root already declares cp:) (C48).
func TestCoreProperties_UnknownChildKnownNamespace(t *testing.T) {
	src := []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
		`<cp:coreProperties xmlns:cp="` + nsCoreProperties + `">` +
		`<cp:vendorThing>v</cp:vendorThing>` +
		`</cp:coreProperties>`)

	cp, err := UnmarshalCoreProperties(src)
	if err != nil {
		t.Fatalf("UnmarshalCoreProperties() error = %v", err)
	}
	out, err := cp.Marshal()
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if !strings.Contains(string(out), `<cp:vendorThing>v</cp:vendorThing>`) {
		t.Errorf("unknown cp-namespace child not preserved verbatim:\n%s", out)
	}
}

// TestCoreProperties_UnknownSelfClosingChild verifies a self-closing unknown
// child round-trips (C48).
func TestCoreProperties_UnknownSelfClosingChild(t *testing.T) {
	src := []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
		`<cp:coreProperties xmlns:cp="` + nsCoreProperties + `">` +
		`<x:mark xmlns:x="urn:x"/>` +
		`</cp:coreProperties>`)

	cp, err := UnmarshalCoreProperties(src)
	if err != nil {
		t.Fatalf("UnmarshalCoreProperties() error = %v", err)
	}
	out, err := cp.Marshal()
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if !strings.Contains(string(out), `<x:mark xmlns:x="urn:x"/>`) {
		t.Errorf("self-closing unknown child not preserved verbatim:\n%s", out)
	}
}

// TestCoreProperties_EmptyDateElementPreserved verifies that a present-but-
// empty date element is re-emitted rather than dropped on regeneration,
// matching the treatment of empty string elements (C209/NEW-9).
func TestCoreProperties_EmptyDateElementPreserved(t *testing.T) {
	src := []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
		`<cp:coreProperties xmlns:cp="` + nsCoreProperties + `" xmlns:dc="` + nsDublinCore + `">` +
		`<dc:title></dc:title>` +
		`<cp:lastPrinted></cp:lastPrinted>` +
		`</cp:coreProperties>`)

	cp, err := UnmarshalCoreProperties(src)
	if err != nil {
		t.Fatalf("UnmarshalCoreProperties() error = %v", err)
	}
	out, err := cp.Marshal()
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if !strings.Contains(string(out), `<cp:lastPrinted></cp:lastPrinted>`) {
		t.Errorf("empty lastPrinted element dropped on regeneration:\n%s", out)
	}
	if !strings.Contains(string(out), `<dc:title></dc:title>`) {
		t.Errorf("empty title element dropped on regeneration:\n%s", out)
	}
}

// TestCoreProperties_EmptyDateReassignedEmitsValue verifies that assigning a
// date after loading an empty date element replaces the empty element.
func TestCoreProperties_EmptyDateReassignedEmitsValue(t *testing.T) {
	src := []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
		`<cp:coreProperties xmlns:cp="` + nsCoreProperties + `">` +
		`<cp:lastPrinted></cp:lastPrinted>` +
		`</cp:coreProperties>`)

	cp, err := UnmarshalCoreProperties(src)
	if err != nil {
		t.Fatalf("UnmarshalCoreProperties() error = %v", err)
	}
	cp.LastPrinted = time.Date(2030, 6, 1, 12, 0, 0, 0, time.UTC)
	out, err := cp.Marshal()
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if !strings.Contains(string(out), `<cp:lastPrinted>2030-06-01T12:00:00Z</cp:lastPrinted>`) {
		t.Errorf("reassigned lastPrinted not emitted:\n%s", out)
	}
}

// TestCoreProperties_CloneAndEqual verifies the deep copy and the exported-
// field comparison used to detect post-open property edits (C10).
func TestCoreProperties_CloneAndEqual(t *testing.T) {
	src := []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
		`<cp:coreProperties xmlns:cp="` + nsCoreProperties + `" xmlns:dc="` + nsDublinCore +
		`" xmlns:dcterms="` + nsDcTerms + `" xmlns:xsi="` + nsXsi + `">` +
		`<dc:title>T</dc:title>` +
		`<dcterms:created xsi:type="dcterms:W3CDTF">2024-01-15</dcterms:created>` +
		`<x:custom xmlns:x="urn:x">v</x:custom>` +
		`</cp:coreProperties>`)

	cp, err := UnmarshalCoreProperties(src)
	if err != nil {
		t.Fatalf("UnmarshalCoreProperties() error = %v", err)
	}

	clone := cp.Clone()
	if !cp.Equal(clone) {
		t.Fatal("clone not Equal to original")
	}

	// The clone marshals identically (bookkeeping was deep-copied).
	origOut, err := cp.Marshal()
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	cloneOut, err := clone.Marshal()
	if err != nil {
		t.Fatalf("clone Marshal() error = %v", err)
	}
	if string(origOut) != string(cloneOut) {
		t.Errorf("clone marshals differently:\n%s\nvs\n%s", origOut, cloneOut)
	}

	// Mutating the clone's bookkeeping must not affect the original.
	clone.rawDates["dcterms:created"] = "1999"
	clone.presentFields["dc:subject"] = true
	if cp.rawDates["dcterms:created"] != "2024-01-15" {
		t.Error("clone shares rawDates map with original")
	}
	if cp.presentFields["dc:subject"] {
		t.Error("clone shares presentFields map with original")
	}

	// Exported-field edits flip Equal.
	clone2 := cp.Clone()
	clone2.Title = "changed"
	if cp.Equal(clone2) {
		t.Error("Equal ignored a Title edit")
	}
	clone3 := cp.Clone()
	clone3.Created = clone3.Created.Add(time.Hour)
	if cp.Equal(clone3) {
		t.Error("Equal ignored a Created edit")
	}

	var nilCP *CoreProperties
	if cp.Equal(nil) {
		t.Error("Equal(nil) = true")
	}
	if nilCP.Clone() != nil {
		t.Error("nil Clone() != nil")
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
