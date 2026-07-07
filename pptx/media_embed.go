package pptx

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"

	"github.com/mgilbir/spine/common/dml"
	coxml "github.com/mgilbir/spine/common/oxml"
	xmlb "github.com/mgilbir/spine/common/xml"
	"github.com/mgilbir/spine/opc"
	"github.com/mgilbir/spine/pptx/internal/oxml"
)

type mediaKind int

const (
	mediaVideo mediaKind = iota
	mediaAudio
)

// Default on-slide media size when the caller set none (4:3, 4 inches wide).
const (
	defaultMediaWidth  = dml.EMU(4 * 914400)
	defaultMediaHeight = dml.EMU(3 * 914400)
)

// mediaShapeOf returns the embedded mediaShape of a Video or Audio shape (and
// its kind name for error messages), or nil for any other shape.
func mediaShapeOf(shape Shape) (*mediaShape, string) {
	switch sh := shape.(type) {
	case *Video:
		return &sh.mediaShape, "video"
	case *Audio:
		return &sh.mediaShape, "audio"
	}
	return nil, ""
}

// resolveMediaContentTypes fills in the content type of media shapes the
// caller added without one, by sniffing the leading magic bytes. Called before
// any shape sync so the media part is never stored under an unregistered
// extension.
func (s *Slide) resolveMediaContentTypes() {
	for _, shape := range s.shapes {
		m, _ := mediaShapeOf(shape)
		if m == nil || m.mediaRelID != "" || m.contentType != "" {
			continue
		}
		m.contentType = sniffMediaContentType(m.mediaData)
	}
}

// validateMediaShapes rejects media that cannot be embedded as a valid OPC
// part: no bytes at all, or no content type (neither declared by the caller
// nor recognizable from the data). Without this check an empty content type
// produced a /ppt/media/mediaN.bin part absent from [Content_Types].xml — an
// invalid package that PowerPoint asks to repair.
func (s *Slide) validateMediaShapes() error {
	s.resolveMediaContentTypes()
	for _, shape := range s.shapes {
		m, kind := mediaShapeOf(shape)
		if m == nil || m.mediaRelID != "" {
			continue
		}
		if len(m.mediaData) == 0 {
			return fmt.Errorf("pptx: slide %d: %s has no media data", s.index+1, kind)
		}
		if m.contentType == "" {
			return fmt.Errorf("pptx: slide %d: %s has no content type and the media format was not recognized", s.index+1, kind)
		}
	}
	return nil
}

// sniffMediaContentType infers a media MIME type from the leading bytes of the
// data. It recognizes the common containers PowerPoint embeds (MP4/M4V, M4A,
// QuickTime, MP3, WAV, AVI, Ogg, WebM/Matroska) and returns "" for anything
// else.
func sniffMediaContentType(data []byte) string {
	if len(data) < 12 {
		return ""
	}
	// ISO base media file format: size + "ftyp" + brand.
	if string(data[4:8]) == "ftyp" {
		switch brand := string(data[8:12]); {
		case strings.HasPrefix(brand, "qt"):
			return "video/quicktime"
		case strings.HasPrefix(brand, "M4A"):
			return "audio/mp4"
		default: // isom, iso2, mp41, mp42, avc1, M4V , ...
			return "video/mp4"
		}
	}
	if string(data[:4]) == "RIFF" {
		switch string(data[8:12]) {
		case "WAVE":
			return "audio/wav"
		case "AVI ":
			return "video/x-msvideo"
		}
		return ""
	}
	if string(data[:3]) == "ID3" {
		return "audio/mpeg"
	}
	// Bare MPEG audio frame sync (11 set bits).
	if data[0] == 0xFF && data[1]&0xE0 == 0xE0 {
		return "audio/mpeg"
	}
	if string(data[:4]) == "OggS" {
		return "audio/ogg"
	}
	// EBML header (WebM/Matroska).
	if data[0] == 0x1A && data[1] == 0x45 && data[2] == 0xDF && data[3] == 0xA3 {
		return "video/webm"
	}
	return ""
}

// embedMediaData stores the media bytes as a part (reusing an existing part
// only when both its bytes and its declared content type match) and creates the
// two relationships an embedded video/audio needs: a Microsoft "media" embed
// reference and an OOXML "video"/"audio" link reference, both pointing at the
// same media part. It returns (mediaRelID, linkRelID).
func (s *Slide) embedMediaData(data []byte, contentType, linkRelType string) (mediaRelID, linkRelID string) {
	p := s.presentation

	mediaName := ""
	for name, part := range p.otherParts {
		if part != nil && strings.HasPrefix(name, "/ppt/media/") &&
			part.ContentType == contentType && bytes.Equal(part.Data, data) {
			mediaName = name
			break
		}
	}
	if mediaName == "" {
		mediaName = s.nextMediaFileName(mediaExtFromContentType(contentType))
		p.otherParts[mediaName] = &coxml.RawPart{ContentType: contentType, Data: data}
	}

	target := relativeTarget(s.partName, mediaName)
	mediaRelID = s.nextRelID()
	p.relationships[s.partName] = append(p.relationships[s.partName], &opc.Relationship{
		ID:         mediaRelID,
		Type:       opc.RelTypeMedia,
		Target:     target,
		TargetMode: opc.TargetModeInternal,
	})
	linkRelID = s.nextRelID()
	p.relationships[s.partName] = append(p.relationships[s.partName], &opc.Relationship{
		ID:         linkRelID,
		Type:       linkRelType,
		Target:     target,
		TargetMode: opc.TargetModeInternal,
	})
	return mediaRelID, linkRelID
}

// nextMediaFileName returns an unused /ppt/media/mediaN.<ext> part name,
// numbering above any existing mediaN part regardless of its extension so
// distinct media (media1.mp4, media2.mp3) never collide on the index.
func (s *Slide) nextMediaFileName(ext string) string {
	p := s.presentation
	max := 0
	for name := range p.otherParts {
		base := strings.TrimPrefix(name, "/ppt/media/media")
		if base == name {
			continue
		}
		if dot := strings.IndexByte(base, '.'); dot > 0 {
			if n, err := strconv.Atoi(base[:dot]); err == nil && n > max {
				max = n
			}
		}
	}
	return fmt.Sprintf("/ppt/media/media%d%s", max+1, ext)
}

// buildMediaPic embeds the media (and poster) if not already embedded, then
// builds the p:pic element that represents the video/audio on the slide.
func (s *Slide) buildMediaPic(m *mediaShape, id uint32, kind mediaKind) *oxml.Picture {
	// Invalid media (no data or no resolvable content type) is not embedded:
	// validateMediaShapes fails the save with a descriptive error, and skipping
	// here keeps an early sync (ReplaceText/Duplicate before the first save)
	// from storing an unregistered part.
	if m.mediaRelID == "" && len(m.mediaData) > 0 && m.contentType != "" {
		linkType := opc.RelTypeVideo
		if kind == mediaAudio {
			linkType = opc.RelTypeAudio
		}
		m.mediaRelID, m.linkRelID = s.embedMediaData(m.mediaData, m.contentType, linkType)
		posterData, posterCT := m.effectivePoster()
		m.posterRelID = s.embedImageData(posterData, posterCT)
	}

	if m.playMode == PlayAutomatically {
		s.autoplayMedia = append(s.autoplayMedia, mediaTimingRef{spid: id, kind: kind})
	}

	name := m.Name()
	if name == "" {
		if kind == mediaAudio {
			name = "Audio"
		} else {
			name = "Video"
		}
	}

	x, y := m.Position()
	w, h := m.Size()
	if w <= 0 || h <= 0 {
		w, h = defaultMediaWidth, defaultMediaHeight
	}

	nvPr := &oxml.NvPr{
		ExtLst: &oxml.ExtensionList{
			Ext: []oxml.Extension{{
				URI:   xmlb.ExtURIMedia,
				Media: &oxml.P14Media{Embed: m.mediaRelID},
			}},
		},
	}
	if kind == mediaAudio {
		nvPr.AudioFile = &dml.AudioFile{Link: m.linkRelID}
	} else {
		nvPr.VideoFile = &dml.VideoFile{Link: m.linkRelID}
	}

	return &oxml.Picture{
		NvPicPr: &oxml.NvPicPr{
			CNvPr: &dml.CNvPr{
				Id:         id,
				Name:       name,
				HlinkClick: &dml.HlinkXML{Action: "ppaction://media"},
			},
			CNvPicPr: &dml.CNvPicPr{
				PicLocks: &dml.PicLocks{NoChangeAspect: true},
			},
			NvPr: nvPr,
		},
		BlipFill: &dml.BlipFill{
			Blip:    &dml.Blip{Embed: m.posterRelID},
			Stretch: &dml.Stretch{FillRect: &dml.RelRect{}},
		},
		SpPr: &dml.SpPr{
			Xfrm: &dml.Xfrm{
				Off: &dml.OffXML{X: int64(x), Y: int64(y)},
				Ext: &dml.ExtXML{Cx: int64(w), Cy: int64(h)},
			},
			PrstGeom: &dml.PrstGeom{
				Prst:  "rect",
				AvLst: &dml.AvLst{},
			},
		},
	}
}

// mediaExtFromContentType maps a media MIME type to a file extension used for
// the media part name (and hence its content-type registration).
func mediaExtFromContentType(ct string) string {
	switch ct {
	case "video/mp4":
		return ".mp4"
	case "video/quicktime":
		return ".mov"
	case "video/x-msvideo":
		return ".avi"
	case "video/x-ms-wmv":
		return ".wmv"
	case "video/webm":
		return ".webm"
	case "audio/mpeg":
		return ".mp3"
	case "audio/mp4", "audio/x-m4a":
		return ".m4a"
	case "audio/wav", "audio/x-wav":
		return ".wav"
	case "audio/aac":
		return ".aac"
	case "audio/ogg":
		return ".ogg"
	}
	// Generic fallback: use the subtype after "/" (stripping an "x-" prefix).
	if i := strings.IndexByte(ct, '/'); i >= 0 && i+1 < len(ct) {
		if sub := strings.TrimPrefix(ct[i+1:], "x-"); sub != "" {
			return "." + sub
		}
	}
	return ".bin"
}
