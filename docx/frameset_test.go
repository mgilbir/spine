package docx

import (
	"bytes"
	"testing"
)

// framesetFixture builds a docx whose web-settings part defines a nested
// frameset with two frames, each pointing at an external source document.
func framesetFixture(t *testing.T) []byte {
	t.Helper()
	const ct = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
		`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
		`<Default Extension="xml" ContentType="application/xml"/>` +
		`<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>` +
		`<Override PartName="/word/webSettings.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.webSettings+xml"/>` +
		`</Types>`
	const docRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/webSettings" Target="webSettings.xml"/>` +
		`</Relationships>`
	const webSettings = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<w:webSettings xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">` +
		`<w:frameset>` +
		`<w:frameLayout w:val="cols"/>` +
		`<w:frameset>` +
		`<w:frameLayout w:val="rows"/>` +
		`<w:frame><w:name w:val="top"/><w:sz w:val="*"/><w:sourceFileName r:id="rId1"/></w:frame>` +
		`</w:frameset>` +
		`<w:frame><w:name w:val="side"/><w:sz w:val="240"/><w:scrollbar w:val="off"/><w:sourceFileName r:id="rId2"/></w:frame>` +
		`</w:frameset>` +
		`</w:webSettings>`
	const wsRels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/frame" Target="top.html" TargetMode="External"/>` +
		`<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/frame" Target="side.html" TargetMode="External"/>` +
		`</Relationships>`
	return buildFixtureDocx(t, map[string]string{
		"[Content_Types].xml":             ct,
		"_rels/.rels":                     fixtureRootRels,
		"word/document.xml":               `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + `<w:document ` + fixtureWNS + `><w:body><w:p/></w:body></w:document>`,
		"word/_rels/document.xml.rels":    docRels,
		"word/webSettings.xml":            webSettings,
		"word/_rels/webSettings.xml.rels": wsRels,
	})
}

// TestFramesetRead parses the nested frameset structure and resolves each
// frame's external source through the web-settings relationships.
func TestFramesetRead(t *testing.T) {
	fixture := framesetFixture(t)
	doc, err := OpenReader(bytes.NewReader(fixture), int64(len(fixture)))
	if err != nil {
		t.Fatal(err)
	}
	fs := doc.Frameset()
	if fs == nil {
		t.Fatal("Frameset = nil")
	}
	if fs.Layout() != "cols" {
		t.Fatalf("top layout = %q", fs.Layout())
	}
	if len(fs.Framesets()) != 1 {
		t.Fatalf("nested framesets = %d, want 1", len(fs.Framesets()))
	}
	if len(fs.Frames()) != 1 {
		t.Fatalf("top-level frames = %d, want 1", len(fs.Frames()))
	}

	nested := fs.Framesets()[0]
	if nested.Layout() != "rows" || len(nested.Frames()) != 1 {
		t.Fatalf("nested frameset = layout %q, %d frames", nested.Layout(), len(nested.Frames()))
	}
	top := nested.Frames()[0]
	if top.Name() != "top" || top.Size() != "*" {
		t.Fatalf("top frame = name %q size %q", top.Name(), top.Size())
	}
	if top.SourceID() != "rId1" || top.SourceTarget() != "top.html" {
		t.Fatalf("top frame source = %q -> %q", top.SourceID(), top.SourceTarget())
	}

	side := fs.Frames()[0]
	if side.Name() != "side" || side.Size() != "240" || side.Scrollbar() != "off" {
		t.Fatalf("side frame = name %q size %q scrollbar %q", side.Name(), side.Size(), side.Scrollbar())
	}
	if side.SourceTarget() != "side.html" {
		t.Fatalf("side frame target = %q", side.SourceTarget())
	}
}

// TestFramesetPreservedByteIdentical guards that reading the frameset leaves the
// web-settings part untouched across a round-trip.
func TestFramesetPreservedByteIdentical(t *testing.T) {
	fixture := framesetFixture(t)
	orig, _ := zipEntry(t, fixture, "word/webSettings.xml")
	doc, err := OpenReader(bytes.NewReader(fixture), int64(len(fixture)))
	if err != nil {
		t.Fatal(err)
	}
	_ = doc.Frameset()
	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	got, ok := zipEntry(t, saved, "word/webSettings.xml")
	if !ok {
		t.Fatal("webSettings part missing after save")
	}
	if !bytes.Equal(orig, got) {
		t.Fatal("webSettings.xml not byte-identical after round-trip")
	}
}

// TestFramesetNone returns nil for a document without a frameset.
func TestFramesetNone(t *testing.T) {
	fixture := fixtureWithDocument(t, fixtureWNS, `<w:body><w:p/></w:body>`)
	doc, err := OpenReader(bytes.NewReader(fixture), int64(len(fixture)))
	if err != nil {
		t.Fatal(err)
	}
	if fs := doc.Frameset(); fs != nil {
		t.Fatalf("Frameset = %v, want nil", fs)
	}
}
