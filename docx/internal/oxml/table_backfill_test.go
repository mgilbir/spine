package oxml

import (
	"strings"
	"testing"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// C352: a CT_Tbl carrying untracked Raw children (range permissions, tracked-
// move markers) must survive the child-order backfill triggered by the first
// AppendRow. CT_Tbl.backfillChildOrder omitted Raw (unlike CT_Tr), so the order
// flip dropped the Raw child on marshal.
func TestCTTbl_BackfillPreservesRaw(t *testing.T) {
	tbl := &CT_Tbl{
		Raw: []*CT_RawNamedElement{
			{Local: "permEnd", Space: NsWml},
		},
	}
	// First tracked append flips marshaling onto the childOrder path.
	tbl.AppendRow(&CT_Tr{})

	b := xmlb.NewWordprocessingMLBuilder()
	tbl.MarshalToBuilder(b, xmlb.NSWordprocessingML, "tbl")
	out := string(b.Bytes())

	if !strings.Contains(out, "permEnd") {
		t.Errorf("Raw child dropped after AppendRow order flip: %q", out)
	}
}
