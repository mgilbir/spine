package chart

import (
	"encoding/xml"
	"reflect"
	"strings"
	"testing"
)

// C31: c:extLst children are c:ext (not a:ext); extensions must survive an
// unmarshal/marshal round-trip with their content and xmlns declarations.
func TestChartExtLst_RoundTrip(t *testing.T) {
	input := `<extLst xmlns="http://schemas.openxmlformats.org/drawingml/2006/chart">` +
		`<ext uri="{02D57815-91ED-43cb-92C2-25804820EDAC}" xmlns:c15="http://schemas.microsoft.com/office/drawing/2012/chart">` +
		`<c15:pivotSource><c15:name>PivotTable1</c15:name></c15:pivotSource>` +
		`</ext></extLst>`

	var el ExtLst
	if err := xml.Unmarshal([]byte(input), &el); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(el.Ext) != 1 {
		t.Fatalf("Ext length = %d, want 1", len(el.Ext))
	}
	ext := el.Ext[0]
	if ext.URI != "{02D57815-91ED-43cb-92C2-25804820EDAC}" {
		t.Errorf("URI = %q", ext.URI)
	}
	if !strings.Contains(string(ext.RawContent), "<c15:pivotSource>") {
		t.Errorf("RawContent = %q, want pivotSource captured", ext.RawContent)
	}
	if len(ext.InlineNSDecls) != 1 || ext.InlineNSDecls[0].Prefix != "c15" {
		t.Errorf("InlineNSDecls = %+v, want xmlns:c15 captured", ext.InlineNSDecls)
	}

	out, err := xml.Marshal(&el)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(out), `xmlns:c15="http://schemas.microsoft.com/office/drawing/2012/chart"`) {
		t.Errorf("xmlns:c15 declaration lost: %s", out)
	}
	if !strings.Contains(string(out), `<c15:pivotSource><c15:name>PivotTable1</c15:name></c15:pivotSource>`) {
		t.Errorf("extension content lost: %s", out)
	}

	var el2 ExtLst
	if err := xml.Unmarshal(out, &el2); err != nil {
		t.Fatalf("second unmarshal: %v", err)
	}
	if !reflect.DeepEqual(el, el2) {
		t.Errorf("round-trip mismatch:\n first %+v\nsecond %+v", el, el2)
	}
}

// C31: extensions nested inside a chart element (spectest-style round-trip
// through the containing type).
func TestChart_ExtLst_RoundTrip(t *testing.T) {
	input := `<chart xmlns="http://schemas.openxmlformats.org/drawingml/2006/chart">` +
		`<autoTitleDeleted val="0"/>` +
		`<plotVisOnly val="1"/>` +
		`<extLst><ext uri="{781A3756-C4B2-4CAC-9D66-4F8BD8637D16}" xmlns:c14="http://schemas.microsoft.com/office/drawing/2007/8/2/chart">` +
		`<c14:pivotOptions><c14:dropZoneFilter val="1"/></c14:pivotOptions>` +
		`</ext></extLst></chart>`

	var ch Chart
	if err := xml.Unmarshal([]byte(input), &ch); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if ch.ExtLst == nil || len(ch.ExtLst.Ext) != 1 {
		t.Fatalf("chart extLst not parsed: %+v", ch.ExtLst)
	}
	if !strings.Contains(string(ch.ExtLst.Ext[0].RawContent), "c14:pivotOptions") {
		t.Errorf("RawContent = %q", ch.ExtLst.Ext[0].RawContent)
	}

	out, err := xml.Marshal(&ch)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var ch2 Chart
	if err := xml.Unmarshal(out, &ch2); err != nil {
		t.Fatalf("second unmarshal: %v\n%s", err, out)
	}
	if !reflect.DeepEqual(ch.ExtLst, ch2.ExtLst) {
		t.Errorf("extLst round-trip mismatch:\n first %+v\nsecond %+v", ch.ExtLst, ch2.ExtLst)
	}
}
