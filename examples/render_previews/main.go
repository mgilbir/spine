// Render a slide, a sheet range and two physical document pages without host
// fonts or external processes. Run: go run ./examples/render_previews -out DIR
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mgilbir/forme/fonts/notosans"
	"github.com/mgilbir/forme/shape"
	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/common/enum"
	"github.com/mgilbir/spine/docx"
	"github.com/mgilbir/spine/docx/docxrender"
	"github.com/mgilbir/spine/pptx"
	"github.com/mgilbir/spine/pptx/pptxrender"
	"github.com/mgilbir/spine/render"
	"github.com/mgilbir/spine/xlsx"
	"github.com/mgilbir/spine/xlsx/xlsxrender"
)

func main() {
	out := flag.String("out", "render-previews", "output directory")
	flag.Parse()
	if err := run(context.Background(), *out); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, dir string) error {
	font, err := notosans.Face()
	if err != nil {
		return err
	}
	// Explicitly substitute Forme's embedded Noto Sans for the sheet's Calibri
	// request and for Times New Roman, which is the family Word uses for text,
	// and for a paragraph mark, that names none. A production resolver should return the requested typeface or
	// deliberately choose its own substitution; no system fonts are discovered.
	// Conservative lookup work counts subtable bytes as well as inspected
	// glyphs. This known embedded font needs a larger cap than the default.
	opts := render.Options{Fonts: func(ctx context.Context, r render.FontRequest) (*shape.Face, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if r.Bold || r.Italic || (r.Family != "Noto Sans" && r.Family != "Calibri" && r.Family != "Times New Roman") {
			return nil, fmt.Errorf("unsupported example font: %+v", r)
		}
		return font, nil
	}, Limits: render.Limits{MaxDimension: 2048, MaxPixels: 4 << 20, MaxOutputBytes: 16 << 20}}
	if err = os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	p := pptx.CreateWithOptions(pptx.CreateOptions{Options: pptx.Options{SlideSize: pptx.SlideSizeCustom}, IncludeDefaultLayouts: true, Width: dml.Pixels(480), Height: dml.Pixels(270)})
	layout, err := p.LayoutByType(pptx.LayoutBlank)
	if err != nil {
		return err
	}
	slide := p.AddSlideFromLayout(layout)
	box := pptx.NewTextBox()
	box.SetPosition(dml.Pixels(32), dml.Pixels(32))
	box.SetSize(dml.Pixels(416), dml.Pixels(206))
	box.SetFill(dml.NewSolidFill(dml.NewRGB(35, 54, 74).ToColor()))
	box.SetNoLine()
	tf := box.TextFrame()
	tf.SetMargins(pptx.TextMargins{Left: dml.Pixels(16), Top: dml.Pixels(16), Right: dml.Pixels(16), Bottom: dml.Pixels(16)})
	tf.SetAutofit(pptx.AutofitNone)
	for i, text := range []string{"Native Go rendering", "PNG and SVG from one immutable snapshot."} {
		para := tf.AddParagraph()
		para.SetAlignment(enum.TextAlignLeft)
		para.SetBullet(pptx.BulletNone)
		para.SetLineSpacing(100000)
		para.SetSpaceBefore(0)
		para.SetSpaceAfter(0)
		r := para.AddRun()
		r.SetText(text)
		r.SetFont("Noto Sans")
		size := 14.0
		if i == 0 {
			size = 24
		}
		r.SetFontSize(size)
		r.SetBold(false)
		r.SetItalic(false)
		r.SetUnderline(enum.UnderlineNone)
		r.SetStrike(enum.StrikeNone)
		r.SetColor(dml.ColorWhite)
	}
	if err = slide.AddShape(box); err != nil {
		return err
	}
	page, err := pptxrender.PrepareSlide(ctx, slide, opts)
	if err != nil {
		return fmt.Errorf("slide: %w", err)
	}
	if err = writePair(ctx, dir, "slide", page); err != nil {
		return err
	}
	w := xlsx.Create()
	sheet, err := w.AddSheet("Preview")
	if err != nil {
		return err
	}
	for c := 1; c <= 3; c++ {
		if err = sheet.SetColWidth(c, 16); err != nil {
			return err
		}
	}
	for r := 1; r <= 5; r++ {
		if err = sheet.SetRowHeight(r, 24); err != nil {
			return err
		}
	}
	for ri, row := range [][]any{{"Preview", "Type", "Value"}, {"Text", "String", "Native Go"}, {"Number", "General", 123}, {"Boolean", "Logical", true}, {"Blank"}} {
		for ci, value := range row {
			if err = sheet.SetCellValue(xlsx.FormatCellRef(ri+1, ci+1), value); err != nil {
				return err
			}
		}
	}
	page, err = xlsxrender.PrepareRange(ctx, sheet, "A1:C5", opts)
	if err != nil {
		return fmt.Errorf("sheet: %w", err)
	}
	if err = writePair(ctx, dir, "sheet", page); err != nil {
		return err
	}
	d := docx.Create()
	section := d.DefaultSection()
	section.SetPageSize(360, 240)
	section.SetMargins(docx.PageMargins{Top: 24, Bottom: 24, Left: 24, Right: 24})
	for i, text := range []string{"This is a physical document page laid out entirely in Go. Fonts are supplied explicitly.", "This paragraph starts on the second physical page. Prepared snapshots remain independent of later edits."} {
		para := d.AddParagraph()
		para.SetAlignment(docx.AlignmentLeft)
		para.SetLineSpacingExact(18)
		para.SetSpaceBefore(0)
		para.SetSpaceAfter(0)
		if i == 1 {
			para.SetPageBreakBefore(true)
		}
		r := para.AddRun()
		r.SetText(text)
		r.SetFont("Noto Sans")
		r.SetFontSize(12)
		r.SetBold(false)
		r.SetItalic(false)
		r.SetStrike(false)
		r.SetColor("23364A")
	}
	for number := 1; number <= 2; number++ {
		page, err = docxrender.PreparePage(ctx, d, number, opts)
		if err != nil {
			return fmt.Errorf("document page %d: %w", number, err)
		}
		if err = writePair(ctx, dir, fmt.Sprintf("document-page-%d", number), page); err != nil {
			return err
		}
	}
	fmt.Printf("Wrote PNG and SVG previews to %s\n", dir)
	return nil
}

func writePair(ctx context.Context, dir, name string, page *render.Page) error {
	for _, ext := range []string{"png", "svg"} {
		file, err := os.Create(filepath.Join(dir, name+"."+ext))
		if err != nil {
			return err
		}
		if ext == "png" {
			err = page.WritePNG(ctx, file, 144)
		} else {
			err = page.WriteSVG(ctx, file, 144)
		}
		closeErr := file.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	}
	return nil
}
