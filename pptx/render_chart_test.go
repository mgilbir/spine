package pptx

import (
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"math"
	"strconv"
	"strings"
	"testing"

	"github.com/mgilbir/spine/chart"
	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/render"
)

// renderChartDeck is the text slide, its text removed, with a column chart
// over (20,10) to (60,40).
func renderChartDeck(t testing.TB, c *chart.Chart) ([]byte, render.Options) {
	t.Helper()
	p, s, _, opts := renderTextSlide(t)
	if err := s.AddChart(c, int64(dml.Pixels(20)), int64(dml.Pixels(10)), int64(dml.Pixels(40)), int64(dml.Pixels(30))); err != nil {
		t.Fatal(err)
	}
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	return data, opts
}

// renderFakeCharts records each specification and draws a solid red image.
func renderFakeCharts(specs *[]map[string]any) render.ChartRenderer {
	return func(_ context.Context, spec []byte, scale float64) (image.Image, error) {
		var v map[string]any
		if err := json.Unmarshal(spec, &v); err != nil {
			return nil, err
		}
		*specs = append(*specs, v)
		w, h := int(v["width"].(float64)*scale), int(v["height"].(float64)*scale)
		img := image.NewNRGBA(image.Rect(0, 0, w, h))
		for i := 0; i < len(img.Pix); i += 4 {
			copy(img.Pix[i:], []byte{255, 0, 0, 255})
		}
		return img, nil
	}
}

func TestRenderChartFrames(t *testing.T) {
	c := chart.NewColumn()
	c.SetTitle("Sales")
	c.SetCategories([]string{"Q1", "Q2", "Q3"})
	c.AddSeries("North", []float64{1, 2, 3})
	c.AddSeries("South", []float64{3, 2, 1}).SetColor("00FF00")
	data, opts := renderChartDeck(t, c)
	noText := map[string]func(string) string{"ppt/slides/slide1.xml": func(s string) string { return renderAnyTxBody.ReplaceAllLiteralString(s, "") }}
	// Without a chart renderer the chart is left out: strict mode fails,
	// best effort warns.
	if _, err := renderRewrittenPNG(t, data, opts, noText); !errors.Is(err, render.ErrUnsupported) {
		t.Fatalf("strict without renderer: %v", err)
	}
	var warnings []string
	lenient := opts
	lenient.Warn = func(err error) { warnings = append(warnings, err.Error()) }
	renderSlidePNG(t, data, lenient, noText)
	if len(warnings) != 1 || !strings.Contains(warnings[0], "chart without a chart renderer") {
		t.Fatalf("warnings: %q", warnings)
	}
	// With one, its image fills the frame.
	var specs []map[string]any
	opts.Charts = renderFakeCharts(&specs)
	got := renderSlidePNG(t, data, opts, noText)
	red, white := color.NRGBA{R: 255, A: 255}, color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	for at, want := range map[[2]int]color.NRGBA{{21, 11}: red, {59, 39}: red, {19, 11}: white, {61, 39}: white, {40, 41}: white} {
		if px := renderPixel(t, got, at[0], at[1]); px != want {
			t.Fatalf("at %v: %+v, want %+v", at, px, want)
		}
	}
	if len(specs) != 1 {
		t.Fatalf("%d specifications", len(specs))
	}
	spec := specs[0]
	if spec["width"].(float64) != 40 || spec["height"].(float64) != 30 || spec["title"].(map[string]any)["text"] != "Sales" {
		t.Fatalf("spec: %v", spec)
	}
	signals := map[string]any{}
	for _, s := range spec["signals"].([]any) {
		m := s.(map[string]any)
		signals[m["name"].(string)] = m["value"]
	}
	if cats := signals["cats"].([]any); len(cats) != 3 || cats[1] != "Q2" {
		t.Fatalf("categories: %v", signals["cats"])
	}
	if names := signals["names"].([]any); len(names) != 2 || names[1] != "South" {
		t.Fatalf("names: %v", signals["names"])
	}
	rows := spec["data"].([]any)[0].(map[string]any)["values"].([]any)
	if len(rows) != 6 {
		t.Fatalf("rows: %v", rows)
	}
	// The second series' own color; the first takes the theme's accent1.
	if f := rows[3].(map[string]any)["fill"]; f != "#00ff00" {
		t.Fatalf("own color: %v", f)
	}
	if f := rows[0].(map[string]any)["fill"]; f == "#00ff00" || f == "" {
		t.Fatalf("accent: %v", f)
	}
	// A renderer's failure is the chart's.
	opts.Charts = func(context.Context, []byte, float64) (image.Image, error) { return nil, errors.New("boom") }
	if _, err := renderRewrittenPNG(t, data, opts, noText); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("renderer failure: %v", err)
	}
}

// renderChartXML is a chart part holding a plot area and chart children.
func renderChartXML(plot, chart string) string {
	return `<?xml version="1.0" encoding="UTF-8" standalone="yes"?><c:chartSpace xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><c:chart>` + chart + `<c:plotArea>` + plot + `</c:plotArea></c:chart></c:chartSpace>`
}

// renderChartSer is a series of the given values over categories A, B, C.
func renderChartSerXML(idx int, name, extra string, values ...string) string {
	v := `<c:val><c:numRef><c:f>Sheet1!$B$2</c:f><c:numCache><c:formatCode>General</c:formatCode><c:ptCount val="` + strconv.Itoa(len(values)) + `"/>`
	for i, x := range values {
		v += `<c:pt idx="` + strconv.Itoa(i) + `"><c:v>` + x + `</c:v></c:pt>`
	}
	v += `</c:numCache></c:numRef></c:val>`
	return `<c:ser><c:idx val="` + strconv.Itoa(idx) + `"/><c:order val="` + strconv.Itoa(idx) + `"/><c:tx><c:strRef><c:f>Sheet1!$B$1</c:f><c:strCache><c:ptCount val="1"/><c:pt idx="0"><c:v>` + name + `</c:v></c:pt></c:strCache></c:strRef></c:tx>` + extra +
		`<c:cat><c:strRef><c:f>Sheet1!$A$2</c:f><c:strCache><c:ptCount val="3"/><c:pt idx="0"><c:v>A</c:v></c:pt><c:pt idx="1"><c:v>B</c:v></c:pt><c:pt idx="2"><c:v>C</c:v></c:pt></c:strCache></c:strRef></c:cat>` + v + `</c:ser>`
}

func TestRenderChartPlans(t *testing.T) {
	c := chart.NewColumn()
	c.SetCategories([]string{"A", "B", "C"})
	c.AddSeries("S", []float64{1, 2, 3})
	data, opts := renderChartDeck(t, c)
	axes := `<c:catAx><c:axId val="1"/><c:delete val="0"/><c:axPos val="b"/></c:catAx><c:valAx><c:axId val="2"/><c:delete val="0"/><c:axPos val="l"/><c:majorGridlines/><c:numFmt formatCode="0%" sourceLinked="0"/></c:valAx>`
	spec := func(t *testing.T, plot, chartXML string) (map[string]any, []string, error) {
		t.Helper()
		var specs []map[string]any
		var warnings []string
		o := opts
		o.Charts = renderFakeCharts(&specs)
		o.Warn = func(err error) { warnings = append(warnings, err.Error()) }
		_, err := renderRewrittenPNG(t, data, o, map[string]func(string) string{
			"ppt/charts/chart1.xml": func(string) string { return renderChartXML(plot, chartXML) },
			"ppt/slides/slide1.xml": func(s string) string { return renderAnyTxBody.ReplaceAllLiteralString(s, "") },
		})
		if len(specs) == 0 {
			return nil, warnings, err
		}
		return specs[0], warnings, err
	}
	signal := func(spec map[string]any, name string) any {
		for _, s := range spec["signals"].([]any) {
			if m := s.(map[string]any); m["name"] == name {
				return m["value"]
			}
		}
		return nil
	}
	find := func(v any, key string) []any {
		var out []any
		var walk func(any)
		walk = func(v any) {
			switch x := v.(type) {
			case map[string]any:
				if f, ok := x[key]; ok {
					out = append(out, f)
				}
				for _, y := range x {
					walk(y)
				}
			case []any:
				for _, y := range x {
					walk(y)
				}
			}
		}
		walk(v)
		return out
	}
	t.Run("percent stacked bars", func(t *testing.T) {
		s, warnings, err := spec(t, `<c:barChart><c:barDir val="bar"/><c:grouping val="percentStacked"/><c:varyColors val="0"/>`+renderChartSerXML(0, "One", "", "1", "2", "3")+renderChartSerXML(1, "Two", `<c:spPr><a:solidFill><a:srgbClr val="FF0000"/></a:solidFill></c:spPr>`, "3", "", "1")+`<c:gapWidth val="50"/><c:overlap val="100"/><c:axId val="1"/><c:axId val="2"/></c:barChart>`+axes, `<c:legend><c:legendPos val="b"/></c:legend>`)
		if err != nil || len(warnings) != 0 {
			t.Fatalf("%v %q", err, warnings)
		}
		stack := find(s["data"], "offset")
		formats := find(s["axes"], "format")
		orients := find(s["legends"], "orient")
		if len(stack) != 1 || stack[0] != "normalize" || len(formats) != 1 || formats[0] != ".0%" || len(orients) != 1 || orients[0] != "bottom" {
			t.Fatalf("stack %v formats %v legend %v", stack, formats, orients)
		}
		rows := s["data"].([]any)[0].(map[string]any)["values"].([]any)
		if r := rows[4].(map[string]any); r["v"] != nil || r["fill"] != "#ff0000" {
			t.Fatalf("blank point: %v", r)
		}
		// Bars lie along the value axis: the first category at the bottom.
		if r := find(s["scales"], "range"); len(r) < 1 {
			t.Fatal("ranges")
		}
	})
	t.Run("pie", func(t *testing.T) {
		s, warnings, err := spec(t, `<c:doughnutChart><c:varyColors val="1"/>`+renderChartSerXML(0, "Share", `<c:dPt><c:idx val="2"/><c:spPr><a:solidFill><a:srgbClr val="00FF00"/></a:solidFill></c:spPr></c:dPt>`, "5", "3", "2")+`<c:dLbls><c:showVal val="1"/></c:dLbls><c:firstSliceAng val="90"/><c:holeSize val="40"/></c:doughnutChart>`, `<c:title/><c:autoTitleDeleted val="0"/>`)
		if err != nil || len(warnings) != 0 {
			t.Fatalf("%v %q", err, warnings)
		}
		rows := s["data"].([]any)[0].(map[string]any)["values"].([]any)
		f0, f1, f2 := rows[0].(map[string]any)["fill"], rows[1].(map[string]any)["fill"], rows[2].(map[string]any)["fill"]
		if f0 == f1 || f2 != "#00ff00" {
			t.Fatalf("slice colors %v %v %v", f0, f1, f2)
		}
		if title := s["title"].(map[string]any)["text"]; title != "Share" {
			t.Fatalf("automatic title %v", title)
		}
		if a := find(s["data"], "startAngle"); len(a) != 1 || math.Abs(a[0].(float64)-math.Pi/2) > 0.01 {
			t.Fatalf("first angle %v", a)
		}
		if len(find(s["marks"], "innerRadius")) != 1 || len(find(s["marks"], "theta")) != 1 {
			t.Fatal("hole or labels")
		}
	})
	t.Run("line markers", func(t *testing.T) {
		s, _, err := spec(t, `<c:lineChart><c:grouping val="standard"/>`+renderChartSerXML(0, "One", `<c:marker><c:symbol val="none"/></c:marker>`, "1", "2", "3")+renderChartSerXML(1, "Two", `<c:spPr><a:ln w="12700"><a:solidFill><a:srgbClr val="0000FF"/></a:solidFill></a:ln></c:spPr>`, "3", "2", "1")+`<c:marker val="1"/><c:axId val="1"/><c:axId val="2"/></c:lineChart>`+axes, "")
		if err != nil {
			t.Fatal(err)
		}
		if m := signal(s, "markers").([]any); m[0] != false || m[1] != true {
			t.Fatalf("markers %v", m)
		}
		if c, w := signal(s, "lineColors").([]any), signal(s, "lineWidths").([]any); c[1] != "#0000ff" || w[1] != float64(1.33) || w[0] != float64(3) {
			t.Fatalf("lines %v %v", c, w)
		}
		if s["legends"] != nil {
			t.Fatal("legend without c:legend")
		}
	})
	t.Run("approximations", func(t *testing.T) {
		_, warnings, err := spec(t, `<c:bar3DChart><c:barDir val="col"/><c:grouping val="clustered"/>`+renderChartSerXML(0, "One", "", "1", "2", "3")+`<c:axId val="1"/><c:axId val="2"/></c:bar3DChart>`+strings.Replace(axes, `formatCode="0%"`, `formatCode="[Red]0.0"`, 1), "")
		if err != nil || len(warnings) != 2 || !strings.Contains(warnings[0], "3-D chart drawn flat") || !strings.Contains(warnings[1], "drawn as General") {
			t.Fatalf("%v %q", err, warnings)
		}
	})
	t.Run("unsupported", func(t *testing.T) {
		_, warnings, err := spec(t, `<c:radarChart><c:radarStyle val="marker"/>`+renderChartSerXML(0, "One", "", "1", "2", "3")+`<c:axId val="1"/><c:axId val="2"/></c:radarChart>`+axes, "")
		if err != nil || len(warnings) != 1 || !strings.Contains(warnings[0], "radar") {
			t.Fatalf("%v %q", err, warnings)
		}
	})
	t.Run("point limit", func(t *testing.T) {
		big := strings.Replace(renderChartSerXML(0, "One", "", "1"), `<c:formatCode>General</c:formatCode><c:ptCount val="1"/>`, `<c:formatCode>General</c:formatCode><c:ptCount val="1"/><c:pt idx="99999"><c:v>1</c:v></c:pt>`, 1)
		_, warnings, err := spec(t, `<c:barChart><c:barDir val="col"/>`+big+`</c:barChart>`+axes, "")
		if err != nil || len(warnings) != 1 || !strings.Contains(warnings[0], "chart points") {
			t.Fatalf("%v %q", err, warnings)
		}
	})
}

func FuzzRenderChartPart(f *testing.F) {
	f.Add(renderChartXML(`<c:barChart><c:barDir val="col"/>`+renderChartSerXML(0, "One", "", "1", "2", "3")+`</c:barChart>`, ""))
	f.Add(renderChartXML(`<c:pieChart>`+renderChartSerXML(0, "One", "", "1", "-2", "x")+`</c:pieChart>`, `<c:title/>`))
	f.Add(renderChartXML(`<c:scatterChart><c:scatterStyle val="marker"/>`+renderChartSerXML(0, "One", "", "1e308", "2")+`</c:scatterChart><c:valAx/><c:valAx/>`, ""))
	c := chart.NewColumn()
	c.SetCategories([]string{"A"})
	c.AddSeries("S", []float64{1})
	data, base := renderChartDeck(f, c)
	f.Fuzz(func(t *testing.T, chartXML string) {
		opts := base
		var specs []map[string]any
		opts.Charts = renderFakeCharts(&specs)
		opts.Warn = func(error) {}
		// Hostile chart parts are reported or refused, never a panic, and
		// every specification is valid JSON with a positive size.
		_, _ = renderRewrittenPNG(t, data, opts, map[string]func(string) string{"ppt/charts/chart1.xml": func(string) string { return chartXML }})
	})
}
