package docx

import (
	"strings"
	"testing"

	"github.com/mgilbir/spine/common/validate"
)

// TestSetBasedOnRefusesSelfReference pins the setter half of C501.
func TestSetBasedOnRefusesSelfReference(t *testing.T) {
	doc := Create()
	s := doc.Styles().AddParagraphStyle("Mine", "Mine")
	s.SetBasedOn("Normal")
	s.SetBasedOn("Mine")
	if got := s.BasedOn(); got != "Normal" {
		t.Errorf("SetBasedOn(own id) took effect: basedOn = %q, want %q", got, "Normal")
	}
}

// TestSetBasedOnRefusesCycle pins the transitive case: A based on B, then B
// based on A closes a cycle Word repairs or misrenders.
func TestSetBasedOnRefusesCycle(t *testing.T) {
	doc := Create()
	m := doc.Styles()
	a := m.AddParagraphStyle("A", "A")
	b := m.AddParagraphStyle("B", "B")
	a.SetBasedOn("B")
	b.SetBasedOn("A")
	if got := b.BasedOn(); got != "" {
		t.Errorf("SetBasedOn closed an A->B->A cycle: B.basedOn = %q, want empty", got)
	}
	if got := a.BasedOn(); got != "B" {
		t.Errorf("the refused call disturbed the other style: A.basedOn = %q, want %q", got, "B")
	}
	if rep := doc.Validate(); rep.HasErrors() {
		t.Errorf("Validate reported errors on a cycle-free document: %v", rep)
	}
}

// TestValidateReportsStyleCycle pins the Validate half of C501: a cycle can
// still arrive with the source package or be introduced by merge rewriting
// basedOn chains, and nothing reported it — the reference check only catches a
// basedOn pointing at a style that does not exist.
func TestValidateReportsStyleCycle(t *testing.T) {
	fixture := stylesFixture(t,
		`<w:style w:type="paragraph" w:styleId="A"><w:name w:val="A"/><w:basedOn w:val="B"/></w:style>`+
			`<w:style w:type="paragraph" w:styleId="B"><w:name w:val="B"/><w:basedOn w:val="A"/></w:style>`)
	doc := openFixture(t, fixture)
	rep := doc.Validate()
	if !reportHasCode(rep, codeStyleCycle) {
		t.Errorf("Validate did not report the basedOn cycle: %v", rep)
	}
	if rep.HasErrors() {
		t.Errorf("the cycle was reported at error severity, which would block Save: %v", rep)
	}
}

// TestValidateAcceptsDeepStyleChain guards against a false positive on a long
// but acyclic inheritance chain, and on a style based on a style that does not
// exist (already reported separately as a dangling reference).
func TestValidateAcceptsDeepStyleChain(t *testing.T) {
	var b strings.Builder
	b.WriteString(`<w:style w:type="paragraph" w:styleId="S0"><w:name w:val="S0"/></w:style>`)
	for i := 1; i < 30; i++ {
		b.WriteString(`<w:style w:type="paragraph" w:styleId="S` + itoa(i) + `"><w:name w:val="S` + itoa(i) +
			`"/><w:basedOn w:val="S` + itoa(i-1) + `"/></w:style>`)
	}
	b.WriteString(`<w:style w:type="paragraph" w:styleId="Dangling"><w:name w:val="D"/><w:basedOn w:val="Nope"/></w:style>`)
	doc := openFixture(t, stylesFixture(t, b.String()))
	if rep := doc.Validate(); reportHasCode(rep, codeStyleCycle) {
		t.Errorf("Validate reported a cycle on an acyclic chain: %v", rep)
	}
}

// TestStyleCycleValidationSurvivesDuplicateIDs guards the walk against a
// package with two styles sharing an id, which must not send it into a loop.
func TestStyleCycleValidationSurvivesDuplicateIDs(t *testing.T) {
	doc := openFixture(t, stylesFixture(t,
		`<w:style w:type="paragraph" w:styleId="Dup"><w:name w:val="A"/></w:style>`+
			`<w:style w:type="paragraph" w:styleId="Dup"><w:name w:val="B"/><w:basedOn w:val="Dup"/></w:style>`))
	_ = doc.Validate()
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// reportHasCode reports whether any finding carries the given code.
func reportHasCode(rep validate.Report, code string) bool {
	for _, e := range rep {
		if e.Code == code {
			return true
		}
	}
	return false
}

// stylesFixture builds a docx whose styles part carries the given style
// definitions.
func stylesFixture(t *testing.T, styles string) []byte {
	t.Helper()
	const ct = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types"><Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/><Default Extension="xml" ContentType="application/xml"/><Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/><Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/></Types>`
	const rels = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships"><Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/></Relationships>`
	return buildFixtureDocx(t, map[string]string{
		"[Content_Types].xml": ct,
		"_rels/.rels":         fixtureRootRels,
		"word/document.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
			`<w:document ` + fixtureWNS + `><w:body><w:p/></w:body></w:document>`,
		"word/_rels/document.xml.rels": rels,
		"word/styles.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\r\n" +
			`<w:styles ` + fixtureWNS + `>` + styles + `</w:styles>`,
	})
}
