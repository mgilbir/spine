package pptxrender

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image/color"
	"image/png"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/mgilbir/forme/shape"
	"github.com/mgilbir/spine/pptx/internal/oxml"
	"github.com/mgilbir/spine/render"
)

const styleAllOptions = ` firstRow="1" lastRow="1" firstCol="1" lastCol="1" bandRow="1"`

var styleTableXML = regexp.MustCompile(`(?s)<a:tbl>.*?</a:tbl>`)

// styleTestTable is the 3 by 4 table of the reference decks, 0.95in columns
// and 0.25in rows: 91.2 by 24 px at 96 dpi. text gives a cell's paragraph
// content, empty for an empty cell.
func styleTestTable(style, flags string, text func(r, c int) string) string {
	var b strings.Builder
	b.WriteString(`<a:tbl><a:tblPr` + flags + `>` + style + `</a:tblPr><a:tblGrid><a:gridCol w="868680"/><a:gridCol w="868680"/><a:gridCol w="868680"/></a:tblGrid>`)
	for r := 0; r < 4; r++ {
		b.WriteString(`<a:tr h="228600">`)
		for c := 0; c < 3; c++ {
			p := `<a:endParaRPr lang="en-US"/>`
			if text != nil && text(r, c) != "" {
				p = text(r, c)
			}
			b.WriteString(`<a:tc><a:txBody><a:bodyPr/><a:lstStyle/><a:p>` + p + `</a:p></a:txBody><a:tcPr/></a:tc>`)
		}
		b.WriteString(`</a:tr>`)
	}
	b.WriteString(`</a:tbl>`)
	return b.String()
}

func styleID(id string) string { return `<a:tableStyleId>` + id + `</a:tableStyleId>` }

// styleTestRender draws a table at (10,10) px in a fresh deck. warn, when set,
// makes the render best effort. rewrites change other parts.
func styleTestRender(t *testing.T, table string, warn func(error), rewrites map[string]func(string) string, mods ...func(*render.Options)) ([]byte, error) {
	t.Helper()
	data, opts := renderTableDeck(t, nil)
	opts.Warn = warn
	for _, mod := range mods {
		mod(&opts)
	}
	all := map[string]func(string) string{"ppt/slides/slide1.xml": func(s string) string { return styleTableXML.ReplaceAllLiteralString(s, table) }}
	for part, f := range rewrites {
		all[part] = f
	}
	return renderRewrittenPNG(t, data, opts, all)
}

// styleCellPixel is a point at the middle of a cell's right third.
func styleCellPixel(r, c int) (int, int) {
	return 10 + int((float64(c)+0.85)*91.2), 10 + r*24 + 12
}

func hexColor(t *testing.T, s string) color.NRGBA {
	t.Helper()
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil || len(s) != 6 {
		t.Fatalf("color %q", s)
	}
	return color.NRGBA{R: uint8(v >> 16), G: uint8(v >> 8), B: uint8(v), A: 255}
}

func closeTo(a, b color.NRGBA, tol int) bool {
	d := func(x, y uint8) int {
		if x > y {
			return int(x - y)
		}
		return int(y - x)
	}
	return d(a.R, b.R) <= tol && d(a.G, b.G) <= tol && d(a.B, b.B) <= tol
}

// themed2Accent reports the styles drawn with a blurred shadow, which is only
// approximated.
func themed2Accent(id string) bool {
	b := builtinTableStyleIDs[id]
	return b.family == themed2 && b.accent > 0
}

func sortedStyleIDs() []string {
	ids := make([]string, 0, len(builtinTableStyleIDs))
	for id := range builtinTableStyleIDs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}

func TestBuiltinTableStyleCatalog(t *testing.T) {
	if len(builtinTableStyleIDs) != 74 || len(builtinStyleFills) != 74 {
		t.Fatalf("%d styles, %d measured", len(builtinTableStyleIDs), len(builtinStyleFills))
	}
	names := map[string]string{}
	for _, id := range sortedStyleIDs() {
		if _, ok := builtinStyleFills[id]; !ok {
			t.Errorf("%s: not measured", id)
		}
		ts := builtinTableStyle(id)
		if ts == nil || ts.StyleId != id || ts.WholeTbl == nil || ts.StyleName == "" {
			t.Fatalf("%s: %+v", id, ts)
		}
		if other, dup := names[ts.StyleName]; dup {
			t.Errorf("%s and %s are both %q", id, other, ts.StyleName)
		}
		names[ts.StyleName] = id
		if again := builtinTableStyle(id); again == ts {
			t.Errorf("%s: the same style handed out twice", id)
		}
	}
	// The accents of a family run from accent 1 to 6, except Dark Style 2,
	// which Office offers for accents 1, 3 and 5.
	for _, name := range []string{"Themed Style 1", "Light Style 3 - Accent 6", "Medium Style 2 - Accent 1", "Dark Style 2 - Accent 5", "Dark Style 2"} {
		if _, ok := names[name]; !ok {
			t.Errorf("no style %q", name)
		}
	}
	if _, ok := names["Dark Style 2 - Accent 2"]; ok {
		t.Error("Dark Style 2 has no accent 2")
	}
	for _, unknown := range []string{"", "{00000000-0000-0000-0000-000000000000}"} {
		if builtinTableStyle(unknown) != nil {
			t.Errorf("%q is not a built-in style id", unknown)
		}
	}
}

// Every built-in style draws what PowerPoint draws: each cell's fill matches
// the sample of PowerPoint's own export, within a few levels (the table's
// theme gradients and translucent fills are composed a little differently).
func TestBuiltinTableStylesMatchPowerPoint(t *testing.T) {
	for _, id := range sortedStyleIDs() {
		var warnings []error
		got, err := styleTestRender(t, styleTestTable(styleID(id), styleAllOptions, nil), func(err error) { warnings = append(warnings, err) }, nil)
		if err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		name := builtinTableStyle(id).StyleName
		want := builtinStyleFills[id]
		for r := 0; r < 4; r++ {
			for c := 0; c < 3; c++ {
				x, y := styleCellPixel(r, c)
				if px, w := renderPixel(t, got, x, y), hexColor(t, want[r*3+c]); !closeTo(px, w, 6) {
					t.Errorf("%s (%s) cell (%d,%d): %+v, PowerPoint draws %+v", name, id, r, c, px, w)
				}
			}
		}
		// Only the blurred shadow of the themed grid styles is approximate.
		if themed2Accent(id) != (len(warnings) > 0) {
			t.Errorf("%s: warnings %v", name, warnings)
		}
		for _, w := range warnings {
			if !errors.Is(w, render.ErrApproximated) || !strings.Contains(w.Error(), "shadow") {
				t.Errorf("%s: %v", name, w)
			}
		}
	}
}

// A strict render draws exactly or fails: every built-in style is drawn
// exactly except those with a blurred shadow.
func TestBuiltinTableStylesStrict(t *testing.T) {
	for _, id := range sortedStyleIDs() {
		_, err := styleTestRender(t, styleTestTable(styleID(id), styleAllOptions, nil), nil, nil)
		if themed2Accent(id) {
			if !errors.Is(err, render.ErrUnsupported) {
				t.Errorf("%s: %v", builtinTableStyle(id).StyleName, err)
			}
		} else if err != nil {
			t.Errorf("%s: %v", builtinTableStyle(id).StyleName, err)
		}
	}
}

func TestBuiltinTableStyleOptions(t *testing.T) {
	const medium2 = "{5C22544A-7EE6-4342-B048-85BDC9FD1C3A}" // Medium Style 2 - Accent 1
	accent, light, mid := hexColor(t, "156082"), hexColor(t, "E7EAED"), hexColor(t, "CCD2D8")
	for _, tc := range []struct {
		name  string
		flags string
		want  [4][3]color.NRGBA // the fill of every cell
	}{
		// With no option on, only the whole table's part applies.
		{"none", ``, [4][3]color.NRGBA{{light, light, light}, {light, light, light}, {light, light, light}, {light, light, light}}},
		// Header and total rows are not banded; the rows between start with
		// the darker band.
		{"banded rows", ` bandRow="1"`, [4][3]color.NRGBA{{mid, mid, mid}, {light, light, light}, {mid, mid, mid}, {light, light, light}}},
		{"header", ` firstRow="1" bandRow="1"`, [4][3]color.NRGBA{{accent, accent, accent}, {mid, mid, mid}, {light, light, light}, {mid, mid, mid}}},
		{"total", ` lastRow="1" bandRow="1"`, [4][3]color.NRGBA{{mid, mid, mid}, {light, light, light}, {mid, mid, mid}, {accent, accent, accent}}},
		{"first column", ` firstCol="1"`, [4][3]color.NRGBA{{accent, light, light}, {accent, light, light}, {accent, light, light}, {accent, light, light}}},
		{"last column", ` lastCol="1"`, [4][3]color.NRGBA{{light, light, accent}, {light, light, accent}, {light, light, accent}, {light, light, accent}}},
		// The header outranks the first column where they meet.
		{"header and first column", ` firstRow="1" firstCol="1"`, [4][3]color.NRGBA{{accent, accent, accent}, {accent, light, light}, {accent, light, light}, {accent, light, light}}},
	} {
		got, err := styleTestRender(t, styleTestTable(styleID(medium2), tc.flags, nil), nil, nil)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		for r := 0; r < 4; r++ {
			for c := 0; c < 3; c++ {
				x, y := styleCellPixel(r, c)
				if px := renderPixel(t, got, x, y); !closeTo(px, tc.want[r][c], 2) {
					t.Errorf("%s cell (%d,%d): %+v, want %+v", tc.name, r, c, px, tc.want[r][c])
				}
			}
		}
	}
	// Banded columns were not compared with PowerPoint, so a strict render
	// refuses them, and best effort draws them like banded rows.
	table := styleTestTable(styleID(medium2), ` bandCol="1"`, nil)
	if _, err := styleTestRender(t, table, nil, nil); !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("banded columns: %v", err)
	}
	var warnings []error
	got, err := styleTestRender(t, table, func(err error) { warnings = append(warnings, err) }, nil)
	if err != nil || len(warnings) != 1 || !errors.Is(warnings[0], render.ErrApproximated) || !strings.Contains(warnings[0].Error(), "banded columns") {
		t.Fatalf("banded columns, best effort: %v %v", err, warnings)
	}
	x, y := styleCellPixel(0, 0)
	if px := renderPixel(t, got, x, y); !closeTo(px, mid, 2) {
		t.Fatalf("banded column: %+v", px)
	}
	x, y = styleCellPixel(0, 1)
	if px := renderPixel(t, got, x, y); !closeTo(px, light, 2) {
		t.Fatalf("second column: %+v", px)
	}
}

// deckStyles rewrites a deck's ppt/tableStyles.xml to define one style.
func deckStyles(id string) map[string]func(string) string {
	return map[string]func(string) string{"ppt/tableStyles.xml": func(s string) string {
		return regexp.MustCompile(`(<a:tblStyleLst[^>]*?)/>`).ReplaceAllString(s, `$1><a:tblStyle styleId="`+id+`" styleName="Deck"/></a:tblStyleLst>`)
	}}
}

func TestTableStyleNotBuiltIn(t *testing.T) {
	white := color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	for name, tc := range map[string]struct {
		style    string
		rewrites map[string]func(string) string
	}{
		"unknown id": {style: styleID("{00000000-0000-0000-0000-000000000000}")},
		// The deck's own definition of an id, which is not read, takes
		// precedence over a built-in style of the same id.
		"defined by the deck": {
			style:    styleID("{5C22544A-7EE6-4342-B048-85BDC9FD1C3A}"),
			rewrites: deckStyles("{5c22544a-7ee6-4342-b048-85bdc9fd1c3a}"),
		},
	} {
		table := styleTestTable(tc.style, styleAllOptions, nil)
		if _, err := styleTestRender(t, table, nil, tc.rewrites); !errors.Is(err, render.ErrUnsupported) {
			t.Fatalf("%s, strict: %v", name, err)
		}
		var warnings []error
		got, err := styleTestRender(t, table, func(err error) { warnings = append(warnings, err) }, tc.rewrites)
		if err != nil || len(warnings) != 1 || !errors.Is(warnings[0], render.ErrApproximated) || !strings.Contains(warnings[0].Error(), "table style left out") {
			t.Fatalf("%s, best effort: %v %v", name, err, warnings)
		}
		// Drawn without a style: no fill anywhere.
		x, y := styleCellPixel(0, 0)
		if px := renderPixel(t, got, x, y); px != white {
			t.Fatalf("%s: header %+v", name, px)
		}
	}
	// A deck that defines other styles still gets the built-in one.
	got, err := styleTestRender(t, styleTestTable(styleID("{5C22544A-7EE6-4342-B048-85BDC9FD1C3A}"), styleAllOptions, nil), nil, deckStyles("{11111111-2222-3333-4444-555555555555}"))
	if err != nil {
		t.Fatal(err)
	}
	x, y := styleCellPixel(0, 0)
	if px := renderPixel(t, got, x, y); !closeTo(px, hexColor(t, "156082"), 2) {
		t.Fatalf("with another style in the deck: %+v", px)
	}
}

// A cell's own properties outrank the style's.
func TestTableStyleCellOverrides(t *testing.T) {
	const medium2 = "{5C22544A-7EE6-4342-B048-85BDC9FD1C3A}"
	table := styleTestTable(styleID(medium2), styleAllOptions, nil)
	own := func(s string) string {
		// Cell (1,1): a red fill; cell (1,2): no fill; cell (2,1): a black
		// 4px border on its left, over the style's white line.
		cells := regexp.MustCompile(`<a:tcPr/>`)
		n := 0
		return cells.ReplaceAllStringFunc(s, func(string) string {
			n++
			switch n {
			case 5: // (1,1)
				return `<a:tcPr><a:solidFill><a:srgbClr val="FF0000"/></a:solidFill></a:tcPr>`
			case 6: // (1,2)
				return `<a:tcPr><a:noFill/></a:tcPr>`
			case 8: // (2,1)
				return `<a:tcPr><a:lnL w="38100"><a:solidFill><a:srgbClr val="000000"/></a:solidFill></a:lnL></a:tcPr>`
			}
			return `<a:tcPr/>`
		})
	}
	// Which of a cell's own border and a style's shows where they meet is not
	// known, so a strict render refuses, and best effort says so.
	if _, err := styleTestRender(t, own(table), nil, nil); !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("strict: %v", err)
	}
	var warnings []error
	got, err := styleTestRender(t, own(table), func(err error) { warnings = append(warnings, err) }, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(warnings) != 1 || !errors.Is(warnings[0], render.ErrApproximated) || !strings.Contains(warnings[0].Error(), "own border meets a table style") {
		t.Fatalf("warnings: %v", warnings)
	}
	red, white := hexColor(t, "FF0000"), color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	for name, tc := range map[string]struct {
		at   [2]int
		want color.NRGBA
	}{
		"own fill":   {[2]int{1, 1}, red},
		"no fill":    {[2]int{1, 2}, white},
		"style fill": {[2]int{2, 1}, hexColor(t, "E7EAED")},
	} {
		x, y := styleCellPixel(tc.at[0], tc.at[1])
		if px := renderPixel(t, got, x, y); !closeTo(px, tc.want, 2) {
			t.Errorf("%s: %+v, want %+v", name, px, tc.want)
		}
	}
	// The border sits on the grid line x = 10 + 91.2, between rows 2 and 3.
	if px := renderPixel(t, got, 101, 10+24*2+12); !closeTo(px, color.NRGBA{A: 255}, 40) {
		t.Errorf("own border: %+v", px)
	}
}

// A cell border with a width but no fill may take the style's color, which is
// not known: a strict render refuses it, and best effort draws no border.
func TestTableStyleBorderWithoutFill(t *testing.T) {
	table := strings.Replace(styleTestTable(styleID("{5C22544A-7EE6-4342-B048-85BDC9FD1C3A}"), styleAllOptions, nil), `<a:tcPr/>`, `<a:tcPr><a:lnL w="38100"/></a:tcPr>`, 1)
	if _, err := styleTestRender(t, table, nil, nil); !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("strict: %v", err)
	}
	var warnings []error
	if _, err := styleTestRender(t, table, func(err error) { warnings = append(warnings, err) }, nil); err != nil || len(warnings) != 1 || !errors.Is(warnings[0], render.ErrApproximated) {
		t.Fatalf("best effort: %v %v", err, warnings)
	}
}

// A style's text properties apply under the text's own: the header and the
// first column are bold and light, the body regular and dark.
func TestTableStyleText(t *testing.T) {
	const medium2 = "{5C22544A-7EE6-4342-B048-85BDC9FD1C3A}"
	// One 30pt letter in a 2/3in cell, so the glyph has a solid core; the others
	// are empty. It returns the picture and the weights of the fonts asked for.
	letter := func(r, c int, attrs, fill string) ([]byte, map[bool]bool) {
		t.Helper()
		table := styleTestTable(styleID(medium2), styleAllOptions, func(rr, cc int) string {
			if rr == r && cc == c {
				return `<a:r><a:rPr lang="en-US" sz="3000"` + attrs + `>` + fill + `</a:rPr><a:t>I</a:t></a:r>`
			}
			return ""
		})
		table = strings.ReplaceAll(strings.ReplaceAll(table, `w="868680"`, `w="609600"`), `<a:tr h="228600">`, `<a:tr h="609600">`)
		weights := map[bool]bool{}
		got, err := styleTestRender(t, table, nil, nil, func(o *render.Options) {
			inner := o.Fonts
			o.Fonts = func(ctx context.Context, req render.FontRequest) (*shape.Face, error) {
				weights[req.Bold] = true
				return inner(ctx, req)
			}
		})
		if err != nil {
			t.Fatal(err)
		}
		return got, weights
	}
	count := func(img []byte, r, c int, want color.NRGBA) int {
		decoded, err := png.Decode(bytes.NewReader(img))
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		for y := 10 + r*64 + 6; y < 10+(r+1)*64-6; y++ {
			for x := 10 + c*64 + 6; x < 10+(c+1)*64-6; x++ {
				if color.NRGBAModel.Convert(decoded.At(x, y)).(color.NRGBA) == want {
					n++
				}
			}
		}
		return n
	}
	white, black, red := hexColor(t, "FFFFFF"), hexColor(t, "000000"), hexColor(t, "FF0000")
	for _, tc := range []struct {
		name         string
		r, c         int
		attrs, fill  string
		ink          color.NRGBA
		bold         bool
		absent       color.NRGBA // a color that must not show
		absentWanted bool
	}{
		{"header", 0, 1, "", "", white, true, black, true},
		{"first column", 1, 0, "", "", white, true, black, true},
		{"last column", 2, 2, "", "", white, true, black, true},
		{"total", 3, 1, "", "", white, true, black, true},
		{"body", 1, 1, "", "", black, false, white, true},
		// The cell's own run properties outrank the style's.
		{"own color", 0, 1, "", `<a:solidFill><a:srgbClr val="FF0000"/></a:solidFill>`, red, true, white, true},
		{"own weight", 0, 1, ` b="0"`, "", white, false, black, true},
	} {
		got, weights := letter(tc.r, tc.c, tc.attrs, tc.fill)
		if n := count(got, tc.r, tc.c, tc.ink); n == 0 {
			t.Errorf("%s: no pixel of the text color %+v", tc.name, tc.ink)
		}
		if tc.absentWanted && count(got, tc.r, tc.c, tc.absent) != 0 {
			t.Errorf("%s: pixels of %+v, which the style does not give the text", tc.name, tc.absent)
		}
		if weights[tc.bold] != true || weights[!tc.bold] {
			t.Errorf("%s: fonts asked for by bold: %v, want only bold=%v", tc.name, weights, tc.bold)
		}
	}
}

// styleEdges describes the border a style gives one side of a cell.
func styleEdges(t *testing.T, id string, flags oxml.ATblPr, rows, cols, r, c, side int) string {
	t.Helper()
	ts := builtinTableStyle(id)
	if ts == nil {
		t.Fatalf("no style %s", id)
	}
	ln, rank := newTableStyler(ts, &flags, rows, cols).edge(r, c, side)
	switch {
	case ln == nil:
		return "unset"
	case ln.SolidFill == nil:
		return "none"
	}
	cmpd := ""
	if ln.Cmpd == "dbl" {
		cmpd = " double"
	}
	return fmt.Sprintf("%d %s%s (rank %d)", *ln.W/3175, ln.SolidFill.SchemeClr.Val, cmpd, rank)
}

// The borders of the families, in 288 dpi pixels (3175 EMU each) as PowerPoint
// draws them in its export.
func TestTableStyleBorders(t *testing.T) {
	all := oxml.ATblPr{FirstRow: true, LastRow: true, FirstCol: true, LastCol: true, BandRow: true}
	const (
		themed1Acc1 = "{3C2FFA5D-87B4-456A-9821-1D502468CF0F}"
		themed2Acc1 = "{D113A9D2-9D6B-4929-AA2D-F23B5EE8CBE7}"
		themed2     = "{5940675A-B579-460E-94D1-54222C63F5DA}"
		light1      = "{9D7B26C5-4107-4FEC-AEDC-1716B250A1EF}"
		light2Acc1  = "{69012ECD-51FC-41F1-AA8D-1B2483CD663E}"
		light3Acc2  = "{5DA37D80-6434-44D0-A028-1B22A696006F}"
		medium1Acc1 = "{B301B821-A1FF-4177-AEE7-76D212191A09}"
		medium2Acc1 = "{5C22544A-7EE6-4342-B048-85BDC9FD1C3A}"
		medium3Acc1 = "{6E25E649-3F16-4E02-A733-19D2CDBF48F0}"
		medium4Acc1 = "{69CF1AB2-1976-4502-BF36-3FF5EA218861}"
		dark1       = "{E8034E78-7F5D-4C2E-B375-FC64B27BC917}"
		dark2Acc3   = "{91EBBBCC-DAD2-459C-BE2E-F6DE35CF9A28}"
	)
	for _, tc := range []struct {
		name    string
		id      string
		r, c    int
		side    int
		want    string
		options *oxml.ATblPr
	}{
		// The rank of a part counts for nothing but order: lower to higher
		// is whole table, bands, last column, first column, total row,
		// header row.
		{"themed 1: inside lines", themed1Acc1, 1, 1, sideBottom, "4 accent1 (rank 0)", nil},
		{"themed 1: header line", themed1Acc1, 0, 1, sideBottom, "6 lt1 (rank 10)", nil},
		{"themed 1: total line", themed1Acc1, 3, 1, sideTop, "6 accent1 (rank 7)", nil},
		{"themed 1: total row has no inside verticals", themed1Acc1, 3, 1, sideLeft, "none", nil},
		{"themed 1: first column's line", themed1Acc1, 1, 0, sideRight, "6 accent1 (rank 6)", nil},
		{"themed 2: grid", themed2, 2, 1, sideTop, "4 dk1 (rank 0)", nil},
		{"themed 2: edge tint", themed2Acc1, 0, 0, sideTop, "4 accent1 (rank 0)", nil},
		{"themed 2: no line under the last column's header", themed2Acc1, 0, 2, sideBottom, "none", nil},
		{"themed 2: no total line under the first column", themed2Acc1, 3, 0, sideTop, "none", nil},
		{"themed 2: total line under the middle", themed2Acc1, 3, 1, sideTop, "6 lt1 (rank 7)", nil},
		{"light 1: top", light1, 0, 1, sideTop, "4 dk1 (rank 0)", nil},
		{"light 1: no inside lines", light1, 2, 1, sideBottom, "none", nil},
		{"light 2: band", light2Acc1, 1, 1, sideBottom, "4 accent1 (rank 1)", nil},
		{"light 2: total", light2Acc1, 3, 1, sideTop, "16 accent1 double (rank 7)", nil},
		{"light 3: grid", light3Acc2, 2, 1, sideRight, "4 accent2 (rank 0)", nil},
		{"light 3: header", light3Acc2, 0, 1, sideBottom, "8 accent2 (rank 10)", nil},
		{"medium 1: no inside verticals", medium1Acc1, 2, 1, sideLeft, "none", nil},
		{"medium 1: inside horizontals", medium1Acc1, 2, 1, sideBottom, "4 accent1 (rank 0)", nil},
		{"medium 2: white grid", medium2Acc1, 2, 1, sideRight, "4 lt1 (rank 0)", nil},
		{"medium 2: header", medium2Acc1, 0, 1, sideBottom, "12 lt1 (rank 10)", nil},
		{"medium 2: total", medium2Acc1, 3, 1, sideTop, "12 lt1 (rank 7)", nil},
		{"medium 3: header", medium3Acc1, 0, 1, sideBottom, "8 dk1 (rank 10)", nil},
		{"medium 3: top and bottom", medium3Acc1, 3, 1, sideBottom, "8 dk1 (rank 0)", nil},
		{"medium 3: no sides", medium3Acc1, 2, 1, sideLeft, "unset", nil},
		{"medium 4: total", medium4Acc1, 3, 1, sideTop, "8 accent1 (rank 7)", nil},
		{"dark 1: header", dark1, 0, 1, sideBottom, "8 lt1 (rank 10)", nil},
		{"dark 1: no verticals in the header", dark1, 0, 1, sideLeft, "none", nil},
		{"dark 1: first column", dark1, 1, 0, sideRight, "8 lt1 (rank 6)", nil},
		{"dark 2: total", dark2Acc3, 3, 1, sideTop, "16 dk1 double (rank 7)", nil},
		{"dark 2: no grid", dark2Acc3, 2, 1, sideLeft, "unset", nil},
		// With the header off, the first row is body: banded, and with no
		// line under it.
		{"no header option", light2Acc1, 0, 1, sideBottom, "4 accent1 (rank 1)", &oxml.ATblPr{BandRow: true}},
		{"no total option", light2Acc1, 2, 1, sideBottom, "4 accent1 (rank 1)", &oxml.ATblPr{BandRow: true}},
	} {
		opts := all
		if tc.options != nil {
			opts = *tc.options
		}
		if got := styleEdges(t, tc.id, opts, 4, 3, tc.r, tc.c, tc.side); got != tc.want {
			t.Errorf("%s: %s, want %s", tc.name, got, tc.want)
		}
	}
}

// Text properties by region, as PowerPoint draws them.
func TestTableStyleTextRegions(t *testing.T) {
	all := oxml.ATblPr{FirstRow: true, LastRow: true, FirstCol: true, LastCol: true, BandRow: true}
	text := func(id string, r, c int, opts oxml.ATblPr) string {
		s := newTableStyler(builtinTableStyle(id), &opts, 4, 3).text(r, c)
		out := "regular"
		if s.bold != nil && *s.bold {
			out = "bold"
		}
		if s.color != nil {
			out += " " + s.color.SchemeClr.Val
		}
		return out
	}
	for _, tc := range []struct {
		id      string
		r, c    int
		options *oxml.ATblPr
		want    string
	}{
		// No style, no grid: plain text.
		{"{2D5ABB26-0587-4C30-8999-92F81FD0307C}", 0, 0, nil, "regular dk1"},
		{"{3C2FFA5D-87B4-456A-9821-1D502468CF0F}", 0, 1, nil, "bold lt1"},
		{"{3C2FFA5D-87B4-456A-9821-1D502468CF0F}", 1, 1, nil, "regular dk1"},
		{"{3C2FFA5D-87B4-456A-9821-1D502468CF0F}", 2, 0, nil, "bold dk1"},
		{"{D113A9D2-9D6B-4929-AA2D-F23B5EE8CBE7}", 1, 1, nil, "regular lt1"},
		{"{5C22544A-7EE6-4342-B048-85BDC9FD1C3A}", 1, 0, nil, "bold lt1"},
		{"{5C22544A-7EE6-4342-B048-85BDC9FD1C3A}", 1, 1, nil, "regular dk1"},
		{"{5C22544A-7EE6-4342-B048-85BDC9FD1C3A}", 1, 0, &oxml.ATblPr{FirstRow: true}, "regular dk1"},
		// The total row of Medium Style 3 is white with dark text, although its
		// first and last cells would be light.
		{"{6E25E649-3F16-4E02-A733-19D2CDBF48F0}", 3, 0, nil, "bold dk1"},
		{"{6E25E649-3F16-4E02-A733-19D2CDBF48F0}", 2, 0, nil, "bold lt1"},
		// The dark styles' text is light, on whatever their fills are.
		{"{E8034E78-7F5D-4C2E-B375-FC64B27BC917}", 1, 1, nil, "regular lt1"},
		{"{5202B0CA-FC54-4496-8BCA-5EF66A818D29}", 1, 1, nil, "regular dk1"},
		{"{5202B0CA-FC54-4496-8BCA-5EF66A818D29}", 0, 1, nil, "bold lt1"},
		// Light styles bold their header but keep its color.
		{"{9D7B26C5-4107-4FEC-AEDC-1716B250A1EF}", 0, 1, nil, "bold dk1"},
		{"{69012ECD-51FC-41F1-AA8D-1B2483CD663E}", 0, 1, nil, "bold lt1"},
	} {
		opts := all
		if tc.options != nil {
			opts = *tc.options
		}
		if got := text(tc.id, tc.r, tc.c, opts); got != tc.want {
			t.Errorf("%s (%d,%d): %s, want %s", builtinTableStyle(tc.id).StyleName, tc.r, tc.c, got, tc.want)
		}
	}
}

// The background a style takes from the theme: Themed Style 1 and 2 are the
// ones with a gradient, and 2's has a shadow.
func TestBuiltinTableStyleBackgrounds(t *testing.T) {
	for _, id := range sortedStyleIDs() {
		ts := builtinTableStyle(id)
		b := builtinTableStyleIDs[id]
		withBg := (b.family == themed1 || b.family == themed2) && b.accent > 0
		if (ts.TblBg != nil) != withBg {
			t.Errorf("%s: background %+v", ts.StyleName, ts.TblBg)
		}
		if ts.TblBg == nil {
			continue
		}
		// Themed Style 1 takes the theme's second fill and its first effect,
		// which is none; Themed Style 2 its third fill and effect.
		wantFill, wantEffect := uint32(2), uint32(1)
		if b.family == themed2 {
			wantFill, wantEffect = 3, 3
		}
		if ts.TblBg.FillRef == nil || ts.TblBg.FillRef.Idx != wantFill || ts.TblBg.EffectRef == nil || ts.TblBg.EffectRef.Idx != wantEffect {
			t.Errorf("%s: %+v", ts.StyleName, ts.TblBg)
		}
	}
}

// Compound lines are two strokes with a gap, each a third of the width; a
// style's line outranks a lower one where they cross, and a cell's own outranks
// them all.
func TestBuiltinTableStyleDoubleLine(t *testing.T) {
	const medium1 = "{793D81CF-94F2-401A-BA57-92F5A7B2D0C5}" // Medium Style 1: the total row's top
	// Rows of 48 px, so the 4pt line is 5.33 px wide at 96 dpi.
	tall := func(s string) string {
		return strings.ReplaceAll(s, `<a:tr h="228600">`, `<a:tr h="457200">`)
	}
	got, err := styleTestRender(t, tall(styleTestTable(styleID(medium1), styleAllOptions, nil)), nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	// The line is centered on y = 10 + 3*48 = 154: strokes at 151.3-153.1 and
	// 154.9-156.7, a gap between. Rows 2 and 3 are white.
	x := 10 + 50
	black, white := color.NRGBA{A: 255}, color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	for y, want := range map[int]color.NRGBA{149: white, 152: black, 154: white, 155: black, 158: white} {
		if px := renderPixel(t, got, x, y); !closeTo(px, want, 45) {
			t.Errorf("y=%d: %+v, want %+v", y, px, want)
		}
	}
}

// A header line is drawn over the lines that meet it.
func TestBuiltinTableStyleLinesCross(t *testing.T) {
	const themed1 = "{3C2FFA5D-87B4-456A-9821-1D502468CF0F}" // Themed Style 1 - Accent 1
	big := func(s string) string {
		s = strings.ReplaceAll(s, `w="868680"`, `w="1828800"`)
		return strings.ReplaceAll(s, `<a:tr h="228600">`, `<a:tr h="1828800">`)
	}
	got, err := styleTestRender(t, big(styleTestTable(styleID(themed1), styleAllOptions, nil)), func(error) {}, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Column 0 ends at x = 10 + 192, the header's line is at y = 10 + 192, 1.5pt
	// (2px) wide: the white header line runs unbroken over the first column's
	// line, which is the same width and starts beneath it.
	white := color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	for _, x := range []int{150, 201, 202, 203, 250} {
		if px := renderPixel(t, got, x, 10+192); !closeTo(px, white, 40) {
			t.Errorf("x=%d on the header line: %+v", x, px)
		}
	}
	accent := hexColor(t, "156082")
	if px := renderPixel(t, got, 202, 10+192+30); !closeTo(px, accent, 4) {
		t.Errorf("first column's line below the header: %+v", px)
	}
}

func TestTableStyleIDCase(t *testing.T) {
	// Case and padding of the GUID do not matter, though Office writes it in
	// capitals.
	for _, id := range []string{" {5C22544A-7EE6-4342-B048-85BDC9FD1C3A} ", "{5c22544a-7ee6-4342-b048-85bdc9fd1c3a}"} {
		got, err := styleTestRender(t, styleTestTable(styleID(id), styleAllOptions, nil), nil, nil)
		if err != nil {
			t.Fatalf("%q: %v", id, err)
		}
		x, y := styleCellPixel(0, 0)
		if px := renderPixel(t, got, x, y); !closeTo(px, hexColor(t, "156082"), 2) {
			t.Errorf("%q: %+v", id, px)
		}
	}
}
