package diagram

import (
	xmlb "github.com/mgilbir/spine/common/xml"
	"sort"
	"strings"
)

// ParseDataModel decodes a diagram data part (dgm:dataModel, the SmartArt data
// part) into a DataModel. The data part holds the node text and the hierarchy;
// it is the high-value part for reading a SmartArt/diagram.
func ParseDataModel(data []byte) (*DataModel, error) {
	var dm DataModel
	if err := xmlb.Unmarshal(data, &dm); err != nil {
		return nil, err
	}
	return &dm, nil
}

// TextNode is a node in a diagram's text hierarchy: the text of a data point
// and its child points, in document order. It is the reading-friendly view of
// the dgm:dataModel's points (dgm:pt) and parent-of connections (dgm:cxn).
type TextNode struct {
	// Text is the point's text, paragraphs joined by "\n".
	Text string
	// ModelID is the data point's modelId, exposed so callers can correlate a
	// node with the underlying data model if needed.
	ModelID string
	// Children are the point's child points, ordered by connection order.
	Children []*TextNode
}

// TextTree returns the diagram's content points as a forest of TextNodes: the
// top-level nodes and their descendants, following the "parOf" connections in
// dgm:cxnLst and ordered by each connection's srcOrd. Presentation-only points
// (pres/parTrans/sibTrans) and their connections are ignored — only content
// points (node, asst, and the doc root) participate. A nil or empty data model
// yields no nodes.
func (dm *DataModel) TextTree() []*TextNode {
	if dm == nil || dm.PtLst == nil {
		return nil
	}

	// Index content points by modelId. Content points are the document root
	// ("doc") and the actual nodes ("node"/"asst"); an empty type defaults to a
	// node. Presentation/transition points are skipped.
	byID := make(map[string]*Pt, len(dm.PtLst.Pt))
	var rootID string
	for _, pt := range dm.PtLst.Pt {
		if pt == nil {
			continue
		}
		switch pt.Type {
		case "doc":
			byID[pt.ModelId] = pt
			rootID = pt.ModelId
		case "", "node", "asst":
			byID[pt.ModelId] = pt
		}
	}
	if len(byID) == 0 {
		return nil
	}

	// Collect parent-of connections between content points, ordered by srcOrd
	// so siblings keep their authored order.
	type childRef struct {
		id  string
		ord uint32
	}
	children := make(map[string][]childRef)
	hasParent := make(map[string]bool)
	if dm.CxnLst != nil {
		for _, cxn := range dm.CxnLst.Cxn {
			// The type attribute defaults to "parOf" (ST_CxnType default), so an
			// omitted type is a parent-of connection.
			if cxn == nil || (cxn.Type != "" && cxn.Type != "parOf") {
				continue
			}
			if _, ok := byID[cxn.SrcId]; !ok {
				continue
			}
			if _, ok := byID[cxn.DestId]; !ok {
				continue
			}
			children[cxn.SrcId] = append(children[cxn.SrcId], childRef{id: cxn.DestId, ord: cxn.SrcOrd})
			hasParent[cxn.DestId] = true
		}
	}
	for id := range children {
		refs := children[id]
		sort.SliceStable(refs, func(i, j int) bool { return refs[i].ord < refs[j].ord })
		children[id] = refs
	}

	// build materializes a point and its subtree, guarding against cycles in
	// malformed connection lists.
	visited := make(map[string]bool)
	var build func(id string) *TextNode
	build = func(id string) *TextNode {
		if visited[id] {
			return nil
		}
		visited[id] = true
		pt := byID[id]
		node := &TextNode{ModelID: id, Text: pointText(pt)}
		for _, ref := range children[id] {
			if child := build(ref.id); child != nil {
				node.Children = append(node.Children, child)
			}
		}
		return node
	}

	// Prefer the doc root's children as the top level. Fall back to any content
	// point without a parent when there is no doc point.
	if rootID != "" {
		root := build(rootID)
		if root != nil {
			return root.Children
		}
	}

	var roots []*TextNode
	for _, pt := range dm.PtLst.Pt {
		if pt == nil {
			continue
		}
		if _, ok := byID[pt.ModelId]; !ok {
			continue
		}
		if pt.Type == "doc" || hasParent[pt.ModelId] || visited[pt.ModelId] {
			continue
		}
		if node := build(pt.ModelId); node != nil {
			roots = append(roots, node)
		}
	}
	return roots
}

// pointText extracts a data point's text: the runs of each paragraph in its
// text body, with paragraphs joined by "\n". Empty points yield "".
func pointText(pt *Pt) string {
	if pt == nil || pt.T == nil {
		return ""
	}
	var paras []string
	for _, p := range pt.T.P {
		if p == nil {
			continue
		}
		var sb strings.Builder
		for _, r := range p.R {
			if r != nil {
				sb.WriteString(r.T)
			}
		}
		paras = append(paras, sb.String())
	}
	return strings.Join(paras, "\n")
}
