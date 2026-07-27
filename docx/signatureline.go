package docx

import (
	"fmt"
	"strings"

	"github.com/mgilbir/spine/docx/internal/oxml"
)

// SignatureLineOptions configures a visible signature line (the "Microsoft
// Office Signature Line" placeholder inserted through Insert > Signature Line).
// It is the in-document request for a signature, distinct from actually signing
// the package (see opc.SignPackage): the placeholder shows the suggested
// signer's name, title, and email, and prompts a reader to sign.
type SignatureLineOptions struct {
	// Signer is the suggested signer's name shown under the line.
	Signer string
	// Title is the suggested signer's title (e.g. "Director").
	Title string
	// Email is the suggested signer's email address.
	Email string
	// Instructions are shown to the signer in Word's signing dialog.
	Instructions string
}

// SignatureLine is a visible signature line read back from a document
// (Document.SignatureLines).
type SignatureLine struct {
	// ID is the signature line's GUID (the o:signatureline id attribute).
	ID string
	// Signer, Title, Email, and Instructions mirror the suggested-signer fields
	// set when the line was created.
	Signer       string
	Title        string
	Email        string
	Instructions string
}

// signatureLineShapeType is the standard signature-line OLE picture shape type
// (t201) the VML shape references.
const signatureLineShapeType = `<v:shapetype id="_x0000_t201" coordsize="21600,21600" o:spt="201" path="m,l,21600r21600,l21600,xe">` +
	`<v:stroke joinstyle="miter"/>` +
	`<v:path shadowok="f" o:extrusionok="f" strokeok="f" fillok="f" o:connecttype="rect"/>` +
	`<o:lock v:ext="edit" shapetype="t"/>` +
	`</v:shapetype>`

// AddSignatureLine appends a paragraph containing a signature line to the
// document body and returns the paragraph. It is a convenience wrapper over
// Paragraph.AddSignatureLine.
func (d *Document) AddSignatureLine(opts SignatureLineOptions) *Paragraph {
	p := d.AddParagraph()
	p.AddSignatureLine(opts)
	return p
}

// AddSignatureLine appends an inline signature line to the paragraph: a VML
// shape carrying an o:signatureline element (the "Microsoft Office Signature
// Line" object). It creates the visible placeholder only; signing it is the
// separate package-signing feature (opc.SignPackage). The returned Run holds
// the shape.
func (p *Paragraph) AddSignatureLine(opts SignatureLineOptions) *Run {
	seq := 1
	if p.document != nil {
		seq = p.document.nextShapeID()
	}
	pict := buildSignatureLinePict(opts, seq)
	r := &oxml.CT_R{}
	r.AppendPict(pict)
	p.mut().AppendR(r)
	return &Run{paragraph: p, r: r}
}

// buildSignatureLinePict builds the w:pict raw element for a signature line: a
// t201 VML shape whose o:signatureline element carries the suggested-signer
// fields. The pict declares the v/o/w10 namespaces inline (see pictAttrs).
func buildSignatureLinePict(opts SignatureLineOptions, seq int) *oxml.CT_RawElement {
	shapeID := fmt.Sprintf("_x0000_s%d", 2050+seq)
	sigID := newBibGUID()

	var b strings.Builder
	b.WriteString(signatureLineShapeType)
	fmt.Fprintf(&b, `<v:shape id="%s" type="#_x0000_t201" style="width:192pt;height:96pt" o:spid="%s">`,
		escapeXMLAttr(shapeID), escapeXMLAttr(shapeID))
	b.WriteString(`<o:lock v:ext="edit" ungrouping="t" rotation="t" cropping="t" verticies="t" grouping="t"/>`)
	b.WriteString(`<o:signatureline v:ext="edit"`)
	fmt.Fprintf(&b, ` id="%s"`, escapeXMLAttr(sigID))
	b.WriteString(` provid="{00000000-0000-0000-0000-000000000000}"`)
	b.WriteString(` issignatureline="t"`)
	b.WriteString(` showsigndate="t"`)
	b.WriteString(` showsigntitle="t"`)
	b.WriteString(` allowcomments="t"`)
	if opts.Signer != "" {
		fmt.Fprintf(&b, ` suggestedsigner="%s"`, escapeXMLAttr(opts.Signer))
	}
	if opts.Title != "" {
		fmt.Fprintf(&b, ` suggestedsigner2="%s"`, escapeXMLAttr(opts.Title))
	}
	if opts.Email != "" {
		fmt.Fprintf(&b, ` suggestedsigneremail="%s"`, escapeXMLAttr(opts.Email))
	}
	if opts.Instructions != "" {
		fmt.Fprintf(&b, ` signinginstructionsset="t" signinginstructions="%s"`, escapeXMLAttr(opts.Instructions))
	}
	b.WriteString(`/>`)
	b.WriteString(`</v:shape>`)

	return &oxml.CT_RawElement{Attrs: pictAttrs(), RawContent: []byte(b.String())}
}

// SignatureLines returns the visible signature lines in the document body, in
// document order. It reads back the placeholders created by AddSignatureLine
// (and the equivalent shapes Word writes), reporting the suggested-signer
// fields; it does not report whether any line has actually been signed.
func (d *Document) SignatureLines() []SignatureLine {
	var out []SignatureLine
	for _, p := range d.Paragraphs() {
		for _, r := range p.p.R {
			for _, pict := range r.Pict {
				if pict != nil {
					out = appendSignatureLines(out, pict.RawContent)
				}
			}
			for _, obj := range r.Object {
				if obj != nil {
					out = appendSignatureLines(out, obj.RawContent)
				}
			}
		}
	}
	return out
}

// appendSignatureLines parses every o:signatureline element in the raw VML and
// appends the signature lines it describes.
func appendSignatureLines(out []SignatureLine, raw []byte) []SignatureLine {
	s := string(raw)
	const marker = "signatureline"
	for {
		i := strings.Index(s, marker)
		if i < 0 {
			break
		}
		// Advance past the element name so the next iteration finds a later one.
		rest := s[i+len(marker):]
		end := strings.IndexByte(rest, '>')
		if end < 0 {
			break
		}
		attrs := rest[:end]
		out = append(out, SignatureLine{
			ID:           vmlAttr(attrs, "id"),
			Signer:       vmlAttr(attrs, "suggestedsigner"),
			Title:        vmlAttr(attrs, "suggestedsigner2"),
			Email:        vmlAttr(attrs, "suggestedsigneremail"),
			Instructions: vmlAttr(attrs, "signinginstructions"),
		})
		s = rest[end:]
	}
	return out
}

// vmlAttr extracts the value of the named attribute from a VML element's raw
// attribute list, unescaping XML entities. It matches on a whitespace or
// element-name boundary so a lookup for "suggestedsigner" does not match
// "suggestedsigner2" or "suggestedsigneremail".
func vmlAttr(attrs, name string) string {
	search := attrs
	offset := 0
	for {
		i := strings.Index(search, name+`="`)
		if i < 0 {
			return ""
		}
		// The character before the name must be a boundary (start or space),
		// and the character after the name must be '=' — guaranteed by the
		// name+`="` match — so "signinginstructions" never matches
		// "signinginstructionsset".
		abs := offset + i
		if abs == 0 || attrs[abs-1] == ' ' {
			val := attrs[abs+len(name)+2:]
			q := strings.IndexByte(val, '"')
			if q < 0 {
				return ""
			}
			return unescapeXMLAttr(val[:q])
		}
		search = search[i+len(name)+2:]
		offset = abs + len(name) + 2
	}
}
