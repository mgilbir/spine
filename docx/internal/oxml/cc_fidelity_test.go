package oxml

import (
	"encoding/xml"
	"strings"
	"testing"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// A w:sectPr child the model does not type (seen in the wild: w:mirrorMargins,
// w:tmGutter as trailing sectPr children) must survive a round-trip instead of
// being silently dropped — dropping them is data loss. Found by Common Crawl
// validation (docx 2f2c38).
func TestCTSectPr_PreservesUnmodeledChildren(t *testing.T) {
	const nsW = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"
	src := `<w:sectPr xmlns:w="` + nsW + `">` +
		`<w:pgSz w:w="12240" w:h="15840"/>` +
		`<w:mirrorMargins/>` +
		`<w:tmGutter w:val="0"/>` +
		`</w:sectPr>`

	var sp CT_SectPr
	if err := xml.Unmarshal([]byte(src), &sp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(sp.unknownChildren) != 2 {
		t.Fatalf("captured %d unknown children, want 2 (mirrorMargins, tmGutter)", len(sp.unknownChildren))
	}
	// Drop the fragment's xmlns scaffolding; it's declared on the root below.
	sp.CapturedAttrs = nil

	b := xmlb.NewWordprocessingMLBuilder()
	b.StartElementWithNS(xmlb.NSWordprocessingML, "body", xmlb.WordprocessingMLNamespaces())
	sp.MarshalToBuilder(b, xmlb.NSWordprocessingML, "sectPr")
	b.EndElement(xmlb.NSWordprocessingML, "body")
	out := b.String()

	for _, want := range []string{"mirrorMargins", "tmGutter"} {
		if !strings.Contains(out, want) {
			t.Errorf("marshaled sectPr dropped w:%s (data loss): %s", want, out)
		}
	}
	// Source order (pgSz, mirrorMargins, tmGutter) must be preserved.
	if i, j := strings.Index(out, "mirrorMargins"), strings.Index(out, "tmGutter"); i < 0 || j < 0 || i > j {
		t.Errorf("unmodeled children out of source order: %s", out)
	}
}

// A paragraph whose only content is a w:contentPart (EG_PContent, the in-body
// reference to an ink/customXML part) must not round-trip as an empty <w:p/>.
// Found by Common Crawl validation and noted by the ink/3D wave (PR #164).
func TestCTP_PreservesContentPart(t *testing.T) {
	const (
		nsW = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"
		nsR = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
	)
	src := `<w:p xmlns:w="` + nsW + `" xmlns:r="` + nsR + `">` +
		`<w:contentPart r:id="rId7"/>` +
		`</w:p>`

	var p CT_P
	if err := xml.Unmarshal([]byte(src), &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(p.Raw) != 1 {
		t.Fatalf("captured %d raw children, want 1 (contentPart)", len(p.Raw))
	}
	p.CapturedAttrs = nil

	b := xmlb.NewWordprocessingMLBuilder()
	b.StartElementWithNS(xmlb.NSWordprocessingML, "body", xmlb.WordprocessingMLNamespaces())
	p.MarshalToBuilder(b, xmlb.NSWordprocessingML, "p")
	b.EndElement(xmlb.NSWordprocessingML, "body")
	out := b.String()

	if !strings.Contains(out, "contentPart") {
		t.Errorf("marshaled paragraph dropped w:contentPart (data loss): %s", out)
	}
	if !strings.Contains(out, `r:id="rId7"`) {
		t.Errorf("contentPart lost its r:id: %s", out)
	}
}

// The verbatim source form of run text (rawText) is retained only when the
// normal escaper cannot reproduce it; for ordinary text it is redundant with
// Text and dropped to avoid doubling a text-heavy document's memory. Either
// way the round-trip must be byte-identical.
func TestCTText_RawTextRetainedOnlyWhenNeeded(t *testing.T) {
	const nsW = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"
	cases := []struct {
		name    string
		inner   string // exact w:t inner bytes as the source wrote them
		wantRaw bool   // whether rawText must be retained
	}{
		{"plain", "just some text", false},
		{"canonical amp", "a &amp; b", false},
		{"canonical lt gt", "x &lt; y &gt; z", false},
		{"apos entity", "it&apos;s", true},
		{"quot entity", "say &quot;hi&quot;", true},
		{"numeric ref", "A&#66;C", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			src := `<w:p xmlns:w="` + nsW + `"><w:r><w:t>` + tc.inner + `</w:t></w:r></w:p>`
			var p CT_P
			if err := xmlb.UnmarshalWithSource([]byte(src), &p); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			txt := p.R[0].T[0]
			if got := txt.rawText != nil; got != tc.wantRaw {
				t.Errorf("rawText retained = %v, want %v (Text=%q)", got, tc.wantRaw, txt.Text)
			}
			p.CapturedAttrs = nil
			b := xmlb.NewWordprocessingMLBuilder()
			b.StartElementWithNS(xmlb.NSWordprocessingML, "body", xmlb.WordprocessingMLNamespaces())
			p.MarshalToBuilder(b, xmlb.NSWordprocessingML, "p")
			b.EndElement(xmlb.NSWordprocessingML, "body")
			out := b.String()
			if want := `<w:t>` + tc.inner + `</w:t>`; !strings.Contains(out, want) {
				t.Errorf("round-trip drift: want %q in\n%s", want, out)
			}
		})
	}
}

// A w:t whose xml:space attribute the producer wrote with single quotes must
// round-trip verbatim even when its text is canonical (so rawText is dropped
// and it goes through the escape/WriteElement path). Regression: WriteElement
// used to re-quote captured attributes, drifting xml:space='preserve' to
// xml:space="preserve" once the rawText redundancy skip routed plain text
// through it (Common Crawl fa465a63).
func TestCTText_SingleQuotedXmlSpacePreserved(t *testing.T) {
	const nsW = "http://schemas.openxmlformats.org/wordprocessingml/2006/main"
	src := `<w:p xmlns:w="` + nsW + `"><w:r><w:t xml:space='preserve'>plain text here</w:t></w:r></w:p>`
	var p CT_P
	if err := xmlb.UnmarshalWithSource([]byte(src), &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	// Plain text: rawText is not retained (escaper reproduces it), so this
	// exercises the WriteElement path.
	if p.R[0].T[0].rawText != nil {
		t.Fatalf("precondition: rawText retained for plain text; test would not exercise WriteElement")
	}
	p.CapturedAttrs = nil
	b := xmlb.NewWordprocessingMLBuilder()
	b.StartElementWithNS(xmlb.NSWordprocessingML, "body", xmlb.WordprocessingMLNamespaces())
	p.MarshalToBuilder(b, xmlb.NSWordprocessingML, "p")
	b.EndElement(xmlb.NSWordprocessingML, "body")
	out := b.String()
	if !strings.Contains(out, `<w:t xml:space='preserve'>plain text here</w:t>`) {
		t.Errorf("single-quoted xml:space not preserved through WriteElement:\n%s", out)
	}
}
