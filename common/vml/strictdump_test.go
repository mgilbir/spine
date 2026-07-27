package vml

import (
	"encoding/xml"
	"os"
	"testing"
)

// TestDumpForExternalValidation writes the package's marshal output for a set
// of realistic fragments to the path in SPINE_VML_DUMP, so an out-of-process
// strict parser (xmllint, ElementTree) can be pointed at it. It is a no-op in
// a normal test run; the in-process strict check is namespaceWellFormed.
func TestDumpForExternalValidation(t *testing.T) {
	path := os.Getenv("SPINE_VML_DUMP")
	if path == "" {
		t.Skip("SPINE_VML_DUMP not set")
	}
	inputs := []string{
		`<v:textbox xmlns:v="urn:schemas-microsoft-com:vml" style="mso-fit-shape-to-text:t">` +
			`<w:txbxContent xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` +
			`<w:p><w:pPr><w:jc w:val="center"/></w:pPr><w:r><w:t>hello</w:t></w:r></w:p>` +
			`</w:txbxContent></v:textbox>`,
		`<v:textbox xmlns:v="urn:schemas-microsoft-com:vml"><div style="text-align:left">x</div></v:textbox>`,
	}
	var out []byte
	out = append(out, []byte("<root>")...)
	for _, in := range inputs {
		var tb Textbox
		if err := xml.Unmarshal([]byte(in), &tb); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		b, err := xml.Marshal(&tb)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		out = append(out, b...)
	}
	for _, v := range []interface{}{
		&Shape{ID: "s1", OGfxData: "UEs", OInsetMode: "auto"},
		&Group{ID: "g1", Lock: &Lock{Ext: "edit"}, Wrap: &Wrap{Type: "square"}},
		&ClientData{ObjectType: "Note", Visible: new(string), NoThreeD: new(string)},
	} {
		b, err := xml.Marshal(v)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		out = append(out, b...)
	}
	out = append(out, []byte("</root>")...)
	if err := os.WriteFile(path, out, 0o600); err != nil {
		t.Fatalf("write dump: %v", err)
	}
}
