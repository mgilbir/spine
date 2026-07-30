package xlsx

import (
	"sort"
	"testing"

	"github.com/mgilbir/spine/common/validate"
	"github.com/mgilbir/spine/opc"
)

// TestValidateDeletedPartRefsReportsInPartOrder pins the order of the findings.
//
// The check used to walk w.relationships directly, so a workbook where several
// parts still pointed at something DeleteSheet removed reported those findings
// in Go's randomized map order — a different Report on every call for the same
// workbook (C497, C515).
func TestValidateDeletedPartRefsReportsInPartOrder(t *testing.T) {
	const gone = "/xl/worksheets/sheet9.xml"
	srcs := []string{
		"/xl/_rels/workbook.xml.rels",
		"/xl/drawings/_rels/drawing1.xml.rels",
		"/xl/worksheets/_rels/sheet1.xml.rels",
		"/xl/worksheets/_rels/sheet2.xml.rels",
		"/xl/worksheets/_rels/sheet3.xml.rels",
	}
	w := &Workbook{
		deletedParts:  map[string]bool{gone: true},
		relationships: map[string][]*opc.Relationship{},
	}
	for _, src := range srcs {
		w.relationships[src] = []*opc.Relationship{
			{ID: "rId1", Target: gone, Type: opc.RelTypeWorksheet},
		}
	}

	// Repeated because a single run of the old code had a fair chance of
	// producing sorted output by luck.
	for run := 0; run < 16; run++ {
		c := &validate.Collector{}
		w.validateDeletedPartRefs(c)
		report := c.Report()
		if len(report) != len(srcs) {
			t.Fatalf("run %d: got %d findings, want %d", run, len(report), len(srcs))
		}
		got := make([]string, len(report))
		for i, f := range report {
			got[i] = f.Part
		}
		want := append([]string(nil), got...)
		sort.Strings(want)
		for i := range got {
			if got[i] != want[i] {
				t.Fatalf("run %d: findings are not in part-name order:\n got %v\nwant %v",
					run, got, want)
			}
		}
	}
}
