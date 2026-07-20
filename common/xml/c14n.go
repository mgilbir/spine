package xml

import (
	"bytes"
	"errors"
	"io"
	"sort"

	stdxml "encoding/xml"
)

// Canonical XML 1.0 (inclusive, without comments) — W3C
// REC-xml-c14n-20010315. This is the canonicalization OPC package digital
// signatures use for the SignedInfo element, for Object references, and as the
// terminal transform of the OPC RelationshipTransform (ECMA-376 Part 2 §13).
//
// The implementation is deliberately scoped to the well-formed, DTD-free,
// UTF-8 XML that signature parts contain: it reuses the standard tokenizer
// (via NewDecoder, so entity references, CDATA sections, and line-ending
// normalization are handled by encoding/xml) and reconstructs namespace
// prefixes from the resolved namespace URIs the tokenizer reports. Because the
// tokenizer resolves prefixes to URIs, the original prefix is recovered from
// the in-scope declarations; when two prefixes are bound to the same URI on an
// element the lexicographically smaller one is chosen. Signature content binds
// each URI to a single prefix, so this reconstruction is exact for the inputs
// that matter and, where it is not, a re-canonicalization simply fails to match
// a digest (a false negative) rather than accepting altered content.
//
// Comments are not part of the node-set (the "#WithComments" variant is not
// implemented); processing instructions and the XML declaration are likewise
// omitted, matching the algorithm URI without the WithComments suffix.

const xmlNamespaceURI = "http://www.w3.org/XML/1998/namespace"

// errNoRoot is returned when the input contains no element.
var errNoRoot = errors.New("xml: canonicalization input has no root element")

// c14nNode is an element or a text node in the lightweight tree built for
// canonicalization.
type c14nNode interface{ isC14N() }

type c14nElem struct {
	prefix   string            // reconstructed prefix ("" = default namespace)
	local    string            // local name
	uri      string            // resolved namespace URI ("" = no namespace)
	inScope  map[string]string // full prefix→URI map in scope ("" key = default)
	attrs    []c14nAttr        // non-namespace attributes
	children []c14nNode
}

func (*c14nElem) isC14N() {}

type c14nText struct{ data string }

func (*c14nText) isC14N() {}

type c14nAttr struct {
	prefix string
	local  string
	uri    string
	value  string
}

// C14NNode is a handle to a parsed element, exposing the navigation and
// canonicalization needed to verify and build XML-DSig signatures without
// tying the c14n engine to any signature schema.
type C14NNode struct{ e *c14nElem }

// ParseC14N parses data into a navigable tree whose nodes can be
// canonicalized. The returned node is the document's root element.
func ParseC14N(data []byte) (*C14NNode, error) {
	root, err := parseC14NTree(data)
	if err != nil {
		return nil, err
	}
	return &C14NNode{e: root}, nil
}

// Canonicalize returns the inclusive Canonical XML 1.0 (without comments)
// serialization of data's root element.
func Canonicalize(data []byte) ([]byte, error) {
	root, err := parseC14NTree(data)
	if err != nil {
		return nil, err
	}
	return root.canonical(), nil
}

// Canonical returns the inclusive Canonical XML 1.0 serialization of the
// subtree rooted at n. Namespace declarations in scope at n but declared on an
// ancestor outside the subtree are rendered on n's start tag, as required for
// canonicalizing a document subset.
func (n *C14NNode) Canonical() []byte { return n.e.canonical() }

// LocalName returns the element's local name.
func (n *C14NNode) LocalName() string { return n.e.local }

// NamespaceURI returns the element's resolved namespace URI.
func (n *C14NNode) NamespaceURI() string { return n.e.uri }

// FindChild returns the first direct child element with the given local name,
// or nil if none exists.
func (n *C14NNode) FindChild(local string) *C14NNode {
	for _, c := range n.e.children {
		if el, ok := c.(*c14nElem); ok && el.local == local {
			return &C14NNode{e: el}
		}
	}
	return nil
}

// FindByID returns the element at or below n whose "Id" attribute (any
// namespace) equals id, searching in document order, or nil if none matches.
// XML-DSig references identify Object elements by their Id attribute.
func (n *C14NNode) FindByID(id string) *C14NNode {
	if el := findByID(n.e, id); el != nil {
		return &C14NNode{e: el}
	}
	return nil
}

func findByID(e *c14nElem, id string) *c14nElem {
	for _, a := range e.attrs {
		if a.local == "Id" && a.value == id {
			return e
		}
	}
	for _, c := range e.children {
		if el, ok := c.(*c14nElem); ok {
			if found := findByID(el, id); found != nil {
				return found
			}
		}
	}
	return nil
}

func parseC14NTree(data []byte) (*c14nElem, error) {
	dec := NewDecoder(bytes.NewReader(data))
	var root *c14nElem
	var stack []*c14nElem
	scopeStack := []map[string]string{{"xml": xmlNamespaceURI}}

	for {
		tok, err := dec.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		switch t := tok.(type) {
		case stdxml.StartElement:
			parentScope := scopeStack[len(scopeStack)-1]
			scope := cloneScope(parentScope)
			for _, a := range t.Attr {
				switch {
				case a.Name.Space == "xmlns":
					scope[a.Name.Local] = a.Value
				case a.Name.Space == "" && a.Name.Local == "xmlns":
					scope[""] = a.Value
				}
			}
			el := &c14nElem{local: t.Name.Local, uri: t.Name.Space, inScope: scope}
			el.prefix = pickElemPrefix(scope, t.Name.Space)
			for _, a := range t.Attr {
				if a.Name.Space == "xmlns" || (a.Name.Space == "" && a.Name.Local == "xmlns") {
					continue
				}
				at := c14nAttr{local: a.Name.Local, uri: a.Name.Space, value: a.Value}
				if a.Name.Space != "" {
					at.prefix = pickAttrPrefix(scope, a.Name.Space)
				}
				el.attrs = append(el.attrs, at)
			}
			if root == nil {
				root = el
			} else {
				parent := stack[len(stack)-1]
				parent.children = append(parent.children, el)
			}
			stack = append(stack, el)
			scopeStack = append(scopeStack, scope)
		case stdxml.EndElement:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
				scopeStack = scopeStack[:len(scopeStack)-1]
			}
		case stdxml.CharData:
			if len(stack) > 0 {
				parent := stack[len(stack)-1]
				parent.children = append(parent.children, &c14nText{data: string(t)})
			}
			// Character data before the root element (whitespace in the
			// prolog) is discarded, matching C14N of a single element subset.
		}
		// Comments, processing instructions, and directives are not part of
		// the node-set for the without-comments variant.
	}

	if root == nil {
		return nil, errNoRoot
	}
	return root, nil
}

func cloneScope(m map[string]string) map[string]string {
	c := make(map[string]string, len(m)+1)
	for k, v := range m {
		c[k] = v
	}
	return c
}

// pickElemPrefix reconstructs the prefix an element used for uri from the
// declarations in scope, preferring the default namespace when it binds uri.
func pickElemPrefix(scope map[string]string, uri string) string {
	if uri == "" {
		return ""
	}
	if scope[""] == uri {
		return ""
	}
	best, found := "", false
	for p, u := range scope {
		if p == "" || u != uri {
			continue
		}
		if !found || p < best {
			best, found = p, true
		}
	}
	return best
}

// pickAttrPrefix reconstructs the prefix an attribute used for uri. Attributes
// never bind to the default namespace, so only non-empty prefixes are
// considered.
func pickAttrPrefix(scope map[string]string, uri string) string {
	best, found := "", false
	for p, u := range scope {
		if p == "" || u != uri {
			continue
		}
		if !found || p < best {
			best, found = p, true
		}
	}
	return best
}

func (e *c14nElem) qname() string {
	if e.prefix == "" {
		return e.local
	}
	return e.prefix + ":" + e.local
}

func (e *c14nElem) canonical() []byte {
	var buf bytes.Buffer
	e.render(&buf, map[string]string{})
	return buf.Bytes()
}

func (e *c14nElem) render(buf *bytes.Buffer, rendered map[string]string) {
	buf.WriteByte('<')
	buf.WriteString(e.qname())

	childRendered := cloneScope(rendered)

	type nsPair struct{ prefix, uri string }
	var toEmit []nsPair
	for p, u := range e.inScope {
		if p == "xml" {
			// The xml prefix is implicitly declared and never emitted.
			continue
		}
		if p == "" {
			if u == "" {
				// Undeclaration of the default namespace: emit only to undo a
				// non-empty default rendered by an output ancestor.
				if rendered[""] != "" {
					toEmit = append(toEmit, nsPair{"", ""})
				}
				continue
			}
			if rendered[""] == u {
				continue
			}
			toEmit = append(toEmit, nsPair{"", u})
			continue
		}
		if rendered[p] == u {
			continue
		}
		toEmit = append(toEmit, nsPair{p, u})
	}
	sort.Slice(toEmit, func(i, j int) bool { return toEmit[i].prefix < toEmit[j].prefix })
	for _, ns := range toEmit {
		if ns.prefix == "" {
			buf.WriteString(` xmlns="`)
		} else {
			buf.WriteString(" xmlns:")
			buf.WriteString(ns.prefix)
			buf.WriteString(`="`)
		}
		writeC14NAttrValue(buf, ns.uri)
		buf.WriteByte('"')
		childRendered[ns.prefix] = ns.uri
	}

	attrs := make([]c14nAttr, len(e.attrs))
	copy(attrs, e.attrs)
	sort.SliceStable(attrs, func(i, j int) bool {
		if attrs[i].uri != attrs[j].uri {
			return attrs[i].uri < attrs[j].uri
		}
		return attrs[i].local < attrs[j].local
	})
	for _, a := range attrs {
		buf.WriteByte(' ')
		if a.prefix != "" {
			buf.WriteString(a.prefix)
			buf.WriteByte(':')
		}
		buf.WriteString(a.local)
		buf.WriteString(`="`)
		writeC14NAttrValue(buf, a.value)
		buf.WriteByte('"')
	}
	buf.WriteByte('>')

	for _, c := range e.children {
		switch n := c.(type) {
		case *c14nElem:
			n.render(buf, childRendered)
		case *c14nText:
			writeC14NText(buf, n.data)
		}
	}

	buf.WriteString("</")
	buf.WriteString(e.qname())
	buf.WriteByte('>')
}

// writeC14NText escapes character data per C14N: &, <, > and carriage return.
// Tab and line feed are left literal.
func writeC14NText(buf *bytes.Buffer, s string) {
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '&':
			buf.WriteString("&amp;")
		case '<':
			buf.WriteString("&lt;")
		case '>':
			buf.WriteString("&gt;")
		case '\r':
			buf.WriteString("&#xD;")
		default:
			buf.WriteByte(s[i])
		}
	}
}

// writeC14NAttrValue escapes an attribute value per C14N: &, <, ", and the tab,
// line feed and carriage return control characters. The greater-than sign is
// not escaped in attribute values.
func writeC14NAttrValue(buf *bytes.Buffer, s string) {
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '&':
			buf.WriteString("&amp;")
		case '<':
			buf.WriteString("&lt;")
		case '"':
			buf.WriteString("&quot;")
		case '\t':
			buf.WriteString("&#x9;")
		case '\n':
			buf.WriteString("&#xA;")
		case '\r':
			buf.WriteString("&#xD;")
		default:
			buf.WriteByte(s[i])
		}
	}
}
