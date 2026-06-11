# SVG PPTX Embedding Plan

## Goal

Add first-class support in Spine for embedding SVG images into PPTX slides using the Office SVG pattern:

- a fallback raster image referenced by the normal `a:blip r:embed="..."`
- an SVG relationship referenced by `a:extLst/a:ext/asvg:svgBlip`

This allows modern PowerPoint versions to render the SVG while older versions fall back to the raster image.

## Current State

Spine already has most of the XML support in place:

- `common/xml/namespace.go`
  - `ExtURISvgBlip`
  - `NSDrawingSVG2016`
  - `PrefixDrawingSVG2016`
- `common/dml/xml_extension.go`
  - `ASvgBlip`
  - `Ext.SvgBlip`
  - marshal/unmarshal support for `SvgBlip`
- `pptx/placeholder.go`
  - `contentTypeFromExt(".svg") -> image/svg+xml`
  - `extFromContentType("image/svg+xml") -> .svg`
- `pptx/image_replace.go`
  - `embedImageData()` already stores arbitrary media parts and creates relationships

What is missing is the write path that embeds both the fallback image and the SVG, and wires the SVG relationship into the picture blip extension list.

## Relevant Files

- `pptx/media.go`
- `pptx/placeholder.go`
- `pptx/image_replace.go`
- `pptx/oxml_to_domain.go`
- `pptx/oxml_to_domain_test.go`
- `common/xml/namespace.go` (already complete, reference only)
- `common/dml/xml_extension.go` (already complete, reference only)

## Required Behavior

When callers set an SVG image on a `Picture` or picture `PlaceholderShape`, Spine should:

1. Embed a fallback raster image part, typically PNG
2. Embed the SVG as a second media part
3. Create two slide relationships
4. Set the fallback relationship on `a:blip@r:embed`
5. Add this extension under `a:blip/a:extLst`

```xml
<a:ext uri="{96DAC541-7B7A-43D3-8B79-37D633B846F1}">
  <asvg:svgBlip r:embed="rIdX"/>
</a:ext>
```

## Implementation Details

### 1. Extend `pptx.Picture`

File: `pptx/media.go`

Add fields to `Picture`:

```go
svgData        []byte
svgContentType string
```

Keep the existing raster fields as the fallback image:

- `imageData`
- `contentType`

Add API methods:

```go
func (p *Picture) SetSVGImageData(svgData, fallbackData []byte, fallbackCT string)
func (p *Picture) SetSVGData(svgData []byte)
```

Behavior:

- `SetSVGImageData` stores the SVG plus a caller-provided fallback image
- `SetSVGData` stores the SVG and uses a built-in minimal transparent PNG fallback

Add a package-level fallback PNG, for example:

```go
var minimalTransparentPNG = []byte{...}
```

This fallback only exists to satisfy Office's dual-image pattern. It does not need to be visually meaningful.
It does need to be a valid image, for example a 1x1 transparent PNG. Office may reject a zero-byte or malformed fallback image.

### 2. Extend `pptx.PlaceholderShape`

File: `pptx/placeholder.go`

Add fields:

```go
pendingSVGData []byte
pendingSVGCT   string
```

Add API methods:

```go
func (p *PlaceholderShape) SetSVGImageData(svgData, fallbackData []byte, fallbackCT string) error
func (p *PlaceholderShape) SetSVGData(svgData []byte) error
```

Behavior:

- only valid for `PlaceholderPicture`
- stores SVG data in the new SVG fields
- stores fallback raster in existing pending image fields
- `SetSVGData` uses `minimalTransparentPNG` as the fallback

This should mirror the existing `SetImageData` behavior and error handling.

### 3. Update Placeholder Replacement Path

File: `pptx/image_replace.go`

Function: `replacePlaceholderWithImage`

Current behavior:

- finds the placeholder `p:sp`
- embeds a single image via `embedImageData`
- converts the shape to `p:pic`

Required change:

- embed the fallback raster first and keep its relationship id as `rasterRelID`
- if `pendingSVGData` is present, embed the SVG too and keep its relationship id as `svgRelID`
- pass both ids into `buildPicFromPlaceholder`

Pseudo-shape:

```go
rasterRelID := s.embedImageData(ph.pendingImageData, ph.pendingImageCT)

var svgRelID string
if len(ph.pendingSVGData) > 0 {
    svgRelID = s.embedImageData(ph.pendingSVGData, ph.pendingSVGCT)
}

pic := buildPicFromPlaceholder(origSp, rasterRelID, svgRelID)
```

Also clear the SVG pending fields after replacement.

### 4. Update `buildPicFromPlaceholder`

File: `pptx/image_replace.go`

Change the signature from:

```go
func buildPicFromPlaceholder(sp *oxmlpkg.Shape, relID string) *oxmlpkg.Picture
```

to:

```go
func buildPicFromPlaceholder(sp *oxmlpkg.Shape, rasterRelID, svgRelID string) *oxmlpkg.Picture
```

Set the normal blip embed to `rasterRelID`.

If `svgRelID != ""`, attach this extension to `pic.BlipFill.Blip.ExtLst`:

```go
&dml.ExtLst{
    Ext: []*dml.Ext{{
        URI:     xmlb.ExtURISvgBlip,
        SvgBlip: &dml.ASvgBlip{Embed: svgRelID},
    }},
}
```

This is safe here because `buildPicFromPlaceholder` creates a fresh `p:pic` element, so there are no pre-existing blip extensions to preserve.

This file will need:

```go
xmlb "github.com/mgilbir/spine/common/xml"
```

### 5. Update Existing Picture Replacement Path

File: `pptx/image_replace.go`

Function: `replacePictureImage`

Current behavior:

- embeds one image
- updates `oxmlPic.BlipFill.Blip.Embed`

Required change:

- embed fallback raster into `rasterRelID`
- set `oxmlPic.BlipFill.Blip.Embed = rasterRelID`
- if `pic.svgData` exists, embed SVG and attach or update the `SvgBlip` extension on the blip without discarding unrelated existing extensions

Pseudo-shape:

```go
rasterRelID := s.embedImageData(pic.imageData, pic.contentType)
oxmlPic.BlipFill.Blip.Embed = rasterRelID

if len(pic.svgData) > 0 {
    svgRelID := s.embedImageData(pic.svgData, pic.svgContentType)
    svgExt := &dml.Ext{
        URI:     xmlb.ExtURISvgBlip,
        SvgBlip: &dml.ASvgBlip{Embed: svgRelID},
    }

    if oxmlPic.BlipFill.Blip.ExtLst == nil {
        oxmlPic.BlipFill.Blip.ExtLst = &dml.ExtLst{}
    }

    replaced := false
    for i, ext := range oxmlPic.BlipFill.Blip.ExtLst.Ext {
        if ext != nil && ext.URI == xmlb.ExtURISvgBlip {
            oxmlPic.BlipFill.Blip.ExtLst.Ext[i] = svgExt
            replaced = true
            break
        }
    }
    if !replaced {
        oxmlPic.BlipFill.Blip.ExtLst.Ext = append(oxmlPic.BlipFill.Blip.ExtLst.Ext, svgExt)
    }
}
```

Also clear SVG pending fields after replacement.

Do not replace the entire `ExtLst` when updating an existing picture. Real PPTX files may already carry unrelated blip extensions such as `a14:useLocalDpi`, and those should be preserved.

### 6. Read Existing SVG Metadata When Loading PPTX

File: `pptx/oxml_to_domain.go`

Current behavior in `oxmlPictureToGoPicture`:

- reads fallback `Blip.Embed` into `p.relID`
- reads crop info

Required change:

- if `pic.BlipFill.Blip.ExtLst` contains an extension with `SvgBlip`, store the SVG relationship id on the Go `Picture`

Suggested new field on `Picture`:

```go
svgRelID string
```

This is not required for initial write-only support, but it is useful for round-trip correctness and future SVG-aware replacement behavior.

If you add it, parse:

```go
if pic.BlipFill != nil && pic.BlipFill.Blip != nil && pic.BlipFill.Blip.ExtLst != nil {
    for _, ext := range pic.BlipFill.Blip.ExtLst.Ext {
        if ext != nil && ext.SvgBlip != nil {
            p.svgRelID = ext.SvgBlip.Embed
            break
        }
    }
}
```

### 7. Ensure New Pictures Follow the Same Save Path

No separate new-picture path should be needed.

Current flow already works for newly added `Picture` shapes:

1. `slide.AddShape(pic)` sets the slide back-reference
2. `syncShapesToXML()` creates the `p:pic`
3. `processPendingImages()` calls `replacePictureImage()` because `hasPendingImage()` is true and `slide != nil`

The SVG support should be implemented in `replacePictureImage()` so it works for both:

- pictures loaded from an existing file
- brand new pictures added in Go

### 8. Tests

Add or extend tests in `pptx/oxml_to_domain_test.go` and/or `pptx/media_test.go`.

Recommended coverage:

1. `Picture.SetSVGData`
   - stores SVG bytes
   - populates fallback PNG
   - preserves `ShapeTypePicture`

2. `PlaceholderShape.SetSVGData`
   - works for `PlaceholderPicture`
   - returns `ErrNotPicturePlaceholder` for non-picture placeholders

3. New picture end-to-end
   - create a presentation
   - add a new `Picture`
   - call `SetSVGData` or `SetSVGImageData`
   - save and reopen
   - assert slide has two image relationships
   - assert the picture blip has `ExtLst -> SvgBlip`

4. Placeholder end-to-end
   - create a picture placeholder
   - call `SetSVGData`
   - save and reopen
   - assert placeholder became a picture
   - assert fallback and SVG relationships both exist
   - assert `SvgBlip` extension is written

5. Optional round-trip test
   - open a PPTX containing an SVG picture
   - assert `svgRelID` is loaded if implemented

## Notes / Constraints

- Use the existing `embedImageData()` helper for both fallback and SVG parts
- The normal `a:blip@r:embed` must always point to the raster fallback
- The SVG must only be referenced through the Office extension list
- Do not replace the normal blip embed with the SVG relationship id
- No changes should be required in `common/dml/xml_extension.go` unless a gap is discovered during tests
- No changes should be required in `common/xml/namespace.go`
- Existing blip extensions on loaded pictures should be preserved when adding or updating `SvgBlip`
- `replacePictureImage` currently only searches top-level `spTree.Pic`. Pictures nested inside group shapes are a pre-existing limitation and are out of scope for this initial SVG change

## Expected Public API After Change

On `*pptx.Picture`:

```go
func (p *Picture) SetSVGImageData(svgData, fallbackData []byte, fallbackCT string)
func (p *Picture) SetSVGData(svgData []byte)
```

On `*pptx.PlaceholderShape`:

```go
func (p *PlaceholderShape) SetSVGImageData(svgData, fallbackData []byte, fallbackCT string) error
func (p *PlaceholderShape) SetSVGData(svgData []byte) error
```

## Suggested Validation

After implementation:

1. Run `go test ./...` in the Spine repo
2. Create a minimal PPTX with one SVG image
3. Open it in PowerPoint desktop
4. Confirm the image displays normally
5. Re-save from PowerPoint and verify the file still round-trips through Spine

## Optional Follow-Up

If later needed, Spine could expose a higher-level API that accepts only SVG and internally generates a better raster fallback instead of the minimal transparent PNG. That is not required for the initial implementation because modern PowerPoint should use the SVG path.
