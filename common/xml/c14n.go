package xml

import (
	"bytes"
	"errors"
	"io"
	"sort"
	"strings"

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
// a digest (a false negative) rather than accepting altered content. Prefix
// reconstruction is the only approximation: document-subset canonicalization
// (C14NNode.Canonical) implements §2.4 in full, rendering both the namespace
// declarations and the xml-namespace attributes (xml:lang, xml:space,
// xml:base) that the apex element inherits from ancestors outside the subset.
//
// Comments are not part of the node-set (the "#WithComments" variant is not
// implemented); processing instructions and the XML declaration are likewise
// omitted, matching the algorithm URI without the WithComments suffix.

const xmlNamespaceURI = "http://www.w3.org/XML/1998/namespace"

// errNoRoot is returned when the input contains no element.
var errNoRoot = errors.New("xml: canonicalization input has no root element")

// errMultipleRoots is returned when the input contains more than one top-level
// element. Canonical XML describes a single document element; the standard
// tokenizer tolerates trailing sibling roots, so this is rejected explicitly
// rather than indexing an empty element stack. A crafted signature part such as
// "<Signature>…</Signature><x/>" reaches this on the signature-verify path.
var errMultipleRoots = errors.New("xml: canonicalization input has multiple root elements")

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
	// inheritedXML holds the xml-namespace attributes (xml:lang, xml:space,
	// xml:base, …) that ancestors declare and this element does not override,
	// nearest ancestor winning. Canonical XML 1.0 §2.4 requires them to be
	// imported onto the apex element of a document subset; they are ignored
	// when the element is rendered as a descendant, where the ancestor's own
	// start tag already carries them.
	inheritedXML map[string]string
}

func (*c14nElem) isC14N() {}

type c14nText struct{ data string }

func (*c14nText) isC14N() {}

// c14nPI is a processing instruction. Canonical XML keeps them: the
// "#WithComments" suffix on the algorithm URI selects whether *comments* are
// in the node-set and says nothing about processing instructions, which are
// part of it either way (Canonical XML 1.0 §2.3). Dropping them is not a
// harmless omission in a signature context — a digest computed over a
// canonical form that ignores them is unchanged when one is added, altered or
// removed, so the signature covers less than it appears to.
type c14nPI struct{ target, data string }

func (*c14nPI) isC14N() {}

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
	doc, err := parseC14NTree(data)
	if err != nil {
		return nil, err
	}
	return &C14NNode{e: doc.root}, nil
}

// Canonicalize returns the inclusive Canonical XML 1.0 (without comments)
// serialization of data's root element.
func Canonicalize(data []byte) ([]byte, error) {
	doc, err := parseC14NTree(data)
	if err != nil {
		return nil, err
	}
	// A processing instruction outside the document element is rendered with
	// the line break that separates it from the element: after it in the
	// prolog, before it in the epilog (C14N 1.0 §2.3).
	var buf bytes.Buffer
	for _, pi := range doc.prolog {
		writeC14NPI(&buf, pi)
		buf.WriteByte('\n')
	}
	buf.Write(doc.root.canonical())
	for _, pi := range doc.epilog {
		buf.WriteByte('\n')
		writeC14NPI(&buf, pi)
	}
	return buf.Bytes(), nil
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

// c14nDoc is a parsed document: its root element plus the processing
// instructions that sit outside it, which document-scope canonicalization
// renders around the root.
type c14nDoc struct {
	root           *c14nElem
	prolog, epilog []*c14nPI
}

func parseC14NTree(data []byte) (*c14nDoc, error) {
	dec := NewDecoder(bytes.NewReader(data))
	var root *c14nElem
	var prologPIs, epilogPIs []*c14nPI
	var stack []*c14nElem
	scopeStack := []map[string]string{{"xml": xmlNamespaceURI}}
	// xmlAttrStack[i] is the set of xml-namespace attributes in force for the
	// element at depth i, i.e. its own merged over its ancestors'.
	xmlAttrStack := []map[string]string{nil}

	var prevTokenEnd int64
	for {
		tagStart := prevTokenEnd
		tok, err := dec.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		prevTokenEnd = dec.InputOffset()
		switch t := tok.(type) {
		case stdxml.StartElement:
			rawTag := data[tagStart:dec.InputOffset()]
			// The single root has already closed (its EndElement popped the
			// stack). A further top-level element would be a second root; reject
			// it instead of indexing the now-empty stack.
			if root != nil && len(stack) == 0 {
				return nil, errMultipleRoots
			}
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
			// Allocated lazily: most elements carry no xml:* attribute, and
			// this runs per element of a signature part.
			var ownXML map[string]string
			for _, a := range t.Attr {
				if a.Name.Space == "xmlns" || (a.Name.Space == "" && a.Name.Local == "xmlns") {
					continue
				}
				at := c14nAttr{local: a.Name.Local, uri: a.Name.Space, value: normalizeAttrValue(rawTag, a)}
				if a.Name.Space != "" {
					at.prefix = pickAttrPrefix(scope, a.Name.Space)
				}
				if a.Name.Space == xmlNamespaceURI {
					if ownXML == nil {
						ownXML = map[string]string{}
					}
					ownXML[a.Name.Local] = a.Value
				}
				el.attrs = append(el.attrs, at)
			}
			// Ancestor xml:* attributes this element does not itself carry are
			// what a subset canonicalization must import onto it.
			ancestorXML := xmlAttrStack[len(xmlAttrStack)-1]
			for name, value := range ancestorXML {
				if _, own := ownXML[name]; own {
					continue
				}
				if el.inheritedXML == nil {
					el.inheritedXML = map[string]string{}
				}
				el.inheritedXML[name] = value
			}
			var inForce map[string]string
			if len(ancestorXML) > 0 || len(ownXML) > 0 {
				inForce = make(map[string]string, len(ancestorXML)+len(ownXML))
				for name, value := range ancestorXML {
					inForce[name] = value
				}
				for name, value := range ownXML {
					inForce[name] = value
				}
			}
			xmlAttrStack = append(xmlAttrStack, inForce)
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
				xmlAttrStack = xmlAttrStack[:len(xmlAttrStack)-1]
			}
		case stdxml.CharData:
			if len(stack) > 0 {
				parent := stack[len(stack)-1]
				parent.children = append(parent.children, &c14nText{data: string(t)})
			}
			// Character data before the root element (whitespace in the
			// prolog) is discarded, matching C14N of a single element subset.
		case stdxml.ProcInst:
			// The XML declaration is not a processing instruction and is not
			// in the node-set, though Go's decoder reports it as one.
			if t.Target == "xml" {
				continue
			}
			pi := &c14nPI{target: t.Target, data: string(t.Inst)}
			if len(stack) > 0 {
				parent := stack[len(stack)-1]
				parent.children = append(parent.children, pi)
			} else if root == nil {
				prologPIs = append(prologPIs, pi)
			} else {
				epilogPIs = append(epilogPIs, pi)
			}
		}
		// Comments and directives are not part of the node-set for the
		// without-comments variant.
	}

	if root == nil {
		return nil, errNoRoot
	}
	return &c14nDoc{root: root, prolog: prologPIs, epilog: epilogPIs}, nil
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
	// e is the apex of the node-set, so ancestor xml:* attributes are imported
	// onto it (Canonical XML 1.0 §2.4). Descendants get nil: their ancestors
	// are inside the subset and render those attributes themselves.
	e.render(&buf, map[string]string{}, e.inheritedXMLAttrs())
	return buf.Bytes()
}

// inheritedXMLAttrs renders the ancestor xml:* attributes as canonical
// attributes for the apex element.
func (e *c14nElem) inheritedXMLAttrs() []c14nAttr {
	if len(e.inheritedXML) == 0 {
		return nil
	}
	out := make([]c14nAttr, 0, len(e.inheritedXML))
	for name, value := range e.inheritedXML {
		out = append(out, c14nAttr{prefix: "xml", local: name, uri: xmlNamespaceURI, value: value})
	}
	return out
}

func (e *c14nElem) render(buf *bytes.Buffer, rendered map[string]string, inherited []c14nAttr) {
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

	attrs := make([]c14nAttr, 0, len(e.attrs)+len(inherited))
	attrs = append(attrs, e.attrs...)
	attrs = append(attrs, inherited...)
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
			n.render(buf, childRendered, nil)
		case *c14nText:
			writeC14NText(buf, n.data)
		case *c14nPI:
			writeC14NPI(buf, n)
		}
	}

	buf.WriteString("</")
	buf.WriteString(e.qname())
	buf.WriteByte('>')
}

// writeC14NPI renders a processing instruction per C14N §2.3: the target, then
// a single space and the data when the data is non-empty, and nothing between
// them when it is.
func writeC14NPI(buf *bytes.Buffer, pi *c14nPI) {
	buf.WriteString("<?")
	buf.WriteString(pi.target)
	if pi.data != "" {
		buf.WriteByte(' ')
		buf.WriteString(pi.data)
	}
	buf.WriteString("?>")
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

// normalizeAttrValue applies XML 1.0 §3.3.3 attribute-value normalization,
// which Go's decoder does not.
//
// A literal tab, line feed or carriage return inside an attribute value is a
// space by the time a conforming parser is done with it, while the same
// character written as a reference (&#x9;) survives and is escaped back to a
// reference by canonicalization. Go's decoder resolves both to the same rune,
// so the raw source text is the only place the difference still exists — hence
// the tag bytes rather than the token.
//
// It matters because the two forms canonicalize differently, and a
// canonicalization that disagrees with every other implementation produces
// signatures nothing else can verify, over documents this library did not
// write. Values with no literal whitespace, which is nearly all of them, take
// the fast path and are returned untouched.
func normalizeAttrValue(rawTag []byte, a stdxml.Attr) string {
	if !strings.ContainsAny(a.Value, "\t\n\r") {
		return a.Value
	}
	raw, ok := rawAttrValue(rawTag, a.Name)
	if !ok {
		return a.Value
	}
	if !bytes.ContainsAny(raw, "\t\n\r") {
		// Every whitespace character came from a reference, so the decoded
		// value is already normalized.
		return a.Value
	}
	// Replace the literal whitespace, then let the decoder resolve the
	// references exactly as it did the first time.
	replaced := bytes.Map(func(r rune) rune {
		if r == '\t' || r == '\n' || r == '\r' {
			return ' '
		}
		return r
	}, raw)
	var probe struct {
		V string `xml:"v,attr"`
	}
	doc := append(append([]byte(`<x v="`), replaced...), []byte(`"/>`)...)
	if err := stdxml.Unmarshal(doc, &probe); err != nil {
		return a.Value
	}
	return probe.V
}

// rawAttrValue finds the source text of one attribute inside a start tag,
// between its quotes.
func rawAttrValue(rawTag []byte, name stdxml.Name) ([]byte, bool) {
	// The qualified name as written is not recoverable from the resolved one,
	// so every attribute whose local name matches is considered and the first
	// with a plausible spelling wins. A tag carrying the same local name under
	// two prefixes is vanishingly rare and falls back to the decoded value.
	for i := 0; i+len(name.Local) < len(rawTag); i++ {
		if !bytes.HasPrefix(rawTag[i:], []byte(name.Local)) {
			continue
		}
		if i > 0 {
			prev := rawTag[i-1]
			if prev != ' ' && prev != ':' && prev != '\t' && prev != '\n' && prev != '\r' {
				continue
			}
		}
		rest := rawTag[i+len(name.Local):]
		j := 0
		for j < len(rest) && (rest[j] == ' ' || rest[j] == '\t' || rest[j] == '\n' || rest[j] == '\r') {
			j++
		}
		if j >= len(rest) || rest[j] != '=' {
			continue
		}
		j++
		for j < len(rest) && (rest[j] == ' ' || rest[j] == '\t' || rest[j] == '\n' || rest[j] == '\r') {
			j++
		}
		if j >= len(rest) || (rest[j] != '"' && rest[j] != '\'') {
			continue
		}
		quote := rest[j]
		j++
		end := bytes.IndexByte(rest[j:], quote)
		if end < 0 {
			return nil, false
		}
		return rest[j : j+end], true
	}
	return nil, false
}
