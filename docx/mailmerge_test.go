package docx

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mgilbir/spine/docx/internal/oxml"
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
