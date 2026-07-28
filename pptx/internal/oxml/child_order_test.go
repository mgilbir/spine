package oxml

import (
	"testing"

	xmlb "github.com/mgilbir/spine/common/xml"
	"github.com/mgilbir/spine/internal/testutil"
)

// TestPmlChildCaptureSchemaOrder pins the rank tables behind C329's fix for the
// PresentationML types that use the common/xml child-capture kit. Transcribed
// from ISO/IEC 29500-4 pml.xsd.
func TestPmlChildCaptureSchemaOrder(t *testing.T) {
	newBuilder := func() *xmlb.Builder { return xmlb.NewPresentationMLBuilder() }
	testutil.CheckSchemaChildOrder(t, newBuilder, xmlb.NSPresentationML, []testutil.SchemaOrder{
		{
			// CT_BuildList is a repeated xsd:choice of the four build kinds,
			// so any interleaving validates and the captured order is the only
			// thing that matters; a build added after parse still lands at its
			// declaration rank rather than after every captured sibling.
			Name: "BuildList", New: func() any { return &BuildList{} }, Model: testutil.Choice,
			Children: []string{"bldP", "bldDgm", "bldOleChart", "bldGraphic"},
		},
		{
			// CT_Picture is an xsd:sequence, so a spPr or style added to a
			// parsed picture must precede the captured extLst.
			Name: "Picture", New: func() any { return &Picture{} }, Model: testutil.Sequence,
			Children: []string{"nvPicPr", "blipFill", "spPr", "style", "extLst"},
		},
	})
}
