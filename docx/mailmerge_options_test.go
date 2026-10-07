package docx

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

func TestFillMergeFieldsWithMissing(t *testing.T) {
	body := `<w:body><w:p>` +
		wordMergeField(` MERGEFIELD A `, `«A»`, "") + `<w:r><w:t>|</w:t></w:r>` +
		wordMergeField(` MERGEFIELD B \* Upper `, `«B»`, "") + `<w:r><w:t>|</w:t></w:r>` +
		wordMergeField(` MERGEFIELD C `, `«C»`, "") +
		`</w:p></w:body>`
	doc := openDocFixture(t, body)
	unfilled := doc.FillMergeFieldsWith(map[string]string{}, MergeOptions{
		Missing: func(name string) (string, bool) {
			switch name {
			case "A":
				return "", true
			case "B":
				return "[" + name + "]", true
			}
			return "", false
		},
	})
	if !reflect.DeepEqual(unfilled, []string{"C"}) {
		t.Errorf("unfilled = %v, want [C]", unfilled)
	}
	if got := doc.Paragraphs()[0].Text(); got != "|[B]|«C»" {
		t.Errorf("paragraph text = %q, want %q", got, "|[B]|«C»")
	}
}

// addressBody is an address block whose lines are each one or two fields,
// followed by a labelled line, a bookmarked line, a table and a last line.
func addressBody() string {
	f := func(name string) string { return wordMergeField(` MERGEFIELD `+name+` `, `«`+name+`»`, "") }
	return `<w:body>` +
		`<w:p>` + f("Company") + `</w:p>` +
		`<w:p>` + f("Street") + `</w:p>` +
		`<w:p>` + f("City") + `<w:r><w:t xml:space="preserve"> </w:t></w:r>` + f("Zip") + `</w:p>` +
		`<w:p><w:r><w:t xml:space="preserve">Phone: </w:t></w:r>` + f("Phone") + `</w:p>` +
		`<w:p><w:bookmarkStart w:id="0" w:name="mark"/>` + f("Extra") + `<w:bookmarkEnd w:id="0"/></w:p>` +
		`<w:p><w:r><w:t>Unrelated</w:t></w:r></w:p>` +
		`<w:p/>` +
		`<w:tbl><w:tblPr/><w:tblGrid><w:gridCol w:w="100"/><w:gridCol w:w="100"/></w:tblGrid><w:tr>` +
		`<w:tc><w:p>` + f("Cell1") + `</w:p><w:p><w:r><w:t>kept</w:t></w:r></w:p></w:tc>` +
		`<w:tc><w:p>` + f("Cell2") + `</w:p></w:tc>` +
		`</w:tr></w:tbl>` +
		`<w:p>` + f("Last") + `</w:p>` +
		`</w:body>`
}

var addressValues = map[string]string{
	"Company": "", "Street": "1 Main St", "City": "", "Zip": "", "Phone": "",
	"Extra": "", "Cell1": "", "Cell2": "", "Last": "",
}

func paragraphTexts(doc *Document) []string {
	var out []string
	for _, p := range doc.Paragraphs() {
		out = append(out, p.Text())
	}
	return out
}

func TestFillMergeFieldsSuppressBlankLines(t *testing.T) {
	doc := openDocFixture(t, addressBody())
	if unfilled := doc.FillMergeFieldsWith(addressValues, MergeOptions{SuppressBlankLines: true}); len(unfilled) != 0 {
		t.Errorf("unfilled = %v, want none", unfilled)
	}
	// Company and City/Zip lines go; the labelled line, the bookmarked line,
	// the paragraph that was empty before the merge and the last paragraph
	// stay.
	want := []string{"1 Main St", "Phone: ", "", "Unrelated", "", ""}
	if got := paragraphTexts(doc); !reflect.DeepEqual(got, want) {
		t.Errorf("body paragraphs = %q, want %q", got, want)
	}
	xml := saveDocXML(t, doc)
	if !strings.Contains(xml, `<w:bookmarkStart w:id="0" w:name="mark"/>`) {
		t.Errorf("the bookmarked paragraph was removed:\n%s", xml)
	}
	// In the table, the first cell loses its blank first paragraph; the second
	// cell keeps its only one.
	if !strings.Contains(xml, `<w:tc><w:p><w:r><w:t>kept</w:t></w:r></w:p></w:tc><w:tc><w:p></w:p></w:tc>`) &&
		!strings.Contains(xml, `<w:tc><w:p><w:r><w:t>kept</w:t></w:r></w:p></w:tc><w:tc><w:p/></w:tc>`) {
		t.Errorf("table cells after suppression:\n%s", xml)
	}
}

func TestFillMergeFieldsKeepsBlankLinesByDefault(t *testing.T) {
	doc := openDocFixture(t, addressBody())
	doc.FillMergeFields(addressValues)
	want := []string{"", "1 Main St", " ", "Phone: ", "", "Unrelated", "", ""}
	if got := paragraphTexts(doc); !reflect.DeepEqual(got, want) {
		t.Errorf("body paragraphs = %q, want %q", got, want)
	}
}

// TestFillMergeFieldsSuppressKeepsSectionBreak checks that a blank paragraph
// carrying a section break is kept.
func TestFillMergeFieldsSuppressKeepsSectionBreak(t *testing.T) {
	body := `<w:body>` +
		`<w:p><w:pPr><w:sectPr/></w:pPr>` + wordMergeField(` MERGEFIELD X `, `«X»`, "") + `</w:p>` +
		`<w:p><w:pPr><w:rPr><w:ins w:id="1" w:author="A" w:date="2020-01-01T00:00:00Z"/></w:rPr></w:pPr>` +
		wordMergeField(` MERGEFIELD X `, `«X»`, "") + `</w:p>` +
		`<w:p><w:r><w:t>end</w:t></w:r></w:p></w:body>`
	doc := openDocFixture(t, body)
	doc.FillMergeFieldsWith(map[string]string{"X": ""}, MergeOptions{SuppressBlankLines: true})
	if got := len(doc.Paragraphs()); got != 3 {
		t.Errorf("paragraphs = %d, want 3 (section break and tracked paragraph mark kept)", got)
	}
}

func TestFillMergeFieldsSuppressInTextBox(t *testing.T) {
	inner := `<w:p>` + wordMergeField(` MERGEFIELD Company `, `«Company»`, "") + `</w:p>` +
		`<w:p>` + wordMergeField(` MERGEFIELD Name `, `«Name»`, "") + `</w:p>`
	data := fixtureWithDocument(t, textBoxNS, `<w:body><w:p>`+textBoxDrawing(inner)+`</w:p></w:body>`)
	doc, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	doc.FillMergeFieldsWith(map[string]string{"Company": "", "Name": "Ada"}, MergeOptions{SuppressBlankLines: true})
	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	re, err := OpenReader(bytes.NewReader(saved), int64(len(saved)))
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if tbs := re.TextBoxes(); len(tbs) != 1 || tbs[0].Text() != "Ada" {
		t.Errorf("text box after suppression = %+v", tbs)
	}
	if n := strings.Count(saveDocXML(t, re), "<w:txbxContent><w:p>"); n != 2 {
		t.Errorf("text box bodies starting with the kept paragraph = %d, want 2", n)
	}
}

func TestMailMergeDoNotSuppressBlankLinesRoundTrip(t *testing.T) {
	doc := Create()
	doc.SetMailMerge(&MailMerge{MainDocumentType: MailMergeFormLetters, DoNotSuppressBlankLines: true})
	data, err := doc.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	if s := zipEntryString(t, data, "word/settings.xml"); !strings.Contains(s, "<w:doNotSuppressBlankLines/>") {
		t.Errorf("w:doNotSuppressBlankLines not written:\n%s", s)
	}
	re, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	if mm := re.MailMerge(); mm == nil || !mm.DoNotSuppressBlankLines {
		t.Errorf("MailMerge() = %+v, want DoNotSuppressBlankLines", mm)
	}
}

// TestFillMergeFieldsSuppressKeepsTableSeparator checks that the only
// paragraph between two tables is kept, as removing it would join them, while
// one of two blank paragraphs between them may go.
func TestFillMergeFieldsSuppressKeepsTableSeparator(t *testing.T) {
	tbl := `<w:tbl><w:tblPr/><w:tblGrid><w:gridCol w:w="100"/></w:tblGrid><w:tr><w:tc><w:p/></w:tc></w:tr></w:tbl>`
	f := wordMergeField(` MERGEFIELD A `, `«A»`, "")
	cases := map[string]struct {
		body string
		want int // paragraphs between the tables afterwards
	}{
		"single separator":                   {tbl + `<w:p>` + f + `</w:p>` + tbl, 1},
		"single separator between bookmarks": {tbl + `<w:bookmarkStart w:id="0" w:name="b"/><w:p>` + f + `</w:p><w:bookmarkEnd w:id="0"/>` + tbl, 1},
		"two separators":                     {tbl + `<w:p>` + f + `</w:p><w:p>` + f + `</w:p>` + tbl, 1},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			doc := openDocFixture(t, `<w:body>`+c.body+`<w:p><w:r><w:t>end</w:t></w:r></w:p></w:body>`)
			doc.FillMergeFieldsWith(map[string]string{"A": ""}, MergeOptions{SuppressBlankLines: true})
			xml := saveDocXML(t, doc)
			if strings.Contains(xml, "</w:tbl><w:tbl>") {
				t.Errorf("the tables were left adjacent:\n%s", xml)
			}
			if got := len(doc.Paragraphs()) - 1; got != c.want {
				t.Errorf("paragraphs between the tables = %d, want %d:\n%s", got, c.want, xml)
			}
		})
	}
}
