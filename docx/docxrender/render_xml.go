package docxrender

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"

	xmlb "github.com/mgilbir/spine/common/xml"
	core "github.com/mgilbir/spine/internal/render"
	"github.com/mgilbir/spine/render"
)

const (
	nsW     = xmlb.NSWordprocessingML
	nsA     = xmlb.NSDrawingML
	nsXML   = "http://www.w3.org/XML/1998/namespace"
	nsRelat = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"
)

// wordNode is one element of a source part. The renderer reads parts as
// serialized from the document model (see wordRenderSource), so unsaved edits
// are drawn and nothing the model preserved is invisible to the profile checks.
type wordNode struct {
	space, name string
	attrs       []wordAttr
	children    []*wordNode
	// text is the element's own character data, in document order, without
	// the whitespace-only text between child elements of a container.
	text string
}

type wordAttr struct{ space, name, value string }

// is reports whether n is the WordprocessingML element local.
func (n *wordNode) is(local string) bool { return n.space == nsW && n.name == local }

// attr returns the value of the w: attribute local.
func (n *wordNode) attr(local string) (string, bool) {
	for _, a := range n.attrs {
		if a.space == nsW && a.name == local {
			return a.value, true
		}
	}
	return "", false
}

// val is the w:val attribute, or "" when absent.
func (n *wordNode) val() string { v, _ := n.attr("val"); return v }

// child returns the first WordprocessingML child named local, or nil.
func (n *wordNode) child(local string) *wordNode {
	if n == nil {
		return nil
	}
	for _, c := range n.children {
		if c.is(local) {
			return c
		}
	}
	return nil
}

// childIn returns the first child in namespace space named local, or nil.
func (n *wordNode) childIn(space, local string) *wordNode {
	if n == nil {
		return nil
	}
	for _, c := range n.children {
		if c.space == space && c.name == local {
			return c
		}
	}
	return nil
}

// on reads an OOXML on/off element: present without a value or with a true
// value is on. ok is false when the value is not a valid on/off literal.
func (n *wordNode) on() (on, ok bool) {
	v, has := n.attr("val")
	if !has {
		return true, true
	}
	switch v {
	case "1", "true", "on":
		return true, true
	case "0", "false", "off":
		return false, true
	}
	return false, false
}

// wordRenderParse reads one part into a tree after bounding it through the
// source budget: bytes, element and attribute count, nesting depth, processing
// instructions and directives. The root must be the WordprocessingML (or, for
// the theme, DrawingML) element named root.
func wordRenderParse(ctx context.Context, data []byte, budget *core.SourceBudget, rootSpace, rootName string) (*wordNode, error) {
	err := budget.CheckXML(ctx, data, func(n core.XMLNode) error {
		if len(n.Path) == 1 && (n.Name.Space != rootSpace || n.Name.Local != rootName) {
			return fmt.Errorf("%w: %s XML root", render.ErrInvalid, rootName)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	dec := xml.NewDecoder(bytes.NewReader(data))
	var stack []*wordNode
	var root *wordNode
	for {
		if err = ctx.Err(); err != nil {
			return nil, err
		}
		tok, e := dec.Token()
		if e == io.EOF {
			break
		}
		if e != nil {
			return nil, fmt.Errorf("%w: source XML: %w", render.ErrInvalid, e)
		}
		switch v := tok.(type) {
		case xml.StartElement:
			n := &wordNode{space: v.Name.Space, name: v.Name.Local}
			for _, a := range v.Attr {
				if a.Name.Space == "xmlns" || (a.Name.Space == "" && a.Name.Local == "xmlns") {
					continue
				}
				n.attrs = append(n.attrs, wordAttr{a.Name.Space, a.Name.Local, a.Value})
			}
			if len(stack) > 0 {
				parent := stack[len(stack)-1]
				parent.children = append(parent.children, n)
			} else {
				root = n
			}
			stack = append(stack, n)
		case xml.EndElement:
			stack = stack[:len(stack)-1]
		case xml.CharData:
			if len(stack) > 0 {
				stack[len(stack)-1].text += string(v)
			}
		}
	}
	if root == nil {
		return nil, fmt.Errorf("%w: empty XML", render.ErrInvalid)
	}
	return root, nil
}

// wordRenderTwips parses a signed twip count into pixels (one twip is 1/15 of
// a CSS pixel).
func wordRenderTwips(s string) (float64, bool) {
	n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 32)
	if err != nil {
		return 0, false
	}
	return float64(n) / 15, true
}

// wordRenderInt parses a signed decimal attribute within 32 bits.
func wordRenderInt(s string) (int, bool) {
	n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 32)
	if err != nil {
		return 0, false
	}
	return int(n), true
}
