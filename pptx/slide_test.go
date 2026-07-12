package pptx

import "testing"

func TestSlide_Index(t *testing.T) {
	p := Create()
	slide0 := p.AddSlide()
	slide1 := p.AddSlide()
	slide2 := p.AddSlide()

	if slide0.Index() != 0 {
		t.Errorf("slide0.Index() = %d, want 0", slide0.Index())
	}
	if slide1.Index() != 1 {
		t.Errorf("slide1.Index() = %d, want 1", slide1.Index())
	}
	if slide2.Index() != 2 {
		t.Errorf("slide2.Index() = %d, want 2", slide2.Index())
	}
}

func TestSlide_Name(t *testing.T) {
	p := Create()
	slide := p.AddSlide()

	// Initial name should be empty
	if slide.Name() != "" {
		t.Errorf("Initial Name() = %q, want empty", slide.Name())
	}

	// Set name
	slide.SetName("Test Slide")
	if slide.Name() != "Test Slide" {
		t.Errorf("After SetName, Name() = %q, want %q", slide.Name(), "Test Slide")
	}
}

func TestSlide_Layout(t *testing.T) {
	p := Create()
	slide := p.AddSlide()

	// Without layout, should return nil
	if slide.Layout() != nil {
		t.Error("Slide without layout should return nil")
	}
}

func TestSlide_Shapes(t *testing.T) {
	p := Create()
	slide := p.AddSlide()

	// Initial shapes should be empty
	if len(slide.Shapes()) != 0 {
		t.Errorf("Initial Shapes() has %d shapes, want 0", len(slide.Shapes()))
	}
}

func TestSlide_AddShape(t *testing.T) {
	p := Create()
	slide := p.AddSlide()

	tb := NewTextBox()
	_ = slide.AddShape(tb)

	if len(slide.Shapes()) != 1 {
		t.Errorf("After AddShape, Shapes() has %d shapes, want 1", len(slide.Shapes()))
	}
}

func TestSlide_RemoveShape(t *testing.T) {
	p := Create()
	slide := p.AddSlide()

	tb1 := NewTextBox()
	tb2 := NewTextBox()
	_ = slide.AddShape(tb1)
	_ = slide.AddShape(tb2)

	slide.RemoveShape(tb1)

	if len(slide.Shapes()) != 1 {
		t.Errorf("After RemoveShape, Shapes() has %d shapes, want 1", len(slide.Shapes()))
	}

	// Remaining shape should be tb2
	if slide.Shapes()[0] != tb2 {
		t.Error("Wrong shape removed")
	}
}

func TestSlide_AddTextBox(t *testing.T) {
	p := Create()
	slide := p.AddSlide()

	tb := slide.AddTextBox()
	if tb == nil {
		t.Fatal("AddTextBox() returned nil")
	}

	if tb.ShapeType() != ShapeTypeTextBox {
		t.Errorf("TextBox.ShapeType() = %v, want ShapeTypeTextBox", tb.ShapeType())
	}

	if len(slide.Shapes()) != 1 {
		t.Errorf("After AddTextBox, Shapes() has %d shapes, want 1", len(slide.Shapes()))
	}
}

func TestSlide_AddTable(t *testing.T) {
	p := Create()
	slide := p.AddSlide()

	table := slide.AddTable(3, 4)
	if table == nil {
		t.Fatal("AddTable() returned nil")
	}

	if table.RowCount() != 3 {
		t.Errorf("Table.RowCount() = %d, want 3", table.RowCount())
	}
	if table.ColCount() != 4 {
		t.Errorf("Table.ColCount() = %d, want 4", table.ColCount())
	}
}

func TestSlide_Placeholders(t *testing.T) {
	p := Create()
	slide := p.AddSlide()

	// Add placeholder shapes
	titlePh := NewPlaceholderShape(PlaceholderTitle)
	bodyPh := NewPlaceholderShape(PlaceholderBody)
	tb := NewTextBox() // Not a placeholder

	_ = slide.AddShape(titlePh)
	_ = slide.AddShape(bodyPh)
	_ = slide.AddShape(tb)

	placeholders := slide.Placeholders()
	if len(placeholders) != 2 {
		t.Errorf("Placeholders() returned %d, want 2", len(placeholders))
	}
}

func TestSlide_GetPlaceholder(t *testing.T) {
	p := Create()
	slide := p.AddSlide()

	titlePh := NewPlaceholderShape(PlaceholderTitle)
	bodyPh := NewPlaceholderShape(PlaceholderBody)

	_ = slide.AddShape(titlePh)
	_ = slide.AddShape(bodyPh)

	// Find title placeholder
	found := slide.GetPlaceholder(PlaceholderTitle)
	if found != titlePh {
		t.Error("GetPlaceholder(PlaceholderTitle) did not return title placeholder")
	}

	// Find body placeholder
	found = slide.GetPlaceholder(PlaceholderBody)
	if found != bodyPh {
		t.Error("GetPlaceholder(PlaceholderBody) did not return body placeholder")
	}

	// Find nonexistent placeholder
	found = slide.GetPlaceholder(PlaceholderChart)
	if found != nil {
		t.Error("GetPlaceholder for nonexistent type should return nil")
	}
}

func TestSlide_TitlePlaceholder(t *testing.T) {
	p := Create()
	slide := p.AddSlide()

	titlePh := NewPlaceholderShape(PlaceholderTitle)
	_ = slide.AddShape(titlePh)

	found := slide.TitlePlaceholder()
	if found != titlePh {
		t.Error("TitlePlaceholder() did not return correct placeholder")
	}
}

func TestSlide_BodyPlaceholder(t *testing.T) {
	p := Create()
	slide := p.AddSlide()

	bodyPh := NewPlaceholderShape(PlaceholderBody)
	_ = slide.AddShape(bodyPh)

	found := slide.BodyPlaceholder()
	if found != bodyPh {
		t.Error("BodyPlaceholder() did not return correct placeholder")
	}
}

func TestSlide_Duplicate(t *testing.T) {
	p := Create()
	original := p.AddSlide()
	original.SetName("Original")
	title := NewPlaceholderShape(PlaceholderTitle)
	title.SetName("Title 2")
	title.SetText("Hello {{name}}")
	_ = original.AddShape(title)

	duplicate := original.Duplicate()

	if p.SlideCount() != 2 {
		t.Errorf("After Duplicate, SlideCount() = %d, want 2", p.SlideCount())
	}

	// Duplicate should be after original
	if duplicate.Index() != 1 {
		t.Errorf("Duplicate.Index() = %d, want 1", duplicate.Index())
	}
	if duplicate.ShapeByName("Title 2") == nil {
		t.Fatal("duplicate should materialize copied shapes")
	}
}

func TestSlide_Delete(t *testing.T) {
	p := Create()
	slide0 := p.AddSlide()
	slide0.SetName("Slide0")
	slide1 := p.AddSlide()
	slide1.SetName("Slide1")
	slide2 := p.AddSlide()
	slide2.SetName("Slide2")

	err := slide1.Delete()
	if err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	if p.SlideCount() != 2 {
		t.Errorf("After Delete, SlideCount() = %d, want 2", p.SlideCount())
	}

	// Check remaining slides
	s, _ := p.Slide(0)
	if s.Name() != "Slide0" {
		t.Errorf("After Delete, slide 0 is %q, want Slide0", s.Name())
	}
	s, _ = p.Slide(1)
	if s.Name() != "Slide2" {
		t.Errorf("After Delete, slide 1 is %q, want Slide2", s.Name())
	}
}

func TestSlide_marshal(t *testing.T) {
	p := Create()
	slide := p.AddSlide()
	slide.SetName("Test Slide")

	data, err := slide.marshal()
	if err != nil {
		t.Fatalf("marshal() error = %v", err)
	}

	// Verify it's valid XML
	xmlStr := string(data)
	if !contains(xmlStr, "<?xml") {
		t.Error("Missing XML declaration")
	}
	if !contains(xmlStr, "sld") {
		t.Error("Missing sld element")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr, 0))
}

func containsAt(s, substr string, start int) bool {
	for i := start; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestSlide_marshalWithTextBox(t *testing.T) {
	p := Create()
	slide := p.AddSlide()

	tb := slide.AddTextBox()
	tb.SetName("MyTextBox")
	tb.SetText("Hello World")
	tb.SetPosition(100000, 200000)
	tb.SetSize(1000000, 500000)

	data, err := slide.marshal()
	if err != nil {
		t.Fatalf("marshal() error = %v", err)
	}

	xmlStr := string(data)

	// Verify the shape is present
	if !contains(xmlStr, "sp") {
		t.Error("Missing sp element in XML")
	}
	if !contains(xmlStr, "MyTextBox") {
		t.Error("Missing shape name in XML")
	}
	if !contains(xmlStr, "Hello World") {
		t.Error("Missing text content in XML")
	}
	if !contains(xmlStr, "txBox") {
		t.Error("Missing txBox attribute in XML")
	}
}

func TestSlide_marshalWithTable(t *testing.T) {
	p := Create()
	slide := p.AddSlide()

	table := slide.AddTable(2, 3)
	table.SetName("MyTable")
	table.Cell(0, 0).SetText("Header 1")
	table.Cell(0, 1).SetText("Header 2")
	table.Cell(1, 0).SetText("Data 1")

	data, err := slide.marshal()
	if err != nil {
		t.Fatalf("marshal() error = %v", err)
	}

	xmlStr := string(data)

	// Verify the table is present
	if !contains(xmlStr, "graphicFrame") {
		t.Error("Missing graphicFrame element in XML")
	}
	if !contains(xmlStr, "tbl") {
		t.Error("Missing tbl element in XML")
	}
	if !contains(xmlStr, "Header 1") {
		t.Error("Missing table cell text in XML")
	}
	if !contains(xmlStr, "gridCol") {
		t.Error("Missing gridCol element in XML")
	}
}

func TestSlide_marshalWithPlaceholder(t *testing.T) {
	p := Create()
	slide := p.AddSlide()

	ph := NewPlaceholderShape(PlaceholderTitle)
	ph.SetText("Title Text")
	ph.SetPosition(100000, 100000)
	ph.SetSize(5000000, 1000000)
	_ = slide.AddShape(ph)

	data, err := slide.marshal()
	if err != nil {
		t.Fatalf("marshal() error = %v", err)
	}

	xmlStr := string(data)

	// Verify the placeholder is present
	if !contains(xmlStr, "sp") {
		t.Error("Missing sp element in XML")
	}
	if !contains(xmlStr, "ph") {
		t.Error("Missing ph element in XML (placeholder reference)")
	}
	if !contains(xmlStr, "title") {
		t.Error("Missing placeholder type in XML")
	}
	if !contains(xmlStr, "Title Text") {
		t.Error("Missing placeholder text in XML")
	}
}

func TestSlide_marshalWithAutoShape(t *testing.T) {
	p := Create()
	slide := p.AddSlide()

	as := NewAutoShape(PresetRect)
	as.SetName("Rectangle")
	as.SetPosition(100000, 100000)
	as.SetSize(1000000, 1000000)
	_ = slide.AddShape(as)

	data, err := slide.marshal()
	if err != nil {
		t.Fatalf("marshal() error = %v", err)
	}

	xmlStr := string(data)

	// Verify the shape is present
	if !contains(xmlStr, "sp") {
		t.Error("Missing sp element in XML")
	}
	if !contains(xmlStr, "prstGeom") {
		t.Error("Missing prstGeom element in XML")
	}
	if !contains(xmlStr, "rect") {
		t.Error("Missing preset geometry 'rect' in XML")
	}
}

func TestSlide_marshalMultipleShapes(t *testing.T) {
	p := Create()
	slide := p.AddSlide()

	// Add multiple shapes
	tb1 := slide.AddTextBox()
	tb1.SetText("First")

	tb2 := slide.AddTextBox()
	tb2.SetText("Second")

	table := slide.AddTable(2, 2)
	table.Cell(0, 0).SetText("Cell")

	data, err := slide.marshal()
	if err != nil {
		t.Fatalf("marshal() error = %v", err)
	}

	xmlStr := string(data)

	// Verify all shapes are present
	if !contains(xmlStr, "First") {
		t.Error("Missing first TextBox text in XML")
	}
	if !contains(xmlStr, "Second") {
		t.Error("Missing second TextBox text in XML")
	}
	if !contains(xmlStr, "Cell") {
		t.Error("Missing table cell text in XML")
	}
}
