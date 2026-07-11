package oxml

import (
	"reflect"
	"testing"
)

// C12: CT_Workbook.EnsureChildOrder must insert a missing child name at its
// CT_Workbook schema position so that ChildOrder-gated marshaling emits it,
// without displacing captured unknown children (the CT_Worksheet analog of
// the C157 fix).
func TestWorkbookEnsureChildOrder(t *testing.T) {
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
			insert: "bookViews",
			want:   nil,
		},
		{
			name:   "already present is not duplicated",
			order:  []string{"bookViews", "sheets", "calcPr"},
			insert: "bookViews",
			want:   []string{"bookViews", "sheets", "calcPr"},
		},
		{
			name:   "bookViews inserted before sheets",
			order:  []string{"fileVersion", "workbookPr", "sheets", "calcPr"},
			insert: "bookViews",
			want:   []string{"fileVersion", "workbookPr", "bookViews", "sheets", "calcPr"},
		},
		{
			name:   "definedNames inserted after sheets",
			order:  []string{"sheets", "calcPr"},
			insert: "definedNames",
			want:   []string{"sheets", "definedNames", "calcPr"},
		},
		{
			name:   "appended when nothing ranks higher",
			order:  []string{"workbookPr", "sheets"},
			insert: "definedNames",
			want:   []string{"workbookPr", "sheets", "definedNames"},
		},
		{
			name:  "rankable unknown child constrains insertion",
			order: []string{"sheets", "unknown:0", "calcPr"},
			unknown: []WbUnknownChild{
				{Data: []byte(`<externalReferences><externalReference r:id="rId9"/></externalReferences>`)},
			},
			insert: "bookViews",
			want:   []string{"bookViews", "sheets", "unknown:0", "calcPr"},
		},
		{
			name:  "unrankable unknown child imposes no constraint",
			order: []string{"fileVersion", "unknown:0", "sheets"},
			unknown: []WbUnknownChild{
				{Data: []byte(`<xr:revisionPtr revIDLastSave="0"/>`)},
			},
			insert: "bookViews",
			want:   []string{"fileVersion", "unknown:0", "bookViews", "sheets"},
		},
		{
			name:   "AlternateContent entries impose no constraint",
			order:  []string{"fileVersion", "workbookPr", "AlternateContent", "sheets"},
			insert: "bookViews",
			want:   []string{"fileVersion", "workbookPr", "AlternateContent", "bookViews", "sheets"},
		},
		{
			name:   "unknown insert name is ignored",
			order:  []string{"sheets"},
			insert: "notAWorkbookChild",
			want:   []string{"sheets"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			wb := &CT_Workbook{ChildOrder: tc.order, UnknownChildren: tc.unknown}
			wb.EnsureChildOrder(tc.insert)
			if !reflect.DeepEqual(wb.ChildOrder, tc.want) {
				t.Errorf("ChildOrder = %v, want %v", wb.ChildOrder, tc.want)
			}
		})
	}
}

// TestWorkbookSchemaSeqMatchesRankMap guards the derived rank map.
func TestWorkbookSchemaSeqMatchesRankMap(t *testing.T) {
	if len(workbookChildRank) != len(workbookSchemaSeq) {
		t.Fatalf("rank map has %d entries, schema seq has %d", len(workbookChildRank), len(workbookSchemaSeq))
	}
	for i, n := range workbookSchemaSeq {
		if workbookChildRank[n] != i {
			t.Errorf("rank of %q = %d, want %d", n, workbookChildRank[n], i)
		}
	}
}
