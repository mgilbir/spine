package docx

import (
	"bytes"
	"testing"

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
}
