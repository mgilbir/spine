package oxml

import (
	"encoding/xml"
	"testing"
)

func TestViewProperties_RoundTrip(t *testing.T) {
	tests := []struct {
		name string
		xml  string
	}{
		{
			name: "basic view properties",
			xml: `<viewPr xmlns="http://schemas.openxmlformats.org/presentationml/2006/main" lastView="sldView" showComments="true">
  <normalViewPr/>
</viewPr>`,
		},
		{
			name: "slide sorter view",
			xml: `<viewPr xmlns="http://schemas.openxmlformats.org/presentationml/2006/main" lastView="sldSorterView">
  <sorterViewPr showFormatting="true"/>
</viewPr>`,
		},
		{
			name: "notes view",
			xml: `<viewPr xmlns="http://schemas.openxmlformats.org/presentationml/2006/main" lastView="notesView"/>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var vp ViewProperties
			if err := xml.Unmarshal([]byte(tt.xml), &vp); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			out, err := xml.Marshal(&vp)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var vp2 ViewProperties
			if err := xml.Unmarshal(out, &vp2); err != nil {
				t.Fatalf("Re-unmarshal failed: %v", err)
			}

			if vp2.LastView != vp.LastView {
				t.Errorf("LastView = %q, want %q", vp2.LastView, vp.LastView)
			}
		})
	}
}

func TestNormalViewProperties_RoundTrip(t *testing.T) {
	tests := []struct {
		name string
		xml  string
	}{
		{
			name: "basic normal view",
			xml:  `<normalViewPr xmlns="http://schemas.openxmlformats.org/presentationml/2006/main" showOutlineIcons="true" snapVertSplitter="true"/>`,
		},
		{
			name: "with bar states",
			xml:  `<normalViewPr xmlns="http://schemas.openxmlformats.org/presentationml/2006/main" vertBarState="restored" horzBarState="minimized"/>`,
		},
		{
			name: "with restored portions",
			xml: `<normalViewPr xmlns="http://schemas.openxmlformats.org/presentationml/2006/main" preferSingleView="true">
  <restoredLeft sz="15620" autoAdjust="true"/>
  <restoredTop sz="94660" autoAdjust="false"/>
</normalViewPr>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var nvp NormalViewProperties
			if err := xml.Unmarshal([]byte(tt.xml), &nvp); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			out, err := xml.Marshal(&nvp)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var nvp2 NormalViewProperties
			if err := xml.Unmarshal(out, &nvp2); err != nil {
				t.Fatalf("Re-unmarshal failed: %v", err)
			}
		})
	}
}

func TestNormalViewPortion_RoundTrip(t *testing.T) {
	tests := []struct {
		name       string
		sz         int32
		autoAdjust bool
	}{
		{"small with auto", 10000, true},
		{"large without auto", 90000, false},
		{"zero", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			nvp := &NormalViewPortion{Sz: tt.sz, AutoAdjust: tt.autoAdjust}
			out, err := xml.Marshal(nvp)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var nvp2 NormalViewPortion
			if err := xml.Unmarshal(out, &nvp2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if nvp2.Sz != tt.sz {
				t.Errorf("Sz = %d, want %d", nvp2.Sz, tt.sz)
			}
			if nvp2.AutoAdjust != tt.autoAdjust {
				t.Errorf("AutoAdjust = %v, want %v", nvp2.AutoAdjust, tt.autoAdjust)
			}
		})
	}
}

func TestSlideViewProperties_RoundTrip(t *testing.T) {
	xmlStr := `<slideViewPr xmlns="http://schemas.openxmlformats.org/presentationml/2006/main">
  <cSldViewPr snapToGrid="true" snapToObjects="true" showGuides="true"/>
</slideViewPr>`

	var svp SlideViewProperties
	if err := xml.Unmarshal([]byte(xmlStr), &svp); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	out, err := xml.Marshal(&svp)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var svp2 SlideViewProperties
	if err := xml.Unmarshal(out, &svp2); err != nil {
		t.Fatalf("Re-unmarshal failed: %v", err)
	}
}

func TestCommonSlideViewProperties_RoundTrip(t *testing.T) {
	tests := []struct {
		name          string
		snapToGrid    bool
		snapToObjects bool
		showGuides    bool
	}{
		{"all true", true, true, true},
		{"all false", false, false, false},
		{"mixed", true, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			csvp := &CommonSlideViewProperties{
				SnapToGrid:    tt.snapToGrid,
				SnapToObjects: tt.snapToObjects,
				ShowGuides:    tt.showGuides,
			}
			out, err := xml.Marshal(csvp)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var csvp2 CommonSlideViewProperties
			if err := xml.Unmarshal(out, &csvp2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if csvp2.SnapToGrid != tt.snapToGrid {
				t.Errorf("SnapToGrid = %v, want %v", csvp2.SnapToGrid, tt.snapToGrid)
			}
			if csvp2.SnapToObjects != tt.snapToObjects {
				t.Errorf("SnapToObjects = %v, want %v", csvp2.SnapToObjects, tt.snapToObjects)
			}
			if csvp2.ShowGuides != tt.showGuides {
				t.Errorf("ShowGuides = %v, want %v", csvp2.ShowGuides, tt.showGuides)
			}
		})
	}
}

func TestCommonViewProperties_RoundTrip(t *testing.T) {
	xmlStr := `<cViewPr xmlns="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" varScale="true">
  <scale>
    <a:sx n="100" d="100"/>
    <a:sy n="100" d="100"/>
  </scale>
</cViewPr>`

	var cvp CommonViewProperties
	if err := xml.Unmarshal([]byte(xmlStr), &cvp); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if !cvp.VarScale {
		t.Error("VarScale should be true")
	}

	out, err := xml.Marshal(&cvp)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var cvp2 CommonViewProperties
	if err := xml.Unmarshal(out, &cvp2); err != nil {
		t.Fatalf("Re-unmarshal failed: %v", err)
	}
}

func TestScalePoint_RoundTrip(t *testing.T) {
	xmlStr := `<scale xmlns="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <a:sx n="75" d="100"/>
  <a:sy n="75" d="100"/>
</scale>`

	var sp ScalePoint
	if err := xml.Unmarshal([]byte(xmlStr), &sp); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	out, err := xml.Marshal(&sp)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var sp2 ScalePoint
	if err := xml.Unmarshal(out, &sp2); err != nil {
		t.Fatalf("Re-unmarshal failed: %v", err)
	}
}

func TestRatio_RoundTrip(t *testing.T) {
	tests := []struct {
		n int32
		d int32
	}{
		{100, 100},
		{75, 100},
		{150, 100},
		{1, 2},
	}

	for _, tt := range tests {
		t.Run("ratio", func(t *testing.T) {
			r := &Ratio{N: tt.n, D: tt.d}
			out, err := xml.Marshal(r)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var r2 Ratio
			if err := xml.Unmarshal(out, &r2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if r2.N != tt.n {
				t.Errorf("N = %d, want %d", r2.N, tt.n)
			}
			if r2.D != tt.d {
				t.Errorf("D = %d, want %d", r2.D, tt.d)
			}
		})
	}
}

func TestGuideList_RoundTrip(t *testing.T) {
	xmlStr := `<guideLst xmlns="http://schemas.openxmlformats.org/presentationml/2006/main">
  <guide orient="horz" pos="2160"/>
  <guide orient="vert" pos="2880"/>
</guideLst>`

	var gl GuideList
	if err := xml.Unmarshal([]byte(xmlStr), &gl); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if len(gl.Guide) != 2 {
		t.Errorf("Expected 2 guides, got %d", len(gl.Guide))
	}

	out, err := xml.Marshal(&gl)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var gl2 GuideList
	if err := xml.Unmarshal(out, &gl2); err != nil {
		t.Fatalf("Re-unmarshal failed: %v", err)
	}
}

func TestGuide_RoundTrip(t *testing.T) {
	tests := []struct {
		orient string
		pos    int32
	}{
		{"horz", 2160},
		{"vert", 2880},
		{"horz", 0},
		{"vert", -1000},
	}

	for _, tt := range tests {
		t.Run(tt.orient, func(t *testing.T) {
			g := &Guide{Orient: tt.orient, Pos: tt.pos}
			out, err := xml.Marshal(g)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var g2 Guide
			if err := xml.Unmarshal(out, &g2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if g2.Orient != tt.orient {
				t.Errorf("Orient = %q, want %q", g2.Orient, tt.orient)
			}
			if g2.Pos != tt.pos {
				t.Errorf("Pos = %d, want %d", g2.Pos, tt.pos)
			}
		})
	}
}

func TestOutlineViewProperties_RoundTrip(t *testing.T) {
	xmlStr := `<outlineViewPr xmlns="http://schemas.openxmlformats.org/presentationml/2006/main">
  <cViewPr varScale="false"/>
  <sldLst>
    <sld xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships" r:id="rId2" collapse="true"/>
  </sldLst>
</outlineViewPr>`

	var ovp OutlineViewProperties
	if err := xml.Unmarshal([]byte(xmlStr), &ovp); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	out, err := xml.Marshal(&ovp)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var ovp2 OutlineViewProperties
	if err := xml.Unmarshal(out, &ovp2); err != nil {
		t.Fatalf("Re-unmarshal failed: %v", err)
	}
}

func TestOutlineViewSlideEntry_RoundTrip(t *testing.T) {
	tests := []struct {
		id       string
		collapse bool
	}{
		{"rId1", true},
		{"rId2", false},
	}

	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			ovse := &OutlineViewSlideEntry{Id: tt.id, Collapse: tt.collapse}
			out, err := xml.Marshal(ovse)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var ovse2 OutlineViewSlideEntry
			if err := xml.Unmarshal(out, &ovse2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if ovse2.Collapse != tt.collapse {
				t.Errorf("Collapse = %v, want %v", ovse2.Collapse, tt.collapse)
			}
		})
	}
}

func TestNotesTextViewProperties_RoundTrip(t *testing.T) {
	xmlStr := `<notesTextViewPr xmlns="http://schemas.openxmlformats.org/presentationml/2006/main">
  <cViewPr varScale="true"/>
</notesTextViewPr>`

	var ntvp NotesTextViewProperties
	if err := xml.Unmarshal([]byte(xmlStr), &ntvp); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	out, err := xml.Marshal(&ntvp)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var ntvp2 NotesTextViewProperties
	if err := xml.Unmarshal(out, &ntvp2); err != nil {
		t.Fatalf("Re-unmarshal failed: %v", err)
	}
}

func TestSorterViewProperties_RoundTrip(t *testing.T) {
	tests := []struct {
		name           string
		showFormatting bool
	}{
		{"show formatting", true},
		{"hide formatting", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svp := &SorterViewProperties{ShowFormatting: tt.showFormatting}
			out, err := xml.Marshal(svp)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var svp2 SorterViewProperties
			if err := xml.Unmarshal(out, &svp2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if svp2.ShowFormatting != tt.showFormatting {
				t.Errorf("ShowFormatting = %v, want %v", svp2.ShowFormatting, tt.showFormatting)
			}
		})
	}
}

func TestNotesViewProperties_RoundTrip(t *testing.T) {
	xmlStr := `<notesViewPr xmlns="http://schemas.openxmlformats.org/presentationml/2006/main">
  <cSldViewPr snapToGrid="true"/>
</notesViewPr>`

	var nvp NotesViewProperties
	if err := xml.Unmarshal([]byte(xmlStr), &nvp); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	out, err := xml.Marshal(&nvp)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var nvp2 NotesViewProperties
	if err := xml.Unmarshal(out, &nvp2); err != nil {
		t.Fatalf("Re-unmarshal failed: %v", err)
	}
}
