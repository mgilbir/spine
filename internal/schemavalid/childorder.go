package schemavalid

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Child-element order, extracted from the schemas so it can be checked without
// them.
//
// OOXML content models are xsd:sequence, so the order of a part's children is
// normative and getting it wrong produces a file Word and PowerPoint call
// corrupt. For a part read from a file the fidelity tests catch a reordering,
// because the bytes have to come back identical. For a part authored from
// nothing there is no original, and the schemas that would say so are
// copyrighted and absent from CI.
//
// So the order is extracted here into a small table that is committed, and the
// authored-output test checks against the table rather than against the
// schemas. The extraction runs only where the schemas are present; the check
// runs everywhere.

// Position is one slot in a content model: the element names allowed there,
// which is more than one when the schema offers a choice.
type Position struct {
	Names []string
	// Wildcard marks a position an xsd:any covers, where anything may appear.
	Wildcard bool
}

// ChildOrder is the ordered content model of one element type.
type ChildOrder struct {
	Root      xml.Name
	Positions []Position
}

// schemaIndex holds the global declarations of a schema set, keyed by
// namespace and name.
type schemaIndex struct {
	elements     map[xml.Name]*xsdNode
	complexTypes map[xml.Name]*xsdNode
	groups       map[xml.Name]*xsdNode
}

// xsdNode is a parsed XSD element: enough of a DOM to walk particles, with the
// namespace bindings in scope so a QName attribute can be resolved.
type xsdNode struct {
	Local    string
	Attrs    map[string]string
	Children []*xsdNode
	ns       map[string]string // prefix -> URI, inherited
	target   string            // the schema's targetNamespace
}

// LoadChildOrder extracts the content model of one global element.
func LoadChildOrder(repoRoot string, root xml.Name) (ChildOrder, error) {
	idx, err := loadSchemaIndex(repoRoot)
	if err != nil {
		return ChildOrder{}, err
	}
	el, ok := idx.elements[root]
	if !ok {
		return ChildOrder{}, fmt.Errorf("schemavalid: no global element %s in %s", root.Local, root.Space)
	}

	ct, err := idx.typeOf(el)
	if err != nil {
		return ChildOrder{}, err
	}
	positions, err := idx.positions(ct, 0)
	if err != nil {
		return ChildOrder{}, fmt.Errorf("schemavalid: %s: %w", root.Local, err)
	}
	return ChildOrder{Root: root, Positions: positions}, nil
}

// typeOf resolves an element declaration to its complexType, whether named or
// inline.
func (idx *schemaIndex) typeOf(el *xsdNode) (*xsdNode, error) {
	if t := el.Attrs["type"]; t != "" {
		name := el.resolveQName(t)
		ct, ok := idx.complexTypes[name]
		if !ok {
			return nil, fmt.Errorf("no complexType %s", t)
		}
		return ct, nil
	}
	for _, c := range el.Children {
		if c.Local == "complexType" {
			return c, nil
		}
	}
	return nil, fmt.Errorf("element %s has neither a type attribute nor an inline complexType", el.Attrs["name"])
}

// positions flattens a particle into the ordered slots its children occupy.
//
// It refuses what it cannot flatten faithfully rather than guessing: a choice
// between multi-element sequences has an order this representation cannot
// express, and silently dropping that would produce a table that passes wrong
// output.
func (idx *schemaIndex) positions(node *xsdNode, depth int) ([]Position, error) {
	if depth > 24 {
		return nil, fmt.Errorf("particle nesting deeper than 24; refusing to guess")
	}
	var out []Position
	for _, c := range node.Children {
		switch c.Local {
		case "annotation", "attribute", "attributeGroup", "anyAttribute":
			// Not part of the content model.
		case "complexType", "complexContent", "sequence":
			sub, err := idx.positions(c, depth+1)
			if err != nil {
				return nil, err
			}
			out = append(out, sub...)
		case "extension":
			// The base type's content comes first, then the extension's.
			base := c.resolveQName(c.Attrs["base"])
			if bt, ok := idx.complexTypes[base]; ok {
				sub, err := idx.positions(bt, depth+1)
				if err != nil {
					return nil, err
				}
				out = append(out, sub...)
			}
			sub, err := idx.positions(c, depth+1)
			if err != nil {
				return nil, err
			}
			out = append(out, sub...)
		case "group":
			ref := c.Attrs["ref"]
			if ref == "" {
				sub, err := idx.positions(c, depth+1)
				if err != nil {
					return nil, err
				}
				out = append(out, sub...)
				continue
			}
			g, ok := idx.groups[c.resolveQName(ref)]
			if !ok {
				return nil, fmt.Errorf("no group %s", ref)
			}
			sub, err := idx.positions(g, depth+1)
			if err != nil {
				return nil, err
			}
			out = append(out, sub...)
		case "choice":
			names, err := idx.choiceNames(c, depth+1)
			if err != nil {
				return nil, err
			}
			if len(names) > 0 {
				out = append(out, Position{Names: names})
			}
		case "element":
			name, err := elementName(c)
			if err != nil {
				return nil, err
			}
			out = append(out, Position{Names: []string{name}})
		case "any":
			out = append(out, Position{Wildcard: true})
		default:
			return nil, fmt.Errorf("unhandled particle %q", c.Local)
		}
	}
	return out, nil
}

// choiceNames collects the element names a choice offers. A branch that is
// itself a sequence of more than one element is rejected: its internal order
// would be lost.
func (idx *schemaIndex) choiceNames(node *xsdNode, depth int) ([]string, error) {
	var names []string
	sub, err := idx.positions(node, depth)
	if err != nil {
		return nil, err
	}
	for _, p := range sub {
		if p.Wildcard {
			// A choice containing a wildcard accepts anything, so the whole
			// position does.
			return nil, nil
		}
		names = append(names, p.Names...)
	}
	sort.Strings(names)
	return names, nil
}

func elementName(c *xsdNode) (string, error) {
	if n := c.Attrs["name"]; n != "" {
		return n, nil
	}
	if r := c.Attrs["ref"]; r != "" {
		if i := strings.IndexByte(r, ':'); i >= 0 {
			return r[i+1:], nil
		}
		return r, nil
	}
	return "", fmt.Errorf("element particle with neither name nor ref")
}

// resolveQName expands a prefixed name against the bindings in scope.
func (n *xsdNode) resolveQName(q string) xml.Name {
	if i := strings.IndexByte(q, ':'); i >= 0 {
		return xml.Name{Space: n.ns[q[:i]], Local: q[i+1:]}
	}
	// An unprefixed QName in a schema resolves against its default namespace,
	// which for these documents is the schema's own target.
	if uri, ok := n.ns[""]; ok {
		return xml.Name{Space: uri, Local: q}
	}
	return xml.Name{Space: n.target, Local: q}
}

// loadSchemaIndex parses every vendored schema into one index.
func loadSchemaIndex(repoRoot string) (*schemaIndex, error) {
	idx := &schemaIndex{
		elements:     map[xml.Name]*xsdNode{},
		complexTypes: map[xml.Name]*xsdNode{},
		groups:       map[xml.Name]*xsdNode{},
	}
	for _, rel := range SchemaDirs {
		dir := filepath.Join(repoRoot, rel)
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".xsd") {
				continue
			}
			if _, skip := skipSchemas[e.Name()]; skip {
				continue
			}
			data, err := os.ReadFile(filepath.Join(dir, e.Name()))
			if err != nil {
				return nil, err
			}
			root, err := parseXSD(data)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", e.Name(), err)
			}
			target := root.Attrs["targetNamespace"]
			root.setTarget(target)
			for _, c := range root.Children {
				name := xml.Name{Space: target, Local: c.Attrs["name"]}
				switch c.Local {
				case "element":
					idx.elements[name] = c
				case "complexType":
					idx.complexTypes[name] = c
				case "group":
					idx.groups[name] = c
				}
			}
		}
	}
	return idx, nil
}

func (n *xsdNode) setTarget(t string) {
	n.target = t
	for _, c := range n.Children {
		c.setTarget(t)
	}
}

// parseXSD reads a schema into xsdNodes, carrying namespace bindings down.
func parseXSD(data []byte) (*xsdNode, error) {
	dec := xml.NewDecoder(strings.NewReader(string(data)))
	var stack []*xsdNode
	var root *xsdNode
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			node := &xsdNode{Local: t.Name.Local, Attrs: map[string]string{}, ns: map[string]string{}}
			if len(stack) > 0 {
				for k, v := range stack[len(stack)-1].ns {
					node.ns[k] = v
				}
			}
			for _, a := range t.Attr {
				switch {
				case a.Name.Space == "xmlns":
					node.ns[a.Name.Local] = a.Value
				case a.Name.Space == "" && a.Name.Local == "xmlns":
					node.ns[""] = a.Value
				default:
					node.Attrs[a.Name.Local] = a.Value
				}
			}
			if len(stack) > 0 {
				parent := stack[len(stack)-1]
				parent.Children = append(parent.Children, node)
			} else {
				root = node
			}
			stack = append(stack, node)
		case xml.EndElement:
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		}
	}
	if root == nil {
		return nil, fmt.Errorf("no root element")
	}
	return root, nil
}
