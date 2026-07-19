package chart

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mgilbir/spine/common/dml"
	dmlchart "github.com/mgilbir/spine/common/dml/chart"
	xmlb "github.com/mgilbir/spine/common/xml"
)

// Axis identifiers. Each chart.xml is an independent part, so fixed IDs are
// fine; they only need to be distinct within one chart.
const (
	catAxisID uint32 = 111111111
	valAxisID uint32 = 222222222
	xAxisID   uint32 = 111111111
	yAxisID   uint32 = 222222222
)

// MarshalChartXML serializes the chart to a DrawingML chart.xml part
// (c:chartSpace). Numeric and string caches are populated from the chart's
// data so it renders without a live data source; c:f formula references point
// at the DataRef sheet.
func (c *Chart) MarshalChartXML() ([]byte, error) {
	cs, err := c.buildChartSpace()
	if err != nil {
		return nil, err
	}

	b := xmlb.NewBuilder()
	b.RegisterNamespace(xmlb.NSDrawingMLChart, xmlb.PrefixDrawingMLChart)
	b.RegisterNamespace(xmlb.NSDrawingML, xmlb.PrefixDrawingML)
	b.RegisterNamespace(xmlb.NSOfficeDocumentRels, xmlb.PrefixRelationships)
	// Producers self-close empty elements (<c:layout/>) with no space.
	b.SetCollapseEmptyElements(true)
	b.SetSelfClosingSpace(false)
	b.WriteHeader()

	nsDecls := []xmlb.NSDecl{
		{Prefix: xmlb.PrefixDrawingMLChart, URI: xmlb.NSDrawingMLChart},
		{Prefix: xmlb.PrefixDrawingML, URI: xmlb.NSDrawingML},
		{Prefix: xmlb.PrefixRelationships, URI: xmlb.NSOfficeDocumentRels},
	}
	b.MarshalRoot(xmlb.NSDrawingMLChart, "chartSpace", cs, nsDecls)
	if err := b.Finish(); err != nil {
		return nil, fmt.Errorf("chart: marshal chartSpace: %w", err)
	}
	return b.Bytes(), nil
}

// buildChartSpace assembles the internal ChartSpace model from the chart.
func (c *Chart) buildChartSpace() (*dmlchart.ChartSpace, error) {
	if len(c.series) == 0 {
		return nil, fmt.Errorf("chart: at least one series is required")
	}

	plot := &dmlchart.PlotArea{Layout: &dmlchart.Layout{}}
	if err := c.buildPlotType(plot); err != nil {
		return nil, err
	}
	if c.usesAxes() {
		c.buildAxes(plot)
	}

	ch := &dmlchart.Chart{
		AutoTitleDeleted: boolElem(c.title == ""),
		PlotArea:         plot,
		PlotVisOnly:      boolElem(true),
		DispBlanksAs:     &dmlchart.DispBlanksAs{Val: "gap"},
	}
	if c.title != "" {
		ch.Title = &dmlchart.Title{
			Tx:      &dmlchart.ChartText{Rich: richText(c.title)},
			Overlay: boolElem(false),
		}
	}
	if c.showLegend {
		ch.Legend = &dmlchart.Legend{
			LegendPos: &dmlchart.LegendPos{Val: string(c.legendPos)},
			Overlay:   boolElem(false),
		}
	}

	return &dmlchart.ChartSpace{
		Date1904:       boolElem(false),
		Lang:           &dmlchart.String{Val: "en-US"},
		RoundedCorners: boolElem(false),
		Style:          &dmlchart.Style{Val: 2},
		Chart:          ch,
	}, nil
}

// buildPlotType appends the chart-type group (barChart, lineChart, ...) with
// its series to the plot area.
func (c *Chart) buildPlotType(plot *dmlchart.PlotArea) error {
	dl := c.layout()
	switch c.kind {
	case KindColumn, KindBar:
		dir := "col"
		if c.kind == KindBar {
			dir = "bar"
		}
		bc := &dmlchart.BarChart{
			BarDir:     &dmlchart.BarDir{Val: dir},
			Grouping:   &dmlchart.BarGrouping{Val: "clustered"},
			VaryColors: boolElem(false),
			DLbls:      c.groupDataLabels(),
			GapWidth:   &dmlchart.GapAmount{Val: 150},
			AxId:       axIDs(catAxisID, valAxisID),
		}
		for i, s := range c.series {
			bc.Ser = append(bc.Ser, &dmlchart.BarSer{
				Idx:              uintElem(uint32(i)),
				Order:            uintElem(uint32(i)),
				Tx:               serName(s.Name, dl.Series[i].NameRef),
				SpPr:             seriesSpPr(s.Color),
				InvertIfNegative: boolElem(false),
				Cat:              c.catSource(dl),
				Val:              numSource(s.Values, dl.Series[i].ValuesRef, c.numberFormat()),
			})
		}
		plot.BarChart = append(plot.BarChart, bc)
	case KindLine:
		lc := &dmlchart.LineChart{
			Grouping:   &dmlchart.Grouping{Val: "standard"},
			VaryColors: boolElem(false),
			DLbls:      c.groupDataLabels(),
			Marker:     boolElem(true),
			AxId:       axIDs(catAxisID, valAxisID),
		}
		for i, s := range c.series {
			lc.Ser = append(lc.Ser, &dmlchart.LineSer{
				Idx:    uintElem(uint32(i)),
				Order:  uintElem(uint32(i)),
				Tx:     serName(s.Name, dl.Series[i].NameRef),
				SpPr:   seriesSpPr(s.Color),
				Marker: &dmlchart.Marker{Symbol: &dmlchart.MarkerStyle{Val: "none"}},
				Cat:    c.catSource(dl),
				Val:    numSource(s.Values, dl.Series[i].ValuesRef, c.numberFormat()),
				Smooth: boolElem(false),
			})
		}
		plot.LineChart = append(plot.LineChart, lc)
	case KindArea:
		ac := &dmlchart.AreaChart{
			Grouping:   &dmlchart.Grouping{Val: "standard"},
			VaryColors: boolElem(false),
			DLbls:      c.groupDataLabels(),
			AxId:       axIDs(catAxisID, valAxisID),
		}
		for i, s := range c.series {
			ac.Ser = append(ac.Ser, &dmlchart.AreaSer{
				Idx:   uintElem(uint32(i)),
				Order: uintElem(uint32(i)),
				Tx:    serName(s.Name, dl.Series[i].NameRef),
				SpPr:  seriesSpPr(s.Color),
				Cat:   c.catSource(dl),
				Val:   numSource(s.Values, dl.Series[i].ValuesRef, c.numberFormat()),
			})
		}
		plot.AreaChart = append(plot.AreaChart, ac)
	case KindPie:
		pc := &dmlchart.PieChart{
			VaryColors: boolElem(true),
			DLbls:      c.groupDataLabels(),
		}
		// A pie chart plots a single series; use the first.
		s := c.series[0]
		pc.Ser = append(pc.Ser, &dmlchart.PieSer{
			Idx:   uintElem(0),
			Order: uintElem(0),
			Tx:    serName(s.Name, dl.Series[0].NameRef),
			SpPr:  seriesSpPr(s.Color),
			Cat:   c.catSource(dl),
			Val:   numSource(s.Values, dl.Series[0].ValuesRef, c.numberFormat()),
		})
		plot.PieChart = append(plot.PieChart, pc)
	case KindDoughnut:
		dc := &dmlchart.DoughnutChart{
			VaryColors: boolElem(true),
			DLbls:      c.groupDataLabels(),
			HoleSize:   &dmlchart.HoleSize{Val: defaultHoleSize},
		}
		// Like a pie, a doughnut plots a single series; use the first.
		s := c.series[0]
		dc.Ser = append(dc.Ser, &dmlchart.PieSer{
			Idx:   uintElem(0),
			Order: uintElem(0),
			Tx:    serName(s.Name, dl.Series[0].NameRef),
			SpPr:  seriesSpPr(s.Color),
			Cat:   c.catSource(dl),
			Val:   numSource(s.Values, dl.Series[0].ValuesRef, c.numberFormat()),
		})
		plot.DoughnutChart = append(plot.DoughnutChart, dc)
	case KindRadar:
		rc := &dmlchart.RadarChart{
			RadarStyle: &dmlchart.RadarStyle{Val: "marker"},
			VaryColors: boolElem(false),
			DLbls:      c.groupDataLabels(),
			AxId:       axIDs(catAxisID, valAxisID),
		}
		for i, s := range c.series {
			rc.Ser = append(rc.Ser, &dmlchart.RadarSer{
				Idx:    uintElem(uint32(i)),
				Order:  uintElem(uint32(i)),
				Tx:     serName(s.Name, dl.Series[i].NameRef),
				SpPr:   seriesSpPr(s.Color),
				Marker: &dmlchart.Marker{Symbol: &dmlchart.MarkerStyle{Val: "none"}},
				Cat:    c.catSource(dl),
				Val:    numSource(s.Values, dl.Series[i].ValuesRef, c.numberFormat()),
			})
		}
		plot.RadarChart = append(plot.RadarChart, rc)
	case KindScatter:
		sc := &dmlchart.ScatterChart{
			ScatterStyle: &dmlchart.ScatterStyle{Val: "lineMarker"},
			VaryColors:   boolElem(false),
			DLbls:        c.groupDataLabels(),
			AxId:         axIDs(xAxisID, yAxisID),
		}
		for i, s := range c.series {
			sc.Ser = append(sc.Ser, &dmlchart.ScatterSer{
				Idx:    uintElem(uint32(i)),
				Order:  uintElem(uint32(i)),
				Tx:     serName(s.Name, dl.Series[i].NameRef),
				SpPr:   seriesSpPr(s.Color),
				Marker: &dmlchart.Marker{Symbol: &dmlchart.MarkerStyle{Val: "circle"}},
				XVal:   axNumSource(s.XValues, dl.Series[i].XValuesRef, "General"),
				YVal:   numSource(s.Values, dl.Series[i].ValuesRef, c.numberFormat()),
				Smooth: boolElem(false),
			})
		}
		plot.ScatterChart = append(plot.ScatterChart, sc)
	default:
		return fmt.Errorf("chart: unsupported kind %v", c.kind)
	}
	return nil
}

// buildAxes appends the axis definitions to the plot area.
func (c *Chart) buildAxes(plot *dmlchart.PlotArea) {
	if c.kind == KindScatter {
		// Scatter uses two value axes.
		plot.ValAx = []*dmlchart.ValAx{
			c.valAx(xAxisID, yAxisID, "b", c.catAxisName),
			c.valAx(yAxisID, xAxisID, "l", c.valAxisName),
		}
		return
	}
	catPos, valPos := "b", "l"
	if c.kind == KindBar {
		// A horizontal bar chart swaps axis orientation.
		catPos, valPos = "l", "b"
	}
	plot.CatAx = []*dmlchart.CatAx{c.catAx(catPos)}
	plot.ValAx = []*dmlchart.ValAx{c.valAx(valAxisID, catAxisID, valPos, c.valAxisName)}
}

func (c *Chart) catAx(pos string) *dmlchart.CatAx {
	ax := &dmlchart.CatAx{
		AxId:          uintElem(catAxisID),
		Scaling:       &dmlchart.Scaling{Orientation: &dmlchart.Orientation{Val: "minMax"}},
		Delete:        boolElem(false),
		AxPos:         &dmlchart.AxPos{Val: pos},
		NumFmt:        &dmlchart.NumFmt{FormatCode: "General", SourceLinked: boolPtr(true)},
		MajorTickMark: &dmlchart.TickMark{Val: "out"},
		MinorTickMark: &dmlchart.TickMark{Val: "none"},
		TickLblPos:    &dmlchart.TickLblPos{Val: "nextTo"},
		CrossAx:       uintElem(valAxisID),
		Crosses:       &dmlchart.Crosses{Val: "autoZero"},
		Auto:          boolElem(true),
		LblAlgn:       &dmlchart.LblAlgn{Val: "ctr"},
		LblOffset:     &dmlchart.LblOffset{Val: 100},
		NoMultiLvlLbl: boolElem(false),
	}
	if c.catAxisName != "" {
		ax.Title = axisTitle(c.catAxisName)
	}
	return ax
}

func (c *Chart) valAx(id, crossID uint32, pos, title string) *dmlchart.ValAx {
	ax := &dmlchart.ValAx{
		AxId:           uintElem(id),
		Scaling:        &dmlchart.Scaling{Orientation: &dmlchart.Orientation{Val: "minMax"}},
		Delete:         boolElem(false),
		AxPos:          &dmlchart.AxPos{Val: pos},
		MajorGridlines: &dmlchart.ChartLines{},
		NumFmt:         &dmlchart.NumFmt{FormatCode: "General", SourceLinked: boolPtr(true)},
		MajorTickMark:  &dmlchart.TickMark{Val: "out"},
		MinorTickMark:  &dmlchart.TickMark{Val: "none"},
		TickLblPos:     &dmlchart.TickLblPos{Val: "nextTo"},
		CrossAx:        uintElem(crossID),
		Crosses:        &dmlchart.Crosses{Val: "autoZero"},
		CrossBetween:   &dmlchart.CrossBetween{Val: "between"},
	}
	if title != "" {
		ax.Title = axisTitle(title)
	}
	return ax
}

// catSource builds the shared category data source (strRef + strCache).
func (c *Chart) catSource(dl DataLayout) *dmlchart.AxDataSource {
	if len(c.categories) == 0 {
		return nil
	}
	return &dmlchart.AxDataSource{
		StrRef: &dmlchart.StrRef{
			F:        dl.CategoriesRef,
			StrCache: strData(c.categories),
		},
	}
}

// numSource builds a numeric data source (numRef + numCache) for c:val / c:yVal.
func numSource(values []float64, ref, formatCode string) *dmlchart.NumDataSource {
	return &dmlchart.NumDataSource{
		NumRef: &dmlchart.NumRef{
			F:        ref,
			NumCache: numData(values, formatCode),
		},
	}
}

// axNumSource builds a numeric axis data source (numRef + numCache) for c:xVal.
func axNumSource(values []float64, ref, formatCode string) *dmlchart.AxDataSource {
	return &dmlchart.AxDataSource{
		NumRef: &dmlchart.NumRef{
			F:        ref,
			NumCache: numData(values, formatCode),
		},
	}
}

// serName builds a series-name source (strRef with a one-point cache).
func serName(name, ref string) *dmlchart.SerTx {
	return &dmlchart.SerTx{
		StrRef: &dmlchart.StrRef{
			F:        ref,
			StrCache: strData([]string{name}),
		},
	}
}

// numData builds a c:numCache / c:numLit body from float values.
func numData(values []float64, formatCode string) *dmlchart.NumData {
	nd := &dmlchart.NumData{
		FormatCode: formatCode,
		PtCount:    uintElem(uint32(len(values))),
	}
	for i, v := range values {
		nd.Pt = append(nd.Pt, &dmlchart.NumVal{
			Idx: uint32(i),
			V:   formatFloat(v),
		})
	}
	return nd
}

// strData builds a c:strCache body from string values.
func strData(values []string) *dmlchart.StrData {
	sd := &dmlchart.StrData{PtCount: uintElem(uint32(len(values)))}
	for i, v := range values {
		sd.Pt = append(sd.Pt, &dmlchart.StrVal{Idx: uint32(i), V: v})
	}
	return sd
}

// richText builds a minimal rich-text body (a:p > a:r > a:t) for a title.
func richText(text string) *dml.TxBody {
	return &dml.TxBody{
		BodyPr:   &dml.BodyPr{},
		LstStyle: &dml.LstStyle{},
		P: []*dml.P{{
			R: []*dml.R{{T: text}},
		}},
	}
}

// axisTitle builds a CT_Title carrying rich text for an axis.
func axisTitle(text string) *dmlchart.Title {
	return &dmlchart.Title{
		Tx:      &dmlchart.ChartText{Rich: richText(text)},
		Overlay: boolElem(false),
	}
}

func axIDs(ids ...uint32) []*dmlchart.UnsignedInt {
	out := make([]*dmlchart.UnsignedInt, len(ids))
	for i, id := range ids {
		out[i] = uintElem(id)
	}
	return out
}

// groupDataLabels returns a c:dLbls that turns on value labels for every point
// in a chart-type group, or nil when the chart has data labels disabled. The
// show flags are emitted explicitly (only showVal true) so a reader does not
// fall back to its own defaults for the omitted ones.
func (c *Chart) groupDataLabels() *dmlchart.DataLabels {
	if !c.dataLabels {
		return nil
	}
	return &dmlchart.DataLabels{
		ShowLegendKey:  boolElem(false),
		ShowVal:        boolElem(true),
		ShowCatName:    boolElem(false),
		ShowSerName:    boolElem(false),
		ShowPercent:    boolElem(false),
		ShowBubbleSize: boolElem(false),
	}
}

// seriesSpPr returns a shape-properties element carrying a solid RGB fill for a
// series, or nil when hex is empty (leaving the series its automatic color).
func seriesSpPr(hex string) *dml.SpPr {
	if hex == "" {
		return nil
	}
	return &dml.SpPr{
		SolidFill: &dml.SolidFill{SrgbClr: &dml.SrgbClr{Val: hex}},
	}
}

// normalizeHexColor trims a leading '#' from an RGB hex string and upper-cases
// it. It does not validate length: callers pass a 6-digit RGB value.
func normalizeHexColor(hex string) string {
	hex = strings.TrimPrefix(hex, "#")
	return strings.ToUpper(hex)
}

func boolElem(v bool) *dmlchart.Boolean { return &dmlchart.Boolean{Val: v} }
func uintElem(v uint32) *dmlchart.UnsignedInt {
	return &dmlchart.UnsignedInt{Val: v}
}
func boolPtr(v bool) *bool { return &v }

// formatFloat renders a float for a cache value: the shortest representation
// that round-trips (integers as "18", fractions as "2.5").
func formatFloat(v float64) string {
	return strconv.FormatFloat(v, 'g', -1, 64)
}
