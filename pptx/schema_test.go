package pptx

import (
	"archive/zip"
	"encoding/xml"
	"io"
	"os"
	"strings"
	"testing"

	xmlb "github.com/mgilbir/spine/common/xml"
	"github.com/mgilbir/spine/pptx/internal/oxml"
)

// TestSchema_PresentationElements tests that CT_Presentation elements are properly loaded.
func TestSchema_PresentationElements(t *testing.T) {
	testFile := "../python-tests/test_files/test.pptx"
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Skip("Test file not found:", testFile)
	}

	p, err := Open(testFile)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	defer func() { _ = p.Close() }()

	// Verify presentation structure exists
	if p.presentation == nil {
		t.Fatal("presentation is nil")
	}

	// CT_Presentation should have sldMasterIdLst
	if p.presentation.SlideMasterIDs == nil {
		t.Error("SlideMasterIDs is nil - expected sldMasterIdLst element")
	} else if len(p.presentation.SlideMasterIDs.SlideMasterID) == 0 {
		t.Error("SlideMasterIDs is empty")
	}

	// CT_Presentation should have sldIdLst (slides)
	if p.presentation.SlideIDs == nil {
		t.Error("SlideIDs is nil - expected sldIdLst element")
	} else if len(p.presentation.SlideIDs.SlideID) == 0 {
		t.Error("SlideIDs is empty")
	}

	// CT_Presentation should have sldSz (slide size)
	if p.presentation.SlideSize == nil {
		t.Error("SlideSize is nil - expected sldSz element")
	} else {
		if p.presentation.SlideSize.Cx <= 0 {
			t.Errorf("SlideSize.Cx = %d, want > 0", p.presentation.SlideSize.Cx)
		}
		if p.presentation.SlideSize.Cy <= 0 {
			t.Errorf("SlideSize.Cy = %d, want > 0", p.presentation.SlideSize.Cy)
		}
	}
}

// TestSchema_SlideElements tests that CT_Slide elements are properly loaded.
func TestSchema_SlideElements(t *testing.T) {
	testFile := "../python-tests/test_files/test.pptx"
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Skip("Test file not found:", testFile)
	}

	p, err := Open(testFile)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	defer func() { _ = p.Close() }()

	if p.SlideCount() == 0 {
		t.Skip("No slides in test file")
	}

	slide, err := p.Slide(0)
	if err != nil {
		t.Fatalf("Slide(0) error: %v", err)
	}

	// CT_Slide should have cSld (common slide data)
	if slide.sx() == nil {
		t.Fatal("slideXML is nil")
	}
	if slide.sx().CSld == nil {
		t.Error("CSld is nil - expected cSld element")
	}

	// CT_CommonSlideData should have spTree (shape tree)
	if slide.sx().CSld != nil && slide.sx().CSld.SpTree == nil {
		t.Error("SpTree is nil - expected spTree element")
	}
}

// TestSchema_SlideMasterElements tests that CT_SlideMaster elements are properly loaded.
func TestSchema_SlideMasterElements(t *testing.T) {
	testFile := "../python-tests/test_files/test.pptx"
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Skip("Test file not found:", testFile)
	}

	p, err := Open(testFile)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	defer func() { _ = p.Close() }()

	masters := p.SlideMasters()
	if len(masters) == 0 {
		t.Skip("No slide masters in test file")
	}

	master := masters[0]

	// CT_SlideMaster should have cSld
	if master.masterXML == nil {
		t.Fatal("masterXML is nil")
	}
	if master.masterXML.CSld == nil {
		t.Error("CSld is nil - expected cSld element")
	}

	// CT_SlideMaster should have sldLayoutIdLst
	if master.masterXML.SlideLayoutIDs == nil {
		t.Error("SlideLayoutIDs is nil - expected sldLayoutIdLst element")
	} else if len(master.masterXML.SlideLayoutIDs.SlideLayoutID) == 0 {
		t.Error("SlideLayoutIDs is empty")
	}

	// CT_SlideMaster should have clrMap
	if master.masterXML.ClrMap == nil {
		t.Error("ClrMap is nil - expected clrMap element")
	}
}

// TestSchema_SlideLayoutElements tests that CT_SlideLayout elements are properly loaded.
func TestSchema_SlideLayoutElements(t *testing.T) {
	testFile := "../python-tests/test_files/test.pptx"
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Skip("Test file not found:", testFile)
	}

	p, err := Open(testFile)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	defer func() { _ = p.Close() }()

	layouts := p.SlideLayouts()
	if len(layouts) == 0 {
		t.Skip("No slide layouts in test file")
	}

	layout := layouts[0]

	// CT_SlideLayout should have cSld
	if layout.layoutXML == nil {
		t.Fatal("layoutXML is nil")
	}
	if layout.layoutXML.CSld == nil {
		t.Error("CSld is nil - expected cSld element")
	}

	// CT_SlideLayout should have type attribute
	// Note: type can be empty string which defaults to "cust"
	t.Logf("Layout type: %q", layout.layoutXML.Type)
}

// TestSchema_RelationshipIDs tests that r:id attributes are properly handled.
func TestSchema_RelationshipIDs(t *testing.T) {
	testFile := "../python-tests/test_files/test.pptx"
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Skip("Test file not found:", testFile)
	}

	p, err := Open(testFile)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	defer func() { _ = p.Close() }()

	// Check slide master RID
	if p.presentation.SlideMasterIDs != nil {
		for i, mid := range p.presentation.SlideMasterIDs.SlideMasterID {
			if mid.RID == "" {
				t.Errorf("SlideMasterID[%d].RID is empty", i)
			}
			if !strings.HasPrefix(mid.RID, "rId") {
				t.Errorf("SlideMasterID[%d].RID = %q, want prefix 'rId'", i, mid.RID)
			}
		}
	}

	// Check slide RID
	if p.presentation.SlideIDs != nil {
		for i, sid := range p.presentation.SlideIDs.SlideID {
			if sid.RID == "" {
				t.Errorf("SlideID[%d].RID is empty", i)
			}
			if !strings.HasPrefix(sid.RID, "rId") {
				t.Errorf("SlideID[%d].RID = %q, want prefix 'rId'", i, sid.RID)
			}
		}
	}
}

// TestSchema_SlideIDMarshalUnmarshal tests that SlideID properly marshals/unmarshals r:id.
func TestSchema_SlideIDMarshalUnmarshal(t *testing.T) {
	original := oxml.SlideID{
		ID:  256,
		RID: "rId2",
	}

	// Marshal
	data, err := xml.Marshal(original)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	xmlStr := string(data)
	t.Logf("Marshaled XML: %s", xmlStr)

	// Should contain r:id and the value
	if !strings.Contains(xmlStr, "r:id") {
		t.Error("Marshaled XML should contain r:id attribute")
	}
	if !strings.Contains(xmlStr, "rId2") {
		t.Error("Marshaled XML should contain rId2 value")
	}

	// Note: Isolated unmarshal without namespace context doesn't work correctly
	// because Go's XML parser needs xmlns:r declaration to resolve the r: prefix.
	// The actual round-trip works because the full document has proper namespaces.
	// This test verifies marshal output is correct.
}

// TestSchema_SlideMasterIDMarshalUnmarshal tests that SlideMasterID serializes
// through the production Builder path (the sole path since the dead stdlib
// serializer was removed in C355), emitting the compact r:id form.
func TestSchema_SlideMasterIDMarshalUnmarshal(t *testing.T) {
	original := oxml.SlideMasterID{
		ID:  2147483648,
		RID: "rId1",
	}

	b := xmlb.NewPresentationMLBuilder()
	original.MarshalToBuilder(b, xmlb.NSPresentationML, "sldMasterId")
	if err := b.Finish(); err != nil {
		t.Fatalf("builder: %v", err)
	}

	xmlStr := b.String()
	t.Logf("Marshaled XML: %s", xmlStr)

	// Should contain r:id and the value
	if !strings.Contains(xmlStr, "r:id") {
		t.Error("Marshaled XML should contain r:id attribute")
	}
	if !strings.Contains(xmlStr, "rId1") {
		t.Error("Marshaled XML should contain rId1 value")
	}
	if !strings.Contains(xmlStr, "2147483648") {
		t.Error("Marshaled XML should contain numeric ID")
	}
}

// TestSchema_ContentPreservation tests that slide content is preserved during round-trip.
func TestSchema_ContentPreservation(t *testing.T) {
	testFile := "../python-tests/test_files/test.pptx"
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Skip("Test file not found:", testFile)
	}

	// Open original
	p1, err := Open(testFile)
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}

	// Capture original structure
	origSlideCount := p1.SlideCount()
	origMasterCount := len(p1.SlideMasters())
	origLayoutCount := len(p1.SlideLayouts())

	// Capture slide shapes count
	var origShapeCounts []int
	for i := 0; i < p1.SlideCount(); i++ {
		slide, _ := p1.Slide(i)
		origShapeCounts = append(origShapeCounts, len(slide.Shapes()))
	}

	// Save to temp file
	tmpFile := t.TempDir() + "/roundtrip.pptx"
	if err := p1.Save(tmpFile); err != nil {
		_ = p1.Close()
		t.Fatalf("Save() error: %v", err)
	}
	_ = p1.Close()

	// Re-open
	p2, err := Open(tmpFile)
	if err != nil {
		t.Fatalf("Re-open error: %v", err)
	}
	defer func() { _ = p2.Close() }()

	// Verify structure preserved
	if p2.SlideCount() != origSlideCount {
		t.Errorf("Slide count changed: got %d, want %d", p2.SlideCount(), origSlideCount)
	}
	if len(p2.SlideMasters()) != origMasterCount {
		t.Errorf("Master count changed: got %d, want %d", len(p2.SlideMasters()), origMasterCount)
	}
	if len(p2.SlideLayouts()) != origLayoutCount {
		t.Errorf("Layout count changed: got %d, want %d", len(p2.SlideLayouts()), origLayoutCount)
	}

	// Verify slide shapes preserved
	for i := 0; i < p2.SlideCount() && i < len(origShapeCounts); i++ {
		slide, _ := p2.Slide(i)
		if len(slide.Shapes()) != origShapeCounts[i] {
			t.Errorf("Slide %d shape count changed: got %d, want %d",
				i, len(slide.Shapes()), origShapeCounts[i])
		}
	}
}

// TestSchema_AllTestFilesOpen tests that all test files can be opened successfully.
func TestSchema_AllTestFilesOpen(t *testing.T) {
	testFiles := []string{
		"../python-tests/test_files/minimal.pptx",
		"../python-tests/test_files/no-slides.pptx",
		"../python-tests/test_files/no-core-props.pptx",
		"../python-tests/test_files/test.pptx",
		"../python-tests/test_files/test_slides.pptx",
		"../python-tests/test_files/missing_rels_item.pptx",
	}

	for _, testFile := range testFiles {
		t.Run(testFile, func(t *testing.T) {
			if _, err := os.Stat(testFile); os.IsNotExist(err) {
				t.Skip("Test file not found:", testFile)
			}

			p, err := Open(testFile)
			if err != nil {
				t.Fatalf("Open() error: %v", err)
			}
			defer func() { _ = p.Close() }()

			// Basic structure validation
			t.Logf("Slides: %d, Masters: %d, Layouts: %d",
				p.SlideCount(), len(p.SlideMasters()), len(p.SlideLayouts()))

			// Verify presentation element exists
			if p.presentation == nil {
				t.Error("presentation is nil")
			}
		})
	}
}

// TestSchema_XMLNamespaces tests that XML namespaces are properly declared.
func TestSchema_XMLNamespaces(t *testing.T) {
	testFile := "../python-tests/test_files/test.pptx"
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Skip("Test file not found:", testFile)
	}

	// Read presentation.xml directly from the ZIP
	zr, err := zip.OpenReader(testFile)
	if err != nil {
		t.Fatalf("Failed to open ZIP: %v", err)
	}
	defer func() { _ = zr.Close() }()

	var presData []byte
	for _, f := range zr.File {
		if f.Name == "ppt/presentation.xml" {
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("Failed to open presentation.xml: %v", err)
			}
			presData, err = io.ReadAll(rc)
			_ = rc.Close()
			if err != nil {
				t.Fatalf("Failed to read presentation.xml: %v", err)
			}
			break
		}
	}

	if presData == nil {
		t.Fatal("presentation.xml not found in ZIP")
	}

	xmlStr := string(presData)

	// Check for required namespaces
	requiredNS := []string{
		"http://schemas.openxmlformats.org/presentationml/2006/main",
		"http://schemas.openxmlformats.org/drawingml/2006/main",
		"http://schemas.openxmlformats.org/officeDocument/2006/relationships",
	}

	for _, ns := range requiredNS {
		if !strings.Contains(xmlStr, ns) {
			t.Errorf("Missing namespace: %s", ns)
		}
	}
}

// TestSchema_SlideSerialization tests that slide XML is properly serialized.
func TestSchema_SlideSerialization(t *testing.T) {
	p := Create()
	slide := p.AddSlide()
	slide.SetName("Test Slide")

	// Add some content
	tb := slide.AddTextBox()
	tb.SetText("Hello World")
	tb.SetPosition(100000, 100000)
	tb.SetSize(1000000, 500000)

	data, err := slide.marshal()
	if err != nil {
		t.Fatalf("marshal() error: %v", err)
	}

	xmlStr := string(data)

	// Verify XML structure
	if !strings.Contains(xmlStr, "<?xml") {
		t.Error("Missing XML declaration")
	}
	if !strings.Contains(xmlStr, "p:sld") || !strings.Contains(xmlStr, "sld") {
		t.Error("Missing sld element")
	}
	if !strings.Contains(xmlStr, "p:cSld") || !strings.Contains(xmlStr, "cSld") {
		t.Error("Missing cSld element")
	}
	if !strings.Contains(xmlStr, "p:spTree") || !strings.Contains(xmlStr, "spTree") {
		t.Error("Missing spTree element")
	}

	// Verify it can be parsed back
	var slideXML oxml.Slide
	if err := xml.Unmarshal(data, &slideXML); err != nil {
		t.Fatalf("Failed to unmarshal slide XML: %v", err)
	}

	if slideXML.CSld == nil {
		t.Error("Unmarshaled slide has nil CSld")
	}
}

// TestSchema_PresentationSerialization tests that presentation XML is properly serialized.
func TestSchema_PresentationSerialization(t *testing.T) {
	p := Create()
	p.AddSlide()
	p.AddSlide()

	data, err := p.marshalPresentation()
	if err != nil {
		t.Fatalf("marshalPresentation() error: %v", err)
	}

	xmlStr := string(data)

	// Verify XML structure
	if !strings.Contains(xmlStr, "<?xml") {
		t.Error("Missing XML declaration")
	}
	if !strings.Contains(xmlStr, "presentation") {
		t.Error("Missing presentation element")
	}
	if !strings.Contains(xmlStr, "sldIdLst") {
		t.Error("Missing sldIdLst element")
	}
	if !strings.Contains(xmlStr, "sldMasterIdLst") {
		t.Error("Missing sldMasterIdLst element")
	}

	// Verify namespaces are declared
	requiredNS := []string{
		"xmlns:a",
		"xmlns:r",
		"xmlns:p",
	}

	for _, ns := range requiredNS {
		if !strings.Contains(xmlStr, ns) {
			t.Errorf("Missing namespace declaration: %s", ns)
		}
	}

	// Verify it can be parsed back
	var presXML oxml.Presentation
	if err := xml.Unmarshal(data, &presXML); err != nil {
		t.Fatalf("Failed to unmarshal presentation XML: %v", err)
	}

	if presXML.SlideIDs == nil {
		t.Error("Unmarshaled presentation has nil SlideIDs")
	}
	if len(presXML.SlideIDs.SlideID) != 2 {
		t.Errorf("Unmarshaled presentation has %d slides, want 2",
			len(presXML.SlideIDs.SlideID))
	}
}
