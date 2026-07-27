package pptx

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"sort"
	"strings"
	"testing"
)

// collectIDAttrs returns the value of the plain (non-namespaced) id attribute of
// every element with the given local name in data, in document order.
func collectIDAttrs(t *testing.T, data []byte, localName string) []string {
	t.Helper()
	var out []string
	dec := xml.NewDecoder(bytes.NewReader(data))
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("decode: %v", err)
		}
		se, ok := tok.(xml.StartElement)
		if !ok || se.Name.Local != localName {
			continue
		}
		for _, a := range se.Attr {
			if a.Name.Space == "" && a.Name.Local == "id" {
				out = append(out, a.Value)
			}
		}
	}
	return out
}

// assertUnique fails when ids contains a repeat, naming what the values are.
func assertUnique(t *testing.T, what string, ids []string) {
	t.Helper()
	seen := map[string]bool{}
	for _, id := range ids {
		if seen[id] {
			sorted := append([]string(nil), ids...)
			sort.Strings(sorted)
			t.Errorf("%s: duplicate id %q (all: %v)", what, id, sorted)
			return
		}
		seen[id] = true
	}
}

// C419: ST_SlideMasterId is document-unique. A destination whose preserved
// master already carries the value the index-derived fallback would produce for
// a later master emitted two sldMasterId entries with the same id.
func TestSlideMasterIDsUniqueWithPreservedID(t *testing.T) {
	// A widescreen destination whose single master carries the id that the old
	// base+index fallback would hand the master at index 1.
	dseed := CreateWidescreen()
	dseed.AddSlide().AddTextBox().TextFrame().SetText("Dst")
	db, err := dseed.SaveBytes()
	if err != nil {
		t.Fatalf("dst SaveBytes: %v", err)
	}
	db = rewriteZipPart(t, db, "ppt/presentation.xml", func(x []byte) []byte {
		return bytes.Replace(x, []byte(`id="2147483648"`), []byte(`id="2147483649"`), 1)
	})
	dst := openBytes(t, db)
	if got := collectIDAttrs(t, zipPart(t, db, "ppt/presentation.xml"), "sldMasterId"); len(got) != 1 || got[0] != "2147483649" {
		t.Fatalf("fixture setup failed, sldMasterId ids = %v", got)
	}

	// Appending a 4:3 source imports its master, which carries no id of its own.
	seed := buildDeck(t, []string{"Seed"})
	sb, err := seed.SaveBytes()
	if err != nil {
		t.Fatalf("seed SaveBytes: %v", err)
	}
	if err := dst.AppendSlidesFrom(openBytes(t, sb)); err != nil {
		t.Fatalf("AppendSlidesFrom: %v", err)
	}
	out, err := dst.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	ids := collectIDAttrs(t, zipPart(t, out, "ppt/presentation.xml"), "sldMasterId")
	if len(ids) != 2 {
		t.Fatalf("sldMasterId count = %d, want 2 (%v)", len(ids), ids)
	}
	assertUnique(t, "sldMasterIdLst", ids)
}

// C419: ST_SlideLayoutId is document-unique too, but a master rebuilding its
// sldLayoutIdLst numbered from a fixed base, so a regenerating master overlapped
// the ids of every other master in the deck.
func TestSlideLayoutIDsUniqueAcrossMasters(t *testing.T) {
	// A created destination's master has no preserved sldLayoutIdLst, so it
	// regenerates; the imported master carries its source ids verbatim, starting
	// at the same base the regenerating one used to claim.
	dst := Create()
	dst.AddSlide().AddTextBox().TextFrame().SetText("Dst")

	seed := buildDeck(t, []string{"Seed"})
	sb, err := seed.SaveBytes()
	if err != nil {
		t.Fatalf("seed SaveBytes: %v", err)
	}
	if err := dst.AppendSlidesFrom(openBytes(t, sb)); err != nil {
		t.Fatalf("AppendSlidesFrom: %v", err)
	}
	if len(dst.slideMasters) < 2 {
		t.Fatalf("fixture setup failed: %d masters, want >= 2", len(dst.slideMasters))
	}
	assertLayoutIDsUnique(t, mustSave(t, dst))
}

// The same invariant when several masters all regenerate their lists: each must
// get its own disjoint block, not the same one.
func TestSlideLayoutIDsUniqueAcrossRegeneratingMasters(t *testing.T) {
	dst := Create()
	dst.AddSlide().AddTextBox().TextFrame().SetText("Dst")

	seed := buildDeck(t, []string{"Seed"})
	sb, err := seed.SaveBytes()
	if err != nil {
		t.Fatalf("seed SaveBytes: %v", err)
	}
	if err := dst.AppendSlidesFrom(openBytes(t, sb)); err != nil {
		t.Fatalf("AppendSlidesFrom: %v", err)
	}
	if len(dst.slideMasters) < 2 {
		t.Fatalf("fixture setup failed: %d masters, want >= 2", len(dst.slideMasters))
	}
	// Force every master to rebuild its id list.
	for _, m := range dst.slideMasters {
		m.layoutsModified = true
	}
	assertLayoutIDsUnique(t, mustSave(t, dst))
}

func mustSave(t *testing.T, p *Presentation) []byte {
	t.Helper()
	out, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	return out
}

// assertLayoutIDsUnique gathers the sldLayoutId ids from every slideMaster part
// of the package and asserts they are unique across the whole document.
func assertLayoutIDsUnique(t *testing.T, out []byte) {
	t.Helper()
	parts := zipParts(t, out)
	var all []string
	masters := 0
	for name, data := range parts {
		if !strings.HasPrefix(name, "/ppt/slideMasters/") || !strings.HasSuffix(name, ".xml") {
			continue
		}
		masters++
		for _, id := range collectIDAttrs(t, data, "sldLayoutId") {
			all = append(all, fmt.Sprintf("%s(%s)", id, name))
		}
	}
	if masters < 2 {
		t.Fatalf("package has %d slideMaster parts, want >= 2", masters)
	}
	bare := make([]string, 0, len(all))
	for _, v := range all {
		bare = append(bare, v[:strings.IndexByte(v, '(')])
	}
	assertUnique(t, "sldLayoutId across all masters", bare)
}
