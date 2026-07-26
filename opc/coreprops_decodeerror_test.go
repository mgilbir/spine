package opc

import (
	"strings"
	"testing"
)

// TestCoreProperties_MalformedChildSurfacesError guards C344: a core property
// child whose content fails DecodeElement (here a raw, unescaped ampersand in
// dc:title) must surface an error rather than being silently swallowed. The
// previous code used `if err := DecodeElement(...); err == nil` and dropped the
// error, leaving the field unset while still recording the key in
// elementOrder/presentFields — so a later regenerate emitted a phantom
// present-but-empty <dc:title></dc:title> with no diagnostic.
func TestCoreProperties_MalformedChildSurfacesError(t *testing.T) {
	src := []byte(
		`<cp:coreProperties xmlns:cp="` + nsCoreProperties + `" xmlns:dc="` + nsDublinCore + `">` +
			`<dc:title>a & b</dc:title>` +
			`</cp:coreProperties>`)

	cp, err := UnmarshalCoreProperties(src)
	if err == nil {
		t.Fatalf("expected error for malformed dc:title, got nil (cp=%+v)", cp)
	}
	if !strings.Contains(err.Error(), "dc:title") {
		t.Errorf("error should identify the failing property; got %v", err)
	}
}

// TestCoreProperties_WellFormedStillParses confirms the error-surfacing change
// does not regress parsing of a normal, well-formed core.xml.
func TestCoreProperties_WellFormedStillParses(t *testing.T) {
	src := []byte(
		`<cp:coreProperties xmlns:cp="` + nsCoreProperties + `" xmlns:dc="` + nsDublinCore +
			`" xmlns:dcterms="` + nsDcTerms + `">` +
			`<dc:title>Title</dc:title>` +
			`<dc:creator>Author</dc:creator>` +
			`<cp:revision>3</cp:revision>` +
			`<dcterms:created>2024-01-15T10:30:00Z</dcterms:created>` +
			`</cp:coreProperties>`)

	cp, err := UnmarshalCoreProperties(src)
	if err != nil {
		t.Fatalf("UnmarshalCoreProperties() error = %v", err)
	}
	if cp.Title != "Title" {
		t.Errorf("Title = %q, want %q", cp.Title, "Title")
	}
	if cp.Creator != "Author" {
		t.Errorf("Creator = %q, want %q", cp.Creator, "Author")
	}
	if cp.Revision != "3" {
		t.Errorf("Revision = %q, want %q", cp.Revision, "3")
	}

	out, err := cp.Marshal()
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if !strings.Contains(string(out), `<dc:title>Title</dc:title>`) {
		t.Errorf("Marshal output missing dc:title; got:\n%s", out)
	}
}
