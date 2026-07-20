package oxml

import (
	"bytes"
	"encoding/xml"
	"strconv"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// ---------------------------------------------------------------------------
// Pivot cache parts.
//
// A pivot table's data lives in a pivot cache, stored as two parts:
//
//   - the cache definition (xl/pivotCache/pivotCacheDefinitionN.xml,
//     CT_PivotCacheDefinition) describing the source range and the derived
//     cache fields with their shared items, and
//   - the cache records (xl/pivotCache/pivotCacheRecordsN.xml,
//     CT_PivotCacheRecords) holding one record per source data row.
//
// The workbook part references its caches through a <pivotCaches> element
// (CT_PivotCaches) that maps each cacheId to the cache definition part via a
// relationship id.
//
// Read (ParsePivotCacheDefinition) is lenient and only decodes the fields the
// public API exposes; pivot parts loaded from an opened workbook round-trip as
// their original bytes among the preserved parts and are never re-marshaled.
// Write (MarshalPivotCacheDefinition / MarshalPivotCacheRecords) serializes a
// cache created via Sheet.AddPivotTable into fresh, canonical parts.
// ---------------------------------------------------------------------------

// CacheFieldKind classifies how a cache field's values are stored.
type CacheFieldKind int

const (
	// CacheFieldString is a field whose distinct values are held as shared
	// string items; records reference them by index.
	CacheFieldString CacheFieldKind = iota
	// CacheFieldNumber is a numeric field; records store the number inline and
	// the field carries no shared items.
	CacheFieldNumber
	// CacheFieldNumberDiscrete is a numeric field whose distinct values are
	// enumerated as shared items (as <n v="..."/>); records reference them by
	// index. Used for the base field of a numeric range grouping, whose group
	// buckets are derived from the enumerated values.
	CacheFieldNumberDiscrete
	// CacheFieldDateDiscrete is a date/time field whose distinct values are
	// enumerated as shared items (as <d v="ISO8601"/>); records reference them by
	// index. Used for the base field of a date range grouping, whose calendar
	// buckets are derived from the enumerated values.
	CacheFieldDateDiscrete
	// CacheFieldGroup is a derived group field: it carries a <fieldGroup> that
	// buckets a base field's values and has no records of its own
	// (databaseField="0").
	CacheFieldGroup
	// CacheFieldCalculated is a calculated field: it carries a formula and no
	// records of its own (databaseField="0").
	CacheFieldCalculated
)

// GroupKind classifies how a CacheFieldGroupInfo buckets its base field.
type GroupKind int

const (
	// GroupNumeric buckets a numeric base field into equal-width value ranges
	// (rangePr with startNum/endNum/groupInterval).
	GroupNumeric GroupKind = iota
	// GroupDate buckets a date/time base field into calendar units (rangePr with
	// groupBy/startDate/endDate).
	GroupDate
	// GroupDiscrete folds selected base items into named parent groups
	// (groupItems + discretePr mapping each base item to a group item).
	GroupDiscrete
)

// CacheFieldGroupInfo describes a grouping applied to a base cache field,
// producing the group field's <fieldGroup>/<rangePr>/<groupItems>/<discretePr>.
type CacheFieldGroupInfo struct {
	// Kind selects numeric-range, date-range or discrete grouping.
	Kind GroupKind
	// Base is the index of the base cache field whose values are bucketed.
	Base int
	// Start, End and Interval define numeric range buckets (GroupNumeric): values
	// below Start fall in the leading "<Start" item, values at or above End in
	// the trailing ">End" item, and the rest into [Start, Start+Interval) buckets.
	Start    float64
	End      float64
	Interval float64
	// GroupBy is the calendar unit for a date grouping (GroupDate): "years",
	// "quarters", "months" or "days".
	GroupBy string
	// StartDate and EndDate bound a date grouping (GroupDate), ISO 8601.
	StartDate string
	EndDate   string
	// DiscreteMap maps each base item index to its group item index for a
	// discrete grouping (GroupDiscrete).
	DiscreteMap []int
	// Items are the group bucket labels, in order.
	Items []string
}

// CT_CacheField models one cacheField in a pivot cache definition.
type CT_CacheField struct {
	Name string
	Kind CacheFieldKind
	// SharedItems holds the distinct string values, in first-seen order, for a
	// CacheFieldString field. Records reference them by position.
	SharedItems []string
	// NumericItems holds the distinct numeric values, in ascending order, for a
	// CacheFieldNumberDiscrete field. Records reference them by position.
	NumericItems []float64
	// DateItems holds the distinct date/time values (ISO 8601), in ascending
	// order, for a CacheFieldDateDiscrete field. Records reference them by
	// position. MinDate and MaxDate bound them.
	DateItems []string
	MinDate   string
	MaxDate   string
	// ContainsBlank reports whether any source cell in the field was empty.
	ContainsBlank bool
	// MinValue and MaxValue bound a numeric field's values.
	MinValue float64
	MaxValue float64
	// ContainsInteger reports whether every numeric value is a whole number.
	ContainsInteger bool
	// Formula is the calculated-field formula (e.g. "Sales-Cost") for a
	// CacheFieldCalculated field; empty otherwise.
	Formula string
	// Group carries the grouping definition for a CacheFieldGroup field; nil
	// otherwise.
	Group *CacheFieldGroupInfo
}

// CT_PivotCacheDefinition models a pivot cache definition part.
type CT_PivotCacheDefinition struct {
	// SourceSheet and SourceRef locate the cache's worksheet source range.
	SourceSheet string
	SourceRef   string
	CacheFields []CT_CacheField
	RecordCount uint32
	// RefreshOnLoad asks Excel to rebuild the cache (and the pivot layout) when
	// the workbook is opened.
	RefreshOnLoad bool
}

// --- Read ---

type xmlPivotCacheDefinition struct {
	XMLName     xml.Name `xml:"pivotCacheDefinition"`
	RecordCount *uint32  `xml:"recordCount,attr"`
	CacheSource struct {
		Type            string `xml:"type,attr"`
		WorksheetSource struct {
			Ref   string `xml:"ref,attr"`
			Sheet string `xml:"sheet,attr"`
		} `xml:"worksheetSource"`
	} `xml:"cacheSource"`
	CacheFields struct {
		Field []struct {
			Name        string `xml:"name,attr"`
			SharedItems struct {
				S []struct {
					V string `xml:"v,attr"`
				} `xml:"s"`
			} `xml:"sharedItems"`
		} `xml:"cacheField"`
	} `xml:"cacheFields"`
}

// ParsePivotCacheDefinition decodes a pivot cache definition part.
func ParsePivotCacheDefinition(data []byte) (*CT_PivotCacheDefinition, error) {
	var x xmlPivotCacheDefinition
	if err := xml.Unmarshal(data, &x); err != nil {
		return nil, err
	}
	def := &CT_PivotCacheDefinition{
		SourceSheet: x.CacheSource.WorksheetSource.Sheet,
		SourceRef:   x.CacheSource.WorksheetSource.Ref,
	}
	if x.RecordCount != nil {
		def.RecordCount = *x.RecordCount
	}
	for _, f := range x.CacheFields.Field {
		cf := CT_CacheField{Name: f.Name}
		for _, s := range f.SharedItems.S {
			cf.SharedItems = append(cf.SharedItems, s.V)
		}
		if len(cf.SharedItems) > 0 {
			cf.Kind = CacheFieldString
		} else {
			cf.Kind = CacheFieldNumber
		}
		def.CacheFields = append(def.CacheFields, cf)
	}
	return def, nil
}

// --- Write ---

// createdVersion values mirror what Excel writes for freshly created pivot
// content (Excel 2013/2016). They are advisory; refreshOnLoad drives the
// actual behaviour on open.
const (
	pivotCreatedVersion  = 6
	pivotRefreshVersion  = 6
	pivotMinRefreshables = 3
)

// MarshalPivotCacheDefinition serializes a pivot cache definition. recordsRID
// is the relationship id (in the cache definition's .rels) that points at the
// records part.
func MarshalPivotCacheDefinition(def *CT_PivotCacheDefinition, recordsRID string) []byte {
	b := xmlb.NewBuilder()
	b.RegisterNamespace(nsSpreadsheetML, "")
	b.RegisterNamespace(nsR, "r")
	b.WriteHeader()

	attrs := []xmlb.Attr{}
	if recordsRID != "" {
		attrs = append(attrs, xmlb.RelAttr("id", recordsRID))
	}
	attrs = append(attrs,
		xmlb.BoolAttr("refreshOnLoad", def.RefreshOnLoad),
		xmlb.StrAttr("refreshedBy", "spine"),
		xmlb.UintAttr("createdVersion", pivotCreatedVersion),
		xmlb.UintAttr("refreshedVersion", pivotRefreshVersion),
		xmlb.UintAttr("minRefreshableVersion", pivotMinRefreshables),
		xmlb.UintAttr("recordCount", def.RecordCount),
	)
	b.StartElementWithNS(nsSpreadsheetML, "pivotCacheDefinition",
		[]xmlb.NSDecl{{Prefix: "", URI: nsSpreadsheetML}, {Prefix: "r", URI: nsR}},
		attrs...)

	b.StartElement(nsSpreadsheetML, "cacheSource", xmlb.StrAttr("type", "worksheet"))
	b.EmptyElement(nsSpreadsheetML, "worksheetSource",
		xmlb.StrAttr("ref", def.SourceRef),
		xmlb.StrAttr("sheet", def.SourceSheet))
	b.EndElement(nsSpreadsheetML, "cacheSource")

	b.StartElement(nsSpreadsheetML, "cacheFields",
		xmlb.UintAttr("count", uint32(len(def.CacheFields))))
	for i := range def.CacheFields {
		marshalCacheField(b, &def.CacheFields[i])
	}
	b.EndElement(nsSpreadsheetML, "cacheFields")

	b.EndElement(nsSpreadsheetML, "pivotCacheDefinition")
	_ = b.Finish()
	return b.Bytes()
}

func marshalCacheField(b *xmlb.Builder, cf *CT_CacheField) {
	attrs := []xmlb.Attr{
		xmlb.StrAttr("name", cf.Name),
		xmlb.IntAttr("numFmtId", 0),
	}
	if cf.Kind == CacheFieldCalculated || cf.Kind == CacheFieldGroup {
		attrs = append(attrs, xmlb.IntAttr("databaseField", 0))
	}
	if cf.Formula != "" {
		attrs = append(attrs, xmlb.StrAttr("formula", cf.Formula))
	}
	b.StartElement(nsSpreadsheetML, "cacheField", attrs...)

	switch cf.Kind {
	case CacheFieldGroup:
		marshalFieldGroup(b, cf.Group)
	case CacheFieldCalculated:
		// A calculated field derives numeric results; refreshOnLoad rebuilds the
		// concrete values on open, so emit only the type hint.
		b.EmptyElement(nsSpreadsheetML, "sharedItems",
			xmlb.BoolAttr("containsSemiMixedTypes", false),
			xmlb.BoolAttr("containsString", false),
			xmlb.BoolAttr("containsNumber", true))
	case CacheFieldNumber:
		siAttrs := []xmlb.Attr{
			xmlb.BoolAttr("containsSemiMixedTypes", false),
			xmlb.BoolAttr("containsString", false),
			xmlb.BoolAttr("containsNumber", true),
		}
		if cf.ContainsInteger {
			siAttrs = append(siAttrs, xmlb.BoolAttr("containsInteger", true))
		}
		siAttrs = append(siAttrs,
			xmlb.StrAttr("minValue", formatFloat(cf.MinValue)),
			xmlb.StrAttr("maxValue", formatFloat(cf.MaxValue)))
		b.EmptyElement(nsSpreadsheetML, "sharedItems", siAttrs...)
	case CacheFieldNumberDiscrete:
		siAttrs := []xmlb.Attr{
			xmlb.BoolAttr("containsSemiMixedTypes", false),
			xmlb.BoolAttr("containsString", false),
			xmlb.BoolAttr("containsNumber", true),
		}
		if cf.ContainsInteger {
			siAttrs = append(siAttrs, xmlb.BoolAttr("containsInteger", true))
		}
		siAttrs = append(siAttrs,
			xmlb.StrAttr("minValue", formatFloat(cf.MinValue)),
			xmlb.StrAttr("maxValue", formatFloat(cf.MaxValue)),
			xmlb.UintAttr("count", uint32(len(cf.NumericItems))))
		b.StartElement(nsSpreadsheetML, "sharedItems", siAttrs...)
		for _, v := range cf.NumericItems {
			b.EmptyElement(nsSpreadsheetML, "n", xmlb.StrAttr("v", formatFloat(v)))
		}
		b.EndElement(nsSpreadsheetML, "sharedItems")
	case CacheFieldDateDiscrete:
		siAttrs := []xmlb.Attr{
			xmlb.BoolAttr("containsSemiMixedTypes", false),
			xmlb.BoolAttr("containsNonDate", false),
			xmlb.BoolAttr("containsDate", true),
			xmlb.BoolAttr("containsString", false),
			xmlb.StrAttr("minDate", cf.MinDate),
			xmlb.StrAttr("maxDate", cf.MaxDate),
			xmlb.UintAttr("count", uint32(len(cf.DateItems))),
		}
		b.StartElement(nsSpreadsheetML, "sharedItems", siAttrs...)
		for _, v := range cf.DateItems {
			b.EmptyElement(nsSpreadsheetML, "d", xmlb.StrAttr("v", v))
		}
		b.EndElement(nsSpreadsheetML, "sharedItems")
	default:
		siAttrs := []xmlb.Attr{}
		if cf.ContainsBlank {
			siAttrs = append(siAttrs, xmlb.BoolAttr("containsBlank", true))
		}
		siAttrs = append(siAttrs, xmlb.UintAttr("count", uint32(len(cf.SharedItems))))
		b.StartElement(nsSpreadsheetML, "sharedItems", siAttrs...)
		for _, v := range cf.SharedItems {
			b.EmptyElement(nsSpreadsheetML, "s", xmlb.StrAttr("v", v))
		}
		b.EndElement(nsSpreadsheetML, "sharedItems")
	}

	b.EndElement(nsSpreadsheetML, "cacheField")
}

// marshalFieldGroup writes a grouping's <fieldGroup>: a <rangePr> for numeric
// or date range groupings, or a <discretePr> item map for discrete groupings,
// each followed by the <groupItems> bucket labels.
func marshalFieldGroup(b *xmlb.Builder, g *CacheFieldGroupInfo) {
	b.StartElement(nsSpreadsheetML, "fieldGroup", xmlb.IntAttr("base", int64(g.Base)))
	switch g.Kind {
	case GroupDate:
		b.EmptyElement(nsSpreadsheetML, "rangePr",
			xmlb.BoolAttr("autoStart", false),
			xmlb.BoolAttr("autoEnd", false),
			xmlb.StrAttr("groupBy", g.GroupBy),
			xmlb.StrAttr("startDate", g.StartDate),
			xmlb.StrAttr("endDate", g.EndDate))
		marshalGroupItems(b, g.Items)
	case GroupDiscrete:
		marshalGroupItems(b, g.Items)
		b.StartElement(nsSpreadsheetML, "discretePr", xmlb.UintAttr("count", uint32(len(g.DiscreteMap))))
		for _, x := range g.DiscreteMap {
			b.EmptyElement(nsSpreadsheetML, "x", xmlb.IntAttr("v", int64(x)))
		}
		b.EndElement(nsSpreadsheetML, "discretePr")
	default: // GroupNumeric
		b.EmptyElement(nsSpreadsheetML, "rangePr",
			xmlb.BoolAttr("autoStart", false),
			xmlb.BoolAttr("autoEnd", false),
			xmlb.StrAttr("startNum", formatFloat(g.Start)),
			xmlb.StrAttr("endNum", formatFloat(g.End)),
			xmlb.StrAttr("groupInterval", formatFloat(g.Interval)))
		marshalGroupItems(b, g.Items)
	}
	b.EndElement(nsSpreadsheetML, "fieldGroup")
}

// marshalGroupItems writes the <groupItems> bucket-label list.
func marshalGroupItems(b *xmlb.Builder, items []string) {
	b.StartElement(nsSpreadsheetML, "groupItems", xmlb.UintAttr("count", uint32(len(items))))
	for _, it := range items {
		b.EmptyElement(nsSpreadsheetML, "s", xmlb.StrAttr("v", it))
	}
	b.EndElement(nsSpreadsheetML, "groupItems")
}

// PivotRecord is one cached source row: one value per cache field. A value is
// either a shared-item index (for a string field) or a number (for a numeric
// field), matching the field's kind.
type PivotRecord struct {
	Values []PivotRecordValue
}

// PivotRecordValue is a single cell in a cached record.
type PivotRecordValue struct {
	// SharedIndex is the index into the field's shared items when the field is a
	// string field; Number holds the value for a numeric field.
	SharedIndex int
	Number      float64
	IsNumber    bool
	// IsMissing marks an empty source cell, emitted as <m/>.
	IsMissing bool
}

// MarshalPivotCacheRecords serializes the pivot cache records part.
func MarshalPivotCacheRecords(records []PivotRecord) []byte {
	b := xmlb.NewBuilder()
	b.RegisterNamespace(nsSpreadsheetML, "")
	b.WriteHeader()

	b.StartElementWithNS(nsSpreadsheetML, "pivotCacheRecords",
		[]xmlb.NSDecl{{Prefix: "", URI: nsSpreadsheetML}},
		xmlb.UintAttr("count", uint32(len(records))))

	for i := range records {
		rec := &records[i]
		b.StartElement(nsSpreadsheetML, "r")
		for _, v := range rec.Values {
			switch {
			case v.IsMissing:
				b.EmptyElement(nsSpreadsheetML, "m")
			case v.IsNumber:
				b.EmptyElement(nsSpreadsheetML, "n", xmlb.StrAttr("v", formatFloat(v.Number)))
			default:
				b.EmptyElement(nsSpreadsheetML, "x", xmlb.IntAttr("v", int64(v.SharedIndex)))
			}
		}
		b.EndElement(nsSpreadsheetML, "r")
	}

	b.EndElement(nsSpreadsheetML, "pivotCacheRecords")
	_ = b.Finish()
	return b.Bytes()
}

// formatFloat renders a float without a trailing ".0" for whole numbers,
// matching how Excel writes numeric cache values.
func formatFloat(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// ---------------------------------------------------------------------------
// Workbook <pivotCaches> element.
// ---------------------------------------------------------------------------

// CT_PivotCaches models the workbook's pivotCaches element, mapping each pivot
// cache id to the relationship that resolves its cache definition part.
type CT_PivotCaches struct {
	Cache []CT_PivotCache
}

// CT_PivotCache is one pivotCache entry.
type CT_PivotCache struct {
	CacheId uint32
	RID     string
}

// IsPivotCachesElement reports whether raw holds a <pivotCaches> element (as
// captured verbatim among a workbook's unknown children).
func IsPivotCachesElement(raw []byte) bool {
	trimmed := bytes.TrimLeft(raw, " \t\r\n")
	return bytes.HasPrefix(trimmed, []byte("<pivotCaches")) &&
		(len(trimmed) <= len("<pivotCaches") || isNameBoundary(trimmed[len("<pivotCaches")]))
}

func isNameBoundary(c byte) bool {
	return c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == '>' || c == '/'
}

// ParsePivotCachesElement decodes a workbook's <pivotCaches> element into its
// entries, mapping each pivot cache id to the relationship id that resolves its
// cache definition part. It is used to preserve a workbook's existing pivot
// caches when a new one is added this session.
func ParsePivotCachesElement(raw []byte) []CT_PivotCache {
	var x struct {
		Cache []struct {
			CacheId uint32 `xml:"cacheId,attr"`
			RID     string `xml:"id,attr"`
		} `xml:"pivotCache"`
	}
	if err := xml.Unmarshal(raw, &x); err != nil {
		return nil
	}
	out := make([]CT_PivotCache, 0, len(x.Cache))
	for _, c := range x.Cache {
		out = append(out, CT_PivotCache{CacheId: c.CacheId, RID: c.RID})
	}
	return out
}

// MarshalToBuilder writes the pivotCaches element.
func (pc *CT_PivotCaches) MarshalToBuilder(b *xmlb.Builder, ns, localName string) {
	b.StartElement(ns, localName)
	for _, c := range pc.Cache {
		b.EmptyElement(ns, "pivotCache",
			xmlb.UintAttr("cacheId", c.CacheId),
			xmlb.RelAttr("id", c.RID))
	}
	b.EndElement(ns, localName)
}
