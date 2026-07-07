package oxml

import (
	"reflect"
	"testing"
)

// C157: EnsureChildOrder must insert a missing child name at its CT_Worksheet
// schema position so that ChildOrder-gated marshaling emits it, without
// displacing captured unknown children.
func TestEnsureChildOrder(t *testing.T) {
	tests := []struct {
		name    string
		order   []string
		unknown []WbUnknownChild
		insert  string
		want    []string
	}{
		{
			name:   "empty order is left alone (default marshal path)",
			order:  nil,
			insert: "mergeCells",
			want:   nil,
		},
		{
			name:   "already present is not duplicated",
			order:  []string{"sheetData", "mergeCells", "pageMargins"},
			insert: "mergeCells",
			want:   []string{"sheetData", "mergeCells", "pageMargins"},
		},
		{
			name:   "inserted between lower and higher ranked children",
			order:  []string{"sheetData", "pageMargins"},
			insert: "mergeCells",
			want:   []string{"sheetData", "mergeCells", "pageMargins"},
		},
		{
			name:   "cols inserted before sheetData",
			order:  []string{"dimension", "sheetData", "pageMargins"},
			insert: "cols",
			want:   []string{"dimension", "cols", "sheetData", "pageMargins"},
		},
		{
			name:   "appended when no higher ranked child exists",
			order:  []string{"dimension", "sheetData"},
			insert: "mergeCells",
			want:   []string{"dimension", "sheetData", "mergeCells"},
		},
		{
			name:  "ranked unknown child constrains the insertion point",
			order: []string{"sheetData", "unknown:0", "unknown:1"},
			unknown: []WbUnknownChild{
				{Data: []byte(`<customSheetViews><customSheetView guid="{1}"/></customSheetViews>`)},
				{Data: []byte(`<oleObjects><oleObject progId="X"/></oleObjects>`)},
			},
			insert: "mergeCells",
			want:   []string{"sheetData", "unknown:0", "mergeCells", "unknown:1"},
		},
		{
			name:  "unranked unknown child does not constrain the insertion point",
			order: []string{"sheetData", "unknown:0", "pageMargins"},
			unknown: []WbUnknownChild{
				{Data: []byte(`<xr:something xmlns:xr="urn:x"/>`)},
			},
			insert: "mergeCells",
			want:   []string{"sheetData", "unknown:0", "mergeCells", "pageMargins"},
		},
		{
			name:   "non-schema name is ignored",
			order:  []string{"sheetData"},
			insert: "notAWorksheetChild",
			want:   []string{"sheetData"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ws := &CT_Worksheet{ChildOrder: tt.order, UnknownChildren: tt.unknown}
			ws.EnsureChildOrder(tt.insert)
			if !reflect.DeepEqual(ws.ChildOrder, tt.want) {
				t.Errorf("ChildOrder = %v, want %v", ws.ChildOrder, tt.want)
			}
		})
	}
}

func TestUnknownElementLocalName(t *testing.T) {
	tests := []struct {
		raw  string
		want string
	}{
		{`<oleObjects><oleObject/></oleObjects>`, "oleObjects"},
		{`<customSheetViews guid="{1}"/>`, "customSheetViews"},
		{`<xr:revisionPtr xmlns:xr="urn:x"/>`, "revisionPtr"},
		{`<picture/>`, "picture"},
		{``, ""},
		{`not xml`, ""},
	}
	for _, tt := range tests {
		if got := unknownElementLocalName([]byte(tt.raw)); got != tt.want {
			t.Errorf("unknownElementLocalName(%q) = %q, want %q", tt.raw, got, tt.want)
		}
	}
}
