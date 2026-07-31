package docx

import (
	"bytes"
	"image"
	"image/png"
	"testing"

	"github.com/mgilbir/spine/internal/fuzzseed"
)

// Every prefix a saved part uses must be bound in that part, whatever the
// document it was opened from happened to declare.
//
// The rule was broken in two different ways, and both were reachable from
// ordinary calls over valid input. The Builder wrote element and attribute
// names in namespaces it had not declared, because the root replayed the
// declaration list captured from the source and everything under it assumed
// that list covered whatever the writer used — so AddHyperlink emitted r:id
// into a document whose root declared only w:. And a helper that splices XML in
// as raw bytes cannot be covered by the Builder at all, so AddOLEObject wrote
// r:id inside VML the Builder never sees, with the same result. Word reports
// such a file as damaged; Go's decoder resolves the unbound prefix to the
// literal prefix instead of failing, so nothing downstream noticed.
//
// The document below is the shape that exposes it: valid WordprocessingML whose
// root declares only w:, which is exactly what a minimal producer writes when
// the body needs nothing else. Every API here splices prefixed XML into a part;
// the list came from an ast-grep sweep for functions that build XML with a
// prefix they do not themselves declare.
const minimalRootDocument = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
	`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` +
	`<w:body><w:p><w:r><w:t>hello</w:t></w:r></w:p></w:body></w:document>`

func TestEmittedPartsBindEveryPrefixTheyUse(t *testing.T) {
	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, image.NewRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		t.Fatal(err)
	}
	img := pngBuf.Bytes()

	insert := map[string]func(*testing.T, *Document){
		"AddHyperlink": func(t *testing.T, d *Document) {
			d.Paragraphs()[0].AddHyperlink("click", "https://example.com")
		},
		"AddImageFromBytes": func(t *testing.T, d *Document) {
			if _, err := d.Paragraphs()[0].AddRun().AddImageFromBytes(img, "image/png"); err != nil {
				t.Fatalf("AddImageFromBytes: %v", err)
			}
		},
		"AddTextBox":       func(t *testing.T, d *Document) { d.AddTextBox("boxed", TextBoxOptions{}) },
		"AddWordArt":       func(t *testing.T, d *Document) { d.AddWordArt("arty", WordArtOptions{}) },
		"AddShapeGroup":    func(t *testing.T, d *Document) { d.AddShapeGroup(GroupOptions{}) },
		"AddSignatureLine": func(t *testing.T, d *Document) { d.AddSignatureLine(SignatureLineOptions{Signer: "S"}) },
		"AddFormField": func(t *testing.T, d *Document) {
			d.Paragraphs()[0].AddFormField(FormFieldOptions{Name: "F"})
		},
		"AddComment": func(t *testing.T, d *Document) { d.Paragraphs()[0].AddComment("A", "c") },
		"AddTableOfContents": func(t *testing.T, d *Document) {
			if err := d.AddTableOfContents(TOCOptions{}); err != nil {
				t.Fatalf("AddTableOfContents: %v", err)
			}
		},
		"SetTextWatermark": func(t *testing.T, d *Document) {
			if err := d.SetTextWatermark("WM", WatermarkOptions{}); err != nil {
				t.Fatalf("SetTextWatermark: %v", err)
			}
		},
		"SetImageWatermark": func(t *testing.T, d *Document) {
			if err := d.SetImageWatermark(img, WatermarkOptions{}); err != nil {
				t.Fatalf("SetImageWatermark: %v", err)
			}
		},
		"AddOLEObject": func(t *testing.T, d *Document) {
			if _, err := d.AddOLEObject([]byte("ole"), "Prog.Id", OLEEmbedOptions{}); err != nil {
				t.Fatalf("AddOLEObject: %v", err)
			}
		},
	}

	base := Create()
	base.AddParagraph().SetText("seed")
	valid, err := base.SaveBytes()
	if err != nil {
		t.Fatalf("building the base package: %v", err)
	}
	sparse := fuzzseed.ReplaceZipEntry(valid, "word/document.xml", []byte(minimalRootDocument))
	if sparse == nil {
		t.Fatal("could not build the minimal-root package")
	}

	for name, apply := range insert {
		t.Run(name, func(t *testing.T) {
			d, err := OpenReader(bytes.NewReader(sparse), int64(len(sparse)))
			if err != nil {
				t.Fatalf("opening a document whose root declares only w:: %v", err)
			}
			apply(t, d)
			out, err := d.SaveBytes()
			if err != nil {
				t.Fatalf("save: %v", err)
			}
			fuzzseed.AssertEmittedNamespacesResolve(t, sparse, out)
		})
	}
}
