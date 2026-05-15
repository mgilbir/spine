package pptx

import (
	"testing"

	"github.com/mgilbir/spine/common/dml"
)

func TestPlaceholderType_Constants(t *testing.T) {
	types := []PlaceholderType{
		PlaceholderTitle,
		PlaceholderBody,
		PlaceholderCenteredTitle,
		PlaceholderSubtitle,
		PlaceholderDateTime,
		PlaceholderSlideNumber,
		PlaceholderFooter,
		PlaceholderHeader,
		PlaceholderObject,
		PlaceholderChart,
		PlaceholderTable,
		PlaceholderClipArt,
		PlaceholderDiagram,
		PlaceholderMedia,
		PlaceholderSlideImage,
		PlaceholderPicture,
	}

	for _, pt := range types {
		if pt == "" {
			t.Error("PlaceholderType constant is empty")
		}
	}
}

func TestNewPlaceholderShape(t *testing.T) {
	ph := NewPlaceholderShape(PlaceholderTitle)
	if ph == nil {
		t.Fatal("NewPlaceholderShape() returned nil")
	}

	if ph.ShapeType() != ShapeTypePlaceholder {
		t.Errorf("ShapeType() = %v, want ShapeTypePlaceholder", ph.ShapeType())
	}

	if ph.PlaceholderType() != PlaceholderTitle {
		t.Errorf("PlaceholderType() = %v, want PlaceholderTitle", ph.PlaceholderType())
	}
}

func TestPlaceholderShape_Orientation(t *testing.T) {
	ph := NewPlaceholderShape(PlaceholderBody)

	if ph.Orientation() != "" {
		t.Errorf("Default Orientation() = %v, want empty", ph.Orientation())
	}

	ph.SetOrientation(PlaceholderOrientationVertical)
	if ph.Orientation() != PlaceholderOrientationVertical {
		t.Errorf("After SetOrientation, Orientation() = %v, want PlaceholderOrientationVertical", ph.Orientation())
	}
}

func TestPlaceholderShape_Size(t *testing.T) {
	ph := NewPlaceholderShape(PlaceholderBody)

	ph.SetPlaceholderSize(PlaceholderSizeHalf)
	if ph.PlaceholderSize() != PlaceholderSizeHalf {
		t.Errorf("PlaceholderSize() = %v, want PlaceholderSizeHalf", ph.PlaceholderSize())
	}
}

func TestPlaceholderShape_Index(t *testing.T) {
	ph := NewPlaceholderShape(PlaceholderBody)

	if ph.Index() != 0 {
		t.Errorf("Default Index() = %d, want 0", ph.Index())
	}

	ph.SetIndex(5)
	if ph.Index() != 5 {
		t.Errorf("After SetIndex, Index() = %d, want 5", ph.Index())
	}
}

func TestPlaceholderShape_Text(t *testing.T) {
	ph := NewPlaceholderShape(PlaceholderTitle)

	ph.SetText("Hello World")
	if ph.Text() != "Hello World" {
		t.Errorf("Text() = %q, want %q", ph.Text(), "Hello World")
	}
}

func TestPlaceholderShape_TextFrame(t *testing.T) {
	ph := NewPlaceholderShape(PlaceholderTitle)
	tf := ph.TextFrame()

	if tf == nil {
		t.Fatal("TextFrame() returned nil")
	}

	tf.SetText("Test")
	if ph.Text() != "Test" {
		t.Errorf("After TextFrame.SetText, Text() = %q, want %q", ph.Text(), "Test")
	}
}

func TestPlaceholderShape_IsTitle(t *testing.T) {
	tests := []struct {
		phType PlaceholderType
		want   bool
	}{
		{PlaceholderTitle, true},
		{PlaceholderCenteredTitle, true},
		{PlaceholderSubtitle, false},
		{PlaceholderBody, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.phType), func(t *testing.T) {
			ph := NewPlaceholderShape(tt.phType)
			if ph.IsTitle() != tt.want {
				t.Errorf("IsTitle() = %v, want %v", ph.IsTitle(), tt.want)
			}
		})
	}
}

func TestPlaceholderShape_IsBody(t *testing.T) {
	tests := []struct {
		phType PlaceholderType
		want   bool
	}{
		{PlaceholderBody, true},
		{PlaceholderObject, true},
		{PlaceholderTitle, false},
		{PlaceholderSubtitle, false},
	}

	for _, tt := range tests {
		t.Run(string(tt.phType), func(t *testing.T) {
			ph := NewPlaceholderShape(tt.phType)
			if ph.IsBody() != tt.want {
				t.Errorf("IsBody() = %v, want %v", ph.IsBody(), tt.want)
			}
		})
	}
}

func TestPlaceholderShape_SetSVGData(t *testing.T) {
	ph := NewPlaceholderShape(PlaceholderPicture)
	svgData := []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="1" height="1"></svg>`)

	if err := ph.SetSVGData(svgData); err != nil {
		t.Fatalf("SetSVGData failed: %v", err)
	}
	if string(ph.pendingSVGData) != string(svgData) {
		t.Fatal("pendingSVGData was not stored")
	}
	if ph.pendingSVGCT != "image/svg+xml" {
		t.Fatalf("pendingSVGCT = %q, want image/svg+xml", ph.pendingSVGCT)
	}
	if string(ph.pendingImageData) != string(minimalTransparentPNG) {
		t.Fatal("fallback PNG was not populated")
	}
	if ph.pendingImageCT != "image/png" {
		t.Fatalf("pendingImageCT = %q, want image/png", ph.pendingImageCT)
	}
}

func TestPlaceholderShape_SetSVGData_NotPicturePlaceholder(t *testing.T) {
	ph := NewPlaceholderShape(PlaceholderTitle)
	err := ph.SetSVGData([]byte(`<svg xmlns="http://www.w3.org/2000/svg"></svg>`))
	if err != ErrNotPicturePlaceholder {
		t.Fatalf("expected ErrNotPicturePlaceholder, got %v", err)
	}
}

func TestDefaultTitlePlaceholder(t *testing.T) {
	ph := DefaultTitlePlaceholder()
	if ph == nil {
		t.Fatal("DefaultTitlePlaceholder() returned nil")
	}

	if ph.PlaceholderType() != PlaceholderTitle {
		t.Errorf("PlaceholderType() = %v, want PlaceholderTitle", ph.PlaceholderType())
	}

	x, y := ph.Position()
	if x == 0 || y == 0 {
		t.Error("Default position should not be (0, 0)")
	}

	w, h := ph.Size()
	if w == 0 || h == 0 {
		t.Error("Default size should not be (0, 0)")
	}
}

func TestDefaultBodyPlaceholder(t *testing.T) {
	ph := DefaultBodyPlaceholder()
	if ph == nil {
		t.Fatal("DefaultBodyPlaceholder() returned nil")
	}

	if ph.PlaceholderType() != PlaceholderBody {
		t.Errorf("PlaceholderType() = %v, want PlaceholderBody", ph.PlaceholderType())
	}
}

func TestDefaultCenteredTitlePlaceholder(t *testing.T) {
	ph := DefaultCenteredTitlePlaceholder()
	if ph == nil {
		t.Fatal("DefaultCenteredTitlePlaceholder() returned nil")
	}

	if ph.PlaceholderType() != PlaceholderCenteredTitle {
		t.Errorf("PlaceholderType() = %v, want PlaceholderCenteredTitle", ph.PlaceholderType())
	}
}

func TestDefaultSubtitlePlaceholder(t *testing.T) {
	ph := DefaultSubtitlePlaceholder()
	if ph == nil {
		t.Fatal("DefaultSubtitlePlaceholder() returned nil")
	}

	if ph.PlaceholderType() != PlaceholderSubtitle {
		t.Errorf("PlaceholderType() = %v, want PlaceholderSubtitle", ph.PlaceholderType())
	}
}

func TestPlaceholderShape_Position(t *testing.T) {
	ph := NewPlaceholderShape(PlaceholderTitle)
	ph.SetPosition(dml.Inches(1), dml.Inches(2))

	x, y := ph.Position()
	if x != dml.Inches(1) || y != dml.Inches(2) {
		t.Errorf("Position() = (%d, %d), want (%d, %d)", x, y, dml.Inches(1), dml.Inches(2))
	}
}

func TestPlaceholderOrientationConstants(t *testing.T) {
	if PlaceholderOrientationHorizontal != "horz" {
		t.Errorf("PlaceholderOrientationHorizontal = %q, want %q", PlaceholderOrientationHorizontal, "horz")
	}
	if PlaceholderOrientationVertical != "vert" {
		t.Errorf("PlaceholderOrientationVertical = %q, want %q", PlaceholderOrientationVertical, "vert")
	}
}

func TestPlaceholderSizeConstants(t *testing.T) {
	if PlaceholderSizeFull != "full" {
		t.Errorf("PlaceholderSizeFull = %q, want %q", PlaceholderSizeFull, "full")
	}
	if PlaceholderSizeHalf != "half" {
		t.Errorf("PlaceholderSizeHalf = %q, want %q", PlaceholderSizeHalf, "half")
	}
	if PlaceholderSizeQuarter != "quarter" {
		t.Errorf("PlaceholderSizeQuarter = %q, want %q", PlaceholderSizeQuarter, "quarter")
	}
}
