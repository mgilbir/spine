package xlsx

import "github.com/mgilbir/spine/xlsx/internal/oxml"

// cellAt builds a cell at ref for tests, applying any tweaks. The reference is
// derived from the cell's position rather than stored as a string, so it is set
// through the constructor instead of a struct field.
func cellAt(ref string, tweak ...func(*oxml.CT_Cell)) *oxml.CT_Cell {
	c := oxml.NewCell(ref)
	for _, f := range tweak {
		f(c)
	}
	return c
}
