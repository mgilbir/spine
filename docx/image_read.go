package docx

import (
	"bytes"
	"strings"

	"github.com/mgilbir/spine/docx/internal/oxml"
	"github.com/mgilbir/spine/opc"
)

// emuToPoints converts EMU to points (914400 EMU per inch, 72 pt per inch).
func emuToPoints(emu int64) float64 {
	return float64(emu) * 72 / 914400
}

// --- read accessors ---

// AltText returns the image's alternative-text description (the drawing's
// docPr descr), or "" if it has none.
func (img *InlineImage) AltText() string { return img.altText }

// Width returns the image's display width in points.
func (img *InlineImage) Width() float64 { return emuToPoints(img.widthEMU) }

// Height returns the image's display height in points.
func (img *InlineImage) Height() float64 { return emuToPoints(img.heightEMU) }

// WidthEMU returns the image's display width in EMUs, the OOXML-native unit
// shared with the xlsx and pptx image readers.
func (img *InlineImage) WidthEMU() int64 { return img.widthEMU }

// HeightEMU returns the image's display height in EMUs.
func (img *InlineImage) HeightEMU() int64 { return img.heightEMU }

// Floating reports whether the image is floating/anchored (positioned relative
// to the page or paragraph) rather than inline in the text flow.
func (img *InlineImage) Floating() bool { return img.floating }

// PartName returns the package part name of the image's binary (e.g.
// /word/media/image1.png), or "" if the relationship cannot be resolved.
func (img *InlineImage) PartName() string {
	owner, ok := img.owningPart()
	if !ok {
		return ""
	}
	for _, rel := range img.run.paragraph.document.relationships[owner] {
		if rel != nil && rel.ID == img.relID {
			return opc.ResolvePartName(owner, rel.Target)
		}
	}
	return ""
}

// ContentType returns the image's MIME content type (e.g. image/png), or "" if
// it cannot be resolved.
func (img *InlineImage) ContentType() string {
	ct, _ := img.resolveBytes()
	return ct
}

// Data returns the image's raw bytes, or nil if they cannot be resolved.
func (img *InlineImage) Data() []byte {
	_, data := img.resolveBytes()
	return data
}

// owningPart returns the part whose relationships resolve the image's r:embed.
func (img *InlineImage) owningPart() (string, bool) {
	if img.run == nil || img.run.paragraph == nil || img.run.paragraph.document == nil {
		return "", false
	}
	return img.run.ownerPart(), true
}

// resolveBytes resolves the image part's content type and bytes across the
// mutation-API image parts, the preserved package parts, and the reader.
func (img *InlineImage) resolveBytes() (string, []byte) {
	name := img.PartName()
	if name == "" {
		return "", nil
	}
	d := img.run.paragraph.document
	for _, ip := range d.imageParts {
		if ip.partName == name {
			return ip.contentType, ip.data
		}
	}
	if part, ok := d.preservedParts[name]; ok {
		return part.ContentType, part.Data
	}
	if part, ok := d.otherParts[name]; ok {
		return part.ContentType, part.Data
	}
	if d.reader != nil {
		if f := d.reader.GetFile(name); f != nil {
			if data, err := f.ReadAll(); err == nil {
				return f.ContentType, data
			}
		}
	}
	return "", nil
}

// --- Document.Images ---

// Images returns every image in the document — inline and floating/anchored —
// in document order, including images nested in tables, headers, and footers.
func (d *Document) Images() []*InlineImage {
	var out []*InlineImage
	if d.doc() != nil && d.doc().Body != nil {
		for _, p := range d.doc().Body.AllParagraphs() {
			out = appendParagraphImages(out, &Paragraph{document: d, p: p})
		}
	}
	for name, hp := range d.headers {
		if hp == nil || hp.hdr == nil {
			continue
		}
		for _, p := range hp.hdr.AllParagraphs() {
			out = appendParagraphImages(out, &Paragraph{document: d, p: p, hfPart: name})
		}
	}
	for name, fp := range d.footers {
		if fp == nil || fp.ftr == nil {
			continue
		}
		for _, p := range fp.ftr.AllParagraphs() {
			out = appendParagraphImages(out, &Paragraph{document: d, p: p, hfPart: name})
		}
	}
	return out
}

// appendParagraphImages appends the images from a paragraph's runs (including
// runs nested in its hyperlinks and tracked-change containers) to out.
func appendParagraphImages(out []*InlineImage, p *Paragraph) []*InlineImage {
	for _, cr := range oxmlParagraphRuns(p.p) {
		run := &Run{paragraph: p, r: cr}
		for _, dr := range cr.Drawing {
			if img := parseDrawingImage(run, dr); img != nil {
				out = append(out, img)
			}
		}
	}
	return out
}

// oxmlParagraphRuns returns the runs directly in a paragraph and in its
// hyperlink / tracked-change children (the run containers that carry drawings).
func oxmlParagraphRuns(p *oxml.CT_P) []*oxml.CT_R {
	if p == nil {
		return nil
	}
	runs := append([]*oxml.CT_R(nil), p.R...)
	for _, h := range p.Hyperlink {
		if h != nil {
			runs = append(runs, h.R...)
		}
	}
	for _, ins := range p.Ins {
		if ins != nil {
			runs = append(runs, ins.R...)
		}
	}
	return runs
}

// parseDrawingImage builds an InlineImage handle from a drawing element,
// extracting the embedded relationship id, alt text, extent, and placement from
// the raw drawing XML. Returns nil for a drawing that carries no picture embed
// (e.g. a shape or chart).
func parseDrawingImage(run *Run, dr *oxml.CT_Drawing) *InlineImage {
	raw := dr.RawContent
	embeds := scanEmbedIDs(raw)
	if len(embeds) == 0 {
		return nil
	}
	// descr is read off the docPr element itself rather than from the first
	// ` descr="` anywhere in the fragment: a nested picture's cNvPr, or a text
	// run inside the drawing, can carry one too (C491).
	altText, _ := rawTagAttr(raw, "docPr", "descr", 0)
	floating, cx, cy, _ := scanDrawingGeometry(raw)
	img := &InlineImage{
		relID:   embeds[0],
		altText: xmlUnescape(altText),
		drawing: dr,
		run:     run,
		// Handles built here own existing markup: their setters patch it
		// rather than regenerate the drawing (C372).
		parsed:    true,
		drawingID: uint32(docPrID(raw)),
		floating:  floating,
		widthEMU:  cx,
		heightEMU: cy,
	}
	// A second embed on the blip is the SVG variant carried by the svgBlip
	// extension.
	if len(embeds) > 1 {
		img.svgRelID = embeds[1]
	}
	return img
}

// attrValue returns the value of the first occurrence of an attribute marker
// (e.g. ` descr="`) in raw, or "" if absent.
func attrValue(raw, marker []byte) string {
	i := bytes.Index(raw, marker)
	if i < 0 {
		return ""
	}
	rest := raw[i+len(marker):]
	j := bytes.IndexByte(rest, '"')
	if j < 0 {
		return ""
	}
	return string(rest[:j])
}

// xmlUnescape reverses xmlEscapeAttr for reading attribute values.
func xmlUnescape(s string) string {
	if !strings.Contains(s, "&") {
		return s
	}
	s = strings.ReplaceAll(s, "&lt;", "<")
	s = strings.ReplaceAll(s, "&gt;", ">")
	s = strings.ReplaceAll(s, "&quot;", `"`)
	s = strings.ReplaceAll(s, "&#39;", "'")
	s = strings.ReplaceAll(s, "&apos;", "'")
	s = strings.ReplaceAll(s, "&amp;", "&")
	return s
}
