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
	themed := bytes.Replace(source, []byte(`<a:srgbClr val="FF0000"/>`), []byte(`<a:schemeClr val="accent1"><a:lumMod val="75000"/><a:lumOff val="25000"/></a:schemeClr>`), 1)
	f.Add(bytes.Replace(themed, []byte(`<p:cSld><p:spTree>`), []byte(`<p:cSld><p:bg><p:bgRef idx="1001"><a:schemeClr val="bg1"/></p:bgRef></p:bg><p:spTree>`), 1))
	f.Add([]byte("<p:sld/>"))
	// Seed a table so mutations reach the table profile.
	tables := CreateWithOptions(CreateOptions{Options: Options{SlideSize: SlideSizeCustom}, Width: dml.Pixels(16), Height: dml.Pixels(16)})
	tbl := tables.AddSlide().AddTable(2, 2)
	tbl.Cell(0, 0).SetText("A")
	tbl.Cell(1, 1).SetBorders(&TableBorder{Width: dml.Pixels(1), Color: dml.ColorBlack, Style: BorderStyleSingle})
	f.Add(bytes.Replace(source, []byte(`</p:spTree>`), []byte(renderConnectorXML(` flipH="1"`, renderDiagonal, renderRedLine, renderLnStyle)+`</p:spTree>`), 1))
	// A rotated custom geometry with guides, an arc and a curve, filled with
	// a gradient.
	f.Add(bytes.Replace(source, []byte(`<a:prstGeom prst="rect"><a:avLst/></a:prstGeom><a:solidFill><a:srgbClr val="FF0000"/></a:solidFill>`), []byte(`<a:custGeom><a:avLst/><a:gdLst><a:gd name="a" fmla="*/ w 1 2"/><a:gd name="b" fmla="sat2 a h w"/></a:gdLst><a:rect l="l" t="t" r="r" b="b"/><a:pathLst><a:path w="100" h="100"><a:moveTo><a:pt x="0" y="50"/></a:moveTo><a:arcTo wR="50" hR="25" stAng="10800000" swAng="-5400000"/><a:cubicBezTo><a:pt x="0" y="0"/><a:pt x="100" y="0"/><a:pt x="100" y="100"/></a:cubicBezTo><a:close/></a:path></a:pathLst></a:custGeom><a:gradFill><a:gsLst><a:gs pos="0"><a:srgbClr val="FF0000"/></a:gs><a:gs pos="100000"><a:prstClr val="ltBlue"/></a:gs></a:gsLst><a:lin ang="2700000" scaled="1"/></a:gradFill>`), 1))
	if withTable, e := tables.SaveBytes(); e == nil {
		f.Add(fuzzseed.ZipEntry(withTable, "ppt/slides/slide1.xml"))
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 16<<10 {
			t.Skip()
		}
		wrapped := fuzzseed.ReplaceZipEntry(valid, "ppt/slides/slide1.xml", data)
		if wrapped == nil {
			t.Fatal("fixture")
		}
		// The lazy parse relies on Open having validated the part; best effort
		// always reaches it, so keep to inputs Open accepts.
		opened, e := OpenReader(bytes.NewReader(wrapped), int64(len(wrapped)))
		if e != nil {
			return
		}
		_ = opened.Close()
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
		// Best effort follows other paths through hostile input; it must not
		// panic either.
		_, _ = deck.Slides()[0].PrepareRender(context.Background(), render.Options{Limits: limits, MaxSourceBytes: 64 << 10, MaxLayoutNodes: 2048, Warn: func(error) {}})
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
