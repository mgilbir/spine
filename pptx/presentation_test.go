package pptx

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/mgilbir/spine/opc"
	"github.com/mgilbir/spine/pptx/internal/oxml"
)

func TestCreate(t *testing.T) {
	p := Create()
	if p == nil {
		t.Fatal("Create() returned nil")
	}

	if p.SlideCount() != 0 {
		t.Errorf("New presentation has %d slides, want 0", p.SlideCount())
	}

	if p.presentation == nil {
		t.Error("presentation XML structure is nil")
	}

	// Check default slide size
	if p.presentation.SlideSize == nil {
		t.Error("SlideSize is nil")
	} else {
		// Should be 4:3 standard size
		if p.presentation.SlideSize.Cx != 9144000 {
			t.Errorf("SlideSize.Cx = %d, want 9144000", p.presentation.SlideSize.Cx)
		}
	}
}

func TestCreateWidescreen(t *testing.T) {
	p := CreateWidescreen()
	if p == nil {
		t.Fatal("CreateWidescreen() returned nil")
	}

	// Should be 16:9 widescreen size
	if p.presentation.SlideSize.Cx != 12192000 {
		t.Errorf("SlideSize.Cx = %d, want 12192000", p.presentation.SlideSize.Cx)
	}
}

func TestPresentation_AddSlide(t *testing.T) {
	p := Create()

	slide := p.AddSlide()
	if slide == nil {
		t.Fatal("AddSlide() returned nil")
	}

	if p.SlideCount() != 1 {
		t.Errorf("SlideCount() = %d, want 1", p.SlideCount())
	}

	if slide.Index() != 0 {
		t.Errorf("Slide.Index() = %d, want 0", slide.Index())
	}

	// Add another slide
	slide2 := p.AddSlide()
	if p.SlideCount() != 2 {
		t.Errorf("SlideCount() = %d, want 2", p.SlideCount())
	}
	if slide2.Index() != 1 {
		t.Errorf("Second slide Index() = %d, want 1", slide2.Index())
	}
}

func TestPresentation_Slide(t *testing.T) {
	p := Create()
	p.AddSlide()
	p.AddSlide()
	p.AddSlide()

	slide, err := p.Slide(1)
	if err != nil {
		t.Fatalf("Slide(1) error = %v", err)
	}
	if slide.Index() != 1 {
		t.Errorf("Slide(1).Index() = %d, want 1", slide.Index())
	}

	// Test out of range
	_, err = p.Slide(-1)
	if err != ErrSlideIndex {
		t.Errorf("Slide(-1) error = %v, want ErrSlideIndex", err)
	}

	_, err = p.Slide(10)
	if err != ErrSlideIndex {
		t.Errorf("Slide(10) error = %v, want ErrSlideIndex", err)
	}
}

func TestPresentation_RemoveSlide(t *testing.T) {
	p := Create()
	p.AddSlide()
	p.AddSlide()
	p.AddSlide()

	err := p.RemoveSlide(1)
	if err != nil {
		t.Fatalf("RemoveSlide(1) error = %v", err)
	}

	if p.SlideCount() != 2 {
		t.Errorf("SlideCount() after remove = %d, want 2", p.SlideCount())
	}

	// Check indices were updated
	slide, _ := p.Slide(1)
	if slide.Index() != 1 {
		t.Errorf("After remove, slide at 1 has Index() = %d, want 1", slide.Index())
	}

	// Test out of range
	err = p.RemoveSlide(10)
	if err != ErrSlideIndex {
		t.Errorf("RemoveSlide(10) error = %v, want ErrSlideIndex", err)
	}
}

func TestPresentation_MoveSlide(t *testing.T) {
	p := Create()
	slide0 := p.AddSlide()
	slide0.SetName("Slide0")
	slide1 := p.AddSlide()
	slide1.SetName("Slide1")
	slide2 := p.AddSlide()
	slide2.SetName("Slide2")

	// Move slide 0 to position 2
	err := p.MoveSlide(0, 2)
	if err != nil {
		t.Fatalf("MoveSlide(0, 2) error = %v", err)
	}

	// Check order: Slide1, Slide2, Slide0
	s, _ := p.Slide(0)
	if s.Name() != "Slide1" {
		t.Errorf("After move, slide at 0 is %q, want Slide1", s.Name())
	}
	s, _ = p.Slide(1)
	if s.Name() != "Slide2" {
		t.Errorf("After move, slide at 1 is %q, want Slide2", s.Name())
	}
	s, _ = p.Slide(2)
	if s.Name() != "Slide0" {
		t.Errorf("After move, slide at 2 is %q, want Slide0", s.Name())
	}

	// Test move to same position (should be no-op)
	err = p.MoveSlide(1, 1)
	if err != nil {
		t.Errorf("MoveSlide(1, 1) error = %v", err)
	}

	// Test out of range
	err = p.MoveSlide(0, 10)
	if err != ErrSlideIndex {
		t.Errorf("MoveSlide(0, 10) error = %v, want ErrSlideIndex", err)
	}
}

func TestPresentation_SlideSize(t *testing.T) {
	p := Create()

	width := p.SlideWidth()
	height := p.SlideHeight()

	if width != 9144000 {
		t.Errorf("SlideWidth() = %d, want 9144000", width)
	}
	if height != 6858000 {
		t.Errorf("SlideHeight() = %d, want 6858000", height)
	}

	// Set custom size
	p.SetSlideSize(12000000, 8000000)
	if p.SlideWidth() != 12000000 {
		t.Errorf("After SetSlideSize, SlideWidth() = %d, want 12000000", p.SlideWidth())
	}
	if p.SlideHeight() != 8000000 {
		t.Errorf("After SetSlideSize, SlideHeight() = %d, want 8000000", p.SlideHeight())
	}
}

func TestPresentation_Properties(t *testing.T) {
	p := Create()

	// Properties should have timestamps
	if p.Properties.Created.IsZero() {
		t.Error("Properties.Created is zero")
	}
	if p.Properties.Modified.IsZero() {
		t.Error("Properties.Modified is zero")
	}

	// Set custom properties
	p.Properties.Title = "Test Presentation"
	p.Properties.Creator = "Test Author"

	if p.Properties.Title != "Test Presentation" {
		t.Errorf("Properties.Title = %q, want %q", p.Properties.Title, "Test Presentation")
	}
}

func TestPresentation_Save(t *testing.T) {
	p := Create()
	p.Properties.Title = "Test Presentation"
	p.Properties.Creator = "Test Author"

	slide := p.AddSlide()
	slide.SetName("First Slide")

	// Create temp file
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.pptx")

	err := p.Save(filePath)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Fatal("Saved file does not exist")
	}

	// Verify it's a valid zip
	zipFile, err := zip.OpenReader(filePath)
	if err != nil {
		t.Fatalf("Failed to open saved file as zip: %v", err)
	}
	defer func() { _ = zipFile.Close() }()

	// Check required files exist
	requiredFiles := []string{
		"[Content_Types].xml",
		"_rels/.rels",
		"ppt/presentation.xml",
		"ppt/slides/slide1.xml",
	}

	fileMap := make(map[string]bool)
	for _, f := range zipFile.File {
		fileMap[f.Name] = true
	}

	for _, required := range requiredFiles {
		if !fileMap[required] {
			t.Errorf("Missing required file: %s", required)
		}
	}
}

func TestPresentation_SaveAndOpen_RoundTrip(t *testing.T) {
	// Create presentation
	original := Create()
	original.Properties.Title = "Round Trip Test"
	original.Properties.Creator = "Test Author"
	original.AddSlide().SetName("Slide 1")
	original.AddSlide().SetName("Slide 2")

	// Save to temp file
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "roundtrip.pptx")

	err := original.Save(filePath)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Open the saved file
	opened, err := Open(filePath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer func() { _ = opened.Close() }()

	// Verify properties
	if opened.Properties.Title != "Round Trip Test" {
		t.Errorf("Opened Title = %q, want %q", opened.Properties.Title, "Round Trip Test")
	}
	if opened.Properties.Creator != "Test Author" {
		t.Errorf("Opened Creator = %q, want %q", opened.Properties.Creator, "Test Author")
	}

	// Verify slide count
	if opened.SlideCount() != 2 {
		t.Errorf("Opened SlideCount() = %d, want 2", opened.SlideCount())
	}
}

func TestPresentation_OpenAddSlideSaveReopen(t *testing.T) {
	p, err := Open("testdata/test_slides.pptx")
	if err != nil {
		t.Skipf("Could not open test_slides.pptx: %v", err)
	}
	defer p.Close()

	originalCount := p.SlideCount()
	if originalCount == 0 {
		t.Fatal("expected test_slides.pptx to contain slides")
	}

	added := p.AddSlide()
	added.SetName("Added After Open")

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "add_after_open.pptx")
	if err := p.Save(filePath); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	reopened, err := Open(filePath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer reopened.Close()

	if reopened.SlideCount() != originalCount+1 {
		t.Fatalf("SlideCount() after reopen = %d, want %d", reopened.SlideCount(), originalCount+1)
	}

	slide, err := reopened.Slide(originalCount)
	if err != nil {
		t.Fatalf("Slide(%d) error = %v", originalCount, err)
	}
	if slide.Name() != "Added After Open" {
		t.Errorf("Added slide name = %q, want %q", slide.Name(), "Added After Open")
	}
	if slide.Index() != originalCount {
		t.Errorf("Added slide index = %d, want %d", slide.Index(), originalCount)
	}
}

func TestPresentation_OpenRemoveSlideSaveReopen(t *testing.T) {
	p, err := Open("testdata/test_slides.pptx")
	if err != nil {
		t.Skipf("Could not open test_slides.pptx: %v", err)
	}
	defer p.Close()

	originalCount := p.SlideCount()
	if originalCount < 2 {
		t.Skip("need at least two slides to test removal")
	}

	removedName := p.Slides()[0].Name()
	nextName := p.Slides()[1].Name()

	if err := p.RemoveSlide(0); err != nil {
		t.Fatalf("RemoveSlide(0) error = %v", err)
	}

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "remove_after_open.pptx")
	if err := p.Save(filePath); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	reopened, err := Open(filePath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer reopened.Close()

	if reopened.SlideCount() != originalCount-1 {
		t.Fatalf("SlideCount() after reopen = %d, want %d", reopened.SlideCount(), originalCount-1)
	}

	first, err := reopened.Slide(0)
	if err != nil {
		t.Fatalf("Slide(0) error = %v", err)
	}
	if first.Name() != nextName {
		t.Errorf("First remaining slide = %q, want %q", first.Name(), nextName)
	}
	if first.Name() == removedName {
		t.Errorf("Removed slide %q still present at index 0", removedName)
	}
}

func TestPresentation_OpenRemoveThenAddSlideSaveReopen(t *testing.T) {
	p, err := Open("testdata/test_slides.pptx")
	if err != nil {
		t.Skipf("Could not open test_slides.pptx: %v", err)
	}
	defer p.Close()

	originalCount := p.SlideCount()
	if originalCount < 1 {
		t.Skip("need at least one slide to test remove/add")
	}

	if err := p.RemoveSlide(0); err != nil {
		t.Fatalf("RemoveSlide(0) error = %v", err)
	}

	added := p.AddSlide()
	added.SetName("Added After Remove")

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "remove_then_add_after_open.pptx")
	if err := p.Save(filePath); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	reopened, err := Open(filePath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer reopened.Close()

	if reopened.SlideCount() != originalCount {
		t.Fatalf("SlideCount() after reopen = %d, want %d", reopened.SlideCount(), originalCount)
	}

	last, err := reopened.Slide(reopened.SlideCount() - 1)
	if err != nil {
		t.Fatalf("last Slide() error = %v", err)
	}
	if last.Name() != "Added After Remove" {
		t.Errorf("Last slide name = %q, want %q", last.Name(), "Added After Remove")
	}
}

func TestPresentation_Close(t *testing.T) {
	p := Create()
	err := p.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

func TestOpen_InvalidPath(t *testing.T) {
	_, err := Open("/nonexistent/file.pptx")
	if err == nil {
		t.Error("Open() should return error for nonexistent file")
	}
}

func TestOpen_NotPPTX(t *testing.T) {
	// Create a zip file that's not a PPTX
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "notpptx.zip")

	buf := &bytes.Buffer{}
	w := opc.NewWriter(buf)
	if err := w.WritePart("/test.xml", "application/xml", []byte("<test/>")); err != nil {
		t.Fatalf("WritePart error: %v", err)
	}
	// Add a relationship that's NOT an office document
	w.AddRelationship("http://some/other/type", "test.xml", opc.TargetModeInternal)
	if err := w.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}

	if err := os.WriteFile(filePath, buf.Bytes(), 0644); err != nil {
		t.Fatalf("WriteFile error: %v", err)
	}

	_, err := Open(filePath)
	if err != ErrNotPPTX {
		t.Errorf("Open(notpptx) error = %v, want ErrNotPPTX", err)
	}
}

func TestPresentation_marshalPresentation(t *testing.T) {
	p := Create()
	p.AddSlide()
	p.AddSlide()

	data, err := p.marshalPresentation()
	if err != nil {
		t.Fatalf("marshalPresentation() error = %v", err)
	}

	// Verify it's valid XML
	var pres oxml.Presentation
	if err := xml.Unmarshal(data, &pres); err != nil {
		t.Fatalf("Invalid XML: %v", err)
	}

	// Verify slide IDs
	if pres.SlideIDs == nil {
		t.Fatal("SlideIDs is nil")
	}
	if len(pres.SlideIDs.SlideID) != 2 {
		t.Errorf("SlideIDs count = %d, want 2", len(pres.SlideIDs.SlideID))
	}
}

func TestPresentation_Slides(t *testing.T) {
	p := Create()
	p.AddSlide()
	p.AddSlide()
	p.AddSlide()

	slides := p.Slides()
	if len(slides) != 3 {
		t.Errorf("Slides() returned %d slides, want 3", len(slides))
	}
}

// Integration test: Create a minimal valid PPTX and verify structure
func TestIntegration_MinimalPPTX(t *testing.T) {
	p := Create()
	p.Properties.Title = "Minimal Test"
	p.Properties.Creator = "Integration Test"
	p.Properties.Created = time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)
	p.Properties.Modified = time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC)

	slide := p.AddSlide()
	slide.SetName("Test Slide")

	// Save to buffer
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "minimal.pptx")

	err := p.Save(filePath)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Read and verify ZIP structure
	zipFile, err := zip.OpenReader(filePath)
	if err != nil {
		t.Fatalf("Failed to open ZIP: %v", err)
	}
	defer func() { _ = zipFile.Close() }()

	// Verify [Content_Types].xml
	ctFile := findZipFile(zipFile, "[Content_Types].xml")
	if ctFile == nil {
		t.Fatal("Missing [Content_Types].xml")
	}

	ctData := readZipFile(t, ctFile)
	if !strings.Contains(string(ctData), "application/vnd.openxmlformats-officedocument.presentationml.presentation.main+xml") {
		t.Error("[Content_Types].xml missing presentation content type")
	}

	// Verify _rels/.rels
	relsFile := findZipFile(zipFile, "_rels/.rels")
	if relsFile == nil {
		t.Fatal("Missing _rels/.rels")
	}

	relsData := readZipFile(t, relsFile)
	if !strings.Contains(string(relsData), "officeDocument") {
		t.Error("_rels/.rels missing office document relationship")
	}

	// Verify ppt/presentation.xml
	presFile := findZipFile(zipFile, "ppt/presentation.xml")
	if presFile == nil {
		t.Fatal("Missing ppt/presentation.xml")
	}

	presData := readZipFile(t, presFile)
	if !strings.Contains(string(presData), "sldIdLst") {
		t.Error("presentation.xml missing slide ID list")
	}

	// Verify ppt/slides/slide1.xml
	slideFile := findZipFile(zipFile, "ppt/slides/slide1.xml")
	if slideFile == nil {
		t.Fatal("Missing ppt/slides/slide1.xml")
	}

	slideData := readZipFile(t, slideFile)
	if !strings.Contains(string(slideData), "sld") {
		t.Error("slide1.xml missing sld element")
	}
}

func findZipFile(zr *zip.ReadCloser, name string) *zip.File {
	for _, f := range zr.File {
		if f.Name == name {
			return f
		}
	}
	return nil
}

func readZipFile(t *testing.T, f *zip.File) []byte {
	t.Helper()
	rc, err := f.Open()
	if err != nil {
		t.Fatalf("Failed to open %s: %v", f.Name, err)
	}
	defer func() { _ = rc.Close() }()

	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatalf("Failed to read %s: %v", f.Name, err)
	}
	return data
}

func TestPresentation_DefaultMasterAndLayouts(t *testing.T) {
	p := Create()

	// Default creation should include a slide master
	masters := p.SlideMasters()
	if len(masters) != 1 {
		t.Errorf("SlideMasters() count = %d, want 1", len(masters))
	}

	// Default creation should include slide layouts
	layouts := p.SlideLayouts()
	if len(layouts) < 1 {
		t.Error("SlideLayouts() is empty, expected default layouts")
	}

	// Verify master has layouts
	if len(masters) > 0 {
		masterLayouts := masters[0].Layouts()
		if len(masterLayouts) < 1 {
			t.Error("Master has no layouts")
		}
	}
}

func TestPresentation_SaveWithMastersAndLayouts(t *testing.T) {
	p := Create()
	p.Properties.Title = "Master/Layout Test"
	p.AddSlide()

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "with_masters.pptx")

	err := p.Save(filePath)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify ZIP structure includes masters and layouts
	zipFile, err := zip.OpenReader(filePath)
	if err != nil {
		t.Fatalf("Failed to open ZIP: %v", err)
	}
	defer func() { _ = zipFile.Close() }()

	// Verify slide master exists
	masterFile := findZipFile(zipFile, "ppt/slideMasters/slideMaster1.xml")
	if masterFile == nil {
		t.Error("Missing ppt/slideMasters/slideMaster1.xml")
	}

	// Verify at least one slide layout exists
	layoutFile := findZipFile(zipFile, "ppt/slideLayouts/slideLayout1.xml")
	if layoutFile == nil {
		t.Error("Missing ppt/slideLayouts/slideLayout1.xml")
	}

	// Verify presentation.xml references the master
	presFile := findZipFile(zipFile, "ppt/presentation.xml")
	if presFile != nil {
		presData := readZipFile(t, presFile)
		if !strings.Contains(string(presData), "sldMasterIdLst") {
			t.Error("presentation.xml missing slide master ID list")
		}
	}
}

func TestSlideLayoutType_DisplayName(t *testing.T) {
	tests := []struct {
		layoutType  SlideLayoutType
		wantDisplay string
	}{
		{LayoutTitle, "Title Slide"},
		{LayoutTitleAndContent, "Title and Content"},
		{LayoutBlank, "Blank"},
		{LayoutTwoContent, "Two Content"},
		{LayoutSectionHeader, "Section Header"},
	}

	for _, tt := range tests {
		t.Run(tt.wantDisplay, func(t *testing.T) {
			if got := tt.layoutType.DisplayName(); got != tt.wantDisplay {
				t.Errorf("DisplayName() = %q, want %q", got, tt.wantDisplay)
			}
		})
	}
}

func TestSlideLayoutTypeFromString(t *testing.T) {
	tests := []struct {
		input string
		want  SlideLayoutType
	}{
		{"title", LayoutTitle},
		{"obj", LayoutTitleAndContent},
		{"blank", LayoutBlank},
		{"custom", SlideLayoutType("custom")},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			if got := LayoutTypeFromString(tt.input); got != tt.want {
				t.Errorf("LayoutTypeFromString(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestSlideMaster_Name(t *testing.T) {
	p := Create()
	masters := p.SlideMasters()
	if len(masters) == 0 {
		t.Skip("No masters in default presentation")
	}

	master := masters[0]
	master.SetName("Custom Master")

	if master.Name() != "Custom Master" {
		t.Errorf("Name() = %q, want %q", master.Name(), "Custom Master")
	}
}

func TestSlideMaster_GetLayout(t *testing.T) {
	p := Create()
	masters := p.SlideMasters()
	if len(masters) == 0 {
		t.Skip("No masters in default presentation")
	}

	master := masters[0]

	// Should find the title layout
	titleLayout := master.GetLayout(LayoutTitle)
	if titleLayout == nil {
		t.Error("GetLayout(LayoutTitle) returned nil")
	}

	// Should not find a non-existent layout type
	nonExistent := master.GetLayout(SlideLayoutType("nonexistent"))
	if nonExistent != nil {
		t.Error("GetLayout for nonexistent type should return nil")
	}
}

func TestSlideLayout_Name(t *testing.T) {
	p := Create()
	layouts := p.SlideLayouts()
	if len(layouts) == 0 {
		t.Skip("No layouts in default presentation")
	}

	layout := layouts[0]
	layout.SetName("Custom Layout")

	if layout.Name() != "Custom Layout" {
		t.Errorf("Name() = %q, want %q", layout.Name(), "Custom Layout")
	}
}

func TestSlideLayout_Type(t *testing.T) {
	p := Create()
	layouts := p.SlideLayouts()
	if len(layouts) == 0 {
		t.Skip("No layouts in default presentation")
	}

	// First layout should be Title layout
	layout := layouts[0]
	if layout.Type() != LayoutTitle {
		t.Errorf("Type() = %v, want LayoutTitle", layout.Type())
	}
}

func TestSlideLayout_Master(t *testing.T) {
	p := Create()
	layouts := p.SlideLayouts()
	masters := p.SlideMasters()

	if len(layouts) == 0 || len(masters) == 0 {
		t.Skip("No layouts or masters in default presentation")
	}

	layout := layouts[0]
	if layout.Master() != masters[0] {
		t.Error("Layout.Master() does not match expected master")
	}
}

func TestCreateFromTemplate(t *testing.T) {
	// First create a "template" file
	template := Create()
	template.Properties.Title = "Template"
	template.AddSlide().SetName("Template Slide 1")
	template.AddSlide().SetName("Template Slide 2")

	tmpDir := t.TempDir()
	templatePath := filepath.Join(tmpDir, "template.pptx")
	if err := template.Save(templatePath); err != nil {
		t.Fatalf("Failed to save template: %v", err)
	}
	_ = template.Close()

	// Now create from template
	p, err := CreateFromTemplate(templatePath)
	if err != nil {
		t.Fatalf("CreateFromTemplate() error = %v", err)
	}
	defer func() { _ = p.Close() }()

	// Template path should be recorded
	if p.TemplatePath() != templatePath {
		t.Errorf("TemplatePath() = %q, want %q", p.TemplatePath(), templatePath)
	}

	// Should have no slides (cleared)
	if p.SlideCount() != 0 {
		t.Errorf("SlideCount() = %d, want 0 (slides should be cleared)", p.SlideCount())
	}

	// Should have masters and layouts from template
	if len(p.SlideMasters()) == 0 {
		t.Error("Should have slide masters from template")
	}
	if len(p.SlideLayouts()) == 0 {
		t.Error("Should have slide layouts from template")
	}

	// Title should be cleared
	if p.Properties.Title != "" {
		t.Errorf("Properties.Title = %q, want empty", p.Properties.Title)
	}
}

func TestCreateFromTemplateWithSlides(t *testing.T) {
	// First create a "template" file
	template := Create()
	template.Properties.Title = "Template"
	template.AddSlide().SetName("Template Slide 1")
	template.AddSlide().SetName("Template Slide 2")

	tmpDir := t.TempDir()
	templatePath := filepath.Join(tmpDir, "template.pptx")
	if err := template.Save(templatePath); err != nil {
		t.Fatalf("Failed to save template: %v", err)
	}
	_ = template.Close()

	// Now create from template with slides
	p, err := CreateFromTemplateWithSlides(templatePath)
	if err != nil {
		t.Fatalf("CreateFromTemplateWithSlides() error = %v", err)
	}
	defer func() { _ = p.Close() }()

	// Should have slides from template
	if p.SlideCount() != 2 {
		t.Errorf("SlideCount() = %d, want 2", p.SlideCount())
	}
}

func TestPresentation_GetLayoutByType(t *testing.T) {
	p := Create()

	layout := p.GetLayoutByType(LayoutTitle)
	if layout == nil {
		t.Error("GetLayoutByType(LayoutTitle) returned nil")
	}

	layout = p.GetLayoutByType(LayoutBlank)
	if layout == nil {
		t.Error("GetLayoutByType(LayoutBlank) returned nil")
	}

	layout = p.GetLayoutByType(SlideLayoutType("nonexistent"))
	if layout != nil {
		t.Error("GetLayoutByType for nonexistent type should return nil")
	}
}

func TestPresentation_GetLayoutByName(t *testing.T) {
	p := Create()

	layout := p.GetLayoutByName("Title Slide")
	if layout == nil {
		t.Error("GetLayoutByName('Title Slide') returned nil")
	}

	layout = p.GetLayoutByName("Nonexistent Layout")
	if layout != nil {
		t.Error("GetLayoutByName for nonexistent name should return nil")
	}
}

func TestPresentation_AddSlideFromLayout(t *testing.T) {
	p := Create()

	layout := p.GetLayoutByType(LayoutTitleAndContent)
	if layout == nil {
		t.Skip("No TitleAndContent layout available")
	}

	slide := p.AddSlideFromLayout(layout)
	if slide == nil {
		t.Fatal("AddSlideFromLayout() returned nil")
	}

	if slide.Layout() != layout {
		t.Error("Slide layout does not match specified layout")
	}
}

func TestPresentation_SaveAsRoundTrip(t *testing.T) {
	// Create original presentation
	original := Create()
	original.Properties.Title = "Original"
	original.AddSlide().SetName("Slide 1")

	tmpDir := t.TempDir()
	originalPath := filepath.Join(tmpDir, "original.pptx")
	if err := original.Save(originalPath); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Open and save as new file
	opened, err := Open(originalPath)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	opened.Properties.Title = "Modified"
	opened.AddSlide().SetName("Slide 2")

	newPath := filepath.Join(tmpDir, "modified.pptx")
	if err := opened.SaveAs(newPath); err != nil {
		t.Fatalf("SaveAs() error = %v", err)
	}
	_ = opened.Close()

	// Verify the new file
	reopened, err := Open(newPath)
	if err != nil {
		t.Fatalf("Open(new) error = %v", err)
	}
	defer func() { _ = reopened.Close() }()

	if reopened.Properties.Title != "Modified" {
		t.Errorf("Title = %q, want 'Modified'", reopened.Properties.Title)
	}

	if reopened.SlideCount() != 2 {
		t.Errorf("SlideCount() = %d, want 2", reopened.SlideCount())
	}
}

func TestPartNameToRelTarget(t *testing.T) {
	tests := []struct {
		partName string
		baseDir  string
		want     string
	}{
		{"/ppt/slideMasters/slideMaster1.xml", "/ppt/", "slideMasters/slideMaster1.xml"},
		{"/ppt/slideLayouts/slideLayout1.xml", "/ppt/slideMasters/", "../slideLayouts/slideLayout1.xml"},
		{"/ppt/slideMasters/slideMaster1.xml", "/ppt/slideLayouts/", "../slideMasters/slideMaster1.xml"},
		{"/ppt/theme/theme1.xml", "/ppt/slideMasters/", "../theme/theme1.xml"},
	}

	for _, tt := range tests {
		t.Run(tt.partName, func(t *testing.T) {
			got := partNameToRelTarget(tt.partName, tt.baseDir)
			if got != tt.want {
				t.Errorf("partNameToRelTarget(%q, %q) = %q, want %q", tt.partName, tt.baseDir, got, tt.want)
			}
		})
	}
}
