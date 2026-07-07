package pptx

// Picture represents a picture shape.
type Picture struct {
	BaseShape
	imagePath    string
	imageData    []byte
	contentType  string
	svgData      []byte
	svgContentType string
	relID        string
	svgRelID     string
	description  string
	cropLeft     float64
	cropRight    float64
	cropTop      float64
	cropBottom   float64
	slide        *Slide // back-reference to owning slide (set during materialization)
	// sourceID is the cNvPr id of the oxml picture this shape was materialized
	// from (0 for API-created pictures). It gives a stable identity for locating
	// the exact picture node on save, so replacing one of two pictures that
	// share an image reference updates the right one.
	sourceID uint32
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
