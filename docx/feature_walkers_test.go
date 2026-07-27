package docx

import (
	"strings"
	"testing"
)

// nestedSdtBody is a body carrying inline content controls in the three places
// the collector used to miss: a nested table (a w:tbl inside a w:tc), a
// hyperlink, and a tracked-change block. Plus one ordinary paragraph-level SDT
// as the control case.
const nestedSdtBody = `<w:body>` +
	`<w:p><w:sdt><w:sdtPr><w:tag w:val="plain"/></w:sdtPr>` +
	`<w:sdtContent><w:r><w:t>plain</w:t></w:r></w:sdtContent></w:sdt></w:p>` +
	`<w:tbl><w:tr><w:tc>` +
	`<w:tbl><w:tr><w:tc><w:p><w:sdt><w:sdtPr><w:tag w:val="nestedtable"/></w:sdtPr>` +
	`<w:sdtContent><w:r><w:t>deep</w:t></w:r></w:sdtContent></w:sdt></w:p></w:tc></w:tr></w:tbl>` +
	`<w:p/></w:tc></w:tr></w:tbl>` +
	`<w:p><w:hyperlink r:id="rIdX"><w:sdt><w:sdtPr><w:tag w:val="inhyperlink"/></w:sdtPr>` +
	`<w:sdtContent><w:r><w:t>link</w:t></w:r></w:sdtContent></w:sdt></w:hyperlink></w:p>` +
	`<w:p><w:ins w:id="7" w:author="A" w:date="2024-01-01T00:00:00Z">` +
	`<w:sdt><w:sdtPr><w:tag w:val="inins"/></w:sdtPr>` +
	`<w:sdtContent><w:r><w:t>ins</w:t></w:r></w:sdtContent></w:sdt></w:ins></w:p>` +
	`</w:body>`

// TestContentControlsReachNestedContainers pins C405: ContentControls()
// documents "including inline controls, controls nested inside other controls,
// and controls inside tables", but the collector read only p.SdtRun, so an
// inline control inside a hyperlink or a tracked-change block was invisible —
// and a consumer templating by tag silently skipped it.
func TestContentControlsReachNestedContainers(t *testing.T) {
	doc := openFixture(t, fixtureWithDocument(t, fixtureWNS, nestedSdtBody))
	got := map[string]bool{}
	for _, c := range doc.ContentControls() {
		got[c.Tag()] = true
	}
	for _, want := range []string{"plain", "nestedtable", "inhyperlink", "inins"} {
		if !got[want] {
			t.Errorf("ContentControls() missed the control tagged %q (found %v)", want, got)
		}
	}
}

// TestContentControlsNestedByteIdentical guards the fidelity side of the C405
// fix: widening the read-only walk must not mutate the model (the shared
// descent deliberately does not backfill child order), so a zero-modification
// save after enumerating stays byte-identical.
func TestContentControlsNestedByteIdentical(t *testing.T) {
	fixture := fixtureWithDocument(t, fixtureWNS, nestedSdtBody)
	doc := openFixture(t, fixture)
	if n := len(doc.ContentControls()); n == 0 {
		t.Fatal("no content controls enumerated")
	}
	saved := saveDoc(t, doc)
	orig := mustZipEntry(t, fixture, "word/document.xml")
	if got := mustZipEntry(t, saved, "word/document.xml"); got != orig {
		t.Errorf("document.xml changed after a read-only ContentControls() walk:\n got: %s\nwant: %s", got, orig)
	}
}

// TestMergeFieldsReachNestedContainersAndSpanParagraphs pins C498:
// MergeFields() walked only p.R and p.Hyperlink[].R, so a MERGEFIELD inside a
// content control or a w:ins was not reported; and the complex-field state
// machine ran per run-slice, so a field whose begin/instrText/end straddle a
// container boundary (legal per ECMA-376 §17.16.18) lost its capture state.
func TestMergeFieldsReachNestedContainersAndSpanParagraphs(t *testing.T) {
	body := `<w:body>` +
		// MERGEFIELD wholly inside an inline content control.
		`<w:p><w:sdt><w:sdtPr><w:tag w:val="t"/></w:sdtPr><w:sdtContent>` +
		complexMergeField("InSdt") +
		`</w:sdtContent></w:sdt></w:p>` +
		// MERGEFIELD wholly inside a tracked insertion.
		`<w:p><w:ins w:id="9" w:author="A" w:date="2024-01-01T00:00:00Z">` +
		complexMergeField("InIns") +
		`</w:ins></w:p>` +
		// A field whose begin sits in a hyperlink and whose end sits after it.
		`<w:p>` +
		`<w:hyperlink r:id="rIdX">` +
		`<w:r><w:fldChar w:fldCharType="begin"/></w:r>` +
		`<w:r><w:instrText xml:space="preserve"> MERGEFIELD Straddle </w:instrText></w:r>` +
		`</w:hyperlink>` +
		`<w:r><w:fldChar w:fldCharType="separate"/></w:r>` +
		`<w:r><w:t>x</w:t></w:r>` +
		`<w:r><w:fldChar w:fldCharType="end"/></w:r>` +
		`</w:p>` +
		`</w:body>`
	doc := openFixture(t, fixtureWithDocument(t, fixtureWNS, body))
	got := strings.Join(doc.MergeFields(), ",")
	for _, want := range []string{"InSdt", "InIns", "Straddle"} {
		if !strings.Contains(got, want) {
			t.Errorf("MergeFields() missed %q (got %q)", want, got)
		}
	}
}

// complexMergeField renders the begin/instrText/separate/result/end run
// sequence Word writes for a MERGEFIELD.
func complexMergeField(name string) string {
	return `<w:r><w:fldChar w:fldCharType="begin"/></w:r>` +
		`<w:r><w:instrText xml:space="preserve"> MERGEFIELD ` + name + ` </w:instrText></w:r>` +
		`<w:r><w:fldChar w:fldCharType="separate"/></w:r>` +
		`<w:r><w:t>«` + name + `»</w:t></w:r>` +
		`<w:r><w:fldChar w:fldCharType="end"/></w:r>`
}

// TestFormFieldsReachNestedContainers pins the FormFields half of C498: a
// FORMTEXT field inside a content control or a tracked insertion was not
// reported, though the godoc promises "anywhere in the document body".
func TestFormFieldsReachNestedContainers(t *testing.T) {
	body := `<w:body>` +
		`<w:p><w:sdt><w:sdtPr><w:tag w:val="t"/></w:sdtPr><w:sdtContent>` +
		legacyTextFormField("InSdt", "alpha") +
		`</w:sdtContent></w:sdt></w:p>` +
		`<w:p><w:ins w:id="9" w:author="A" w:date="2024-01-01T00:00:00Z">` +
		legacyTextFormField("InIns", "beta") +
		`</w:ins></w:p>` +
		`</w:body>`
	doc := openFixture(t, fixtureWithDocument(t, fixtureWNS, body))
	names := map[string]string{}
	for _, ff := range doc.FormFields() {
		names[ff.Name] = ff.Value
	}
	for name, want := range map[string]string{"InSdt": "alpha", "InIns": "beta"} {
		if got, ok := names[name]; !ok {
			t.Errorf("FormFields() missed the field named %q (found %v)", name, names)
		} else if got != want {
			t.Errorf("FormFields()[%q] = %q, want %q", name, got, want)
		}
	}
}

// legacyTextFormField renders the run sequence Word writes for a FORMTEXT
// legacy form field with the given bookmark name and result text.
func legacyTextFormField(name, result string) string {
	return `<w:r><w:fldChar w:fldCharType="begin"><w:ffData>` +
		`<w:name w:val="` + name + `"/><w:enabled/><w:textInput/>` +
		`</w:ffData></w:fldChar></w:r>` +
		`<w:r><w:instrText xml:space="preserve"> FORMTEXT </w:instrText></w:r>` +
		`<w:r><w:fldChar w:fldCharType="separate"/></w:r>` +
		`<w:r><w:t>` + result + `</w:t></w:r>` +
		`<w:r><w:fldChar w:fldCharType="end"/></w:r>`
}
