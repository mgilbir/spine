package docprops

import (
	"encoding/xml"
	"testing"
)

func TestCoreProperties_RoundTrip(t *testing.T) {
	tests := []struct {
		name string
		xml  string
	}{
		{
			name: "basic core properties",
			xml: `<coreProperties xmlns="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:dcterms="http://purl.org/dc/terms/">
  <dc:title>Test Document</dc:title>
  <dc:creator>Test Author</dc:creator>
  <dc:description>A test document</dc:description>
</coreProperties>`,
		},
		{
			name: "with dates",
			xml: `<coreProperties xmlns="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:dcterms="http://purl.org/dc/terms/">
  <dcterms:created>2024-01-15T10:30:00Z</dcterms:created>
  <dcterms:modified>2024-01-16T14:00:00Z</dcterms:modified>
</coreProperties>`,
		},
		{
			name: "full properties",
			xml: `<coreProperties xmlns="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:dcterms="http://purl.org/dc/terms/">
  <dc:title>Full Document</dc:title>
  <dc:subject>Testing</dc:subject>
  <dc:creator>Author Name</dc:creator>
  <keywords>test,document,xml</keywords>
  <dc:description>Full description</dc:description>
  <lastModifiedBy>Editor Name</lastModifiedBy>
  <revision>5</revision>
  <dcterms:created>2024-01-01T00:00:00Z</dcterms:created>
  <dcterms:modified>2024-01-15T12:00:00Z</dcterms:modified>
  <dc:category>Test Category</dc:category>
  <dc:language>en-US</dc:language>
  <version>1.0</version>
</coreProperties>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cp CoreProperties
			if err := xml.Unmarshal([]byte(tt.xml), &cp); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			out, err := xml.Marshal(&cp)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var cp2 CoreProperties
			if err := xml.Unmarshal(out, &cp2); err != nil {
				t.Fatalf("Re-unmarshal failed: %v", err)
			}
		})
	}
}

func TestExtendedProperties_RoundTrip(t *testing.T) {
	tests := []struct {
		name string
		xml  string
	}{
		{
			name: "basic extended properties",
			xml: `<Properties xmlns="http://schemas.openxmlformats.org/officeDocument/2006/extended-properties">
  <Application>Microsoft Office PowerPoint</Application>
  <AppVersion>16.0000</AppVersion>
  <Company>Test Company</Company>
</Properties>`,
		},
		{
			name: "with statistics",
			xml: `<Properties xmlns="http://schemas.openxmlformats.org/officeDocument/2006/extended-properties">
  <Application>Microsoft Office Word</Application>
  <Template>Normal.dotm</Template>
  <TotalTime>120</TotalTime>
  <Pages>10</Pages>
  <Words>2500</Words>
  <Characters>15000</Characters>
  <CharactersWithSpaces>17500</CharactersWithSpaces>
  <Lines>150</Lines>
  <Paragraphs>30</Paragraphs>
</Properties>`,
		},
		{
			name: "presentation properties",
			xml: `<Properties xmlns="http://schemas.openxmlformats.org/officeDocument/2006/extended-properties">
  <Application>Microsoft Office PowerPoint</Application>
  <PresentationFormat>On-screen Show (4:3)</PresentationFormat>
  <Slides>15</Slides>
  <Notes>10</Notes>
  <HiddenSlides>2</HiddenSlides>
  <MMClips>3</MMClips>
</Properties>`,
		},
		{
			name: "spreadsheet properties",
			xml: `<Properties xmlns="http://schemas.openxmlformats.org/officeDocument/2006/extended-properties">
  <Application>Microsoft Office Excel</Application>
  <Worksheets>5</Worksheets>
</Properties>`,
		},
		{
			name: "with security",
			xml: `<Properties xmlns="http://schemas.openxmlformats.org/officeDocument/2006/extended-properties">
  <Application>Microsoft Office Word</Application>
  <DocSecurity>4</DocSecurity>
  <ScaleCrop>false</ScaleCrop>
  <LinksUpToDate>false</LinksUpToDate>
  <SharedDoc>false</SharedDoc>
  <HyperlinksChanged>false</HyperlinksChanged>
</Properties>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ep ExtendedProperties
			if err := xml.Unmarshal([]byte(tt.xml), &ep); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			out, err := xml.Marshal(&ep)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var ep2 ExtendedProperties
			if err := xml.Unmarshal(out, &ep2); err != nil {
				t.Fatalf("Re-unmarshal failed: %v", err)
			}
		})
	}
}

func TestVectorVariant_RoundTrip(t *testing.T) {
	tests := []struct {
		name string
		xml  string
	}{
		{
			name: "heading pairs vector",
			xml: `<HeadingPairs xmlns="http://schemas.openxmlformats.org/officeDocument/2006/extended-properties" xmlns:vt="http://schemas.openxmlformats.org/officeDocument/2006/docPropsVTypes">
  <vt:vector size="4" baseType="variant">
    <vt:variant><vt:lpstr>Theme</vt:lpstr></vt:variant>
    <vt:variant><vt:i4>1</vt:i4></vt:variant>
    <vt:variant><vt:lpstr>Slide Titles</vt:lpstr></vt:variant>
    <vt:variant><vt:i4>5</vt:i4></vt:variant>
  </vt:vector>
</HeadingPairs>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			type HeadingPairs struct {
				Vector *VectorVariant `xml:"vector"`
			}
			var hp HeadingPairs
			if err := xml.Unmarshal([]byte(tt.xml), &hp); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			out, err := xml.Marshal(&hp)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var hp2 HeadingPairs
			if err := xml.Unmarshal(out, &hp2); err != nil {
				t.Fatalf("Re-unmarshal failed: %v", err)
			}
		})
	}
}

func TestVectorLpstr_RoundTrip(t *testing.T) {
	tests := []struct {
		name string
		xml  string
	}{
		{
			name: "titles of parts",
			xml: `<TitlesOfParts xmlns="http://schemas.openxmlformats.org/officeDocument/2006/extended-properties" xmlns:vt="http://schemas.openxmlformats.org/officeDocument/2006/docPropsVTypes">
  <vt:vector size="3" baseType="lpstr">
    <vt:lpstr>Office Theme</vt:lpstr>
    <vt:lpstr>Title Slide</vt:lpstr>
    <vt:lpstr>Content Slide</vt:lpstr>
  </vt:vector>
</TitlesOfParts>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			type TitlesOfParts struct {
				Vector *VectorLpstr `xml:"vector"`
			}
			var top TitlesOfParts
			if err := xml.Unmarshal([]byte(tt.xml), &top); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			out, err := xml.Marshal(&top)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var top2 TitlesOfParts
			if err := xml.Unmarshal(out, &top2); err != nil {
				t.Fatalf("Re-unmarshal failed: %v", err)
			}
		})
	}
}

func TestCustomProperties_RoundTrip(t *testing.T) {
	tests := []struct {
		name string
		xml  string
	}{
		{
			name: "string property",
			xml: `<Properties xmlns="http://schemas.openxmlformats.org/officeDocument/2006/custom-properties" xmlns:vt="http://schemas.openxmlformats.org/officeDocument/2006/docPropsVTypes">
  <property fmtid="{D5CDD505-2E9C-101B-9397-08002B2CF9AE}" pid="2" name="CustomString">
    <vt:lpwstr>Custom Value</vt:lpwstr>
  </property>
</Properties>`,
		},
		{
			name: "multiple properties",
			xml: `<Properties xmlns="http://schemas.openxmlformats.org/officeDocument/2006/custom-properties" xmlns:vt="http://schemas.openxmlformats.org/officeDocument/2006/docPropsVTypes">
  <property fmtid="{D5CDD505-2E9C-101B-9397-08002B2CF9AE}" pid="2" name="Project">
    <vt:lpwstr>Project A</vt:lpwstr>
  </property>
  <property fmtid="{D5CDD505-2E9C-101B-9397-08002B2CF9AE}" pid="3" name="Version">
    <vt:i4>5</vt:i4>
  </property>
  <property fmtid="{D5CDD505-2E9C-101B-9397-08002B2CF9AE}" pid="4" name="Approved">
    <vt:bool>true</vt:bool>
  </property>
</Properties>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cp CustomProperties
			if err := xml.Unmarshal([]byte(tt.xml), &cp); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			out, err := xml.Marshal(&cp)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var cp2 CustomProperties
			if err := xml.Unmarshal(out, &cp2); err != nil {
				t.Fatalf("Re-unmarshal failed: %v", err)
			}
		})
	}
}

func TestCustomDocumentProperty_RoundTrip(t *testing.T) {
	tests := []struct {
		name    string
		fmtid   string
		pid     int32
		propName string
	}{
		{"string prop", "{D5CDD505-2E9C-101B-9397-08002B2CF9AE}", 2, "MyString"},
		{"int prop", "{D5CDD505-2E9C-101B-9397-08002B2CF9AE}", 3, "MyNumber"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			strVal := "test"
			cdp := &CustomDocumentProperty{
				FMTID:  tt.fmtid,
				PID:    tt.pid,
				Name:   tt.propName,
				Lpwstr: strVal,
			}
			out, err := xml.Marshal(cdp)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var cdp2 CustomDocumentProperty
			if err := xml.Unmarshal(out, &cdp2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if cdp2.FMTID != tt.fmtid {
				t.Errorf("FMTID = %q, want %q", cdp2.FMTID, tt.fmtid)
			}
			if cdp2.PID != tt.pid {
				t.Errorf("PID = %d, want %d", cdp2.PID, tt.pid)
			}
			if cdp2.Name != tt.propName {
				t.Errorf("Name = %q, want %q", cdp2.Name, tt.propName)
			}
		})
	}
}

func TestVariant_RoundTrip(t *testing.T) {
	tests := []struct {
		name  string
		lpstr string
		i4    int32
	}{
		{"string variant", "Test String", 0},
		{"int variant", "", 42},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := &Variant{Lpstr: tt.lpstr, I4: tt.i4}
			out, err := xml.Marshal(v)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var v2 Variant
			if err := xml.Unmarshal(out, &v2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}
		})
	}
}
