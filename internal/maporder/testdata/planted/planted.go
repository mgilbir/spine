// Package planted holds the violations the guard exists to catch, next to the
// order-independent patterns it must leave alone.
//
// It lives under testdata so the Go tool ignores it, and it is type-checked
// directly by TestGuardCatchesPlantedViolations. Without it, the guard's
// detection would only ever have been observed once, by hand, by whoever wrote
// it — which is indistinguishable from a guard that matches nothing.
package planted

import (
	"bytes"
	"sort"

	xmlb "github.com/mgilbir/spine/common/xml"
)

type part struct {
	name string
	data []byte
}

// ---------------------------------------------------------------------------
// Violations
// ---------------------------------------------------------------------------

// badEmitToBuffer writes each entry straight into a buffer that outlives the
// loop, so the bytes come out in whatever order the runtime chose.
func badEmitToBuffer(parts map[string]*part) []byte {
	var buf bytes.Buffer
	for name := range parts {
		buf.WriteString(name)
	}
	return buf.Bytes()
}

// badEmitToBuilder is the same defect against the XML builder this library
// actually serializes with.
func badEmitToBuilder(b *xmlb.Builder, parts map[string]*part) {
	for name, p := range parts {
		b.StartElement("", "Override")
		b.WriteRaw(p.data)
		b.EndElement("", name)
	}
}

// badEmitThroughHelper hides the write one call deeper: writeOne does not own
// the builder it writes to, so calling it from a map range is the same defect.
func badEmitThroughHelper(b *xmlb.Builder, parts map[string]*part) {
	for name := range parts {
		writeOne(b, name)
	}
}

func writeOne(b *xmlb.Builder, name string) {
	b.EmptyElement("", name)
}

// badCollectUnsorted is C497: the returned slice is in map order, but the
// caller — and the godoc — treat it as an order.
func badCollectUnsorted(parts map[string]*part) []string {
	var names []string
	for name := range parts {
		names = append(names, name)
	}
	return names
}

// badSelectFirstMatch is C515: with two byte-identical parts stored under
// different names, which name is returned varies between runs.
func badSelectFirstMatch(parts map[string]*part, data []byte) string {
	for name, p := range parts {
		if bytes.Equal(p.data, data) {
			return name
		}
	}
	return ""
}

// badEscapeOutward is the same selection written as an assignment.
func badEscapeOutward(parts map[string]*part, data []byte) string {
	found := ""
	for name, p := range parts {
		if bytes.Equal(p.data, data) {
			found = name
			break
		}
	}
	return found
}

// ---------------------------------------------------------------------------
// Order-independent patterns the guard must not report
// ---------------------------------------------------------------------------

// okCollectThenSort is the established pattern in this codebase.
func okCollectThenSort(parts map[string]*part) []string {
	names := make([]string, 0, len(parts))
	for name := range parts {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// okCollectThenSortViaClosure writes the same thing with the `add` helper this
// repository's knownPartNames functions use; inlining the closure is what keeps
// it from reading as an opaque call.
func okCollectThenSortViaClosure(parts map[string]*part) []string {
	seen := map[string]bool{}
	var out []string
	add := func(name string) {
		if seen[name] {
			return
		}
		seen[name] = true
		out = append(out, name)
	}
	for name := range parts {
		add(name)
	}
	sort.Strings(out)
	return out
}

// okSetInsert builds another map; a set has no order to get wrong.
func okSetInsert(parts map[string]*part) map[string]bool {
	used := make(map[string]bool, len(parts))
	for name := range parts {
		used[name] = true
	}
	return used
}

// okMaxFold folds into an aggregate whose result is the same whatever order the
// candidates arrive in.
func okMaxFold(sizes map[string]int) (int, int) {
	max, n := 0, 0
	for _, size := range sizes {
		if size > max {
			max = size
		}
		n++
	}
	return max, n
}

// okExistenceCheck returns a bool, so which entry satisfied it does not show.
func okExistenceCheck(parts map[string]*part, data []byte) bool {
	for _, p := range parts {
		if bytes.Equal(p.data, data) {
			return true
		}
	}
	return false
}

// okPositionalStore writes by index, not by sequence.
func okPositionalStore(cols map[int]string, width int) []string {
	fields := make([]string, width)
	for col, v := range cols {
		fields[col-1] = v
	}
	return fields
}

// okMarshalOwnsItsBuilder must not be reported: the builder is created here, so
// each iteration's output is self-contained and lands in a map, not in a shared
// stream. This is the case that makes a naive "calls something that writes"
// rule useless — every Marshal method in the repository has this shape.
func okMarshalOwnsItsBuilder(parts map[string]*part) map[string][]byte {
	out := make(map[string][]byte, len(parts))
	for name, p := range parts {
		b := xmlb.NewBuilder()
		b.StartElement("", "part")
		b.WriteRaw(p.data)
		b.EndElement("", "part")
		out[name] = []byte(b.String())
	}
	return out
}

// okDeleteFromMap removes entries; the surviving set does not depend on order.
func okDeleteFromMap(parts map[string]*part, drop map[string]bool) {
	for name := range drop {
		delete(parts, name)
	}
}
