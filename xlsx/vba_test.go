package xlsx

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mgilbir/spine/internal/testutil"
	"github.com/mgilbir/spine/opc"
)

var testVBABytes = []byte("fake vbaProject.bin blob \x00\x01\x02MSVBA")

func TestVBAInjectRoundTrip(t *testing.T) {
	wb, err := Open("testdata/minimal.xlsx")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if wb.HasMacros() {
		t.Fatal("plain workbook should report no macros")
	}
	if wb.Flavor() != opc.ContentTypeWorkbook {
		t.Fatalf("flavor = %q, want workbook", wb.Flavor())
	}

	wb.SetVBAProject(testVBABytes)
	if !wb.HasMacros() {
		t.Fatal("HasMacros false after SetVBAProject")
	}
	if wb.Flavor() != opc.ContentTypeWorkbookMacroMain {
		t.Fatalf("flavor = %q, want macro-enabled after inject", wb.Flavor())
	}

	data, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	re, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	if !re.HasMacros() {
		t.Fatal("reopened workbook reports no macros")
	}
	if re.Flavor() != opc.ContentTypeWorkbookMacroMain {
		t.Fatalf("reopened flavor = %q, want macro-enabled", re.Flavor())
	}
	if got := re.VBAProject(); !bytes.Equal(got, testVBABytes) {
		t.Fatalf("VBAProject = %q, want %q", got, testVBABytes)
	}
}

func TestVBAUnmodifiedRoundTripByteIdentical(t *testing.T) {
	wb, err := Open("testdata/minimal.xlsx")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	wb.SetVBAProject(testVBABytes)
	macroWb, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	re, err := OpenReader(bytes.NewReader(macroWb), int64(len(macroWb)))
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
	if !re2.HasMacros() || re2.Flavor() != opc.ContentTypeWorkbookMacroMain {
		t.Fatal("macro flavor not preserved across unmodified round-trip")
	}
}

func TestVBARemove(t *testing.T) {
	wb, err := Open("testdata/minimal.xlsx")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	wb.SetVBAProject(testVBABytes)
	macroWb, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	re, err := OpenReader(bytes.NewReader(macroWb), int64(len(macroWb)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	re.RemoveVBAProject()
	if re.HasMacros() {
		t.Fatal("HasMacros true after RemoveVBAProject")
	}
	if re.Flavor() != opc.ContentTypeWorkbook {
		t.Fatalf("flavor = %q, want plain workbook after removal", re.Flavor())
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
		t.Fatal("reopened workbook still reports macros after removal")
	}
	if re2.Flavor() != opc.ContentTypeWorkbook {
		t.Fatalf("reopened flavor = %q, want plain workbook", re2.Flavor())
	}
}

// TestVBARemoveDropsSignature checks that removing a signed project also
// removes the parts its own relationships target and its signature, the project's .rels, and
// their content-type overrides.
func TestVBARemoveDropsSignature(t *testing.T) {
	wb, err := Open("testdata/minimal.xlsx")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	wb.SetVBAProject(testVBABytes)
	macro, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	signed := testutil.AddVBASignature(t, macro, "xl", false)

	re, err := OpenReader(bytes.NewReader(signed), int64(len(signed)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	re.RemoveVBAProject()
	out, err := re.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes after remove: %v", err)
	}
	assertVBADependentsGone(t, out, "xl")
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

// TestVBAReplaceDropsSignature checks that replacing a signed project drops
// the old project's signature, which no longer matches the new bytes.
func TestVBAReplaceDropsSignature(t *testing.T) {
	wb, err := Open("testdata/minimal.xlsx")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	wb.SetVBAProject(testVBABytes)
	macro, err := wb.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	signed := testutil.AddVBASignature(t, macro, "xl", false)

	re, err := OpenReader(bytes.NewReader(signed), int64(len(signed)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	replacement := []byte("replacement vbaProject.bin blob")
	re.SetVBAProject(replacement)
	out, err := re.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes after replace: %v", err)
	}
	parts, err := testutil.ReadZipPartsBytes(out)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := parts["xl/vbaProjectSignature.bin"]; ok {
		t.Error("the old project's signature is still in the package")
	}
	if ct := string(parts["[Content_Types].xml"]); strings.Contains(ct, "vbaProjectSignature") {
		t.Errorf("content types still name the signature:\n%s", ct)
	}
	if _, ok := parts["xl/_rels/vbaProject.bin.rels"]; ok {
		t.Error("the emptied project .rels is still in the package")
	}
	re2, err := OpenReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatalf("OpenReader after replace: %v", err)
	}
	if !bytes.Equal(re2.VBAProject(), replacement) {
		t.Error("the replacement project was not written")
	}
}
