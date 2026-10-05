package pptxrender

import (
	"context"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"math"
	"regexp"
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
	t.Run("radar", func(t *testing.T) {
		s, warnings, err := spec(t, `<c:radarChart><c:radarStyle val="filled"/>`+renderChartSerXML(0, "One", "", "1", "2", "3")+`<c:axId val="1"/><c:axId val="2"/></c:radarChart>`+axes, "")
		if err != nil || len(warnings) != 0 {
			t.Fatalf("%v %q", err, warnings)
		}
		// Each series closes on its first point; rings follow the value
		// axis' nice steps from 0 to 3.
		var outline, rings []any
		for _, d := range s["data"].([]any) {
			m := d.(map[string]any)
			switch m["name"] {
			case "outline":
				outline = m["values"].([]any)
			case "rings":
				rings = m["values"].([]any)
			}
		}
		if len(outline) != 4 || outline[3].(map[string]any)["c"] != float64(3) || outline[3].(map[string]any)["v"] != float64(1) {
			t.Fatalf("outline: %v", outline)
		}
		if len(rings) == 0 || len(find(s["marks"], "fill")) == 0 {
			t.Fatalf("rings %d, fill %v", len(rings), find(s["marks"], "fill"))
		}
	})
	t.Run("bubble", func(t *testing.T) {
		ser := strings.Replace(renderChartSerXML(0, "One", "", "1", "2", "3"), `<c:cat>`, `<c:xVal>`, 1)
		ser = strings.Replace(ser, `</c:cat>`, `</c:xVal>`, 1)
		ser = strings.Replace(ser, `<c:val>`, `<c:yVal>`, 1)
		ser = strings.Replace(ser, `</c:val>`, `</c:yVal><c:bubbleSize><c:numLit><c:ptCount val="3"/><c:pt idx="0"><c:v>4</c:v></c:pt><c:pt idx="1"><c:v>0</c:v></c:pt><c:pt idx="2"><c:v>9</c:v></c:pt></c:numLit></c:bubbleSize>`, 1)
		s, warnings, err := spec(t, `<c:bubbleChart>`+ser+`<c:bubbleScale val="50"/><c:sizeRepresents val="w"/><c:axId val="1"/><c:axId val="2"/></c:bubbleChart><c:valAx><c:axId val="1"/><c:axPos val="b"/></c:valAx><c:valAx><c:axId val="2"/><c:axPos val="l"/></c:valAx>`, "")
		if err != nil || len(warnings) != 0 {
			t.Fatalf("%v %q", err, warnings)
		}
		var size map[string]any
		for _, sc := range s["scales"].([]any) {
			if m := sc.(map[string]any); m["name"] == "size" {
				size = m
			}
		}
		if size == nil || size["type"] != "pow" || size["domain"].([]any)[1] != float64(9) {
			t.Fatalf("size scale: %v", size)
		}
		// Largest first, so smaller bubbles draw over it.
		rows := s["data"].([]any)[0].(map[string]any)["values"].([]any)
		if rows[0].(map[string]any)["z"] != float64(9) {
			t.Fatalf("bubble order: %v", rows)
		}
	})
	t.Run("stock", func(t *testing.T) {
		hidden := `<c:spPr><a:ln><a:noFill/></a:ln></c:spPr>`
		s, warnings, err := spec(t, `<c:stockChart>`+renderChartSerXML(0, "Open", hidden, "5", "6", "4")+renderChartSerXML(1, "High", hidden, "7", "8", "6")+renderChartSerXML(2, "Low", hidden, "3", "5", "2")+renderChartSerXML(3, "Close", hidden, "6", "5", "3")+
			`<c:hiLowLines><c:spPr><a:ln><a:solidFill><a:srgbClr val="FF0000"/></a:solidFill></a:ln></c:spPr></c:hiLowLines><c:upDownBars><c:gapWidth val="100"/><c:upBars/><c:downBars/></c:upDownBars><c:axId val="1"/><c:axId val="2"/></c:stockChart>`+axes, "")
		if err != nil || len(warnings) != 0 {
			t.Fatalf("%v %q", err, warnings)
		}
		var updown []any
		for _, d := range s["data"].([]any) {
			if m := d.(map[string]any); m["name"] == "updown" {
				updown = m["values"].([]any)
			}
		}
		// Up where the close is above the open: white, else black.
		if len(updown) != 3 || updown[0].(map[string]any)["fill"] != "#ffffff" || updown[1].(map[string]any)["fill"] != "#000000" {
			t.Fatalf("up-down bars: %v", updown)
		}
		if strokes := find(s["marks"], "stroke"); len(strokes) == 0 || strokes[0].(map[string]any)["value"] != "#ff0000" {
			t.Fatalf("high-low lines: %v", strokes)
		}
	})
	t.Run("pie of pie", func(t *testing.T) {
		s, warnings, err := spec(t, `<c:ofPieChart><c:ofPieType val="bar"/><c:varyColors val="1"/>`+renderChartSerXML(0, "Share", "", "5", "3", "2")+`<c:splitType val="pos"/><c:splitPos val="2"/><c:secondPieSize val="50"/><c:serLines/></c:ofPieChart>`, "")
		if err != nil || len(warnings) != 0 {
			t.Fatalf("%v %q", err, warnings)
		}
		values := map[string][]any{}
		for _, d := range s["data"].([]any) {
			m := d.(map[string]any)
			if v, ok := m["values"].([]any); ok {
				values[m["name"].(string)] = v
			}
		}
		// The last two points move; the first pie holds the first and their
		// sum.
		if len(values["first"]) != 2 || values["first"][1].(map[string]any)["v"] != float64(5) || len(values["second"]) != 2 {
			t.Fatalf("split: %v", values)
		}
		if len(find(s["marks"], "x2")) != 2 {
			t.Fatal("series lines")
		}
	})
	t.Run("surface", func(t *testing.T) {
		_, warnings, err := spec(t, `<c:surfaceChart>`+renderChartSerXML(0, "a", "", "1", "2", "3")+renderChartSerXML(1, "b", "", "3", "4", "5")+`<c:axId val="1"/><c:axId val="2"/></c:surfaceChart>`+axes, "")
		if err != nil || len(warnings) != 1 || !strings.Contains(warnings[0], "banded contour") {
			t.Fatalf("%v %q", err, warnings)
		}
	})
	t.Run("automatic formatting", func(t *testing.T) {
		// Formatting left out is Office's automatic look: black lines and
		// text, tick marks pointing out, a legend centred at the right.
		s, warnings, err := spec(t, `<c:barChart><c:barDir val="col"/><c:grouping val="clustered"/>`+renderChartSerXML(0, "One", "", "1", "2", "3")+renderChartSerXML(1, "Two", "", "3", "2", "1")+`<c:axId val="1"/><c:axId val="2"/></c:barChart><c:catAx><c:axId val="1"/><c:axPos val="b"/></c:catAx><c:valAx><c:axId val="2"/><c:axPos val="l"/><c:majorGridlines/></c:valAx>`, `<c:legend/>`)
		if err != nil || len(warnings) != 0 {
			t.Fatalf("%v %q", err, warnings)
		}
		for _, ax := range s["axes"].([]any) {
			a := ax.(map[string]any)
			if a["domainColor"] != "#000000" || a["tickColor"] != "#000000" || a["labelColor"] != "#000000" || a["domain"] != true || a["ticks"] != true || a["tickSize"] != float64(5) {
				t.Fatalf("axis %v", a)
			}
			if a["scale"] == "val" && a["gridColor"] != "#000000" {
				t.Fatalf("gridlines %v", a["gridColor"])
			}
			if a["scale"] == "catAxis" && a["tickBand"] != "extent" {
				t.Fatalf("category ticks %v", a)
			}
		}
		lg := s["legends"].([]any)[0].(map[string]any)
		anchor := s["config"].(map[string]any)["legend"].(map[string]any)["layout"].(map[string]any)["right"].(map[string]any)["anchor"]
		if lg["orient"] != "right" || lg["labelColor"] != "#000000" || anchor != "middle" || lg["values"] != nil {
			t.Fatalf("legend %v anchor %v", lg, anchor)
		}
		if s["config"].(map[string]any)["text"].(map[string]any)["fill"] != "#000000" {
			t.Fatal("text color")
		}
	})
	t.Run("explicit formatting wins", func(t *testing.T) {
		text := func(color string) string {
			return `<c:txPr><a:bodyPr/><a:p><a:pPr><a:defRPr sz="900"><a:solidFill><a:srgbClr val="` + color + `"/></a:solidFill></a:defRPr></a:pPr><a:endParaRPr lang="en-US"/></a:p></c:txPr>`
		}
		line := func(c string) string {
			return `<c:spPr><a:ln><a:solidFill><a:srgbClr val="` + c + `"/></a:solidFill></a:ln></c:spPr>`
		}
		s, warnings, err := spec(t, `<c:barChart><c:barDir val="col"/><c:grouping val="clustered"/>`+renderChartSerXML(0, "One", "", "1", "2", "3")+`<c:dLbls>`+text("00AA00")+`<c:showVal val="1"/></c:dLbls><c:axId val="1"/><c:axId val="2"/></c:barChart>`+
			`<c:catAx><c:axId val="1"/><c:axPos val="b"/><c:majorTickMark val="none"/>`+line("FF0000")+text("0000FF")+`</c:catAx>`+
			`<c:valAx><c:axId val="2"/><c:scaling><c:min val="0"/><c:max val="8"/></c:scaling><c:axPos val="l"/><c:majorGridlines>`+line("D9D9D9")+`</c:majorGridlines><c:majorTickMark val="in"/><c:spPr><a:ln><a:noFill/></a:ln></c:spPr>`+text("0000FF")+`<c:majorUnit val="2"/></c:valAx>`,
			`<c:legend><c:legendPos val="b"/>`+text("FF00FF")+`</c:legend>`)
		if err != nil || len(warnings) != 0 {
			t.Fatalf("%v %q", err, warnings)
		}
		var cat, val map[string]any
		for _, ax := range s["axes"].([]any) {
			a := ax.(map[string]any)
			if a["scale"] == "val" {
				val = a
			} else {
				cat = a
			}
		}
		if cat["domainColor"] != "#ff0000" || cat["ticks"] != false || cat["labelColor"] != "#0000ff" || cat["labelFontSize"] != float64(12) {
			t.Fatalf("category axis %v", cat)
		}
		if val["domain"] != false || val["ticks"] != true || val["tickSize"] != float64(-5) || val["gridColor"] != "#d9d9d9" || val["labelColor"] != "#0000ff" {
			t.Fatalf("value axis %v", val)
		}
		if v := val["values"].([]any); len(v) != 5 || v[1] != float64(2) || v[4] != float64(8) {
			t.Fatalf("explicit major unit: %v", v)
		}
		lg := s["legends"].([]any)[0].(map[string]any)
		if lg["labelColor"] != "#ff00ff" || lg["orient"] != "bottom" {
			t.Fatalf("legend %v", lg)
		}
		found := false
		for _, c := range find(s["marks"], "fill") {
			if m, ok := c.(map[string]any); ok && m["value"] == "#00aa00" {
				found = true
			}
		}
		if !found {
			t.Fatal("data label color")
		}
	})
	t.Run("horizontal bars list series last first", func(t *testing.T) {
		s, _, err := spec(t, `<c:barChart><c:barDir val="bar"/><c:grouping val="clustered"/>`+renderChartSerXML(0, "One", "", "1", "2", "3")+renderChartSerXML(1, "Two", "", "3", "2", "1")+`<c:axId val="1"/><c:axId val="2"/></c:barChart>`+axes, `<c:legend/>`)
		if err != nil {
			t.Fatal(err)
		}
		if v := s["legends"].([]any)[0].(map[string]any)["values"].([]any); len(v) != 2 || v[0] != float64(1) || v[1] != float64(0) {
			t.Fatalf("legend order %v", v)
		}
		// A column chart's legend keeps the document's order.
		s, _, err = spec(t, `<c:barChart><c:barDir val="col"/><c:grouping val="clustered"/>`+renderChartSerXML(0, "One", "", "1", "2", "3")+renderChartSerXML(1, "Two", "", "3", "2", "1")+`<c:axId val="1"/><c:axId val="2"/></c:barChart>`+axes, `<c:legend/>`)
		if err != nil || s["legends"].([]any)[0].(map[string]any)["values"] != nil {
			t.Fatalf("column legend: %v", err)
		}
	})
	t.Run("pie slice outlines", func(t *testing.T) {
		stroke := func(plot string) (any, any) {
			s, _, err := spec(t, plot, "")
			if err != nil {
				t.Fatal(err)
			}
			enter := find(s["marks"], "enter")[0].(map[string]any)
			return enter["stroke"].(map[string]any)["value"], enter["strokeWidth"].(map[string]any)["value"]
		}
		if c, w := stroke(`<c:pieChart><c:varyColors val="1"/>` + renderChartSerXML(0, "S", "", "1", "2", "3") + `</c:pieChart>`); c != "transparent" || w != float64(0) {
			t.Fatalf("automatic outline %v %v", c, w)
		}
		if c, w := stroke(`<c:pieChart><c:varyColors val="1"/>` + renderChartSerXML(0, "S", `<c:spPr><a:ln w="19050"><a:solidFill><a:srgbClr val="FFFFFF"/></a:solidFill></a:ln></c:spPr>`, "1", "2", "3") + `</c:pieChart>`); c != "#ffffff" || w != float64(2) {
			t.Fatalf("explicit outline %v %v", c, w)
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

// renderAxisValues is the major steps of a plan's value axis in a w by h
// chart.
func renderAxisValues(t *testing.T, p *renderChartPlan, w, h float64) []float64 {
	t.Helper()
	data, err := p.spec(w, h)
	if err != nil {
		t.Fatal(err)
	}
	var s map[string]any
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatal(err)
	}
	for _, ax := range s["axes"].([]any) {
		if a := ax.(map[string]any); a["scale"] == "val" {
			var out []float64
			for _, v := range a["values"].([]any) {
				out = append(out, v.(float64))
			}
			return out
		}
	}
	t.Fatal("no value axis")
	return nil
}

func TestRenderAutomaticMajorUnit(t *testing.T) {
	plan := func(horizontal bool, vals ...float64) *renderChartPlan {
		st := renderAxisStyle{shown: true, color: renderAutoColor, tick: "out", size: 10 * 4.0 / 3, textColor: renderAutoColor}
		p := &renderChartPlan{kind: "bar", horizontal: horizontal, grouping: "clustered", categories: []string{"A", "B", "C"}, font: "Calibri", textSize: st.size, textColor: renderAutoColor,
			cat: st, val: st, catAxis: true, valAxis: true, legendSize: st.size}
		s := renderChartSeries{name: "S", color: "#156082"}
		for i := range vals {
			s.values = append(s.values, &vals[i])
		}
		p.series = []renderChartSeries{s}
		return p
	}
	// Data up to 4.5 over a plot about 3 inches (288 pixels) tall: 0 to 5 in
	// steps of 0.5.
	got := renderAxisValues(t, plan(false, 1, 2, 4.5), 500, 324)
	if len(got) != 11 || got[0] != 0 || got[1] != 0.5 || got[10] != 5 {
		t.Fatalf("vertical axis: %v", got)
	}
	// The same data on a value axis about 6 inches (576 pixels) wide: 0 to 5
	// in steps of 1.
	got = renderAxisValues(t, plan(true, 1, 2, 4.5), 607, 300)
	if len(got) != 6 || got[0] != 0 || got[1] != 1 || got[5] != 5 {
		t.Fatalf("horizontal axis: %v", got)
	}
	// A taller plot affords finer steps; data of a narrow range does not start
	// at zero.
	if got = renderAxisValues(t, plan(false, 1, 2, 4.5), 500, 500); got[1] != 0.2 {
		t.Fatalf("tall axis: %v", got)
	}
	if got = renderAxisValues(t, plan(false, 100, 102, 104), 500, 324); got[0] <= 90 || got[len(got)-1] < 104 {
		t.Fatalf("narrow range: %v", got)
	}
	// Negative values extend the axis below zero, with padding.
	if got = renderAxisValues(t, plan(false, -3, 2, 4.5), 500, 324); got[0] > -3 || got[len(got)-1] < 4.5 {
		t.Fatalf("negative range: %v", got)
	}
	// Percent stacked axes run 0 to 1 whatever the totals.
	p := plan(false, 3, 1, 2)
	p.grouping = "percentStacked"
	if got = renderAxisValues(t, p, 500, 324); got[0] != 0 || got[len(got)-1] != 1 {
		t.Fatalf("percent axis: %v", got)
	}
	// Explicit minimum, maximum and major unit win.
	lo, hi, unit := 1.0, 9.0, 4.0
	p = plan(false, 1, 2, 4.5)
	p.valMin, p.valMax, p.valMajor = &lo, &hi, &unit
	if got = renderAxisValues(t, p, 500, 324); len(got) != 3 || got[0] != 1 || got[1] != 5 || got[2] != 9 {
		t.Fatalf("explicit axis: %v", got)
	}
	// An absurd major unit gives way to the automatic one.
	unit = 1e-9
	if got = renderAxisValues(t, p, 500, 324); len(got) > 20 {
		t.Fatalf("tiny major unit: %d steps", len(got))
	}
}

func FuzzRenderChartPart(f *testing.F) {
	f.Add(renderChartXML(`<c:barChart><c:barDir val="col"/>`+renderChartSerXML(0, "One", "", "1", "2", "3")+`</c:barChart>`, ""))
	f.Add(renderChartXML(`<c:pieChart>`+renderChartSerXML(0, "One", "", "1", "-2", "x")+`</c:pieChart>`, `<c:title/>`))
	f.Add(renderChartXML(`<c:scatterChart><c:scatterStyle val="marker"/>`+renderChartSerXML(0, "One", "", "1e308", "2")+`</c:scatterChart><c:valAx/><c:valAx/>`, ""))
	c := chart.NewColumn()
	c.SetCategories([]string{"A"})
	c.AddSeries("S", []float64{1})
	data, base := renderChartDeck(f, c)
	for _, plot := range []string{
		`<c:radarChart><c:radarStyle val="filled"/>` + renderChartSerXML(0, "One", "", "1", "-2", "3") + `</c:radarChart>`,
		`<c:stockChart>` + renderChartSerXML(0, "H", "", "5", "6") + renderChartSerXML(1, "L", "", "1", "2") + `<c:hiLowLines/><c:upDownBars><c:upBars/><c:downBars/></c:upDownBars></c:stockChart>`,
		`<c:ofPieChart><c:ofPieType val="pie"/>` + renderChartSerXML(0, "One", "", "5", "3", "2") + `<c:splitType val="percent"/><c:splitPos val="30"/></c:ofPieChart>`,
		`<c:surfaceChart>` + renderChartSerXML(0, "a", "", "1", "2") + renderChartSerXML(1, "b", "", "3", "4") + `</c:surfaceChart>`,
	} {
		f.Add(renderChartXML(plot, ""))
	}
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

func TestRenderGroupedChart(t *testing.T) {
	c := chart.NewColumn()
	c.SetCategories([]string{"A"})
	c.AddSeries("S", []float64{1})
	data, opts := renderChartDeck(t, c)
	var specs []map[string]any
	opts.Charts = renderFakeCharts(&specs)
	frame := regexp.MustCompile(`(?s)<p:graphicFrame>.*</p:graphicFrame>`)
	// A group moved 10px right and half the size of its child space draws
	// the chart's (20,10) 40 by 30px frame at (20,5), 20 by 15px.
	got := renderSlidePNG(t, data, opts, map[string]func(string) string{"ppt/slides/slide1.xml": func(s string) string {
		gf := frame.FindString(s)
		if gf == "" {
			t.Fatalf("no chart in %s", s)
		}
		s = renderAnyTxBody.ReplaceAllLiteralString(frame.ReplaceAllLiteralString(s, ""), "")
		return renderAddToTree(`<p:grpSp><p:nvGrpSpPr><p:cNvPr id="95" name="Group"/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr><p:grpSpPr><a:xfrm><a:off x="95250" y="0"/><a:ext cx="381000" cy="285750"/><a:chOff x="0" y="0"/><a:chExt cx="762000" cy="571500"/></a:xfrm></p:grpSpPr>` + gf + `</p:grpSp>`)(s)
	}})
	red, white := color.NRGBA{R: 255, A: 255}, color.NRGBA{R: 255, G: 255, B: 255, A: 255}
	for at, want := range map[[2]int]color.NRGBA{{21, 6}: red, {39, 19}: red, {41, 19}: white, {21, 21}: white} {
		if px := renderPixel(t, got, at[0], at[1]); px != want {
			t.Fatalf("at %v: %+v, want %+v", at, px, want)
		}
	}
	if len(specs) != 1 || specs[0]["width"].(float64) != 20 {
		t.Fatalf("specs: %v", specs)
	}
}

func TestRenderChartScaleFitsImageBudget(t *testing.T) {
	c := chart.NewColumn()
	c.SetCategories([]string{"A"})
	c.AddSeries("S", []float64{1})
	data, opts := renderChartDeck(t, c)
	var scales []float64
	opts.Charts = func(_ context.Context, spec []byte, scale float64) (image.Image, error) {
		scales = append(scales, scale)
		var v map[string]any
		if err := json.Unmarshal(spec, &v); err != nil {
			return nil, err
		}
		w, h := int(v["width"].(float64)*scale), int(v["height"].(float64)*scale)
		return image.NewNRGBA(image.Rect(0, 0, w, h)), nil
	}
	noText := map[string]func(string) string{"ppt/slides/slide1.xml": func(s string) string { return renderAnyTxBody.ReplaceAllLiteralString(s, "") }}
	// The 40 by 30px frame is drawn at twice its size when the budget holds
	// it, at the scale that fits when it does not, and refused below half.
	for _, tc := range []struct {
		pixels int64
		scale  float64
		ok     bool
	}{{1 << 20, 2, true}, {1200, 1, true}, {200, 0, false}} {
		scales = nil
		o := opts
		o.Limits.MaxImagePixels = tc.pixels
		_, err := renderRewrittenPNG(t, data, o, noText)
		if tc.ok != (err == nil) || (tc.ok && (len(scales) != 1 || math.Abs(scales[0]-tc.scale) > 1e-9)) {
			t.Fatalf("%d pixels: scales %v, err %v", tc.pixels, scales, err)
		}
		if !tc.ok && (len(scales) != 0 || !errors.Is(err, render.ErrLimit)) {
			t.Fatalf("%d pixels: renderer called %v, err %v", tc.pixels, scales, err)
		}
	}
}
