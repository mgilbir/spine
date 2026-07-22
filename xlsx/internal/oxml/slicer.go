package oxml

import (
	"encoding/xml"
	"strings"
)

// Slicer and timeline namespaces. Slicers are a PowerPoint/Excel 2010 (x14)
// feature; timelines are an Excel 2013 (x15) feature. The parts live outside
// the shared workbook/worksheet parts (xl/slicers, xl/slicerCaches,
// xl/timelines, xl/timelineCaches) and are read-only in this package: they are
// preserved byte-for-byte on round-trip while these lenient parsers expose
// their contents for inspection.
const (
	// NSX15 is the Microsoft 2010/11 SpreadsheetML extension namespace (x15),
	// which carries the timeline feature.
	NSX15 = "http://schemas.microsoft.com/office/spreadsheetml/2010/11/main"
	// SlicerListExtURI identifies the worksheet extension listing a sheet's
	// slicer parts (an x14:slicerList referencing slicer parts by r:id).
	SlicerListExtURI = "{A8765BA9-456A-4DAB-B4F3-ACF838C121DE}"
	// SlicerCachesExtURI identifies the workbook extension listing the shared
	// slicer cache parts (an x14:slicerCaches referencing them by r:id).
	SlicerCachesExtURI = "{BBE1A952-AA13-448E-AADC-164F8A28A991}"
	// TimelineRefsExtURI identifies the worksheet extension listing a sheet's
	// timeline parts (an x15:timelineRefs referencing timeline parts by r:id).
	TimelineRefsExtURI = "{7E03D99C-DC04-49D9-9315-930204A7B6E9}"
	// TimelineCachesExtURI identifies the workbook extension listing the shared
	// timeline cache parts (an x15:timelineCacheRefs referencing them by r:id).
	TimelineCachesExtURI = "{C9C9C9C9-8B10-4B87-9E0D-2C0D3A3B0A0F}"
)

// SlicerCachePivotTable names a pivot table a slicer or timeline cache is wired
// to: the tabId indexes the workbook's pivot tables and name is the pivot
// table's name.
type SlicerCachePivotTable struct {
	TabID uint32
	Name  string
}

// CT_SlicerCacheDefinition is a read view of a slicerCacheDefinition part
// (xl/slicerCaches/slicerCacheN.xml): the shared cache that binds one or more
// slicers to a pivot cache field.
type CT_SlicerCacheDefinition struct {
	// Name is the cache name slicers reference through their cache attribute
	// (e.g. "Slicer_Region").
	Name string
	// SourceName is the pivot field the slicer filters (e.g. "Region").
	SourceName string
	// PivotCacheID is the id of the pivot cache the slicer draws its items
	// from (from <data><tabular pivotCacheId="...">); HasPivotCacheID reports
	// whether it was present.
	PivotCacheID    uint32
	HasPivotCacheID bool
	// PivotTables are the pivot tables the slicer controls.
	PivotTables []SlicerCachePivotTable
}

// CT_Slicer is a read view of one slicer within a slicers part
// (xl/slicers/slicerN.xml).
type CT_Slicer struct {
	// Name is the slicer's unique name.
	Name string
	// Cache is the slicerCacheDefinition name the slicer draws from.
	Cache string
	// Caption is the slicer's display caption (defaults to the field name).
	Caption string
	// ColumnCount is the number of button columns; HasColumnCount reports
	// whether it was present.
	ColumnCount    uint32
	HasColumnCount bool
	// UID is the slicer's stable {GUID} identifier, when present.
	UID string
}

// CT_TimelineCacheDefinition is a read view of a timelineCacheDefinition part
// (xl/timelineCaches/timelineCacheN.xml).
type CT_TimelineCacheDefinition struct {
	// Name is the cache name timelines reference through their cache attribute
	// (e.g. "NativeTimeline_Date").
	Name string
	// SourceName is the date pivot field the timeline filters (e.g. "Date").
	SourceName string
	// PivotTables are the pivot tables the timeline controls.
	PivotTables []SlicerCachePivotTable
}

// CT_Timeline is a read view of one timeline within a timelines part
// (xl/timelines/timelineN.xml).
type CT_Timeline struct {
	// Name is the timeline's unique name.
	Name string
	// Cache is the timelineCacheDefinition name the timeline draws from.
	Cache string
	// Caption is the timeline's display caption.
	Caption string
	// Level is the time grouping level (0=years, 1=quarters, 2=months, 3=days);
	// HasLevel reports whether it was present.
	Level    uint32
	HasLevel bool
	// UID is the timeline's stable {GUID} identifier, when present.
	UID string
}

// attrLookup returns the value of the first attribute whose local name matches,
// ignoring namespace, and whether it was found.
func attrLookup(start xml.StartElement, local string) (string, bool) {
	for _, a := range start.Attr {
		if a.Name.Local == local {
			return a.Value, true
		}
	}
	return "", false
}

// parsePivotTablesList reads a <pivotTables> element's <pivotTable tabId name/>
// children, leaving the decoder positioned after the closing tag.
func parsePivotTablesList(dec *xml.Decoder) ([]SlicerCachePivotTable, error) {
	var out []SlicerCachePivotTable
	for {
		tok, err := dec.Token()
		if err != nil {
			return out, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if t.Name.Local == "pivotTable" {
				var pt SlicerCachePivotTable
				if v, ok := attrLookup(t, "tabId"); ok {
					if n := parseUintPtr(v); n != nil {
						pt.TabID = *n
					}
				}
				if v, ok := attrLookup(t, "name"); ok {
					pt.Name = v
				}
				out = append(out, pt)
			}
			if err := dec.Skip(); err != nil {
				return out, err
			}
		case xml.EndElement:
			return out, nil
		}
	}
}

// ParseSlicerCacheDefinition parses a slicerCacheDefinition part. Parsing is
// lenient and keys on local names, so the x14 (or x15) prefix binding does not
// matter.
func ParseSlicerCacheDefinition(raw []byte) (*CT_SlicerCacheDefinition, error) {
	dec := xml.NewDecoder(strings.NewReader(string(raw)))
	def := &CT_SlicerCacheDefinition{}
	for {
		tok, err := dec.Token()
		if err != nil {
			break // io.EOF or truncated fragment: return what was parsed.
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "slicerCacheDefinition":
				if v, ok := attrLookup(t, "name"); ok {
					def.Name = v
				}
				if v, ok := attrLookup(t, "sourceName"); ok {
					def.SourceName = v
				}
			case "pivotTables":
				pts, err := parsePivotTablesList(dec)
				if err != nil {
					return nil, err
				}
				def.PivotTables = append(def.PivotTables, pts...)
			case "tabular":
				if v, ok := attrLookup(t, "pivotCacheId"); ok {
					if n := parseUintPtr(v); n != nil {
						def.PivotCacheID = *n
						def.HasPivotCacheID = true
					}
				}
			}
		case xml.EndElement:
			if t.Name.Local == "slicerCacheDefinition" {
				return def, nil
			}
		}
	}
	return def, nil
}

// ParseSlicers parses a slicers part into its slicer views.
func ParseSlicers(raw []byte) ([]CT_Slicer, error) {
	dec := xml.NewDecoder(strings.NewReader(string(raw)))
	var out []CT_Slicer
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		start, ok := tok.(xml.StartElement)
		if !ok || start.Name.Local != "slicer" {
			continue
		}
		s := CT_Slicer{}
		if v, ok := attrLookup(start, "name"); ok {
			s.Name = v
		}
		if v, ok := attrLookup(start, "cache"); ok {
			s.Cache = v
		}
		if v, ok := attrLookup(start, "caption"); ok {
			s.Caption = v
		}
		if v, ok := attrLookup(start, "uid"); ok {
			s.UID = v
		}
		if v, ok := attrLookup(start, "columnCount"); ok {
			if n := parseUintPtr(v); n != nil {
				s.ColumnCount = *n
				s.HasColumnCount = true
			}
		}
		out = append(out, s)
	}
	return out, nil
}

// ParseTimelineCacheDefinition parses a timelineCacheDefinition part.
func ParseTimelineCacheDefinition(raw []byte) (*CT_TimelineCacheDefinition, error) {
	dec := xml.NewDecoder(strings.NewReader(string(raw)))
	def := &CT_TimelineCacheDefinition{}
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "timelineCacheDefinition":
				if v, ok := attrLookup(t, "name"); ok {
					def.Name = v
				}
				if v, ok := attrLookup(t, "sourceName"); ok {
					def.SourceName = v
				}
			case "pivotTables":
				pts, err := parsePivotTablesList(dec)
				if err != nil {
					return nil, err
				}
				def.PivotTables = append(def.PivotTables, pts...)
			}
		case xml.EndElement:
			if t.Name.Local == "timelineCacheDefinition" {
				return def, nil
			}
		}
	}
	return def, nil
}

// ParseTimelines parses a timelines part into its timeline views.
func ParseTimelines(raw []byte) ([]CT_Timeline, error) {
	dec := xml.NewDecoder(strings.NewReader(string(raw)))
	var out []CT_Timeline
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		start, ok := tok.(xml.StartElement)
		if !ok || start.Name.Local != "timeline" {
			continue
		}
		tl := CT_Timeline{}
		if v, ok := attrLookup(start, "name"); ok {
			tl.Name = v
		}
		if v, ok := attrLookup(start, "cache"); ok {
			tl.Cache = v
		}
		if v, ok := attrLookup(start, "caption"); ok {
			tl.Caption = v
		}
		if v, ok := attrLookup(start, "uid"); ok {
			tl.UID = v
		}
		if v, ok := attrLookup(start, "level"); ok {
			if n := parseUintPtr(v); n != nil {
				tl.Level = *n
				tl.HasLevel = true
			}
		}
		out = append(out, tl)
	}
	return out, nil
}
