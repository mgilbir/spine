package docx

import (
	"strings"
	"testing"
)

// fixtureWNS14 extends the minimal namespace set with the Word 2010
// extension namespace used by w14:ligatures.
const fixtureWNS14 = fixtureWNS + ` xmlns:w14="http://schemas.microsoft.com/office/word/2010/wordml"`

// w:fldChar children (w:ffData form-field definitions, w:fldData) were
// silently dropped: CT_FldChar modeled only the attributes, so legacy Word
// forms lost their checkbox/text-input definitions on save.
func TestFldCharChildrenRoundTrip(t *testing.T) {
	fld := `<w:r><w:fldChar w:fldCharType="begin"><w:ffData><w:name w:val="Check1"/><w:enabled/>` +
		`<w:calcOnExit w:val="0"/><w:checkBox><w:sizeAuto/><w:default w:val="0"/></w:checkBox></w:ffData>` +
		`</w:fldChar></w:r>`
	fixture := fixtureWithDocument(t, fixtureWNS,
		`<w:body><w:p>`+fld+`</w:p></w:body>`)
	out := openSave(t, fixture)
	if !strings.Contains(out, fld) {
		t.Fatalf("fldChar children not preserved.\nwant substring:\n%s\ngot document:\n%s", fld, out)
	}
}

// A childless w:fldChar keeps its self-closing form.
func TestFldCharEmptySelfCloses(t *testing.T) {
	fixture := fixtureWithDocument(t, fixtureWNS,
		`<w:body><w:p><w:r><w:fldChar w:fldCharType="end"/></w:r></w:p></w:body>`)
	out := openSave(t, fixture)
	if !strings.Contains(out, `<w:fldChar w:fldCharType="end"/>`) {
		t.Fatalf("childless fldChar not self-closing:\n%s", out)
	}
}

// w:customMarkFollows on w:footnoteReference was not modeled and vanished,
// detaching custom footnote marks from their references.
func TestFootnoteReferenceCustomMarkFollows(t *testing.T) {
	ref := `<w:footnoteReference w:customMarkFollows="1" w:id="4"/>`
	fixture := fixtureWithDocument(t, fixtureWNS,
		`<w:body><w:p><w:r>`+ref+`<w:t>*</w:t></w:r></w:p></w:body>`)
	out := openSave(t, fixture)
	if !strings.Contains(out, ref) {
		t.Fatalf("customMarkFollows dropped:\n%s", out)
	}
}

// w14:ligatures (always the last w:rPr child in Word output) was silently
// dropped from run properties.
func TestRPrLigaturesRoundTrip(t *testing.T) {
	rpr := `<w:rPr><w:kern w:val="0"/><w:sz w:val="18"/><w14:ligatures w14:val="none"/></w:rPr>`
	fixture := fixtureWithDocument(t, fixtureWNS14,
		`<w:body><w:p><w:r>`+rpr+`<w:t>x</w:t></w:r></w:p></w:body>`)
	out := openSave(t, fixture)
	if !strings.Contains(out, rpr) {
		t.Fatalf("w14:ligatures not preserved in position.\nwant substring:\n%s\ngot document:\n%s", rpr, out)
	}
}

// Word attribute orders: w:hyperlink (tgtFrame/tooltip before history),
// w:p (rsidDel between rsidRPr and rsidRDefault), and w:spacing (the *Lines
// variant precedes each twip value).
func TestWordAttributeOrders(t *testing.T) {
	body := `<w:body>` +
		`<w:p w:rsidR="00397A5B" w:rsidRPr="005B3203" w:rsidDel="00397A5B" w:rsidRDefault="00397A5B" w:rsidP="00CE454A">` +
		`<w:pPr><w:spacing w:beforeLines="40" w:before="96" w:afterLines="40" w:after="96"/></w:pPr>` +
		`<w:hyperlink r:id="rId1" w:tgtFrame="_blank" w:history="1"><w:r><w:t>x</w:t></w:r></w:hyperlink>` +
		`</w:p></w:body>`
	fixture := fixtureWithDocument(t, fixtureWNS, body)
	out := openSave(t, fixture)
	for _, want := range []string{
		`<w:p w:rsidR="00397A5B" w:rsidRPr="005B3203" w:rsidDel="00397A5B" w:rsidRDefault="00397A5B" w:rsidP="00CE454A">`,
		`<w:spacing w:beforeLines="40" w:before="96" w:afterLines="40" w:after="96"/>`,
		`<w:hyperlink r:id="rId1" w:tgtFrame="_blank" w:history="1">`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("attribute order not preserved.\nwant substring:\n%s\ngot document:\n%s", want, out)
		}
	}
}

// w:cnfStyle sits at its XSD position: after the other pPrBase children,
// immediately before w:rPr — never ahead of w:pStyle or w:jc.
func TestPPrCnfStylePosition(t *testing.T) {
	ppr := `<w:pPr><w:pStyle w:val="NoSpacing"/><w:jc w:val="center"/>` +
		`<w:cnfStyle w:val="000000100000" w:firstRow="0"/><w:rPr><w:b/></w:rPr></w:pPr>`
	fixture := fixtureWithDocument(t, fixtureWNS,
		`<w:body><w:p>`+ppr+`<w:r><w:t>x</w:t></w:r></w:p></w:body>`)
	out := openSave(t, fixture)
	if !strings.Contains(out, ppr) {
		t.Fatalf("cnfStyle position not preserved.\nwant substring:\n%s\ngot document:\n%s", ppr, out)
	}
}

// A bare <w:vMerge/> ("continue") must not gain an empty w:val attribute.
func TestBareVMergeKeepsNoVal(t *testing.T) {
	tbl := `<w:tbl><w:tblPr><w:tblW w:w="0" w:type="auto"/></w:tblPr><w:tblGrid><w:gridCol w:w="100"/></w:tblGrid>` +
		`<w:tr><w:tc><w:tcPr><w:vMerge w:val="restart"/></w:tcPr><w:p/></w:tc></w:tr>` +
		`<w:tr><w:tc><w:tcPr><w:vMerge/></w:tcPr><w:p/></w:tc></w:tr></w:tbl>`
	fixture := fixtureWithDocument(t, fixtureWNS, `<w:body>`+tbl+`</w:body>`)
	out := openSave(t, fixture)
	if strings.Contains(out, `<w:vMerge w:val=""/>`) {
		t.Fatalf("bare vMerge gained an empty w:val:\n%s", out)
	}
	if !strings.Contains(out, `<w:vMerge/>`) || !strings.Contains(out, `<w:vMerge w:val="restart"/>`) {
		t.Fatalf("vMerge forms not preserved:\n%s", out)
	}
}

// An empty w:sdtEndPr inside a run-level SDT keeps its self-closed form:
// the marshal used to write its (empty) raw content unconditionally, which
// defeated the builder's collapse-empty capture.
func TestRunSdtEndPrSelfCloses(t *testing.T) {
	sdt := `<w:sdt><w:sdtPr><w:id w:val="1"/></w:sdtPr><w:sdtEndPr/>` +
		`<w:sdtContent><w:r><w:t>x</w:t></w:r></w:sdtContent></w:sdt>`
	fixture := fixtureWithDocument(t, fixtureWNS,
		`<w:body><w:p>`+sdt+`</w:p></w:body>`)
	out := openSave(t, fixture)
	if !strings.Contains(out, `<w:sdtEndPr/>`) {
		t.Fatalf("empty sdtEndPr expanded:\n%s", out)
	}
}
