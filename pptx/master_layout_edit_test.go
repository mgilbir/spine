package pptx

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mgilbir/spine/common/dml"
)

// --- Layout background ---

func TestLayoutBackgroundSolidFill(t *testing.T) {
	pres := Create()
	master := pres.SlideMasters()[0]
	layout := master.Layouts()[0]

	if layout.HasBackground() {
		t.Fatal("new layout unexpectedly reports a background")
	}
	layout.SetBackgroundFill(dml.NewSolidFill(dml.NewRGB(0xAB, 0xCD, 0xEF).ToColor()))
	if !layout.HasBackground() {
		t.Fatal("HasBackground() false after SetBackgroundFill")
	}
	if c, ok := layout.BackgroundColor(); !ok || c.RGB != dml.NewRGB(0xAB, 0xCD, 0xEF) {
		t.Errorf("layout BackgroundColor = %v ok=%v", c, ok)
	}

	data, err := pres.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	xml := string(zipPart(t, data, "ppt/slideLayouts/slideLayout1.xml"))
	if !strings.Contains(xml, `<a:srgbClr val="ABCDEF"`) {
		t.Errorf("layout background fill not emitted:\n%s", xml)
	}

	// Reopen and confirm the fill survives the round trip.
	p2, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	l2 := p2.SlideMasters()[0].Layouts()[0]
	if c, ok := l2.BackgroundColor(); !ok || c.RGB != dml.NewRGB(0xAB, 0xCD, 0xEF) {
		t.Errorf("after reopen BackgroundColor = %v ok=%v", c, ok)
	}

	layout.ClearBackground()
	if layout.HasBackground() {
		t.Error("HasBackground() true after ClearBackground")
	}
}

// --- Master text styles ---

func TestMasterTextStyleEditRoundTrip(t *testing.T) {
	p, err := Open("testdata/test_slides.pptx")
	if err != nil {
		t.Fatal(err)
	}
	master := p.SlideMasters()[0]

	ts := master.TitleStyle()
	ts.SetLevelFont(0, "Georgia")
	ts.SetLevelFontSize(0, 40)
	ts.SetLevelBold(0, true)
	ts.SetLevelColor(0, dml.NewRGB(0x11, 0x22, 0x33).ToColor())

	body := master.BodyStyle()
	body.SetLevelBulletChar(1, "-")

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}

	p2, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	lvl := p2.SlideMasters()[0].TitleStyle().Level(0)
	if lvl == nil {
		t.Fatal("title style level 0 missing after round trip")
	}
	if lvl.FontName != "Georgia" {
		t.Errorf("FontName = %q, want Georgia", lvl.FontName)
	}
	if lvl.FontSize != 40 {
		t.Errorf("FontSize = %v, want 40", lvl.FontSize)
	}
	if !lvl.Bold {
		t.Error("Bold = false, want true")
	}
	if !lvl.HasColor || lvl.Color.RGB != dml.NewRGB(0x11, 0x22, 0x33) {
		t.Errorf("Color = %v hasColor=%v", lvl.Color, lvl.HasColor)
	}

	bl := p2.SlideMasters()[0].BodyStyle().Level(1)
	if bl == nil || bl.BulletType != BulletChar || bl.BulletChar != "-" {
		t.Errorf("body level 1 bullet = %+v", bl)
	}
}

// Editing only the master's text styles must not disturb any layout part: the
// layouts round-trip byte-for-byte, identical to a clean (unedited) save.
func TestMasterTextStyleEdit_LayoutsByteIdentical(t *testing.T) {
	clean, err := Open("testdata/test_slides.pptx")
	if err != nil {
		t.Fatal(err)
	}
	cleanData, err := clean.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}

	edited, err := Open("testdata/test_slides.pptx")
	if err != nil {
		t.Fatal(err)
	}
	edited.SlideMasters()[0].TitleStyle().SetLevelFontSize(0, 48)
	editedData, err := edited.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{
		"ppt/slideLayouts/slideLayout1.xml",
		"ppt/slideLayouts/slideLayout5.xml",
		"ppt/slideLayouts/slideLayout11.xml",
	} {
		if !bytes.Equal(zipPart(t, cleanData, name), zipPart(t, editedData, name)) {
			t.Errorf("%s changed after editing only the master text style", name)
		}
	}
	// The master itself must have changed (the edit landed somewhere).
	m := "ppt/slideMasters/slideMaster1.xml"
	if bytes.Equal(zipPart(t, cleanData, m), zipPart(t, editedData, m)) {
		t.Error("master unchanged despite text-style edit")
	}
}

// --- Layout placeholder geometry ---

// firstLayoutWithPlaceholder returns the first layout carrying at least one
// placeholder, along with its index within p.SlideLayouts().
func firstLayoutWithPlaceholder(t *testing.T, p *Presentation) (int, *SlideLayout) {
	t.Helper()
	for i, l := range p.SlideLayouts() {
		if len(l.EditablePlaceholders()) > 0 {
			return i, l
		}
	}
	t.Fatal("no layout with a placeholder found")
	return 0, nil
}

func TestLayoutPlaceholderGeometryRoundTrip(t *testing.T) {
	p, err := Open("testdata/test_slides.pptx")
	if err != nil {
		t.Fatal(err)
	}
	idx, layout := firstLayoutWithPlaceholder(t, p)
	ph := layout.EditablePlaceholders()[0]
	phType := ph.Type()

	ph.SetPosition(dml.Inches(1), dml.Inches(2))
	ph.SetSize(dml.Inches(3), dml.Inches(4))

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}

	p2, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	ph2 := p2.SlideLayouts()[idx].EditablePlaceholder(phType)
	if ph2 == nil {
		t.Fatalf("placeholder %q missing after round trip", phType)
	}
	x, y, ok := ph2.Position()
	if !ok || x != dml.Inches(1) || y != dml.Inches(2) {
		t.Errorf("position = (%v,%v) ok=%v", x, y, ok)
	}
	w, h, ok := ph2.Size()
	if !ok || w != dml.Inches(3) || h != dml.Inches(4) {
		t.Errorf("size = (%v,%v) ok=%v", w, h, ok)
	}
}

// Editing one layout's placeholder geometry leaves the master and the other
// layouts byte-identical to a clean save.
func TestLayoutPlaceholderGeometry_UntouchedByteIdentical(t *testing.T) {
	clean, err := Open("testdata/test_slides.pptx")
	if err != nil {
		t.Fatal(err)
	}
	cleanData, err := clean.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}

	edited, err := Open("testdata/test_slides.pptx")
	if err != nil {
		t.Fatal(err)
	}
	_, layout := firstLayoutWithPlaceholder(t, edited)
	layout.EditablePlaceholders()[0].SetPosition(dml.Inches(1), dml.Inches(1))
	editedData, err := edited.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}

	editedName := strings.TrimPrefix(layout.partName, "/")
	for _, name := range []string{
		"ppt/slideMasters/slideMaster1.xml",
		"ppt/slideLayouts/slideLayout1.xml",
		"ppt/slideLayouts/slideLayout11.xml",
	} {
		if name == editedName {
			continue
		}
		if !bytes.Equal(zipPart(t, cleanData, name), zipPart(t, editedData, name)) {
			t.Errorf("%s changed after editing a different layout's placeholder", name)
		}
	}
	if bytes.Equal(zipPart(t, cleanData, editedName), zipPart(t, editedData, editedName)) {
		t.Errorf("edited layout %s unchanged despite geometry edit", editedName)
	}
}

// --- Add layout ---

func TestAddLayoutRoundTrip(t *testing.T) {
	pres := Create()
	master := pres.SlideMasters()[0]
	before := len(master.Layouts())

	layout := master.AddLayout(LayoutTitleOnly)
	layout.SetName("Custom Title Only")
	if got := len(master.Layouts()); got != before+1 {
		t.Fatalf("layout count = %d, want %d", got, before+1)
	}

	slide := pres.AddSlideWithLayout(layout)
	slide.AddTextBox().TextFrame().SetText("hi")

	data, err := pres.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}

	p2, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	if got := len(p2.SlideMasters()[0].Layouts()); got != before+1 {
		t.Errorf("after reopen layout count = %d, want %d", got, before+1)
	}
	if p2.SlideMasters()[0].GetLayoutByName("Custom Title Only") == nil {
		t.Error("added layout not found after round trip")
	}
}

func TestAddLayoutOnOpenedDeckRoundTrip(t *testing.T) {
	p, err := Open("testdata/test_slides.pptx")
	if err != nil {
		t.Fatal(err)
	}
	master := p.SlideMasters()[0]
	before := len(master.Layouts())
	layout := master.AddLayout(LayoutBlank)
	layout.SetName("Injected Blank")

	data, err := p.SaveBytes()
	if err != nil {
		t.Fatal(err)
	}
	p2, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatal(err)
	}
	m2 := p2.SlideMasters()[0]
	if got := len(m2.Layouts()); got != before+1 {
		t.Fatalf("after reopen layout count = %d, want %d", got, before+1)
	}
	if m2.GetLayoutByName("Injected Blank") == nil {
		t.Error("injected layout not found after round trip")
	}
}
