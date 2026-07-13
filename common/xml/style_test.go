package xml

import "testing"

func TestDetectCollapsedEmptyElements(t *testing.T) {
	tests := []struct {
		name string
		data string
		want bool
	}{
		{"self-closing only", `<?xml?><w:document><w:p w:rsidR="A"/><w:r><w:t>x</w:t></w:r></w:document>`, true},
		{"expanded empty present", `<w:document><w:sz w:val="2"/><w:rPr></w:rPr></w:document>`, false},
		{"no self-closing at all", `<w:document><w:p></w:p></w:document>`, false},
		{"close-after-close is not an expanded empty", `<a><b><c/></b></a>`, true},
		{"text content is not an expanded empty", `<a><b>x</b><c/></a>`, true},
		{"whitespace content is not adjacency", "<a><b>\n</b><c/></a>", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DetectCollapsedEmptyElements([]byte(tt.data)); got != tt.want {
				t.Errorf("DetectCollapsedEmptyElements = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCollapseEmptyElements(t *testing.T) {
	b := NewBuilder()
	b.RegisterNamespace("urn:w", "w")
	b.SetCollapseEmptyElements(true)
	b.StartElement("urn:w", "document")
	b.StartElement("urn:w", "p", Attr{Name: "w:rsidR", Value: "00263AF7"})
	b.EndElement("urn:w", "p")
	b.StartElement("urn:w", "p")
	b.WriteElement("urn:w", "t", "x")
	b.EndElement("urn:w", "p")
	b.EndElement("urn:w", "document")
	if err := b.Finish(); err != nil {
		t.Fatal(err)
	}
	want := `<w:document><w:p w:rsidR="00263AF7"/><w:p><w:t>x</w:t></w:p></w:document>`
	if got := b.String(); got != want {
		t.Errorf("collapsed output = %q, want %q", got, want)
	}
}

func TestCollapseEmptyElementsDisabledKeepsPairs(t *testing.T) {
	b := NewBuilder()
	b.RegisterNamespace("urn:w", "w")
	b.StartElement("urn:w", "p")
	b.EndElement("urn:w", "p")
	if err := b.Finish(); err != nil {
		t.Fatal(err)
	}
	if got, want := b.String(), "<w:p></w:p>"; got != want {
		t.Errorf("default output = %q, want %q", got, want)
	}
}

func TestCollapseEmptyWithSelfClosingSpace(t *testing.T) {
	b := NewBuilder()
	b.RegisterNamespace("urn:w", "w")
	b.SetCollapseEmptyElements(true)
	b.SetSelfClosingSpace(true)
	b.StartElement("urn:w", "p")
	b.EndElement("urn:w", "p")
	if got, want := b.String(), "<w:p />"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}

func TestCollapseEmptyRawContentFlushesTag(t *testing.T) {
	b := NewBuilder()
	b.RegisterNamespace("urn:w", "w")
	b.SetCollapseEmptyElements(true)
	b.StartElement("urn:w", "p")
	b.WriteRaw([]byte("<x/>"))
	b.EndElement("urn:w", "p")
	if got, want := b.String(), "<w:p><x/></w:p>"; got != want {
		t.Errorf("output = %q, want %q", got, want)
	}
}
