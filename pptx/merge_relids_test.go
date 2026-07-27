package pptx

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
	"testing"

	"github.com/mgilbir/spine/opc"
)

// relNamespace is the OPC relationship namespace that carries r:id attributes.
const relNamespace = "http://schemas.openxmlformats.org/officeDocument/2006/relationships"

// presRelRef is one r:id reference found in presentation.xml, tagged with the
// element that carried it and the relationship type that element requires.
type presRelRef struct {
	elem     string // local name of the referencing element
	relID    string
	wantType string // relationship type the reference must resolve to
}

// scanPresentationRelRefs walks presentation.xml and returns every r:id
// reference from the id lists and custom shows, together with the relationship
// type each reference is required by ECMA-376 to resolve to.
func scanPresentationRelRefs(t *testing.T, data []byte) []presRelRef {
	t.Helper()
	want := map[string]string{
		"sldId":           opc.RelTypeSlide,
		"sldMasterId":     opc.RelTypeSlideMaster,
		"notesMasterId":   opc.RelTypeNotesMaster,
		"handoutMasterId": opc.RelTypeHandoutMaster,
		// p:sld inside p:custShow/p:sldLst references a slide by r:id.
		"sld": opc.RelTypeSlide,
	}
	var out []presRelRef
	dec := xml.NewDecoder(bytes.NewReader(data))
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("decode presentation.xml: %v", err)
		}
		se, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		wantType, tracked := want[se.Name.Local]
		if !tracked {
			continue
		}
		for _, a := range se.Attr {
			if a.Name.Space == relNamespace && a.Name.Local == "id" {
				out = append(out, presRelRef{elem: se.Name.Local, relID: a.Value, wantType: wantType})
			}
		}
	}
	return out
}

// assertPresentationRelIntegrity asserts the three invariants a saved package
// must satisfy for its presentation part (C363):
//
//	(a) relationship ids are unique within presentation.xml.rels;
//	(b) every p:sldId / p:sldMasterId / p:notesMasterId / p:handoutMasterId /
//	    p:custShow r:id resolves to a relationship of the matching type;
//	(c) every internal relationship target exists in the written package.
//
// These are asserted against the OUTPUT zip, not the in-memory model, because
// the collision this guards against is introduced by the save path itself.
func assertPresentationRelIntegrity(t *testing.T, out []byte) {
	t.Helper()
	parts := zipParts(t, out)

	relsData, ok := parts["/ppt/_rels/presentation.xml.rels"]
	if !ok {
		t.Fatal("output has no /ppt/_rels/presentation.xml.rels")
	}
	rels, err := opc.UnmarshalRelationships(relsData)
	if err != nil {
		t.Fatalf("UnmarshalRelationships: %v", err)
	}

	// (a) unique ids within the presentation rels scope.
	byID := make(map[string]*opc.Relationship, len(rels))
	for _, rel := range rels {
		if rel == nil {
			continue
		}
		if prev, dup := byID[rel.ID]; dup {
			t.Errorf("presentation.xml.rels: duplicate relationship id %q (%s -> %s and %s -> %s)",
				rel.ID, prev.Type, prev.Target, rel.Type, rel.Target)
			continue
		}
		byID[rel.ID] = rel
	}

	// (b) every id-list / custom-show reference resolves to the right rel type.
	presData, ok := parts["/ppt/presentation.xml"]
	if !ok {
		t.Fatal("output has no /ppt/presentation.xml")
	}
	for _, ref := range scanPresentationRelRefs(t, presData) {
		rel, ok := byID[ref.relID]
		if !ok {
			t.Errorf("presentation.xml: p:%s references relationship %q with no matching relationship", ref.elem, ref.relID)
			continue
		}
		if rel.Type != ref.wantType {
			t.Errorf("presentation.xml: p:%s r:id=%q resolves to a %q relationship (target %s), want %q",
				ref.elem, ref.relID, rel.Type, rel.Target, ref.wantType)
		}
	}

	// (c) every internal target is a part actually present in the output.
	for _, rel := range rels {
		if rel == nil || rel.TargetMode == opc.TargetModeExternal {
			continue
		}
		target := opc.ResolvePartName("/ppt/presentation.xml", rel.Target)
		if _, ok := parts[target]; !ok {
			t.Errorf("presentation.xml.rels: %s -> %s targets %s, absent from the written package",
				rel.ID, rel.Target, target)
		}
	}
}

// assertAllRelTargetsPresent asserts every internal relationship target in every
// .rels part of the package resolves to a part present in that same package.
func assertAllRelTargetsPresent(t *testing.T, parts map[string][]byte) {
	t.Helper()
	for name, data := range parts {
		if !strings.HasSuffix(name, ".rels") {
			continue
		}
		owner := relsPathToPartName(name)
		rels, err := opc.UnmarshalRelationships(data)
		if err != nil {
			t.Fatalf("UnmarshalRelationships %s: %v", name, err)
		}
		for _, rel := range rels {
			if rel == nil || rel.TargetMode == opc.TargetModeExternal {
				continue
			}
			target := opc.ResolvePartName(owner, rel.Target)
			if _, ok := parts[target]; !ok {
				t.Errorf("%s: %s -> %s targets %s, absent from the written package", name, rel.ID, rel.Target, target)
			}
		}
	}
}

// relsPathToPartName converts "/ppt/_rels/presentation.xml.rels" back to
// "/ppt/presentation.xml" (and "/_rels/.rels" to "/").
func relsPathToPartName(relsPath string) string {
	dir, file := path.Split(relsPath)
	dir = strings.TrimSuffix(dir, "_rels/")
	return dir + strings.TrimSuffix(file, ".rels")
}

// appendOntoOpenedDeck builds the C363 scenario: an opened source appended onto
// an opened destination whose master differs, so the source master is genuinely
// imported while the appended slides' rel ids are still pending.
func appendOntoOpenedDeck(t *testing.T, srcTitles []string, dstSlides int) []byte {
	t.Helper()
	seed := buildDeck(t, srcTitles)
	sb, err := seed.SaveBytes()
	if err != nil {
		t.Fatalf("seed SaveBytes: %v", err)
	}
	src, err := OpenReader(bytes.NewReader(sb), int64(len(sb)))
	if err != nil {
		t.Fatalf("open source: %v", err)
	}

	// Widescreen destination: its master differs from the 4:3 source master, so
	// importMaster genuinely imports rather than deduplicating.
	dseed := CreateWidescreen()
	for i := 0; i < dstSlides; i++ {
		dseed.AddSlide().AddTextBox().TextFrame().SetText(fmt.Sprintf("Dst-%d", i))
	}
	db, err := dseed.SaveBytes()
	if err != nil {
		t.Fatalf("dst seed SaveBytes: %v", err)
	}
	dst, err := OpenReader(bytes.NewReader(db), int64(len(db)))
	if err != nil {
		t.Fatalf("open dst: %v", err)
	}

	if err := dst.AppendSlidesFrom(src); err != nil {
		t.Fatalf("AppendSlidesFrom: %v", err)
	}
	if r := dst.Validate(); r.HasErrors() {
		t.Fatalf("Validate after append: %v", r)
	}
	out, err := dst.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	return out
}

// TestAppendOntoOpenedDestPresentationRelIDs is the C363 regression: appending
// onto an opened destination allocated presentation rel ids from two blind
// allocators, so an imported master's rel took the id AddSlide had already
// handed the pending slide. The save path then kept the master rel and dropped
// the slide rel as a duplicate, leaving p:sldId entries bound to slideMaster and
// notesMaster relationships.
func TestAppendOntoOpenedDestPresentationRelIDs(t *testing.T) {
	for _, tc := range []struct {
		name      string
		srcTitles []string
		dstSlides int
	}{
		{"one-source-slide", []string{"Seed-A"}, 1},
		{"multi-source-slides", []string{"Seed-A", "Seed-B", "Seed-C"}, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out := appendOntoOpenedDeck(t, tc.srcTitles, tc.dstSlides)
			assertPresentationRelIntegrity(t, out)

			parts := zipParts(t, out)
			assertNoDuplicateRelIDs(t, parts)
			assertAllRelTargetsPresent(t, parts)

			// Every appended slide must survive as a real slide part.
			re, err := OpenReader(bytes.NewReader(out), int64(len(out)))
			if err != nil {
				t.Fatalf("OpenReader: %v", err)
			}
			if got, want := re.SlideCount(), tc.dstSlides+len(tc.srcTitles); got != want {
				t.Errorf("reopened SlideCount = %d, want %d", got, want)
			}
		})
	}
}

// TestPresentationRelIDsNeverReused walks the presentation-level allocation
// entry points in sequence and asserts no id is handed out twice, including the
// interleavings that mix a pending AddSlide id with a merge-time import.
func TestPresentationRelIDsNeverReused(t *testing.T) {
	p := Create()
	p.AddSlide()
	sb, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	dst, err := OpenReader(bytes.NewReader(sb), int64(len(sb)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}

	seen := make(map[int]string)
	claim := func(id int, who string) {
		t.Helper()
		if prev, dup := seen[id]; dup {
			t.Errorf("rId%d handed out to both %s and %s", id, prev, who)
			return
		}
		seen[id] = who
	}

	// Interleave the two historical allocators: a pending slide id (AddSlide)
	// followed by a presentation-level furniture id (the merge importers).
	for i := 0; i < 4; i++ {
		s := dst.AddSlide()
		var n int
		if _, err := fmt.Sscanf(s.relID, "rId%d", &n); err != nil {
			t.Fatalf("unparseable slide relID %q", s.relID)
		}
		claim(n, fmt.Sprintf("AddSlide#%d", i))
		claim(dst.nextPresentationRelID(), fmt.Sprintf("nextPresentationRelID#%d", i))
	}
	// Every id already registered on the presentation part must be distinct from
	// the freshly allocated ones too.
	for _, rel := range dst.relationships[presentationPartName] {
		if rel == nil {
			continue
		}
		var n int
		if _, err := fmt.Sscanf(rel.ID, "rId%d", &n); err == nil {
			claim(n, "existing "+rel.Type)
		}
	}

	ids := make([]int, 0, len(seen))
	for id := range seen {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	t.Logf("allocated presentation rel ids: %v", ids)
}
