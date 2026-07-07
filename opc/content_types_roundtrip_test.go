package opc

import (
	"strings"
	"testing"
)

// TestContentTypes_PreservesExtensionCase verifies that an uppercase Default
// extension round-trips with its original case rather than being folded to
// lowercase, while lookups remain case-insensitive (C6).
func TestContentTypes_PreservesExtensionCase(t *testing.T) {
	src := []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
		`<Types xmlns="` + ContentTypesNamespace + `">` +
		`<Default Extension="JPG" ContentType="image/jpeg"/>` +
		`<Override PartName="/xl/workbook.xml" ContentType="` + ContentTypeWorkbook + `"/>` +
		`</Types>`)

	ct, err := UnmarshalContentTypes(src)
	if err != nil {
		t.Fatalf("UnmarshalContentTypes() error = %v", err)
	}

	// Lookup is case-insensitive: both /a.JPG and /a.jpg resolve.
	if got := ct.GetContentType("/media/a.JPG"); got != "image/jpeg" {
		t.Errorf("GetContentType(.JPG) = %q, want image/jpeg", got)
	}
	if got := ct.GetContentType("/media/a.jpg"); got != "image/jpeg" {
		t.Errorf("GetContentType(.jpg) = %q, want image/jpeg", got)
	}

	out, err := ct.Marshal()
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if !strings.Contains(string(out), `Extension="JPG"`) {
		t.Errorf("expected original-case extension in output, got: %s", out)
	}
}

// TestContentTypes_PreservesXMLSeparator verifies that a bare-LF prolog
// separator round-trips instead of being rewritten to CRLF (C7).
func TestContentTypes_PreservesXMLSeparator(t *testing.T) {
	// Note the single "\n" between the declaration and <Types>.
	src := []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n" +
		`<Types xmlns="` + ContentTypesNamespace + `">` +
		`<Default Extension="xml" ContentType="application/xml"/>` +
		`</Types>`)

	ct, err := UnmarshalContentTypes(src)
	if err != nil {
		t.Fatalf("UnmarshalContentTypes() error = %v", err)
	}
	out, err := ct.Marshal()
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if string(out) != string(src) {
		t.Errorf("round-trip not byte-identical:\n got: %q\nwant: %q", out, src)
	}
}

// TestContentTypes_DefaultSeparatorIsCRLF verifies a from-scratch ContentTypes
// still emits the conventional CRLF prolog separator.
func TestContentTypes_DefaultSeparatorIsCRLF(t *testing.T) {
	ct := NewContentTypes()
	out, err := ct.Marshal()
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if !strings.HasPrefix(string(out), `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`+"\r\n<Types") {
		t.Errorf("expected CRLF prolog separator by default, got: %q", out)
	}
}
