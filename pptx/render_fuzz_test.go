package pptx

import (
	"bytes"
	"context"
	"errors"
	"image/png"
	"testing"

	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/internal/fuzzseed"
	"github.com/mgilbir/spine/opc"
	"github.com/mgilbir/spine/render"
)

func FuzzPptxRender(f *testing.F) {
	p := CreateWithOptions(CreateOptions{Options: Options{SlideSize: SlideSizeCustom}, Width: dml.Pixels(16), Height: dml.Pixels(16)})
	p.Properties.Created = fuzzseed.FixtureModified
	p.Properties.Modified = fuzzseed.FixtureModified
	slide := p.AddSlide()
	shape := NewAutoShape(PresetRect)
	shape.SetSize(dml.Pixels(8), dml.Pixels(8))
	shape.SetFill(dml.NewSolidFill(dml.ColorRed))
	shape.SetNoLine()
	if e := slide.AddShape(shape); e != nil {
		f.Fatal(e)
	}
	valid, e := p.SaveBytes()
	if e != nil {
		f.Fatal(e)
	}
	source := fuzzseed.ZipEntry(valid, "ppt/slides/slide1.xml")
	f.Add(source)
	// Seed the skipped-extension path so mutations probe where skipping ends.
	f.Add(bytes.Replace(source, []byte(`<p:cNvPr id="2" name="Shape"/>`), []byte(`<p:cNvPr id="2" name="Shape">`+renderTestCreationID+`</p:cNvPr>`), 1))
	f.Add([]byte("<p:sld/>"))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 16<<10 {
			t.Skip()
		}
		wrapped := fuzzseed.ReplaceZipEntry(valid, "ppt/slides/slide1.xml", data)
		if wrapped == nil {
			t.Fatal("fixture")
		}
		// Open-path fuzzers already cover eager PML parsing. Construct a lazy
		// selected slide so this target exercises the renderer's preventive
		// source gate before model projection, rather than an earlier parser.
		reader, e := opc.NewReader(bytes.NewReader(wrapped), int64(len(wrapped)), opc.WithMaxDecompressedPartSize(64<<10), opc.WithMaxDecompressedPackageSize(256<<10), opc.WithMaxNestingDepth(64))
		if e != nil {
			return
		}
		deck := CreateWithOptions(CreateOptions{Options: Options{SlideSize: SlideSizeCustom}, Width: dml.Pixels(16), Height: dml.Pixels(16)})
		selected := deck.AddSlide()
		selected.sxModel = nil
		selected.sxParsed = false
		deck.reader = &opc.ReadCloser{Reader: *reader}

		limits := render.Limits{MaxDimension: 64, MaxPixels: 4096, MaxOperations: 128, MaxPathSegments: 256, MaxPixelVisits: 1 << 20, MaxEdgeChecks: 1 << 20, MaxOutputBytes: 16 << 10, MaxImagePixels: 4096, MaxImageBytes: 16 << 10}
		page, e := deck.Slides()[0].PrepareRender(context.Background(), render.Options{Limits: limits, MaxSourceBytes: 64 << 10, MaxLayoutNodes: 2048})
		if e != nil {
			return
		}
		var out bytes.Buffer
		if e = page.WritePNG(context.Background(), &out, 96); e != nil {
			if errors.Is(e, render.ErrLimit) {
				return
			}
			t.Fatal(e)
		}
		img, e := png.Decode(&out)
		if e != nil || img.Bounds().Dx() != 16 || img.Bounds().Dy() != 16 {
			t.Fatalf("PNG: %v", e)
		}
	})
}
