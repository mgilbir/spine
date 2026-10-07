package docx

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mgilbir/spine/docx/internal/oxml"
	"github.com/mgilbir/spine/internal/testutil"
)

// sampleMailMerge is a representative configuration exercising the scalar
// fields plus an ODSO data source with a field mapping.
func sampleMailMerge() *MailMerge {
	return &MailMerge{
		MainDocumentType: MailMergeFormLetters,
		DataType:         "native",
		ConnectString:    "Provider=Microsoft.Office.15.0",
		Query:            "SELECT * FROM Sheet1$",
		LinkToQuery:      true,
		ViewMergedData:   true,
		Destination:      "newDocument",
		DataSourceRef:    "rId7",
		DataSource: &MailMergeDataSource{
			SourceRef:        "rId8",
			Table:            "Sheet1$",
			ConnectionType:   "spreadsheet",
			FirstRowHeader:   true,
			ColumnDelimiter:  9,
			UDLConnectString: "Provider=Microsoft.ACE.OLEDB.12.0",
			FieldMappings: []MailMergeFieldMapping{
				{Name: "FirstName", MappedName: "First Name", Column: 0, Type: "dbColumn", LanguageID: "1033"},
				{Name: "City", MappedName: "City", Column: 3, Type: "dbColumn"},
			},
		},
	}
}

// TestMailMergeRoundTrip: a configuration written with SetMailMerge reads back
// equal after a save/reopen.
func TestMailMergeRoundTrip(t *testing.T) {
	doc := Create()
	doc.AddParagraphWithText("body")
	doc.SetMailMerge(sampleMailMerge())

	data, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	// The settings part must carry the merge configuration.
	settings := zipPart(t, data, "word/settings.xml")
	for _, want := range []string{
		`<w:mailMerge>`,
		`<w:mainDocumentType w:val="formLetters"`,
		`<w:odso>`,
		`<w:fieldMapData>`,
		`<w:mappedName w:val="First Name"`,
	} {
		if !strings.Contains(settings, want) {
			t.Errorf("settings.xml missing %q\n%s", want, settings)
		}
	}

	doc2, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	got := doc2.MailMerge()
	if got == nil {
		t.Fatal("MailMerge() nil after reopen")
	}
	want := sampleMailMerge()
	if got.MainDocumentType != want.MainDocumentType || got.DataType != want.DataType ||
		got.ConnectString != want.ConnectString || got.Query != want.Query ||
		got.LinkToQuery != want.LinkToQuery || got.ViewMergedData != want.ViewMergedData ||
		got.Destination != want.Destination || got.DataSourceRef != want.DataSourceRef {
		t.Errorf("scalar mismatch:\n got=%+v\nwant=%+v", got, want)
	}
	if got.DataSource == nil {
		t.Fatal("DataSource nil after reopen")
	}
	gds, wds := got.DataSource, want.DataSource
	if gds.SourceRef != wds.SourceRef || gds.Table != wds.Table ||
		gds.ConnectionType != wds.ConnectionType || gds.FirstRowHeader != wds.FirstRowHeader ||
		gds.ColumnDelimiter != wds.ColumnDelimiter || gds.UDLConnectString != wds.UDLConnectString {
		t.Errorf("odso mismatch:\n got=%+v\nwant=%+v", gds, wds)
	}
	if len(gds.FieldMappings) != len(wds.FieldMappings) {
		t.Fatalf("field mappings: got %d want %d", len(gds.FieldMappings), len(wds.FieldMappings))
	}
	for i := range wds.FieldMappings {
		if gds.FieldMappings[i] != wds.FieldMappings[i] {
			t.Errorf("field mapping %d: got %+v want %+v", i, gds.FieldMappings[i], wds.FieldMappings[i])
		}
	}
}

// TestMailMergeReadDoesNotPerturb: reading MailMerge() must not modify the
// settings, so an unmodified reopen+save is byte-identical for the settings
// part (the raw child is preserved verbatim).
func TestMailMergeReadDoesNotPerturb(t *testing.T) {
	doc := Create()
	doc.AddParagraphWithText("body")
	doc.SetMailMerge(sampleMailMerge())
	data, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	doc2, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if doc2.MailMerge() == nil { // read, then do not modify
		t.Fatal("MailMerge() nil after reopen")
	}
	data2, err := doc2.SaveBytes()
	if err != nil {
		t.Fatalf("second SaveBytes: %v", err)
	}
	if a, b := zipPart(t, data, "word/settings.xml"), zipPart(t, data2, "word/settings.xml"); a != b {
		t.Errorf("settings.xml changed after read-only reopen+save:\n%s\n---\n%s", a, b)
	}
}

// TestMailMergeRemove: SetMailMerge(nil) drops the element.
func TestMailMergeRemove(t *testing.T) {
	doc := Create()
	doc.SetMailMerge(sampleMailMerge())
	doc.SetMailMerge(nil)
	data, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	if strings.Contains(zipPart(t, data, "word/settings.xml"), "mailMerge") {
		t.Error("mailMerge not removed")
	}
	doc2, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if doc2.MailMerge() != nil {
		t.Error("MailMerge() non-nil after removal")
	}
}

// TestAddAndReadMergeFields: AddMergeField writes MERGEFIELD simple fields and
// MergeFields reads the names back, distinct and in order.
func TestAddAndReadMergeFields(t *testing.T) {
	doc := Create()
	p := doc.AddParagraph()
	p.AddText("Dear ")
	p.AddMergeField("FirstName")
	p.AddText(" ")
	p.AddMergeField("Last Name") // whitespace -> quoted in instruction
	p2 := doc.AddParagraph()
	p2.AddMergeField("FirstName") // duplicate must collapse

	data, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	body := zipPart(t, data, "word/document.xml")
	if !strings.Contains(body, `w:instr=" MERGEFIELD FirstName \* MERGEFORMAT "`) {
		t.Errorf("FirstName merge field instruction missing:\n%s", body)
	}
	if !strings.Contains(body, `MERGEFIELD &#34;Last Name&#34;`) && !strings.Contains(body, `MERGEFIELD &quot;Last Name&quot;`) {
		t.Errorf("quoted merge field instruction missing:\n%s", body)
	}

	doc2, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	got := doc2.MergeFields()
	want := []string{"FirstName", "Last Name"}
	if len(got) != len(want) {
		t.Fatalf("MergeFields: got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("MergeFields[%d]: got %q want %q (%v)", i, got[i], want[i], got)
		}
	}
}

// TestReadComplexMergeField: a complex field (w:fldChar/w:instrText) whose
// instruction is split across two instrText runs is recognized by MergeFields.
func TestReadComplexMergeField(t *testing.T) {
	doc := Create()
	p := doc.AddParagraph()

	begin := &oxml.CT_R{}
	begin.AppendFldChar(&oxml.CT_FldChar{FldCharType: "begin"})
	i1 := &oxml.CT_R{}
	i1.AppendInstrText(&oxml.CT_Text{Space: "preserve", Text: " MERGEFIELD Ema"})
	i2 := &oxml.CT_R{}
	i2.AppendInstrText(&oxml.CT_Text{Space: "preserve", Text: `il \* MERGEFORMAT `})
	sep := &oxml.CT_R{}
	sep.AppendFldChar(&oxml.CT_FldChar{FldCharType: "separate"})
	res := &oxml.CT_R{}
	res.SetTexts([]*oxml.CT_Text{{Space: "preserve", Text: "«Email»"}})
	end := &oxml.CT_R{}
	end.AppendFldChar(&oxml.CT_FldChar{FldCharType: "end"})
	for _, r := range []*oxml.CT_R{begin, i1, i2, sep, res, end} {
		p.p.AppendR(r)
	}

	data, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	doc2, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	got := doc2.MergeFields()
	if len(got) != 1 || got[0] != "Email" {
		t.Fatalf("MergeFields: got %v want [Email]", got)
	}
}

// mailMergeSettingsFixture builds a mail-merge main document whose settings
// part links an external data source (rId1, referenced twice: w:dataSource and
// w:odso/w:src), an internal recipient-data part (rId2), and an attached
// template (rId3) that has nothing to do with the merge.
func mailMergeSettingsFixture(t *testing.T) []byte {
	t.Helper()
	const relsNS = `http://schemas.openxmlformats.org/officeDocument/2006/relationships`
	return buildFixtureDocx(t, map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n" +
			`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
			`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
			`<Default Extension="xml" ContentType="application/xml"/>` +
			`<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>` +
			`<Override PartName="/word/settings.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.settings+xml"/>` +
			`<Override PartName="/word/recipientData.xml" ContentType="application/vnd.ms-word.mailMergeRecipientData+xml"/>` +
			`</Types>`,
		"_rels/.rels": fixtureRootRels,
		"word/document.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n" +
			`<w:document ` + fixtureWNS + `><w:body><w:p><w:r><w:t>Body</w:t></w:r></w:p></w:body></w:document>`,
		"word/_rels/document.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n" +
			`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId1" Type="` + relsNS + `/settings" Target="settings.xml"/>` +
			`</Relationships>`,
		"word/settings.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n" +
			`<w:settings ` + fixtureWNS + `>` +
			`<w:attachedTemplate r:id="rId3"/>` +
			`<w:mailMerge><w:mainDocumentType w:val="formLetters"/><w:dataType w:val="textFile"/>` +
			`<w:query w:val="SELECT * FROM recipients.csv"/><w:dataSource r:id="rId1"/>` +
			`<w:odso><w:src r:id="rId1"/><w:recipientData r:id="rId2"/></w:odso></w:mailMerge>` +
			`</w:settings>`,
		"word/_rels/settings.xml.rels": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n" +
			`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
			`<Relationship Id="rId3" Type="` + relsNS + `/attachedTemplate" Target="file:///C:/templates/letter.dotx" TargetMode="External"/>` +
			`<Relationship Id="rId2" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/recipientData" Target="recipientData.xml"/>` +
			`<Relationship Id="rId1" Type="` + relsNS + `/mailMergeSource" Target="file:///C:/merge/recipients.csv" TargetMode="External"/>` +
			`</Relationships>`,
		"word/recipientData.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n" +
			`<w:recipients ` + fixtureWNS + `><w:recipientData><w:active w:val="1"/></w:recipientData></w:recipients>`,
	})
}

// TestMailMergeRemoveReleasesDataSource checks that removing the merge
// configuration also removes the relationships only it referenced and the
// recipient-data part, while an unrelated settings relationship survives.
func TestMailMergeRemoveReleasesDataSource(t *testing.T) {
	data := mailMergeSettingsFixture(t)
	doc, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	if doc.MailMerge() == nil {
		t.Fatal("fixture has no mail-merge configuration")
	}

	doc.SetMailMerge(nil)
	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("save: %v", err)
	}

	rels := zipEntryString(t, saved, "word/_rels/settings.xml.rels")
	if strings.Contains(rels, "recipients.csv") || strings.Contains(rels, "mailMergeSource") {
		t.Errorf("the data-source relationship survived:\n%s", rels)
	}
	if strings.Contains(rels, "recipientData") {
		t.Errorf("the recipient-data relationship survived:\n%s", rels)
	}
	if !strings.Contains(rels, `Id="rId3"`) || !strings.Contains(rels, "letter.dotx") {
		t.Errorf("the attached-template relationship was lost:\n%s", rels)
	}
	if _, ok := zipEntry(t, saved, "word/recipientData.xml"); ok {
		t.Error("the recipient-data part is still in the package")
	}
	if ct := zipEntryString(t, saved, "[Content_Types].xml"); strings.Contains(ct, "recipientData.xml") {
		t.Errorf("content types still name the recipient-data part:\n%s", ct)
	}
	settings := zipEntryString(t, saved, "word/settings.xml")
	if strings.Contains(settings, "mailMerge") {
		t.Errorf("w:mailMerge survived:\n%s", settings)
	}
	if !strings.Contains(settings, `<w:attachedTemplate r:id="rId3"`) {
		t.Errorf("w:attachedTemplate was lost:\n%s", settings)
	}

	re, err := OpenReader(bytes.NewReader(saved), int64(len(saved)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if re.MailMerge() != nil {
		t.Error("reopened document still has a mail-merge configuration")
	}
}

// TestMailMergeRemoveKeepsStillReferencedRelationship checks that a
// relationship the merge configuration shares with another settings element is
// kept.
func TestMailMergeRemoveKeepsStillReferencedRelationship(t *testing.T) {
	data := mailMergeSettingsFixture(t)
	// Point the attached template at the merge data source's relationship.
	data = editZipEntry(t, data, "word/settings.xml", func(s string) string {
		return strings.Replace(s, `<w:attachedTemplate r:id="rId3"/>`, `<w:attachedTemplate r:id="rId1"/>`, 1)
	})
	doc, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	doc.SetMailMerge(nil)
	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	rels := zipEntryString(t, saved, "word/_rels/settings.xml.rels")
	if !strings.Contains(rels, `Id="rId1"`) {
		t.Errorf("a relationship still referenced by w:attachedTemplate was removed:\n%s", rels)
	}
	if strings.Contains(rels, `Id="rId2"`) {
		t.Errorf("the recipient-data relationship survived:\n%s", rels)
	}
}

// TestMailMergeRemoveKeepsSharedPart checks that a part the recipient-data
// relationship targets is kept when another relationship still targets it.
func TestMailMergeRemoveKeepsSharedPart(t *testing.T) {
	data := mailMergeSettingsFixture(t)
	data = editZipEntry(t, data, "word/_rels/document.xml.rels", func(s string) string {
		return strings.Replace(s, `</Relationships>`,
			`<Relationship Id="rId9" Type="http://example.com/relationships/other" Target="recipientData.xml"/></Relationships>`, 1)
	})
	doc, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	doc.SetMailMerge(nil)
	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if _, ok := zipEntry(t, saved, "word/recipientData.xml"); !ok {
		t.Error("a part another relationship still targets was dropped")
	}
	if ct := zipEntryString(t, saved, "[Content_Types].xml"); !strings.Contains(ct, "recipientData.xml") {
		t.Errorf("the kept part lost its content type:\n%s", ct)
	}
}

// TestMailMergeRemoveWithoutRelationships checks that removing a merge
// configuration that references no relationship leaves the settings .rels
// untouched.
func TestMailMergeRemoveWithoutRelationships(t *testing.T) {
	data := mailMergeSettingsFixture(t)
	data = editZipEntry(t, data, "word/settings.xml", func(s string) string {
		s = strings.Replace(s, `<w:dataSource r:id="rId1"/>`, "", 1)
		return strings.Replace(s, `<w:odso><w:src r:id="rId1"/><w:recipientData r:id="rId2"/></w:odso>`, "", 1)
	})
	before, _ := zipEntry(t, data, "word/_rels/settings.xml.rels")
	doc, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	doc.SetMailMerge(nil)
	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	after, _ := zipEntry(t, saved, "word/_rels/settings.xml.rels")
	if !bytes.Equal(before, after) {
		t.Errorf("settings .rels changed:\nwant %s\ngot  %s", before, after)
	}
}

// TestMailMergeRemoveDropsEmptySettingsRels checks that the settings .rels
// part is left out once removing the merge configuration empties it.
func TestMailMergeRemoveDropsEmptySettingsRels(t *testing.T) {
	data := mailMergeSettingsFixture(t)
	data = editZipEntry(t, data, "word/settings.xml", func(s string) string {
		return strings.Replace(s, `<w:attachedTemplate r:id="rId3"/>`, "", 1)
	})
	data = editZipEntry(t, data, "word/_rels/settings.xml.rels", func(s string) string {
		i := strings.Index(s, `<Relationship Id="rId3"`)
		j := i + strings.Index(s[i:], "/>") + len("/>")
		return s[:i] + s[j:]
	})
	doc, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	doc.SetMailMerge(nil)
	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if rels, ok := zipEntry(t, saved, "word/_rels/settings.xml.rels"); ok {
		t.Errorf("an empty settings .rels was written:\n%s", rels)
	}
	if _, err := OpenReader(bytes.NewReader(saved), int64(len(saved))); err != nil {
		t.Fatalf("reopen: %v", err)
	}
}

// TestMailMergeRemoveThenSetRestoresRelationship checks that a configuration
// set again after removal, referencing a released relationship by id, gets the
// relationship back instead of a dangling r:id.
func TestMailMergeRemoveThenSetRestoresRelationship(t *testing.T) {
	data := mailMergeSettingsFixture(t)
	doc, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	mm := doc.MailMerge()
	if mm == nil || mm.DataSourceRef != "rId1" {
		t.Fatalf("fixture configuration = %+v", mm)
	}
	doc.SetMailMerge(nil)
	mm.Query = "SELECT 1"
	doc.SetMailMerge(mm)
	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	settings := zipEntryString(t, saved, "word/settings.xml")
	if !strings.Contains(settings, `<w:dataSource r:id="rId1"/>`) {
		t.Fatalf("data source reference missing:\n%s", settings)
	}
	rels := zipEntryString(t, saved, "word/_rels/settings.xml.rels")
	if !strings.Contains(rels, `Id="rId1"`) || !strings.Contains(rels, "recipients.csv") {
		t.Errorf("the referenced data-source relationship was not restored:\n%s", rels)
	}
	// The recipient data is not part of the new configuration: it stays gone.
	if strings.Contains(rels, `Id="rId2"`) {
		t.Errorf("the recipient-data relationship came back:\n%s", rels)
	}
	if _, ok := zipEntry(t, saved, "word/recipientData.xml"); ok {
		t.Error("the recipient-data part came back")
	}
}

// TestMailMergeRemoveThenSetRestoresDroppedPart checks that restoring a
// released relationship to an internal part brings the dropped part back.
func TestMailMergeRemoveThenSetRestoresDroppedPart(t *testing.T) {
	data := mailMergeSettingsFixture(t)
	data = editZipEntry(t, data, "word/_rels/settings.xml.rels", func(s string) string {
		return strings.Replace(s, `Target="file:///C:/merge/recipients.csv" TargetMode="External"`, `Target="mergeSource.xml"`, 1)
	})
	parts, err := testutil.ReadZipPartsBytes(data)
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{}
	for name, b := range parts {
		files[name] = string(b)
	}
	files["word/mergeSource.xml"] = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n" + `<source/>`
	data = buildFixtureDocx(t, files)

	doc, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	mm := doc.MailMerge()
	doc.SetMailMerge(nil)
	removed, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if _, ok := zipEntry(t, removed, "word/mergeSource.xml"); ok {
		t.Fatal("the released source part was not dropped")
	}

	doc.SetMailMerge(mm)
	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if _, ok := zipEntry(t, saved, "word/mergeSource.xml"); !ok {
		t.Error("the source part did not come back with its relationship")
	}
	if rels := zipEntryString(t, saved, "word/_rels/settings.xml.rels"); !strings.Contains(rels, `Target="mergeSource.xml"`) {
		t.Errorf("relationship not restored:\n%s", rels)
	}
}

// editZipEntry rewrites one entry of a package through edit.
func editZipEntry(t *testing.T, data []byte, name string, edit func(string) string) []byte {
	t.Helper()
	return rewriteZipEntry(t, data, name, edit(zipEntryString(t, data, name)))
}
