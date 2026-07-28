package oxml

import (
	"strings"
	"testing"

	xmlb "github.com/mgilbir/spine/common/xml"
	"github.com/mgilbir/spine/internal/testutil"
)

// C329: property bags parsed from a document replay their captured children
// verbatim, but a property *added* after parse used to be appended after all of
// them — which for a type whose XSD content model is a sequence emits an
// element in a position the schema forbids. The two shapes the audit recorded
// are covered below, together with the byte-identity constraint that made the
// fix delicate: a source whose children were already out of order must still
// round-trip exactly as it came in.

// wmlBuilder is the marshaling context these probes emit into.
func wmlBuilder() *xmlb.Builder { return xmlb.NewWordprocessingMLBuilder() }

// marshalProbe renders v as a <w:probe> element so the child sequence can be
// read off the output.
func marshalProbe(v any, local string) string {
	b := wmlBuilder()
	b.MarshalElement(NsWml, local, v)
	return string(b.Bytes())
}

// TestRPrAddedPropertyPrecedesLaterSibling is the audit's first repro: a parsed
// run carrying only <w:sz>, given bold after parse, emitted
// <w:rPr><w:sz/><w:b/></w:rPr>.
//
// EG_RPrBase is a repeated xsd:choice, so that output does validate; the order
// is nonetheless wrong against every producer's canonical emission and against
// the type's own declaration order, and it is the same append that produces the
// genuinely invalid rPrChange shape below.
func TestRPrAddedPropertyPrecedesLaterSibling(t *testing.T) {
	src := `<w:rPr xmlns:w="` + NsWml + `"><w:sz w:val="24"/></w:rPr>`
	var rp CT_RPr
	if err := xmlb.UnmarshalWithSource([]byte(src), &rp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	rp.B = &CT_OnOff{} // the SetBold path

	out := marshalProbe(&rp, "rPr")
	if iB, iSz := strings.Index(out, "<w:b/>"), strings.Index(out, "<w:sz "); iB < 0 || iSz < 0 || iB > iSz {
		t.Errorf("w:b must precede w:sz (CT_RPr declares b before sz): %s", out)
	}
}

// TestRPrAddedPropertyPrecedesRPrChange is the schema-invalid rPr shape.
// EG_RPrContent is `EG_RPrBase*, rPrChange?`, so rPrChange is pinned last: a
// w:b appended after a captured w:rPrChange cannot validate.
func TestRPrAddedPropertyPrecedesRPrChange(t *testing.T) {
	src := `<w:rPr xmlns:w="` + NsWml + `"><w:sz w:val="24"/>` +
		`<w:rPrChange w:id="1" w:author="a"><w:rPr/></w:rPrChange></w:rPr>`
	var rp CT_RPr
	if err := xmlb.UnmarshalWithSource([]byte(src), &rp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	rp.B = &CT_OnOff{}

	out := marshalProbe(&rp, "rPr")
	if iB, iChange := strings.Index(out, "<w:b/>"), strings.Index(out, "<w:rPrChange "); iB < 0 || iChange < 0 || iB > iChange {
		t.Errorf("w:rPrChange is the last member of EG_RPrContent; w:b must precede it: %s", out)
	}
}

// TestPPrAddedPropertyPrecedesSectPr is the audit's second repro, and the one
// that is unambiguously schema-invalid: CT_PPr extends CT_PPrBase (an
// xsd:sequence) with rPr, sectPr, pPrChange in that order, so a w:jc emitted
// after a w:sectPr violates the sequence.
func TestPPrAddedPropertyPrecedesSectPr(t *testing.T) {
	src := `<w:pPr xmlns:w="` + NsWml + `"><w:sectPr><w:type w:val="nextPage"/></w:sectPr></w:pPr>`
	var pp CT_PPr
	if err := xmlb.UnmarshalWithSource([]byte(src), &pp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	pp.Jc = &CT_Jc{Val: "center"} // the SetAlignment path

	out := marshalProbe(&pp, "pPr")
	if iJc, iSect := strings.Index(out, "<w:jc "), strings.Index(out, "<w:sectPr>"); iJc < 0 || iSect < 0 || iJc > iSect {
		t.Errorf("w:jc must precede w:sectPr (CT_PPr is an xsd:sequence): %s", out)
	}
}

// TestPPrAddedPropertyBetweenCapturedSiblings checks the insertion point, not
// just the endpoints: a property added to a populated bag lands between the
// captured children it belongs between, not at either edge.
func TestPPrAddedPropertyBetweenCapturedSiblings(t *testing.T) {
	src := `<w:pPr xmlns:w="` + NsWml + `"><w:pStyle w:val="Body"/><w:ind w:left="720"/>` +
		`<w:rPr><w:b/></w:rPr></w:pPr>`
	var pp CT_PPr
	if err := xmlb.UnmarshalWithSource([]byte(src), &pp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	pp.Jc = &CT_Jc{Val: "both"}

	out := marshalProbe(&pp, "pPr")
	iInd, iJc, iRPr := strings.Index(out, "<w:ind "), strings.Index(out, "<w:jc "), strings.Index(out, "<w:rPr>")
	if iInd < 0 || iJc < 0 || iRPr < 0 {
		t.Fatalf("missing child in %s", out)
	}
	if iInd >= iJc || iJc >= iRPr {
		t.Errorf("w:jc belongs between w:ind and w:rPr in CT_PPr's sequence: %s", out)
	}
}

// TestTcPrAddedPropertyPrecedesTcPrChange covers the sweep's other docx
// sequence bags in the same shape: the trailing revision element must stay
// last when a property is added after parse.
func TestTcPrAddedPropertyPrecedesTcPrChange(t *testing.T) {
	src := `<w:tcPr xmlns:w="` + NsWml + `"><w:tcPrChange w:id="3" w:author="a"><w:tcPr/></w:tcPrChange></w:tcPr>`
	var tcp CT_TcPr
	if err := xmlb.UnmarshalWithSource([]byte(src), &tcp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	tcp.TcW = &CT_TblWidth{W: "1440", Type: "dxa"}

	out := marshalProbe(&tcp, "tcPr")
	if iW, iChange := strings.Index(out, "<w:tcW "), strings.Index(out, "<w:tcPrChange "); iW < 0 || iChange < 0 || iW > iChange {
		t.Errorf("w:tcW must precede w:tcPrChange (CT_TcPr is an xsd:sequence): %s", out)
	}
}

// TestTblPrAddedPropertyPrecedesTblPrChange is the CT_TblPr instance of the
// same class.
func TestTblPrAddedPropertyPrecedesTblPrChange(t *testing.T) {
	src := `<w:tblPr xmlns:w="` + NsWml + `"><w:tblW w:w="5000" w:type="pct"/>` +
		`<w:tblPrChange w:id="4" w:author="a"><w:tblPr/></w:tblPrChange></w:tblPr>`
	var tp CT_TblPr
	if err := xmlb.UnmarshalWithSource([]byte(src), &tp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	tp.Jc = &CT_Jc{Val: "center"}

	out := marshalProbe(&tp, "tblPr")
	if iJc, iChange := strings.Index(out, "<w:jc "), strings.Index(out, "<w:tblPrChange "); iJc < 0 || iChange < 0 || iJc > iChange {
		t.Errorf("w:jc must precede w:tblPrChange: %s", out)
	}
}

// TestSectPrAddedPropertyPrecedesLaterSibling covers the sweep's first
// hand-rolled property bag. CT_SectPr keeps its own child-name sequence rather
// than using the common/xml capture kit, and emitted anything the sequence did
// not cover after every captured child: a w:type set on a section parsed with a
// w:cols came out as <w:cols/><w:type/>, which EG_SectPrContents forbids.
func TestSectPrAddedPropertyPrecedesLaterSibling(t *testing.T) {
	src := `<w:sectPr xmlns:w="` + NsWml + `"><w:cols w:num="2"/></w:sectPr>`
	var sp CT_SectPr
	if err := xmlb.UnmarshalWithSource([]byte(src), &sp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	sp.Type = &CT_String{Val: "nextPage"}

	out := marshalProbe(&sp, "sectPr")
	if iType, iCols := strings.Index(out, "<w:type "), strings.Index(out, "<w:cols "); iType < 0 || iCols < 0 || iType > iCols {
		t.Errorf("w:type must precede w:cols in EG_SectPrContents: %s", out)
	}
}

// TestSectPrHeaderReferencesStayFirst checks the leading group: header/footer
// references precede every EG_SectPrContents child, so references added to a
// section parsed without any must not land after the captured properties.
func TestSectPrHeaderReferencesStayFirst(t *testing.T) {
	src := `<w:sectPr xmlns:w="` + NsWml + `"><w:pgSz w:w="12240" w:h="15840"/></w:sectPr>`
	var sp CT_SectPr
	if err := xmlb.UnmarshalWithSource([]byte(src), &sp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	sp.HeaderReference = append(sp.HeaderReference, &CT_HdrFtrRef{Type: "default", RID: "rId7"})

	out := marshalProbe(&sp, "sectPr")
	if iRef, iSz := strings.Index(out, "<w:headerReference"), strings.Index(out, "<w:pgSz "); iRef < 0 || iSz < 0 || iRef > iSz {
		t.Errorf("EG_HdrFtrReferences precedes EG_SectPrContents: %s", out)
	}
}

// TestSdtPrAddedPropertyPrecedesLaterSiblings covers the second hand-rolled
// bag. CT_SdtPr is an xsd:sequence ending in the control-type choice, so an
// alias added to a parsed content control must precede both the captured w:id
// and the control.
func TestSdtPrAddedPropertyPrecedesLaterSiblings(t *testing.T) {
	src := `<w:sdtPr xmlns:w="` + NsWml + `"><w:id w:val="1"/>` +
		`<w:docPartObj><w:docPartGallery w:val="x"/></w:docPartObj></w:sdtPr>`
	var pr CT_SdtPr
	if err := xmlb.UnmarshalWithSource([]byte(src), &pr); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	pr.SetAlias("Title")

	out := marshalProbe(&pr, "sdtPr")
	iAlias, iID, iCtl := strings.Index(out, "<w:alias "), strings.Index(out, "<w:id "), strings.Index(out, "<w:docPartObj")
	if iAlias < 0 || iID < 0 || iCtl < 0 {
		t.Fatalf("missing child in %s", out)
	}
	if iAlias >= iID || iID >= iCtl {
		t.Errorf("CT_SdtPr's sequence is alias, id, then the control choice: %s", out)
	}
}

// TestSdtPrAddedPropertyPrecedesRawSchemaChild checks the same insertion
// against a child the model preserves raw rather than types: w:dataBinding
// follows w:tag in the sequence, so a tag added after parse belongs before it.
func TestSdtPrAddedPropertyPrecedesRawSchemaChild(t *testing.T) {
	src := `<w:sdtPr xmlns:w="` + NsWml + `"><w:dataBinding w:xpath="/a/b"/></w:sdtPr>`
	var pr CT_SdtPr
	if err := xmlb.UnmarshalWithSource([]byte(src), &pr); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	pr.SetTag("t")

	out := marshalProbe(&pr, "sdtPr")
	if iTag, iBind := strings.Index(out, "<w:tag "), strings.Index(out, "<w:dataBinding"); iTag < 0 || iBind < 0 || iTag > iBind {
		t.Errorf("w:tag must precede w:dataBinding: %s", out)
	}
}

// TestPropertyBagsReplayOutOfOrderSourceVerbatim is the byte-identity half of
// C329 and the constraint that makes the fix safe: real producers emit property
// children out of schema order and Word tolerates it, so a zero-modification
// save must reproduce the source exactly. Rank ordering applies only to what
// this library adds — nothing already captured is ever moved.
func TestPropertyBagsReplayOutOfOrderSourceVerbatim(t *testing.T) {
	cases := []struct {
		name  string
		src   string
		local string
		parse func([]byte) (any, error)
	}{
		{
			name:  "rPr sz before b",
			local: "rPr",
			src:   `<w:rPr xmlns:w="` + NsWml + `"><w:sz w:val="24"/><w:b/><w:rFonts w:ascii="Arial"/></w:rPr>`,
			parse: func(b []byte) (any, error) { v := &CT_RPr{}; return v, xmlb.UnmarshalWithSource(b, v) },
		},
		{
			name:  "pPr jc after sectPr",
			local: "pPr",
			src: `<w:pPr xmlns:w="` + NsWml + `"><w:sectPr><w:type w:val="nextPage"/></w:sectPr>` +
				`<w:jc w:val="center"/><w:pStyle w:val="Body"/></w:pPr>`,
			parse: func(b []byte) (any, error) { v := &CT_PPr{}; return v, xmlb.UnmarshalWithSource(b, v) },
		},
		{
			name:  "tcPr reversed",
			local: "tcPr",
			src:   `<w:tcPr xmlns:w="` + NsWml + `"><w:vAlign w:val="center"/><w:gridSpan w:val="2"/><w:tcW w:w="0" w:type="auto"/></w:tcPr>`,
			parse: func(b []byte) (any, error) { v := &CT_TcPr{}; return v, xmlb.UnmarshalWithSource(b, v) },
		},
		{
			name:  "tblPr reversed",
			local: "tblPr",
			src:   `<w:tblPr xmlns:w="` + NsWml + `"><w:tblLook w:val="04A0"/><w:tblW w:w="0" w:type="auto"/><w:tblStyle w:val="Grid"/></w:tblPr>`,
			parse: func(b []byte) (any, error) { v := &CT_TblPr{}; return v, xmlb.UnmarshalWithSource(b, v) },
		},
		{
			name:  "trPr reversed",
			local: "trPr",
			src:   `<w:trPr xmlns:w="` + NsWml + `"><w:tblHeader/><w:trHeight w:val="300"/><w:cnfStyle w:val="100000000000"/></w:trPr>`,
			parse: func(b []byte) (any, error) { v := &CT_TrPr{}; return v, xmlb.UnmarshalWithSource(b, v) },
		},
		{
			// The hand-rolled bags hold the same contract.
			name:  "sectPr cols before type",
			local: "sectPr",
			src:   `<w:sectPr xmlns:w="` + NsWml + `"><w:cols w:num="2"/><w:type w:val="nextPage"/><w:pgSz w:w="12240"/></w:sectPr>`,
			parse: func(b []byte) (any, error) { v := &CT_SectPr{}; return v, xmlb.UnmarshalWithSource(b, v) },
		},
		{
			name:  "sdtPr id before alias",
			local: "sdtPr",
			src:   `<w:sdtPr xmlns:w="` + NsWml + `"><w:id w:val="1"/><w:alias w:val="Title"/><w:tag w:val="t"/></w:sdtPr>`,
			parse: func(b []byte) (any, error) { v := &CT_SdtPr{}; return v, xmlb.UnmarshalWithSource(b, v) },
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v, err := tc.parse([]byte(tc.src))
			if err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			// Whether the probe re-declares the w prefix on the root depends on
			// the marshal path, not on the child order under test; strip the
			// declaration from both sides.
			wns := ` xmlns:w="` + NsWml + `"`
			want := strings.Replace(tc.src, wns, "", 1)
			if got := strings.Replace(marshalProbe(v, tc.local), wns, "", 1); got != want {
				t.Errorf("zero-modification replay changed the source\n got: %s\nwant: %s", got, want)
			}
		})
	}
}

// TestDocxChildCaptureSchemaOrder pins the rank tables the insertion relies on.
// Every docx type carrying a CapturedChildren field is listed with its XSD
// content model transcribed from ISO/IEC 29500-4 wml.xsd; the check asserts the
// struct's field declaration order matches, because the insertion ranks
// children by field index.
func TestDocxChildCaptureSchemaOrder(t *testing.T) {
	testutil.CheckSchemaChildOrder(t, wmlBuilder, NsWml, []testutil.SchemaOrder{
		{
			// CT_PPr = CT_PPrBase (xsd:sequence) + rPr, sectPr, pPrChange.
			// w:textboxTightWrap (between textAlignment and outlineLvl) is not
			// modeled and keeps its captured position.
			Name: "CT_PPr", New: func() any { return &CT_PPr{} }, Model: testutil.Sequence,
			Children: []string{
				"pStyle", "keepNext", "keepLines", "pageBreakBefore", "framePr", "widowControl",
				"numPr", "suppressLineNumbers", "pBdr", "shd", "tabs", "suppressAutoHyphens",
				"kinsoku", "wordWrap", "overflowPunct", "topLinePunct", "autoSpaceDE",
				"autoSpaceDN", "bidi", "adjustRightInd", "snapToGrid", "spacing", "ind",
				"contextualSpacing", "mirrorIndents", "suppressOverlap", "jc", "textDirection",
				"textAlignment", "outlineLvl", "divId", "cnfStyle", "rPr", "sectPr", "pPrChange",
			},
		},
		{
			// CT_RPr = EG_RPrContent = EG_RPrBase* (a repeated xsd:choice, so
			// the property order is free) followed by rPrChange, which the
			// sequence pins last. mc:AlternateContent (Word wraps w:rFonts in
			// one) and w14:ligatures are extensions outside the schema; they
			// sit at the slot Word emits them in.
			Name: "CT_RPr", New: func() any { return &CT_RPr{} }, Model: testutil.Sequence,
			Children: []string{
				"rStyle", "AlternateContent", "rFonts", "b", "bCs", "i", "iCs", "caps",
				"smallCaps", "strike", "dstrike", "outline", "shadow", "emboss", "imprint",
				"noProof", "snapToGrid", "vanish", "webHidden", "color", "spacing", "w", "kern",
				"position", "sz", "szCs", "highlight", "u", "effect", "bdr", "shd", "fitText",
				"vertAlign", "rtl", "cs", "em", "lang", "specVanish", "oMath", "rPrChange",
				"ligatures",
			},
		},
		{
			// CT_TblPr = CT_TblPrBase (xsd:sequence) + tblPrChange.
			Name: "CT_TblPr", New: func() any { return &CT_TblPr{} }, Model: testutil.Sequence,
			Children: []string{
				"tblStyle", "tblpPr", "tblOverlap", "bidiVisual", "tblStyleRowBandSize",
				"tblStyleColBandSize", "tblW", "jc", "tblCellSpacing", "tblInd", "tblBorders",
				"shd", "tblLayout", "tblCellMar", "tblLook", "tblCaption", "tblDescription",
				"tblPrChange",
			},
		},
		{
			// CT_TrPr = CT_TrPrBase (a repeated xsd:choice — the row property
			// order is free) + ins, del, trPrChange, which are ordered.
			Name: "CT_TrPr", New: func() any { return &CT_TrPr{} }, Model: testutil.Sequence,
			Children: []string{
				"cnfStyle", "divId", "gridBefore", "gridAfter", "wBefore", "wAfter", "cantSplit",
				"trHeight", "tblHeader", "tblCellSpacing", "jc", "hidden", "ins", "del",
				"trPrChange",
			},
		},
		{
			// CT_TcPr = CT_TcPrBase (xsd:sequence) + EG_CellMarkupElements
			// (cellIns, cellDel, cellMerge) + tcPrChange. w:headers, the last
			// CT_TcPrBase member, is not modeled.
			Name: "CT_TcPr", New: func() any { return &CT_TcPr{} }, Model: testutil.Sequence,
			Children: []string{
				"cnfStyle", "tcW", "gridSpan", "hMerge", "vMerge", "tcBorders", "shd", "noWrap",
				"tcMar", "textDirection", "tcFitText", "vAlign", "hideMark", "cellIns", "cellDel",
				"cellMerge", "tcPrChange",
			},
		},
		{
			// CT_Styles (the styles part root) is an xsd:sequence.
			Name: "CT_Styles", New: func() any { return &CT_Styles{} }, Model: testutil.Sequence,
			Children: []string{"docDefaults", "latentStyles", "style"},
		},
		{
			// CT_Background models no element children at all: its content is
			// the VML fill (xsd:any) plus an unmodeled w:drawing, both carried
			// as raw captured children, so there is no rank to get wrong.
			Name: "CT_Background", New: func() any { return &CT_Background{} }, Model: testutil.Sequence,
			Children: nil,
		},
	})
}
