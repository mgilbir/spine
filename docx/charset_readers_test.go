package docx

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/mgilbir/spine/internal/testutil"
)

// The readers below walk a part with a token loop of their own. They used a
// decoder with no charset support, so on a part declaring a code page they
// failed and, being best-effort, said nothing: a frameset read as absent, a
// custom XML part had no item id, setting a frameset replaced the whole
// web-settings part, and adding a building block failed the save with "no
// <w:docParts> container found".

func openWindows1252(t *testing.T, data []byte, parts ...string) *Document {
	t.Helper()
	src, _, err := testutil.WithWindows1252(data, parts...)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := OpenReader(bytes.NewReader(src), int64(len(src)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = doc.Close() })
	return doc
}

func TestFramesetReadsWindows1252(t *testing.T) {
	doc := openWindows1252(t, framesetFixture(t), "word/webSettings.xml")
	fs := doc.Frameset()
	if fs == nil || fs.Layout() != "cols" || len(fs.Frames()) != 1 || fs.Frames()[0].SourceTarget() != "side.html" {
		t.Fatalf("Frameset = %v", fs)
	}
}

func TestSetFramesetSplicesWindows1252(t *testing.T) {
	const webSettings = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<w:webSettings xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` +
		`<!-- réglages --><w:optimizeForBrowser/>` +
		`</w:webSettings>`
	fixture := buildFixtureDocx(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
			`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
			`<Default Extension="xml" ContentType="application/xml"/>` +
			`<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>` +
			`<Override PartName="/word/webSettings.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.webSettings+xml"/>` +
			`</Types>`,
		"_rels/.rels":       fixtureRootRels,
		"word/document.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + `<w:document ` + fixtureWNS + `><w:body><w:p/></w:body></w:document>`,
		"word/_rels/document.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
			`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/webSettings" Target="webSettings.xml"/>` +
			`</Relationships>`,
		"word/webSettings.xml": webSettings,
	})
	doc := openWindows1252(t, fixture, "word/webSettings.xml")
	if err := doc.SetFrameset(FramesetDef{Layout: "cols", Frames: []FrameDef{{Name: "été", SourceTarget: "main.html"}}}); err != nil {
		t.Fatal(err)
	}
	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	ws, _ := zipEntry(t, saved, "word/webSettings.xml")
	// The existing settings are kept, and the spliced part is UTF-8 and says so.
	if !bytes.Contains(ws, []byte("<!-- réglages --><w:optimizeForBrowser/>")) || !bytes.Contains(ws, []byte(`encoding="UTF-8"`)) {
		t.Errorf("web settings: %s", ws)
	}
	re, err := OpenReader(bytes.NewReader(saved), int64(len(saved)))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = re.Close() }()
	if fs := re.Frameset(); fs == nil || len(fs.Frames()) != 1 || fs.Frames()[0].Name() != "été" {
		t.Errorf("Frameset = %v", fs)
	}
}

func TestAddBuildingBlockSplicesWindows1252(t *testing.T) {
	orig, err := os.ReadFile("testdata/chart.docx")
	if err != nil {
		t.Fatal(err)
	}
	doc := openWindows1252(t, orig, "word/glossary/document.xml")
	if err = doc.AddBuildingBlock(BuildingBlockDef{Name: "Ajouté", Gallery: "AutoText", Types: []string{"autoTxt"}, GUID: "{22222222-2222-2222-2222-222222222222}"}); err != nil {
		t.Fatal(err)
	}
	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	re, err := OpenReader(bytes.NewReader(saved), int64(len(saved)))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = re.Close() }()
	if blocks := re.BuildingBlocks(); len(blocks) != 2 || blocks[1].Name() != "Ajouté" {
		t.Errorf("BuildingBlocks = %v", blocks)
	}
}

func TestCustomXMLPartReadsWindows1252(t *testing.T) {
	orig, err := os.ReadFile("testdata/chart.docx")
	if err != nil {
		t.Fatal(err)
	}
	doc := openWindows1252(t, orig, "customXml/itemProps1.xml")
	parts := doc.CustomXMLParts()
	if len(parts) != 2 || parts[0].ItemID() != "{B1977F7D-205B-4081-913C-38D41E755F92}" || len(parts[0].SchemaRefs()) != 1 {
		t.Fatalf("custom XML parts: %v", parts)
	}

	data := []byte("<?xml version=\"1.0\" encoding=\"windows-1252\"?><r\xe9sum\xe9 xmlns=\"http://example.com/ns\"/>")
	part, err := Create().AddCustomXMLPart(data)
	if err != nil {
		t.Fatal(err)
	}
	if refs := part.SchemaRefs(); len(refs) != 1 || refs[0] != "http://example.com/ns" || !strings.HasPrefix(part.ItemID(), "{") {
		t.Errorf("added part: %v %q", refs, part.ItemID())
	}
}
