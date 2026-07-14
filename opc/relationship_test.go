package opc

import (
	"encoding/xml"
	"strings"
	"testing"
)

func TestRelationship_IsExternal(t *testing.T) {
	tests := []struct {
		name       string
		targetMode TargetMode
		want       bool
	}{
		{name: "internal", targetMode: TargetModeInternal, want: false},
		{name: "external", targetMode: TargetModeExternal, want: true},
		{name: "empty (defaults to internal)", targetMode: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &Relationship{TargetMode: tt.targetMode}
			if got := r.IsExternal(); got != tt.want {
				t.Errorf("Relationship.IsExternal() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMarshalRelationships(t *testing.T) {
	rels := []*Relationship{
		{
			ID:         "rId1",
			Type:       RelTypeOfficeDocument,
			Target:     "ppt/presentation.xml",
			TargetMode: TargetModeInternal,
		},
		{
			ID:         "rId2",
			Type:       RelTypeCore,
			Target:     "docProps/core.xml",
			TargetMode: TargetModeInternal,
		},
		{
			ID:         "rId3",
			Type:       "http://example.com/external",
			Target:     "https://example.com/resource",
			TargetMode: TargetModeExternal,
		},
	}

	data, err := MarshalRelationships(rels)
	if err != nil {
		t.Fatalf("MarshalRelationships() error = %v", err)
	}

	// Verify XML structure
	xmlStr := string(data)

	// Check XML declaration
	if !strings.HasPrefix(xmlStr, "<?xml") {
		t.Error("Missing XML declaration")
	}

	// Check namespace
	if !strings.Contains(xmlStr, RelationshipsNamespace) {
		t.Error("Missing relationships namespace")
	}

	// Check relationships are present
	if !strings.Contains(xmlStr, `Id="rId1"`) {
		t.Error("Missing rId1 relationship")
	}
	if !strings.Contains(xmlStr, `Id="rId2"`) {
		t.Error("Missing rId2 relationship")
	}
	if !strings.Contains(xmlStr, `Id="rId3"`) {
		t.Error("Missing rId3 relationship")
	}

	// Check external target mode is present
	if !strings.Contains(xmlStr, `TargetMode="External"`) {
		t.Error("Missing TargetMode for external relationship")
	}

	// Verify it's valid XML by parsing it back
	var parsed relationshipsXML
	if err := xml.Unmarshal(data, &parsed); err != nil {
		t.Errorf("Output is not valid XML: %v", err)
	}
}

func TestUnmarshalRelationships(t *testing.T) {
	xmlData := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
  <Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/>
  <Relationship Id="rId2" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="docProps/core.xml"/>
  <Relationship Id="rId3" Type="http://example.com/external" Target="https://example.com" TargetMode="External"/>
</Relationships>`)

	rels, err := UnmarshalRelationships(xmlData)
	if err != nil {
		t.Fatalf("UnmarshalRelationships() error = %v", err)
	}

	if len(rels) != 3 {
		t.Fatalf("UnmarshalRelationships() returned %d relationships, want 3", len(rels))
	}

	// Check first relationship
	if rels[0].ID != "rId1" {
		t.Errorf("rels[0].ID = %q, want %q", rels[0].ID, "rId1")
	}
	if rels[0].Type != RelTypeOfficeDocument {
		t.Errorf("rels[0].Type = %q, want %q", rels[0].Type, RelTypeOfficeDocument)
	}
	if rels[0].Target != "ppt/presentation.xml" {
		t.Errorf("rels[0].Target = %q, want %q", rels[0].Target, "ppt/presentation.xml")
	}
	if rels[0].TargetMode != TargetModeInternal {
		t.Errorf("rels[0].TargetMode = %q, want %q", rels[0].TargetMode, TargetModeInternal)
	}

	// Check external relationship
	if rels[2].TargetMode != TargetModeExternal {
		t.Errorf("rels[2].TargetMode = %q, want %q", rels[2].TargetMode, TargetModeExternal)
	}
}

func TestMarshalUnmarshalRelationships_RoundTrip(t *testing.T) {
	original := []*Relationship{
		{ID: "rId1", Type: RelTypeOfficeDocument, Target: "ppt/presentation.xml", TargetMode: TargetModeInternal},
		{ID: "rId2", Type: RelTypeCore, Target: "docProps/core.xml", TargetMode: TargetModeInternal},
		{ID: "rId3", Type: "http://ext", Target: "https://example.com", TargetMode: TargetModeExternal},
	}

	data, err := MarshalRelationships(original)
	if err != nil {
		t.Fatalf("MarshalRelationships() error = %v", err)
	}

	parsed, err := UnmarshalRelationships(data)
	if err != nil {
		t.Fatalf("UnmarshalRelationships() error = %v", err)
	}

	if len(parsed) != len(original) {
		t.Fatalf("Round trip changed count: got %d, want %d", len(parsed), len(original))
	}

	for i := range original {
		if parsed[i].ID != original[i].ID {
			t.Errorf("rels[%d].ID = %q, want %q", i, parsed[i].ID, original[i].ID)
		}
		if parsed[i].Type != original[i].Type {
			t.Errorf("rels[%d].Type = %q, want %q", i, parsed[i].Type, original[i].Type)
		}
		if parsed[i].Target != original[i].Target {
			t.Errorf("rels[%d].Target = %q, want %q", i, parsed[i].Target, original[i].Target)
		}
		if parsed[i].TargetMode != original[i].TargetMode {
			t.Errorf("rels[%d].TargetMode = %q, want %q", i, parsed[i].TargetMode, original[i].TargetMode)
		}
	}
}

func TestGetRelationshipsPartName(t *testing.T) {
	tests := []struct {
		name     string
		partName string
		want     string
	}{
		{
			name:     "package root empty",
			partName: "",
			want:     "/_rels/.rels",
		},
		{
			name:     "package root slash",
			partName: "/",
			want:     "/_rels/.rels",
		},
		{
			name:     "presentation part",
			partName: "/ppt/presentation.xml",
			want:     "/ppt/_rels/presentation.xml.rels",
		},
		{
			name:     "slide part",
			partName: "/ppt/slides/slide1.xml",
			want:     "/ppt/slides/_rels/slide1.xml.rels",
		},
		{
			name:     "root level part",
			partName: "/document.xml",
			want:     "/_rels/document.xml.rels",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetRelationshipsPartName(tt.partName)
			if got != tt.want {
				t.Errorf("GetRelationshipsPartName(%q) = %q, want %q", tt.partName, got, tt.want)
			}
		})
	}
}

func TestUnmarshalRelationships_InvalidXML(t *testing.T) {
	invalidXML := []byte(`not valid xml`)
	_, err := UnmarshalRelationships(invalidXML)
	if err == nil {
		t.Error("UnmarshalRelationships() should return error for invalid XML")
	}
}

func TestUnmarshalRelationships_EmptyRelationships(t *testing.T) {
	xmlData := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">
</Relationships>`)

	rels, err := UnmarshalRelationships(xmlData)
	if err != nil {
		t.Fatalf("UnmarshalRelationships() error = %v", err)
	}

	if len(rels) != 0 {
		t.Errorf("UnmarshalRelationships() returned %d relationships, want 0", len(rels))
	}
}

// TestMarshalRelationships_AttrEscaping verifies attributes are escaped with
// the project-wide attribute policy (C205): quotes become &quot;, ampersands
// &amp;, and apostrophes stay literal (xml.EscapeText would emit &#39;).
func TestMarshalRelationships_AttrEscaping(t *testing.T) {
	rels := []*Relationship{{
		ID:         "rId1",
		Type:       RelTypeHyperlink,
		Target:     `https://example.com/a?b=1&c="x"&d='y'`,
		TargetMode: TargetModeExternal,
	}}

	data, err := MarshalRelationships(rels)
	if err != nil {
		t.Fatalf("MarshalRelationships() error = %v", err)
	}
	out := string(data)
	want := `Target="https://example.com/a?b=1&amp;c=&quot;x&quot;&amp;d='y'"`
	if !strings.Contains(out, want) {
		t.Errorf("escaped Target attribute not found:\nwant substring: %s\ngot: %s", want, out)
	}
	if strings.Contains(out, "&#39;") || strings.Contains(out, "&#34;") {
		t.Errorf("xml.EscapeText-style numeric references present, want policy escaping: %s", out)
	}

	// The escaped output must still parse back to the original target.
	parsed, err := UnmarshalRelationships(data)
	if err != nil {
		t.Fatalf("UnmarshalRelationships() error = %v", err)
	}
	if len(parsed) != 1 || parsed[0].Target != rels[0].Target {
		t.Errorf("round-trip Target = %+v, want %q", parsed, rels[0].Target)
	}
}

// TestRelationshipsEquivalent verifies the order-insensitive set comparison
// used to decide whether source .rels bytes may be preserved verbatim.
func TestRelationshipsEquivalent(t *testing.T) {
	a := []*Relationship{
		{ID: "rId1", Type: RelTypeWorksheet, Target: "worksheets/sheet1.xml", TargetMode: TargetModeInternal},
		{ID: "rId2", Type: RelTypeStyles, Target: "styles.xml", TargetMode: TargetModeInternal},
	}
	reordered := []*Relationship{a[1], a[0]}
	if !RelationshipsEquivalent(a, reordered) {
		t.Error("RelationshipsEquivalent(reordered) = false, want true")
	}
	if RelationshipsEqual(a, reordered) {
		t.Error("RelationshipsEqual(reordered) = true, want false (order-sensitive)")
	}

	changedTarget := []*Relationship{a[0], {ID: "rId2", Type: RelTypeStyles, Target: "styles2.xml", TargetMode: TargetModeInternal}}
	if RelationshipsEquivalent(a, changedTarget) {
		t.Error("RelationshipsEquivalent(changed target) = true, want false")
	}

	missing := []*Relationship{a[0]}
	if RelationshipsEquivalent(a, missing) {
		t.Error("RelationshipsEquivalent(different length) = true, want false")
	}

	// Duplicate IDs make the comparison conservatively fail.
	dup := []*Relationship{a[0], a[0]}
	if RelationshipsEquivalent(dup, dup) {
		t.Error("RelationshipsEquivalent(duplicate ids) = true, want false")
	}
}
