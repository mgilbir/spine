package pptx

import (
	"archive/zip"
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/mgilbir/spine/opc"
)

// zipParts reads every entry of an OPC package into a name->bytes map. Names are
// normalized to the leading-slash part-name form (e.g. "/ppt/...").
func zipParts(t *testing.T, data []byte) map[string][]byte {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("zip.NewReader: %v", err)
	}
	out := make(map[string][]byte, len(zr.File))
	for _, f := range zr.File {
		rc, err := f.Open()
		if err != nil {
			t.Fatalf("open %s: %v", f.Name, err)
		}
		b, err := io.ReadAll(rc)
		_ = rc.Close()
		if err != nil {
			t.Fatalf("read %s: %v", f.Name, err)
		}
		out["/"+f.Name] = b
	}
	return out
}

// assertNoDuplicateRelIDs parses every *.rels part in the package and fails when
// any single .rels lists a relationship Id more than once (an OPC violation that
// makes PowerPoint offer to repair the file).
func assertNoDuplicateRelIDs(t *testing.T, parts map[string][]byte) {
	t.Helper()
	for name, data := range parts {
		if !strings.HasSuffix(name, ".rels") {
			continue
		}
		rels, err := opc.UnmarshalRelationships(data)
		if err != nil {
			t.Fatalf("UnmarshalRelationships %s: %v", name, err)
		}
		seen := make(map[string]bool, len(rels))
		for _, rel := range rels {
			if rel == nil {
				continue
			}
			if seen[rel.ID] {
				t.Errorf("%s: duplicate relationship Id %q", name, rel.ID)
			}
			seen[rel.ID] = true
		}
	}
}

// embedIDs returns every r:embed="rIdN" id referenced in the given part bytes.
func embedIDs(xmlBytes []byte) []string {
	const marker = `r:embed="`
	s := string(xmlBytes)
	var ids []string
	for {
		i := strings.Index(s, marker)
		if i < 0 {
			break
		}
		s = s[i+len(marker):]
		j := strings.IndexByte(s, '"')
		if j < 0 {
			break
		}
		ids = append(ids, s[:j])
		s = s[j+1:]
	}
	return ids
}

// srcDeckWithMasterAndLayoutImages builds a created deck whose slide master and
// the layout used by slide 0 each carry a real image-background relationship
// (beyond layouts+theme), so a merge must carry those media parts and keep the
// r:embed references bound to them.
func srcDeckWithMasterAndLayoutImages(t *testing.T) *Presentation {
	t.Helper()
	src := Create()
	master := src.SlideMasters()[0]
	if err := master.SetBackgroundImage(createMinimalPNG(), opc.ContentTypePNG); err != nil {
		t.Fatalf("master SetBackgroundImage: %v", err)
	}
	layout := master.GetLayout(LayoutTitleAndContent)
	if layout == nil {
		t.Fatalf("no TitleAndContent layout")
	}
	if err := layout.SetBackgroundImage(createMinimalPNG(), opc.ContentTypePNG); err != nil {
		t.Fatalf("layout SetBackgroundImage: %v", err)
	}
	s := src.AddSlideFromLayout(layout)
	s.AddTextBox().TextFrame().SetText("Src-0")
	return src
}

// TestExtractSlidesNoDuplicateRelIDs is the C236 regression: extracting a slide
// whose master/layout carry a real relationship beyond layouts+theme must not
// emit any .rels with a duplicated relationship Id.
func TestExtractSlidesNoDuplicateRelIDs(t *testing.T) {
	src := srcDeckWithMasterAndLayoutImages(t)

	out, err := src.ExtractSlides([]int{0})
	if err != nil {
		t.Fatalf("ExtractSlides: %v", err)
	}
	if r := out.Validate(); r.HasErrors() {
		t.Fatalf("Validate extracted: %v", r)
	}
	data, err := out.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	parts := zipParts(t, data)
	assertNoDuplicateRelIDs(t, parts)

	// Reopening must also succeed with a clean package.
	re, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	if r := re.Validate(); r.HasErrors() {
		t.Fatalf("Validate reopened: %v", r)
	}
}

// TestAppendSlidesNoDuplicateMasterRels is the C236 regression for the append
// path: appending into a created deck must not duplicate the theme or
// presentation->master relationships.
func TestAppendSlidesNoDuplicateMasterRels(t *testing.T) {
	src := srcDeckWithMasterAndLayoutImages(t)

	dst := Create()
	dst.AddSlide().AddTextBox().TextFrame().SetText("Dst-0")
	if err := dst.AppendSlidesFrom(src); err != nil {
		t.Fatalf("AppendSlidesFrom: %v", err)
	}
	if r := dst.Validate(); r.HasErrors() {
		t.Fatalf("Validate after append: %v", r)
	}
	data, err := dst.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	parts := zipParts(t, data)
	assertNoDuplicateRelIDs(t, parts)

	// No .rels may list the same (type,target) twice either — the specific shape
	// of the pre-fix bug for the theme and presentation->master rels.
	for name, b := range parts {
		if !strings.HasSuffix(name, ".rels") {
			continue
		}
		rels, err := opc.UnmarshalRelationships(b)
		if err != nil {
			t.Fatalf("UnmarshalRelationships %s: %v", name, err)
		}
		type key struct{ t, tgt string }
		seen := make(map[key]bool, len(rels))
		for _, rel := range rels {
			if rel == nil || rel.TargetMode == opc.TargetModeExternal {
				continue
			}
			k := key{rel.Type, rel.Target}
			if seen[k] {
				t.Errorf("%s: duplicate relationship (type=%s target=%s)", name, rel.Type, rel.Target)
			}
			seen[k] = true
		}
	}
}

// TestExtractSlidesCarriesMasterLayoutMedia is the C237 regression: the
// master's and layout's image parts must be present in the output package and
// their r:embed references must resolve to those image parts, not to a theme.
func TestExtractSlidesCarriesMasterLayoutMedia(t *testing.T) {
	src := srcDeckWithMasterAndLayoutImages(t)

	out, err := src.ExtractSlides([]int{0})
	if err != nil {
		t.Fatalf("ExtractSlides: %v", err)
	}
	data, err := out.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	parts := zipParts(t, data)

	// Every master and layout that carries an r:embed must resolve it to an
	// image part present in the package (never to a theme part).
	checked := 0
	for name, body := range parts {
		isMaster := strings.HasPrefix(name, "/ppt/slideMasters/") && strings.HasSuffix(name, ".xml")
		isLayout := strings.HasPrefix(name, "/ppt/slideLayouts/") && strings.HasSuffix(name, ".xml")
		if !isMaster && !isLayout {
			continue
		}
		ids := embedIDs(body)
		if len(ids) == 0 {
			continue
		}
		relsName := partNameToRelsPath(name)
		relsData, ok := parts[relsName]
		if !ok {
			t.Fatalf("%s references media but has no .rels", name)
		}
		rels, err := opc.UnmarshalRelationships(relsData)
		if err != nil {
			t.Fatalf("UnmarshalRelationships %s: %v", relsName, err)
		}
		byID := make(map[string]*opc.Relationship, len(rels))
		for _, rel := range rels {
			byID[rel.ID] = rel
		}
		for _, id := range ids {
			rel, ok := byID[id]
			if !ok {
				t.Errorf("%s: r:embed %q has no relationship", name, id)
				continue
			}
			if rel.Type != opc.RelTypeImage {
				t.Errorf("%s: r:embed %q bound to %s, want an image relationship", name, id, rel.Type)
				continue
			}
			target := opc.ResolvePartName(name, rel.Target)
			if _, ok := parts[target]; !ok {
				t.Errorf("%s: image part %q missing from package", name, target)
				continue
			}
			if !strings.HasPrefix(target, "/ppt/media/") {
				t.Errorf("%s: r:embed %q resolves to %q, want a media part", name, id, target)
			}
			checked++
		}
	}
	if checked < 2 {
		t.Fatalf("expected to verify both master and layout image embeds, checked %d", checked)
	}
}
