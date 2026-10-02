package docx

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"

	"github.com/mgilbir/forme/layout"
	"github.com/mgilbir/forme/shape"
	"github.com/mgilbir/forme/style"
	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/docx/internal/oxml"
	core "github.com/mgilbir/spine/internal/render"
	"github.com/mgilbir/spine/opc"
	"github.com/mgilbir/spine/render"
)

// ErrRenderPageOutOfRange identifies a page beyond the laid-out document.
var ErrRenderPageOutOfRange = errors.New("render page out of range")

// PrepareRender prepares a 1-based physical page from a bounded plain document
// flow. The first profile has one explicitly sized section, plain ASCII runs,
// one style per paragraph, exact line spacing and zero before/after spacing.
// It paginates the complete flow, respecting widow control and pageBreakBefore.
// Unsupported content returns no page, including on unselected pages. Native
// hhea line metrics do not promise identical Word pagination. See rendering.md.
func (d *Document) PrepareRender(ctx context.Context, page int, opts render.Options) (*render.Page, error) {
	if ctx == nil || d == nil || page < 1 {
		return nil, fmt.Errorf("%w: document/page", render.ErrInvalid)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	limits, err := core.ResolveLimits(opts.Limits)
	if err != nil {
		return nil, err
	}
	if page > limits.MaxOperations {
		return nil, render.ErrLimit
	}
	b, err := core.NewSourceBudget(opts.MaxSourceBytes, opts.MaxLayoutNodes)
	if err != nil {
		return nil, err
	}
	if d.settings != nil {
		return nil, fmt.Errorf("%w: document settings", render.ErrUnsupported)
	}
	rels := d.relationships[d.mainPart()]
	if len(rels) > b.Nodes {
		return nil, render.ErrLimit
	}
	b.Nodes -= len(rels)
	styleRel := false
	for _, rel := range rels {
		if rel == nil {
			return nil, render.ErrInvalid
		}
		if rel.Type == opc.RelTypeSettings {
			return nil, fmt.Errorf("%w: document settings relationship", render.ErrUnsupported)
		}
		if rel.Type == opc.RelTypeStyles {
			if styleRel || rel.TargetMode == opc.TargetModeExternal || opc.ResolvePartName(d.mainPart(), rel.Target) != d.stylesPartName() {
				return nil, fmt.Errorf("%w: styles relationship", render.ErrUnsupported)
			}
			styleRel = true
		}
	}
	if d.reader != nil {
		file := d.reader.GetFile(d.mainPart())
		if file == nil {
			return nil, fmt.Errorf("%w: missing main part", render.ErrInvalid)
		}
		stream, e := file.Open()
		if e != nil {
			return nil, e
		}
		e = b.CheckReader(ctx, stream, wordRenderXML)
		closeErr := stream.Close()
		if e != nil {
			return nil, fmt.Errorf("docx: main part: %w", e)
		}
		if closeErr != nil {
			return nil, closeErr
		}
	}
	if part := d.preservedParts[d.stylesPartName()]; part != nil {
		profile := wordRenderStyleProfile()
		if err = b.CheckXML(ctx, part.Data, profile); err != nil {
			return nil, fmt.Errorf("docx: styles: %w", err)
		}
	} else if styleRel && d.styles == nil {
		return nil, fmt.Errorf("%w: missing styles", render.ErrInvalid)
	}
	if err = wordRenderStyles(d.styles, b); err != nil {
		return nil, err
	}
	model := d.doc()
	if model == nil || model.Body == nil {
		return nil, render.ErrInvalid
	}
	if err = wordRenderOnly(model, "XMLName Body OriginalNSDecls OriginalRootAttrs Prolog SelfClosingSpace CollapseEmpty RootExtras"); err != nil {
		return nil, err
	}
	body := model.Body
	if err = wordRenderOnly(body, "P SectPr childOrder"); err != nil {
		return nil, err
	}
	section := body.SectPr
	if section == nil || section.PgSz == nil || section.PgMar == nil {
		return nil, fmt.Errorf("%w: explicit page geometry required", render.ErrUnsupported)
	}
	if err = wordRenderOnly(section, "PgSz PgMar CapturedAttrs RsidR RsidRPr RsidSect childSeq hdrFtrOrder"); err != nil {
		return nil, err
	}
	w, err := wordRenderTwips(section.PgSz.W)
	if err != nil {
		return nil, err
	}
	h, err := wordRenderTwips(section.PgSz.H)
	if err != nil {
		return nil, err
	}
	m := section.PgMar
	left, err := wordRenderTwips(m.Left)
	if err != nil {
		return nil, err
	}
	right, err := wordRenderTwips(m.Right)
	if err != nil {
		return nil, err
	}
	top, err := wordRenderTwips(m.Top)
	if err != nil {
		return nil, err
	}
	bottomMargin, err := wordRenderTwips(m.Bottom)
	if err != nil {
		return nil, err
	}
	if m.Gutter != "" && m.Gutter != "0" {
		return nil, fmt.Errorf("%w: page gutter", render.ErrUnsupported)
	}
	if w <= 0 || h <= 0 || left+right >= w || top+bottomMargin >= h {
		return nil, fmt.Errorf("%w: page geometry", render.ErrInvalid)
	}
	if math.Ceil(w) > float64(limits.MaxDimension) || math.Ceil(h) > float64(limits.MaxDimension) || math.Ceil(w)*math.Ceil(h) > float64(limits.MaxPixels) {
		return nil, render.ErrLimit
	}
	if _, ok := style.FromPx(w); !ok {
		return nil, render.ErrLimit
	}
	if _, ok := style.FromPx(h); !ok {
		return nil, render.ErrLimit
	}
	if len(body.P) > b.Nodes {
		return nil, render.ErrLimit
	}
	b.Nodes -= len(body.P)
	breaker, err := core.NewTextLayout(limits)
	if err != nil {
		return nil, err
	}
	faces := map[render.FontRequest]*shape.Face{}
	ops := []layout.Op{layout.FillRect{Rect: layout.Rect{W: wordRenderUnit(w), H: wordRenderUnit(h)}, Color: style.RGBA{R: 255, G: 255, B: 255, A: 1}}}
	current, y, drawCount := 1, top, 1
	bottom := h - bottomMargin
	newPage := func() error {
		if current >= limits.MaxOperations {
			return render.ErrLimit
		}
		current++
		y = top
		return nil
	}
	for _, p := range body.P {
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		if p == nil {
			return nil, render.ErrInvalid
		}
		if err = wordRenderOnly(p, "PPr R CapturedAttrs CapturedEmptyTag RsidR RsidRPr RsidRDefault RsidP RsidDel ParaId TextId childOrder"); err != nil {
			return nil, err
		}
		pp := p.PPr
		if pp == nil || pp.Spacing == nil || pp.Jc == nil || len(p.R) == 0 {
			return nil, fmt.Errorf("%w: explicit paragraph layout required", render.ErrUnsupported)
		}
		if err = wordRenderOnly(pp, "Spacing Jc WidowControl PageBreakBefore KeepNext KeepLines CapturedChildren CapturedEmptyTag"); err != nil {
			return nil, err
		}
		if (pp.KeepNext != nil && pp.KeepNext.IsOn()) || (pp.KeepLines != nil && pp.KeepLines.IsOn()) {
			return nil, fmt.Errorf("%w: paragraph keep constraints", render.ErrUnsupported)
		}
		if pp.Jc.Val != "left" && pp.Jc.Val != "center" && pp.Jc.Val != "right" {
			return nil, fmt.Errorf("%w: paragraph alignment", render.ErrUnsupported)
		}
		sp := pp.Spacing
		if sp.LineRule != "exact" || sp.Before != "0" || sp.After != "0" || sp.BeforeLines != "" || sp.AfterLines != "" || sp.BeforeAutospacing != "" || sp.AfterAutospacing != "" {
			return nil, fmt.Errorf("%w: exact line spacing and zero paragraph spacing required", render.ErrUnsupported)
		}
		step, err := wordRenderTwips(sp.Line)
		if err != nil {
			return nil, err
		}
		if step <= 0 {
			return nil, render.ErrInvalid
		}
		if len(p.R) > b.Nodes {
			return nil, render.ErrLimit
		}
		b.Nodes -= len(p.R)
		var text strings.Builder
		var first wordRenderRun
		for ri, r := range p.R {
			if r == nil {
				return nil, render.ErrInvalid
			}
			if err = wordRenderOnly(r, "RPr T CapturedAttrs RsidRPr RsidDel RsidR childOrder"); err != nil {
				return nil, err
			}
			properties, e := wordRenderRunStyle(r.RPr)
			if e != nil {
				return nil, e
			}
			if ri == 0 {
				first = properties
			} else if properties != first {
				return nil, fmt.Errorf("%w: mixed paragraph styles", render.ErrUnsupported)
			}
			if len(r.T) > b.Nodes {
				return nil, render.ErrLimit
			}
			b.Nodes -= len(r.T)
			for _, part := range r.T {
				if part == nil {
					return nil, render.ErrInvalid
				}
				if len(part.Text) > limits.MaxRunBytes-text.Len() {
					return nil, render.ErrLimit
				}
				text.WriteString(part.Text)
			}
		}
		face := faces[first.font]
		if face == nil {
			if opts.Fonts == nil {
				return nil, fmt.Errorf("%w: explicit font resolver required", render.ErrUnsupported)
			}
			if len(faces) >= limits.MaxFonts {
				return nil, render.ErrLimit
			}
			face, err = opts.Fonts(ctx, first.font)
			if err != nil {
				return nil, err
			}
			if err = ctx.Err(); err != nil {
				return nil, err
			}
			if face == nil {
				return nil, fmt.Errorf("%w: unresolved font", render.ErrUnsupported)
			}
			faces[first.font] = face
		}
		lines, err := breaker.PlainLinesWithFeatures(ctx, face, text.String(), first.size, wordRenderUnit(w-left-right), shape.Features{TagsOff: "liga,clig,kern,calt"})
		if err != nil {
			return nil, err
		}
		metrics := lines[0].Face.Descriptor()
		em := float64(lines[0].Face.UnitsPerEm())
		ascent := float64(metrics.Ascent) * first.size.Px() / em
		descent := -float64(metrics.Descent) * first.size.Px() / em
		if ascent <= 0 || descent < 0 || step < ascent+descent || step > bottom-top {
			return nil, fmt.Errorf("%w: line metrics exceed line/page height", render.ErrUnsupported)
		}
		widow := pp.WidowControl == nil || pp.WidowControl.IsOn()
		if pp.PageBreakBefore != nil && pp.PageBreakBefore.IsOn() && y > top {
			if err = newPage(); err != nil {
				return nil, err
			}
		}
		for from := 0; from < len(lines); {
			if err = ctx.Err(); err != nil {
				return nil, err
			}
			room := int(math.Floor((bottom-y)/step + 1e-9))
			remaining := len(lines) - from
			take := min(room, remaining)
			if widow && remaining > 1 && take < remaining {
				if take < 2 {
					take = 0
				} else if remaining-take == 1 {
					take--
				}
				if take < 2 {
					take = 0
				}
			}
			if take == 0 {
				if y == top {
					return nil, fmt.Errorf("%w: page too small for paragraph constraints", render.ErrUnsupported)
				}
				if err = newPage(); err != nil {
					return nil, err
				}
				continue
			}
			for _, line := range lines[from : from+take] {
				if drawCount >= limits.MaxOperations {
					return nil, render.ErrLimit
				}
				drawCount++
				if current == page {
					x := left
					if pp.Jc.Val == "center" {
						x += (w - left - right - line.Width.Px()) / 2
					}
					if pp.Jc.Val == "right" {
						x = w - right - line.Width.Px()
					}
					ops = append(ops, layout.DrawGlyphs{At: layout.Point{X: wordRenderUnit(x), Y: wordRenderUnit(y + (step-ascent-descent)/2 + ascent)}, Text: line.Text, Glyphs: line.Glyphs, Face: line.Face, Size: first.size, Color: first.color})
				}
				y += step
			}
			from += take
			if from < len(lines) {
				if err = newPage(); err != nil {
					return nil, err
				}
			}
		}
	}
	if page > current {
		return nil, fmt.Errorf("%w: %w: page %d beyond %d pages", render.ErrInvalid, ErrRenderPageOutOfRange, page, current)
	}
	return render.Prepare(ctx, dml.EMU(math.Round(w*float64(dml.EMUsPerPixel))), dml.EMU(math.Round(h*float64(dml.EMUsPerPixel))), ops, limits)
}

func wordRenderUnit(px float64) style.Unit { u, _ := style.FromPx(px); return u }
func wordRenderTwips(s string) (float64, error) {
	n, e := strconv.ParseInt(s, 10, 32)
	if e != nil || n < 0 {
		return 0, fmt.Errorf("%w: twip length", render.ErrInvalid)
	}
	return float64(n) / 15, nil
}

type wordRenderRun struct {
	font  render.FontRequest
	size  style.Unit
	color style.RGBA
}

func wordRenderRunStyle(r *oxml.CT_RPr) (wordRenderRun, error) {
	bad := func() (wordRenderRun, error) {
		return wordRenderRun{}, fmt.Errorf("%w: explicit plain run style required", render.ErrUnsupported)
	}
	if r == nil || r.RFonts == nil || r.Sz == nil || r.Color == nil || r.B == nil || r.I == nil || r.Strike == nil {
		return bad()
	}
	if err := wordRenderOnly(r, "RFonts Sz Color B I U Strike CapturedChildren CapturedEmptyTag"); err != nil {
		return wordRenderRun{}, err
	}
	if r.Strike.IsOn() || (r.U != nil && r.U.Val != "none") {
		return bad()
	}
	f := r.RFonts
	if len(f.Ascii) == 0 || len(f.Ascii) > 1024 || f.AsciiTheme != "" || f.HAnsiTheme != "" || f.EastAsiaTheme != "" || f.CsTheme != "" || f.Hint != "" || (f.HAnsi != "" && f.HAnsi != f.Ascii) || (f.EastAsia != "" && f.EastAsia != f.Ascii) || (f.Cs != "" && f.Cs != f.Ascii) {
		return bad()
	}
	if r.Color.ThemeColor != "" || r.Color.ThemeTint != "" || r.Color.ThemeShade != "" || len(r.Color.Val) != 6 {
		return bad()
	}
	rgb, err := dml.ParseRGB(r.Color.Val)
	if err != nil {
		return wordRenderRun{}, render.ErrInvalid
	}
	half, err := strconv.ParseUint(r.Sz.Val, 10, 16)
	if err != nil || half == 0 || half > 8192 {
		return bad()
	}
	size, _ := style.FromPx(float64(half) * 2 / 3)
	return wordRenderRun{font: render.FontRequest{Family: f.Ascii, Bold: r.B.IsOn(), Italic: r.I.IsOn()}, size: size, color: style.RGBA{R: float64(rgb.R), G: float64(rgb.G), B: float64(rgb.B), A: 1}}, nil
}

// Only fixed schema structs are inspected. Captured source content has already
// passed the XML profile; known mutable values are checked independently here.
func wordRenderOnly(value any, allowed string) error {
	allowed = " " + allowed + " "
	v := reflect.ValueOf(value)
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return nil
		}
		v = v.Elem()
	}
	for i := 0; i < v.NumField(); i++ {
		name := v.Type().Field(i).Name
		if !strings.Contains(allowed, " "+name+" ") && !v.Field(i).IsZero() {
			return fmt.Errorf("%w: %s.%s", render.ErrUnsupported, v.Type().Name(), name)
		}
	}
	return nil
}

func wordRenderStyles(styles *oxml.CT_Styles, b *core.SourceBudget) error {
	if styles == nil {
		return nil
	}
	if len(styles.Style) > b.Nodes {
		return render.ErrLimit
	}
	b.Nodes -= len(styles.Style)
	if def := styles.DocDefaults; def != nil {
		if def.PPrDefault != nil {
			if err := wordRenderOnly(def.PPrDefault.PPr, "Spacing CapturedChildren CapturedEmptyTag"); err != nil {
				return err
			}
		}
		if def.RPrDefault != nil {
			if err := wordRenderOnly(def.RPrDefault.RPr, "RFonts Sz SzCs CapturedChildren CapturedEmptyTag"); err != nil {
				return err
			}
		}
	}
	for _, s := range styles.Style {
		if s == nil {
			return render.ErrInvalid
		}
		if s.Default == "1" || s.Default == "true" || s.Default == "on" || s.StyleId == "Normal" || s.StyleId == "DefaultParagraphFont" {
			if s.BasedOn != nil || s.Link != nil {
				return fmt.Errorf("%w: inherited default style", render.ErrUnsupported)
			}
			if s.PPr != nil {
				return fmt.Errorf("%w: default paragraph style properties", render.ErrUnsupported)
			}
			if s.RPr != nil {
				return fmt.Errorf("%w: default run style properties", render.ErrUnsupported)
			}
		}
	}
	return nil
}
