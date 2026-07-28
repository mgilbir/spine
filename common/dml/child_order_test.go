package dml

import (
	"testing"

	xmlb "github.com/mgilbir/spine/common/xml"
	"github.com/mgilbir/spine/internal/testutil"
)

// TestDmlChildCaptureSchemaOrder pins the rank tables behind C329's fix for the
// DrawingML types that use the common/xml child-capture kit. Transcribed from
// ISO/IEC 29500-4 dml-main.xsd.
func TestDmlChildCaptureSchemaOrder(t *testing.T) {
	newBuilder := func() *xmlb.Builder { return xmlb.NewPresentationMLBuilder() }
	testutil.CheckSchemaChildOrder(t, newBuilder, xmlb.NSDrawingML, []testutil.SchemaOrder{
		{
			// CT_Blip is a repeated xsd:choice of image-transform effects
			// (order free, though it is semantically significant since effects
			// compose) followed by extLst, which the sequence pins last — the
			// element an appended addition used to overtake.
			Name: "Blip", New: func() any { return &Blip{} }, Model: testutil.Sequence,
			Children: []string{
				"alphaBiLevel", "alphaCeiling", "alphaFloor", "alphaInv", "alphaMod",
				"alphaModFix", "alphaRepl", "biLevel", "blur", "clrChange", "clrRepl",
				"duotone", "fillOverlay", "grayscl", "hsl", "lum", "tint", "extLst",
			},
		},
		{
			// CT_TextListStyle is an xsd:sequence; its trailing extLst is
			// unmodeled and replays as a raw captured child. This is the type
			// whose EnsureLevel motivated InsertTypedField, the one-off form of
			// the insertion the marshaler now applies everywhere.
			Name: "LstStyle", New: func() any { return &LstStyle{} }, Model: testutil.Sequence,
			Children: []string{
				"defPPr", "lvl1pPr", "lvl2pPr", "lvl3pPr", "lvl4pPr", "lvl5pPr",
				"lvl6pPr", "lvl7pPr", "lvl8pPr", "lvl9pPr",
			},
		},
	})
}
