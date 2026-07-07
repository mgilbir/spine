package docx

import (
	"testing"

	"github.com/mgilbir/spine/opc"
)

// C62: AddHeading clamps the level to 1-9 instead of producing a garbage style
// name via rune arithmetic (level 10 previously yielded "Heading:").
func TestAddHeading_ClampsLevel(t *testing.T) {
	cases := []struct {
		level int
		want  string
	}{
		{1, "Heading1"},
		{9, "Heading9"},
		{10, "Heading9"},
		{0, "Heading1"},
		{-3, "Heading1"},
	}
	for _, c := range cases {
		d := Create()
		p := d.AddHeading("x", c.level)
		if got := p.Style(); got != c.want {
			t.Errorf("AddHeading level %d -> style %q, want %q", c.level, got, c.want)
		}
	}
}

// C56: nextRelID seeds past the highest existing numeric rId, so it does not
// collide on non-contiguous relationship ids.
func TestNextRelID_NonContiguous(t *testing.T) {
	d := &Document{
		relationships: map[string][]*opc.Relationship{
			"/word/document.xml": {{ID: "rId1"}, {ID: "rId3"}},
		},
	}
	if id := d.nextRelID(); id <= 3 {
		t.Errorf("nextRelID = %d, want > 3 (collides with existing rId3)", id)
	}
}
