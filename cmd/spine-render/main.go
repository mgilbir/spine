// Command spine-render writes native PNG/SVG previews of Office files.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/mgilbir/forme/fonts/notosans"
	"github.com/mgilbir/forme/shape"
	"github.com/mgilbir/spine/docx"
	"github.com/mgilbir/spine/pptx"
	"github.com/mgilbir/spine/render"
	"github.com/mgilbir/spine/xlsx"
)

// defaultShapeWork replaces the library's 64 Mi default, which suits a single
// paragraph: a lookup unit is not a duration, and complete fonts charge about a
// million units per byte of text (Noto Sans) while shaping it in microseconds.
// Within a run the document controls only the text length, the caller chooses
// the fonts, and -timeout bounds the whole command.
const defaultShapeWork = 16 << 30

// defaultEdgeChecks replaces the library's 64 Mi path painting budget, which a
// slide of text, circles and outlined boxes exceeds at the default 144 DPI;
// one such slide needed up to 256 Mi and painted in about a quarter of a
// second. -timeout bounds the command.
const defaultEdgeChecks = 1 << 30

type fontFlags []string

func (f *fontFlags) String() string     { return strings.Join(*f, ";") }
func (f *fontFlags) Set(s string) error { *f = append(*f, s); return nil }

type config struct {
	input, out, format, cellRange, sheet string
	dpi                                  float64
	maxPages                             int
	work, edges                          int64
	timeout                              time.Duration
	fonts                                fontFlags
	fallback                             bool
	keepGoing, strict                    bool
	warn                                 io.Writer // skipped-page reports; nil is standard error
}

func main() {
	var c config
	flag.StringVar(&c.input, "input", "", "input .docx, .pptx or .xlsx file (required)")
	flag.StringVar(&c.out, "out", "", "output directory (required; existing files are not overwritten)")
	flag.StringVar(&c.format, "format", "png", "png, svg or both")
	flag.StringVar(&c.cellRange, "range", "", "XLSX cell range, e.g. A1:D20 (required for XLSX)")
	flag.StringVar(&c.sheet, "sheet", "", "XLSX sheet name; empty selects all sheets")
	flag.Float64Var(&c.dpi, "dpi", 144, "output DPI")
	flag.IntVar(&c.maxPages, "max-pages", 100, "maximum total output pages/slides/sheets")
	flag.Int64Var(&c.edges, "edge-checks", 0, "path painting budget per output in edge checks; 0 uses 1 Gi")
	flag.Int64Var(&c.work, "shape-work", 0, "shaping-work budget per slide, page or sheet in conservative lookup units; 0 uses 16 Gi")
	flag.DurationVar(&c.timeout, "timeout", time.Minute, "total rendering timeout")
	flag.Var(&c.fonts, "font", "repeatable FAMILY[:regular|bold|italic|bolditalic]=FONT_FILE mapping")
	flag.BoolVar(&c.fallback, "fallback-noto", false, "explicitly substitute embedded Noto Sans for unresolved regular fonts")
	flag.BoolVar(&c.keepGoing, "keep-going", false, "report and skip slides or sheets that cannot be rendered, then exit with an error")
	flag.BoolVar(&c.strict, "strict", false, "fail on any content that cannot be drawn instead of warning and drawing the rest")
	flag.Parse()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if err := run(ctx, c); err != nil {
		fmt.Fprintln(os.Stderr, "spine-render:", err)
		os.Exit(1)
	}
}

func resolver(c config) (render.FontResolver, error) {
	if len(c.fonts) > 32 {
		return nil, fmt.Errorf("at most 32 font mappings are allowed")
	}
	faces := make(map[render.FontRequest]*shape.Face)
	var total int64
	for _, spec := range c.fonts {
		key, path, ok := strings.Cut(spec, "=")
		if !ok || key == "" || path == "" {
			return nil, fmt.Errorf("invalid font mapping %q", spec)
		}
		r := render.FontRequest{Family: key}
		if family, style, found := strings.Cut(key, ":"); found {
			r.Family = family
			switch style {
			case "regular":
			case "bold":
				r.Bold = true
			case "italic":
				r.Italic = true
			case "bolditalic":
				r.Bold = true
				r.Italic = true
			default:
				return nil, fmt.Errorf("invalid font style %q", style)
			}
		}
		if r.Family == "" || faces[r] != nil {
			return nil, fmt.Errorf("empty or duplicate font family %q", key)
		}
		f, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		data, err := io.ReadAll(io.LimitReader(f, (32<<20)+1))
		closeErr := f.Close()
		if err != nil {
			return nil, err
		}
		if closeErr != nil {
			return nil, closeErr
		}
		total += int64(len(data))
		if total > 32<<20 {
			return nil, fmt.Errorf("font files exceed 32 MiB budget")
		}
		face, err := shape.Load(data)
		if err != nil {
			return nil, fmt.Errorf("font %q: %w", key, err)
		}
		faces[r] = face
	}
	var fallback *shape.Face
	if c.fallback {
		var err error
		fallback, err = notosans.Face()
		if err != nil {
			return nil, err
		}
	}
	return func(ctx context.Context, r render.FontRequest) (*shape.Face, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if face := faces[r]; face != nil {
			return face, nil
		}
		if fallback != nil && !r.Bold && !r.Italic {
			return fallback, nil
		}
		return nil, fmt.Errorf("unresolved font %+v; provide -font or explicitly choose -fallback-noto for regular fonts", r)
	}, nil
}

func run(ctx context.Context, c config) (result error) {
	if c.input == "" || c.out == "" || c.maxPages < 1 || c.maxPages > 10000 || c.work < 0 || c.edges < 0 || c.timeout <= 0 || c.dpi <= 0 || math.IsNaN(c.dpi) || math.IsInf(c.dpi, 0) {
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
	fonts, err := resolver(c)
	if err != nil {
		return err
	}
	work := c.work
	if work == 0 {
		work = defaultShapeWork
	}
	edges := c.edges
	if edges == 0 {
		edges = defaultEdgeChecks
	}
	opts := render.Options{Fonts: fonts, Limits: render.Limits{MaxShapeWork: work, MaxEdgeChecks: edges}}
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
	warnings := 0
	// Unless -strict, slides draw what they can and report the rest.
	withWarnings := func(label string) render.Options {
		o := opts
		if !c.strict {
			o.Warn = func(err error) {
				warnings++
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
				warnings++
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
			page, err := slide.PrepareRender(ctx, withWarnings(fmt.Sprintf("slide %d", i+1)))
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
		for i := 1; i <= c.maxPages+1; i++ {
			page, err := d.PrepareRender(ctx, i, opts)
			if errors.Is(err, docx.ErrRenderPageOutOfRange) {
				break
			}
			if err != nil {
				return fmt.Errorf("page %d: %w", i, err)
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
			page, err := sheet.PrepareRender(ctx, c.cellRange, opts)
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
