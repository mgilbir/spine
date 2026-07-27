package docx

import (
	"bytes"
	"encoding/xml"
	"os"
	"strings"
	"testing"
)

// TestCustomXMLPartsReadFromChart reads the two custom-XML data parts carried by
// the chart fixture, resolving each part's datastore item id and schema refs
// through its itemProps.
func TestCustomXMLPartsReadFromChart(t *testing.T) {
	doc, err := Open("testdata/chart.docx")
	if err != nil {
		t.Fatal(err)
	}
	defer doc.Close() //nolint:errcheck

	parts := doc.CustomXMLParts()
	if len(parts) != 2 {
		t.Fatalf("CustomXMLParts len = %d, want 2", len(parts))
	}
	item1 := parts[0]
	if item1.PartName() != "/customXml/item1.xml" {
		t.Fatalf("part 0 name = %q", item1.PartName())
	}
	if item1.ItemID() != "{B1977F7D-205B-4081-913C-38D41E755F92}" {
		t.Fatalf("part 0 itemID = %q", item1.ItemID())
	}
	if len(item1.SchemaRefs()) != 1 || item1.SchemaRefs()[0] != "http://www.wps.cn/officeDocument/2013/wpsCustomData" {
		t.Fatalf("part 0 schemaRefs = %v", item1.SchemaRefs())
	}
	if !bytes.Contains(item1.Data(), []byte("s:customData")) {
		t.Fatalf("part 0 data missing expected content: %s", item1.Data())
	}
}

// TestCustomXMLPartsPreservedByteIdentical guards that reading custom-XML parts
// does not disturb their bytes: an untouched round-trip keeps item and itemProps
// parts identical.
func TestCustomXMLPartsPreservedByteIdentical(t *testing.T) {
	orig, err := os.ReadFile("testdata/chart.docx")
	if err != nil {
		t.Fatal(err)
	}
	doc, err := Open("testdata/chart.docx")
	if err != nil {
		t.Fatal(err)
	}
	_ = doc.CustomXMLParts() // exercise the read path
	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"customXml/item1.xml", "customXml/itemProps1.xml", "customXml/_rels/item1.xml.rels"} {
		a, _ := zipEntry(t, orig, name)
		b, ok := zipEntry(t, saved, name)
		if !ok {
			t.Fatalf("%s missing after save", name)
		}
		if !bytes.Equal(a, b) {
			t.Fatalf("%s not byte-identical after round-trip", name)
		}
	}
}

// TestAddCustomXMLPart adds a custom-XML data part to a created document and
// verifies the item, itemProps, item→itemProps relationship, and document
// relationship are all written and reopen cleanly.
func TestAddCustomXMLPart(t *testing.T) {
	doc := Create()
	doc.AddParagraphWithText("hi")
	data := []byte(`<?xml version="1.0"?><root xmlns="http://example.com/ns"><a>1</a></root>`)
	part, err := doc.AddCustomXMLPart(data)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(part.ItemID(), "{") || !strings.HasSuffix(part.ItemID(), "}") {
		t.Fatalf("generated itemID not a GUID: %q", part.ItemID())
	}
	if part.PartName() != "/customXml/item1.xml" {
		t.Fatalf("part name = %q", part.PartName())
	}
	if len(part.SchemaRefs()) != 1 || part.SchemaRefs()[0] != "http://example.com/ns" {
		t.Fatalf("schemaRefs = %v", part.SchemaRefs())
	}

	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}

	itemData, ok := zipEntry(t, saved, "customXml/item1.xml")
	if !ok || !bytes.Equal(itemData, data) {
		t.Fatalf("item1.xml missing or altered: %q", itemData)
	}
	props, ok := zipEntry(t, saved, "customXml/itemProps1.xml")
	if !ok || !bytes.Contains(props, []byte(part.ItemID())) {
		t.Fatalf("itemProps1.xml missing itemID: %q", props)
	}
	rels, ok := zipEntry(t, saved, "customXml/_rels/item1.xml.rels")
	if !ok || !bytes.Contains(rels, []byte("customXmlProps")) {
		t.Fatalf("item rels missing: %q", rels)
	}
	docRels, _ := zipEntry(t, saved, "word/_rels/document.xml.rels")
	if !bytes.Contains(docRels, []byte("../customXml/item1.xml")) {
		t.Fatalf("document rels missing customXml relationship: %q", docRels)
	}
	ct, _ := zipEntry(t, saved, "[Content_Types].xml")
	if !bytes.Contains(ct, []byte("customXmlProperties+xml")) {
		t.Fatalf("content types missing itemProps override: %q", ct)
	}

	// Reopen and confirm the part is listed with the same id.
	rd, err := OpenReader(bytes.NewReader(saved), int64(len(saved)))
	if err != nil {
		t.Fatalf("saved doc does not reopen: %v", err)
	}
	got := rd.CustomXMLParts()
	if len(got) != 1 || got[0].ItemID() != part.ItemID() {
		t.Fatalf("reopened parts = %+v", got)
	}
}

// TestAddCustomXMLPartEscapesSchemaURI adds a data part whose root-element
// namespace URI legally contains an ampersand. That URI is recorded as a
// schemaRef and written into the itemProps ds:uri attribute; concatenated raw
// it produces malformed XML (a bare & in an attribute value), corrupting the
// saved part. Regression test for C350: the URI must be attribute-escaped so
// the itemProps part is well-formed and reparses to the original URI.
func TestAddCustomXMLPartEscapesSchemaURI(t *testing.T) {
	doc := Create()
	const nsURI = "http://x/?a=1&b=2"
	data := []byte(`<root xmlns="http://x/?a=1&amp;b=2"><a>1</a></root>`)
	part, err := doc.AddCustomXMLPart(data)
	if err != nil {
		t.Fatal(err)
	}
	if len(part.SchemaRefs()) != 1 || part.SchemaRefs()[0] != nsURI {
		t.Fatalf("schemaRefs = %v, want [%q]", part.SchemaRefs(), nsURI)
	}

	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	props, ok := zipEntry(t, saved, "customXml/itemProps1.xml")
	if !ok {
		t.Fatal("itemProps1.xml missing")
	}
	// The ampersand must be escaped, and the raw (malformed) form must be absent.
	if !bytes.Contains(props, []byte(`ds:uri="http://x/?a=1&amp;b=2"`)) {
		t.Errorf("itemProps did not attribute-escape the schema URI: %q", props)
	}
	if bytes.Contains(props, []byte(`ds:uri="http://x/?a=1&b=2"`)) {
		t.Errorf("itemProps contains an unescaped ampersand: %q", props)
	}
	// The saved part must be well-formed XML.
	if err := xml.Unmarshal(props, new(struct {
		XMLName xml.Name
	})); err != nil {
		t.Errorf("itemProps1.xml is not well-formed: %v", err)
	}

	// Reopen and confirm the schema ref round-trips to the original URI.
	rd, err := OpenReader(bytes.NewReader(saved), int64(len(saved)))
	if err != nil {
		t.Fatalf("saved doc does not reopen: %v", err)
	}
	got := rd.CustomXMLParts()
	if len(got) != 1 || len(got[0].SchemaRefs()) != 1 || got[0].SchemaRefs()[0] != nsURI {
		t.Fatalf("reopened schemaRefs = %+v", got)
	}
}

// TestSetDataBinding binds a content control to a custom-XML node and verifies
// the w:dataBinding element is emitted before the control-type child.
func TestSetDataBinding(t *testing.T) {
	fixture := fixtureWithDocument(t, fixtureWNS, `<w:body>`+
		`<w:sdt><w:sdtPr><w:tag w:val="t"/><w:text/></w:sdtPr>`+
		`<w:sdtContent><w:r><w:t>x</w:t></w:r></w:sdtContent></w:sdt></w:body>`)
	doc, err := OpenReader(bytes.NewReader(fixture), int64(len(fixture)))
	if err != nil {
		t.Fatal(err)
	}
	ctrls := doc.ContentControls()
	if len(ctrls) != 1 {
		t.Fatalf("controls = %d", len(ctrls))
	}
	ctrls[0].SetDataBinding("/root[1]/a[1]", "{ABC}")

	xpath, sid, _, ok := ctrls[0].DataBinding()
	if !ok || xpath != "/root[1]/a[1]" || sid != "{ABC}" {
		t.Fatalf("DataBinding = %q %q %v", xpath, sid, ok)
	}

	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	out, _ := zipEntry(t, saved, "word/document.xml")
	s := string(out)
	db := `<w:dataBinding w:xpath="/root[1]/a[1]" w:storeItemID="{ABC}"/>`
	if !strings.Contains(s, db) {
		t.Fatalf("dataBinding not emitted as expected.\ngot: %s", s)
	}
	// Must sit before the control-type child (schema order).
	if strings.Index(s, db) > strings.Index(s, `<w:text/>`) {
		t.Fatalf("dataBinding emitted after control child.\ngot: %s", s)
	}

	// Reopen, replace the binding, and confirm a single element results.
	rd, err := OpenReader(bytes.NewReader(saved), int64(len(saved)))
	if err != nil {
		t.Fatal(err)
	}
	rc := rd.ContentControls()[0]
	rc.SetDataBinding("/root[1]/b[1]", "{XYZ}")
	saved2, err := rd.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	s2, _ := zipEntry(t, saved2, "word/document.xml")
	if strings.Count(string(s2), "<w:dataBinding") != 1 {
		t.Fatalf("expected exactly one dataBinding after replace: %s", s2)
	}
	if !strings.Contains(string(s2), `w:xpath="/root[1]/b[1]"`) {
		t.Fatalf("replaced xpath missing: %s", s2)
	}

	// Removal drops the element.
	if !rc.RemoveDataBinding() {
		t.Fatal("RemoveDataBinding returned false")
	}
	saved3, err := rd.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	s3, _ := zipEntry(t, saved3, "word/document.xml")
	if strings.Contains(string(s3), "<w:dataBinding") {
		t.Fatalf("dataBinding not removed: %s", s3)
	}
}

// TestSetDataBindingOnCreatedControl binds a programmatically added content
// control (whose w:sdtPr child order was built by the API, not parsed from a
// file) and confirms the binding lands before the control child.
func TestSetDataBindingOnCreatedControl(t *testing.T) {
	doc := Create()
	part, err := doc.AddCustomXMLPart([]byte(`<root xmlns="http://ex.com"><a>1</a></root>`))
	if err != nil {
		t.Fatal(err)
	}
	cc := doc.AddContentControl("bound", "1")
	cc.SetDataBinding("/ns0:root[1]/ns0:a[1]", part.ItemID())

	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	out, _ := zipEntry(t, saved, "word/document.xml")
	s := string(out)
	if !strings.Contains(s, `w:storeItemID="`+part.ItemID()+`"`) {
		t.Fatalf("storeItemID not emitted: %s", s)
	}
	if strings.Index(s, "<w:dataBinding") > strings.Index(s, "<w:richText") {
		t.Fatalf("dataBinding after control child: %s", s)
	}
	// Reopen and read the binding back.
	rd, err := OpenReader(bytes.NewReader(saved), int64(len(saved)))
	if err != nil {
		t.Fatal(err)
	}
	xpath, sid, _, ok := rd.ContentControls()[0].DataBinding()
	if !ok || sid != part.ItemID() || xpath != "/ns0:root[1]/ns0:a[1]" {
		t.Fatalf("read-back binding = %q %q %v", xpath, sid, ok)
	}
}
