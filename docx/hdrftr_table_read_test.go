package docx

import (
	"testing"

	"github.com/mgilbir/spine/docx/internal/oxml"
	"github.com/mgilbir/spine/opc"
)

// C299: the document-wide read scans descend into header/footer tables. An image
// and a hyperlink living in a table cell of a header must be reported by
// Images() and Hyperlinks() (the godoc promises headers and footers), where the
// previous hdr.P-only walk skipped table-nested content.
func TestHeaderTableImageAndHyperlink_Found(t *testing.T) {
	d := Create()
	const hdrName = "/word/header1.xml"

	hl := &oxml.CT_Hyperlink{RID: "rId10", History: "1"}
	hl.R = []*oxml.CT_R{{T: []*oxml.CT_Text{{Text: "click"}}}}

	imgRun := &oxml.CT_R{}
	imgRun.AppendDrawing(&oxml.CT_Drawing{RawContent: []byte(
		`<wp:inline><wp:extent cx="100" cy="100"/><a:graphic><a:graphicData>` +
			`<pic:pic><pic:blipFill><a:blip r:embed="rId11"/></pic:blipFill></pic:pic>` +
			`</a:graphicData></a:graphic></wp:inline>`)})

	cellP := &oxml.CT_P{
		Hyperlink: []*oxml.CT_Hyperlink{hl},
		R:         []*oxml.CT_R{imgRun},
	}
	tbl := &oxml.CT_Tbl{Tr: []*oxml.CT_Tr{{Tc: []*oxml.CT_Tc{{P: []*oxml.CT_P{cellP}}}}}}
	hdr := &oxml.CT_HdrFtr{Tbl: []*oxml.CT_Tbl{tbl}}

	d.headers[hdrName] = &headerPart{hdr: hdr}
	d.relationships[hdrName] = []*opc.Relationship{
		{ID: "rId10", Type: opc.RelTypeHyperlink, Target: "http://example.com/", TargetMode: opc.TargetModeExternal},
		{ID: "rId11", Type: opc.RelTypeImage, Target: "media/image1.png"},
	}

	imgs := d.Images()
	if len(imgs) != 1 {
		t.Fatalf("Images() = %d, want 1 (image in header table cell)", len(imgs))
	}
	if got := imgs[0].PartName(); got != "/word/media/image1.png" {
		t.Errorf("header image PartName = %q, want /word/media/image1.png", got)
	}

	hls := d.Hyperlinks()
	if len(hls) != 1 {
		t.Fatalf("Hyperlinks() = %d, want 1 (hyperlink in header table cell)", len(hls))
	}
	if got := hls[0].URL(); got != "http://example.com/" {
		t.Errorf("header hyperlink URL = %q, want http://example.com/", got)
	}
}
