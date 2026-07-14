package pptx

import (
	"archive/zip"
	"bytes"
	"testing"
)

// fuzzPptxZip assembles an in-memory zip archive from name/body pairs.
func fuzzPptxZip(entries [][2]string) []byte {
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

// FuzzOpenPptx feeds arbitrary bytes to the presentation opener and, when a
// package opens, walks a bounded slice of the model and round-trips it
// (SaveBytes then re-open). Any panic is a bug; errors are expected.
func FuzzOpenPptx(f *testing.F) {
	p := Create()
	slide := p.AddSlide()
	tb := slide.AddTextBox()
	tb.TextFrame().SetText("fuzz seed")
	valid, err := p.SaveBytes()
	if err != nil {
		f.Fatalf("building valid pptx seed: %v", err)
	}

	f.Add(valid)
	f.Add([]byte{})
	f.Add([]byte("PK\x03\x04"))
	f.Add(valid[:len(valid)/2])
	corrupt := append([]byte(nil), valid...)
	corrupt[len(corrupt)/2] ^= 0xFF
	f.Add(corrupt)
	// A package that claims to be a presentation but whose main part is
	// truncated garbage, so the PML parse paths see hostile XML.
	f.Add(fuzzPptxZip([][2]string{
		{"[Content_Types].xml", `<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/></Types>`},
		{"_rels/.rels", `<?xml version="1.0"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/></Relationships>`},
		{"ppt/presentation.xml", `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:sldIdLst><p:sldId id="256"`},
	}))

	f.Fuzz(func(t *testing.T, data []byte) {
		p, err := OpenReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			return
		}
		defer func() { _ = p.Close() }()

		for i, s := range p.Slides() {
			if i >= 8 {
				break
			}
			_ = s.Name()
			_ = s.Layout()
			for j, sh := range s.Shapes() {
				if j >= 32 {
					break
				}
				_ = sh.Name()
				_, _ = sh.Position()
				_, _ = sh.Size()
			}
			for j, ph := range s.Placeholders() {
				if j >= 32 {
					break
				}
				_ = ph.Name()
			}
		}

		out, err := p.SaveBytes()
		if err != nil {
			return
		}
		p2, err := OpenReader(bytes.NewReader(out), int64(len(out)))
		if err != nil {
			return
		}
		_ = p2.Slides()
		_ = p2.Close()
	})
}
