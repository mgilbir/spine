package pptx

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mgilbir/spine/internal/testutil"
)

// Real-world PPTX packages (Common Crawl corpus) carry junk zip entries like
// [trash]/0000.dat with no [Content_Types].xml entry. Round-tripping such a
// package must preserve the entry exactly — including its missing
// content-type status — instead of failing the save with
// ErrInvalidContentType.
func TestRoundTripPreservesPartWithoutContentType(t *testing.T) {
	fixture := testutil.AppendZipEntry(t, "testdata/minimal.pptx", "[trash]/0000.dat", []byte{0xde, 0xad, 0xbe, 0xef})

	p, err := OpenReader(bytes.NewReader(fixture), int64(len(fixture)))
	if err != nil {
		t.Fatalf("Open fixture: %v", err)
	}
	out, err := p.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	_ = p.Close()

	missing, extra, changed := testutil.CompareZipBytes(t, fixture, out)
	if len(missing)+len(extra)+len(changed) > 0 {
		t.Errorf("round trip not part-identical: missing=%v extra=%v changed=%v", missing, extra, changed)
	}

	outParts, err := testutil.ReadZipPartsBytes(out)
	if err != nil {
		t.Fatalf("read output zip: %v", err)
	}
	if !bytes.Equal(outParts["[trash]/0000.dat"], []byte{0xde, 0xad, 0xbe, 0xef}) {
		t.Errorf("trash entry content changed: %v", outParts["[trash]/0000.dat"])
	}
	// The output must not have grown a content-type entry for the junk part.
	if ct := string(outParts["[Content_Types].xml"]); strings.Contains(ct, "trash") || strings.Contains(ct, "dat") {
		t.Errorf("[Content_Types].xml gained an entry for the junk part:\n%s", ct)
	}

	// The saved package must reopen.
	if _, err := OpenReader(bytes.NewReader(out), int64(len(out))); err != nil {
		t.Fatalf("reopen saved package: %v", err)
	}
}
