package docxrender

import (
	"bytes"
	"context"
	"errors"
	"image/color"
	"image/png"
	"reflect"
	"testing"

	"github.com/mgilbir/forme/fonttest"
	"github.com/mgilbir/forme/shape"
	"github.com/mgilbir/spine/docx"
	"github.com/mgilbir/spine/internal/fuzzseed"
	"github.com/mgilbir/spine/render"
)

func renderTestDocument(t testing.TB) (*docx.Document, *docx.Run, render.Options) {
	t.Helper()
	d := docx.Create()
	section := d.DefaultSection()
	section.SetPageSize(69, 45)
	section.SetMargins(docx.PageMargins{Top: 4.5, Bottom: 4.5, Left: 4.5, Right: 4.5})
	p := d.AddParagraph()
	p.SetAlignment(docx.AlignmentLeft)
	p.SetLineSpacingExact(12)
	p.SetSpaceBefore(0)
	p.SetSpaceAfter(0)
	r := p.AddRun()
	r.SetText("AA AA AA AA AA AA AA AA")
	r.SetFont("Fixture")
	r.SetFontSize(12)
	r.SetBold(false)
	r.SetItalic(false)
	r.SetStrike(false)
	r.SetColor("000000")
	f, err := shape.Load(fonttest.SFNT(fonttest.SFNTOptions{UnitsPerEm: 2000, Ascent: 1600, Descent: -400, Glyphs: []fonttest.Glyph{{Rune: 'A', Advance: 2000, HasShape: true, Ink: [4]int{0, 0, 2000, 1600}}, {Rune: ' ', Advance: 1000}}}))
	if err != nil {
		t.Fatal(err)
	}
	opts := render.Options{Fonts: func(ctx context.Context, r render.FontRequest) (*shape.Face, error) {
		if r.Family != "Fixture" || r.Bold || r.Italic {
			t.Fatalf("font: %+v", r)
		}
		return f, nil
	}}
	return d, r, opts
}

func TestRenderPhysicalPagesWidowControlAndOwnership(t *testing.T) {
	d, r, opts := renderTestDocument(t)
	before, err := d.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	page1, err := PreparePage(context.Background(), d, 1, opts)
	if err != nil {
		t.Fatal(err)
	}
	page2, err := PreparePage(context.Background(), d, 2, opts)
	if err != nil {
		t.Fatal(err)
	}
	if page, err := PreparePage(context.Background(), d, 3, opts); page != nil || (!errors.Is(err, render.ErrInvalid) || !errors.Is(err, ErrPageOutOfRange)) {
		t.Fatalf("outside: %v %v", page, err)
	}
	after, err := d.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(readAllZipParts(t, before), readAllZipParts(t, after)) {
		t.Fatal("render changed source")
	}
	r.SetText("AA")
	for _, page := range []*render.Page{page1, page2} {
		w, h, err := page.Size(96)
		if err != nil || w != 92 || h != 60 {
			t.Fatalf("size %d,%d: %v", w, h, err)
		}
		var out bytes.Buffer
		if err = page.WritePNG(context.Background(), &out, 96); err != nil {
			t.Fatal(err)
		}
		img, err := png.Decode(&out)
		if err != nil {
			t.Fatal(err)
		}
		for _, tc := range []struct {
			x, y  int
			black bool
		}{{7, 7, true}, {7, 23, true}, {7, 39, false}, {0, 0, false}} {
			want := color.NRGBA{A: 255}
			if !tc.black {
				want.R = 255
				want.G = 255
				want.B = 255
			}
			if got := color.NRGBAModel.Convert(img.At(tc.x, tc.y)).(color.NRGBA); got != want {
				t.Fatalf("%d,%d: %+v != %+v", tc.x, tc.y, got, want)
			}
		}
	}
	if page, err := PreparePage(context.Background(), d, 2, opts); page != nil || !errors.Is(err, render.ErrInvalid) {
		t.Fatalf("unsaved text: %v %v", page, err)
	}
	opened, err := docx.OpenReader(bytes.NewReader(before), int64(len(before)))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if e := opened.Close(); e != nil {
			t.Error(e)
		}
	}()
	if _, err = PreparePage(context.Background(), opened, 2, opts); err != nil {
		t.Fatal(err)
	}
}

func TestRenderPagesCheckWholeFlowAndBudgets(t *testing.T) {
	d, _, opts := renderTestDocument(t)
	opts.Limits.MaxShapeWork = 1
	if page, err := PreparePage(context.Background(), d, 1, opts); page != nil || !errors.Is(err, render.ErrLimit) {
		t.Fatalf("budget: %v %v", page, err)
	}
	opts.Limits = render.Limits{}
	// Unsupported later content must not produce a misleading first-page success.
	d.AddParagraphWithText("unsupported unstyled paragraph")
	if page, err := PreparePage(context.Background(), d, 1, opts); page != nil || !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("whole flow: %v %v", page, err)
	}
}

func TestRenderOriginalWordTextEffectRejectedBeforeFonts(t *testing.T) {
	d, _, opts := renderTestDocument(t)
	data, err := d.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	part := fuzzseed.ZipEntry(data, "word/document.xml")
	modified := bytes.Replace(part, []byte("</w:rPr>"), []byte("<w:unknownTextEffect/></w:rPr>"), 1)
	if bytes.Equal(part, modified) {
		t.Fatal("fixture unchanged")
	}
	wrapped := fuzzseed.ReplaceZipEntry(data, "word/document.xml", modified)
	if wrapped == nil {
		t.Fatal("seed package")
	}
	opened, err := docx.OpenReader(bytes.NewReader(wrapped), int64(len(wrapped)))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if e := opened.Close(); e != nil {
			t.Error(e)
		}
	}()
	opts.Fonts = func(context.Context, render.FontRequest) (*shape.Face, error) {
		t.Fatal("font resolution before source check")
		return nil, nil
	}
	if page, err := PreparePage(context.Background(), opened, 1, opts); page != nil || !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("source: %v %v", page, err)
	}
}

func TestRenderDefaultStyleUnknownEffectRejectedBeforeFonts(t *testing.T) {
	d, _, opts := renderTestDocument(t)
	data, err := d.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	part := fuzzseed.ZipEntry(data, "word/styles.xml")
	changed := bytes.Replace(part, []byte("</w:style>"), []byte("<w:rPr><w:unknownInheritedEffect/></w:rPr></w:style>"), 1)
	if bytes.Equal(part, changed) {
		t.Fatal("fixture unchanged")
	}
	wrapped := fuzzseed.ReplaceZipEntry(data, "word/styles.xml", changed)
	opened, err := docx.OpenReader(bytes.NewReader(wrapped), int64(len(wrapped)))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if e := opened.Close(); e != nil {
			t.Error(e)
		}
	}()
	opts.Fonts = func(context.Context, render.FontRequest) (*shape.Face, error) {
		t.Fatal("font resolution before default-style source check")
		return nil, nil
	}
	if page, err := PreparePage(context.Background(), opened, 1, opts); page != nil || !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("default source: %v %v", page, err)
	}
}

func TestRenderWidowConstraintCannotFitAndExplicitBreak(t *testing.T) {
	d, r, opts := renderTestDocument(t)
	// Three lines cannot be split over pages holding only two while preserving
	// both widow and orphan constraints. Refuse instead of silently violating one.
	d.DefaultSection().SetPageSize(69, 33)
	r.SetText("AA AA AA AA AA AA")
	if page, err := PreparePage(context.Background(), d, 1, opts); page != nil || !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("widow: %v %v", page, err)
	}
	d.DefaultSection().SetPageSize(69, 45)
	r.SetText("AA")
	p := d.AddParagraph()
	p.SetAlignment(docx.AlignmentLeft)
	p.SetLineSpacingExact(12)
	p.SetSpaceBefore(0)
	p.SetSpaceAfter(0)
	p.SetPageBreakBefore(true)
	newRun := p.AddRun()
	newRun.SetText("AA")
	newRun.SetFont("Fixture")
	newRun.SetFontSize(12)
	newRun.SetBold(false)
	newRun.SetItalic(false)
	newRun.SetStrike(false)
	newRun.SetColor("000000")
	if _, err := PreparePage(context.Background(), d, 2, opts); err != nil {
		t.Fatal(err)
	}
}

func FuzzDocxRender(f *testing.F) {
	d, _, prototype := renderTestDocument(f)
	valid, err := d.SaveBytes()
	if err != nil {
		f.Fatal(err)
	}
	face, err := prototype.Fonts(context.Background(), render.FontRequest{Family: "Fixture"})
	if err != nil {
		f.Fatal(err)
	}
	const part = "word/document.xml"
	f.Add(fuzzseed.ZipEntry(valid, part))
	f.Add([]byte(`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:p><w:r><w:t>AA</w:t></w:r></w:p></w:body></w:document>`))
	f.Add([]byte(`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main"><w:body><w:sectPr><w:pgSz w:w="2147483647" w:h="2147483647"/></w:sectPr></w:body></w:document>`))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 65536 {
			t.Skip()
		}
		wrapped := fuzzseed.ReplaceZipEntry(valid, part, data)
		if wrapped == nil {
			t.Fatal("seed")
		}
		doc, err := docx.OpenReader(bytes.NewReader(wrapped), int64(len(wrapped)))
		if err != nil {
			return
		}
		defer func() {
			if e := doc.Close(); e != nil {
				t.Error(e)
			}
		}()
		opts := render.Options{Fonts: func(context.Context, render.FontRequest) (*shape.Face, error) { return face, nil }, MaxSourceBytes: 65536, MaxLayoutNodes: 512, Limits: render.Limits{MaxDimension: 256, MaxPixels: 65536, MaxOperations: 128, MaxGlyphs: 256, MaxShapeWork: 65536, MaxOutputBytes: 65536, MaxTextBytes: 4096, MaxRunBytes: 1024}}
		page, err := PreparePage(context.Background(), doc, 1, opts)
		if err != nil {
			if page != nil {
				t.Fatal("partial page")
			}
			return
		}
		var out bytes.Buffer
		if err = page.WritePNG(context.Background(), &out, 96); err != nil && !errors.Is(err, render.ErrLimit) {
			t.Fatal(err)
		}
	})
}

func BenchmarkRenderPlainPhysicalPage(b *testing.B) {
	d, _, opts := renderTestDocument(b)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		page, err := PreparePage(context.Background(), d, 1, opts)
		if err != nil {
			b.Fatal(err)
		}
		var out bytes.Buffer
		if err = page.WritePNG(context.Background(), &out, 96); err != nil {
			b.Fatal(err)
		}
	}
}
