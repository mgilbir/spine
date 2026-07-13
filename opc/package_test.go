package opc

import (
	"strings"
	"testing"
	"time"
)

func TestCoreProperties_Marshal(t *testing.T) {
	created := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	modified := time.Date(2024, 1, 16, 14, 45, 0, 0, time.UTC)

	cp := &CoreProperties{
		Title:          "Test Presentation",
		Subject:        "Testing",
		Creator:        "Test Author",
		Keywords:       "test, opc, spine",
		Description:    "A test document",
		LastModifiedBy: "Another Author",
		Revision:       "1",
		Created:        created,
		Modified:       modified,
		Category:       "Test Category",
		ContentStatus:  "Draft",
		Language:       "en-US",
	}

	data, err := cp.Marshal()
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	xmlStr := string(data)

	// Check XML declaration
	if !strings.HasPrefix(xmlStr, "<?xml") {
		t.Error("Missing XML declaration")
	}

	// Check root element with proper namespace prefix
	if !strings.Contains(xmlStr, "<cp:coreProperties") {
		t.Error("Missing cp:coreProperties root element")
	}

	// Check namespace declarations
	requiredNamespaces := []string{
		`xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties"`,
		`xmlns:dc="http://purl.org/dc/elements/1.1/"`,
		`xmlns:dcterms="http://purl.org/dc/terms/"`,
		`xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"`,
	}
	for _, ns := range requiredNamespaces {
		if !strings.Contains(xmlStr, ns) {
			t.Errorf("Missing namespace declaration: %s", ns)
		}
	}

	// Check properties are present
	checks := []struct {
		name  string
		value string
	}{
		{"title", "Test Presentation"},
		{"creator", "Test Author"},
		{"subject", "Testing"},
		{"keywords", "test, opc, spine"},
		{"description", "A test document"},
		{"lastModifiedBy", "Another Author"},
		{"revision", "1"},
		{"category", "Test Category"},
		{"contentStatus", "Draft"},
		{"language", "en-US"},
	}

	for _, check := range checks {
		if !strings.Contains(xmlStr, check.value) {
			t.Errorf("Missing %s value: %s", check.name, check.value)
		}
	}

	// Check dates are in RFC3339 format
	if !strings.Contains(xmlStr, "2024-01-15T10:30:00Z") {
		t.Error("Created date not in expected format")
	}
	if !strings.Contains(xmlStr, "2024-01-16T14:45:00Z") {
		t.Error("Modified date not in expected format")
	}

	// Check elements use correct namespace prefixes
	if !strings.Contains(xmlStr, "<dc:title>") {
		t.Error("dc:title element missing")
	}
	if !strings.Contains(xmlStr, "<dc:creator>") {
		t.Error("dc:creator element missing")
	}
	if !strings.Contains(xmlStr, "<cp:keywords>") {
		t.Error("cp:keywords element missing")
	}
	if !strings.Contains(xmlStr, "<dcterms:created") {
		t.Error("dcterms:created element missing")
	}
	if !strings.Contains(xmlStr, `xsi:type="dcterms:W3CDTF"`) {
		t.Error("xsi:type attribute missing on date elements")
	}

	// Verify it can be unmarshaled back
	parsed, err := UnmarshalCoreProperties(data)
	if err != nil {
		t.Errorf("Output cannot be parsed back: %v", err)
	}
	if parsed.Title != cp.Title {
		t.Errorf("Round-trip Title = %q, want %q", parsed.Title, cp.Title)
	}
}

func TestCoreProperties_Marshal_EmptyDates(t *testing.T) {
	cp := &CoreProperties{
		Title:   "Test",
		Creator: "Author",
		// Created and Modified are zero values
	}

	data, err := cp.Marshal()
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	xmlStr := string(data)

	// Should not contain dcterms:created or dcterms:modified when dates are zero
	if strings.Contains(xmlStr, "<dcterms:created") && strings.Contains(xmlStr, "0001-01-01") {
		t.Error("Should not include zero date for created")
	}
}

func TestUnmarshalCoreProperties(t *testing.T) {
	xmlData := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<coreProperties>
  <title>Test Presentation</title>
  <subject>Testing</subject>
  <creator>Test Author</creator>
  <keywords>test, opc</keywords>
  <description>A test document</description>
  <lastModifiedBy>Another Author</lastModifiedBy>
  <revision>2</revision>
  <created type="dcterms:W3CDTF">2024-01-15T10:30:00Z</created>
  <modified type="dcterms:W3CDTF">2024-01-16T14:45:00Z</modified>
  <category>Test Category</category>
  <contentStatus>Final</contentStatus>
  <language>en-US</language>
</coreProperties>`)

	cp, err := UnmarshalCoreProperties(xmlData)
	if err != nil {
		t.Fatalf("UnmarshalCoreProperties() error = %v", err)
	}

	checks := []struct {
		name string
		got  string
		want string
	}{
		{"Title", cp.Title, "Test Presentation"},
		{"Subject", cp.Subject, "Testing"},
		{"Creator", cp.Creator, "Test Author"},
		{"Keywords", cp.Keywords, "test, opc"},
		{"Description", cp.Description, "A test document"},
		{"LastModifiedBy", cp.LastModifiedBy, "Another Author"},
		{"Revision", cp.Revision, "2"},
		{"Category", cp.Category, "Test Category"},
		{"ContentStatus", cp.ContentStatus, "Final"},
		{"Language", cp.Language, "en-US"},
	}

	for _, check := range checks {
		if check.got != check.want {
			t.Errorf("%s = %q, want %q", check.name, check.got, check.want)
		}
	}

	// Check dates
	expectedCreated := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	if !cp.Created.Equal(expectedCreated) {
		t.Errorf("Created = %v, want %v", cp.Created, expectedCreated)
	}

	expectedModified := time.Date(2024, 1, 16, 14, 45, 0, 0, time.UTC)
	if !cp.Modified.Equal(expectedModified) {
		t.Errorf("Modified = %v, want %v", cp.Modified, expectedModified)
	}
}

func TestCoreProperties_MarshalUnmarshal_RoundTrip(t *testing.T) {
	original := &CoreProperties{
		Title:          "Round Trip Test",
		Subject:        "Testing",
		Creator:        "Test Author",
		Keywords:       "test, round, trip",
		Description:    "Testing round trip",
		LastModifiedBy: "Another Author",
		Revision:       "5",
		Created:        time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC),
		Modified:       time.Date(2024, 6, 16, 12, 0, 0, 0, time.UTC),
		Category:       "Testing",
		ContentStatus:  "Draft",
		Language:       "en-GB",
	}

	data, err := original.Marshal()
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	parsed, err := UnmarshalCoreProperties(data)
	if err != nil {
		t.Fatalf("UnmarshalCoreProperties() error = %v", err)
	}

	checks := []struct {
		name string
		got  string
		want string
	}{
		{"Title", parsed.Title, original.Title},
		{"Subject", parsed.Subject, original.Subject},
		{"Creator", parsed.Creator, original.Creator},
		{"Keywords", parsed.Keywords, original.Keywords},
		{"Description", parsed.Description, original.Description},
		{"LastModifiedBy", parsed.LastModifiedBy, original.LastModifiedBy},
		{"Revision", parsed.Revision, original.Revision},
		{"Category", parsed.Category, original.Category},
		{"ContentStatus", parsed.ContentStatus, original.ContentStatus},
		{"Language", parsed.Language, original.Language},
	}

	for _, check := range checks {
		if check.got != check.want {
			t.Errorf("Round trip %s = %q, want %q", check.name, check.got, check.want)
		}
	}

	if !parsed.Created.Equal(original.Created) {
		t.Errorf("Round trip Created = %v, want %v", parsed.Created, original.Created)
	}
	if !parsed.Modified.Equal(original.Modified) {
		t.Errorf("Round trip Modified = %v, want %v", parsed.Modified, original.Modified)
	}
}

func TestUnmarshalCoreProperties_InvalidXML(t *testing.T) {
	invalidXML := []byte(`not valid xml`)
	_, err := UnmarshalCoreProperties(invalidXML)
	if err == nil {
		t.Error("UnmarshalCoreProperties() should return error for invalid XML")
	}
}

func TestUnmarshalCoreProperties_EmptyProperties(t *testing.T) {
	xmlData := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<coreProperties>
</coreProperties>`)

	cp, err := UnmarshalCoreProperties(xmlData)
	if err != nil {
		t.Fatalf("UnmarshalCoreProperties() error = %v", err)
	}

	if cp.Title != "" {
		t.Errorf("Title = %q, want empty", cp.Title)
	}
	if !cp.Created.IsZero() {
		t.Errorf("Created should be zero, got %v", cp.Created)
	}
}

func TestUnmarshalCoreProperties_InvalidDate(t *testing.T) {
	xmlData := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<coreProperties>
  <created type="dcterms:W3CDTF">not-a-date</created>
</coreProperties>`)

	cp, err := UnmarshalCoreProperties(xmlData)
	if err != nil {
		t.Fatalf("UnmarshalCoreProperties() error = %v", err)
	}

	// Invalid date should result in zero time, not an error
	if !cp.Created.IsZero() {
		t.Errorf("Created should be zero for invalid date, got %v", cp.Created)
	}
}

// TestUnmarshalCoreProperties_ForeignNamespaceNotLaundered verifies that an
// element in an unknown namespace whose local name collides with a standard
// property is neither captured into the typed field nor re-emitted as the
// genuine dc/cp element (C184: metadata laundering). Since C48 the element is
// preserved verbatim under its own namespace instead of being dropped; the
// prefix declaration it inherited from the source root is injected inline so
// the regenerated document stays namespace-well-formed.
func TestUnmarshalCoreProperties_ForeignNamespaceNotLaundered(t *testing.T) {
	src := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties"` +
		` xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:evil="urn:evil">` +
		`<evil:creator>Mallory</evil:creator>` +
		`<dc:title>Real Title</dc:title>` +
		`</cp:coreProperties>`

	cp, err := UnmarshalCoreProperties([]byte(src))
	if err != nil {
		t.Fatalf("UnmarshalCoreProperties() error = %v", err)
	}
	if cp.Creator != "" {
		t.Errorf("foreign-namespace element captured into Creator = %q, want empty", cp.Creator)
	}
	if cp.Title != "Real Title" {
		t.Errorf("Title = %q, want %q", cp.Title, "Real Title")
	}

	out, err := cp.Marshal()
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if strings.Contains(string(out), "<dc:creator>") {
		t.Errorf("foreign-namespace element re-emitted as genuine <dc:creator>:\n%s", out)
	}
	// Preserved under its own namespace, made self-contained (C48).
	want := `<evil:creator xmlns:evil="urn:evil">Mallory</evil:creator>`
	if !strings.Contains(string(out), want) {
		t.Errorf("foreign-namespace element not preserved verbatim; want %s in:\n%s", want, out)
	}
}

// TestUnmarshalCoreProperties_LegacyNoNamespace verifies that the legacy
// local-name mapping still works for elements with no namespace at all
// (C184: only unknown namespaces lose the fallback).
func TestUnmarshalCoreProperties_LegacyNoNamespace(t *testing.T) {
	src := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<coreProperties>` +
		`<creator>Alice</creator>` +
		`<title>Legacy Title</title>` +
		`</coreProperties>`

	cp, err := UnmarshalCoreProperties([]byte(src))
	if err != nil {
		t.Fatalf("UnmarshalCoreProperties() error = %v", err)
	}
	if cp.Creator != "Alice" {
		t.Errorf("legacy Creator = %q, want %q", cp.Creator, "Alice")
	}
	if cp.Title != "Legacy Title" {
		t.Errorf("legacy Title = %q, want %q", cp.Title, "Legacy Title")
	}

	out, err := cp.Marshal()
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if !strings.Contains(string(out), "<dc:creator>Alice</dc:creator>") {
		t.Errorf("legacy creator not re-emitted as dc:creator:\n%s", out)
	}
}
