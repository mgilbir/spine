// Package view exposes the xlsx model's unexported state to the range
// renderer, xlsx/xlsxrender, without the xlsx package importing it. xlsx
// sets the function variables at init.
package view

import (
	coxml "github.com/mgilbir/spine/common/oxml"
	"github.com/mgilbir/spine/opc"
	"github.com/mgilbir/spine/xlsx/internal/oxml"
)

// Sheet is a worksheet's state as the renderer reads it. Load parses the
// worksheet part on first use; the renderer calls it only after checking
// the part's source bytes.
type Sheet struct {
	Opaque         bool
	PartName       string
	WorkbookPart   string
	Relationships  []*opc.Relationship
	PreservedParts map[string]*coxml.RawPart
	Stylesheet     *oxml.CT_Stylesheet
	SharedStrings  *oxml.CT_Sst
	Load           func() *oxml.CT_Worksheet
}

var (
	// SheetOf snapshots s, a *xlsx.Sheet; it returns nil for a sheet
	// detached from its workbook.
	SheetOf func(s any) *Sheet
	// DefaultStylesheet returns the stylesheet a new workbook saves.
	DefaultStylesheet func() *oxml.CT_Stylesheet
	// ParseRange parses an A1 range or single cell reference into its
	// normalized 1-based bounds.
	ParseRange func(ref string) (minRow, minCol, maxRow, maxCol int, err error)
)
