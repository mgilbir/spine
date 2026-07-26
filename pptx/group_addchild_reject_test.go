package pptx

import "testing"

// C311: GroupShape.AddChild must reject shape kinds appendGroupChild cannot
// serialize (ChartFrame/SmartArtFrame/OLEObjectFrame) instead of accepting
// them into Children() only to silently drop them on save.
func TestGroupShape_AddChildRejectsUnserializable(t *testing.T) {
	cases := []struct {
		name  string
		shape Shape
	}{
		{"ChartFrame", &ChartFrame{}},
		{"SmartArtFrame", &SmartArtFrame{}},
		{"OLEObjectFrame", &OLEObjectFrame{}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := NewGroupShape()
			if err := g.AddChild(tc.shape); err == nil {
				t.Fatalf("AddChild(%s) returned nil error; want rejection", tc.name)
			}
			if len(g.Children()) != 0 {
				t.Fatalf("AddChild(%s) added a dropped-on-save child to Children() (len=%d)", tc.name, len(g.Children()))
			}
		})
	}
}

// The supported kinds must still be accepted with no error.
func TestGroupShape_AddChildAcceptsSupported(t *testing.T) {
	g := NewGroupShape()
	if err := g.AddChild(NewTextBox()); err != nil {
		t.Fatalf("AddChild(TextBox) = %v, want nil", err)
	}
	if err := g.AddChild(NewTable(1, 1)); err != nil {
		t.Fatalf("AddChild(Table) = %v, want nil", err)
	}
	if len(g.Children()) != 2 {
		t.Fatalf("Children() = %d, want 2", len(g.Children()))
	}
}
