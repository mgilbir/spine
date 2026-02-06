// Package dml tests for DrawingML media types from dml-main.xsd
package dml

import (
	"encoding/xml"
	"testing"
)

// TestDML_CT_AudioFile tests CT_AudioFile type (a:audioFile)
func TestDML_CT_AudioFile(t *testing.T) {
	var v AudioFile
	input := `<a:audioFile xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
		xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"
		r:link="rId1" contentType="audio/mp3"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Link != "rId1" {
		t.Errorf("Link = %q, want rId1", v.Link)
	}
	if v.ContentType != "audio/mp3" {
		t.Errorf("ContentType = %q, want audio/mp3", v.ContentType)
	}
}

// TestDML_CT_VideoFile tests CT_VideoFile type (a:videoFile)
func TestDML_CT_VideoFile(t *testing.T) {
	var v VideoFile
	input := `<a:videoFile xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
		xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"
		r:link="rId2" contentType="video/mp4"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Link != "rId2" {
		t.Errorf("Link = %q, want rId2", v.Link)
	}
	if v.ContentType != "video/mp4" {
		t.Errorf("ContentType = %q, want video/mp4", v.ContentType)
	}
}

// TestDML_CT_QuickTimeFile tests CT_QuickTimeFile type (a:quickTimeFile)
func TestDML_CT_QuickTimeFile(t *testing.T) {
	var v QuickTimeFile
	input := `<a:quickTimeFile xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
		xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"
		r:link="rId3"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Link != "rId3" {
		t.Errorf("Link = %q, want rId3", v.Link)
	}
}

// TestDML_CT_AudioCD tests CT_AudioCD type (a:audioCd)
func TestDML_CT_AudioCD(t *testing.T) {
	var v AudioCD
	input := `<a:audioCd xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<a:st track="1" time="0"/>
		<a:end track="1" time="180000"/>
	</a:audioCd>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.St == nil {
		t.Error("St is nil")
	}
	if v.End == nil {
		t.Error("End is nil")
	}
}

// TestDML_CT_AudioCDTime tests CT_AudioCDTime type (a:st, a:end)
func TestDML_CT_AudioCDTime(t *testing.T) {
	var v AudioCDTime
	input := `<a:st xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" track="5" time="30000"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Track != 5 {
		t.Errorf("Track = %d, want 5", v.Track)
	}
	if v.Time != 30000 {
		t.Errorf("Time = %d, want 30000", v.Time)
	}
}

// TestDML_CT_MediaBookmark tests CT_MediaBookmark type (a:bmk)
func TestDML_CT_MediaBookmark(t *testing.T) {
	var v MediaBookmark
	input := `<a:bmk xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" name="Chapter 1" time="60000"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.Name != "Chapter 1" {
		t.Errorf("Name = %q, want 'Chapter 1'", v.Name)
	}
	if v.Time != 60000 {
		t.Errorf("Time = %d, want 60000", v.Time)
	}
}

// TestDML_CT_MediaBookmarkList tests CT_MediaBookmarkList type (a:bmkLst)
func TestDML_CT_MediaBookmarkList(t *testing.T) {
	var v MediaBookmarkList
	input := `<a:bmkLst xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<a:bmk name="Start" time="0"/>
		<a:bmk name="Middle" time="60000"/>
		<a:bmk name="End" time="120000"/>
	</a:bmkLst>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if len(v.Bmk) != 3 {
		t.Errorf("Bmk length = %d, want 3", len(v.Bmk))
	}
}

// TestDML_CT_MediaTrim tests CT_MediaTrim type (a:trim)
func TestDML_CT_MediaTrim(t *testing.T) {
	var v MediaTrim
	input := `<a:trim xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" st="5000" end="120000"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.St != 5000 {
		t.Errorf("St = %d, want 5000", v.St)
	}
	if v.End != 120000 {
		t.Errorf("End = %d, want 120000", v.End)
	}
}

// TestDML_CT_MediaFade tests CT_MediaFade type (a:fade)
func TestDML_CT_MediaFade(t *testing.T) {
	var v MediaFade
	input := `<a:fade xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" in="1000" out="2000"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.In != 1000 {
		t.Errorf("In = %d, want 1000", v.In)
	}
	if v.Out != 2000 {
		t.Errorf("Out = %d, want 2000", v.Out)
	}
}

// TestDML_CNvAudioPr tests CT_NonVisualAudioProperties type (a:cNvAudioPr)
func TestDML_CNvAudioPr(t *testing.T) {
	var v CNvAudioPr
	input := `<a:cNvAudioPr xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" isPhoto="1"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if !v.IsPhoto {
		t.Error("IsPhoto should be true")
	}
}

// TestDML_CNvVideoPr tests CT_NonVisualVideoProperties type (a:cNvVideoPr)
func TestDML_CNvVideoPr(t *testing.T) {
	var v CNvVideoPr
	input := `<a:cNvVideoPr xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	// CNvVideoPr is an empty element
}

// TestDML_OleObject tests OleObject type
func TestDML_OleObject(t *testing.T) {
	var v OleObject
	input := `<oleObject xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"
		progId="Excel.Sheet.12" r:id="rId4" imgW="914400" imgH="457200"/>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.ProgId != "Excel.Sheet.12" {
		t.Errorf("ProgId = %q, want Excel.Sheet.12", v.ProgId)
	}
	if v.Id != "rId4" {
		t.Errorf("Id = %q, want rId4", v.Id)
	}
	if v.ImgW != 914400 {
		t.Errorf("ImgW = %d, want 914400", v.ImgW)
	}
}

// TestDML_CT_GraphicalObject tests CT_GraphicalObject type (a:graphic)
func TestDML_CT_GraphicalObject(t *testing.T) {
	var v Graphic
	input := `<a:graphic xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
		<a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/chart">
			<c:chart xmlns:c="http://schemas.openxmlformats.org/drawingml/2006/chart" r:id="rId1"/>
		</a:graphicData>
	</a:graphic>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.GraphicData == nil {
		t.Error("GraphicData is nil")
	}
}

// TestDML_CT_GraphicalObjectData tests CT_GraphicalObjectData type (a:graphicData)
func TestDML_CT_GraphicalObjectData(t *testing.T) {
	var v GraphicData
	input := `<a:graphicData xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main"
		uri="http://schemas.openxmlformats.org/drawingml/2006/table">
		<a:tbl><a:tblPr/><a:tblGrid/></a:tbl>
	</a:graphicData>`
	if err := xml.Unmarshal([]byte(input), &v); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if v.URI != "http://schemas.openxmlformats.org/drawingml/2006/table" {
		t.Errorf("URI = %q, want http://schemas.openxmlformats.org/drawingml/2006/table", v.URI)
	}
}
