package render

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
)

// SourceBudget bounds adapter capability inspection across parts. Callers must
// also bound model construction and font/image input before invoking painters.
type SourceBudget struct {
	Bytes int64
	Nodes int
}

func NewSourceBudget(bytes int64, nodes int) (*SourceBudget, error) {
	if bytes < 0 || nodes < 0 {
		return nil, fmt.Errorf("%w: source limits", ErrInvalid)
	}
	if bytes == 0 {
		bytes = 16 << 20
	}
	if nodes == 0 {
		nodes = 100000
	}
	return &SourceBudget{Bytes: bytes, Nodes: nodes}, nil
}

// utf8BOM is the UTF-8 encoding of U+FEFF. XML 1.0 §4.3.3 permits it as the
// first bytes of a UTF-8 entity; Word writes it on some parts.
var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

// CheckXML checks every namespace-qualified element and attribute through allow.
// Unknown source markup must not disappear through a lossy model projection.
// Namespace declarations and the ordinary XML declaration are metadata. Other
// processing instructions/directives are rejected. Bytes/nodes are cumulative.
// A byte order mark is accepted only as the first bytes of data, and the XML
// declaration only as the first token after it.
func (b *SourceBudget) CheckXML(ctx context.Context, data []byte, allow func(XMLNode) error) error {
	if int64(len(data)) > b.Bytes {
		return fmt.Errorf("%w: source bytes", ErrLimit)
	}
	b.Bytes -= int64(len(data))
	// encoding/xml returns a leading BOM as character data, which would read
	// as content outside the root.
	dec := xml.NewDecoder(bytes.NewReader(bytes.TrimPrefix(data, utf8BOM)))
	first := true
	depth := 0
	path := []xml.Name{}
	siblings := []map[xml.Name]int{{}}
	roots := 0
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		tok, err := dec.Token()
		if err == io.EOF {
			if depth != 0 || roots != 1 {
				return fmt.Errorf("%w: XML depth", ErrInvalid)
			}
			return nil
		}
		if err != nil {
			return fmt.Errorf("%w: source XML: %w", ErrInvalid, err)
		}
		atStart := first
		first = false
		switch v := tok.(type) {
		case xml.StartElement:
			if depth == 0 {
				roots++
				if roots > 1 {
					return fmt.Errorf("%w: multiple XML roots", ErrInvalid)
				}
			}
			b.Nodes--
			depth++
			if b.Nodes < 0 || depth > 64 {
				return fmt.Errorf("%w: source nodes/depth", ErrLimit)
			}
			if len(v.Attr) > b.Nodes {
				return fmt.Errorf("%w: source attributes", ErrLimit)
			}
			b.Nodes -= len(v.Attr)
			seen := map[xml.Name]bool{}
			for _, a := range v.Attr {
				if seen[a.Name] {
					return fmt.Errorf("%w: duplicate XML attribute", ErrInvalid)
				}
				seen[a.Name] = true
			}
			parent := siblings[len(siblings)-1]
			parent[v.Name]++
			path = append(path, v.Name)
			if err = allow(XMLNode{StartElement: v, Path: path, Occurrence: parent[v.Name]}); err != nil {
				return err
			}
			siblings = append(siblings, map[xml.Name]int{})
		case xml.EndElement:
			depth--
			path = path[:len(path)-1]
			siblings = siblings[:len(siblings)-1]
		case xml.CharData:
			if isXMLSpace(v) {
				break
			}
			if depth == 0 {
				return fmt.Errorf("%w: content outside XML root", ErrInvalid)
			}
			if err = allow(XMLNode{Path: path, Text: true}); err != nil {
				return err
			}
		case xml.Directive:
			return fmt.Errorf("%w: XML directive", ErrUnsupported)
		case xml.ProcInst:
			if v.Target != "xml" {
				return fmt.Errorf("%w: XML instruction", ErrUnsupported)
			}
			if !atStart {
				return fmt.Errorf("%w: misplaced XML declaration", ErrInvalid)
			}
		}
	}
}

// isXMLSpace reports whether text is XML white space (XML 1.0 §2.3 production
// S). Unicode spaces such as U+00A0 and U+FEFF are content, so
// strings.TrimSpace is the wrong test.
func isXMLSpace(text []byte) bool {
	for _, c := range text {
		if c != ' ' && c != '\t' && c != '\r' && c != '\n' {
			return false
		}
	}
	return true
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r *contextReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}

// XMLNode carries a temporary element path and its occurrence within its parent.
// A capability profile must reject repeated singleton elements before projection.
type XMLNode struct {
	xml.StartElement
	Path       []xml.Name
	Occurrence int
	Text       bool
}

// CheckReader bounds bytes before allocating a source copy. The underlying
// reader must provide its own cancellation when a Read can block.
func (b *SourceBudget) CheckReader(ctx context.Context, r io.Reader, allow func(XMLNode) error) error {
	maxInt := int64(int(^uint(0) >> 1))
	if b.Bytes >= maxInt {
		return fmt.Errorf("%w: source address space", ErrLimit)
	}
	data, err := io.ReadAll(io.LimitReader(&contextReader{ctx: ctx, r: r}, b.Bytes+1))
	if err != nil {
		return err
	}
	return b.CheckXML(ctx, data, allow)
}
