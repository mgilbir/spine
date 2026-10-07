package docx

import (
	"bytes"
	"strings"
	"testing"

	"github.com/mgilbir/spine/internal/testutil"
	"github.com/mgilbir/spine/opc"
)

var testVBABytes = []byte("fake vbaProject.bin blob \x00\x01\x02MSVBA")

func TestVBAInjectRoundTrip(t *testing.T) {
	doc, err := Open("testdata/minimal.docx")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if doc.HasMacros() {
		t.Fatal("plain document should report no macros")
	}
	if doc.Flavor() != opc.ContentTypeDocument {
		t.Fatalf("flavor = %q, want document", doc.Flavor())
	}
	if doc.VBAProject() != nil {
		t.Fatal("VBAProject on plain document should be nil")
	}

	doc.SetVBAProject(testVBABytes)
	if !doc.HasMacros() {
		t.Fatal("HasMacros false after SetVBAProject")
	}
	if doc.Flavor() != opc.ContentTypeDocumentMacroMain {
		t.Fatalf("flavor = %q, want macro-enabled after inject", doc.Flavor())
	}

	data, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	re, err := OpenReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	if !re.HasMacros() {
		t.Fatal("reopened document reports no macros")
	}
	if re.Flavor() != opc.ContentTypeDocumentMacroMain {
		t.Fatalf("reopened flavor = %q, want macro-enabled", re.Flavor())
	}
	if got := re.VBAProject(); !bytes.Equal(got, testVBABytes) {
		t.Fatalf("VBAProject = %q, want %q", got, testVBABytes)
	}

	// The vbaProject part and its main relationship must be declared.
	if !re.partHasVBARel() {
		t.Fatal("reopened document has no vbaProject relationship")
	}
}

// partHasVBARel is a test helper: reports whether the main part carries a VBA
// relationship.
func (d *Document) partHasVBARel() bool { return d.vbaRelID() != "" }

func TestVBAUnmodifiedRoundTripByteIdentical(t *testing.T) {
	// Build a macro-enabled document by injecting and saving.
	doc, err := Open("testdata/minimal.docx")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	doc.SetVBAProject(testVBABytes)
	macroDoc, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	// Reopen and save again WITHOUT touching the VBA project: the part must be
	// preserved byte-for-byte.
	re, err := OpenReader(bytes.NewReader(macroDoc), int64(len(macroDoc)))
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
	if !re2.HasMacros() || re2.Flavor() != opc.ContentTypeDocumentMacroMain {
		t.Fatal("macro flavor not preserved across unmodified round-trip")
	}
}

func TestVBARemove(t *testing.T) {
	doc, err := Open("testdata/minimal.docx")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	doc.SetVBAProject(testVBABytes)
	macroDoc, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}

	re, err := OpenReader(bytes.NewReader(macroDoc), int64(len(macroDoc)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	if !re.HasMacros() {
		t.Fatal("expected macros before removal")
	}
	re.RemoveVBAProject()
	if re.HasMacros() {
		t.Fatal("HasMacros true after RemoveVBAProject")
	}
	if re.Flavor() != opc.ContentTypeDocument {
		t.Fatalf("flavor = %q, want plain document after removal", re.Flavor())
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
		t.Fatal("reopened document still reports macros after removal")
	}
	if re2.Flavor() != opc.ContentTypeDocument {
		t.Fatalf("reopened flavor = %q, want plain document", re2.Flavor())
	}
	if re2.VBAProject() != nil {
		t.Fatal("VBAProject not nil after removal")
	}
	if ct := zipEntryString(t, out, "[Content_Types].xml"); strings.Contains(ct, "vbaProject.bin") {
		t.Errorf("content types still name the removed VBA project:\n%s", ct)
	}
}

// TestVBARemoveDropsSignature checks that removing a signed project also
// removes the parts its own relationships target, its signature and Word's macro data, the project's .rels, and
// their content-type overrides.
func TestVBARemoveDropsSignature(t *testing.T) {
	doc, err := Open("testdata/minimal.docx")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	doc.SetVBAProject(testVBABytes)
	macro, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	signed := testutil.AddVBASignature(t, macro, "word", true)

	re, err := OpenReader(bytes.NewReader(signed), int64(len(signed)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	re.RemoveVBAProject()
	out, err := re.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes after remove: %v", err)
	}
	assertVBADependentsGone(t, out, "word")
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

// TestVBARemoveKeepsSharedDependent checks that a part the project's
// relationships target is kept when another relationship still targets it.
func TestVBARemoveKeepsSharedDependent(t *testing.T) {
	doc, err := Open("testdata/minimal.docx")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	doc.SetVBAProject(testVBABytes)
	macro, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	signed := testutil.AddVBASignature(t, macro, "word", true)
	rels := zipEntryString(t, signed, "word/_rels/document.xml.rels")
	signed = rewriteZipEntry(t, signed, "word/_rels/document.xml.rels", strings.Replace(rels, "</Relationships>",
		`<Relationship Id="rId99" Type="http://example.com/relationships/other" Target="vbaData.xml"/></Relationships>`, 1))

	re, err := OpenReader(bytes.NewReader(signed), int64(len(signed)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	re.RemoveVBAProject()
	out, err := re.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes after remove: %v", err)
	}
	if _, ok := zipEntry(t, out, "word/vbaData.xml"); !ok {
		t.Error("a part another relationship still targets was dropped")
	}
	if _, ok := zipEntry(t, out, "word/vbaProjectSignature.bin"); ok {
		t.Error("the unshared signature part is still in the package")
	}
	if _, ok := zipEntry(t, out, "word/_rels/vbaProject.bin.rels"); ok {
		t.Error("the removed project's .rels is still in the package")
	}
}

// TestVBAReplaceDropsSignature checks that replacing a signed project drops
// the old project's signature, which no longer matches the new bytes.
func TestVBAReplaceDropsSignature(t *testing.T) {
	doc, err := Open("testdata/minimal.docx")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	doc.SetVBAProject(testVBABytes)
	macro, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	signed := testutil.AddVBASignature(t, macro, "word", true)

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
	if _, ok := parts["word/vbaProjectSignature.bin"]; ok {
		t.Error("the old project's signature is still in the package")
	}
	if ct := string(parts["[Content_Types].xml"]); strings.Contains(ct, "vbaProjectSignature") {
		t.Errorf("content types still name the signature:\n%s", ct)
	}
	if _, ok := parts["word/vbaData.xml"]; !ok {
		t.Error("Word's macro data part was dropped with the signatures")
	}
	if rels := string(parts["word/_rels/vbaProject.bin.rels"]); !strings.Contains(rels, "vbaData.xml") || strings.Contains(rels, "Signature") {
		t.Errorf("project .rels after replacement:\n%s", rels)
	}
	re2, err := OpenReader(bytes.NewReader(out), int64(len(out)))
	if err != nil {
		t.Fatalf("OpenReader after replace: %v", err)
	}
	if !bytes.Equal(re2.VBAProject(), replacement) {
		t.Error("the replacement project was not written")
	}
}

// TestVBARemoveDropsBinDefault checks that removing a project whose content
// type came from a "bin" Default also removes that Default.
func TestVBARemoveDropsBinDefault(t *testing.T) {
	doc, err := Open("testdata/minimal.docx")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	doc.SetVBAProject(testVBABytes)
	macro, err := doc.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes: %v", err)
	}
	macro = testutil.UseBinDefaultForVBA(t, macro, "word")

	re, err := OpenReader(bytes.NewReader(macro), int64(len(macro)))
	if err != nil {
		t.Fatalf("OpenReader: %v", err)
	}
	if !re.HasMacros() {
		t.Fatal("fixture lost its macros")
	}
	re.RemoveVBAProject()
	out, err := re.SaveBytes()
	if err != nil {
		t.Fatalf("SaveBytes after remove: %v", err)
	}
	assertVBADependentsGone(t, out, "word")
	if _, err := OpenReader(bytes.NewReader(out), int64(len(out))); err != nil {
		t.Fatalf("OpenReader after remove: %v", err)
	}
}
