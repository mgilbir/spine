package docx

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mgilbir/spine/internal/fuzzseed"
)

// buildValidDocxFuzzSeed creates a small valid document in-process so no
// corpus binaries need committing.
func buildValidDocxFuzzSeed(f *testing.F) []byte {
	f.Helper()
	d := Create()
	d.AddParagraphWithText("fuzz seed")
	tbl := d.AddTable(2, 2)
	tbl.Rows()[0].Cells()[0].AddParagraph().SetText("cell")
	valid, err := d.SaveBytes()
	if err != nil {
		f.Fatalf("building valid docx seed: %v", err)
	}
	return valid
}

// fuzzExerciseDocx opens the bytes as a document and, on success, walks a
// bounded slice of the model and round-trips it (SaveBytes then re-open).
// Any panic is a bug; errors are expected and fine.
func fuzzExerciseDocx(data []byte) {
	d, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return
	}
	defer func() { _ = d.Close() }()

	for i, p := range d.Paragraphs() {
		if i >= 64 {
			break
		}
		_ = p.Text()
		_ = p.Style()
		_ = p.Runs()
	}
	for i, tbl := range d.Tables() {
		if i >= 8 {
			break
		}
		for j, row := range tbl.Rows() {
			if j >= 16 {
				break
			}
			for k, cell := range row.Cells() {
				if k >= 16 {
					break
				}
				_ = cell.Text()
			}
		}
	}
	_ = d.Body()

	out, err := d.SaveBytes()
	if err != nil {
		return
	}
	d2, err := OpenReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		return
	}
	_ = d2.Paragraphs()
	_ = d2.Close()
}

// FuzzOpenDocx feeds arbitrary bytes to the document opener.
func FuzzOpenDocx(f *testing.F) {
	valid := buildValidDocxFuzzSeed(f)

	f.Add(valid)
	f.Add([]byte{})
	f.Add([]byte("PK\x03\x04"))
	f.Add(valid[:len(valid)/2])
	corrupt := append([]byte(nil), valid...)
	corrupt[len(corrupt)/2] ^= 0xFF
	f.Add(corrupt)
	// A package that claims to be a document but whose main part is
	// truncated garbage, so the WML parse paths see hostile XML.
	f.Add(fuzzseed.BuildZip([][2]string{
		{"[Content_Types].xml", `<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/></Types>`},
		{"_rels/.rels", `<?xml version="1.0"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>`},
		{"word/document.xml", `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>x`},
	}))
	// A handful of small real files when the gitignored corpus is present;
	// never committed.
	for _, seed := range fuzzseed.CorpusSeeds("docx", 5, 256<<10) {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		fuzzExerciseDocx(data)
	})
}

// FuzzDocxDocumentXML packs the fuzz bytes into the main document part of
// an otherwise-valid package, so the WML parsers see hostile XML directly
// instead of the fuzzer having to invent whole valid zip archives.
func FuzzDocxDocumentXML(f *testing.F) {
	valid := buildValidDocxFuzzSeed(f)
	const docPart = "word/document.xml"
	orig := fuzzseed.ZipEntry(valid, docPart)
	if orig == nil {
		f.Fatalf("valid docx seed has no %s", docPart)
	}

	f.Add(orig)
	f.Add([]byte{})
	f.Add([]byte("not xml"))
	f.Add(orig[:len(orig)/2])
	// Deeply nested tables.
	f.Add([]byte(`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body>` +
		strings.Repeat(`<w:tbl><w:tr><w:tc>`, 150) + strings.Repeat(`</w:tc></w:tr></w:tbl>`, 150) +
		`</w:body></w:document>`))
	// Structural fields with no required children and hostile numbers.
	f.Add([]byte(`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:pPr><w:spacing w:before="99999999999999999999"/></w:pPr><w:r><w:t xml:space="preserve"/></w:r></w:p><w:sectPr/></w:body></w:document>`))

	f.Fuzz(func(t *testing.T, data []byte) {
		wrapped := fuzzseed.ReplaceZipEntry(valid, docPart, data)
		if wrapped == nil {
			t.Skip("seed package unreadable")
		}
		fuzzExerciseDocx(wrapped)
	})
}

// wmlBodyDoc wraps a body-content fragment in a full w:document part carrying
// the namespace declarations the WML parsers expect, so a fuzzer can feed
// arbitrary w:sdt / revision XML without inventing whole valid documents.
const wmlDocOpen = `<w:document ` +
	`xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" ` +
	`xmlns:w14="http://schemas.microsoft.com/office/word/2010/wordml" ` +
	`xmlns:mc="http://schemas.openxmlformats.org/markup-compatibility/2006" ` +
	`xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">` +
	`<w:body>`
const wmlDocClose = `</w:body></w:document>`

func wmlBodyDoc(fragment string) []byte {
	return []byte(wmlDocOpen + fragment + wmlDocClose)
}

// FuzzDocxContentControl fuzzes AddContentControl and SetValue: the tag, the
// initial value and a replacement value, at both block and inline (run) level.
// It creates a document, adds the controls, mutates them, then saves and
// re-opens, reading the controls back.
func FuzzDocxContentControl(f *testing.F) {
	f.Add("cite", "hello", "world")
	f.Add("", "", "")
	f.Add("t", "<b>&amp;</b>", "\x00￿")
	f.Add("multi\nline", "a\tb", "]]>")
	f.Add("dup", "x", strings.Repeat("z", 4096))

	f.Fuzz(func(t *testing.T, tag, value, newValue string) {
		d := Create()
		cc := d.AddContentControl(tag, value)
		cc.SetTag(tag)
		cc.SetAlias(value)
		cc.SetValue(newValue)

		p := d.AddParagraphWithText("host")
		icc := p.AddContentControl(tag, value)
		icc.SetValue(newValue)

		out, err := d.SaveBytes()
		if err != nil {
			return
		}
		d2, err := OpenReader(bytes.NewReader(out), int64(len(out)))
		if err != nil {
			return
		}
		defer func() { _ = d2.Close() }()
		for _, c := range d2.ContentControls() {
			_ = c.Tag()
			_ = c.Alias()
			_ = c.Type()
			_ = c.Value()
			_ = c.IsInline()
			_ = c.Options()
			_, _ = c.Checked()
		}
	})
}

// FuzzDocxSdtPr feeds arbitrary structured-document-tag XML (with a fuzzed
// w:sdtPr) through Open, targeting the sdtPr parse/retype path directly. On a
// successful parse it reads the content controls back, mutates them, and
// round-trips. Any panic is a bug; parse errors are expected.
func FuzzDocxSdtPr(f *testing.F) {
	f.Add(`<w:sdt><w:sdtPr><w:id w:val="1"/><w:tag w:val="a"/></w:sdtPr>` +
		`<w:sdtContent><w:p><w:r><w:t>x</w:t></w:r></w:p></w:sdtContent></w:sdt>`)
	f.Add(`<w:sdt><w:sdtPr/><w:sdtContent/></w:sdt>`)
	f.Add(`<w:sdt><w:sdtPr>`) // truncated
	f.Add(`<w:p><w:sdt><w:sdtPr><w:rPr><w:b/></w:rPr>` +
		`<w:date w:fullDate="not-a-date"><w:dateFormat w:val="M/d"/></w:date>` +
		`<w:dropDownList><w:listItem w:displayText="a" w:value="1"/></w:dropDownList>` +
		`</w:sdtPr><w:sdtContent><w:r><w:t>run</w:t></w:r></w:sdtContent></w:sdt></w:p>`)
	f.Add(`<w:sdt><w:sdtPr><w:id w:val="-99999999999999999999"/>` +
		`<w14:checkbox><w14:checked w14:val="1"/></w14:checkbox></w:sdtPr>` +
		`<w:sdtContent><w:p/></w:sdtContent></w:sdt>`)

	valid := buildValidDocxFuzzSeed(f)
	const docPart = "word/document.xml"

	f.Fuzz(func(t *testing.T, fragment string) {
		wrapped := fuzzseed.ReplaceZipEntry(valid, docPart, wmlBodyDoc(fragment))
		if wrapped == nil {
			t.Skip("seed package unreadable")
		}
		d, err := OpenReader(bytes.NewReader(wrapped), int64(len(wrapped)))
		if err != nil {
			return
		}
		defer func() { _ = d.Close() }()
		for _, c := range d.ContentControls() {
			_ = c.Tag()
			_ = c.Type()
			_ = c.Value()
			_ = c.Options()
			_, _ = c.Checked()
			c.SetValue("mut")
		}
		out, err := d.SaveBytes()
		if err != nil {
			return
		}
		if d2, err := OpenReader(bytes.NewReader(out), int64(len(out))); err == nil {
			_ = d2.ContentControls()
			_ = d2.Close()
		}
	})
}

// FuzzDocxRevisions feeds arbitrary revision-bearing document bodies through
// Open and then applies AcceptAllRevisions or RejectAllRevisions (plus a pass
// of per-revision Accept/Reject), saving and re-opening. Any panic is a bug.
func FuzzDocxRevisions(f *testing.F) {
	ins := `<w:p><w:ins w:id="1" w:author="a" w:date="2020-01-01T00:00:00Z">` +
		`<w:r><w:t>added</w:t></w:r></w:ins></w:p>`
	del := `<w:p><w:del w:id="2" w:author="b" w:date="2020-01-01T00:00:00Z">` +
		`<w:r><w:delText>gone</w:delText></w:r></w:del></w:p>`
	fmtChg := `<w:p><w:pPr><w:rPr><w:ins w:id="3"/></w:rPr>` +
		`<w:pPrChange w:id="4" w:author="c" w:date="x"><w:pPr/></w:pPrChange></w:pPr>` +
		`<w:r><w:rPr><w:rPrChange w:id="5" w:author="d" w:date="y"><w:rPr/></w:rPrChange></w:rPr>` +
		`<w:t>styled</w:t></w:r></w:p>`

	f.Add(ins+del+fmtChg, true)
	f.Add(ins, false)
	f.Add(del, true)
	f.Add(`<w:p><w:ins`, true) // truncated
	f.Add(`<w:tbl><w:tr><w:trPr><w:ins w:id="9"/></w:trPr><w:tc><w:p/></w:tc></w:tr></w:tbl>`, false)
	f.Add(``, true)

	valid := buildValidDocxFuzzSeed(f)
	const docPart = "word/document.xml"

	f.Fuzz(func(t *testing.T, fragment string, accept bool) {
		wrapped := fuzzseed.ReplaceZipEntry(valid, docPart, wmlBodyDoc(fragment))
		if wrapped == nil {
			t.Skip("seed package unreadable")
		}
		d, err := OpenReader(bytes.NewReader(wrapped), int64(len(wrapped)))
		if err != nil {
			return
		}
		defer func() { _ = d.Close() }()

		// Per-revision transforms first, then the batch operation.
		for _, r := range d.Revisions() {
			_ = r.Author()
			_ = r.Date()
			_ = r.Type()
			_ = r.Text()
			if r.Editable() {
				if accept {
					_ = r.Accept()
				} else {
					_ = r.Reject()
				}
			}
		}
		if accept {
			_ = d.AcceptAllRevisions()
		} else {
			_ = d.RejectAllRevisions()
		}
		out, err := d.SaveBytes()
		if err != nil {
			return
		}
		if d2, err := OpenReader(bytes.NewReader(out), int64(len(out))); err == nil {
			_ = d2.Revisions()
			_ = d2.Close()
		}
	})
}
