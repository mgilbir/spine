package opc

import (
	"strings"
	"testing"
)

const customPropsRoot = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
	`<Properties xmlns="http://schemas.openxmlformats.org/officeDocument/2006/custom-properties" ` +
	`xmlns:vt="http://schemas.openxmlformats.org/officeDocument/2006/docPropsVTypes">` +
	`<property fmtid="{D5CDD505-2E9C-101B-9397-08002B2CF9AE}" pid="2" name="Probe">%s</property>` +
	`</Properties>`

func roundTripCustomProps(t *testing.T, child string) string {
	t.Helper()
	in := strings.Replace(customPropsRoot, "%s", child, 1)
	cp, err := UnmarshalCustomProperties([]byte(in))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	out, err := cp.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return string(out)
}

// A property value whose element is in some other namespace must stay in it.
//
// The variant scalar set is keyed on the local name alone — deliberately, since
// producers spell the variant namespace several ways — and the value was
// re-emitted by writing "vt:" in front of that local name. Together those moved
// the element into docPropsVTypes: an <evil:i4> came back as a <vt:i4>. That is
// inert markup, ignored by every reader, promoted to a live custom property by
// the act of opening and saving the file.
func TestUnparseableScalarKeepsItsNamespace(t *testing.T) {
	got := roundTripCustomProps(t,
		`<evil:i4 xmlns:evil="urn:example.invalid/other">not-a-number</evil:i4>`)

	if strings.Contains(got, "<vt:i4>") {
		t.Errorf("an element in another namespace was re-homed into docPropsVTypes:\n%s", got)
	}
	if !strings.Contains(got, `<evil:i4 xmlns:evil="urn:example.invalid/other">not-a-number</evil:i4>`) {
		t.Errorf("the source element was not preserved verbatim:\n%s", got)
	}
}

// The ordinary case is unchanged: a genuine vt: scalar whose text does not parse
// for its declared type is still preserved, still under vt:.
func TestUnparseableVTScalarStillPreserved(t *testing.T) {
	got := roundTripCustomProps(t, `<vt:i4>not-a-number</vt:i4>`)
	if !strings.Contains(got, `<vt:i4>not-a-number</vt:i4>`) {
		t.Errorf("a vt: scalar with unparseable text was not preserved:\n%s", got)
	}
}

// And a scalar that does parse is still modeled rather than preserved raw, so
// this changes nothing for the values the library actually understands.
func TestParseableScalarIsStillModeled(t *testing.T) {
	got := roundTripCustomProps(t, `<vt:i4>42</vt:i4>`)
	if !strings.Contains(got, `<vt:i4>42</vt:i4>`) {
		t.Errorf("a parseable scalar did not survive the round trip:\n%s", got)
	}
}
