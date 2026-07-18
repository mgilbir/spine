package pptx

import (
	"bytes"
	"image"

	// Register the stdlib decoders so AddPicture can read the intrinsic
	// dimensions of the common image formats for its default frame size.
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"

	"github.com/mgilbir/spine/common/dml"
)

// emuPerPixel is the EMU size of one pixel at 96 DPI (914400 EMU per inch / 96).
const emuPerPixel = 9525

// nativeImageSize returns the intrinsic size of the encoded image at 96 DPI.
// ok is false when the data is not in a decodable format (only the image
// header is read, never the full pixel data).
func nativeImageSize(data []byte) (w, h dml.EMU, ok bool) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 {
		return 0, 0, false
	}
	return dml.EMU(cfg.Width) * emuPerPixel, dml.EMU(cfg.Height) * emuPerPixel, true
}

// Picture represents a picture shape.
type Picture struct {
	BaseShape
	imagePath      string
	imageData      []byte
	contentType    string
	svgData        []byte
	svgContentType string
	relID          string
	svgRelID       string
	description    string
	cropLeft       float64
	cropRight      float64
	cropTop        float64
	cropBottom     float64
	slide          *Slide // back-reference to owning slide (set during materialization)
	// isMedia marks a p:pic that backs an embedded video/audio (its blip is a
	// poster, not a standalone image). Such pics are excluded from Slide.Pictures.
	isMedia bool
	// The picture's stable node identity (used e.g. by replacePictureImage to
	// locate the exact node when two pictures share an image reference) lives
	// in BaseShape.sourceID.
}

// NewPicture creates a new picture shape.
func NewPicture() *Picture {
	return &Picture{}
}

// ShapeType returns ShapeTypePicture.
func (p *Picture) ShapeType() ShapeType {
	return ShapeTypePicture
}

// ImagePath returns the path to the image file.
func (p *Picture) ImagePath() string {
	return p.imagePath
}

// SetImagePath sets the path to the image file.
func (p *Picture) SetImagePath(path string) {
	p.imagePath = path
}

// ImageData returns the raw image data.
func (p *Picture) ImageData() []byte {
	return p.imageData
}

// SetImageData sets the raw image data and content type.
func (p *Picture) SetImageData(data []byte, contentType string) {
	p.imageData = data
	p.contentType = contentType
	p.svgData = nil
	p.svgContentType = ""
}

// SetSVGImageData sets SVG data plus a raster fallback image.
// The raster fallback is written to a:blip@r:embed and the SVG is referenced
// through the Office svgBlip extension.
func (p *Picture) SetSVGImageData(svgData, fallbackData []byte, fallbackCT string) {
	p.svgData = svgData
	p.svgContentType = "image/svg+xml"
	p.imageData = fallbackData
	p.contentType = fallbackCT
	p.imagePath = ""
}

// SetSVGData sets SVG data using a built-in transparent PNG fallback.
func (p *Picture) SetSVGData(svgData []byte) {
	p.SetSVGImageData(svgData, minimalTransparentPNG, "image/png")
}

// SetImage sets an image on this picture from a file path.
// The file is read immediately; the image is embedded when the presentation is saved.
func (p *Picture) SetImage(imagePath string) error {
	data, ct, err := readImageFile(imagePath)
	if err != nil {
		return err
	}

	p.imagePath = imagePath
	p.imageData = data
	p.contentType = ct
	p.svgData = nil
	p.svgContentType = ""

	return nil
}

// hasPendingImage returns true if this picture has pending image data to embed
// as a replacement for an existing image. Only applies to pictures loaded from
// an existing file (which have a slide back-reference set).
func (p *Picture) hasPendingImage() bool {
	return p.slide != nil && (len(p.imageData) > 0 || len(p.svgData) > 0)
}

// ContentType returns the MIME type of the image. For a picture read from an
// opened deck it is resolved from the embedded media part.
func (p *Picture) ContentType() string {
	if p.contentType != "" {
		return p.contentType
	}
	if p.slide != nil && p.relID != "" {
		if name := p.slide.relTargetPart(p.relID); name != "" {
			if part := p.slide.presentation.otherParts[name]; part != nil {
				return part.ContentType
			}
		}
	}
	return ""
}

// Data returns the raw image bytes: the pending bytes for a picture added via
// the API, or the embedded media part for a picture read from an opened deck.
// Returns nil when the bytes cannot be resolved.
func (p *Picture) Data() []byte {
	if len(p.imageData) > 0 {
		return p.imageData
	}
	if p.slide != nil && p.relID != "" {
		if name := p.slide.relTargetPart(p.relID); name != "" {
			return p.slide.presentation.rawPartData(name)
		}
	}
	return nil
}

// AltText returns the picture's alternative text (the descr on p:cNvPr). It is
// the cross-format name shared with the docx and xlsx picture readers; see also
// Description.
func (p *Picture) AltText() string {
	return p.description
}

// SetHyperlink attaches an external-URL hyperlink to the picture. The External
// relationship is allocated on save.
func (p *Picture) SetHyperlink(url string) *Hyperlink { return p.setHyperlinkURL(url) }

// SetActionHyperlink attaches a slide-show action hyperlink (e.g. ActionNextSlide)
// to the picture.
func (p *Picture) SetActionHyperlink(action string) *Hyperlink { return p.setActionHyperlink(action) }

// SetHyperlinkToSlide attaches an internal jump to the slide at the given 0-based
// index; the RelTypeSlide relationship is allocated on save.
func (p *Picture) SetHyperlinkToSlide(index int) *Hyperlink { return p.setSlideLink(index) }

// Description returns the image description (alt text).
func (p *Picture) Description() string {
	return p.description
}

// SetDescription sets the image description (alt text).
func (p *Picture) SetDescription(desc string) {
	p.description = desc
	p.dirty = true
}

// CropLeft returns the left crop amount (0.0 to 1.0).
func (p *Picture) CropLeft() float64 {
	return p.cropLeft
}

// SetCropLeft sets the left crop amount (0.0 to 1.0).
func (p *Picture) SetCropLeft(crop float64) {
	p.cropLeft = clampCrop(crop)
	p.dirty = true
}

// CropRight returns the right crop amount (0.0 to 1.0).
func (p *Picture) CropRight() float64 {
	return p.cropRight
}

// SetCropRight sets the right crop amount (0.0 to 1.0).
func (p *Picture) SetCropRight(crop float64) {
	p.cropRight = clampCrop(crop)
	p.dirty = true
}

// CropTop returns the top crop amount (0.0 to 1.0).
func (p *Picture) CropTop() float64 {
	return p.cropTop
}

// SetCropTop sets the top crop amount (0.0 to 1.0).
func (p *Picture) SetCropTop(crop float64) {
	p.cropTop = clampCrop(crop)
	p.dirty = true
}

// CropBottom returns the bottom crop amount (0.0 to 1.0).
func (p *Picture) CropBottom() float64 {
	return p.cropBottom
}

// SetCropBottom sets the bottom crop amount (0.0 to 1.0).
func (p *Picture) SetCropBottom(crop float64) {
	p.cropBottom = clampCrop(crop)
	p.dirty = true
}

// SetCrop sets all crop amounts.
func (p *Picture) SetCrop(left, top, right, bottom float64) {
	p.cropLeft = clampCrop(left)
	p.cropTop = clampCrop(top)
	p.cropRight = clampCrop(right)
	p.cropBottom = clampCrop(bottom)
	p.dirty = true
}

// clampCrop ensures the crop value is between 0 and 1.
func clampCrop(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// mediaShape holds the state shared by embedded Video and Audio shapes: the raw
// media bytes, its content type, an optional poster/preview image, and the
// relationship IDs assigned when the media is embedded on save.
type mediaShape struct {
	BaseShape
	mediaData   []byte
	contentType string
	posterData  []byte
	posterCT    string
	playMode    PlayMode

	// Relationship IDs, assigned during serialization.
	mediaRelID  string // r:embed on p14:media (Microsoft "media" reltype)
	linkRelID   string // r:link on a:videoFile/a:audioFile ("video"/"audio" reltype)
	posterRelID string // r:embed on the poster blip (image reltype)

	// timingDirty/posterDirty mark SetPlayMode/SetPoster edits made after the
	// shape was synced into the slide XML; the shape sync flushes them into
	// the parsed node and the timing bookkeeping (see flushMediaProps).
	// Without them such edits were silent no-ops on already-saved shapes.
	timingDirty bool
	posterDirty bool
}

// hasPendingProps reports whether SetPlayMode/SetPoster edits await a flush.
func (m *mediaShape) hasPendingProps() bool { return m.timingDirty || m.posterDirty }

// PlayMode controls when embedded media starts playing.
type PlayMode int

const (
	// PlayOnClick plays the media when the viewer clicks it (the default; no
	// timing tree is emitted).
	PlayOnClick PlayMode = iota
	// PlayAutomatically plays the media automatically when the slide appears,
	// via a timing tree. Applied only when the slide has no existing timing.
	PlayAutomatically
)

// PlayMode returns the media's play mode.
func (m *mediaShape) PlayMode() PlayMode { return m.playMode }

// SetPlayMode sets when the media starts playing. It also applies to media
// already written by an earlier save: the generated timing tree is updated
// (or created/dropped) on the next save. Slides whose timing tree was parsed
// from a file are left untouched.
func (m *mediaShape) SetPlayMode(mode PlayMode) {
	if m.playMode == mode {
		return
	}
	m.playMode = mode
	m.timingDirty = true
}

// MediaData returns the raw media bytes.
func (m *mediaShape) MediaData() []byte { return m.mediaData }

// ContentType returns the MIME type of the media (e.g. "video/mp4").
func (m *mediaShape) ContentType() string { return m.contentType }

// Poster returns the poster/preview image bytes and content type, if set.
func (m *mediaShape) Poster() (data []byte, contentType string) {
	return m.posterData, m.posterCT
}

// SetPoster sets the poster/preview image shown before playback. When unset, a
// minimal placeholder image is generated on save so the file stays valid.
// Setting a poster on media already written by an earlier save swaps the
// poster blip on the next save.
func (m *mediaShape) SetPoster(data []byte, contentType string) {
	m.posterData = data
	m.posterCT = contentType
	m.posterDirty = true
}

// effectivePoster returns the caller-provided poster, or a generated placeholder
// when none was set (PowerPoint requires a blip fill on a media picture).
func (m *mediaShape) effectivePoster() (data []byte, contentType string) {
	if len(m.posterData) > 0 {
		return m.posterData, m.posterCT
	}
	return minimalTransparentPNG, "image/png"
}

// Video represents an embedded video shape. On save it is serialized as a
// p:pic referencing the embedded media, so it opens and plays in PowerPoint.
type Video struct {
	mediaShape
}

// NewVideo creates a video shape from raw media bytes and their content type
// (e.g. "video/mp4"). Use Slide.AddVideo to attach it to a slide.
func NewVideo(data []byte, contentType string) *Video {
	return &Video{mediaShape{mediaData: data, contentType: contentType}}
}

// ShapeType returns ShapeTypeVideo.
func (v *Video) ShapeType() ShapeType { return ShapeTypeVideo }

// Audio represents an embedded audio shape, serialized as a p:pic referencing
// the embedded media on save.
type Audio struct {
	mediaShape
}

// NewAudio creates an audio shape from raw media bytes and their content type
// (e.g. "audio/mpeg"). Use Slide.AddAudio to attach it to a slide.
func NewAudio(data []byte, contentType string) *Audio {
	return &Audio{mediaShape{mediaData: data, contentType: contentType}}
}

// ShapeType returns ShapeTypeAudio.
func (a *Audio) ShapeType() ShapeType { return ShapeTypeAudio }

// Pictures returns every picture on the slide, descending into groups, in
// document order. Pictures that back embedded video/audio (their blip is a
// poster) are excluded. A slide with no pictures returns nil.
func (s *Slide) Pictures() []*Picture {
	var out []*Picture
	forEachShape(s.shapes, func(shape Shape) {
		if pic, ok := shape.(*Picture); ok && !pic.isMedia {
			out = append(out, pic)
		}
	})
	return out
}

// Pictures returns every picture across all slides, slide by slide in slide
// order (see Slide.Pictures).
func (p *Presentation) Pictures() []*Picture {
	var out []*Picture
	for _, s := range p.slides {
		out = append(out, s.Pictures()...)
	}
	return out
}
