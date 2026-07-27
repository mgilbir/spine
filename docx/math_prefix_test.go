package docx

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mgilbir/spine/common/omml"
)

// C435: unmarshalMath binds every prefix the document root declares, so Word
// content under any root-declared alias parses; marshalMathContent used the
// fixed WML prefix set, so a raw child in wps:, w16:, v:, o:, w10: or a vendor
// namespace had no prefix to write. The documented MathZones → AddMath
// read-modify-write loop then failed on legitimately-parsed input.
func TestMathRoundTripThroughRootDeclaredNamespaces(t *testing.T) {
	tests := []struct {
		name string
		decl string
		raw  string
	}{
		{
			"wordprocessing shape",
			`xmlns:wps="http://schemas.microsoft.com/office/word/2010/wordprocessingShape"`,
			`<wps:txbx><wps:t>k</wps:t></wps:txbx>`,
		},
		{
			"vml office",
			`xmlns:o="urn:schemas-microsoft-com:office:office"`,
			`<o:lock v:ext="edit"/>`,
		},
		{
			"word 2016",
			`xmlns:w16="http://schemas.microsoft.com/office/word/2018/wordml"`,
			`<w16:commentEx/>`,
		},
		{
			"vendor uri",
			`xmlns:foo="urn:x-foo"`,
			`<foo:bar a="1"/>`,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			eq := `<m:oMath><m:r>` + tc.raw + `<m:t>z</m:t></m:r></m:oMath>`
			body := `<w:body><w:p>` + eq + `</w:p><w:sectPr/></w:body>`
			roots := fixtureWNS + " " + mathNSDecl + " " + tc.decl
			// v: is used by the o:lock attribute above.
			roots += ` xmlns:v="urn:schemas-microsoft-com:vml"`
			fixture := fixtureWithDocument(t, roots, body)

			doc, err := OpenReader(bytes.NewReader(fixture), int64(len(fixture)))
			if err != nil {
				t.Fatal(err)
			}
			paras := doc.Paragraphs()
			zones, err := paras[0].MathZones()
			if err != nil {
				t.Fatalf("MathZones: %v", err)
			}
			if len(zones) != 1 {
				t.Fatalf("zones = %d, want 1", len(zones))
			}
			// Read-modify-write: the parsed zone must be writable again.
			if err := paras[0].AddMath(zones[0]); err != nil {
				t.Fatalf("AddMath on a parsed zone: %v", err)
			}
			data, err := doc.SaveBytes()
			if err != nil {
				t.Fatalf("SaveBytes: %v", err)
			}
			docXML, ok := zipEntry(t, data, "word/document.xml")
			if !ok {
				t.Fatal("word/document.xml missing")
			}
			if n := strings.Count(string(docXML), tc.raw); n != 2 {
				t.Errorf("raw child %q appears %d times, want 2\n%s", tc.raw, n, docXML)
			}
			if _, err := OpenReader(bytes.NewReader(data), int64(len(data))); err != nil {
				t.Fatalf("saved document does not reopen: %v", err)
			}
		})
	}
}

// A raw child that declares its own namespace on the element is bound by
// nothing the Builder knows; the capture carries the declaration and must
// replay it (this is the audit's literal reproduction).
func TestMathRawChildWithInlineNamespaceDeclaration(t *testing.T) {
	eq := `<m:oMath><m:r><foo:bar xmlns:foo="urn:x-foo" a="1"/><m:t>z</m:t></m:r></m:oMath>`
	body := `<w:body><w:p>` + eq + `</w:p><w:sectPr/></w:body>`
	fixture := fixtureWithDocument(t, fixtureWNS+" "+mathNSDecl, body)

	doc, err := OpenReader(bytes.NewReader(fixture), int64(len(fixture)))
	if err != nil {
		t.Fatal(err)
	}
	zones, err := doc.Paragraphs()[0].MathZones()
	if err != nil {
		t.Fatalf("MathZones: %v", err)
	}
	if err := doc.Paragraphs()[0].AddMath(zones[0]); err != nil {
		t.Fatalf("AddMath on a parsed zone: %v", err)
	}
	data, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	docXML, _ := zipEntry(t, data, "word/document.xml")
	want := `<foo:bar xmlns:foo="urn:x-foo" a="1"/>`
	if n := strings.Count(string(docXML), want); n != 2 {
		t.Errorf("inline-declared raw child appears %d times, want 2\n%s", n, docXML)
	}
}

// The m:t lexical captures ride the docx parse path, which must register the
// source bytes for them to work at all.
func TestMathTextCapturesSurviveTheDocxPath(t *testing.T) {
	eq := `<m:oMath><m:r><m:t m:foo="1"></m:t></m:r></m:oMath>`
	body := `<w:body><w:p>` + eq + `</w:p><w:sectPr/></w:body>`
	fixture := fixtureWithDocument(t, fixtureWNS+" "+mathNSDecl, body)

	doc, err := OpenReader(bytes.NewReader(fixture), int64(len(fixture)))
	if err != nil {
		t.Fatal(err)
	}
	zones, err := doc.Paragraphs()[0].MathZones()
	if err != nil {
		t.Fatalf("MathZones: %v", err)
	}
	txt := zones[0].Items[0].(*omml.Run).Items[0].(*omml.Text)
	if txt.CapturedAttrs == nil {
		t.Error("m:t attributes not captured on the docx parse path")
	}
	if err := doc.Paragraphs()[0].AddMath(zones[0]); err != nil {
		t.Fatalf("AddMath: %v", err)
	}
	data, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	docXML, _ := zipEntry(t, data, "word/document.xml")
	if n := strings.Count(string(docXML), `<m:t m:foo="1"></m:t>`); n != 2 {
		t.Errorf("m:t capture not replayed (%d occurrences)\n%s", n, docXML)
	}
}
