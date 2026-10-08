package pptx

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mgilbir/spine/internal/testutil"
	"github.com/mgilbir/spine/opc"
)

var testVBABytes = []byte("fake vbaProject.bin blob \x00\x01\x02MSVBA")

func TestVBAInjectRoundTrip(t *testing.T) {
	pres, err := Open("testdata/minimal.pptx")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if pres.HasMacros() {
		t.Fatal("plain presentation should report no macros")
	}
	if pres.Flavor() != opc.ContentTypePresentationMain {
		t.Fatalf("flavor = %q, want presentation", pres.Flavor())
	}

	pres.SetVBAProject(testVBABytes)
	if !pres.HasMacros() {
		t.Fatal("HasMacros false after SetVBAProject")
	}
	if pres.Flavor() != opc.ContentTypePresentationMacroMain {
		t.Fatalf("flavor = %q, want macro-enabled after inject", pres.Flavor())
	}

	data, err := pres.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	re, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	if !re.HasMacros() {
		t.Fatal("reopened presentation reports no macros")
	}
	if re.Flavor() != opc.ContentTypePresentationMacroMain {
		t.Fatalf("reopened flavor = %q, want macro-enabled", re.Flavor())
	}
	if got := re.VBAProject(); !bytes.Equal(got, testVBABytes) {
		t.Fatalf("VBAProject = %q, want %q", got, testVBABytes)
	}
}

func TestVBAUnmodifiedRoundTripByteIdentical(t *testing.T) {
	pres, err := Open("testdata/minimal.pptx")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	pres.SetVBAProject(testVBABytes)
	macroPptx, err := pres.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	re, err := OpenReader(bytes.NewReader(macroPptx), int64(len(macroPptx)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	before := append([]byte(nil), re.VBAProject()...)
	again, err := re.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes 2: %v", err)
	}
	re2, err := OpenReader(bytes.NewReader(again), int64(len(again)))
	if err != nil {
		t.Fatalf("OpenReader 2: %v", err)
	}
	if !bytes.Equal(re2.VBAProject(), before) {
		t.Fatal("vbaProject bytes drifted across an unmodified round-trip")
	}
	if !re2.HasMacros() || re2.Flavor() != opc.ContentTypePresentationMacroMain {
		t.Fatal("macro flavor not preserved across unmodified round-trip")
	}
}

func TestVBARemove(t *testing.T) {
	pres, err := Open("testdata/minimal.pptx")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	pres.SetVBAProject(testVBABytes)
	macroPptx, err := pres.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	re, err := OpenReader(bytes.NewReader(macroPptx), int64(len(macroPptx)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	re.RemoveVBAProject()
	if re.HasMacros() {
		t.Fatal("HasMacros true after RemoveVBAProject")
	}
	if re.Flavor() != opc.ContentTypePresentationMain {
		t.Fatalf("flavor = %q, want plain presentation after removal", re.Flavor())
	}

	out, err := re.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes after remove: %v", err)
	}
	re2, err := OpenReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatalf("OpenReader after remove: %v", err)
	}
	if re2.HasMacros() {
		t.Fatal("reopened presentation still reports macros after removal")
	}
	if re2.Flavor() != opc.ContentTypePresentationMain {
		t.Fatalf("reopened flavor = %q, want plain presentation", re2.Flavor())
	}
}

// TestVBARemoveDropsSignature checks that removing a signed project also
// removes the parts its own relationships target and its signature, the project's .rels, and
// their content-type overrides.
func TestVBARemoveDropsSignature(t *testing.T) {
	pres, err := Open("testdata/minimal.pptx")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	pres.SetVBAProject(testVBABytes)
	macro, err := pres.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	signed := testutil.AddVBASignature(t, macro, "ppt", false)

	re, err := OpenReader(bytes.NewReader(signed), int64(len(signed)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	re.RemoveVBAProject()
	out, err := re.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes after remove: %v", err)
	}
	assertVBADependentsGone(t, out, "ppt")
	if _, err := OpenReader(bytes.NewReader(out), int64(len(out))); err != nil {
		t.Fatalf("OpenReader after remove: %v", err)
	}
}

// assertVBADependentsGone checks a saved package for the parts a signed VBA
// project carries and their content-type overrides.
func assertVBADependentsGone(t *testing.T, saved []byte, dir string) {
	t.Helper()
	parts, err := testutil.ReadZipPartsBytes(saved)
	if err != nil {
		t.Fatalf("read saved package: %v", err)
	}
	for _, name := range []string{
		dir + "/vbaProject.bin", dir + "/_rels/vbaProject.bin.rels",
		dir + "/vbaProjectSignature.bin", dir + "/vbaData.xml",
	} {
		if _, ok := parts[name]; ok {
			t.Errorf("%s is still in the package", name)
		}
	}
	if ct := string(parts["[Content_Types].xml"]); strings.Contains(ct, "vbaProject") || strings.Contains(ct, "vbaData") {
		t.Errorf("content types still name a VBA part:\n%s", ct)
	}
}
