package docx

import (
	"bytes"
	"strings"
	"testing"
)

// C27: sectPr children that were parsed-then-skipped or never marshaled
// (lnNumType, formProt, vAlign, noEndnote, textDirection, bidi, rtlGutter,
// paperSrc, printerSettings, sectPrChange) were stripped from every saved
// document. A sectPr carrying all of them must round-trip byte-faithfully.
func TestSectPrChildrenRoundTrip(t *testing.T) {
	sectPr := `<w:sectPr w:rsidR="00AA11BB">` +
		`<w:footnotePr><w:pos w:val="pageBottom"/></w:footnotePr>` +
		`<w:endnotePr><w:pos w:val="docEnd"/></w:endnotePr>` +
		`<w:type w:val="nextPage"/>` +
		`<w:pgSz w:w="12240" w:h="15840"/>` +
		`<w:pgMar w:top="1440" w:right="1440" w:bottom="1440" w:left="1440" w:header="708" w:footer="708" w:gutter="0"/>` +
		`<w:paperSrc w:first="7" w:other="7"/>` +
		`<w:pgBorders w:offsetFrom="page"><w:top w:val="single" w:sz="4" w:space="24" w:color="auto"/></w:pgBorders>` +
		`<w:lnNumType w:countBy="1" w:start="1" w:restart="continuous"/>` +
		`<w:pgNumType w:start="1"/>` +
		`<w:cols w:space="708"/>` +
		`<w:formProt w:val="true"/>` +
		`<w:vAlign w:val="center"/>` +
		`<w:noEndnote/>` +
		`<w:titlePg/>` +
		`<w:textDirection w:val="tbRl"/>` +
		`<w:bidi/>` +
		`<w:rtlGutter/>` +
		`<w:docGrid w:linePitch="360"/>` +
		`<w:sectPrChange w:id="5" w:author="Reviewer" w:date="2024-01-01T00:00:00Z">` +
		`<w:sectPr><w:pgSz w:w="11906" w:h="16838"/></w:sectPr>` +
		`</w:sectPrChange>` +
		`</w:sectPr>`
	fixture := fixtureWithDocument(t, fixtureWNS,
		`<w:body><w:p><w:r><w:t>x</w:t></w:r></w:p>`+sectPr+`</w:body>`)

	out := openSave(t, fixture)
	if !strings.Contains(out, sectPr) {
		t.Fatalf("sectPr not preserved byte-faithfully.\nwant substring:\n%s\ngot document:\n%s", sectPr, out)
	}
}

// C27: printerSettings carries an r:id and must survive as well (raw capture).
func TestSectPrPrinterSettingsRoundTrip(t *testing.T) {
	sectPr := `<w:sectPr><w:pgSz w:w="12240" w:h="15840"/><w:printerSettings r:id="rId9"/></w:sectPr>`
	fixture := fixtureWithDocument(t, fixtureWNS,
		`<w:body><w:p/>`+sectPr+`</w:body>`)
	out := openSave(t, fixture)
	if !strings.Contains(out, `<w:printerSettings r:id="rId9"/>`) {
		t.Fatalf("printerSettings dropped:\n%s", out)
	}
}

// C28: body-level children the model does not type (w:altChunk, w:customXml,
// move range markers) were silently dropped on save. They must round-trip
// verbatim, in position.
func TestBodyLevelRawChildrenRoundTrip(t *testing.T) {
	body := `<w:body>` +
		`<w:altChunk r:id="rId9"/>` +
		`<w:customXml w:uri="urn:x" w:element="node"><w:p><w:r><w:t>CXTEXT</w:t></w:r></w:p></w:customXml>` +
		`<w:p><w:r><w:t>after</w:t></w:r></w:p>` +
		`</w:body>`
	fixture := fixtureWithDocument(t, fixtureWNS, body)
	out := openSave(t, fixture)
	if !strings.Contains(out, body) {
		t.Fatalf("body-level altChunk/customXml not preserved verbatim:\n%s", out)
	}
}

// C28: inline customXml and smartTag carry runs whose text was lost entirely.
func TestInlineCustomXmlAndSmartTagRoundTrip(t *testing.T) {
	para := `<w:p>` +
		`<w:customXml w:uri="urn:y" w:element="inline"><w:r><w:t>INLINECX</w:t></w:r></w:customXml>` +
		`<w:smartTag w:uri="urn:z" w:element="tag"><w:smartTagPr><w:attr w:name="a" w:val="b"/></w:smartTagPr><w:r><w:t>SMART</w:t></w:r></w:smartTag>` +
		`<w:r><w:t>tail</w:t></w:r>` +
		`</w:p>`
	fixture := fixtureWithDocument(t, fixtureWNS, `<w:body>`+para+`</w:body>`)
	out := openSave(t, fixture)
	if !strings.Contains(out, para) {
		t.Fatalf("inline customXml/smartTag not preserved verbatim:\n%s", out)
	}
}

// C28: tracked-move content (w:moveFrom/w:moveTo and their range markers) was
// dropped, deleting the moved text from both its source and destination. A
// tracked-move document must keep the moved text through open -> save.
func TestTrackedMoveContentPreserved(t *testing.T) {
	from := `<w:p>` +
		`<w:moveFromRangeStart w:id="1" w:author="A" w:date="2024-01-01T00:00:00Z" w:name="move1"/>` +
		`<w:moveFrom w:id="2" w:author="A" w:date="2024-01-01T00:00:00Z"><w:r><w:t>MOVEDTEXT</w:t></w:r></w:moveFrom>` +
		`<w:moveFromRangeEnd w:id="1"/>` +
		`</w:p>`
	to := `<w:p>` +
		`<w:moveToRangeStart w:id="3" w:author="A" w:date="2024-01-01T00:00:00Z" w:name="move1"/>` +
		`<w:moveTo w:id="4" w:author="A" w:date="2024-01-01T00:00:00Z"><w:r><w:t>MOVEDTEXT</w:t></w:r></w:moveTo>` +
		`<w:moveToRangeEnd w:id="3"/>` +
		`</w:p>`
	fixture := fixtureWithDocument(t, fixtureWNS, `<w:body>`+from+to+`</w:body>`)
	out := openSave(t, fixture)
	if !strings.Contains(out, from) {
		t.Fatalf("moveFrom content not preserved:\n%s", out)
	}
	if !strings.Contains(out, to) {
		t.Fatalf("moveTo content not preserved:\n%s", out)
	}
	if got := strings.Count(out, "MOVEDTEXT"); got != 2 {
		t.Fatalf("moved text should appear twice (source and destination), got %d", got)
	}
}

// C28: row-level w:tblPrEx (table property exceptions) was dropped.
func TestRowTblPrExRoundTrip(t *testing.T) {
	row := `<w:tr>` +
		`<w:tblPrEx><w:tblBorders><w:top w:val="single" w:sz="4" w:space="0" w:color="auto"/></w:tblBorders></w:tblPrEx>` +
		`<w:tc><w:p><w:r><w:t>CELL</w:t></w:r></w:p></w:tc>` +
		`</w:tr>`
	body := `<w:body><w:tbl><w:tblGrid><w:gridCol/></w:tblGrid>` + row + `</w:tbl><w:p/></w:body>`
	fixture := fixtureWithDocument(t, fixtureWNS, body)
	out := openSave(t, fixture)
	if !strings.Contains(out, row) {
		t.Fatalf("row tblPrEx not preserved verbatim:\n%s", out)
	}
}

// The childOrder-backfill invariant: mutating a parsed document must not drop
// the raw-preserved children, and their document position must hold.
func TestRawChildrenSurviveMutation(t *testing.T) {
	body := `<w:body>` +
		`<w:altChunk r:id="rId9"/>` +
		`<w:p><w:smartTag w:uri="urn:z" w:element="tag"><w:r><w:t>SMART</w:t></w:r></w:smartTag></w:p>` +
		`</w:body>`
	fixture := fixtureWithDocument(t, fixtureWNS, body)

	doc, err := OpenReader(bytes.NewReader(fixture), int64(len(fixture)))
	if err != nil {
		t.Fatal(err)
	}
	doc.AddParagraphWithText("added")
	paras := doc.Paragraphs()
	paras[0].AddRun().SetText("appended-run")

	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	data, ok := zipEntry(t, saved, "word/document.xml")
	if !ok {
		t.Fatal("word/document.xml missing")
	}
	out := string(data)
	for _, want := range []string{
		`<w:altChunk r:id="rId9"/>`,
		`<w:smartTag w:uri="urn:z" w:element="tag"><w:r><w:t>SMART</w:t></w:r></w:smartTag>`,
		`appended-run`,
		`added`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q after mutation:\n%s", want, out)
		}
	}
	// The altChunk must still precede the paragraphs.
	if strings.Index(out, "altChunk") > strings.Index(out, "SMART") {
		t.Error("altChunk lost its body position")
	}
}
