package docx

import (
	"strings"
	"testing"

	"github.com/mgilbir/spine/internal/fuzzseed"
)

// Two parts this package parses into a model and no target covered.
//
// Both are reached from ordinary calls — Frameset() and BuildingBlocks() — over
// bytes that came from a file, so every parser in them runs on input the caller
// did not write. The rest of the docx parts have had a target since the fuzz
// wave; these two were missed because nothing enumerated the parsers against
// the targets.
//
// Both targets are part-level rather than round-trip: the accessor is called,
// the document is saved, and the result is reopened, which is what proves a
// value survived rather than merely being echoed back off the object the setter
// just mutated.

func FuzzDocxWebSettingsXML(f *testing.F) {
	valid := buildRichDocxFuzzSeed(f)
	const part = "word/webSettings.xml"

	const open = `<w:webSettings xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">`
	f.Add([]byte(open + `</w:webSettings>`))
	f.Add([]byte{})
	f.Add([]byte("<w:webSettings"))
	// A frameset, which is what the model reads out of this part.
	f.Add([]byte(open +
		`<w:frameset><w:framesetSplitbar><w:w w:val="60"/></w:framesetSplitbar>` +
		`<w:frameset><w:frame><w:name w:val="left"/><w:sz w:val="50%"/></w:frame></w:frameset>` +
		`<w:frame><w:name w:val="right"/><w:sz w:val="*"/></w:frame></w:frameset>` +
		`</w:webSettings>`))
	// Hostile sizes and a frameset nested into itself.
	f.Add([]byte(open +
		`<w:frameset><w:frameset><w:frameset><w:frame><w:sz w:val="-99999999999"/>` +
		`<w:name w:val="` + strings.Repeat("n", 2048) + `"/></w:frame></w:frameset></w:frameset></w:frameset>` +
		`</w:webSettings>`))
	// Children the model does not type, which must survive the round trip.
	f.Add([]byte(open + `<w:divs><w:div w:id="1"><w:marLeft w:val="0"/></w:div></w:divs>` +
		`<w:optimizeForBrowser/><w:relyOnVML/></w:webSettings>`))

	f.Fuzz(func(t *testing.T, data []byte) {
		skipOversizedParts(t, data)
		pkg := fuzzseed.ReplaceZipEntry(valid, part, data)
		if pkg == nil {
			t.Skip("seed package unreadable")
		}
		d := openFuzzedPackage(t, pkg)
		if d == nil {
			return
		}
		defer func() { _ = d.Close() }()

		// Reading the frameset must not panic and must agree with itself.
		fs := d.Frameset()
		first := framesetShape(fs)
		if again := framesetShape(d.Frameset()); again != first {
			t.Fatalf("Frameset() disagrees with itself across two calls: %q then %q", first, again)
		}

		out, err := d.SaveBytes()
		if err != nil {
			return // refusing to write a document built on a corrupt part is legitimate
		}
		assertPartsAreWellFormed(t, out)
		assertEmittedNamespacesResolve(t, pkg, out)

		d2 := openFuzzedPackage(t, out)
		if d2 == nil {
			return
		}
		defer func() { _ = d2.Close() }()
		if got := framesetShape(d2.Frameset()); got != first {
			t.Fatalf("the frameset changed across a save: %q before, %q after", first, got)
		}
	})
}

// framesetShape renders a frameset's structure compactly, so two of them can be
// compared without depending on the model's field-by-field shape.
func framesetShape(fs *Frameset) string {
	if fs == nil {
		return "<nil>"
	}
	var b strings.Builder
	var walk func(f *Frameset, depth int)
	walk = func(f *Frameset, depth int) {
		if f == nil || depth > 32 {
			return
		}
		for _, fr := range f.Frames() {
			b.WriteString(strings.Repeat(" ", depth))
			b.WriteString(fr.Name())
			b.WriteByte('|')
			b.WriteString(fr.Size())
			b.WriteByte('\n')
		}
		for _, child := range f.Framesets() {
			b.WriteString(strings.Repeat(" ", depth))
			b.WriteString("{\n")
			walk(child, depth+1)
			b.WriteString("}\n")
		}
	}
	walk(fs, 0)
	return b.String()
}

func FuzzDocxGlossaryXML(f *testing.F) {
	valid := buildRichDocxFuzzSeed(f)
	const part = "word/glossary/document.xml"

	const open = `<w:glossaryDocument xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">`
	f.Add([]byte(open + `</w:glossaryDocument>`))
	f.Add([]byte{})
	f.Add([]byte("<w:glossaryDocument"))
	// One building block, which is what the model reads out of this part.
	f.Add([]byte(open + `<w:docParts><w:docPart>` +
		`<w:docPartPr><w:name w:val="Greeting"/><w:category><w:name w:val="General"/>` +
		`<w:gallery w:val="autoText"/></w:category>` +
		`<w:types><w:type w:val="autoText"/></w:types></w:docPartPr>` +
		`<w:docPartBody><w:p><w:r><w:t>Hello</w:t></w:r></w:p></w:docPartBody>` +
		`</w:docPart></w:docParts></w:glossaryDocument>`))
	// Two entries with the same name, one with no properties at all.
	f.Add([]byte(open + `<w:docParts>` +
		`<w:docPart><w:docPartPr><w:name w:val="Dup"/></w:docPartPr><w:docPartBody><w:p/></w:docPartBody></w:docPart>` +
		`<w:docPart><w:docPartPr><w:name w:val="Dup"/></w:docPartPr><w:docPartBody><w:p><w:r><w:t>second</w:t></w:r></w:p></w:docPartBody></w:docPart>` +
		`<w:docPart/>` +
		`</w:docParts></w:glossaryDocument>`))
	// A name carrying every character XML has to escape.
	f.Add([]byte(open + `<w:docParts><w:docPart><w:docPartPr>` +
		`<w:name w:val="a &amp; b &lt;c&gt; &quot;d&quot;"/></w:docPartPr>` +
		`<w:docPartBody><w:p><w:r><w:t>]]&gt;</w:t></w:r></w:p></w:docPartBody>` +
		`</w:docPart></w:docParts></w:glossaryDocument>`))

	f.Fuzz(func(t *testing.T, data []byte) {
		skipOversizedParts(t, data)
		pkg := fuzzseed.ReplaceZipEntry(valid, part, data)
		if pkg == nil {
			t.Skip("seed package unreadable")
		}
		d := openFuzzedPackage(t, pkg)
		if d == nil {
			return
		}
		defer func() { _ = d.Close() }()

		before := buildingBlockNames(d)

		out, err := d.SaveBytes()
		if err != nil {
			return
		}
		assertPartsAreWellFormed(t, out)
		assertEmittedNamespacesResolve(t, pkg, out)

		d2 := openFuzzedPackage(t, out)
		if d2 == nil {
			return
		}
		defer func() { _ = d2.Close() }()
		if after := buildingBlockNames(d2); after != before {
			t.Fatalf("the building blocks changed across a save:\n before: %q\n after:  %q", before, after)
		}
	})
}

func buildingBlockNames(d *Document) string {
	var b strings.Builder
	for _, bb := range d.BuildingBlocks() {
		if bb == nil {
			b.WriteString("<nil>\n")
			continue
		}
		b.WriteString(bb.Name())
		b.WriteByte('\t')
		b.WriteString(bb.Gallery())
		b.WriteByte('\n')
	}
	return b.String()
}
