package opc

import "testing"

// TestTokenWalkingParsersCheckTheDocumentBounds pins that the parsers which
// walk a part's tokens themselves (content types, core and custom properties)
// refuse what the unmarshaling ones do around the root element. They checked
// nothing there: text before or after the root, or a declaration inside it,
// parsed, and the content types part carried it into every save.
func TestTokenWalkingParsersCheckTheDocumentBounds(t *testing.T) {
	const decl = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`
	const ct = `<Types xmlns="` + ContentTypesNamespace + `">`
	const core = `<cp:coreProperties xmlns:cp="` + nsCoreProperties + `" xmlns:dc="` + nsDublinCore + `">`
	const custom = `<Properties xmlns="http://schemas.openxmlformats.org/officeDocument/2006/custom-properties" xmlns:vt="http://schemas.openxmlformats.org/officeDocument/2006/docPropsVTypes">`
	for _, p := range []struct {
		name         string
		root, inside string
		fn           func([]byte) error
	}{
		{"content types",
			ct + `<Default Extension="xml" ContentType="application/xml"/></Types>`,
			ct + decl + `<Default Extension="xml" ContentType="application/xml"/></Types>`,
			func(b []byte) error { _, err := UnmarshalContentTypes(b); return err }},
		{"core properties",
			core + `<dc:title>T</dc:title></cp:coreProperties>`,
			core + `<dc:title>T</dc:title>` + decl + `</cp:coreProperties>`,
			func(b []byte) error { _, err := UnmarshalCoreProperties(b); return err }},
		{"custom properties",
			custom + `</Properties>`,
			custom + decl + `</Properties>`,
			func(b []byte) error { _, err := UnmarshalCustomProperties(b); return err }},
	} {
		if err := p.fn([]byte("\xEF\xBB\xBF" + decl + "\r\n" + p.root + "\r\n")); err != nil {
			t.Errorf("%s: well-formed part refused: %v", p.name, err)
		}
		for name, bad := range map[string]string{
			"text before":          "junk" + p.root,
			"no-break space after": p.root + " ",
			"declaration after":    p.root + decl,
			"declaration inside":   p.inside,
		} {
			if p.fn([]byte(bad)) == nil {
				t.Errorf("%s: %s accepted", p.name, name)
			}
		}
	}
}
