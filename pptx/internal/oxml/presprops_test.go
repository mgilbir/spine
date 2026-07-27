package oxml

import (
	"encoding/xml"
	"testing"
)

func TestPresentationProperties_RoundTrip(t *testing.T) {
	xmlStr := `<presentationPr xmlns="http://schemas.openxmlformats.org/presentationml/2006/main">
  <showPr loop="true" showNarration="true" showAnimation="true" useTimings="true">
    <sldAll/>
  </showPr>
  <prnPr prnWhat="slides" clrMode="clr" scaleToFitPaper="true"/>
</presentationPr>`

	var pp PresentationProperties
	if err := xml.Unmarshal([]byte(xmlStr), &pp); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	out, err := xml.Marshal(&pp)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var pp2 PresentationProperties
	if err := xml.Unmarshal(out, &pp2); err != nil {
		t.Fatalf("Re-unmarshal failed: %v", err)
	}
}

func TestHtmlPublishProperties_RoundTrip(t *testing.T) {
	tests := []struct {
		name             string
		showSpeakerNotes bool
		pubBrowser       string
		title            string
	}{
		{"v4 browser", true, "v4", "My Presentation"},
		{"v3 browser", false, "v3", "Another Presentation"},
		{"v3v4 browser", true, "v3v4", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hpp := &HtmlPublishProperties{
				ShowSpeakerNotes: &tt.showSpeakerNotes,
				PubBrowser:       tt.pubBrowser,
				Title:            tt.title,
			}
			out, err := xml.Marshal(hpp)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var hpp2 HtmlPublishProperties
			if err := xml.Unmarshal(out, &hpp2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			// showSpeakerNotes is XSD default-TRUE, so an explicit false must
			// survive as a pointer rather than be deleted by omitempty (C526).
			if hpp2.ShowSpeakerNotes == nil || *hpp2.ShowSpeakerNotes != tt.showSpeakerNotes {
				t.Errorf("ShowSpeakerNotes = %v, want %v", hpp2.ShowSpeakerNotes, tt.showSpeakerNotes)
			}
			if hpp2.PubBrowser != tt.pubBrowser {
				t.Errorf("PubBrowser = %q, want %q", hpp2.PubBrowser, tt.pubBrowser)
			}
		})
	}
}

func TestWebProperties_RoundTrip(t *testing.T) {
	tests := []struct {
		name   string
		imgSz  string
		clr    string
	}{
		{"640x480", "screen640x480", "none"},
		{"1024x768", "screen1024x768", "browser"},
		{"1920x1200", "screen1920x1200", "presentationText"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			yes := true
			wp := &WebProperties{
				ShowAnimation:     true,
				ResizeGraphics:    &yes,
				AllowPng:          true,
				RelyOnVml:         false,
				OrganizeInFolders: &yes,
				UseLongFilenames:  &yes,
				ImgSz:             tt.imgSz,
				Encoding:          "utf-8",
				Clr:               tt.clr,
			}
			out, err := xml.Marshal(wp)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var wp2 WebProperties
			if err := xml.Unmarshal(out, &wp2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if wp2.ImgSz != tt.imgSz {
				t.Errorf("ImgSz = %q, want %q", wp2.ImgSz, tt.imgSz)
			}
			if wp2.Clr != tt.clr {
				t.Errorf("Clr = %q, want %q", wp2.Clr, tt.clr)
			}
		})
	}
}

func TestPrintProperties_RoundTrip(t *testing.T) {
	tests := []struct {
		name            string
		prnWhat         string
		clrMode         string
		hiddenSlides    bool
		scaleToFitPaper bool
		frameSlides     bool
	}{
		{"slides color", "slides", "clr", false, true, false},
		{"handouts bw", "handouts4", "bw", true, false, true},
		{"notes gray", "notes", "gray", false, false, false},
		{"outline", "outline", "clr", false, true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pp := &PrintProperties{
				PrnWhat:         tt.prnWhat,
				ClrMode:         tt.clrMode,
				HiddenSlides:    tt.hiddenSlides,
				ScaleToFitPaper: tt.scaleToFitPaper,
				FrameSlides:     tt.frameSlides,
			}
			out, err := xml.Marshal(pp)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var pp2 PrintProperties
			if err := xml.Unmarshal(out, &pp2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if pp2.PrnWhat != tt.prnWhat {
				t.Errorf("PrnWhat = %q, want %q", pp2.PrnWhat, tt.prnWhat)
			}
			if pp2.ClrMode != tt.clrMode {
				t.Errorf("ClrMode = %q, want %q", pp2.ClrMode, tt.clrMode)
			}
		})
	}
}

func TestShowProperties_RoundTrip(t *testing.T) {
	tests := []struct {
		name string
		xml  string
	}{
		{
			name: "loop presentation",
			xml: `<showPr xmlns="http://schemas.openxmlformats.org/presentationml/2006/main" loop="true" showNarration="true" showAnimation="true" useTimings="true">
  <sldAll/>
</showPr>`,
		},
		{
			name: "browse mode",
			xml: `<showPr xmlns="http://schemas.openxmlformats.org/presentationml/2006/main">
  <browse showScrollbar="true"/>
  <sldAll/>
</showPr>`,
		},
		{
			name: "kiosk mode",
			xml: `<showPr xmlns="http://schemas.openxmlformats.org/presentationml/2006/main">
  <kiosk restart="5000"/>
  <sldAll/>
</showPr>`,
		},
		{
			name: "slide range",
			xml: `<showPr xmlns="http://schemas.openxmlformats.org/presentationml/2006/main">
  <sldRg st="2" end="10"/>
</showPr>`,
		},
		{
			name: "custom show",
			xml: `<showPr xmlns="http://schemas.openxmlformats.org/presentationml/2006/main">
  <custShow id="1"/>
</showPr>`,
		},
		{
			name: "presenter view",
			xml: `<showPr xmlns="http://schemas.openxmlformats.org/presentationml/2006/main">
  <present/>
  <sldAll/>
</showPr>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var sp ShowProperties
			if err := xml.Unmarshal([]byte(tt.xml), &sp); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			out, err := xml.Marshal(&sp)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var sp2 ShowProperties
			if err := xml.Unmarshal(out, &sp2); err != nil {
				t.Fatalf("Re-unmarshal failed: %v", err)
			}
		})
	}
}

func TestShowInfoBrowse_RoundTrip(t *testing.T) {
	tests := []struct {
		name          string
		showScrollbar bool
	}{
		{"with scrollbar", true},
		{"without scrollbar", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sib := &ShowInfoBrowse{ShowScrollbar: &tt.showScrollbar}
			out, err := xml.Marshal(sib)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var sib2 ShowInfoBrowse
			if err := xml.Unmarshal(out, &sib2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			// showScrollbar is XSD default-TRUE (C526).
			if sib2.ShowScrollbar == nil || *sib2.ShowScrollbar != tt.showScrollbar {
				t.Errorf("ShowScrollbar = %v, want %v", sib2.ShowScrollbar, tt.showScrollbar)
			}
		})
	}
}

func TestShowInfoKiosk_RoundTrip(t *testing.T) {
	tests := []struct {
		name    string
		restart uint32
	}{
		{"5 seconds", 5000},
		{"10 seconds", 10000},
		{"no restart", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sik := &ShowInfoKiosk{Restart: tt.restart}
			out, err := xml.Marshal(sik)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var sik2 ShowInfoKiosk
			if err := xml.Unmarshal(out, &sik2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if sik2.Restart != tt.restart {
				t.Errorf("Restart = %d, want %d", sik2.Restart, tt.restart)
			}
		})
	}
}

func TestCustomShowRef_RoundTrip(t *testing.T) {
	ids := []uint32{0, 1, 5, 100}
	for _, id := range ids {
		t.Run("id", func(t *testing.T) {
			csr := &CustomShowRef{Id: id}
			out, err := xml.Marshal(csr)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var csr2 CustomShowRef
			if err := xml.Unmarshal(out, &csr2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if csr2.Id != id {
				t.Errorf("Id = %d, want %d", csr2.Id, id)
			}
		})
	}
}

func TestIndexRange_RoundTrip(t *testing.T) {
	tests := []struct {
		st  uint32
		end uint32
	}{
		{1, 10},
		{1, 1},
		{5, 20},
		{0, 100},
	}

	for _, tt := range tests {
		t.Run("range", func(t *testing.T) {
			ir := &IndexRange{St: tt.st, End: tt.end}
			out, err := xml.Marshal(ir)
			if err != nil {
				t.Fatalf("Marshal failed: %v", err)
			}

			var ir2 IndexRange
			if err := xml.Unmarshal(out, &ir2); err != nil {
				t.Fatalf("Unmarshal failed: %v", err)
			}

			if ir2.St != tt.st {
				t.Errorf("St = %d, want %d", ir2.St, tt.st)
			}
			if ir2.End != tt.end {
				t.Errorf("End = %d, want %d", ir2.End, tt.end)
			}
		})
	}
}

func TestColorMRU_RoundTrip(t *testing.T) {
	xmlStr := `<clrMru xmlns="http://schemas.openxmlformats.org/presentationml/2006/main" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
  <a:srgbClr val="FF0000"/>
  <a:srgbClr val="00FF00"/>
  <a:srgbClr val="0000FF"/>
</clrMru>`

	var cm ColorMRU
	if err := xml.Unmarshal([]byte(xmlStr), &cm); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	out, err := xml.Marshal(&cm)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var cm2 ColorMRU
	if err := xml.Unmarshal(out, &cm2); err != nil {
		t.Fatalf("Re-unmarshal failed: %v", err)
	}
}

func TestEmptyElement_RoundTrip(t *testing.T) {
	ee := &EmptyElement{}
	out, err := xml.Marshal(ee)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var ee2 EmptyElement
	if err := xml.Unmarshal(out, &ee2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
}

func TestShowInfoPresent_RoundTrip(t *testing.T) {
	sip := &ShowInfoPresent{}
	out, err := xml.Marshal(sip)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var sip2 ShowInfoPresent
	if err := xml.Unmarshal(out, &sip2); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}
}
