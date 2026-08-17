package oxml

import (
	"encoding/xml"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// ---------------------------------------------------------------------------
// Table part (xl/tables/tableN.xml) — CT_Table / ST_ListObject.
//
// This models the SpreadsheetML table (a.k.a. ListObject) definition part. It
// is used two ways:
//
//   - Read: ParseTable decodes an existing table part so the public API can
//     surface its name, range, columns, header/totals rows and style. Tables
//     parsed from an opened workbook are never re-marshaled — the original part
//     bytes round-trip verbatim among the preserved parts — so the read path is
//     deliberately lenient and only decodes the fields the API exposes.
//   - Write: MarshalTable serializes a table created via Sheet.AddTable into a
//     fresh, canonical part.
// ---------------------------------------------------------------------------

// CT_Table models a table (ListObject) definition part.
type CT_Table struct {
	ID             uint32
	Name           string
	DisplayName    string
	Ref            string
	HeaderRowCount *uint32 // nil means the schema default of 1
	TotalsRowCount *uint32 // nil means the schema default of 0
	TotalsRowShown *bool   // nil means the schema default of true
	AutoFilterRef  string  // empty when the table has no autoFilter element
	Columns        []CT_TableColumn
	StyleInfo      *CT_TableStyleInfo
}

// CT_TableColumn models one tableColumn entry.
type CT_TableColumn struct {
	ID                      uint32
	Name                    string
	TotalsRowFunction       string // e.g. "sum", "count", "average"; "" for none
	TotalsRowLabel          string
	CalculatedColumnFormula string // content of the calculatedColumnFormula child
	TotalsRowFormula        string // content of the totalsRowFormula child
}

// CT_TableStyleInfo models the tableStyleInfo element.
type CT_TableStyleInfo struct {
	Name              string
	ShowFirstColumn   bool
	ShowLastColumn    bool
	ShowRowStripes    bool
	ShowColumnStripes bool
}

// HeaderRowShown reports whether the table shows a header row (headerRowCount
// defaults to 1, i.e. shown).
func (t *CT_Table) HeaderRowShown() bool {
	return t.HeaderRowCount == nil || *t.HeaderRowCount > 0
}

// TotalsRowVisible reports whether the table shows a totals row.
func (t *CT_Table) TotalsRowVisible() bool {
	return t.TotalsRowCount != nil && *t.TotalsRowCount > 0
}

// ---------------------------------------------------------------------------
// Read
// ---------------------------------------------------------------------------

// xmlTable is the read-only decode target. Attributes are matched by local
// name (namespace-agnostic) so a table part carrying its own default namespace
// decodes regardless of any prefixing.
type xmlTable struct {
	XMLName        xml.Name `xml:"table"`
	ID             uint32   `xml:"id,attr"`
	Name           string   `xml:"name,attr"`
	DisplayName    string   `xml:"displayName,attr"`
	Ref            string   `xml:"ref,attr"`
	HeaderRowCount *uint32  `xml:"headerRowCount,attr"`
	TotalsRowCount *uint32  `xml:"totalsRowCount,attr"`
	TotalsRowShown *string  `xml:"totalsRowShown,attr"`
	AutoFilter     *struct {
		Ref string `xml:"ref,attr"`
	} `xml:"autoFilter"`
	TableColumns struct {
		Column []xmlTableColumn `xml:"tableColumn"`
	} `xml:"tableColumns"`
	StyleInfo *xmlTableStyleInfo `xml:"tableStyleInfo"`
}

type xmlTableColumn struct {
	ID                      uint32 `xml:"id,attr"`
	Name                    string `xml:"name,attr"`
	TotalsRowFunction       string `xml:"totalsRowFunction,attr"`
	TotalsRowLabel          string `xml:"totalsRowLabel,attr"`
	CalculatedColumnFormula string `xml:"calculatedColumnFormula"`
	TotalsRowFormula        string `xml:"totalsRowFormula"`
}

type xmlTableStyleInfo struct {
	Name              string  `xml:"name,attr"`
	ShowFirstColumn   *string `xml:"showFirstColumn,attr"`
	ShowLastColumn    *string `xml:"showLastColumn,attr"`
	ShowRowStripes    *string `xml:"showRowStripes,attr"`
	ShowColumnStripes *string `xml:"showColumnStripes,attr"`
}

// ParseTable decodes a table definition part.
func ParseTable(data []byte) (*CT_Table, error) {
	var x xmlTable
	if err := xmlb.Unmarshal(data, &x); err != nil {
		return nil, err
	}
	t := &CT_Table{
		ID:             x.ID,
		Name:           x.Name,
		DisplayName:    x.DisplayName,
		Ref:            x.Ref,
		HeaderRowCount: x.HeaderRowCount,
		TotalsRowCount: x.TotalsRowCount,
	}
	if x.TotalsRowShown != nil {
		v := parseOnOff(*x.TotalsRowShown)
		t.TotalsRowShown = &v
	}
	if x.AutoFilter != nil {
		t.AutoFilterRef = x.AutoFilter.Ref
	}
	for _, c := range x.TableColumns.Column {
		t.Columns = append(t.Columns, CT_TableColumn(c))
	}
	if x.StyleInfo != nil {
		t.StyleInfo = &CT_TableStyleInfo{
			Name:              x.StyleInfo.Name,
			ShowFirstColumn:   onOffPtr(x.StyleInfo.ShowFirstColumn),
			ShowLastColumn:    onOffPtr(x.StyleInfo.ShowLastColumn),
			ShowRowStripes:    onOffPtr(x.StyleInfo.ShowRowStripes),
			ShowColumnStripes: onOffPtr(x.StyleInfo.ShowColumnStripes),
		}
	}
	return t, nil
}

// onOffPtr parses an optional ST_OnOff attribute, defaulting to false when the
// attribute is absent.
func onOffPtr(v *string) bool {
	return v != nil && parseOnOff(*v)
}

// ---------------------------------------------------------------------------
// Write
// ---------------------------------------------------------------------------

// MarshalTable serializes a table definition part in canonical form. Only
// tables created (or modified) this session go through here; tables loaded from
// an opened workbook round-trip as their original bytes, so this need not
// reproduce every producer's formatting.
func MarshalTable(t *CT_Table) ([]byte, error) {
	b := xmlb.NewBuilder()
	b.RegisterNamespace(nsSpreadsheetML, "")
	b.WriteHeader()

	attrs := []xmlb.Attr{
		xmlb.UintAttr("id", t.ID),
		xmlb.StrAttr("name", t.Name),
		xmlb.StrAttr("displayName", t.DisplayName),
		xmlb.StrAttr("ref", t.Ref),
	}
	if t.HeaderRowCount != nil && *t.HeaderRowCount != 1 {
		attrs = append(attrs, xmlb.UintAttr("headerRowCount", *t.HeaderRowCount))
	}
	if t.TotalsRowVisible() {
		attrs = append(attrs, xmlb.UintAttr("totalsRowCount", *t.TotalsRowCount))
	} else {
		// Excel emits totalsRowShown="0" on tables without a totals row.
		attrs = append(attrs, xmlb.BoolAttr("totalsRowShown", false))
	}

	b.StartElementWithNS(nsSpreadsheetML, "table",
		[]xmlb.NSDecl{{Prefix: "", URI: nsSpreadsheetML}}, attrs...)

	if t.AutoFilterRef != "" {
		b.EmptyElement(nsSpreadsheetML, "autoFilter", xmlb.StrAttr("ref", t.AutoFilterRef))
	}

	b.StartElement(nsSpreadsheetML, "tableColumns",
		xmlb.UintAttr("count", uint32(len(t.Columns))))
	for i := range t.Columns {
		c := &t.Columns[i]
		colAttrs := []xmlb.Attr{
			xmlb.UintAttr("id", c.ID),
			xmlb.StrAttr("name", c.Name),
		}
		if c.TotalsRowFunction != "" {
			colAttrs = append(colAttrs, xmlb.StrAttr("totalsRowFunction", c.TotalsRowFunction))
		}
		if c.TotalsRowLabel != "" {
			colAttrs = append(colAttrs, xmlb.StrAttr("totalsRowLabel", c.TotalsRowLabel))
		}
		if c.CalculatedColumnFormula == "" && c.TotalsRowFormula == "" {
			b.EmptyElement(nsSpreadsheetML, "tableColumn", colAttrs...)
			continue
		}
		b.StartElement(nsSpreadsheetML, "tableColumn", colAttrs...)
		if c.CalculatedColumnFormula != "" {
			b.WriteElement(nsSpreadsheetML, "calculatedColumnFormula", c.CalculatedColumnFormula)
		}
		if c.TotalsRowFormula != "" {
			b.WriteElement(nsSpreadsheetML, "totalsRowFormula", c.TotalsRowFormula)
		}
		b.EndElement(nsSpreadsheetML, "tableColumn")
	}
	b.EndElement(nsSpreadsheetML, "tableColumns")

	if t.StyleInfo != nil {
		si := t.StyleInfo
		styleAttrs := []xmlb.Attr{}
		if si.Name != "" {
			styleAttrs = append(styleAttrs, xmlb.StrAttr("name", si.Name))
		}
		styleAttrs = append(styleAttrs,
			xmlb.BoolAttr("showFirstColumn", si.ShowFirstColumn),
			xmlb.BoolAttr("showLastColumn", si.ShowLastColumn),
			xmlb.BoolAttr("showRowStripes", si.ShowRowStripes),
			xmlb.BoolAttr("showColumnStripes", si.ShowColumnStripes),
		)
		b.EmptyElement(nsSpreadsheetML, "tableStyleInfo", styleAttrs...)
	}

	b.EndElement(nsSpreadsheetML, "table")
	if err := b.Finish(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}
