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
