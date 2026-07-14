package docx

import (
	"archive/zip"
	"bytes"
	"testing"
)

// fuzzDocxZip assembles an in-memory zip archive from name/body pairs.
func fuzzDocxZip(entries [][2]string) []byte {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, e := range entries {
		w, err := zw.Create(e[0])
		if err != nil {
			continue
		}
		_, _ = w.Write([]byte(e[1]))
	}
	_ = zw.Close()
	return buf.Bytes()
}

// FuzzOpenDocx feeds arbitrary bytes to the document opener and, when a
// package opens, walks a bounded slice of the model and round-trips it
// (SaveBytes then re-open). Any panic is a bug; errors are expected.
func FuzzOpenDocx(f *testing.F) {
	d := Create()
	d.AddParagraphWithText("fuzz seed")
	tbl := d.AddTable(2, 2)
	tbl.Rows()[0].Cells()[0].AddParagraph().SetText("cell")
	valid, err := d.SaveBytes()
	if err != nil {
		f.Fatalf("building valid docx seed: %v", err)
	}

	f.Add(valid)
	f.Add([]byte{})
	f.Add([]byte("PK\x03\x04"))
	f.Add(valid[:len(valid)/2])
	corrupt := append([]byte(nil), valid...)
	corrupt[len(corrupt)/2] ^= 0xFF
	f.Add(corrupt)
	// A package that claims to be a document but whose main part is
	// truncated garbage, so the WML parse paths see hostile XML.
	f.Add(fuzzDocxZip([][2]string{
		{"[Content_Types].xml", `<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/></Types>`},
		{"_rels/.rels", `<?xml version="1.0"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/></Relationships>`},
		{"word/document.xml", `<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>x`},
	}))

	f.Fuzz(func(t *testing.T, data []byte) {
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
	})
}
