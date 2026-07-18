package xlsx

import (
	"encoding/xml"

	"github.com/mgilbir/spine/chart"
)

// Charts returns every chart on the sheet: those parsed from the opened file's
// drawing part and any added this session via AddChart. Each is returned as a
// parsed *chart.Chart carrying its type, title, categories, and series (names
// and values recovered from the chart's caches). The slice is nil when the
// sheet has no charts.
func (s *Sheet) Charts() []*chart.Chart {
	var out []*chart.Chart
	out = append(out, s.openedCharts()...)
	for i := range s.charts {
		if s.charts[i].def != nil {
			out = append(out, s.charts[i].def)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// Charts returns every chart across all of the workbook's sheets, in sheet
// order. The slice is nil when the workbook has no charts.
func (w *Workbook) Charts() []*chart.Chart {
	var out []*chart.Chart
	for _, sheet := range w.sheets {
		if sheet == nil {
			continue
		}
		out = append(out, sheet.Charts()...)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// openedCharts parses the sheet's drawing part (if any), finds each chart
// graphicFrame, resolves its relationship to the chart part, and parses it.
func (s *Sheet) openedCharts() []*chart.Chart {
	if s.workbook == nil || s.worksheet == nil || s.worksheet.Drawing == nil {
		return nil
	}
	drawingPart := s.resolveRelTarget(s.partName, s.worksheet.Drawing.RID)
	if drawingPart == "" {
		return nil
	}
	part, ok := s.workbook.preservedParts[drawingPart]
	if !ok {
		return nil
	}
	var wsDr xdrWsDr
	if err := xml.Unmarshal(part.Data, &wsDr); err != nil {
		return nil
	}
	anchors := make([]xdrAnchor, 0, len(wsDr.OneCell)+len(wsDr.TwoCell)+len(wsDr.AbsAnchor))
	anchors = append(anchors, wsDr.OneCell...)
	anchors = append(anchors, wsDr.TwoCell...)
	anchors = append(anchors, wsDr.AbsAnchor...)

	var out []*chart.Chart
	for i := range anchors {
		gf := anchors[i].GraphicFrame
		if gf == nil || gf.Chart == nil || gf.Chart.RID == "" {
			continue
		}
		chartPart := s.resolveRelTarget(drawingPart, gf.Chart.RID)
		if chartPart == "" {
			continue
		}
		cp, ok := s.workbook.preservedParts[chartPart]
		if !ok {
			continue
		}
		c, err := chart.Parse(cp.Data)
		if err != nil {
			continue
		}
		out = append(out, c)
	}
	return out
}
