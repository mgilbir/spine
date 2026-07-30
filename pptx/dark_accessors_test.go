package pptx

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mgilbir/spine/common/dml"
	coxml "github.com/mgilbir/spine/common/oxml"
	"github.com/mgilbir/spine/opc"
	"github.com/mgilbir/spine/pptx/internal/oxml"
)

// tableNodeText collects the text of every cell in a parsed a:tbl, so an
// assertion can look at what SyncXML actually wrote into the source frame.
func tableNodeText(t *testing.T, tbl *oxml.ATable) string {
	t.Helper()
	var sb strings.Builder
	for _, tr := range tbl.Tr {
		for _, tc := range tr.Tc {
			if tc == nil || tc.TxBody == nil {
				continue
			}
			for _, para := range tc.TxBody.P {
				for _, r := range para.R {
					sb.WriteString(r.T)
					sb.WriteString("|")
				}
			}
		}
	}
	return sb.String()
}

// TestPlaceholderIsPicture covers PlaceholderShape.IsPicture alongside its two
// siblings, so a predicate that tested the wrong constant (or that shared a
// case arm with IsTitle/IsBody) fails. The table is over every placeholder type
// the package declares, with exactly one expected true per predicate group.
func TestPlaceholderIsPicture(t *testing.T) {
	all := []PlaceholderType{
		PlaceholderTitle, PlaceholderBody, PlaceholderCenteredTitle,
		PlaceholderSubtitle, PlaceholderDateTime, PlaceholderSlideNumber,
		PlaceholderFooter, PlaceholderHeader, PlaceholderObject,
		PlaceholderChart, PlaceholderTable, PlaceholderClipArt,
		PlaceholderDiagram, PlaceholderMedia, PlaceholderSlideImage,
		PlaceholderPicture,
	}
	wantPicture := map[PlaceholderType]bool{PlaceholderPicture: true}
	wantTitle := map[PlaceholderType]bool{PlaceholderTitle: true, PlaceholderCenteredTitle: true}
	wantBody := map[PlaceholderType]bool{PlaceholderBody: true, PlaceholderObject: true}

	for _, pt := range all {
		ph := NewPlaceholderShape(pt)
		if got := ph.IsPicture(); got != wantPicture[pt] {
			t.Errorf("%q: IsPicture() = %v, want %v", pt, got, wantPicture[pt])
		}
		if got := ph.IsTitle(); got != wantTitle[pt] {
			t.Errorf("%q: IsTitle() = %v, want %v", pt, got, wantTitle[pt])
		}
		if got := ph.IsBody(); got != wantBody[pt] {
			t.Errorf("%q: IsBody() = %v, want %v", pt, got, wantBody[pt])
		}
		// The three predicates partition the types they claim: none may be
		// true for two groups at once.
		n := 0
		for _, b := range []bool{ph.IsPicture(), ph.IsTitle(), ph.IsBody()} {
			if b {
				n++
			}
		}
		if n > 1 {
			t.Errorf("%q matches more than one of IsPicture/IsTitle/IsBody", pt)
		}
	}

	// A picture placeholder is the only one SetImage accepts; the predicate and
	// the setter must agree on which type that is.
	pic := NewPlaceholderShape(PlaceholderPicture)
	if !pic.IsPicture() {
		t.Fatal("PlaceholderPicture.IsPicture() = false")
	}
	if err := NewPlaceholderShape(PlaceholderBody).SetImage("nonexistent.png"); err != ErrNotPicturePlaceholder {
		t.Errorf("SetImage on a body placeholder = %v, want ErrNotPicturePlaceholder", err)
	}
}

// TestSlideMasterHasBackground drives the master background through its three
// states. A HasBackground that only checked for a non-nil p:bg (rather than an
// actual fill) would report true for a background element with no properties,
// and one that never cleared would stay true after ClearBackground.
func TestSlideMasterHasBackground(t *testing.T) {
	p := Create()
	sm := p.SlideMasters()[0]

	if sm.HasBackground() {
		t.Error("a freshly created master reports a background")
	}

	sm.SetBackgroundFill(dml.NewSolidFill(dml.NewRGB(0x12, 0x34, 0x56).ToColor()))
	if !sm.HasBackground() {
		t.Fatal("HasBackground() = false after SetBackgroundFill")
	}
	col, ok := sm.BackgroundColor()
	if !ok || col.RGB != dml.NewRGB(0x12, 0x34, 0x56) {
		t.Errorf("BackgroundColor() = (%+v, %v), want 123456", col, ok)
	}

	sm.ClearBackground()
	if sm.HasBackground() {
		t.Error("HasBackground() = true after ClearBackground")
	}
	if _, ok := sm.BackgroundColor(); ok {
		t.Error("BackgroundColor() reported a colour after ClearBackground")
	}

	// A p:bg carrying a bgPr with no fill at all is not a background: a
	// HasBackground that only nil-checked the element would say true here.
	sm.masterXML.CSld.Bg = &oxml.Background{BgPr: &oxml.BackgroundProps{}}
	if sm.HasBackground() {
		t.Error("an empty p:bgPr was reported as a background")
	}
	// ...but an explicit noFill IS a background.
	sm.masterXML.CSld.Bg = &oxml.Background{BgPr: &oxml.BackgroundProps{NoFill: &dml.NoFillXML{}}}
	if !sm.HasBackground() {
		t.Error("an explicit a:noFill background was reported as absent")
	}

	// A master with no XML must not panic.
	sm.masterXML = nil
	if sm.HasBackground() {
		t.Error("a master with no XML reports a background")
	}
}

// TestRemoveCustomProperty covers the remove path: a hit removes exactly the
// named property and reports true, a miss changes nothing and reports false,
// and the removal reaches docProps/custom.xml.
func TestRemoveCustomProperty(t *testing.T) {
	p := Create()

	// Removing from a deck that has no custom properties at all.
	if p.RemoveCustomProperty("anything") {
		t.Error("RemoveCustomProperty on a deck with no properties returned true")
	}

	for name, val := range map[string]any{"Alpha": "one", "Beta": "two", "Gamma": "three"} {
		if err := p.SetCustomProperty(name, val); err != nil {
			t.Fatalf("SetCustomProperty(%q): %v", name, err)
		}
	}

	if p.RemoveCustomProperty("NoSuchProperty") {
		t.Error("RemoveCustomProperty on an absent name returned true")
	}
	if got := len(p.CustomProperties()); got != 3 {
		t.Errorf("a failed removal changed the property count: %d, want 3", got)
	}

	if !p.RemoveCustomProperty("Beta") {
		t.Fatal("RemoveCustomProperty(\"Beta\") returned false")
	}
	props := p.CustomProperties()
	if _, still := props["Beta"]; still {
		t.Error("Beta survived RemoveCustomProperty")
	}
	// The siblings must be untouched: a remove that cleared the whole set, or
	// removed by position rather than by name, fails here.
	if props["Alpha"] != "one" || props["Gamma"] != "three" {
		t.Errorf("neighbouring properties were disturbed: %v", props)
	}
	if len(props) != 2 {
		t.Errorf("CustomProperties() = %d entries, want 2", len(props))
	}

	// Removing the same name twice reports false the second time.
	if p.RemoveCustomProperty("Beta") {
		t.Error("a second RemoveCustomProperty(\"Beta\") returned true")
	}

	// The removal must reach the saved part.
	rp := saveReopen(t, p)
	back := rp.CustomProperties()
	if _, still := back["Beta"]; still {
		t.Error("Beta reappeared after save/reopen")
	}
	if back["Alpha"] != "one" || back["Gamma"] != "three" {
		t.Errorf("surviving properties after reopen = %v", back)
	}
}

// TestSmartArtSetBounds covers SmartArt.SetBounds: it must move the created
// frame, be a no-op (not a panic) for a diagram read back from a file, and use
// x/y/width/height in the documented order — the four values are all different
// so a transposed argument fails.
func TestSmartArtSetBounds(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	sa := s.AddSmartArt(SmartArtProcess, &SmartArtNode{Text: "one"}, &SmartArtNode{Text: "two"})
	if sa == nil {
		t.Fatal("AddSmartArt returned nil")
	}
	if sa.frame == nil {
		t.Fatal("AddSmartArt produced no frame")
	}

	// The default placement must differ from the values below, so "SetBounds
	// did nothing" is distinguishable from "SetBounds worked".
	dx, dy := sa.frame.Position()
	dw, dh := sa.frame.Size()

	const (
		wantX = int64(101)
		wantY = int64(202)
		wantW = int64(30303)
		wantH = int64(404)
	)
	if int64(dx) == wantX && int64(dy) == wantY && int64(dw) == wantW && int64(dh) == wantH {
		t.Fatal("the default bounds equal the values under test")
	}

	sa.SetBounds(wantX, wantY, wantW, wantH)
	if x, y := sa.frame.Position(); int64(x) != wantX || int64(y) != wantY {
		t.Errorf("Position() = (%d,%d), want (%d,%d)", x, y, wantX, wantY)
	}
	if w, h := sa.frame.Size(); int64(w) != wantW || int64(h) != wantH {
		t.Errorf("Size() = (%d,%d), want (%d,%d)", w, h, wantW, wantH)
	}

	// The bounds must reach the graphic frame in the saved part.
	data, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	slideXML := string(zipPart(t, data, "ppt/slides/slide1.xml"))
	if !strings.Contains(slideXML, `x="101" y="202"`) {
		t.Errorf("the diagram offset did not reach the part:\n%s", slideXML)
	}
	if !strings.Contains(slideXML, `cx="30303" cy="404"`) {
		t.Errorf("the diagram extent did not reach the part:\n%s", slideXML)
	}

	// A diagram read back from a file has no created frame; SetBounds is a
	// documented no-op there and must not panic.
	rp := saveReopen(t, p)
	rs, err := rp.Slide(0)
	if err != nil {
		t.Fatalf("Slide(0): %v", err)
	}
	read := rs.SmartArt()
	if len(read) != 1 {
		t.Fatalf("SmartArt() = %d, want 1", len(read))
	}
	if read[0].frame != nil {
		t.Fatal("a parsed diagram unexpectedly carries a created frame")
	}
	read[0].SetBounds(1, 2, 3, 4)

	// A nil receiver is likewise a no-op.
	var nilSA *SmartArt
	nilSA.SetBounds(1, 2, 3, 4)
}

// TestTableSyncXML covers Table.SyncXML on both sides of its contract: false
// (and no effect) for a table with no source frame, true for one parsed from a
// file, and the flushed edit visible in the frame it reports having written.
func TestTableSyncXML(t *testing.T) {
	p := Create()
	s := p.AddSlide()
	tbl := s.AddTable(2, 2)
	tbl.Cell(0, 0).SetText("original")
	tbl.Cell(1, 1).SetText("corner")

	if tbl.SyncXML() {
		t.Error("SyncXML() = true for a table created through AddTable (it has no source frame)")
	}

	rp := saveReopen(t, p)
	rs, err := rp.Slide(0)
	if err != nil {
		t.Fatalf("Slide(0): %v", err)
	}
	var back *Table
	for _, sh := range rs.Shapes() {
		if tt, ok := sh.(*Table); ok {
			back = tt
		}
	}
	if back == nil {
		t.Fatal("no table on the reopened slide")
	}
	if back.sourceFrame == nil {
		t.Fatal("the reopened table carries no source frame")
	}
	if got := back.Cell(0, 0).Text(); got != "original" {
		t.Fatalf("reopened cell text = %q, want original", got)
	}

	back.Cell(0, 0).SetText("edited")
	if !back.SyncXML() {
		t.Fatal("SyncXML() = false for a table parsed from a file")
	}
	// The flush must have rewritten the frame SyncXML claims to write.
	node := back.sourceFrame.Graphic.GraphicData.Table
	if node == nil {
		t.Fatal("SyncXML left no a:tbl on the source frame")
	}
	flushed := tableNodeText(t, node)
	if !strings.Contains(flushed, "edited") {
		t.Errorf("the edit did not reach the source frame; text = %q", flushed)
	}
	if strings.Contains(flushed, "original") {
		t.Errorf("the pre-edit text survived the flush; text = %q", flushed)
	}
	// Untouched cells must survive the regeneration.
	if !strings.Contains(flushed, "corner") {
		t.Errorf("an untouched cell was lost by SyncXML; text = %q", flushed)
	}
}

// TestActiveXControls injects an ActiveX control part plus its persistence
// binary and checks the reader resolves both, in part-name order, with the
// class id and persistence mode read from the control part.
func TestActiveXControls(t *testing.T) {
	p, err := Open("testdata/test.pptx")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if got := p.ActiveXControls(); len(got) != 0 {
		t.Fatalf("a deck with no ActiveX reported %d controls", len(got))
	}

	const (
		classA = "{8BD21D40-EC42-11CE-9E0D-00AA006002F3}"
		classB = "{8BD21D20-EC42-11CE-9E0D-00AA006002F3}"
	)
	ctrlA := []byte(`<?xml version="1.0"?><ax:ocx xmlns:ax="http://schemas.microsoft.com/office/2006/activeX" ax:classid="` + classA + `" ax:persistence="persistPropertyBag"/>`)
	ctrlB := []byte(`<?xml version="1.0"?><ax:ocx xmlns:ax="http://schemas.microsoft.com/office/2006/activeX" ax:classid="` + classB + `" ax:persistence="persistStream"/>`)
	binA := []byte("ACTIVEX-BINARY-A")
	binB := []byte("ACTIVEX-BINARY-B")

	// Control 1 resolves its binary through an explicit relationship whose
	// target is deliberately NOT the sibling name, so the relationship path and
	// the sibling fallback are distinguishable.
	p.otherParts["/ppt/activeX/activeX1.xml"] = &coxml.RawPart{ContentType: contentTypeActiveXXML, Data: ctrlA}
	p.otherParts["/ppt/activeX/persistedA.bin"] = &coxml.RawPart{ContentType: contentTypeActiveXBin, Data: binA}
	p.relationships["/ppt/activeX/activeX1.xml"] = []*opc.Relationship{{
		ID:         "rId1",
		Type:       "http://schemas.microsoft.com/office/2006/relationships/activeXControlBinary",
		Target:     "persistedA.bin",
		TargetMode: opc.TargetModeInternal,
	}}
	// Control 2 has no relationship and must fall back to its sibling .bin.
	p.otherParts["/ppt/activeX/activeX2.xml"] = &coxml.RawPart{ContentType: contentTypeActiveXXML, Data: ctrlB}
	p.otherParts["/ppt/activeX/activeX2.bin"] = &coxml.RawPart{ContentType: contentTypeActiveXBin, Data: binB}

	got := p.ActiveXControls()
	if len(got) != 2 {
		t.Fatalf("ActiveXControls() = %d controls, want 2", len(got))
	}
	// Ordered by part name for determinism.
	if got[0].Name != "/ppt/activeX/activeX1.xml" || got[1].Name != "/ppt/activeX/activeX2.xml" {
		t.Errorf("controls out of order: %q then %q", got[0].Name, got[1].Name)
	}

	if got[0].ClassID != classA {
		t.Errorf("control 1 ClassID = %q, want %q", got[0].ClassID, classA)
	}
	if got[0].Persistence != "persistPropertyBag" {
		t.Errorf("control 1 Persistence = %q, want persistPropertyBag", got[0].Persistence)
	}
	if got[0].BinaryName != "/ppt/activeX/persistedA.bin" {
		t.Errorf("control 1 BinaryName = %q, want the relationship target", got[0].BinaryName)
	}
	if !bytes.Equal(got[0].BinaryData, binA) {
		t.Errorf("control 1 BinaryData = %q, want %q", got[0].BinaryData, binA)
	}
	if !bytes.Equal(got[0].Data, ctrlA) {
		t.Error("control 1 Data is not the control part verbatim")
	}

	// The second control's metadata must be its own, not the first's: a reader
	// that hoisted the first match would fail here.
	if got[1].ClassID != classB {
		t.Errorf("control 2 ClassID = %q, want %q", got[1].ClassID, classB)
	}
	if got[1].Persistence != "persistStream" {
		t.Errorf("control 2 Persistence = %q, want persistStream", got[1].Persistence)
	}
	if got[1].BinaryName != "/ppt/activeX/activeX2.bin" {
		t.Errorf("control 2 BinaryName = %q, want the sibling .bin", got[1].BinaryName)
	}
	if !bytes.Equal(got[1].BinaryData, binB) {
		t.Errorf("control 2 BinaryData = %q, want %q", got[1].BinaryData, binB)
	}

	// A control part with no binary at all reports an empty BinaryName rather
	// than borrowing a neighbour's.
	p.otherParts["/ppt/activeX/activeX3.xml"] = &coxml.RawPart{ContentType: contentTypeActiveXXML, Data: ctrlB}
	got = p.ActiveXControls()
	if len(got) != 3 {
		t.Fatalf("ActiveXControls() = %d, want 3", len(got))
	}
	if got[2].BinaryName != "" || got[2].BinaryData != nil {
		t.Errorf("a control with no binary reported %q / %q", got[2].BinaryName, got[2].BinaryData)
	}
}
