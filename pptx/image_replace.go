package pptx

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/mgilbir/spine/common/dml"
	coxml "github.com/mgilbir/spine/common/oxml"
	xmlb "github.com/mgilbir/spine/common/xml"
	"github.com/mgilbir/spine/opc"
	oxmlpkg "github.com/mgilbir/spine/pptx/internal/oxml"
)

// processPendingImages processes any pending image replacements on picture placeholders
// and regular picture shapes.
// For placeholders, it converts the p:sp element to a p:pic element.
// For regular pictures, it updates the blip reference to point to the new image.
func (s *Slide) processPendingImages() error {
	if s.presentation == nil || s.sx() == nil || s.sx().CSld == nil || s.sx().CSld.SpTree == nil {
		return nil
	}

	for _, shape := range s.shapeCache {
		switch sh := shape.(type) {
		case *PlaceholderShape:
			if sh.hasPendingImage() {
				if err := s.replacePlaceholderWithImage(sh); err != nil {
					return err
				}
			}
		case *Picture:
			if sh.hasPendingImage() {
				if err := s.replacePictureImage(sh); err != nil {
					return err
				}
			}
		}
	}

	return nil
}

// embedImageData creates a media part and relationship for the given image data,
// returning the relationship ID that can be used as a blip embed reference.
func (s *Slide) embedImageData(data []byte, contentType string) string {
	return s.presentation.embedImageForPart(s.partName, data, contentType)
}

// embedImageForPart stores image data as a /ppt/media part (reusing an existing
// part with identical bytes rather than storing the same image twice) and adds
// an image relationship from ownerPart to it, returning the relationship id
// usable as a blip r:embed. It is the part-agnostic core shared by slide image
// embedding and slide-layout / slide-master image backgrounds.
func (p *Presentation) embedImageForPart(ownerPart string, data []byte, contentType string) string {
	mediaName := p.embedImagePart(data, contentType)
	relID := p.nextRelIDForPart(ownerPart)
	p.addImageRel(ownerPart, mediaName, relID)
	return relID
}

// embedImagePart stores image data as a /ppt/media part (reusing an existing
// part with identical bytes) and returns the part name. It creates no
// relationship — callers that need control over the relationship id (a master
// whose id space is shared with its layout rels) allocate it and call
// addImageRel themselves.
func (p *Presentation) embedImagePart(data []byte, contentType string) string {
	for name, part := range p.otherParts {
		if part != nil && strings.HasPrefix(name, "/ppt/media/") && bytes.Equal(part.Data, data) {
			return name
		}
	}
	name := p.nextMediaName(extFromContentType(contentType))
	p.otherParts[name] = &coxml.RawPart{ContentType: contentType, Data: data}
	return name
}

// addImageRel appends an image relationship (id relID) from ownerPart to the
// media part mediaName.
func (p *Presentation) addImageRel(ownerPart, mediaName, relID string) {
	p.relationships[ownerPart] = append(p.relationships[ownerPart], &opc.Relationship{
		ID:         relID,
		Type:       opc.RelTypeImage,
		Target:     relativeTarget(ownerPart, mediaName),
		TargetMode: opc.TargetModeInternal,
	})
}

// replacePlaceholderWithImage converts a placeholder p:sp element to a p:pic element
// and sets up the image relationship and media part.
func (s *Slide) replacePlaceholderWithImage(ph *PlaceholderShape) error {
	spTree := s.sx().CSld.SpTree

	// Find the oxml.Shape that matches this placeholder (by index and type)
	spIdx := -1
	for i, sp := range spTree.Sp {
		if sp.NvSpPr != nil && sp.NvSpPr.NvPr != nil && sp.NvSpPr.NvPr.Ph != nil {
			xmlPh := sp.NvSpPr.NvPr.Ph
			if xmlPh.Idx == ph.idx && xmlPh.Type == string(ph.phType) {
				spIdx = i
				break
			}
		}
	}

	if spIdx < 0 {
		return fmt.Errorf("pptx: placeholder (type=%q, idx=%d) not found in slide XML", ph.phType, ph.idx)
	}

	origSp := spTree.Sp[spIdx]

	// Embed the raster fallback and optional SVG payload.
	rasterRelID := s.embedImageData(ph.pendingImageData, ph.pendingImageCT)
	var svgRelID string
	if len(ph.pendingSVGData) > 0 {
		svgRelID = s.embedImageData(ph.pendingSVGData, ph.pendingSVGCT)
	}

	// Build the p:pic element
	pic := buildPicFromPlaceholder(origSp, rasterRelID, svgRelID)

	// Replace the p:sp with a p:pic in the shape tree:
	// 1. Remove the sp from the Sp slice
	spTree.Sp = append(spTree.Sp[:spIdx], spTree.Sp[spIdx+1:]...)

	// 2. Add the pic to the Pic slice
	picIdx := len(spTree.Pic)
	spTree.Pic = append(spTree.Pic, pic)

	// 3. Update childOrder to replace the sp reference with a pic reference
	for i, ref := range spTree.ChildOrder() {
		if ref.Kind == oxmlpkg.ChildSp && ref.Index == spIdx {
			spTree.SetChildRef(i, oxmlpkg.ChildRef{Kind: oxmlpkg.ChildPic, Index: picIdx})
			break
		}
	}

	// 4. Update childOrder indices: any sp references with index > spIdx need to be decremented
	for i, ref := range spTree.ChildOrder() {
		if ref.Kind == oxmlpkg.ChildSp && ref.Index > spIdx {
			spTree.SetChildRef(i, oxmlpkg.ChildRef{Kind: oxmlpkg.ChildSp, Index: ref.Index - 1})
		}
	}

	// 5. Apply the same remap to the slide's shapeRefs, which mirror the tree
	// across save cycles: the placeholder's ref now points at the pic, and sp
	// refs above the removed index shift down.
	for i, ref := range s.shapeRefs {
		switch {
		case ref.Kind == oxmlpkg.ChildSp && ref.Index == spIdx:
			s.shapeRefs[i] = oxmlpkg.ChildRef{Kind: oxmlpkg.ChildPic, Index: picIdx}
		case ref.Kind == oxmlpkg.ChildSp && ref.Index > spIdx:
			s.shapeRefs[i] = oxmlpkg.ChildRef{Kind: oxmlpkg.ChildSp, Index: ref.Index - 1}
		}
	}

	// Clear the pending image data
	ph.pendingImageData = nil
	ph.pendingImagePath = ""
	ph.pendingImageCT = ""
	ph.pendingSVGData = nil
	ph.pendingSVGCT = ""

	return nil
}

// buildPicFromPlaceholder creates an oxml.Picture element from a placeholder oxml.Shape,
// preserving the placeholder info, position, and size.
func buildPicFromPlaceholder(sp *oxmlpkg.Shape, rasterRelID, svgRelID string) *oxmlpkg.Picture {
	blip := &dml.Blip{Embed: rasterRelID}
	if svgRelID != "" {
		blip.ExtLst = &dml.ExtLst{Ext: []*dml.Ext{{
			URI:     xmlb.ExtURISvgBlip,
			SvgBlip: &dml.ASvgBlip{Embed: svgRelID},
		}}}
	}

	pic := &oxmlpkg.Picture{
		NvPicPr: &oxmlpkg.NvPicPr{
			CNvPr: &dml.CNvPr{
				Id:   sp.NvSpPr.CNvPr.Id,
				Name: sp.NvSpPr.CNvPr.Name,
			},
			CNvPicPr: &dml.CNvPicPr{
				PicLocks: &dml.PicLocks{NoChangeAspect: true},
			},
			NvPr: &oxmlpkg.NvPr{
				// Preserve placeholder info so layout inheritance still works
				Ph: sp.NvSpPr.NvPr.Ph,
			},
		},
		BlipFill: &dml.BlipFill{
			Blip: blip,
			Stretch: &dml.Stretch{
				FillRect: &dml.RelRect{},
			},
		},
		SpPr: sp.SpPr, // Preserve original position, size, and transforms
	}

	return pic
}

// replacePictureImage updates an existing p:pic element to point to new image data.
// Unlike placeholder replacement, the p:pic element already exists — we just swap the
// blip reference and create a new media part + relationship.
func (s *Slide) replacePictureImage(pic *Picture) error {
	spTree := s.sx().CSld.SpTree

	// Prefer the stable cNvPr id captured at materialization: two pictures can
	// share the same blip embed (the same image), so matching on the embed alone
	// could update the wrong node.
	var oxmlPic *oxmlpkg.Picture
	if pic.sourceID != 0 {
		for _, op := range spTree.Pic {
			if op.NvPicPr != nil && op.NvPicPr.CNvPr != nil && op.NvPicPr.CNvPr.Id == pic.sourceID {
				oxmlPic = op
				break
			}
		}
	}

	// Fallback: match by current blip embed (covers API-created pictures).
	if oxmlPic == nil {
		for _, op := range spTree.Pic {
			if op.BlipFill != nil && op.BlipFill.Blip != nil && op.BlipFill.Blip.Embed == pic.relID {
				oxmlPic = op
				break
			}
		}
	}

	// Fallback: match by name if relID didn't match
	if oxmlPic == nil {
		for _, op := range spTree.Pic {
			if op.NvPicPr != nil && op.NvPicPr.CNvPr != nil && op.NvPicPr.CNvPr.Name == pic.name {
				oxmlPic = op
				break
			}
		}
	}

	if oxmlPic == nil {
		return fmt.Errorf("pptx: picture %q (relID=%q) not found in slide XML", pic.name, pic.relID)
	}

	// Embed the raster fallback and update the primary blip reference.
	relID := s.embedImageData(pic.imageData, pic.contentType)

	// Update the blip reference in the existing p:pic element
	if oxmlPic.BlipFill == nil {
		oxmlPic.BlipFill = &dml.BlipFill{}
	}
	if oxmlPic.BlipFill.Blip == nil {
		oxmlPic.BlipFill.Blip = &dml.Blip{}
	}
	oxmlPic.BlipFill.Blip.Embed = relID
	if len(pic.svgData) > 0 {
		svgRelID := s.embedImageData(pic.svgData, pic.svgContentType)
		setBlipSVGExtension(oxmlPic.BlipFill.Blip, svgRelID)
		pic.svgRelID = svgRelID
	} else {
		removeBlipSVGExtension(oxmlPic.BlipFill.Blip)
		pic.svgRelID = ""
	}

	// Sync geometry from the domain shape: loaded slides keep their parsed
	// XML, so a SetPosition/SetSize on the Go Picture would otherwise be lost
	// (writing the parsed-back values is an identity for untouched shapes).
	if oxmlPic.SpPr == nil {
		oxmlPic.SpPr = &dml.SpPr{}
	}
	if oxmlPic.SpPr.Xfrm == nil {
		oxmlPic.SpPr.Xfrm = &dml.Xfrm{}
	}
	oxmlPic.SpPr.Xfrm.Off = &dml.OffXML{X: int64(pic.x), Y: int64(pic.y)}
	oxmlPic.SpPr.Xfrm.Ext = &dml.ExtXML{Cx: int64(pic.width), Cy: int64(pic.height)}

	// Update the Go-level relID
	pic.relID = relID

	// Clear the pending image data
	pic.imageData = nil
	pic.imagePath = ""
	pic.svgData = nil
	pic.svgContentType = ""

	return nil
}

func setBlipSVGExtension(blip *dml.Blip, svgRelID string) {
	if blip == nil {
		return
	}
	if blip.ExtLst == nil {
		blip.ExtLst = &dml.ExtLst{}
	}
	svgExt := &dml.Ext{
		URI:     xmlb.ExtURISvgBlip,
		SvgBlip: &dml.ASvgBlip{Embed: svgRelID},
	}
	for i, ext := range blip.ExtLst.Ext {
		if ext != nil && ext.URI == xmlb.ExtURISvgBlip {
			blip.ExtLst.Ext[i] = svgExt
			return
		}
	}
	blip.ExtLst.Ext = append(blip.ExtLst.Ext, svgExt)
}

func removeBlipSVGExtension(blip *dml.Blip) {
	if blip == nil || blip.ExtLst == nil {
		return
	}
	filtered := blip.ExtLst.Ext[:0]
	for _, ext := range blip.ExtLst.Ext {
		if ext != nil && ext.URI == xmlb.ExtURISvgBlip {
			continue
		}
		filtered = append(filtered, ext)
	}
	if len(filtered) == 0 {
		blip.ExtLst = nil
		return
	}
	blip.ExtLst.Ext = filtered
}

// nextMediaName generates a unique media part name for the presentation.
func (p *Presentation) nextMediaName(ext string) string {
	for i := 1; ; i++ {
		name := fmt.Sprintf("/ppt/media/image%d%s", i, ext)
		if _, exists := p.otherParts[name]; !exists {
			return name
		}
	}
}

// nextRelID generates a unique relationship ID for this slide's relationships.
func (s *Slide) nextRelID() string {
	return s.presentation.nextRelIDForPart(s.partName)
}

// nextRelIDForPart returns an rId that is unused in the given part's
// relationship set (one greater than the highest existing rIdN).
func (p *Presentation) nextRelIDForPart(partName string) string {
	maxID := 0
	for _, r := range p.relationships[partName] {
		if len(r.ID) > 3 && r.ID[:3] == "rId" {
			var id int
			if _, err := fmt.Sscanf(r.ID, "rId%d", &id); err == nil {
				if id > maxID {
					maxID = id
				}
			}
		}
	}
	return fmt.Sprintf("rId%d", maxID+1)
}

// relativeTarget computes a relative target path from a source part to a target part.
// For example, from "/ppt/slides/slide1.xml" to "/ppt/media/image1.png" -> "../media/image1.png"
func relativeTarget(sourcePart, targetPart string) string {
	sourceDir := sourcePart[:strings.LastIndex(sourcePart, "/")+1]
	targetDir := targetPart[:strings.LastIndex(targetPart, "/")+1]
	targetFile := targetPart[strings.LastIndex(targetPart, "/")+1:]

	// Count how many directories we need to go up from source to find common prefix
	if sourceDir == targetDir {
		return targetFile
	}

	// Simple approach: find common prefix, then compute relative path
	sourceParts := strings.Split(strings.Trim(sourceDir, "/"), "/")
	targetParts := strings.Split(strings.Trim(targetDir, "/"), "/")

	commonLen := 0
	for i := 0; i < len(sourceParts) && i < len(targetParts); i++ {
		if sourceParts[i] == targetParts[i] {
			commonLen = i + 1
		} else {
			break
		}
	}

	ups := len(sourceParts) - commonLen
	var result strings.Builder
	for i := 0; i < ups; i++ {
		result.WriteString("../")
	}
	for i := commonLen; i < len(targetParts); i++ {
		result.WriteString(targetParts[i])
		result.WriteString("/")
	}
	result.WriteString(targetFile)

	return result.String()
}
