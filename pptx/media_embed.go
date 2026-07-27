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

// forEachShape calls fn for every shape in shapes, descending into group
// shapes (media can live inside groups via GroupShape.AddChild).
func forEachShape(shapes []Shape, fn func(Shape)) {
	for _, shape := range shapes {
		fn(shape)
		if grp, ok := shape.(*GroupShape); ok {
			forEachShape(grp.children, fn)
		}
	}
}

// resolveMediaContentTypes fills in the content type of media shapes the
// caller added without one, by sniffing the leading magic bytes. Called before
// any shape sync so the media part is never stored under an unregistered
// extension.
func (s *Slide) resolveMediaContentTypes() {
	forEachShape(s.shapeCache, func(shape Shape) {
		m, _ := mediaShapeOf(shape)
		if m == nil || m.mediaRelID != "" || m.contentType != "" {
			return
		}
		m.contentType = sniffMediaContentType(m.mediaData)
	})
}

// validateMediaShapes rejects media that cannot be embedded as a valid OPC
// part: no bytes at all, or no content type (neither declared by the caller
// nor recognizable from the data). Without this check an empty content type
// produced a /ppt/media/mediaN.bin part absent from [Content_Types].xml — an
// invalid package that PowerPoint asks to repair.
func (s *Slide) validateMediaShapes() error {
	s.resolveMediaContentTypes()
	var err error
	forEachShape(s.shapeCache, func(shape Shape) {
		if err != nil {
			return
		}
		m, kind := mediaShapeOf(shape)
		if m == nil || m.mediaRelID != "" {
			return
		}
		if len(m.mediaData) == 0 {
			err = fmt.Errorf("pptx: slide %d: %s has no media data", s.index+1, kind)
			return
		}
		if m.contentType == "" {
			err = fmt.Errorf("pptx: slide %d: %s has no content type and the media format was not recognized", s.index+1, kind)
		}
	})
	return err
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

// embedAudioPart stores audio bytes as a /ppt/media part (reusing an existing
// part with identical bytes and content type) and adds a single "audio"
// relationship from the slide to it, returning the relationship id. A
// transition start sound (p:sndAc/p:stSnd) references the part by this r:embed —
// unlike an on-slide audio shape it needs no separate p14 media embed.
func (s *Slide) embedAudioPart(data []byte, contentType string) string {
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

	relID := s.nextRelID()
	p.relationships[s.partName] = append(p.relationships[s.partName], &opc.Relationship{
		ID:         relID,
		Type:       opc.RelTypeAudio,
		Target:     relativeTarget(s.partName, mediaName),
		TargetMode: opc.TargetModeInternal,
	})
	return relID
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
		// Write the default back into the domain shape so both
		// representations agree: otherwise a later SetPosition/SetName flush
		// (updateXfrm) would write the domain's 0x0 into the node, collapsing
		// the shape (C192). Assigned directly — this is a sync, not a caller
		// mutation, so it must not set the dirty flag.
		m.width, m.height = w, h
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

// flushMediaShapeProps applies SetPlayMode/SetPoster edits made after the
// shape was synced to its parsed p:pic node and the timing bookkeeping
// (previously such edits were silent no-ops on already-saved shapes, C220).
func (s *Slide) flushMediaShapeProps(pic *oxml.Picture, shape Shape) {
	switch sh := shape.(type) {
	case *Video:
		s.flushMediaProps(pic, &sh.mediaShape, mediaVideo)
	case *Audio:
		s.flushMediaProps(pic, &sh.mediaShape, mediaAudio)
	}
}

func (s *Slide) flushMediaProps(pic *oxml.Picture, m *mediaShape, kind mediaKind) {
	if m.posterDirty {
		m.posterDirty = false
		// Only media already embedded needs an explicit swap; before the
		// first embed, buildMediaPic reads the current poster anyway.
		if m.mediaRelID != "" {
			oldID := ""
			if pic.BlipFill != nil && pic.BlipFill.Blip != nil {
				oldID = pic.BlipFill.Blip.Embed
			}
			data, ct := m.effectivePoster()
			newID := s.embedImageData(data, ct)
			if pic.BlipFill == nil {
				pic.BlipFill = &dml.BlipFill{Stretch: &dml.Stretch{FillRect: &dml.RelRect{}}}
			}
			if pic.BlipFill.Blip == nil {
				pic.BlipFill.Blip = &dml.Blip{}
			}
			pic.BlipFill.Blip.Embed = newID
			m.posterRelID = newID
			if oldID != "" && oldID != newID {
				s.gcSlideRels([]string{oldID})
			}
		}
	}
	if m.timingDirty {
		m.timingDirty = false
		var spid uint32
		if pic.NvPicPr != nil && pic.NvPicPr.CNvPr != nil {
			spid = pic.NvPicPr.CNvPr.Id
		}
		s.syncPlayMode(spid, kind, m.playMode)
	}
}

// collectRemovedPicRefs gathers, for each removed node, the shape ids and the
// relationship ids (poster/image blip, svg blip, p14:media embed, video/audio
// file link) that go away with it. Called before the nodes are deleted from the
// tree; whether a rel id is safe to drop is decided later by gcSlideRels, which
// checks the remaining slide XML for other references.
//
// A removed p:grpSp is descended into, recursively. Matching only ChildPic left
// media inside a removed group leaking its rels and its media part, and left the
// generated p:timing tree targeting a spid that no longer existed — the same
// defect the top-level case fixes, one level of nesting down (C379). The add and
// flush paths have always descended into groups (forEachShape, flushGroupChild,
// paragraphCountInGroup); the removal side was the asymmetric one, so the walk
// below collects every descendant's shape id regardless of kind. Extra spids are
// harmless — pruneAutoTiming matches them against the refs it built the tree
// from — and a per-shape resource added later to some other node kind is one
// switch arm here rather than a new instance of this bug.
func collectRemovedPicRefs(spTree *oxml.ShapeTree, refs []oxml.ChildRef) (spids []uint32, relIDs []string) {
	collect := func(pic *oxml.Picture) {
		if pic == nil {
			return
		}
		if pic.NvPicPr != nil {
			if pic.NvPicPr.CNvPr != nil {
				spids = append(spids, pic.NvPicPr.CNvPr.Id)
			}
			if nv := pic.NvPicPr.NvPr; nv != nil {
				if nv.VideoFile != nil && nv.VideoFile.Link != "" {
					relIDs = append(relIDs, nv.VideoFile.Link)
				}
				if nv.AudioFile != nil && nv.AudioFile.Link != "" {
					relIDs = append(relIDs, nv.AudioFile.Link)
				}
				if nv.ExtLst != nil {
					for _, ext := range nv.ExtLst.Ext {
						if ext.Media != nil && ext.Media.Embed != "" {
							relIDs = append(relIDs, ext.Media.Embed)
						}
					}
				}
			}
		}
		if pic.BlipFill != nil && pic.BlipFill.Blip != nil {
			if pic.BlipFill.Blip.Embed != "" {
				relIDs = append(relIDs, pic.BlipFill.Blip.Embed)
			}
			if pic.BlipFill.Blip.ExtLst != nil {
				for _, ext := range pic.BlipFill.Blip.ExtLst.Ext {
					if ext != nil && ext.SvgBlip != nil && ext.SvgBlip.Embed != "" {
						relIDs = append(relIDs, ext.SvgBlip.Embed)
					}
				}
			}
		}
	}

	descend := func(g *oxml.GroupShape, depth int) {
		eachShapeIDInGroup(g, depth, func(id uint32) { spids = append(spids, id) })
		eachPictureInGroup(g, depth, collect)
	}

	for _, ref := range refs {
		if ref.Index < 0 {
			continue
		}
		switch ref.Kind {
		case oxml.ChildPic:
			if ref.Index < len(spTree.Pic) {
				collect(spTree.Pic[ref.Index])
			}
		case oxml.ChildGrpSp:
			if ref.Index < len(spTree.GrpSp) {
				descend(spTree.GrpSp[ref.Index], 0)
			}
		}
	}
	return spids, relIDs
}

// eachShapeIDInGroup calls fn with the cNvPr id of the group itself and of
// every descendant, regardless of kind, recursing through nested groups.
//
// It is shared by the removal sweep (collectRemovedPicRefs, C379) and by the
// animation target check (Slide.shapeIDsInTree, C416) so the two cannot drift:
// both need the answer to "which shape ids does this subtree contain", and the
// removal side having its own narrower version is what let C379 exist.
func eachShapeIDInGroup(g *oxml.GroupShape, depth int, fn func(uint32)) {
	// Groups nest arbitrarily; a parsed tree cannot be cyclic, but bound the
	// recursion anyway so a hand-built model cannot spin here.
	if g == nil || depth > maxGroupNestDepth {
		return
	}
	if g.NvGrpSpPr != nil && g.NvGrpSpPr.CNvPr != nil {
		fn(g.NvGrpSpPr.CNvPr.Id)
	}
	for _, sp := range g.Shapes {
		if sp != nil && sp.NvSpPr != nil && sp.NvSpPr.CNvPr != nil {
			fn(sp.NvSpPr.CNvPr.Id)
		}
	}
	for _, pic := range g.Pictures {
		if pic != nil && pic.NvPicPr != nil && pic.NvPicPr.CNvPr != nil {
			fn(pic.NvPicPr.CNvPr.Id)
		}
	}
	for _, gf := range g.GraphicFrames {
		if gf != nil && gf.NvGraphicFramePr != nil && gf.NvGraphicFramePr.CNvPr != nil {
			fn(gf.NvGraphicFramePr.CNvPr.Id)
		}
	}
	for _, cxn := range g.ConnectionShapes {
		if cxn != nil && cxn.NvCxnSpPr != nil && cxn.NvCxnSpPr.CNvPr != nil {
			fn(cxn.NvCxnSpPr.CNvPr.Id)
		}
	}
	for _, sub := range g.GroupShapes {
		eachShapeIDInGroup(sub, depth+1, fn)
	}
}

// eachPictureInGroup calls fn with every picture in the group, recursing
// through nested groups.
func eachPictureInGroup(g *oxml.GroupShape, depth int, fn func(*oxml.Picture)) {
	if g == nil || depth > maxGroupNestDepth {
		return
	}
	for _, pic := range g.Pictures {
		fn(pic)
	}
	for _, sub := range g.GroupShapes {
		eachPictureInGroup(sub, depth+1, fn)
	}
}

// maxGroupNestDepth bounds the group recursion in collectRemovedPicRefs. Real
// decks nest a handful deep; the bound exists only so a malformed or
// hand-assembled model cannot make the walk unbounded.
const maxGroupNestDepth = 64

// removableRelType reports whether a slide relationship type may be garbage
// collected once its id is no longer referenced by the slide XML. Only the
// per-shape media/image types are removable; everything else (layout, notes,
// hyperlinks, ...) is kept.
func removableRelType(relType string) bool {
	switch relType {
	case opc.RelTypeMedia, opc.RelTypeVideo, opc.RelTypeAudio, opc.RelTypeImage:
		return true
	}
	return false
}

// gcSlideRels removes this slide's media/image relationships with the given
// ids when the current slide XML no longer references them — e.g. after a
// media shape was surgically removed. Ids still referenced by any remaining
// node are kept: parts and rels can be shared by several shapes on one slide.
// Package-level parts are never touched here (they may be shared across
// slides).
func (s *Slide) gcSlideRels(relIDs []string) {
	if len(relIDs) == 0 || s.presentation == nil || s.sx() == nil {
		return
	}
	slideXML, err := marshalSlide(s.sx())
	if err != nil {
		// A slide that fails to marshal keeps its relationships; the error
		// surfaces from the Save path that marshals the slide part itself.
		return
	}
	s.presentation.gcPartRels(s.partName, slideXML, relIDs)
}

// gcPartRels removes partName's media/image relationships with the given ids
// when partXML — the freshly marshaled part — no longer references them. It is
// the part-agnostic core of gcSlideRels, shared with slide-layout and
// slide-master background swaps. Ids still referenced by partXML are kept
// (parts and rels can be shared by several nodes in one part); package-level
// media parts are never removed here, only their rels, so the save's media GC
// (mediaGCNeeded) decides part removal after re-checking every part.
func (p *Presentation) gcPartRels(partName string, partXML []byte, relIDs []string) {
	if len(relIDs) == 0 || p == nil {
		return
	}
	candidates := make(map[string]bool, len(relIDs))
	for _, id := range relIDs {
		if id != "" {
			candidates[id] = true
		}
	}
	if len(candidates) == 0 {
		return
	}
	rels := p.relationships[partName]
	kept := rels[:0]
	changed := false
	for _, rel := range rels {
		if candidates[rel.ID] && removableRelType(rel.Type) &&
			!bytes.Contains(partXML, []byte(`"`+rel.ID+`"`)) {
			changed = true
			continue
		}
		kept = append(kept, rel)
	}
	if changed {
		p.relationships[partName] = kept
		// Dropped relationships may leave media parts unreferenced; allow the
		// save to garbage-collect them (C221).
		p.mediaGCNeeded = true
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
