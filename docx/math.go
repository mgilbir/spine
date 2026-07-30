package docx

import (
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
	raw, err := p.marshalMathContent(m.MarshalContent)
	if err != nil {
		return fmt.Errorf("docx: marshal math zone: %w", err)
	}
	p.mut().AppendOMath(raw)
	return nil
}

// AddMathPara appends a math paragraph (m:oMathPara) to the paragraph (see
// AddMath).
func (p *Paragraph) AddMathPara(mp *omml.OMathPara) error {
	raw, err := p.marshalMathContent(mp.MarshalContent)
	if err != nil {
		return fmt.Errorf("docx: marshal math paragraph: %w", err)
	}
	p.mut().AppendOMathPara(raw)
	return nil
}

// wmlBuilderNamespaces is the URI→prefix table xmlb.NewWordprocessingMLBuilder
// installs. A root declaration must not disturb it in either direction: an
// alias for one of these URIs would re-prefix every modeled element, and a
// foreign URI claiming one of these prefixes would make two namespaces render
// identically.
var wmlBuilderNamespaces = map[string]string{
	xmlb.NSWordprocessingML:        xmlb.PrefixWordprocessingML,
	xmlb.NSOfficeDocumentRels:      xmlb.PrefixRelationships,
	xmlb.NSDrawingML:               xmlb.PrefixDrawingML,
	xmlb.NSMarkupCompatibility:     xmlb.PrefixMarkupCompatibility,
	xmlb.NSDrawingMLWordprocessing: "wp",
	xmlb.NSWord2010:                xmlb.PrefixWord2010,
	xmlb.NSWord2012:                xmlb.PrefixWord2012,
	xmlb.NSMath:                    xmlb.PrefixMath,
}

// marshalMathContent renders inner math content (the children of an
// m:oMath / m:oMathPara element) with the standard WML prefix set bound —
// the same prefixes the document marshaler emits around the raw bytes — plus
// every prefix the document root declares.
//
// The two halves of the loop have to agree: unmarshalMath binds the root's
// declarations so Word content under any root-declared prefix parses, and
// without the same registration here a raw child in one of them (wps:, wne:,
// w16:, v:, o:, w10:, a vendor URI) has no prefix to write, so writeQName
// fails the whole marshal — a math zone that parsed cleanly could not be
// written back through MathZones → AddMath.
func (p *Paragraph) marshalMathContent(fn func(*xmlb.Builder)) ([]byte, error) {
	b := xmlb.NewWordprocessingMLBuilder()
	if p.document != nil && p.document.doc() != nil {
		usedPrefix := make(map[string]bool, len(wmlBuilderNamespaces))
		for _, prefix := range wmlBuilderNamespaces {
			usedPrefix[prefix] = true
		}
		for _, decl := range p.document.doc().OriginalNSDecls {
			// A default declaration binds no prefix an element name can use,
			// and a prefix already spoken for by the standard set (or by an
			// earlier declaration) must not be handed a second URI.
			if decl.Prefix == "" || decl.URI == "" || usedPrefix[decl.Prefix] {
				continue
			}
			if _, standard := wmlBuilderNamespaces[decl.URI]; standard {
				continue
			}
			usedPrefix[decl.Prefix] = true
			b.RegisterNamespace(decl.URI, decl.Prefix)
		}
	}
	fn(b)
	if err := b.Finish(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// unmarshalMath parses raw-captured math content by wrapping it in an
// element that binds the prefixes the content may reference: the standard
// WML set plus every declaration from the document root, so content written
// by Word under root-declared prefixes resolves. The decode registers its
// source bytes so the model's lexical captures (m:t's <m:t/> vs <m:t></m:t>)
// see the form the producer wrote.
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
	return xmlb.UnmarshalWithSource([]byte(sb.String()), v)
}
