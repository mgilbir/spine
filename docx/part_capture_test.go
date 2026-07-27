package docx

import (
	"strings"
	"testing"
)

// C370: word/styles.xml, word/comments.xml, word/footnotes.xml,
// word/endnotes.xml and every header/footer part were parsed with plain
// xmlb.Unmarshal, so the capture kit (which needs the decoder's source bytes
// registered) recorded nothing. Any mutation that flips one of those parts to
// regenerate therefore deleted every child the model does not type.

// TestStylesPartKeepsUnmodeledChildren covers repro (a) from the audit: a
// w14:cntxtAlts inside Normal's rPr, plus the root mc:Ignorable and the
// xmlns:w14 declaration, all vanished on AddParagraphStyle.
func TestStylesPartKeepsUnmodeledChildren(t *testing.T) {
	styles := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
		`<w:styles xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006" ` +
		`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" ` +
		`xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" ` +
		`xmlns:w14="http://schemas.microsoft.com/office/word/2010/wordml" mc:Ignorable="w14">` +
		`<w:style w:type="paragraph" w:default="1" w:styleId="Normal">` +
		`<w:name w:val="Normal"/><w:rPr><w:b/><w14:cntxtAlts/></w:rPr>` +
		`</w:style>` +
		`</w:styles>`
	fixture := fixtureWithStyles(t, styles)

	doc := openFixture(t, fixture)
	doc.Styles().AddParagraphStyle("MyStyle", "My Style")
	saved := saveDoc(t, doc)

	out := mustZipEntry(t, saved, "word/styles.xml")
	if !strings.Contains(out, `<w14:cntxtAlts/>`) {
		t.Errorf("unmodeled w14:cntxtAlts dropped from regenerated styles.xml:\n%s", out)
	}
	if !strings.Contains(out, `mc:Ignorable="w14"`) {
		t.Errorf("root mc:Ignorable dropped from regenerated styles.xml:\n%s", out)
	}
	if !strings.Contains(out, `xmlns:w14="http://schemas.microsoft.com/office/word/2010/wordml"`) {
		t.Errorf("root xmlns:w14 declaration dropped from regenerated styles.xml:\n%s", out)
	}
	if !strings.Contains(out, `w:styleId="MyStyle"`) {
		t.Errorf("newly added style missing from styles.xml:\n%s", out)
	}
}

// TestStylesPartZeroModRoundTrip asserts arming the capture kit does not change
// the output of a styles part that had nothing to capture.
func TestStylesPartZeroModRoundTrip(t *testing.T) {
	styles := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
		`<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` +
		`<w:style w:type="paragraph" w:default="1" w:styleId="Normal"><w:name w:val="Normal"/></w:style>` +
		`</w:styles>`
	fixture := fixtureWithStyles(t, styles)
	doc := openFixture(t, fixture)
	saved := saveDoc(t, doc)
	if got := mustZipEntry(t, saved, "word/styles.xml"); got != styles {
		t.Errorf("zero-modification save changed styles.xml:\ngot:  %s\nwant: %s", got, styles)
	}
}

const fixtureHeaderCT = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/><Override PartName="/word/header1.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.header+xml"/></Types>`

// TestHeaderPartKeepsUnmodeledChildren covers repro (b): editing a header run
// dropped a w14:cntxtAlts elsewhere in the same header.
func TestHeaderPartKeepsUnmodeledChildren(t *testing.T) {
	header := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
		`<w:hdr xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" ` +
		`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" ` +
		`xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006" ` +
		`xmlns:w14="http://schemas.microsoft.com/office/word/2010/wordml" mc:Ignorable="w14">` +
		`<w:p><w:hyperlink r:id="rId9"><w:r><w:rPr><w:b/><w14:cntxtAlts/></w:rPr><w:t>link</w:t></w:r></w:hyperlink></w:p>` +
		`</w:hdr>`
	fixture := buildFixtureDocx(t, map[string]string{
		"[Content_Types].xml": fixtureHeaderCT,
		"_rels/.rels":         fixtureRootRels,
		"word/document.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
			`<w:document ` + fixtureWNS + `><w:body><w:p/>` +
			`<w:sectPr><w:headerReference w:type="default" r:id="rId1"/></w:sectPr></w:body></w:document>`,
		"word/_rels/document.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/header" Target="header1.xml"/></Relationships>`,
		"word/_rels/header1.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId9" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/hyperlink" Target="https://example.com/" TargetMode="External"/></Relationships>`,
		"word/header1.xml": header,
	})

	doc := openFixture(t, fixture)
	links := doc.Hyperlinks()
	if len(links) == 0 {
		t.Fatal("no hyperlinks found in header")
	}
	runs := links[0].Runs()
	if len(runs) == 0 {
		t.Fatal("hyperlink has no runs")
	}
	runs[0].SetText("edited")
	saved := saveDoc(t, doc)

	out := mustZipEntry(t, saved, "word/header1.xml")
	if !strings.Contains(out, `<w14:cntxtAlts/>`) {
		t.Errorf("unmodeled w14:cntxtAlts dropped from regenerated header1.xml:\n%s", out)
	}
	if !strings.Contains(out, `xmlns:w14="http://schemas.microsoft.com/office/word/2010/wordml"`) {
		t.Errorf("root xmlns:w14 declaration dropped from regenerated header1.xml:\n%s", out)
	}
	if !strings.Contains(out, `mc:Ignorable="w14"`) {
		t.Errorf("root mc:Ignorable dropped from regenerated header1.xml:\n%s", out)
	}
	if !strings.Contains(out, "edited") {
		t.Errorf("edit not applied to header1.xml:\n%s", out)
	}
}

const fixtureFootnotesCT = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/><Override PartName="/word/footnotes.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.footnotes+xml"/></Types>`

// TestFootnotesPartKeepsUnmodeledChildren covers repro (c): AddFootnote on a
// document with an existing note dropped that note's unmodeled children.
func TestFootnotesPartKeepsUnmodeledChildren(t *testing.T) {
	footnotes := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
		`<w:footnotes xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" ` +
		`xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006" ` +
		`xmlns:w14="http://schemas.microsoft.com/office/word/2010/wordml" mc:Ignorable="w14">` +
		`<w:footnote w:type="separator" w:id="-1"><w:p><w:r><w:separator/></w:r></w:p></w:footnote>` +
		`<w:footnote w:id="1"><w:p><w:r><w:rPr><w:b/><w14:cntxtAlts/></w:rPr><w:t>note</w:t></w:r></w:p></w:footnote>` +
		`</w:footnotes>`
	fixture := buildFixtureDocx(t, map[string]string{
		"[Content_Types].xml": fixtureFootnotesCT,
		"_rels/.rels":         fixtureRootRels,
		"word/document.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
			`<w:document ` + fixtureWNS + `><w:body><w:p><w:r><w:t>body</w:t></w:r></w:p></w:body></w:document>`,
		"word/_rels/document.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/footnotes" Target="footnotes.xml"/></Relationships>`,
		"word/footnotes.xml": footnotes,
	})

	doc := openFixture(t, fixture)
	paras := doc.Paragraphs()
	if len(paras) == 0 {
		t.Fatal("no paragraphs")
	}
	runs := paras[0].Runs()
	if len(runs) == 0 {
		t.Fatal("no runs")
	}
	runs[0].AddFootnote("new note")
	saved := saveDoc(t, doc)

	out := mustZipEntry(t, saved, "word/footnotes.xml")
	if !strings.Contains(out, `<w14:cntxtAlts/>`) {
		t.Errorf("unmodeled w14:cntxtAlts dropped from regenerated footnotes.xml:\n%s", out)
	}
	if !strings.Contains(out, "new note") {
		t.Errorf("newly added footnote missing:\n%s", out)
	}
}
