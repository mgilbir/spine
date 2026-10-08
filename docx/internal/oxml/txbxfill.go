package oxml

import (
	"bytes"
	"encoding/xml"
	"strings"

	xmlb "github.com/mgilbir/spine/common/xml"
)

// Text box bodies (w:txbxContent) live inside drawings, mc:AlternateContent
// and VML pictures, which the model keeps as raw markup. This file reads and
// rewrites the paragraphs of those bodies in place, so field operations reach
// text boxes too.

// CT_TxbxContent is a text box body: block-level content like a note's.
type CT_TxbxContent struct {
	P             []*CT_P
	Tbl           []*CT_Tbl
	SdtBlock      []*CT_SdtBlock
	BookmarkStart []*CT_BookmarkStart
	BookmarkEnd   []*CT_BookmarkEnd
	Raw           []*CT_RawNamedElement
	childOrder    []bodyChildRef
}

// UnmarshalXML decodes the body's block content.
func (c *CT_TxbxContent) UnmarshalXML(d *xml.Decoder, _ xml.StartElement) error {
	return unmarshalBodyContent(d, &c.P, &c.Tbl, &c.SdtBlock, &c.BookmarkStart, &c.BookmarkEnd, &c.Raw, &c.childOrder)
}

// AllParagraphs returns the body's paragraphs in document order, descending
// into tables and block-level content controls.
func (c *CT_TxbxContent) AllParagraphs() []*CT_P {
	var out []*CT_P
	visitBlockContent(c.childOrder, c.P, c.Tbl, c.SdtBlock, blockVisitor{
		Para: func(p *CT_P) { out = append(out, p) },
	})
	return out
}

// NSScope is the namespace declarations in scope at a point of a part, outer
// first: the part root's, then those of the elements around the point.
type NSScope []xmlb.NSDecl

// With returns the scope extended by the declarations among attrs.
func (s NSScope) With(attrs []xml.Attr) NSScope {
	out := s
	for _, a := range attrs {
		switch {
		case a.Name.Space == "xmlns":
			out = append(out[:len(out):len(out)], xmlb.NSDecl{Prefix: a.Name.Local, URI: a.Value})
		case a.Name.Space == "" && a.Name.Local == "xmlns":
			out = append(out[:len(out):len(out)], xmlb.NSDecl{Prefix: "", URI: a.Value})
		}
	}
	return out
}

// WithRootAttrs returns the scope extended by the namespace declarations among
// captured attributes.
func (s NSScope) WithRootAttrs(attrs []xmlb.RootAttr) NSScope {
	out := s
	for _, a := range attrs {
		if a.IsNS {
			out = append(out[:len(out):len(out)], xmlb.NSDecl{Prefix: a.Prefix, URI: a.Value})
		}
	}
	return out
}

// bound returns the URI prefix is bound to, the innermost declaration winning.
func (s NSScope) bound(prefix string) string {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i].Prefix == prefix {
			return s[i].URI
		}
	}
	return ""
}

// declAttrs renders the scope as namespace declaration attributes, one per
// prefix (the innermost binding).
func (s NSScope) declAttrs() string {
	var b strings.Builder
	seen := map[string]bool{}
	for i := len(s) - 1; i >= 0; i-- {
		d := s[i]
		if seen[d.Prefix] {
			continue
		}
		seen[d.Prefix] = true
		b.WriteString(" xmlns")
		if d.Prefix != "" {
			b.WriteString(":")
			b.WriteString(d.Prefix)
		}
		b.WriteString(`="`)
		_ = xml.EscapeText(&b, []byte(d.URI))
		b.WriteString(`"`)
	}
	return b.String()
}

// txbxSpan is one outermost w:txbxContent element in raw markup: the byte
// offsets of its start tag, its content and its end, and the declarations in
// scope at its start tag (its own included).
type txbxSpan struct {
	start, contentStart, contentEnd, end int
	scope                                NSScope
}

// txbxSpans finds the outermost w:txbxContent elements of raw, a run child's
// content markup whose prefixes resolve against scope. Raw markup the decoder
// cannot read yields the spans found before the error.
func txbxSpans(raw []byte, scope NSScope) []txbxSpan {
	if !bytes.Contains(raw, []byte("txbxContent")) {
		return nil
	}
	dec := xml.NewDecoder(bytes.NewReader(raw))
	dec.Strict = false
	var spans []txbxSpan
	stack := []NSScope{scope}
	depth := 0 // nesting inside the current span
	var cur txbxSpan
	for {
		before := int(dec.InputOffset())
		tok, err := dec.RawToken()
		if err != nil {
			return spans
		}
		switch t := tok.(type) {
		case xml.StartElement:
			sc := stack[len(stack)-1].With(t.Attr)
			stack = append(stack, sc)
			if depth > 0 {
				depth++
				continue
			}
			if t.Name.Local == "txbxContent" && sc.bound(t.Name.Space) == NsWml {
				cur = txbxSpan{start: before, contentStart: int(dec.InputOffset()), scope: sc}
				depth = 1
			}
		case xml.EndElement:
			if len(stack) > 1 {
				stack = stack[:len(stack)-1]
			}
			if depth == 0 {
				continue
			}
			depth--
			if depth == 0 {
				cur.contentEnd = before
				cur.end = int(dec.InputOffset())
				if cur.contentEnd >= cur.contentStart {
					spans = append(spans, cur)
				}
			}
		}
	}
}

// parseTxbx parses one span's content, binding its prefixes from the scope.
func parseTxbx(raw []byte, sp txbxSpan) (*CT_TxbxContent, bool) {
	return parseTxbxContent(raw[sp.contentStart:sp.contentEnd], sp.scope)
}

func parseTxbxContent(content []byte, scope NSScope) (*CT_TxbxContent, bool) {
	var buf bytes.Buffer
	buf.WriteString("<w:txbxContent")
	buf.WriteString(scope.declAttrs())
	buf.WriteString(">")
	buf.Write(content)
	buf.WriteString("</w:txbxContent>")
	c := &CT_TxbxContent{}
	if err := xmlb.Unmarshal(buf.Bytes(), c); err != nil {
		return nil, false
	}
	return c, true
}

// TextBoxRawChildren calls fn with the raw markup, and the namespace scope it
// resolves against, of each run child of r that can hold a text box: drawings,
// mc:AlternateContent and VML pictures.
func TextBoxRawChildren(r *CT_R, scope NSScope, fn func(raw *[]byte, scope NSScope)) {
	if r == nil {
		return
	}
	for _, dr := range r.Drawing {
		if dr != nil {
			fn(&dr.RawContent, scope.WithRootAttrs(dr.CapturedAttrs))
		}
	}
	for _, ac := range r.AlternateContent {
		if ac != nil {
			fn(&ac.RawContent, scope.With(ac.Attrs))
		}
	}
	for _, pict := range r.Pict {
		if pict != nil {
			fn(&pict.RawContent, scope.With(pict.Attrs))
		}
	}
}

// VisitTextBoxes calls fn with the paragraphs of every text box body in raw,
// nested ones included, in document order. Bodies that do not parse are
// skipped.
func VisitTextBoxes(raw []byte, scope NSScope, fn func(paras []*CT_P)) {
	for _, sp := range txbxSpans(raw, scope) {
		c, ok := parseTxbx(raw, sp)
		if !ok {
			continue
		}
		paras := c.AllParagraphs()
		fn(paras)
		for _, p := range paras {
			for _, r := range ContainerRuns(p) {
				TextBoxRawChildren(r, sp.scope, func(inner *[]byte, sc NSScope) {
					VisitTextBoxes(*inner, sc, fn)
				})
			}
		}
	}
}

// FillTextBoxFields runs FillFields over every text box body in raw, nested
// ones included, and rewrites each body that changed in place, keeping its
// start and end tags. A body is rewritten only when the new content parses
// with the same namespace scope; otherwise the body is left as it was and its
// fields are reported through f.Skipped. It reports whether raw changed.
func FillTextBoxFields(raw *[]byte, scope NSScope, f FieldFiller) bool {
	spans := txbxSpans(*raw, scope)
	if len(spans) == 0 {
		return false
	}
	src := *raw
	var out bytes.Buffer
	last := 0
	changed := false
	for _, sp := range spans {
		c, ok := parseTxbx(src, sp)
		if !ok {
			continue
		}
		// c is parsed from the raw bytes, so filling it leaves raw untouched
		// until the body is written back below.
		paras := c.AllParagraphs()
		inner := f
		emptied := map[*CT_P]bool{}
		inner.Emptied = func(p *CT_P) { emptied[p] = true }
		filled := FillFields(paras, inner)
		for _, p := range paras {
			for _, r := range ContainerRuns(p) {
				TextBoxRawChildren(r, sp.scope, func(inner *[]byte, sc NSScope) {
					if FillTextBoxFields(inner, sc, f) {
						filled = true
					}
				})
			}
		}
		if !filled {
			continue
		}
		if f.SuppressBlank {
			SuppressBlankParagraphs(c, emptied)
		}
		content, ok := marshalTxbxContent(c, sp.scope)
		if !ok {
			reportTxbxFields(src, sp, f)
			continue
		}
		out.Write(src[last:sp.contentStart])
		out.Write(content)
		last = sp.contentEnd
		changed = true
	}
	if !changed {
		return false
	}
	out.Write(src[last:])
	*raw = out.Bytes()
	return true
}

// marshalTxbxContent serializes a body's content and checks it against the
// scope it goes back into: every prefix the builder writes names with must be
// bound there to the builder's namespace, and the content must parse back
// under the scope, which checks that every prefix it uses is bound.
func marshalTxbxContent(c *CT_TxbxContent, scope NSScope) ([]byte, bool) {
	if scope.bound("w") != NsWml {
		return nil, false
	}
	b := xmlb.NewWordprocessingMLBuilder()
	marshalBodyContent(b, NsWml, c.P, c.Tbl, c.SdtBlock, c.BookmarkStart, c.BookmarkEnd, c.Raw, c.childOrder)
	if err := b.Finish(); err != nil {
		return nil, false
	}
	content := append([]byte(nil), b.Bytes()...)
	builderNS := b.NamespacePrefixes()
	for _, prefix := range usedPrefixes(content) {
		if uri, ok := builderNS[prefix]; ok && scope.bound(prefix) != uri {
			return nil, false
		}
	}
	if _, ok := parseTxbxContent(content, scope); !ok {
		return nil, false
	}
	return content, true
}

// usedPrefixes returns the prefixes of the element and attribute names in
// content, namespace declarations aside, each once.
func usedPrefixes(content []byte) []string {
	dec := xml.NewDecoder(bytes.NewReader(content))
	dec.Strict = false
	seen := map[string]bool{}
	var out []string
	add := func(p string) {
		if p != "" && p != "xmlns" && p != "xml" && !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	for {
		tok, err := dec.RawToken()
		if err != nil {
			return out
		}
		if se, ok := tok.(xml.StartElement); ok {
			add(se.Name.Space)
			for _, a := range se.Attr {
				add(a.Name.Space)
			}
		}
	}
}

// reportTxbxFields reports every field of a body FillTextBoxFields could not
// write back, nested text boxes included, parsing the original content again.
func reportTxbxFields(raw []byte, sp txbxSpan, f FieldFiller) {
	if f.Skipped == nil {
		return
	}
	c, ok := parseTxbx(raw, sp)
	if !ok {
		return
	}
	report := FieldFiller{
		Fill: func(instr string, _ bool) (string, bool) {
			f.Skipped(instr)
			return "", false
		},
		Skipped: f.Skipped,
	}
	paras := c.AllParagraphs()
	FillFields(paras, report)
	for _, p := range paras {
		for _, r := range ContainerRuns(p) {
			TextBoxRawChildren(r, sp.scope, func(inner *[]byte, sc NSScope) {
				VisitTextBoxes(*inner, sc, func(nested []*CT_P) { FillFields(nested, report) })
			})
		}
	}
}
