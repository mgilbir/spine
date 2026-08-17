package xlsx

import (
	xmlb "github.com/mgilbir/spine/common/xml"

	"github.com/mgilbir/spine/chart"
)

// Charts returns every chart on the sheet: those parsed from the opened file's
// drawing part and any added this session via AddChart. Each is returned as a
// parsed *chart.Chart carrying its type, title, categories, and series (names
// and values recovered from the chart's caches). The slice is nil when the
// sheet has no charts.
//
// Chartsheets are included: a chartsheet has no cell grid, so its drawing
// reference is read straight from its preserved part rather than through a
// worksheet model, and the chart it anchors is then resolved the same way a
// worksheet's is (C564).
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

// Charts returns every chart across all of the workbook's sheets — worksheets
// and chartsheets alike — in sheet order. The slice is nil when the workbook
// has no charts.
//
// Charts held by a sheet copied in with Workbook.Merge are not included: merge
// does not carry a source sheet's drawing across, so no chart part comes with
// it.
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
	if s.workbook == nil {
		return nil
	}
	rid := s.drawingRelID()
	if rid == "" {
		return nil
	}
	drawingPart := s.resolveRelTarget(s.partName, rid)
	if drawingPart == "" {
		return nil
	}
	part, ok := s.workbook.preservedParts[drawingPart]
	if !ok {
		return nil
	}
	var wsDr xdrWsDr
	if err := xmlb.Unmarshal(part.Data, &wsDr); err != nil {
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

// drawingRelID returns the relationship id of the sheet's drawing part, from
// the parsed worksheet model when there is one, otherwise from the sheet's
// preserved bytes.
//
// The fallback covers the opaque sheets — chartsheet, dialogsheet, macrosheet —
// whose parts are preserved verbatim. A chartsheet's chart is reachable today
// only because ws() happens to decode <chartsheet> into a CT_Worksheet (the
// model matches children by local name, so <drawing r:id> lands in the same
// field): the sheet is documented as never parsed as a worksheet, so relying on
// that would make every chart on a chartsheet vanish the moment the opaque
// contract is enforced. Reading the reference straight from the raw part does
// not build or mark a worksheet model, so the part still round-trips byte for
// byte either way (C564).
func (s *Sheet) drawingRelID() string {
	if ws := s.ws(); ws != nil {
		if ws.Drawing == nil {
			return ""
		}
		return ws.Drawing.RID
	}
	if s.partName == "" {
		return ""
	}
	part, ok := s.workbook.preservedParts[s.partName]
	if !ok || part == nil {
		return ""
	}
	var cs opaqueSheetDrawing
	if err := xmlb.Unmarshal(part.Data, &cs); err != nil {
		return ""
	}
	if cs.Drawing == nil {
		return ""
	}
	return cs.Drawing.RID
}

// opaqueSheetDrawing is the minimal read model for the drawing reference of a
// non-worksheet sheet part (chartsheet, dialogsheet, macrosheet): the root
// element is matched by local name and only its <drawing r:id> child decoded.
type opaqueSheetDrawing struct {
	Drawing *xdrChartRef `xml:"drawing"`
}
