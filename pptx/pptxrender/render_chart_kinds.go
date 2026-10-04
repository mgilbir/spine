package pptxrender

import (
	"fmt"
	"math"
	"sort"
	"strconv"

	dmlchart "github.com/mgilbir/spine/common/dml/chart"
	"github.com/mgilbir/spine/render"
)

// renderUpDown is a stock chart's up-down bars: from the first series'
// value to the last's in each category, in up's color where the last is
// higher and down's otherwise, gap their spacing as gapWidth is a bar's.
type renderUpDown struct {
	up, down, line string
	gap            float64
}

// renderOfPie is a pie-of-pie or bar-of-pie's split: which points leave the
// first pie for the second pie or bar, and how that one is drawn.
type renderOfPie struct {
	bar bool
	// split selects the points that move, by index and value.
	split func(i int, v, total float64, values []*float64) bool
	// second is the second plot's size, percent of the first's.
	second, gap float64
	line        string
}

// morePlans plans the chart types beyond bar, line, area, pie and scatter:
// bubble, radar, stock, pie-of-pie and surface charts. It returns the group
// kinds it found.
func (b *renderChartBuilder) morePlans(p *renderChartPlan, pa *dmlchart.PlotArea, sers *[]renderChartSer) ([]string, error) {
	var groups []string
	for _, g := range pa.BubbleChart {
		if g == nil {
			continue
		}
		groups = append(groups, "bubble")
		p.kind = "bubble"
		p.bubbleScale = 1
		if g.BubbleScale != nil {
			p.bubbleScale = float64(min(g.BubbleScale.Val, 300)) / 100
		}
		p.bubbleWidth = g.SizeRepresents != nil && g.SizeRepresents.Val == "w"
		if (g.Bubble3D != nil && g.Bubble3D.Val) || (g.ShowNegBubbles != nil && g.ShowNegBubbles.Val) {
			if err := b.approx("3-D or negative bubbles drawn flat or left out"); err != nil {
				return nil, err
			}
		}
		p.varyColors = g.VaryColors != nil && g.VaryColors.Val
		for i, s := range g.Ser {
			if s != nil {
				*sers = append(*sers, renderChartSer{idx: renderSerIdx(s.Idx, i), tx: s.Tx, spPr: s.SpPr, dPt: s.DPt, dLbls: s.DLbls, cat: s.XVal, val: s.YVal, size: s.BubbleSize,
					marker: &dmlchart.Marker{Symbol: &dmlchart.MarkerStyle{Val: "none"}}, extras: len(s.Trendline) > 0 || len(s.ErrBars) > 0})
			}
		}
		if g.DLbls != nil {
			p.valLabels = renderShowVal(g.DLbls)
		}
	}
	for _, g := range pa.RadarChart {
		if g == nil {
			continue
		}
		groups = append(groups, "radar")
		p.kind = "radar"
		style := "marker"
		if g.RadarStyle != nil && g.RadarStyle.Val != "" {
			style = g.RadarStyle.Val
		}
		p.radarFill = style == "filled"
		for i, s := range g.Ser {
			if s == nil {
				continue
			}
			m := s.Marker
			if m == nil {
				m = &dmlchart.Marker{}
				if style != "marker" {
					m.Symbol = &dmlchart.MarkerStyle{Val: "none"}
				}
			}
			*sers = append(*sers, renderChartSer{idx: renderSerIdx(s.Idx, i), tx: s.Tx, spPr: s.SpPr, dPt: s.DPt, dLbls: s.DLbls, cat: s.Cat, val: s.Val, marker: m})
		}
		if g.DLbls != nil {
			p.valLabels = renderShowVal(g.DLbls)
		}
	}
	for _, g := range pa.StockChart {
		if g == nil {
			continue
		}
		// A stock chart is a line chart of its prices, whose lines are
		// usually hidden, with high-low lines and up-down bars.
		groups = append(groups, "line")
		p.kind, p.grouping = "line", "standard"
		for i, s := range g.Ser {
			if s == nil {
				continue
			}
			m := s.Marker
			if m == nil {
				m = &dmlchart.Marker{}
			}
			*sers = append(*sers, renderChartSer{idx: renderSerIdx(s.Idx, i), tx: s.Tx, spPr: s.SpPr, dPt: s.DPt, dLbls: s.DLbls, cat: s.Cat, val: s.Val, marker: m})
		}
		if g.HiLowLines != nil {
			p.hiLow = "#000000"
			if c, none, _, err := b.line(g.HiLowLines.SpPr); err != nil {
				return nil, err
			} else if none {
				p.hiLow = ""
			} else if c != "" {
				p.hiLow = c
			}
		}
		if u := g.UpDownBars; u != nil {
			ud := &renderUpDown{up: "#ffffff", down: "#000000", line: "#000000", gap: 150}
			if u.GapWidth != nil {
				ud.gap = float64(u.GapWidth.Val)
			}
			for _, bar := range []struct {
				src *dmlchart.UpDownBar
				dst *string
			}{{u.UpBars, &ud.up}, {u.DownBars, &ud.down}} {
				if bar.src == nil {
					continue
				}
				c, none, err := b.fill(bar.src.SpPr)
				if err != nil {
					return nil, err
				}
				if none {
					*bar.dst = "transparent"
				} else if c != "" {
					*bar.dst = c
				}
				if lc, _, _, err := b.line(bar.src.SpPr); err != nil {
					return nil, err
				} else if lc != "" {
					ud.line = lc
				}
			}
			p.upDown = ud
		}
		if g.DLbls != nil {
			p.valLabels = renderShowVal(g.DLbls)
		}
	}
	for _, g := range pa.OfPieChart {
		if g == nil {
			continue
		}
		groups = append(groups, "pie")
		p.kind = "pie"
		p.varyColors = g.VaryColors == nil || g.VaryColors.Val
		op := &renderOfPie{bar: g.OfPieType != nil && g.OfPieType.Val == "bar", second: 75, gap: 100, line: "#000000"}
		if g.SecondPieSize != nil {
			op.second = float64(min(max(g.SecondPieSize.Val, 5), 200))
		}
		if g.GapWidth != nil {
			op.gap = float64(min(g.GapWidth.Val, 500))
		}
		if len(g.SerLines) > 0 && g.SerLines[0] != nil {
			if c, none, _, err := b.line(g.SerLines[0].SpPr); err != nil {
				return nil, err
			} else if none {
				op.line = ""
			} else if c != "" {
				op.line = c
			}
		}
		splitType := "auto"
		if g.SplitType != nil && g.SplitType.Val != "" {
			splitType = g.SplitType.Val
		}
		var pos float64
		if g.SplitPos != nil {
			pos = g.SplitPos.Val
		}
		switch splitType {
		case "auto":
			// Office's automatic split moves the last third of the points,
			// at least one.
			if err := b.approx("automatic pie-of-pie split drawn as the last third of its points"); err != nil {
				return nil, err
			}
			op.split = func(i int, _, _ float64, values []*float64) bool { return i >= len(values)-max(1, len(values)/3) }
		case "pos":
			n := int(math.Max(0, math.Round(pos)))
			op.split = func(i int, _, _ float64, values []*float64) bool { return i >= len(values)-n }
		case "val":
			op.split = func(_ int, v, _ float64, _ []*float64) bool { return v < pos }
		case "percent":
			op.split = func(_ int, v, total float64, _ []*float64) bool { return total > 0 && v/total*100 < pos }
		case "cust":
			chosen := map[int]bool{}
			if g.CustSplit != nil {
				for _, pt := range g.CustSplit.SecondPiePt {
					if pt != nil && pt.Val >= 0 && pt.Val < renderMaxChartPoints {
						chosen[int(pt.Val)] = true
					}
				}
			}
			op.split = func(i int, _, _ float64, _ []*float64) bool { return chosen[i] }
		default:
			return nil, fmt.Errorf("%w: pie-of-pie split", render.ErrInvalid)
		}
		p.ofPie = op
		for i, s := range g.Ser {
			if s != nil {
				*sers = append(*sers, renderChartSer{idx: renderSerIdx(s.Idx, i), tx: s.Tx, spPr: s.SpPr, dPt: s.DPt, dLbls: s.DLbls, cat: s.Cat, val: s.Val})
			}
		}
		if g.DLbls != nil {
			p.valLabels = renderShowVal(g.DLbls)
		}
	}
	surfaces := func(wire *dmlchart.Boolean, ser []*dmlchart.SurfaceSer) error {
		groups = append(groups, "surface")
		p.kind = "surface"
		if err := b.approx("surface chart drawn as a banded contour of its cells"); err != nil {
			return err
		}
		if wire != nil && wire.Val {
			if err := b.approx("wireframe surface chart drawn filled"); err != nil {
				return err
			}
		}
		// Bands take the theme's accents in turn.
		p.bandColors = p.bandColors[:0]
		for i := 0; i < 16; i++ {
			c, err := b.accent(i)
			if err != nil {
				return err
			}
			p.bandColors = append(p.bandColors, c)
		}
		for i, s := range ser {
			if s != nil {
				*sers = append(*sers, renderChartSer{idx: renderSerIdx(s.Idx, i), tx: s.Tx, spPr: s.SpPr, cat: s.Cat, val: s.Val})
			}
		}
		return nil
	}
	for _, g := range pa.SurfaceChart {
		if g != nil {
			if err := surfaces(g.Wireframe, g.Ser); err != nil {
				return nil, err
			}
		}
	}
	for _, g := range pa.Surface3DChart {
		if g != nil {
			if err := surfaces(g.Wireframe, g.Ser); err != nil {
				return nil, err
			}
		}
	}
	return groups, nil
}

// renderNiceTicks is the axis from 0, or a negative minimum, to max in
// about count nice steps: 1, 2 or 5 times a power of ten.
func renderNiceTicks(lo, hi float64, count int) (ticks []float64) {
	lo, hi = math.Min(lo, 0), math.Max(hi, 0)
	if hi <= lo {
		hi = lo + 1
	}
	raw := (hi - lo) / float64(max(1, count))
	mag := math.Pow(10, math.Floor(math.Log10(raw)))
	step := mag
	for _, m := range []float64{1, 2, 5, 10} {
		if m*mag >= raw {
			step = m * mag
			break
		}
	}
	start := math.Floor(lo/step) * step
	for v := start; v <= hi+step*1e-9 || len(ticks) < 2; v += step {
		ticks = append(ticks, math.Round(v/step)*step)
		if len(ticks) > 64 {
			break
		}
	}
	if ticks[len(ticks)-1] < hi {
		ticks = append(ticks, ticks[len(ticks)-1]+step)
	}
	return ticks
}

// renderSpecNumber writes a number for an expression the specification
// builds; only numbers computed here are written so.
func renderSpecNumber(v float64) string {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		v = 0
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// bubbleSpec draws a bubble chart over scatter axes: each point a circle
// whose area, or with bubbleWidth width, is its size's share of the
// largest, which spans a quarter of the plot's smaller side times
// bubbleScale. Bubbles of no or negative size are left out.
func (p *renderChartPlan) bubbleSpec(rows []map[string]any) (data, scales, marks []map[string]any) {
	type obj = map[string]any
	biggest := 0.0
	for _, s := range p.series {
		for _, z := range s.sizes {
			if z != nil && *z > biggest {
				biggest = *z
			}
		}
	}
	if biggest <= 0 {
		biggest = 1
	}
	area := "pow(min(width, height) * 0.25 * " + renderSpecNumber(p.bubbleScale) + ", 2) * PI / 4"
	size := obj{"name": "size", "type": "linear", "domain": []any{0, biggest}, "range": []any{0, obj{"signal": area}}, "zero": true}
	if p.bubbleWidth {
		size["type"], size["exponent"] = "pow", 2
	}
	scales = append(scales, size)
	data = append(data, obj{"name": "bubbles", "source": "table", "transform": []obj{{"type": "filter", "expr": "datum.z != null && datum.z > 0 && datum.v != null"}}})
	marks = append(marks, obj{"type": "symbol", "from": obj{"data": "bubbles"}, "encode": obj{"enter": obj{
		"x": obj{"scale": "x", "field": "x"}, "y": obj{"scale": "y", "field": "v"}, "size": obj{"scale": "size", "field": "z"},
		"shape": obj{"value": "circle"}, "fill": obj{"field": "fill"}, "stroke": obj{"signal": "lineColors[datum.s]"}, "strokeWidth": obj{"signal": "lineWidths[datum.s]"},
	}}})
	// Larger bubbles first, so smaller ones stay visible over them.
	sort.SliceStable(rows, func(i, j int) bool {
		zi, _ := rows[i]["z"].(float64)
		zj, _ := rows[j]["z"].(float64)
		return zi > zj
	})
	return data, scales, marks
}

// radarSpec draws a radar chart about the plot's centre: a spoke per
// category, clockwise from 12 o'clock, rings at the value axis' steps, and
// each series a closed line through its values, filled with radarFill.
func (p *renderChartPlan) radarSpec(rows []map[string]any) (data, scales, marks []map[string]any) {
	type obj = map[string]any
	n := len(p.categories)
	lo, hi := 0.0, 0.0
	for _, s := range p.series {
		for _, v := range s.values {
			if v != nil {
				lo, hi = math.Min(lo, *v), math.Max(hi, *v)
			}
		}
	}
	if p.valMin != nil {
		lo = *p.valMin
	}
	if p.valMax != nil {
		hi = *p.valMax
	}
	ticks := renderNiceTicks(lo, hi, 5)
	scales = append(scales, obj{"name": "r", "type": "linear", "domain": []any{ticks[0], ticks[len(ticks)-1]}, "range": []any{0, obj{"signal": "min(width, height) / 2 - " + renderSpecNumber(p.textSize*1.5)}}})
	angle := "2 * PI * datum.c / " + strconv.Itoa(n)
	x := func(r string) obj { return obj{"signal": "width / 2 + (" + r + ") * sin(" + angle + ")"} }
	y := func(r string) obj { return obj{"signal": "height / 2 - (" + r + ") * cos(" + angle + ")"} }
	// Rings and spokes.
	var rings []obj
	for _, t := range ticks {
		for c := 0; c <= n; c++ {
			rings = append(rings, obj{"t": t, "c": c % n})
		}
	}
	var spokes []obj
	for c := 0; c < n; c++ {
		spokes = append(spokes, obj{"c": c})
	}
	data = append(data, obj{"name": "rings", "values": rings}, obj{"name": "spokes", "values": spokes},
		obj{"name": "ticks", "values": func() []obj {
			var out []obj
			for _, t := range ticks {
				out = append(out, obj{"t": t, "c": 0})
			}
			return out
		}()})
	if p.grid {
		marks = append(marks, obj{"type": "group", "from": obj{"facet": obj{"name": "ring", "data": "rings", "groupby": "t"}}, "marks": []obj{{
			"type": "line", "from": obj{"data": "ring"}, "encode": obj{"enter": obj{"x": x("scale('r', datum.t)"), "y": y("scale('r', datum.t)"), "stroke": obj{"value": p.gridColor}, "strokeWidth": obj{"value": 1}}},
		}}})
	}
	if p.catAxis {
		outer := "scale('r', " + renderSpecNumber(ticks[len(ticks)-1]) + ")"
		marks = append(marks,
			obj{"type": "rule", "from": obj{"data": "spokes"}, "encode": obj{"enter": obj{"x": obj{"signal": "width / 2"}, "y": obj{"signal": "height / 2"}, "x2": x(outer), "y2": y(outer), "stroke": obj{"value": p.axisColor}}}},
			obj{"type": "text", "from": obj{"data": "spokes"}, "encode": obj{"enter": obj{
				"x": x(outer + " + " + renderSpecNumber(p.textSize*0.8)), "y": y(outer + " + " + renderSpecNumber(p.textSize*0.8)),
				"text": obj{"signal": "cats[datum.c]"}, "align": obj{"signal": "abs(sin(" + angle + ")) < 0.1 ? 'center' : sin(" + angle + ") > 0 ? 'left' : 'right'"},
				"baseline": obj{"signal": "abs(cos(" + angle + ")) < 0.1 ? 'middle' : cos(" + angle + ") > 0 ? 'bottom' : 'top'"},
			}}})
	}
	if p.valAxis {
		format := p.valFormat
		if format == "" {
			format = "~f"
		}
		marks = append(marks, obj{"type": "text", "from": obj{"data": "ticks"}, "encode": obj{"enter": obj{
			"x": obj{"signal": "width / 2 - 4"}, "y": obj{"signal": "height / 2 - scale('r', datum.t)"},
			"text": obj{"signal": "format(datum.t, '" + format + "')"}, "align": obj{"value": "right"}, "baseline": obj{"value": "middle"},
		}}})
	}
	// Each series closes on its first point.
	var closing []obj
	for _, r := range rows {
		if r["c"] == 0 {
			c := obj{}
			for k, v := range r {
				c[k] = v
			}
			c["c"] = n
			closing = append(closing, c)
		}
	}
	poly := obj{"x": x("scale('r', datum.v)"), "y": y("scale('r', datum.v)"), "stroke": obj{"signal": "lineColors[parent.s]"}, "strokeWidth": obj{"signal": "lineWidths[parent.s]"},
		"defined": obj{"signal": "datum.v != null"}, "strokeJoin": obj{"value": "round"}}
	if p.radarFill {
		poly["fill"] = obj{"scale": "color", "signal": "parent.s"}
	}
	marks = append(marks, obj{"type": "group", "from": obj{"facet": obj{"name": "series", "data": "outline", "groupby": "s"}}, "marks": []obj{
		{"type": "line", "from": obj{"data": "series"}, "encode": obj{"enter": poly}},
		{"type": "symbol", "from": obj{"data": "series"}, "encode": obj{"enter": obj{
			"x": x("scale('r', datum.v)"), "y": y("scale('r', datum.v)"), "fill": obj{"field": "fill"}, "size": obj{"value": 30},
			"opacity": obj{"signal": "markers[parent.s] && datum.v != null && datum.c < " + strconv.Itoa(n) + " ? 1 : 0"},
		}}},
	}})
	data = append(data, obj{"name": "outline", "values": append(append([]obj{}, rows...), closing...)})
	return data, scales, marks
}

// stockSpec adds a stock chart's high-low lines, from the lowest to the
// highest value of each category, and up-down bars, from the first
// series' value to the last's.
func (p *renderChartPlan) stockSpec() (data, marks []map[string]any) {
	type obj = map[string]any
	pos, val := "x", "y"
	if p.horizontal {
		pos, val = "y", "x"
	}
	if p.hiLow != "" {
		data = append(data, obj{"name": "hilo", "source": "table", "transform": []obj{
			{"type": "filter", "expr": "datum.v != null"},
			{"type": "aggregate", "groupby": []string{"c"}, "fields": []string{"v", "v"}, "ops": []string{"min", "max"}, "as": []string{"lo", "hi"}},
		}})
		marks = append(marks, obj{"type": "rule", "from": obj{"data": "hilo"}, "encode": obj{"enter": obj{
			pos: obj{"scale": "cat", "field": "c", "band": 0.5}, val: obj{"scale": "val", "field": "lo"}, val + "2": obj{"scale": "val", "field": "hi"},
			"stroke": obj{"value": p.hiLow}, "strokeWidth": obj{"value": 1},
		}}})
	}
	if u := p.upDown; u != nil && len(p.series) >= 2 {
		first, last := p.series[0].values, p.series[len(p.series)-1].values
		var bars []obj
		for c := 0; c < len(p.categories) && c < len(first) && c < len(last); c++ {
			if first[c] == nil || last[c] == nil {
				continue
			}
			fill := u.down
			if *last[c] >= *first[c] {
				fill = u.up
			}
			bars = append(bars, obj{"c": c, "o": *first[c], "cl": *last[c], "fill": fill})
		}
		width := 1 / (1 + u.gap/100)
		data = append(data, obj{"name": "updown", "values": bars})
		marks = append(marks, obj{"type": "rect", "from": obj{"data": "updown"}, "encode": obj{"enter": obj{
			pos: obj{"scale": "cat", "field": "c", "band": round4((1 - width) / 2)},
			map[string]string{"x": "width", "y": "height"}[pos]: obj{"scale": "cat", "band": round4(width)},
			val: obj{"scale": "val", "field": "o"}, val + "2": obj{"scale": "val", "field": "cl"},
			"fill": obj{"field": "fill"}, "stroke": obj{"value": u.line}, "strokeWidth": obj{"value": 1},
		}}})
	}
	return data, marks
}

// ofPieSpec draws a pie-of-pie or bar-of-pie: the first pie, left, holds
// the points that stay and one slice for those that move, turned to face
// the second plot, right, which holds the moved points; series lines join
// the slice's edges to the second plot.
func (p *renderChartPlan) ofPieSpec(pieFills []string) (data, marks []map[string]any) {
	type obj = map[string]any
	op := p.ofPie
	values := p.series[0].values
	total := 0.0
	for _, v := range values {
		if v != nil && *v > 0 {
			total += *v
		}
	}
	var stay, moved []obj
	other := 0.0
	for i, v := range values {
		if v == nil || *v <= 0 {
			continue
		}
		row := obj{"c": i, "v": *v, "fill": pieFills[i]}
		if op.split(i, *v, total, values) {
			moved = append(moved, row)
			other += *v
		} else {
			stay = append(stay, row)
		}
	}
	// The moved points' slice comes last, centred on 3 o'clock.
	start := math.Pi/2 - 2*math.Pi
	if total > 0 {
		start += math.Pi * other / total
	}
	if other > 0 {
		stay = append(stay, obj{"c": -1, "v": other, "fill": "#a5a5a5"})
	}
	// The plots share the width: the first pie, the gap, the second.
	scale := op.second / 100
	r := "min(height / 2, width / (2 + " + renderSpecNumber(op.gap/100) + " + 2 * " + renderSpecNumber(scale) + ")) * 0.9"
	cx1 := "(" + r + ") / 0.9"
	cx2 := "width - (" + r + ") / 0.9 * " + renderSpecNumber(scale)
	data = append(data,
		obj{"name": "first", "values": stay, "transform": []obj{{"type": "pie", "field": "v", "startAngle": round4(start), "endAngle": round4(start + 2*math.Pi), "sort": false}}},
		obj{"name": "second", "values": moved, "transform": []obj{{"type": "pie", "field": "v", "sort": false}}},
	)
	marks = append(marks, obj{"type": "arc", "from": obj{"data": "first"}, "encode": obj{"enter": obj{
		"x": obj{"signal": cx1}, "y": obj{"signal": "height / 2"}, "startAngle": obj{"field": "startAngle"}, "endAngle": obj{"field": "endAngle"},
		"outerRadius": obj{"signal": r}, "fill": obj{"field": "fill"}, "stroke": obj{"value": "#ffffff"}, "strokeWidth": obj{"value": 1},
	}}})
	r2 := "(" + r + ") * " + renderSpecNumber(scale)
	if op.bar {
		// A bar as tall as the second pie would be, its points stacked.
		data[len(data)-1] = obj{"name": "second", "values": moved, "transform": []obj{{"type": "stack", "field": "v"}}}
		marks = append(marks, obj{"type": "rect", "from": obj{"data": "second"}, "encode": obj{"enter": obj{
			"x": obj{"signal": cx2 + " - " + r2 + " / 2"}, "width": obj{"signal": r2},
			"y":    obj{"signal": "height / 2 + " + r2 + " - 2 * " + r2 + " * datum.y0 / " + renderSpecNumber(math.Max(other, 1e-9))},
			"y2":   obj{"signal": "height / 2 + " + r2 + " - 2 * " + r2 + " * datum.y1 / " + renderSpecNumber(math.Max(other, 1e-9))},
			"fill": obj{"field": "fill"}, "stroke": obj{"value": "#ffffff"}, "strokeWidth": obj{"value": 1},
		}}})
	} else {
		marks = append(marks, obj{"type": "arc", "from": obj{"data": "second"}, "encode": obj{"enter": obj{
			"x": obj{"signal": cx2}, "y": obj{"signal": "height / 2"}, "startAngle": obj{"field": "startAngle"}, "endAngle": obj{"field": "endAngle"},
			"outerRadius": obj{"signal": r2}, "fill": obj{"field": "fill"}, "stroke": obj{"value": "#ffffff"}, "strokeWidth": obj{"value": 1},
		}}})
	}
	if op.line != "" && other > 0 && total > 0 {
		half := math.Pi * other / total
		for _, sign := range []float64{-1, 1} {
			a := math.Pi/2 + sign*half
			marks = append(marks, obj{"type": "rule", "encode": obj{"enter": obj{
				"x":      obj{"signal": cx1 + " + (" + r + ") * " + renderSpecNumber(math.Sin(a))},
				"y":      obj{"signal": "height / 2 - (" + r + ") * " + renderSpecNumber(math.Cos(a))},
				"x2":     obj{"signal": cx2 + " - (" + r2 + ") * " + renderSpecNumber(map[bool]float64{true: 0.5, false: 0}[op.bar])},
				"y2":     obj{"signal": "height / 2 + " + renderSpecNumber(sign) + " * (" + r2 + ")"},
				"stroke": obj{"value": op.line}, "strokeWidth": obj{"value": 1},
			}}})
		}
	}
	return data, marks
}

// renderSurfaceSteps is how finely each surface cell is interpolated, on
// each side, and renderMaxSurfaceCells bounds the pieces drawn in all.
const (
	renderSurfaceSteps    = 8
	renderMaxSurfaceCells = 1 << 15
)

// surfaceSpec draws a surface chart from above as a banded contour: the
// values are interpolated bilinearly across each cell between two
// categories and two series, in pieces up to renderSurfaceSteps a side,
// each piece taking the band of its value. Bands follow the value axis'
// steps in the theme's accents, and the legend names them.
func (p *renderChartPlan) surfaceSpec(bandColors []string) (data, scales, marks, legends []map[string]any) {
	type obj = map[string]any
	lo, hi := 0.0, 0.0
	for _, s := range p.series {
		for _, v := range s.values {
			if v != nil {
				lo, hi = math.Min(lo, *v), math.Max(hi, *v)
			}
		}
	}
	ticks := renderNiceTicks(lo, hi, 6)
	bands := min(len(ticks)-1, len(bandColors))
	value := func(s, c int) float64 {
		s = max(0, min(s, len(p.series)-1))
		if c < len(p.series[s].values) && p.series[s].values[c] != nil {
			return *p.series[s].values[c]
		}
		return 0
	}
	rows := max(1, len(p.series)-1)
	cols := max(1, len(p.categories)-1)
	steps := renderSurfaceSteps
	for steps > 1 && rows*cols*steps*steps > renderMaxSurfaceCells {
		steps--
	}
	var cells []obj
	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			v00, v01, v10, v11 := value(r, c), value(r, c+1), value(r+1, c), value(r+1, c+1)
			for i := 0; i < steps; i++ {
				for j := 0; j < steps; j++ {
					u, w := (float64(j)+0.5)/float64(steps), (float64(i)+0.5)/float64(steps)
					v := v00*(1-u)*(1-w) + v01*u*(1-w) + v10*(1-u)*w + v11*u*w
					band := max(0, min(sort.SearchFloat64s(ticks, v+1e-12)-1, bands-1))
					x0 := (float64(c) + float64(j)/float64(steps)) / float64(cols)
					y0 := (float64(r) + float64(i)/float64(steps)) / float64(rows)
					cells = append(cells, obj{"x0": round4(x0), "x1": round4(x0 + 1/float64(cols*steps)), "y0": round4(y0), "y1": round4(y0 + 1/float64(rows*steps)), "fill": bandColors[band]})
				}
			}
		}
	}
	data = append(data, obj{"name": "cells", "values": cells})
	// Pieces overlap by a hair so no seam shows between them.
	marks = append(marks, obj{"type": "rect", "from": obj{"data": "cells"}, "encode": obj{"enter": obj{
		"x": obj{"signal": "width * datum.x0"}, "x2": obj{"signal": "width * datum.x1 + 0.5"},
		"y": obj{"signal": "height * (1 - datum.y1) - 0.5"}, "y2": obj{"signal": "height * (1 - datum.y0)"},
		"fill": obj{"field": "fill"},
	}}})
	if p.legend != "" {
		format := p.valFormat
		if format == "" {
			format = "~f"
		}
		names := make([]string, bands)
		var labels []obj
		for i := 0; i < bands; i++ {
			names[i] = strconv.Itoa(i)
			labels = append(labels, obj{"lo": ticks[i], "hi": ticks[i+1]})
		}
		data = append(data, obj{"name": "bands", "values": labels})
		scales = append(scales, obj{"name": "band", "type": "ordinal", "domain": names, "range": bandColors[:bands]})
		legends = append(legends, obj{"orient": p.legend, "fill": "band", "symbolType": "square", "labelFont": p.font, "labelFontSize": round2(p.textSize), "labelColor": p.textColor,
			"encode": obj{"labels": obj{"update": obj{"text": obj{"signal": "format(data('bands')[toNumber(datum.value)].lo, '" + format + "') + '-' + format(data('bands')[toNumber(datum.value)].hi, '" + format + "')"}}}}})
	}
	return data, scales, marks, legends
}
