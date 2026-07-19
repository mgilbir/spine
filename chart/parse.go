package chart

import (
	"encoding/xml"
	"fmt"
	"strconv"

	"github.com/mgilbir/spine/common/dml"
	dmlchart "github.com/mgilbir/spine/common/dml/chart"
)

// Parse reads a DrawingML chart.xml part (c:chartSpace) into a Chart: its type,
// title, categories, and series names and values (recovered from the cached
// values, falling back to literal data). It is the reader Phase B's Charts()
// integrations build on.
func Parse(chartXML []byte) (*Chart, error) {
	var cs dmlchart.ChartSpace
	if err := xml.Unmarshal(chartXML, &cs); err != nil {
		return nil, fmt.Errorf("chart: parse chartSpace: %w", err)
	}
	if cs.Chart == nil || cs.Chart.PlotArea == nil {
		return nil, fmt.Errorf("chart: chartSpace has no plotArea")
	}
	pa := cs.Chart.PlotArea

	var c *Chart
	switch {
	case len(pa.BarChart) > 0:
		c = parseBar(pa.BarChart[0])
	case len(pa.LineChart) > 0:
		c = parseLine(pa.LineChart[0])
	case len(pa.PieChart) > 0:
		c = parsePie(pa.PieChart[0])
	case len(pa.DoughnutChart) > 0:
		c = parseDoughnut(pa.DoughnutChart[0])
	case len(pa.RadarChart) > 0:
		c = parseRadar(pa.RadarChart[0])
	case len(pa.AreaChart) > 0:
		c = parseArea(pa.AreaChart[0])
	case len(pa.ScatterChart) > 0:
		c = parseScatter(pa.ScatterChart[0])
	default:
		return nil, fmt.Errorf("chart: unsupported or empty chart type")
	}

	if cs.Chart.Title != nil {
		c.title = titleText(cs.Chart.Title)
	}
	if cs.Chart.Legend != nil {
		c.showLegend = true
		if lp := cs.Chart.Legend.LegendPos; lp != nil && lp.Val != "" {
			c.legendPos = LegendPosition(lp.Val)
		}
	} else {
		c.showLegend = false
	}
	c.catAxisName, c.valAxisName = axisTitles(pa)
	if ref := firstCatRef(pa); ref != "" {
		if sheet := sheetOf(ref); sheet != "" {
			c.DataRef = sheet
		}
	}
	return c, nil
}

func parseBar(bc *dmlchart.BarChart) *Chart {
	kind := KindColumn
	if bc.BarDir != nil && bc.BarDir.Val == "bar" {
		kind = KindBar
	}
	c := newChart(kind)
	c.showLegend = false
	c.dataLabels = dLblsShowVal(bc.DLbls)
	for i, s := range bc.Ser {
		if i == 0 {
			c.categories = categoriesFrom(s.Cat)
		}
		c.series = append(c.series, &Series{
			Name:   seriesName(s.Tx),
			Values: numbersFrom(s.Val),
			Color:  seriesColor(s.SpPr),
		})
	}
	return c
}

func parseLine(lc *dmlchart.LineChart) *Chart {
	c := newChart(KindLine)
	c.showLegend = false
	c.dataLabels = dLblsShowVal(lc.DLbls)
	for i, s := range lc.Ser {
		if i == 0 {
			c.categories = categoriesFrom(s.Cat)
		}
		c.series = append(c.series, &Series{
			Name:   seriesName(s.Tx),
			Values: numbersFrom(s.Val),
			Color:  seriesColor(s.SpPr),
		})
	}
	return c
}

func parseArea(ac *dmlchart.AreaChart) *Chart {
	c := newChart(KindArea)
	c.showLegend = false
	c.dataLabels = dLblsShowVal(ac.DLbls)
	for i, s := range ac.Ser {
		if i == 0 {
			c.categories = categoriesFrom(s.Cat)
		}
		c.series = append(c.series, &Series{
			Name:   seriesName(s.Tx),
			Values: numbersFrom(s.Val),
			Color:  seriesColor(s.SpPr),
		})
	}
	return c
}

func parsePie(pc *dmlchart.PieChart) *Chart {
	c := newChart(KindPie)
	c.showLegend = false
	c.dataLabels = dLblsShowVal(pc.DLbls)
	for i, s := range pc.Ser {
		if i == 0 {
			c.categories = categoriesFrom(s.Cat)
		}
		c.series = append(c.series, &Series{
			Name:   seriesName(s.Tx),
			Values: numbersFrom(s.Val),
			Color:  seriesColor(s.SpPr),
		})
	}
	return c
}

func parseDoughnut(dc *dmlchart.DoughnutChart) *Chart {
	c := newChart(KindDoughnut)
	c.showLegend = false
	c.dataLabels = dLblsShowVal(dc.DLbls)
	for i, s := range dc.Ser {
		if i == 0 {
			c.categories = categoriesFrom(s.Cat)
		}
		c.series = append(c.series, &Series{
			Name:   seriesName(s.Tx),
			Values: numbersFrom(s.Val),
			Color:  seriesColor(s.SpPr),
		})
	}
	return c
}

func parseRadar(rc *dmlchart.RadarChart) *Chart {
	c := newChart(KindRadar)
	c.showLegend = false
	c.dataLabels = dLblsShowVal(rc.DLbls)
	for i, s := range rc.Ser {
		if i == 0 {
			c.categories = categoriesFrom(s.Cat)
		}
		c.series = append(c.series, &Series{
			Name:   seriesName(s.Tx),
			Values: numbersFrom(s.Val),
			Color:  seriesColor(s.SpPr),
		})
	}
	return c
}

func parseScatter(sc *dmlchart.ScatterChart) *Chart {
	c := newChart(KindScatter)
	c.showLegend = false
	c.dataLabels = dLblsShowVal(sc.DLbls)
	for _, s := range sc.Ser {
		c.series = append(c.series, &Series{
			Name:    seriesName(s.Tx),
			XValues: numbersFromAx(s.XVal),
			Values:  numbersFrom(s.YVal),
			Color:   seriesColor(s.SpPr),
		})
	}
	return c
}

// dLblsShowVal reports whether a chart-type group's data labels turn on value
// display (c:dLbls/c:showVal val="1").
func dLblsShowVal(d *dmlchart.DataLabels) bool {
	return d != nil && d.ShowVal != nil && d.ShowVal.Val
}

// seriesColor recovers a series' solid RGB fill from its shape properties, or
// "" when it has no solid srgbClr fill.
func seriesColor(spPr *dml.SpPr) string {
	if spPr != nil && spPr.SolidFill != nil && spPr.SolidFill.SrgbClr != nil {
		return spPr.SolidFill.SrgbClr.Val
	}
	return ""
}

// seriesName recovers a series name from its tx (cached strRef or literal v).
func seriesName(tx *dmlchart.SerTx) string {
	if tx == nil {
		return ""
	}
	if tx.V != "" {
		return tx.V
	}
	if tx.StrRef != nil && tx.StrRef.StrCache != nil {
		if pts := tx.StrRef.StrCache.Pt; len(pts) > 0 {
			return pts[0].V
		}
	}
	return ""
}

// categoriesFrom recovers category labels from a c:cat source, preferring the
// string cache/literal, falling back to numeric points formatted as strings.
func categoriesFrom(src *dmlchart.AxDataSource) []string {
	if src == nil {
		return nil
	}
	switch {
	case src.StrRef != nil && src.StrRef.StrCache != nil:
		return strPoints(src.StrRef.StrCache)
	case src.StrLit != nil:
		return strPoints(src.StrLit)
	case src.NumRef != nil && src.NumRef.NumCache != nil:
		return numPointsAsStrings(src.NumRef.NumCache)
	case src.NumLit != nil:
		return numPointsAsStrings(src.NumLit)
	}
	return nil
}

func strPoints(sd *dmlchart.StrData) []string {
	if sd == nil {
		return nil
	}
	out := orderedStrings(len(sd.Pt))
	for _, p := range sd.Pt {
		if int(p.Idx) < len(out) {
			out[p.Idx] = p.V
		}
	}
	return out
}

func numPointsAsStrings(nd *dmlchart.NumData) []string {
	if nd == nil {
		return nil
	}
	out := orderedStrings(len(nd.Pt))
	for _, p := range nd.Pt {
		if int(p.Idx) < len(out) {
			out[p.Idx] = p.V
		}
	}
	return out
}

// numbersFrom recovers numeric values from a c:val / c:yVal source.
func numbersFrom(src *dmlchart.NumDataSource) []float64 {
	if src == nil {
		return nil
	}
	switch {
	case src.NumRef != nil && src.NumRef.NumCache != nil:
		return numPoints(src.NumRef.NumCache)
	case src.NumLit != nil:
		return numPoints(src.NumLit)
	}
	return nil
}

// numbersFromAx recovers numeric values from a c:xVal (AxDataSource) source.
func numbersFromAx(src *dmlchart.AxDataSource) []float64 {
	if src == nil {
		return nil
	}
	switch {
	case src.NumRef != nil && src.NumRef.NumCache != nil:
		return numPoints(src.NumRef.NumCache)
	case src.NumLit != nil:
		return numPoints(src.NumLit)
	}
	return nil
}

func numPoints(nd *dmlchart.NumData) []float64 {
	if nd == nil {
		return nil
	}
	out := make([]float64, len(nd.Pt))
	for i, p := range nd.Pt {
		idx := int(p.Idx)
		if idx >= len(out) {
			idx = i
		}
		f, _ := strconv.ParseFloat(p.V, 64)
		out[idx] = f
	}
	return out
}

func orderedStrings(n int) []string { return make([]string, n) }

// titleText extracts the plain text of a chart/axis title.
func titleText(t *dmlchart.Title) string {
	if t == nil || t.Tx == nil {
		return ""
	}
	if t.Tx.Rich != nil {
		return richTextString(t.Tx.Rich)
	}
	if t.Tx.StrRef != nil && t.Tx.StrRef.StrCache != nil {
		if pts := t.Tx.StrRef.StrCache.Pt; len(pts) > 0 {
			return pts[0].V
		}
	}
	return ""
}

func richTextString(tb *dml.TxBody) string {
	var s string
	for _, p := range tb.P {
		for _, r := range p.R {
			s += r.T
		}
	}
	return s
}

// axisTitles returns the (category, value) axis titles from the plot area.
func axisTitles(pa *dmlchart.PlotArea) (cat, val string) {
	if len(pa.CatAx) > 0 && pa.CatAx[0].Title != nil {
		cat = titleText(pa.CatAx[0].Title)
	}
	if len(pa.ValAx) > 0 {
		// For scatter the first valAx is the X (category-like) axis.
		if len(pa.CatAx) == 0 && len(pa.ValAx) >= 2 {
			cat = titleText(pa.ValAx[0].Title)
			val = titleText(pa.ValAx[1].Title)
			return cat, val
		}
		if pa.ValAx[0].Title != nil {
			val = titleText(pa.ValAx[0].Title)
		}
	}
	return cat, val
}

// firstCatRef returns the first category/value formula reference found, used to
// recover the DataRef sheet name.
func firstCatRef(pa *dmlchart.PlotArea) string {
	catOf := func(src *dmlchart.AxDataSource) string {
		if src == nil {
			return ""
		}
		if src.StrRef != nil {
			return src.StrRef.F
		}
		if src.NumRef != nil {
			return src.NumRef.F
		}
		return ""
	}
	for _, bc := range pa.BarChart {
		for _, s := range bc.Ser {
			if r := catOf(s.Cat); r != "" {
				return r
			}
		}
	}
	for _, lc := range pa.LineChart {
		for _, s := range lc.Ser {
			if r := catOf(s.Cat); r != "" {
				return r
			}
		}
	}
	for _, pc := range pa.PieChart {
		for _, s := range pc.Ser {
			if r := catOf(s.Cat); r != "" {
				return r
			}
		}
	}
	for _, dc := range pa.DoughnutChart {
		for _, s := range dc.Ser {
			if r := catOf(s.Cat); r != "" {
				return r
			}
		}
	}
	for _, rc := range pa.RadarChart {
		for _, s := range rc.Ser {
			if r := catOf(s.Cat); r != "" {
				return r
			}
		}
	}
	for _, ac := range pa.AreaChart {
		for _, s := range ac.Ser {
			if r := catOf(s.Cat); r != "" {
				return r
			}
		}
	}
	for _, sc := range pa.ScatterChart {
		for _, s := range sc.Ser {
			if r := catOf(s.XVal); r != "" {
				return r
			}
		}
	}
	return ""
}

// sheetOf extracts the sheet name from a formula reference like
// "Sheet1!$A$2:$A$5" or "'My Sheet'!$A$1".
func sheetOf(ref string) string {
	bang := -1
	inQuote := false
	for i := 0; i < len(ref); i++ {
		switch ref[i] {
		case '\'':
			inQuote = !inQuote
		case '!':
			if !inQuote {
				bang = i
			}
		}
		if bang >= 0 {
			break
		}
	}
	if bang <= 0 {
		return ""
	}
	name := ref[:bang]
	if len(name) >= 2 && name[0] == '\'' && name[len(name)-1] == '\'' {
		name = name[1 : len(name)-1]
	}
	return name
}
