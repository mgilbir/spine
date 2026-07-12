package docx

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/mgilbir/spine/common/omml"
)

// mathRun builds a plain-text math run.
func mathRun(text string) *omml.Run {
	return &omml.Run{Items: []omml.RunChild{&omml.Text{Value: text}}}
}

// mathArg wraps a single run in an argument element.
func mathArg(text string) *omml.Element {
	return &omml.Element{Items: []omml.MathItem{mathRun(text)}}
}

// TestMathZonesTypedRead: an opened document's raw-captured equation is
// parseable into the typed model on demand, and the raw bytes stay the
// storage format — a zero-modification save re-emits them verbatim.
func TestMathZonesTypedRead(t *testing.T) {
	eq := `<m:oMath><m:r><m:rPr><m:sty m:val="p"/></m:rPr><w:rPr><w:b/></w:rPr><m:t>x=</m:t></m:r>` +
		`<m:f><m:num><m:r><m:t>1</m:t></m:r></m:num><m:den><m:r><m:t>2</m:t></m:r></m:den></m:f></m:oMath>`
	body := `<w:body><w:p><w:r><w:t>Eq: </w:t></w:r>` + eq + `</w:p><w:sectPr/></w:body>`
	fixture := fixtureWithDocument(t, fixtureWNS+" "+mathNSDecl, body)

	doc, err := OpenReader(bytes.NewReader(fixture), int64(len(fixture)))
	if err != nil {
		t.Fatal(err)
	}
	paras := doc.Paragraphs()
	if len(paras) != 1 {
		t.Fatalf("paragraphs = %d, want 1", len(paras))
	}

	zones, err := paras[0].MathZones()
	if err != nil {
		t.Fatalf("MathZones: %v", err)
	}
	if len(zones) != 1 {
		t.Fatalf("zones = %d, want 1", len(zones))
	}
	if got, want := zones[0].Text(), "x=12"; got != want {
		t.Errorf("Text() = %q, want %q", got, want)
	}

	// Typed structure: run (with math rPr and raw-captured w:rPr) + fraction.
	if len(zones[0].Items) != 2 {
		t.Fatalf("items = %d, want 2", len(zones[0].Items))
	}
	r, ok := zones[0].Items[0].(*omml.Run)
	if !ok {
		t.Fatalf("Items[0] = %T, want *omml.Run", zones[0].Items[0])
	}
	if r.RPr == nil || r.RPr.Sty == nil || r.RPr.Sty.Val != "p" {
		t.Error("math run properties not parsed")
	}
	raw, ok := r.Items[0].(*omml.Raw)
	if !ok || raw.Local != "rPr" {
		t.Errorf("w:rPr not raw-captured in position, got %T", r.Items[0])
	}
	if _, ok := zones[0].Items[1].(*omml.Fraction); !ok {
		t.Errorf("Items[1] = %T, want *omml.Fraction", zones[0].Items[1])
	}

	// Zero-modification save: the equation bytes are written back verbatim.
	saved, err := doc.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	data, ok := zipEntry(t, saved, "word/document.xml")
	if !ok {
		t.Fatal("word/document.xml missing")
	}
	if !strings.Contains(string(data), eq) {
		t.Errorf("saved document.xml does not contain the original equation bytes:\n%s", data)
	}
}

// TestMathParasTypedRead: display equations (m:oMathPara) parse typed too.
func TestMathParasTypedRead(t *testing.T) {
	para := `<m:oMathPara><m:oMathParaPr><m:jc m:val="center"/></m:oMathParaPr>` +
		`<m:oMath><m:r><m:t>a+b</m:t></m:r></m:oMath></m:oMathPara>`
	body := `<w:body><w:p>` + para + `</w:p><w:sectPr/></w:body>`
	fixture := fixtureWithDocument(t, fixtureWNS+" "+mathNSDecl, body)

	doc, err := OpenReader(bytes.NewReader(fixture), int64(len(fixture)))
	if err != nil {
		t.Fatal(err)
	}
	paras, err := doc.Paragraphs()[0].MathParas()
	if err != nil {
		t.Fatalf("MathParas: %v", err)
	}
	if len(paras) != 1 {
		t.Fatalf("math paras = %d, want 1", len(paras))
	}
	if paras[0].OMathParaPr == nil || paras[0].OMathParaPr.Jc == nil || paras[0].OMathParaPr.Jc.Val != "center" {
		t.Error("oMathParaPr not parsed")
	}
	if got, want := paras[0].Text(), "a+b"; got != want {
		t.Errorf("Text() = %q, want %q", got, want)
	}
}

// TestAddMathOnCreatedDocument: create → AddMath → save → reopen. The C173
// machinery must declare xmlns:m at the root, the document must reopen
// cleanly, and the math must parse back into an identical typed model.
func TestAddMathOnCreatedDocument(t *testing.T) {
	doc := Create()
	p := doc.AddParagraphWithText("Formula: ")

	m := &omml.OMath{Items: []omml.MathItem{
		mathRun("x="),
		&omml.Fraction{
			FPr: &omml.FractionPr{Type: &omml.FType{Val: "bar"}},
			Num: mathArg("1"),
			Den: mathArg("2"),
		},
		&omml.Radical{
			RadPr: &omml.RadPr{DegHide: &omml.OnOff{Val: "1"}},
			Deg:   &omml.Element{},
			E:     mathArg("y"),
		},
	}}
	if err := p.AddMath(m); err != nil {
		t.Fatalf("AddMath: %v", err)
	}

	data, err := doc.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	docXML, ok := zipEntry(t, data, "word/document.xml")
	if !ok {
		t.Fatal("word/document.xml missing")
	}
	if !strings.Contains(string(docXML), `xmlns:m="http://schemas.openxmlformats.org/officeDocument/2006/math"`) {
		t.Error("math namespace not declared on the document root")
	}
	// The backfilled child order must keep the pre-existing text run before
	// the appended math.
	ti, mi := strings.Index(string(docXML), "Formula: "), strings.Index(string(docXML), "<m:oMath>")
	if ti < 0 || mi < 0 || ti > mi {
		t.Errorf("text run and math zone out of order: text=%d math=%d", ti, mi)
	}

	reopened, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("saved document does not reopen: %v", err)
	}
	var zones []*omml.OMath
	for _, rp := range reopened.Paragraphs() {
		zs, err := rp.MathZones()
		if err != nil {
			t.Fatalf("MathZones after reopen: %v", err)
		}
		zones = append(zones, zs...)
	}
	if len(zones) != 1 {
		t.Fatalf("zones after reopen = %d, want 1", len(zones))
	}
	if got, want := zones[0].Text(), "x=12y"; got != want {
		t.Errorf("Text() = %q, want %q", got, want)
	}
	if !reflect.DeepEqual(m, zones[0]) {
		t.Errorf("typed model does not survive save/reopen:\nwrote: %#v\n read: %#v", m, zones[0])
	}
}

// TestAddMathParaOnCreatedDocument: the oMathPara variant of AddMath.
func TestAddMathParaOnCreatedDocument(t *testing.T) {
	doc := Create()
	p := doc.AddParagraph()

	mp := &omml.OMathPara{
		OMathParaPr: &omml.OMathParaPr{Jc: &omml.MathJc{Val: "center"}},
		OMath: []*omml.OMath{
			{Items: []omml.MathItem{mathRun("a")}},
			{Items: []omml.MathItem{mathRun("b")}},
		},
	}
	if err := p.AddMathPara(mp); err != nil {
		t.Fatalf("AddMathPara: %v", err)
	}

	data, err := doc.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	reopened, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("saved document does not reopen: %v", err)
	}
	var paras []*omml.OMathPara
	for _, rp := range reopened.Paragraphs() {
		ps, err := rp.MathParas()
		if err != nil {
			t.Fatalf("MathParas after reopen: %v", err)
		}
		paras = append(paras, ps...)
	}
	if len(paras) != 1 {
		t.Fatalf("math paras after reopen = %d, want 1", len(paras))
	}
	if !reflect.DeepEqual(mp, paras[0]) {
		t.Errorf("typed model does not survive save/reopen:\nwrote: %#v\n read: %#v", mp, paras[0])
	}
}

// TestMathZonesEmpty: a paragraph without math returns empty slices, no error.
func TestMathZonesEmpty(t *testing.T) {
	doc := Create()
	p := doc.AddParagraphWithText("no math here")
	zones, err := p.MathZones()
	if err != nil || len(zones) != 0 {
		t.Errorf("MathZones = %v, %v; want empty, nil", zones, err)
	}
	paras, err := p.MathParas()
	if err != nil || len(paras) != 0 {
		t.Errorf("MathParas = %v, %v; want empty, nil", paras, err)
	}
}

// TestAddMathBuilderErrorSurfaces: a model referencing a namespace the WML
// builder cannot bind must fail AddMath instead of storing broken bytes.
func TestAddMathBuilderErrorSurfaces(t *testing.T) {
	doc := Create()
	p := doc.AddParagraph()
	m := &omml.OMath{Items: []omml.MathItem{
		&omml.Raw{Local: "foo", Space: "urn:unbound-namespace"},
	}}
	if err := p.AddMath(m); err == nil {
		t.Error("AddMath accepted content with an unbindable namespace")
	}
}
