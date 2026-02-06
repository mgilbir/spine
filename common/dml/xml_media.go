// Package dml provides DrawingML XML media types from dml-main.xsd.
// These types handle audio, video, and OLE object embedding.
package dml

// AudioFile represents CT_AudioFile (a:audioFile)
type AudioFile struct {
	Link      string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships link,attr,omitempty"`
	ContentType string `xml:"contentType,attr,omitempty"`
	ExtLst    *ExtLst `xml:"http://schemas.openxmlformats.org/drawingml/2006/main extLst,omitempty"`
}

// VideoFile represents CT_VideoFile (a:videoFile)
type VideoFile struct {
	Link      string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships link,attr,omitempty"`
	ContentType string `xml:"contentType,attr,omitempty"`
	ExtLst    *ExtLst `xml:"http://schemas.openxmlformats.org/drawingml/2006/main extLst,omitempty"`
}

// QuickTimeFile represents CT_QuickTimeFile (a:quickTimeFile)
type QuickTimeFile struct {
	Link   string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships link,attr,omitempty"`
	ExtLst *ExtLst `xml:"http://schemas.openxmlformats.org/drawingml/2006/main extLst,omitempty"`
}

// AudioCD represents CT_AudioCD (a:audioCd)
type AudioCD struct {
	St     *AudioCDTime `xml:"http://schemas.openxmlformats.org/drawingml/2006/main st,omitempty"`
	End    *AudioCDTime `xml:"http://schemas.openxmlformats.org/drawingml/2006/main end,omitempty"`
	ExtLst *ExtLst      `xml:"http://schemas.openxmlformats.org/drawingml/2006/main extLst,omitempty"`
}

// AudioCDTime represents CT_AudioCDTime (a:st, a:end for audio CD)
type AudioCDTime struct {
	Track uint32 `xml:"track,attr"`
	Time  uint32 `xml:"time,attr,omitempty"`
}

// NvAudioPr represents non-visual audio properties
type NvAudioPr struct {
	IsPhoto bool `xml:"isPhoto,attr,omitempty"`
}

// CNvAudioPr represents CT_NonVisualAudioProperties (a:cNvAudioPr)
type CNvAudioPr struct {
	IsPhoto bool `xml:"isPhoto,attr,omitempty"`
}

// NvVideoPr represents non-visual video properties
type NvVideoPr struct{}

// CNvVideoPr represents CT_NonVisualVideoProperties (a:cNvVideoPr)
type CNvVideoPr struct{}

// MediaBookmark represents CT_MediaBookmark (a:bmk)
type MediaBookmark struct {
	Name string `xml:"name,attr,omitempty"`
	Time uint64 `xml:"time,attr"`
}

// MediaBookmarkList represents CT_MediaBookmarkList (a:bmkLst)
type MediaBookmarkList struct {
	Bmk []*MediaBookmark `xml:"http://schemas.openxmlformats.org/drawingml/2006/main bmk,omitempty"`
}

// MediaTrim represents CT_MediaTrim (a:trim)
type MediaTrim struct {
	St  uint64 `xml:"st,attr,omitempty"`
	End uint64 `xml:"end,attr,omitempty"`
}

// MediaFade represents CT_MediaFade (a:fade)
type MediaFade struct {
	In  uint64 `xml:"in,attr,omitempty"`
	Out uint64 `xml:"out,attr,omitempty"`
}

// OleObject represents CT_OleObject for OLE embedding
type OleObject struct {
	ProgId   string `xml:"progId,attr,omitempty"`
	Link     string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships link,attr,omitempty"`
	Id       string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships id,attr,omitempty"`
	ImgW     int64  `xml:"imgW,attr,omitempty"`
	ImgH     int64  `xml:"imgH,attr,omitempty"`
	UpdateAutomatic bool `xml:"updateAutomatic,attr,omitempty"`
}

// Graphic represents CT_GraphicalObject (a:graphic)
type Graphic struct {
	GraphicData *GraphicData `xml:"http://schemas.openxmlformats.org/drawingml/2006/main graphicData,omitempty"`
}

// GraphicData represents CT_GraphicalObjectData (a:graphicData)
type GraphicData struct {
	URI      string `xml:"uri,attr"`
	InnerXML []byte `xml:",innerxml"`
}
