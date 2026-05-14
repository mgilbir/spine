package pptx

import (
	"github.com/mgilbir/spine/common/dml"
)

// Picture represents a picture shape.
type Picture struct {
	BaseShape
	imagePath    string
	imageData    []byte
	contentType  string
	relID        string
	description  string
	cropLeft     float64
	cropRight    float64
	cropTop      float64
	cropBottom   float64
	slide        *Slide // back-reference to owning slide (set during materialization)
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

	return nil
}

// hasPendingImage returns true if this picture has pending image data to embed
// as a replacement for an existing image. Only applies to pictures loaded from
// an existing file (which have a slide back-reference set).
func (p *Picture) hasPendingImage() bool {
	return len(p.imageData) > 0 && p.slide != nil
}

// ContentType returns the MIME type of the image.
func (p *Picture) ContentType() string {
	return p.contentType
}

// Description returns the image description (alt text).
func (p *Picture) Description() string {
	return p.description
}

// SetDescription sets the image description (alt text).
func (p *Picture) SetDescription(desc string) {
	p.description = desc
}

// CropLeft returns the left crop amount (0.0 to 1.0).
func (p *Picture) CropLeft() float64 {
	return p.cropLeft
}

// SetCropLeft sets the left crop amount (0.0 to 1.0).
func (p *Picture) SetCropLeft(crop float64) {
	p.cropLeft = clampCrop(crop)
}

// CropRight returns the right crop amount (0.0 to 1.0).
func (p *Picture) CropRight() float64 {
	return p.cropRight
}

// SetCropRight sets the right crop amount (0.0 to 1.0).
func (p *Picture) SetCropRight(crop float64) {
	p.cropRight = clampCrop(crop)
}

// CropTop returns the top crop amount (0.0 to 1.0).
func (p *Picture) CropTop() float64 {
	return p.cropTop
}

// SetCropTop sets the top crop amount (0.0 to 1.0).
func (p *Picture) SetCropTop(crop float64) {
	p.cropTop = clampCrop(crop)
}

// CropBottom returns the bottom crop amount (0.0 to 1.0).
func (p *Picture) CropBottom() float64 {
	return p.cropBottom
}

// SetCropBottom sets the bottom crop amount (0.0 to 1.0).
func (p *Picture) SetCropBottom(crop float64) {
	p.cropBottom = clampCrop(crop)
}

// SetCrop sets all crop amounts.
func (p *Picture) SetCrop(left, top, right, bottom float64) {
	p.cropLeft = clampCrop(left)
	p.cropTop = clampCrop(top)
	p.cropRight = clampCrop(right)
	p.cropBottom = clampCrop(bottom)
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

// Video represents a video shape.
type Video struct {
	BaseShape
	videoPath   string
	videoData   []byte
	contentType string
	posterFrame *Picture // preview image
}

// NewVideo creates a new video shape.
func NewVideo() *Video {
	return &Video{}
}

// ShapeType returns ShapeTypeUnknown (video is a special shape type).
func (v *Video) ShapeType() ShapeType {
	return ShapeTypeUnknown
}

// VideoPath returns the path to the video file.
func (v *Video) VideoPath() string {
	return v.videoPath
}

// SetVideoPath sets the path to the video file.
func (v *Video) SetVideoPath(path string) {
	v.videoPath = path
}

// VideoData returns the raw video data.
func (v *Video) VideoData() []byte {
	return v.videoData
}

// SetVideoData sets the raw video data and content type.
func (v *Video) SetVideoData(data []byte, contentType string) {
	v.videoData = data
	v.contentType = contentType
}

// ContentType returns the MIME type of the video.
func (v *Video) ContentType() string {
	return v.contentType
}

// PosterFrame returns the poster frame (preview image).
func (v *Video) PosterFrame() *Picture {
	return v.posterFrame
}

// SetPosterFrame sets the poster frame (preview image).
func (v *Video) SetPosterFrame(frame *Picture) {
	v.posterFrame = frame
}

// Audio represents an audio shape.
type Audio struct {
	BaseShape
	audioPath   string
	audioData   []byte
	contentType string
	icon        *Picture // icon to display
}

// NewAudio creates a new audio shape.
func NewAudio() *Audio {
	return &Audio{}
}

// ShapeType returns ShapeTypeUnknown (audio is a special shape type).
func (a *Audio) ShapeType() ShapeType {
	return ShapeTypeUnknown
}

// AudioPath returns the path to the audio file.
func (a *Audio) AudioPath() string {
	return a.audioPath
}

// SetAudioPath sets the path to the audio file.
func (a *Audio) SetAudioPath(path string) {
	a.audioPath = path
}

// AudioData returns the raw audio data.
func (a *Audio) AudioData() []byte {
	return a.audioData
}

// SetAudioData sets the raw audio data and content type.
func (a *Audio) SetAudioData(data []byte, contentType string) {
	a.audioData = data
	a.contentType = contentType
}

// ContentType returns the MIME type of the audio.
func (a *Audio) ContentType() string {
	return a.contentType
}

// Icon returns the icon picture.
func (a *Audio) Icon() *Picture {
	return a.icon
}

// SetIcon sets the icon picture.
func (a *Audio) SetIcon(icon *Picture) {
	a.icon = icon
}

// OLEObject represents an embedded OLE object.
type OLEObject struct {
	BaseShape
	progID      string
	data        []byte
	icon        *Picture
}

// NewOLEObject creates a new OLE object.
func NewOLEObject() *OLEObject {
	return &OLEObject{}
}

// ShapeType returns ShapeTypeUnknown (OLE is a special shape type).
func (o *OLEObject) ShapeType() ShapeType {
	return ShapeTypeUnknown
}

// ProgID returns the programmatic identifier of the OLE object.
func (o *OLEObject) ProgID() string {
	return o.progID
}

// SetProgID sets the programmatic identifier of the OLE object.
func (o *OLEObject) SetProgID(progID string) {
	o.progID = progID
}

// Data returns the raw OLE data.
func (o *OLEObject) Data() []byte {
	return o.data
}

// SetData sets the raw OLE data.
func (o *OLEObject) SetData(data []byte) {
	o.data = data
}

// Icon returns the icon picture.
func (o *OLEObject) Icon() *Picture {
	return o.icon
}

// SetIcon sets the icon picture.
func (o *OLEObject) SetIcon(icon *Picture) {
	o.icon = icon
}

// MediaPosition represents a position within media playback.
type MediaPosition struct {
	Start dml.EMU // start time in EMUs (for time-based positioning)
	End   dml.EMU // end time in EMUs
}
