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

// embedMediaData stores the media bytes as a part (reusing an identical existing
// part) and creates the two relationships an embedded video/audio needs: a
// Microsoft "media" embed reference and an OOXML "video"/"audio" link reference,
// both pointing at the same media part. It returns (mediaRelID, linkRelID).
func (s *Slide) embedMediaData(data []byte, contentType, linkRelType string) (mediaRelID, linkRelID string) {
	p := s.presentation

	mediaName := ""
	for name, part := range p.otherParts {
		if part != nil && strings.HasPrefix(name, "/ppt/media/") && bytes.Equal(part.Data, data) {
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
	if m.mediaRelID == "" {
		linkType := opc.RelTypeVideo
		if kind == mediaAudio {
			linkType = opc.RelTypeAudio
		}
		m.mediaRelID, m.linkRelID = s.embedMediaData(m.mediaData, m.contentType, linkType)
		posterData, posterCT := m.effectivePoster()
		m.posterRelID = s.embedImageData(posterData, posterCT)
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
