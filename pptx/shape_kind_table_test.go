package pptx

import (
	"testing"

	"github.com/mgilbir/spine/common/dml"
)

// ShapeType is implemented on a dozen receivers, several of which are one-line
// `return ShapeTypeX` bodies copied from a sibling. A per-receiver happy-path
// call cannot see a duplicated case arm; the table below asserts each concrete
// shape reports its own value AND that no two receivers share one, which is the
// assertion that catches a copy/paste slip.

// TestShapeTypePerReceiver builds one instance of every shape kind and checks
// its ShapeType, its String(), and that the mapping receiver -> ShapeType is
// injective.
func TestShapeTypePerReceiver(t *testing.T) {
	p := Create()
	s := p.AddSlide()

	mp4 := append([]byte{0, 0, 0, 0x18}, []byte("ftypmp42")...)
	mp4 = append(mp4, make([]byte, 16)...)
	mp3 := append([]byte("ID3\x03\x00\x00\x00\x00\x00\x00"), make([]byte, 16)...)

	of, err := s.AddOLEObject([]byte("\xd0\xcf\x11\xe0payload"), "Excel.Sheet.12")
	if err != nil {
		t.Fatalf("AddOLEObject: %v", err)
	}
	sa := s.AddSmartArt(SmartArtProcess, &SmartArtNode{Text: "n"})
	if sa == nil || sa.frame == nil {
		t.Fatal("AddSmartArt returned no frame")
	}

	cases := []struct {
		name     string
		shape    Shape
		want     ShapeType
		wantName string
	}{
		{"TextBox", NewTextBox(), ShapeTypeTextBox, "textbox"},
		{"Picture", NewPicture(), ShapeTypePicture, "picture"},
		{"Table", NewTable(1, 1), ShapeTypeTable, "table"},
		{"ChartFrame", &ChartFrame{}, ShapeTypeChart, "chart"},
		{"GroupShape", NewGroupShape(), ShapeTypeGroup, "group"},
		{"PlaceholderShape", NewPlaceholderShape(PlaceholderTitle), ShapeTypePlaceholder, "placeholder"},
		{"Connector", NewConnector(ConnectorStraight), ShapeTypeConnector, "connector"},
		{"AutoShape", NewAutoShape(PresetRect), ShapeTypeAutoShape, "autoshape"},
		{"Video", NewVideo(mp4, "video/mp4"), ShapeTypeVideo, "video"},
		{"Audio", NewAudio(mp3, "audio/mpeg"), ShapeTypeAudio, "audio"},
		{"SmartArtFrame", sa.frame, ShapeTypeDiagram, "diagram"},
		{"OLEObjectFrame", of, ShapeTypeOLEObject, "oleobject"},
	}

	owner := map[ShapeType]string{}
	names := map[string]string{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.shape.ShapeType()
			if got != tc.want {
				t.Errorf("%s.ShapeType() = %v, want %v", tc.name, got, tc.want)
			}
			if s := got.String(); s != tc.wantName {
				t.Errorf("%s.ShapeType().String() = %q, want %q", tc.name, s, tc.wantName)
			}
		})
		if prev, dup := owner[tc.want]; dup {
			t.Errorf("%s and %s both report ShapeType %v; every kind must be distinct", prev, tc.name, tc.want)
		}
		owner[tc.want] = tc.name
		if prev, dup := names[tc.wantName]; dup {
			t.Errorf("%s and %s both stringify to %q", prev, tc.name, tc.wantName)
		}
		names[tc.wantName] = tc.name
	}

	// Every named ShapeType constant except Unknown must be claimed by some
	// receiver above, so a kind added tomorrow forces this table to grow.
	for st := ShapeTypeTextBox; st <= ShapeTypeOLEObject; st++ {
		if _, ok := owner[st]; !ok {
			t.Errorf("ShapeType %v (%q) is not covered by any receiver in this table", int(st), st)
		}
	}
}

// TestShapeTypeString checks the enum's own String, including the fallback for
// a value outside the declared range.
func TestShapeTypeString(t *testing.T) {
	want := map[ShapeType]string{
		ShapeTypeUnknown:     "unknown",
		ShapeTypeTextBox:     "textbox",
		ShapeTypePicture:     "picture",
		ShapeTypeTable:       "table",
		ShapeTypeChart:       "chart",
		ShapeTypeGroup:       "group",
		ShapeTypePlaceholder: "placeholder",
		ShapeTypeConnector:   "connector",
		ShapeTypeAutoShape:   "autoshape",
		ShapeTypeVideo:       "video",
		ShapeTypeAudio:       "audio",
		ShapeTypeDiagram:     "diagram",
		ShapeTypeOLEObject:   "oleobject",
	}
	for st, s := range want {
		if got := st.String(); got != s {
			t.Errorf("ShapeType(%d).String() = %q, want %q", int(st), got, s)
		}
	}
	if got := ShapeType(999).String(); got != "unknown" {
		t.Errorf("out-of-range ShapeType.String() = %q, want unknown", got)
	}
}

// TestConnectorKindString covers ConnectorKind.String and pins it to the
// preset geometry the same kind serializes as, so a case arm that returns a
// sibling's name (or a preset that belongs to another routing) fails.
func TestConnectorKindString(t *testing.T) {
	cases := []struct {
		kind       ConnectorKind
		wantName   string
		wantPreset string
	}{
		{ConnectorStraight, "straight", "straightConnector1"},
		{ConnectorElbow, "elbow", "bentConnector3"},
		{ConnectorCurved, "curved", "curvedConnector3"},
	}
	seen := map[string]ConnectorKind{}
	for _, tc := range cases {
		if got := tc.kind.String(); got != tc.wantName {
			t.Errorf("ConnectorKind(%d).String() = %q, want %q", int(tc.kind), got, tc.wantName)
		}
		if got := tc.kind.presetGeom(); got != tc.wantPreset {
			t.Errorf("%s.presetGeom() = %q, want %q", tc.wantName, got, tc.wantPreset)
		}
		// String and presetGeom must agree on which kind they describe.
		if got := connectorKindFromPreset(tc.kind.presetGeom()); got != tc.kind {
			t.Errorf("connectorKindFromPreset(%q) = %v, want %v", tc.kind.presetGeom(), got, tc.kind)
		}
		if prev, dup := seen[tc.wantName]; dup {
			t.Errorf("kinds %v and %v both stringify to %q", prev, tc.kind, tc.wantName)
		}
		seen[tc.wantName] = tc.kind
	}
	// An unrecognized routing falls back to straight, not to a zero-value name.
	if got := ConnectorKind(42).String(); got != "straight" {
		t.Errorf("unknown ConnectorKind.String() = %q, want straight", got)
	}
}

// TestConnectorLineReadBack sets the line width and dash on a connector, saves,
// reopens, and reads them back through LineWidth/LineDash. This proves the
// values reached the a:ln in the slide part rather than only the in-memory
// struct.
func TestConnectorLineReadBack(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	c := s.AddConnector(ConnectorElbow)
	c.SetPosition(dml.Inches(1), dml.Inches(1))
	c.SetSize(dml.Inches(2), dml.Inches(1))

	// A connector with no explicit line reports zero/empty, not a default.
	if got := c.LineWidth(); got != 0 {
		t.Errorf("LineWidth() before SetLine = %v, want 0", got)
	}
	if got := c.LineDash(); got != "" {
		t.Errorf("LineDash() before SetLine = %q, want \"\"", got)
	}

	c.SetLineWidth(2.5)
	c.SetLineDash(dml.DashDash)
	c.SetLineColor(dml.NewRGB(0x11, 0x22, 0x33).ToColor())

	if got := c.LineWidth(); got != 2.5 {
		t.Errorf("LineWidth() = %v, want 2.5", got)
	}
	if got := c.LineDash(); got != dml.DashDash {
		t.Errorf("LineDash() = %q, want %q", got, dml.DashDash)
	}

	rp := saveReopen(t, p)
	rs, err := rp.Slide(0)
	if err != nil {
		t.Fatalf("Slide(0): %v", err)
	}
	var got *Connector
	for _, sh := range rs.Shapes() {
		if cn, ok := sh.(*Connector); ok {
			got = cn
		}
	}
	if got == nil {
		t.Fatal("no connector on the reopened slide")
	}
	if w := got.LineWidth(); w != 2.5 {
		t.Errorf("reopened LineWidth() = %v, want 2.5", w)
	}
	if d := got.LineDash(); d != dml.DashDash {
		t.Errorf("reopened LineDash() = %q, want %q", d, dml.DashDash)
	}
	// Width and dash must not be crossed with the colour that was set alongside.
	if col := got.LineColor(); col == nil || col.RGB != dml.NewRGB(0x11, 0x22, 0x33) {
		t.Errorf("reopened LineColor() = %v, want 112233", col)
	}
}
