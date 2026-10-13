package pptxrender

import (
	"bytes"
	"context"
	"reflect"
	"slices"
	"testing"

	"github.com/mgilbir/forme/fonts/notosans"
	"github.com/mgilbir/forme/shape"
	"github.com/mgilbir/spine/chart"
	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/internal/testutil"
	"github.com/mgilbir/spine/pptx"
	"github.com/mgilbir/spine/render"
)

// TestRenderBOMPrefixedParts is the #493 regression for presentations: a UTF-8
// byte order mark at the start of a part (Office writes them) read as content
// outside the root, so strict rendering failed with "content outside XML
// root". With a BOM on every XML part, every part the renderer reads (slide,
// layout, master, theme, table styles, chart) must render exactly as without
// one.
func TestRenderBOMPrefixedParts(t *testing.T) {
	p := pptx.CreateWidescreen()
	layout, err := p.LayoutByType(pptx.LayoutTitleAndContent)
	if err != nil {
		t.Fatal(err)
	}
	titled := p.AddSlideFromLayout(layout)
	for _, ph := range titled.Placeholders() {
		ph.SetText("Some text")
	}
	sh := pptx.NewAutoShape(pptx.PresetRect)
	sh.SetPosition(dml.Inches(1), dml.Inches(5))
	sh.SetSize(dml.Inches(2), dml.Inches(1))
	sh.SetFill(dml.NewSolidFill(dml.ThemeColorAccent1.ToColor()))
	if err = titled.AddShape(sh); err != nil {
		t.Fatal(err)
	}
	table := p.AddSlide().AddTable(3, 2)
	table.SetStyleID("{5C22544A-7EE6-4342-B048-85BDC9FD1C3A}") // Medium Style 2 - Accent 1
	table.SetFirstRow(true)
	table.SetBandedRows(true)
	table.Cell(0, 0).SetText("A")
	c := chart.NewColumn()
	c.SetTitle("Sales")
	c.SetCategories([]string{"Q1", "Q2"})
	c.AddSeries("North", []float64{1, 2})
	if err = p.AddSlide().AddChart(c, int64(dml.Inches(1)), int64(dml.Inches(1)), int64(dml.Inches(4)), int64(dml.Inches(3))); err != nil {
		t.Fatal(err)
	}
	plain, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	withBOM, changed, err := testutil.WithBOM(plain)
	if err != nil {
		t.Fatal(err)
	}
	for _, part := range []string{"ppt/slides/slide1.xml", "ppt/slideLayouts/slideLayout2.xml", "ppt/slideMasters/slideMaster1.xml", "ppt/theme/theme1.xml", "ppt/tableStyles.xml", "ppt/presentation.xml", "ppt/charts/chart1.xml"} {
		if !slices.Contains(changed, part) {
			t.Fatalf("fixture has no %s to mark: %v", part, changed)
		}
	}

	noto, err := notosans.Face()
	if err != nil {
		t.Fatal(err)
	}
	render := func(data []byte) ([]byte, []map[string]any) {
		t.Helper()
		deck, err := pptx.OpenReader(bytes.NewReader(data), int64(len(data)))
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = deck.Close() }()
		var specs []map[string]any
		opts := render.Options{Charts: renderFakeCharts(&specs), Fonts: func(context.Context, render.FontRequest) (*shape.Face, error) { return noto, nil }}
		var out bytes.Buffer
		for i, s := range deck.Slides() {
			page, err := PrepareSlide(context.Background(), s, opts) // strict
			if err != nil {
				t.Fatalf("slide %d: %v", i+1, err)
			}
			if err = page.WriteSVG(context.Background(), &out, 48); err != nil {
				t.Fatal(err)
			}
		}
		return out.Bytes(), specs
	}
	wantSVG, wantSpecs := render(plain)
	gotSVG, gotSpecs := render(withBOM)
	if len(wantSpecs) != 1 {
		t.Fatalf("fixture drew %d charts", len(wantSpecs))
	}
	if !bytes.Equal(gotSVG, wantSVG) || !reflect.DeepEqual(gotSpecs, wantSpecs) {
		t.Error("a presentation with BOM-prefixed parts renders differently")
	}
}
