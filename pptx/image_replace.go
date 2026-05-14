package pptx

import (
	"fmt"
	"strings"

	"github.com/mgilbir/spine/common/dml"
	coxml "github.com/mgilbir/spine/common/oxml"
	"github.com/mgilbir/spine/opc"
	oxmlpkg "github.com/mgilbir/spine/pptx/internal/oxml"
)

// processPendingImages processes any pending image replacements on picture placeholders
// and regular picture shapes.
// For placeholders, it converts the p:sp element to a p:pic element.
// For regular pictures, it updates the blip reference to point to the new image.
func (s *Slide) processPendingImages() error {
	if s.presentation == nil || s.slideXML == nil || s.slideXML.CSld == nil || s.slideXML.CSld.SpTree == nil {
		return nil
	}

	for _, shape := range s.shapes {
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
	p := s.presentation

	ext := extFromContentType(contentType)
	mediaName := s.nextMediaName(ext)

	// Store the image data as a media part in the presentation
	p.otherParts[mediaName] = &coxml.RawPart{
		ContentType: contentType,
		Data:        data,
	}

	// Create a relationship for this slide pointing to the media part
	relID := s.nextRelID()
	mediaTarget := relativeTarget(s.partName, mediaName)
	rel := &opc.Relationship{
		ID:         relID,
		Type:       opc.RelTypeImage,
		Target:     mediaTarget,
		TargetMode: opc.TargetModeInternal,
	}
	p.relationships[s.partName] = append(p.relationships[s.partName], rel)

	return relID
}

// replacePlaceholderWithImage converts a placeholder p:sp element to a p:pic element
// and sets up the image relationship and media part.
func (s *Slide) replacePlaceholderWithImage(ph *PlaceholderShape) error {
	spTree := s.slideXML.CSld.SpTree

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

	// Embed the image and get the relationship ID
	relID := s.embedImageData(ph.pendingImageData, ph.pendingImageCT)

	// Build the p:pic element
	pic := buildPicFromPlaceholder(origSp, relID)

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

	// Clear the pending image data
	ph.pendingImageData = nil
	ph.pendingImagePath = ""
	ph.pendingImageCT = ""

	return nil
}

// buildPicFromPlaceholder creates an oxml.Picture element from a placeholder oxml.Shape,
// preserving the placeholder info, position, and size.
func buildPicFromPlaceholder(sp *oxmlpkg.Shape, relID string) *oxmlpkg.Picture {
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
			Blip: &dml.Blip{
				Embed: relID,
			},
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
	spTree := s.slideXML.CSld.SpTree

	// Find the oxml.Picture that matches this Picture shape (by old relID or name)
	var oxmlPic *oxmlpkg.Picture
	for _, op := range spTree.Pic {
		if op.BlipFill != nil && op.BlipFill.Blip != nil && op.BlipFill.Blip.Embed == pic.relID {
			oxmlPic = op
			break
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

	// Embed the image and get the relationship ID
	relID := s.embedImageData(pic.imageData, pic.contentType)

	// Update the blip reference in the existing p:pic element
	if oxmlPic.BlipFill == nil {
		oxmlPic.BlipFill = &dml.BlipFill{}
	}
	if oxmlPic.BlipFill.Blip == nil {
		oxmlPic.BlipFill.Blip = &dml.Blip{}
	}
	oxmlPic.BlipFill.Blip.Embed = relID

	// Update the Go-level relID
	pic.relID = relID

	// Clear the pending image data
	pic.imageData = nil
	pic.imagePath = ""

	return nil
}

// nextMediaName generates a unique media part name for the presentation.
func (s *Slide) nextMediaName(ext string) string {
	p := s.presentation
	for i := 1; ; i++ {
		name := fmt.Sprintf("/ppt/media/image%d%s", i, ext)
		if _, exists := p.otherParts[name]; !exists {
			return name
		}
	}
}

// nextRelID generates a unique relationship ID for this slide's relationships.
func (s *Slide) nextRelID() string {
	p := s.presentation
	rels := p.relationships[s.partName]

	maxID := 0
	for _, r := range rels {
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
