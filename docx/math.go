package docx

import (
	"encoding/xml"
	"fmt"
	"strings"

	"github.com/mgilbir/spine/common/omml"
	xmlb "github.com/mgilbir/spine/common/xml"
)

// MathZones returns the paragraph's math zones (m:oMath children), parsed on
// demand into typed common/omml models from the raw-captured bytes the
// document model stores. The returned values are snapshots: mutating them
// does not change the document — write math back with AddMath. Display
// equations wrapped in a math paragraph (m:oMathPara) are returned by
// MathParas instead.
func (p *Paragraph) MathZones() ([]*omml.OMath, error) {
	zones := make([]*omml.OMath, 0, len(p.p.OMath))
	for i, raw := range p.p.OMath {
		m := &omml.OMath{}
		if err := p.unmarshalMath(raw, "oMath", m); err != nil {
			return nil, fmt.Errorf("docx: parse math zone %d: %w", i, err)
		}
		zones = append(zones, m)
	}
	return zones, nil
}

// MathParas returns the paragraph's math paragraphs (m:oMathPara children,
// Word's container for display equations), parsed on demand into typed
// common/omml models (see MathZones).
func (p *Paragraph) MathParas() ([]*omml.OMathPara, error) {
	paras := make([]*omml.OMathPara, 0, len(p.p.OMathPara))
	for i, raw := range p.p.OMathPara {
		mp := &omml.OMathPara{}
		if err := p.unmarshalMath(raw, "oMathPara", mp); err != nil {
			return nil, fmt.Errorf("docx: parse math paragraph %d: %w", i, err)
		}
		paras = append(paras, mp)
	}
	return paras, nil
}

// AddMath appends a math zone (m:oMath) to the paragraph. The typed model is
// marshaled once, up front, and stored in the same raw-captured form the
// parser produces, so the document's byte-fidelity machinery is untouched;
// the document marshaler declares the math namespace on the root when math
// is present.
func (p *Paragraph) AddMath(m *omml.OMath) error {
	raw, err := marshalMathContent(m.MarshalContent)
	if err != nil {
		return fmt.Errorf("docx: marshal math zone: %w", err)
	}
	p.p.AppendOMath(raw)
	return nil
}

// AddMathPara appends a math paragraph (m:oMathPara) to the paragraph (see
// AddMath).
func (p *Paragraph) AddMathPara(mp *omml.OMathPara) error {
	raw, err := marshalMathContent(mp.MarshalContent)
	if err != nil {
		return fmt.Errorf("docx: marshal math paragraph: %w", err)
	}
	p.p.AppendOMathPara(raw)
	return nil
}

// marshalMathContent renders inner math content (the children of an
// m:oMath / m:oMathPara element) with the standard WML prefix set bound —
// the same prefixes the document marshaler emits around the raw bytes.
func marshalMathContent(fn func(*xmlb.Builder)) ([]byte, error) {
	b := xmlb.NewWordprocessingMLBuilder()
	fn(b)
	if err := b.Finish(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// unmarshalMath parses raw-captured math content by wrapping it in an
// element that binds the prefixes the content may reference: the standard
// WML set plus every declaration from the document root, so content written
// by Word under root-declared prefixes resolves.
func (p *Paragraph) unmarshalMath(raw []byte, localName string, v interface{}) error {
	var sb strings.Builder
	sb.WriteString("<")
	sb.WriteString(localName)
	seen := map[string]bool{}
	writeDecl := func(prefix, uri string) {
		if prefix == "" || seen[prefix] {
			return
		}
		seen[prefix] = true
		sb.WriteString(` xmlns:`)
		sb.WriteString(prefix)
		sb.WriteString(`="`)
		sb.WriteString(xmlb.EscapeAttrValue(uri))
		sb.WriteString(`"`)
	}
	writeDecl(xmlb.PrefixMath, xmlb.NSMath)
	writeDecl(xmlb.PrefixWordprocessingML, xmlb.NSWordprocessingML)
	writeDecl(xmlb.PrefixRelationships, xmlb.NSOfficeDocumentRels)
	writeDecl(xmlb.PrefixMarkupCompatibility, xmlb.NSMarkupCompatibility)
	writeDecl(xmlb.PrefixWord2010, xmlb.NSWord2010)
	if p.document != nil && p.document.doc() != nil {
		for _, decl := range p.document.doc().OriginalNSDecls {
			writeDecl(decl.Prefix, decl.URI)
		}
	}
	sb.WriteString(">")
	sb.Write(raw)
	sb.WriteString("</")
	sb.WriteString(localName)
	sb.WriteString(">")
	return xml.Unmarshal([]byte(sb.String()), v)
}
