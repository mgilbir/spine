package pptx

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mgilbir/spine/internal/fuzzseed"
)

// buildValidPptxFuzzSeed creates a small valid presentation in-process so no
// corpus binaries need committing.
func buildValidPptxFuzzSeed(f *testing.F) []byte {
	f.Helper()
	p := Create()
	slide := p.AddSlide()
	tb := slide.AddTextBox()
	tb.TextFrame().SetText("fuzz seed")
	valid, err := p.SaveBytes()
	if err != nil {
		f.Fatalf("building valid pptx seed: %v", err)
	}
	return valid
}

// fuzzExercisePptx opens the bytes as a presentation and, on success, walks
// a bounded slice of the model and round-trips it (SaveBytes then re-open).
// Any panic is a bug; errors are expected and fine.
func fuzzExercisePptx(data []byte) {
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
}

// FuzzOpenPptx feeds arbitrary bytes to the presentation opener.
func FuzzOpenPptx(f *testing.F) {
	valid := buildValidPptxFuzzSeed(f)

	f.Add(valid)
	f.Add([]byte{})
	f.Add([]byte("PK\x03\x04"))
	f.Add(valid[:len(valid)/2])
	corrupt := append([]byte(nil), valid...)
	corrupt[len(corrupt)/2] ^= 0xFF
	f.Add(corrupt)
	// A package that claims to be a presentation but whose main part is
	// truncated garbage, so the PML parse paths see hostile XML.
	f.Add(fuzzseed.BuildZip([][2]string{
		{"[Content_Types].xml", `<?xml version="1.0"?><Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/ppt/presentation.xml" ContentType="application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml"/></Types>`},
		{"_rels/.rels", `<?xml version="1.0"?><Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="ppt/presentation.xml"/></Relationships>`},
		{"ppt/presentation.xml", `<p:presentation xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main"><p:sldIdLst><p:sldId id="256"`},
	}))
	// A handful of small real files when the gitignored corpus is present;
	// never committed.
	for _, seed := range fuzzseed.CorpusSeeds("pptx", 5, 256<<10) {
		f.Add(seed)
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		fuzzExercisePptx(data)
	})
}

// FuzzPptxSlideXML packs the fuzz bytes into the first slide part of an
// otherwise-valid presentation, so the PML parsers see hostile XML directly
// instead of the fuzzer having to invent whole valid zip archives.
func FuzzPptxSlideXML(f *testing.F) {
	valid := buildValidPptxFuzzSeed(f)
	const slidePart = "ppt/slides/slide1.xml"
	orig := fuzzseed.ZipEntry(valid, slidePart)
	if orig == nil {
		f.Fatalf("valid pptx seed has no %s", slidePart)
	}

	f.Add(orig)
	f.Add([]byte{})
	f.Add([]byte("not xml"))
	f.Add(orig[:len(orig)/2])
	f.Add([]byte(`<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:cSld><p:spTree>` +
		strings.Repeat(`<p:grpSp><p:grpSpPr/>`, 200) + strings.Repeat(`</p:grpSp>`, 200) +
		`</p:spTree></p:cSld></p:sld>`))
	f.Add([]byte(`<p:sld xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"><p:cSld><p:spTree><p:sp><p:spPr><a:xfrm><a:off x="99999999999999999999" y="-99999999999999999999"/><a:ext cx="0" cy="0"/></a:xfrm></p:spPr></p:sp></p:spTree></p:cSld></p:sld>`))

	f.Fuzz(func(t *testing.T, data []byte) {
		wrapped := fuzzseed.ReplaceZipEntry(valid, slidePart, data)
		if wrapped == nil {
			t.Skip("seed package unreadable")
		}
		fuzzExercisePptx(wrapped)
	})
}
