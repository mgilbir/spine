package schemavalid

import (
	"bufio"
	"encoding/xml"
	"fmt"
	"sort"
	"strings"
)

// The committed content-order table: one line per element type, listing the
// slots its children occupy in order.
//
// Format, tab-separated: namespace, local name, then the slots. A slot is a
// comma-separated list of the names allowed there — more than one when the
// schema offers a choice — or "*" for a wildcard position, where anything goes.
//
// It is regenerated from the schemas with SPINE_UPDATE_ORDER_TABLE=1 (see
// TestUpdateChildOrderTable) and committed, because the schemas themselves
// cannot be: they are copyrighted, and CI has no copy. A generated table under
// review is the closest thing to having them there.

// FormatOrderTable renders orders as the committed file's contents.
func FormatOrderTable(orders []ChildOrder) string {
	var b strings.Builder
	b.WriteString("# Child-element order, extracted from the ISO 29500 schemas.\n")
	b.WriteString("# Regenerate with SPINE_UPDATE_ORDER_TABLE=1 on a machine that has spec/part2 and spec/part4.\n")
	b.WriteString("# namespace<TAB>element<TAB>slot<TAB>slot...   (a slot is name[,name...] or * for a wildcard)\n")

	sorted := append([]ChildOrder(nil), orders...)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Root.Space != sorted[j].Root.Space {
			return sorted[i].Root.Space < sorted[j].Root.Space
		}
		return sorted[i].Root.Local < sorted[j].Root.Local
	})
	for _, o := range sorted {
		fields := []string{o.Root.Space, o.Root.Local}
		for _, p := range o.Positions {
			if p.Wildcard {
				fields = append(fields, "*")
				continue
			}
			fields = append(fields, strings.Join(p.Names, ","))
		}
		b.WriteString(strings.Join(fields, "\t"))
		b.WriteByte('\n')
	}
	return b.String()
}

// ParseOrderTable reads the committed table.
func ParseOrderTable(contents string) (map[xml.Name]ChildOrder, error) {
	out := map[xml.Name]ChildOrder{}
	sc := bufio.NewScanner(strings.NewReader(contents))
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for line := 1; sc.Scan(); line++ {
		text := sc.Text()
		if strings.HasPrefix(text, "#") || strings.TrimSpace(text) == "" {
			continue
		}
		fields := strings.Split(text, "\t")
		if len(fields) < 2 {
			return nil, fmt.Errorf("order table line %d: want at least a namespace and an element", line)
		}
		root := xml.Name{Space: fields[0], Local: fields[1]}
		order := ChildOrder{Root: root}
		for _, f := range fields[2:] {
			if f == "*" {
				order.Positions = append(order.Positions, Position{Wildcard: true})
				continue
			}
			order.Positions = append(order.Positions, Position{Names: strings.Split(f, ",")})
		}
		out[root] = order
	}
	return out, sc.Err()
}

// CheckOrder reports how a part's children depart from the content model, or
// "" when they follow it.
//
// The children have to form a subsequence of the model: a part may leave any
// optional element out, and may repeat one the schema lets it repeat, but it
// may not move one. That is exactly the property a from-scratch part has
// nothing else to check it against.
func CheckOrder(order ChildOrder, children []string) string {
	if len(order.Positions) == 0 {
		return ""
	}
	slot := 0
	for _, child := range children {
		matched := false
		for i := slot; i < len(order.Positions); i++ {
			p := order.Positions[i]
			if p.Wildcard || contains(p.Names, child) {
				// Staying on the matched slot rather than advancing past it is
				// what allows a repeated element (maxOccurs > 1).
				slot, matched = i, true
				break
			}
		}
		if matched {
			continue
		}
		// Either the child belongs to an earlier slot — it was moved — or the
		// model does not have it at all.
		if pos := indexOfSlot(order.Positions, child); pos >= 0 {
			return fmt.Sprintf("<%s> appears after <%s>, but the schema puts it before: %s",
				child, describeSlot(order.Positions[slot]), describeModel(order.Positions))
		}
		return fmt.Sprintf("<%s> is not in the content model of <%s>: %s",
			child, order.Root.Local, describeModel(order.Positions))
	}
	return ""
}

func contains(names []string, want string) bool {
	for _, n := range names {
		if n == want {
			return true
		}
	}
	return false
}

func indexOfSlot(positions []Position, child string) int {
	for i, p := range positions {
		if contains(p.Names, child) {
			return i
		}
	}
	return -1
}

func describeSlot(p Position) string {
	if p.Wildcard {
		return "*"
	}
	return strings.Join(p.Names, "|")
}

func describeModel(positions []Position) string {
	parts := make([]string, 0, len(positions))
	for _, p := range positions {
		parts = append(parts, describeSlot(p))
	}
	model := strings.Join(parts, " ")
	if len(model) > 300 {
		model = model[:300] + "..."
	}
	return model
}

// RootChildren returns the local names of a part root's child elements, after
// the markup-compatibility rewrite so a Choice's content is not mistaken for a
// child of the root.
func RootChildren(part []byte) (xml.Name, []string) {
	part = StripIgnorable(ResolveAlternateContent(part))
	dec := xml.NewDecoder(strings.NewReader(string(part)))
	var (
		root     xml.Name
		children []string
		depth    int
	)
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch depth {
			case 0:
				root = t.Name
			case 1:
				children = append(children, t.Name.Local)
			}
			depth++
		case xml.EndElement:
			depth--
		}
	}
	return root, children
}
