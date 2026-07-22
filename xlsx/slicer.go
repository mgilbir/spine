package xlsx

import (
	"github.com/mgilbir/spine/opc"
	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// Slicer is a read view of a pivot slicer: an on-sheet control that filters one
// or more pivot tables by the distinct values of a single pivot field. A Slicer
// returned by Sheet.Slicers or Workbook.Slicers reflects the slicer as stored
// in the opened workbook; its accessors are read-only.
//
// Creating slicers is not yet supported (see the package documentation); an
// opened workbook's slicers, slicer caches and their worksheet/workbook
// extension references round-trip byte-for-byte.
type Slicer struct {
	sheet    *Sheet
	slicer   oxml.CT_Slicer
	cacheDef *oxml.CT_SlicerCacheDefinition
}

// Name returns the slicer's unique name.
func (s *Slicer) Name() string { return s.slicer.Name }

// Caption returns the slicer's display caption, or "" when it uses the field
// name.
func (s *Slicer) Caption() string { return s.slicer.Caption }

// Cache returns the name of the slicer cache the slicer draws from.
func (s *Slicer) Cache() string { return s.slicer.Cache }

// SourceField returns the pivot field the slicer filters (e.g. "Region"), or ""
// when the slicer cache could not be resolved.
func (s *Slicer) SourceField() string {
	if s.cacheDef == nil {
		return ""
	}
	return s.cacheDef.SourceName
}

// ColumnCount returns the number of button columns the slicer lays its items
// out in, or 0 when unset.
func (s *Slicer) ColumnCount() int { return int(s.slicer.ColumnCount) }

// SheetName returns the name of the sheet the slicer is anchored on.
func (s *Slicer) SheetName() string {
	if s.sheet == nil {
		return ""
	}
	return s.sheet.name
}

// PivotTables returns the names of the pivot tables the slicer controls, or nil
// when the slicer cache could not be resolved.
func (s *Slicer) PivotTables() []string {
	if s.cacheDef == nil {
		return nil
	}
	return cachePivotTableNames(s.cacheDef.PivotTables)
}

// Timeline is a read view of a pivot timeline: an on-sheet control that filters
// one or more pivot tables by a range on a date pivot field. A Timeline
// returned by Sheet.Timelines or Workbook.Timelines reflects the timeline as
// stored in the opened workbook; its accessors are read-only.
//
// Creating timelines is not yet supported (see the package documentation); an
// opened workbook's timelines, timeline caches and their worksheet/workbook
// extension references round-trip byte-for-byte.
type Timeline struct {
	sheet    *Sheet
	timeline oxml.CT_Timeline
	cacheDef *oxml.CT_TimelineCacheDefinition
}

// Name returns the timeline's unique name.
func (t *Timeline) Name() string { return t.timeline.Name }

// Caption returns the timeline's display caption, or "" when unset.
func (t *Timeline) Caption() string { return t.timeline.Caption }

// Cache returns the name of the timeline cache the timeline draws from.
func (t *Timeline) Cache() string { return t.timeline.Cache }

// SourceField returns the date pivot field the timeline filters (e.g. "Date"),
// or "" when the timeline cache could not be resolved.
func (t *Timeline) SourceField() string {
	if t.cacheDef == nil {
		return ""
	}
	return t.cacheDef.SourceName
}

// Level returns the time grouping level: 0 years, 1 quarters, 2 months, 3 days.
func (t *Timeline) Level() int { return int(t.timeline.Level) }

// SheetName returns the name of the sheet the timeline is anchored on.
func (t *Timeline) SheetName() string {
	if t.sheet == nil {
		return ""
	}
	return t.sheet.name
}

// PivotTables returns the names of the pivot tables the timeline controls, or
// nil when the timeline cache could not be resolved.
func (t *Timeline) PivotTables() []string {
	if t.cacheDef == nil {
		return nil
	}
	return cachePivotTableNames(t.cacheDef.PivotTables)
}

// cachePivotTableNames maps parsed slicer/timeline cache pivot-table entries to
// their names.
func cachePivotTableNames(pts []oxml.SlicerCachePivotTable) []string {
	if len(pts) == 0 {
		return nil
	}
	out := make([]string, 0, len(pts))
	for _, pt := range pts {
		out = append(out, pt.Name)
	}
	return out
}

// slicerCaches parses the workbook's slicer cache parts (referenced from the
// workbook relationships) into a map keyed by cache name.
func (w *Workbook) slicerCaches() map[string]*oxml.CT_SlicerCacheDefinition {
	out := make(map[string]*oxml.CT_SlicerCacheDefinition)
	for _, rel := range w.relationships[w.mainPart()] {
		if rel == nil || rel.Type != opc.RelTypeSlicerCache {
			continue
		}
		part, ok := w.preservedParts[opc.ResolvePartName(w.mainPart(), rel.Target)]
		if !ok {
			continue
		}
		def, err := oxml.ParseSlicerCacheDefinition(part.Data)
		if err != nil || def == nil {
			continue
		}
		out[def.Name] = def
	}
	return out
}

// timelineCaches parses the workbook's timeline cache parts into a map keyed by
// cache name.
func (w *Workbook) timelineCaches() map[string]*oxml.CT_TimelineCacheDefinition {
	out := make(map[string]*oxml.CT_TimelineCacheDefinition)
	for _, rel := range w.relationships[w.mainPart()] {
		if rel == nil || rel.Type != opc.RelTypeTimelineCache {
			continue
		}
		part, ok := w.preservedParts[opc.ResolvePartName(w.mainPart(), rel.Target)]
		if !ok {
			continue
		}
		def, err := oxml.ParseTimelineCacheDefinition(part.Data)
		if err != nil || def == nil {
			continue
		}
		out[def.Name] = def
	}
	return out
}

// Slicers returns every slicer anchored on the sheet, resolving each slicer's
// cache (and therefore its source field and controlled pivot tables). The slice
// is nil when the sheet has no slicers.
func (s *Sheet) Slicers() []*Slicer {
	if s.workbook == nil || s.partName == "" {
		return nil
	}
	return s.slicersWithCaches(s.workbook.slicerCaches())
}

// slicersWithCaches enumerates the sheet's slicer parts and joins each slicer to
// the shared cache map by name.
func (s *Sheet) slicersWithCaches(caches map[string]*oxml.CT_SlicerCacheDefinition) []*Slicer {
	var out []*Slicer
	for _, rel := range s.workbook.relationships[s.partName] {
		if rel == nil || rel.Type != opc.RelTypeSlicer {
			continue
		}
		part, ok := s.workbook.preservedParts[opc.ResolvePartName(s.partName, rel.Target)]
		if !ok {
			continue
		}
		slicers, err := oxml.ParseSlicers(part.Data)
		if err != nil {
			continue
		}
		for _, sl := range slicers {
			out = append(out, &Slicer{sheet: s, slicer: sl, cacheDef: caches[sl.Cache]})
		}
	}
	return out
}

// Timelines returns every timeline anchored on the sheet, resolving each
// timeline's cache. The slice is nil when the sheet has no timelines.
func (s *Sheet) Timelines() []*Timeline {
	if s.workbook == nil || s.partName == "" {
		return nil
	}
	return s.timelinesWithCaches(s.workbook.timelineCaches())
}

// timelinesWithCaches enumerates the sheet's timeline parts and joins each
// timeline to the shared cache map by name.
func (s *Sheet) timelinesWithCaches(caches map[string]*oxml.CT_TimelineCacheDefinition) []*Timeline {
	var out []*Timeline
	for _, rel := range s.workbook.relationships[s.partName] {
		if rel == nil || rel.Type != opc.RelTypeTimeline {
			continue
		}
		part, ok := s.workbook.preservedParts[opc.ResolvePartName(s.partName, rel.Target)]
		if !ok {
			continue
		}
		timelines, err := oxml.ParseTimelines(part.Data)
		if err != nil {
			continue
		}
		for _, tl := range timelines {
			out = append(out, &Timeline{sheet: s, timeline: tl, cacheDef: caches[tl.Cache]})
		}
	}
	return out
}

// Slicers returns every slicer across all of the workbook's sheets, in sheet
// order. The slice is nil when the workbook has no slicers.
func (w *Workbook) Slicers() []*Slicer {
	caches := w.slicerCaches()
	var out []*Slicer
	for _, sheet := range w.sheets {
		if sheet == nil || sheet.partName == "" {
			continue
		}
		out = append(out, sheet.slicersWithCaches(caches)...)
	}
	return out
}

// Timelines returns every timeline across all of the workbook's sheets, in
// sheet order. The slice is nil when the workbook has no timelines.
func (w *Workbook) Timelines() []*Timeline {
	caches := w.timelineCaches()
	var out []*Timeline
	for _, sheet := range w.sheets {
		if sheet == nil || sheet.partName == "" {
			continue
		}
		out = append(out, sheet.timelinesWithCaches(caches)...)
	}
	return out
}
