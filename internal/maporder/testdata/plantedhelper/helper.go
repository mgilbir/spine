// Package plantedhelper holds the far side of a cross-package call, so the
// fixture exercises the part of the analysis that only works because every
// package in one packages.Load shares object identity: the callee resolved from
// planted.go is the very *types.Func indexed from this file's syntax.
package plantedhelper

import xmlb "github.com/mgilbir/spine/common/xml"

// WriteInto appends to a builder it does not own, so any map range that calls
// it is emitting in map order.
func WriteInto(b *xmlb.Builder, name string) {
	b.EmptyElement("", name)
}

// Render builds its own output and returns it, so calling it in a map range is
// harmless however many times and in whatever order it happens.
func Render(name string) []byte {
	b := xmlb.NewBuilder()
	b.EmptyElement("", name)
	return []byte(b.String())
}
