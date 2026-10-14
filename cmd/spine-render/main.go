// Command spine-render writes native PNG/SVG previews of Office files.
//
// It exits 0 on success and for -help, 1 when the run fails (including a run
// that skipped pages under -keep-going), and 2 when the flags cannot be
// parsed. See docs/rendering.md for its flags and errors.
package main

import (
	"bytes"
	"context"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/png"
	"io"
	"math"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/mgilbir/aster"
	"github.com/mgilbir/forme/shape"
	"github.com/mgilbir/spine/docx"
	"github.com/mgilbir/spine/docx/docxrender"
	"github.com/mgilbir/spine/internal/renderfonts"
	"github.com/mgilbir/spine/pptx"
	"github.com/mgilbir/spine/pptx/pptxrender"
	"github.com/mgilbir/spine/render"
	"github.com/mgilbir/spine/xlsx"
	"github.com/mgilbir/spine/xlsx/xlsxrender"
)

// defaultDPI renders sharp on a display of twice the standard density.
const defaultDPI = 288

// defaultEdgeChecks replaces the library's 64 Mi path painting budget, which a
// slide of text, circles and outlined boxes exceeds; one such slide needed up
// to 256 Mi at 144 DPI, and the work grows with the square of the DPI, so the
// default allows four times that at 288 DPI. -timeout bounds the command.
const defaultEdgeChecks = 4 << 30

// Painting budgets scaled for 288 DPI, where a widescreen slide is 3840 by
// 2160 pixels: the library's 16 Mi pixels a page, 8192 pixels a side, 64 Mi
// pixel visits and 32 MiB of output suit about 144 DPI. A page's pixels are
// four bytes each in memory.
const (
	defaultMaxPixels      = 64 << 20
	defaultMaxDimension   = 16384
	defaultPixelVisits    = 1 << 30
	defaultMaxOutputBytes = 256 << 20
	// defaultPathSegments replaces the library's 100,000 glyph and path
	// segments a page, which a full page of body text in a complete font
	// exceeds (about 30 segments a glyph).
	defaultPathSegments = 8 << 20
	// defaultDocumentNodes replaces the library's 100,000 source and layout
	// nodes for a document, which a document of about fifteen pages exceeds.
	defaultDocumentNodes = 8 << 20
)

// defaultImagePixels replaces the library's 4 Mi decoded image pixels per
// page, which one phone photo exceeds. Previews downscale pictures after
// decoding; the budget bounds decoding memory, at four bytes a pixel.
const defaultImagePixels = 64 << 20

type config struct {
	input, out, format, cellRange, sheet string
	dpi                                  float64
	maxPages                             int
	work, edges, imagePixels             int64
	timeout                              time.Duration
	fonts                                renderfonts.Flags
	fallback                             bool
	keepGoing, strict                    bool
	charts                               bool
	warn                                 io.Writer // skipped-page reports; nil is standard error
}

func main() {
	var c config
	flag.StringVar(&c.input, "input", "", "input .docx, .pptx or .xlsx file (required)")
	flag.StringVar(&c.out, "out", "", "output directory (required; existing files are not overwritten)")
	flag.StringVar(&c.format, "format", "png", "png, svg or both")
	flag.StringVar(&c.cellRange, "range", "", "XLSX cell range, e.g. A1:D20 (required for XLSX)")
	flag.StringVar(&c.sheet, "sheet", "", "XLSX sheet name; empty selects all sheets")
	flag.Float64Var(&c.dpi, "dpi", defaultDPI, "output DPI")
	flag.IntVar(&c.maxPages, "max-pages", 100, "maximum total output pages/slides/sheets")
	flag.Int64Var(&c.edges, "edge-checks", 0, "path painting budget per output in edge checks; 0 uses "+strconv.Itoa(defaultEdgeChecks>>30)+" Gi")
	flag.Int64Var(&c.imagePixels, "image-pixels", 0, "decoded image pixels per slide, page or sheet; 0 uses 64 Mi")
	flag.Int64Var(&c.work, "shape-work", 0, "shaping-work budget per slide, page or sheet in Forme work units (about 50 a byte of Latin text, 300 of Devanagari); 0 uses the library default, 64 Mi")
	flag.DurationVar(&c.timeout, "timeout", time.Minute, "total rendering timeout")
	flag.Var(&c.fonts, "font", "repeatable FAMILY[:regular|bold|italic|bolditalic]=FONT_FILE[#FACE] mapping; FACE is the index from 0 of a face of a .ttc or .otc collection, required for one of several faces")
	flag.BoolVar(&c.fallback, "fallback-noto", false, "explicitly substitute embedded Noto Sans for unresolved regular fonts")
	flag.BoolVar(&c.keepGoing, "keep-going", false, "report and skip slides, pages or sheets that cannot be rendered (a DOCX that cannot be laid out is skipped whole), then exit with an error")
	flag.BoolVar(&c.strict, "strict", false, "fail on any content that cannot be drawn instead of warning and drawing the rest")
	flag.BoolVar(&c.charts, "charts", true, "draw PPTX charts with the embedded Vega renderer; false leaves them out")
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := run(ctx, c); err != nil {
		fmt.Fprintln(os.Stderr, "spine-render:", err)
		os.Exit(1)
	}
}

// fontFile is the renderfonts.File charts are given.
type fontFile = renderfonts.File

func resolver(c config) (render.FontResolver, []fontFile, error) {
	return renderfonts.Resolver([]string(c.fonts), c.fallback)
}

// chartMemory bounds what one chart's Vega specification may make the
// renderer hold.
const chartMemory = 256 << 20

// chartRenderer draws charts with aster, which loads nothing from outside
// the specification, and uses the mapped fonts, falling back to its embedded
// Liberation Sans. Each chart is bounded by chartMemory and by timeout; the
// renderer cannot be interrupted sooner. close releases it.
func chartRenderer(fonts []fontFile, timeout time.Duration) (render.ChartRenderer, func() error, error) {
	opts := []aster.Option{aster.WithLoader(aster.DenyLoader{}), aster.WithMemoryLimit(chartMemory), aster.WithTimeout(timeout)}
	var copied int64
	for _, f := range fonts {
		program := f.Program()
		if f.Face != nil {
			// Each face copies out the tables it shares with the others.
			if copied += int64(len(program)); copied > renderfonts.Budget {
				return nil, nil, fmt.Errorf("chart fonts copied out of font collections exceed 32 MiB budget; map fewer faces or use -charts=false")
			}
		}
		opts = append(opts, aster.WithFont(f.Family, program))
	}
	conv, err := aster.New(opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("chart renderer: %w", err)
	}
	draw := func(ctx context.Context, spec []byte, scale float64) (image.Image, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		data, err := conv.VegaToPNG(spec, aster.WithScale(scale))
		if err != nil {
			return nil, err
		}
		return png.Decode(bytes.NewReader(data))
	}
	return draw, conv.Close, nil
}

func run(ctx context.Context, c config) (result error) {
	if c.input == "" || c.out == "" || c.maxPages < 1 || c.maxPages > 10000 || c.work < 0 || c.edges < 0 || c.imagePixels < 0 || c.timeout <= 0 || c.dpi <= 0 || math.IsNaN(c.dpi) || math.IsInf(c.dpi, 0) {
		return fmt.Errorf("require -input, -out and positive bounded rendering options")
	}
	if c.format != "png" && c.format != "svg" && c.format != "both" {
		return fmt.Errorf("-format must be png, svg or both")
	}
	ext := strings.ToLower(filepath.Ext(c.input))
	if ext != ".docx" && ext != ".pptx" && ext != ".xlsx" {
		return fmt.Errorf("input must be .docx, .pptx or .xlsx")
	}
	if ext == ".xlsx" && c.cellRange == "" {
		return fmt.Errorf("XLSX requires -range, e.g. A1:D20")
	}
	if ext != ".xlsx" && (c.cellRange != "" || c.sheet != "") {
		return fmt.Errorf("-range and -sheet apply only to XLSX")
	}
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	fonts, fontFiles, err := resolver(c)
	if err != nil {
		return err
	}
	// Zero keeps the library's default, which Forme's work-based charging
	// sizes for whole documents.
	work := c.work
	edges := c.edges
	if edges == 0 {
		edges = defaultEdgeChecks
	}
	imagePixels := c.imagePixels
	if imagePixels == 0 {
		imagePixels = defaultImagePixels
	}
	// Image bytes and counts scale with the pixel budget.
	opts := render.Options{Fonts: fonts, Limits: render.Limits{MaxShapeWork: work, MaxEdgeChecks: edges, MaxImagePixels: imagePixels, MaxImageBytes: 256 << 20, MaxImages: 256,
		MaxPixels: defaultMaxPixels, MaxDimension: defaultMaxDimension, MaxPixelVisits: defaultPixelVisits, MaxOutputBytes: defaultMaxOutputBytes, MaxPathSegments: defaultPathSegments}}
	if c.charts && ext == ".pptx" {
		charts, closeCharts, err := chartRenderer(fontFiles, c.timeout)
		if err != nil {
			return err
		}
		defer func() { result = errors.Join(result, closeCharts()) }()
		opts.Charts = charts
	}
	if err = os.MkdirAll(c.out, 0755); err != nil {
		return err
	}
	warn := c.warn
	if warn == nil {
		warn = os.Stderr
	}
	var skipped []string
	// failed reports a page that could not be prepared. With -keep-going it is
	// skipped unless the run itself was cancelled or timed out.
	failed := func(label string, err error) error {
		if !c.keepGoing || ctx.Err() != nil {
			return fmt.Errorf("%s: %w", label, err)
		}
		// A skip that cannot be reported is not skipped silently.
		if _, werr := fmt.Fprintf(warn, "spine-render: %s: skipped: %v\n", label, err); werr != nil {
			return errors.Join(fmt.Errorf("%s: %w", label, err), werr)
		}
		skipped = append(skipped, label)
		return nil
	}
	warnings, approximations := 0, 0
	// Unless -strict, slides draw what they can and report the rest.
	withWarnings := func(label string) render.Options {
		o := opts
		if !c.strict {
			o.Warn = func(err error) {
				if errors.Is(err, render.ErrApproximated) {
					approximations++
				} else {
					warnings++
				}
				_, _ = fmt.Fprintf(warn, "spine-render: %s: warning: %v\n", label, err)
			}
		}
		return o
	}
	if !c.strict {
		// Best effort draws a bold or italic face nothing provides with the
		// family's regular face, or the regular fallback, reporting it once.
		exact, reported := opts.Fonts, map[render.FontRequest]bool{}
		opts.Fonts = func(ctx context.Context, r render.FontRequest) (*shape.Face, error) {
			face, err := exact(ctx, r)
			if err == nil || (!r.Bold && !r.Italic) || ctx.Err() != nil {
				return face, err
			}
			regular := r
			regular.Bold, regular.Italic = false, false
			face, rerr := exact(ctx, regular)
			if rerr != nil {
				return nil, err
			}
			if !reported[r] {
				reported[r] = true
				approximations++
				_, _ = fmt.Fprintf(warn, "spine-render: warning: %s %s drawn with a regular face\n", r.Family, renderStyleName(r))
			}
			return face, nil
		}
	}
	count := 0
	emit := func(label string, page *render.Page) error {
		if count >= c.maxPages {
			return fmt.Errorf("output exceeds -max-pages=%d", c.maxPages)
		}
		if err := writePage(ctx, c, label, page); err != nil {
			return err
		}
		count++
		return nil
	}
	switch ext {
	case ".pptx":
		p, err := pptx.Open(c.input)
		if err != nil {
			return err
		}
		defer func() { result = errors.Join(result, p.Close()) }()
		if p.SlideCount() > c.maxPages {
			return fmt.Errorf("slides exceed -max-pages")
		}
		for i, slide := range p.Slides() {
			page, err := pptxrender.PrepareSlide(ctx, slide, withWarnings(fmt.Sprintf("slide %d", i+1)))
			if err != nil {
				if err = failed(fmt.Sprintf("slide %d", i+1), err); err != nil {
					return err
				}
				continue
			}
			if err = emit(fmt.Sprintf("slide-%04d", i+1), page); err != nil {
				return err
			}
		}
	case ".docx":
		d, err := docx.Open(c.input)
		if err != nil {
			return err
		}
		defer func() { result = errors.Join(result, d.Close()) }()
		// The document is laid out and paginated once; pages are then prepared
		// from that layout.
		docOpts := withWarnings("document")
		if docOpts.MaxLayoutNodes == 0 {
			docOpts.MaxLayoutNodes = defaultDocumentNodes
		}
		pages, err := docxrender.Prepare(ctx, d, docOpts)
		if err != nil {
			if err = failed("document", err); err != nil {
				return err
			}
			break
		}
		if pages.Count() > c.maxPages {
			return fmt.Errorf("pages exceed -max-pages")
		}
		for i := 1; i <= pages.Count(); i++ {
			page, err := pages.Page(ctx, i)
			if err != nil {
				if err = failed(fmt.Sprintf("page %d", i), err); err != nil {
					return err
				}
				continue
			}
			if err = emit(fmt.Sprintf("page-%04d", i), page); err != nil {
				return err
			}
		}
	case ".xlsx":
		w, err := xlsx.Open(c.input)
		if err != nil {
			return err
		}
		defer func() { result = errors.Join(result, w.Close()) }()
		for i, sheet := range w.Sheets() {
			if c.sheet != "" && sheet.Name() != c.sheet {
				continue
			}
			page, err := xlsxrender.PrepareRange(ctx, sheet, c.cellRange, opts)
			if err != nil {
				if err = failed(fmt.Sprintf("sheet %q", sheet.Name()), err); err != nil {
					return err
				}
				continue
			}
			if err = emit(fmt.Sprintf("sheet-%04d", i+1), page); err != nil {
				return err
			}
		}
	}
	if warnings > 0 {
		fmt.Printf("%d warnings: some content was left out of the previews\n", warnings)
	}
	if approximations > 0 {
		fmt.Printf("%d warnings: some content was drawn approximately\n", approximations)
	}
	if len(skipped) > 0 {
		fmt.Printf("Rendered %d previews to %s\n", count, c.out)
		return fmt.Errorf("skipped %d of %d: %s", len(skipped), count+len(skipped), strings.Join(skipped, ", "))
	}
	if count == 0 {
		return fmt.Errorf("no pages selected; check the sheet name or input contents")
	}
	fmt.Printf("Rendered %d previews to %s\n", count, c.out)
	return nil
}

func writePage(ctx context.Context, c config, name string, page *render.Page) error {
	exts := []string{c.format}
	if c.format == "both" {
		exts = []string{"png", "svg"}
	}
	for _, ext := range exts {
		path := filepath.Join(c.out, name+"."+ext)
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
		if err != nil {
			return err
		}
		if ext == "png" {
			err = page.WritePNG(ctx, file, c.dpi)
		} else {
			err = page.WriteSVG(ctx, file, c.dpi)
		}
		closeErr := file.Close()
		if err == nil {
			err = closeErr
		}
		if err != nil {
			_ = os.Remove(path)
			return fmt.Errorf("%s: %w", path, err)
		}
	}
	return nil
}

func renderStyleName(r render.FontRequest) string {
	switch {
	case r.Bold && r.Italic:
		return "bold italic"
	case r.Bold:
		return "bold"
	}
	return "italic"
}
