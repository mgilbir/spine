package docx

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"image"
	// Register the decoders used to size an image watermark and sniff its
	// content type from the raw bytes.
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"strings"

	xmlb "github.com/mgilbir/spine/common/xml"
	"github.com/mgilbir/spine/docx/internal/oxml"
	"github.com/mgilbir/spine/opc"
)

// Watermark namespace URIs used by the VML shapes that carry a watermark. The
// w:pict element declares them inline (the header root only declares w:, r: and
// mc:), so the shape renders regardless of what the header part happened to
// declare.
const (
	nsVML       = "urn:schemas-microsoft-com:vml"
	nsOfficeVML = "urn:schemas-microsoft-com:office:office"
	nsWordVML   = "urn:schemas-microsoft-com:office:word"
)

// WatermarkType classifies a detected watermark.
type WatermarkType int

const (
	// WatermarkNone means no watermark was detected.
	WatermarkNone WatermarkType = iota
	// WatermarkText is a WordArt text watermark (VML v:textpath).
	WatermarkText
	// WatermarkImage is a washed-out image watermark (VML v:imagedata).
	WatermarkImage
)

// Watermark reports a detected watermark. Text is set for text watermarks.
type Watermark struct {
	Type WatermarkType
	Text string
}

// WatermarkOptions configures a text or image watermark. The zero value is
// valid: a horizontal, silver watermark in Calibri.
type WatermarkOptions struct {
	// Font is the font family for a text watermark. Defaults to "Calibri".
	Font string
	// Color is the fill color of a text watermark as a hex RGB string
	// (e.g. "C0C0C0" or "#C0C0C0"). Defaults to silver ("C0C0C0"). Ignored for
	// image watermarks.
	Color string
	// Diagonal lays the watermark out on a 45° diagonal (the classic Word
	// look). Ignored when Rotation is set.
	Diagonal bool
	// Rotation rotates the shape by this many degrees clockwise. When zero,
	// Diagonal decides the angle (315° when set, otherwise horizontal).
	Rotation float64
	// DrawingML emits a text watermark as a DrawingML text box wrapped in an
	// mc:AlternateContent whose fallback is the classic VML shape, matching the
	// form newer Word versions write. Consumers that understand DrawingML
	// (Requires="wps") render the text box; older ones fall back to the VML.
	// Ignored for image watermarks, which remain VML-only.
	DrawingML bool
}

// rotationDegrees resolves the effective clockwise rotation for the options.
func (o WatermarkOptions) rotationDegrees() float64 {
	if o.Rotation != 0 {
		return o.Rotation
	}
	if o.Diagonal {
		return 315
	}
	return 0
}

// SetTextWatermark stamps a WordArt text watermark across the document's header
// furniture. The watermark is inserted into the default header (created when the
// document has none) and into any first-page and even-page headers already
// referenced by the default section, so it shows on every page. Calling it again
// replaces the existing watermark.
func (d *Document) SetTextWatermark(text string, opts WatermarkOptions) error {
	headers, err := d.watermarkTargetHeaders()
	if err != nil {
		return err
	}
	d.applyTextWatermark(headers, text, opts)
	return nil
}

// SetSectionTextWatermark stamps a WordArt text watermark across the headers of
// a specific section (as returned by Document.Sections or DefaultSection),
// rather than the document's final section. This allows distinct watermarks per
// section. The default header of the section is created when absent; existing
// first-page and even-page headers of the section are covered too. Calling it
// again replaces the section's existing watermark.
func (d *Document) SetSectionTextWatermark(sec *Section, text string, opts WatermarkOptions) error {
	if sec == nil || sec.sectPr == nil {
		return fmt.Errorf("docx: nil section")
	}
	d.applyTextWatermark(d.watermarkTargetHeadersFor(sec.sectPr), text, opts)
	return nil
}

// applyTextWatermark replaces any watermark in each header with a fresh text
// watermark shape and flags the header parts for regeneration.
func (d *Document) applyTextWatermark(headers []*Header, text string, opts WatermarkOptions) {
	for _, h := range headers {
		removeWatermarkParagraphs(h.hdr)
		seq := d.nextWatermarkSeq()
		if opts.DrawingML {
			ac := buildTextWatermarkDrawingML(text, opts, seq)
			appendAltContentParagraph(h.hdr, ac)
		} else {
			pict := buildTextWatermarkPict(text, opts, seq)
			appendPictParagraph(h.hdr, pict)
		}
		d.markHdrFtrModified(h.partName)
	}
}

// SetImageWatermark stamps a washed-out image watermark across the document's
// header furniture (see SetTextWatermark for the header selection). The image
// content type and dimensions are read from imageBytes; PNG, JPEG and GIF are
// supported. Calling it again replaces the existing watermark.
func (d *Document) SetImageWatermark(imageBytes []byte, opts WatermarkOptions) error {
	if len(imageBytes) == 0 {
		return fmt.Errorf("docx: image watermark data is empty")
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(imageBytes))
	if err != nil {
		return fmt.Errorf("docx: decoding image watermark: %w", err)
	}
	contentType, ext := watermarkImageType(format)
	if contentType == "" {
		return fmt.Errorf("docx: unsupported image watermark format: %s", format)
	}
	widthPt, heightPt := fitWatermarkImage(cfg.Width, cfg.Height)

	headers, err := d.watermarkTargetHeaders()
	if err != nil {
		return err
	}
	d.applyImageWatermark(headers, imageBytes, contentType, ext, widthPt, heightPt)
	return nil
}

// SetSectionImageWatermark stamps a washed-out image watermark across the
// headers of a specific section (see SetSectionTextWatermark for the header
// selection). Calling it again replaces the section's existing watermark.
func (d *Document) SetSectionImageWatermark(sec *Section, imageBytes []byte, opts WatermarkOptions) error {
	if sec == nil || sec.sectPr == nil {
		return fmt.Errorf("docx: nil section")
	}
	if len(imageBytes) == 0 {
		return fmt.Errorf("docx: image watermark data is empty")
	}
	cfg, format, err := image.DecodeConfig(bytes.NewReader(imageBytes))
	if err != nil {
		return fmt.Errorf("docx: decoding image watermark: %w", err)
	}
	contentType, ext := watermarkImageType(format)
	if contentType == "" {
		return fmt.Errorf("docx: unsupported image watermark format: %s", format)
	}
	widthPt, heightPt := fitWatermarkImage(cfg.Width, cfg.Height)
	d.applyImageWatermark(d.watermarkTargetHeadersFor(sec.sectPr), imageBytes, contentType, ext, widthPt, heightPt)
	return nil
}

// applyImageWatermark replaces any watermark in each header with a fresh image
// watermark shape and flags the header parts for regeneration.
func (d *Document) applyImageWatermark(headers []*Header, imageBytes []byte, contentType, ext string, widthPt, heightPt float64) {
	for _, h := range headers {
		removeWatermarkParagraphs(h.hdr)
		// Each header part resolves relationships in its own scope, so register
		// a relationship per part; the media bytes are deduplicated by
		// registerImagePart, so all headers share one media part.
		relID, _ := d.registerImagePart(h.partName, imageBytes, contentType, ext)
		pict := buildImageWatermarkPict(relID, widthPt, heightPt, d.nextWatermarkSeq())
		appendPictParagraph(h.hdr, pict)
		d.markHdrFtrModified(h.partName)
	}
}

// Watermark returns the watermark detected in the document's header furniture,
// or nil when there is none. Text and image watermarks are recognized; for text
// watermarks the Text field carries the watermark string.
func (d *Document) Watermark() *Watermark {
	for _, hdr := range d.watermarkHeaders() {
		for _, p := range hdr.Paragraphs() {
			for _, raw := range paragraphPictContents(p) {
				if wm := classifyWatermark(raw); wm != nil {
					return wm
				}
			}
		}
	}
	return nil
}

// RemoveWatermark removes any watermark from the document's header furniture. It
// reports whether a watermark was found and removed.
func (d *Document) RemoveWatermark() bool {
	removed := false
	for name, hp := range d.headers {
		if hp == nil || hp.hdr == nil {
			continue
		}
		if removeWatermarkParagraphs(hp.hdr) {
			d.markHdrFtrModified(name)
			removed = true
		}
	}
	return removed
}

// watermarkHeaders returns every parsed header model in the document, used for
// read-only watermark detection.
// Headers are visited in part-name order: Watermark() returns the *first*
// watermark it finds, so map order would make the answer depend on Go's
// randomized map iteration for a document whose headers carry different
// watermarks (C497).
func (d *Document) watermarkHeaders() []*oxml.CT_HdrFtr {
	headers := make([]*oxml.CT_HdrFtr, 0, len(d.headers))
	for _, hp := range d.sortedHeaderParts() {
		if hp != nil && hp.hdr != nil {
			headers = append(headers, hp.hdr)
		}
	}
	return headers
}

// watermarkTargetHeaders returns the header handles a watermark is applied to
// for the document's final (default) section (see watermarkTargetHeadersFor).
func (d *Document) watermarkTargetHeaders() ([]*Header, error) {
	if d.doc() == nil {
		return nil, fmt.Errorf("docx: document has no body")
	}
	if d.doc().Body == nil {
		d.doc().Body = &oxml.CT_Body{}
	}
	if d.doc().Body.SectPr == nil {
		d.doc().Body.SectPr = &oxml.CT_SectPr{}
	}
	return d.watermarkTargetHeadersFor(d.doc().Body.SectPr), nil
}

// watermarkTargetHeadersFor returns the header handles a watermark is applied to
// for the given section: its default header (created if absent) plus any
// first-page and even-page headers the section already references.
func (d *Document) watermarkTargetHeadersFor(sectPr *oxml.CT_SectPr) []*Header {
	var headers []*Header
	hasDefault := false
	for _, ref := range sectPr.HeaderReference {
		if ref == nil || ref.RID == "" {
			continue
		}
		name := d.headerPartForRID(ref.RID)
		if name == "" {
			continue
		}
		hp, ok := d.headers[name]
		if !ok || hp.hdr == nil {
			continue
		}
		if ref.Type == HeaderDefault.xmlVal() {
			hasDefault = true
		}
		headers = append(headers, &Header{document: d, hdr: hp.hdr, relID: ref.RID, partName: name})
	}

	// A watermark must appear on ordinary pages, so ensure a default header
	// exists even when the section only referenced a first-page or even-page
	// header.
	if !hasDefault {
		headers = append(headers, d.addHeaderTo(sectPr, HeaderDefault))
	}
	return headers
}

// headerPartForRID resolves a header relationship id in the main part's scope to
// its target part name, or "" when there is no matching header relationship.
func (d *Document) headerPartForRID(rid string) string {
	for _, rel := range d.relationships[d.mainPart()] {
		if rel.ID == rid && rel.Type == opc.RelTypeHeader {
			return opc.ResolvePartName(d.mainPart(), rel.Target)
		}
	}
	return ""
}

// markHdrFtrModified flags an existing header/footer part for regeneration on
// save. Parts created in this session (not in preservedParts) are written via
// newHeaderParts/newFooterParts and are skipped so they are not written twice.
func (d *Document) markHdrFtrModified(partName string) {
	if _, ok := d.preservedParts[partName]; !ok {
		return
	}
	if d.modifiedHdrFtrParts == nil {
		d.modifiedHdrFtrParts = make(map[string]bool)
	}
	d.modifiedHdrFtrParts[partName] = true
}

// nextWatermarkSeq hands out a monotonically increasing sequence number for
// watermark shape ids and spids.
func (d *Document) nextWatermarkSeq() int {
	d.watermarkSeq++
	return d.watermarkSeq
}

// appendPictParagraph appends a paragraph holding a single run that carries the
// watermark w:pict.
func appendPictParagraph(hdr *oxml.CT_HdrFtr, pict *oxml.CT_RawElement) {
	r := &oxml.CT_R{}
	r.AppendPict(pict)
	p := &oxml.CT_P{}
	p.AppendR(r)
	hdr.AppendP(p)
}

// appendAltContentParagraph appends a paragraph holding a single run that
// carries the watermark mc:AlternateContent (DrawingML choice + VML fallback).
func appendAltContentParagraph(hdr *oxml.CT_HdrFtr, ac *oxml.CT_RawElement) {
	r := &oxml.CT_R{}
	r.AppendAlternateContent(ac)
	p := &oxml.CT_P{}
	p.AppendR(r)
	hdr.AppendP(p)
}

// removeWatermarkParagraphs drops every paragraph that carries a watermark
// w:pict from the header/footer, reporting whether any were removed.
func removeWatermarkParagraphs(hdr *oxml.CT_HdrFtr) bool {
	var targets []*oxml.CT_P
	for _, p := range hdr.Paragraphs() {
		for _, raw := range paragraphPictContents(p) {
			if classifyWatermark(raw) != nil {
				targets = append(targets, p)
				break
			}
		}
	}
	for _, p := range targets {
		hdr.RemoveP(p)
	}
	return len(targets) > 0
}

// paragraphPictContents returns the raw inner XML of every w:pict (and the raw
// mc:AlternateContent, which Word sometimes wraps a watermark in) among the
// paragraph's runs.
func paragraphPictContents(p *oxml.CT_P) [][]byte {
	var out [][]byte
	for _, r := range p.R {
		for _, pict := range r.Pict {
			if pict != nil {
				out = append(out, pict.RawContent)
			}
		}
		for _, ac := range r.AlternateContent {
			if ac != nil {
				out = append(out, ac.RawContent)
			}
		}
	}
	return out
}

// classifyWatermark inspects raw VML inner XML and returns a Watermark when it
// carries a watermark shape, or nil otherwise. Detection keys on the shape id
// prefixes Word assigns to watermarks and, as a fallback, on the WordArt text
// path shape type, so ordinary pictures in a header are not misread.
func classifyWatermark(raw []byte) *Watermark {
	s := string(raw)
	isText := strings.Contains(s, "PowerPlusWaterMarkObject") ||
		(strings.Contains(s, "_x0000_t136") && strings.Contains(s, "textpath"))
	isImage := strings.Contains(s, "WordPictureWatermark")
	if !isText && !isImage {
		return nil
	}
	if isText {
		return &Watermark{Type: WatermarkText, Text: extractTextpathString(s)}
	}
	return &Watermark{Type: WatermarkImage}
}

// extractTextpathString pulls the string attribute of the v:textpath element
// that carries the watermark text, unescaping XML entities. The shape type
// declares a second, string-less v:textpath, so the search keys on the
// string attribute rather than the first textpath element.
func extractTextpathString(s string) string {
	const marker = ` string="`
	sidx := strings.Index(s, marker)
	if sidx < 0 {
		return ""
	}
	val := s[sidx+len(marker):]
	q := strings.IndexByte(val, '"')
	if q < 0 {
		return ""
	}
	return unescapeXMLAttr(val[:q])
}

// unescapeXMLAttr reverses the escaping applied by escapeXMLAttr.
func unescapeXMLAttr(s string) string {
	r := strings.NewReplacer(
		"&#xA;", "\n",
		"&#xD;", "\r",
		"&#x9;", "\t",
		"&quot;", `"`,
		"&gt;", ">",
		"&lt;", "<",
		"&amp;", "&",
	)
	return r.Replace(s)
}

// pictAttrs returns the xmlns declarations placed on a watermark w:pict element.
// r: is already declared on the header root; v:, o: and w10: are not, so they
// are declared here.
func pictAttrs() []xml.Attr {
	return []xml.Attr{
		{Name: xml.Name{Space: "xmlns", Local: "v"}, Value: nsVML},
		{Name: xml.Name{Space: "xmlns", Local: "o"}, Value: nsOfficeVML},
		{Name: xml.Name{Space: "xmlns", Local: "w10"}, Value: nsWordVML},
	}
}

// watermarkShapeStyle builds the VML shape style string common to watermarks,
// centering the shape on the page margins with the given size and z-index.
func watermarkShapeStyle(widthPt, heightPt float64, rotation float64, zIndex int) string {
	var b strings.Builder
	fmt.Fprintf(&b, "position:absolute;margin-left:0;margin-top:0;width:%spt;height:%spt",
		trimFloat(widthPt), trimFloat(heightPt))
	if rotation != 0 {
		fmt.Fprintf(&b, ";rotation:%s", trimFloat(rotation))
	}
	fmt.Fprintf(&b, ";z-index:%d;mso-position-horizontal:center;mso-position-horizontal-relative:margin;"+
		"mso-position-vertical:center;mso-position-vertical-relative:margin", zIndex)
	return b.String()
}

// buildTextWatermarkPict builds the w:pict raw element for a text watermark.
func buildTextWatermarkPict(text string, opts WatermarkOptions, seq int) *oxml.CT_RawElement {
	font := opts.Font
	if font == "" {
		font = "Calibri"
	}
	color := normalizeWatermarkColor(opts.Color)
	widthPt, heightPt := textWatermarkSize(text)
	style := watermarkShapeStyle(widthPt, heightPt, opts.rotationDegrees(), -251658752)
	shapeID := fmt.Sprintf("PowerPlusWaterMarkObject%d", seq)
	spid := fmt.Sprintf("_x0000_s%d", 2049+seq)

	var b strings.Builder
	b.WriteString(vmlTextShapetype)
	fmt.Fprintf(&b, `<v:shape id="%s" o:spid="%s" type="#_x0000_t136" style="%s" o:allowincell="f" fillcolor="%s" stroked="f">`,
		escapeXMLAttr(shapeID), escapeXMLAttr(spid), escapeXMLAttr(style), escapeXMLAttr(color))
	fmt.Fprintf(&b, `<v:textpath style="font-family:&quot;%s&quot;;font-size:1pt" string="%s"/>`,
		escapeXMLAttr(font), escapeXMLAttr(text))
	b.WriteString(`</v:shape>`)

	return &oxml.CT_RawElement{Attrs: pictAttrs(), RawContent: []byte(b.String())}
}

// Namespace URIs used by the DrawingML text-box watermark (the VML namespaces
// nsVML/nsOfficeVML/nsWordVML declare the fallback shape).
const (
	nsDrawingML          = "http://schemas.openxmlformats.org/drawingml/2006/main"
	nsWordprocessingDraw = "http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing"
	nsWordShape          = "http://schemas.microsoft.com/office/word/2010/wordprocessingShape"
)

// altContentWatermarkAttrs returns the xmlns declarations placed on the
// watermark mc:AlternateContent element. mc:, w: and r: are declared on the
// header root; the DrawingML (a/wp/wps) and VML (v/o/w10) namespaces are not, so
// they are declared here.
func altContentWatermarkAttrs() []xml.Attr {
	return []xml.Attr{
		{Name: xml.Name{Space: "xmlns", Local: "wp"}, Value: nsWordprocessingDraw},
		{Name: xml.Name{Space: "xmlns", Local: "a"}, Value: nsDrawingML},
		{Name: xml.Name{Space: "xmlns", Local: "wps"}, Value: nsWordShape},
		{Name: xml.Name{Space: "xmlns", Local: "v"}, Value: nsVML},
		{Name: xml.Name{Space: "xmlns", Local: "o"}, Value: nsOfficeVML},
		{Name: xml.Name{Space: "xmlns", Local: "w10"}, Value: nsWordVML},
	}
}

// buildTextWatermarkDrawingML builds the mc:AlternateContent raw element for a
// text watermark: a DrawingML text box (mc:Choice Requires="wps") with a classic
// VML shape as the mc:Fallback, so DrawingML-aware consumers render the text box
// and others fall back to the VML.
func buildTextWatermarkDrawingML(text string, opts WatermarkOptions, seq int) *oxml.CT_RawElement {
	font := opts.Font
	if font == "" {
		font = "Calibri"
	}
	colorHex := strings.TrimPrefix(normalizeWatermarkColor(opts.Color), "#")
	widthPt, heightPt := textWatermarkSize(text)
	cx, cy := pointsToEMU(widthPt), pointsToEMU(heightPt)
	rot := int64(math.Round(opts.rotationDegrees()*60000)) % 21600000
	if rot < 0 {
		rot += 21600000
	}
	shapeName := fmt.Sprintf("PowerPlusWaterMarkObject%d", seq)

	var b strings.Builder
	// DrawingML choice: an anchored, behind-text text box carrying the text.
	b.WriteString(`<mc:Choice Requires="wps">`)
	b.WriteString(`<w:drawing>`)
	fmt.Fprintf(&b, `<wp:anchor distT="0" distB="0" distL="0" distR="0" simplePos="0" relativeHeight="%d" behindDoc="1" locked="0" layoutInCell="1" allowOverlap="1">`, 251658752+seq)
	b.WriteString(`<wp:simplePos x="0" y="0"/>`)
	b.WriteString(`<wp:positionH relativeFrom="margin"><wp:align>center</wp:align></wp:positionH>`)
	b.WriteString(`<wp:positionV relativeFrom="margin"><wp:align>center</wp:align></wp:positionV>`)
	fmt.Fprintf(&b, `<wp:extent cx="%d" cy="%d"/>`, cx, cy)
	b.WriteString(`<wp:effectExtent l="0" t="0" r="0" b="0"/>`)
	b.WriteString(`<wp:wrapNone/>`)
	fmt.Fprintf(&b, `<wp:docPr id="%d" name="%s"/>`, seq, escapeXMLAttr(shapeName))
	b.WriteString(`<wp:cNvGraphicFramePr/>`)
	fmt.Fprintf(&b, `<a:graphic><a:graphicData uri="%s">`, nsWordShape)
	b.WriteString(`<wps:wsp>`)
	b.WriteString(`<wps:cNvSpPr txBox="1"/>`)
	b.WriteString(`<wps:spPr>`)
	fmt.Fprintf(&b, `<a:xfrm rot="%d"><a:off x="0" y="0"/><a:ext cx="%d" cy="%d"/></a:xfrm>`, rot, cx, cy)
	b.WriteString(`<a:prstGeom prst="rect"><a:avLst/></a:prstGeom>`)
	b.WriteString(`<a:noFill/><a:ln><a:noFill/></a:ln>`)
	b.WriteString(`</wps:spPr>`)
	b.WriteString(`<wps:txbx><w:txbxContent>`)
	b.WriteString(`<w:p><w:pPr><w:jc w:val="center"/></w:pPr>`)
	fmt.Fprintf(&b, `<w:r><w:rPr><w:rFonts w:ascii="%s" w:hAnsi="%s"/><w:color w:val="%s"/><w:sz w:val="72"/></w:rPr>`,
		escapeXMLAttr(font), escapeXMLAttr(font), escapeXMLAttr(colorHex))
	fmt.Fprintf(&b, `<w:t xml:space="preserve">%s</w:t></w:r>`, xmlb.EscapeText(text))
	b.WriteString(`</w:p>`)
	b.WriteString(`</w:txbxContent></wps:txbx>`)
	b.WriteString(`<wps:bodyPr rot="0" vertOverflow="overflow" horzOverflow="overflow" wrap="none" lIns="0" tIns="0" rIns="0" bIns="0" anchor="ctr" anchorCtr="0"><a:noAutofit/></wps:bodyPr>`)
	b.WriteString(`</wps:wsp>`)
	b.WriteString(`</a:graphicData></a:graphic>`)
	b.WriteString(`</wp:anchor>`)
	b.WriteString(`</w:drawing>`)
	b.WriteString(`</mc:Choice>`)
	// VML fallback: the classic watermark shape, reusing the VML builder's body.
	b.WriteString(`<mc:Fallback><w:pict>`)
	b.Write(buildTextWatermarkPict(text, opts, seq).RawContent)
	b.WriteString(`</w:pict></mc:Fallback>`)

	return &oxml.CT_RawElement{Attrs: altContentWatermarkAttrs(), RawContent: []byte(b.String())}
}

// buildImageWatermarkPict builds the w:pict raw element for an image watermark
// referencing the media relationship relID.
func buildImageWatermarkPict(relID string, widthPt, heightPt float64, seq int) *oxml.CT_RawElement {
	style := watermarkShapeStyle(widthPt, heightPt, 0, -251657728)
	shapeID := fmt.Sprintf("WordPictureWatermark%d", seq)
	spid := fmt.Sprintf("_x0000_s%d", 2049+seq)

	var b strings.Builder
	b.WriteString(vmlImageShapetype)
	fmt.Fprintf(&b, `<v:shape id="%s" o:spid="%s" type="#_x0000_t75" style="%s" o:allowincell="f">`,
		escapeXMLAttr(shapeID), escapeXMLAttr(spid), escapeXMLAttr(style))
	// gain/blacklevel wash the image out, the standard watermark rendering.
	fmt.Fprintf(&b, `<v:imagedata r:id="%s" o:title="watermark" gain="19661f" blacklevel="22938f"/>`,
		escapeXMLAttr(relID))
	b.WriteString(`</v:shape>`)

	return &oxml.CT_RawElement{Attrs: pictAttrs(), RawContent: []byte(b.String())}
}

// vmlTextShapetype is the standard WordArt "text on path" shape type (t136)
// referenced by text watermarks.
const vmlTextShapetype = `<v:shapetype id="_x0000_t136" coordsize="21600,21600" o:spt="136" adj="10800" path="m@7,l@8,m@5,21600l@11,21600e">` +
	`<v:formulas>` +
	`<v:f eqn="sum #0 0 10800"/><v:f eqn="prod #0 2 1"/><v:f eqn="sum 21600 0 @1"/><v:f eqn="sum 0 0 @2"/>` +
	`<v:f eqn="sum 21600 0 @3"/><v:f eqn="if @0 @3 0"/><v:f eqn="if @0 21600 @1"/><v:f eqn="if @0 0 @2"/>` +
	`<v:f eqn="if @0 @4 21600"/><v:f eqn="mid @5 @6"/><v:f eqn="mid @8 @5"/><v:f eqn="mid @7 @8"/>` +
	`<v:f eqn="mid @6 @7"/><v:f eqn="sum @6 0 @5"/>` +
	`</v:formulas>` +
	`<v:path textpathok="t" o:connecttype="custom" o:connectlocs="@9,0;@10,10800;@9,21600;@12,10800" o:connectangles="270,180,90,0"/>` +
	`<v:textpath on="t" fitshape="t"/>` +
	`<v:handles><v:h position="#0,bottomRight" xrange="6629,14971"/></v:handles>` +
	`<o:lock v:ext="edit" text="t" shapetype="t"/>` +
	`</v:shapetype>`

// vmlImageShapetype is the standard picture frame shape type (t75) referenced by
// image watermarks.
const vmlImageShapetype = `<v:shapetype id="_x0000_t75" coordsize="21600,21600" o:spt="75" o:preferrelative="t" path="m@4@5l@4@11@9@11@9@5xe" filled="f" stroked="f">` +
	`<v:stroke joinstyle="miter"/>` +
	`<v:formulas>` +
	`<v:f eqn="if lineDrawn pixelLineWidth 0"/><v:f eqn="sum @0 1 0"/><v:f eqn="sum 0 0 @1"/>` +
	`<v:f eqn="prod @2 1 2"/><v:f eqn="prod @3 21600 pixelWidth"/><v:f eqn="prod @3 21600 pixelHeight"/>` +
	`<v:f eqn="sum @0 0 1"/><v:f eqn="prod @6 1 2"/><v:f eqn="prod @7 21600 pixelWidth"/>` +
	`<v:f eqn="sum @8 21600 0"/><v:f eqn="prod @7 21600 pixelHeight"/><v:f eqn="sum @10 21600 0"/>` +
	`</v:formulas>` +
	`<v:path o:extrusionok="f" gradientshapeok="t" o:connecttype="rect"/>` +
	`<o:lock v:ext="edit" aspectratio="t"/>` +
	`</v:shapetype>`

// textWatermarkSize returns a shape width/height in points scaled to the length
// of the watermark text. The WordArt textpath is stretched to fill the box
// (fitshape="t"), so the box aspect governs the rendered text size.
func textWatermarkSize(text string) (widthPt, heightPt float64) {
	n := len([]rune(strings.TrimSpace(text)))
	if n < 1 {
		n = 1
	}
	widthPt = float64(n)*36 + 60
	if widthPt > 585 {
		widthPt = 585
	}
	heightPt = widthPt / 4
	return widthPt, heightPt
}

// fitWatermarkImage scales pixel dimensions into a shape size in points that
// fits within a landscape watermark box while preserving the aspect ratio.
func fitWatermarkImage(pxW, pxH int) (widthPt, heightPt float64) {
	const maxWidthPt = 468  // 6.5 inches
	const maxHeightPt = 585 // ~8 inches
	if pxW <= 0 || pxH <= 0 {
		return maxWidthPt, maxWidthPt
	}
	widthPt = maxWidthPt
	heightPt = widthPt * float64(pxH) / float64(pxW)
	if heightPt > maxHeightPt {
		heightPt = maxHeightPt
		widthPt = heightPt * float64(pxW) / float64(pxH)
	}
	return widthPt, heightPt
}

// watermarkImageType maps an image.DecodeConfig format name to an OPC content
// type and file extension, or "" when the format is unsupported.
func watermarkImageType(format string) (contentType, ext string) {
	switch format {
	case "png":
		return opc.ContentTypePNG, ".png"
	case "jpeg":
		return opc.ContentTypeJPEG, ".jpg"
	case "gif":
		return opc.ContentTypeGIF, ".gif"
	default:
		return "", ""
	}
}

// normalizeWatermarkColor returns a VML fill color ("#rrggbb") from a hex RGB
// string, defaulting to silver.
func normalizeWatermarkColor(color string) string {
	color = strings.TrimSpace(color)
	color = strings.TrimPrefix(color, "#")
	if color == "" {
		color = "C0C0C0"
	}
	return "#" + strings.ToLower(color)
}

// escapeXMLAttr escapes a string for use inside a double-quoted XML attribute in
// the raw VML the watermark builders emit.
func escapeXMLAttr(s string) string {
	r := strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		`"`, "&quot;",
		"\n", "&#xA;",
		"\r", "&#xD;",
		"\t", "&#x9;",
	)
	return r.Replace(s)
}

// trimFloat formats a float without a trailing ".0", keeping style strings
// compact (e.g. "468" not "468.000000").
func trimFloat(f float64) string {
	if f == math.Trunc(f) {
		return fmt.Sprintf("%d", int64(f))
	}
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", f), "0"), ".")
}
