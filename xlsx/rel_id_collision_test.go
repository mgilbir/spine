package xlsx

import (
	"testing"

	"github.com/mgilbir/spine/opc"
)

// TestSaveNewRelIDsUniqueWithPivotAndVBA guards C258: saveNew allocated the VBA
// relationship id with a hand-maintained counter while the pivot-cache
// relationship was scan-allocated, so Create -> AddPivotTable -> SetVBAProject ->
// Save emitted Id="rId3" twice in xl/_rels/workbook.xml.rels (pivotCacheDefinition
// and vbaProject).
func TestSaveNewRelIDsUniqueWithPivotAndVBA(t *testing.T) {
	wb := buildPivotSourceWorkbook(t)
	report, err := wb.SheetByName("Report")
	if err != nil {
		t.Fatalf("SheetByName(Report): %v", err)
	}
	if _, err := report.AddPivotTable("Data!A1:C5", "A3", PivotOptions{
		RowFields:   []string{"Region"},
		ValueFields: []PivotValueField{{Field: "Sales", Aggregation: PivotSum}},
	}); err != nil {
		t.Fatalf("AddPivotTable: %v", err)
	}
	wb.SetVBAProject(testVBABytes)

	out, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	rels, err := opc.UnmarshalRelationships(readZipPart(t, out, "xl/_rels/workbook.xml.rels"))
	if err != nil {
		t.Fatalf("UnmarshalRelationships: %v", err)
	}

	seen := make(map[string]string)
	var hasPivot, hasVBA bool
	for _, rel := range rels {
		if prev, dup := seen[rel.ID]; dup {
			t.Errorf("relationship id %q used twice: %s and %s", rel.ID, prev, rel.Type)
		}
		seen[rel.ID] = rel.Type
		switch rel.Type {
		case opc.RelTypePivotCacheDef:
			hasPivot = true
		case opc.RelTypeVBAProject:
			hasVBA = true
		}
	}
	if !hasPivot || !hasVBA {
		t.Fatalf("expected both pivot-cache and VBA relationships; pivot=%v vba=%v", hasPivot, hasVBA)
	}
}
