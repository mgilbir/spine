package docx

import (
	"bytes"
	"testing"
)

// psmdcpCoreFixture builds a docx whose core properties live in a *.psmdcp part
// (as System.IO.Packaging writes them), referenced from the root .rels by the
// core-properties relationship rather than a /docProps/core.xml part.
func psmdcpCoreFixture(t *testing.T) []byte {
	t.Helper()
	const psmdcpPart = "package/services/metadata/core-properties/item.psmdcp"
	const contentTypes = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Default Extension="psmdcp" ContentType="application/vnd.openxmlformats-package.core-properties+xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/></Types>`
	const rootRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/><Relationship Id="rId2" Type="http://schemas.openxmlformats.org/package/2006/relationships/metadata/core-properties" Target="/` + psmdcpPart + `"/></Relationships>`
	const documentXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
		`<w:document ` + fixtureWNS + `><w:body><w:p><w:r><w:t>body</w:t></w:r></w:p></w:body></w:document>`
	const coreXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><cp:coreProperties xmlns:cp="http://schemas.openxmlformats.org/package/2006/metadata/core-properties" xmlns:dc="http://purl.org/dc/elements/1.1/"><dc:title>Old Title</dc:title><dc:creator>orig</dc:creator></cp:coreProperties>`

	return buildFixtureDocx(t, map[string]string{
		"[Content_Types].xml":          contentTypes,
		"_rels/.rels":                  rootRels,
		"word/document.xml":            documentXML,
		"word/_rels/document.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n" + `<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"></Relationships>`,
		psmdcpPart:                     coreXML,
	})
}

// TestPsmdcpCorePropsEditWritesBack verifies that editing Properties on a
// package whose core properties live in a *.psmdcp part writes the edit back
// into that part (not into an orphan docProps/core.xml the root .rels never
// points at), so the edit is read back and the package keeps exactly one
// core-properties part (C294).
func TestPsmdcpCorePropsEditWritesBack(t *testing.T) {
	fixture := psmdcpCoreFixture(t)
	doc, err := OpenReader(bytes.NewReader(fixture), int64(len(fixture)))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if got := doc.Properties.Title; got != "Old Title" {
		t.Fatalf("pre-edit Title = %q, want %q", got, "Old Title")
	}
	doc.Properties.Title = "New Psmdcp Title"

	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	// No orphan standard core part must be synthesized.
	if _, ok := zipEntry(t, saved, "docProps/core.xml"); ok {
		t.Error("orphan docProps/core.xml written for a psmdcp-flavored package")
	}
	// The psmdcp part must still be present (exactly one core-property part).
	if _, ok := zipEntry(t, saved, "package/services/metadata/core-properties/item.psmdcp"); !ok {
		t.Error("psmdcp core part missing from saved package")
	}

	re, err := OpenReader(bytes.NewReader(saved), int64(len(saved)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if got := re.Properties.Title; got != "New Psmdcp Title" {
		t.Errorf("reopened Title = %q, want %q (edit not written back)", got, "New Psmdcp Title")
	}
}
