package opc

import (
	"strings"
	"testing"
)

func TestNewContentTypes(t *testing.T) {
	ct := NewContentTypes()

	if ct.Defaults == nil {
		t.Error("NewContentTypes() Defaults is nil")
	}
	if ct.Overrides == nil {
		t.Error("NewContentTypes() Overrides is nil")
	}

	// Check default mappings exist
	expectedDefaults := map[string]string{
		"rels": ContentTypeRelationships,
		"xml":  "application/xml",
		"png":  ContentTypePNG,
		"jpeg": ContentTypeJPEG,
	}

	for ext, contentType := range expectedDefaults {
		if ct.Defaults[ext] != contentType {
			t.Errorf("Default for %q = %q, want %q", ext, ct.Defaults[ext], contentType)
		}
	}
}

func TestContentTypes_GetContentType(t *testing.T) {
	ct := NewContentTypes()
	ct.SetOverride("/ppt/presentation.xml", ContentTypePresentationMain)

	tests := []struct {
		name     string
		partName string
		want     string
	}{
		{
			name:     "override takes precedence",
			partName: "/ppt/presentation.xml",
			want:     ContentTypePresentationMain,
		},
		{
			name:     "default by extension",
			partName: "/ppt/slides/slide1.xml",
			want:     "application/xml",
		},
		{
			name:     "rels extension",
			partName: "/_rels/.rels",
			want:     ContentTypeRelationships,
		},
		{
			name:     "png image",
			partName: "/ppt/media/image1.png",
			want:     ContentTypePNG,
		},
		{
			name:     "case insensitive extension",
			partName: "/ppt/media/image1.PNG",
			want:     ContentTypePNG,
		},
		{
			name:     "unknown extension",
			partName: "/unknown.xyz",
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ct.GetContentType(tt.partName)
			if got != tt.want {
				t.Errorf("GetContentType(%q) = %q, want %q", tt.partName, got, tt.want)
			}
		})
	}
}

func TestContentTypes_SetDefault(t *testing.T) {
	ct := NewContentTypes()

	ct.SetDefault("docx", "application/vnd.openxmlformats-officedocument.wordprocessingml.document")
	ct.SetDefault(".pptx", "application/vnd.openxmlformats-officedocument.presentationml.presentation")
	ct.SetDefault("XLSX", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")

	// All should be stored lowercase without leading dot
	if _, ok := ct.Defaults["docx"]; !ok {
		t.Error("SetDefault did not store 'docx'")
	}
	if _, ok := ct.Defaults["pptx"]; !ok {
		t.Error("SetDefault did not store 'pptx' (should strip dot)")
	}
	if _, ok := ct.Defaults["xlsx"]; !ok {
		t.Error("SetDefault did not store 'xlsx' (should lowercase)")
	}
}

func TestContentTypes_SetOverride(t *testing.T) {
	ct := NewContentTypes()

	ct.SetOverride("/ppt/presentation.xml", ContentTypePresentationMain)
	ct.SetOverride("/ppt/slides/slide1.xml", ContentTypeSlide)

	if ct.Overrides["/ppt/presentation.xml"] != ContentTypePresentationMain {
		t.Error("SetOverride did not store presentation content type")
	}
	if ct.Overrides["/ppt/slides/slide1.xml"] != ContentTypeSlide {
		t.Error("SetOverride did not store slide content type")
	}
}

func TestContentTypes_Marshal(t *testing.T) {
	ct := NewContentTypes()
	ct.SetOverride("/ppt/presentation.xml", ContentTypePresentationMain)
	ct.SetOverride("/ppt/slides/slide1.xml", ContentTypeSlide)

	data, err := ct.Marshal()
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	xmlStr := string(data)

	// Check XML declaration
	if !strings.HasPrefix(xmlStr, "<?xml") {
		t.Error("Missing XML declaration")
	}

	// Check namespace
	if !strings.Contains(xmlStr, ContentTypesNamespace) {
		t.Error("Missing content types namespace")
	}

	// Check defaults are present
	if !strings.Contains(xmlStr, `Extension="rels"`) {
		t.Error("Missing rels default")
	}
	if !strings.Contains(xmlStr, `Extension="xml"`) {
		t.Error("Missing xml default")
	}

	// Check overrides are present
	if !strings.Contains(xmlStr, `PartName="/ppt/presentation.xml"`) {
		t.Error("Missing presentation override")
	}
	if !strings.Contains(xmlStr, `PartName="/ppt/slides/slide1.xml"`) {
		t.Error("Missing slide override")
	}

	// Verify it's valid XML by re-parsing it
	if _, err := UnmarshalContentTypes(data); err != nil {
		t.Errorf("Output is not valid XML: %v", err)
	}
}

func TestUnmarshalContentTypes(t *testing.T) {
	xmlData := []byte(`<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>
  <Default Extension="xml" ContentType="application/xml"/>
  <Default Extension="PNG" ContentType="image/png"/>
  <Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/>
  <Override PartName="/ppt/slides/slide1.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.slide+xml"/>
</Types>`)

	ct, err := UnmarshalContentTypes(xmlData)
	if err != nil {
		t.Fatalf("UnmarshalContentTypes() error = %v", err)
	}

	// Check defaults (should be lowercase)
	if ct.Defaults["rels"] != ContentTypeRelationships {
		t.Errorf("Defaults[rels] = %q, want %q", ct.Defaults["rels"], ContentTypeRelationships)
	}
	if ct.Defaults["xml"] != "application/xml" {
		t.Errorf("Defaults[xml] = %q, want %q", ct.Defaults["xml"], "application/xml")
	}
	if ct.Defaults["png"] != ContentTypePNG {
		t.Errorf("Defaults[png] = %q, want %q (should lowercase extension)", ct.Defaults["png"], ContentTypePNG)
	}

	// Check overrides
	if ct.Overrides["/ppt/presentation.xml"] != ContentTypePresentationMain {
		t.Errorf("Overrides[presentation] = %q, want %q", ct.Overrides["/ppt/presentation.xml"], ContentTypePresentationMain)
	}
	if ct.Overrides["/ppt/slides/slide1.xml"] != ContentTypeSlide {
		t.Errorf("Overrides[slide] = %q, want %q", ct.Overrides["/ppt/slides/slide1.xml"], ContentTypeSlide)
	}
}

func TestContentTypes_MarshalUnmarshal_RoundTrip(t *testing.T) {
	original := NewContentTypes()
	original.SetDefault("docx", "application/docx")
	original.SetOverride("/doc/document.xml", "application/doc")
	original.SetOverride("/doc/styles.xml", "application/styles")

	data, err := original.Marshal()
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}

	parsed, err := UnmarshalContentTypes(data)
	if err != nil {
		t.Fatalf("UnmarshalContentTypes() error = %v", err)
	}

	// Compare defaults
	for ext, contentType := range original.Defaults {
		if parsed.Defaults[ext] != contentType {
			t.Errorf("Round trip: Defaults[%q] = %q, want %q", ext, parsed.Defaults[ext], contentType)
		}
	}

	// Compare overrides
	for partName, contentType := range original.Overrides {
		if parsed.Overrides[partName] != contentType {
			t.Errorf("Round trip: Overrides[%q] = %q, want %q", partName, parsed.Overrides[partName], contentType)
		}
	}
}

func TestUnmarshalContentTypes_InvalidXML(t *testing.T) {
	invalidXML := []byte(`not valid xml`)
	_, err := UnmarshalContentTypes(invalidXML)
	if err == nil {
		t.Error("UnmarshalContentTypes() should return error for invalid XML")
	}
}

func TestContentTypeConstants(t *testing.T) {
	// Verify content type constants are valid MIME types
	contentTypes := []string{
		ContentTypeRelationships,
		ContentTypeCoreProps,
		ContentTypePresentationMain,
		ContentTypeSlide,
		ContentTypeSlideLayout,
		ContentTypeSlideMaster,
		ContentTypeTheme,
		ContentTypeWorkbook,
		ContentTypeWorksheet,
		ContentTypeDocument,
		ContentTypePNG,
		ContentTypeJPEG,
	}

	for _, ct := range contentTypes {
		if ct == "" {
			t.Error("Content type constant is empty")
		}
		if !strings.Contains(ct, "/") {
			t.Errorf("Content type %q does not contain '/'", ct)
		}
	}
}

// TestContentTypes_CaseVariantDuplicateOverridesStable verifies that when a
// hostile [Content_Types].xml carries two overrides differing only in case,
// the case-insensitive fallback deterministically returns the first-declared
// one on every call instead of a per-call-random map-iteration winner (C211).
func TestContentTypes_CaseVariantDuplicateOverridesStable(t *testing.T) {
	data := []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
		`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
		`<Override PartName="/Part.xml" ContentType="application/first"/>` +
		`<Override PartName="/part.XML" ContentType="application/second"/>` +
		`</Types>`)

	ct, err := UnmarshalContentTypes(data)
	if err != nil {
		t.Fatalf("UnmarshalContentTypes() error = %v", err)
	}

	// Query with a third casing so neither exact-match entry hits.
	for i := 0; i < 100; i++ {
		if got := ct.GetContentType("/PART.xml"); got != "application/first" {
			t.Fatalf("GetContentType iteration %d = %q, want stable first-declared %q", i, got, "application/first")
		}
	}
}

// TestContentTypes_Clone verifies that Clone produces an independent deep
// copy: mutations of the clone are invisible to the original and vice versa
// (C53).
func TestContentTypes_Clone(t *testing.T) {
	data := []byte(`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n" +
		`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
		`<Default Extension="JPG" ContentType="image/jpeg"/>` +
		`<Override PartName="/a.xml" ContentType="application/a"/>` +
		`</Types>`)

	ct, err := UnmarshalContentTypes(data)
	if err != nil {
		t.Fatalf("UnmarshalContentTypes() error = %v", err)
	}

	clone := ct.Clone()

	origOut, err := ct.Marshal()
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

	clone.SetOverride("/b.xml", "application/b")
	clone.SetDefault("png", "image/png")
	if _, ok := ct.Overrides["/b.xml"]; ok {
		t.Error("clone SetOverride mutated original Overrides")
	}
	if _, ok := ct.Defaults["png"]; ok {
		t.Error("clone SetDefault mutated original Defaults")
	}
	if len(ct.overrideOrder) != 1 {
		t.Errorf("original overrideOrder len = %d, want 1", len(ct.overrideOrder))
	}

	ct.SetOverride("/c.xml", "application/c")
	if _, ok := clone.Overrides["/c.xml"]; ok {
		t.Error("original SetOverride mutated clone Overrides")
	}

	// Original-cased extension survives the clone (JPG, not jpg).
	if got := clone.displayExtension("jpg"); got != "JPG" {
		t.Errorf("clone displayExtension = %q, want %q", got, "JPG")
	}

	var nilCT *ContentTypes
	if nilCT.Clone() != nil {
		t.Error("nil Clone() != nil")
	}
}

// A pretty-printed [Content_Types].xml round-trips byte-identically: the
// verbatim root tag (extra xmlns declarations), each entry's leading
// whitespace and self-closing style (sources mix " />" and "/>"), and the
// whitespace before </Types> are all captured and replayed.
func TestContentTypes_ByteFaithfulFormatting(t *testing.T) {
	src := "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\r\n" +
		"<Types xmlns=\"http://schemas.openxmlformats.org/package/2006/content-types\" xmlns:xsd=\"http://www.w3.org/2001/XMLSchema\">\r\n" +
		"  <Default Extension=\"xml\" ContentType=\"application/xml\"/>\r\n" +
		"  <Override PartName=\"/xl/workbook.xml\" ContentType=\"application/vnd.test+xml\" />\r\n" +
		"</Types>\r\n"

	ct, err := UnmarshalContentTypes([]byte(src))
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out, err := ct.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(out) != src {
		t.Errorf("round-trip mismatch:\ngot  %q\nwant %q", out, src)
	}
}

// A trailing newline before </Types> in an otherwise compact file survives.
func TestContentTypes_TrailingNewlineBeforeClose(t *testing.T) {
	src := "<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n" +
		"<Types xmlns=\"http://schemas.openxmlformats.org/package/2006/content-types\">" +
		"<Default Extension=\"xml\" ContentType=\"application/xml\"/>\n</Types>"

	ct, err := UnmarshalContentTypes([]byte(src))
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	out, err := ct.Marshal()
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if string(out) != src {
		t.Errorf("round-trip mismatch:\ngot  %q\nwant %q", out, src)
	}
}
