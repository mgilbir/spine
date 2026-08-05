package xml

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
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
			// Only white space is Misc. Text after the root element is what a
			// truncated or concatenated part looks like.
			if len(bytes.TrimSpace(t)) != 0 {
				return fmt.Errorf("text after the root element: %.40q", string(t))
			}
		case xml.Comment, xml.ProcInst:
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
