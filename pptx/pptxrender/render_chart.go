package pptxrender

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/mgilbir/forme/style"
	"github.com/mgilbir/spine/common/dml"
	dmlchart "github.com/mgilbir/spine/common/dml/chart"
	xmlb "github.com/mgilbir/spine/common/xml"
	"github.com/mgilbir/spine/pptx/internal/oxml"
	"github.com/mgilbir/spine/render"
)

// Chart bounds: series per chart and points per series.
const (
	renderMaxChartSeries = 256
	renderMaxChartPoints = 4096
)

// renderChartScale is the image pixels per CSS pixel a chart is drawn at.
const renderChartScale = 2.0

// renderMinChartScale is the least scale a chart is drawn at where the image
// budget cannot hold renderChartScale.
const renderMinChartScale = 0.5

// renderFrameName is a graphic frame's name, for reports.
func renderFrameName(gf *oxml.GraphicFrame) string {
	if gf != nil && gf.NvGraphicFramePr != nil && gf.NvGraphicFramePr.CNvPr != nil {
		return gf.NvGraphicFramePr.CNvPr.Name
	}
	return ""
}

// renderChartSeries is one series' resolved data and look.
type renderChartSeries struct {
	name   string
	values []*float64 // nil where a point is blank
	xs     []*float64 // scatter and bubble X values
	sizes  []*float64 // bubble sizes
	color  string
	// pointColors are the colors of points whose own fill differs, by index.
	pointColors map[int]string
	line        string  // stroke color, "" for none
	lineWidth   float64 // pixels
	markers     bool
	labels      bool
}

// renderChartPlan is a chart reduced to what its Vega specification draws.
type renderChartPlan struct {
	kind       string // bar, line, area, pie, scatter, bubble, radar, surface
	horizontal bool
	grouping   string // clustered, stacked, percentStacked, standard
	categories []string
	series     []renderChartSeries
	gapWidth   float64 // percent of a bar's width
	overlap    float64 // percent, -100 to 100
	hole       float64 // doughnut hole, percent of the radius
	firstAngle float64 // degrees clockwise from 12 o'clock
	varyColors bool

	title               string
	titleSize, textSize float64 // pixels
	// textSized says the chart space sets its text size.
	textSized bool
	// seriesLine is the automatic width of a line series, in pixels.
	seriesLine      float64
	textColor, font string
	titleColor      string
	// legend is the Vega legend orient, "" for none; legendSize and
	// legendColor style its entries, labelColor the shown values.
	legend               string
	legendSize           float64
	legendColor          string
	labelColor           string
	catAxis, valAxis     bool // the axis' labels are drawn
	cat, val             renderAxisStyle
	catTitle, valTitle   string
	grid                 bool
	gridColor, axisColor string
	valFormat            string
	valMin, valMax       *float64
	valMajor             *float64
	xMin, xMax, xMajor   *float64 // a scatter or bubble chart's X axis
	background           string
	// midCat puts points on category boundaries rather than in between.
	midCat bool
	// bubbleScale is the largest bubble's share of a quarter of the plot's
	// smaller side, and bubbleWidth sizes bubbles by width, not area.
	bubbleScale float64
	bubbleWidth bool
	// radarFill fills each radar series' polygon.
	radarFill bool
	// hiLow is the color of a stock chart's high-low lines, "" for none;
	// upDown its up-down bars.
	hiLow  string
	upDown *renderUpDown
	// ofPie splits points into a second pie or bar.
	ofPie *renderOfPie
	// bandColors color a surface chart's value bands.
	bandColors             []string
	reverseCats, valLabels bool
}

// renderAxisStyle is how an axis draws itself: its line, tick marks and
// labels. Formatting an axis leaves out is Office's automatic: a black line
// with tick marks pointing out.
type renderAxisStyle struct {
	shown     bool   // the axis is not deleted
	color     string // line and tick color
	noLine    bool
	tick      string // major tick marks: out, in or none
	size      float64
	textColor string
}

// renderAutoColor is the automatic color of lines and text: what Office
// draws for a chart that carries no formatting, as the charts spine writes.
const renderAutoColor = "#000000"

// renderChartColor formats a color for Vega.
func renderChartColor(c style.RGBA) string {
	if c.A >= 1 {
		return fmt.Sprintf("#%02x%02x%02x", uint8(math.Round(c.R)), uint8(math.Round(c.G)), uint8(math.Round(c.B)))
	}
	return fmt.Sprintf("rgba(%d,%d,%d,%s)", uint8(math.Round(c.R)), uint8(math.Round(c.G)), uint8(math.Round(c.B)), strconv.FormatFloat(c.A, 'f', 3, 64))
}

// renderChartPart parses a chart part and plans it. Details the plan leaves
// out or approximates are reported through colors.
func renderChartPart(data []byte, colors *renderColors) (*renderChartPlan, error) {
	var cs dmlchart.ChartSpace
	if err := xmlb.Unmarshal(data, &cs); err != nil {
		return nil, fmt.Errorf("%w: chart part: %w", render.ErrInvalid, err)
	}
	if cs.Chart == nil || cs.Chart.PlotArea == nil {
		return nil, fmt.Errorf("%w: chart without a plot area", render.ErrInvalid)
	}
	b := renderChartBuilder{colors: colors}
	return b.plan(&cs)
}

type renderChartBuilder struct {
	colors *renderColors
}

func (b *renderChartBuilder) approx(format string, args ...any) error {
	return b.colors.approximate(fmt.Errorf("%w: "+format, append([]any{render.ErrUnsupported}, args...)...))
}

// accent is the automatic color of the i-th series or point: the theme's
// six accents, then each darkened and lightened in turn, as Office cycles
// them.
func (b *renderChartBuilder) accent(i int) (string, error) {
	c, err := b.colors.scheme(fmt.Sprintf("accent%d", i%6+1), nil)
	if err != nil {
		return "", err
	}
	if round := i / 6; round > 0 {
		// Rounds alternate 60% darker, 60% lighter, 40% darker, ...
		f := []float64{0.6, -0.6, 0.4, -0.4, 0.8, -0.8}[(round-1)%6]
		if f > 0 {
			c.R, c.G, c.B = c.R*f, c.G*f, c.B*f
		} else {
			f = -f
			c.R, c.G, c.B = 255-(255-c.R)*f, 255-(255-c.G)*f, 255-(255-c.B)*f
		}
	}
	return renderChartColor(c), nil
}

// fill resolves a solid fill, or reports "" for none or unset.
func (b *renderChartBuilder) fill(p *dml.SpPr) (color string, none bool, err error) {
	if p == nil {
		return "", false, nil
	}
	switch {
	case p.NoFill != nil:
		return "", true, nil
	case p.SolidFill != nil:
		c, err := b.colors.solid(p.SolidFill, nil)
		return renderChartColor(c), false, err
	case p.GradFill != nil || p.PattFill != nil:
		c, err := b.colors.representative(p.GradFill, p.PattFill, nil)
		return renderChartColor(c), false, err
	case p.BlipFill != nil:
		return "", false, b.approx("chart picture fill drawn automatic")
	}
	return "", false, nil
}

// line resolves an outline: its color, "" for none or unset, and width in
// pixels, 0 for unset.
func (b *renderChartBuilder) line(p *dml.SpPr) (color string, none bool, width float64, err error) {
	if p == nil || p.Ln == nil {
		return "", false, 0, nil
	}
	ln := p.Ln
	if ln.W != nil {
		width = float64(*ln.W) / float64(dml.EMUsPerPixel)
	}
	switch {
	case ln.NoFill != nil:
		return "", true, width, nil
	case ln.SolidFill != nil:
		c, err := b.colors.solid(ln.SolidFill, nil)
		return renderChartColor(c), false, width, err
	case ln.GradFill != nil || ln.PattFill != nil:
		c, err := b.colors.representative(ln.GradFill, ln.PattFill, nil)
		return renderChartColor(c), false, width, err
	}
	if ln.PrstDash != nil || ln.CustDash != nil {
		if err := b.approx("dashed chart line drawn solid"); err != nil {
			return "", false, 0, err
		}
	}
	return "", false, width, nil
}

// seriesLine is the automatic width of a line series: the theme's first line
// style (0.75pt, the standard Office theme's, when the theme sets none) times
// 3, 5 or 7 by the chart's c:style (2 when absent), as Office draws it.
func (b *renderChartBuilder) seriesLine(cs *dmlchart.ChartSpace) (float64, error) {
	style := uint32(2)
	if cs.Style != nil {
		style = cs.Style.Val
	}
	rel := 3.0
	switch {
	case style >= 9 && style <= 24, style >= 33:
		rel = 5
	case style >= 25 && style <= 32:
		rel = 7
	}
	base := 9525.0
	th, err := b.colors.loadTheme()
	if err != nil {
		return 0, err
	}
	if th != nil && th.ThemeElements != nil && th.ThemeElements.FmtScheme != nil {
		if l := th.ThemeElements.FmtScheme.LnStyleLst; l != nil && len(l.Ln) > 0 && l.Ln[0] != nil && l.Ln[0].W != nil && *l.Ln[0].W > 0 {
			base = float64(*l.Ln[0].W)
		}
	}
	return base * rel / float64(dml.EMUsPerPixel), nil
}

// text resolves a text body's default run size, in pixels, and color.
func (b *renderChartBuilder) text(t *dml.TxBody, size float64, color string) (float64, string, error) {
	if t == nil {
		return size, color, nil
	}
	for _, p := range t.P {
		if p == nil || p.PPr == nil || p.PPr.DefRPr == nil {
			continue
		}
		r := p.PPr.DefRPr
		if r.Sz > 0 {
			size = float64(r.Sz) / 100 * 4 / 3
		}
		if r.SolidFill != nil {
			c, err := b.colors.solid(r.SolidFill, nil)
			if err != nil {
				return size, color, err
			}
			color = renderChartColor(c)
		}
		break
	}
	return size, color, nil
}

// richText is a text body's characters, paragraphs joined by spaces.
func renderRichText(t *dml.TxBody) string {
	if t == nil {
		return ""
	}
	var parts []string
	for _, p := range t.P {
		if p == nil {
			continue
		}
		var line strings.Builder
		for _, c := range p.Children() {
			switch {
			case c.R != nil:
				line.WriteString(c.R.T)
			case c.Fld != nil:
				line.WriteString(c.Fld.T)
			case c.Br != nil:
				line.WriteString(" ")
			}
		}
		if line.Len() > 0 {
			parts = append(parts, line.String())
		}
	}
	return strings.Join(parts, " ")
}

// renderChartStrings reads a category or series-name cache.
func renderChartStrings(src *dmlchart.AxDataSource) ([]string, error) {
	if src == nil {
		return nil, nil
	}
	var pts []*dmlchart.StrVal
	count := 0
	switch {
	case src.StrRef != nil && src.StrRef.StrCache != nil:
		pts, count = src.StrRef.StrCache.Pt, renderPtCount(src.StrRef.StrCache.PtCount)
	case src.StrLit != nil:
		pts, count = src.StrLit.Pt, renderPtCount(src.StrLit.PtCount)
	case src.NumRef != nil && src.NumRef.NumCache != nil:
		nums, err := renderChartNumbers(&dmlchart.NumDataSource{NumRef: src.NumRef})
		out := make([]string, len(nums))
		for i, v := range nums {
			if v != nil {
				out[i] = strconv.FormatFloat(*v, 'f', -1, 64)
			}
		}
		return out, err
	case src.NumLit != nil:
		nums, err := renderChartNumbers(&dmlchart.NumDataSource{NumLit: src.NumLit})
		out := make([]string, len(nums))
		for i, v := range nums {
			if v != nil {
				out[i] = strconv.FormatFloat(*v, 'f', -1, 64)
			}
		}
		return out, err
	case src.MultiLvlStrRef != nil && src.MultiLvlStrRef.MultiLvlStrCache != nil:
		// The innermost level labels each point.
		c := src.MultiLvlStrRef.MultiLvlStrCache
		if len(c.Lvl) > 0 && c.Lvl[0] != nil {
			pts, count = c.Lvl[0].Pt, renderPtCount(c.PtCount)
		}
	}
	for _, p := range pts {
		if p != nil && int(p.Idx) >= count {
			count = int(p.Idx) + 1
		}
	}
	if count > renderMaxChartPoints {
		return nil, fmt.Errorf("%w: chart points", render.ErrLimit)
	}
	out := make([]string, count)
	for _, p := range pts {
		if p != nil {
			out[p.Idx] = p.V
		}
	}
	return out, nil
}

func renderPtCount(c *dmlchart.UnsignedInt) int {
	if c == nil || c.Val < 0 || c.Val > renderMaxChartPoints {
		return 0
	}
	return int(c.Val)
}

// renderChartNumbers reads a value cache; blank points are nil.
func renderChartNumbers(src *dmlchart.NumDataSource) ([]*float64, error) {
	if src == nil {
		return nil, nil
	}
	var d *dmlchart.NumData
	switch {
	case src.NumRef != nil:
		d = src.NumRef.NumCache
	case src.NumLit != nil:
		d = src.NumLit
	}
	if d == nil {
		return nil, nil
	}
	count := renderPtCount(d.PtCount)
	for _, p := range d.Pt {
		if p != nil && int(p.Idx) >= count {
			count = int(p.Idx) + 1
		}
	}
	if count > renderMaxChartPoints {
		return nil, fmt.Errorf("%w: chart points", render.ErrLimit)
	}
	out := make([]*float64, count)
	for _, p := range d.Pt {
		if p == nil {
			continue
		}
		v, err := strconv.ParseFloat(strings.TrimSpace(p.V), 64)
		if err != nil || math.IsNaN(v) || math.IsInf(v, 0) {
			continue
		}
		out[p.Idx] = &v
	}
	return out, nil
}

// renderChartSer is the part of a series every chart type shares.
type renderChartSer struct {
	idx    int
	tx     *dmlchart.SerTx
	spPr   *dml.SpPr
	dPt    []*dmlchart.DataPoint
	dLbls  *dmlchart.DataLabels
	cat    *dmlchart.AxDataSource
	val    *dmlchart.NumDataSource
	marker *dmlchart.Marker
	size   *dmlchart.NumDataSource // bubble sizes
	extras bool                    // trendlines or error bars
}

func renderSerIdx(i *dmlchart.UnsignedInt, fallback int) int {
	if i == nil || i.Val < 0 || i.Val > renderMaxChartSeries*64 {
		return fallback
	}
	return int(i.Val)
}

func (b *renderChartBuilder) plan(cs *dmlchart.ChartSpace) (*renderChartPlan, error) {
	c := cs.Chart
	pa := c.PlotArea
	p := &renderChartPlan{gapWidth: 150, varyColors: false, textColor: renderAutoColor, gridColor: renderAutoColor, axisColor: renderAutoColor, catAxis: true, valAxis: true}
	var err error
	// Text: the chart space's defaults, 10pt by the standard's default.
	if p.textSize, p.textColor, err = b.text(cs.TxPr, 10*4.0/3, p.textColor); err != nil {
		return nil, err
	}
	p.textSized = p.textSize != 10*4.0/3
	if p.seriesLine, err = b.seriesLine(cs); err != nil {
		return nil, err
	}
	p.cat = renderAxisStyle{shown: true, color: renderAutoColor, tick: "out", size: p.textSize, textColor: p.textColor}
	p.val = p.cat
	p.legendSize, p.legendColor, p.labelColor = p.textSize, p.textColor, p.textColor
	p.font = "Calibri"
	if theme, err := b.colors.loadTheme(); err == nil && theme.ThemeElements != nil && theme.ThemeElements.FontScheme != nil {
		if f := theme.ThemeElements.FontScheme.MinorFont; f != nil && f.Latin != nil && f.Latin.Typeface != "" {
			p.font = f.Latin.Typeface
		}
	}
	if fill, _, err := b.fill(cs.SpPr); err != nil {
		return nil, err
	} else if fill != "" {
		p.background = fill
	}
	var sers []renderChartSer
	var groups []string
	addGroup := func(name string) { groups = append(groups, name) }
	for _, g := range pa.BarChart {
		if g == nil {
			continue
		}
		addGroup("bar")
		p.kind = "bar"
		p.horizontal = g.BarDir != nil && g.BarDir.Val == "bar"
		p.grouping = "clustered"
		if g.Grouping != nil && g.Grouping.Val != "" {
			p.grouping = g.Grouping.Val
		}
		if g.GapWidth != nil {
			p.gapWidth = float64(g.GapWidth.Val)
		}
		if g.Overlap != nil {
			p.overlap = float64(g.Overlap.Val)
		}
		p.varyColors = g.VaryColors != nil && g.VaryColors.Val
		for i, s := range g.Ser {
			if s != nil {
				sers = append(sers, renderChartSer{idx: renderSerIdx(s.Idx, i), tx: s.Tx, spPr: s.SpPr, dPt: s.DPt, dLbls: s.DLbls, cat: s.Cat, val: s.Val, extras: len(s.Trendline) > 0 || s.ErrBars != nil})
			}
		}
		if g.DLbls != nil {
			p.valLabels = b.showVal(p, g.DLbls)
		}
	}
	for _, g := range pa.LineChart {
		if g == nil {
			continue
		}
		addGroup("line")
		p.kind = "line"
		p.grouping = "standard"
		if g.Grouping != nil && g.Grouping.Val != "" {
			p.grouping = g.Grouping.Val
		}
		markers := g.Marker != nil && g.Marker.Val
		for i, s := range g.Ser {
			if s == nil {
				continue
			}
			m := s.Marker
			if m == nil && markers {
				m = &dmlchart.Marker{}
			}
			if m == nil {
				m = &dmlchart.Marker{Symbol: &dmlchart.MarkerStyle{Val: "none"}}
			}
			if s.Smooth != nil && s.Smooth.Val {
				if err := b.approx("smoothed chart line drawn straight"); err != nil {
					return nil, err
				}
			}
			sers = append(sers, renderChartSer{idx: renderSerIdx(s.Idx, i), tx: s.Tx, spPr: s.SpPr, dPt: s.DPt, dLbls: s.DLbls, cat: s.Cat, val: s.Val, marker: m, extras: len(s.Trendline) > 0 || s.ErrBars != nil})
		}
		if g.DLbls != nil {
			p.valLabels = b.showVal(p, g.DLbls)
		}
	}
	for _, g := range pa.AreaChart {
		if g == nil {
			continue
		}
		addGroup("area")
		p.kind = "area"
		p.grouping = "standard"
		if g.Grouping != nil && g.Grouping.Val != "" {
			p.grouping = g.Grouping.Val
		}
		for i, s := range g.Ser {
			if s != nil {
				sers = append(sers, renderChartSer{idx: renderSerIdx(s.Idx, i), tx: s.Tx, spPr: s.SpPr, dPt: s.DPt, dLbls: s.DLbls, cat: s.Cat, val: s.Val, extras: len(s.Trendline) > 0 || s.ErrBars != nil})
			}
		}
		if g.DLbls != nil {
			p.valLabels = b.showVal(p, g.DLbls)
		}
	}
	pie := func(vary *dmlchart.Boolean, ser []*dmlchart.PieSer, dl *dmlchart.DataLabels, first *dmlchart.UnsignedInt) {
		addGroup("pie")
		p.kind = "pie"
		p.varyColors = vary == nil || vary.Val
		if first != nil {
			p.firstAngle = float64(first.Val % 360)
		}
		for i, s := range ser {
			if s != nil {
				sers = append(sers, renderChartSer{idx: renderSerIdx(s.Idx, i), tx: s.Tx, spPr: s.SpPr, dPt: s.DPt, dLbls: s.DLbls, cat: s.Cat, val: s.Val})
			}
		}
		if dl != nil {
			p.valLabels = b.showVal(p, dl)
		}
	}
	for _, g := range pa.PieChart {
		if g != nil {
			pie(g.VaryColors, g.Ser, g.DLbls, g.FirstSliceAng)
		}
	}
	for _, g := range pa.DoughnutChart {
		if g != nil {
			pie(g.VaryColors, g.Ser, g.DLbls, g.FirstSliceAng)
			p.hole = 50
			if g.HoleSize != nil {
				p.hole = float64(min(max(g.HoleSize.Val, 1), 90))
			}
		}
	}
	for _, g := range pa.ScatterChart {
		if g == nil {
			continue
		}
		addGroup("scatter")
		p.kind = "scatter"
		style := "marker"
		if g.ScatterStyle != nil {
			style = g.ScatterStyle.Val
		}
		for i, s := range g.Ser {
			if s == nil {
				continue
			}
			m := s.Marker
			if m == nil && style != "line" && style != "smooth" && style != "none" {
				m = &dmlchart.Marker{}
			}
			if m == nil {
				m = &dmlchart.Marker{Symbol: &dmlchart.MarkerStyle{Val: "none"}}
			}
			ser := renderChartSer{idx: renderSerIdx(s.Idx, i), tx: s.Tx, spPr: s.SpPr, dPt: s.DPt, dLbls: s.DLbls, cat: s.XVal, val: s.YVal, marker: m, extras: len(s.Trendline) > 0 || len(s.ErrBars) > 0}
			if style == "marker" || style == "none" {
				// A marker-only scatter has no lines unless a series draws one.
				if ser.spPr == nil || ser.spPr.Ln == nil {
					sp := dml.SpPr{}
					if ser.spPr != nil {
						sp = *ser.spPr
					}
					sp.Ln = &dml.Ln{NoFill: &dml.NoFillXML{}}
					ser.spPr = &sp
				}
			}
			sers = append(sers, ser)
		}
		if g.DLbls != nil {
			p.valLabels = b.showVal(p, g.DLbls)
		}
	}
	if len(pa.Bar3DChart)+len(pa.Line3DChart)+len(pa.Pie3DChart)+len(pa.Area3DChart) > 0 {
		// 3-D charts draw as their flat kinds.
		if err := b.approx("3-D chart drawn flat"); err != nil {
			return nil, err
		}
		for _, g := range pa.Bar3DChart {
			if g == nil {
				continue
			}
			addGroup("bar")
			p.kind, p.grouping = "bar", "clustered"
			p.horizontal = g.BarDir != nil && g.BarDir.Val == "bar"
			if g.Grouping != nil && g.Grouping.Val != "" && g.Grouping.Val != "standard" {
				p.grouping = g.Grouping.Val
			}
			if g.GapWidth != nil {
				p.gapWidth = float64(g.GapWidth.Val)
			}
			for i, s := range g.Ser {
				if s != nil {
					sers = append(sers, renderChartSer{idx: renderSerIdx(s.Idx, i), tx: s.Tx, spPr: s.SpPr, dPt: s.DPt, dLbls: s.DLbls, cat: s.Cat, val: s.Val})
				}
			}
		}
		for _, g := range pa.Line3DChart {
			if g == nil {
				continue
			}
			addGroup("line")
			p.kind, p.grouping = "line", "standard"
			for i, s := range g.Ser {
				if s != nil {
					sers = append(sers, renderChartSer{idx: renderSerIdx(s.Idx, i), tx: s.Tx, spPr: s.SpPr, dPt: s.DPt, dLbls: s.DLbls, cat: s.Cat, val: s.Val, marker: &dmlchart.Marker{Symbol: &dmlchart.MarkerStyle{Val: "none"}}})
				}
			}
		}
		for _, g := range pa.Pie3DChart {
			if g != nil {
				pie(g.VaryColors, g.Ser, g.DLbls, nil)
			}
		}
		for _, g := range pa.Area3DChart {
			if g == nil {
				continue
			}
			addGroup("area")
			p.kind, p.grouping = "area", "standard"
			if g.Grouping != nil && g.Grouping.Val != "" {
				p.grouping = g.Grouping.Val
			}
			for i, s := range g.Ser {
				if s != nil {
					sers = append(sers, renderChartSer{idx: renderSerIdx(s.Idx, i), tx: s.Tx, spPr: s.SpPr, dPt: s.DPt, dLbls: s.DLbls, cat: s.Cat, val: s.Val})
				}
			}
		}
	}
	more, err := b.morePlans(p, pa, &sers)
	if err != nil {
		return nil, err
	}
	groups = append(groups, more...)
	if len(groups) == 0 {
		return nil, fmt.Errorf("%w: chart without a drawn chart type", render.ErrUnsupported)
	}
	if len(groups) > 1 {
		// Mixed types draw as the last group's type.
		if err := b.approx("combination chart drawn as one chart type"); err != nil {
			return nil, err
		}
		if p.kind != "bar" && p.kind != "line" && p.kind != "area" {
			return nil, fmt.Errorf("%w: combination with a %s chart", render.ErrUnsupported, p.kind)
		}
	}
	if len(sers) == 0 {
		return nil, fmt.Errorf("%w: chart without series", render.ErrUnsupported)
	}
	if len(sers) > renderMaxChartSeries {
		return nil, fmt.Errorf("%w: chart series", render.ErrLimit)
	}
	if (len(pa.ValAx) > 1 && p.kind != "scatter" && p.kind != "bubble") || len(pa.CatAx) > 1 || len(pa.DateAx) > 0 {
		if err := b.approx("secondary or date axis drawn as the primary category axis"); err != nil {
			return nil, err
		}
	}
	if err := b.axes(p, pa); err != nil {
		return nil, err
	}
	if err := b.legend(p, c); err != nil {
		return nil, err
	}
	if err := b.title(p, c, sers); err != nil {
		return nil, err
	}
	for _, s := range sers {
		rs, cats, err := b.series(p, s, len(sers))
		if err != nil {
			return nil, err
		}
		if len(cats) > len(p.categories) {
			p.categories = cats
		}
		p.series = append(p.series, rs)
	}
	if p.kind == "pie" && len(p.series) > 1 {
		// A pie draws its first series.
		p.series = p.series[:1]
	}
	n := len(p.categories)
	for _, s := range p.series {
		n = max(n, len(s.values))
	}
	if n == 0 {
		return nil, fmt.Errorf("%w: chart without points", render.ErrUnsupported)
	}
	for len(p.categories) < n {
		p.categories = append(p.categories, strconv.Itoa(len(p.categories)+1))
	}
	return p, nil
}

// showVal reports whether d shows values, taking their text color from its
// formatting; labels without any are in the chart's automatic text color.
func (b *renderChartBuilder) showVal(p *renderChartPlan, d *dmlchart.DataLabels) bool {
	if d != nil && d.TxPr != nil {
		if _, color, err := b.text(d.TxPr, p.textSize, p.labelColor); err == nil {
			p.labelColor = color
		}
	}
	return renderShowVal(d)
}

func renderShowVal(d *dmlchart.DataLabels) bool {
	if d == nil || (d.Delete != nil && d.Delete.Val) {
		return false
	}
	return d.ShowVal != nil && d.ShowVal.Val
}

// series resolves one series of total: name, values and colors.
func (b *renderChartBuilder) series(p *renderChartPlan, s renderChartSer, total int) (renderChartSeries, []string, error) {
	var rs renderChartSeries
	if s.extras {
		if err := b.approx("trendlines and error bars left out"); err != nil {
			return rs, nil, err
		}
	}
	if s.tx != nil {
		if s.tx.StrRef != nil && s.tx.StrRef.StrCache != nil {
			for _, pt := range s.tx.StrRef.StrCache.Pt {
				if pt != nil {
					rs.name = pt.V
					break
				}
			}
		} else {
			rs.name = s.tx.V
		}
	}
	if rs.name == "" {
		rs.name = fmt.Sprintf("Series%d", s.idx+1)
	}
	var err error
	if rs.values, err = renderChartNumbers(s.val); err != nil {
		return rs, nil, err
	}
	var cats []string
	if p.kind == "bubble" {
		if rs.sizes, err = renderChartNumbers(s.size); err != nil {
			return rs, nil, err
		}
	}
	if p.kind == "scatter" || p.kind == "bubble" {
		var xs []string
		if xs, err = renderChartStrings(s.cat); err != nil {
			return rs, nil, err
		}
		rs.xs = make([]*float64, len(rs.values))
		for i := range rs.values {
			v := float64(i + 1)
			if i < len(xs) {
				if f, err := strconv.ParseFloat(xs[i], 64); err == nil && !math.IsNaN(f) && !math.IsInf(f, 0) {
					v = f
				}
			}
			rs.xs[i] = &v
		}
	} else if cats, err = renderChartStrings(s.cat); err != nil {
		return rs, nil, err
	}
	// Colors: the series' own, or its automatic accent; lines and markers
	// draw in the line's color, and areas and bars in the fill's.
	auto, err := b.accent(s.idx)
	if err != nil {
		return rs, nil, err
	}
	fill, noFill, err := b.fill(s.spPr)
	if err != nil {
		return rs, nil, err
	}
	line, noLine, width, err := b.line(s.spPr)
	if err != nil {
		return rs, nil, err
	}
	switch p.kind {
	case "line", "scatter", "radar":
		rs.color, rs.line = auto, auto
		if line != "" {
			rs.color, rs.line = line, line
		}
		if noLine {
			rs.line = ""
		}
		rs.lineWidth = p.seriesLine
		if width > 0 {
			rs.lineWidth = width
		}
		if m := s.marker; m != nil && (m.Symbol == nil || m.Symbol.Val != "none") {
			rs.markers = true
			if mf, _, err := b.fill(m.SpPr); err != nil {
				return rs, nil, err
			} else if mf != "" {
				rs.color = mf
			}
		}
	default:
		rs.color = auto
		if fill != "" {
			rs.color = fill
		}
		if noFill {
			rs.color = "transparent"
		}
		if line != "" && !noLine {
			rs.line, rs.lineWidth = line, max(width, 1)
		}
	}
	// Varied colors give each point its own accent, in a pie or a chart of
	// one series.
	rs.pointColors = map[int]string{}
	if p.varyColors && (p.kind == "pie" || total == 1) {
		for i := range rs.values {
			c, err := b.accent(i)
			if err != nil {
				return rs, nil, err
			}
			rs.pointColors[i] = c
		}
	}
	for _, d := range s.dPt {
		if d == nil || d.Idx == nil || d.Idx.Val < 0 || d.Idx.Val >= int64(len(rs.values)) {
			continue
		}
		c, _, err := b.fill(d.SpPr)
		if err != nil {
			return rs, nil, err
		}
		if c != "" {
			rs.pointColors[int(d.Idx.Val)] = c
		}
	}
	rs.labels = p.valLabels
	if s.dLbls != nil {
		rs.labels = b.showVal(p, s.dLbls)
	}
	return rs, cats, nil
}

// axisStyle resolves an axis' line, tick marks and text. Absent formatting is
// Office's automatic look: a black line, tick marks pointing out and text in
// the chart's color.
func (b *renderChartBuilder) axisStyle(p *renderChartPlan, deleted *dmlchart.Boolean, ln *dml.SpPr, tick *dmlchart.TickMark, tx *dml.TxBody) (renderAxisStyle, error) {
	st := renderAxisStyle{shown: deleted == nil || !deleted.Val, color: renderAutoColor, tick: "out", size: p.textSize, textColor: p.textColor}
	color, none, _, err := b.line(ln)
	if err != nil {
		return st, err
	}
	if none {
		st.noLine = true
	} else if color != "" {
		st.color = color
	}
	if tick != nil {
		switch tick.Val {
		case "none", "out", "in":
			st.tick = tick.Val
		default:
			// A tick mark crossing the axis is drawn pointing out.
			if err := b.approx("chart axis tick marks that cross drawn pointing out"); err != nil {
				return st, err
			}
		}
	}
	st.size, st.textColor, err = b.text(tx, st.size, st.textColor)
	return st, err
}

func (b *renderChartBuilder) axes(p *renderChartPlan, pa *dmlchart.PlotArea) error {
	if p.kind == "pie" {
		p.catAxis, p.valAxis = false, false
		p.cat.shown, p.val.shown = false, false
		return nil
	}
	if p.kind == "radar" {
		// A radar's value axis runs up its first spoke; its category
		// labels ring it.
		p.catAxis = true
	}
	var err error
	labelled := func(deleted *dmlchart.Boolean, pos *dmlchart.TickLblPos) bool {
		return (deleted == nil || !deleted.Val) && (pos == nil || pos.Val != "none")
	}
	if len(pa.CatAx) > 0 && pa.CatAx[0] != nil {
		a := pa.CatAx[0]
		p.catAxis = labelled(a.Delete, a.TickLblPos)
		p.reverseCats = a.Scaling != nil && a.Scaling.Orientation != nil && a.Scaling.Orientation.Val == "maxMin"
		if a.Title != nil {
			p.catTitle = renderTitleText(a.Title)
		}
		if p.cat, err = b.axisStyle(p, a.Delete, a.SpPr, a.MajorTickMark, a.TxPr); err != nil {
			return err
		}
	}
	// A scatter chart's X axis is its first value axis, its Y the second.
	vals := pa.ValAx
	if (p.kind == "scatter" || p.kind == "bubble") && len(vals) == 2 {
		x := vals[0]
		if x != nil && x.AxPos != nil && (x.AxPos.Val == "b" || x.AxPos.Val == "t") {
			vals = vals[1:]
		} else {
			x = vals[1]
			vals = vals[:1]
		}
		p.catAxis = x == nil || labelled(x.Delete, nil)
		if x != nil {
			p.catAxis = labelled(x.Delete, x.TickLblPos)
			if x.Title != nil {
				p.catTitle = renderTitleText(x.Title)
			}
			if p.cat, err = b.axisStyle(p, x.Delete, x.SpPr, x.MajorTickMark, x.TxPr); err != nil {
				return err
			}
			p.xMin, p.xMax, p.xMajor = renderScaling(x)
		}
	}
	if len(vals) > 0 && vals[0] != nil {
		a := vals[0]
		p.valAxis = labelled(a.Delete, a.TickLblPos)
		p.midCat = a.CrossBetween != nil && a.CrossBetween.Val == "midCat" && (p.kind == "line" || p.kind == "area")
		p.grid = a.MajorGridlines != nil
		if a.MajorGridlines != nil {
			if color, none, _, err := b.line(a.MajorGridlines.SpPr); err != nil {
				return err
			} else if none {
				p.grid = false
			} else if color != "" {
				p.gridColor = color
			}
		}
		if a.Title != nil {
			p.valTitle = renderTitleText(a.Title)
		}
		p.valMin, p.valMax, p.valMajor = renderScaling(a)
		if a.Scaling != nil && a.Scaling.LogBase != nil {
			if err := b.approx("logarithmic chart axis drawn linear"); err != nil {
				return err
			}
		}
		if a.NumFmt != nil && (a.NumFmt.SourceLinked == nil || !*a.NumFmt.SourceLinked) {
			if p.valFormat, err = b.format(a.NumFmt.FormatCode); err != nil {
				return err
			}
		}
		if p.val, err = b.axisStyle(p, a.Delete, a.SpPr, a.MajorTickMark, a.TxPr); err != nil {
			return err
		}
	}
	if p.grouping == "percentStacked" && p.valFormat == "" {
		p.valFormat = ".0%"
	}
	p.axisColor = p.cat.color
	if p.kind == "radar" {
		// A radar's labels all draw in the specification's text style.
		p.textSize, p.textColor = p.cat.size, p.cat.textColor
	}
	return nil
}

// renderScaling is a value axis' explicit minimum, maximum and major unit.
func renderScaling(a *dmlchart.ValAx) (lo, hi, major *float64) {
	if a.Scaling != nil {
		if a.Scaling.Min != nil {
			v := a.Scaling.Min.Val
			lo = &v
		}
		if a.Scaling.Max != nil {
			v := a.Scaling.Max.Val
			hi = &v
		}
	}
	if a.MajorUnit != nil && a.MajorUnit.Val > 0 {
		v := a.MajorUnit.Val
		major = &v
	}
	return lo, hi, major
}

// format maps a number format code to a d3 format: the common forms
// exactly, others as General, approximately.
func (b *renderChartBuilder) format(code string) (string, error) {
	switch strings.TrimSpace(code) {
	case "", "General":
		return "", nil
	case "0":
		return ",.0f", nil
	case "0.0":
		return ".1f", nil
	case "0.00":
		return ".2f", nil
	case "#,##0":
		return ",.0f", nil
	case "#,##0.0":
		return ",.1f", nil
	case "#,##0.00":
		return ",.2f", nil
	case "0%":
		return ".0%", nil
	case "0.0%":
		return ".1%", nil
	case "0.00%":
		return ".2%", nil
	}
	return "", b.approx("number format %q drawn as General", code)
}

func renderTitleText(t *dmlchart.Title) string {
	if t == nil || t.Tx == nil {
		return ""
	}
	if t.Tx.Rich != nil {
		return renderRichText(t.Tx.Rich)
	}
	if r := t.Tx.StrRef; r != nil && r.StrCache != nil {
		for _, pt := range r.StrCache.Pt {
			if pt != nil {
				return pt.V
			}
		}
	}
	return ""
}

func (b *renderChartBuilder) legend(p *renderChartPlan, c *dmlchart.Chart) error {
	if c.Legend == nil {
		return nil
	}
	p.legend = "right"
	var err error
	if p.legendSize, p.legendColor, err = b.text(c.Legend.TxPr, p.legendSize, p.legendColor); err != nil {
		return err
	}
	if c.Legend.LegendPos != nil {
		switch c.Legend.LegendPos.Val {
		case "b":
			p.legend = "bottom"
		case "t":
			p.legend = "top"
		case "l":
			p.legend = "left"
		case "tr":
			p.legend = "top-right"
		}
	}
	if len(c.Legend.LegendEntry) > 0 {
		if err := b.approx("legend entry formatting left out"); err != nil {
			return err
		}
	}
	return nil
}

func (b *renderChartBuilder) title(p *renderChartPlan, c *dmlchart.Chart, sers []renderChartSer) error {
	if c.Title == nil || (c.AutoTitleDeleted != nil && c.AutoTitleDeleted.Val && c.Title.Tx == nil) {
		return nil
	}
	p.title = renderTitleText(c.Title)
	if p.title == "" {
		// An automatic title names the only series, or else is "Chart
		// Title".
		p.title = "Chart Title"
		if len(sers) == 1 && sers[0].tx != nil {
			if r := sers[0].tx.StrRef; r != nil && r.StrCache != nil && len(r.StrCache.Pt) > 0 && r.StrCache.Pt[0] != nil {
				p.title = r.StrCache.Pt[0].V
			} else if sers[0].tx.V != "" {
				p.title = sers[0].tx.V
			}
		}
	}
	// A title is 18pt by default, 1.2 times the chart's default text when
	// the chart space sets one, as Office draws it.
	var err error
	p.titleSize, p.titleColor = 18*4.0/3, p.textColor
	if p.textSized {
		p.titleSize = p.textSize * 1.2
	}
	if p.titleSize, p.titleColor, err = b.text(c.Title.TxPr, p.titleSize, p.titleColor); err != nil {
		return err
	}
	if c.Title.Tx != nil && c.Title.Tx.Rich != nil {
		if p.titleSize, p.titleColor, err = b.text(c.Title.Tx.Rich, p.titleSize, p.titleColor); err != nil {
			return err
		}
	}
	return nil
}

// spec writes the plan as a Vega specification w by h CSS pixels.
func (p *renderChartPlan) spec(w, h float64) ([]byte, error) {
	type obj = map[string]any
	n := len(p.categories)
	var rows []obj
	for s, ser := range p.series {
		for i := 0; i < n; i++ {
			var v any
			if i < len(ser.values) && ser.values[i] != nil {
				v = *ser.values[i]
			}
			row := obj{"c": i, "s": s, "v": v}
			if p.kind == "scatter" || p.kind == "bubble" {
				if i >= len(ser.xs) || ser.values[i] == nil {
					continue
				}
				row["x"] = *ser.xs[i]
				if p.kind == "bubble" && i < len(ser.sizes) && ser.sizes[i] != nil {
					row["z"] = *ser.sizes[i]
				}
			}
			if c, ok := ser.pointColors[i]; ok {
				row["fill"] = c
			} else {
				row["fill"] = ser.color
			}
			rows = append(rows, row)
		}
	}
	names := make([]string, len(p.series))
	fills := make([]string, len(p.series))
	lineColors := make([]string, len(p.series))
	lineWidths := make([]float64, len(p.series))
	markers := make([]bool, len(p.series))
	labelled := make([]bool, len(p.series))
	for i, s := range p.series {
		names[i], fills[i], markers[i], labelled[i] = s.name, s.color, s.markers, s.labels
		lineColors[i], lineWidths[i] = s.line, round2(s.lineWidth)
		if s.line == "" {
			lineColors[i], lineWidths[i] = "transparent", 0
		}
		if p.kind == "line" && s.line != "" {
			fills[i] = s.line
		}
	}
	text := obj{"font": p.font, "fontSize": round2(p.textSize), "fill": p.textColor}
	// label is an axis in the style st: its line and tick marks (Office's
	// ticks point out) and its text.
	label := func(st renderAxisStyle, extra obj) obj {
		tick := 5
		if st.tick == "in" {
			tick = -5
		}
		o := obj{"labelFont": p.font, "labelFontSize": round2(st.size), "labelColor": st.textColor, "titleFont": p.font, "titleFontSize": round2(st.size * 1.1), "titleColor": st.textColor, "titleFontWeight": "normal",
			"domain": st.shown && !st.noLine, "domainColor": st.color, "ticks": st.shown && st.tick != "none", "tickColor": st.color, "tickSize": tick}
		for k, v := range extra {
			o[k] = v
		}
		return o
	}
	spec := obj{
		"$schema":  "https://vega.github.io/schema/vega/v5.json",
		"width":    round2(w),
		"height":   round2(h),
		"padding":  8,
		"autosize": obj{"type": "fit", "contains": "padding"},
		"config": obj{"text": text, "title": obj{"font": p.font, "fontWeight": "normal", "color": p.textColor},
			// Office centres a legend along the side it sits on.
			"legend": obj{"layout": obj{"left": obj{"anchor": "middle"}, "right": obj{"anchor": "middle"}, "top": obj{"anchor": "middle"}, "bottom": obj{"anchor": "middle"}}}},
		"signals": []obj{{"name": "cats", "value": p.categories}, {"name": "names", "value": names}, {"name": "lineColors", "value": lineColors},
			{"name": "lineWidths", "value": lineWidths}, {"name": "markers", "value": markers}, {"name": "labelled", "value": labelled}},
	}
	if p.background != "" {
		spec["background"] = p.background
	}
	if p.title != "" {
		spec["title"] = obj{"text": p.title, "fontSize": round2(p.titleSize), "color": p.titleColor, "anchor": "middle", "offset": 6}
	}
	data := []obj{{"name": "table", "values": rows}}
	var scales, axes, marks, legends []obj
	var xSteps, ySteps []float64
	color := obj{"name": "color", "type": "ordinal", "domain": obj{"signal": "sequence(length(names))"}, "range": fills}
	if p.legend != "" {
		orient := p.legend
		lg := obj{"orient": orient, "labelFont": p.font, "labelFontSize": round2(p.legendSize), "labelColor": p.legendColor, "symbolType": "square",
			"encode": obj{"labels": obj{"update": obj{"text": obj{"signal": "names[datum.value]"}}}}}
		switch {
		case p.kind == "line" || (p.kind == "radar" && !p.radarFill):
			lg["symbolType"] = "stroke"
		case p.kind == "scatter" || p.kind == "bubble":
			lg["symbolType"] = "circle"
		}
		switch {
		case p.kind == "pie":
			lg["fill"] = "pieColor"
			lg["encode"] = obj{"labels": obj{"update": obj{"text": obj{"signal": "cats[datum.value]"}}}}
		case p.kind == "line" || (p.kind == "radar" && !p.radarFill):
			lg["stroke"] = "color"
		default:
			lg["fill"] = "color"
		}
		if orient == "bottom" || orient == "top" {
			lg["direction"] = "horizontal"
		}
		if p.kind == "bar" && p.horizontal && (orient == "right" || orient == "left") {
			// A bar chart's first series sits lowest, and a legend beside it
			// lists the series in the order they stack: the last on top.
			values := make([]int, len(p.series))
			for i := range values {
				values[i] = len(values) - 1 - i
			}
			lg["values"] = values
		}
		// A surface's legend would name its bands; it is left out.
		if p.kind != "surface" {
			legends = append(legends, lg)
		}
	}
	valueField := "v"
	switch p.kind {
	case "pie":
		data = append(data, obj{"name": "slices", "source": "table", "transform": []obj{
			{"type": "filter", "expr": "datum.v != null && datum.v > 0"},
			{"type": "pie", "field": "v", "startAngle": round2(p.firstAngle * math.Pi / 180), "endAngle": round2(p.firstAngle*math.Pi/180 + 2*math.Pi), "sort": false},
		}})
		pieFills := make([]string, n)
		if len(p.series) > 0 {
			for i := range pieFills {
				pieFills[i] = p.series[0].color
				if c, ok := p.series[0].pointColors[i]; ok {
					pieFills[i] = c
				}
			}
		}
		scales = append(scales, obj{"name": "pieColor", "type": "ordinal", "domain": obj{"signal": "sequence(length(cats))"}, "range": pieFills})
		if p.ofPie != nil && len(p.series) > 0 {
			d, m := p.ofPieSpec(pieFills)
			data, marks = append(data, d...), append(marks, m...)
			break
		}
		radius := obj{"signal": "min(width, height) / 2"}
		sc, sw := p.sliceStroke()
		arc := obj{"type": "arc", "from": obj{"data": "slices"}, "encode": obj{"enter": obj{
			"x": obj{"signal": "width / 2"}, "y": obj{"signal": "height / 2"},
			"startAngle": obj{"field": "startAngle"}, "endAngle": obj{"field": "endAngle"},
			"outerRadius": radius, "fill": obj{"field": "fill"}, "stroke": obj{"value": sc}, "strokeWidth": obj{"value": sw},
		}}}
		if p.hole > 0 {
			arc["encode"].(obj)["enter"].(obj)["innerRadius"] = obj{"signal": fmt.Sprintf("min(width, height) / 2 * %s", strconv.FormatFloat(p.hole/100, 'f', 4, 64))}
		}
		marks = append(marks, arc)
		if len(p.series) > 0 && p.series[0].labels {
			marks = append(marks, obj{"type": "text", "from": obj{"data": "slices"}, "encode": obj{"enter": obj{
				"x": obj{"signal": "width / 2"}, "y": obj{"signal": "height / 2"},
				"radius": obj{"signal": fmt.Sprintf("min(width, height) / 2 * %s", strconv.FormatFloat(0.5+p.hole/200, 'f', 4, 64))},
				"theta":  obj{"signal": "(datum.startAngle + datum.endAngle) / 2"}, "align": obj{"value": "center"}, "baseline": obj{"value": "middle"},
				"text": obj{"signal": "format(datum.v, '~f')"}, "fill": obj{"value": p.labelColor},
			}}})
		}
	case "radar":
		d, sc, m := p.radarSpec(rows)
		data, scales, marks = append(data, d...), append(scales, sc...), append(marks, m...)
	case "surface":
		d, sc, m, lg := p.surfaceSpec(p.bandColors)
		data, scales, marks, legends = append(data, d...), append(scales, sc...), append(marks, m...), append(legends, lg...)
	case "scatter", "bubble":
		xs := obj{"name": "x", "type": "linear", "domain": obj{"data": "table", "field": "x"}, "range": "width", "nice": obj{"signal": "max(2, round(width / 80))"}, "zero": true}
		ys := obj{"name": "y", "type": "linear", "domain": obj{"data": "table", "field": "v"}, "range": "height", "nice": p.ticks(), "zero": true}
		if lo, hi, ok := p.scatterRange(false); ok {
			as := renderAutoScale(lo, hi, renderAxisHints{p.xMin, p.xMax, p.xMajor}, p.plotLength(w, h, false), p.cat.size*renderHorizGap, false)
			xs["domainMin"], xs["domainMax"], xs["nice"], xs["zero"] = as.min, as.max, false, false
			xSteps = as.ticks
		}
		if lo, hi, ok := p.scatterRange(true); ok {
			as := renderAutoScale(lo, hi, renderAxisHints{p.valMin, p.valMax, p.valMajor}, p.plotLength(w, h, true), p.val.size*renderVertGap, false)
			ys["domainMin"], ys["domainMax"], ys["nice"], ys["zero"] = as.min, as.max, false, false
			ySteps = as.ticks
		}
		scales = append(scales, xs, ys)
		if p.kind == "bubble" {
			d, sc, m := p.bubbleSpec(rows)
			data, scales, marks = append(data, d...), append(scales, sc...), append(marks, m...)
		} else {
			marks = append(marks, p.seriesMarks("x", "y", "v", false)...)
		}
	default:
		stacked := p.grouping == "stacked" || p.grouping == "percentStacked"
		if stacked {
			offset := "zero"
			if p.grouping == "percentStacked" {
				offset = "normalize"
			}
			data[0]["transform"] = []obj{
				{"type": "formula", "as": "sv", "expr": "datum.v == null ? 0 : datum.v"},
				{"type": "stack", "groupby": []string{"c"}, "field": "sv", "sort": obj{"field": "s"}, "offset": offset},
			}
			valueField = "y1"
		}
		bandRange, valueRange := any("width"), any("height")
		if p.horizontal {
			// The first category sits at the bottom.
			bandRange, valueRange = []any{obj{"signal": "height"}, 0}, "width"
		}
		if p.reverseCats {
			if p.horizontal {
				bandRange = "height"
			} else {
				bandRange = []any{obj{"signal": "width"}, 0}
			}
		}
		catScale := obj{"name": "cat", "type": "band", "domain": obj{"signal": "sequence(length(cats))"}, "range": bandRange}
		if p.kind == "bar" {
			// gapWidth is the gap between categories as a share of a bar.
			k := float64(len(p.series))
			if stacked {
				k = 1
			}
			ov := p.overlap / 100
			group := k - (k-1)*ov
			catScale["paddingInner"] = round4(p.gapWidth / 100 / (group + p.gapWidth/100))
			catScale["paddingOuter"] = round4(p.gapWidth / 100 / (group + p.gapWidth/100) / 2)
		} else if p.midCat {
			// Points sit on the category boundaries, the first and last at
			// the plot's edges.
			catScale["type"], catScale["padding"] = "point", 0
		} else {
			// Points sit in the middle of their categories.
			catScale["paddingInner"], catScale["paddingOuter"] = 0, 0
		}
		valScale := obj{"name": "val", "type": "linear", "domain": obj{"data": "table", "field": valueField}, "range": valueRange, "nice": p.ticks(), "zero": true}
		if stacked {
			valScale["domain"] = obj{"fields": []obj{{"data": "table", "field": "y0"}, {"data": "table", "field": "y1"}}}
		}
		valLen, gap := p.plotLength(w, h, !p.horizontal), p.val.size*renderVertGap
		if p.horizontal {
			gap = p.val.size * renderHorizGap
		}
		var valSteps []float64
		if lo, hi, ok := p.valueRange(); ok {
			as := renderAutoScale(lo, hi, renderAxisHints{p.valMin, p.valMax, p.valMajor}, valLen, gap, p.grouping == "percentStacked")
			valScale["domainMin"], valScale["domainMax"], valScale["nice"], valScale["zero"] = as.min, as.max, false, false
			valSteps = as.ticks
		} else {
			if p.valMin != nil {
				valScale["domainMin"], valScale["nice"] = *p.valMin, false
			}
			if p.valMax != nil {
				valScale["domainMax"], valScale["nice"] = *p.valMax, false
			}
		}
		scales = append(scales, catScale, valScale)
		axisScale := "cat"
		if p.kind == "bar" {
			// The axis' tick marks fall between categories, evenly: on the
			// bands the bars' gaps would leave out of step.
			axisScale = "catAxis"
			scales = append(scales, obj{"name": axisScale, "type": "band", "domain": catScale["domain"], "range": bandRange, "padding": 0})
		}
		if p.cat.shown {
			orient := "bottom"
			if p.horizontal {
				orient = "left"
			}
			ax := label(p.cat, obj{"orient": orient, "scale": axisScale, "labelPadding": 4, "labelOverlap": false, "labels": p.catAxis,
				"encode": obj{"labels": obj{"update": obj{"text": obj{"signal": "cats[datum.value]"}}}}})
			if catScale["type"] == "band" {
				// Tick marks lie between categories.
				ax["tickBand"] = "extent"
			}
			if p.catTitle != "" {
				ax["title"] = p.catTitle
			}
			axes = append(axes, ax)
		}
		if p.val.shown || p.grid {
			orient := "left"
			if p.horizontal {
				orient = "bottom"
			}
			ax := label(p.val, obj{"orient": orient, "scale": "val", "grid": p.grid, "gridColor": p.gridColor, "labelPadding": 6, "labels": p.valAxis && p.val.shown})
			p.valueTicks(ax, valSteps)
			if p.valTitle != "" && p.valAxis {
				ax["title"] = p.valTitle
			}
			axes = append(axes, ax)
		}
		if p.hiLow != "" || p.upDown != nil {
			d, m := p.stockSpec()
			data, marks = append(data, d...), append(marks, m...)
		}
		marks = append(marks, p.categoryMarks(stacked)...)
	}
	if p.kind == "scatter" || p.kind == "bubble" {
		if p.cat.shown {
			ax := label(p.cat, obj{"orient": "bottom", "scale": "x", "grid": false, "labelPadding": 4, "labels": p.catAxis})
			if p.catTitle != "" {
				ax["title"] = p.catTitle
			}
			p.valueTicks(ax, xSteps)
			axes = append(axes, ax)
		}
		if p.val.shown || p.grid {
			ax := label(p.val, obj{"orient": "left", "scale": "y", "grid": p.grid, "gridColor": p.gridColor, "labels": p.valAxis && p.val.shown})
			p.valueTicks(ax, ySteps)
			axes = append(axes, ax)
		}
	}
	scales = append(scales, color)
	if p.anyLabels() && p.kind != "pie" && p.kind != "radar" && p.kind != "surface" {
		data = append(data, obj{"name": "labelled", "source": "table", "transform": []obj{{"type": "filter", "expr": "labelled[datum.s] && datum.v != null"}}})
	}
	spec["data"] = data
	spec["scales"] = scales
	if len(axes) > 0 {
		spec["axes"] = axes
	}
	if len(legends) > 0 {
		spec["legends"] = legends
	}
	spec["marks"] = marks
	return json.Marshal(spec)
}

// categoryMarks draws bars, lines or areas over the cat and val scales.
func (p *renderChartPlan) categoryMarks(stacked bool) []map[string]any {
	type obj = map[string]any
	pos, size, val := "x", "width", "y"
	if p.horizontal {
		pos, size, val = "y", "height", "x"
	}
	low, high := val, val+"2"
	var marks []obj
	switch p.kind {
	case "bar":
		enter := obj{"fill": obj{"field": "fill"}}
		if stacked {
			enter[pos] = obj{"scale": "cat", "field": "c"}
			enter[size] = obj{"scale": "cat", "band": 1}
			enter[low] = obj{"scale": "val", "field": "y0"}
			enter[high] = obj{"scale": "val", "field": "y1"}
			marks = append(marks, obj{"type": "rect", "from": obj{"data": "table"}, "encode": obj{"enter": enter}})
		} else {
			// Each category holds its series side by side, overlapping by
			// the overlap's share of a bar.
			k := float64(len(p.series))
			ov := p.overlap / 100
			group := k - (k-1)*ov
			step := (1 - ov) / group
			enter[pos] = obj{"scale": "cat", "field": "c", "offset": obj{"signal": fmt.Sprintf("bandwidth('cat') * %s * datum.s", strconv.FormatFloat(step, 'f', 6, 64))}}
			if p.horizontal {
				// The first series sits lowest, nearest the axis.
				enter[pos] = obj{"scale": "cat", "field": "c", "offset": obj{"signal": fmt.Sprintf("bandwidth('cat') * %s * (%d - datum.s)", strconv.FormatFloat(step, 'f', 6, 64), len(p.series)-1)}}
			}
			enter[size] = obj{"signal": fmt.Sprintf("bandwidth('cat') / %s", strconv.FormatFloat(group, 'f', 6, 64))}
			enter[low] = obj{"scale": "val", "value": 0}
			enter[high] = obj{"scale": "val", "field": "v"}
			marks = append(marks, obj{"type": "rect", "from": obj{"data": "table"}, "encode": obj{"enter": enter, "update": obj{"opacity": obj{"signal": "datum.v == null ? 0 : 1"}}}})
		}
		// Bars draw their series' outlines.
		enter["stroke"] = obj{"signal": "lineColors[datum.s]"}
		enter["strokeWidth"] = obj{"signal": "lineWidths[datum.s]"}
	default:
		field := "v"
		if stacked {
			field = "y1"
		}
		marks = append(marks, p.seriesMarks("cat", "val", field, stacked)...)
	}
	if p.anyLabels() {
		field := "v"
		if stacked {
			field = "y1"
		}
		at := obj{"scale": "cat", "field": "c", "band": 0.5}
		enter := obj{"text": obj{"signal": "datum.v == null ? '' : format(datum.v, '" + p.labelFormat() + "')"}, "align": obj{"value": "center"}, "baseline": obj{"value": "bottom"}, "dy": obj{"value": -2}, "fill": obj{"value": p.labelColor}}
		if p.horizontal {
			enter["align"], enter["baseline"], enter["dx"] = obj{"value": "left"}, obj{"value": "middle"}, obj{"value": 3}
			delete(enter, "dy")
		}
		enter[pos] = at
		enter[val] = obj{"scale": "val", "field": field}
		marks = append(marks, obj{"type": "text", "from": obj{"data": "labelled"}, "encode": obj{"enter": enter}})
	}
	return marks
}

func (p *renderChartPlan) labelFormat() string {
	if p.valFormat != "" && p.grouping != "percentStacked" {
		return p.valFormat
	}
	return "~f"
}

func (p *renderChartPlan) anyLabels() bool {
	for _, s := range p.series {
		if s.labels {
			return true
		}
	}
	return false
}

// seriesMarks draws each series as a line, area or markers over the named
// scales, faceted by series.
func (p *renderChartPlan) seriesMarks(xs, ys, field string, stacked bool) []map[string]any {
	type obj = map[string]any
	x := obj{"scale": xs, "field": "c", "band": 0.5}
	if xs == "x" {
		x = obj{"scale": "x", "field": "x"}
	}
	y := obj{"scale": ys, "field": field}
	var inner []obj
	if p.horizontal {
		x, y = obj{"scale": ys, "field": field}, obj{"scale": xs, "field": "c", "band": 0.5}
	}
	switch p.kind {
	case "area":
		enter := obj{"x": x, "y": y, "fill": obj{"field": "fill"}, "fillOpacity": obj{"value": 1}, "defined": obj{"signal": "datum.v != null || " + strconv.FormatBool(stacked)}}
		if stacked {
			enter["y2"] = obj{"scale": ys, "field": "y0"}
		} else {
			enter["y2"] = obj{"scale": ys, "value": 0}
		}
		inner = append(inner, obj{"type": "area", "from": obj{"data": "series"}, "encode": obj{"enter": enter}})
	default:
		inner = append(inner, obj{"type": "line", "from": obj{"data": "series"}, "encode": obj{"enter": obj{
			"x": x, "y": y, "stroke": obj{"signal": "lineColors[parent.s]"}, "strokeWidth": obj{"signal": "lineWidths[parent.s]"},
			"defined": obj{"signal": "datum.v != null"}, "strokeCap": obj{"value": "round"}, "strokeJoin": obj{"value": "round"},
		}}})
		inner = append(inner, obj{"type": "symbol", "from": obj{"data": "series"}, "encode": obj{"enter": obj{
			"x": x, "y": y, "fill": obj{"field": "fill"}, "size": obj{"value": 30}, "shape": obj{"value": "circle"},
			"opacity": obj{"signal": "markers[parent.s] && datum.v != null ? 1 : 0"},
		}}})
	}
	return []obj{{
		"type":  "group",
		"from":  obj{"facet": obj{"name": "series", "data": "table", "groupby": "s"}},
		"marks": inner,
	}}
}

// sliceStroke is the outline of a pie's slices: the series' own, and else
// none, as Office draws slices of a series without one.
func (p *renderChartPlan) sliceStroke() (string, float64) {
	if len(p.series) > 0 && p.series[0].line != "" {
		return p.series[0].line, round2(p.series[0].lineWidth)
	}
	return "transparent", 0
}

// valueRange is the range of the values a category chart's value axis must
// span: a stacked chart's totals, with zero, or else its values.
func (p *renderChartPlan) valueRange() (lo, hi float64, ok bool) {
	if p.grouping == "stacked" || p.grouping == "percentStacked" {
		lo, hi = 0, 0
		for c := range p.categories {
			pos, neg := 0.0, 0.0
			for _, s := range p.series {
				if c < len(s.values) && s.values[c] != nil {
					if v := *s.values[c]; v >= 0 {
						pos += v
					} else {
						neg += v
					}
				}
			}
			lo, hi = math.Min(lo, neg), math.Max(hi, pos)
		}
		return lo, hi, true
	}
	return p.scatterRange(true)
}

// scatterRange is the range of the Y values, or the X values of a scatter or
// bubble chart.
func (p *renderChartPlan) scatterRange(y bool) (lo, hi float64, ok bool) {
	for _, s := range p.series {
		vals := s.values
		if !y {
			vals = s.xs
		}
		for i, v := range vals {
			if v == nil || (!y && (i >= len(s.values) || s.values[i] == nil)) {
				continue
			}
			if !ok {
				lo, hi, ok = *v, *v, true
			}
			lo, hi = math.Min(lo, *v), math.Max(hi, *v)
		}
	}
	return lo, hi, ok
}

// plotLength estimates the pixels a value axis runs along in a w by h chart:
// the frame less its padding, title, legend and the other axis' labels. The
// vertical axis is the one that runs up.
func (p *renderChartPlan) plotLength(w, h float64, vertical bool) float64 {
	const padding = 16
	longest := func(names []string) float64 {
		n := 0
		for _, s := range names {
			n = max(n, len([]rune(s)))
		}
		return float64(n)
	}
	if vertical {
		l := h - padding
		if p.title != "" {
			l -= p.titleSize*1.2 + 6
		}
		if p.legend == "top" || p.legend == "bottom" {
			l -= p.legendSize*1.2 + 18
		}
		if p.cat.shown && p.catAxis {
			l -= p.cat.size*1.2 + 4
		}
		if p.catTitle != "" {
			l -= p.cat.size*1.3 + 6
		}
		return math.Max(l, 40)
	}
	l := w - padding
	if p.legend == "left" || p.legend == "right" {
		names := make([]string, len(p.series))
		for i, s := range p.series {
			names[i] = s.name
		}
		if p.kind == "pie" {
			names = p.categories
		}
		l -= math.Min(longest(names), 22)*0.55*p.legendSize + 20 + 18
	}
	// The labels of the axis the value axis meets.
	if p.horizontal {
		l -= math.Min(longest(p.categories), 22)*0.55*p.cat.size + 8
	} else {
		l -= 3*0.55*p.val.size + 10
	}
	if p.valTitle != "" || (p.horizontal && p.catTitle != "") {
		l -= p.val.size*1.3 + 6
	}
	return math.Max(l, 40)
}

// valueTicks sets the steps of a value axis: at the given values, or else
// about one per 60 pixels.
func (p *renderChartPlan) valueTicks(ax map[string]any, steps []float64) {
	if steps == nil {
		ax["tickCount"] = p.ticks()
		if p.valFormat != "" {
			ax["format"] = p.valFormat
		}
		return
	}
	ax["values"] = steps
	ax["format"] = "~f"
	if p.valFormat != "" && ax["scale"] != "x" {
		ax["format"] = p.valFormat
	}
}

// ticks is how many value-axis steps the plot's length suits, about one
// per 60 pixels, which the scale rounds its domain to.
func (p *renderChartPlan) ticks() map[string]any {
	length := "height"
	if p.horizontal {
		length = "width"
	}
	return map[string]any{"signal": "max(2, round(" + length + " / 60))"}
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }
func round4(v float64) float64 { return math.Round(v*10000) / 10000 }
