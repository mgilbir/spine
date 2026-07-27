package oxml

import (
	"reflect"
	"strings"
	"testing"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// C411: the WML revision/annotation records all carry CT_TrackChange's
// w:id/w:author/w:date attribute group (CT_TblGridChange only w:id), and Word
// 2021 decorates every one of them with w16du:dateUtc. Capture was applied to
// three of them symptomatically, so the rest silently dropped the unmodeled
// attribute and the producer's attribute order.
//
// This is a class assertion, not an instance one: the table below is the
// complete set of revision-record types in the model, and each entry is
// round-tripped through a source carrying an unmodeled extension attribute.
func TestRevisionRecordsCaptureAttrs(t *testing.T) {
	const w16du = ` xmlns:w16du="http://schemas.microsoft.com/office/word/2023/wordml/word16du"`

	cases := []struct {
		local string
		new   func() any
		// inner is the record's typed child content, if any.
		inner string
	}{
		{local: "ins", new: func() any { return &CT_RunTrackChange{} }},
		{local: "rPrChange", new: func() any { return &CT_RPrChange{} }, inner: `<w:rPr/>`},
		{local: "cellIns", new: func() any { return &CT_TrackChange{} }},
		{local: "pPrChange", new: func() any { return &CT_PPrChange{} }, inner: `<w:pPr/>`},
		{local: "sectPrChange", new: func() any { return &CT_SectPrChange{} }, inner: `<w:sectPr/>`},
		{local: "tblPrChange", new: func() any { return &CT_TblPrChange{} }, inner: `<w:tblPr/>`},
		{local: "trPrChange", new: func() any { return &CT_TrPrChange{} }, inner: `<w:trPr/>`},
		{local: "tcPrChange", new: func() any { return &CT_TcPrChange{} }, inner: `<w:tcPr/>`},
		{local: "tblPrExChange", new: func() any { return &CT_TblPrExChange{} }, inner: `<w:tblPrEx/>`},
		{local: "tblGridChange", new: func() any { return &CT_TblGridChange{} }, inner: `<w:tblGrid/>`},
		{local: "cellMerge", new: func() any { return &CT_CellMerge{} }},
		{local: "comment", new: func() any { return &CT_Comment{} }, inner: `<w:p/>`},
	}

	for _, tc := range cases {
		t.Run(tc.local, func(t *testing.T) {
			v := tc.new()
			if _, ok := reflect.TypeOf(v).Elem().FieldByName("CapturedAttrs"); !ok {
				t.Fatalf("%T has no CapturedAttrs field: unmodeled revision attributes are dropped",
					v)
			}
			src := `<w:` + tc.local + wnsDecl + w16du +
				` w:id="7" w:author="A" w:date="2024-01-01T00:00:00Z" w16du:dateUtc="2024-01-01T00:00:00Z">` +
				tc.inner + `</w:` + tc.local + `>`
			if err := xmlb.UnmarshalWithSource([]byte(src), v); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			b := xmlb.NewWordprocessingMLBuilder()
			b.StartElementWithNS(xmlb.NSWordprocessingML, "document", xmlb.WordprocessingMLNamespaces())
			if m, ok := v.(xmlb.BuilderMarshaler); ok {
				m.MarshalToBuilder(b, xmlb.NSWordprocessingML, tc.local)
			} else {
				b.MarshalElement(xmlb.NSWordprocessingML, tc.local, v)
			}
			b.EndElement(xmlb.NSWordprocessingML, "document")
			if err := b.Finish(); err != nil {
				t.Fatalf("marshal: %v", err)
			}
			out := string(b.Bytes())
			if !strings.Contains(out, `w16du:dateUtc="2024-01-01T00:00:00Z"`) {
				t.Errorf("unmodeled w16du:dateUtc dropped from w:%s:\n%s", tc.local, out)
			}
		})
	}
}
