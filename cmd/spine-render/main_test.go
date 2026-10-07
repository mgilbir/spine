package main

import (
	"bytes"
	"context"
	"image"
	"io"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mgilbir/spine/chart"
	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/docx"
	"github.com/mgilbir/spine/pptx"
	"github.com/mgilbir/spine/pptx/pptxrender"
	"github.com/mgilbir/spine/render"
	"github.com/mgilbir/spine/xlsx"
)

func TestRenderInputFiles(t *testing.T) {
	dir := t.TempDir()
	d := docx.Create()
	s := d.DefaultSection()
	s.SetPageSize(200, 150)
	s.SetMargins(docx.PageMargins{Top: 10, Bottom: 10, Left: 10, Right: 10})
	for i := 0; i < 2; i++ {
		p := d.AddParagraph()
		p.SetAlignment(docx.AlignmentLeft)
		p.SetLineSpacingExact(18)
		p.SetSpaceBefore(0)
		p.SetSpaceAfter(0)
		if i == 1 {
			p.SetPageBreakBefore(true)
		}
		r := p.AddRun()
		r.SetText("Page")
		r.SetFont("Calibri")
		r.SetFontSize(12)
		r.SetBold(false)
		r.SetItalic(false)
		r.SetStrike(false)
		r.SetColor("000000")
	}
	if err := d.Save(filepath.Join(dir, "in.docx")); err != nil {
		t.Fatal(err)
	}
	p := pptx.Create()
	layout, err := p.LayoutByType(pptx.LayoutBlank)
	if err != nil {
		t.Fatal(err)
	}
	p.AddSlideFromLayout(layout)
	p.AddSlideFromLayout(layout)
	if err = p.Save(filepath.Join(dir, "in.pptx")); err != nil {
		t.Fatal(err)
	}
	w := xlsx.Create()
	sheet, err := w.AddSheet("Data")
	if err != nil {
		t.Fatal(err)
	}
	if err = sheet.SetColWidth(1, 16); err != nil {
		t.Fatal(err)
	}
	if err = sheet.SetRowHeight(1, 24); err != nil {
		t.Fatal(err)
	}
	cell, err := sheet.Cell("A1")
	if err != nil {
		t.Fatal(err)
	}
	cell.SetString("Cell")
	if err = w.Save(filepath.Join(dir, "in.xlsx")); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		ext, prefix string
		count       int
	}{{"docx", "page", 2}, {"pptx", "slide", 2}, {"xlsx", "sheet", 1}} {
		t.Run(tc.ext, func(t *testing.T) {
			out := filepath.Join(dir, tc.ext)
			c := config{input: filepath.Join(dir, "in."+tc.ext), out: out, format: "both", dpi: 96, maxPages: 10, work: 1 << 30, timeout: time.Minute, fallback: true}
			if tc.ext == "xlsx" {
				c.cellRange = "A1:A2"
			}
			if err := run(context.Background(), c); err != nil {
				t.Fatal(err)
			}
			files, err := os.ReadDir(out)
			if err != nil || len(files) != tc.count*2 {
				t.Fatalf("outputs: %v %v", files, err)
			}
			data, err := os.ReadFile(filepath.Join(out, tc.prefix+"-0001.png"))
			if err != nil {
				t.Fatal(err)
			}
			if _, err = png.Decode(strings.NewReader(string(data))); err != nil {
				t.Fatal(err)
			}
			if err = run(context.Background(), c); err == nil {
				t.Fatal("existing outputs overwritten")
			}
			after, err := os.ReadFile(filepath.Join(out, tc.prefix+"-0001.png"))
			if err != nil || string(after) != string(data) {
				t.Fatal("existing PNG changed")
			}
		})
	}
	c := config{input: filepath.Join(dir, "in.docx"), out: filepath.Join(dir, "limited"), format: "png", dpi: 96, maxPages: 1, work: 1 << 30, timeout: time.Minute, fallback: true}
	if err := run(context.Background(), c); err == nil || !strings.Contains(err.Error(), "max-pages") {
		t.Fatalf("page limit: %v", err)
	}
	c.maxPages = 10
	c.out = filepath.Join(dir, "missing-font")
	c.fallback = false
	if err := run(context.Background(), c); err == nil || !strings.Contains(err.Error(), "unresolved font") {
		t.Fatalf("font: %v", err)
	}
	c.input = filepath.Join(dir, "in.xlsx")
	c.cellRange = ""
	c.out = filepath.Join(dir, "no-range")
	if err := run(context.Background(), c); err == nil || !strings.Contains(err.Error(), "requires -range") {
		t.Fatalf("range: %v", err)
	}
}

func TestKeepGoingSkipsUnrenderableSlides(t *testing.T) {
	dir := t.TempDir()
	p := pptx.Create()
	layout, err := p.LayoutByType(pptx.LayoutBlank)
	if err != nil {
		t.Fatal(err)
	}
	p.AddSlideFromLayout(layout)
	unsupported := p.AddSlideFromLayout(layout)
	if err = unsupported.AddShape(pptx.NewAutoShape("notAShape")); err != nil {
		t.Fatal(err)
	}
	p.AddSlideFromLayout(layout)
	input := filepath.Join(dir, "in.pptx")
	if err = p.Save(input); err != nil {
		t.Fatal(err)
	}
	c := config{input: input, out: filepath.Join(dir, "strict"), format: "png", dpi: 96, maxPages: 10, timeout: time.Minute, strict: true}
	if err = run(context.Background(), c); err == nil || !strings.Contains(err.Error(), "slide 2") {
		t.Fatalf("strict: %v", err)
	}
	if _, err = os.Stat(filepath.Join(c.out, "slide-0003.png")); !os.IsNotExist(err) {
		t.Fatalf("strict run continued: %v", err)
	}
	var warnings strings.Builder
	c.out, c.keepGoing, c.warn = filepath.Join(dir, "lenient"), true, &warnings
	err = run(context.Background(), c)
	if err == nil || !strings.Contains(err.Error(), "skipped 1 of 3: slide 2") {
		t.Fatalf("keep going: %v", err)
	}
	if !strings.Contains(warnings.String(), "slide 2: skipped:") || !strings.Contains(warnings.String(), "notAShape") {
		t.Fatalf("warnings: %q", warnings.String())
	}
	for name, want := range map[string]bool{"slide-0001.png": true, "slide-0002.png": false, "slide-0003.png": true} {
		if _, err := os.Stat(filepath.Join(c.out, name)); (err == nil) != want {
			t.Fatalf("%s: %v", name, err)
		}
	}
	// A cancelled run stops rather than skipping every remaining page.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	c.out, warnings = filepath.Join(dir, "cancelled"), strings.Builder{}
	if err = run(ctx, c); err == nil || strings.Contains(err.Error(), "skipped") {
		t.Fatalf("cancelled: %v", err)
	}
}

func TestDefaultShapeWorkCoversSlideText(t *testing.T) {
	dir := t.TempDir()
	p := pptx.Create()
	layout, err := p.LayoutByType(pptx.LayoutBlank)
	if err != nil {
		t.Fatal(err)
	}
	slide := p.AddSlideFromLayout(layout)
	box := pptx.NewTextBox()
	box.SetPosition(dml.Inches(0.5), dml.Inches(0.5))
	box.SetSize(dml.Inches(8), dml.Inches(6))
	r := box.TextFrame().AddParagraph().AddRun()
	r.SetText(strings.Repeat("The quick brown fox jumps over the lazy dog. ", 14))
	r.SetFont("Calibri")
	r.SetFontSize(12)
	r.SetColor(dml.ColorBlack)
	if err = slide.AddShape(box); err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(dir, "in.pptx")
	if err = p.Save(input); err != nil {
		t.Fatal(err)
	}
	c := config{input: input, out: filepath.Join(dir, "default"), format: "png", dpi: 96, maxPages: 10, timeout: time.Minute, fallback: true}
	if err = run(context.Background(), c); err != nil {
		t.Fatalf("default budget: %v", err)
	}
	// The default is the library's; a budget far below the text's work is
	// still refused.
	c.out, c.work, c.strict = filepath.Join(dir, "small"), 1<<10, true
	if err = run(context.Background(), c); err == nil || !strings.Contains(err.Error(), "resource limit") {
		t.Fatalf("small budget: %v", err)
	}
}

func TestDefaultEdgeChecksCoverBusySlides(t *testing.T) {
	dir := t.TempDir()
	p := pptx.Create()
	layout, err := p.LayoutByType(pptx.LayoutBlank)
	if err != nil {
		t.Fatal(err)
	}
	slide := p.AddSlideFromLayout(layout)
	for i := 0; i < 3; i++ {
		e := pptx.NewAutoShape("ellipse")
		e.SetPosition(0, 0)
		e.SetSize(dml.Inches(10), dml.Inches(7.5))
		e.SetFill(dml.NewSolidFill(dml.ColorBlue))
		e.SetNoLine()
		if err = slide.AddShape(e); err != nil {
			t.Fatal(err)
		}
	}
	input := filepath.Join(dir, "in.pptx")
	if err = p.Save(input); err != nil {
		t.Fatal(err)
	}
	c := config{input: input, out: filepath.Join(dir, "default"), format: "png", dpi: 144, maxPages: 10, timeout: time.Minute}
	if err = run(context.Background(), c); err != nil {
		t.Fatalf("default budget: %v", err)
	}
	c.out, c.edges = filepath.Join(dir, "library"), 64<<20
	if err = run(context.Background(), c); err == nil || !strings.Contains(err.Error(), "resource limit") {
		t.Fatalf("library budget: %v", err)
	}
}

func TestBestEffortWarnsAndDraws(t *testing.T) {
	dir := t.TempDir()
	p := pptx.Create()
	layout, err := p.LayoutByType(pptx.LayoutBlank)
	if err != nil {
		t.Fatal(err)
	}
	if err = p.AddSlideFromLayout(layout).AddShape(pptx.NewAutoShape("notAShape")); err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(dir, "in.pptx")
	if err = p.Save(input); err != nil {
		t.Fatal(err)
	}
	var warnings strings.Builder
	c := config{input: input, out: filepath.Join(dir, "out"), format: "png", dpi: 96, maxPages: 10, timeout: time.Minute, warn: &warnings}
	if err = run(context.Background(), c); err != nil {
		t.Fatalf("best effort: %v", err)
	}
	if !strings.Contains(warnings.String(), "slide 1: warning:") || !strings.Contains(warnings.String(), "notAShape") {
		t.Fatalf("warnings: %q", warnings.String())
	}
	if _, err = os.Stat(filepath.Join(c.out, "slide-0001.png")); err != nil {
		t.Fatal(err)
	}
}

func TestBestEffortDrawsMissingBoldWithRegular(t *testing.T) {
	dir := t.TempDir()
	p := pptx.Create()
	layout, err := p.LayoutByType(pptx.LayoutBlank)
	if err != nil {
		t.Fatal(err)
	}
	box := pptx.NewTextBox()
	box.SetPosition(dml.Pixels(10), dml.Pixels(10))
	box.SetSize(dml.Pixels(200), dml.Pixels(40))
	r := box.TextFrame().AddParagraph().AddRun()
	r.SetText("Bold")
	r.SetFont("Unmapped")
	r.SetBold(true)
	r.SetColor(dml.ColorBlack)
	if err = p.AddSlideFromLayout(layout).AddShape(box); err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(dir, "in.pptx")
	if err = p.Save(input); err != nil {
		t.Fatal(err)
	}
	var warnings strings.Builder
	c := config{input: input, out: filepath.Join(dir, "out"), format: "png", dpi: 96, maxPages: 10, timeout: time.Minute, fallback: true, warn: &warnings}
	if err = run(context.Background(), c); err != nil {
		t.Fatal(err)
	}
	if got := warnings.String(); strings.Count(got, "Unmapped bold drawn with a regular face") != 1 || strings.Contains(got, "text left out") {
		t.Fatalf("warnings: %q", got)
	}
	c.out, c.strict, warnings = filepath.Join(dir, "strict"), true, strings.Builder{}
	if err = run(context.Background(), c); err == nil || !strings.Contains(err.Error(), "unresolved font") {
		t.Fatalf("strict: %v", err)
	}
}

// A widescreen slide of overlapping full-slide shapes and a photo renders
// at the default DPI within the command's default budgets, which the
// library's own would refuse.
func TestDefaultBudgetsCoverWidescreenAtDefaultDPI(t *testing.T) {
	dir := t.TempDir()
	p := pptx.CreateWidescreen()
	layout, err := p.LayoutByType(pptx.LayoutBlank)
	if err != nil {
		t.Fatal(err)
	}
	slide := p.AddSlideFromLayout(layout)
	for i := 0; i < 6; i++ {
		e := pptx.NewAutoShape("roundRect")
		e.SetPosition(0, 0)
		e.SetSize(dml.Inches(13.333), dml.Inches(7.5))
		e.SetFill(dml.NewSolidFill(dml.ColorBlue))
		e.SetNoLine()
		if err = slide.AddShape(e); err != nil {
			t.Fatal(err)
		}
	}
	photo := image.NewNRGBA(image.Rect(0, 0, 2000, 1500))
	for i := range photo.Pix {
		photo.Pix[i] = uint8(i * 7)
	}
	var encoded bytes.Buffer
	if err = png.Encode(&encoded, photo); err != nil {
		t.Fatal(err)
	}
	pic := pptx.NewPicture()
	pic.SetImageData(encoded.Bytes(), "image/png")
	pic.SetPosition(0, 0)
	pic.SetSize(dml.Inches(13.333), dml.Inches(7.5))
	if err = slide.AddShape(pic); err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(dir, "in.pptx")
	if err = p.Save(input); err != nil {
		t.Fatal(err)
	}
	c := config{input: input, out: filepath.Join(dir, "default"), format: "png", dpi: defaultDPI, maxPages: 10, timeout: time.Minute}
	if err = run(context.Background(), c); err != nil {
		t.Fatalf("default budgets: %v", err)
	}
	f, err := os.Open(filepath.Join(c.out, "slide-0001.png"))
	if err != nil {
		t.Fatal(err)
	}
	cfg, err := png.DecodeConfig(f)
	if e := f.Close(); err == nil {
		err = e
	}
	if err != nil || cfg.Width != 3840 || cfg.Height != 2160 {
		t.Fatalf("size %dx%d, %v", cfg.Width, cfg.Height, err)
	}
}

func TestDefaultDPIBudgetsAreNeeded(t *testing.T) {
	// The library's own limits refuse a widescreen slide at the default
	// DPI once a dozen full-slide shapes overlap.
	dir := t.TempDir()
	p := pptx.CreateWidescreen()
	layout, err := p.LayoutByType(pptx.LayoutBlank)
	if err != nil {
		t.Fatal(err)
	}
	slide := p.AddSlideFromLayout(layout)
	for i := 0; i < 12; i++ {
		e := pptx.NewAutoShape("rect")
		e.SetSize(dml.Inches(13.333), dml.Inches(7.5))
		e.SetFill(dml.NewSolidFill(dml.ColorBlue))
		e.SetNoLine()
		if err = slide.AddShape(e); err != nil {
			t.Fatal(err)
		}
	}
	input := filepath.Join(dir, "in.pptx")
	if err = p.Save(input); err != nil {
		t.Fatal(err)
	}
	page, err := pptxrender.PrepareSlide(context.Background(), p.Slides()[0], render.Options{})
	if err != nil {
		t.Fatal(err)
	}
	if err = page.WritePNG(context.Background(), io.Discard, defaultDPI); err == nil || !strings.Contains(err.Error(), "resource limit") {
		t.Fatalf("library limits at %v DPI: %v", float64(defaultDPI), err)
	}
}

func TestChartsDrawnWithEmbeddedRenderer(t *testing.T) {
	dir := t.TempDir()
	p := pptx.Create()
	layout, err := p.LayoutByType(pptx.LayoutBlank)
	if err != nil {
		t.Fatal(err)
	}
	c := chart.NewColumn()
	c.SetCategories([]string{"A", "B"})
	c.AddSeries("S", []float64{3, 5}).SetColor("FF0000")
	if err = p.AddSlideFromLayout(layout).AddChart(c, int64(dml.Pixels(100)), int64(dml.Pixels(100)), int64(dml.Pixels(400)), int64(dml.Pixels(300))); err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(dir, "in.pptx")
	if err = p.Save(input); err != nil {
		t.Fatal(err)
	}
	// red counts the chart's red bar pixels.
	red := func(out string) int {
		data, err := os.ReadFile(filepath.Join(out, "slide-0001.png"))
		if err != nil {
			t.Fatal(err)
		}
		img, err := png.Decode(bytes.NewReader(data))
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		for y := 100; y < 400; y++ {
			for x := 100; x < 500; x++ {
				r, g, b, _ := img.At(x, y).RGBA()
				if r>>8 > 200 && g>>8 < 60 && b>>8 < 60 {
					n++
				}
			}
		}
		return n
	}
	var warnings strings.Builder
	cfg := config{input: input, out: filepath.Join(dir, "charts"), format: "png", dpi: 96, maxPages: 10, timeout: time.Minute, warn: &warnings, charts: true}
	if err = run(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	if n := red(cfg.out); n < 1000 || warnings.Len() != 0 {
		t.Fatalf("charts: %d red pixels, warnings %q", n, warnings.String())
	}
	// Without the renderer the chart is left out and reported.
	warnings.Reset()
	cfg.out, cfg.charts = filepath.Join(dir, "none"), false
	if err = run(context.Background(), cfg); err != nil {
		t.Fatal(err)
	}
	if n := red(cfg.out); n != 0 || !strings.Contains(warnings.String(), "chart without a chart renderer") {
		t.Fatalf("no charts: %d red pixels, warnings %q", n, warnings.String())
	}
}
