package xml

import (
	"bytes"
	"strconv"
	"strings"
	"testing"
)

// TestDrainToProducesTheSameDocument is the property the streaming worksheet
// writer depends on: draining partway through must change nothing about the
// bytes produced, only when they leave the builder.
func TestDrainToProducesTheSameDocument(t *testing.T) {
	const ns = "http://example.com/ns"

	build := func(drainEvery int, sink *bytes.Buffer) string {
		b := NewBuilder()
		b.RegisterNamespace(ns, "e")
		b.WriteHeader()
		b.StartElementWithNS(ns, "root", []NSDecl{{Prefix: "e", URI: ns}})
		for i := 0; i < 20; i++ {
			b.StartElement(ns, "row", StrAttr("r", strconv.Itoa(i)))
			b.WriteElement(ns, "v", "value "+strconv.Itoa(i))
			b.EndElement(ns, "row")
			if drainEvery > 0 && i%drainEvery == drainEvery-1 {
				if _, err := b.DrainTo(sink); err != nil {
					t.Fatalf("drain: %v", err)
				}
			}
		}
		b.EndElement(ns, "root")
		if err := b.Finish(); err != nil {
			t.Fatalf("finish: %v", err)
		}
		if drainEvery > 0 {
			if _, err := b.DrainTo(sink); err != nil {
				t.Fatalf("final drain: %v", err)
			}
			return sink.String()
		}
		return string(b.Bytes())
	}

	whole := build(0, nil)
	for _, every := range []int{1, 3, 7, 20} {
		var sink bytes.Buffer
		got := build(every, &sink)
		if got != whole {
			t.Errorf("draining every %d rows changed the output:\n got %q\nwant %q", every, got, whole)
		}
	}
	if !strings.Contains(whole, "value 19") {
		t.Fatalf("test built nothing useful: %q", whole)
	}
}

// TestDrainToKeepsEmptyElementCollapse pins the construct that looks like
// backtracking. An element with no content is written as a self-closing tag by
// deferring its '>' — if a drain landed between the start tag and that
// decision, the deferred byte must still arrive.
func TestDrainToKeepsEmptyElementCollapse(t *testing.T) {
	const ns = "http://example.com/ns"
	var sink bytes.Buffer
	b := NewBuilder()
	b.RegisterNamespace(ns, "e")
	b.StartElementWithNS(ns, "root", []NSDecl{{Prefix: "e", URI: ns}})
	b.StartElement(ns, "empty")
	if _, err := b.DrainTo(&sink); err != nil {
		t.Fatal(err)
	}
	b.EndElement(ns, "empty")
	b.EndElement(ns, "root")
	if err := b.Finish(); err != nil {
		t.Fatal(err)
	}
	if _, err := b.DrainTo(&sink); err != nil {
		t.Fatal(err)
	}
	got := sink.String()
	if !strings.Contains(got, "<e:empty") || !strings.Contains(got, "</e:root>") {
		t.Fatalf("drained document is malformed: %q", got)
	}
	if strings.Count(got, "<e:empty") != 1 {
		t.Errorf("empty element emitted oddly: %q", got)
	}
}

// TestDrainToKeepsContentBaseline exercises both places the builder records a
// position and reads it back later. A drain in between resets the buffer, so
// those baselines have to be absolute; if either is not, the comparison
// silently flips and the document changes.
//
// Both modes are needed because they take different paths. Without
// collapseEmpty, an element that wrote nothing reaches writeCloseIndent, which
// compares against the baseline recorded when the element opened — in separator
// mode that decides whether a separator lands inside it, turning <t></t> into
// <t> </t>. With collapseEmpty the start tag's '>' is deferred instead, and the
// baseline is re-recorded by markContentStart when it is finally written.
func TestDrainToKeepsContentBaseline(t *testing.T) {
	const ns = "http://example.com/ns"

	// drainAt: -1 never drains; otherwise drain at that step.
	build := func(collapse bool, drainAt int) string {
		var sink bytes.Buffer
		b := NewBuilder()
		b.RegisterNamespace(ns, "e")
		b.SetElementSeparator(" ")
		b.SetCollapseEmptyElements(collapse)
		b.StartElementWithNS(ns, "root", []NSDecl{{Prefix: "e", URI: ns}})
		step := 0
		maybeDrain := func() {
			if drainAt >= 0 && step == drainAt {
				if _, err := b.DrainTo(&sink); err != nil {
					t.Fatalf("drain: %v", err)
				}
			}
			step++
		}
		maybeDrain()
		b.StartElement(ns, "empty") // writes nothing: must not gain a separator
		maybeDrain()
		b.EndElement(ns, "empty")
		maybeDrain()
		b.StartElement(ns, "full")
		maybeDrain()
		b.WriteElement(ns, "v", "x")
		maybeDrain()
		b.EndElement(ns, "full")
		maybeDrain()
		b.EndElement(ns, "root")
		if err := b.Finish(); err != nil {
			t.Fatalf("finish: %v", err)
		}
		if _, err := b.DrainTo(&sink); err != nil {
			t.Fatalf("final drain: %v", err)
		}
		return sink.String()
	}

	for _, collapse := range []bool{false, true} {
		want := build(collapse, -1)
		if strings.Contains(want, "<e:empty> </e:empty>") {
			t.Fatalf("collapse=%v baseline already wrong, separator landed inside the empty element: %q",
				collapse, want)
		}
		for at := 0; at <= 6; at++ {
			if got := build(collapse, at); got != want {
				t.Errorf("collapse=%v, draining at step %d changed the output:\n got %q\nwant %q",
					collapse, at, got, want)
			}
		}
	}
}
