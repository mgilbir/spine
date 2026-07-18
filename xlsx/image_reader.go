package xlsx

import (
	"encoding/xml"

	"github.com/mgilbir/spine/opc"
)

// Image is a read-only view of an image on a worksheet. The AltText/Data/
// ContentType surface matches the image readers of the docx and pptx packages;
// the xlsx-specific anchor is the top-left cell the image is pinned to.
type Image struct {
	altText     string
	data        []byte
	contentType string
	anchorCell  string // top-left anchor cell (e.g. "B2"); "" if not cell-anchored
	widthEMU    int64  // rendered width in EMU (0 if not readily available)
	heightEMU   int64  // rendered height in EMU (0 if not readily available)
}

// AltText returns the image's alternative-text description, or "" if none.
func (i *Image) AltText() string { return i.altText }

// Data returns the raw image bytes.
func (i *Image) Data() []byte { return i.data }

// ContentType returns the image's OPC content type (e.g. "image/png").
func (i *Image) ContentType() string { return i.contentType }

// AnchorCell returns the top-left cell the image is anchored to (e.g. "B2"), or
// "" when the image is not anchored to a cell (an absolute-position anchor).
func (i *Image) AnchorCell() string { return i.anchorCell }

// WidthEMU returns the image's rendered width in English Metric Units, or 0 when
// the width is not readily available (e.g. a two-cell anchor sizes to the span).
func (i *Image) WidthEMU() int64 { return i.widthEMU }

// HeightEMU returns the image's rendered height in EMU, or 0 when not readily
// available.
func (i *Image) HeightEMU() int64 { return i.heightEMU }

// Images returns every image on the sheet: those loaded from the opened file's
// drawing part and any added this session via AddImage. The returned slice is
// nil when the sheet has no images.
func (s *Sheet) Images() []*Image {
	var out []*Image
	out = append(out, s.openedImages()...)
	out = append(out, s.pendingImages()...)
	if len(out) == 0 {
		return nil
	}
	return out
}

// pendingImages returns the images added this session via AddImage as read views.
func (s *Sheet) pendingImages() []*Image {
	if len(s.images) == 0 {
		return nil
	}
	out := make([]*Image, 0, len(s.images))
	for i := range s.images {
		img := &s.images[i]
		out = append(out, &Image{
			data:        img.data,
			contentType: img.contentType,
			anchorCell:  FormatCellRef(img.fromRow+1, img.fromCol+1),
			widthEMU:    img.widthEMU,
			heightEMU:   img.heightEMU,
		})
	}
	return out
}

// openedImages parses the sheet's drawing part (if any) and returns its images,
// resolving each blip to the media bytes via the drawing relationships.
func (s *Sheet) openedImages() []*Image {
	if s.workbook == nil || s.worksheet == nil || s.worksheet.Drawing == nil {
		return nil
	}
	drawingPart := s.resolveRelTarget(s.partName, s.worksheet.Drawing.RID)
	if drawingPart == "" {
		return nil
	}
	part, ok := s.workbook.preservedParts[drawingPart]
	if !ok {
		return nil
	}
	var wsDr xdrWsDr
	if err := xml.Unmarshal(part.Data, &wsDr); err != nil {
		return nil
	}
	anchors := make([]xdrAnchor, 0, len(wsDr.OneCell)+len(wsDr.TwoCell)+len(wsDr.AbsAnchor))
	anchors = append(anchors, wsDr.OneCell...)
	anchors = append(anchors, wsDr.TwoCell...)
	anchors = append(anchors, wsDr.AbsAnchor...)

	var out []*Image
	for i := range anchors {
		a := &anchors[i]
		if a.Pic == nil || a.Pic.BlipFill.Blip.Embed == "" {
			continue
		}
		mediaPart := s.resolveRelTarget(drawingPart, a.Pic.BlipFill.Blip.Embed)
		if mediaPart == "" {
			continue
		}
		media, ok := s.workbook.preservedParts[mediaPart]
		if !ok {
			continue
		}
		img := &Image{
			altText:     a.Pic.NvPicPr.CNvPr.Descr,
			data:        media.Data,
			contentType: media.ContentType,
		}
		if a.From != nil {
			img.anchorCell = FormatCellRef(a.From.Row+1, a.From.Col+1)
		}
		if a.Ext != nil {
			img.widthEMU = a.Ext.Cx
			img.heightEMU = a.Ext.Cy
		}
		out = append(out, img)
	}
	return out
}

// resolveRelTarget resolves a relationship id on a source part to the absolute
// part name it targets, or "" if the relationship is missing or external.
func (s *Sheet) resolveRelTarget(sourcePart, rid string) string {
	if s.workbook == nil {
		return ""
	}
	for _, rel := range s.workbook.relationships[sourcePart] {
		if rel != nil && rel.ID == rid {
			if rel.TargetMode == opc.TargetModeExternal {
				return ""
			}
			return opc.ResolvePartName(sourcePart, rel.Target)
		}
	}
	return ""
}

// ---------------------------------------------------------------------------
// Minimal SpreadsheetDrawing parse model (read-only). Only the fields needed to
// surface an image — anchor cell, extent, alt text, and the raster blip — are
// decoded; namespaces are matched by local name, and the r:embed attribute by
// the officeDocument relationships namespace URI.
// ---------------------------------------------------------------------------

type xdrWsDr struct {
	XMLName   xml.Name    `xml:"wsDr"`
	OneCell   []xdrAnchor `xml:"oneCellAnchor"`
	TwoCell   []xdrAnchor `xml:"twoCellAnchor"`
	AbsAnchor []xdrAnchor `xml:"absoluteAnchor"`
}

type xdrAnchor struct {
	From *xdrMarker `xml:"from"`
	Ext  *xdrExt    `xml:"ext"`
	Pic  *xdrPic    `xml:"pic"`
}

type xdrMarker struct {
	Col int `xml:"col"`
	Row int `xml:"row"`
}

type xdrExt struct {
	Cx int64 `xml:"cx,attr"`
	Cy int64 `xml:"cy,attr"`
}

type xdrPic struct {
	NvPicPr  xdrNvPicPr  `xml:"nvPicPr"`
	BlipFill xdrBlipFill `xml:"blipFill"`
}

type xdrNvPicPr struct {
	CNvPr xdrCNvPr `xml:"cNvPr"`
}

type xdrCNvPr struct {
	Name  string `xml:"name,attr"`
	Descr string `xml:"descr,attr"`
}

type xdrBlipFill struct {
	Blip xdrBlip `xml:"blip"`
}

type xdrBlip struct {
	Embed string `xml:"http://schemas.openxmlformats.org/officeDocument/2006/relationships embed,attr"`
}
