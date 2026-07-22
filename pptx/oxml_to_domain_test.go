package pptx

import (
	"bytes"
	"encoding/xml"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mgilbir/spine/common/dml"
	"github.com/mgilbir/spine/pptx/internal/oxml"
)

func testPresentationWithoutLayouts() *Presentation {
	opts := DefaultCreateOptions()
	opts.IncludeDefaultLayouts = false
	return CreateWithOptions(opts)
}

func mustParseShapeTree(t *testing.T, src string) *oxml.ShapeTree {
	t.Helper()
	var st oxml.ShapeTree
	if err := xml.Unmarshal([]byte(src), &st); err != nil {
		t.Fatalf("xml.Unmarshal ShapeTree failed: %v", err)
	}
	return &st
}

// mustClose is a test helper that closes p and fails the test if Close
// reports an error. Intended for use with defer.
func mustClose(t *testing.T, p *Presentation) {
	t.Helper()
	if err := p.Close(); err != nil {
		t.Errorf("Close failed: %v", err)
	}
}

// mustSlide is a test helper that calls Slide and fails on error.
func mustSlide(t *testing.T, p *Presentation, index int) *Slide {
	t.Helper()
	s, err := p.Slide(index)
	if err != nil {
		t.Fatalf("Slide(%d) failed: %v", index, err)
	}
	return s
}

// --- Materialization Tests ---

func TestMaterializeShapes_Placeholder(t *testing.T) {
	// Create a presentation with a slide that has placeholders, save it, reopen it.
	p := Create()
	slide := p.AddSlide()

	// Add a title placeholder
	title := NewPlaceholderShape(PlaceholderTitle)
	title.SetName("Title 1")
	title.SetText("Hello World")
	title.SetPosition(dml.Inches(0.5), dml.Inches(0.3))
	title.SetSize(dml.Inches(9), dml.Inches(1.2))
	_ = slide.AddShape(title)

	// Add a body placeholder
	body := NewPlaceholderShape(PlaceholderBody)
	body.SetName("Content Placeholder")
	body.SetText("Body text here")
	body.SetPosition(dml.Inches(0.5), dml.Inches(1.6))
	body.SetSize(dml.Inches(9), dml.Inches(5.1))
	_ = slide.AddShape(body)

	// Save to temp file
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test_materialize.pptx")
	if err := p.Save(path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Reopen
	p2, err := Open(path)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer mustClose(t, p2)

	if p2.SlideCount() != 1 {
		t.Fatalf("SlideCount = %d, want 1", p2.SlideCount())
	}

	slide2 := mustSlide(t, p2, 0)

	// Verify shapes were materialized
	shapes := slide2.Shapes()
	if len(shapes) < 2 {
		t.Fatalf("len(Shapes) = %d, want >= 2", len(shapes))
	}

	// Verify placeholders are accessible
	placeholders := slide2.Placeholders()
	if len(placeholders) < 2 {
		t.Fatalf("len(Placeholders) = %d, want >= 2", len(placeholders))
	}

	// Verify title placeholder
	titlePh := slide2.TitlePlaceholder()
	if titlePh == nil {
		t.Fatal("TitlePlaceholder() returned nil")
	}
	if titlePh.Text() != "Hello World" {
		t.Errorf("Title text = %q, want %q", titlePh.Text(), "Hello World")
	}
	if titlePh.PlaceholderType() != PlaceholderTitle {
		t.Errorf("PlaceholderType = %q, want %q", titlePh.PlaceholderType(), PlaceholderTitle)
	}

	// Verify body placeholder
	bodyPh := slide2.BodyPlaceholder()
	if bodyPh == nil {
		t.Fatal("BodyPlaceholder() returned nil")
	}
	if bodyPh.Text() != "Body text here" {
		t.Errorf("Body text = %q, want %q", bodyPh.Text(), "Body text here")
	}
}

func TestMaterializeShapes_TextBox(t *testing.T) {
	p := Create()
	slide := p.AddSlide()

	tb := slide.AddTextBox()
	tb.SetName("TextBox 1")
	tb.SetText("Test text box")
	tb.SetPosition(dml.Inches(1), dml.Inches(1))
	tb.SetSize(dml.Inches(3), dml.Inches(1))

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test_textbox.pptx")
	if err := p.Save(path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	p2, err := Open(path)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer mustClose(t, p2)

	slide2 := mustSlide(t, p2, 0)
	shapes := slide2.Shapes()

	found := false
	for _, s := range shapes {
		if tb, ok := s.(*TextBox); ok {
			if tb.Text() == "Test text box" {
				found = true
				break
			}
		}
	}
	if !found {
		t.Error("TextBox with text 'Test text box' not found after materialization")
	}
}

func TestMaterializeShapes_AutoShape(t *testing.T) {
	p := Create()
	slide := p.AddSlide()

	as := NewAutoShape(PresetEllipse)
	as.SetName("Ellipse 1")
	as.SetPosition(dml.Inches(2), dml.Inches(2))
	as.SetSize(dml.Inches(2), dml.Inches(2))
	_ = slide.AddShape(as)

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test_autoshape.pptx")
	if err := p.Save(path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	p2, err := Open(path)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer mustClose(t, p2)

	slide2 := mustSlide(t, p2, 0)
	shapes := slide2.Shapes()

	found := false
	for _, s := range shapes {
		if as, ok := s.(*AutoShape); ok {
			if as.PresetGeometry() == PresetEllipse {
				found = true
				break
			}
		}
	}
	if !found {
		t.Error("AutoShape with ellipse geometry not found after materialization")
	}
}

func TestMaterializeShapes_Table(t *testing.T) {
	p := Create()
	slide := p.AddSlide()

	tbl := slide.AddTable(2, 3)
	tbl.SetName("Table 1")
	tbl.Cell(0, 0).SetText("Cell A1")
	tbl.Cell(0, 1).SetText("Cell B1")
	tbl.Cell(1, 0).SetText("Cell A2")

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test_table.pptx")
	if err := p.Save(path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	p2, err := Open(path)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer mustClose(t, p2)

	slide2 := mustSlide(t, p2, 0)
	shapes := slide2.Shapes()

	found := false
	for _, s := range shapes {
		if tbl, ok := s.(*Table); ok {
			if tbl.RowCount() == 2 && tbl.ColCount() == 3 {
				if tbl.Cell(0, 0).Text() == "Cell A1" {
					found = true
					break
				}
			}
		}
	}
	if !found {
		t.Error("Table with expected content not found after materialization")
	}
}

// --- Materialization from existing test files ---

func TestMaterializeShapes_ExistingFile(t *testing.T) {
	// Test with the test_slides.pptx fixture which should have multiple slides
	p, err := Open("testdata/test_slides.pptx")
	if err != nil {
		t.Skipf("Could not open test_slides.pptx: %v", err)
	}
	defer mustClose(t, p)

	for i := 0; i < p.SlideCount(); i++ {
		slide := mustSlide(t, p, i)
		shapes := slide.Shapes()
		// Verify shapes were materialized (at least we got something)
		t.Logf("Slide %d: %d shapes materialized", i, len(shapes))
		for _, s := range shapes {
			t.Logf("  - %s: %q", s.ShapeType(), s.Name())
		}
	}
}

func TestMaterializeShapes_PreservesRoundTrip(t *testing.T) {
	// Open a file, don't modify anything, save it, and verify it's still valid
	p, err := Open("testdata/test.pptx")
	if err != nil {
		t.Skipf("Could not open test.pptx: %v", err)
	}
	defer mustClose(t, p)

	// Verify shapes are materialized
	for i := 0; i < p.SlideCount(); i++ {
		slide := mustSlide(t, p, i)
		_ = slide.Shapes() // Access shapes to ensure materialization happened
	}

	// Save to buffer
	var buf bytes.Buffer
	if err := p.SaveTo(&buf); err != nil {
		t.Fatalf("SaveTo failed: %v", err)
	}

	// Verify the saved file can be reopened
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "roundtrip.pptx")
	if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	p2, err := Open(path)
	if err != nil {
		t.Fatalf("Could not reopen saved file: %v", err)
	}
	defer mustClose(t, p2)

	if p2.SlideCount() != p.SlideCount() {
		t.Errorf("Slide count mismatch: got %d, want %d", p2.SlideCount(), p.SlideCount())
	}
}

// --- Template Replacement Tests ---

func TestReplaceText_SingleRun(t *testing.T) {
	p := Create()
	slide := p.AddSlide()

	ph := NewPlaceholderShape(PlaceholderTitle)
	ph.SetText("Hello {{name}}")
	_ = slide.AddShape(ph)

	// Save and reopen so that XML is populated
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "template.pptx")
	if err := p.Save(path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	p2, err := Open(path)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer mustClose(t, p2)

	// Perform replacement
	p2.ReplaceText(map[string]string{
		"{{name}}": "John",
	})

	// Verify
	slide2 := mustSlide(t, p2, 0)
	titlePh := slide2.TitlePlaceholder()
	if titlePh == nil {
		t.Fatal("TitlePlaceholder() returned nil")
	}
	if titlePh.Text() != "Hello John" {
		t.Errorf("After replacement, text = %q, want %q", titlePh.Text(), "Hello John")
	}
}

func TestReplaceText_MultipleKeys(t *testing.T) {
	p := Create()
	slide := p.AddSlide()

	ph := NewPlaceholderShape(PlaceholderTitle)
	ph.SetText("{{greeting}} {{name}}, welcome to {{place}}")
	_ = slide.AddShape(ph)

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "multi_template.pptx")
	if err := p.Save(path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	p2, err := Open(path)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer mustClose(t, p2)

	p2.ReplaceText(map[string]string{
		"{{greeting}}": "Hi",
		"{{name}}":     "Jane",
		"{{place}}":    "Spine",
	})

	slide2 := mustSlide(t, p2, 0)
	titlePh := slide2.TitlePlaceholder()
	if titlePh == nil {
		t.Fatal("TitlePlaceholder() returned nil")
	}
	expected := "Hi Jane, welcome to Spine"
	if titlePh.Text() != expected {
		t.Errorf("After replacement, text = %q, want %q", titlePh.Text(), expected)
	}
}

func TestReplaceText_NoMatch(t *testing.T) {
	p := Create()
	slide := p.AddSlide()

	ph := NewPlaceholderShape(PlaceholderTitle)
	ph.SetText("No templates here")
	_ = slide.AddShape(ph)

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "no_match.pptx")
	if err := p.Save(path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	p2, err := Open(path)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer mustClose(t, p2)

	// Should not crash or modify text
	p2.ReplaceText(map[string]string{
		"{{missing}}": "value",
	})

	slide2 := mustSlide(t, p2, 0)
	titlePh := slide2.TitlePlaceholder()
	if titlePh == nil {
		t.Fatal("TitlePlaceholder() returned nil")
	}
	if titlePh.Text() != "No templates here" {
		t.Errorf("Text was modified unexpectedly: %q", titlePh.Text())
	}
}

func TestReplaceText_TextBox(t *testing.T) {
	p := Create()
	slide := p.AddSlide()

	tb := slide.AddTextBox()
	tb.SetText("Dear {{name}}")

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "textbox_template.pptx")
	if err := p.Save(path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	p2, err := Open(path)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer mustClose(t, p2)

	slide2 := mustSlide(t, p2, 0)
	slide2.ReplaceText(map[string]string{
		"{{name}}": "Bob",
	})

	shapes := slide2.Shapes()
	found := false
	for _, s := range shapes {
		if tb, ok := s.(*TextBox); ok {
			if tb.Text() == "Dear Bob" {
				found = true
				break
			}
		}
	}
	if !found {
		t.Error("TextBox replacement not found")
	}
}

func TestReplaceText_SaveAndReopen(t *testing.T) {
	// Test that replacements survive save/reopen
	p := Create()
	slide := p.AddSlide()

	ph := NewPlaceholderShape(PlaceholderTitle)
	ph.SetText("Company: {{company}}")
	_ = slide.AddShape(ph)

	tmpDir := t.TempDir()
	path1 := filepath.Join(tmpDir, "before.pptx")
	if err := p.Save(path1); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	p2, err := Open(path1)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	p2.ReplaceText(map[string]string{
		"{{company}}": "Acme Corp",
	})

	path2 := filepath.Join(tmpDir, "after.pptx")
	if err := p2.Save(path2); err != nil {
		t.Fatalf("Save after replacement failed: %v", err)
	}
	if err := p2.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Reopen the replaced file
	p3, err := Open(path2)
	if err != nil {
		t.Fatalf("Open replaced file failed: %v", err)
	}
	defer mustClose(t, p3)

	slide3 := mustSlide(t, p3, 0)
	titlePh := slide3.TitlePlaceholder()
	if titlePh == nil {
		t.Fatal("TitlePlaceholder() returned nil after reopen")
	}
	if titlePh.Text() != "Company: Acme Corp" {
		t.Errorf("After save/reopen, text = %q, want %q", titlePh.Text(), "Company: Acme Corp")
	}
}

func TestReplaceText_Table(t *testing.T) {
	p := Create()
	slide := p.AddSlide()

	tbl := slide.AddTable(2, 2)
	tbl.Cell(0, 0).SetText("Name: {{name}}")
	tbl.Cell(0, 1).SetText("Age: {{age}}")

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "table_template.pptx")
	if err := p.Save(path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	p2, err := Open(path)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer mustClose(t, p2)

	slide2 := mustSlide(t, p2, 0)
	slide2.ReplaceText(map[string]string{
		"{{name}}": "Alice",
		"{{age}}":  "30",
	})

	// Save and reopen to verify
	path2 := filepath.Join(tmpDir, "table_replaced.pptx")
	if err := p2.Save(path2); err != nil {
		t.Fatalf("Save after replacement failed: %v", err)
	}

	p3, err := Open(path2)
	if err != nil {
		t.Fatalf("Open replaced file failed: %v", err)
	}
	defer mustClose(t, p3)

	// Find the table
	slide3 := mustSlide(t, p3, 0)
	for _, s := range slide3.Shapes() {
		if tbl, ok := s.(*Table); ok {
			cell00 := tbl.Cell(0, 0).Text()
			cell01 := tbl.Cell(0, 1).Text()
			if cell00 != "Name: Alice" {
				t.Errorf("Cell(0,0) = %q, want %q", cell00, "Name: Alice")
			}
			if cell01 != "Age: 30" {
				t.Errorf("Cell(0,1) = %q, want %q", cell01, "Age: 30")
			}
			return
		}
	}
	t.Error("Table not found after replacement")
}

// --- Image Replacement Tests ---

func TestSetImage_NotPicturePlaceholder(t *testing.T) {
	ph := NewPlaceholderShape(PlaceholderTitle)
	err := ph.SetImage("nonexistent.png")
	if err != ErrNotPicturePlaceholder {
		t.Errorf("Expected ErrNotPicturePlaceholder, got %v", err)
	}
}

func TestSetImageData_NotPicturePlaceholder(t *testing.T) {
	ph := NewPlaceholderShape(PlaceholderBody)
	err := ph.SetImageData([]byte{0x89, 0x50}, "image/png")
	if err != ErrNotPicturePlaceholder {
		t.Errorf("Expected ErrNotPicturePlaceholder, got %v", err)
	}
}

func TestSetImageData_PicturePlaceholder(t *testing.T) {
	ph := NewPlaceholderShape(PlaceholderPicture)
	err := ph.SetImageData([]byte{0x89, 0x50, 0x4E, 0x47}, "image/png")
	if err != nil {
		t.Fatalf("SetImageData failed: %v", err)
	}

	if !ph.hasPendingImage() {
		t.Error("hasPendingImage() = false after SetImageData")
	}
}

func TestImageReplacement_EndToEnd(t *testing.T) {
	// Create a presentation with a picture placeholder
	p := testPresentationWithoutLayouts()
	slide := p.AddSlide()

	// Add a picture placeholder
	picPh := NewPlaceholderShape(PlaceholderPicture)
	picPh.SetName("Picture Placeholder")
	picPh.SetIndex(10)
	picPh.SetPosition(dml.Inches(1), dml.Inches(1))
	picPh.SetSize(dml.Inches(5), dml.Inches(4))
	_ = slide.AddShape(picPh)

	// Save
	tmpDir := t.TempDir()
	path1 := filepath.Join(tmpDir, "with_pic_ph.pptx")
	if err := p.Save(path1); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Reopen
	p2, err := Open(path1)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	// Find the picture placeholder
	slide2 := mustSlide(t, p2, 0)
	picPh2 := slide2.GetPlaceholder(PlaceholderPicture)
	if picPh2 == nil {
		t.Fatal("Picture placeholder not found after materialization")
	}

	// Create a minimal valid PNG image for testing
	pngData := createMinimalPNG()

	// Set image data
	if err := picPh2.SetImageData(pngData, "image/png"); err != nil {
		t.Fatalf("SetImageData failed: %v", err)
	}

	// Save with the image
	path2 := filepath.Join(tmpDir, "with_image.pptx")
	if err := p2.Save(path2); err != nil {
		t.Fatalf("Save with image failed: %v", err)
	}
	if err := p2.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Verify the saved file can be opened and contains the image
	p3, err := Open(path2)
	if err != nil {
		t.Fatalf("Open file with image failed: %v", err)
	}
	defer mustClose(t, p3)

	// The picture placeholder should now be a Picture shape
	slide3 := mustSlide(t, p3, 0)
	shapes := slide3.Shapes()

	foundPic := false
	for _, s := range shapes {
		if _, ok := s.(*Picture); ok {
			foundPic = true
			break
		}
	}
	if !foundPic {
		t.Error("No Picture shape found after image replacement")
		for _, s := range shapes {
			t.Logf("  - %s: %q", s.ShapeType(), s.Name())
		}
	}

	// Verify that a media part was created
	foundMedia := false
	for name := range p3.otherParts {
		if strings.HasPrefix(name, "/ppt/media/") {
			foundMedia = true
			break
		}
	}
	if !foundMedia {
		t.Error("No media part found in the saved file")
	}
}

func TestImageReplacement_NewSlidePlaceholder(t *testing.T) {
	p := testPresentationWithoutLayouts()
	slide := p.AddSlide()

	placeholder := NewPlaceholderShape(PlaceholderPicture)
	placeholder.SetName("Direct Placeholder")
	placeholder.SetIndex(1)
	_ = slide.AddShape(placeholder)

	if err := placeholder.SetImageData(createMinimalPNG(), "image/png"); err != nil {
		t.Fatalf("SetImageData failed: %v", err)
	}

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "new_slide_placeholder_image.pptx")
	if err := p.Save(path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	p2, err := Open(path)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer mustClose(t, p2)

	loaded := mustSlide(t, p2, 0)
	shape := loaded.ShapeByName("Direct Placeholder")
	if shape == nil {
		t.Fatal("ShapeByName returned nil after save/reopen")
	}
	if _, ok := shape.(*Picture); !ok {
		t.Fatalf("expected placeholder to become *Picture, got %T", shape)
	}
}

func TestPictureImageReplacement_EndToEnd(t *testing.T) {
	// Use the test fixture that contains a Picture shape
	p, err := Open("testdata/test_slides.pptx")
	if err != nil {
		t.Skipf("Could not open test_slides.pptx: %v", err)
	}

	slide := mustSlide(t, p, 0)

	// Find a Picture shape
	var pic *Picture
	for _, s := range slide.Shapes() {
		if p, ok := s.(*Picture); ok {
			pic = p
			break
		}
	}
	if pic == nil {
		t.Skip("No Picture shape found in test_slides.pptx slide 0")
	}

	originalRelID := pic.relID
	t.Logf("Found picture %q with relID=%q", pic.Name(), originalRelID)

	// Replace the image with a minimal PNG
	pngData := createMinimalPNG()
	pic.SetImageData(pngData, "image/png")

	if !pic.hasPendingImage() {
		t.Fatal("hasPendingImage() = false after SetImageData")
	}

	// Save
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "pic_replaced.pptx")
	if err := p.Save(path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}
	if err := p.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	// Reopen and verify
	p2, err := Open(path)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer mustClose(t, p2)

	slide2 := mustSlide(t, p2, 0)

	// The picture should still exist
	var pic2 *Picture
	for _, s := range slide2.Shapes() {
		if p, ok := s.(*Picture); ok {
			pic2 = p
			break
		}
	}
	if pic2 == nil {
		t.Fatal("Picture shape not found after image replacement")
	}

	// The relID should have changed (new relationship was created)
	if pic2.relID == originalRelID {
		t.Errorf("relID did not change after image replacement: still %q", pic2.relID)
	}

	// A media part should exist
	foundMedia := false
	for name := range p2.otherParts {
		if strings.HasPrefix(name, "/ppt/media/") {
			foundMedia = true
			break
		}
	}
	if !foundMedia {
		t.Error("No media part found after picture image replacement")
	}
}

func TestPictureSetImageData_NewPicture(t *testing.T) {
	// Verify that hasPendingImage returns false for a newly created Picture
	// (no slide back-reference), preventing false positives in processPendingImages.
	pic := NewPicture()
	pic.SetImageData([]byte{0x89, 0x50, 0x4E, 0x47}, "image/png")

	if pic.hasPendingImage() {
		t.Error("hasPendingImage() = true for new Picture without slide back-reference")
	}
}

func TestPictureImageReplacement_NewSlidePicture(t *testing.T) {
	p := testPresentationWithoutLayouts()
	slide := p.AddSlide()

	pic := NewPicture()
	pic.SetName("Generated Picture")
	pic.SetPosition(dml.Inches(1), dml.Inches(1))
	pic.SetSize(dml.Inches(2), dml.Inches(2))
	pic.SetImageData(createMinimalPNG(), "image/png")
	_ = slide.AddShape(pic)

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "new_slide_picture_image.pptx")
	if err := p.Save(path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	p2, err := Open(path)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer mustClose(t, p2)

	loaded := mustSlide(t, p2, 0)
	shape := loaded.ShapeByName("Generated Picture")
	if shape == nil {
		t.Fatal("ShapeByName returned nil after save/reopen")
	}
	pic2, ok := shape.(*Picture)
	if !ok {
		t.Fatalf("expected *Picture, got %T", shape)
	}
	if pic2.relID == "" {
		t.Fatal("expected saved picture to have a relationship ID")
	}
	if len(p2.relationships[loaded.partName]) == 0 {
		t.Fatal("expected slide relationships to include image rel")
	}
}

func TestPictureSVGReplacement_NewSlidePicture(t *testing.T) {
	p := testPresentationWithoutLayouts()
	slide := p.AddSlide()

	pic := NewPicture()
	pic.SetName("SVG Picture")
	pic.SetPosition(dml.Inches(1), dml.Inches(1))
	pic.SetSize(dml.Inches(2), dml.Inches(2))
	pic.SetSVGData(createMinimalSVG())
	_ = slide.AddShape(pic)

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "new_slide_picture_svg.pptx")
	if err := p.Save(path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	p2, err := Open(path)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer mustClose(t, p2)

	loaded := mustSlide(t, p2, 0)
	shape := loaded.ShapeByName("SVG Picture")
	if shape == nil {
		t.Fatal("ShapeByName returned nil after save/reopen")
	}
	pic2, ok := shape.(*Picture)
	if !ok {
		t.Fatalf("expected *Picture, got %T", shape)
	}
	if pic2.relID == "" {
		t.Fatal("expected saved picture to have a raster relationship ID")
	}
	if pic2.svgRelID == "" {
		t.Fatal("expected saved picture to have an SVG relationship ID")
	}
	if len(p2.relationships[loaded.partName]) < 2 {
		t.Fatalf("expected at least 2 slide relationships, got %d", len(p2.relationships[loaded.partName]))
	}

	var foundXMLPic *oxml.Picture
	for _, candidate := range loaded.sx().CSld.SpTree.Pic {
		if candidate.NvPicPr != nil && candidate.NvPicPr.CNvPr != nil && candidate.NvPicPr.CNvPr.Name == "SVG Picture" {
			foundXMLPic = candidate
			break
		}
	}
	if foundXMLPic == nil {
		t.Fatal("picture XML not found after save/reopen")
	}
	if foundXMLPic.BlipFill == nil || foundXMLPic.BlipFill.Blip == nil {
		t.Fatal("picture blip missing after save/reopen")
	}
	if foundXMLPic.BlipFill.Blip.ExtLst == nil {
		t.Fatal("expected SVG extension list on picture blip")
	}
	foundSvgExt := false
	for _, ext := range foundXMLPic.BlipFill.Blip.ExtLst.Ext {
		if ext != nil && ext.SvgBlip != nil {
			foundSvgExt = true
			if ext.SvgBlip.Embed == "" {
				t.Fatal("svgBlip embed was empty")
			}
		}
	}
	if !foundSvgExt {
		t.Fatal("expected svgBlip extension on picture blip")
	}
}

func TestImageReplacement_SVGPlaceholderEndToEnd(t *testing.T) {
	p := testPresentationWithoutLayouts()
	slide := p.AddSlide()

	picPh := NewPlaceholderShape(PlaceholderPicture)
	picPh.SetName("SVG Placeholder")
	picPh.SetIndex(10)
	picPh.SetPosition(dml.Inches(1), dml.Inches(1))
	picPh.SetSize(dml.Inches(5), dml.Inches(4))
	_ = slide.AddShape(picPh)

	tmpDir := t.TempDir()
	path1 := filepath.Join(tmpDir, "with_svg_pic_ph.pptx")
	if err := p.Save(path1); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	p2, err := Open(path1)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}

	slide2 := mustSlide(t, p2, 0)
	picPh2 := slide2.GetPlaceholder(PlaceholderPicture)
	if picPh2 == nil {
		t.Fatal("Picture placeholder not found after materialization")
	}
	if err := picPh2.SetSVGData(createMinimalSVG()); err != nil {
		t.Fatalf("SetSVGData failed: %v", err)
	}

	path2 := filepath.Join(tmpDir, "with_svg_image.pptx")
	if err := p2.Save(path2); err != nil {
		t.Fatalf("Save with SVG image failed: %v", err)
	}
	if err := p2.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	p3, err := Open(path2)
	if err != nil {
		t.Fatalf("Open file with SVG image failed: %v", err)
	}
	defer mustClose(t, p3)

	slide3 := mustSlide(t, p3, 0)
	shape := slide3.ShapeByName("SVG Placeholder")
	if shape == nil {
		t.Fatal("ShapeByName returned nil after save/reopen")
	}
	pic3, ok := shape.(*Picture)
	if !ok {
		t.Fatalf("expected placeholder to become *Picture, got %T", shape)
	}
	if pic3.relID == "" || pic3.svgRelID == "" {
		t.Fatal("expected picture to include both raster and SVG relationship IDs")
	}
	if len(p3.relationships[slide3.partName]) < 2 {
		t.Fatalf("expected at least 2 slide relationships, got %d", len(p3.relationships[slide3.partName]))
	}

	var foundXMLPic *oxml.Picture
	for _, candidate := range slide3.sx().CSld.SpTree.Pic {
		if candidate.NvPicPr != nil && candidate.NvPicPr.CNvPr != nil && candidate.NvPicPr.CNvPr.Name == "SVG Placeholder" {
			foundXMLPic = candidate
			break
		}
	}
	if foundXMLPic == nil {
		t.Fatal("picture XML not found after placeholder replacement")
	}
	if foundXMLPic.BlipFill == nil || foundXMLPic.BlipFill.Blip == nil || foundXMLPic.BlipFill.Blip.ExtLst == nil {
		t.Fatal("expected svgBlip extension on converted placeholder picture")
	}
	foundSvgExt := false
	for _, ext := range foundXMLPic.BlipFill.Blip.ExtLst.Ext {
		if ext != nil && ext.SvgBlip != nil {
			foundSvgExt = true
		}
	}
	if !foundSvgExt {
		t.Fatal("expected svgBlip extension on converted placeholder picture")
	}
}

func TestMaterializeShapes_ExistingSVGPicture(t *testing.T) {
	p, err := Open("testdata/svg_test.pptx")
	if err != nil {
		t.Skipf("Could not open svg_test.pptx: %v", err)
	}
	defer mustClose(t, p)

	foundSVGPicture := false
	for i := 0; i < p.SlideCount(); i++ {
		slide := mustSlide(t, p, i)
		for _, shape := range slide.Shapes() {
			if pic, ok := shape.(*Picture); ok && pic.svgRelID != "" {
				foundSVGPicture = true
				break
			}
		}
		if foundSVGPicture {
			break
		}
	}

	if !foundSVGPicture {
		t.Fatal("expected to materialize at least one picture with svgRelID from svg_test.pptx")
	}
}

func TestReplaceText_CrossRunPreservesRunOrdering(t *testing.T) {
	p := testPresentationWithoutLayouts()
	slide := p.AddSlide()

	paragraph := &dml.P{
		R: []*dml.R{
			{T: "{{na"},
			{T: "me}}"},
		},
	}
	paragraph.ResetRunOrder()
	spTree := mustParseShapeTree(t, `
		<p:spTree xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		  <p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>
		  <p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/><a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm></p:grpSpPr>
		  <p:sp>
		    <p:nvSpPr><p:cNvPr id="2" name="Text With Break"/><p:cNvSpPr txBox="1"/><p:nvPr/></p:nvSpPr>
		    <p:spPr/>
		    <p:txBody><a:bodyPr/><a:p><a:r><a:t>{{na</a:t></a:r><a:r><a:t>me}}</a:t></a:r></a:p></p:txBody>
		  </p:sp>
		</p:spTree>`)
	spTree.Sp[0].TxBody.P[0] = paragraph
	slide.sx().CSld.SpTree = spTree

	slide.ReplaceText(map[string]string{"{{name}}": "Alice"})

	updatedParagraph := slide.sx().CSld.SpTree.Sp[0].TxBody.P[0]
	if len(updatedParagraph.R) != 1 {
		t.Fatalf("len(R) = %d, want 1", len(updatedParagraph.R))
	}
	if updatedParagraph.R[0].T != "Alice" {
		t.Fatalf("replacement text = %q, want %q", updatedParagraph.R[0].T, "Alice")
	}
}

func TestReplaceTextInShape(t *testing.T) {
	p := Create()
	slide := p.AddSlide()

	title := NewPlaceholderShape(PlaceholderTitle)
	title.SetName("Title 2")
	title.SetText("#TITLE#")
	_ = slide.AddShape(title)

	body := slide.AddTextBox()
	body.SetName("Body 1")
	body.SetText("#TITLE#")

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "shape_scope.pptx")
	if err := p.Save(path); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	p2, err := Open(path)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer mustClose(t, p2)

	slide2 := mustSlide(t, p2, 0)
	slide2.ReplaceTextInShape("Title 2", map[string]string{"#TITLE#": "Updated Title"})

	titleShape, ok := slide2.ShapeByName("Title 2").(*PlaceholderShape)
	if !ok {
		t.Fatalf("title shape type = %T, want *PlaceholderShape", slide2.ShapeByName("Title 2"))
	}
	if titleShape.Text() != "Updated Title" {
		t.Fatalf("title text = %q, want %q", titleShape.Text(), "Updated Title")
	}
	bodyShape, ok := slide2.ShapeByName("Body 1").(*TextBox)
	if !ok {
		t.Fatalf("body shape type = %T, want *TextBox", slide2.ShapeByName("Body 1"))
	}
	if bodyShape.Text() != "#TITLE#" {
		t.Fatalf("body text = %q, want %q", bodyShape.Text(), "#TITLE#")
	}
}

// --- ShapeByName Tests ---

func TestShapeByName(t *testing.T) {
	p := Create()
	slide := p.AddSlide()

	tb := slide.AddTextBox()
	tb.SetName("MyTextBox")
	tb.SetText("Hello")

	ph := NewPlaceholderShape(PlaceholderTitle)
	ph.SetName("MyTitle")
	ph.SetText("Title")
	_ = slide.AddShape(ph)

	// Find by name
	found := slide.ShapeByName("MyTextBox")
	if found == nil {
		t.Fatal("ShapeByName(MyTextBox) returned nil")
	}
	if found.Name() != "MyTextBox" {
		t.Errorf("Name = %q, want %q", found.Name(), "MyTextBox")
	}
	if _, ok := found.(*TextBox); !ok {
		t.Errorf("Expected *TextBox, got %T", found)
	}

	// Find placeholder by name
	found2 := slide.ShapeByName("MyTitle")
	if found2 == nil {
		t.Fatal("ShapeByName(MyTitle) returned nil")
	}
	if _, ok := found2.(*PlaceholderShape); !ok {
		t.Errorf("Expected *PlaceholderShape, got %T", found2)
	}

	// Not found
	if slide.ShapeByName("DoesNotExist") != nil {
		t.Error("ShapeByName(DoesNotExist) should return nil")
	}
}

func TestShapeByName_GroupChild(t *testing.T) {
	p := testPresentationWithoutLayouts()
	slide := p.AddSlide()

	slide.sx().CSld.SpTree = mustParseShapeTree(t, `
		<p:spTree xmlns:p="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		  <p:nvGrpSpPr><p:cNvPr id="1" name=""/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>
		  <p:grpSpPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="0" cy="0"/><a:chOff x="0" y="0"/><a:chExt cx="0" cy="0"/></a:xfrm></p:grpSpPr>
		  <p:grpSp>
		    <p:nvGrpSpPr><p:cNvPr id="2" name="Outer Group"/><p:cNvGrpSpPr/><p:nvPr/></p:nvGrpSpPr>
		    <p:grpSpPr/>
		    <p:sp>
		      <p:nvSpPr><p:cNvPr id="3" name="Inner Box"/><p:cNvSpPr txBox="1"/><p:nvPr/></p:nvSpPr>
		      <p:spPr/>
		      <p:txBody><a:bodyPr/><a:p><a:r><a:t>Grouped text</a:t></a:r></a:p></p:txBody>
		    </p:sp>
		  </p:grpSp>
		</p:spTree>`)
	slide.sx().CSld.SpTree.GrpSp[0].Shapes[0].TxBody.P[0].ResetRunOrder()
	slide.materializeShapes()

	if got := slide.ShapeByName("Outer Group"); got == nil {
		t.Fatal("ShapeByName on group returned nil")
	}
	if got := slide.ShapeByName("Inner Box"); got == nil {
		t.Fatal("ShapeByName on nested child returned nil")
	}
}

func TestShapeByName_AfterLoad(t *testing.T) {
	p, err := Open("testdata/test_slides.pptx")
	if err != nil {
		t.Skipf("Could not open test_slides.pptx: %v", err)
	}
	defer mustClose(t, p)

	slide := mustSlide(t, p, 0)

	// List shapes for debugging
	for _, s := range slide.Shapes() {
		name := s.Name()
		if name != "" {
			found := slide.ShapeByName(name)
			if found == nil {
				t.Errorf("ShapeByName(%q) returned nil but shape exists", name)
			}
			if found != s {
				t.Errorf("ShapeByName(%q) returned different shape instance", name)
			}
		}
	}
}

// --- Reverse Mapping Unit Tests ---

func TestOxmlToTextFrame(t *testing.T) {
	txBody := &dml.TxBody{
		BodyPr: &dml.BodyPr{
			Wrap:   "square",
			Anchor: "t",
		},
		P: []*dml.P{
			{R: []*dml.R{{T: "Hello"}}},
			{R: []*dml.R{{T: "World"}}},
		},
	}

	tf := oxmlToTextFrame(txBody)
	if tf == nil {
		t.Fatal("oxmlToTextFrame returned nil")
	}

	if len(tf.Paragraphs()) != 2 {
		t.Fatalf("len(Paragraphs) = %d, want 2", len(tf.Paragraphs()))
	}

	if tf.Text() != "Hello\nWorld" {
		t.Errorf("Text() = %q, want %q", tf.Text(), "Hello\nWorld")
	}
}

func TestOxmlToRun_Formatting(t *testing.T) {
	bTrue := true
	sz := int32(2400) // 24pt
	baseline := int32(30000)

	r := &dml.R{
		T: "Formatted",
		RPr: &dml.RPr{
			Sz:       sz,
			B:        &bTrue,
			I:        &bTrue,
			U:        "sng",
			Strike:   "sngStrike",
			Baseline: &baseline,
			Latin:    &dml.TextFont{Typeface: "Arial"},
		},
	}

	run := oxmlToRun(r)

	if run.Text() != "Formatted" {
		t.Errorf("Text = %q, want %q", run.Text(), "Formatted")
	}
	if run.FontSize() != 24.0 {
		t.Errorf("FontSize = %f, want 24.0", run.FontSize())
	}
	if !run.Bold() {
		t.Error("Bold = false, want true")
	}
	if !run.Italic() {
		t.Error("Italic = false, want true")
	}
	if run.Font() != "Arial" {
		t.Errorf("Font = %q, want %q", run.Font(), "Arial")
	}
	if run.Baseline() != 30000 {
		t.Errorf("Baseline = %d, want 30000", run.Baseline())
	}
}

func TestOxmlShapeToGoShape_Placeholder(t *testing.T) {
	sp := &oxml.Shape{
		NvSpPr: &oxml.NvSpPr{
			CNvPr:   &dml.CNvPr{Id: 2, Name: "Title 1"},
			CNvSpPr: &dml.CNvSpPr{},
			NvPr: &oxml.NvPr{
				Ph: &oxml.Placeholder{
					Type: "title",
					Idx:  0,
				},
			},
		},
		SpPr: &dml.SpPr{
			Xfrm: &dml.Xfrm{
				Off: &dml.OffXML{X: 457200, Y: 274638},
				Ext: &dml.ExtXML{Cx: 8229600, Cy: 1143000},
			},
		},
		TxBody: &dml.TxBody{
			BodyPr: &dml.BodyPr{},
			P:      []*dml.P{{R: []*dml.R{{T: "Test Title"}}}},
		},
	}

	shape := oxmlShapeToGoShape(sp)
	if shape == nil {
		t.Fatal("oxmlShapeToGoShape returned nil")
	}

	ph, ok := shape.(*PlaceholderShape)
	if !ok {
		t.Fatalf("Expected *PlaceholderShape, got %T", shape)
	}

	if ph.PlaceholderType() != PlaceholderTitle {
		t.Errorf("PlaceholderType = %q, want %q", ph.PlaceholderType(), PlaceholderTitle)
	}
	if ph.Name() != "Title 1" {
		t.Errorf("Name = %q, want %q", ph.Name(), "Title 1")
	}
	if ph.Text() != "Test Title" {
		t.Errorf("Text = %q, want %q", ph.Text(), "Test Title")
	}
}

func TestOxmlShapeToGoShape_TextBox(t *testing.T) {
	sp := &oxml.Shape{
		NvSpPr: &oxml.NvSpPr{
			CNvPr:   &dml.CNvPr{Id: 3, Name: "TextBox 1"},
			CNvSpPr: &dml.CNvSpPr{TxBox: true},
			NvPr:    &oxml.NvPr{},
		},
		SpPr: &dml.SpPr{
			Xfrm: &dml.Xfrm{
				Off: &dml.OffXML{X: 100000, Y: 200000},
				Ext: &dml.ExtXML{Cx: 300000, Cy: 400000},
			},
		},
		TxBody: &dml.TxBody{
			BodyPr: &dml.BodyPr{},
			P:      []*dml.P{{R: []*dml.R{{T: "Text box content"}}}},
		},
	}

	shape := oxmlShapeToGoShape(sp)
	if shape == nil {
		t.Fatal("oxmlShapeToGoShape returned nil")
	}

	tb, ok := shape.(*TextBox)
	if !ok {
		t.Fatalf("Expected *TextBox, got %T", shape)
	}

	if tb.Text() != "Text box content" {
		t.Errorf("Text = %q, want %q", tb.Text(), "Text box content")
	}
}

func TestOxmlShapeToGoShape_AutoShape(t *testing.T) {
	sp := &oxml.Shape{
		NvSpPr: &oxml.NvSpPr{
			CNvPr:   &dml.CNvPr{Id: 4, Name: "Oval 1"},
			CNvSpPr: &dml.CNvSpPr{},
			NvPr:    &oxml.NvPr{},
		},
		SpPr: &dml.SpPr{
			Xfrm: &dml.Xfrm{
				Off: &dml.OffXML{X: 0, Y: 0},
				Ext: &dml.ExtXML{Cx: 500000, Cy: 500000},
			},
			PrstGeom: &dml.PrstGeom{Prst: "ellipse"},
		},
	}

	shape := oxmlShapeToGoShape(sp)
	if shape == nil {
		t.Fatal("oxmlShapeToGoShape returned nil")
	}

	as, ok := shape.(*AutoShape)
	if !ok {
		t.Fatalf("Expected *AutoShape, got %T", shape)
	}

	if as.PresetGeometry() != "ellipse" {
		t.Errorf("PresetGeometry = %q, want %q", as.PresetGeometry(), "ellipse")
	}
}

// --- Helper Tests ---

func TestContentTypeFromExt(t *testing.T) {
	tests := []struct {
		ext  string
		want string
	}{
		{".png", "image/png"},
		{".jpg", "image/jpeg"},
		{".jpeg", "image/jpeg"},
		{".gif", "image/gif"},
		{".PNG", "image/png"},
		{".unknown", "image/png"},
	}

	for _, tt := range tests {
		got := contentTypeFromExt(tt.ext)
		if got != tt.want {
			t.Errorf("contentTypeFromExt(%q) = %q, want %q", tt.ext, got, tt.want)
		}
	}
}

func TestRelativeTarget(t *testing.T) {
	tests := []struct {
		source string
		target string
		want   string
	}{
		{"/ppt/slides/slide1.xml", "/ppt/media/image1.png", "../media/image1.png"},
		{"/ppt/slides/slide1.xml", "/ppt/slides/image.png", "image.png"},
	}

	for _, tt := range tests {
		got := relativeTarget(tt.source, tt.target)
		if got != tt.want {
			t.Errorf("relativeTarget(%q, %q) = %q, want %q", tt.source, tt.target, got, tt.want)
		}
	}
}

func TestParseThemeColorString(t *testing.T) {
	tests := []struct {
		val  string
		want dml.ThemeColor
	}{
		{"dk1", dml.ThemeColorDark1},
		{"lt1", dml.ThemeColorLight1},
		{"accent1", dml.ThemeColorAccent1},
		{"hlink", dml.ThemeColorHyperlink},
		{"unknown", dml.ThemeColorDark1}, // fallback
	}

	for _, tt := range tests {
		got := parseThemeColorString(tt.val)
		if got != tt.want {
			t.Errorf("parseThemeColorString(%q) = %d, want %d", tt.val, got, tt.want)
		}
	}
}

// createMinimalPNG creates a minimal valid 1x1 PNG image for testing.
func createMinimalPNG() []byte {
	// Minimal 1x1 white PNG
	return []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, // PNG signature
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52, // IHDR chunk
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x02, 0x00, 0x00, 0x00, 0x90, 0x77, 0x53,
		0xde, 0x00, 0x00, 0x00, 0x0c, 0x49, 0x44, 0x41, // IDAT chunk
		0x54, 0x08, 0xd7, 0x63, 0xf8, 0xcf, 0xc0, 0x00,
		0x00, 0x00, 0x02, 0x00, 0x01, 0xe2, 0x21, 0xbc,
		0x33, 0x00, 0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, // IEND chunk
		0x44, 0xae, 0x42, 0x60, 0x82,
	}
}

func createMinimalSVG() []byte {
	return []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="1" height="1" viewBox="0 0 1 1"><rect width="1" height="1" fill="#000"/></svg>`)
}
