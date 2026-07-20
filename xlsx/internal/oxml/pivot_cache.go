package oxml

import (
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
)

// CT_CacheField models one cacheField in a pivot cache definition.
type CT_CacheField struct {
	Name string
	Kind CacheFieldKind
	// SharedItems holds the distinct string values, in first-seen order, for a
	// CacheFieldString field. Records reference them by position.
	SharedItems []string
	// ContainsBlank reports whether any source cell in the field was empty.
	ContainsBlank bool
	// MinValue and MaxValue bound a CacheFieldNumber field's values.
	MinValue float64
	MaxValue float64
	// ContainsInteger reports whether every numeric value is a whole number.
	ContainsInteger bool
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
	b.StartElement(nsSpreadsheetML, "cacheField",
		xmlb.StrAttr("name", cf.Name),
		xmlb.IntAttr("numFmtId", 0))

	switch cf.Kind {
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
