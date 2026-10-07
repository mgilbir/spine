package docx

import (
	"bytes"
	"encoding/xml"
	"reflect"
	"strings"
	"testing"
)

// wordMergeField is the markup Word writes for a MERGEFIELD it shows as a
// «name» placeholder: begin, instruction, separate, result and end runs, each
// with the same run properties.
func wordMergeField(instr, result, rPr string) string {
	run := func(content string) string { return `<w:r>` + rPr + content + `</w:r>` }
	return run(`<w:fldChar w:fldCharType="begin"/>`) +
		run(`<w:instrText xml:space="preserve">`+instr+`</w:instrText>`) +
		run(`<w:fldChar w:fldCharType="separate"/>`) +
		run(`<w:t>`+result+`</w:t>`) +
		run(`<w:fldChar w:fldCharType="end"/>`)
}

func fillBody(t *testing.T, body string, values map[string]string) (*Document, []string, string) {
	t.Helper()
	doc := openDocFixture(t, body)
	unfilled := doc.FillMergeFields(values)
	return doc, unfilled, saveDocXML(t, doc)
}

func TestFillMergeFieldsComplexField(t *testing.T) {
	body := `<w:body><w:p>` +
		`<w:r><w:t xml:space="preserve">Dear </w:t></w:r>` +
		wordMergeField(` MERGEFIELD FirstName `, `«FirstName»`, `<w:rPr><w:i/></w:rPr>`) +
		`<w:r><w:t>,</w:t></w:r>` +
		`</w:p></w:body>`
	doc, unfilled, xml := fillBody(t, body, map[string]string{"FirstName": "Ada"})

	if len(unfilled) != 0 {
		t.Errorf("unfilled = %v, want none", unfilled)
	}
	if got := doc.Paragraphs()[0].Text(); got != "Dear Ada," {
		t.Errorf("paragraph text = %q, want %q", got, "Dear Ada,")
	}
	if strings.Contains(xml, "fldChar") || strings.Contains(xml, "instrText") {
		t.Errorf("field markup survived:\n%s", xml)
	}
	if !strings.Contains(xml, `<w:r><w:rPr><w:i/></w:rPr><w:t xml:space="preserve">Ada</w:t></w:r>`) {
		t.Errorf("value did not take the result's formatting:\n%s", xml)
	}
	if got := doc.MergeFields(); len(got) != 0 {
		t.Errorf("MergeFields after fill = %v, want none", got)
	}
}

// TestFillMergeFieldsSplitInstruction covers an instruction Word split across
// several runs, a quoted name with a space, and a name used twice.
func TestFillMergeFieldsSplitInstruction(t *testing.T) {
	body := `<w:body><w:p>` +
		`<w:r><w:fldChar w:fldCharType="begin"/></w:r>` +
		`<w:r><w:instrText xml:space="preserve"> MERGE</w:instrText></w:r>` +
		`<w:r><w:instrText xml:space="preserve">FIELD "Last Name" \* MERGEFORMAT </w:instrText></w:r>` +
		`<w:r><w:fldChar w:fldCharType="separate"/></w:r>` +
		`<w:r><w:t>«Last</w:t></w:r><w:r><w:t xml:space="preserve"> Name»</w:t></w:r>` +
		`<w:r><w:fldChar w:fldCharType="end"/></w:r>` +
		`</w:p><w:p>` +
		wordMergeField(` MERGEFIELD "Last Name" `, `«Last Name»`, "") +
		`</w:p></w:body>`
	doc, unfilled, _ := fillBody(t, body, map[string]string{"Last Name": "Lovelace"})

	if len(unfilled) != 0 {
		t.Errorf("unfilled = %v, want none", unfilled)
	}
	for i, p := range doc.Paragraphs() {
		if got := p.Text(); got != "Lovelace" {
			t.Errorf("paragraph %d text = %q, want %q", i, got, "Lovelace")
		}
	}
}

func TestFillMergeFieldsMissingValueLeavesField(t *testing.T) {
	body := `<w:body><w:p>` +
		wordMergeField(` MERGEFIELD City `, `«City»`, "") +
		wordMergeField(` MERGEFIELD Zip `, `«Zip»`, "") +
		wordMergeField(` MERGEFIELD City `, `«City»`, "") +
		`</w:p></w:body>`
	baseline := saveDocXML(t, openDocFixture(t, body))

	doc, unfilled, xml := fillBody(t, body, map[string]string{"Other": "x"})
	if want := []string{"City", "Zip"}; !reflect.DeepEqual(unfilled, want) {
		t.Errorf("unfilled = %v, want %v", unfilled, want)
	}
	if xml != baseline {
		t.Errorf("a fill that replaced nothing changed the document:\nwant %s\ngot  %s", baseline, xml)
	}
	if got := doc.MergeFields(); !reflect.DeepEqual(got, []string{"City", "Zip"}) {
		t.Errorf("MergeFields = %v", got)
	}
}

// TestFillMergeFieldsLeavesOtherFields checks that fields other than
// MERGEFIELD are not touched or reported.
func TestFillMergeFieldsLeavesOtherFields(t *testing.T) {
	body := `<w:body><w:p>` +
		wordMergeField(` PAGE `, `1`, "") +
		wordMergeField(` MERGEFIELD City `, `«City»`, "") +
		`</w:p></w:body>`
	doc, unfilled, xml := fillBody(t, body, map[string]string{"City": "Paris"})
	if len(unfilled) != 0 {
		t.Errorf("unfilled = %v, want none", unfilled)
	}
	if !strings.Contains(xml, `> PAGE </w:instrText>`) {
		t.Errorf("the PAGE field was touched:\n%s", xml)
	}
	if got := doc.Paragraphs()[0].Text(); got != "1Paris" {
		t.Errorf("paragraph text = %q, want %q", got, "1Paris")
	}
}

// TestFillMergeFieldsSimpleField fills the w:fldSimple fields AddMergeField
// writes, keeping the placeholder run's formatting.
func TestFillMergeFieldsSimpleField(t *testing.T) {
	doc := Create()
	p := doc.AddParagraph()
	p.AddText("Hello ")
	p.AddMergeField("FirstName").SetBold(true)
	p.AddText("!")

	unfilled := doc.FillMergeFields(map[string]string{"FirstName": "Grace"})
	if len(unfilled) != 0 {
		t.Errorf("unfilled = %v, want none", unfilled)
	}
	if got := p.Text(); got != "Hello Grace!" {
		t.Errorf("paragraph text = %q, want %q", got, "Hello Grace!")
	}
	xml := saveDocXML(t, doc)
	if strings.Contains(xml, "fldSimple") {
		t.Errorf("the simple field survived:\n%s", xml)
	}
	if !strings.Contains(xml, `<w:b/></w:rPr><w:t xml:space="preserve">Grace</w:t>`) {
		t.Errorf("value did not keep the placeholder's bold formatting:\n%s", xml)
	}
}

func TestFillMergeFieldsSwitches(t *testing.T) {
	cases := []struct {
		instr, value, want string
	}{
		{` MERGEFIELD Name \b "Dear " \f ", " `, "Ada", "Dear Ada, "},
		{` MERGEFIELD Name \b"Dear "\f"!" `, "Ada", "Dear Ada!"},
		{` MERGEFIELD Name \b "Dear " `, "", ""},
		{` MERGEFIELD Name \* Upper `, "Ada Lovelace", "ADA LOVELACE"},
		{` MERGEFIELD Name \* lower `, "ADA", "ada"},
		{` MERGEFIELD Name \* FirstCap `, "ada lovelace", "Ada lovelace"},
		{` MERGEFIELD Name \* Caps `, "ada  lovelace", "Ada  Lovelace"},
		{` MERGEFIELD Name \* Caps `, "ADA LOVELACE", "Ada Lovelace"},
		{` MERGEFIELD Name \* FirstCap `, "aDA LOVELACE", "ADA LOVELACE"},
		{` MERGEFIELD Name \* Lower \* FirstCap `, "ADA LOVELACE", "Ada lovelace"},
		{` MERGEFIELD Name \*Upper \b "x " `, "ada", "x ADA"},
		{` MERGEFIELD Name \* MERGEFORMAT `, "Ada", "Ada"},
		{` MERGEFIELD Name \@ "dd.MM.yyyy" `, "01.02.2003", "01.02.2003"},
	}
	for _, c := range cases {
		if got := mergeFieldText(c.value, c.instr); got != c.want {
			t.Errorf("mergeFieldText(%q, %q) = %q, want %q", c.value, c.instr, got, c.want)
		}
	}
}

// TestFillMergeFieldsEmptyValueRemovesField checks that an empty value leaves
// no field and no empty run behind.
func TestFillMergeFieldsEmptyValueRemovesField(t *testing.T) {
	body := `<w:body><w:p><w:r><w:t>a</w:t></w:r>` +
		wordMergeField(` MERGEFIELD Middle \b " " `, `«Middle»`, "") +
		`<w:r><w:t>b</w:t></w:r></w:p></w:body>`
	doc, unfilled, xml := fillBody(t, body, map[string]string{"Middle": ""})
	if len(unfilled) != 0 {
		t.Errorf("unfilled = %v, want none", unfilled)
	}
	if got := doc.Paragraphs()[0].Text(); got != "ab" {
		t.Errorf("paragraph text = %q, want %q", got, "ab")
	}
	if strings.Count(xml, "<w:r>") != 2 {
		t.Errorf("want exactly the two surrounding runs:\n%s", xml)
	}
}

func TestFillMergeFieldsLineBreaksAndTabs(t *testing.T) {
	body := `<w:body><w:p>` + wordMergeField(` MERGEFIELD Address `, `«Address»`, "") + `</w:p></w:body>`
	_, _, xml := fillBody(t, body, map[string]string{"Address": "1 Main St\r\nSpringfield\tUSA\nEarth"})
	want := `<w:r><w:t xml:space="preserve">1 Main St</w:t><w:br/><w:t xml:space="preserve">Springfield</w:t>` +
		`<w:tab/><w:t xml:space="preserve">USA</w:t><w:br/><w:t xml:space="preserve">Earth</w:t></w:r>`
	if !strings.Contains(xml, want) {
		t.Errorf("value runs:\n%s\nwant %s", xml, want)
	}
}

// TestFillMergeFieldsKeepsMarkers checks that bookmark and proofing markers in
// a field survive, start markers before the value and end markers after it.
func TestFillMergeFieldsKeepsMarkers(t *testing.T) {
	body := `<w:body><w:p>` +
		`<w:bookmarkStart w:id="1" w:name="outside"/>` +
		`<w:r><w:fldChar w:fldCharType="begin"/></w:r>` +
		`<w:r><w:instrText xml:space="preserve"> MERGEFIELD City </w:instrText></w:r>` +
		`<w:r><w:fldChar w:fldCharType="separate"/></w:r>` +
		`<w:bookmarkStart w:id="0" w:name="inside"/>` +
		`<w:proofErr w:type="spellStart"/>` +
		`<w:r><w:t>«City»</w:t></w:r>` +
		`<w:proofErr w:type="spellEnd"/>` +
		`<w:bookmarkEnd w:id="1"/>` +
		`<w:bookmarkEnd w:id="0"/>` +
		`<w:r><w:fldChar w:fldCharType="end"/></w:r>` +
		`</w:p></w:body>`
	_, _, xml := fillBody(t, body, map[string]string{"City": "Oslo"})
	want := `<w:bookmarkStart w:id="1" w:name="outside"/>` +
		`<w:bookmarkStart w:id="0" w:name="inside"/><w:proofErr w:type="spellStart"/>` +
		`<w:r><w:t xml:space="preserve">Oslo</w:t></w:r>` +
		`<w:proofErr w:type="spellEnd"/><w:bookmarkEnd w:id="1"/><w:bookmarkEnd w:id="0"/>`
	if !strings.Contains(xml, want) {
		t.Errorf("markers around the value:\n%s\nwant %s", xml, want)
	}
}

// TestFillMergeFieldsInsideIFCondition checks that a MERGEFIELD in the
// condition of an IF field becomes a quoted instruction argument and the IF
// stays.
func TestFillMergeFieldsInsideIFCondition(t *testing.T) {
	body := `<w:body><w:p>` +
		`<w:r><w:fldChar w:fldCharType="begin"/></w:r>` +
		`<w:r><w:instrText xml:space="preserve"> IF </w:instrText></w:r>` +
		wordMergeField(` MERGEFIELD Gender `, `«Gender»`, "") +
		`<w:r><w:instrText xml:space="preserve"> = "f" "Dear Ms" "Dear Mr" </w:instrText></w:r>` +
		`<w:r><w:fldChar w:fldCharType="separate"/></w:r>` +
		`<w:r><w:t>Dear Mr</w:t></w:r>` +
		`<w:r><w:fldChar w:fldCharType="end"/></w:r>` +
		`</w:p></w:body>`
	_, unfilled, xml := fillBody(t, body, map[string]string{"Gender": "f"})
	if len(unfilled) != 0 {
		t.Errorf("unfilled = %v, want none", unfilled)
	}
	if strings.Contains(xml, "MERGEFIELD") {
		t.Errorf("the MERGEFIELD survived:\n%s", xml)
	}
	want := `<w:instrText xml:space="preserve"> IF </w:instrText></w:r>` +
		`<w:r><w:instrText xml:space="preserve">"f"</w:instrText></w:r>` +
		`<w:r><w:instrText xml:space="preserve"> = "f" "Dear Ms" "Dear Mr" </w:instrText></w:r>`
	if !strings.Contains(xml, want) {
		t.Errorf("IF instruction:\n%s\nwant %s", xml, want)
	}
	if strings.Count(xml, `w:fldCharType="begin"`) != 1 || strings.Count(xml, `w:fldCharType="end"`) != 1 {
		t.Errorf("the IF field's characters were not kept:\n%s", xml)
	}
}

// TestFillMergeFieldsInsideIFResult checks that a MERGEFIELD in an IF result
// becomes ordinary text.
func TestFillMergeFieldsInsideIFResult(t *testing.T) {
	body := `<w:body><w:p>` +
		`<w:r><w:fldChar w:fldCharType="begin"/></w:r>` +
		`<w:r><w:instrText xml:space="preserve"> IF 1 = 1 "x" "y" </w:instrText></w:r>` +
		`<w:r><w:fldChar w:fldCharType="separate"/></w:r>` +
		wordMergeField(` MERGEFIELD City `, `«City»`, "") +
		`<w:r><w:fldChar w:fldCharType="end"/></w:r>` +
		`</w:p></w:body>`
	doc, unfilled, _ := fillBody(t, body, map[string]string{"City": "Rome"})
	if len(unfilled) != 0 {
		t.Errorf("unfilled = %v, want none", unfilled)
	}
	if got := doc.Paragraphs()[0].Text(); got != "Rome" {
		t.Errorf("paragraph text = %q, want %q", got, "Rome")
	}
}

// TestFillMergeFieldsSkipsUnsafeShapes checks the fields FillMergeFields will
// not rewrite: each is left exactly as it was and reported.
func TestFillMergeFieldsSkipsUnsafeShapes(t *testing.T) {
	cases := map[string]string{
		"spans paragraphs": `<w:p><w:r><w:fldChar w:fldCharType="begin"/></w:r>` +
			`<w:r><w:instrText xml:space="preserve"> MERGEFIELD Name </w:instrText></w:r>` +
			`<w:r><w:fldChar w:fldCharType="separate"/></w:r><w:r><w:t>«Na</w:t></w:r></w:p>` +
			`<w:p><w:r><w:t>me»</w:t></w:r><w:r><w:fldChar w:fldCharType="end"/></w:r></w:p>`,
		"result holds a picture": `<w:p><w:r><w:fldChar w:fldCharType="begin"/></w:r>` +
			`<w:r><w:instrText xml:space="preserve"> MERGEFIELD Name </w:instrText></w:r>` +
			`<w:r><w:fldChar w:fldCharType="separate"/></w:r>` +
			`<w:r><w:pict><v:shape xmlns:v="urn:schemas-microsoft-com:vml"/></w:pict></w:r>` +
			`<w:r><w:fldChar w:fldCharType="end"/></w:r></w:p>`,
		"locked": `<w:p><w:r><w:fldChar w:fldCharType="begin" w:fldLock="true"/></w:r>` +
			`<w:r><w:instrText xml:space="preserve"> MERGEFIELD Name </w:instrText></w:r>` +
			`<w:r><w:fldChar w:fldCharType="separate"/></w:r><w:r><w:t>«Name»</w:t></w:r>` +
			`<w:r><w:fldChar w:fldCharType="end"/></w:r></w:p>`,
		"field character shares its run": `<w:p><w:r><w:t>x</w:t><w:fldChar w:fldCharType="begin"/></w:r>` +
			`<w:r><w:instrText xml:space="preserve"> MERGEFIELD Name </w:instrText></w:r>` +
			`<w:r><w:fldChar w:fldCharType="separate"/></w:r><w:r><w:t>«Name»</w:t></w:r>` +
			`<w:r><w:fldChar w:fldCharType="end"/></w:r></w:p>`,
		"straddles a hyperlink": `<w:p><w:r><w:fldChar w:fldCharType="begin"/></w:r>` +
			`<w:r><w:instrText xml:space="preserve"> MERGEFIELD Name </w:instrText></w:r>` +
			`<w:r><w:fldChar w:fldCharType="separate"/></w:r>` +
			`<w:hyperlink w:anchor="a"><w:r><w:t>«Name»</w:t></w:r></w:hyperlink>` +
			`<w:r><w:fldChar w:fldCharType="end"/></w:r></w:p>`,
		"in a tracked deletion": `<w:p><w:del w:id="1" w:author="A" w:date="2020-01-01T00:00:00Z">` +
			`<w:r><w:fldChar w:fldCharType="begin"/></w:r>` +
			`<w:r><w:instrText xml:space="preserve"> MERGEFIELD Name </w:instrText></w:r>` +
			`<w:r><w:fldChar w:fldCharType="separate"/></w:r><w:r><w:delText>«Name»</w:delText></w:r>` +
			`<w:r><w:fldChar w:fldCharType="end"/></w:r></w:del></w:p>`,
		"never closed": `<w:p><w:r><w:fldChar w:fldCharType="begin"/></w:r>` +
			`<w:r><w:instrText xml:space="preserve"> MERGEFIELD Name </w:instrText></w:r></w:p>`,
		"locked simple field": `<w:p><w:fldSimple w:instr=" MERGEFIELD Name " w:fldLock="true">` +
			`<w:r><w:t>«Name»</w:t></w:r></w:fldSimple></w:p>`,
	}
	for name, paras := range cases {
		t.Run(name, func(t *testing.T) {
			body := `<w:body>` + paras + `</w:body>`
			baseline := saveDocXML(t, openDocFixture(t, body))
			_, unfilled, xml := fillBody(t, body, map[string]string{"Name": "Ada"})
			if !reflect.DeepEqual(unfilled, []string{"Name"}) {
				t.Errorf("unfilled = %v, want [Name]", unfilled)
			}
			if xml != baseline {
				t.Errorf("a skipped field changed the document:\nwant %s\ngot  %s", baseline, xml)
			}
		})
	}
}

// TestFillMergeFieldsInContainers fills fields inside a hyperlink, a tracked
// insertion, a content control and a table cell.
func TestFillMergeFieldsInContainers(t *testing.T) {
	f := wordMergeField(` MERGEFIELD Name `, `«Name»`, "")
	body := `<w:body>` +
		`<w:p><w:hyperlink w:anchor="a">` + f + `</w:hyperlink></w:p>` +
		`<w:p><w:ins w:id="1" w:author="A" w:date="2020-01-01T00:00:00Z">` + f + `</w:ins></w:p>` +
		`<w:p><w:sdt><w:sdtPr/><w:sdtContent>` + f + `</w:sdtContent></w:sdt></w:p>` +
		`<w:tbl><w:tblPr/><w:tblGrid><w:gridCol w:w="100"/></w:tblGrid><w:tr><w:tc><w:p>` + f + `</w:p></w:tc></w:tr></w:tbl>` +
		`</w:body>`
	doc, unfilled, xml := fillBody(t, body, map[string]string{"Name": "Ada"})
	if len(unfilled) != 0 {
		t.Errorf("unfilled = %v, want none", unfilled)
	}
	if strings.Contains(xml, "MERGEFIELD") {
		t.Errorf("a field survived:\n%s", xml)
	}
	if n := strings.Count(xml, ">Ada<"); n != 4 {
		t.Errorf("value written %d times, want 4:\n%s", n, xml)
	}
	if !strings.Contains(xml, `<w:hyperlink w:anchor="a"><w:r><w:t xml:space="preserve">Ada</w:t></w:r></w:hyperlink>`) {
		t.Errorf("hyperlink content:\n%s", xml)
	}
	if got := doc.MergeFields(); len(got) != 0 {
		t.Errorf("MergeFields after fill = %v, want none", got)
	}
}

func TestFillMergeFieldsHeader(t *testing.T) {
	doc := Create()
	doc.AddParagraphWithText("body")
	doc.AddHeader(HeaderDefault).AddParagraph().AddMergeField("Company")
	doc, _ = reopen(t, doc)

	unfilled := doc.FillMergeFields(map[string]string{"Company": "Acme"})
	if len(unfilled) != 0 {
		t.Errorf("unfilled = %v, want none", unfilled)
	}
	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	hdr := zipEntryString(t, saved, "word/header1.xml")
	if strings.Contains(hdr, "MERGEFIELD") || !strings.Contains(hdr, ">Acme<") {
		t.Errorf("header not filled:\n%s", hdr)
	}
	re, err := OpenReader(bytes.NewReader(saved), int64(len(saved)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if got := re.MergeFields(); len(got) != 0 {
		t.Errorf("MergeFields after reopen = %v, want none", got)
	}
}

// TestFillMergeFieldsLayoutHint checks that the page-break layout hint Word
// writes into a field's begin run does not stop the rewrite.
func TestFillMergeFieldsLayoutHint(t *testing.T) {
	body := `<w:body><w:p>` +
		`<w:r><w:lastRenderedPageBreak/><w:fldChar w:fldCharType="begin"/></w:r>` +
		`<w:r><w:instrText xml:space="preserve"> MERGEFIELD Notes </w:instrText></w:r>` +
		`<w:r><w:fldChar w:fldCharType="separate"/></w:r>` +
		`<w:r><w:t>«Notes»</w:t></w:r>` +
		`<w:r><w:fldChar w:fldCharType="end"/></w:r>` +
		`</w:p></w:body>`
	doc, unfilled, xml := fillBody(t, body, map[string]string{"Notes": "none"})
	if len(unfilled) != 0 {
		t.Errorf("unfilled = %v, want none", unfilled)
	}
	if got := doc.Paragraphs()[0].Text(); got != "none" || strings.Contains(xml, "fldChar") {
		t.Errorf("field not replaced (text %q):\n%s", got, xml)
	}
}

// TestFillMergeFieldsNoSeparate fills a field Word never showed a result for,
// which takes its begin run's formatting.
func TestFillMergeFieldsNoSeparate(t *testing.T) {
	body := `<w:body><w:p>` +
		`<w:r><w:rPr><w:b/></w:rPr><w:fldChar w:fldCharType="begin"/></w:r>` +
		`<w:r><w:instrText xml:space="preserve"> MERGEFIELD Code </w:instrText></w:r>` +
		`<w:r><w:fldChar w:fldCharType="end"/></w:r>` +
		`</w:p></w:body>`
	_, unfilled, xml := fillBody(t, body, map[string]string{"Code": "42"})
	if len(unfilled) != 0 {
		t.Errorf("unfilled = %v, want none", unfilled)
	}
	if !strings.Contains(xml, `<w:r><w:rPr><w:b/></w:rPr><w:t xml:space="preserve">42</w:t></w:r>`) {
		t.Errorf("value run:\n%s", xml)
	}
}

// TestFillMergeFieldsInstructionArguments checks how a value is written into
// an enclosing field's instruction: one quoted argument with quotes and
// backslashes escaped, only escaped inside an already quoted argument, and ""
// when empty.
func TestFillMergeFieldsInstructionArguments(t *testing.T) {
	ifField := func(before, after string) string {
		return `<w:body><w:p>` +
			`<w:r><w:fldChar w:fldCharType="begin"/></w:r>` +
			`<w:r><w:instrText xml:space="preserve">` + before + `</w:instrText></w:r>` +
			wordMergeField(` MERGEFIELD V `, `«V»`, "") +
			`<w:r><w:instrText xml:space="preserve">` + after + `</w:instrText></w:r>` +
			`<w:r><w:fldChar w:fldCharType="separate"/></w:r>` +
			`<w:r><w:fldChar w:fldCharType="end"/></w:r>` +
			`</w:p></w:body>`
	}
	cases := []struct {
		name, before, after, value, want string
	}{
		{"spaces and quotes", ` IF `, ` = "x" "a" "b" `, `New "York" \ 1`, `"New \"York\" \\ 1"`},
		{"empty", ` IF `, ` = "" "a" "b" `, ``, `""`},
		{"inside a quoted argument", ` IF 1 = 1 "Dear `, `!" "b" `, `Ada "A"`, `Ada \"A\"`},
		{"after an escaped quote", ` IF 1 = 1 "say \"`, `\"" "b" `, `hi`, `hi`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, unfilled, xml := fillBody(t, ifField(c.before, c.after), map[string]string{"V": c.value})
			if len(unfilled) != 0 {
				t.Errorf("unfilled = %v, want none", unfilled)
			}
			want := `<w:r><w:instrText xml:space="preserve">` + xmlEscape(c.want) + `</w:instrText></w:r>`
			if !strings.Contains(xml, want) {
				t.Errorf("instruction:\n%s\nwant %s", xml, want)
			}
		})
	}
	// Inside a quoted argument an empty value writes nothing.
	_, _, xml := fillBody(t, ifField(` IF 1 = 1 "Dear `, `!" "b" `), map[string]string{"V": ""})
	if strings.Count(xml, "<w:instrText") != 2 {
		t.Errorf("an empty value inside quotes wrote an argument:\n%s", xml)
	}
}

// xmlEscape escapes text the way the marshaller writes it in element content.
func xmlEscape(s string) string {
	var b bytes.Buffer
	_ = xml.EscapeText(&b, []byte(s))
	return strings.ReplaceAll(b.String(), "&#34;", `"`)
}

// TestFillMergeFieldsReportsDeletedSimpleField checks that a simple field in
// a tracked deletion is reported, like a complex one.
func TestFillMergeFieldsReportsDeletedSimpleField(t *testing.T) {
	body := `<w:body><w:p><w:del w:id="1" w:author="A" w:date="2020-01-01T00:00:00Z">` +
		`<w:fldSimple w:instr=" MERGEFIELD Gone "><w:r><w:delText>«Gone»</w:delText></w:r></w:fldSimple>` +
		`</w:del></w:p></w:body>`
	_, unfilled, _ := fillBody(t, body, map[string]string{"Gone": "x"})
	if !reflect.DeepEqual(unfilled, []string{"Gone"}) {
		t.Errorf("unfilled = %v, want [Gone]", unfilled)
	}
}

// TestFillMergeFieldsReportsInDocumentOrder checks that a field skipped for
// spanning paragraphs is reported before a later field that only lacks a
// value.
func TestFillMergeFieldsReportsInDocumentOrder(t *testing.T) {
	body := `<w:body>` +
		`<w:p><w:r><w:fldChar w:fldCharType="begin"/></w:r>` +
		`<w:r><w:instrText xml:space="preserve"> MERGEFIELD First </w:instrText></w:r>` +
		`<w:r><w:fldChar w:fldCharType="separate"/></w:r><w:r><w:t>«First</w:t></w:r></w:p>` +
		`<w:p><w:r><w:t>»</w:t></w:r><w:r><w:fldChar w:fldCharType="end"/></w:r></w:p>` +
		`<w:p>` + wordMergeField(` MERGEFIELD Second `, `«Second»`, "") + `</w:p>` +
		`</w:body>`
	_, unfilled, _ := fillBody(t, body, map[string]string{"First": "a"})
	if want := []string{"First", "Second"}; !reflect.DeepEqual(unfilled, want) {
		t.Errorf("unfilled = %v, want %v", unfilled, want)
	}
}
