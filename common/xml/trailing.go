package xml

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

// CheckDocumentEnd reports whether anything follows the root element that a
// well-formed document may not carry.
//
// It exists because [xml.Decoder.Decode] stops at the end tag that closes the
// element it was asked for and never looks further. Everything after that is
// accepted in silence: a second root element, a stray end tag, raw text, the
// truncated tail of some other part. None of it is well-formed XML, and none of
// it produces an error.
//
// That silence is not academic. A part is preserved as raw bytes and rewritten
// verbatim when nothing modifies it, so bytes the parser waved through are the
// bytes the next save emits. The library therefore opened a document, reported
// success, and wrote a package whose parts do not parse — one Word will refuse.
// Four of the fuzzers found it independently, through word/comments.xml,
// word/numbering.xml, word/header1.xml and the document part, each reporting a
// different symptom of this one cause:
//
//	<w:comments>...</w:comments>"Fuzz Author" w:date="..."   -> invalid XML name
//	<w:hdr>...</w:hdr></hdr>                                 -> unexpected end element
//
// XML 1.0 §2.1 gives the production as `prolog element Misc*`, and Misc as a
// comment, a processing instruction, or white space. Anything else after the
// root element makes the document ill-formed, which is exactly the set rejected
// here.
//
// The decoder must be the one that just decoded the root element; the check
// consumes the remainder of its input.
func CheckDocumentEnd(d *xml.Decoder) error {
	for {
		tok, err := d.Token()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			// Malformed bytes after the root element are themselves a reason to
			// reject: the document does not end, it breaks off.
			return fmt.Errorf("content after the root element is not well-formed: %w", err)
		}
		switch t := tok.(type) {
		case xml.CharData:
			// Only white space is Misc, and XML's white space at that: a no-break
			// space is text. Text after the root element is what a truncated or
			// concatenated part looks like.
			if !IsSpace(t) {
				return fmt.Errorf("text after the root element: %.40q", string(t))
			}
		case xml.ProcInst:
			// Misc, unless it is named xml: that is the declaration, which only
			// opens a document (PITarget excludes the name in any case).
			if strings.EqualFold(t.Target, "xml") {
				return fmt.Errorf("XML declaration after the root element")
			}
		case xml.Comment:
			// Misc: allowed.
		case xml.StartElement:
			return fmt.Errorf("a second root element <%s> after the first", t.Name.Local)
		case xml.EndElement:
			return fmt.Errorf("end tag </%s> after the root element closed", t.Name.Local)
		case xml.Directive:
			// A doctype is prolog-only; after the root element it is ill-formed.
			return fmt.Errorf("directive after the root element: %.40q", string(t))
		}
	}
}

// CheckDocumentStart reads the decoder up to the root element and returns its
// start tag, reporting anything before it that a well-formed document may not
// carry. It is CheckDocumentEnd's other half, for the same reason: Decode
// skips whatever precedes the root element in silence, so "junk<w:document>"
// parsed, and a preserved part with it was written back verbatim.
//
// XML 1.0 §2.8 gives the prolog as `XMLDecl? Misc* (doctypedecl Misc*)?`: the
// declaration, if there is one, comes first, and then only comments,
// processing instructions and white space. The one thing allowed ahead of the
// declaration is a byte order mark, the encoding's signature rather than
// content (§4.3.3). A document type declaration is left to the decoder.
//
// The decoder must be fresh; DecodeElement with the returned start tag then
// decodes the root.
func CheckDocumentStart(d *xml.Decoder) (xml.StartElement, error) {
	// atStart holds while nothing but a leading byte order mark has been read,
	// which is where the declaration has to be.
	atStart := true
	for {
		offset := d.InputOffset()
		tok, err := d.Token()
		if err != nil {
			return xml.StartElement{}, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			return t, nil
		case xml.CharData:
			text := []byte(t)
			if offset == 0 && bytes.HasPrefix(text, utf8BOM) {
				text = text[len(utf8BOM):]
				if len(text) == 0 {
					continue
				}
			}
			if !IsSpace(text) {
				return xml.StartElement{}, fmt.Errorf("text before the root element: %.40q", string(t))
			}
		case xml.ProcInst:
			if strings.EqualFold(t.Target, "xml") && (t.Target != "xml" || !atStart) {
				return xml.StartElement{}, fmt.Errorf("XML declaration not at the start of the document")
			}
		}
		atStart = false
	}
}

// checkDeclarationsInside reports a processing instruction named xml inside
// the root element, which the decode that just read data cannot see: unmarshal
// hooks consume element content's tokens. XML reserves the name in any case
// for the declaration (PITarget), which only opens a document. The byte scan
// is the fast path; only a candidate, which may yet sit in a comment or a
// CDATA section, costs a token pass.
func checkDeclarationsInside(data []byte) error {
	candidate := false
	for i := 1; i < len(data); {
		j := bytes.Index(data[i:], []byte("<?"))
		if j < 0 {
			break
		}
		at := i + j
		if at+5 <= len(data) && bytes.EqualFold(data[at+2:at+5], []byte("xml")) &&
			(at+5 == len(data) || isXMLSpace(data[at+5]) || data[at+5] == '?') {
			if at != len(utf8BOM) || !bytes.HasPrefix(data, utf8BOM) {
				candidate = true
				break
			}
		}
		i = at + 2
	}
	if !candidate {
		return nil
	}
	d := NewDecoder(bytes.NewReader(data))
	if _, err := CheckDocumentStart(d); err != nil {
		return err
	}
	for {
		tok, err := d.Token()
		if err != nil {
			return nil // the decode reports malformed input itself
		}
		if pi, ok := tok.(xml.ProcInst); ok && strings.EqualFold(pi.Target, "xml") {
			return fmt.Errorf("XML declaration inside the document")
		}
	}
}

// CheckDocumentBounds reports, for a parser that walks a part's tokens itself
// rather than unmarshaling it, what the part-level entry points check around
// the root element: CheckDocumentStart, CheckDocumentEnd, and an XML
// declaration inside the document. It costs one token pass over the part's
// UTF-8 form.
func CheckDocumentBounds(data []byte) error {
	d, data, err := NewPartDecoder(data)
	if err != nil {
		return err
	}
	if _, err := CheckDocumentStart(d); err != nil {
		return err
	}
	if err := d.Skip(); err != nil {
		return err
	}
	if err := CheckDocumentEnd(d); err != nil {
		return err
	}
	return checkDeclarationsInside(data)
}
